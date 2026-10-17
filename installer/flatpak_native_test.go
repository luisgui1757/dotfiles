package installer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	witness := filepath.Join(home, ".var", "app", "md.obsidian.Obsidian", "personal-note")
	writeConfigFixture(t, witness, "personal app data\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"obsidian"}})
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
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"obsidian"}})
	reinstalled, err := d.Flatpak.verifiedApplication(ctx)
	if err != nil || !reinstalled.Healthy {
		t.Fatal(reinstalled, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}
