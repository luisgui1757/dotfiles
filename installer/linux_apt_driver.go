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

// LinuxAPTDriver attributes only packages proved by saved native commands.
// The controller must hold its mutation session and call approveInventory before
// mutation; previews remain read-only. Source/manual/hold changes never enlarge
// removal authority. Source means dpkg's source package, not repository origin:
// dpkg does not persist the repository that supplied an installed package.
type LinuxAPTDriver struct {
	Directory, WorkerDirectory string
	Location                   LinuxAPTLocation
	Packages                   map[string]string
	Checks                     map[string][]string
	Keep                       []string
	Query                      nativeCommandRunner
	Run                        func(context.Context, nativeCommand) ([]byte, error)
	// Authenticate belongs to the foreground application. The durable worker
	// always uses sudo -n and never reads a password. Machine requests omit it.
	Authenticate func(context.Context) error
	approval     *linuxAPTApproval
}

type linuxAPTOwnership struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	CreatedBy string `json:"created_by"`
}

type linuxAPTLedger struct {
	Schema int                          `json:"schema"`
	Roots  map[string]linuxAPTOwnership `json:"roots"`
	Pool   map[string]linuxAPTOwnership `json:"pool"`
}

type linuxAPTCommand struct {
	Kind    string        `json:"kind"`
	Command nativeCommand `json:"command"`
}

type linuxAPTIntent struct {
	Schema    int                      `json:"schema"`
	Resource  string                   `json:"resource"`
	Operation string                   `json:"operation"`
	Package   string                   `json:"package"`
	Action    string                   `json:"action"`
	Before    map[string]nativePackage `json:"before"`
	Commands  []linuxAPTCommand        `json:"commands"`
	Remove    []string                 `json:"remove,omitempty"`
	Preserved []string                 `json:"preserved,omitempty"`
	// Claim references the original native introduction when a selected package
	// was already installed incidentally by an earlier approved operation.
	Claim             string             `json:"claim,omitempty"`
	Deferred          *linuxAPTOwnership `json:"deferred,omitempty"`
	PreserveAutomatic bool               `json:"preserve_automatic,omitempty"`
	Complete          bool               `json:"complete,omitempty"`
}

type linuxAPTApproval struct{ expected string }

func (d *LinuxAPTDriver) validate(r Resource) (string, error) {
	name, bound := d.Packages[r.ID]
	if !bound || !aptArgument.MatchString(name) || !resourceID.MatchString(r.ID) || d.Query == nil {
		return "", errors.New("invalid APT resource boundary")
	}
	if err := d.Location.validate(); err != nil {
		return "", err
	}
	for _, directory := range []string{d.Directory, d.WorkerDirectory} {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
			return "", errors.New("APT requires canonical native state directories")
		}
		bound, err := bindConfigDestination(filepath.Join(directory, "packages.json"))
		if err != nil || bound != filepath.Join(directory, "packages.json") {
			return "", errors.Join(errors.New("APT state directory was redirected"), err)
		}
	}
	if check, ok := d.Checks[r.ID]; ok {
		if len(check) == 0 {
			return "", errors.New("APT health command is empty")
		}
		if err := (nativeCommand{Program: check[0], Arguments: check[1:]}).validate(); err != nil {
			return "", err
		}
	}
	return name, nil
}

func (d *LinuxAPTDriver) ledger() (linuxAPTLedger, error) {
	state := linuxAPTLedger{Schema: 1, Roots: map[string]linuxAPTOwnership{}, Pool: map[string]linuxAPTOwnership{}}
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
	if state.Schema != 1 || state.Roots == nil || state.Pool == nil {
		return state, errors.New("invalid APT ownership ledger")
	}
	for id, owned := range state.Roots {
		if !resourceID.MatchString(id) || !aptPackageName.MatchString(owned.Name) || !aptArgument.MatchString(owned.Source) || !operationID.MatchString(owned.CreatedBy) {
			return state, errors.New("invalid APT root ownership")
		}
	}
	for name, owned := range state.Pool {
		if owned.Name != name || !aptPackageName.MatchString(name) || !aptArgument.MatchString(owned.Source) || !operationID.MatchString(owned.CreatedBy) {
			return state, errors.New("invalid APT incidental ownership")
		}
	}
	return state, nil
}

