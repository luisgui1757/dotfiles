package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Saved steps are data. Each kind has fixed semantics below; commands are
// fixed system programs with reviewed arguments, never decoded shell text.
//
//   - move: rename one path aside with its inode identity; never deletes.
//   - discard: delete a bounded, recorded user-owned tree entry by entry.
//   - rmdir: remove a directory only when empty; otherwise disclose it.
//   - remove: privileged removal of one recorded file or link.
//   - edit: privileged removal of one exact line from a system file.
//   - command: one fixed native command with explicit pre/postconditions.
//   - homebrew-install: the pinned official Homebrew installer payload.
var legacyStepKinds = []string{"move", "discard", "rmdir", "remove", "edit", "command", "homebrew-install"}

type legacyRetirementStep struct {
	Kind         string                 `json:"kind"`
	Label        string                 `json:"label"`
	Privileged   bool                   `json:"privileged,omitempty"`
	Irreversible bool                   `json:"irreversible,omitempty"`
	Path         string                 `json:"path,omitempty"`
	Backup       string                 `json:"backup,omitempty"`
	Restores     bool                   `json:"restores,omitempty"`
	Identity     string                 `json:"identity,omitempty"`
	Mode         uint32                 `json:"mode,omitempty"`
	Owner        string                 `json:"owner,omitempty"`
	Group        string                 `json:"group,omitempty"`
	Content      []byte                 `json:"content,omitempty"`
	Entries      []configEntry          `json:"entries,omitempty"`
	Before       []legacyRetirementFile `json:"before,omitempty"`
	After        []legacyRetirementFile `json:"after,omitempty"`
	Absent       []string               `json:"absent,omitempty"`
	Gates        []legacyRetirementGate `json:"gates,omitempty"`
	Checks       []legacyRetirementGate `json:"checks,omitempty"`
	Command      *nativeCommand         `json:"command,omitempty"`
}

// Privileged selects POSIX sudo dispatch. Windows vendors can elevate inside
// their existing fixed command instead. Read its typed native gate and argv,
// which also exist in historical intents; never rewrite saved approval data.
// This is disclosure only and does not select a dispatch mechanism.
func legacyStepNeedsAdministrator(step legacyRetirementStep) bool {
	if step.Privileged || step.Kind == "homebrew-install" {
		return true
	}
	if step.Kind != "command" || step.Command == nil {
		return false
	}
	for _, gate := range step.Gates {
		switch gate.Kind {
		case "choco-package":
			return true
		case "winget-package":
			// The fixed recipe appends its optional native scope last. Missing
			// or unfamiliar scope cannot promise an operation without UAC.
			args := step.Command.Arguments
			return len(args) < 2 || args[len(args)-2] != "--scope" || args[len(args)-1] != "user"
		}
	}
	return false
}

// Gates are fresh native observations. Expected is the approved value; an
// empty value for a consumer gate means none may exist.
type legacyRetirementGate struct {
	Kind     string `json:"kind"`
	Subject  string `json:"subject,omitempty"`
	Expected string `json:"expected"`
}

var legacyGateKinds = append([]string{"replacements", "nix-consumers", "nix-consumer-roots", "nix-homebrew", "nix-store", "determinate", "account", "group", "account-processes", "login-shell", "launchd", "systemd-stopped", "volume", "homebrew", "brew-inventory", "brew-preservation", "brew-formula", "gh-extension", "path-identity"}, legacyDeltaGateKinds...)

const (
	legacyOutcomeNone      = "none"
	legacyOutcomeSucceeded = "succeeded"
	legacyOutcomeFailed    = "failed"
	legacyOutcomeUnknown   = "unknown"
)

type legacyOutcome struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var errLegacyOutcomeUnknown = errors.New("the saved native command has no recorded result")

