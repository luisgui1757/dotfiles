package installer

import (
	"context"
	"errors"
	"fmt"
)

// NativeDriver routes one catalog resource to one provider. It has no fallback
// cascade: a missing route is an error before approval or any mutation.
type NativeDriver struct {
	Catalog            *Catalog
	Context            Context
	Archives           *ArchiveDriver
	Configurations     *ConfigDriver
	Desktop            *DesktopDriver
	Fonts              *FontDriver
	Profiles           *ProfileDriver
	Brew               *BrewDriver
	Homebrew           *HomebrewBootstrapDriver
	HomebrewIssue      string
	APT                *LinuxAPTDriver
	APTIssue           string
	WindowsVendor      *WindowsVendorDriver
	WindowsVendorIssue string
	NvimSync           *NvimSyncDriver
	BuildTools         *WindowsBuildToolsDriver
	ApplePrerequisites *MacOSPrerequisiteDriver
	AppleCLT           *AppleCLTDriver
	LinuxClipboard     LinuxClipboardCapability
	StatePath          string
	shellSelection     func([]string, Receipt) (*ProfileDriver, error)
	session            *nativeSession
}

func (d *NativeDriver) provider(r Resource) (Driver, error) {
	if r.ID == "tool.font" && d.Fonts != nil {
		return d.Fonts, nil
	}
	if d.Desktop != nil {
		switch r.ID {
		case "desktop.vscode", "desktop.aerospace", "integration.ghostty", "desktop.rose-pine", "desktop.windows-terminal":
			return d.Desktop, nil
		}
	}
	if r.ID == "infra.apple-clt" && d.AppleCLT != nil {
		return d.AppleCLT, nil
	}
	if r.ID == "nvim.sync" && d.NvimSync != nil {
		return d.NvimSync, nil
	}
	if r.ID == "tool.compiler" && d.BuildTools != nil {
		return d.BuildTools, nil
	}
	if d.ApplePrerequisites != nil && (r.ID == "tool.compiler" || r.ID == "tool.make" || r.ID == "tool.clipboard") {
		return d.ApplePrerequisites, nil
	}
	if r.ID == "infra.homebrew" && d.Homebrew != nil {
		return d.Homebrew, nil
	}
	if d.Brew != nil {
		if _, bound := d.Brew.Packages[r.ID]; bound {
			return d.Brew, nil
		}
	}
	if d.APT != nil && r.Bindings[d.Context.OS].Provider == "apt" {
		return d.APT, nil
	}
	if d.WindowsVendor != nil && d.Context.OS == "windows" && r.ID == "tool.vcredist" {
		return d.WindowsVendor, nil
	}
	if d.Context.OS == "darwin" && r.ID == "tool.zsh" {
		return macOSZshDriver{}, nil
	}
	if r.Bindings[d.Context.OS].Provider == "archive" {
		return d.Archives, nil
	}
	if r.Action == "config" {
		return d.Configurations, nil
	}
	if d.Profiles != nil {
		if _, ok := d.Profiles.Targets[r.ID]; ok {
			return d.Profiles, nil
		}
	}
	return nil, fmt.Errorf("%s has no connected native provider in this development revision", r.ID)
}
func (d *NativeDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if d.Brew == nil && d.HomebrewIssue == "" && r.Bindings[d.Context.OS].Provider == "homebrew-formula" {
		return Observation{Unknown: true, Provider: "homebrew-formula", Scope: "machine", Pending: "install Homebrew, then resume to review its package inventory", Preserved: receipt.After.Preserved}, ctx.Err()
	}
	if d.WindowsVendorIssue != "" && d.Context.OS == "windows" && (r.ID == "tool.vcredist" || r.ID == "tool.compiler") {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		return Observation{Unknown: true, Provider: "windows-vendor", Scope: "machine", Pending: d.WindowsVendorIssue, Preserved: receipt.After.Preserved}, nil
	}
	if d.APTIssue != "" && r.Bindings[d.Context.OS].Provider == "apt" {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		return Observation{Unknown: true, Provider: "apt", Scope: "machine", Pending: d.APTIssue, Preserved: receipt.After.Preserved}, nil
	}
	if d.HomebrewIssue != "" && (r.ID == "infra.homebrew" || r.Bindings[d.Context.OS].Provider == "homebrew-formula") {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		return Observation{Unknown: true, Provider: "homebrew-formula", Scope: "machine", Pending: d.HomebrewIssue, Preserved: receipt.After.Preserved}, nil
	}
	p, err := d.provider(r)
	if err != nil {
		return Observation{}, err
	}
	observed, err := p.Observe(ctx, r, receipt)
	if err == nil && d.Context.OS == "linux" && r.ID == "tool.clipboard" && observed.Healthy {
		observed.UnverifiedApplications = sortedUnique(append(observed.UnverifiedApplications, d.LinuxClipboard.Disclosures()...))
	}
	return observed, err
}
func (d *NativeDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	p, err := d.provider(r)
	if err != nil {
		return Observation{}, err
	}
	return p.Apply(ctx, r, op, receipt)
}
func (d *NativeDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	p, err := d.provider(r)
	if err != nil {
		return Observation{}, err
	}
	return p.Remove(ctx, r, receipt)
}
func (d *NativeDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	p, err := d.provider(r)
	if err != nil {
		return Observation{}, err
	}
	resume, ok := p.(ResourceResumeDriver)
	if !ok {
		return Observation{}, errors.New("provider has no saved-operation recovery")
	}
	if brew, ok := p.(*BrewDriver); ok {
		if err := brew.approveInventory(ctx, []string{op.Observed.Inventory}); err != nil {
			return Observation{}, err
		}
	}
	if apt, ok := p.(*LinuxAPTDriver); ok {
		if err := apt.approveInventory(ctx, []string{op.Observed.Inventory}); err != nil {
			return Observation{}, err
		}
	}
	return resume.ResumeResource(ctx, r, op, receipt)
}
func (d *NativeDriver) ObserveRestore(ctx context.Context, id string, receipt Receipt) (Observation, error) {
	restore, err := d.restoreProvider(id, receipt)
	if err != nil {
		return Observation{}, err
	}
	return restore.ObserveRestore(ctx, id, receipt)
}
func (d *NativeDriver) RestoreResource(ctx context.Context, id string, observed Observation, receipt Receipt) error {
	restore, err := d.restoreProvider(id, receipt)
	if err != nil {
		return err
	}
	return restore.RestoreResource(ctx, id, observed, receipt)
}

