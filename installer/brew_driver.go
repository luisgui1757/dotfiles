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

// BrewDriver owns only packages attributed to its recorded commands. Query is
// the read-only native process boundary; Run uses the locked native worker.
// The controller must hold both locks for the entire mutation, not per command.
type BrewDriver struct {
	Directory, Program, Cellar string
	Packages                   map[string]string
	Checks                     map[string][]string
	Environment                []string
	Keep                       []string
	Query                      nativeCommandRunner
	Run                        func(context.Context, nativeCommand) ([]byte, error)
	approval                   *brewApproval
}

var brewProcessControls = []string{"LC_ALL=C", "HOMEBREW_NO_ANALYTICS=1", "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_INSTALL_CLEANUP=1", "HOMEBREW_NO_AUTOREMOVE=1", "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK=", "HOMEBREW_NO_INSTALL_UPGRADE=1", "HOMEBREW_NO_ASK=1", "HOMEBREW_NO_COLOR=1"}

type brewOwnership struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	CreatedBy string `json:"created_by"`
}

type brewLedger struct {
	Schema int                      `json:"schema"`
	Cellar string                   `json:"cellar"`
	Roots  map[string]brewOwnership `json:"roots"`
	Pool   map[string]brewOwnership `json:"pool"`
}

type brewIntent struct {
	Schema         int                      `json:"schema"`
	Resource       string                   `json:"resource"`
	Operation      string                   `json:"operation"`
	Package        string                   `json:"package"`
	Action         string                   `json:"action"`
	Before         map[string]nativePackage `json:"before"`
	Remove         []string                 `json:"remove,omitempty"`
	Commands       []nativeCommand          `json:"commands"`
	Complete       bool                     `json:"complete,omitempty"`
	Preserved      []string                 `json:"preserved,omitempty"`
	UncheckedCasks []string                 `json:"unchecked_casks,omitempty"`
}

func (d *BrewDriver) validate(r Resource) (string, error) {
	full, ok := d.Packages[r.ID]
	name, err := brewShortName(full)
	if !ok || err != nil || !resourceID.MatchString(r.ID) || d.Query == nil ||
		!filepath.IsAbs(d.Directory) || filepath.Clean(d.Directory) != d.Directory ||
		!filepath.IsAbs(d.Cellar) || filepath.Clean(d.Cellar) != d.Cellar ||
		!filepath.IsAbs(d.Program) || filepath.Clean(d.Program) != d.Program {
		return "", errors.New("invalid Homebrew provider boundary")
	}
	bound, err := bindConfigDestination(filepath.Join(d.Directory, "packages.json"))
	if err != nil || bound != filepath.Join(d.Directory, "packages.json") {
		return "", errors.Join(errors.New("Homebrew recovery directory was redirected"), err)
	}
	for _, value := range d.Environment {
		if !validBrewEnvironment(value) {
			return "", errors.New("Homebrew command environment contains an unsupported override")
		}
	}
	for _, required := range brewProcessControls {
		if !slices.Contains(d.Environment, required) {
			return "", errors.New("Homebrew provider lacks its unrelated-package preservation controls")
		}
	}
	if check, ok := d.Checks[r.ID]; ok {
		if len(check) == 0 {
			return "", errors.New("Homebrew health command is empty")
		}
		if err := (nativeCommand{Program: check[0], Arguments: check[1:]}).validate(); err != nil {
			return "", err
		}
	}
	return name, nil
}

// Do not persist inherited environment or credentials. These are the small
// explicit process controls needed to prevent unrelated upgrades and cleanup.
func validBrewEnvironment(value string) bool {
	if value == "LC_ALL=C" {
		return true
	}
	key, content, valid := strings.Cut(value, "=")
	if !valid {
		return false
	}
	if key == "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK" {
		return content == ""
	}
	if key == "XDG_CONFIG_HOME" {
		return filepath.IsAbs(content) && filepath.Clean(content) == content && !strings.ContainsAny(content, "\x00\r\n")
	}
	return content == "1" && slices.Contains([]string{"HOMEBREW_NO_ANALYTICS", "HOMEBREW_NO_AUTO_UPDATE", "HOMEBREW_NO_INSTALL_CLEANUP", "HOMEBREW_NO_AUTOREMOVE", "HOMEBREW_NO_INSTALL_UPGRADE", "HOMEBREW_NO_ASK", "HOMEBREW_NO_COLOR", "HOMEBREW_NO_ENV_HINTS"}, key)
}

