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

// Flatpak uses a private per-user installation. Native runtimes and the remote
// are retained, and uninstall never deletes ~/.var/app or a user's documents.
// No operation targets the user's ordinary or system-wide Flatpak installation.
type FlatpakDriver struct {
	Directory, WorkerDirectory, Home, Arch string
	Run                                    func(context.Context, nativeCommand) ([]byte, error)
	Query                                  func(context.Context, nativeCommand) ([]byte, error)
}

type flatpakIntent struct {
	PrivateDependencies                                 bool `json:"private_dependencies,omitempty"`
	Schema                                              int  `json:"schema"`
	Operation, Resource, Action, Directory, Fingerprint string
	Attempts                                            []int `json:"attempts,omitempty"`
	Commands                                            []nativeCommand
	Complete                                            bool
}

type flatpakOwner struct {
	Schema               int    `json:"schema"`
	ApplicationOperation string `json:"application_operation,omitempty"`
	CreatedBy, Directory string
}

func (d *FlatpakDriver) store(id string) string {
	return filepath.Join(d.Directory, id, "installation")
}
func (d *FlatpakDriver) intentPath(op string) string {
	return filepath.Join(d.Directory, "operations", op+".json")
}
func (d *FlatpakDriver) ownerPath(id string) string {
	return filepath.Join(d.Directory, id, "owner.json")
}
func (d *FlatpakDriver) ref(r Resource) string {
	return "app/" + r.Bindings["linux"].Package + "/" + d.Arch + "/stable"
}
func (d *FlatpakDriver) command(r Resource, args ...string) nativeCommand {
	// The portal used for sandboxed subprocesses also opens SYSTEM_DIR/repo.
	// Use this same private store for both lookups, never an absent placeholder
	// or the machine's ordinary installation. All mutations still use --user.
	return nativeCommand{Program: "/usr/bin/flatpak", Arguments: args, Environment: []string{
		"HOME=" + d.Home, "FLATPAK_USER_DIR=" + d.store(r.ID), "LC_ALL=C", "FLATPAK_TTY_PROGRESS=0",
		"FLATPAK_SYSTEM_DIR=" + d.store(r.ID),
		"FLATPAK_CONFIG_DIR=" + filepath.Join(d.store(r.ID), "config"),
	}}
}
func (d *FlatpakDriver) validate(r Resource) error {
	if r.ID != "tool.obsidian" || r.Bindings["linux"] != (Binding{Provider: "flatpak", Package: "md.obsidian.Obsidian"}) || !slices.Contains([]string{"x86_64", "aarch64"}, d.Arch) || d.Query == nil {
		return errors.New("invalid fixed Flatpak application")
	}
	for _, path := range []string{d.Directory, d.WorkerDirectory, d.Home} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("Flatpak requires canonical absolute paths")
		}
	}
	for _, path := range []string{d.ownerPath(r.ID), d.intentPath(strings.Repeat("a", 64)), filepath.Join(d.store(r.ID), "repo")} {
		bound, err := bindConfigDestination(path)
		if err != nil || bound != path {
			return errors.Join(errors.New("Flatpak installation parent was redirected"), err)
		}
	}
	return nil
}
func (d *FlatpakDriver) query(ctx context.Context, r Resource, args ...string) (string, error) {
	out, err := d.Query(ctx, d.command(r, args...))
	return strings.TrimSpace(string(out)), err
}

type flatpakCommandFailure struct{ message string }

func (e *flatpakCommandFailure) Error() string { return e.message }