func (d *LinuxAPTDriver) inventoryDigest(state linuxAPTLedger, installed map[string]nativePackage) (string, error) {
	return digest(struct {
		Ledger    linuxAPTLedger
		Installed map[string]nativePackage
		Keep      []string
	}{state, installed, sortedUnique(d.Keep)})
}

func (d *LinuxAPTDriver) approveInventory(ctx context.Context, approved []string) error {
	if d.approval == nil {
		return errors.New("APT requires its controller approval session")
	}
	state, err := d.ledger()
	if err != nil {
		return err
	}
	installed, err := linuxAPTInventory(ctx, d.Query)
	if err != nil {
		return err
	}
	hash, err := d.inventoryDigest(state, installed)
	if err != nil {
		return err
	}
	if d.approval.expected == "" {
		if !slices.Contains(approved, hash) {
			return errors.New("APT packages or ownership changed after approval")
		}
		d.approval.expected = hash
	} else if hash != d.approval.expected {
		return errors.New("APT packages or ownership changed during the operation")
	}
	return nil
}

func (d *LinuxAPTDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *LinuxAPTDriver) intent(operation string) (linuxAPTIntent, error) {
	var intent linuxAPTIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid APT operation identity")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 1 || intent.Operation != operation || !resourceID.MatchString(intent.Resource) || !aptArgument.MatchString(intent.Package) || intent.Before == nil || len(intent.Commands) > 32 || !slices.Contains([]string{"install", "update", "repair", "remove"}, intent.Action) {
		return intent, errors.New("invalid saved APT intent")
	}
	if intent.Claim != "" && (!operationID.MatchString(intent.Claim) || intent.Claim == operation || intent.Action != "install" || len(intent.Commands) != 0) || intent.Deferred != nil && intent.Action != "remove" || intent.PreserveAutomatic && intent.Action != "update" && intent.Action != "repair" {
		return intent, errors.New("invalid APT ownership reconciliation intent")
	}
	if intent.Deferred != nil {
		owned := *intent.Deferred
		if !aptPackageName.MatchString(owned.Name) || !aptArgument.MatchString(owned.Source) || !operationID.MatchString(owned.CreatedBy) || !slices.Contains(intent.Preserved, owned.Name) {
			return intent, errors.New("invalid APT retained-root evidence")
		}
	}
	if intent.PreserveAutomatic {
		identity, err := linuxAPTIdentity(intent.Package, intent.Before)
		if err != nil || identity == "" || !intent.Before[identity].Automatic {
			return intent, errors.Join(errors.New("APT automatic classification differs from its saved baseline"), err)
		}
	}
	for name, pkg := range intent.Before {
		if !aptPackageName.MatchString(name) || pkg.Name != name || pkg.Version == "" || !aptArgument.MatchString(pkg.Source) {
			return intent, errors.New("invalid saved APT baseline")
		}
	}
	for _, name := range append(slices.Clone(intent.Remove), intent.Preserved...) {
		if !aptPackageName.MatchString(name) {
			return intent, errors.New("invalid saved APT exact package set")
		}
	}
	for index, saved := range intent.Commands {
		prefix := d.commandPrefixLength()
		if len(saved.Command.Arguments) < prefix {
			return intent, errors.New("invalid saved APT command prefix")
		}
		expected, err := d.command(intent, index, saved.Kind, saved.Command.Input, saved.Command.Arguments[prefix:])
		if err != nil {
			return intent, err
		}
		actualHash, err := digest(saved.Command)
		if err != nil {
			return intent, err
		}
		expectedHash, err := digest(expected)
		if err != nil || actualHash != expectedHash {
			return intent, errors.Join(errors.New("saved APT command differs from its trusted recipe"), err)
		}
	}
	return intent, nil
}