func (d *BrewDriver) ledger() (brewLedger, error) {
	state := brewLedger{Schema: 1, Cellar: d.Cellar, Roots: map[string]brewOwnership{}, Pool: map[string]brewOwnership{}}
	data, err := readDocument(filepath.Join(d.Directory, "packages.json"))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := Decode(data, &state); err != nil {
		return state, err
	}
	if state.Schema != 1 || state.Cellar != d.Cellar || state.Roots == nil || state.Pool == nil {
		return state, errors.New("invalid Homebrew ownership ledger")
	}
	for id, owned := range state.Roots {
		name, err := brewShortName(owned.Source)
		if err != nil || name != owned.Name || !resourceID.MatchString(id) || !brewPackageName.MatchString(owned.Name) || !operationID.MatchString(owned.CreatedBy) {
			return state, errors.New("invalid Homebrew root ownership")
		}
	}
	for name, owned := range state.Pool {
		source, err := brewShortName(owned.Source)
		if err != nil || source != name || name != owned.Name || !brewPackageName.MatchString(name) || !operationID.MatchString(owned.CreatedBy) {
			return state, errors.New("invalid Homebrew incidental ownership")
		}
	}
	return state, nil
}

func (d *BrewDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *BrewDriver) intent(operation string) (brewIntent, error) {
	var intent brewIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Homebrew operation identity")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 3 || intent.Before == nil || intent.Operation != operation || !resourceID.MatchString(intent.Resource) || !slices.Contains([]string{"install", "update", "repair", "remove"}, intent.Action) || len(intent.Commands) == 0 || len(intent.Commands) > 32 {
		return intent, errors.New("invalid Homebrew operation intent")
	}
	if _, err := brewShortName(intent.Package); err != nil {
		return intent, err
	}
	for name, baseline := range intent.Before {
		if err := validateBrewBaseline(name, baseline); err != nil {
			return intent, err
		}
	}
	for _, name := range intent.UncheckedCasks {
		if !validBrewCaskIdentity(name) || intent.Before[name].Name != name || intent.Action == "remove" {
			return intent, errors.New("invalid saved Homebrew external application disclosure")
		}
	}
	for _, name := range intent.Remove {
		if !brewPackageName.MatchString(name) && !validBrewCaskIdentity(name) {
			return intent, errors.New("invalid saved Homebrew inventory identity")
		}
	}
	for i, command := range intent.Commands {
		if err := command.validate(); err != nil {
			return intent, err
		}
		expected, err := digest(struct {
			Operation string
			Attempt   int
		}{intent.Operation, i})
		if err != nil || command.Operation != expected || command.Program != d.Program || len(command.Input) != 0 || len(command.Arguments) < 3 || command.Arguments[1] != "--formula" {
			return intent, errors.Join(errors.New("saved Homebrew command differs from its trusted provider"), err)
		}
		expectedEnvironment := append(slices.Clone(d.Environment), "HOMEBREW_INSTALL_BADGE=dotfiles-operation-"+command.Operation)
		if !slices.Equal(command.Environment, expectedEnvironment) {
			return intent, errors.New("saved Homebrew command changed its process environment")
		}
		if intent.Action == "remove" {
			if slices.Equal(command.Arguments, []string{"list", "--formula", "--versions"}) {
				continue
			}
			if len(command.Arguments) < 4 || command.Arguments[0] != "uninstall" || command.Arguments[2] != "--force" {
				return intent, errors.New("invalid saved Homebrew removal")
			}
			for _, name := range command.Arguments[3:] {
				if !brewPackageName.MatchString(name) || !slices.Contains(intent.Remove, name) {
					return intent, errors.New("Homebrew removal exceeds its saved exact set")
				}
			}
		} else {
			mode := map[string]string{"install": "install", "update": "upgrade", "repair": "reinstall"}[intent.Action]
			rootCommand := len(command.Arguments) == 3 && command.Arguments[0] == mode && command.Arguments[2] == intent.Package
			if !rootCommand {
				if i == 0 || len(command.Arguments) != 3 || command.Arguments[0] != "reinstall" {
					return intent, errors.New("invalid saved Homebrew package command")
				}
				candidates, err := d.maintenanceCandidates(intent, i)
				name, nameErr := brewShortName(command.Arguments[2])
				if err != nil || nameErr != nil || candidates[name] != command.Arguments[2] {
					return intent, errors.Join(errors.New("saved Homebrew maintenance exceeds affected native consumers"), err, nameErr)
				}
			}
		}

		if err := command.validate(); err != nil {
			return intent, errors.Join(errors.New("invalid saved Homebrew command"), err)
		}
	}
	return intent, nil
}

func validBrewCaskIdentity(name string) bool {
	return len(name) > 5 && name[:5] == "cask/" && brewPackageName.MatchString(name[5:])
}

