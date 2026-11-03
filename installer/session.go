package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A reused inspection never authorizes a change: Execute observes again and
// must reproduce the approved plan identity. The bound keeps statuses and plans
// from describing a machine that has moved on while menus stayed open.
const inspectionReuse = 5 * time.Minute

type inspection struct {
	observed map[string]Observation
	at       time.Time
}

type backgroundInspection struct {
	key      string
	cancel   context.CancelFunc
	done     chan struct{}
	observed map[string]Observation
	err      error
	at       time.Time
}

// menuSession serves one interactive run. Each resolved selection is
// inspected once; its plans, Check, the tool status and every review revision
// reuse that observation. Only the controller goroutine uses a session, and an
// inspection running in the background is cancelled or joined before any other
// preview or the apply takes the same non-blocking locks.
type menuSession struct {
	c          Controller
	state      State
	drafts     map[string][]string
	inspected  map[string]inspection
	background *backgroundInspection
	statusErr  error
	status     *ToolStatus
}

func newMenuSession(c Controller, state State) *menuSession {
	return &menuSession{c: c, state: state, drafts: map[string][]string{}, inspected: map[string]inspection{}}
}

// inspectionKey names everything an observation depends on besides the loaded
// state: the resolved closure binds generated configuration and native keep
// sets, and selects which resources are observed.
func (s *menuSession) inspectionKey(request Request) (string, error) {
	effective, err := effectiveRequest(s.state, request)
	if err != nil {
		return "", err
	}
	ids, err := s.c.Catalog.Closure(append(slices.Clone(effective.Selected), effective.Keep...), s.c.Context)
	if err != nil {
		return "", err
	}
	return strings.Join(sortedUnique(ids), "\n"), nil
}

func (c Controller) acquirePreview(ctx context.Context) (func() error, error) {
	if provider, ok := c.Driver.(PreviewGuard); ok {
		return provider.AcquirePreview(ctx)
	}
	release, err := lockFile(filepath.Dir(c.StatePath), false)
	if errors.Is(err, os.ErrNotExist) {
		return func() error { return nil }, nil
	}
	return release, err
}

func (c Controller) observeGuarded(ctx context.Context, state State, request Request) (map[string]Observation, error) {
	release, err := c.acquirePreview(ctx)
	if err != nil {
		return nil, err
	}
	observed, err := Observe(ctx, c.Catalog, c.Context, state, request, c.Driver)
	return observed, errors.Join(err, release())
}

// inspectInBackground starts observing while the user reads a menu.
func (s *menuSession) inspectInBackground(ctx context.Context, request Request) {
	key, err := s.inspectionKey(request)
	if err != nil {
		return // A later foreground inspection computes the same key and reports it.
	}
	ctx, cancel := context.WithCancel(ctx)
	b := &backgroundInspection{key: key, cancel: cancel, done: make(chan struct{}), at: time.Now()}
	go func() {
		defer close(b.done)
		b.observed, b.err = s.c.observeGuarded(ctx, s.state, request)
	}()
	s.background = b
}

// collect takes a finished background inspection, without waiting.
func (s *menuSession) collect() {
	b := s.background
	if b == nil {
		return
	}
	select {
	case <-b.done:
	default:
		return
	}
	s.background = nil
	if b.err != nil {
		s.statusErr = b.err
		return
	}
	s.inspected[b.key] = inspection{observed: b.observed, at: b.at}
}

func (s *menuSession) close() {
	if b := s.background; b != nil {
		b.cancel()
		<-b.done
		s.background = nil
	}
}

// refresh discards every inspection after the system changed under a review.
func (s *menuSession) refresh() error {
	s.close()
	state, err := LoadState(s.c.StatePath, s.c.Home)
	if err != nil {
		return err
	}
	s.state, s.inspected, s.statusErr, s.status = state, map[string]inspection{}, nil, nil
	return nil
}

func (s *menuSession) observe(ctx context.Context, request Request) (map[string]Observation, error) {
	key, err := s.inspectionKey(request)
	if err != nil {
		return nil, err
	}
	if entry, ok := s.inspected[key]; ok && time.Since(entry.at) < inspectionReuse {
		return entry.observed, nil
	}
	if b := s.background; b != nil {
		if b.key != key {
			b.cancel()
		}
		<-b.done
		s.background = nil
		if b.key == key && b.err == nil && time.Since(b.at) < inspectionReuse {
			s.inspected[key] = inspection{observed: b.observed, at: b.at}
			return b.observed, nil
		}
		// A cancelled, failed or stale background inspection proves nothing;
		// the foreground inspection below reports any persistent error.
	}
	at := time.Now()
	observed, err := s.c.observeGuarded(ctx, s.state, request)
	if err != nil {
		return nil, err
	}
	s.inspected[key] = inspection{observed: observed, at: at}
	return observed, nil
}

