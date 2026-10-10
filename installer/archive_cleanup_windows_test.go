package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestArchiveLockedOldFileLeavesCleanupRetryable(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	old, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(filepath.Join(old, "tool"))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		}
	})
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "version two", Mode: 0755}}), "tar.gz")
	pin.Version = "2.0"
	d.Pins["tool.shared"], d.Client = pin, client
	request := Request{Schema: 1, Mode: "update"}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "cleanup remains") {
		t.Fatal("locked cleanup reported completion or permanent preservation", err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction == nil {
		t.Fatal("lost retry intent", err)
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update", Retry: true})
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Fatal("old generation not cleaned after unlock", err)
	}
}
