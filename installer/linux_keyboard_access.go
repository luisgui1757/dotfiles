package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Mac keyboard shortcuts on Linux run xremap as the logged-in user. It reads
// keyboards and creates a virtual one. By owner choice (2026-10-21) one udev
// rule tags those devices for logind's seat ACLs (uaccess) instead of adding the
// account to the input group: only the user at the local screen gets access,
// never an SSH session. It is the only root-owned file setup writes outside a
// package. It is linked into place without replacing anything, names its
// operation, and is removed only while its bytes are exactly the installed ones.
const keyboardAccessRules = "/etc/udev/rules.d/70-dotfiles-mac-keyboard.rules"

// uaccess must be tagged before 73-seat-late.rules applies the seat ACLs.
const keyboardAccessBody = `KERNEL=="uinput", SUBSYSTEM=="misc", TAG+="uaccess", OPTIONS+="static_node=uinput"
SUBSYSTEM=="input", KERNEL=="event*", ENV{ID_INPUT_KEYBOARD}=="1", TAG+="uaccess"
`

const keyboardAccessHeader = "# Managed by dotfiles (Mac keyboard shortcuts), operation "

var keyboardAccessOperation = regexp.MustCompile(`\A` + regexp.QuoteMeta(keyboardAccessHeader) + `([0-9a-f]{64})\.\n`)

func keyboardAccessContent(operation string) []byte {
	return []byte(keyboardAccessHeader + operation + ".\n" +
		"# Lets the user logged in at this seat read keyboards and create a virtual\n" +
		"# keyboard for the xremap key remapper. Unselect the tool to remove it.\n" + keyboardAccessBody)
}

// keyboardAccessOwner returns the operation that wrote these exact bytes.
func keyboardAccessOwner(data []byte) (string, bool) {
	match := keyboardAccessOperation.FindSubmatch(data)
	if match == nil || !bytes.Equal(data, keyboardAccessContent(string(match[1]))) {
		return "", false
	}
	return string(match[1]), true
}

type keyboardAccessPrograms struct{ Install, Link, Remove, Move, Udevadm string }

var defaultKeyboardAccessPrograms = keyboardAccessPrograms{Install: "/usr/bin/install", Link: "/usr/bin/ln", Remove: "/usr/bin/rm", Move: "/usr/bin/mv", Udevadm: "/usr/bin/udevadm"}

type KeyboardAccessDriver struct {
	Directory, WorkerDirectory string
	// Sudo is empty when the controller already runs as root.
	Sudo     string
	Rules    string
	Programs keyboardAccessPrograms
	// Owner is the account that must own the rule: root, except in fixtures.
	Owner        int
	Issue        string
	Run          func(context.Context, nativeCommand) ([]byte, error)
	Authenticate func(context.Context) error
}

type keyboardAccessIntent struct {
	Schema    int    `json:"schema"`
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
	Action    string `json:"action"`
	// Expected is the installed fingerprint a removal may delete.
	Expected string          `json:"expected,omitempty"`
	Commands []nativeCommand `json:"commands"`
	Attempts []int           `json:"attempts,omitempty"`
	Complete bool            `json:"complete,omitempty"`
}

func configureKeyboardAccess(apt *LinuxAPTLocation, issue, stateDirectory string, session *nativeSession) *KeyboardAccessDriver {
	d := &KeyboardAccessDriver{Directory: filepath.Join(stateDirectory, "native", "keyboard-access"), WorkerDirectory: session.Directory,
		Rules: keyboardAccessRules, Programs: defaultKeyboardAccessPrograms, Run: session.run}
	switch {
	case apt == nil:
		d.Issue = "keyboard access needs the system administrator tools that APT setup uses"
		if issue != "" {
			d.Issue += ": " + issue
		}
	case !apt.root:
		d.Sudo = apt.Sudo
	}
	return d
}

