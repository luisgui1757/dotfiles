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

// Temporary Windows diagnostic: call after the existing private Git install,
// before that fixture removes it. No production invocation changes.
func probeNvimGitConfigBoundary(t *testing.T, git string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Fatal("Windows-only boundary probe")
	}
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixed := filepath.Join(base, "isolated.config")
	writeConfigFixture(t, fixed, "[core]\nlongpaths = true\n")
	source := filepath.Join(base, "source")
	writeConfigFixture(t, filepath.Join(source, "proof.txt"), "locked proof\n")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	run := func(system bool, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, git, args...)
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_COUNT=0", "GIT_CONFIG_PARAMETERS=", "GIT_TEMPLATE_DIR=", "GIT_TERMINAL_PROMPT=0")
		if system {
			cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=0", "GIT_CONFIG_SYSTEM="+fixed)
		}
		cmd.Dir = base
		return cmd.CombinedOutput()
	}
	for _, args := range [][]string{{"-C", source, "init", "--quiet"}, {"-C", source, "add", "proof.txt"}, {"-C", source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture"}} {
		if out, err := run(false, args...); err != nil {
			t.Fatalf("probe remote: %s %v", out, err)
		}
	}
	commitOut, err := run(false, "-C", source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(commitOut))
	for _, stageLength := range []int{254, 257} {
		stage := filepath.Join(base, strings.Repeat("g", stageLength-len(base)-1))
		if len(stage) != stageLength {
			t.Fatal("probe physical root boundary", len(stage))
		}
		if err := os.Mkdir(stage, 0700); err != nil {
			t.Fatal(err)
		}
		for _, v := range []struct {
			name   string
			gitDir string
			system bool
		}{{"isolated-command-config", "", false}, {"extended-git-dir", "extended", false}, {"isolated-system-config", "", true}, {"isolated-system-explicit-git-dir", "relative", true}} {
			if err := os.RemoveAll(filepath.Join(stage, ".git")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(stage, "proof.txt")); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			prefix := []string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.hooksPath=/dev/null", "-c", "core.longpaths=true", "-c", "init.templateDir=", "-C", stage}
			if v.gitDir == "extended" {
				prefix = append(prefix, "--git-dir="+strings.ReplaceAll(windowsExtendedPath(filepath.Join(stage, ".git")), "\\", "/"))
			} else if v.gitDir == "relative" {
				prefix = append(prefix, "--git-dir=.git")
			}
			steps := [][]string{{"init", "--quiet"}, {"remote", "add", "origin", source}, {"fetch", "--depth", "1", "origin", commit}, {"checkout", "--detach", commit}, {"rev-parse", "HEAD"}, {"status", "--porcelain"}}
			success := true
			for _, step := range steps {
				args := append(append([]string{}, prefix...), step...)
				out, err := run(v.system, args...)
				if len(out) > 8192 {
					out = append(out[:8192], []byte(" [bounded]")...)
				}
				t.Logf("NvimGitConfigBoundary variant=%s stage=%d config=%d step=%s exit=%v output=%q", v.name, len(stage), len(filepath.Join(stage, ".git", "config")), step[0], err, out)
				if err != nil {
					success = false
					break
				}
			}
			t.Logf("NvimGitConfigBoundary COMPLETE variant=%s stage=%d success=%t", v.name, len(stage), success)
			if success {
				data, err := os.ReadFile(filepath.Join(stage, "proof.txt"))
				if err != nil || string(data) != "locked proof\n" && string(data) != "locked proof\r\n" {
					t.Fatal("probe checkout content", string(data), err)
				}
			}
		}
	}
}
