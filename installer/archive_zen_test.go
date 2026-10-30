package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestZenLauncherKeepsProfileSelectionAndPayloadStable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux browser preparation")
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	pin := pins["tool.zen"]
	for _, generation := range []string{"first install", "replacement install"} {
		root := filepath.Join(t.TempDir(), generation)
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		// This fixture is the upstream process boundary, not a mocked provider.
		for _, name := range []string{"zen", "zen-bin"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nprintf '%s\\n' \"$MOZ_LEGACY_PROFILES\" \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := prepareZen(pin, root); err != nil {
			t.Fatal(err)
		}
		policy, err := os.ReadFile(filepath.Join(root, "distribution", "policies.json"))
		if err != nil || string(policy) != "{\"policies\":{\"DisableAppUpdate\":true}}\n" {
			t.Fatal("missing native updater policy", string(policy), err)
		}
		before, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(filepath.Join(root, "zen"), "https://example.com/a b?c=$d").CombinedOutput()
		if err != nil || string(out) != "1\nhttps://example.com/a b?c=$d\n" {
			t.Fatal(string(out), err)
		}
		// nsProfileLock's documented O_CREAT|O_TRUNC operation must leave the
		// sealed content/mode fingerprint unchanged, under any caller umask.
		lock, err := os.OpenFile(filepath.Join(root, ".parentlock"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		after, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil || before != after {
			t.Fatal("normal startup changed the payload", before, after, err)
		}
		if err := os.WriteFile(filepath.Join(root, ".parentlock"), []byte("personal change"), 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil || changed == before {
			t.Fatal("lock changes must remain protected", err)
		}
	}
}

func TestZenPreparationRevisionBoundToPlatform(t *testing.T) {
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	for revision := 0; revision <= 3; revision++ {
		pin := pins["tool.zen"]
		pin.ZenProfileRevision = revision
		if err := validateZenPin(pin); (err == nil) != (revision == 1) {
			t.Fatal(revision, err)
		}
	}
}
