package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Like the APT contract, this intentionally uses the real package database of
// an opted-in disposable runner. Local data-only formulae avoid external builds.
func TestNativeBrewTransactionAndRemovalBoundary(t *testing.T) {
	host := newNativeBrewFixture(t)
	ctx, root, env := host.ctx, host.root, host.env
	run, require := host.run, host.require
	before, cellar, tap, nonce, prefix := host.before, host.cellar, host.tap, host.nonce, host.prefix
	names := []string{prefix + "-first", prefix + "-second", prefix + "-outside", prefix + "-shared", prefix + "-unrelated", prefix + "-setup"}
	for i, name := range names {
		dependency := ""
		if i < 3 {
			dependency = fmt.Sprintf("  depends_on %q\n", tap+"/"+names[3])
		} else if i == 5 {
			dependency = fmt.Sprintf("  depends_on %q\n", tap+"/"+names[4])
		}
		host.formula(name, "1.0", dependency, fmt.Sprintf("    (share/%q).install %q", name, "data.txt"))
	}
	require("", "trust", "--tap", tap)
	require("setup", "install", "--formula", tap+"/"+names[5])
	require("", "uninstall", "--formula", names[5])
	firstOperation, err := digest(nonce + "-first")
	if err != nil {
		t.Fatal(err)
	}
	firstMarker := "dotfiles-operation-" + firstOperation
	first := string(require(firstMarker, "install", "--formula", tap+"/"+names[0]))
	kegs, err := brewInstallEvidence([]byte(first), cellar, firstOperation)
	if err != nil || len(kegs) != 2 {
		t.Fatal("native completion evidence", kegs, err, first)
	}
	for _, name := range []string{names[0], names[3]} {
		if !slices.ContainsFunc(kegs, func(keg brewInstalledKeg) bool { return keg.Name == name && keg.Version == "1.0" }) {
			t.Fatal("actual installer summary lacks the installed keg", name, first)
		}
	}
	secondOperation, err := digest(nonce + "-second")
	if err != nil {
		t.Fatal(err)
	}
	secondMarker := "dotfiles-operation-" + secondOperation
	second := string(require(secondMarker, "install", "--formula", tap+"/"+names[1]))
	kegs, err = brewInstallEvidence([]byte(second), cellar, secondOperation)
	if err != nil || len(kegs) != 1 || kegs[0].Name != names[1] {
		t.Fatal("second installer record is missing or claims a reused dependency", second)
	}
	if again := require(firstMarker, "install", "--formula", tap+"/"+names[0]); bytes.Contains(again, []byte(firstMarker+"  ")) {
		t.Fatal("reused formula produced new installation authority", string(again))
	}
	inspect := func() map[string]nativePackage {
		t.Helper()
		query := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
			if privileged || program != "brew" || len(input) != 0 {
				t.Fatal("invalid native inventory query")
			}
			command := exec.CommandContext(ctx, program, args...)
			command.Env = env
			return command.Output()
		}
		installed, err := brewInventory(ctx, query)
		if err != nil {
			t.Fatal("actual native inventory", err)
		}
		return installed
	}
	firstRemoval, protected, err := nativeRemovalCandidates([]string{names[0]}, []string{names[3]}, nil, inspect())
	if err != nil || !slices.Equal(firstRemoval, []string{names[0]}) || !slices.Contains(protected[names[3]], names[1]) {
		t.Fatal("native inventory failed to protect the second consumer", firstRemoval, protected, err)
	}
	require("", append([]string{"uninstall", "--formula"}, firstRemoval...)...)
	lastRemoval, _, err := nativeRemovalCandidates([]string{names[1]}, []string{names[3]}, nil, inspect())
	if err != nil || !slices.Equal(lastRemoval, sortedUnique([]string{names[1], names[3]})) {
		t.Fatal("last consumer stranded a shared dependency", lastRemoval, err)
	}
	require("outside", "install", "--formula", tap+"/"+names[2])
	if output, err := run("", "uninstall", "--formula", "--force", names[3]); err == nil {
		t.Fatal("native uninstall removed a dependency with outside consumers", string(output))
	}
	require("", "list", "--formula", "--versions", names[2], names[3], names[4])
	require("", append([]string{"uninstall", "--formula"}, names[1:4]...)...)
	if output := require("", "list", "--formula", "--versions", names[4]); !bytes.Contains(output, []byte(names[4]+" 1.0")) {
		t.Fatal("exact removal swept an unrelated pre-existing orphan", string(output))
	}
	// Now exercise the actual adapter with the same real native packages. The
	// first half above independently validates the native manager contracts.
	directory := filepath.Join(root, "driver")
	directory, err = bindConfigDestination(directory)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := workerFixture(filepath.Join(directory, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	program, err := exec.LookPath("brew")
	if err != nil {
		t.Fatal(err)
	}
	query := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged || program != "brew" || len(input) != 0 {
			return nil, errors.New("invalid native inventory query")
		}
		command := exec.CommandContext(ctx, program, args...)
		command.Env = slices.Clone(env)
		if len(args) > 0 && args[0] == "linkage" {
			command.Env = append(command.Env, "HOMEBREW_DEV_CMD_RUN=1")
		}
		return command.Output()
	}
	controls := []string{}
	for _, value := range env {
		if validBrewEnvironment(value) {
			controls = append(controls, value)
		}
	}
	driver := &BrewDriver{Directory: directory, Program: program, Cellar: cellar, Packages: map[string]string{"tool.first": tap + "/" + names[0], "tool.second": tap + "/" + names[1]}, Environment: controls, Query: query, Run: worker.run}
	receipts := map[string]Receipt{}
	sequence := 0
	apply := func(id, action string, loseResponse bool) {
		t.Helper()
		resource := Resource{ID: id, Name: id, Action: "native"}
		receipt, exists := receipts[id]
		before, err := driver.Observe(ctx, resource, receipt)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			receipt.Before = before
		}
		sequence++
		receipt.OperationID, err = digest(fmt.Sprintf("%s-%s-%s-%d", nonce, id, action, sequence))
		if err != nil {
			t.Fatal(err)
		}
		receipt.Status = "in-progress"
		if loseResponse {
			driver.Run = func(ctx context.Context, request nativeCommand) ([]byte, error) {
				output, err := worker.run(ctx, request)
				if err != nil {
					return output, err
				}
				return output, errors.New("fixture lost the completed process response")
			}
		}
		op := Operation{Resource: id, Action: action, Observed: before}
		after, err := driver.Apply(ctx, resource, op, receipt)
		if loseResponse {
			if err == nil || !strings.Contains(err.Error(), "fixture lost") {
				t.Fatal("lost-response fixture did not interrupt publication", err)
			}
			if err := worker.close(); err != nil {
				t.Fatal(err)
			}
			worker, err = workerFixture(filepath.Join(directory, "worker"))
			if err != nil {
				t.Fatal(err)
			}
			driver.Run = worker.run
			pending, err := driver.Observe(ctx, resource, receipt)
			if err != nil || pending.ResourceResume == nil {
				t.Fatal("saved native operation lacks recovery", pending, err)
			}
			after, err = driver.ResumeResource(ctx, resource, op, receipt)
			if err != nil {
				t.Fatal("native recovery", err)
			}
		}
		if err != nil || !after.Present || !after.Healthy || after.CompletedOperation != receipt.OperationID {
			t.Fatal("native adapter application", after, err)
		}
		receipt.Ownership, receipt.Status, receipt.After = "created", "ready", after
		receipts[id] = receipt
	}
	apply("tool.first", "install", false)
	apply("tool.second", "install", true)
	apply("tool.first", "update", false)
	apply("tool.first", "repair", false)
	for _, id := range []string{"tool.first", "tool.second"} {
		receipt := receipts[id]
		receipt.OperationID, err = digest(nonce + "-remove-" + id)
		if err != nil {
			t.Fatal(err)
		}
		receipt.Status = "in-progress"
		after, err := driver.Remove(ctx, Resource{ID: id, Name: id, Action: "native"}, receipt)
		if err != nil || after.Present || after.CompletedOperation != receipt.OperationID {
			t.Fatal("native adapter removal", after, err)
		}
		if id == "tool.first" {
			if !slices.Contains(after.Preserved, filepath.Join(cellar, names[3])) {
				t.Fatal("first adapter removal concealed its retained shared dependency", after.Preserved)
			}
			inventory := inspect()
			if _, present := inventory[names[1]]; !present {
				t.Fatal("first adapter removal broke the second root")
			}
			if _, present := inventory[names[3]]; !present {
				t.Fatal("first adapter removal broke the shared dependency")
			}
		}
	}
	state, err := driver.ledger()
	if err != nil || len(state.Roots) != 0 || len(state.Pool) != 0 {
		t.Fatal("adapter stranded its owned native packages", state, err)
	}
	if _, present := inspect()[names[4]]; !present {
		t.Fatal("adapter swept the pre-existing orphan")
	}
	// Repeat through the real controller's preview/approval, locks, journal and
	// selection binding. This proves adapter contracts are composed correctly.
	engineRoot := filepath.Join(directory, "controller")
	session := &nativeSession{Directory: filepath.Join(engineRoot, "native", "worker")}
	session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(session.Directory) }
	brew := &BrewDriver{Directory: filepath.Dir(session.Directory), Program: program, Cellar: cellar, Packages: driver.Packages, Environment: controls, Query: query, Run: session.run, approval: &brewApproval{}}
	catalog := &Catalog{Schema: 1, Resources: []Resource{
		{ID: "first", Name: "First", Capability: true, Requires: []string{"tool.first"}},
		{ID: "second", Name: "Second", Capability: true, Requires: []string{"tool.second"}},
		{ID: "tool.first", Name: "First package", Action: "native", Scope: "machine", Shared: true},
		{ID: "tool.second", Name: "Second package", Action: "native", Scope: "machine", Shared: true},
	}}
	native := &NativeDriver{Catalog: catalog, Context: Context{OS: "darwin", Arch: runtime.GOARCH}, Brew: brew, StatePath: filepath.Join(engineRoot, "state.json"), session: session}
	controller := Controller{Catalog: catalog, Context: native.Context, Home: directory, Source: "native-brew-controller", StatePath: native.StatePath, Driver: native}
	dispatchApproved(t, controller, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	brew.Run = func(ctx context.Context, request nativeCommand) ([]byte, error) {
		output, err := session.run(ctx, request)
		if err == nil {
			err = errors.New("fixture lost controller response")
		}
		return output, err
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"first", "second"}}
	plan, err := controller.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := controller.Dispatch(ctx, request); err == nil || !strings.Contains(err.Error(), "fixture lost controller response") {
		t.Fatal("controller did not retain interrupted native intent", err)
	}
	brew.Run = session.run
	retry := Request{Schema: 1, Mode: "apply", Retry: true}
	plan, err = controller.Preview(ctx, retry)
	if err != nil || plan.ResourceResume == nil {
		t.Fatal("controller lacks explicit native recovery", plan, err)
	}
	retry.ExpectedPlan = plan.ID
	if recovered, err := controller.Dispatch(ctx, retry); err != nil || recovered.Status != "needs-action" {
		t.Fatal("controller could not recover its exact native operation", recovered, err)
	}
	retry.ExpectedPlan = ""
	dispatchApproved(t, controller, retry)
	dispatchApproved(t, controller, Request{Schema: 1, Mode: "update"})
	require("", "unlink", tap+"/"+names[0])
	dispatchApproved(t, controller, Request{Schema: 1, Mode: "repair"})
	dispatchApproved(t, controller, Request{Schema: 1, Mode: "apply", Selected: []string{"second"}, RemoveShared: []string{"tool.first"}})
	if _, present := inspect()[names[3]]; !present {
		t.Fatal("controller removed the retained root's shared dependency")
	}
	dispatchApproved(t, controller, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.second"}})
	coreState, err := LoadState(controller.StatePath, controller.Home)
	if err != nil || coreState.Transaction != nil || len(coreState.Selected) != 0 || len(coreState.Receipts) != 0 {
		t.Fatal("controller did not finish native removal", coreState, err)
	}
	state, err = brew.ledger()
	if err != nil || len(state.Roots) != 0 || len(state.Pool) != 0 {
		t.Fatal("controller stranded its attributable native pool", state, err)
	}
	if _, present := inspect()[names[4]]; !present {
		t.Fatal("controller swept the pre-existing orphan")
	}
	require("", "uninstall", "--formula", names[4])
	after := require("", "list", "--formula", "--versions")
	if !bytes.Equal(before, after) {
		t.Fatal("native contract changed unrelated installed formulae")
	}
	t.Log("operation-marked completion summaries distinguish installed and reused kegs; native uninstall protects consumers without sweeping unrelated orphans")
}
