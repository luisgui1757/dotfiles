package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPrivatePythonConsoleIsolationAndLegacyRecovery(t *testing.T) {
	for _, name := range []string{"latex2text", "yamllint"} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", name, legacy), func(t *testing.T) {
				fixture := latexPreparationFixture
				if name == "yamllint" {
					fixture = yamllintPreparationFixture
				}
				d, _ := fixture(t)
				old := d.Pins["tool.shared"]
				latest := old
				latest.Commands = map[string]string{name: "commands/" + filepath.Base(old.Commands[name])}
				latest.BinDirs = []string{"commands"}
				if !legacy {
					d.Pins["tool.shared"] = latest
				}
				r := Resource{ID: "tool.shared"}
				receipt := Receipt{OperationID: strings.Repeat("b", 64), Status: "in-progress"}
				before, err := d.Observe(context.Background(), r, receipt)
				if err != nil {
					t.Fatal(err)
				}
				run, interrupted := d.Run, false
				d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
					if legacy && !interrupted && slices.Contains(command.Arguments, "pip") {
						interrupted = true
						return nil, errors.New("interrupted legacy preparation")
					}
					return run(ctx, command)
				}
				_, err = d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt)
				if legacy {
					if err == nil {
						t.Fatal("interrupted preparation succeeded")
					}
					d.Pins["tool.shared"] = latest
					observed, observeErr := d.Observe(context.Background(), r, receipt)
					if observeErr != nil || observed.ResourceResume == nil {
						t.Fatal(observed, observeErr)
					}
					_, err = d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
				}
				if err != nil {
					t.Fatal(err)
				}
				payload, err := d.PayloadPath(r.ID)
				if err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(payload, filepath.FromSlash(old.Commands[name]))
				if legacy {
					if _, err := os.Stat(source); err != nil {
						t.Fatal("saved legacy console was not prepared", err)
					}
					if _, err := os.Lstat(filepath.Join(payload, "commands")); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("new console recipe rewrote saved legacy intent", err)
					}
					return
				}
				console := filepath.Join(payload, filepath.FromSlash(latest.Commands[name]))
				nativePrivatePythonConsoleDirectory(t, console)
				original, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				copied, err := os.ReadFile(console)
				if err != nil || !bytes.Equal(original, copied) {
					t.Fatal("generated console changed while publishing its public command", err)
				}
			})
		}
	}
}

func TestPrivatePythonWindowsRecipeIdentityAndLegacyShape(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"tool.latex2text", "tool.yamllint"} {
			pin := pins[name]
			want := 0
			if target.OS == "windows" {
				want = 1
			}
			if pin.WindowsVenvRevision != want {
				t.Fatal(target, name, "wrong preparation revision")
			}
			current := archivePinID(pin)
			name := strings.TrimPrefix(name, "tool.")
			legacyDirectory, suffix := "bin", ""
			if target.OS == "windows" {
				legacyDirectory, suffix = "Scripts", ".exe"
			}
			if pin.Commands[name] != "commands/"+name+suffix || len(pin.BinDirs) != 1 || pin.BinDirs[0] != "commands" {
				t.Fatal("private environment remains on the public command path", target, name)
			}
			if target.OS == "windows" {
				invalid := pin
				invalid.WindowsVenvRevision = 0
				if invalid.Validate() == nil {
					t.Fatal("new Windows console accepted legacy venv preparation")
				}
			}
			pin.Commands = map[string]string{name: legacyDirectory + "/" + name + suffix}
			pin.BinDirs = []string{legacyDirectory}
			pin.WindowsVenvRevision = 0
			data, err := json.Marshal(pin)
			if err != nil || strings.Contains(string(data), "windows_venv_revision") {
				t.Fatal("legacy serialized shape changed", err)
			}
			var restored ArchivePin
			if err := Decode(data, &restored); err != nil || restored.Validate() != nil || archivePinID(restored) != archivePinID(pin) {
				t.Fatal("legacy pin no longer reads with its original identity", err)
			}
			if archivePinID(restored) == current {
				t.Fatal("isolated commands require a new desired identity", target, name)
			}
			for _, invalid := range []int{-1, 2} {
				pin.WindowsVenvRevision = invalid
				if pin.Validate() == nil {
					t.Fatal("unreviewed preparation accepted", invalid)
				}
			}
			if target.OS != "windows" {
				pin.WindowsVenvRevision = 1
				if pin.Validate() == nil {
					t.Fatal("Windows preparation accepted for POSIX layout")
				}
			}
		}
		base := pins["tool.python"]
		base.WindowsVenvRevision = 1
		if base.Validate() == nil {
			t.Fatal("private-environment preparation accepted for base Python")
		}
	}
}
