package installer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// The manager supplies the runtime graph. Follow consumers of packages actually
// touched by this operation; do not solve versions or broaden to unrelated tools.
func (d *BrewDriver) maintenanceCandidates(intent brewIntent, before int) (map[string]string, error) {
	root, err := brewShortName(intent.Package)
	if err != nil {
		return nil, err
	}
	affected := map[string]string{root: intent.Package}
	for _, command := range intent.Commands[:before] {
		reply, err := d.commandResult(command)
		if err != nil {
			return nil, err
		}
		kegs, err := brewInstallEvidence(reply.Output, filepath.ToSlash(d.Cellar), command.Operation)
		if err != nil {
			return nil, err
		}
		for _, keg := range kegs {
			if !brewPackageName.MatchString(keg.Name) {
				return nil, errors.New("invalid Homebrew maintenance identity")
			}
			if keg.Name != root {
				affected[keg.Name] = keg.Name
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for name, pkg := range intent.Before {
			if _, exists := affected[name]; exists {
				affected[name] = pkg.Source
				continue
			}
			for _, dependency := range pkg.Dependencies {
				if _, affectedDependency := affected[dependency]; affectedDependency {
					affected[name], changed = pkg.Source, true
					break
				}
			}
		}
	}
	return affected, nil
}

func (d *BrewDriver) runCommand(ctx context.Context, intent *brewIntent, args []string) error {
	if len(intent.Commands) >= 32 {
		return errors.New("Homebrew operation reached its command bound; preserve its records for inspection")
	}
	if len(args) == 3 && args[0] == "reinstall" && args[2] != intent.Package {
		installed, err := brewInventory(ctx, d.Query)
		if err != nil {
			return err
		}
		name, err := brewShortName(args[2])
		pkg, present := installed[name]
		if err != nil || !present || pkg.Held {
			return errors.Join(fmt.Errorf("Homebrew maintenance package %s is absent or pinned", args[2]), err)
		}
		if old, existed := intent.Before[name]; existed && brewClassificationOf(old) != brewClassificationOf(pkg) {
			return fmt.Errorf("pre-existing Homebrew maintenance package %s changed classification", args[2])
		}
	}
	operation, err := digest(struct {
		Operation string
		Attempt   int
	}{intent.Operation, len(intent.Commands)})
	if err != nil {
		return err
	}
	command := nativeCommand{Operation: operation, Program: d.Program, Arguments: args, Environment: append(slices.Clone(d.Environment), "HOMEBREW_INSTALL_BADGE=dotfiles-operation-"+operation)}
	if err := command.validate(); err != nil {
		return err
	}
	intent.Commands = append(intent.Commands, command)
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return err
	}
	return d.runSavedCommand(ctx, *intent, command)
}

func (d *BrewDriver) runSavedCommand(ctx context.Context, intent brewIntent, command nativeCommand) error {
	if command.Arguments[0] == "uninstall" {
		// All-version removal retains Homebrew's dependency checks, but bypasses
		// its pin check. Revalidate classifications and consumers immediately
		// before every dispatch, including recovery of an unrecorded command.
		installed, err := brewInventory(ctx, d.Query)
		if err != nil {
			return err
		}
		names := command.Arguments[3:]
		for _, name := range names {
			if pkg, present := installed[name]; present {
				old, existed := intent.Before[name]
				if !existed || pkg.Held || brewClassificationOf(old) != brewClassificationOf(pkg) {
					return fmt.Errorf("Homebrew removal package %s changed source, pin or manual classification", name)
				}
			}
		}
		if _, _, err := nativeRemovalCandidates(names, nil, d.Keep, installed); err != nil {
			return err
		}
	}
	_, err := d.Run(ctx, command)
	return err
}

// Homebrew's automatic check can miss a consumer of a dependency upgraded under
// a leaf root. Check native linkage explicitly and rebuild only failing affected
// formulae. Recheck after each repair because it can maintain other dependencies.
func (d *BrewDriver) maintainDependents(ctx context.Context, intent *brewIntent) error {
	attempted := map[string]bool{}
	for {
		candidates, err := d.maintenanceCandidates(*intent, len(intent.Commands))
		if err != nil {
			return err
		}
		installed, err := brewInventory(ctx, d.Query)
		if err != nil {
			return err
		}
		if err := d.verifyPreexisting(*intent, installed); err != nil {
			return err
		}
		names := make([]string, 0, len(candidates))
		for name := range candidates {
			names = append(names, name)
		}
		slices.Sort(names)
		arguments := []string{"linkage", "--test"}
		formulae, casks := []string{}, []string{}
		for _, name := range names {
			pkg, present := installed[name]
			if !present {
				return fmt.Errorf("Homebrew affected package %s is missing", name)
			}
			if strings.HasPrefix(name, "cask/") {
				baseline := intent.Before[name]
				if baseline.Version != pkg.Version || brewClassificationOf(baseline) != brewClassificationOf(pkg) {
					return fmt.Errorf("pre-existing Homebrew application %s changed during formula maintenance", name)
				}
				// Homebrew declares presence dependencies for casks, not an ABI
				// or application execution contract. Verify those native edges;
				// disclose the external application separately from root health.
				for _, dependency := range sortedUnique(append(slices.Clone(baseline.Dependencies), pkg.Dependencies...)) {
					if current, exists := installed[dependency]; !exists || !current.Healthy {
						return fmt.Errorf("dependency %s of Homebrew application %s is missing or inactive; restore it and retry", dependency, name)
					}
				}
				casks = append(casks, name)
				continue
			}
			formulae = append(formulae, name)
			arguments = append(arguments, pkg.Source)
		}
		if len(formulae) == 0 {
			return errors.New("Homebrew dependent verification requires an explicit formula set")
		}
		batchOutput, batchErr := d.Query(ctx, false, "brew", nil, arguments...)
		if batchErr == nil {
			intent.UncheckedCasks = casks
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(batchErr, context.DeadlineExceeded) || errors.Is(batchErr, context.Canceled) {
			return fmt.Errorf("Homebrew affected-package linkage inspection interrupted: %w%s", batchErr, nativeDiagnostic(batchOutput))
		}
		repaired := false
		for _, name := range formulae {
			pkg := installed[name]
			output, linkErr := d.Query(ctx, false, "brew", nil, "linkage", "--test", pkg.Source)
			if linkErr == nil {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(linkErr, context.DeadlineExceeded) || errors.Is(linkErr, context.Canceled) {
				return fmt.Errorf("Homebrew affected package %s linkage inspection interrupted: %w%s", name, linkErr, nativeDiagnostic(output))
			}
			if pkg.Held || attempted[name] {
				return fmt.Errorf("Homebrew affected package %s remains broken after repair or is pinned: %w%s", name, linkErr, nativeDiagnostic(output))
			}
			attempted[name] = true
			if err := d.runCommand(ctx, intent, []string{"reinstall", "--formula", candidates[name]}); err != nil {
				return err
			}
			repaired = true
			break
		}
		if !repaired {
			return fmt.Errorf("Homebrew affected-package linkage check failed: %w%s", batchErr, nativeDiagnostic(batchOutput))
		}
	}
}