// Recovery routes from saved provider identity, never today's catalog. The
// selected adapter independently validates the journal against this receipt.
func (d *NativeDriver) restoreProvider(id string, receipt Receipt) (ResourceRestoreDriver, error) {
	provider := receipt.Before.Provider
	if provider == "" {
		provider = receipt.After.Provider
	}
	if receipt.After.Provider != "" && provider != receipt.After.Provider {
		return nil, errors.New("saved recovery provider identities disagree")
	}
	switch provider {
	case "configuration":
		if id == "tool.font" {
			if d.Fonts != nil {
				return d.Fonts, nil
			}
			return nil, ErrNoResourceRestore
		}
		if d.Configurations != nil {
			return d.Configurations, nil
		}
	case "profile-block":
		if d.Profiles != nil {
			return d.Profiles, nil
		}
	}
	return nil, ErrNoResourceRestore
}
func (d *NativeDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	report := map[string][]string{}
	providers := []TransactionDriver{}
	if d.Profiles != nil {
		providers = append(providers, d.Profiles)
	}
	if d.Configurations != nil {
		providers = append(providers, d.Configurations)
	}
	if d.Archives != nil {
		providers = append(providers, d.Archives)
	}
	if d.NvimSync != nil {
		providers = append(providers, d.NvimSync)
	}
	for _, provider := range providers {
		preserved, err := provider.FinishTransaction(ctx, plan, receipts)
		if err != nil {
			return nil, err
		}
		for id, paths := range preserved {
			report[id] = sortedUnique(append(report[id], paths...))
		}
	}
	return report, nil
}

// ForSelection returns a per-request adapter; previews never mutate shared
// driver state, including concurrent first-run previews before a lock exists.
func (d *NativeDriver) ForSelection(wanted []string, state State) (Driver, error) {
	clone := *d
	if d.shellSelection != nil {
		profiles, err := d.shellSelection(wanted, state.Receipts["integration.shells"])
		if err != nil {
			return nil, err
		}
		clone.Profiles = profiles
		clone.shellSelection = nil
	}
	if d.Brew != nil {
		brew := *d.Brew
		brew.Keep = nil
		for _, id := range wanted {
			if full, bound := brew.Packages[id]; bound {
				name, err := brewShortName(full)
				if err != nil {
					return nil, err
				}
				brew.Keep = append(brew.Keep, name)
			}
		}
		clone.Brew = &brew
	}
	if d.APT != nil {
		apt := *d.APT
		apt.Keep = nil
		for _, id := range wanted {
			if name, bound := apt.Packages[id]; bound {
				apt.Keep = append(apt.Keep, name)
			}
		}
		clone.APT = &apt
	}
	return &clone, nil
}
