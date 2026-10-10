package installer

import (
	"context"
	"testing"
)

func TestPendingRemovalObservationCannotForgetOwnership(t *testing.T) {
	state, observed := installed()
	observed["node"] = Observation{Pending: "publication is incomplete"}
	plan, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"node"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if op := operation(t, plan, "node"); op.Action != "retain" {
		t.Fatalf("unknown presence discarded ownership: %+v", op)
	}
}

func TestCompletedSharedRemovalSurvivesRepeatedFinalizationFailure(t *testing.T) {
	c, files := engineFixture(t)
	single := &finalizingFileDriver{fileDriver: files}
	var driver Driver = single
	setFailure := func(fail bool) { single.failFinalization = fail }
	apply := func(request Request) (Result, error) {
		t.Helper()
		state, err := LoadState(files.statePath, files.home)
		if err != nil {
			t.Fatal(err)
		}
		observed, err := Observe(context.Background(), c, linux, state, request, driver)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := PlanChanges(c, linux, state, request, observed, "source")
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = plan.ID
		return Execute(context.Background(), c, linux, "source", files.home, files.statePath, request, driver)
	}
	if result, err := apply(Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}); err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	setFailure(true)
	if _, err := apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"node"}}); err == nil {
		t.Fatal("fixture did not interrupt")
	}
	if _, err := apply(Request{Schema: 1, Mode: "apply", Retry: true}); err == nil {
		t.Fatal("fixture did not interrupt again")
	}
	setFailure(false)
	if result, err := apply(Request{Schema: 1, Mode: "apply", Retry: true}); err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	state, err := LoadState(files.statePath, files.home)
	if err != nil || state.Transaction != nil {
		t.Fatal("removal retry left unfinished intent", err)
	}
	if _, retained := state.Receipts["node"]; retained {
		t.Fatal("completed absent resource kept its receipt")
	}
}
