package installer

import (
	"slices"
	"testing"
)

func TestLinuxCodeUsesCanonicalNativePackage(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	r, ok := catalog.Resource("tool.vscode")
	if !ok || r.Bindings["linux"] != (Binding{Provider: "apt", Package: "code"}) {
		t.Fatal("Linux Code must use its canonical native package so distro sandbox policy can attach", r.Bindings["linux"])
	}
	if len(r.PlatformRequires["linux"]) != 0 {
		t.Fatal("APT owns Code's native dependency resolution", r.PlatformRequires["linux"])
	}
	for _, arch := range []string{"amd64", "arm64"} {
		pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "linux", Arch: arch}, Distro: "ubuntu", Family: "apt", Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		if _, private := pins["tool.vscode"]; private {
			t.Fatal("Linux Code cannot launch a private archive outside the distro sandbox profile")
		}
	}
}

func TestLinuxCodeUsesPackageLauncherWithoutPrivateShellAttachment(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		closure, err := catalog.Closure([]string{"vscode"}, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"desktop.vscode", "integration.shells"} {
			if slices.Contains(closure, id) != (target.OS != "linux") {
				t.Fatal(target, id, closure)
			}
		}
	}
	d := &DesktopDriver{Target: linux, Archives: &ArchiveDriver{}}
	if _, err := d.recipe("desktop.vscode"); err == nil {
		t.Fatal("Linux Code has a redundant private launcher")
	}
}
