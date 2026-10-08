package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var homebrewBootstrapSource = regexp.MustCompile(`^https://raw\.githubusercontent\.com/Homebrew/install/[a-f0-9]{40}/install\.sh$`)

func (d *HomebrewBootstrapDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *HomebrewBootstrapDriver) intent(operation string) (homebrewBootstrapIntent, error) {
	var intent homebrewBootstrapIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Homebrew bootstrap operation")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 1 || intent.Operation != operation || intent.Location != d.location() ||
		intent.Action != "install" || len(intent.Commands) > 16 ||
		!homebrewBootstrapSource.MatchString(intent.Recipe.URL) || !operationID.MatchString(intent.Recipe.SHA256) {
		return intent, errors.New("invalid saved Homebrew bootstrap intent")
	}
	for i, saved := range intent.Commands {
		command, err := d.command(intent, i, saved.Kind)
		if err != nil {
			return intent, err
		}
		expected, err := digest(command)
		if err != nil {
			return intent, err
		}
		actual, err := digest(saved.Command)
		if err != nil || expected != actual {
			return intent, errors.Join(errors.New("saved Homebrew bootstrap command changed its reviewed recipe"), err)
		}
	}
	return intent, nil
}

func (d *HomebrewBootstrapDriver) command(intent homebrewBootstrapIntent, index int, kind string) (nativeCommand, error) {
	id, err := digest(struct {
		Operation string
		Step      int
	}{intent.Operation, index})
	if err != nil {
		return nativeCommand{}, err
	}
	command := nativeCommand{Operation: id, Program: "/usr/bin/env"}
	if kind == "reserve" {
		if intent.Action != "install" {
			return command, errors.New("only fresh Homebrew bootstrap can reserve a prefix")
		}
		command.Program = "/usr/bin/sudo"
		command.Arguments = []string{"-n", "/bin/mkdir", intent.Location.Prefix}
		return command, command.validate()
	}
	command.Arguments = d.environment()
	switch kind {
	case "install":
		if intent.Action != "install" {
			return command, errors.New("existing Homebrew infrastructure cannot rerun the bootstrap installer")
		}
		payload, err := d.payload(intent.Operation, intent.Recipe)
		if err != nil {
			return command, err
		}
		command.Input = payload
		command.Arguments = append(command.Arguments, "/bin/bash", "--noprofile", "--norc", "-s", "--", "--path="+intent.Location.Prefix)
	case "verify":
		command.Arguments = append(command.Arguments, intent.Location.Program, "--version")
	default:
		return command, errors.New("unsupported Homebrew infrastructure command")
	}
	return command, command.validate()
}

func (d *HomebrewBootstrapDriver) environment() []string {
	// env -i removes BASH_ENV, Git environment hooks, askpass programs, mirrors
	// and inherited credentials from bootstrap and every manager inspection.
	values := []string{"-i", "HOME=" + d.Home, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "SHELL=/bin/zsh",
		"NONINTERACTIVE=1", "SUDO_ASKPASS=/usr/bin/false", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
		"HOMEBREW_BREW_GIT_REMOTE=https://github.com/Homebrew/brew", "HOMEBREW_CORE_GIT_REMOTE=https://github.com/Homebrew/homebrew-core"}
	return append(values, brewProcessControls...)
}

func (d *HomebrewBootstrapDriver) runStep(ctx context.Context, intent *homebrewBootstrapIntent, kind string) error {
	if len(intent.Commands) >= 16 {
		return errors.New("Homebrew bootstrap exceeded its bounded recovery attempts")
	}
	command, err := d.command(*intent, len(intent.Commands), kind)
	if err != nil {
		return err
	}
	step := homebrewBootstrapStep{Kind: kind, Command: command}
	intent.Commands = append(intent.Commands, step)
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return err
	}
	return d.dispatch(ctx, *intent, step)
}

