package installer

import (
	"context"
	"os"
	"runtime"
	"testing"
)

// Read-only native proof: these checks execute only version/discovery commands
// and inspect OS binaries. They never install developer tools, compile output or
// read/change clipboard contents. Fresh CLT provisioning has a separate fixture.
func TestNativeMacOSPrerequisitesReadOnly(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" || os.Getenv("DOTFILES_TEST_NATIVE_INVENTORY") != "1" && os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicit Apple Silicon native inventory inspection")
	}
	d, err := configureMacOSPrerequisites(NativePlatform{Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tool.compiler", "tool.make", "tool.clipboard"} {
		o, err := d.Observe(context.Background(), Resource{ID: id}, Receipt{})
		if err != nil || !o.Present || !o.Healthy || o.Pending != "" || o.CompletedOperation != "" {
			t.Fatal(id, o, err)
		}
		t.Logf("%s: selected native commands verified; reused without ownership", id)
	}
}
