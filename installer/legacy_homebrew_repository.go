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

// Homebrew 7.0.9. Conversion restores only this reviewed repository; the
// general installer also runs `brew update`, which can migrate other packages.
const legacyHomebrewCommit = "cdc1ca503231b9a96da64526774915d913c3ec9f"

// Tracked roots at the pinned commit. Library and bin are shared with installed
// software, so only their exact source children may be replaced. Every existing
// source root is preserved beside its original before Git can publish anything.
var legacyHomebrewSourcePaths = []string{
	".claude", ".codex", ".cursor", ".devcontainer", ".dockerignore", ".editorconfig", ".gitattributes", ".github", ".gitignore", ".shellcheckrc", ".sublime", ".vale.ini", ".vscode",
	"AGENTS.md", "CHANGELOG.md", "CLAUDE.md", "CODEOWNERS", "CONTRIBUTING.md", "Dockerfile", "LICENSE.txt", "README.md",
	"Library/.rubocop.yml", "Library/Homebrew", "Library/README.md", "Library/tap_rubocop_style.yml", "bin/brew", "completions", "docs", "manpages", "package",
}

// Read installation records without evaluating old Homebrew Ruby. An older
// nix-homebrew can neither list newer casks nor reliably include them in its
// JSON inventory. Conversion does not remove packages, so package recipes and
// dependency evaluation are unnecessary here. Their original records stay exact.
func (d *legacyRetirementDriver) brewPreservationIdentity(ctx context.Context) (string, error) {
	location := d.Platform.Homebrew
	if location == nil {
		return "", errors.New("Homebrew is not installed")
	}
	files := []legacyRetirementFile{}
	add := func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(files) >= 16384 {
			return errors.New("Homebrew installation records exceed their review bound")
		}
		file, err := legacyFile(path)
		if err == nil {
			files = append(files, file)
		}
		return err
	}
	// Directory membership records include empty racks and every installed
	// version. Links are recorded themselves and never followed into app data.
	children := func(path string) ([]os.DirEntry, error) {
		entries, err := readPlainDirectory(path)
		if errors.Is(err, os.ErrNotExist) {
			files = append(files, legacyRetirementFile{Path: path, Snapshot: configSnapshot{Kind: "absent"}})
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if len(entries) > 16384 || len(files) >= 16384 {
			return nil, errors.New("Homebrew installation membership exceeds its review bound")
		}
		names := []struct {
			Name string
			Type os.FileMode
		}{}
		for _, entry := range entries {
			names = append(names, struct {
				Name string
				Type os.FileMode
			}{entry.Name(), entry.Type()})
		}
		hash, err := digest(names)
		if err == nil {
			files = append(files, legacyRetirementFile{Path: path, Snapshot: configSnapshot{Kind: "directory", Hash: hash}})
		}
		return entries, err
	}
	for _, root := range []string{location.Cellar, filepath.Join(location.Prefix, "Caskroom")} {
		packages, err := children(root)
		if err != nil {
			return "", err
		}
		for _, pkg := range packages {
			path := filepath.Join(root, pkg.Name())
			if !pkg.IsDir() {
				if err := add(path); err != nil {
					return "", err
				}
				continue
			}
			versions, err := children(path)
			if err != nil {
				return "", err
			}
			if root != location.Cellar {
				if err := add(filepath.Join(path, ".metadata")); err != nil {
					return "", err
				}
				continue
			}
			for _, version := range versions {
				if !version.IsDir() {
					if err := add(filepath.Join(path, version.Name())); err != nil {
						return "", err
					}
					continue
				}
				for _, name := range []string{"INSTALL_RECEIPT.json", ".brew"} {
					if err := add(filepath.Join(path, version.Name(), name)); err != nil {
						return "", err
					}
				}
			}
		}
	}
	for _, name := range []string{"opt", "var/homebrew/pinned", "var/homebrew/linked"} {
		if err := add(filepath.Join(location.Prefix, filepath.FromSlash(name))); err != nil {
			return "", err
		}
	}
	// Git reads each tap's tracked and untracked changes, including dirty local
	// definitions. No Brew query or updater runs against these repositories.
	taps := filepath.Join(location.Prefix, "Library", "Taps")
	owners, err := children(taps)
	if err != nil {
		return "", err
	}
	for _, owner := range owners {
		path := filepath.Join(taps, owner.Name())
		if !owner.IsDir() {
			if owner.Type()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("Homebrew tap owner %s is redirected; preserve and inspect it before retiring Nix", path)
			}
			if err := add(path); err != nil {
				return "", err
			}
			continue
		}
		repositories, err := children(path)
		if err != nil {
			return "", err
		}
		for _, repository := range repositories {
			path := filepath.Join(path, repository.Name())
			if !repository.IsDir() {
				if repository.Type()&os.ModeSymlink != 0 {
					return "", fmt.Errorf("Homebrew tap %s is redirected; preserve and inspect it before retiring Nix", path)
				}
				if err := add(path); err != nil {
					return "", err
				}
				continue
			}
			// Exact snapshots are bounded. This is source metadata, not the
			// potentially enormous Cellar/application payload or Git objects.
			for _, name := range []string{filepath.Join(".git", "HEAD"), filepath.Join(".git", "config")} {
				if err := add(filepath.Join(path, name)); err != nil {
					return "", err
				}
			}
			if err := d.checkGitInspection(ctx, path, false); err != nil {
				return "", err
			}
			var records []string
			for _, arguments := range [][]string{{"rev-parse", "HEAD"}, {"status", "--porcelain=v1", "--untracked-files=all"}, {"diff", "--no-ext-diff", "--no-textconv", "--binary", "HEAD", "--"}} {
				data, err := d.git(ctx, path, arguments...)
				if err != nil {
					return "", err
				}
				records = append(records, string(data))
			}
			untracked, err := d.git(ctx, path, "ls-files", "--others", "--exclude-standard", "-z")
			if err != nil {
				return "", err
			}
			for _, name := range strings.Split(strings.TrimSuffix(string(untracked), "\x00"), "\x00") {
				if name == "" {
					continue
				}
				if !filepath.IsLocal(name) {
					return "", errors.New("Homebrew tap returned a nonlocal untracked path")
				}
				if err := add(filepath.Join(path, name)); err != nil {
					return "", err
				}
			}
			hash, err := digest(records)
			if err != nil {
				return "", err
			}
			files = append(files, legacyRetirementFile{Path: path, Snapshot: configSnapshot{Kind: "directory", Hash: hash}})
		}
	}
	return digest(files)
}

