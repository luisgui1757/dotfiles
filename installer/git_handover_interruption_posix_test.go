//go:build darwin || linux

package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Directory permissions interrupt the real provider at exact physical
// boundaries; root bypasses them, so these require an ordinary account.
func interruptLegacyGitRestoration(t *testing.T, c Controller, readOnly string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the directory permission used to interrupt publication")
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"config.git"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })
	request.ExpectedPlan = plan.ID
	_, err = c.Dispatch(context.Background(), request)
	if chmodErr := os.Chmod(readOnly, 0700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if err == nil {
		t.Fatal("fixture did not interrupt the approved restoration")
	}
}

func TestGitLegacyRestorationAbandonedBeforeAnyMoveStaysOptional(t *testing.T) {
	c, target, original, changed := changedLegacyGitFixture(t)
	// Staging cannot create the operation workspace beside the target.
	interruptLegacyGitRestoration(t, c, filepath.Dir(target))
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := c.Catalog.Resource("config.git")
	j, err := c.Driver.(*NativeDriver).Configurations.readJournal(r, state.Receipts["config.git"])
	if err != nil || j.Complete || j.Entries[0].Phase != "" {
		t.Fatal("fixture did not stop after durable intent and before any move", j, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
	requireOptionalLegacyRestoration(t, c, target, original, changed)
}

func TestGitLegacyRestorationRestoredAfterMoveStaysOptional(t *testing.T) {
	c, target, original, changed := changedLegacyGitFixture(t)
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := c.Catalog.Resource("config.git")
	baseline, err := c.Driver.(*NativeDriver).Configurations.readBaseline(r, state.Receipts["config.git"])
	if err != nil {
		t.Fatal(err)
	}
	// The current file moves aside, then the first baseline cannot leave its
	// private workspace. Restoration returns both files to their prior places.
	interruptLegacyGitRestoration(t, c, filepath.Dir(baseline.Targets[0].Backup))
	state, err = LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	j, err := c.Driver.(*NativeDriver).Configurations.readJournal(r, state.Receipts["config.git"])
	if err != nil || j.Complete || j.Entries[0].Phase != "publishing" {
		t.Fatal("fixture did not stop after moving the current file", j, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
	requireOptionalLegacyRestoration(t, c, target, original, changed)
}
