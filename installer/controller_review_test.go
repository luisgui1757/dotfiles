package installer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCheckNamesUnhealthyResourcesAndRecovery(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"agent"})
	d.unhealthy = "node"
	result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, "node") {
		t.Fatalf("check hid the failing resource: %+v %v", result, err)
	}
	d.unhealthy = ""
	c.Driver = &finalizingFileDriver{fileDriver: d, failFinalization: true}
	request := Request{Schema: 1, Mode: "update"}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not fail finalization")
	}
	before, err := os.ReadFile(d.statePath)
	if err != nil {
		t.Fatal(err)
	}
	result, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, "Resume") || !strings.Contains(result.Message, c.Source) {
		t.Fatalf("unfinished operation was reported ready: %+v %v", result, err)
	}
	after, err := os.ReadFile(d.statePath)
	if err != nil || string(before) != string(after) {
		t.Fatal("check changed the ledger", err)
	}
}

func TestSharedRemovalDoesNotOfferPrerequisiteOfRetainedApplication(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"editor"})
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	r := state.Receipts["plugins"]
	r.Ownership = "reused" // A pre-existing application must survive deselection.
	r.Before = r.After
	state.Receipts["plugins"] = r
	if err := SaveState(d.statePath, state); err != nil {
		t.Fatal(err)
	}
	u := &scriptedInteraction{t: t, answers: [][]string{{"remove"}, {"editor"}}, accept: true}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	state, err = LoadState(d.statePath, d.home)
	if err != nil || len(state.Keep) != 0 || len(u.menus) != 2 {
		t.Fatal("impossible removal created an unsolicited Keep root", state.Keep, err)
	}
	if _, err := os.Stat(filepath.Join(d.dir, "node")); err != nil {
		t.Fatal("retained application lost its runtime", err)
	}
}

func TestDispatchRequiresExplicitApplySelection(t *testing.T) {
	c, _ := controllerFixture(t)
	if _, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply"}); err == nil {
		t.Fatal("missing selection was accepted as remove everything")
	}
	if _, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}}); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryRefusalNamesOriginalSource(t *testing.T) {
	c, d := controllerFixture(t)
	d.fail = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not interrupt")
	}
	c.Source = "different-source"
	_, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Retry: true})
	if err == nil || !strings.Contains(err.Error(), "source\"") || strings.Contains(err.Error(), "or explicitly abandon") {
		t.Fatal("source mismatch gave an ambiguous or unsafe recovery path", err)
	}
}

func TestFirstRunOpensCapabilitySelection(t *testing.T) {
	c, _ := controllerFixture(t)
	u := &scriptedInteraction{t: t, cancelAt: 1}
	if _, err := c.Run(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if len(u.menus) != 1 || !slices.ContainsFunc(u.menus[0], func(choice Choice) bool { return choice.ID == "editor" }) {
		t.Fatal("first run did not open tool selection", u.menus)
	}
}
