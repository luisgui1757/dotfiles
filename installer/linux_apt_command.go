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

func (d *LinuxAPTDriver) command(intent linuxAPTIntent, index int, kind string, input []byte, arguments []string) (nativeCommand, error) {
	id, err := digest(struct {
		Operation string
		Attempt   int
	}{intent.Operation, index})
	if err != nil {
		return nativeCommand{}, err
	}
	program := d.Location.Dpkg
	args := []string{}
	switch kind {
	case "refresh":
		if intent.Action == "remove" || len(input) != 0 {
			return nativeCommand{}, errors.New("invalid APT repository refresh")
		}
		program = d.Location.APTGet
		args = []string{"-o", "APT::Update::Error-Mode=any", "update"}
	case "install":
		if intent.Action == "remove" || len(input) != 0 {
			return nativeCommand{}, errors.New("invalid APT installation command")
		}
		program = d.Location.APTGet
		log := filepath.Join(d.Directory, "logs", id)
		args = []string{"--yes", "--no-remove", "--no-install-recommends", "-o", "Dpkg::Use-Pty=0", "-o", "DPkg::Lock::Timeout=30",
			"-o", "Dpkg::Options::=--force-confold", "-o", "Dir::Log::History=" + log + ".history", "-o", "Dir::Log::Terminal=" + log + ".terminal",
			"-o", "Dpkg::Options::=--log=" + log + ".dpkg", "-o", "Dotfiles::Operation=" + id}
		reinstall := intent.Action == "repair"
		for _, previous := range intent.Commands[:min(index, len(intent.Commands))] {
			reinstall = reinstall || previous.Kind == "install"
		}
		if reinstall {
			args = append(args, "--reinstall")
		} else if intent.Action == "install" {
			// If an outside installer wins the package-manager lock after our
			// final observation, never upgrade its newly appeared root.
			args = append(args, "--no-upgrade")
		}
		if intent.PreserveAutomatic {
			// --mark-auto preserves an upgrading root; --only-upgrade also
			// prevents TryToInstall from promoting an already-current root.
			// Both are limited to a proved, already installed automatic root.
			args = append(args, "--mark-auto", "--only-upgrade")
		}
		args = append(args, "install", intent.Package)
	case "remove":
		if intent.Action != "remove" || len(input) != 0 || len(arguments) < 2 || arguments[0] != "--remove" {
			return nativeCommand{}, errors.New("invalid APT exact removal command")
		}
		seen := map[string]bool{}
		for _, name := range arguments[1:] {
			if !aptPackageName.MatchString(name) || !slices.Contains(intent.Remove, name) || seen[name] {
				return nativeCommand{}, errors.New("APT removal exceeds its saved exact set")
			}
			seen[name] = true
		}
		args = slices.Clone(arguments)
	case "verify-remove":
		if intent.Action != "remove" || len(input) != 0 {
			return nativeCommand{}, errors.New("invalid APT removal verification")
		}
		program = d.Location.DpkgQuery
		args = []string{"-W", "-f=${Package}:${Architecture}\t${Version}\t${db:Status-Abbrev}\n"}
	case "restore":
		if intent.Action != "remove" || !slices.Equal(arguments, []string{"--set-selections"}) || len(input) == 0 {
			return nativeCommand{}, errors.New("invalid APT selection restoration")
		}
		seen := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
			name, selection, ok := strings.Cut(line, "\t")
			if !ok || selection != "install" || !slices.Contains(intent.Remove, name) || seen[name] {
				return nativeCommand{}, errors.New("APT selection restoration exceeds its refused removal")
			}
			seen[name] = true
		}
		args = []string{"--set-selections"}
	default:
		return nativeCommand{}, errors.New("unsupported saved APT command")
	}
	command := nativeCommand{Operation: id, Program: d.Location.Sudo,
		Arguments: append([]string{"-n", "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive", program}, args...), Input: input}
	if d.Location.root {
		command.Program, command.Arguments = program, args
		command.Environment = []string{"LC_ALL=C", "DEBIAN_FRONTEND=noninteractive"}
	}
	return command, command.validate()
}

func (d *LinuxAPTDriver) commandPrefixLength() int {
	if d.Location.root {
		return 0
	}
	return 4
}

func (d *LinuxAPTDriver) runCommand(ctx context.Context, intent *linuxAPTIntent, kind string, input []byte, args []string) ([]byte, error) {
	if len(intent.Commands) >= 32 {
		return nil, errors.New("APT recovery exceeded its bounded command attempts")
	}
	command, err := d.command(*intent, len(intent.Commands), kind, input, args)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(d.Directory, "logs"), 0700); err != nil {
		return nil, err
	}
	bound, err := bindConfigDestination(filepath.Join(d.Directory, "logs", command.Operation+".history"))
	if err != nil || bound != filepath.Join(d.Directory, "logs", command.Operation+".history") {
		return nil, errors.Join(errors.New("APT operation log directory was redirected"), err)
	}
	for _, suffix := range []string{".history", ".dpkg", ".terminal"} {
		path := filepath.Join(d.Directory, "logs", command.Operation+suffix)
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(errors.New("APT command evidence path already exists or cannot be inspected"), err)
		}
	}
	saved := linuxAPTCommand{Kind: kind, Command: command}
	if err := d.prepareDispatch(ctx, *intent, saved); err != nil {
		return nil, err
	}
	intent.Commands = append(intent.Commands, saved)
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return nil, err
	}
	output, err := d.Run(ctx, command)
	if err != nil {
		return output, d.mutationError(saved, err)
	}
	return output, nil
}

func (d *LinuxAPTDriver) prepareDispatch(ctx context.Context, intent linuxAPTIntent, saved linuxAPTCommand) error {
	if !d.Location.root && d.Authenticate != nil {
		if err := d.Authenticate(ctx); err != nil {
			return err
		}
	}
	// Authentication may take time; recheck package protection afterwards.
	return d.validateDispatch(ctx, intent, saved)
}