func (d *legacyRetirementDriver) homebrewRepositoryCommand(prefix, identity string) nativeCommand {
	values := d.homebrewBootstrap().environment()
	// Save the complete source recipe in the command, so future pin updates
	// cannot silently alter an already approved recovery operation.
	values = append(values, "/bin/bash", "--noprofile", "--norc", "-c", legacyHomebrewRepositoryScript, "sh", prefix, legacyHomebrewCommit, identity)
	values = append(values, legacyHomebrewSourcePaths...)
	return nativeCommand{Program: "/usr/bin/env", Arguments: values}
}

const legacyHomebrewRepositoryScript = `set -euo pipefail
prefix=$1 commit=$2 approval=$3
shift 3
cd "$prefix"
git=/usr/bin/git
marker=.git/dotfiles-retirement
if [ ! -e .git ] && [ ! -L .git ]; then
	"$git" -c init.defaultBranch=stable init --template= --quiet
	"$git" remote add origin https://github.com/Homebrew/brew
	identity=$(/usr/bin/stat -f '%d:%i:%u' .git)
	configuration=$(/usr/bin/shasum -a 256 .git/config)
	(umask 077; set -C; printf '%s\n%s\n%s\n' "$approval" "$identity" "$configuration" > "$marker")
else
	if [ ! -d .git ] || [ -L .git ] || [ ! -f "$marker" ] || [ -L "$marker" ] || [ ! -f .git/config ] || [ -L .git/config ]; then exit 110; fi
	identity=$(/usr/bin/stat -f '%d:%i:%u' .git)
	configuration=$(/usr/bin/shasum -a 256 .git/config)
	if [ "$(cat "$marker")" != "$(printf '%s\n%s\n%s\n' "$approval" "$identity" "$configuration")" ]; then exit 111; fi
fi
if [ "$("$git" config --local --get remote.origin.url)" != https://github.com/Homebrew/brew ]; then exit 112; fi
if head=$("$git" rev-parse --verify HEAD 2>/dev/null); then
	if [ "$head" != "$commit" ] || [ -n "$("$git" status --porcelain --untracked-files=no)" ]; then exit 113; fi
fi
"$git" -c core.autocrlf=false -c core.symlinks=true fetch --quiet --depth=1 origin refs/tags/7.0.9:refs/tags/7.0.9
if [ "$("$git" rev-parse 'refs/tags/7.0.9^{commit}')" != "$commit" ]; then exit 114; fi
paths=$(mktemp)
trap 'rm -f "$paths"' EXIT
"$git" ls-tree -rz --name-only "$commit" > "$paths"
while IFS= read -r -d '' path; do
	allowed=
	for root in "$@"; do
		case "$path" in "$root"|"$root"/*) allowed=1; break;; esac
	done
	if [ -z "$allowed" ]; then printf 'Unreviewed Homebrew source path: %s\n' "$path" >&2; exit 115; fi
done < "$paths"
"$git" -c core.autocrlf=false -c core.symlinks=true -c core.hooksPath=/dev/null checkout --quiet --no-overwrite-ignore -B stable "$commit"
"$git" update-ref refs/remotes/origin/main "$commit"
"$git" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
./bin/brew --version
# Provision only Homebrew's own verified Ruby runtime. This does not run a
# global update, evaluate installed casks or migrate packages and taps.
./bin/brew setup-ruby
`

func (d *legacyRetirementDriver) planHomebrewRepository(scope *legacyRetirementScope, prefix, inventory string) error {
	identity, err := legacyPathIdentity(filepath.Join(prefix, "bin", "brew"))
	if err != nil {
		return err
	}
	approval, err := digest([]string{prefix, identity, inventory, legacyHomebrewCommit})
	if err != nil {
		return err
	}
	for _, name := range legacyHomebrewSourcePaths {
		if slices.Contains([]string{"bin/brew", "Library/Homebrew"}, name) {
			continue // Their Nix links already have preservation steps.
		}
		if err := d.addUserMove(scope, "Preserve the previous Homebrew repository source", filepath.Join(prefix, filepath.FromSlash(name))); err != nil {
			return err
		}
	}
	command := d.homebrewRepositoryCommand(prefix, approval)
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Restore Homebrew 7.0.9's own repository at " + prefix, Command: &command,
		Gates:  []legacyRetirementGate{{Kind: "brew-preservation", Expected: inventory}},
		Checks: []legacyRetirementGate{{Kind: "homebrew", Expected: "healthy"}, {Kind: "brew-preservation", Expected: inventory}}})
	scope.Detail = append(scope.Detail, fmt.Sprintf("Restore Homebrew's repository at commit %s. Existing repository source is backed up; package records, installed versions, pins, links and taps must stay unchanged. No global brew update or package migration runs.", legacyHomebrewCommit))
	return nil
}