func (d *BrewDriver) commandResult(command nativeCommand) (nativeReply, error) {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash || record.Reply == nil {
		return nativeReply{}, errors.Join(errors.New("Homebrew command completion is unproved"), err)
	}
	if record.Reply.Ready || record.Reply.ExitCode == nil && record.Reply.Error == "" || len(record.Reply.Output) > 1<<20 {
		return nativeReply{}, errors.New("invalid Homebrew command result")
	}
	return *record.Reply, nil
}

func (d *BrewDriver) introduced(intent brewIntent) (map[string]bool, error) {
	introduced := map[string]bool{}
	for _, command := range intent.Commands {
		reply, err := d.commandResult(command)
		if err != nil {
			return nil, err
		}
		kegs, err := brewInstallEvidence(reply.Output, filepath.ToSlash(d.Cellar), command.Operation)
		if err != nil {
			return nil, err
		}
		for _, keg := range kegs {
			if _, existed := intent.Before[keg.Name]; !existed {
				introduced[keg.Name] = true
			}
		}
	}
	return introduced, nil
}

func (d *BrewDriver) proveOwnership(owned brewOwnership) error {
	intent, err := d.intent(owned.CreatedBy)
	if err != nil || !intent.Complete || intent.Action == "remove" {
		return errors.Join(errors.New("Homebrew ownership lacks a completed introduction"), err)
	}
	introduced, err := d.introduced(intent)
	if err != nil || !introduced[owned.Name] {
		return errors.Join(errors.New("Homebrew package was not introduced by its claimed operation"), err)
	}
	last, err := d.commandResult(intent.Commands[len(intent.Commands)-1])
	if err != nil || last.Error != "" || *last.ExitCode != 0 {
		return errors.Join(errors.New("Homebrew ownership lacks a successful final command"), err)
	}
	return nil
}

func (d *BrewDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	name, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	state, err := d.ledger()
	if err != nil {
		return Observation{}, err
	}
	pkg, present := installed[name]
	o := Observation{Provider: "homebrew-formula", Identity: filepath.Join(d.Cellar, name), Scope: "machine", Present: present, Healthy: present && pkg.Healthy}
	if o.Healthy {
		if err := d.check(ctx, r); err != nil {
			if ctx.Err() != nil {
				return o, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return o, err
			}
			o.Healthy, o.HealthIssue = false, err.Error()
		}
	}
	o.Inventory, err = digest(brewProviderSnapshot(state, installed, d.Keep))
	if err != nil {
		return o, err
	}
	if present {
		o.Fingerprint, err = digest(struct {
			Name, Source string
			Held         bool
		}{name, pkg.Source, pkg.Held})
		if err != nil {
			return o, err
		}
		// A package version is not its ownership identity: normal native updates
		// must not strand an installer-created root. Source/pin changes do matter.
		for consumer, p := range installed {
			if slices.Contains(p.Dependencies, name) && consumer != name {
				o.Consumers = append(o.Consumers, consumer)
			}
		}
		o.Consumers = sortedUnique(o.Consumers)
	}
	if receipt.OperationID != "" {
		intent, err := d.intent(receipt.OperationID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return o, err
		}
		if err == nil {
			if intent.Resource != r.ID {
				return o, errors.New("Homebrew receipt points to a different resource")
			}
			if intent.Complete {
				last, err := d.commandResult(intent.Commands[len(intent.Commands)-1])
				if err != nil || last.Error != "" || *last.ExitCode != 0 {
					return o, errors.Join(errors.New("Homebrew completion lacks its successful command evidence"), err)
				}
				if intent.Action == "remove" && !present || intent.Action != "remove" && present && state.Roots[r.ID].Name == name && state.Roots[r.ID].Source == pkg.Source {
					o.CompletedOperation = intent.Operation
				}
				for _, kept := range intent.Preserved {
					if !brewPackageName.MatchString(kept) {
						return o, errors.New("invalid retained Homebrew dependency")
					}
					if _, present := installed[kept]; present {
						o.Preserved = append(o.Preserved, filepath.Join(d.Cellar, kept))
					}
				}
				for _, name := range intent.UncheckedCasks {
					if pkg, present := installed[name]; present {
						o.UnverifiedApplications = append(o.UnverifiedApplications, pkg.Source)
					}
				}
			} else if receipt.Status == "in-progress" {
				o.Pending = "resume the saved Homebrew package operation"
				token, err := digest(struct {
					Intent    brewIntent
					Inventory map[string]nativePackage
				}{intent, installed})
				if err != nil {
					return o, err
				}
				o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
			}
		}
	}
	return o, nil
}

