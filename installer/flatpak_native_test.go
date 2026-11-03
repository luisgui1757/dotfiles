package installer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativeFlatpakApplicationLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_FLATPAK") != "1" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicitly opted-in disposable Linux Flatpak acceptance")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Minute)
	defer cancel()
	platform := nativePlatform(t)
	var err error
	platform.APT, err = DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Data: filepath.Join(home, "data"), Zsh: home}
	c, err := NewNativeController(repository, "native-flatpak", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	d.session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(d.session.Directory) }
	// Obsidian no longer selects this private Flatpak; Linux machines set up
	// before 2026-10-22 still hold one. A test-only capability recreates that
	// installation so its observation and removal stay proven natively.
	const earlier = "obsidian-flatpak-fixture"
	catalog := *c.Catalog
	catalog.Resources = append(slices.Clone(catalog.Resources), Resource{ID: earlier, Name: "Earlier Obsidian Flatpak", Capability: true, Category: "Desktop apps", Platforms: []string{"linux"}, Requires: []string{"tool.obsidian"}, Scope: "user"})
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Catalog, d.Catalog = &catalog, &catalog
	witness := filepath.Join(home, ".var", "app", "md.obsidian.Obsidian", "personal-note")
	writeConfigFixture(t, witness, "personal app data\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{earlier}})
	r, _ := c.Catalog.Resource("tool.obsidian")
	app, err := d.Flatpak.verifiedApplication(ctx)
	if err != nil || !app.Healthy {
		t.Fatal(app, err)
	}
	// Enter the real app sandbox and consume its application/runtime files.
	// Interactive editing is separately exercised in desktop acceptance.
	// Zypak/Electron launches renderer subprocesses through Flatpak's portal.
	// A shell alone misses an invalid installation lookup used by that portal.
	command := d.Flatpak.command(r, "run", "--user", "--command=/bin/sh", d.Flatpak.ref(r), "-c", "test -x /app/obsidian && flatpak-spawn --sandbox /usr/bin/true")
	probe := exec.CommandContext(ctx, command.Program, command.Arguments...)
	probe.Env = append(os.Environ(), command.Environment...)
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatal(err, nativeDiagnostic(output))
	}
	before, err := os.ReadFile(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || checked.Status != "ready" {
		t.Fatal(checked, err)
	}
	after, err := os.ReadFile(c.StatePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("Check changed state", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	updated, err := d.Flatpak.verifiedApplication(ctx)
	if err != nil || !updated.Healthy {
		t.Fatal(updated, err)
	}
	t.Log("Flathub commits before/after", app.Version, updated.Version)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	absent, err := d.Flatpak.Observe(ctx, r, Receipt{})
	if err != nil || absent.Present {
		t.Fatal(absent, err)
	}
	if data, err := os.ReadFile(witness); err != nil || string(data) != "personal app data\n" {
		t.Fatal("personal data changed", err)
	}
	runtimes, err := d.Flatpak.query(ctx, r, "list", "--user", "--runtime", "--columns=ref")
	if err != nil || !strings.Contains(runtimes, "org.freedesktop.Platform/") {
		t.Fatal("private runtime was not retained", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{earlier}})
	reinstalled, err := d.Flatpak.verifiedApplication(ctx)
	if err != nil || !reinstalled.Healthy {
		t.Fatal(reinstalled, err)
	}
	// Selecting Obsidian now moves that installation to the portable build:
	// one approved apply removes the Flatpak and installs the build and its
	// launcher, keeping Flatpak app data and the private runtime.
	moved, err := c.Preview(ctx, Request{Schema: 1, Mode: "apply", Selected: []string{"obsidian"}})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"tool.obsidian": "remove", obsidianPortable: "install", "desktop.obsidian": "install"} {
		if got := operation(t, moved, id).Action; got != want {
			t.Fatalf("transition plans %s as %q, want %q", id, got, want)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"obsidian"}})
	if absent, err := d.Flatpak.Observe(ctx, r, Receipt{}); err != nil || absent.Present {
		t.Fatal("the earlier Flatpak was not removed", absent, err)
	}
	if portable, err := d.Archives.Observe(ctx, Resource{ID: obsidianPortable}, Receipt{}); err != nil || !portable.Healthy {
		t.Fatal("the portable build was not installed", portable, err)
	}
	launcher, err := os.ReadFile(filepath.Join(folders.Data, "applications", "dotfiles-obsidian.desktop"))
	if err != nil || !strings.Contains(string(launcher), " --no-sandbox %U\n") || strings.Contains(string(launcher), "flatpak") {
		t.Fatalf("the launcher does not start the portable build: %v\n%s", err, launcher)
	}
	if data, err := os.ReadFile(witness); err != nil || string(data) != "personal app data\n" {
		t.Fatal("the transition changed Flatpak app data", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}
