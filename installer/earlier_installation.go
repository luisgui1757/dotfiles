package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// MigrationRelease is the last release with migrate. It moves a v0.4.x
// installation to this installer. This checkout only names it.
const MigrationRelease = "v1.0.1"

// EarlierInstallationHints lists the marks of a v0.4.x installation that
// migrate did not retire. Setup only reports them.
func EarlierInstallationHints(platform NativePlatform, folders ConfigFolders) []string {
	hints := []string{}
	exists := func(path string) bool {
		_, err := os.Lstat(path)
		return err == nil
	}
	share := filepath.Join(folders.Home, ".local", "share", "dotfiles")
	for _, item := range []struct{ path, text string }{
		{filepath.Join(share, "provenance"), "released direct-binary provenance markers"},
		{filepath.Join(share, "tmux-plugins"), "released pinned tmux plugin checkouts"},
		{filepath.Join(share, "zsh-plugins"), "released pinned zsh plugin checkouts"},
		{filepath.Join(share, "sentinel"), "released Sentinel source cache"},
		{filepath.Join(folders.Home, ".config", "chezmoi"), "chezmoi configuration and state"},
		{filepath.Join(folders.Home, ".local", "state", "home-manager"), "Home Manager state"},
	} {
		if exists(item.path) {
			hints = append(hints, item.text+" at "+item.path)
		}
	}
	if platform.Homebrew != nil && NixHomebrewIssue(*platform.Homebrew) != "" {
		hints = append(hints, "Nix-managed Homebrew at "+platform.Homebrew.Prefix)
	}
	if platform.OS == "darwin" && exists("/run/current-system") {
		hints = append(hints, "nix-darwin system activation at /run/current-system")
	}
	if len(hints) != 0 && platform.OS != "windows" && exists("/nix") {
		hints = append(hints, "Nix runtime at /nix")
	}
	return hints
}

// NixHomebrewIssue identifies a Homebrew prefix still run from the Nix
// store by nix-homebrew. Setup must not reuse it as native infrastructure.
func NixHomebrewIssue(location HomebrewLocation) string {
	issue := "Homebrew at " + location.Prefix + " is managed by nix-homebrew; run migrate from a " + MigrationRelease + " checkout to convert it before setup uses Homebrew"
	for _, path := range []string{filepath.Join(location.Prefix, ".managed_by_nix_darwin"), filepath.Join(location.Prefix, "Library", ".homebrew-is-managed-by-nix")} {
		if _, err := os.Lstat(path); err == nil {
			return issue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
	}
	for _, path := range []string{location.Program, filepath.Join(location.Prefix, "Library", "Homebrew")} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
		if strings.HasPrefix(target, "/nix/store/") {
			return issue
		}
	}
	return ""
}
