package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNativeBrewSharedLibraryUpdatePreservesOutsideConsumer(t *testing.T) {
	testNativeBrewSharedLibrary(t, false, false)
}

func TestNativeBrewFailedDependentRepairMustFinishDuringRecovery(t *testing.T) {
	testNativeBrewSharedLibrary(t, true, false)
}

func testNativeBrewSharedLibrary(t *testing.T, failMaintenance, withCask bool) {
	host := newNativeBrewFixture(t)
	// A caller preference cannot disable the adapter's required native repair.
	t.Setenv("HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK", "1")
	library, outside, root := host.prefix+"-library", host.prefix+"-outside", host.prefix+"-root"
	dependency := fmt.Sprintf("  depends_on %q\n", host.tap+"/"+library)
	writeLibrary := func(abi int) {
		host.formula(library, fmt.Sprintf("%d.0", abi), "", fmt.Sprintf(`    (buildpath/"value.c").write "int fixture_value(void) { return %d; }"
	system ENV.cc, "-dynamiclib", "value.c", "-install_name", "#{opt_lib}/lib%s.%d.dylib", "-o", "lib%s.%d.dylib"
	lib.install "lib%s.%d.dylib"
	(lib/"lib%s.dylib").make_symlink "lib%s.%d.dylib"`, abi, library, abi, library, abi, library, abi, library, library, abi))
	}
	writeConsumer := func(name, version string) {
		host.formula(name, version, dependency, fmt.Sprintf(`    (buildpath/"consumer.c").write <<~C
		#include <stdio.h>
		extern int fixture_value(void);
		int main(void) { printf("%%d", fixture_value()); return 0; }
	C
	system ENV.cc, "consumer.c", "-L#{Formula[%q].opt_lib}", "-l%s", "-o", %q
	bin.install %q`, host.tap+"/"+library, library, name, name))
	}
	writeLibrary(1)
	writeConsumer(outside, "1.0")
	writeConsumer(root, "1.0")
	host.require("", "trust", "--tap", host.tap)
	host.require("outside-install", "install", "--formula", host.tap+"/"+outside)
	cask, caskCommand := "", ""
	if withCask {
		cask = host.prefix + "-app"
		caskCommand = host.installConsumerCask(cask, host.tap+"/"+outside, filepath.Join(host.cellar, outside, "1.0", "bin", outside))
	}
	casksBefore := host.require("", "list", "--cask", "--versions")
	checkCask := func() {
		t.Helper()
		if !withCask {
			return
		}
		output, err := exec.CommandContext(host.ctx, caskCommand).CombinedOutput()
		if err != nil || string(output) != "1" && string(output) != "2" {
			t.Fatalf("external cask command is unusable: %v\n%s", err, output)
		}
		if after := host.require("", "list", "--cask", "--versions"); !bytes.Equal(casksBefore, after) {
			t.Fatal("formula operation changed the installed casks")
		}
	}
	checkCask()
	commandValue := func(name, version string) string {
		t.Helper()
		command := exec.CommandContext(host.ctx, filepath.Join(host.cellar, name, version, "bin", name))
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("shared-library consumer %s became unusable: %v\n%s", name, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if value := commandValue(outside, "1.0"); value != "1" {
		t.Fatal("compiled outside consumer lacks its original library", value)
	}
	before, err := brewInventory(host.ctx, host.query)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(host.root, "provider")
	worker, err := workerFixture(filepath.Join(directory, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	controls := []string{}
	for _, value := range host.env {
		if validBrewEnvironment(value) {
			controls = append(controls, value)
		}
	}
	prefix := strings.TrimSpace(string(host.require("", "--prefix")))
	if !filepath.IsAbs(prefix) {
		t.Fatal("Homebrew prefix must be absolute", prefix)
	}
	probe := filepath.Join(prefix, "opt", root, "bin", root)
	query := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if program == probe && !privileged && len(input) == 0 && len(args) == 0 {
			return exec.CommandContext(ctx, probe).CombinedOutput()
		}
		return host.query(ctx, privileged, program, input, args...)
	}
	driver := &BrewDriver{Directory: directory, Program: host.program, Cellar: host.cellar, Packages: map[string]string{"tool.root": host.tap + "/" + root}, Checks: map[string][]string{"tool.root": {probe}}, Environment: controls, Query: query, Run: worker.run}
	resource := Resource{ID: "tool.root", Name: "Compiled root", Action: "native", Scope: "machine"}
	receipt := Receipt{Status: "in-progress"}
	receipt.Before, err = driver.Observe(host.ctx, resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	change := func(action string) {
		t.Helper()
		receipt.OperationID, err = digest(host.nonce + "-" + action)
		if err != nil {
			t.Fatal(err)
		}
		receipt.Status = "in-progress"
		after, err := driver.Apply(host.ctx, resource, Operation{Action: action}, receipt)
		if err != nil || !after.Present || !after.Healthy || after.CompletedOperation != receipt.OperationID {
			t.Fatal("compiled native root lifecycle", action, after, err)
		}
		receipt.Ownership, receipt.Status, receipt.After = "created", "ready", after
	}
	change("install")
	if withCask && len(receipt.After.UnverifiedApplications) != 0 {
		t.Fatal("installing an unrelated root incorrectly reported an unaffected application", receipt.After)
	}
	if value := commandValue(root, "1.0"); value != "1" {
		t.Fatal("initial root did not link to its native dependency", value)
	}
	// A linked keg with an unexecutable command must be reported unhealthy.
	info, err := os.Stat(probe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(probe, 0600); err != nil {
		t.Fatal(err)
	}
	observed, observeErr := driver.Observe(host.ctx, resource, receipt)
	restoreErr := os.Chmod(probe, info.Mode())
	if observeErr != nil || restoreErr != nil || !observed.Present || observed.Healthy || observed.HealthIssue == "" {
		t.Fatal("native command permissions were not checked", observed, observeErr, restoreErr)
	}
	writeLibrary(2)
	writeConsumer(root, "2.0")
	outdated := string(host.require("", "outdated", "--formula"))
	if !strings.Contains(outdated, library) || !strings.Contains(outdated, root) {
		t.Fatal("ABI fixture did not establish an actual dependency update", outdated)
	}
	if failMaintenance {
		host.formula(outside, "1.0", dependency, `    raise "fixture-dependent-rebuild-refused"`)
		receipt.OperationID, err = digest(host.nonce + "-failed-update")
		if err != nil {
			t.Fatal(err)
		}
		receipt.Status = "in-progress"
		_, updateErr := driver.Apply(host.ctx, resource, Operation{Action: "update"}, receipt)
		if updateErr == nil || !strings.Contains(updateErr.Error(), "fixture-dependent-rebuild-refused") {
			t.Fatal("fixture did not reach the native dependent rebuild failure", updateErr)
		}
		writeConsumer(outside, "1.0")
		// Reproduce the old recovery boundary independently: the root upgrade
		// now exits zero, but cannot repair its already upgraded dependency.
		host.require("", "upgrade", "--formula", host.tap+"/"+root)
		if output, err := exec.CommandContext(host.ctx, filepath.Join(host.cellar, outside, "1.0", "bin", outside)).CombinedOutput(); err == nil {
			t.Fatal("root-only upgrade unexpectedly repaired this fixture", string(output))
		}
		after, err := driver.ResumeResource(host.ctx, resource, Operation{Action: "update"}, receipt)
		if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID {
			t.Fatal("native maintenance recovery", after, err)
		}
		receipt.After, receipt.Status = after, "ready"
	} else {
		change("update")
	}
	intent, err := driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	upgraded := false
	for _, command := range intent.Commands {
		result, err := driver.commandResult(command)
		if err != nil {
			t.Fatal(err)
		}
		kegs, err := brewInstallEvidence(result.Output, filepath.ToSlash(host.cellar), command.Operation)
		if err != nil {
			t.Fatal(err)
		}
		upgraded = upgraded || slices.ContainsFunc(kegs, func(keg brewInstalledKeg) bool { return keg.Name == library && keg.Version == "2.0" })
	}
	if !upgraded {
		t.Fatal("native update did not actually replace the shared ABI")
	}
	checkCask()
	if withCask && !slices.Contains(receipt.After.UnverifiedApplications, host.tap+"/"+cask) {
		t.Fatal("formula update concealed the external cask", receipt.After)
	}
	if value := commandValue(root, "2.0"); value != "2" {
		t.Fatal("updated root did not use the new library", value)
	}
	// Metadata alone is insufficient: actually execute the pre-existing binary.
	// Keeping its old usable keg or rebuilding it for v2 are both valid outcomes.
	if value := commandValue(outside, "1.0"); value != "1" && value != "2" {
		t.Fatal("outside consumer returned an invalid result", value)
	}
	after, err := brewInventory(host.ctx, host.query)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{library, outside} {
		old, current := before[name], after[name]
		if current.Name != name || old.Source != current.Source || old.Held != current.Held || old.Automatic != current.Automatic {
			t.Fatal("native update changed pre-existing ownership classification", name, old, current)
		}
	}
	state, err := driver.ledger()
	if err != nil || len(state.Roots) != 1 || len(state.Pool) != 0 {
		t.Fatal("native maintenance acquired pre-existing packages", state, err)
	}
	receipt.OperationID, err = digest(host.nonce + "-remove")
	if err != nil {
		t.Fatal(err)
	}
	receipt.Status = "in-progress"
	if observed, err := driver.Remove(host.ctx, resource, receipt); err != nil || observed.Present {
		t.Fatal("compiled root removal", observed, err)
	}
	checkCask()
	if value := commandValue(outside, "1.0"); value != "1" && value != "2" {
		t.Fatal("removal broke the pre-existing consumer", value)
	}
	state, err = driver.ledger()
	if err != nil || len(state.Roots) != 0 || len(state.Pool) != 0 {
		t.Fatal("compiled native removal left owned packages", state, err)
	}
}