func (d *KeyboardAccessDriver) intentPath(op string) string {
	return filepath.Join(d.Directory, "operations", op+".json")
}
func (d *KeyboardAccessDriver) stagePath(op string) string {
	return filepath.Join(d.Directory, "inputs", op+".rules")
}

// Neither name ends in .rules, so udev never reads a partial file.
func (d *KeyboardAccessDriver) sibling(op, suffix string) string {
	return filepath.Join(filepath.Dir(d.Rules), ".dotfiles-mac-keyboard-"+op[:16]+suffix)
}

func (d *KeyboardAccessDriver) validate(r Resource) error {
	if r.ID != "system.keyboard-access" {
		return errors.New("invalid keyboard access resource")
	}
	for _, path := range []string{d.Directory, d.WorkerDirectory, d.Rules, d.Programs.Install, d.Programs.Link, d.Programs.Remove, d.Programs.Move, d.Programs.Udevadm} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("keyboard access requires canonical absolute paths")
		}
	}
	if d.Sudo != "" && !filepath.IsAbs(d.Sudo) {
		return errors.New("keyboard access requires the observed absolute sudo")
	}
	return nil
}

func (d *KeyboardAccessDriver) command(program string, args ...string) nativeCommand {
	if d.Sudo == "" {
		return nativeCommand{Program: program, Arguments: args, Environment: []string{"LC_ALL=C"}}
	}
	return nativeCommand{Program: d.Sudo, Arguments: append([]string{"-n", "--", program}, args...), Environment: []string{"LC_ALL=C"}}
}

func (d *KeyboardAccessDriver) steps(in keyboardAccessIntent) []nativeCommand {
	p := d.Programs
	commands := []nativeCommand{}
	switch in.Action {
	case "install":
		tmp := d.sibling(in.Operation, ".tmp")
		commands = append(commands,
			d.command(p.Install, "-m", "0644", "-o", "root", "-g", "root", "-T", d.stagePath(in.Operation), tmp),
			// A hard link never replaces an existing rule.
			d.command(p.Link, "-T", tmp, d.Rules),
			d.command(p.Remove, "-f", "--", tmp))
	case "remove":
		aside := d.sibling(in.Operation, ".removing")
		commands = append(commands, d.command(p.Move, "-T", d.Rules, aside), d.command(p.Remove, "-f", "--", aside))
	}
	commands = append(commands,
		d.command(p.Udevadm, "control", "--reload-rules"),
		d.command(p.Udevadm, "trigger", "--action=change", "--subsystem-match=misc", "--sysname-match=uinput"),
		d.command(p.Udevadm, "trigger", "--action=change", "--subsystem-match=input"))
	for i := range commands {
		attempt := 0
		if len(in.Attempts) > i {
			attempt = in.Attempts[i]
		}
		commands[i].Operation, _ = digest(struct {
			Operation     string
			Step, Attempt int
		}{in.Operation, i, attempt})
	}
	return commands
}

func (d *KeyboardAccessDriver) intent(r Resource, op string) (keyboardAccessIntent, error) {
	var in keyboardAccessIntent
	if !operationID.MatchString(op) {
		return in, errors.New("invalid keyboard access operation")
	}
	data, err := readDocument(d.intentPath(op))
	if err != nil {
		return in, err
	}
	if err := Decode(data, &in); err != nil {
		return in, err
	}
	if in.Schema != 1 || in.Operation != op || in.Resource != r.ID || !slices.Contains([]string{"install", "remove"}, in.Action) || in.Action == "remove" && in.Expected == "" {
		return in, errors.New("invalid saved keyboard access intent")
	}
	if len(in.Attempts) != 0 && len(in.Attempts) != len(in.Commands) {
		return in, errors.New("invalid keyboard access attempts")
	}
	for _, attempt := range in.Attempts {
		if attempt < 0 || attempt > 16 {
			return in, errors.New("keyboard access recovery attempt limit")
		}
	}
	expected, err := digest(d.steps(in))
	if err != nil {
		return in, err
	}
	actual, err := digest(in.Commands)
	if err != nil || expected != actual {
		return in, errors.New("saved keyboard access commands differ from the fixed recipe")
	}
	return in, nil
}