// Root verifies the exact approved bytes itself; the user-owned staged input
// only supplies replacement content. The original is kept beside the file.
const legacyEditScript = `set -u
file=$1 expected=$2 staged=$3 backup=$4 mode=$5 owner=$6 group=$7 replacement=$8
umask 077
tmp=
trap 'if [ -n "$tmp" ]; then rm -f "$tmp"; fi' EXIT
file_hash() {
case $(uname -s) in
Darwin) shasum -a 256 "$1" ;;
*) sha256sum "$1" ;;
esac
}
if [ ! -f "$file" ] || [ -L "$file" ]; then exit 70; fi
sum=$(file_hash "$file") || exit 71
if [ "${sum%% *}" != "$expected" ]; then exit 72; fi
if [ "$replacement" != absent ]; then
	if [ ! -f "$staged" ] || [ -L "$staged" ]; then exit 78; fi
	tmp=$(mktemp "$file.dotfiles.XXXXXX") || exit 75
	cat "$staged" > "$tmp" || exit 76
	sum=$(file_hash "$tmp") || exit 78
	if [ "${sum%% *}" != "$replacement" ]; then exit 78; fi
	chown "$owner:$group" "$tmp" || exit 76
	chmod "$mode" "$tmp" || exit 76
fi
if [ -e "$backup" ] || [ -L "$backup" ]; then
	if [ ! -f "$backup" ] || [ -L "$backup" ]; then exit 73; fi
	sum=$(file_hash "$backup") || exit 73
	if [ "${sum%% *}" != "$expected" ]; then exit 73; fi
else
	cp -p "$file" "$backup" || exit 74
fi
if [ "$replacement" != absent ]; then
	mv -f "$tmp" "$file" || exit 76
	tmp=
else
	rm -f "$file" || exit 77
fi
`

// Root's Nix files are unreadable to the controller. Each fixed name has its
// own native outcome and adjacent backup; one failed move cannot replay another.
const legacyRootFilesScript = `set -u
source=$1 target=$2
if [ ! -e "$source" ] && [ ! -L "$source" ]; then exit 0; fi
if [ -e "$target" ] || [ -L "$target" ]; then exit 80; fi
mv "$source" "$target" || exit 81
`

func legacyCanonicalPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != filepath.VolumeName(path)+string(filepath.Separator) && !strings.ContainsAny(path, "\x00\r\n")
}

func validateLegacyGate(gate legacyRetirementGate) error {
	if !slices.Contains(legacyGateKinds, gate.Kind) || len(gate.Subject) > 4096 || len(gate.Expected) > 4096 || strings.ContainsAny(gate.Subject+gate.Expected, "\x00\r\n") {
		return errors.New("invalid legacy retirement gate")
	}
	return nil
}

func (d *legacyRetirementDriver) validateStep(step legacyRetirementStep) error {
	if !slices.Contains(legacyStepKinds, step.Kind) || step.Label == "" || len(step.Label) > 1024 || len(step.Before) > 64 || len(step.After) > 64 ||
		len(step.Absent) > 64 || len(step.Gates) > 16 || len(step.Checks) > 16 || len(step.Content) > maxProfileBytes || step.Mode > 07777 {
		return errors.New("invalid legacy retirement step")
	}
	paths := slices.Clone(step.Absent)
	for _, file := range append(slices.Clone(step.Before), step.After...) {
		paths = append(paths, file.Path)
		if file.Snapshot.Kind != "absent" && !operationID.MatchString(file.Snapshot.Hash) {
			return errors.New("legacy retirement file lacks its observed fingerprint")
		}
	}
	for _, path := range paths {
		if !legacyCanonicalPath(path) {
			return errors.New("invalid legacy retirement path")
		}
	}
	for _, gate := range append(slices.Clone(step.Gates), step.Checks...) {
		if err := validateLegacyGate(gate); err != nil {
			return err
		}
	}
	plain := step.Command == nil && len(step.Entries) == 0 && len(step.Content) == 0 && step.Owner == "" && step.Group == ""
	valid := false
	switch step.Kind {
	case "move":
		valid = plain && legacyCanonicalPath(step.Path) && legacyCanonicalPath(step.Backup) && step.Identity != "" && step.Identity != "absent"
	case "discard":
		valid = step.Command == nil && !step.Privileged && legacyCanonicalPath(step.Path) && validateCleanupEntries(step.Entries, maxConfigEntries) == nil && slices.Contains(step.Absent, step.Path)
	case "rmdir":
		valid = plain && legacyCanonicalPath(step.Path)
	case "remove":
		valid = plain && step.Privileged && legacyCanonicalPath(step.Path) && len(step.Before) == 1 && step.Before[0].Path == step.Path &&
			(step.Before[0].Snapshot.Kind == "file" || step.Before[0].Snapshot.Kind == "link") && slices.Contains(step.Absent, step.Path)
	case "edit":
		_, ownerErr := strconv.ParseUint(step.Owner, 10, 32)
		_, groupErr := strconv.ParseUint(step.Group, 10, 32)
		valid = step.Command == nil && len(step.Entries) == 0 && step.Privileged && ownerErr == nil && groupErr == nil && operationID.MatchString(step.Identity) &&
			legacyCanonicalPath(step.Path) && legacyCanonicalPath(step.Backup) && len(step.Before) == 1 && step.Before[0].Path == step.Path && step.Before[0].Snapshot.Kind == "file"
	case "command":
		valid = step.Command != nil && step.Path == "" && len(step.Entries) == 0 && step.Command.Operation == "" && step.Command.validate() == nil &&
			(step.Backup == "" || legacyCanonicalPath(step.Backup))
	case "homebrew-install":
		valid = plain && !step.Privileged && legacyCanonicalPath(step.Path)
	}
	if !valid {
		return fmt.Errorf("invalid legacy retirement %s step", step.Kind)
	}
	return nil
}

