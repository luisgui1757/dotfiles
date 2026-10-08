package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type macOSPrerequisiteFixture struct {
	d                                      *MacOSPrerequisiteDriver
	root                                   string
	commands                               []string
	selectedError, findError, versionError error
	selected, outside, version, sdk        string
}

func macOSPrerequisitesFixture(t *testing.T) *macOSPrerequisiteFixture {
	t.Helper()
	if os.PathSeparator != '/' {
		t.Skip("Apple developer tools use POSIX paths")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := configureMacOSPrerequisites(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	f := &macOSPrerequisiteFixture{d: d, root: root, selected: root, sdk: filepath.Join(root, "SDKs", "macos.sdk")}
	if err := os.MkdirAll(f.sdk, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clang", "clang++", "make", "pbcopy", "pbpaste"} {
		path := filepath.Join(root, "usr", "bin", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture executable; never executed"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	d.clipboardCommands = []string{filepath.Join(root, "usr", "bin", "pbcopy"), filepath.Join(root, "usr", "bin", "pbpaste")}
	d.Query = func(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged || len(input) != 0 {
			t.Fatal("prerequisite observation attempted mutation")
		}
		f.commands = append(f.commands, program+" "+strings.Join(args, " "))
		switch program {
		case "/usr/bin/xcode-select":
			return []byte(f.selected + "\n"), f.selectedError
		case "/usr/bin/xcrun":
			if slices.Contains(args, "--show-sdk-path") {
				return []byte(f.sdk + "\n"), f.findError
			}
			if f.outside != "" {
				return []byte(f.outside + "\n"), nil
			}
			return []byte(filepath.Join(root, "usr", "bin", args[len(args)-1]) + "\n"), f.findError
		default:
			if f.version != "" {
				return []byte(f.version), f.versionError
			}
			if filepath.Base(program) == "make" {
				return []byte("GNU Make 3.81\nfixture build\n"), f.versionError
			}
			return []byte("Apple clang version 17.0.0\nfixture build\n"), f.versionError
		}
	}
	return f
}

func TestMacOSPrerequisitesReuseSelectedCommandsWithoutOwnership(t *testing.T) {
	f := macOSPrerequisitesFixture(t)
	for _, id := range []string{"tool.compiler", "tool.make", "tool.clipboard"} {
		r := Resource{ID: id}
		o, err := f.d.Observe(context.Background(), r, Receipt{})
		if err != nil || !o.Present || !o.Healthy || o.Fingerprint == "" || o.CompletedOperation != "" || o.Pending != "" {
			t.Fatal(id, o, err)
		}
		if _, err := f.d.Apply(context.Background(), r, Operation{Action: "install"}, Receipt{}); err == nil {
			t.Fatal("OS component claimed as newly installed", id)
		}
		if _, err := f.d.Remove(context.Background(), r, Receipt{}); err == nil {
			t.Fatal("OS component removed", id)
		}
	}
	for _, name := range []string{"clang", "clang++", "make"} {
		if !slices.Contains(f.commands, filepath.Join(f.root, "usr", "bin", name)+" --version") {
			t.Fatal("selected real command was not checked", name, f.commands)
		}
	}
}

func TestMacOSPrerequisitesQueryIgnoresForeignToolchainOverrides(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("Apple query environment uses POSIX commands")
	}
	for _, key := range []string{"DEVELOPER_DIR", "SDKROOT", "TOOLCHAINS"} {
		t.Setenv(key, "/foreign/fixture/toolchain")
	}
	output, err := macOSPrerequisiteQuery(context.Background(), false, "/bin/sh", nil, "-c", `test -z "${DEVELOPER_DIR+x}${SDKROOT+x}${TOOLCHAINS+x}" && test "$LC_ALL" = C`)
	if err != nil || len(output) != 0 {
		t.Fatal("query inherited foreign toolchain overrides", err)
	}
}

func TestMacOSPrerequisitesMissingSelectionNeverExecutesDeveloperShim(t *testing.T) {
	for _, missing := range []string{"selection", "directory"} {
		t.Run(missing, func(t *testing.T) {
			f := macOSPrerequisitesFixture(t)
			if missing == "selection" {
				f.selectedError = errors.New("no developer directory")
			} else {
				f.selected = filepath.Join(f.root, "missing")
			}
			o, err := f.d.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
			if err != nil || !o.Unknown || o.Present || o.Pending == "" || len(f.commands) != 1 {
				t.Fatal("missing selection reached compiler shim", o, err, f.commands)
			}
		})
	}
}

func TestMacOSPrerequisitesRediscoversAfterBootstrap(t *testing.T) {
	f := macOSPrerequisitesFixture(t)
	f.selectedError = errors.New("CLT absent before bootstrap")
	o, err := f.d.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil || !o.Unknown {
		t.Fatal(o, err)
	}
	f.selectedError = nil
	o, err = f.d.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil || !o.Healthy || o.Unknown {
		t.Fatal("cached pre-bootstrap developer failure", o, err)
	}
}

func TestMacOSPrerequisitesRejectsUnselectedExecutables(t *testing.T) {
	f := macOSPrerequisitesFixture(t)
	outside, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.outside = filepath.Join(outside, "clang")
	if err := os.WriteFile(f.outside, []byte("unselected"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{}); err == nil {
		t.Fatal("unselected developer executable accepted")
	}
	if len(f.commands) != 2 {
		t.Fatal("unselected executable was run", f.commands)
	}
}

func TestMacOSPrerequisitesRejectsWrongCompilerOrMake(t *testing.T) {
	for _, id := range []string{"tool.compiler", "tool.make"} {
		t.Run(id, func(t *testing.T) {
			f := macOSPrerequisitesFixture(t)
			f.version = "unrecognized native command\n"
			o, err := f.d.Observe(context.Background(), Resource{ID: id}, Receipt{})
			if err != nil || o.Healthy || o.ApplyBlocked == "" {
				t.Fatal("wrong command accepted", o, err)
			}
		})
	}
}

func TestMacOSPrerequisitesRequiresActualSDK(t *testing.T) {
	f := macOSPrerequisitesFixture(t)
	f.sdk = filepath.Join(f.root, "missing-sdk")
	o, err := f.d.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil || o.Healthy || !strings.Contains(o.ApplyBlocked, "SDK") {
		t.Fatal("compiler accepted without SDK", o, err)
	}
}

func TestMacOSPrerequisitesClipboardDoesNotReadOrWriteClipboard(t *testing.T) {
	f := macOSPrerequisitesFixture(t)
	o, err := f.d.Observe(context.Background(), Resource{ID: "tool.clipboard"}, Receipt{})
	if err != nil || !o.Healthy || len(o.UnverifiedApplications) != 1 || len(f.commands) != 0 {
		t.Fatal("clipboard contents were probed or roundtrip claimed", o, err, f.commands)
	}
	if err := os.Remove(f.d.clipboardCommands[1]); err != nil {
		t.Fatal(err)
	}
	o, err = f.d.Observe(context.Background(), Resource{ID: "tool.clipboard"}, Receipt{})
	if err != nil || o.Healthy || o.ApplyBlocked == "" {
		t.Fatal("missing paste command was ignored", o, err)
	}
}

func TestMacOSPrerequisitesRejectsInvalidPathsAndCancellation(t *testing.T) {
	for _, path := range []string{"/", "relative/path", "/a/../b", "/a\n/b", "/a\x00b"} {
		if _, err := macOSDeveloperPath([]byte(path)); err == nil {
			t.Fatal("invalid native path accepted", path)
		}
	}
	f := macOSPrerequisitesFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.d.Observe(ctx, Resource{ID: "tool.compiler"}, Receipt{}); !errors.Is(err, context.Canceled) || len(f.commands) != 0 {
		t.Fatal("canceled query ran", err, f.commands)
	}
}

func TestMacOSPrerequisitesConstructionIsPureAndPlatformBound(t *testing.T) {
	if _, err := configureMacOSPrerequisites(NativePlatform{Context: Context{OS: "darwin", Arch: "amd64"}}); err == nil {
		t.Fatal("unsupported macOS architecture accepted")
	}
	if d, err := configureMacOSPrerequisites(NativePlatform{Context: Context{OS: "linux", Arch: "arm64"}}); err != nil || d != nil {
		t.Fatal("Apple provider escaped its platform", d, err)
	}
	d, err := configureMacOSPrerequisites(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil || d.Query == nil || !slices.Equal(d.clipboardCommands, []string{"/usr/bin/pbcopy", "/usr/bin/pbpaste"}) {
		t.Fatal(d, err)
	}
}
