package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Execute must reach providers with the exact plan that was previewed. These
// regressions approve and execute instead of stopping at preview.
func executeApproved(t *testing.T, c Controller, request Request) (Plan, Result, error) {
	t.Helper()
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := c.Dispatch(context.Background(), request)
	return preview.Plan, result, err
}

func TestGitDiscoveryFailureDoesNotBlockExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX dangling directory link")
	}
	c, source, target := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err := os.Rename(filepath.Dir(target), filepath.Dir(target)+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(c.Home, "missing-personal-config"), filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	folders := c.Driver.(*NativeDriver).Configurations.Folders
	rebuilt, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt.Catalog = c.Catalog
	plan, result, err := executeApproved(t, rebuilt, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil || result.Plan.ID != plan.ID || result.Status != "ready" {
		t.Fatal("resource-local Git discovery failure blocked an unrelated execution", result, err)
	}
	if op := operation(t, plan, gitSettingsResource); op.Action != "retain" || !strings.Contains(op.Observed.Pending, "missing-personal-config") {
		t.Fatal("uninspectable Git target was not retained with its real cause", op)
	}
}

func TestGitSavedCheckoutTargetCanBeRemoved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX redirected directory fixture")
	}
	c, source, target := gitSettingsFixture(t)
	defaults, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(source), filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	// An earlier build bound this redirect without its checkout-overlap check
	// and published a self-include into the defaults source.
	earlier := c.Driver.(*NativeDriver)
	earlier.shellSelection = nil
	earlier.Profiles.Targets[gitSettingsResource][0].Path = source
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if data, err := os.ReadFile(source); err != nil || bytes.Equal(data, defaults) {
		t.Fatal("fixture did not reproduce the earlier checkout publication", err)
	}
	folders := earlier.Configurations.Folders
	current, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	current.Catalog = c.Catalog
	plan, result, err := executeApproved(t, current, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil || result.Plan.ID != plan.ID || result.Status != "ready" {
		t.Fatal("saved checkout target could not be cleaned up", result, err)
	}
	if op := operation(t, plan, gitSettingsResource); op.Action != "remove" || strings.Count(op.Observed.ApplyBlocked, "inside the dotfiles checkout") != 1 {
		t.Fatal("checkout overlap was not explained exactly once", op)
	}
	if data, err := os.ReadFile(source); err != nil || !bytes.Equal(data, defaults) {
		t.Fatal("removal did not restore the checkout defaults", string(data), err)
	}
}

// A changed current discovery cannot strand removal of the saved target.
func TestGitRemovalUsesSavedTargetWhenDiscoveryIsUnusable(t *testing.T) {
	for _, discovery := range []string{"relative", "dangling"} {
		t.Run(discovery, func(t *testing.T) {
			if discovery == "dangling" && runtime.GOOS == "windows" {
				t.Skip("POSIX dangling directory link")
			}
			c, source, target := gitSettingsFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
			gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email", "fixture@example.invalid")
			folders := c.Driver.(*NativeDriver).Configurations.Folders
			folders.GitIssue = "Git HOME must resolve to an absolute path"
			folders.GitConfig = filepath.Join("relative", "git", "config")
			if discovery == "dangling" {
				folders.GitIssue = ""
				moved := filepath.Join(c.Home, "offline-xdg")
				if err := os.Symlink(filepath.Join(c.Home, "missing-volume"), moved); err != nil {
					t.Fatal(err)
				}
				folders.GitConfig = filepath.Join(moved, "git", "config")
			}
			rebuilt, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
			if err != nil {
				t.Fatal(err)
			}
			rebuilt.Catalog = c.Catalog
			check, err := rebuilt.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "needs-action" || strings.Contains(check.Message, "exactly one target") {
				t.Fatal("current discovery problem was not explained", check, err)
			}
			if discovery == "relative" && !strings.Contains(check.Message, folders.GitIssue) {
				t.Fatal("folder discovery cause was dropped", check.Message)
			}
			plan, result, err := executeApproved(t, rebuilt, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if err != nil || result.Status != "ready" || operation(t, plan, gitSettingsResource).Action != "remove" {
				t.Fatal("saved Git target could not be removed", plan, result, err)
			}
			data, err := os.ReadFile(target)
			if err != nil || bytes.Contains(data, []byte("dotfiles:integration.git")) || !bytes.Contains(data, []byte("fixture@example.invalid")) {
				t.Fatal("removal did not restore the saved target", string(data), err)
			}
		})
	}
}

func TestGitFolderDiscoveryCauseIsReportedWithBindingFailure(t *testing.T) {
	c, source, _ := gitSettingsFixture(t)
	folders := c.Driver.(*NativeDriver).Configurations.Folders
	folders.GitIssue = "Git HOME must resolve to an absolute path"
	folders.GitConfig = filepath.Join("relative", "git", "config")
	rebuilt, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt.Catalog = c.Catalog
	plan, err := rebuilt.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil {
		t.Fatal(err)
	}
	op := operation(t, plan, gitSettingsResource)
	if op.Action != "pending" || !strings.Contains(op.Reason, "canonical paths") || !strings.Contains(op.Reason, folders.GitIssue) {
		t.Fatal("Git discovery explanation lost its folder cause", op)
	}
}
