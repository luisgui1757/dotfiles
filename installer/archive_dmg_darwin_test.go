package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// Apple's tar otherwise synthesizes AppleDouble files for mounted xattrs. The
// bounded Go reader treats those as real files, breaking sealed app signatures.
func TestNativeDMGPreservesBundleFilesWithoutSyntheticAppleDouble(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_DESKTOP_ARCHIVES") != "1" {
		t.Skip("requires an opted-in disposable Mac for a private disk-image mount")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	image := filepath.Join(root, "fixture.dmg")
	run := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("/usr/bin/hdiutil", args...).CombinedOutput(); err != nil {
			t.Fatalf("private disk image %v: %v\n%s", args, err, output)
		}
	}
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	run("create", "-size", "64m", "-fs", "APFS", "-type", "UDIF", image)
	run("attach", "-nobrowse", "-mountpoint", source, image)
	attached := true
	t.Cleanup(func() {
		if attached {
			if output, err := exec.Command("/usr/bin/hdiutil", "detach", source).CombinedOutput(); err != nil {
				t.Errorf("detach private source image: %v\n%s", err, output)
			}
		}
	})
	// Write inside the mounted fixture: hdiutil -srcfolder itself interprets
	// ._ names as metadata and would discard this real-file preservation case.
	bundle := filepath.Join(source, "Fixture.app")
	for _, name := range []string{"Contents/Resources/note.txt", "Contents/Resources/._actual-file"} {
		path := filepath.Join(bundle, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("original "+name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := unix.Setxattr(filepath.Join(bundle, "Contents/Resources/note.txt"), "com.apple.metadata:pr87-test", []byte("fixture-metadata"), 0); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(bundle, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	original, err := inspectTree(bundle, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	run("detach", source)
	attached = false
	file, err := os.Open(image)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "extracted")
	if err := extractArchive(context.Background(), file, info.Size(), ArchivePin{Format: "dmg", AppBundle: "Fixture.app", DMGRevision: 1}, destination, ""); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotTree(filepath.Join(destination, "Fixture.app"), maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		extracted, extractedErr := inspectTree(filepath.Join(destination, "Fixture.app"), maxPackageBytes, maxPackageEntries)
		t.Fatalf("native disk-image transport changed actual bundle files: before=%+v after=%+v err=%v; original=%+v; extracted=%+v (%v)", before, after, err, original, extracted, extractedErr)
	}
}
