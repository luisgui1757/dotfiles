package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLegacyLinuxProcessScanRefusesUninspectedLiveProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux kernel process inspection")
	}
	root := t.TempDir()
	entry := filepath.Join(root, "42")
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw,nosuid,nodev,noexec - proc proc rw\n")
	writeConfigFixture(t, filepath.Join(entry, "status"), "Name:\tfixture\nState:\tS (sleeping)\nKthread:\t0\n")
	writeConfigFixture(t, filepath.Join(entry, "maps"), "")
	// A visible live process with an unreadable executable cannot be treated
	// as evidence that no process consumes Nix. No native process is changed.
	command := exec.Command("/bin/sh", "-c", legacyLinuxProcessScript, "sh", root)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatal("uninspected live process was silently excluded", string(output))
	}
}

func TestLegacyLinuxProcessScanRefusesHiddenProcesses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux kernel process inspection")
	}
	root := t.TempDir()
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw,nosuid,nodev,noexec - proc proc rw,hidepid=2\n")
	command := exec.Command("/bin/sh", "-c", legacyLinuxProcessScript, "sh", root)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatal("a mount that hides processes was accepted as an empty inventory", string(output))
	}
}

func TestLegacyRetirementRejectsChangedReplacementState(t *testing.T) {
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "old-runtime")
	writeConfigFixture(t, path, "preserve this artifact")
	scope := legacyRetirementScope{}
	if err := d.addUserMove(&scope, "Preserve fixture", path); err != nil {
		t.Fatal(err)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	before, _, err := d.resumeToken(context.Background(), *intent)
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Target: d.Normal.Home, Generation: 1, Receipts: map[string]Receipt{}}
	if err := SaveState(d.Normal.StatePath, state); err != nil {
		t.Fatal(err)
	}
	after, _, err := d.resumeToken(context.Background(), *intent)
	if err != nil || before == after {
		t.Fatal("changed setup did not invalidate recovery approval", err)
	}
	if err := d.executeStep(context.Background(), intent, 0, true); err == nil {
		t.Fatal("changed setup reused an earlier retirement scope")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "preserve this artifact" {
		t.Fatal("rejected retirement moved the artifact", err)
	}
}

func TestLegacyMachinePreviewDisclosesExactScope(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-ai")
	writeConfigFixture(t, filepath.Join(pkg, "package.json"), `{"name":"@earendil-works/pi-ai","version":"1.0.4"}`)
	result, err := retirement.Dispatch(ctx, Request{Schema: 1, Mode: "apply", Selected: []string{"retire.pi"}})
	if err != nil || result.Status != "preview" || !strings.Contains(result.Message, pkg+" -> ") || !strings.Contains(result.Message, ".dotfiles-retired-") {
		t.Fatal("machine preview omitted exact preservation paths", result, err)
	}
	if _, err := os.Stat(pkg); err != nil {
		t.Fatal("preview changed the package", err)
	}
}

func TestLegacyMacProcessWarningsCannotAuthorizeRemoval(t *testing.T) {
	for _, output := range []string{"lsof: WARNING: cannot inspect volume\n", "p42\nftxt\nn/nix/store/prog/bin/prog\nlsof: incomplete inspection\n", "p42\n", ""} {
		if _, err := legacyDarwinProcessConsumers([]byte(output), "", "", false); err == nil {
			t.Fatalf("incomplete inventory accepted: %q", output)
		}
	}
}

func TestLegacyMacDaemonExclusionRequiresServicePIDAndExecutable(t *testing.T) {
	const actual = "/nix/store/actual/bin/nix-daemon"
	output := "p42\nftxt\nn" + actual + "\np43\nftxt\nn" + actual + "\np44\nftxt\nn/nix/store/unrelated/bin/nix-daemon\n"
	processes, err := legacyDarwinProcessConsumers([]byte(output), "42", actual, false)
	if err != nil || len(processes) != 2 || !strings.Contains(processes[0], "process 43 ") || !strings.Contains(processes[1], "process 44 ") {
		t.Fatal(processes, err)
	}
}

