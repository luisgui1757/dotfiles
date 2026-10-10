package installer

import (
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// This is deliberately separate from the fast core suite: it provisions the
// complete checkbox through the compiled entrypoint on a disposable native host.
func TestNativeNeovimInstalledApplicationLifecycle(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_NVIM_APPLICATION") != "1" {
		t.Skip("requires an explicitly opted-in disposable native GitHub runner")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := nativePlatform(t)
	requireFreshLinuxMasonPrerequisitesAbsent(t)
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Data: filepath.Join(home, ".local", "share"), Zsh: home}
	stateDirectory := filepath.Join(home, ".local", "state", "dotfiles")
	if runtime.GOOS == "windows" {
		folders, err = DiscoverConfigFolders()
		if err != nil {
			t.Fatal(err)
		}
		folders.Data = filepath.Join(home, ".local", "share")
		stateDirectory, err = DiscoverStateDirectory(target.Context, folders)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(stateDirectory); !os.IsNotExist(err) {
		t.Fatal("native public Neovim acceptance needs a fresh installer state", err)
	}
	// Observe every configuration/profile target before the production command
	// can touch it, then require exact restoration after the final removal.
	c, err := NewNativeController(repository, "read-only-native-fixture", stateDirectory, target, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	t.Cleanup(func() {
		if t.Failed() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			detail, err := nativeNodeArchiveDiagnostic(ctx, d.Archives, filepath.Join(t.TempDir(), "node-reference"))
			t.Logf("failed native Neovim Node archive diagnostic: %v\n%s", err, detail)
		}
	})
	before := map[string]configSnapshot{}
	for _, profile := range d.Profiles.Targets["integration.shells"] {
		before[profile.Path], err = snapshotConfig(profile.Path)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, resource := range []string{"config.nvim", "config.pi"} {
		paths, err := d.Configurations.Manifest.Resolve(c.Catalog, target.Context, folders, repository, resource)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			snapshot, err := snapshotConfig(path.Destination)
			if err != nil || snapshot.Kind != "absent" {
				t.Fatal("native fixture must not adopt personal editor/agent configuration", path.Destination, snapshot, err)
			}
			before[path.Destination] = snapshot
		}
	}
	runtimeDirectory, err := nvimRuntimeDirectory(target.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(filepath.Dir(runtimeDirectory), "personal-session.txt")
	writeConfigFixture(t, personal, "preserve editor data outside the managed runtime\n")
	invoke := installedMachineFixture(t, repository, home, 35*time.Minute)
	t.Setenv("DOTFILES_ENTRYPOINT", "setup")
	var reviewed Plan
	var applying *Request
	t.Cleanup(func() {
		if runtime.GOOS == "windows" && t.Failed() && applying != nil {
			nativeNeovimPlanDiagnostic(t, repository, home, *applying, reviewed)
		}
	})
	apply := func(request Request) {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" || preview.Plan.ID == "" {
			t.Fatal("missing reviewed plan", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		reviewed, applying = preview.Plan, &request
		result := invoke(request)
		applying = nil
		if result.Status != "ready" {
			t.Fatal("native Neovim request incomplete", result)
		}
		if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
			t.Fatal("native installed application check failed", result)
		}
		t.Logf("native installed command %s selected=%v: ready", request.Mode, request.Selected)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"neovim"}})
	if runtime.GOOS == "windows" {
		binary := filepath.Join(home, "dotfiles.exe")
		before, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		// Change only the executable build ID. The public check must continue
		// accepting the intact Rust generation prepared by the previous binary.
		build := exec.Command("go", "build", "-trimpath", "-ldflags=-buildid=dotfiles-fixture-rebuild", "-o", binary, "./cmd/dotfiles")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("rebuild installed entrypoint: %v%s", err, nativeDiagnostic(output))
		}
		after, err := os.ReadFile(binary)
		if err != nil || sha256.Sum256(before) == sha256.Sum256(after) {
			t.Fatal("fixture did not change the installer binary", err)
		}
		if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
			t.Fatal("installer-only rebuild invalidated installed Neovim/Rust", result)
		}
		t.Logf("public check stayed ready after installer-only rebuild: %x -> %x", sha256.Sum256(before), sha256.Sum256(after))
	}
	requireFreshLinuxMasonPrerequisitesOwned(t, stateDirectory, home)
	platform, err := DiscoverPlatform()
	if err != nil {
		t.Fatal(err)
	}
	platform.SystemCommandDirectories, err = DiscoverSystemCommandDirectories()
	if err != nil {
		t.Fatal(err)
	}
	if target.OS == "darwin" {
		platform.Homebrew, err = DiscoverHomebrew(context.Background())
	} else if target.OS == "windows" {
		platform.WindowsPowerShell, err = DiscoverWindowsPowerShell()
		if err == nil {
			platform.WindowsBuildToolsDirectory, err = DiscoverWindowsBuildToolsDirectory()
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	// Construct the production command path/environment after native prerequisites
	// exist. Normal init must find the published runtime without an explicit root.
	c, err = NewNativeController(repository, "read-only-native-fixture", stateDirectory, platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d = c.Driver.(*NativeDriver)
	run := func(program string, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, program, args...)
		command.Dir = repository
		config := folders.Config
		if runtime.GOOS == "windows" {
			config = "" // Native Neovim uses its actual LocalAppData config target.
		}
		command.Env = append(os.Environ(), d.NvimSync.Environment...)
		command.Env = append(command.Env, "HOME="+home, "USERPROFILE="+home, "LOCALAPPDATA="+folders.LocalAppData, "APPDATA="+folders.AppData,
			"XDG_CONFIG_HOME="+config, "XDG_DATA_HOME="+folders.Data, "XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
			"XDG_CACHE_HOME="+filepath.Join(home, ".cache"), "NVIM_APPNAME=nvim", "DOTFILES_NVIM_RUNTIME=", "DOTFILES_NVIM_LOCKFILE=",
			"VIMINIT=", "EXINIT=", "DOTFILES_LSP_SMOKE=strict", "DOTFILES_TREESITTER_SYNC_INSTALL=", "TERM=xterm-256color")
		var output boundedCommandOutput
		output.limit = 8 << 20
		command.Stdout, command.Stderr = &output, &output
		if err := command.Run(); err != nil {
			t.Fatalf("native consumer %s %v: %v\n%s", program, args, err, output.data.Bytes())
		}
		t.Log(output.data.String())
		return output.data.String()
	}
	neovim, err := d.Archives.CommandPath("tool.nvim", "nvim")
	if err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(home, "personal-notes")
	t.Setenv("NOTES_VAULT", vault)
	t.Setenv("DOTFILES_NOTES_SMOKE", "absent")
	run(neovim, "--headless", "-i", "NONE", "-c", "luafile tests/nvim/notes_smoke.lua", "+qa")
	personalNote := filepath.Join(vault, "personal-note.md")
	writeConfigFixture(t, personalNote, "preserve personal vault contents\n")
	t.Setenv("DOTFILES_NOTES_SMOKE", "existing")
	run(neovim, "--headless", "-i", "NONE", "-c", "luafile tests/nvim/notes_smoke.lua", "+qa")
	run(neovim, "--headless", "-i", "NONE", "-c", "luafile tests/nvim/lsp_smoke.lua", "+qa")
	apply(Request{Schema: 1, Mode: "update"})
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"neovim", "pi"}})
	statePath := filepath.Join(stateDirectory, "state.json")
	sharedRemoval := func() []string {
		t.Helper()
		state, err := LoadState(statePath, folders.Home)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for id, receipt := range state.Receipts {
			resource, _ := c.Catalog.Resource(id)
			if (resource.Shared || resource.Scope == "machine" || receipt.After.Scope == "machine") && receipt.Ownership == "created" && !resource.Retain {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		return ids
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"pi"}, RemoveShared: sharedRemoval()})
	pi, err := d.Archives.CommandPath("tool.pi", "pi")
	if err != nil {
		t.Fatal(err)
	}
	if output := run(pi, "--version"); !strings.Contains(output, d.Archives.Pins["tool.pi"].Version) {
		t.Fatal("Pi did not survive shared Neovim prerequisite removal", output)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: sharedRemoval()})
	requireFreshLinuxMasonPrerequisitesAbsent(t)
	for path, snapshot := range before {
		if err := verifyConfigSnapshot(path, snapshot); err != nil {
			t.Fatal("native lifecycle failed to restore a profile/configuration", path, err)
		}
	}
	if data, err := os.ReadFile(personalNote); err != nil || string(data) != "preserve personal vault contents\n" {
		t.Fatal("personal notes did not survive removal", err)
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "preserve editor data outside the managed runtime\n" {
		t.Fatal("editor personal data did not survive removal", err)
	}
}
