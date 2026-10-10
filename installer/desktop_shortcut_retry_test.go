package installer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

// This child substitutes only the Windows COM process boundary. The controller,
// native command records, source files and configuration publication are real.
func TestDesktopShortcutProcessFixture(t *testing.T) {
	if os.Getenv("DOTFILES_SHORTCUT_FIXTURE") != "1" {
		return
	}
	stage, control := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	mode, err := os.ReadFile(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("shortcut "+string(mode)), 0600); err != nil {
		t.Fatal(err)
	}
	if string(mode) == "failed" {
		os.Exit(23)
	}
	fmt.Printf("%x", sha256.Sum256([]byte("shortcut "+string(mode))))
	os.Exit(0)
}

func shortcutFixture(t *testing.T) (Controller, *DesktopDriver, string, string, *[]nativeCommand) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "space ü ' root")
	data := archiveTar(t, []archiveFixtureEntry{{Name: "Code.exe", Text: "application", Mode: 0755}})
	pin, client, _ := archivePinFor(t, data, "tar.gz")
	pin.Commands, pin.RequiredFiles = map[string]string{"code": "Code.exe"}, []string{"Code.exe"}
	d := &DesktopDriver{Target: Context{OS: "windows", Arch: "amd64"}, Directory: filepath.Join(root, "state"), Folders: ConfigFolders{Home: filepath.Join(root, "home"), Programs: filepath.Join(root, "Programs")}}
	d.Archives = &ArchiveDriver{Directory: filepath.Join(d.Directory, "packages"), Pins: map[string]ArchivePin{"tool.vscode": pin}, Client: client}
	ac := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"tool.vscode"}}, {ID: "tool.vscode", Name: "Code", Action: "archive"}}}, Context: d.Target, Source: "shortcut-archive", Home: d.Folders.Home, StatePath: filepath.Join(root, "archive-state.json"), Driver: d.Archives}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	d.PowerShell, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	control := filepath.Join(root, "native-result")
	writeConfigFixture(t, control, "success")
	var calls []nativeCommand
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		calls = append(calls, command)
		script := shortcutPowerShellScript(t, command.Arguments)
		line := strings.Split(strings.SplitN(script, "$destination=", 2)[1], "\n")[0]
		stage := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(line, "'"), "'"), "''", "'")
		command.Arguments = []string{"-test.run=^TestDesktopShortcutProcessFixture$", "--", stage, control}
		command.Environment = []string{"DOTFILES_SHORTCUT_FIXTURE=1"}
		worker := filepath.Join(root, "worker")
		mode, err := os.ReadFile(control)
		if err != nil {
			t.Fatal(err)
		}
		if string(mode) == "unfinished" {
			hash, err := digest(command)
			if err != nil {
				t.Fatal(err)
			}
			if err := saveDocument(filepath.Join(worker, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, stage, "uncertain partial shortcut")
		}
		reply, err := executeNativeCommand(worker, command)
		if err == nil && reply.Error != "" {
			err = errors.New(reply.Error)
		}
		return reply.Output, err
	}
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"desktop.vscode"}}, {ID: "desktop.vscode", Name: "Code launcher", Action: "desktop"}}}, Context: d.Target, Source: "shortcut-fixture", Home: d.Folders.Home, StatePath: filepath.Join(d.Directory, "state.json"), Driver: d}
	return c, d, filepath.Join(d.Folders.Programs, "Dotfiles Visual Studio Code.lnk"), control, &calls
}

func TestDesktopShortcutRetriesFailedAndUnfinishedPreparation(t *testing.T) {
	for _, failure := range []string{"failed", "unfinished"} {
		t.Run(failure, func(t *testing.T) {
			c, d, destination, control, calls := shortcutFixture(t)
			writeConfigFixture(t, control, failure)
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"app"}}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil {
				t.Fatal("fixture did not fail native preparation")
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed preparation published a shortcut", err)
			}
			partials, err := filepath.Glob(filepath.Join(d.Directory, "desktop", "inputs", "*", "staged.lnk"))
			if err != nil || len(partials) != 1 {
				t.Fatal("failed preparation evidence missing", partials, err)
			}
			partial, err := os.ReadFile(partials[0])
			if err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, control, "success")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			if len(*calls) != 2 || (*calls)[0].Operation == (*calls)[1].Operation {
				t.Fatal("retry reused poisoned native preparation identity", *calls)
			}
			preserved, err := os.ReadFile(partials[0])
			if err != nil || string(preserved) != string(partial) {
				t.Fatal("retry changed uncertain previous native input", string(preserved), err)
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != "shortcut success" {
				t.Fatal(string(data), err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			writeConfigFixture(t, control, "new-operation")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
			data, err = os.ReadFile(destination)
			if err != nil || string(data) != "shortcut new-operation" || len(*calls) != 3 {
				t.Fatal("new install reused previous COM bytes", string(data), err, len(*calls))
			}
		})
	}
}

type interruptedDesktopDriver struct{ *DesktopDriver }

func (d interruptedDesktopDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.DesktopDriver.Apply(ctx, r, op, receipt)
	return o, errors.Join(err, errors.New("fixture interruption after shortcut publication"))
}