func TestLegacyNewApprovalKeepsEarlierPreservationPaths(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pi-ai", "pi-tui"} {
		pkg := filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", name)
		writeConfigFixture(t, filepath.Join(pkg, "package.json"), `{"name":"@earendil-works/`+name+`","version":"1.0.4"}`)
	}
	scope, err := retirement.driver.inspect(ctx, "retire.pi")
	if err != nil || len(scope.Steps) != 3 {
		t.Fatal(scope, err)
	}
	first, second := scope.Steps[0].Backup, scope.Steps[1].Backup
	writeConfigFixture(t, second, "occupied")
	apply := func(request Request) (Result, error) {
		preview, err := retirement.Controller.Dispatch(ctx, request)
		if err != nil {
			return preview, err
		}
		request.ExpectedPlan = preview.Plan.ID
		return retirement.Controller.Dispatch(ctx, request)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.pi"}}
	if _, err := apply(request); err == nil {
		t.Fatal("fixture did not stop at occupied second backup")
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatal("first package was not preserved", err)
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if _, err := apply(Request{Schema: 1, Mode: "abandon"}); err != nil {
		t.Fatal(err)
	}
	if result, err := apply(request); err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	state, err := LoadState(retirement.Controller.StatePath, home)
	if err != nil {
		t.Fatal(err)
	}
	paths := state.Receipts["retire.pi"].After.Preserved
	if !slices.Contains(paths, first) || !slices.Contains(paths, second) {
		t.Fatal("fresh approval lost an earlier recovery path", paths)
	}
}

func TestLegacyReinstalledRuntimeRequiresNewApproval(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-ai")
	var previous Receipt
	for attempt := 0; attempt < 2; attempt++ {
		writeConfigFixture(t, filepath.Join(pkg, "package.json"), `{"name":"@earendil-works/pi-ai","version":"1.0.4"}`)
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.pi"}}
		preview, err := retirement.Controller.Dispatch(ctx, request)
		if err != nil || preview.Status != "preview" || operation(t, preview.Plan, "retire.pi").Action != "install" {
			t.Fatal("each installation needs a new retirement preview", attempt, preview, err)
		}
		state, err := LoadState(retirement.Controller.StatePath, home)
		if err != nil {
			t.Fatal(err)
		}
		choices, err := retirement.choices(ctx, state, LegacyRetirementRuntime)
		if err != nil || !slices.ContainsFunc(choices, func(c Choice) bool { return c.ID == "retire.pi" }) {
			t.Fatal("reinstalled package cannot be selected", choices, err)
		}
		if _, err := os.Stat(pkg); err != nil {
			t.Fatal("preview changed the package", err)
		}
		request.ExpectedPlan = preview.Plan.ID
		result, err := retirement.Controller.Dispatch(ctx, request)
		if err != nil || result.Status != "ready" {
			t.Fatal(attempt, result, err)
		}
		state, err = LoadState(retirement.Controller.StatePath, home)
		if err != nil {
			t.Fatal(err)
		}
		receipt := state.Receipts["retire.pi"]
		if attempt != 0 {
			if receipt.OperationID == previous.OperationID || len(receipt.After.Preserved) != len(previous.After.Preserved)+1 {
				t.Fatal("fresh retirement lost earlier preservation evidence", previous, receipt)
			}
			for _, path := range previous.After.Preserved {
				if !slices.Contains(receipt.After.Preserved, path) {
					t.Fatal("earlier preservation path was lost", path)
				}
			}
			if _, err := retirement.driver.readIntent("retire.pi", previous); err != nil {
				t.Fatal("earlier journal was not preserved", err)
			}
		}
		previous = receipt
	}
}

func TestLegacyRetirementDeclinedReinstallationStaysUnchanged(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-ai")
	packageJSON := filepath.Join(pkg, "package.json")
	writeConfigFixture(t, packageJSON, `{"name":"@earendil-works/pi-ai","version":"1.0.4"}`)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.pi"}}
	preview, err := retirement.Controller.Dispatch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if result, err := retirement.Controller.Dispatch(ctx, request); err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	state, err := LoadState(retirement.Controller.StatePath, home)
	if err != nil {
		t.Fatal(err)
	}
	history := state.Receipts["retire.pi"].OperationID
	writeConfigFixture(t, packageJSON, `{"name":"@earendil-works/pi-ai","version":"1.0.4"}`)
	writeConfigFixture(t, filepath.Join(home, ".config", "chezmoi", "chezmoi.toml"), "# released configuration\n")
	ui := &scriptedInteraction{t: t, accept: true, answers: [][]string{{"retire.chezmoi"}}}
	if _, err := retirement.Run(ctx, ui, LegacyRetirementRuntime); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(packageJSON); err != nil {
		t.Fatal("unchecked reinstalled Pi was retired", err)
	}
	state, err = LoadState(retirement.Controller.StatePath, home)
	if err != nil || state.Receipts["retire.pi"].OperationID != history {
		t.Fatal("declining retirement erased or replaced its earlier history", state.Receipts["retire.pi"], err)
	}
}

func TestLegacyUserMoveResolvesRedirectedParent(t *testing.T) {
	d, root := legacyStepFixture(t)
	cache := filepath.Join(root, "actual-cache")
	writeConfigFixture(t, filepath.Join(cache, "nix", "cache"), "preserve")
	link := filepath.Join(root, ".cache")
	if err := os.Symlink(cache, link); err != nil {
		t.Fatal(err)
	}
	scope := legacyRetirementScope{}
	if err := d.addUserMove(&scope, "Preserve cache", filepath.Join(link, "nix")); err != nil {
		t.Fatal(err)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	if err := d.executeStep(context.Background(), intent, 0, false); err != nil {
		t.Fatal("redirected cache could not be preserved", err)
	}
	if data, err := os.ReadFile(filepath.Join(scope.Steps[0].Backup, "cache")); err != nil || string(data) != "preserve" {
		t.Fatal("cache backup differs", err)
	}
	if target, err := os.Readlink(link); err != nil || target != cache {
		t.Fatal("parent redirection changed", target, err)
	}
}

func TestLegacySystemEditRefusesMissingStagedBytes(t *testing.T) {
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "fstab")
	const original = "# personal\nnix-entry\n"
	writeConfigFixture(t, path, original)
	scope := legacyRetirementScope{}
	if err := d.addLineRemoval(context.Background(), &scope, "Remove Nix line", path, "nix-entry"); err != nil {
		t.Fatal(err)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	run := d.Run
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if err := os.Remove(d.stagedEditPath(intent.Operation, 0)); err != nil {
			t.Fatal(err)
		}
		return run(ctx, command)
	}
	if err := d.executeStep(context.Background(), intent, 0, false); err == nil {
		t.Fatal("missing staged content accepted")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatal("missing staged input damaged the original", string(data), err)
	}
}

func TestLegacySystemEditReusesItsIntactBackup(t *testing.T) {
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "fstab")
	const original = "# personal\nnix-entry\n"
	writeConfigFixture(t, path, original)
	scope := legacyRetirementScope{}
	if err := d.addLineRemoval(context.Background(), &scope, "Remove Nix line", path, "nix-entry"); err != nil {
		t.Fatal(err)
	}
	// A prior interrupted command copied the approved original, then stopped
	// before publication. Its native bytes are the recovery evidence.
	writeConfigFixture(t, scope.Steps[0].Backup, original)
	intent := legacyTestIntent(t, d, scope.Steps...)
	if err := d.executeStep(context.Background(), intent, 0, false); err != nil {
		t.Fatal("intact backup prevented completion", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "# personal\n" {
		t.Fatal("personal content differs", string(data), err)
	}
}

func TestLegacyLinuxKernelThreadWithoutStatusFlag(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux procfs")
	}
	root := t.TempDir()
	entry := filepath.Join(root, "42")
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
	writeConfigFixture(t, filepath.Join(entry, "status"), "Name:\tkworker\nState:\tI (idle)\n")
	writeConfigFixture(t, filepath.Join(entry, "stat"), "42 (kworker ) name) I 2 0 0 0 -1 2097152 0 0 0\n")
	command := exec.Command("/bin/sh", "-c", legacyLinuxProcessScript, "sh", root)
	if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
		t.Fatal("kernel thread was treated as an uninspectable user process", string(output), err)
	}
}

