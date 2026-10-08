//go:build darwin || linux

package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestProfileCopyRefusesAnUnpreservableOwner(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can preserve the fixture owner; this assertion requires an ordinary account")
	}
	// This public protocol-number table is a readable, root-owned fixture.
	// Unlike protected executables on macOS it has no restricted file flags
	// that could fail the copy before ownership is checked.
	source := "/etc/protocols"
	var before unix.Stat_t
	if err := unix.Stat(source, &before); err != nil {
		if os.IsNotExist(err) {
			t.Skip("public root-owned fixture is unavailable on this image")
		}
		t.Fatal(err)
	}
	if before.Uid != 0 {
		t.Fatal("fixture must have a different, privileged owner")
	}
	destination := filepath.Join(t.TempDir(), "staged")
	if err := copyProfileMetadata(context.Background(), source, destination); err == nil {
		t.Fatal("silently changed a root-owned file into a user-owned file")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatal("refused ownership copy left a profile copy behind", err)
	}
	var after unix.Stat_t
	if err := unix.Stat(source, &after); err != nil || before.Uid != after.Uid || before.Gid != after.Gid {
		t.Fatal("source ownership changed", err)
	}
}

func TestProfilePublicationExplainsPermissionRecovery(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires an ordinary account that cannot preserve root ownership")
	}
	source := "/etc/protocols"
	before, err := snapshotTree(source, maxProfileBytes, 1)
	if err != nil {
		t.Fatal(err)
	}
	if before.Kind == "absent" {
		t.Skip("public root-owned fixture is unavailable on this image")
	}
	var owner unix.Stat_t
	if err := unix.Stat(source, &owner); err != nil || owner.Uid != 0 {
		t.Fatal("fixture must be root-owned", err)
	}
	root := t.TempDir()
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	d := &ProfileDriver{Directory: root}
	e := profilePublication{Path: source, Workspace: filepath.Join(root, "restore"), Before: before, After: configSnapshot{Kind: "file"}, Content: []byte("never publish this")}
	err = d.publishProfile(context.Background(), &profileJournal{}, &e)
	for _, want := range []string{source, "original profile remains active", "permissions", "retry the saved operation"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("permission refusal lacks actionable recovery %q: %v", want, err)
		}
	}
	if e.Staged || e.Moved || e.Published {
		t.Fatal("permission refusal advanced publication")
	}
	if err := verifyConfigSnapshot(source, before); err != nil {
		t.Fatal("refused publication changed the source", err)
	}
	if _, err := os.Lstat(filepath.Join(e.Workspace, "next")); !os.IsNotExist(err) {
		t.Fatal("refused publication left a staged copy", err)
	}
}
