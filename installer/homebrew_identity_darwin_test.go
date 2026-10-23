package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// A remount reproduces the device-number change seen after an actual guest
// reboot. Run only on an opted-in disposable Mac; both images are test-owned.
func TestNativeHomebrewIdentitySurvivesVolumeRemount(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires an explicitly opted-in disposable Mac")
	}
	f := newHomebrewBootstrapFixture(t)
	run := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("/usr/bin/hdiutil", args...).CombinedOutput(); err != nil {
			t.Fatalf("hdiutil %v: %v\n%s", args, err, output)
		}
	}
	type volume struct {
		image, mount string
		attached     bool
	}
	create := func(name string) *volume {
		t.Helper()
		v := &volume{image: filepath.Join(f.d.Home, name+".sparseimage"), mount: filepath.Join(f.d.Home, name)}
		if err := os.Mkdir(v.mount, 0700); err != nil {
			t.Fatal(err)
		}
		run("create", "-size", "512m", "-fs", "APFS", "-type", "SPARSE", "-volname", name, "-o", v.image)
		t.Cleanup(func() {
			if v.attached {
				if output, err := exec.Command("/usr/bin/hdiutil", "detach", v.mount).CombinedOutput(); err != nil {
					t.Errorf("detach test volume: %v\n%s", err, output)
				}
			}
		})
		return v
	}
	attach := func(v *volume) {
		t.Helper()
		run("attach", "-nobrowse", "-owners", "on", "-mountpoint", v.mount, v.image)
		v.attached = true
	}
	retained, other := create("retained"), create("other")
	attach(retained)
	f.d.prefix = filepath.Join(retained.mount, "prefix")
	installed, err := f.apply("install", "a")
	if err != nil || !installed.Healthy {
		t.Fatal("initial bootstrap", installed, err)
	}
	var before, after unix.Stat_t
	if err := unix.Stat(f.d.prefix, &before); err != nil {
		t.Fatal(err)
	}
	state, err := snapshotConfig(f.d.Directory)
	if err != nil {
		t.Fatal(err)
	}
	run("detach", retained.mount)
	retained.attached = false
	attach(other) // Occupy the former device slot before remounting the same volume.
	attach(retained)
	if err := unix.Stat(f.d.prefix, &after); err != nil {
		t.Fatal(err)
	}
	if before.Dev == after.Dev || before.Ino != after.Ino {
		t.Fatalf("fixture did not reproduce remount identity: %d:%d -> %d:%d", before.Dev, before.Ino, after.Dev, after.Ino)
	}
	observed, err := f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
	if err != nil || !observed.Healthy || observed.ApplyBlocked != "" || observed.CompletedOperation != installed.CompletedOperation {
		t.Fatalf("same retained volume became unusable after remount: %+v %v", observed, err)
	}
	if err := verifyConfigSnapshot(f.d.Directory, state); err != nil {
		t.Fatal("read-only rediscovery rewrote ownership evidence", err)
	}
	t.Logf("retained prefix survives device-number change %d:%d -> %d:%d without rewriting state", before.Dev, before.Ino, after.Dev, after.Ino)

	// An outside volume at the same path is not the retained installation.
	run("detach", retained.mount)
	retained.attached = false
	run("detach", other.mount)
	other.attached = false
	other.mount = retained.mount
	attach(other)
	f.installManager()
	// Isolate the volume component: a private ownership fixture with the new
	// inode must still refuse this different UUID. No real installer state is
	// involved, and the production Observe path remains read-only.
	record, err := f.d.record()
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Stat(f.d.prefix, &after); err != nil {
		t.Fatal(err)
	}
	volumeIdentity, _, found := strings.Cut(record.Directory, ":inode:")
	if !found {
		t.Fatal("missing volume identity", record.Directory)
	}
	record.Directory = fmt.Sprintf("%s:inode:%d", volumeIdentity, after.Ino)
	if err := saveDocument(filepath.Join(f.d.Directory, "created.json"), record); err != nil {
		t.Fatal(err)
	}
	state, err = snapshotConfig(f.d.Directory)
	if err != nil {
		t.Fatal(err)
	}
	observed, err = f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
	if err != nil || observed.ApplyBlocked == "" || observed.CompletedOperation != "" {
		t.Fatal("replacement volume acquired retained ownership", observed, err)
	}
	if err := verifyConfigSnapshot(f.d.Directory, state); err != nil {
		t.Fatal("outside-volume refusal rewrote ownership evidence", err)
	}
}

func TestHomebrewLegacyDeviceIdentityCannotAcquireVolumeIdentity(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	if _, err := f.apply("install", "a"); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(f.d.prefix)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := configDirectoryIdentity(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	record, err := f.d.record()
	if err != nil || !strings.HasPrefix(record.Directory, "volume:") {
		t.Fatal("new retained record lacks durable volume identity", record, err)
	}
	record.Directory = legacy // Actual former persisted shape, in a private fixture.
	record.Schema = 1
	if err := saveDocument(filepath.Join(f.d.Directory, "created.json"), record); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(f.d.Directory)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
	if err != nil || observed.ApplyBlocked != errHomebrewBootIdentity.Error() || observed.CompletedOperation != "" {
		t.Fatal("legacy inode evidence silently acquired a volume UUID", observed, err)
	}
	if err := verifyConfigSnapshot(f.d.Directory, before); err != nil {
		t.Fatal("legacy observation rewrote ownership evidence", err)
	}
}