func (d *BrewDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}

func (d *BrewDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}

func (d *BrewDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *BrewDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (observed Observation, resultErr error) {
	name, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "update", "repair", "remove"}, action) {
		return Observation{}, errors.New("Homebrew mutation requires its locked worker and saved operation")
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	state, err := d.ledger()
	if err != nil {
		return Observation{}, err
	}
	before := brewProviderSnapshot(state, installed, d.Keep)
	if d.approval != nil {
		hash, err := digest(before)
		if err != nil || d.approval.expected == "" || hash != d.approval.expected {
			return Observation{}, errors.Join(errors.New("Homebrew mutation differs from its approved inventory"), err)
		}
	}
	defer func() {
		if resultErr == nil {
			resultErr = d.acceptMutation(ctx, before, receipt.OperationID)
		}
	}()
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		if _, present := installed[name]; action == "install" && present {
			return Observation{}, errors.New("Homebrew root appeared before installation; preserve it and review a fresh plan")
		}
		intent = brewIntent{Schema: 3, Resource: r.ID, Operation: receipt.OperationID, Package: d.Packages[r.ID], Action: action, Before: map[string]nativePackage{}}
		for name, pkg := range installed {
			intent.Before[name] = pkg
		}
	} else if err != nil {
		return Observation{}, err
	} else if intent.Resource != r.ID || intent.Package != d.Packages[r.ID] || intent.Action != action {
		return Observation{}, errors.New("Homebrew recovery differs from the saved operation")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	var retryMaintenance []string
	if len(intent.Commands) > 0 {
		command := intent.Commands[len(intent.Commands)-1]
		last, err := d.commandResult(command)
		if errors.Is(err, os.ErrNotExist) {
			if err := d.runSavedCommand(ctx, intent, command); err != nil {
				return Observation{}, err
			}
			last, err = d.commandResult(command)
		}
		if err != nil {
			return Observation{}, err
		}
		if last.Error == "" && *last.ExitCode == 0 {
			return d.finish(ctx, r, receipt, intent, state)
		}
		if !resume {
			return Observation{}, errors.New("failed Homebrew command requires explicit recovery approval")
		}
		if action != "remove" && command.Arguments[2] != intent.Package {
			retryMaintenance = command.Arguments
		}
	}
	if pkg, exists := installed[name]; exists && pkg.Held {
		return Observation{}, errors.New("Homebrew package is pinned; preserve the pin and review it before mutation")
	}
	arguments := []string{"install", "--formula", intent.Package}
	if action == "install" && resume && len(intent.Commands) > 0 {
		if pkg, present := installed[name]; present {
			introduced, err := d.introduced(intent)
			if err != nil || !introduced[name] || pkg.Source != intent.Package {
				return Observation{}, errors.Join(errors.New("existing Homebrew root cannot be attributed to the interrupted installation"), err)
			}
			arguments[0] = "reinstall"
		}
	}
	if action == "update" {
		arguments[0] = "upgrade"
	}
	if action == "repair" {
		arguments[0] = "reinstall"
	}
	if action == "remove" {
		owned, ok := state.Roots[r.ID]
		if !ok || owned.Name != name {
			return Observation{}, errors.New("Homebrew root is not owned by this installation")
		}
		if err := d.proveOwnership(owned); err != nil {
			return Observation{}, err
		}
		if pkg, present := installed[name]; present && pkg.Source != owned.Source {
			return Observation{}, errors.New("Homebrew root changed its source; preserve the current package")
		}
		pool := []string{}
		for name, owned := range state.Pool {
			if err := d.proveOwnership(owned); err != nil {
				return Observation{}, err
			}
			if pkg, exists := installed[name]; exists && pkg.Source == owned.Source {
				pool = append(pool, name)
			}
		}
		keep := slices.Clone(d.Keep)
		for other, owned := range state.Roots {
			if other != r.ID {
				keep = append(keep, owned.Name)
			}
		}
		removal, _, err := nativeRemovalCandidates([]string{name}, pool, keep, installed)
		if err != nil {
			return Observation{}, err
		}
		intent.Remove = sortedUnique(append(intent.Remove, removal...))
		// For formulae --force selects all installed versions. Homebrew still
		// checks outside dependents; --ignore-dependencies is never permitted.
		arguments = append([]string{"uninstall", "--formula", "--force"}, removal...)
		if len(removal) == 0 {
			arguments = []string{"list", "--formula", "--versions"}
		}
	} else if action != "install" {
		owned, ok := state.Roots[r.ID]
		if !ok || owned.Name != name {
			return Observation{}, errors.New("pre-existing Homebrew package cannot be updated or repaired implicitly")
		}
		if err := d.proveOwnership(owned); err != nil {
			return Observation{}, err
		}
		if pkg, present := installed[name]; present && pkg.Source != owned.Source {
			return Observation{}, errors.New("Homebrew root changed its source; preserve the current package")
		}
	}
	if len(retryMaintenance) > 0 {
		arguments = retryMaintenance
	}
	if err := d.runCommand(ctx, &intent, arguments); err != nil {
		return Observation{}, err
	}
	return d.finish(ctx, r, receipt, intent, state)
}