// read inspects one root-owned rule without following links.
func (d *KeyboardAccessDriver) read(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, info, fmt.Errorf("%s is not a small regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, info, err
	}
	data, readErr := io.ReadAll(io.LimitReader(f, 4097))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, info, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || int64(len(data)) != info.Size() {
		return nil, info, errors.Join(fmt.Errorf("%s changed while it was read", path), err)
	}
	return data, info, nil
}

func (d *KeyboardAccessDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	// The rule serves only this capability, so it is not a shared machine
	// resource kept after deselection; changing it still needs sudo.
	o := Observation{Provider: "keyboard-access", Scope: "user", Privileged: true, Identity: d.Rules,
		ApplyDetail: "udev rule " + d.Rules + ": keyboards and the virtual input device for the user logged in at this screen"}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	if err := d.validate(r); err != nil {
		return o, err
	}
	var err error
	if o.Desired, err = digest(struct{ Rules, Body string }{d.Rules, keyboardAccessBody}); err != nil {
		return o, err
	}
	if d.Issue != "" {
		o.Unknown, o.Pending = true, d.Issue
		return o, nil
	}
	data, info, err := d.read(d.Rules)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		o.Unknown, o.Pending = true, "cannot inspect "+d.Rules+"; nothing was changed: "+err.Error()
		return o, nil
	default:
		o.Present, o.Fingerprint = true, sha256Hex(data)
		_, ours := keyboardAccessOwner(data)
		owner, _, mode, ownerErr := fileOwner(d.Rules)
		o.Healthy = ours && ownerErr == nil && int(owner) == d.Owner && mode == 0644 && info.Mode().Perm() == 0644
		if !o.Healthy {
			o.Pending = d.Rules + " exists but is not the exact rule dotfiles installs; dotfiles never replaces or removes it. Inspect it, and remove it yourself to let setup install its rule"
		}
	}
	if o.Present && receipt.Ownership == "created" && receipt.After.Fingerprint != "" && receipt.After.Fingerprint != o.Fingerprint && receipt.Status != "in-progress" {
		o.Healthy, o.Pending = false, ""
		o.HealthIssue = d.Rules + " changed after setup installed it; it is preserved for inspection"
	}
	if receipt.OperationID == "" {
		return o, nil
	}
	in, err := d.intent(r, receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		o.Unknown, o.Pending = true, "saved keyboard access operation "+d.intentPath(receipt.OperationID)+" is unreadable; preserve it: "+err.Error()
		return o, nil
	}
	if in.Complete {
		if in.Action == "remove" && !o.Present || in.Action == "install" && o.Healthy {
			o.CompletedOperation = in.Operation
		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "resume the saved keyboard access operation"
		token, err := digest(in)
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: in.Operation, Token: token}
	}
	return o, nil
}

func (d *KeyboardAccessDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}
func (d *KeyboardAccessDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}
func (d *KeyboardAccessDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	in, err := d.intent(r, receipt.OperationID)
	if err != nil {
		return Observation{}, err
	}
	return d.change(ctx, r, in.Action, receipt, true)
}

func (d *KeyboardAccessDriver) result(command nativeCommand) error {
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
		return errors.Join(errors.New("keyboard access command evidence changed"), err)
	}
	if record.Reply == nil {
		return errors.New("keyboard access command outcome is unknown; inspect " + d.Rules + " before retrying")
	}
	if record.Reply.Error != "" || record.Reply.ExitCode == nil || *record.Reply.ExitCode != 0 || record.Reply.Ready {
		return &flatpakCommandFailure{fmt.Sprintf("keyboard access command failed%s", nativeDiagnostic(record.Reply.Output))}
	}
	return nil
}

