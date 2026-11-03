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

func TestGitHandoverExecutesWhileGitStaysSelected(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	original := "[user]\n\tname = Original baseline\n"
	writeConfigFixture(t, target, original)
	dispatchApproved(t, legacyGitController(c), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
	plan, result, err := executeApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil || result.Plan.ID != plan.ID || result.Status != "needs-action" {
		t.Fatal("approved Git handover could not execute", result, err)
	}
	if op := operation(t, plan, gitSettingsResource); op.Action != "pending" || strings.Count(op.Reason, "An earlier installer records the whole Git file") != 1 {
		t.Fatal("handover was not explained exactly once", op)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != original {
		t.Fatal("unchanged legacy file did not restore its first baseline", string(data), err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--get", "user.name"); got != "Original baseline" {
		t.Fatal("resumed handover lost the personal baseline", got)
	}
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--bool", "--get", "pull.rebase"); got != "true" {
		t.Fatal("resumed handover did not enable scoped defaults", got)
	}
}

// Retry preserves a declined choice, so the guidance must lead through
// abandonment before the optional restoration can be reviewed again.
func TestGitDeclinedHandoverWhileSelectedFollowsItsGuidance(t *testing.T) {
	c, target, original, changed := changedLegacyGitFixture(t)
	run := func(answers ...[]string) Result {
		t.Helper()
		ui := &scriptedInteraction{t: t, accept: true, answers: answers}
		result, err := c.Run(context.Background(), ui)
		if err != nil || len(ui.answers) != 0 {
			t.Fatal(result, err)
		}
		return result
	}
	declined := run([]string{"apply"}, []string{"git"}, []string{})
	if declined.Status != "needs-action" || !strings.Contains(declined.Message, "If you keep the earlier file, abandon the unfinished setup run") {
		t.Fatal("declined handover did not explain how to continue", declined)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != changed {
		t.Fatal("declined handover changed personal settings", string(data), err)
	}
	if result := run([]string{"abandon"}); result.Status != "ready" {
		t.Fatal(result)
	}
	if result := run([]string{"apply"}, []string{"git"}, []string{"config.git"}); result.Status != "needs-action" {
		t.Fatal("restoration with Git selected did not hand over", result)
	}
	if result := run([]string{"retry"}); result.Status != "ready" {
		t.Fatal("resume after restoration did not enable scoped defaults", result)
	}
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--get", "user.name"); got != "Original personal baseline" {
		t.Fatal("handover lost the first baseline", got)
	}
	data, err := os.ReadFile(target)
	if err != nil || !strings.HasPrefix(strings.TrimLeft(string(data), "\n"), "# >>> dotfiles:integration.git >>>") || !strings.HasSuffix(string(data), original) {
		t.Fatal("scoped defaults did not prepend to the restored baseline", string(data), err)
	}
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

// A changed legacy whole file plus the user's approved restoration, interrupted
// by a real filesystem failure. Recovery must return to the optional choice.
func changedLegacyGitFixture(t *testing.T) (Controller, string, string, string) {
	t.Helper()
	c, _, target := gitSettingsFixture(t)
	original := "[user]\n\tname = Original personal baseline\n"
	writeConfigFixture(t, target, original)
	dispatchApproved(t, legacyGitController(c), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	changed := "[user]\n\temail = later@example.invalid\n"
	writeConfigFixture(t, target, changed)
	return c, target, original, changed
}

func requireOptionalLegacyRestoration(t *testing.T, c Controller, target, original, changed string) {
	t.Helper()
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if op := operation(t, plan, "config.git"); op.Action != "retain" || op.Observed.Pending != "" {
		t.Fatal("interrupted restoration became mandatory or lost its optional choice", op)
	}
	ui := &scriptedInteraction{t: t, accept: true, answers: [][]string{{"apply"}, {}, {}}}
	result, err := c.Run(context.Background(), ui)
	if err != nil || result.Status != "ready" || len(ui.answers) != 0 {
		t.Fatal("declining the optional restoration did not complete", result, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != changed {
		t.Fatal("declined restoration changed personal settings", string(data), err)
	}
	ui = &scriptedInteraction{t: t, accept: true, answers: [][]string{{"apply"}, {}, {"config.git"}}}
	result, err = c.Run(context.Background(), ui)
	if err != nil || result.Status != "ready" || len(ui.answers) != 0 {
		t.Fatal("approved restoration was not available after recovery", result, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != original {
		t.Fatal("approved restoration did not return the first baseline", string(data), err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	preserved := false
	for _, path := range state.Receipts["config.git"].After.Preserved {
		if data, err := os.ReadFile(path); err == nil && string(data) == changed {
			preserved = true
		}
	}
	if !preserved {
		t.Fatal("changed file was not preserved separately", state.Receipts["config.git"])
	}
}

// The persisted crash shape follows the payload's publication rename; only the
// following phase write is rewound. Restoration then uses real inverse moves.
func restoredUncertainFirstPublication(t *testing.T, original string) (*ConfigDriver, Resource, Receipt, configJournal, string) {
	t.Helper()
	_, d, destination := nativeConfigFixture(t, "copy")
	action := "install"
	if original != "" {
		action = "adopt"
		writeConfigFixture(t, destination, original)
	}
	r, _ := d.Catalog.Resource("config.test")
	before, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{Ownership: "uncertain", Before: before, Adopted: original != "", Status: "in-progress", OperationID: strings.Repeat("b", 64), Recovery: filepath.Join("recovery", strings.Repeat("a", 64), r.ID)}
	if _, err := d.Apply(context.Background(), r, Operation{Resource: r.ID, Action: action, Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal(r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	j.Complete, j.Entries[0].Phase = false, "publishing"
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	restoring, err := d.ObserveRestore(context.Background(), r.ID, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RestoreResource(context.Background(), r.ID, restoring, receipt); err != nil {
		t.Fatal(err)
	}
	r.ReplacedBy, receipt.Status = "integration.git", "needs-action"
	return d, r, receipt, j, destination
}

func TestGitRestoredFirstPublicationReleasesAfterPersonalWrite(t *testing.T) {
	for _, original := range []string{"user baseline", ""} {
		t.Run(map[bool]string{true: "adopted", false: "absent"}[original != ""], func(t *testing.T) {
			d, r, receipt, _, destination := restoredUncertainFirstPublication(t, original)
			personal := "[user]\n\temail = written-after-restoration@example.invalid\n"
			writeConfigFixture(t, destination, personal)
			after, err := d.Observe(context.Background(), r, receipt)
			if err != nil || after.Pending != "" || after.RestoredOperation != receipt.OperationID {
				t.Fatal("a later personal write revoked completed restoration proof", after, err)
			}
			if data, err := os.ReadFile(destination); err != nil || string(data) != personal {
				t.Fatal("releasing bookkeeping changed the personal file", string(data), err)
			}
		})
	}
}

func TestGitRestoredFirstPublicationKeepsBookkeepingWhilePayloadOccupiesTarget(t *testing.T) {
	d, r, receipt, j, destination := restoredUncertainFirstPublication(t, "")
	if err := moveConfigExclusive(filepath.Join(j.Entries[0].Workspace, "next"), destination); err != nil {
		t.Fatal(err)
	}
	after, err := d.Observe(context.Background(), r, receipt)
	if err != nil || after.Pending == "" || after.RestoredOperation != "" {
		t.Fatal("released bookkeeping while the managed payload occupied the target", after, err)
	}
}

func TestGitLegacyRestorationAbandonedBeforeJournalStaysOptional(t *testing.T) {
	c, target, original, changed := changedLegacyGitFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"config.git"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	id, err := digest(struct{ Plan, Resource, Action string }{plan.ID, "config.git", "remove"})
	if err != nil {
		t.Fatal(err)
	}
	journal := c.Driver.(*NativeDriver).Configurations.journalPath(id)
	// Durable engine intent exists, but the provider journal cannot be written.
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not stop before the provider journal")
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
	requireOptionalLegacyRestoration(t, c, target, original, changed)
}