func (d *BrewDriver) finish(ctx context.Context, r Resource, receipt Receipt, intent brewIntent, state brewLedger) (Observation, error) {
	last, err := d.commandResult(intent.Commands[len(intent.Commands)-1])
	if err != nil || last.Error != "" || *last.ExitCode != 0 {
		return Observation{}, errors.Join(errors.New("Homebrew command has no successful completion"), err)
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	if err := d.verifyPreexisting(intent, installed); err != nil {
		return Observation{}, err
	}
	name, err := brewShortName(intent.Package)
	if err != nil {
		return Observation{}, err
	}
	if intent.Action == "remove" {
		if _, present := installed[name]; present {
			return Observation{}, errors.New("Homebrew did not remove the requested root")
		}
		removed := intent.Commands[len(intent.Commands)-1].Arguments[3:]
		if intent.Commands[len(intent.Commands)-1].Arguments[0] == "list" {
			removed = nil
		}
		for _, name := range removed {
			if _, present := installed[name]; present {
				return Observation{}, fmt.Errorf("Homebrew did not remove %s", name)
			}
		}
		delete(state.Roots, r.ID)
		for name, owned := range state.Pool {
			if pkg, present := installed[name]; present {
				intent.Preserved = append(intent.Preserved, name)
				if pkg.Source != owned.Source || !pkg.Automatic {
					delete(state.Pool, name)
				}
			} else {
				delete(state.Pool, name)
			}
		}
		intent.Preserved = sortedUnique(intent.Preserved)
	} else {
		pkg, present := installed[name]
		if !present || !pkg.Healthy || pkg.Source != intent.Package {
			return Observation{}, errors.New("Homebrew formula did not pass native activation checks")
		}
		if err := d.check(ctx, r); err != nil {
			return Observation{}, err
		}
		if err := d.maintainDependents(ctx, &intent); err != nil {
			return Observation{}, err
		}
		installed, err = brewInventory(ctx, d.Query)
		if err != nil {
			return Observation{}, err
		}
		if err := d.verifyPreexisting(intent, installed); err != nil {
			return Observation{}, err
		}
		pkg, present = installed[name]
		if !present || !pkg.Healthy || pkg.Source != intent.Package {
			return Observation{}, errors.New("Homebrew root changed during dependent maintenance")
		}
		if err := d.check(ctx, r); err != nil {
			return Observation{}, err
		}
		introduced, err := d.introduced(intent)
		if err != nil {
			return Observation{}, err
		}
		if _, owned := state.Roots[r.ID]; !owned {
			if !introduced[name] {
				return Observation{}, errors.New("Homebrew did not prove introduction of this root; preserve the existing package")
			}
			state.Roots[r.ID] = brewOwnership{Name: name, Source: pkg.Source, CreatedBy: intent.Operation}
		}
		for added := range introduced {
			if added == name {
				continue
			}
			pkg, present := installed[added]
			if !present || !pkg.Healthy {
				return Observation{}, fmt.Errorf("introduced Homebrew dependency %s is not healthy", added)
			}
			state.Pool[added] = brewOwnership{Name: added, Source: pkg.Source, CreatedBy: intent.Operation}
		}
	}
	// The ledger can be replayed after a crash between these writes. Intent is
	// complete only after its root/pool publication, and Observe still probes the
	// current native installation before returning completion.
	// Persist retained-package disclosure before relinquishing pool ownership.
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	if err := saveDocument(filepath.Join(d.Directory, "packages.json"), state); err != nil {
		return Observation{}, err
	}
	intent.Complete = true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// Checks come from the trusted provider catalog, never from saved operations or
// PATH. Data-only contract formulae omit them; production CLI mappings supply one.
func (d *BrewDriver) check(ctx context.Context, r Resource) error {
	command, ok := d.Checks[r.ID]
	if !ok {
		return nil
	}
	output, err := d.Query(ctx, false, command[0], nil, command[1:]...)
	if err != nil {
		return fmt.Errorf("Homebrew %s failed its native command check: %w%s", r.Name, err, nativeDiagnostic(output))
	}
	return nil
}