func (s *menuSession) plan(ctx context.Context, request Request) (Plan, error) {
	if request.Mode == "abandon" || request.Mode == "restore" {
		s.close()
		return s.c.Preview(ctx, request)
	}
	observed, err := s.observe(ctx, request)
	if err != nil {
		return Plan{}, err
	}
	return PlanChanges(s.c.Catalog, s.c.Context, s.state, request, observed, s.c.Source)
}

// ToolStatus describes recorded tools for the selection screen. It may become
// Ready after the screen opens.
type ToolStatus struct {
	Ready   bool
	Summary string
	Tools   map[string]string
}

// toolStatus labels recorded tools from the plans Choose tools and Update would
// make for the unchanged selection, worst first: needs action, needs repair,
// update available.
func (s *menuSession) toolStatus() ToolStatus {
	if s.status != nil {
		return *s.status
	}
	status := s.computeToolStatus()
	if status.Ready {
		s.status = &status // The screen polls; plan once per inspection.
	}
	return status
}

func (s *menuSession) computeToolStatus() ToolStatus {
	if len(s.state.Selected) == 0 {
		return ToolStatus{Ready: true}
	}
	s.collect()
	request := Request{Schema: 1, Mode: "apply", Selected: s.state.Selected}
	key, err := s.inspectionKey(request)
	if err != nil {
		return ToolStatus{Ready: true, Summary: "Could not check installed tools: " + err.Error()}
	}
	entry, ok := s.inspected[key]
	switch {
	case ok:
	case s.background != nil && s.background.key == key:
		return ToolStatus{Summary: "Checking installed tools..."}
	case s.statusErr != nil:
		return ToolStatus{Ready: true, Summary: "Could not check installed tools: " + s.statusErr.Error()}
	default:
		return ToolStatus{Ready: true}
	}
	current, err := PlanChanges(s.c.Catalog, s.c.Context, s.state, request, entry.observed, s.c.Source)
	if err != nil {
		return ToolStatus{Ready: true, Summary: "Could not check installed tools: " + err.Error()}
	}
	update, err := PlanChanges(s.c.Catalog, s.c.Context, s.state, Request{Schema: 1, Mode: "update"}, entry.observed, s.c.Source)
	if err != nil {
		return ToolStatus{Ready: true, Summary: "Could not check installed tools: " + err.Error()}
	}
	tools := map[string]string{}
	rank := map[string]int{"update available": 1, "needs repair": 2, "needs action": 3}
	mark := func(op Operation, label string) {
		for _, root := range op.RequiredBy {
			if rank[label] > rank[tools[root]] {
				tools[root] = label
			}
		}
	}
	for _, op := range current.Operations {
		switch op.Action {
		case "pending":
			mark(op, "needs action")
		case "repair":
			mark(op, "needs repair")
		case "install":
			// A recorded component that disappeared is damage; a component the
			// current checkout newly requires is part of an update.
			if receipt, recorded := s.state.Receipts[op.Resource]; recorded && receipt.Status != "removed" {
				mark(op, "needs repair")
			} else {
				mark(op, "update available")
			}
		}
	}
	for _, op := range update.Operations {
		if op.Action == "update" && op.Observed.Desired != "" {
			mark(op, "update available")
		}
	}
	summary := fmt.Sprintf("All %d installed tools are healthy and current.", len(s.state.Selected))
	if len(tools) > 0 {
		summary = fmt.Sprintf("%d of %d installed tools need attention.", len(tools), len(s.state.Selected))
	}
	return ToolStatus{Ready: true, Summary: summary, Tools: tools}
}

// ReviewChoice is an optional decision shown with the confirmation. Toggling
// one revises the whole proposal.
type ReviewChoice struct {
	Group, ID, Label, Detail string
	Checked                  bool
}

// Proposal is a reviewed plan, its complete details and its optional choices.
type Proposal struct {
	Plan    Plan
	Details string
	Notice  string
	Choices []ReviewChoice
}

// Revise recomputes a proposal after the user changes its choices.
type Revise func(context.Context, []ReviewChoice) (Proposal, error)