func (d *LinuxAPTDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	name, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	installed, err := linuxAPTInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	state, err := d.ledger()
	if err != nil {
		return Observation{}, err
	}
	identity, err := linuxAPTIdentity(name, installed)
	if err != nil {
		return Observation{}, err
	}
	pkg, present := installed[identity]
	o := Observation{Provider: "apt", Identity: "apt:" + name, Scope: "machine", Present: present, Healthy: present && pkg.Healthy}
	o.Inventory, err = d.inventoryDigest(state, installed)
	if err != nil {
		return o, err
	}
	if present {
		o.Fingerprint, err = digest(struct {
			Name, Source    string
			Held, Automatic bool
		}{identity, pkg.Source, pkg.Held, pkg.Automatic})
		if err != nil {
			return o, err
		}
		for consumer, p := range installed {
			if consumer != identity && slices.Contains(p.Dependencies, identity) && !d.ownedConsumer(state, p) {
				o.Consumers = append(o.Consumers, consumer)
			}
		}
		o.Consumers = sortedUnique(o.Consumers)
		if o.Healthy {
			if err := d.check(ctx, r); err != nil {
				if ctx.Err() != nil {
					return o, ctx.Err()
				}
				o.Healthy, o.HealthIssue = false, err.Error()
			}
		}
	}
	if owned, ok := state.Pool[identity]; ok && o.Healthy && !pkg.Held && d.matchesOwnership(owned, pkg) {
		o.CompletedOperation = owned.CreatedBy
	}
	if receipt.OperationID == "" {
		return o, nil
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if intent.Resource != r.ID || intent.Package != name {
		return o, errors.New("APT receipt points to a different resource")
	}
	if intent.Complete {
		if intent.Action == "remove" && present && intent.Deferred != nil && state.Roots[r.ID].Name == "" && state.Pool[identity] == *intent.Deferred && d.matchesOwnership(*intent.Deferred, pkg) {
			o.RemovalDeferred = true
		}
		if intent.Action == "remove" && (!present || o.RemovalDeferred) || intent.Action != "remove" && o.Healthy && state.Roots[r.ID].Name == identity && state.Roots[r.ID].Source == pkg.Source {
			if err := d.verifyCompletion(intent); err != nil {
				return o, err
			}
			o.CompletedOperation = intent.Operation
		}
		for _, kept := range intent.Preserved {
			if _, present := installed[kept]; present {
				o.Preserved = append(o.Preserved, "apt:"+kept)
			}
		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "resume the saved APT package operation"
		token, err := digest(struct {
			Intent    linuxAPTIntent
			Installed map[string]nativePackage
		}{intent, installed})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	}
	return o, nil
}

func (d *LinuxAPTDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}

func (d *LinuxAPTDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}

func (d *LinuxAPTDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *LinuxAPTDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	name, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	if d.Run == nil || d.approval == nil || d.approval.expected == "" || !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "update", "repair", "remove"}, action) {
		return Observation{}, errors.New("APT mutation requires controller approval, its locked worker and saved operation")
	}
	if err := d.approveInventory(ctx, nil); err != nil {
		return Observation{}, err
	}
	installed, err := linuxAPTInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	state, err := d.ledger()
	if err != nil {
		return Observation{}, err
	}
	identity, err := linuxAPTIdentity(name, installed)
	if err != nil {
		return Observation{}, err
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		intent = linuxAPTIntent{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Package: name, Action: action, Before: installed}
		if action == "install" && identity != "" {
			owned, ok := state.Pool[identity]
			if receipt.Before.Present || !ok || installed[identity].Held || !d.matchesOwnership(owned, installed[identity]) {
				return Observation{}, errors.New("APT root appeared before installation; preserve it and review a fresh plan")
			}
			intent.Claim = owned.CreatedBy
		}
		intent.PreserveAutomatic = (action == "update" || action == "repair") && installed[identity].Automatic
	} else if err != nil {
		return Observation{}, err
	} else if intent.Resource != r.ID || intent.Package != name || intent.Action != action {
		return Observation{}, errors.New("APT recovery differs from its saved operation")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	if intent.Claim != "" {
		return d.finish(ctx, r, receipt, intent, state)
	}
	if len(intent.Commands) > 0 {
		last := intent.Commands[len(intent.Commands)-1]
		reply, err := d.commandResult(last.Command)
		if errors.Is(err, os.ErrNotExist) {
			if err := d.prepareDispatch(ctx, intent, last); err != nil {
				return Observation{}, err
			}
			if _, err := d.Run(ctx, last.Command); err != nil {
				return Observation{}, d.mutationError(last, err)
			}
			reply, err = d.commandResult(last.Command)
		}
		if err != nil {
			return Observation{}, err
		}
		if reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0 && last.Kind != "restore" && last.Kind != "refresh" {
			return d.finish(ctx, r, receipt, intent, state)
		}
		if !resume && (reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0) {
			return Observation{}, errors.New("failed APT command requires explicit recovery approval")
		}
	}
	if pkg, exists := installed[identity]; exists && pkg.Held {
		return Observation{}, errors.New("APT root is held; preserve its hold before reviewing further work")
	}
	if action != "install" {
		owned, ok := state.Roots[r.ID]
		base, _, _ := strings.Cut(owned.Name, ":")
		if !ok || name != owned.Name && name != base || identity != "" && (identity != owned.Name || installed[identity].Source != owned.Source) {
			return Observation{}, errors.New("APT root is pre-existing or changed its source; preserve it")
		}
		if err := d.proveOwnership(owned); err != nil {
			return Observation{}, err
		}
	}
	if action == "remove" {
		owned := state.Roots[r.ID]
		candidates, err := d.removalCandidates(r.ID, state, installed)
		if err != nil {
			return Observation{}, err
		}
		if identity != "" && !slices.Contains(candidates, owned.Name) {
			// The root's ownership is released, but its proved native package is
			// retained in the pool until the final native consumer is removed.
			intent.Deferred = &owned
			intent.Preserved = sortedUnique(append(intent.Preserved, owned.Name))
		}
		intent.Remove = sortedUnique(append(intent.Remove, candidates...))
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
		run := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
			if !privileged {
				return d.Query(ctx, false, program, input, args...)
			}
			kind := "remove"
			if len(args) == 1 && args[0] == "--set-selections" {
				kind = "restore"
			}
			return d.runCommand(ctx, &intent, kind, input, args)
		}
		if _, err := removeAPTPackages(ctx, run, candidates); err != nil {
			return Observation{}, err
		}
		if len(candidates) == 0 {
			if _, err := d.runCommand(ctx, &intent, "verify-remove", nil, nil); err != nil {
				return Observation{}, err
			}
		}
	} else {
		refreshed := false
		if len(intent.Commands) > 0 {
			last := intent.Commands[len(intent.Commands)-1]
			reply, err := d.commandResult(last.Command)
			refreshed = err == nil && last.Kind == "refresh" && reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0
		}
		if !refreshed {
			if _, err := d.runCommand(ctx, &intent, "refresh", nil, nil); err != nil {
				return Observation{}, err
			}
		}
		if _, err := d.runCommand(ctx, &intent, "install", nil, nil); err != nil {
			return Observation{}, err
		}
	}
	return d.finish(ctx, r, receipt, intent, state)
}

