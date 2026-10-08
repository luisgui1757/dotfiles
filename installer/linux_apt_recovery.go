package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A failed APT history lists planned packages, not completed installations.
// These provisional candidates therefore require an actual dpkg install action.
// dpkg uses install for absent payloads, including config-files state with a
// remembered old version; other prior states produce upgrade (dpkg unpack.c).
// They authorize only a reviewed retry of the same saved operation.
// Ownership additionally requires a successful final command, completed dpkg
// configuration across those attempts and current installed-state verification.
func linuxAPTAttemptEvidence(history, log []byte, operation string) (map[string]aptInstalledPackage, map[string]string, map[string]string, error) {
	introduced := map[string]aptInstalledPackage{}
	configured, changed := map[string]string{}, map[string]string{}
	if len(history) == 0 && len(log) == 0 {
		return introduced, configured, changed, nil
	}
	if !operationID.MatchString(operation) || len(history) == 0 || len(history) > 8<<20 || len(log) > 8<<20 || strings.ContainsRune(string(history), 0) || strings.ContainsRune(string(log), 0) {
		return nil, nil, nil, errors.New("invalid APT recovery evidence")
	}
	fields := map[string]string{}
	ended := false
	for _, line := range strings.Split(string(history), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok || key == "" || ended || len(fields) == 0 && key != "Start-Date" {
			return nil, nil, nil, errors.New("malformed or multiple APT recovery transactions")
		}
		if _, exists := fields[key]; exists {
			return nil, nil, nil, errors.New("duplicate APT recovery transaction field")
		}
		fields[key] = value
		ended = key == "End-Date"
	}
	if !ended || fields["Start-Date"] == "" || fields["End-Date"] == "" {
		return nil, nil, nil, errors.New("unfinished APT recovery transaction requires native inspection")
	}
	matched := 0
	for _, argument := range strings.Fields(fields["Commandline"]) {
		if argument == "Dotfiles::Operation="+operation {
			matched++
		}
	}
	if matched != 1 {
		return nil, nil, nil, errors.New("APT recovery transaction belongs to another operation")
	}
	for _, field := range []string{"Remove", "Purge", "Downgrade", "Disappeared"} {
		if fields[field] != "" {
			return nil, nil, nil, errors.New("APT recovery transaction contains unexpected removal or downgrade")
		}
	}
	planned := map[string]aptInstalledPackage{}
	for rest := fields["Install"]; rest != ""; {
		match := aptInstallItem.FindStringSubmatch(rest)
		if match == nil || len(match[2]) > 256 || planned[match[1]].Name != "" {
			return nil, nil, nil, errors.New("invalid planned APT recovery installation")
		}
		planned[match[1]] = aptInstalledPackage{Name: match[1], Version: match[2], Automatic: match[3] != ""}
		rest = rest[len(match[0]):]
		if rest == "" && match[4] != "" {
			return nil, nil, nil, errors.New("empty planned APT recovery installation")
		}
	}
	actual := map[string]string{}
	for _, line := range strings.Split(string(log), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			if strings.TrimSpace(line) != "" {
				return nil, nil, nil, errors.New("truncated dpkg recovery record")
			}
			continue
		}
		switch parts[2] {
		case "install", "upgrade":
			if len(parts) != 6 || !aptPackageName.MatchString(parts[3]) {
				return nil, nil, nil, errors.New("invalid dpkg recovery mutation")
			}
			if parts[2] == "install" {
				if actual[parts[3]] != "" {
					return nil, nil, nil, errors.New("duplicate dpkg recovery installation")
				}
				actual[parts[3]] = parts[5]
			}
			changed[parts[3]] = parts[5]
		case "status":
			if len(parts) != 6 || !aptPackageName.MatchString(parts[4]) {
				return nil, nil, nil, errors.New("invalid dpkg recovery state")
			}
			configured[parts[4]] = ""
			if parts[3] == "installed" {
				configured[parts[4]] = parts[5]
			}
		case "remove", "purge", "disappear":
			return nil, nil, nil, errors.New("dpkg recovery record unexpectedly removed a package")
		}
	}
	for _, pkg := range planned {
		if actual[pkg.Name] == "" {
			base, _, _ := strings.Cut(pkg.Name, ":")
			if actual[base+":all"] == pkg.Version {
				pkg.Name = base + ":all"
			}
		}
		if actual[pkg.Name] == pkg.Version {
			if introduced[pkg.Name].Name != "" {
				return nil, nil, nil, errors.New("APT recovery has ambiguous package architecture")
			}
			introduced[pkg.Name] = pkg
		}
	}
	for name := range actual {
		if introduced[name].Name == "" {
			return nil, nil, nil, fmt.Errorf("dpkg introduction of %s lacks operation-matched APT planning", name)
		}
	}
	return introduced, configured, changed, nil
}

func (d *LinuxAPTDriver) attemptEvidence(intent linuxAPTIntent) (map[string]aptInstalledPackage, map[string]string, map[string]string, error) {
	introduced := map[string]aptInstalledPackage{}
	configured, changes := map[string]string{}, map[string]string{}
	for _, saved := range intent.Commands {
		if saved.Kind != "install" {
			continue
		}
		// A missing result means the command may still be running. Never derive
		// retry permission solely from a log that has not finished being written.
		reply, err := d.commandResult(saved.Command)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		base := filepath.Join(d.Directory, "logs", saved.Command.Operation)
		history, err := readBoundedLinuxAPTLog(base + ".history")
		if err != nil {
			return nil, nil, nil, err
		}
		log, err := readBoundedLinuxAPTLog(base + ".dpkg")
		if err != nil {
			return nil, nil, nil, err
		}
		if len(history) > 0 && reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0 {
			if _, err := aptInstallEvidence(history, log, saved.Command.Operation); err != nil {
				return nil, nil, nil, err
			}
		}
		added, statuses, changed, err := linuxAPTAttemptEvidence(history, log, saved.Command.Operation)
		if err != nil {
			return nil, nil, nil, err
		}
		for name, pkg := range added {
			if _, existed := intent.Before[name]; !existed {
				introduced[name] = pkg
			}
		}
		for name, version := range statuses {
			configured[name] = version
		}
		for name, version := range changed {
			changes[name] = version
		}
	}
	return introduced, configured, changes, nil
}
