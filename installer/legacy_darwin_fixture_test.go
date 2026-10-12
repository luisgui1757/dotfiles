package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
}