func (d *LinuxAPTDriver) check(ctx context.Context, r Resource) error {
	command, ok := d.Checks[r.ID]
	if !ok {
		return nil
	}
	output, err := d.Query(ctx, false, command[0], nil, command[1:]...)
	if err != nil {
		return fmt.Errorf("APT %s failed its command check: %w%s", r.Name, err, nativeDiagnostic(output))
	}
	// dpkg --verify reports changed files on stdout while returning zero.
	if r.Action == "apt-library" && strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("APT %s has changed package files%s", r.Name, nativeDiagnostic(output))
	}
	return nil
}

func (d *LinuxAPTDriver) finish(ctx context.Context, r Resource, receipt Receipt, intent linuxAPTIntent, state linuxAPTLedger) (Observation, error) {
	if err := d.verifyCompletion(intent); err != nil {
		return Observation{}, err
	}
	installed, err := linuxAPTInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	introduced, changes, err := d.evidence(intent)
	if err != nil {
		return Observation{}, err
	}
	provisional, _, _, err := d.attemptEvidence(intent)
	if err != nil {
		return Observation{}, err
	}
	for name := range introduced {
		pkg, present := installed[name]
		if !present || !pkg.Healthy || pkg.Version != changes[name] || pkg.Automatic != provisional[name].Automatic {
			return Observation{}, fmt.Errorf("introduced APT package %s changed from its saved installation evidence", name)
		}
	}
	for name := range installed {
		if _, existed := intent.Before[name]; !existed && !introduced[name] && intent.Action != "remove" {
			return Observation{}, fmt.Errorf("APT package %s appeared without operation-specific introduction evidence", name)
		}
	}
	for name, old := range intent.Before {
		pkg, present := installed[name]
		if intent.Action == "remove" && slices.Contains(intent.Remove, name) && !present {
			continue
		}
		if !present || pkg.Source != old.Source || pkg.Held != old.Held || pkg.Automatic != old.Automatic || old.Healthy && !pkg.Healthy {
			return Observation{}, fmt.Errorf("APT changed protected package %s; preserve the operation for inspection", name)
		}
		if pkg.Version != old.Version && changes[name] != pkg.Version {
			return Observation{}, fmt.Errorf("APT version change for %s lacks operation-specific dpkg evidence", name)
		}
	}
	identity, err := linuxAPTIdentity(intent.Package, installed)
	if err != nil {
		return Observation{}, err
	}
	if intent.Action == "remove" {
		if identity != "" && (intent.Deferred == nil || intent.Deferred.Name != identity || !d.matchesOwnership(*intent.Deferred, installed[identity])) {
			return Observation{}, errors.New("APT did not remove or prove retention of the requested root")
		}
		if intent.Deferred != nil && identity != "" {
			state.Pool[identity] = *intent.Deferred
		}
		removed := []string{}
		if len(intent.Commands) > 0 {
			last := intent.Commands[len(intent.Commands)-1]
			if last.Kind == "remove" {
				removed = last.Command.Arguments[d.commandPrefixLength()+1:]
			}
		}
		for _, name := range removed {
			if _, present := installed[name]; present {
				return Observation{}, fmt.Errorf("APT exact removal left %s installed", name)
			}
		}
		delete(state.Roots, r.ID)
		for name, owned := range state.Pool {
			pkg, present := installed[name]
			if present {
				intent.Preserved = append(intent.Preserved, name)
			}
			if !present || !d.matchesOwnership(owned, pkg) {
				delete(state.Pool, name)
			}
		}
		intent.Preserved = sortedUnique(intent.Preserved)
	} else {
		pkg, present := installed[identity]
		if !present || !pkg.Healthy {
			return Observation{}, errors.New("APT root has not reached installed native state")
		}
		if err := d.check(ctx, r); err != nil {
			return Observation{}, err
		}
		if intent.Claim != "" {
			owned := linuxAPTOwnership{identity, pkg.Source, intent.Claim}
			if !d.matchesOwnership(owned, pkg) || state.Pool[identity] != owned && state.Roots[r.ID] != owned {
				return Observation{}, errors.New("APT incidental root claim lost its exact ownership evidence")
			}
			state.Roots[r.ID] = owned
			delete(state.Pool, identity)
		} else if introduced[identity] {
			// A missing formerly owned root can be installed again. Bind that
			// actual reintroduction, not its obsolete native classification.
			state.Roots[r.ID] = linuxAPTOwnership{identity, pkg.Source, intent.Operation}
			delete(state.Pool, identity)
		} else if _, owned := state.Roots[r.ID]; !owned {
			return Observation{}, errors.New("APT did not prove introduction of this root; preserve the existing package")
		}
		for name := range introduced {
			if name == identity {
				continue
			}
			pkg, present := installed[name]
			if !present || !pkg.Healthy {
				return Observation{}, fmt.Errorf("introduced APT package %s is not healthy", name)
			}
			state.Pool[name] = linuxAPTOwnership{name, pkg.Source, intent.Operation}
		}
	}
	// Disclosure precedes relinquishing the shared pool; an interrupted write is
	// replayable, and old receipts filter disclosures against current inventory.
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
	d.approval.expected, err = d.inventoryDigest(state, installed)
	if err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *LinuxAPTDriver) mutationError(command linuxAPTCommand, err error) error {
	reply, readErr := d.commandResult(command.Command)
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	diagnostic := nativeDiagnostic(reply.Output)
	if strings.Contains(string(reply.Output), "sudo:") {
		return fmt.Errorf("APT needs administrator authorization in the foreground; authorize sudo and retry the saved operation: %w%s", err, diagnostic)
	}
	return fmt.Errorf("APT native command failed: %w%s", err, diagnostic)
}