func TestLegacyLinuxProcessNameCannotImpersonateKernelThread(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux procfs")
	}
	root := t.TempDir()
	entry := filepath.Join(root, "42")
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
	writeConfigFixture(t, filepath.Join(entry, "status"), "Name:\tfixture\nState:\tS (sleeping)\n")
	writeConfigFixture(t, filepath.Join(entry, "stat"), "42 (fake) I 2 0 0 0 -1 2097152\nactual) S 2 0 0 0 -1 0 0 0 0\n")
	command := exec.Command("/bin/sh", "-c", legacyLinuxProcessScript, "sh", root)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatal("a name containing newlines impersonated a kernel flag", string(output))
	}
}

func TestLegacyInspectionFailureDoesNotHideIndependentChoices(t *testing.T) {
	retirement, _, home := legacyEngineFixture(t)
	pi, _ := retirement.Controller.Catalog.Resource("retire.pi")
	tools, _ := retirement.Controller.Catalog.Resource("retire.tools")
	retirement.Controller.Catalog = &Catalog{Schema: 1, Resources: []Resource{tools, pi}}
	writeConfigFixture(t, filepath.Join(home, ".local", "share", "dotfiles", "zsh-plugins"), "not a directory")
	writeConfigFixture(t, filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-ai", "package.json"), `{"name":"@earendil-works/pi-ai","version":"1.0.4"}`)
	choices, err := retirement.choices(context.Background(), State{}, LegacyRetirementRuntime)
	if err != nil || !slices.ContainsFunc(choices, func(c Choice) bool { return c.ID == "retire.pi" }) {
		t.Fatal("an uninspectable tool inventory hid an independent retirement", choices, err)
	}
	remaining, _, err := retirement.remaining(context.Background(), LegacyRetirementRuntime)
	if err != nil || !slices.ContainsFunc(remaining, func(c Choice) bool { return c.ID == "retire.tools" && strings.Contains(c.Detail, "inspection failed") }) {
		t.Fatal("the local inspection failure was not disclosed", remaining, err)
	}
}

func TestLegacyAbsentRootFileDoesNotInventBackup(t *testing.T) {
	d, root := legacyStepFixture(t)
	source, backup := filepath.Join(root, "missing"), filepath.Join(root, "backup")
	command := d.systemCommand(true, "/bin/sh", "-c", legacyRootFilesScript, "sh", source, backup)
	step := legacyRetirementStep{Kind: "command", Label: "Preserve root file", Privileged: true, Backup: backup, Command: &command}
	intent := legacyTestIntent(t, d, step)
	if err := d.executeStep(context.Background(), intent, 0, false); err != nil {
		t.Fatal(err)
	}
	if len(intent.Preserved) != 0 {
		t.Fatal("a nonexistent backup was reported as preserved", intent.Preserved)
	}
}

func TestLegacyRejectedUserMoveDoesNotInventBackup(t *testing.T) {
	d, home := legacyStepFixture(t)
	path := filepath.Join(home, "earlier-config")
	writeConfigFixture(t, path, "approved contents")
	scope := legacyRetirementScope{}
	if err := d.addUserMove(&scope, "Preserve old config", path); err != nil {
		t.Fatal(err)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, "edited after approval")
	if err := d.executeStep(context.Background(), intent, 0, false); err == nil {
		t.Fatal("changed file was moved without a fresh approval")
	}
	if len(intent.Preserved) != 0 {
		t.Fatal("failed move disclosed a backup it never created", intent.Preserved)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "edited after approval" {
		t.Fatal("rejected move changed the personal file", err)
	}
}

func TestLegacyBlockedRecoveryDoesNotConsumeAttempt(t *testing.T) {
	d, root := legacyStepFixture(t)
	gatePath := filepath.Join(root, "gate")
	writeConfigFixture(t, gatePath, "approved")
	identity, err := legacyPathIdentity(gatePath)
	if err != nil {
		t.Fatal(err)
	}
	command := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", "exit 3"}}
	step := legacyRetirementStep{Kind: "command", Label: "Known failure", Command: &command,
		Gates: []legacyRetirementGate{{Kind: "path-identity", Subject: gatePath, Expected: identity}}}
	intent := legacyTestIntent(t, d, step)
	if err := d.executeStep(context.Background(), intent, 0, false); err == nil {
		t.Fatal("native fixture did not fail")
	}
	if err := os.Rename(gatePath, gatePath+".old"); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, gatePath, "different inode")
	if err := d.executeStep(context.Background(), intent, 0, true); err == nil {
		t.Fatal("changed gate accepted")
	}
	if intent.Attempts[0] != 0 {
		t.Fatal("a refused dispatch consumed a native attempt", intent.Attempts)
	}
}

func TestLegacyLinuxProcessDataConsumers(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux procfs")
	}
	for _, link := range []string{"cwd", "root", "fd/3"} {
		t.Run(link, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "42")
			writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
			writeConfigFixture(t, filepath.Join(entry, "maps"), "")
			if err := os.MkdirAll(filepath.Join(entry, "fd"), 0700); err != nil {
				t.Fatal(err)
			}
			for name, target := range map[string]string{"exe": "/usr/bin/sleep", "cwd": "/tmp", "root": "/", "fd/3": "/tmp/personal"} {
				if name == link {
					target = "/nix/store/fixture-data"
				}
				if err := os.Symlink(target, filepath.Join(entry, name)); err != nil {
					t.Fatal(err)
				}
			}
			output, err := exec.Command("/bin/sh", "-c", legacyLinuxProcessScript, "sh", root).CombinedOutput()
			if err != nil || !strings.Contains(string(output), "42\t") {
				t.Fatal("live Nix data consumer was missed", string(output), err)
			}
		})
	}
}