func legacyNativeStep(step legacyRetirementStep) bool {
	return step.Kind == "remove" || step.Kind == "edit" || step.Kind == "command" || step.Kind == "homebrew-install" || step.Privileged && (step.Kind == "move" || step.Kind == "rmdir")
}

// Fixed system programs run with a minimal environment. Privileged commands
// receive no user HOME; sudo is added only at dispatch.
func (d *legacyRetirementDriver) systemCommand(privileged bool, program string, arguments ...string) nativeCommand {
	values := []string{"-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	if !privileged {
		values = append(values, "HOME="+d.Folders.Home, "USER="+d.Environment.Username, "LOGNAME="+d.Environment.Username)
	}
	return nativeCommand{Program: "/usr/bin/env", Arguments: append(append(values, program), arguments...)}
}

func (d *legacyRetirementDriver) stagedEditPath(operation string, index int) string {
	return filepath.Join(d.Directory, "staged", operation, strconv.Itoa(index))
}

func (d *legacyRetirementDriver) homebrewBootstrap() *HomebrewBootstrapDriver {
	return &HomebrewBootstrapDriver{Directory: filepath.Join(d.Directory, "homebrew"), Home: d.Folders.Home, recipe: homebrewBootstrapRecipe{homebrewBootstrapURL, homebrewBootstrapSHA256}}
}

// stepCommand is a pure function of the saved intent and attempt. Recovery
// therefore reconstructs exactly the command whose native record it reads.
// A homebrew-install command needs its prepared payload; absence means the
// command was never dispatched.
func (d *legacyRetirementDriver) stepCommand(intent legacyRetirementIntent, index int) (nativeCommand, error) {
	step := intent.Scope.Steps[index]
	id, err := digest(struct {
		Operation     string
		Step, Attempt int
	}{intent.Operation, index, intent.Attempts[index]})
	if err != nil {
		return nativeCommand{}, err
	}
	var command nativeCommand
	switch step.Kind {
	case "command":
		command = *step.Command
	case "move":
		command = d.systemCommand(true, "/bin/mv", "-n", "--", step.Path, step.Backup)
	case "rmdir":
		command = d.systemCommand(true, "/bin/rmdir", "--", step.Path)
	case "remove":
		command = d.systemCommand(true, "/bin/rm", "-f", "--", step.Path)
	case "edit":
		replacement := "absent"
		if len(step.Content) != 0 {
			sum := sha256.Sum256(step.Content)
			replacement = hex.EncodeToString(sum[:])
		}
		command = d.systemCommand(true, "/bin/sh", "-c", legacyEditScript, "sh", step.Path, step.Identity, d.stagedEditPath(intent.Operation, index), step.Backup, strconv.FormatUint(uint64(step.Mode), 8), step.Owner, step.Group, replacement)
	case "homebrew-install":
		bootstrap := d.homebrewBootstrap()
		payload, err := bootstrap.payload(intent.Operation, bootstrap.recipe)
		if err != nil {
			return nativeCommand{}, err
		}
		command = nativeCommand{Program: "/usr/bin/env", Arguments: append(bootstrap.environment(), "/bin/bash", "--noprofile", "--norc", "-s", "--"), Input: payload}
	default:
		return nativeCommand{}, fmt.Errorf("%s is not a native retirement step", step.Kind)
	}
	command.Operation = id
	if step.Privileged && !d.Environment.Root {
		command.Arguments = append([]string{"-n", "--", command.Program}, command.Arguments...)
		command.Program = "/usr/bin/sudo"
	}
	return command, command.validate()
}

