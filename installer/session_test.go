package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// countingDriver counts complete inspections by their first resource.
type countingDriver struct {
	*fileDriver
	mutex  sync.Mutex
	rounds map[string]int
}

func (d *countingDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID == "node" {
		d.mutex.Lock()
		d.rounds["node"]++
		d.mutex.Unlock()
	}
	return d.fileDriver.Observe(ctx, r, receipt)
}

func (d *countingDriver) count() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.rounds["node"]
}

func sessionFixture(t *testing.T) (Controller, *countingDriver) {
	t.Helper()
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"agent"})
	counting := &countingDriver{fileDriver: d, rounds: map[string]int{}}
	c.Driver = counting
	return c, counting
}

func TestSessionInspectsEachResolvedSelectionOnce(t *testing.T) {
	c, d := sessionFixture(t)
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	s := newMenuSession(c, state)
	defer s.close()
	ctx := context.Background()
	// Apply with the recorded selection, Update and Check share one closure.
	for _, request := range []Request{{Schema: 1, Mode: "apply", Selected: []string{"agent"}}, {Schema: 1, Mode: "update"}, {Schema: 1, Mode: "check"}} {
		if _, err := s.plan(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	if d.count() != 1 {
		t.Fatal("unchanged selection was inspected again", d.count())
	}
	if _, err := s.plan(ctx, Request{Schema: 1, Mode: "apply", Selected: []string{"agent", "editor"}}); err != nil || d.count() != 2 {
		t.Fatal("a different resolved selection reused another observation", d.count(), err)
	}
	if err := s.refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.plan(ctx, Request{Schema: 1, Mode: "update"}); err != nil || d.count() != 3 {
		t.Fatal("refresh kept a stale observation", d.count(), err)
	}
}

func TestSessionReusesBackgroundInspectionAndJoinsItBeforeOtherWork(t *testing.T) {
	c, d := sessionFixture(t)
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	s := newMenuSession(c, state)
	ctx := context.Background()
	s.inspectInBackground(ctx, Request{Schema: 1, Mode: "apply", Selected: state.Selected})
	plan, err := s.plan(ctx, Request{Schema: 1, Mode: "update"})
	if err != nil || s.background != nil || d.count() != 1 {
		t.Fatal("foreground did not reuse the background inspection", d.count(), err)
	}
	if !slices.ContainsFunc(plan.Operations, func(op Operation) bool { return op.Resource == "node" }) {
		t.Fatal("reused inspection lost resources", plan)
	}
	// A preview elsewhere must not race the background's non-blocking lock.
	s.inspectInBackground(ctx, Request{Schema: 1, Mode: "apply", Selected: state.Selected})
	s.close()
	if s.background != nil {
		t.Fatal("close left a background inspection")
	}
	if _, err := c.Preview(ctx, Request{Schema: 1, Mode: "update"}); err != nil {
		t.Fatal("joined inspection still held the preview lock", err)
	}
}

func TestToolStatusLabelsRecordedToolsWorstFirst(t *testing.T) {
	c, d := sessionFixture(t)
	runController(t, c, []string{"apply"}, []string{"agent", "editor"})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	s := newMenuSession(c, state)
	defer s.close()
	if status := s.toolStatus(); !status.Ready || len(status.Tools) != 0 || status.Summary != "" {
		t.Fatal("status claimed an inspection that never ran", status)
	}
	s = newMenuSession(c, state)
	d.unhealthy = "plugins"
	s.inspectInBackground(context.Background(), Request{Schema: 1, Mode: "apply", Selected: state.Selected})
	<-s.background.done
	status := s.toolStatus()
	if !status.Ready || status.Tools["editor"] != "needs repair" || status.Tools["agent"] != "" || !strings.Contains(status.Summary, "1 of 2") {
		t.Fatal("status mislabelled the damaged tool", status)
	}
	d.unhealthy = ""
	if again := s.toolStatus(); again.Tools["editor"] != "needs repair" {
		t.Fatal("status was recomputed from a different inspection", again)
	}
}

func TestReviewRejectsChoicesItDidNotOffer(t *testing.T) {
	offered := []ReviewChoice{{Group: ReviewShared, ID: "node"}}
	checked := map[string]bool{}
	for _, returned := range [][]ReviewChoice{
		{},
		{{Group: ReviewShared, ID: "editor", Checked: true}},
		{{Group: ReviewAdopt, ID: "node", Checked: true}},
	} {
		if err := recordChoices(offered, returned, checked); err == nil {
			t.Fatal("forged review choice accepted", returned)
		}
	}
	if err := recordChoices(offered, []ReviewChoice{{Group: ReviewShared, ID: "node", Checked: true}}, checked); err != nil || !checked[reviewChoiceKey(ReviewShared, "node")] {
		t.Fatal(err, checked)
	}
}

// Toggling a decision in one review recomputes the dependent plan; the user
// never sees a separate screen per question.
func TestOneReviewRevisesSharedRemovalInPlace(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"agent"})
	var counts []int
	ui := &revisingInteraction{t: t, toggle: "node", counts: &counts}
	result, err := c.Run(context.Background(), ui.with([][]string{{"apply"}, {}}))
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	if len(counts) != 2 || counts[0] != 0 || counts[1] != 1 {
		t.Fatal("review did not revise removal counts in place", counts)
	}
	if _, err := os.Stat(filepath.Join(d.dir, "node")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("approved shared removal did not run", err)
	}
}

// revisingInteraction behaves like the terminal: one review, one toggle.
type revisingInteraction struct {
	*scriptedInteraction
	t      *testing.T
	toggle string
	counts *[]int
}

func (u *revisingInteraction) with(answers [][]string) *revisingInteraction {
	u.scriptedInteraction = &scriptedInteraction{t: u.t, answers: answers, accept: true}
	return u
}

func (u *revisingInteraction) Review(ctx context.Context, _ *Catalog, proposal Proposal, revise Revise) (bool, error) {
	removals := func(plan Plan) int {
		count := 0
		for _, op := range plan.Operations {
			if op.Action == "remove" {
				count++
			}
		}
		return count
	}
	*u.counts = append(*u.counts, removals(proposal.Plan))
	choices := slices.Clone(proposal.Choices)
	index := slices.IndexFunc(choices, func(choice ReviewChoice) bool { return choice.ID == u.toggle })
	if index < 0 {
		u.t.Fatal("review lacks the expected choice", choices)
	}
	choices[index].Checked = true
	revised, err := revise(ctx, choices)
	if err != nil {
		return false, err
	}
	*u.counts = append(*u.counts, removals(revised.Plan))
	return true, nil
}
