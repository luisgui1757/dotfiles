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
	if runtime.GOOS != "darwin" {
		t.Skip("macOS system-shell lifecycle")
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
	c, err := NewNativeController(repository, "zsh-composition", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	run := func(fzf bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		script := "set -e; bindkey -M viins '^I'; bindkey -M viins '^R'; whence -w _zsh_autosuggest_start; command -v fzf; command -v starship; print -r -- $DOTFILES_LOCAL_OVERRIDE; print DOTFILES_TEST_COMPLETE"
		cmd := exec.CommandContext(ctx, "/usr/bin/script", "-q", "/dev/null", "/bin/zsh", "-d", "-i", "-c", script)
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
}
