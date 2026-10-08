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

// Exercise the production discovery, catalog and controller without permitting
// package or profile mutations. Fresh package lifecycle uses the disposable gate.
func TestNativeBrewApplicationPreviewReadOnly(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("DOTFILES_TEST_NATIVE_INVENTORY") != "1" && os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicit native inventory inspection")
	}
	ctx := context.Background()
	platform := nativePlatform(t)
	var err error
	platform.Homebrew, err = DiscoverHomebrew(ctx)
	if err != nil || platform.Homebrew == nil {
		t.Fatal("native fixture requires discoverable Homebrew", err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(home, "state")
	c, err := NewNativeController(repository, "native-homebrew-preview", state, platform, ConfigFolders{Home: home, Config: filepath.Join(home, ".config")})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := c.Preview(ctx, Request{Schema: 1, Mode: "apply", Selected: []string{"gh-dash"}})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, op := range plan.Operations {
		switch op.Resource {
		case "tool.git":
			found[op.Resource] = true
			if op.Observed.Provider != "homebrew-formula" || op.Observed.Unknown {
				t.Fatal("production Git route could not inspect Homebrew", op)
			}
		case "infra.homebrew":
			found[op.Resource] = true
			if op.Action != "keep" || !op.Observed.Healthy || op.Observed.Provider != "homebrew-infrastructure" {
				t.Fatal("existing infrastructure was not reused", op)
			}
		}
	}
	if !found["tool.git"] || !found["infra.homebrew"] {
		t.Fatal("dashboard did not resolve its native prerequisites", found)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("preview created installer state or configuration", entries, err)
	}
}

func TestNativeBrewInstalledApplicationLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicitly enabled disposable GitHub macOS package host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	location, err := DiscoverHomebrew(ctx)
	if err != nil || location == nil {
		t.Fatal("native fixture requires Homebrew", err)
	}
	inventory := func() map[string]nativePackage {
		t.Helper()
		got, err := brewInventory(ctx, location.query)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// The earlier plugin fixture needs tmux. Remove that runner prerequisite now
	// so this test proves native installation, not reuse or PATH hiding. Homebrew's
	// outside-dependency protection remains enabled, even during fixture setup.
	if _, present := inventory()["tmux"]; present {
		command := exec.CommandContext(ctx, location.Program, "uninstall", "--formula", "--force", "tmux")
		command.Env = append(os.Environ(), brewProcessControls...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("establish actual tmux absence: %v\n%s", err, output)
		}
	}
	before := inventory()
	if _, present := before["tmux"]; present {
		t.Fatal("fixture failed to remove native tmux")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "native home with 'quotes'")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, ".bashrc")
	writeConfigFixture(t, profile, "# personal shell configuration\n")
	invoke := installedMachineFixture(t, repository, home, 8*time.Minute)
	apply := func(request Request) {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal("application did not present an approval plan", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		if result := invoke(request); result.Status != "ready" {
			t.Fatal("approved application operation did not complete", result)
		}
		if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
			t.Fatal("read-only check failed after operation", result)
		}
		t.Logf("installed application %s selected=%v: ready", request.Mode, request.Selected)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"gh-dash", "tmux"}})
	statePath := filepath.Join(home, ".local", "state", "dotfiles", "state.json")
	state, err := LoadState(statePath, home)
	if err != nil || state.Receipts["tool.tmux"].Ownership != "created" || state.Receipts["tool.tmux"].After.Provider != "homebrew-formula" {
		t.Fatal("actual tmux installation was not attributed to the native provider", state.Receipts["tool.tmux"], err)
	}
	_, gitExisted := before["git"]
	if (state.Receipts["tool.git"].Ownership == "created") == gitExisted {
		t.Fatal("Git acquisition did not respect its actual baseline", state.Receipts["tool.git"], gitExisted)
	}
	t.Logf("native baseline: tmux absent, Git pre-existing=%t", gitExisted)
	shell := func(script string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--rcfile", profile, "-ic", script)
		command.Dir = home
		command.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "ZDOTDIR="+home, "TERM=xterm-256color", "TMUX=")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("installed shell consumers failed: %v\n%s", err, output)
		}
		return string(output)
	}
	output := shell("set -e; command -v git; git --version; command -v gh; gh --version; command -v gh-dash; gh-dash --version; command -v tmux; tmux -V")
	if !strings.Contains(output, filepath.Join(location.Prefix, "bin", "git")) || !strings.Contains(output, filepath.Join(location.Prefix, "bin", "tmux")) || !strings.Contains(output, filepath.Join(home, ".local", "state", "dotfiles", "packages")) {
		t.Fatal("fresh shell did not resolve actual native and private tools", output)
	}
	apply(Request{Schema: 1, Mode: "update"})
	// Damage the owned package's actual native linkage before repair. Merely
	// hiding its executable on PATH would not test the provider's repair path.
	unlink := exec.CommandContext(ctx, location.Program, "unlink", "tmux")
	unlink.Env = append(os.Environ(), brewProcessControls...)
	if output, err := unlink.CombinedOutput(); err != nil {
		t.Fatalf("unlink owned tmux for repair: %v\n%s", err, output)
	}
	if _, err := os.Lstat(filepath.Join(location.Prefix, "bin", "tmux")); !os.IsNotExist(err) {
		t.Fatal("fixture did not damage native command linkage", err)
	}
	apply(Request{Schema: 1, Mode: "repair"})
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"tmux"}, RemoveShared: []string{"tool.git"}})
	if _, present := inventory()["tmux"]; !present {
		t.Fatal("dashboard removal broke the remaining tmux selection")
	}
	shell("set -e; tmux -V; test -f \"$HOME/.tmux.plugins.conf\"")
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.tmux"}})
	after := inventory()
	if _, present := after["tmux"]; present {
		t.Fatal("owned native tmux survived complete removal")
	}
	for name, old := range before {
		current, present := after[name]
		if !present || brewClassificationOf(current) != brewClassificationOf(old) {
			t.Fatal("application changed pre-existing package ownership", name, current, old)
		}
	}
	if data, err := os.ReadFile(profile); err != nil || string(data) != "# personal shell configuration\n" {
		t.Fatal("application removal changed personal profile bytes", string(data), err)
	}
	for _, path := range []string{filepath.Join(home, ".tmux.conf"), filepath.Join(home, ".tmux.plugins.conf"), filepath.Join(home, ".config", "gh-dash", "config.yml")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("owned configuration survived full removal", path, err)
		}
	}
}
