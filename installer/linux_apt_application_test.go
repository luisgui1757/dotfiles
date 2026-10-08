//go:build linux

package installer

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestNativeControllerConnectsAPTAndKeepsSelectionStatePrivate(t *testing.T) {
	f := newLinuxAPTFixture(t)
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Dir(f.driver.Directory)
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config")}
	c, err := NewNativeController(repository, "test", filepath.Join(home, "state"), NativePlatform{Context: linux, Distro: "debian", Family: "apt", Libc: "glibc", APT: &f.driver.Location}, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	r, _ := c.Catalog.Resource("tool.git")
	p, err := d.provider(r)
	if err != nil || p != d.APT || d.APT == nil {
		t.Fatal("APT was not connected", err)
	}
	selected, err := d.ForSelection([]string{"tool.git"}, State{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected.(*NativeDriver).APT.Keep, []string{"git"}) || len(d.APT.Keep) != 0 {
		t.Fatal("selection leaked mutable native package retention state")
	}
	d.APTIssue = "sudo is unavailable"
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil || !o.Unknown || o.Pending != d.APTIssue {
		t.Fatal("APT discovery issue was not resource-local", o, err)
	}
	archive, _ := c.Catalog.Resource("tool.fd")
	if _, err := d.provider(archive); err != nil {
		t.Fatal("APT issue disabled independent archives", err)
	}
}