func (d *HomebrewBootstrapDriver) dispatch(ctx context.Context, intent homebrewBootstrapIntent, step homebrewBootstrapStep) error {
	identity, err := homebrewBootstrapDirectory(intent.Location.Prefix)
	if step.Kind == "reserve" {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errors.New("Homebrew prefix appeared before exclusive reservation; preserve it"), err)
		}
	} else {
		if err != nil || identity != intent.Directory || identity == "" {
			return errors.Join(errors.New("Homebrew prefix changed after its saved reservation"), err)
		}
		if step.Kind == "install" {
			if _, err := os.Lstat(filepath.Join(intent.Location.Prefix, ".git")); err == nil {
				if err := d.source(ctx); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			for _, directory := range []string{"Cellar", "Caskroom"} {
				entries, err := os.ReadDir(filepath.Join(intent.Location.Prefix, directory))
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if len(entries) > 0 {
					return errors.New("Homebrew now contains native packages; preserve them and repair the manager before resuming")
				}
			}
		} else {
			if err := d.source(ctx); err != nil {
				return err
			}
		}
	}
	if step.Kind != "verify" && d.Authenticate != nil {
		if err := d.Authenticate(ctx); err != nil {
			return err
		}
	}
	_, runErr := d.Run(ctx, step.Command)
	if runErr != nil {
		reply, readErr := d.result(step.Command)
		if readErr != nil {
			return errors.Join(runErr, readErr)
		}
		if reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0 {
			return errors.Join(runErr, homebrewBootstrapFailure(reply))
		}
		// Preserve a lost transport reply even if the worker persisted success.
		// Recovery verifies that record and never repeats the successful command.
		return runErr
	}
	return nil
}

func (d *HomebrewBootstrapDriver) result(command nativeCommand) (nativeReply, error) {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash {
		return nativeReply{}, errors.Join(errors.New("Homebrew bootstrap command differs from its saved native evidence"), err)
	}
	if record.Reply == nil {
		return nativeReply{}, errors.New("Homebrew bootstrap has an unknown native outcome; inspect and repair the prefix, then explicitly abandon this operation and replan to reuse the manager")
	}
	if record.Reply.Ready || record.Reply.ExitCode == nil && record.Reply.Error == "" || len(record.Reply.Output) > 1<<20 {
		return nativeReply{}, errors.New("invalid saved Homebrew bootstrap command result")
	}
	return *record.Reply, nil
}

func (d *HomebrewBootstrapDriver) payload(operation string, recipe homebrewBootstrapRecipe) ([]byte, error) {
	data, err := readDocument(filepath.Join(d.Directory, "payloads", operation+".sh"))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if len(data) > 512<<10 || hex.EncodeToString(sum[:]) != recipe.SHA256 {
		return nil, errors.New("Homebrew bootstrap payload differs from its reviewed SHA-256")
	}
	return data, nil
}

func (d *HomebrewBootstrapDriver) preparePayload(ctx context.Context, operation string, recipe homebrewBootstrapRecipe) (result error) {
	if _, err := d.payload(operation, recipe); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !homebrewBootstrapSource.MatchString(recipe.URL) || !operationID.MatchString(recipe.SHA256) {
		return errors.New("Homebrew bootstrap source is not a pinned official installer")
	}
	client := d.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	safe := *client
	safe.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return errors.New("Homebrew bootstrap does not accept source redirects")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recipe.URL, nil)
	if err != nil {
		return err
	}
	response, err := safe.Do(req)
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, (512<<10)+1))
	if err := errors.Join(readErr, response.Body.Close()); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if response.StatusCode != http.StatusOK || len(data) == 0 || len(data) > 512<<10 || hex.EncodeToString(sum[:]) != recipe.SHA256 {
		return errors.New("Homebrew bootstrap download failed its status, size or pinned checksum check")
	}
	directory := filepath.Join(d.Directory, "payloads")
	if err := prepareStateDirectory(directory); err != nil {
		return err
	}
	bound, err := bindConfigDestination(filepath.Join(directory, operation+".sh"))
	if err != nil || bound != filepath.Join(directory, operation+".sh") {
		return errors.Join(errors.New("Homebrew bootstrap payload directory was redirected"), err)
	}
	file, err := os.CreateTemp(directory, ".bootstrap-*.sh")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return err
	}
	return moveConfigExclusive(file.Name(), bound)
}
