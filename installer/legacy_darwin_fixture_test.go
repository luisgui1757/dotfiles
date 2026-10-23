package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// v0.4.4 pins Homebrew 6.0.1 but leaves its taps mutable. Current cask DSL
// revisions cannot be evaluated by that runtime. Reconstruct the official tap
// revisions at the release date (2026-07-26), without editing the old release or
// its package definitions. Preserve the disposable runner's original tap trees.
func prepareLegacyDarwinBrew(t *testing.T) {
	t.Helper()
	t.Setenv("HOMEBREW_NO_AUTO_UPDATE", "1")
	brew := "/opt/homebrew/bin/brew"
	root := strings.TrimSpace(legacyRun(t, "", nil, brew, "--repository"))
	if !filepath.IsAbs(root) || strings.ContainsAny(root, "\r\n\x00") {
		t.Fatal("the runner's Homebrew repository is not an absolute path", root)
	}
	for _, pin := range []struct{ tap, revision string }{
		// Core and cask migration maps must be contemporaneous: mixing the
		// current core with this cask snapshot loops on the runner's azure-cli.
		{"homebrew/homebrew-core", "a3073f1a0290ce9d45dbcb115140d5140ef3b1b4"},
		{"homebrew/homebrew-cask", "3ae5c66cb716f7ac822f475a0675b1f5bc646bcd"},
		{"nikitabobko/homebrew-tap", "13b49a6711cc9d2f4555056b4f7023c8ad2bbb4b"},
	} {
		tap := filepath.Join(root, "Library", "Taps", filepath.FromSlash(pin.tap))
		backup := filepath.Join(t.TempDir(), "original")
		existed := false
		if _, err := os.Lstat(tap); err == nil {
			if err := os.Rename(tap, backup); err != nil {
				t.Fatal(err)
			}
			existed = true
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		prepared := false
		t.Cleanup(func() {
			// Runtime retirement may untap the released AeroSpace source after
			// removing its last cask. Conversion's before/after fingerprint still
			// requires every tap unchanged at the earlier manager-only boundary.
			_, statErr := os.Lstat(tap)
			retired := pin.tap == "nikitabobko/homebrew-tap" && errors.Is(statErr, os.ErrNotExist)
			if prepared && !retired {
				if head := strings.TrimSpace(legacyRun(t, tap, nil, "git", "rev-parse", "HEAD")); head != pin.revision {
					t.Fatal("the historical tap revision changed", pin.tap, head)
				}
				if status := legacyRun(t, tap, nil, "git", "status", "--porcelain"); status != "" {
					t.Fatal("the historical tap has changed files", pin.tap, status)
				}
			}
			if err := os.RemoveAll(tap); err != nil {
				t.Fatal(err)
			}
			if existed {
				if err := os.Rename(backup, tap); err != nil {
					t.Fatal(err)
				}
			}
		})
		if err := os.MkdirAll(tap, 0755); err != nil {
			t.Fatal(err)
		}
		legacyRun(t, tap, nil, "git", "init")
		legacyRun(t, tap, nil, "git", "remote", "add", "origin", "https://github.com/"+pin.tap+".git")
		legacyRun(t, tap, nil, "git", "fetch", "--depth=1", "origin", pin.revision)
		legacyRun(t, tap, nil, "git", "checkout", "--detach", pin.revision)
		if head := strings.TrimSpace(legacyRun(t, tap, nil, "git", "rev-parse", "HEAD")); head != pin.revision {
			t.Fatal("the historical tap differs from its reviewed revision", pin.tap, head)
		}
		prepared = true
	}
	// The release accepts an already installed Ghostty cask. Install that real
	// historical package through the runner's current Brew before nix-homebrew
	// takes over, retaining its native cask receipt throughout migration.
	legacyRun(t, "", []string{"HOMEBREW_NO_INSTALL_FROM_API=1"}, brew, "install", "--cask", "ghostty")

	// Vanilla macOS has Python 3.9, which the release accepts but its pinned
	// setuptools 83 cannot use. New API metadata also uses post-install steps
	// unknown to the release's Brew 6.0.1. Prepare real packages with the current
	// manager and historical definitions, as for Ghostty; never patch the release
	// or reuse a partly failed installation as the healthy legacy baseline.
	prepareLegacyBrewFormulae(t, brew, []string{"HOMEBREW_NO_INSTALL_FROM_API=1", "HOMEBREW_NO_INSTALL_UPGRADE=1", "HOMEBREW_NO_INSTALL_CLEANUP=1", "HOMEBREW_NO_AUTOREMOVE=1"},
		"fontconfig", "yamllint", "python@3.12")
	// Current API dependencies can also contain placeholder-based post-install
	// actions unsupported by Brew 6.0.1. Prepare only tmux's dependencies with
	// current Brew; the unmodified release still installs tmux itself.
	legacyRun(t, "", []string{"HOMEBREW_NO_INSTALL_CLEANUP=1", "HOMEBREW_NO_AUTOREMOVE=1"},
		brew, "install", "--formula", "--only-dependencies", "tmux")
	python := "/opt/homebrew/opt/python@3.12/libexec/bin/python3"
	legacyRun(t, "", nil, python, "-c", "import sys, venv, ensurepip; assert sys.version_info >= (3, 10); print(sys.version); print(sys.executable)")
	legacyRun(t, "", nil, "/opt/homebrew/bin/fc-cache", "--version")
	legacyRun(t, "", nil, "/opt/homebrew/bin/yamllint", "--version")
	// The versioned formula deliberately keeps its unversioned names in libexec.
	// Supply that documented PATH entry without changing the user's login files.
	t.Setenv("PATH", filepath.Dir(python)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func prepareLegacyBrewFormulae(t *testing.T, brew string, environment []string, formulae ...string) {
	t.Helper()
	// An explicit install promotes an existing automatic dependency to a
	// requested package even when Brew installs nothing. Preserve that native
	// ownership evidence; the release already reuses healthy installed tools.
	query := func(ctx context.Context, privileged bool, program string, input []byte, arguments ...string) ([]byte, error) {
		if privileged || program != "brew" || len(input) != 0 {
			return nil, errors.New("invalid prerequisite inventory query")
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, brew, arguments...)
		command.Env = append(os.Environ(), environment...)
		output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{limit: 1 << 20}
		command.Stdout, command.Stderr = &output, &diagnostic
		if err := command.Run(); err != nil {
			return nil, errors.Join(err, ctx.Err(), errors.New(nativeDiagnostic(diagnostic.data.Bytes())))
		}
		return output.data.Bytes(), nil
	}
	ctx := context.Background()
	installed, err := brewInventory(ctx, query)
	if err != nil {
		t.Fatal("read existing prerequisite formulae", err)
	}
	output, err := query(ctx, false, "brew", nil, "--cellar")
	cellar := strings.TrimSpace(string(output))
	if err != nil || !filepath.IsAbs(cellar) || filepath.Clean(cellar) != cellar || strings.ContainsAny(cellar, "\r\n\x00") {
		t.Fatal("invalid prerequisite Cellar", cellar, err)
	}
	missing := []string{}
	for _, formula := range formulae {
		name, err := brewShortName(formula)
		if err != nil {
			t.Fatal(err)
		}
		if pkg, present := installed[name]; present {
			if !strings.EqualFold(pkg.Source, formula) {
				t.Fatal("prerequisite belongs to a different source; preserve it", pkg.Source, formula)
			}
			continue
		}
		if _, err := os.Lstat(filepath.Join(cellar, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("unlisted prerequisite Cellar entry; preserve it", name, err)
		}
		missing = append(missing, formula)
	}
	if len(missing) != 0 {
		legacyRun(t, "", environment, brew, append([]string{"install", "--formula"}, missing...)...)
	}
	after, err := brewInventory(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	for name, pkg := range installed {
		if !legacyPrerequisitePreserved(pkg, after[name]) {
			t.Fatal("prerequisite preparation changed a pre-existing package", name, pkg, after[name])
		}
		if !reflect.DeepEqual(pkg, after[name]) {
			t.Logf("prerequisite preparation added a newer keg beside pre-existing %s: %+v -> %+v", name, pkg, after[name])
		}
	}
}

// Homebrew upgrades an outdated dependency of a formula it installs even with
// HOMEBREW_NO_INSTALL_UPGRADE, which covers only the named formulae. The
// historical snapshot can therefore add a newer keg beside a runner's older one:
// the 2026-10-21 macOS 26 image replaced OpenSSL 3.6.5 with 3.5.9, and the
// 2026-07-26 snapshot's python@3.12 brings openssl@3 3.6.3. Ownership evidence
// stays exact: source, pin, protection, requested/automatic classification,
// every earlier keg and every runtime dependency. Only an added keg, linked and
// healthy, is accepted (owner decision 2026-10-23).
func legacyPrerequisitePreserved(before, after nativePackage) bool {
	if reflect.DeepEqual(before, after) {
		return true
	}
	if after.Name != before.Name || after.Source != before.Source || after.Automatic != before.Automatic ||
		after.Held != before.Held || after.Protected != before.Protected || !after.Healthy {
		return false
	}
	previous, current := strings.Split(before.Version, ","), strings.Split(after.Version, ",")
	if len(current) <= len(previous) {
		return false
	}
	for _, version := range previous {
		if !slices.Contains(current, version) {
			return false
		}
	}
	for _, dependency := range before.Dependencies {
		if !slices.Contains(after.Dependencies, dependency) {
			return false
		}
	}
	return true
}

func TestLegacyPrerequisitePreservationAcceptsOnlyAnAddedKeg(t *testing.T) {
	// Observed on macOS 26 image 20261021 (run 39903643201).
	before := nativePackage{Name: "openssl@3", Version: "3.5.9", Source: "openssl@3", Dependencies: []string{"ca-certificates"}}
	added := before
	added.Version, added.Healthy = "3.5.9,3.6.3", true
	if !legacyPrerequisitePreserved(before, before) || !legacyPrerequisitePreserved(before, added) {
		t.Fatal("an unchanged package or a newer keg beside the original was refused")
	}
	for name, change := range map[string]func(*nativePackage){
		"promoted to requested": func(p *nativePackage) { p.Automatic = !before.Automatic },
		"pin changed":           func(p *nativePackage) { p.Held = true },
		"protection changed":    func(p *nativePackage) { p.Protected = true },
		"different source":      func(p *nativePackage) { p.Source = "other/tap/openssl@3" },
		"original keg removed":  func(p *nativePackage) { p.Version = "3.6.3" },
		"replaced, not added":   func(p *nativePackage) { p.Version = "3.6.3,3.6.4" },
		"no keg added":          func(p *nativePackage) { p.Version = before.Version },
		"left unlinked":         func(p *nativePackage) { p.Healthy = false },
		"dependency dropped":    func(p *nativePackage) { p.Dependencies = nil },
		"missing afterwards":    func(p *nativePackage) { *p = nativePackage{} },
	} {
		changed := added
		changed.Dependencies = slices.Clone(added.Dependencies)
		change(&changed)
		if legacyPrerequisitePreserved(before, changed) {
			t.Error("accepted a changed pre-existing package:", name, changed)
		}
	}
}
