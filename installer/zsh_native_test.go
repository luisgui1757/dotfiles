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

func TestNativeArchiveZshCompositionAndSharedFzfRemoval(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("enable upstream package lifecycle")
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX system-shell lifecycle")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: filepath.Join(home, "custom zsh")}
	platform := nativePlatform(t)
	if platform.OS == "linux" {
		platform.APT, err = DiscoverLinuxAPT(t.Context())
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewNativeController(repository, "zsh-composition", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	resource, ok := c.Catalog.Resource("tool.zsh")
	if !ok {
		t.Fatal("missing native zsh resource")
	}
	before, err := d.Observe(t.Context(), resource, Receipt{})
	if err != nil || !before.Present || !before.Healthy {
		t.Fatal("archive fixture requires a healthy pre-existing native zsh; native installation belongs to the fresh APT lane", before, err)
	}
	if platform.OS == "linux" && before.Provider != "apt" {
		t.Fatal("Linux zsh did not use the connected APT provider", before)
	}
	run := func(fzf bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		script := "set -e; bindkey -M viins '^I'; bindkey -M viins '^R'; whence -w _zsh_autosuggest_start; command -v fzf; command -v starship; print -r -- $DOTFILES_LOCAL_OVERRIDE; print DOTFILES_TEST_COMPLETE"
		cmd := exec.CommandContext(ctx, "/usr/bin/script", "-q", "/dev/null", "/bin/zsh", "-d", "-i", "-c", script)
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "/usr/bin/script", "-q", "-e", "-c", "/bin/zsh -d -i -c "+shellLiteral(script), "/dev/null")
		}
		cmd.Env = append(os.Environ(), "HOME="+home, "ZDOTDIR="+folders.Zsh, "XDG_CONFIG_HOME="+folders.Config, "TERM=xterm-256color")
		output, err := cmd.CombinedOutput()
		wanted := "history-incremental-search-backward"
		if fzf {
			wanted = "fzf-history-widget"
		}
		if err != nil || strings.Contains(string(output), "can't change option") || !strings.Contains(string(output), "DOTFILES_TEST_COMPLETE") || !strings.Contains(string(output), "fzf-tab-complete") || !strings.Contains(string(output), wanted) || !strings.Contains(string(output), "_zsh_autosuggest_start: function") || !strings.Contains(string(output), "personal-local-last") {
			t.Fatalf("zsh fzf=%v: %s %v", fzf, output, err)
		}
		t.Logf("zsh fzf=%v: %s", fzf, output)
	}
	local := filepath.Join(home, ".zshrc.local")
	writeConfigFixture(t, local, "export DOTFILES_LOCAL_OVERRIDE=personal-local-last\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zsh", "starship", "fzf"}})
	run(true)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zsh", "starship"}})
	run(false)
	command, err := d.Archives.CommandPath("tool.fzf", "fzf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(command); err != nil {
		t.Fatal("zsh's shared fzf dependency was removed", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, id := range []string{"plugin.fzf-tab", "plugin.zsh-autosuggestions", "tool.fzf", "tool.starship"} {
		if _, err := os.Lstat(d.Archives.currentLink(id)); !os.IsNotExist(err) {
			t.Fatal("unused private dependency remains", id, err)
		}
	}
	data, err := os.ReadFile(local)
	if err != nil || string(data) != "export DOTFILES_LOCAL_OVERRIDE=personal-local-last\n" {
		t.Fatal("personal local hook changed", err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Receipts["tool.zsh"].Ownership != "reused" {
		t.Fatal("system shell ownership changed", err)
	}
	after, err := d.Observe(t.Context(), resource, state.Receipts["tool.zsh"])
	if err != nil || !after.Present || !after.Healthy || after.Fingerprint != before.Fingerprint || after.Inventory != before.Inventory {
		t.Fatal("pre-existing system shell or native package inventory changed", before, after, err)
	}
	t.Logf("pre-existing %s zsh stayed healthy, reused and unchanged after selective/full removal", before.Provider)
}
