package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The compiled public command must discover and use APT itself. This fixture
// runs only in explicitly fresh disposable Ubuntu/Debian fixtures. Git, tmux
// and zsh must really be absent. Do not remove runner packages to manufacture
// absence: installed consumers such as ubuntu-server legitimately retain tmux.
func TestNativeLinuxAPTInstalledApplicationLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_LINUX_FRESH") != "1" {
		t.Skip("requires an explicitly enabled fresh disposable GitHub Linux package host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	location, err := DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inventory := func() map[string]nativePackage {
		t.Helper()
		got, err := linuxAPTInventory(ctx, location.query)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	native := func(program string, args ...string) {
		t.Helper()
		if !location.root {
			args = append([]string{"-n", program}, args...)
			program = location.Sudo
		}
		command := exec.CommandContext(ctx, program, args...)
		command.Env = append(os.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("native fixture setup: %v\n%s", err, output)
		}
	}
	before := inventory()
	for _, name := range []string{"git", "tmux", "zsh"} {
		if identity, err := linuxAPTIdentity(name, before); err != nil || identity != "" {
			t.Fatal("fresh fixture requires real package absence; preserve existing packages and their consumers", name, identity, err)
		}
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "native home with 'quotes'")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, ".bashrc")
	writeConfigFixture(t, profile, "# personal shell configuration\n")
	invoke := installedMachineFixture(t, repository, home, 8*time.Minute)
	apply := func(request Request) {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" || preview.Plan.ID == "" {
			t.Fatal("missing application preview", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		if result := invoke(request); result.Status != "ready" {
			t.Fatal("application mutation incomplete", result)
		}
		if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
			t.Fatal("installed application check failed", result)
		}
		t.Logf("installed Linux application %s selected=%v: ready", request.Mode, request.Selected)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"gh-dash", "tmux", "zsh"}})
	statePath := filepath.Join(home, ".local", "state", "dotfiles", "state.json")
	state, err := LoadState(statePath, home)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"git", "tmux", "zsh"} {
		identity, err := linuxAPTIdentity(name, before)
		if err != nil {
			t.Fatal(err)
		}
		receipt := state.Receipts["tool."+name]
		if receipt.After.Provider != "apt" || (receipt.Ownership == "created") != (identity == "") {
			t.Fatal("APT acquisition does not match native baseline", name, receipt)
		}
		t.Logf("native baseline: %s absent=%t", name, identity == "")
	}
	shell := func(script string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--rcfile", profile, "-ic", script)
		command.Dir = home
		command.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "ZDOTDIR="+home, "TERM=xterm-256color", "TMUX=")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fresh installed shell: %v\n%s", err, output)
		}
		return string(output)
	}
	output := shell("set -e; git --version; gh --version; gh-dash --version; tmux -V; zsh --version; command -v gh-dash")
	if !strings.Contains(output, filepath.Join(home, ".local", "state", "dotfiles", "packages")) {
		t.Fatal("shell did not expose managed commands", output)
	}
	apply(Request{Schema: 1, Mode: "update"})
	// Damage a file of the genuinely owned native package, then require repair.
	native("/usr/bin/rm", "--", "/usr/bin/tmux")
	apply(Request{Schema: 1, Mode: "repair"})
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"tmux"}, RemoveShared: []string{"tool.git", "tool.zsh"}})
	shell("set -e; tmux -V; test -f \"$HOME/.tmux.plugins.conf\"")
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.tmux"}})
	after := inventory()
	if identity, err := linuxAPTIdentity("tmux", after); err != nil || identity != "" {
		t.Fatal("owned tmux survived removal", identity, err)
	}
	for name, old := range before {
		current, present := after[name]
		if !present || current.Source != old.Source || current.Automatic != old.Automatic || current.Held != old.Held {
			t.Fatal("changed pre-existing package classification", name, old, current)
		}
	}
	for name := range after {
		if _, existed := before[name]; !existed {
			t.Error("owned incidental package survived complete removal", name)
		}
	}
	if data, err := os.ReadFile(profile); err != nil || string(data) != "# personal shell configuration\n" {
		t.Fatal("personal shell profile was not restored", err)
	}
	for _, path := range []string{filepath.Join(home, ".tmux.conf"), filepath.Join(home, ".tmux.plugins.conf"), filepath.Join(home, ".config", "gh-dash", "config.yml")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("owned config survived removal", path, err)
		}
	}
}
