package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// OneDrive marks the folders it syncs read-only, including the operation
// workspaces beside a profile in a redirected Documents folder (observed
// 2026-10-21). Windows refuses to delete a read-only directory, and os.Remove
// clears that attribute only for files.
func markReadOnlyDirectory(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if info, err := os.Lstat(dir); err == nil && info.Mode().Perm()&0200 == 0 {
			_ = os.Chmod(dir, 0755)
		}
	})
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0200 != 0 {
		t.Fatal("fixture directory is not read-only", info, err)
	}
}

func TestRemoveOwnedClearsReadOnlyDirectoryAttribute(t *testing.T) {
	for _, remover := range []string{"owned", "created"} {
		t.Run(remover, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workspace")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			markReadOnlyDirectory(t, dir)
			// Verify the native refusal before testing the policy.
			if err := os.Remove(dir); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				t.Fatal("Windows did not refuse a read-only directory", err)
			}
			var err error
			if remover == "owned" {
				err = removeOwned(dir)
			} else {
				var removed bool
				removed, err = removeEmptyDirectory(dir)
				if !removed && err == nil {
					err = errors.New("empty directory was kept")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("read-only empty directory survived", err)
			}
		})
	}
}

func TestReadOnlyDirectoryRemovalRestoresAttributeWhenOccupied(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workspace")
	writeConfigFixture(t, filepath.Join(dir, "personal.txt"), "keep")
	markReadOnlyDirectory(t, dir)
	if err := removeOwned(dir); err == nil {
		t.Fatal("removed an occupied directory")
	}
	if removed, err := removeEmptyDirectory(dir); removed || err != nil {
		t.Fatal("occupied directory was not kept quietly", removed, err)
	}
	info, err := os.Lstat(dir)
	if err != nil || info.Mode().Perm()&0200 != 0 {
		t.Fatal("read-only attribute was not restored", info, err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "personal.txt")); err != nil || string(data) != "keep" {
		t.Fatal("occupied directory content changed", string(data), err)
	}
}

// A completed publication whose emptied workspace OneDrive marked read-only
// must finish cleanup instead of failing the whole apply on every retry.
func TestProfileFinalizationRemovesReadOnlyWorkspace(t *testing.T) {
	c, d, _ := profileController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	path := d.journalPath(state.Receipts["profile.test"].OperationID)
	data, err := os.ReadFile(path)
	var j profileJournal
	if err == nil {
		err = Decode(data, &j)
	}
	if err != nil || !j.Complete || !j.Sealed {
		t.Fatal("install did not seal its profile journal", j, err)
	}
	j.Sealed = false
	if err := saveDocument(path, j); err != nil {
		t.Fatal(err)
	}
	workspace := j.Entries[0].Workspace
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	markReadOnlyDirectory(t, workspace)
	if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "apply"}, nil); err != nil {
		t.Fatal("read-only empty workspace failed finalization", err)
	}
	if _, err := os.Lstat(workspace); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only workspace survived finalization", err)
	}
	data, err = os.ReadFile(path)
	if err == nil {
		err = Decode(data, &j)
	}
	if err != nil || !j.Sealed || len(j.Preserved) != 0 {
		t.Fatal("finalization did not seal the journal cleanly", j, err)
	}
}

func TestConfigurationFinalizationRemovesReadOnlyWorkspace(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	d.Manifest.Targets[0].Directory, d.Manifest.Targets[0].Source = true, "tree"
	writeConfigFixture(t, filepath.Join(d.Repository, "tree", "first"), "first")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := d.Catalog.Resource("config.test")
	receipt := state.Receipts[r.ID]
	receipt.Status, receipt.OperationID = "in-progress", strings.Repeat("c", 64)
	writeConfigFixture(t, filepath.Join(d.Repository, "tree", "second"), "second")
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "update", Observed: observed}, receipt); err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal(r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	// The previous payload still sits inside; Windows does not apply a
	// directory's read-only attribute to its children.
	markReadOnlyDirectory(t, j.Entries[0].Workspace)
	if _, err := d.FinishTransaction(context.Background(), Plan{}, nil); err != nil {
		t.Fatal("read-only workspace failed finalization", err)
	}
	if _, err := os.Lstat(j.Entries[0].Workspace); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only workspace survived finalization", err)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "second")); err != nil || string(data) != "second" {
		t.Fatal("cleanup changed the current configuration", string(data), err)
	}
}
