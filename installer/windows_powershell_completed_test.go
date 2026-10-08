package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The existing native boundary fixtures persist real provider/engine journals;
// no Windows package or process is installed by this regression.
func completedWindowsVendorController(t *testing.T, kind string) (Controller, string, string, string, *int) {
	t.Helper()
	var c Controller
	var directory, worker, script string
	var resource Resource
	var runs *int
	switch kind {
	case "runtime":
		f := newWindowsVendorFixture(t)
		c.Driver, resource, directory, worker, script, runs = f.driver, f.resource, f.driver.Directory, f.driver.WorkerDirectory, windowsVendorInstallScript, &f.runs
		run := f.driver.Run
		f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			f.receipt = state.Receipts[resource.ID]
			if !slices.Equal(command.Arguments, windowsVendorArguments(script)) {
				t.Fatal("new runtime command lacks current module prelude")
			}
			return run(ctx, command)
		}
	case "build-tools":
		f := newWindowsBuildToolsFixture(t)
		c.Driver, resource, directory, worker, script, runs = f.driver, Resource{ID: "tool.compiler", Action: "windows-buildtools", Scope: "machine", Shared: true}, f.driver.Directory, f.driver.WorkerDirectory, windowsBuildToolsInstallScript, &f.runs
		run := f.driver.Run
		f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			f.receipt = state.Receipts[resource.ID]
			if !slices.Equal(command.Arguments, windowsVendorArguments(script)) {
				t.Fatal("new Build Tools command lacks current module prelude")
			}
			return run(ctx, command)
		}
	default:
		t.Fatal("unknown fixed vendor fixture")
	}
	resource.Name, resource.Capability = kind, true
	c.Catalog = &Catalog{Schema: 1, Resources: []Resource{resource}}
	c.Context, c.Source = Context{OS: "windows", Arch: "amd64"}, "legacy-completed-vendor"
	c.Home, c.StatePath = filepath.Dir(directory), filepath.Join(filepath.Dir(directory), "state", "state.json")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{resource.ID}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "operations", state.Receipts[resource.ID].OperationID+".json")
	return c, path, worker, script, runs
}

func legacyCompletedWindowsVendorCommand(t *testing.T, path, worker, script string, corrupt func(*nativeCommand), nativeEvidence bool) {
	t.Helper()
	data, err := readDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Complete bool
		Commands []nativeCommand
	}
	if err := json.Unmarshal(data, &envelope); err != nil || !envelope.Complete || len(envelope.Commands) != 1 {
		t.Fatal("fixture is not a completed vendor journal", err)
	}
	command := &envelope.Commands[0]
	command.Arguments = legacyWindowsPowerShellArguments(script)
	if corrupt != nil {
		corrupt(command)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	key := "commands"
	if document["Commands"] != nil {
		key = "Commands"
	}
	document[key], err = json.Marshal(envelope.Commands)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(path, document); err != nil {
		t.Fatal(err)
	}
	if nativeEvidence {
		hash, err := digest(*command)
		if err != nil {
			t.Fatal(err)
		}
		code := 0
		if err := saveDocument(filepath.Join(worker, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsPowerShellLegacyCompletedLifecycle(t *testing.T) {
	for _, kind := range []string{"runtime", "build-tools"} {
		t.Run(kind, func(t *testing.T) {
			c, path, worker, script, runs := completedWindowsVendorController(t, kind)
			legacyCompletedWindowsVendorCommand(t, path, worker, script, nil, true)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r := c.Catalog.Resources[0]
			receipt := state.Receipts[r.ID]
			observed, err := c.Driver.Observe(t.Context(), r, receipt)
			if err != nil || !observed.Healthy || observed.CompletedOperation != receipt.OperationID {
				t.Fatal("legacy completed observation failed", observed, err)
			}
			check, err := c.Dispatch(t.Context(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("completed legacy receipt blocks check", check, err)
			}
			// Re-entering the completed provider operation must only observe it.
			if _, err := c.Driver.Apply(t.Context(), r, Operation{Action: "install"}, receipt); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) || *runs != 1 {
				t.Fatal("completed historical command was rewritten or replayed", err, *runs)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{r.ID}}
			plan, err := c.Preview(t.Context(), request)
			if err != nil || len(plan.Operations) != 1 || plan.Operations[0].Action != "remove" {
				t.Fatal("legacy ownership cannot preview exact removal", plan, err)
			}
			dispatchApproved(t, c, request)
			check, err = c.Dispatch(t.Context(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" || *runs != 2 {
				t.Fatal("new removal did not complete", check, err, *runs)
			}
			observed, err = c.Driver.Observe(t.Context(), r, Receipt{})
			if err != nil || observed.Present {
				t.Fatal("owned native component remains after removal", observed, err)
			}
			after, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("removal changed historical installation evidence", err)
			}
		})
	}
}

func TestWindowsPowerShellLegacyCompletedRejectsChangedCommand(t *testing.T) {
	changes := map[string]func(*nativeCommand){
		"script":  func(c *nativeCommand) { c.Arguments = legacyWindowsPowerShellArguments("throw 'changed'\n") },
		"flags":   func(c *nativeCommand) { c.Arguments[1] = "-NoExit" },
		"program": func(c *nativeCommand) { c.Program += ".other" },
		"input":   func(c *nativeCommand) { c.Input = append(c.Input, ' ') },
		"operation": func(c *nativeCommand) {
			c.Operation = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		"environment":         func(c *nativeCommand) { c.Environment = []string{"CHANGED=1"} },
		"completion-evidence": nil,
	}
	for _, kind := range []string{"runtime", "build-tools"} {
		for name, change := range changes {
			t.Run(kind+"/"+name, func(t *testing.T) {
				c, path, worker, script, runs := completedWindowsVendorController(t, kind)
				legacyCompletedWindowsVendorCommand(t, path, worker, script, change, name != "completion-evidence")
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := c.Dispatch(t.Context(), Request{Schema: 1, Mode: "check"}); err == nil {
					t.Fatal("changed or unproved legacy completion became trusted")
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) || *runs != 1 {
					t.Fatal("rejected completion was changed or dispatched", err, *runs)
				}
			})
		}
	}
}

func TestWindowsVendorNewCommandsAlwaysBindBuiltInModules(t *testing.T) {
	for _, script := range []string{windowsVendorInstallScript, windowsBuildToolsInstallScript} {
		want := legacyWindowsPowerShellArguments("$env:PSModulePath = [IO.Path]::Combine($PSHOME, 'Modules')\n" + script)
		if !slices.Equal(windowsVendorArguments(script), want) {
			t.Fatal("new vendor command omitted or changed its exact built-in module binding")
		}
	}
}
