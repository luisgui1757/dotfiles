package installer

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const gitSettingsResource = "integration.git"

// Git reads XDG settings before ~/.gitconfig, and includes expand in place.
// Prepending defaults here preserves both personal override layers. This file
// stays regular and writable; --global never writes through an included file.
func configureGitSettings(d *ProfileDriver, folders ConfigFolders, repository string) error {
	d.GitIssue = folders.GitIssue
	// The checkout source is independent of Git's discovered target. Bind it
	// first so a saved target remains removable when discovery later fails.
	source := filepath.Join(repository, "git", "config")
	if !canonicalGitPath(source) {
		return errors.New("Git settings require explicit canonical paths without control characters")
	}
	d.GitDefaults = source
	path := gitConfigPath(folders)
	if !canonicalGitPath(path) {
		return errors.New("Git settings require explicit canonical paths without control characters")
	}
	bound, err := bindConfigDestination(path)
	if err != nil {
		return err
	}
	d.Targets[gitSettingsResource] = []ProfileTarget{{Path: bound, Script: gitIncludeScript(source)}}
	blockGitCheckoutOverlap(d, bound)
	return nil
}

func canonicalGitPath(value string) bool {
	return utf8.ValidString(value) && filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func gitIncludeScript(source string) string {
	if source == "" {
		return "" // No canonical source: removal remains possible, publication is blocked.
	}
	quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(filepath.ToSlash(source))
	return "[include]\n\tpath = \"" + quoted + "\"\n"
}

func blockGitCheckoutOverlap(d *ProfileDriver, target string) {
	repository := filepath.Dir(filepath.Dir(d.GitDefaults))
	if relative, err := filepath.Rel(repository, target); err == nil && (relative == "." || filepath.IsLocal(relative)) {
		d.GitIssue = strings.TrimSpace(d.GitIssue + " Git's writable global configuration resolves inside the dotfiles checkout. Choose a personal Git configuration directory outside the checkout before enabling defaults.")
	}
}

// Keep removal and recovery attached to the originally approved path when a
// caller changes HOME/XDG between shells. New publication at another path needs
// explicit removal of the old integration first.
func retainGitSettingsTarget(d *ProfileDriver, state State) error {
	receipt := state.Receipts[gitSettingsResource]
	if receipt.Recovery == "" || receipt.Ownership == "reused" {
		return nil
	}
	if !validProfileRecovery(receipt.Recovery) {
		return errors.New("invalid saved Git configuration recovery path")
	}
	data, err := readDocument(d.baselinePath(receipt.Recovery))
	if errors.Is(err, os.ErrNotExist) && receipt.Status == "in-progress" {
		return nil
	}
	if err != nil {
		return err
	}
	var baseline profileBaseline
	if err := Decode(data, &baseline); err != nil {
		return err
	}
	if len(baseline.Entries) != 1 || !filepath.IsAbs(baseline.Entries[0].Path) || filepath.Clean(baseline.Entries[0].Path) != baseline.Entries[0].Path {
		return errors.New("invalid saved Git settings target")
	}
	saved := baseline.Entries[0].Path
	targets := d.Targets[gitSettingsResource]
	moved := true
	if discovery := d.Issues[gitSettingsResource]; len(targets) == 0 && discovery != "" {
		// Removal does not need today's discovery. Its failure blocks only new
		// publication and keeps the original cause beside the saved target.
		d.GitIssue = strings.TrimSpace(d.GitIssue + " " + discovery)
		d.Issues = maps.Clone(d.Issues)
		delete(d.Issues, gitSettingsResource)
		targets = []ProfileTarget{{Path: saved, Script: gitIncludeScript(d.GitDefaults)}}
	} else if len(targets) != 1 {
		return errors.New("Git settings require exactly one target")
	} else if moved = targets[0].Path != saved && !(d.Windows && strings.EqualFold(targets[0].Path, saved)); moved {
		d.GitIssue = strings.TrimSpace(d.GitIssue + " Git's global configuration path changed. Remove the saved Git integration through setup before selecting Git at the new path.")
	}
	d.Targets = maps.Clone(d.Targets)
	d.Targets[gitSettingsResource] = []ProfileTarget{{Path: saved, Script: targets[0].Script}}
	if moved {
		// Discovery already checked an unchanged target.
		blockGitCheckoutOverlap(d, saved)
	}
	_, err = d.readBaseline(gitSettingsResource, receipt)
	return err
}

func gitConfigPath(folders ConfigFolders) string {
	if folders.GitConfig != "" {
		return folders.GitConfig
	}
	root := folders.Config
	if root == "" {
		root = filepath.Join(folders.Home, ".config")
	}
	return filepath.Join(root, "git", "config")
}

func validateGitEncoding(data []byte) error {
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return errors.New("Git configuration must be UTF-8; preserve it for explicit conversion")
	}
	return nil
}

func gitDefaultsSnapshot(path string) (configSnapshot, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return configSnapshot{}, errors.New("Git defaults require an explicit checkout source")
	}
	data, snapshot, err := readProfile(path)
	if err == nil && snapshot.Kind == "absent" {
		err = os.ErrNotExist
	}
	if err == nil {
		err = validateGitEncoding(data)
	}
	return snapshot, err
}
