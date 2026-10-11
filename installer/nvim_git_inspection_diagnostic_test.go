package installer

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// No download, command execution, retry or changed-file content: preserve the
// failed transaction and inspect only its exact managed Git provenance.
func nativeGitInspectionDiagnostic(t *testing.T, archives *ArchiveDriver, statePath, home string) {
	t.Helper()
	state, err := LoadState(statePath, home)
	if err != nil {
		t.Logf("Git inspection diagnostic state: %v", err)
		return
	}
	if state.Transaction != nil {
		t.Logf("Git inspection diagnostic transaction: next=%d in_flight=%q git_pending=%q", state.Transaction.Next, state.Transaction.InFlight, state.Transaction.Pending["tool.git"])
	}
	for _, id := range []string{"tool.git", "nvim.sync"} {
		receipt := state.Receipts[id]
		t.Logf("Git inspection diagnostic %s receipt: status=%q ownership=%q operation=%s after_fingerprint=%s completed=%s", id, receipt.Status, receipt.Ownership, receipt.OperationID, receipt.After.Fingerprint, receipt.After.CompletedOperation)
	}
	current, err := archives.current("tool.git")
	if err != nil || current.Version == "" {
		t.Logf("Git inspection diagnostic current: %+v error=%v", current, err)
		return
	}
	version, err := archives.version("tool.git", current.Version)
	if err != nil {
		t.Logf("Git inspection diagnostic saved version: %v", err)
		return
	}
	t.Logf("Git inspection diagnostic current: version=%s operation=%s desired_pin=%s saved_pin=%s saved_snapshot=%+v", current.Version, current.Operation, archivePinID(archives.Pins["tool.git"]), archivePinID(version.Pin), version.Payload)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	observed, err := archives.Observe(ctx, Resource{ID: "tool.git"}, state.Receipts["tool.git"])
	t.Logf("Git inspection diagnostic subsequent observation: present=%v healthy=%v unknown=%v fingerprint=%s pending=%q error=%v", observed.Present, observed.Healthy, observed.Unknown, observed.Fingerprint, observed.Pending, err)
	actual, err := snapshotTree(filepath.Join(archives.versionDirectory("tool.git", current.Version), "payload"), maxPackageBytes, maxPackageEntries)
	t.Logf("Git inspection diagnostic subsequent payload snapshot: actual=%+v saved=%+v error=%v", actual, version.Payload, err)
}
