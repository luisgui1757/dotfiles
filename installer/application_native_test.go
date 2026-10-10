package installer

import (
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

func TestNativeArchiveApplicationInstallsUsableStarshipWithoutNixOrChezmoi(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_ARCHIVES=1 for native upstream installation and shell execution")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZDOTDIR", home)
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	c, err := NewNativeController(repository, "native-application", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	for _, target := range d.Profiles.Targets["integration.shells"] {
		relative, err := filepath.Rel(home, target.Path)
		if err != nil || !filepath.IsLocal(relative) {
			t.Fatalf("native fixture profile escaped its disposable home: %s", target.Path)
		}
		writeConfigFixture(t, target.Path, "# existing personal settings\n")
	}
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"starship"}})
	for _, op := range result.Plan.Operations {
		if op.Resource == "infra.nix" || op.Resource == "infra.chezmoi" {
			t.Fatal("new installation pulled retired infrastructure", op)
		}
	}
	command, err := d.Archives.CommandPath("tool.starship", "starship")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var shell *exec.Cmd
	if runtime.GOOS == "windows" {
		profile := d.Profiles.Targets["integration.shells"][1].Path
		shell = exec.CommandContext(ctx, "pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", ". "+powershellLiteral(profile)+"; (Get-Command starship).Source; starship --version; if (Test-Path Function:global:prompt) { Write-Output 'profile-loaded' }")
	} else {
		shell = exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--rcfile", filepath.Join(home, ".bashrc"), "-ic", "command -v starship; starship --version; printf '%s\\n' \"$STARSHIP_CONFIG\"; declare -F starship_precmd")
	}
	shell.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+folders.Config, "ZDOTDIR="+home, "TERM=xterm-256color")
	shell.Dir = home
	output, err := shell.CombinedOutput()
	if err != nil || !strings.Contains(string(output), command) || !strings.Contains(string(output), "starship "+d.Archives.Pins["tool.starship"].Version) || runtime.GOOS != "windows" && !strings.Contains(string(output), filepath.Join(folders.Config, "starship.toml")) {
		t.Fatalf("fresh shell did not use the selected tool/config: %s %v", output, err)
	}
	t.Logf("native shell integration on %s/%s: %s", runtime.GOOS, runtime.GOARCH, output)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	check, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
	for _, target := range d.Profiles.Targets["integration.shells"] {
		data, err := os.ReadFile(target.Path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target.Path, append(data, []byte("# later personal settings\n")...), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, target := range d.Profiles.Targets["integration.shells"] {
		data, err := os.ReadFile(target.Path)
		if err != nil || string(data) != "# existing personal settings\n# later personal settings\n" {
			t.Fatalf("profile removal lost user settings: %q %v", data, err)
		}
	}
	if _, err := os.Stat(command); !os.IsNotExist(err) {
		t.Fatal("private command survived removal", err)
	}
}

func TestNativeArchiveInstalledCommandMachineLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_ARCHIVES=1 for the real installed command lifecycle")
	}
	if runtime.GOOS == "windows" && (os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_PUBLIC_WINDOWS") != "1") {
		t.Skip("public Windows profile writes require the explicitly enabled disposable GitHub runner")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Windows obtains actual known folders; environment variables cannot fake
	// them. Only the explicitly opted-in disposable hosted account is mutated.
	before := map[string]configSnapshot{}
	if runtime.GOOS == "windows" {
		folders, err := DiscoverConfigFolders()
		if err != nil {
			t.Fatal(err)
		}
		target := Context{OS: runtime.GOOS, Arch: runtime.GOARCH}
		state, err := DiscoverStateDirectory(target, folders)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(state); !os.IsNotExist(err) {
			t.Fatal("hosted public fixture requires a fresh installer state directory", err)
		}
		c, err := NewNativeController(repository, "inspection", state, NativePlatform{Context: target}, folders)
		if err != nil {
			t.Fatal(err)
		}
		d := c.Driver.(*NativeDriver)
		for _, profile := range d.Profiles.Targets["integration.shells"] {
			snapshot, err := snapshotConfig(profile.Path)
			if err != nil {
				t.Fatal(err)
			}
			before[profile.Path] = snapshot
		}
		for _, configuration := range []string{filepath.Join(folders.Config, "starship.toml"), filepath.Join(folders.AppData, "herdr", "config.toml")} {
			snapshot, err := snapshotConfig(configuration)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Kind != "absent" {
				t.Fatal("hosted public fixture requires no pre-existing selected configuration", configuration)
			}
			before[configuration] = snapshot
		}
	}
	invoke := installedMachineFixture(t, repository, home, 3*time.Minute)
	removeAll := Request{Schema: 1, Mode: "apply", Selected: []string{}}
	if runtime.GOOS == "windows" {
		// Herdr selects the shared PowerShell runtime. Full removal includes
		// its explicit per-item choice, just as the interactive UI does.
		removeAll.RemoveShared = []string{"tool.powershell"}
	}
	herdrSelected := false
	for _, request := range []Request{{Schema: 1, Mode: "apply", Selected: []string{"starship", "herdr"}}, {Schema: 1, Mode: "update"}, {Schema: 1, Mode: "repair"}, {Schema: 1, Mode: "apply", Selected: []string{"herdr"}}, removeAll} {
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal(preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		result := invoke(request)
		if result.Status != "ready" {
			t.Fatal(result)
		}
		check := invoke(Request{Schema: 1, Mode: "check"})
		if check.Status != "ready" {
			t.Fatal(check)
		}
		if request.Mode == "apply" {
			herdrSelected = slices.Contains(request.Selected, "herdr")
		}
		if herdrSelected {
			exerciseInstalledHerdrConfiguration(t, repository, home)
		}
		t.Logf("installed entrypoint %s selected=%v: %s", request.Mode, request.Selected, result.Status)
	}
	for path, snapshot := range before {
		if err := verifyConfigSnapshot(path, snapshot); err != nil {
			t.Fatal("public Windows lifecycle did not restore the original profile/config", path, err)
		}
	}
}

// Configuration consumption is part of the installed-command lifecycle. The
// archive pin/version checks alone cannot prove the configured app accepts it.
func exerciseInstalledHerdrConfiguration(t *testing.T, repository, home string) {
	t.Helper()
	platform := nativePlatform(t)
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: home}
	stateDirectory := filepath.Join(home, ".local", "state", "dotfiles")
	var err error
	if runtime.GOOS == "windows" {
		folders, err = DiscoverConfigFolders()
		if err != nil {
			t.Fatal(err)
		}
		stateDirectory = filepath.Join(folders.LocalAppData, "dotfiles")
	}
	c, err := NewNativeController(repository, "configuration-consumer", stateDirectory, platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	driver := c.Driver.(*NativeDriver)
	paths, err := driver.Configurations.Manifest.Resolve(c.Catalog, platform.Context, folders, repository, "config.herdr")
	if err != nil || len(paths) != 1 {
		t.Fatal("resolve installed Herdr configuration", paths, err)
	}
	command, err := driver.Archives.CommandPath("tool.herdr", "herdr")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer := exec.CommandContext(ctx, command, "config", "check")
	consumer.Env = append(os.Environ(), "HOME="+folders.Home, "HERDR_CONFIG_PATH="+paths[0].Destination)
	if output, err := consumer.CombinedOutput(); err != nil {
		t.Fatalf("installed Herdr rejected managed configuration: %v\n%s", err, output)
	}
}
