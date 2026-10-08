package installer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// LegacyMigration detaches released whole-file shell profiles before the normal
// controller adopts selected configurations. Its receipts describe a completed
// historical transition, not permanent ownership of the user's new profile.
// Publication, interrupted resume and restoration use the ordinary ConfigDriver.
type LegacyMigration struct {
	Controller Controller
	driver     *legacyMigrationDriver
}

type legacyProfile struct{ ID, Kind, Folder, Path string }

func NewLegacyMigration(repository, source, stateDirectory string, target Context, folders ConfigFolders) (*LegacyMigration, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	for _, path := range []string{repository, stateDirectory, folders.Home} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil, errors.New("migration requires canonical absolute source, state and home paths")
		}
	}
	if source == "" {
		return nil, errors.New("migration requires the authenticated source identity")
	}
	evidence, err := releasedProfileEvidence()
	if err != nil {
		return nil, err
	}
	profiles := []legacyProfile{{"legacy.zshrc", "zshrc", "home", ".zshrc"}, {"legacy.zshenv", "zshenv", "home", ".zshenv"}, {"legacy.bash-hook", "bash", "home", ".bashrc"}}
	if target.OS == "windows" {
		if !filepath.IsAbs(folders.Documents) || filepath.Clean(folders.Documents) != folders.Documents {
			return nil, errors.New("Windows migration requires the actual Documents known folder")
		}
		profiles = []legacyProfile{
			{"legacy.powershell", "powershell", "documents", "PowerShell/Microsoft.PowerShell_profile.ps1"},
			{"legacy.powershell-vscode", "powershell", "documents", "PowerShell/Microsoft.VSCode_profile.ps1"},
			{"legacy.windows-powershell", "powershell", "documents", "WindowsPowerShell/Microsoft.PowerShell_profile.ps1"},
			{"legacy.windows-powershell-ise", "powershell", "documents", "WindowsPowerShell/Microsoft.PowerShellISE_profile.ps1"},
		}
		conventional := filepath.Join(folders.Home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
		actual := filepath.Join(folders.Documents, "PowerShell", "Microsoft.PowerShell_profile.ps1")
		conventional, err = bindConfigDestination(conventional)
		if err != nil {
			return nil, err
		}
		actual, err = bindConfigDestination(actual)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(conventional, actual) {
			profiles = append(profiles, legacyProfile{"legacy.conventional-powershell", "powershell", "home", "Documents/PowerShell/Microsoft.PowerShell_profile.ps1"})
		}
	}
	c := &Catalog{Schema: 1}
	manifest := ConfigManifest{Schema: 1}
	for _, item := range profiles {
		c.Resources = append(c.Resources, Resource{ID: item.ID, Name: item.Path, Description: "Preserve the entire original and its readable bytes, then detach this legacy shell profile. Personal additions require review before reapplying them.", Capability: true, Platforms: []string{target.OS}, Action: "config", Scope: "user"})
		manifest.Targets = append(manifest.Targets, ConfigTarget{Resource: item.ID, Source: item.ID + ".profile", Folder: item.Folder, Path: item.Path, Platforms: []string{target.OS}, Mode: "copy"})
	}
	directory := filepath.Join(stateDirectory, "migration")
	d := &legacyMigrationDriver{ConfigDriver: &ConfigDriver{Catalog: c, Manifest: manifest, Target: target, Folders: folders, Repository: filepath.Join(directory, "payloads"), Directory: directory}, profiles: profiles, evidence: evidence}
	m := &LegacyMigration{driver: d, Controller: Controller{Catalog: c, Context: target, Source: source, Home: folders.Home, StatePath: filepath.Join(directory, "state.json"), Driver: d}}
	if err := m.Controller.validate(); err != nil {
		return nil, err
	}
	if err := manifest.Validate(c); err != nil {
		return nil, err
	}
	return m, nil
}

// Dispatch uses the existing machine protocol. Completed items cannot be
// deselected, repaired or updated: doing so would overwrite newer shell blocks.
func (m *LegacyMigration) Dispatch(ctx context.Context, request Request) (Result, error) {
	if err := m.driver.checkReleasedRecovery(); err != nil {
		return Result{Status: "needs-action"}, err
	}
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if !slices.Contains([]string{"apply", "check", "restore", "abandon"}, request.Mode) {
		return Result{Status: "failed"}, errors.New("migration supports apply, check and interrupted restore/abandon; normal update/removal uses setup")
	}
	if request.Mode == "apply" && !request.Retry {
		for id, receipt := range state.Receipts {
			if receipt.Ownership == "created" && receipt.Status == "ready" && (!slices.Contains(request.Selected, id) || slices.Contains(request.Adopt, id)) {
				return Result{Status: "failed"}, fmt.Errorf("%s was already migrated; retain its historical receipt and use setup for new configuration", id)
			}
		}
		for _, id := range request.Selected {
			r, ok := m.Controller.Catalog.Resource(id)
			if !ok {
				return Result{Status: "failed"}, fmt.Errorf("unknown migration item %s", id)
			}
			o, err := m.driver.Observe(ctx, r, state.Receipts[id])
			if err != nil {
				return Result{Status: "failed"}, err
			}
			if !o.Present {
				return Result{Status: "failed"}, fmt.Errorf("%s is absent; migration never creates an unused legacy target", id)
			}
		}
	}
	result, err := m.Controller.Dispatch(ctx, request)
	if err != nil || result.Status == "preview" {
		return result, err
	}
	return m.result(ctx, result)
}

