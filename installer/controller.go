package installer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// PreviewGuard holds both the existing engine lock and provider observation
// guards without creating files. A provider that already locks observation
// supplies this hook to keep the whole preview coherent without double locking.
type PreviewGuard interface {
	AcquirePreview(context.Context) (func() error, error)
}

type Controller struct {
	Catalog                 *Catalog
	Context                 Context
	Source, Home, StatePath string
	Driver                  Driver
}

func (c Controller) validate() error {
	if c.Catalog == nil || c.Driver == nil || c.Source == "" || !filepath.IsAbs(c.Home) || !filepath.IsAbs(c.StatePath) {
		return errors.New("controller requires its catalog, provider, source and absolute target/state paths")
	}
	return c.Catalog.Validate()
}

// Preview preserves the requested operation mode: an update preview authorizes
// that update, not a different apply request. It never initializes the ledger.
func (c Controller) Preview(ctx context.Context, request Request) (plan Plan, resultErr error) {
	if err := c.validate(); err != nil {
		return plan, err
	}
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	release, err := c.acquirePreview(ctx)
	if err != nil {
		return plan, err
	}
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		return plan, err
	}
	observed, err := Observe(ctx, c.Catalog, c.Context, state, request, c.Driver)
	if err != nil {
		return plan, err
	}
	return PlanChanges(c.Catalog, c.Context, state, request, observed, c.Source)
}

// Dispatch is also the machine boundary: without expected_plan it only
// previews; with it the engine independently verifies that exact approval.
// No default selection or environment variable can turn a preview into apply.
func (c Controller) Dispatch(ctx context.Context, request Request) (Result, error) {
	if err := c.validate(); err != nil {
		return Result{Status: "failed"}, err
	}
	if request.ExpectedPlan != "" {
		return Execute(ctx, c.Catalog, c.Context, c.Source, c.Home, c.StatePath, request, c.Driver)
	}
	plan, err := c.Preview(ctx, request)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if request.Mode == "check" {
		return checkResult(c.Catalog, plan), nil
	}
	return Result{Status: "preview", Plan: plan}, nil
}

var ErrCancelled = errors.New("installer interaction cancelled")

// ErrBack leaves the current screen for the main menu. It wraps ErrCancelled,
// so flows without a menu treat going back as cancellation.
var ErrBack = fmt.Errorf("%w: back to the main menu", ErrCancelled)

// ReportedError marks a failure the interaction already showed with its
// result, so an entrypoint can fail without printing it a second time.
type ReportedError struct{ Err error }

func (e *ReportedError) Error() string { return e.Err.Error() }
func (e *ReportedError) Unwrap() error { return e.Err }

type Choice struct{ ID, Label, Detail, Category string }

// ToolMenu is the tool selection screen. Initial holds the draft or recorded
// selection, Installed the recorded one; Status may become ready while the
// screen is open and is read only on the calling goroutine.
type ToolMenu struct {
	Title     string
	Choices   []Choice
	Initial   []string
	Installed []string
	Status    func() ToolStatus
}

// Interaction is an input/output boundary only. Every method must restore
// terminal input before returning. Review shows a proposal and calls revise,
// when it is not nil, after each change to the proposal's choices. BeginApply
// releases the screen before approved changes run, particularly before
// elevation or provider prompts, and returns the context that carries their
// progress display.
type Interaction interface {
	Choose(context.Context, string, []Choice, []string, bool) ([]string, error)
	ChooseTools(context.Context, ToolMenu) ([]string, error)
	Review(context.Context, *Catalog, Proposal, Revise) (bool, error)
	BeginApply(context.Context) (context.Context, error)
	Report(Result) error
}

