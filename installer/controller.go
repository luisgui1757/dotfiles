package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	var release func() error
	var err error
	if provider, ok := c.Driver.(PreviewGuard); ok {
		release, err = provider.AcquirePreview(ctx)
	} else {
		release, err = lockFile(filepath.Dir(c.StatePath), false)
		if errors.Is(err, os.ErrNotExist) {
			release, err = func() error { return nil }, nil
		}
	}
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
	status := "preview"
	message := ""
	if request.Mode == "check" {
		status = "ready"
		reasons := []string{}
		if plan.InterruptedSource != "" {
			reasons = append(reasons, "Resume the unfinished operation using installer source "+plan.InterruptedSource+". Abandon is available only when provider publication is unchanged or complete.")
		}
		for _, op := range plan.Operations {
			if op.Action != "keep" && op.Action != "retain" && op.Action != "forget" {
				r, _ := c.Catalog.Resource(op.Resource)
				reasons = append(reasons, r.Name+" ("+r.ID+"): "+op.Reason)
			}
		}
		message = "All selected resources passed verification."
		if len(reasons) > 0 {
			status, message = "needs-action", strings.Join(reasons, "\n")
		}
		preserved, unverified := []string{}, []string{}
		for _, op := range plan.Operations {
			preserved = append(preserved, op.Observed.Preserved...)
			unverified = append(unverified, op.Observed.UnverifiedApplications...)
		}
		for _, path := range sortedUnique(preserved) {
			message += "\nPreserved files for inspection: " + path
		}
		for _, name := range sortedUnique(unverified) {
			message += "\nExternal application not exercised: " + name
		}
	}
	return Result{Status: status, Plan: plan, Message: message}, nil
}

var ErrCancelled = errors.New("installer interaction cancelled")

type Choice struct{ ID, Label, Detail string }