func TestLegacyRetirementExplainsReusedNativeFont(t *testing.T) {
	retirement, _, home := legacyEngineFixture(t)
	font, ok := retirement.Controller.Catalog.Resource("retire.delta.font")
	if !ok {
		t.Fatal("font retirement is not in the catalog")
	}
	retirement.Controller.Catalog = &Catalog{Schema: 1, Resources: []Resource{font}}
	retirement.driver.Normal.Catalog = &Catalog{Schema: 1, Resources: []Resource{
		{ID: "font", Name: "Font", Capability: true, Requires: []string{"tool.font"}},
		{ID: "tool.font", Name: "Native font", Action: "font"},
	}}
	state := State{Schema: 1, Context: linux, Target: home, Source: "fixture", Generation: 1,
		Selected: []string{"font"}, Receipts: map[string]Receipt{
			"tool.font": {Ownership: "reused", Status: "ready", After: Observation{Provider: "native-font", Present: true, Healthy: true}},
		}}
	if err := SaveState(retirement.driver.Normal.StatePath, state); err != nil {
		t.Fatal(err)
	}
	result, err := retirement.result(context.Background(), Result{Status: "ready"}, LegacyRetirementRuntime)
	if err != nil || result.Status != "ready" || !strings.Contains(result.Message, "Kept retire.delta.font:") || !strings.Contains(result.Message, "reused by the selected setup") {
		t.Fatal("a kept native installation lost its explanation", result, err)
	}
}

func TestLegacyAccountShellsRejectsIncompleteInventory(t *testing.T) {
	for _, data := range []string{
		"fixture:x:1000:1000::/home/fixture:/bin/bash\nmalformed\n",
		"other:x:1001:1001::/home/other:/nix/store/shell/bin/zsh\nother:x:1001:1001::/home/other:/bin/bash\n",
	} {
		d, _ := legacyStepFixture(t)
		d.Platform = NativePlatform{Context: linux}
		d.Query = func(context.Context, nativeCommand) ([]byte, error) { return []byte(data), nil }
		if shells, err := d.accountShells(context.Background()); err == nil {
			t.Fatal("an incomplete account inventory could hide a shell consumer", shells)
		}
	}
}