func (d *LinuxAPTDriver) commandResult(command nativeCommand) (nativeReply, error) {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash || record.Reply == nil {
		return nativeReply{}, errors.Join(errors.New("APT command has unfinished evidence; inspect the native package state before retrying"), err)
	}
	if record.Reply.Ready || record.Reply.ExitCode == nil && record.Reply.Error == "" || len(record.Reply.Output) > 1<<20 {
		return nativeReply{}, errors.New("invalid saved APT command result")
	}
	return *record.Reply, nil
}

func (d *LinuxAPTDriver) verifyCompletion(intent linuxAPTIntent) error {
	if intent.Claim != "" {
		identity, err := linuxAPTIdentity(intent.Package, intent.Before)
		if err != nil || identity == "" {
			return errors.Join(errors.New("APT claim lacks its exact package baseline"), err)
		}
		pkg := intent.Before[identity]
		if !d.matchesOwnership(linuxAPTOwnership{identity, pkg.Source, intent.Claim}, pkg) {
			return errors.New("APT claim lacks completed native introduction evidence")
		}
		return nil
	}
	if len(intent.Commands) == 0 {
		return errors.New("APT operation lacks command evidence")
	}
	last := intent.Commands[len(intent.Commands)-1]
	reply, err := d.commandResult(last.Command)
	if err != nil || reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0 || last.Kind == "restore" || last.Kind == "refresh" {
		return errors.Join(errors.New("APT completion lacks a successful native command"), err)
	}
	return nil
}

// Recheck the native classification just before dispatch. In particular dpkg
// itself does not treat apt-mark manual promotion as a dependency constraint.
func (d *LinuxAPTDriver) validateDispatch(ctx context.Context, intent linuxAPTIntent, saved linuxAPTCommand) error {
	if saved.Kind == "restore" {
		names := []string{}
		for _, line := range strings.Split(strings.TrimSuffix(string(saved.Command.Input), "\n"), "\n") {
			name, _, _ := strings.Cut(line, "\t")
			names = append(names, name)
		}
		statuses, err := aptStatuses(ctx, d.Query, names)
		if err != nil || len(statuses) != len(names) {
			return errors.Join(errors.New("APT refused-removal recovery changed package identity"), err)
		}
		for _, pkg := range statuses {
			if pkg.Version != intent.Before[pkg.Name].Version || pkg.Status != "installed" || pkg.Error != "ok" || pkg.Want != "install" && pkg.Want != "deinstall" {
				return errors.New("APT refused-removal recovery changed package version or selection")
			}
		}
		return nil
	}
	if saved.Kind == "verify-remove" {
		return nil
	}
	installed, err := linuxAPTInventory(ctx, d.Query)
	if err != nil {
		return err
	}
	state, err := d.ledger()
	if err != nil {
		return err
	}
	if saved.Kind != "remove" {
		identity, err := linuxAPTIdentity(intent.Package, installed)
		if err != nil {
			return err
		}
		if pkg, present := installed[identity]; present {
			if pkg.Held {
				return errors.New("APT package became held before native dispatch")
			}
			if intent.Action == "install" {
				provisional, _, changes, err := d.attemptEvidence(intent)
				candidate, proved := provisional[identity]
				if err != nil || !proved || pkg.Version != changes[identity] || pkg.Automatic != candidate.Automatic {
					return errors.Join(errors.New("APT root appeared before dispatch without matching saved introduction evidence"), err)
				}
				return nil
			}
			owned := state.Roots[intent.Resource]
			if owned.Name != identity || owned.Source != pkg.Source || pkg.Automatic != intent.Before[identity].Automatic {
				return errors.New("APT root changed ownership before native dispatch")
			}
		}
		return nil
	}
	candidates, err := d.removalCandidates(intent.Resource, state, installed)
	if err != nil {
		return err
	}
	for _, name := range saved.Command.Arguments[d.commandPrefixLength()+1:] {
		if !slices.Contains(candidates, name) || !installed[name].Healthy {
			return fmt.Errorf("APT package %s changed before exact removal", name)
		}
	}
	return nil
}

func (d *LinuxAPTDriver) evidence(intent linuxAPTIntent) (map[string]bool, map[string]string, error) {
	if err := d.verifyCompletion(intent); err != nil {
		return nil, nil, err
	}
	candidates, configured, changes, err := d.attemptEvidence(intent)
	if err != nil {
		return nil, nil, err
	}
	introduced := map[string]bool{}
	for name := range candidates {
		if configured[name] == "" || configured[name] != changes[name] {
			return nil, nil, fmt.Errorf("APT introduction of %s lacks completed configuration across its saved attempts", name)
		}
		introduced[name] = true
	}
	for name, version := range changes {
		if configured[name] != version {
			return nil, nil, fmt.Errorf("APT mutation of %s lacks completed configuration across its saved attempts", name)
		}
	}
	return introduced, changes, nil
}

func readBoundedLinuxAPTLog(path string) ([]byte, error) {
	data, err := readDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func (d *LinuxAPTDriver) proveOwnership(owned linuxAPTOwnership) error {
	intent, err := d.intent(owned.CreatedBy)
	if err != nil || !intent.Complete || intent.Action == "remove" || intent.Claim != "" {
		return errors.Join(errors.New("APT package lacks its completed introduction"), err)
	}
	if err := d.verifyCompletion(intent); err != nil {
		return err
	}
	introduced, _, err := d.evidence(intent)
	if err != nil || !introduced[owned.Name] {
		return errors.Join(errors.New("APT package lacks operation-attributed introduction evidence"), err)
	}
	return nil
}