func (d *legacyRetirementDriver) savedReply(command nativeCommand) (nativeReply, error) {
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	var record nativeCommandRecord
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash {
		return nativeReply{}, errors.Join(errors.New("saved native record differs from the approved retirement step"), err)
	}
	if record.Reply == nil {
		return nativeReply{}, errLegacyOutcomeUnknown
	}
	if record.Reply.Ready || record.Reply.ExitCode == nil && record.Reply.Error == "" || len(record.Reply.Output) > 1<<20 {
		return nativeReply{}, errors.New("invalid saved native retirement result")
	}
	return *record.Reply, nil
}

func legacyReplySucceeded(reply nativeReply) bool {
	return reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0
}

func legacyStepFailure(reply nativeReply) error {
	code := "unknown"
	if reply.ExitCode != nil {
		code = strconv.Itoa(*reply.ExitCode)
	}
	return &nativeCommandError{Message: fmt.Sprintf("native retirement command failed (exit %s): %s%s", code, reply.Error, nativeDiagnostic(reply.Output)), ExitCode: reply.ExitCode}
}

func (d *legacyRetirementDriver) stepOutcome(intent legacyRetirementIntent, index int) (legacyOutcome, error) {
	if !legacyNativeStep(intent.Scope.Steps[index]) {
		return legacyOutcome{Status: legacyOutcomeNone}, nil
	}
	command, err := d.stepCommand(intent, index)
	if errors.Is(err, os.ErrNotExist) {
		return legacyOutcome{Status: legacyOutcomeNone}, nil
	}
	if err != nil {
		return legacyOutcome{}, err
	}
	reply, err := d.savedReply(command)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return legacyOutcome{Status: legacyOutcomeNone}, nil
	case errors.Is(err, errLegacyOutcomeUnknown):
		return legacyOutcome{Status: legacyOutcomeUnknown}, nil
	case err != nil:
		return legacyOutcome{}, err
	case legacyReplySucceeded(reply):
		return legacyOutcome{Status: legacyOutcomeSucceeded}, nil
	}
	return legacyOutcome{Status: legacyOutcomeFailed, Error: truncateLegacyText(legacyStepFailure(reply).Error(), 2048)}, nil
}