const (
	ReviewKeep   = "Keep retained resources"
	ReviewAdopt  = "Replace existing configurations (originals are preserved)"
	ReviewShared = "Also remove unused shared dependencies"
	ReviewLegacy = "Optional removals and legacy settings restoration"
)

func reviewChoiceKey(group, id string) string { return group + "\x00" + id }

// propose applies the review's decisions in their dependent order: retained
// resources shape the plan that offers adoption, and the adopted plan decides
// which shared dependencies become removable. Unchecked shared dependencies
// become Keep roots; recovery-only mappings never do.
func (s *menuSession) propose(ctx context.Context, base Request, keepChoices bool, checked map[string]bool) (Proposal, Request, error) {
	request := base
	choices := []ReviewChoice{}
	isChecked := func(group, id string, initial bool) bool {
		if value, ok := checked[reviewChoiceKey(group, id)]; ok {
			return value
		}
		return initial
	}
	if keepChoices && len(s.state.Keep) != 0 {
		request.Keep = []string{}
		for _, id := range s.state.Keep {
			r, ok := s.c.Catalog.Resource(id)
			if !ok {
				return Proposal{}, request, fmt.Errorf("retained resource %s requires catalog migration", id)
			}
			if r.ReplacedBy != "" {
				continue // Recovery-only files remain protected without a Keep root.
			}
			keep := isChecked(ReviewKeep, id, true)
			choices = append(choices, ReviewChoice{Group: ReviewKeep, ID: id, Label: r.Name, Detail: "Keep this resource even without a selected tool.", Checked: keep})
			if keep {
				request.Keep = append(request.Keep, id)
			}
		}
	}
	plan, err := s.plan(ctx, request)
	if err != nil {
		return Proposal{}, request, err
	}
	if (request.Mode == "apply" || request.Mode == "update" || request.Mode == "repair") && !request.Retry {
		adoptions := s.adoptionChoices(request, plan)
		if len(adoptions) != 0 {
			request.Adopt = []string{}
			for i := range adoptions {
				adoptions[i].Checked = isChecked(ReviewAdopt, adoptions[i].ID, false)
				if adoptions[i].Checked {
					request.Adopt = append(request.Adopt, adoptions[i].ID)
				}
			}
			choices = append(choices, adoptions...)
			if plan, err = s.plan(ctx, request); err != nil {
				return Proposal{}, request, err
			}
		}
	}
	if request.Mode == "apply" && !request.Retry {
		shared, err := s.sharedRemovalChoices(ctx, request, plan)
		if err != nil {
			return Proposal{}, request, err
		}
		if len(shared) != 0 {
			group := ReviewShared
			for _, choice := range shared {
				if r, _ := s.c.Catalog.Resource(choice.ID); r.ReplacedBy != "" {
					group = ReviewLegacy
				}
			}
			request.RemoveShared = []string{}
			if request.Keep == nil {
				request.Keep = slices.Clone(s.state.Keep)
			}
			for i := range shared {
				shared[i].Group = group
				shared[i].Checked = isChecked(group, shared[i].ID, false)
				r, _ := s.c.Catalog.Resource(shared[i].ID)
				if shared[i].Checked {
					request.RemoveShared = append(request.RemoveShared, shared[i].ID)
				} else if r.ReplacedBy == "" {
					request.Keep = append(request.Keep, shared[i].ID)
				}
			}
			choices = append(choices, shared...)
			if plan, err = s.plan(ctx, request); err != nil {
				return Proposal{}, request, err
			}
		}
	}
	return Proposal{Plan: plan, Choices: choices}, request, nil
}

func (s *menuSession) adoptionChoices(request Request, plan Plan) []ReviewChoice {
	choices := []ReviewChoice{}
	for _, op := range plan.Operations {
		r, _ := s.c.Catalog.Resource(op.Resource)
		if op.Action != "retain" && op.Action != "remove" && op.Action != "forget" && op.Observed.Present && op.Observed.Adoptable && op.Observed.Pending == "" &&
			!r.Retain && r.ReplacedBy == "" && r.Scope != "host" && (request.Mode == "apply" || op.Ownership == "created") && (op.Ownership != "created" || !sameArtifact(s.state.Receipts[r.ID].After, op.Observed)) {
			detail := "Preserve the current configuration as a recoverable baseline, then apply the selected configuration."
			if op.Ownership == "created" {
				detail = "Save your edits separately, apply the selected configuration, and keep the original uninstall baseline."
			}
			choices = append(choices, ReviewChoice{Group: ReviewAdopt, ID: r.ID, Label: r.Name, Detail: detail})
		}
	}
	return choices
}

