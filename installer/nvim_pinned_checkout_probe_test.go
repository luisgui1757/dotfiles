package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Temporary native acceptance through the unchanged pinned Lua test harness.
func probeNvimPinnedCheckoutSpecs(t *testing.T, git string) {
	t.Helper()
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	c, archives, _ := archiveController(t)
	archives.Pins["tool.shared"], archives.Client = pins["tool.nvim"], nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	binary, err := archives.CommandPath("tool.shared", "nvim")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join(repository, "tests", "nvim", "spec", "pinned_git_checkout_spec.lua"))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(c.Home, "spec")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pinned_git_checkout_spec.lua"), spec, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	init := filepath.Join(repository, "tests", "nvim", "minimal_init.lua")
	command := exec.CommandContext(ctx, binary, "--headless", "-i", "NONE", "-u", init, "-c", "lua require('plenary.test_harness').test_directory(vim.env.DOTFILES_PROBE_SPEC, { minimal_init = vim.env.DOTFILES_PROBE_INIT, sequential = true, timeout = 180000 })")
	command.Dir = repository
	command.Env = append(os.Environ(), "HOME="+c.Home, "USERPROFILE="+c.Home,
		"PATH="+filepath.Dir(git)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"XDG_CONFIG_HOME="+filepath.Join(c.Home, "config"), "XDG_DATA_HOME="+filepath.Join(c.Home, "data"),
		"XDG_STATE_HOME="+filepath.Join(c.Home, "state"), "XDG_CACHE_HOME="+filepath.Join(c.Home, "cache"),
		"DOTFILES_PROBE_SPEC="+directory, "DOTFILES_PROBE_INIT="+init)
	command.WaitDelay = 2 * time.Second
	var output boundedCommandOutput
	output.limit = 1 << 20
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		t.Fatalf("pinned native Neovim checkout suite: %v\n%s", err, output.data.Bytes())
	}
	plain := regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(output.data.String(), "")
	if !regexp.MustCompile(`Success:\s+13\b`).MatchString(plain) || !strings.Contains(plain, "refuses an unsupported native Git root without replacing the original target") {
		t.Fatal("native suite did not execute all13 cases including the exact254 refusal", plain)
	}
	t.Log(plain)
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("pinned Neovim changed during the Lua suite", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.shared"}})
}
