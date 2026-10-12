package installer

import (
	"slices"
	"testing"
)

func TestRetiredWezTermHasNoActiveSelectionOrPublication(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := DefaultConfigManifest(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"wezterm", "tool.wezterm", "config.wezterm", "desktop.wezterm"} {
		if _, ok := catalog.Resource(id); ok {
			t.Fatal("retired resource remains selectable", id)
		}
		for _, target := range manifest.Targets {
			if target.Resource == id {
				t.Fatal("retired configuration still publishes", target)
			}
		}
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			if _, err := catalog.Closure([]string{"wezterm"}, target); err == nil {
				t.Fatal("retired terminal can still be selected")
			}
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := pins["tool.wezterm"]; ok {
				t.Fatal("retired terminal still has an active archive")
			}
			driver := &DesktopDriver{Target: target, Archives: &ArchiveDriver{Pins: pins}}
			if _, err := driver.recipe("desktop.wezterm"); err == nil {
				t.Fatal("retired terminal still has a launcher recipe")
			}
			terminal := "ghostty"
			if target.OS == "windows" {
				terminal = "windows-terminal"
			}
			for _, id := range []string{terminal, "vscode"} {
				closure, err := catalog.Closure([]string{id}, target)
				if err != nil || !slices.Contains(closure, "tool.font") {
					t.Fatal("remaining desktop selection lost shared font provisioning", id, closure, err)
				}
			}
		})
	}
}
