package installer

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The installed command fetches the real pinned Sentinel commit and runs its
// installer in a disposable home: install, check, an unchanged second run and
// removal. It needs the network, so it runs only on request.
func TestNativeSentinelInstalledLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_SENTINEL") != "1" {
		t.Skip("set DOTFILES_TEST_SENTINEL=1 to fetch and run the pinned Sentinel commit")
	}
	if runtime.GOOS == "windows" {
		t.Skip("HOME does not move the Windows known folders")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Their folders make Codex, Claude Code, Kiro and OpenCode hosts. Kiro
	// uses a custom KIRO_HOME. PATH holds no agent command, so no other agent
	// counts.
	codexRules := filepath.Join(home, ".codex", "AGENTS.md")
	writeConfigFixture(t, codexRules, "# My Codex rules\n")
	writeConfigFixture(t, filepath.Join(home, ".claude", "settings.json"), "{}\n")
	writeConfigFixture(t, filepath.Join(home, ".config", "opencode", "opencode.json"), "{}\n")
	kiro := filepath.Join(home, "work kiro")
	writeConfigFixture(t, filepath.Join(kiro, "settings", "cli.json"), "{}\n")
	personal := filepath.Join(home, ".agents", "skills", "my-skill", "SKILL.md")
	writeConfigFixture(t, personal, "# My skill\n")
	invoke := installedMachineFixture(t, repository, home, 5*time.Minute, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "COPILOT_HOME="+filepath.Join(home, ".copilot"), "KIRO_HOME="+kiro)
	approve := func(request Request) Result {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal("preview", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		result := invoke(request)
		if result.Status != "ready" {
			t.Fatal("apply", result)
		}
		return result
	}
	approve(Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel"}})

	checkout := filepath.Join(home, ".local", "state", "dotfiles", "sentinel", "checkout")
	head, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != sentinelCommit {
		t.Fatal("the cached checkout is not the pinned commit", string(head), err)
	}
	rules, err := os.ReadFile(filepath.Join(checkout, "rules", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	ruleFiles := []string{codexRules, filepath.Join(home, ".claude", "CLAUDE.md"), filepath.Join(home, ".config", "opencode", "AGENTS.md"), filepath.Join(kiro, "steering", "AGENTS.md")}
	skillRoots := []string{filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".claude", "skills"), filepath.Join(kiro, "skills")}
	for _, path := range ruleFiles {
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte("<!-- AGENT-RULES:BEGIN do-not-edit-inside-this-block -->\n")) || !bytes.Contains(data, rules) {
			t.Fatal("the Sentinel rules are not installed", path, err)
		}
	}
	if data, err := os.ReadFile(codexRules); err != nil || !strings.HasPrefix(string(data), "# My Codex rules\n") {
		t.Fatal("the install changed the text outside the managed block", err)
	}
	profiles, err := os.ReadFile(filepath.Join(checkout, "skills", "profiles.txt"))
	if err != nil {
		t.Fatal(err)
	}
	skills := []string{}
	for _, line := range strings.Split(string(profiles), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && !strings.HasPrefix(line, "#") {
			skills = append(skills, fields[1])
			for _, root := range skillRoots {
				if _, err := os.Stat(filepath.Join(root, fields[1], ".sentinel-owned")); err != nil {
					t.Fatal("a Sentinel skill is not installed", root, fields[1], err)
				}
			}
		}
	}
	if len(skills) < 10 {
		t.Fatal("the core and UI profiles installed too few skills", skills)
	}
	if _, err := os.Lstat(filepath.Join(home, ".kiro")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Sentinel wrote Kiro files outside KIRO_HOME", err)
	}
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("check after install", result)
	}

	agentFiles := func() configSnapshot {
		t.Helper()
		snapshots := []configSnapshot{}
		for _, path := range []string{filepath.Join(home, ".codex"), filepath.Join(home, ".claude"), filepath.Join(home, ".config", "opencode"), filepath.Join(home, ".agents"), kiro} {
			snapshot, err := snapshotConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			snapshots = append(snapshots, snapshot)
		}
		hash, err := digest(snapshots)
		if err != nil {
			t.Fatal(err)
		}
		return configSnapshot{Kind: "tree", Hash: hash}
	}
	before := agentFiles()
	for _, request := range []Request{{Schema: 1, Mode: "update"}, {Schema: 1, Mode: "apply", Selected: []string{"sentinel"}}} {
		preview := invoke(request)
		if operation(t, preview.Plan, sentinelResource).Action != "keep" {
			t.Fatal("a second run wants to change Sentinel", request.Mode, preview.Plan.Operations)
		}
		approve(request)
	}
	if agentFiles() != before {
		t.Fatal("a second run changed the agent files")
	}

	approve(Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if data, err := os.ReadFile(codexRules); err != nil || !strings.HasPrefix(string(data), "# My Codex rules\n") || bytes.Contains(data, []byte("AGENT-RULES")) {
		t.Fatal("removal did not leave only the personal Codex rules", string(data), err)
	}
	for _, path := range append(ruleFiles[1:], filepath.Join(home, ".local", "state", "dotfiles", "sentinel")) {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("removal left a Sentinel file", path, err)
		}
	}
	for _, name := range skills {
		for _, root := range skillRoots {
			if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("removal left a Sentinel skill", root, name, err)
			}
		}
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "# My skill\n" {
		t.Fatal("removal changed a personal skill", err)
	}
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("check after removal", result)
	}
	t.Logf("pinned Sentinel %s: install for codex, claude, kiro at a custom KIRO_HOME and opencode, check, unchanged second run and removal passed", sentinelCommit)
}

// A rules file with a non-default mode keeps that mode through install and
// removal, as other managed files do.
func TestNativeSentinelKeepsRulesFileMode(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_SENTINEL") != "1" {
		t.Skip("set DOTFILES_TEST_SENTINEL=1 to fetch and run the pinned Sentinel commit")
	}
	if runtime.GOOS == "windows" {
		t.Skip("HOME does not move the Windows known folders")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(home, ".codex", "AGENTS.md")
	writeConfigFixture(t, rules, "# My Codex rules\n")
	if err := os.Chmod(rules, 0640); err != nil {
		t.Fatal(err)
	}
	invoke := installedMachineFixture(t, repository, home, 5*time.Minute, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "COPILOT_HOME="+filepath.Join(home, ".copilot"))
	for _, selected := range [][]string{{"sentinel"}, {}} {
		request := Request{Schema: 1, Mode: "apply", Selected: selected}
		request.ExpectedPlan = invoke(request).Plan.ID
		if result := invoke(request); result.Status != "ready" {
			t.Fatal(result)
		}
		info, err := os.Stat(rules)
		if err != nil || info.Mode().Perm() != 0640 {
			t.Fatal("Sentinel changed the mode of the rules file", selected, info, err)
		}
	}
}
