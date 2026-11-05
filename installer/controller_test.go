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

// Only human input/output is scripted. Observation, approval, journaling and
// mutation use the real controller, engine and filesystem provider boundary.
type scriptedInteraction struct {
	t              *testing.T
	answers        [][]string
	accept         bool
	cancelAt       int
	menus          [][]Choice
	titles         []string
	initials       [][]string
	statuses       []func() ToolStatus
	notices        []string
	plans          []Plan
	details        []string
	reports        []Result
	beforeApproval func()
	reviewBack     int // Reviews answered with Back before accepting.
}

// goBack answers a menu like the Esc key.
var goBack = []string{"<back>"}

func (u *scriptedInteraction) Choose(_ context.Context, title string, choices []Choice, initial []string, _ bool) ([]string, error) {
	if title != "Choose the tools you want installed" {
		for _, choice := range choices {
			if choice.Category != "" {
				u.t.Fatal("action, adoption, retention and shared-removal choices must stay flat", title, choice)
			}
		}
	}
	u.menus = append(u.menus, choices)
	u.titles = append(u.titles, title)
	u.initials = append(u.initials, initial)
	if u.cancelAt == len(u.menus) {
		return nil, ErrCancelled
	}
	if len(u.answers) == 0 {
		u.t.Fatal("unexpected additional menu", choices)
	}
	answer := u.answers[0]
	u.answers = u.answers[1:]
	if slices.Equal(answer, goBack) {
		return nil, ErrBack
	}
	return answer, nil
}
func (u *scriptedInteraction) ChooseTools(ctx context.Context, menu ToolMenu) ([]string, error) {
	u.statuses = append(u.statuses, menu.Status)
	return u.Choose(ctx, menu.Title, menu.Choices, menu.Initial, true)
}

// Review answers each optional group in the order the controller once asked
// them as separate menus, revising the proposal after every answer.
func (u *scriptedInteraction) Review(ctx context.Context, _ *Catalog, proposal Proposal, revise Revise) (bool, error) {
	for _, group := range []string{ReviewKeep, ReviewAdopt, ReviewShared} {
		options, initial := []Choice{}, []string{}
		for _, choice := range proposal.Choices {
			if choice.Group == group {
				options = append(options, Choice{ID: choice.ID, Label: choice.Label, Detail: choice.Detail})
				if choice.Checked {
					initial = append(initial, choice.ID)
				}
			}
		}
		if len(options) == 0 {
			continue
		}
		answer, err := u.Choose(ctx, group, options, initial, true)
		if err != nil {
			return false, err
		}
		for i := range proposal.Choices {
			if proposal.Choices[i].Group == group {
				proposal.Choices[i].Checked = slices.Contains(answer, proposal.Choices[i].ID)
			}
		}
		if proposal, err = revise(ctx, proposal.Choices); err != nil {
			return false, err
		}
	}
	u.plans = append(u.plans, proposal.Plan)
	u.details = append(u.details, proposal.Details)
	u.notices = append(u.notices, proposal.Notice)
	if u.reviewBack > 0 {
		u.reviewBack--
		return false, ErrBack
	}
	if u.beforeApproval != nil {
		u.beforeApproval()
	}
	return u.accept, nil
}
func (u *scriptedInteraction) BeginApply(ctx context.Context) (context.Context, error) {
	return ctx, nil
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
			runController(t, c, []string{"apply"}, []string{order[1]})
			if _, err := os.Stat(filepath.Join(d.dir, "node")); err != nil {
				t.Fatal("first consumer removal broke shared runtime", err)
			}
			runController(t, c, []string{"apply"}, []string{}, []string{"node"})
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

// A changed system never applies the stale approval. Setup shows the fresh
// plan with a notice, and only its separate acceptance applies.
func TestControllerMenuExcludesDependenciesAndRejectsStaleApproval(t *testing.T) {
	c, d := controllerFixture(t)
	u := &scriptedInteraction{t: t, answers: [][]string{{"agent"}}, accept: true}
	changed := false
	u.beforeApproval = func() {
		if changed {
			return
		}
		changed = true
		if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("external installation"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "ready" || len(d.calls) != 0 {
		t.Fatal("stale approval mutated a resource", result, err, d.calls)
	}
	if len(u.plans) != 2 || u.plans[0].ID == u.plans[1].ID || u.notices[0] != "" || u.notices[1] == "" {
		t.Fatal("changed system was not reviewed again", len(u.plans), u.notices)
	}
	if u.plans[0].Operations[0].Action != "install" || u.plans[1].Operations[0].Action != "keep" {
		t.Fatal("fresh review does not describe the changed system", u.plans)
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
	runController(t, c, []string{"apply"}, []string{}, []string{})
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
	// Update repairs failed checks; it is the menu's maintenance action.
	runController(t, c, []string{"update"})
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

func TestMainMenuStartsOnTheLikelyAction(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"editor"})
	u := &scriptedInteraction{t: t, answers: [][]string{{"exit"}}}
	if _, err := c.Run(context.Background(), u); err != nil || !slices.Equal(u.initials[0], []string{"apply"}) {
		t.Fatal("unchanged checkout should start on Choose tools", u.initials, err)
	}
	// A rebuilt installer means the checkout changed: Update is likely next.
	c.Source = "newer source"
	u = &scriptedInteraction{t: t, answers: [][]string{{"exit"}}}
	if _, err := c.Run(context.Background(), u); err != nil || !slices.Equal(u.initials[0], []string{"update"}) {
		t.Fatal("changed checkout should start on Update", u.initials, err)
	}
	if !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
		t.Fatal("opening the menu changed something", d.calls)
	}
}

func TestFailedApplyIsReportedOnceWithItsLog(t *testing.T) {
	c, d := controllerFixture(t)
	d.fail = "node"
	u := &scriptedInteraction{t: t, answers: [][]string{{"editor"}}, accept: true}
	result, err := c.Run(context.Background(), u)
	var reported *ReportedError
	if !errors.As(err, &reported) || result.Status != "failed" || len(u.reports) != 1 {
		t.Fatal("failure was not marked as already reported", result, err, len(u.reports))
	}
	shown := u.reports[0]
	if shown.Status != "failed" || !strings.HasPrefix(shown.Message, reported.Err.Error()) || shown.Log == "" {
		t.Fatal("report lacks the failure or its log", shown)
	}
	if _, err := os.Stat(shown.Log); err != nil {
		t.Fatal(err)
	}
}
