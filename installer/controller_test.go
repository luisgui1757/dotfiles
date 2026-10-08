package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Only human input/output is scripted. Observation, approval, journaling and
// mutation use the real controller, engine and filesystem provider boundary.
type scriptedInteraction struct {
	t              *testing.T
	answers        [][]string
	accept         bool
	cancelAt       int
	menus          [][]Choice
	plans          []Plan
	reports        []Result
	beforeApproval func()
}

func (u *scriptedInteraction) Choose(_ context.Context, _ string, choices []Choice, _ []string, _ bool) ([]string, error) {
	u.menus = append(u.menus, choices)
	if u.cancelAt == len(u.menus) {
		return nil, ErrCancelled
	}
	if len(u.answers) == 0 {
		u.t.Fatal("unexpected additional menu", choices)
	}
	answer := u.answers[0]
	u.answers = u.answers[1:]
	return answer, nil
}
func (u *scriptedInteraction) Review(_ context.Context, _ *Catalog, plan Plan) (bool, error) {
	u.plans = append(u.plans, plan)
	if u.beforeApproval != nil {
		u.beforeApproval()
	}
	return u.accept, nil
}
func (u *scriptedInteraction) Report(result Result) error {
	u.reports = append(u.reports, result)
	return nil
}

func controllerFixture(t *testing.T) (Controller, *fileDriver) {
	catalog, driver := engineFixture(t)
	return Controller{Catalog: catalog, Context: linux, Source: "source", Home: driver.home, StatePath: driver.statePath, Driver: driver}, driver
}

func runController(t *testing.T, c Controller, answers ...[]string) Result {
	t.Helper()
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Schema == 0 && len(answers) > 0 && slices.Equal(answers[0], []string{"apply"}) {
		answers = answers[1:]
	}
	u := &scriptedInteraction{t: t, answers: answers, accept: true}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "ready" || len(u.answers) != 0 {
		t.Fatalf("controller result=%+v error=%v unused answers=%v", result, err, u.answers)
	}
	return result
}

func TestControllerSharedConsumersInstallUpdateRemoveInEitherOrder(t *testing.T) {
	for _, order := range [][]string{{"editor", "agent"}, {"agent", "editor"}} {
		t.Run(order[0]+"-first", func(t *testing.T) {
			c, d := controllerFixture(t)
			runController(t, c, []string{"apply"}, []string{"editor", "agent"})
			if !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
				t.Fatal("dependencies were not installed once", d.calls)
			}
			runController(t, c, []string{"update"})
			state, err := LoadState(d.statePath, d.home)
			if err != nil || !slices.Equal(state.Selected, []string{"agent", "editor"}) {
				t.Fatal("update changed selection", state, err)
			}
			runController(t, c, []string{"remove"}, []string{order[0]})
			if _, err := os.Stat(filepath.Join(d.dir, "node")); err != nil {
				t.Fatal("first consumer removal broke shared runtime", err)
			}
			runController(t, c, []string{"remove"}, []string{order[1]}, []string{"node"})
			for _, id := range []string{"node", "plugins"} {
				if _, err := os.Stat(filepath.Join(d.dir, id)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("final removal left resource", id, err)
				}
			}
		})
	}
}

func TestControllerCancelBeforeApplyDoesNotCreateState(t *testing.T) {
	for _, cancel := range []int{0, 1, 2} {
		c, d := controllerFixture(t)
		u := &scriptedInteraction{t: t, answers: [][]string{{"editor"}}, cancelAt: cancel}
		result, err := c.Run(context.Background(), u)
		if err != nil || result.Status != "cancelled" || len(d.calls) != 0 {
			t.Fatal(result, err, d.calls)
		}
		if _, err := os.Stat(filepath.Dir(d.statePath)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cancel created managed state", err)
		}
	}
}

func TestControllerMenuExcludesDependenciesAndRejectsStaleApproval(t *testing.T) {
	c, d := controllerFixture(t)
	u := &scriptedInteraction{t: t, answers: [][]string{{"agent"}}, accept: true}
	u.beforeApproval = func() {
		if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("external installation"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := c.Run(context.Background(), u)
	if err == nil || result.Status != "failed" || len(d.calls) != 0 {
		t.Fatal("stale approval mutated a resource", result, err)
	}
	for _, option := range u.menus[0] {
		r, ok := c.Catalog.Resource(option.ID)
		if !ok || !r.Capability {
			t.Fatal("dependency exposed as top-level choice", option)
		}
	}
}

func TestControllerKeepAndLaterRemoveUnusedSharedDependency(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"agent"})
	runController(t, c, []string{"remove"}, []string{"agent"}, []string{})
	state, err := LoadState(d.statePath, d.home)
	if err != nil || !slices.Equal(state.Keep, []string{"node"}) {
		t.Fatal("unchecked shared resource was not recorded as Keep", state, err)
	}
	runController(t, c, []string{"update"})
	runController(t, c, []string{"apply"}, []string{}, []string{}, []string{"node"})
	state, err = LoadState(d.statePath, d.home)
	if err != nil || len(state.Keep) != 0 || len(state.Receipts) != 0 {
		t.Fatal("explicit keep removal did not reconcile", state, err)
	}
}

func TestControllerCheckAndMachinePreviewAreReadOnly(t *testing.T) {
	c, d := controllerFixture(t)
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
	if err != nil || preview.Status != "preview" || preview.Plan.Mode != "apply" {
		t.Fatal(preview, err)
	}
	if _, err := os.Stat(filepath.Dir(d.statePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview created state", err)
	}
	result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "ready" || len(d.calls) != 0 {
		t.Fatal("check mutated or requested apply", result, err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}, ExpectedPlan: preview.Plan.ID}
	result, err = c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "ready" {
		t.Fatal("same-intent machine approval failed", result, err)
	}
}

func TestControllerRepairPreservesChoicesAndRecoveryRequiresExplicitAction(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"editor"})
	d.unhealthy = "node"
	runController(t, c, []string{"repair"})
	state, err := LoadState(d.statePath, d.home)
	if err != nil || !slices.Equal(state.Selected, []string{"editor"}) || d.unhealthy != "" {
		t.Fatal("repair did not restore selected dependency", err)
	}
	d.fail = "node"
	u := &scriptedInteraction{t: t, answers: [][]string{{"update"}}, accept: true}
	if _, err := c.Run(context.Background(), u); err == nil {
		t.Fatal("fixture did not interrupt")
	}
	u = &scriptedInteraction{t: t, answers: [][]string{{"exit"}}}
	if _, err := c.Run(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(u.menus[0], func(o Choice) bool { return o.ID == "apply" || o.ID == "update" }) {
		t.Fatal("unfinished operation exposed fresh mutation")
	}
	d.fail = ""
	runController(t, c, []string{"retry"})
}