func (m *LegacyMigration) Run(ctx context.Context, ui Interaction) (Result, error) {
	if ui == nil {
		return Result{Status: "failed"}, errors.New("migration requires an interaction surface")
	}
	if err := m.driver.checkReleasedRecovery(); err != nil {
		return Result{Status: "needs-action"}, err
	}
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if state.Transaction != nil {
		result, err := m.Controller.Run(ctx, ui)
		if err != nil || result.Status == "cancelled" {
			return result, err
		}
		return m.result(ctx, result)
	}
	choices, err := m.choices(ctx, state)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if len(choices) == 0 {
		result, err := m.result(ctx, Result{Status: "ready"})
		if err != nil {
			return result, err
		}
		return result, ui.Report(result)
	}
	selected, err := ui.Choose(ctx, "Detach these legacy shell profiles? Each selected original is preserved. Unchecked profiles stay unchanged.", choices, nil, true)
	if errors.Is(err, ErrCancelled) {
		return Result{Status: "cancelled"}, nil
	}
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if err := validateChoice(selected, choices, true); err != nil {
		return Result{Status: "failed"}, err
	}
	if len(selected) == 0 {
		return Result{Status: "cancelled", Message: "No legacy profiles changed."}, nil
	}
	wanted := slices.Clone(selected)
	for id, receipt := range state.Receipts {
		if receipt.Ownership == "created" && receipt.Status == "ready" {
			wanted = append(wanted, id)
		}
	}
	request := Request{Schema: 1, Mode: "apply", Selected: sortedUnique(wanted), Adopt: selected}
	preview, err := m.Dispatch(ctx, request)
	if err != nil {
		return preview, err
	}
	approved, err := ui.Review(ctx, m.Controller.Catalog, preview.Plan)
	if errors.Is(err, ErrCancelled) || err == nil && !approved {
		return Result{Status: "cancelled"}, nil
	}
	if err != nil {
		return Result{Status: "failed"}, err
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := m.Dispatch(ctx, request)
	if err != nil {
		return result, err
	}
	return result, ui.Report(result)
}

func (m *LegacyMigration) choices(ctx context.Context, state State) ([]Choice, error) {
	choices := []Choice{}
	for _, r := range m.Controller.Catalog.Resources {
		receipt := state.Receipts[r.ID]
		o, err := m.driver.Observe(ctx, r, receipt)
		if err != nil {
			return nil, err
		}
		if receipt.Ownership == "created" && receipt.Status == "ready" && o.Healthy {
			continue
		}
		if !o.Present {
			continue
		}
		item := m.driver.profile(r.ID)
		content, _, err := readLegacyProfile(m.driver.destination(item))
		if err != nil {
			return nil, err
		}
		detail := "Unknown or edited profile: the entire original is saved for review; personal additions are not silently carried into the new shell."
		if m.driver.evidence.matches(item.Kind, content) {
			detail = "Exact released profile bytes recognized. The original and a readable copy are preserved before replacement."
		}
		if item.Kind == "bash" {
			detail = "Remove only the exact released bash-to-zsh block; keep every surrounding byte and the original file."
		}
		if o.ApplyBlocked != "" {
			detail = o.ApplyBlocked
		}
		choices = append(choices, Choice{ID: r.ID, Label: m.driver.destination(item), Detail: detail})
	}
	return choices, nil
}

func (m *LegacyMigration) result(ctx context.Context, result Result) (Result, error) {
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return result, err
	}
	choices, err := m.choices(ctx, state)
	if err != nil {
		return result, err
	}
	if state.Transaction != nil || len(choices) != 0 {
		result.Status = "needs-action"
		result.Message = "Legacy shell preparation is unfinished. Retained profiles remain unchanged; use migrate to review them before setup."
		for _, choice := range choices {
			result.Message += "\nReview " + choice.ID + ": " + choice.Label + ". " + choice.Detail
		}
	} else if result.Status != "cancelled" {
		result.Status = "ready"
		result.Message = "Legacy shell profiles are detached. Continue in setup to choose tools and explicitly adopt their configurations."
	}
	result.Message += " Existing Nix, Home Manager, nix-darwin, chezmoi state, packages, login-shell settings and unrelated configurations remain retained; migration does not claim or uninstall shared infrastructure."
	for _, receipt := range state.Receipts {
		for _, path := range receipt.After.Preserved {
			result.Message += "\nPreserved for review: " + path
		}
	}
	return result, nil
}