func (s *menuSession) sharedRemovalChoices(ctx context.Context, request Request, plan Plan) ([]ReviewChoice, error) {
	choices := []ReviewChoice{}
	for _, op := range plan.Operations {
		r, _ := s.c.Catalog.Resource(op.Resource)
		if op.Action == "retain" && r.ReplacedBy != "" && op.Observed.Pending == "" && op.Observed.Adoptable && (op.Ownership == "created" || op.Ownership == "uncertain") {
			choices = append(choices, ReviewChoice{ID: r.ID, Label: r.Name, Detail: "Save the current file in adjacent recovery storage and restore its original baseline. Changes to the earlier managed settings become inactive until you copy them back; the result reports the saved path."})
			continue
		}
		if op.Action != "retain" || r.Retain || op.Observed.Pending != "" || len(op.Observed.Consumers) != 0 || op.Ownership != "created" ||
			!sameArtifact(s.state.Receipts[r.ID].After, op.Observed) || !(r.Shared || r.Scope == "machine" || op.Observed.Scope == "machine") {
			continue
		}
		choices = append(choices, ReviewChoice{ID: r.ID, Label: r.Name, Detail: "Created by this installer; another application may use it. " + op.Reason})
	}
	if len(choices) == 0 {
		return choices, nil
	}
	// Retained applications can still require an otherwise eligible resource.
	// Ask the planner which candidates really become removable together; do not
	// create Keep roots for resources that were never removal choices.
	trial := request
	trial.RemoveShared = slices.Clone(request.RemoveShared)
	for _, choice := range choices {
		trial.RemoveShared = append(trial.RemoveShared, choice.ID)
	}
	removal, err := s.plan(ctx, trial)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(choices, func(choice ReviewChoice) bool {
		return !slices.ContainsFunc(removal.Operations, func(op Operation) bool { return op.Resource == choice.ID && op.Action == "remove" })
	}), nil
}

// recordChoices accepts exactly the current proposal's choices back from the
// interaction and remembers their checked state for later revisions.
func recordChoices(offered, returned []ReviewChoice, checked map[string]bool) error {
	if len(offered) != len(returned) {
		return errors.New("review returned a different set of choices")
	}
	seen := map[string]bool{}
	for _, choice := range returned {
		key := reviewChoiceKey(choice.Group, choice.ID)
		if seen[key] || !slices.ContainsFunc(offered, func(o ReviewChoice) bool { return o.Group == choice.Group && o.ID == choice.ID }) {
			return fmt.Errorf("invalid review choice %q", choice.ID)
		}
		seen[key] = true
		checked[key] = choice.Checked
	}
	return nil
}

// checkResult turns a read-only plan into the Check report.
func checkResult(c *Catalog, plan Plan) Result {
	status := "ready"
	reasons := []string{}
	if plan.InterruptedSource != "" {
		reasons = append(reasons, "Resume the unfinished operation using installer executable SHA-256 "+plan.InterruptedSource+" and its original matching checkout; see docs/UPGRADING.md. Abandon is available only when provider publication is unchanged or complete.")
	}
	for _, op := range plan.Operations {
		if op.Action != "keep" && op.Action != "retain" && op.Action != "forget" {
			r, _ := c.Resource(op.Resource)
			reasons = append(reasons, r.Name+" ("+r.ID+"): "+op.Reason)
		}
	}
	message := "All selected resources passed verification."
	if len(reasons) > 0 {
		status, message = "needs-action", strings.Join(reasons, "\n")
	}
	preserved, unverified, notes := []string{}, []string{}, []string{}
	for _, op := range plan.Operations {
		preserved = append(preserved, op.Observed.Preserved...)
		unverified = append(unverified, op.Observed.UnverifiedApplications...)
		notes = append(notes, op.Observed.Notes...)
	}
	for _, path := range ExistingPreservedPaths(sortedUnique(preserved)) {
		message += "\nPreserved files for inspection: " + path
	}
	for _, name := range sortedUnique(unverified) {
		message += "\nExternal application not exercised: " + name
	}
	for _, note := range sortedUnique(notes) {
		message += "\nNote: " + note
	}
	return Result{Status: status, Plan: plan, Message: message}
}