func TestDesktopShortcutResumesJournalWithoutRegeneratingInput(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint("legacy=", legacy), func(t *testing.T) {
			c, d, destination, control, calls := shortcutFixture(t)
			c.Driver = interruptedDesktopDriver{d}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"app"}}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "fixture interruption") {
				t.Fatal("fixture did not interrupt after real publication", err)
			}
			c.Driver = d
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := c.Catalog.Resource("desktop.vscode")
			receipt := state.Receipts[r.ID]
			configuration, cr, _, desired, err := d.configuration(r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			journal, err := configuration.readJournal(cr, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				// Existing releases record one recipe-only input. No schema rewrite
				// or new native command is needed to read/resume that exact source.
				old := filepath.Join(d.Directory, "desktop", "inputs", desired+".lnk")
				if err := moveConfigExclusive(journal.Entries[0].State.Target.Source, old); err != nil {
					t.Fatal(err)
				}
				journal.Entries[0].State.Target.Source = old
				configuration.Manifest.Targets[0].Source, err = integrationRelative(d.Directory, old)
				if err != nil {
					t.Fatal(err)
				}
				_, observed, err := inspectConfiguration(configuration.Catalog, configuration.Manifest, configuration.Target, configuration.Folders, configuration.Repository, cr.ID)
				if err != nil {
					t.Fatal(err)
				}
				journal.Before.Desired = observed.Desired
			}
			// This is the existing persisted cut after target publication and before
			// the configuration journal's completion marker is saved.
			journal.Complete = false
			if err := saveDocument(configuration.journalPath(journal.Operation), journal); err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, control, "failed")
			retry := Request{Schema: 1, Mode: "apply", Retry: true}
			preview, err := c.Preview(context.Background(), retry)
			if err != nil {
				t.Fatal(err)
			}
			retry.ExpectedPlan = preview.ID
			result, err := c.Dispatch(context.Background(), retry)
			if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, "saved operation recovered") {
				t.Fatal("saved publication did not recover", result, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" || len(*calls) != 1 {
				t.Fatal("resume regenerated the journal's source", check, err, len(*calls))
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != "shortcut success" {
				t.Fatal(string(data), err)
			}
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt = state.Receipts[r.ID]
			receipt.Status = "in-progress"
			observed, err := d.Observe(context.Background(), r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt); err == nil || !strings.Contains(err.Error(), "operation already exists") || len(*calls) != 1 {
				t.Fatal("completed journal allowed shortcut re-preparation", err, len(*calls))
			}
		})
	}
}

func TestDesktopShortcutRejectsNativeHashMismatchBeforePublication(t *testing.T) {
	c, d, destination, _, _ := shortcutFixture(t)
	run := d.Run
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		_, err := run(ctx, command)
		return []byte(strings.Repeat("0", 64)), err
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"app"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "shortcut bytes differ") {
		t.Fatal("native hash mismatch was accepted", err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("mismatched input was published", err)
	}
	ready, err := filepath.Glob(filepath.Join(d.Directory, "desktop", "inputs", "*", "shortcut.lnk"))
	if err != nil || len(ready) != 0 {
		t.Fatal("unverified private input was published", ready, err)
	}
}

func shortcutPowerShellScript(t *testing.T, arguments []string) string {
	t.Helper()
	encoded, err := base64.StdEncoding.DecodeString(arguments[4])
	if err != nil || len(encoded)%2 != 0 {
		t.Fatal("invalid encoded PowerShell", err)
	}
	units := make([]uint16, len(encoded)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(encoded[i*2:])
	}
	return string(utf16.Decode(units))
}

func TestDesktopShortcutPowerShellProgressPreservesHashProtocol(t *testing.T) {
	name := "pwsh"
	if runtime.GOOS == "windows" {
		name = "powershell.exe"
	}
	program, err := exec.LookPath(name)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("optional PowerShell process boundary; Windows PowerShell 5.1 is required on Windows")
	}
	c, d, _, _, _ := shortcutFixture(t)
	d.PowerShell = program
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		script := shortcutPowerShellScript(t, command.Arguments)
		// Substitute only COM creation; file hashing,
		// progress serialization and the durable native worker are real.
		begin := strings.Index(script, "$link=(New-Object -ComObject WScript.Shell)")
		end := strings.Index(script, "(Get-FileHash")
		if begin < 0 || end <= begin {
			t.Fatal("missing COM boundary")
		}
		script = script[:begin] + "[IO.File]::WriteAllText($destination, 'private shortcut fixture')\nWrite-Progress -Activity 'Preparing modules for first use.' -Status 'Loading' -PercentComplete 10\n" + script[end:]
		command.Arguments = windowsVendorArguments(script)
		reply, err := executeNativeCommand(filepath.Join(d.Directory, "worker"), command)
		if strings.Contains(string(reply.Output), "#< CLIXML") {
			t.Logf("fixture hash plus real PowerShell progress: %q", reply.Output)
		}
		if err == nil && reply.Error != "" {
			err = errors.New(reply.Error)
		}
		return reply.Output, err
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	if check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
}
