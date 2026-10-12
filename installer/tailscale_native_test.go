package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeTailscalePackageLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_TAILSCALE") != "1" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicit disposable Linux Tailscale package acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	platform := nativePlatform(t)
	var err error
	platform.APT, err = DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := linuxAPTInventory(ctx, platform.APT.query)
	if err != nil {
		t.Fatal(err)
	}
	if identity, err := linuxAPTIdentity("tailscale", inventory); err != nil || identity != "" {
		t.Fatal("fixture requires absent Tailscale", identity, err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Data: filepath.Join(home, "data"), Zsh: home}
	c, err := NewNativeController(repository, "native-tailscale", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	d.session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(d.session.Directory) }
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"tailscale"}})
	run := func(program string, args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, program, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native application probe: %v%s", err, nativeDiagnostic(output))
		}
		return output
	}
	if got := string(run("/usr/bin/tailscale", "version")); !strings.Contains(got, d.APT.Debs["tool.tailscale"].Version) {
		t.Fatal("wrong Tailscale version")
	}
	// This contacts the actual daemon without supplying credentials or joining
	// a network. Do not print its JSON: it can contain machine/account metadata.
	var status struct{ BackendState string }
	if err := json.Unmarshal(run("/usr/bin/tailscale", "status", "--json"), &status); err != nil || status.BackendState != "NeedsLogin" {
		t.Fatal("fresh daemon did not reach unauthenticated readiness", status.BackendState, err)
	}
	const config = "/etc/default/tailscaled"
	original, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	personal := append(append([]byte(nil), original...), []byte("\n# Personal service options survive installer maintenance.\n")...)
	edit := exec.CommandContext(ctx, "/usr/bin/sudo", "-n", "/usr/bin/tee", config)
	edit.Stdin = bytes.NewReader(personal)
	if output, err := edit.CombinedOutput(); err != nil {
		t.Fatal(err, nativeDiagnostic(output))
	}
	before, err := os.ReadFile(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(c.StatePath)
	if err != nil || string(before) != string(after) {
		t.Fatal("Check changed saved state", err)
	}
	update := dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	for _, op := range update.Plan.Operations {
		if op.Action != "keep" {
			t.Fatal("unchanged pin performed mutation", op)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	// Machine-wide applications need a separate explicit removal choice.
	retained, err := linuxAPTInventory(ctx, platform.APT.query)
	if err != nil {
		t.Fatal(err)
	}
	if identity, err := linuxAPTIdentity("tailscale", retained); err != nil || identity == "" {
		t.Fatal("deselection removed shared machine infrastructure", identity, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.tailscale"}})
	installed, err := linuxAPTInventory(ctx, platform.APT.query)
	if err != nil {
		t.Fatal(err)
	}
	if identity, err := linuxAPTIdentity("tailscale", installed); err != nil || identity != "" {
		t.Fatal("owned Tailscale remains", identity, err)
	}
	if after, err := os.ReadFile(config); err != nil || !bytes.Equal(after, personal) {
		t.Fatal("native maintenance/removal changed personal service configuration", err)
	}
	for name, before := range inventory {
		after, ok := installed[name]
		if !ok || after.Source != before.Source || after.Held != before.Held || after.Automatic != before.Automatic {
			t.Fatal("pre-existing native package changed", name)
		}
	}
}
