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

// Retirement keeps whatever it cannot prove safe to remove, and leaves unselected
// retirements alone. By owner decision (2026-10-21) the owner may explicitly keep
// such leftovers so migration can finish. The choice binds the exact inspected
// scope: any later change, a new step or a different leftover, makes the resource
// unfinished again. It records no ownership, grants no removal authority and
// never hides a retirement that later becomes possible to select.
const legacyKeptSchema = 1

type legacyKeptLeftovers struct {
	Schema int `json:"schema"`
	// Resource ID to the digest of the scope the owner reviewed.
	Resources map[string]string `json:"resources"`
}

const legacyKeptHeader = "Kept by your choice; dotfiles will not remove it. Remove what is listed yourself if you want, or select it in a later migrate run:"

func (d *legacyRetirementDriver) keptPath() string { return filepath.Join(d.Directory, "kept.json") }

func (d *legacyRetirementDriver) readKept() (legacyKeptLeftovers, error) {
	kept := legacyKeptLeftovers{Schema: legacyKeptSchema, Resources: map[string]string{}}
	data, err := readDocument(d.keptPath())
	if errors.Is(err, os.ErrNotExist) {
		return kept, nil
	}
	if err == nil {
		err = Decode(data, &kept)
	}
	if err == nil && (kept.Schema != legacyKeptSchema || kept.Resources == nil) {
		err = errors.New("unsupported record")
	}
	if err != nil {
		return kept, fmt.Errorf("kept-leftovers record %s is unreadable; preserve or remove it: %w", d.keptPath(), err)
	}
	return kept, nil
}

// legacyKeepable reports whether the owner can keep this scope as it is. Blocked
// work needs action; an empty scope has nothing to keep.
func legacyKeepable(scope legacyRetirementScope) bool {
	return scope.Blocked == "" && (len(scope.Steps) != 0 || len(scope.Retained) != 0)
}

// keptScope reports whether the owner kept exactly this scope.
func (d *legacyRetirementDriver) keptScope(id string, scope legacyRetirementScope) (bool, error) {
	if !legacyKeepable(scope) {
		return false, nil
	}
	kept, err := d.readKept()
	if err != nil {
		return false, err
	}
	identity, err := digest(scope)
	return err == nil && kept.Resources[id] == identity, err
}

type legacyLeftover struct {
	Choice
	Scope string
}

// keepLeftovers asks once, after a runtime retirement finished every approved
// step, whether to keep the remaining resources. It is offered only when every
// unfinished resource is keepable: interrupted, unreadable, uninspectable or
// blocked work still needs action, so keeping the rest could not finish.
func (m *LegacyRetirement) keepLeftovers(ctx context.Context, ui Interaction) (string, error) {
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return "", err
	}
	offered := []legacyLeftover{}
	for _, r := range legacyPhaseResources(m.Controller.Catalog, m.Controller.Context, LegacyRetirementRuntime) {
		receipt := state.Receipts[r.ID]
		if intent, err := m.driver.readIntent(r.ID, receipt); err == nil && !intent.Complete && receipt.Status == "in-progress" || err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		scope, err := m.driver.inspect(ctx, r.ID)
		if err != nil || scope.Blocked != "" {
			return "", nil
		}
		if !legacyKeepable(scope) {
			continue
		}
		if kept, err := m.driver.keptScope(r.ID, scope); err != nil || kept {
			if err != nil {
				return "", err
			}
			continue
		}
		identity, err := digest(scope)
		if err != nil {
			return "", err
		}
		offered = append(offered, legacyLeftover{Choice{ID: r.ID, Label: r.Name, Detail: strings.Join(legacyScopeLines(scope), "\n")}, identity})
	}
	if len(offered) == 0 {
		return "", nil
	}
	choices, initial := []Choice{}, []string{}
	for _, item := range offered {
		choices, initial = append(choices, item.Choice), append(initial, item.ID)
	}
	selected, err := ui.Choose(ctx, "Keep these earlier leftovers and finish migration? Migrate could not prove them safe to remove, or you left them unselected. "+
		"Checked items stay exactly as they are and the final report lists them so you can remove them yourself. Unchecked items keep migration unfinished.", choices, initial, true)
	if errors.Is(err, ErrCancelled) || errors.Is(err, ErrBack) {
		return "", nil
	}
	if err == nil {
		err = validateChoice(selected, choices, true)
	}
	if err != nil || len(selected) == 0 {
		return "", err
	}
	// Bind the decision to what was shown: anything that changed since stays
	// unfinished and appears in the next run.
	kept, err := m.driver.readKept()
	if err != nil {
		return "", err
	}
	changed := []string{}
	for _, item := range offered {
		if !slices.Contains(selected, item.ID) {
			continue
		}
		scope, err := m.driver.inspect(ctx, item.ID)
		identity := ""
		if err == nil {
			identity, err = digest(scope)
		}
		if err != nil || identity != item.Scope {
			changed = append(changed, item.ID)
			continue
		}
		kept.Resources[item.ID] = identity
	}
	if err := saveDocument(m.driver.keptPath(), kept); err != nil {
		return "", err
	}
	if len(changed) != 0 {
		return "Not kept because they changed after you reviewed them: " + strings.Join(changed, ", ") + ". Run migrate again to review them.", nil
	}
	return "", nil
}

// finish reports the phase result. A runtime retirement that otherwise
// finished first offers to keep its remaining leftovers.
func (m *LegacyRetirement) finish(ctx context.Context, ui Interaction, result Result, phase string) (Result, error) {
	reported, err := m.result(ctx, result, phase)
	if err == nil && phase == LegacyRetirementRuntime && result.Status == "ready" && reported.Status == "needs-action" {
		var notice string
		if notice, err = m.keepLeftovers(ctx, ui); err == nil {
			if reported, err = m.result(ctx, result, phase); notice != "" {
				reported.Message = strings.TrimSpace(reported.Message + "\n" + notice)
			}
		}
	}
	return reported, errors.Join(err, ui.Report(reported))
}