func legacyStepPaths(step legacyRetirementStep) []string {
	paths := slices.Clone(step.Absent)
	for _, file := range append(slices.Clone(step.Before), step.After...) {
		paths = append(paths, file.Path)
	}
	for _, path := range []string{step.Path, step.Backup} {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return sortedUnique(paths)
}

// The token binds recovery approval to the saved intent, the saved native
// outcome of the next step and the current identity of every path it touches.
func (d *legacyRetirementDriver) resumeToken(ctx context.Context, intent legacyRetirementIntent) (string, legacyOutcome, error) {
	outcome := legacyOutcome{Status: legacyOutcomeNone}
	current := []string{}
	if intent.Next < len(intent.Scope.Steps) {
		var err error
		outcome, err = d.stepOutcome(intent, intent.Next)
		if err != nil {
			return "", outcome, err
		}
		for _, path := range legacyStepPaths(intent.Scope.Steps[intent.Next]) {
			identity, err := legacyPathIdentity(path)
			if err != nil {
				return "", outcome, err
			}
			current = append(current, path+"="+identity)
		}
	}
	if err := ctx.Err(); err != nil {
		return "", outcome, err
	}
	replacementState, err := d.replacementState()
	if err != nil {
		return "", outcome, err
	}
	token, err := digest(struct {
		Intent           legacyRetirementIntent
		Outcome          legacyOutcome
		Current          []string
		ReplacementState string
	}{intent, outcome, current, replacementState})
	return token, outcome, err
}

func (d *legacyRetirementDriver) preserve(intent *legacyRetirementIntent, path string) error {
	if slices.Contains(intent.Preserved, path) {
		return nil
	}
	intent.Preserved = sortedUnique(append(intent.Preserved, path))
	return saveDocument(d.intentPath(intent.Operation), intent)
}

func (d *legacyRetirementDriver) executeStep(ctx context.Context, intent *legacyRetirementIntent, index int, recovery bool) error {
	step := intent.Scope.Steps[index]
	if err := d.validateStep(step); err != nil {
		return err
	}
	switch {
	case step.Kind == "discard":
		return d.executeDiscard(ctx, intent, step)
	case !legacyNativeStep(step):
		return d.executeLocal(ctx, intent, step)
	}
	return d.executeNative(ctx, intent, index, recovery)
}

func (d *legacyRetirementDriver) executeNative(ctx context.Context, intent *legacyRetirementIntent, index int, recovery bool) (result error) {
	step := intent.Scope.Steps[index]
	// The saved scope already names the intended recovery location. Report it
	// as preserved only if it actually exists, including after a failed step.
	defer func() { result = errors.Join(result, d.recordExistingBackup(ctx, intent, step)) }()
	// The saved native result is authoritative before any precondition: a
	// completed command has already changed the state those checks describe.
	command, err := d.stepCommand(*intent, index)
	newAttempt := false
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		reply, err := d.savedReply(command)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return err
		case legacyReplySucceeded(reply):
			return d.postconditions(ctx, step)
		case !recovery:
			return legacyStepFailure(reply)
		default:
			if intent.Attempts[index]+1 >= legacyAttemptLimit {
				return errors.Join(errors.New("this step exhausted its bounded recovery attempts"), legacyStepFailure(reply))
			}
			newAttempt = true
		}
	}
	if done, err := d.alreadyRetired(ctx, step); err != nil || done {
		return err
	}
	if err := d.verifyReplacementState(*intent); err != nil {
		return err
	}
	if err := d.preconditions(ctx, step); err != nil {
		return err
	}
	switch step.Kind {
	case "edit":
		if err := saveLegacyBytes(d.stagedEditPath(intent.Operation, index), step.Content); err != nil {
			return err
		}
	case "homebrew-install":
		bootstrap := d.homebrewBootstrap()
		if err := bootstrap.preparePayload(ctx, intent.Operation, bootstrap.recipe); err != nil {
			return err
		}
	}
	command, err = d.stepCommand(*intent, index)
	if err != nil {
		return err
	}
	if (step.Privileged || step.Kind == "homebrew-install") && !d.Environment.Root && d.Authenticate != nil {
		if err := d.Authenticate(ctx); err != nil {
			return err
		}
	}
	// Consumer and identity gates run after authentication, immediately before
	// the native command, so a privileged scan observes the current machine.
	if err := d.evaluateGates(ctx, step.Gates); err != nil {
		return err
	}
	if newAttempt {
		intent.Attempts[index]++
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return err
		}
		command, err = d.stepCommand(*intent, index)
		if err != nil {
			return err
		}
	}
	if _, runErr := d.Run(ctx, command); runErr != nil {
		reply, readErr := d.savedReply(command)
		if readErr != nil {
			return errors.Join(runErr, readErr)
		}
		if !legacyReplySucceeded(reply) {
			return errors.Join(runErr, legacyStepFailure(reply))
		}
		// A lost transport reply after persisted success is verified on resume.
		return runErr
	}
	return d.postconditions(ctx, step)
}

func (d *legacyRetirementDriver) recordExistingBackup(ctx context.Context, intent *legacyRetirementIntent, step legacyRetirementStep) error {
	if step.Backup == "" || step.Restores {
		return nil
	}
	_, err := os.Lstat(step.Backup)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if errors.Is(err, os.ErrPermission) && step.Privileged {
		// Root's home is intentionally unreadable to the controller. This
		// bounded query runs only after the approved native preservation step.
		_, err = d.privilegedQuery(ctx, d.systemCommand(false, "/bin/sh", "-c", `test -e "$1" || test -L "$1"`, "sh", step.Backup))
		if code, known := legacyExitCode(err); known && code == 1 {
			return nil
		}
	}
	if err != nil {
		return err
	}
	return d.preserve(intent, step.Backup)
}

