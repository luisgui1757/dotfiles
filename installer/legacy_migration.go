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

type legacyProfile struct{ ID, Kind, Folder, Path, Source, Problem string }

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
	profiles := []legacyProfile{{ID: "legacy.zshrc", Kind: "zshrc", Folder: "home", Path: ".zshrc"}, {ID: "legacy.zshenv", Kind: "zshenv", Folder: "home", Path: ".zshenv"}, {ID: "legacy.bash-hook", Kind: "bash", Folder: "home", Path: ".bashrc"}}
	if target.OS == "windows" {
		if !filepath.IsAbs(folders.Documents) || filepath.Clean(folders.Documents) != folders.Documents {
			return nil, errors.New("Windows migration requires the actual Documents known folder")
		}
		profiles = []legacyProfile{
			{ID: "legacy.powershell", Kind: "powershell", Folder: "documents", Path: "PowerShell/Microsoft.PowerShell_profile.ps1"},
			{ID: "legacy.powershell-vscode", Kind: "powershell", Folder: "documents", Path: "PowerShell/Microsoft.VSCode_profile.ps1"},
			{ID: "legacy.windows-powershell", Kind: "powershell", Folder: "documents", Path: "WindowsPowerShell/Microsoft.PowerShell_profile.ps1"},
			{ID: "legacy.windows-powershell-ise", Kind: "powershell", Folder: "documents", Path: "WindowsPowerShell/Microsoft.PowerShellISE_profile.ps1"},
		}
		conventional, conventionalErr := bindConfigDestination(filepath.Join(folders.Home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"))
		actual, actualErr := bindConfigDestination(filepath.Join(folders.Documents, "PowerShell", "Microsoft.PowerShell_profile.ps1"))
		if err := errors.Join(conventionalErr, actualErr); err != nil {
			// Without both physical locations, the conventional path may alias
			// the known folder. Report it as blocked; never give it an owner.
			profiles = append(profiles, legacyProfile{ID: "legacy.conventional-powershell", Kind: "powershell", Folder: "home", Path: "Documents/PowerShell/Microsoft.PowerShell_profile.ps1", Problem: "conventional and known Documents PowerShell profile locations must be available before reviewing this item: " + err.Error()})
		} else if !strings.EqualFold(conventional, actual) {
			profiles = append(profiles, legacyProfile{ID: "legacy.conventional-powershell", Kind: "powershell", Folder: "home", Path: "Documents/PowerShell/Microsoft.PowerShell_profile.ps1"})
		}
	}
	gitPath := gitConfigPath(folders)
	gitProblem := ""
	if !filepath.IsAbs(gitPath) || filepath.Clean(gitPath) != gitPath || filepath.Base(gitPath) != "config" || filepath.Base(filepath.Dir(gitPath)) != "git" {
		gitProblem = "migration requires the actual canonical Git global configuration path; correct Git's HOME/XDG settings before reviewing this item"
		// This placeholder identifies a blocked item; it grants no authority
		// to inspect or mutate a different Git configuration.
		gitPath = filepath.Join(folders.Home, ".config", "git", "config")
	}
	profiles = append(profiles, legacyProfile{ID: "legacy.git-config", Kind: "git", Folder: "config", Path: "git/config", Source: "home/dot_config/git/config", Problem: gitProblem})
	// Only the Git migration item uses this root. Windows' actual Git location
	// may differ from the native application configuration folder.
	folders.Config = filepath.Dir(filepath.Dir(gitPath))
	profiles, err = releasedConfigurationLinks(target, folders, profiles)
	if err != nil {
		return nil, err
	}
	c := &Catalog{Schema: 1}
	manifest := ConfigManifest{Schema: 1}
	for _, item := range profiles {
		c.Resources = append(c.Resources, Resource{ID: item.ID, Name: item.Path, Description: "Preserve the entire original and its readable bytes, then detach this legacy shell profile. Personal additions require review before reapplying them.", Capability: true, Platforms: []string{target.OS}, Action: "config", Scope: "user"})
		if item.Kind == "git" {
			c.Resources[len(c.Resources)-1].Description = "Preserve the Git symlink and copy all its readable settings into a regular personal file, so global Git writes cannot change the checkout."
		}
		if item.Kind == "config" {
			c.Resources[len(c.Resources)-1].Description = "Detach a released configuration link while keeping every readable byte in a regular personal file and preserving the original link."
		}
		manifest.Targets = append(manifest.Targets, ConfigTarget{Resource: item.ID, Source: item.ID + ".profile", Folder: item.Folder, Path: item.Path, Platforms: []string{target.OS}, Mode: "copy"})
	}
	directory := filepath.Join(stateDirectory, "migration")
	d := &legacyMigrationDriver{ConfigDriver: &ConfigDriver{Catalog: c, Manifest: manifest, Target: target, Folders: folders, Repository: filepath.Join(directory, "payloads"), Directory: directory}, profiles: profiles, evidence: evidence, checkout: repository}
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
			if !o.Present && !o.Unknown {
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
	choices, _, err := m.choices(ctx, state)
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
	selected, err := ui.Choose(ctx, "Migrate these existing profiles and configuration links? Each selected original is preserved. Unchecked files stay unchanged.", choices, nil, true)
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
	approved, err := ui.Review(ctx, m.Controller.Catalog, Proposal{Plan: preview.Plan}, nil)
	if errors.Is(err, ErrCancelled) || err == nil && !approved {
		return Result{Status: "cancelled"}, nil
	}
	if err != nil {
		return Result{Status: "failed"}, err
	}
	request.ExpectedPlan = preview.Plan.ID
	applying, err := ui.BeginApply(ctx)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	result, err := m.Dispatch(applying, request)
	if err != nil {
		return result, err
	}
	return result, ui.Report(result)
}

// choices returns reviewable items and, separately, items that cannot be
// inspected. Blocked items are reported but never offered for selection.
func (m *LegacyMigration) choices(ctx context.Context, state State) ([]Choice, []Choice, error) {
	choices, blocked := []Choice{}, []Choice{}
	for _, r := range m.Controller.Catalog.Resources {
		receipt := state.Receipts[r.ID]
		o, err := m.driver.Observe(ctx, r, receipt)
		if err != nil {
			return nil, nil, err
		}
		if receipt.Ownership == "created" && receipt.Status == "ready" && o.Healthy {
			continue
		}
		if o.Unknown {
			blocked = append(blocked, Choice{ID: r.ID, Label: r.Name, Detail: o.Pending})
			continue
		}
		if !o.Present {
			continue
		}
		item := m.driver.profile(r.ID)
		content, _, err := readLegacyProfile(m.driver.destination(item))
		if err != nil {
			blocked = append(blocked, Choice{ID: r.ID, Label: m.driver.destination(item), Detail: err.Error()})
			continue
		}
		detail := "Unknown or edited profile: the entire original is saved for review; personal additions are not silently carried into the new shell."
		if m.driver.evidence.matches(item.Kind, content) {
			detail = "Exact released profile bytes recognized. The original and a readable copy are preserved before replacement."
		}
		if item.Kind == "bash" {
			detail = "Remove only the exact released bash-to-zsh block; keep every surrounding byte and the original file."
		}
		if item.Kind == "git" {
			detail = "Copy the complete Git configuration into a regular writable file and preserve the original link. Existing identity and personal settings remain active; the linked checkout is unchanged."
		}
		if item.Kind == "config" {
			detail = "Released configuration link detected. Keep all readable settings in a regular personal file and preserve the original link; the old checkout stays unchanged."
		}
		if o.ApplyBlocked != "" {
			detail = o.ApplyBlocked
		}
		choices = append(choices, Choice{ID: r.ID, Label: m.driver.destination(item), Detail: detail})
	}
	return choices, blocked, nil
}

func (m *LegacyMigration) result(ctx context.Context, result Result) (Result, error) {
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return result, err
	}
	choices, blocked, err := m.choices(ctx, state)
	if err != nil {
		return result, err
	}
	if state.Transaction != nil || len(choices) != 0 || len(blocked) != 0 {
		result.Status = "needs-action"
		result.Message = "Legacy shell preparation is unfinished. Retained profiles remain unchanged; use migrate to review them before setup."
		for _, choice := range choices {
			result.Message += "\nReview " + choice.ID + ": " + choice.Label + ". " + choice.Detail
		}
		for _, choice := range blocked {
			result.Message += "\nCannot inspect " + choice.ID + ": " + choice.Label + ". " + choice.Detail + " Resolve this location, then run migrate again."
		}
	} else if result.Status != "cancelled" {
		result.Status = "ready"
		result.Message = "Legacy shell profiles are detached. Continue in setup to choose tools and explicitly adopt their configurations."
	}
	result.Message += " This preparation phase preserves existing runtimes. After replacement setup passes its check, migrate separately previews the verified earlier runtimes and tools for retirement; unrelated configurations stay unchanged."
	for _, receipt := range state.Receipts {
		for _, path := range receipt.After.Preserved {
			result.Message += "\nPreserved for review: " + path
		}
	}
	return result, nil
}
