package installer

import (
	"strings"
	"testing"
)

// Linux machines set up before 2026-10-22 hold a completed private-Flatpak
// Obsidian (tool.obsidian) and its owned launcher. With Obsidian still selected,
// Choose tools and Update install the portable build and rewrite that launcher
// in place. Choose tools removes the former Flatpak through the ordinary
// approved removal; Update, which never removes, retains it until then. Nothing
// waits on adoption and the Flatpak infrastructure package stays.
func TestObsidianFlatpakReceiptsMoveToThePortableBuild(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{linux, {OS: "linux", Arch: "arm64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		packages, err := resolveConfigPath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		portable, err := (&ArchiveDriver{Directory: packages, Pins: pins}).Observe(t.Context(), Resource{ID: obsidianPortable}, Receipt{})
		if err != nil || portable.Present || portable.Desired == "" {
			t.Fatal("the portable build must observe as a fresh pinned install", portable, err)
		}
		closure, err := catalog.Closure([]string{"obsidian"}, target)
		if err != nil {
			t.Fatal(err)
		}
		observed := map[string]Observation{}
		for _, id := range closure {
			if r, _ := catalog.Resource(id); r.Action != "" {
				observed[id] = Observation{Present: true, Healthy: true, Provider: "apt", Identity: id, Fingerprint: id, Scope: "machine"}
			}
		}
		observed[obsidianPortable] = portable
		launcher := Observation{Present: true, Healthy: true, Provider: "configuration", Identity: "/home/.local/share/applications/dotfiles-obsidian.desktop", Fingerprint: "flatpak-launcher", Desired: "flatpak-launcher", Scope: "user"}
		current := launcher
		current.Healthy, current.Desired = false, "portable-launcher"
		observed["desktop.obsidian"] = current
		flatpak := Observation{Present: true, Healthy: true, Provider: "flatpak", Identity: "/state/native/flatpak/tool.obsidian/installation:app/md.obsidian.Obsidian/x86_64/stable", Fingerprint: "deployment", Version: strings.Repeat("b", 64), Scope: "user"}
		observed["tool.obsidian"] = flatpak
		infrastructure := Observation{Present: true, Healthy: true, Provider: "apt", Identity: "flatpak", Fingerprint: "flatpak", Scope: "machine"}
		observed["tool.flatpak"] = infrastructure
		state := State{Schema: 1, Context: target, Selected: []string{"obsidian"}, Receipts: map[string]Receipt{
			"tool.obsidian":    {Ownership: "created", Status: "ready", OperationID: strings.Repeat("a", 64), After: flatpak},
			"desktop.obsidian": {Ownership: "created", Status: "ready", OperationID: strings.Repeat("c", 64), After: launcher},
			"tool.flatpak":     {Ownership: "created", Status: "ready", OperationID: strings.Repeat("d", 64), After: infrastructure},
		}}
		for mode, former := range map[string]string{"apply": "remove", "update": "retain"} {
			plan, err := PlanChanges(catalog, target, state, Request{Schema: 1, Mode: mode, Selected: []string{"obsidian"}}, observed, "fixture")
			if err != nil {
				t.Fatal(target, mode, err)
			}
			for id, want := range map[string]string{obsidianPortable: "install", "desktop.obsidian": "repair", "tool.obsidian": former, "tool.flatpak": "retain"} {
				if got := operation(t, plan, id).Action; got != want {
					t.Errorf("%s %s: %s is %q, want %q (%s)", target.Arch, mode, id, got, want, operation(t, plan, id).Reason)
				}
			}
			for _, op := range plan.Operations {
				if op.Action == "pending" {
					t.Errorf("%s %s: %s waits: %s", target.Arch, mode, op.Resource, op.Reason)
				}
			}
			if !strings.Contains(operation(t, plan, obsidianPortable).Reason, "without Chromium's sandbox") {
				t.Error("the preview must disclose the disabled Chromium sandbox")
			}
		}
	}
}