// Run completes one explicitly selected lifecycle operation. Retrying exact
// provider publication can require a second review of remaining work; it never
// automatically accepts a new plan on behalf of the user.
func (c Controller) Run(ctx context.Context, ui Interaction) (result Result, resultErr error) {
	result.Status = "cancelled"
	defer func() {
		if errors.Is(resultErr, ErrCancelled) {
			resultErr = nil
			result = Result{Status: "cancelled"}
		}
		if resultErr != nil {
			result.Status = "failed"
		}
	}()
	if err := c.validate(); err != nil {
		return result, err
	}
	if ui == nil {
		return result, errors.New("interactive controller requires an input/output surface")
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		return result, err
	}
	// Choose tools also repairs failed checks and removes unchecked tools;
	// Update additionally applies this checkout's reviewed versions.
	actions := []Choice{
		{ID: "apply", Label: "Choose tools", Detail: "Install, remove or repair tools; required dependencies are included automatically."},
		{ID: "update", Label: "Update tools", Detail: "Apply this checkout's reviewed versions and repair failed checks; your selection stays the same."},
		{ID: "check", Label: "Check installation", Detail: "Inspect health and required actions without changing anything."},
	}
	initial := []string{"apply"}
	if state.Transaction != nil {
		initial = []string{"retry"}
		actions = []Choice{
			{ID: "retry", Label: "Resume interrupted operation", Detail: "Keep the original selection and removal choices; review recovery before continuing."},
			{ID: "abandon", Label: "Abandon interrupted operation", Detail: "Archive intent and preserve resources and receipts. Partial activation must finish first."},
			{ID: "check", Label: "Check installation", Detail: "Inspect the recorded selection without changing anything."},
		}
		if _, supported := c.Driver.(ResourceRestoreDriver); supported && state.Transaction.InFlight != "" {
			if restoration, err := c.Preview(ctx, Request{Schema: 1, Mode: "restore"}); err == nil {
				if len(restoration.Operations) == 1 && restoration.Operations[0].Observed.Restoring {
					actions = slices.DeleteFunc(actions, func(choice Choice) bool { return choice.ID == "retry" || choice.ID == "abandon" })
					initial = []string{"restore"}
				}
				actions = append(actions, Choice{ID: "restore", Label: "Restore interrupted files", Detail: "Use the saved journal to recover prior files, preserve conflicts and cancel the interrupted operation."})
			} else if !errors.Is(err, ErrNoResourceRestore) {
				if reportErr := ui.Report(Result{Status: "needs-action", Message: "Saved-file restoration unavailable: " + err.Error()}); reportErr != nil {
					return result, reportErr
				}
			}
		}
	} else if state.Source != "" && state.Source != c.Source || slices.ContainsFunc(slices.Collect(maps.Values(state.Receipts)), func(r Receipt) bool { return r.Status == "needs-action" || r.Status == "failed" }) {
		initial = []string{"update"} // The checkout changed or recorded work needs repair.
	}
	actions = append(actions, Choice{ID: "exit", Label: "Exit"})
	session := newMenuSession(c, state)
	defer session.close()
	if state.Schema != 0 && state.Transaction == nil && len(state.Selected)+len(state.Keep) != 0 {
		session.inspectInBackground(ctx, Request{Schema: 1, Mode: "apply", Selected: state.Selected})
	}
	// Going back from any later screen returns to this menu, keeping the tool
	// choices made so far. A first run opens tool selection directly.
	menu := state.Schema != 0
	for {
		mode := "apply"
		if menu {
			choice, err := ui.Choose(ctx, "What would you like to do?", actions, initial, false)
			if errors.Is(err, ErrBack) {
				continue
			}
			if err != nil {
				return result, err
			}
			if err := validateChoice(choice, actions, false); err != nil {
				return result, err
			}
			mode = choice[0]
		}
		menu = true
		if mode == "exit" {
			return result, nil
		}
		result, resultErr = c.runMode(ctx, ui, session, mode)
		// A completed check returns to the menu; it changed nothing.
		if !errors.Is(resultErr, ErrBack) && (mode != "check" || resultErr != nil) {
			return result, resultErr
		}
		result = Result{Status: "cancelled"}
	}
}

func (c Controller) runMode(ctx context.Context, ui Interaction, session *menuSession, mode string) (Result, error) {
	result := Result{Status: "cancelled"}
	state := session.state
	request := Request{Schema: 1, Mode: mode}
	if mode == "retry" {
		request.Mode, request.Retry = state.Transaction.Plan.Mode, true
	}
	if mode == "check" {
		plan, err := session.plan(ctx, request)
		if err != nil {
			return result, err
		}
		result = checkResult(c.Catalog, plan)
		return result, ui.Report(result)
	}
	if mode == "apply" {
		options := []Choice{}
		for _, r := range c.Catalog.Resources {
			if r.Capability && r.Available(c.Context) {
				options = append(options, Choice{ID: r.ID, Label: r.Name, Detail: r.Description, Category: r.Category})
			}
		}
		initial := state.Selected
		if draft, ok := session.drafts[mode]; ok {
			initial = draft
		}
		selected := []string{}
		if len(options) > 0 {
			var err error
			selected, err = ui.ChooseTools(ctx, ToolMenu{Title: "Choose the tools you want installed", Choices: options, Initial: initial, Installed: state.Selected, Status: session.toolStatus})
			if err != nil {
				return result, err
			}
			if err := validateChoice(selected, options, true); err != nil {
				return result, err
			}
			session.drafts[mode] = selected
		}
		request.Selected = selected
	}
	checked := map[string]bool{}
	notice := ""
	for {
		proposal, current, err := session.propose(ctx, request, mode == "apply", checked)
		if err != nil {
			return result, err
		}
		proposal.Notice = notice
		revise := func(ctx context.Context, choices []ReviewChoice) (Proposal, error) {
			if err := recordChoices(proposal.Choices, choices, checked); err != nil {
				return Proposal{}, err
			}
			next, nextRequest, err := session.propose(ctx, request, mode == "apply", checked)
			if err != nil {
				return Proposal{}, err
			}
			proposal, current = next, nextRequest
			return proposal, nil
		}
		accepted, err := ui.Review(ctx, c.Catalog, proposal, revise)
		if err != nil || !accepted {
			return result, err
		}
		current.ExpectedPlan = proposal.Plan.ID
		// The apply takes the same non-blocking locks as an inspection.
		session.close()
		applying, err := ui.BeginApply(ctx)
		if err != nil {
			return result, err
		}
		result, err = c.Dispatch(applying, current)
		if errors.Is(err, ErrPlanChanged) {
			// Nothing was changed. Show the fresh plan instead of failing;
			// approval still requires the user to accept it.
			if err := session.refresh(); err != nil {
				return result, err
			}
			notice = "Your system changed after the review. This is the updated plan."
			continue
		}
		if err != nil {
			result.Status, result.Message = "failed", strings.TrimSpace(err.Error()+"\n"+result.Message)
		}
		if reportErr := ui.Report(result); reportErr != nil || err == nil {
			return result, errors.Join(err, reportErr)
		}
		return result, &ReportedError{Err: err}
	}
}

func validateChoice(selected []string, options []Choice, multiple bool) error {
	if !multiple && len(selected) != 1 {
		return errors.New("menu requires exactly one choice")
	}
	seen := map[string]bool{}
	for _, id := range selected {
		if seen[id] || !slices.ContainsFunc(options, func(o Choice) bool { return o.ID == id }) {
			return fmt.Errorf("invalid menu choice %q", id)
		}
		seen[id] = true
	}
	return nil
}