func (d *legacyRetirementDriver) executeLocal(ctx context.Context, intent *legacyRetirementIntent, step legacyRetirementStep) (result error) {
	if err := d.verifyReplacementState(*intent); err != nil {
		return err
	}
	switch step.Kind {
	case "move":
		defer func() { result = errors.Join(result, d.recordExistingBackup(ctx, intent, step)) }()
		if done, err := d.alreadyRetired(ctx, step); err != nil || done {
			return err
		}
		if err := d.preconditions(ctx, step); err != nil {
			return err
		}
		if err := d.evaluateGates(ctx, step.Gates); err != nil {
			return err
		}
		if err := moveConfigExclusive(step.Path, step.Backup); err != nil {
			return err
		}
		return d.postconditions(ctx, step)
	case "rmdir":
		info, err := os.Lstat(step.Path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is no longer a plain directory; it was preserved", step.Path)
		}
		entries, err := os.ReadDir(step.Path)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			// Other files remain; disclose the directory instead of removing it.
			return d.preserve(intent, step.Path)
		}
		if err := os.Remove(step.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return fmt.Errorf("%s is not a local retirement step", step.Kind)
}

func (d *legacyRetirementDriver) executeDiscard(ctx context.Context, intent *legacyRetirementIntent, step legacyRetirementStep) error {
	if err := d.verifyReplacementState(*intent); err != nil {
		return err
	}
	if err := d.evaluateGates(ctx, step.Gates); err != nil {
		return err
	}
	preserved, err := discardRecordedTree(step.Path, step.Entries, maxConfigBytes, maxConfigEntries)
	if len(preserved) != 0 {
		intent.Preserved = sortedUnique(append(intent.Preserved, preserved...))
		saveErr := saveDocument(d.intentPath(intent.Operation), intent)
		return errors.Join(fmt.Errorf("changed or new files were preserved instead of removed: %s", strings.Join(preserved, ", ")), err, saveErr)
	}
	if err != nil {
		return err
	}
	return d.postconditions(ctx, step)
}

func (d *legacyRetirementDriver) preconditions(ctx context.Context, step legacyRetirementStep) error {
	for _, file := range step.Before {
		if err := d.verifyLegacySnapshot(ctx, file, step.Privileged); err != nil {
			return fmt.Errorf("%s changed after approval: %w", file.Path, err)
		}
	}
	if step.Kind == "move" {
		current, err := legacyPathIdentity(step.Path)
		if err != nil {
			return err
		}
		backup, err := legacyPathIdentity(step.Backup)
		if err != nil {
			return err
		}
		if current != step.Identity || backup != "absent" {
			return fmt.Errorf("%s changed after approval or its preservation path is occupied; both are preserved", step.Path)
		}
	}
	return nil
}

func (d *legacyRetirementDriver) postconditions(ctx context.Context, step legacyRetirementStep) error {
	for _, path := range step.Absent {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(fmt.Errorf("%s did not remove %s", step.Label, path), err)
		}
	}
	for _, file := range step.After {
		if err := d.verifyLegacySnapshot(ctx, file, step.Privileged); err != nil {
			return err
		}
	}
	if step.Kind == "move" {
		current, err := legacyPathIdentity(step.Path)
		if err != nil {
			return err
		}
		backup, err := legacyPathIdentity(step.Backup)
		if err != nil {
			return err
		}
		if current != "absent" || backup != step.Identity {
			return fmt.Errorf("%s was not moved intact to %s", step.Path, step.Backup)
		}
	}
	for _, check := range step.Checks {
		if check.Kind == "launchd" && check.Expected == "absent" {
			if err := d.waitLaunchdAbsent(ctx, check.Subject); err != nil {
				return err
			}
		}
	}
	return d.evaluateGates(ctx, step.Checks)
}

// State gates describe the approved original of the thing a step changes.
// Consumer gates are not original state and may need administrator rights.
var legacyStateGates = append([]string{"account", "group", "launchd", "volume", "gh-extension", "brew-formula", "path-identity", "login-shell", "nix-store"}, legacyDeltaStateGates...)