func (d *FlatpakDriver) result(command nativeCommand) error {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return err
	}
	if err := Decode(data, &record); err != nil {
		return err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash {
		return errors.Join(errors.New("Flatpak native command evidence changed"), err)
	}
	if record.Reply == nil {
		return errors.New("Flatpak command outcome is unknown; preserve the installation for inspection")
	}
	if record.Reply.Error != "" || record.Reply.ExitCode == nil || *record.Reply.ExitCode != 0 || record.Reply.Ready {
		return &flatpakCommandFailure{fmt.Sprintf("Flatpak command failed; preserve its saved operation%s", nativeDiagnostic(record.Reply.Output))}
	}
	return nil
}
func (d *FlatpakDriver) steps(r Resource, in flatpakIntent) []nativeCommand {
	var commands []nativeCommand
	if in.Directory == "" {
		commands = append(commands, nativeCommand{Program: "/bin/mkdir", Arguments: []string{"-m", "0700", d.store(r.ID)}})
	}
	switch in.Action {
	case "install":
		commands = append(commands, d.command(r, "remote-add", "--user", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo"), d.command(r, "install", "--user", "--noninteractive", "--assumeyes", "flathub", d.ref(r)))
	case "update":
		commands = append(commands, d.command(r, "update", "--user", "--noninteractive", "--assumeyes", d.ref(r)))
	case "repair":
		commands = append(commands, d.command(r, "install", "--user", "--reinstall", "--noninteractive", "--assumeyes", "flathub", d.ref(r)))
	case "remove":
		commands = append(commands, d.command(r, "uninstall", "--user", "--noninteractive", "--assumeyes", d.ref(r)))
	}
	for i := range commands {
		// Earlier saved commands used the ordinary dependency search path.
		// Preserve their evidence exactly; every new operation is isolated.
		if !in.PrivateDependencies && commands[i].Program == "/usr/bin/flatpak" {
			commands[i].Environment = commands[i].Environment[:4]
		}
		commands[i].Operation, _ = digest(struct {
			Operation string
			Step      int
		}{in.Operation, i})
		if len(in.Attempts) > i && in.Attempts[i] > 0 {
			commands[i].Operation, _ = digest(struct {
				Operation     string
				Step, Attempt int
			}{in.Operation, i, in.Attempts[i]})
		}
	}
	return commands
}
func (d *FlatpakDriver) intent(r Resource, op string) (flatpakIntent, error) {
	var in flatpakIntent
	if !operationID.MatchString(op) {
		return in, errors.New("invalid Flatpak operation")
	}
	data, err := readDocument(d.intentPath(op))
	if err != nil {
		return in, err
	}
	if err := Decode(data, &in); err != nil {
		return in, err
	}
	if in.Schema != 1 || in.Operation != op || in.Resource != r.ID || !slices.Contains([]string{"install", "update", "repair", "remove"}, in.Action) || in.Directory == "" && in.Action != "install" {
		return in, errors.New("invalid saved Flatpak intent")
	}
	if len(in.Attempts) != 0 && len(in.Attempts) != len(in.Commands) {
		return in, errors.New("invalid Flatpak attempts")
	}
	for _, attempt := range in.Attempts {
		if attempt < 0 || attempt > 16 {
			return in, errors.New("Flatpak recovery attempt limit")
		}
	}
	expected, err := digest(d.steps(r, in))
	if err != nil {
		return in, err
	}
	actual, err := digest(in.Commands)
	if err != nil || expected != actual {
		return in, errors.New("saved Flatpak commands differ from the fixed recipe")
	}
	return in, nil
}
func (d *FlatpakDriver) owner(r Resource) (flatpakOwner, error) {
	var owner flatpakOwner
	data, err := readDocument(d.ownerPath(r.ID))
	if err != nil {
		return owner, err
	}
	if err := Decode(data, &owner); err != nil {
		return owner, err
	}
	if owner.Schema != 1 || owner.Directory == "" {
		return owner, errors.New("invalid Flatpak directory ownership")
	}
	in, err := d.intent(r, owner.CreatedBy)
	if err != nil {
		return owner, err
	}
	if in.Directory != "" || in.Action != "install" {
		return owner, errors.New("Flatpak directory lacks an exclusive reservation")
	}
	if err := d.result(in.Commands[0]); err != nil {
		return owner, err
	}
	identity, err := homebrewBootstrapDirectory(d.store(r.ID))
	if err != nil || identity != owner.Directory {
		return owner, errors.Join(errors.New("Flatpak installation directory changed"), err)
	}
	return owner, nil
}
func (d *FlatpakDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o := Observation{Provider: "flatpak", Scope: "user", Identity: d.store(r.ID) + ":" + d.ref(r), ApplyDetail: "Flathub stable in a private per-user installation; runtimes, app data and documents are retained on removal"}
	if err := d.validate(r); err != nil {
		return o, err
	}
	if _, err := os.Lstat(d.store(r.ID)); errors.Is(err, os.ErrNotExist) {
		return o, nil
	} else if err != nil {
		return o, err
	}
	o.Preserved = []string{d.store(r.ID), filepath.Join(d.Home, ".var", "app", r.Bindings["linux"].Package)}
	owner, ownerErr := d.owner(r)
	if ownerErr != nil {
		// An unfinished exclusive reservation is recoverable only via its exact
		// worker record, never by claiming an already existing directory.
		if receipt.Status != "in-progress" {
			return o, ownerErr
		}
	}
	list, err := d.query(ctx, r, "list", "--user", "--app", "--columns=ref")
	if err != nil {
		return o, err
	}
	o.Present = slices.Contains(strings.Fields(list), strings.TrimPrefix(d.ref(r), "app/"))
	if o.Present {
		origin, err := d.query(ctx, r, "info", "--user", "--show-origin", d.ref(r))
		if err != nil {
			return o, err
		}
		commit, err := d.query(ctx, r, "info", "--user", "--show-commit", d.ref(r))
		if err != nil {
			return o, err
		}
		if origin != "flathub" || !operationID.MatchString(commit) {
			return o, errors.New("Flatpak application changed its source")
		}
		location := filepath.Join(d.store(r.ID), "app", r.Bindings["linux"].Package, d.Arch, "stable", commit)
		actual, err := d.query(ctx, r, "info", "--user", "--show-location", d.ref(r))
		if err != nil || actual != location {
			return o, errors.Join(errors.New("Flatpak deployment location changed"), err)
		}
		files, err := snapshotTree(location, maxPackageBytes, maxPackageEntries)
		if err != nil {
			return o, err
		}
		config, err := snapshotConfig(filepath.Join(d.store(r.ID), "repo", "config"))
		if err != nil {
			return o, err
		}
		o.Version = commit
		o.Fingerprint, err = digest(struct {
			Directory, Commit string
			Files, Config     configSnapshot
		}{owner.Directory, commit, files, config})
		if err != nil {
			return o, err
		}
		o.Healthy = files.Kind == "directory"
		runtimeRef, err := d.query(ctx, r, "info", "--user", "--show-runtime", d.ref(r))
		if err != nil {
			return o, err
		}
		runtimes, err := d.query(ctx, r, "list", "--user", "--runtime", "--columns=ref")
		if err != nil {
			return o, err
		}
		if runtimeRef == "" || !slices.Contains(strings.Fields(runtimes), runtimeRef) {
			o.Healthy = false
			o.HealthIssue = "the application's private Flatpak runtime is missing; repair the application"
		}
	}
	if o.Present && receipt.Ownership == "created" && receipt.After.Fingerprint != "" && receipt.After.Fingerprint != o.Fingerprint && receipt.Status != "in-progress" {
		o.Healthy = false
		o.HealthIssue = "Flatpak deployment or remote configuration changed; preserve it for inspection"
	}
	if receipt.OperationID != "" {
		in, err := d.intent(r, receipt.OperationID)
		if errors.Is(err, os.ErrNotExist) {
			return o, nil
		}
		if err != nil {
			return o, err
		}
		if in.Complete {
			for _, command := range in.Commands {
				if err := d.result(command); err != nil {
					return o, err
				}
			}
			if in.Action == "remove" && !o.Present || in.Action != "remove" && o.Healthy && in.Fingerprint == o.Fingerprint {
				o.CompletedOperation = in.Operation
			}
		} else if receipt.Status == "in-progress" {
			o.Pending = "resume the saved Flatpak operation"
			token, err := digest(in)
			if err != nil {
				return o, err
			}
			o.ResourceResume = &ResourceResume{Operation: in.Operation, Token: token}
		}
	}
	return o, nil
}
func (d *FlatpakDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}
func (d *FlatpakDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}
func (d *FlatpakDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}
func (d *FlatpakDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) {
		return Observation{}, errors.New("Flatpak mutation requires its locked worker and saved operation")
	}
	in, err := d.intent(r, receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		if !slices.Contains([]string{"install", "update", "repair", "remove"}, action) {
			return Observation{}, errors.New("invalid Flatpak action")
		}
		in = flatpakIntent{Schema: 1, PrivateDependencies: true, Operation: receipt.OperationID, Resource: r.ID, Action: action}
		if _, err := os.Lstat(d.store(r.ID)); !errors.Is(err, os.ErrNotExist) {
			owner, err := d.owner(r)
			if err != nil {
				return Observation{}, err
			}
			in.Directory = owner.Directory
		} else if action != "install" {
			return Observation{}, errors.New("Flatpak installation disappeared")
		}
		current, err := d.Observe(ctx, r, Receipt{})
		if err != nil {
			return Observation{}, err
		}
		if action == "install" && current.Present || action != "install" && (receipt.Ownership != "created" || !current.Present || !sameArtifact(current, receipt.After)) {
			return Observation{}, errors.New("Flatpak application changed since approval")
		}
		in.Commands = d.steps(r, in)
		if err := os.MkdirAll(filepath.Dir(d.store(r.ID)), 0700); err != nil {
			return Observation{}, err
		}
		if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if in.Action != action {
		return Observation{}, errors.New("Flatpak recovery differs from its saved action")
	}
	if in.Complete {
		if err := d.finishOwner(r, in); err != nil {
			return Observation{}, err
		}
		return d.Observe(ctx, r, receipt)
	}
	for index, command := range in.Commands {
		resultErr := d.result(command)
		var failed *flatpakCommandFailure
		if errors.As(resultErr, &failed) && resume && !(index == 0 && in.Directory == "") {
			if len(in.Attempts) == 0 {
				in.Attempts = make([]int, len(in.Commands))
			}
			if in.Attempts[index] >= 16 {
				return Observation{}, errors.New("Flatpak recovery attempt limit reached")
			}
			in.Attempts[index]++
			in.Commands = d.steps(r, in)
			command = in.Commands[index]
			if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
				return Observation{}, err
			}
			resultErr = d.result(command)
		}
		if err := resultErr; errors.Is(err, os.ErrNotExist) {
			if index == 0 && in.Directory == "" {
				if _, err := os.Lstat(d.store(r.ID)); !errors.Is(err, os.ErrNotExist) {
					return Observation{}, errors.New("Flatpak directory appeared before exclusive reservation")
				}
			} else if _, err := d.owner(r); err != nil {
				return Observation{}, err
			}
			if _, err := d.Run(ctx, command); err != nil {
				return Observation{}, err
			}
			if err := d.result(command); err != nil {
				return Observation{}, err
			}
		} else if err != nil {
			return Observation{}, err
		}
		if index == 0 && in.Directory == "" {
			if _, err := d.owner(r); errors.Is(err, os.ErrNotExist) {
				identity, err := homebrewBootstrapDirectory(d.store(r.ID))
				if err != nil {
					return Observation{}, err
				}
				if err := saveDocument(d.ownerPath(r.ID), flatpakOwner{Schema: 1, CreatedBy: in.Operation, Directory: identity}); err != nil {
					return Observation{}, err
				}
			} else if err != nil {
				return Observation{}, err
			}
		}
	}
	o, err := d.Observe(ctx, r, Receipt{})
	if err != nil {
		return o, err
	}
	if action == "remove" && o.Present || action != "remove" && !o.Healthy {
		return o, errors.New("Flatpak did not reach the requested native state")
	}
	in.Fingerprint, in.Complete = o.Fingerprint, true
	if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
		return o, err
	}
	if err := d.finishOwner(r, in); err != nil {
		return o, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *FlatpakDriver) finishOwner(r Resource, in flatpakIntent) error {
	for _, command := range in.Commands {
		if err := d.result(command); err != nil {
			return err
		}
	}
	owner, err := d.owner(r)
	if err != nil {
		return err
	}
	owner.ApplicationOperation = in.Operation
	return saveDocument(d.ownerPath(r.ID), owner)
}

func (d *FlatpakDriver) verifiedApplication(ctx context.Context) (Observation, error) {
	r := Resource{ID: "tool.obsidian", Name: "Obsidian", Bindings: map[string]Binding{"linux": {Provider: "flatpak", Package: "md.obsidian.Obsidian"}}}
	owner, err := d.owner(r)
	if err != nil {
		return Observation{}, err
	}
	op := owner.ApplicationOperation
	if op == "" {
		op = owner.CreatedBy
	} // first-install legacy record
	in, err := d.intent(r, op)
	if err != nil {
		return Observation{}, err
	}
	if !in.Complete || in.Action == "remove" {
		return Observation{}, errors.New("Flatpak application installation is incomplete")
	}
	return d.Observe(ctx, r, Receipt{Ownership: "created", Status: "complete", OperationID: op, After: Observation{Fingerprint: in.Fingerprint}})
}