func (d *KeyboardAccessDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	if d.Issue != "" {
		return Observation{}, errors.New(d.Issue)
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) {
		return Observation{}, errors.New("keyboard access requires its locked worker and saved operation")
	}
	in, err := d.intent(r, receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		in = keyboardAccessIntent{Schema: 1, Operation: receipt.OperationID, Resource: r.ID, Action: action}
		current, err := d.Observe(ctx, r, Receipt{})
		if err != nil {
			return Observation{}, err
		}
		switch {
		case action == "install":
			if current.Present || current.Unknown {
				return Observation{}, errors.New(d.Rules + " changed since approval; review a fresh plan")
			}
			if err := os.MkdirAll(filepath.Dir(d.stagePath(in.Operation)), 0700); err != nil {
				return Observation{}, err
			}
			if err := writeIntegrationInput(d.stagePath(in.Operation), keyboardAccessContent(in.Operation)); err != nil {
				return Observation{}, err
			}
		case action == "remove":
			if receipt.Ownership != "created" || !current.Present || current.Fingerprint != receipt.After.Fingerprint {
				return Observation{}, errors.New(d.Rules + " is not the rule this setup installed; it is preserved")
			}
			in.Expected = current.Fingerprint
		default:
			return Observation{}, errors.New("the keyboard access rule cannot be repaired in place; remove it yourself if it changed, then apply again")
		}
		in.Commands = d.steps(in)
		if err := os.MkdirAll(filepath.Dir(d.intentPath(in.Operation)), 0700); err != nil {
			return Observation{}, err
		}
		if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if in.Action != action {
		return Observation{}, errors.New("keyboard access recovery differs from its saved action")
	}
	if !in.Complete {
		for index := range in.Commands {
			if err := d.dispatch(ctx, &in, index, resume); err != nil {
				return Observation{}, err
			}
		}
		o, err := d.Observe(ctx, r, Receipt{})
		if err != nil {
			return o, err
		}
		if action == "remove" && o.Present {
			return o, errors.New("the keyboard access rule is still present after removal")
		}
		if action == "install" {
			data, _, err := d.read(d.Rules)
			if owner, ok := keyboardAccessOwner(data); err != nil || !ok || owner != in.Operation || !o.Healthy {
				return o, errors.Join(errors.New("the keyboard access rule is not the one this operation installed"), err)
			}
		}
		in.Complete = true
		if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
			return o, err
		}
		if err := os.Remove(d.stagePath(in.Operation)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return o, err
		}
	}
	return d.Observe(ctx, r, receipt)
}

// dispatch runs one saved command once. A removal checks the exact installed
// bytes immediately before moving the rule aside and before deleting it.
func (d *KeyboardAccessDriver) dispatch(ctx context.Context, in *keyboardAccessIntent, index int, resume bool) error {
	command := in.Commands[index]
	err := d.result(command)
	var failed *flatpakCommandFailure
	if errors.As(err, &failed) && resume {
		if len(in.Attempts) == 0 {
			in.Attempts = make([]int, len(in.Commands))
		}
		if in.Attempts[index] >= 16 {
			return errors.New("keyboard access recovery attempt limit reached")
		}
		in.Attempts[index]++
		in.Commands = d.steps(*in)
		command = in.Commands[index]
		if err := saveDocument(d.intentPath(in.Operation), *in); err != nil {
			return err
		}
		err = d.result(command)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if in.Action == "remove" && index < 2 {
		path := d.Rules
		if index == 1 {
			path = d.sibling(in.Operation, ".removing")
		}
		data, _, readErr := d.read(path)
		if index == 1 && errors.Is(readErr, os.ErrNotExist) {
			// The moved rule is already gone: nothing is left to delete.
		} else if readErr != nil || sha256Hex(data) != in.Expected {
			return errors.Join(errors.New("the keyboard access rule changed during removal; preserve "+path), readErr)
		}
	}
	if _, err := d.Run(ctx, command); err != nil {
		return err
	}
	return d.result(command)
}