// A step whose approved outcome already holds while its approved original no
// longer does is complete without dispatch, for example a link that systemctl
// disable already removed. Steps without an explicit original and outcome
// never qualify. A failed outcome check is reported by the preconditions that
// run next, so it is not treated as completion.
func (d *legacyRetirementDriver) alreadyRetired(ctx context.Context, step legacyRetirementStep) (bool, error) {
	if step.Kind == "move" {
		return d.postconditions(ctx, step) == nil, nil
	}
	if step.Kind == "rmdir" {
		// Removing only an empty directory has absence as its whole outcome.
		_, err := os.Lstat(step.Path)
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if len(step.Absent)+len(step.After)+len(step.Checks) == 0 {
		return false, nil
	}
	// These dedicated completion checks describe only a systemd stop. An
	// inactive, failed or unloaded unit needs no further mutation, even when the
	// unchanged Nix store remains an approval gate for later deletion.
	if len(step.Checks) > 0 && !slices.ContainsFunc(step.Checks, func(g legacyRetirementGate) bool {
		return g.Kind != "systemd-stopped" || g.Expected != "inactive" && g.Expected != "stopped"
	}) {
		for _, check := range step.Checks {
			state, err := d.gateValue(ctx, check)
			if err != nil || state != check.Expected {
				return false, err
			}
		}
		return true, nil
	}
	described := len(step.Before) != 0
	if described && d.preconditions(ctx, step) == nil {
		return false, nil
	}
	for _, gate := range step.Gates {
		if !slices.Contains(legacyStateGates, gate.Kind) {
			continue
		}
		described = true
		value, err := d.gateValue(ctx, gate)
		if err != nil {
			return false, err
		}
		if value == gate.Expected {
			return false, nil
		}
	}
	return described && d.postconditions(ctx, step) == nil, nil
}

func (d *legacyRetirementDriver) evaluateGates(ctx context.Context, gates []legacyRetirementGate) error {
	for _, gate := range gates {
		value, err := d.gateValue(ctx, gate)
		if err != nil {
			return fmt.Errorf("%s %s check could not be completed: %w", gate.Kind, gate.Subject, err)
		}
		if value != gate.Expected {
			return fmt.Errorf("%s %s differs from the approved state: found %q, approved %q", gate.Kind, gate.Subject, truncateLegacyText(value, 512), truncateLegacyText(gate.Expected, 512))
		}
	}
	return nil
}

func (d *legacyRetirementDriver) gateValue(ctx context.Context, gate legacyRetirementGate) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	switch gate.Kind {
	case "replacements":
		_, reasons, err := d.replacementCheck(ctx)
		return strings.Join(reasons, "; "), err
	case "nix-consumers":
		return d.nixConsumers(ctx, true, gate.Subject)
	case "nix-consumer-roots":
		return d.nixConsumers(ctx, false, "")
	case "nix-homebrew":
		// Removing Nix must never strand a Homebrew prefix still run from it.
		if d.Platform.Homebrew == nil {
			return "", nil
		}
		return LegacyNixHomebrewIssue(*d.Platform.Homebrew), nil
	case "nix-store":
		return d.nixStoreGate(ctx, gate)
	case "determinate":
		return d.determinateGateValue(ctx)
	case "account":
		return d.accountRecord(ctx, gate.Subject)
	case "group":
		return d.groupRecord(ctx, gate.Subject)
	case "account-processes":
		return d.accountProcesses(ctx, gate.Subject)
	case "login-shell":
		return d.loginShell(ctx)
	case "launchd":
		return d.launchdState(ctx, gate.Subject)
	case "systemd-stopped":
		state, err := d.systemdActiveState(ctx, gate.Subject)
		// Preserve the strict inactive check in older saved steps. New stop
		// recipes accept failed as stopped, without clearing failure records.
		if err == nil && gate.Expected == "stopped" && (state == "inactive" || state == "failed") {
			return "stopped", nil
		}
		return state, err
	case "volume":
		return d.volumeIdentity(ctx, gate.Subject)
	case "homebrew":
		return d.homebrewHealth(ctx)
	case "brew-inventory":
		return d.brewInventoryIdentity(ctx)
	case "brew-preservation":
		return d.brewPreservationIdentity(ctx)
	case "brew-formula":
		return d.brewFormula(ctx, gate.Subject)
	case "gh-extension":
		return d.ghExtension(ctx, gate.Subject)
	case "path-identity":
		return legacyPathIdentity(gate.Subject)
	}
	if slices.Contains(legacyDeltaGateKinds, gate.Kind) {
		return d.deltaGateValue(ctx, gate)
	}
	return "", fmt.Errorf("unknown legacy gate %q", gate.Kind)
}

// Inspection helpers build exact steps from observed evidence.

func legacyFile(path string) (legacyRetirementFile, error) {
	snapshot, err := snapshotConfig(path)
	return legacyRetirementFile{Path: path, Snapshot: snapshot}, err
}

func (d *legacyRetirementDriver) preservedPath(path, identity string) (string, error) {
	return legacySiblingBackup(path, identity)
}

func legacySiblingBackup(path, identity string) (string, error) {
	name, err := digest(path + "\x00" + identity)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), ".dotfiles-retired-"+name[:12]+"-"+filepath.Base(path)), nil
}