// Interaction is an input/output boundary only. Every method must restore the
// terminal before returning, particularly before elevation or provider prompts.
type Interaction interface {
	Choose(context.Context, string, []Choice, []string, bool) ([]string, error)
	Review(context.Context, *Catalog, Plan) (bool, error)
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
	actions := []Choice{
		{ID: "apply", Label: "Choose tools", Detail: "Install or remove tools; required dependencies are included automatically."},
		{ID: "remove", Label: "Remove tools", Detail: "Choose installed tools to remove; shared dependencies get a separate review."},
		{ID: "update", Label: "Update selected tools", Detail: "Preserve your selections and retained resources."},
		{ID: "repair", Label: "Repair selected tools", Detail: "Check installed tools and repair failures with verified ownership."},
		{ID: "check", Label: "Check installation", Detail: "Inspect health and required actions without changing anything."},
	}
	if state.Transaction != nil {
		actions = []Choice{
			{ID: "retry", Label: "Resume interrupted operation", Detail: "Keep the original selection and removal choices; review recovery before continuing."},
			{ID: "abandon", Label: "Abandon interrupted operation", Detail: "Archive intent and preserve resources and receipts. Partial activation must finish first."},
			{ID: "check", Label: "Check installation", Detail: "Inspect the recorded selection without changing anything."},
		}
		if _, supported := c.Driver.(ResourceRestoreDriver); supported && state.Transaction.InFlight != "" {
			if restoration, err := c.Preview(ctx, Request{Schema: 1, Mode: "restore"}); err == nil {
				if len(restoration.Operations) == 1 && restoration.Operations[0].Observed.Restoring {
					actions = slices.DeleteFunc(actions, func(choice Choice) bool { return choice.ID == "retry" || choice.ID == "abandon" })
				}
				actions = append(actions, Choice{ID: "restore", Label: "Restore interrupted files", Detail: "Use the saved journal to recover prior files, preserve conflicts and cancel the interrupted operation."})
			} else if !errors.Is(err, ErrNoResourceRestore) {
				if reportErr := ui.Report(Result{Status: "needs-action", Message: "Saved-file restoration unavailable: " + err.Error()}); reportErr != nil {
					return result, reportErr
				}
			}
		}
	}
	actions = append(actions, Choice{ID: "exit", Label: "Exit"})
	mode := "apply"
	if state.Schema != 0 {
		choice, err := ui.Choose(ctx, "What would you like to do?", actions, nil, false)
		if err != nil {
			return result, err
		}
		if err := validateChoice(choice, actions, false); err != nil {
			return result, err
		}
		mode = choice[0]
	}
	if mode == "exit" {
		return result, nil
	}
	request := Request{Schema: 1, Mode: mode}
	if mode == "retry" {
		request.Mode, request.Retry = state.Transaction.Plan.Mode, true
	}
	if mode == "apply" || mode == "remove" {
		options := []Choice{}
		for _, r := range c.Catalog.Resources {
			if r.Capability && r.Available(c.Context) && (mode != "remove" || slices.Contains(state.Selected, r.ID)) {
				options = append(options, Choice{ID: r.ID, Label: r.Name, Detail: r.Description})
			}
		}
		initial, title := state.Selected, "Choose the tools you want installed"
		if mode == "remove" {
			initial, title = nil, "Choose tools to remove"
		}
		selected := []string{}
		if len(options) > 0 {
			selected, err = ui.Choose(ctx, title, options, initial, true)
			if err != nil {
				return result, err
			}
			if err := validateChoice(selected, options, true); err != nil {
				return result, err
			}
		}
		request.Mode, request.Selected = "apply", selected
		if mode == "remove" {
			request.Selected = []string{}
			for _, id := range state.Selected {
				if !slices.Contains(selected, id) {
					request.Selected = append(request.Selected, id)
				}
			}
		}
		// Retained choices remain durable roots until explicitly unchecked.
		if len(state.Keep) != 0 {
			options = nil
			for _, id := range state.Keep {
				r, ok := c.Catalog.Resource(id)
				if !ok {
					return result, fmt.Errorf("retained resource %s requires catalog migration", id)
				}
				options = append(options, Choice{ID: id, Label: r.Name, Detail: "Keep this resource even without a selected tool."})
			}
			request.Keep, err = ui.Choose(ctx, "Keep previously retained resources", options, state.Keep, true)
			if err != nil {
				return result, err
			}
			if err := validateChoice(request.Keep, options, true); err != nil {
				return result, err
			}
			if request.Keep == nil {
				request.Keep = []string{}
			}
		}
	}
	preview, err := c.Dispatch(ctx, request)
	if err != nil {
		return result, err
	}
	if request.Mode == "check" {
		return preview, ui.Report(preview)
	}
	if (request.Mode == "apply" || request.Mode == "update" || request.Mode == "repair") && !request.Retry {
		adoptions := []Choice{}
		for _, op := range preview.Plan.Operations {
			r, _ := c.Catalog.Resource(op.Resource)
			if op.Action != "retain" && op.Action != "remove" && op.Action != "forget" && op.Observed.Present && op.Observed.Adoptable && op.Observed.Pending == "" &&
				!r.Retain && r.Scope != "host" && (request.Mode == "apply" || op.Ownership == "created") && (op.Ownership != "created" || !sameArtifact(state.Receipts[r.ID].After, op.Observed)) {
				detail := "Preserve the current configuration as a recoverable baseline, then apply the selected configuration."
				if op.Ownership == "created" {
					detail = "Save your edits separately, apply the selected configuration, and keep the original uninstall baseline."
				}
				adoptions = append(adoptions, Choice{ID: r.ID, Label: r.Name, Detail: detail})
			}
		}
		if len(adoptions) != 0 {
			request.Adopt, err = ui.Choose(ctx, "Replace these configurations? Your existing files will be preserved. Unchecked items stay unchanged.", adoptions, nil, true)
			if err != nil {
				return result, err
			}
			if err := validateChoice(request.Adopt, adoptions, true); err != nil {
				return result, err
			}
			preview, err = c.Dispatch(ctx, request)
			if err != nil {
				return result, err
			}
		}
	}
	if request.Mode == "apply" && !request.Retry {
		options, err := c.sharedRemovalChoices(ctx, state, request, preview.Plan)
		if err != nil {
			return result, err
		}
		if len(options) > 0 {
			request.RemoveShared, err = ui.Choose(ctx, "Also remove unused shared dependencies? Unchecked items are kept.", options, nil, true)
			if err != nil {
				return result, err
			}
			if err := validateChoice(request.RemoveShared, options, true); err != nil {
				return result, err
			}
			// Unchecked resources become explicit roots, not accidental leftovers.
			if request.Keep == nil {
				request.Keep = slices.Clone(state.Keep)
			}
			for _, option := range options {
				if !slices.Contains(request.RemoveShared, option.ID) {
					request.Keep = append(request.Keep, option.ID)
				}
			}
			preview, err = c.Dispatch(ctx, request)
			if err != nil {
				return result, err
			}
		}
	}
	accepted, err := ui.Review(ctx, c.Catalog, preview.Plan)
	if err != nil || !accepted {
		return result, err
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err = c.Dispatch(ctx, request)
	return result, errors.Join(err, ui.Report(result))
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

func (c Controller) sharedRemovalChoices(ctx context.Context, state State, request Request, plan Plan) ([]Choice, error) {
	choices := []Choice{}
	for _, op := range plan.Operations {
		r, _ := c.Catalog.Resource(op.Resource)
		if op.Action != "retain" || r.Retain || op.Observed.Pending != "" || len(op.Observed.Consumers) != 0 || op.Ownership != "created" ||
			!sameArtifact(state.Receipts[r.ID].After, op.Observed) || !(r.Shared || r.Scope == "machine" || op.Observed.Scope == "machine") {
			continue
		}
		choices = append(choices, Choice{ID: r.ID, Label: r.Name, Detail: "Created by this installer; another application may use it. " + op.Reason})
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
	removal, err := c.Preview(ctx, trial)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(choices, func(choice Choice) bool {
		return !slices.ContainsFunc(removal.Operations, func(op Operation) bool { return op.Resource == choice.ID && op.Action == "remove" })
	}), nil
}
