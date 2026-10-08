package installer

import "testing"

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

func TestLinuxCodeDesktopUsesNativePackageWithoutArchivePin(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		d := &DesktopDriver{Target: Context{OS: "linux", Arch: arch}, Archives: &ArchiveDriver{Pins: map[string]ArchivePin{}}}
		p, err := d.recipe("desktop.vscode")
		if err != nil || p.Command != "/usr/share/code/code" || p.Icon != "/usr/share/pixmaps/vscode.png" {
			t.Fatal("launcher escaped the canonical native package", p, err)
		}
	}
}