// Preserve beside the original so redirected folders and Homebrew prefixes on
// other filesystems keep the same atomic rename and recovery guarantees.
func (d *legacyRetirementDriver) addUserMove(scope *legacyRetirementScope, label, path string) error {
	identity, err := legacyPathIdentity(path)
	if err != nil || identity == "absent" {
		return err
	}
	path, err = bindConfigDestination(path)
	if err != nil {
		return err
	}
	for _, active := range []string{d.Repository, d.Normal.StatePath, d.Environment.Executable} {
		if active == "" {
			continue
		}
		if relative, err := filepath.Rel(path, active); err == nil && (relative == "." || filepath.IsLocal(relative)) {
			return fmt.Errorf("preserve %s in place: it contains the active checkout, installer or state", path)
		}
	}
	backup, err := d.preservedPath(path, identity)
	if err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "move", Label: label + ": " + path + " -> " + backup, Path: path, Backup: backup, Identity: identity})
	return nil
}

// Move a present root-owned path aside in its own directory.
func (d *legacyRetirementDriver) addSystemMove(scope *legacyRetirementScope, label, path string) error {
	identity, err := legacyPathIdentity(path)
	if err != nil || identity == "absent" {
		return err
	}
	backup, err := legacySiblingBackup(path, identity)
	if err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "move", Label: label + ": " + path + " -> " + backup, Privileged: true, Path: path, Backup: backup, Identity: identity})
	return nil
}

// Discard a bounded user-owned tree recorded entry by entry.
func addLegacyDiscard(scope *legacyRetirementScope, label, path string) error {
	entries, err := inspectTree(path, maxConfigBytes, maxConfigEntries)
	if err != nil {
		scope.Retained = append(scope.Retained, path+" could not be recorded for exact removal: "+err.Error())
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "discard", Label: label + ": " + path, Path: path, Entries: entries, Absent: []string{path}})
	return nil
}

// Remove one root-owned recorded file or link.
func addLegacyRemoval(scope *legacyRetirementScope, label, path string) error {
	file, err := legacyFile(path)
	if err != nil || file.Snapshot.Kind == "absent" {
		return err
	}
	if file.Snapshot.Kind != "file" && file.Snapshot.Kind != "link" {
		scope.Retained = append(scope.Retained, path+" is not a single recorded file or link")
		return nil
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "remove", Label: label + ": " + path, Privileged: true, Path: path, Before: []legacyRetirementFile{file}, Absent: []string{path}})
	return nil
}

// Remove exact lines, each present once, from a root-owned system file. Every
// other byte stays and the original is kept beside it. An emptied file is
// removed. Personal edits elsewhere in the file are preserved.
func (d *legacyRetirementDriver) addLineRemoval(ctx context.Context, scope *legacyRetirementScope, label, path string, remove ...string) error {
	data, snapshot, err := d.readSystemText(ctx, path)
	if err != nil {
		return err
	}
	if snapshot.Kind == "absent" || len(remove) == 0 {
		return nil
	}
	uid, gid, mode, err := legacyFileOwner(path)
	if err != nil {
		return err
	}
	kept, found := []string{}, map[string]int{}
	for _, current := range strings.SplitAfter(string(data), "\n") {
		if line := strings.TrimSuffix(current, "\n"); slices.Contains(remove, line) {
			found[line]++
			continue
		}
		kept = append(kept, current)
	}
	for _, line := range remove {
		if found[line] != 1 {
			scope.Blocked = fmt.Sprintf("%s must contain the exact line %q once; found %d. Review it manually", path, line, found[line])
			return nil
		}
	}
	content := []byte(strings.Join(kept, ""))
	backup, err := legacySiblingBackup(path, legacyHash(data))
	if err != nil {
		return err
	}
	quoted := []string{}
	for _, line := range remove {
		quoted = append(quoted, strconv.Quote(line))
	}
	step := legacyRetirementStep{Kind: "edit", Label: label + ": remove " + strings.Join(quoted, ", ") + " from " + path, Privileged: true, Path: path, Backup: backup}
	step.Identity, step.Mode, step.Content = legacyHash(data), mode, content
	step.Owner, step.Group = strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(gid), 10)
	step.Before = []legacyRetirementFile{{Path: path, Snapshot: snapshot}}
	step.After = []legacyRetirementFile{{Path: backup, Snapshot: snapshot}}
	if len(content) == 0 {
		step.Absent = []string{path}
	} else {
		after, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash(content), Mode: mode & 0777}})
		if err != nil {
			return err
		}
		step.After = append(step.After, legacyRetirementFile{Path: path, Snapshot: after})
	}
	scope.Steps = append(scope.Steps, step)
	return nil
}
