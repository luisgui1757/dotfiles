package installer

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
)

// Bind only owned roots/pool members and their installed consumers. Changes to
// unrelated packages must not invalidate approval or become removal authority.
type brewSnapshot struct {
	Ledger    brewLedger
	Packages  map[string]nativePackage
	Consumers map[string][]string
	Keep      []string
}

// Only the mutation controller reads/writes this session value. Preview derives
// independent snapshots and never changes the approval shared by selection clones.
type brewApproval struct{ expected string }

func brewProviderSnapshot(state brewLedger, installed map[string]nativePackage, keep []string) brewSnapshot {
	state.Roots, state.Pool = maps.Clone(state.Roots), maps.Clone(state.Pool)
	view := brewSnapshot{Ledger: state, Packages: map[string]nativePackage{}, Consumers: map[string][]string{}, Keep: sortedUnique(keep)}
	owned := map[string]bool{}
	for _, p := range state.Roots {
		owned[p.Name] = true
	}
	for name := range state.Pool {
		owned[name] = true
	}
	for name, p := range installed {
		if owned[name] {
			view.Packages[name] = p
		}
		for _, dependency := range p.Dependencies {
			if owned[dependency] {
				view.Consumers[name] = append(view.Consumers[name], dependency)
			}
		}
		if len(view.Consumers[name]) > 0 {
			view.Consumers[name] = sortedUnique(view.Consumers[name])
		}
	}
	return view
}

func (d *BrewDriver) snapshot(ctx context.Context) (brewSnapshot, error) {
	state, err := d.ledger()
	if err != nil {
		return brewSnapshot{}, err
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return brewSnapshot{}, err
	}
	return brewProviderSnapshot(state, installed, d.Keep), nil
}

func (d *BrewDriver) approveInventory(ctx context.Context, approved []string) error {
	if d.approval == nil {
		return errors.New("Homebrew requires its controller approval session")
	}
	view, err := d.snapshot(ctx)
	if err != nil {
		return err
	}
	hash, err := digest(view)
	if err != nil {
		return err
	}
	if d.approval.expected == "" {
		if !slices.Contains(approved, hash) {
			return errors.New("Homebrew ownership or consumers changed after approval")
		}
		d.approval.expected = hash
	} else if d.approval.expected != hash {
		return errors.New("Homebrew ownership or consumers changed during the operation")
	}
	return nil
}

func (view brewSnapshot) excluding(changed map[string]bool) brewSnapshot {
	view.Ledger.Roots = maps.Clone(view.Ledger.Roots)
	view.Ledger.Pool = maps.Clone(view.Ledger.Pool)
	view.Packages = maps.Clone(view.Packages)
	view.Consumers = maps.Clone(view.Consumers)
	for id, owned := range view.Ledger.Roots {
		if changed[owned.Name] {
			delete(view.Ledger.Roots, id)
		}
	}
	for name := range changed {
		delete(view.Ledger.Pool, name)
		delete(view.Packages, name)
		delete(view.Consumers, name)
	}
	for consumer, dependencies := range view.Consumers {
		remaining := []string{}
		for _, name := range dependencies {
			if !changed[name] {
				remaining = append(remaining, name)
			}
		}
		if len(remaining) == 0 {
			delete(view.Consumers, consumer)
		} else {
			view.Consumers[consumer] = remaining
		}
	}
	return view
}

func verifyBrewMutation(before, after brewSnapshot, changed map[string]bool) error {
	for name := range changed {
		old, existed := before.Packages[name]
		current, remains := after.Packages[name]
		if existed && remains && (old.Source != current.Source || old.Held != current.Held || old.Automatic != current.Automatic) {
			return errors.New("native operation changed a protected Homebrew source, pin or manual classification")
		}
	}
	old, err := digest(before.excluding(changed))
	if err != nil {
		return err
	}
	current, err := digest(after.excluding(changed))
	if err != nil {
		return err
	}
	if old != current {
		return errors.New("native operation changed Homebrew packages or consumers outside its recorded command")
	}
	return nil
}

func (d *BrewDriver) acceptMutation(ctx context.Context, before brewSnapshot, operation string) error {
	intent, err := d.intent(operation)
	if err != nil {
		return err
	}
	name, err := brewShortName(intent.Package)
	if err != nil {
		return err
	}
	changed := map[string]bool{name: true}
	if intent.Action == "remove" {
		// Removal also relinquishes pool entries already absent, manually
		// promoted or replaced before this approved operation. This changes
		// only our ledger; their native packages are never removal targets.
		for name, owned := range before.Ledger.Pool {
			pkg, present := before.Packages[name]
			if !present || pkg.Source != owned.Source || !pkg.Automatic {
				changed[name] = true
			}
		}
	}
	for _, command := range intent.Commands {
		reply, err := d.commandResult(command)
		if err != nil {
			return err
		}
		kegs, err := brewInstallEvidence(reply.Output, filepath.ToSlash(d.Cellar), command.Operation)
		if err != nil {
			return err
		}
		for _, keg := range kegs {
			changed[keg.Name] = true
		}
		if command.Arguments[0] == "uninstall" {
			for _, removed := range command.Arguments[3:] {
				changed[removed] = true
			}
		}
	}
	after, err := d.snapshot(ctx)
	if err != nil {
		return err
	}
	if err := verifyBrewMutation(before, after, changed); err != nil {
		return err
	}
	if d.approval != nil {
		d.approval.expected, err = digest(after)
	}
	return err
}

// Resource observations carry the scoped inventory digest. There are no
// additional catalog members to discover from arbitrary native packages.
func (d *NativeDriver) ObserveInventory(context.Context, *Catalog, Context, State) (map[string]Observation, error) {
	return map[string]Observation{}, nil
}

func (d *NativeDriver) VerifyInventory(ctx context.Context, _ *Catalog, _ Context, _ State, plan Plan) error {
	for _, op := range plan.Operations {
		if d.Brew != nil && op.Observed.Provider == "homebrew-formula" {
			return d.Brew.approveInventory(ctx, plan.Inventories)
		}
		if d.APT != nil && op.Observed.Provider == "apt" {
			return d.APT.approveInventory(ctx, plan.Inventories)
		}
	}
	return nil
}
