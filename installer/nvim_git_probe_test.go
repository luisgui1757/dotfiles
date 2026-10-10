package installer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Temporary native proof: use the real default Windows data layout, a complete
// random operation identity, and unchanged private Git. Every expectation is fatal.
func probeNvimGitConfigBoundary(t *testing.T, git string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Fatal("Windows-only boundary proof")
	}
	t.Setenv("XDG_DATA_HOME", "")
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	runtimeDirectory, err := nvimRuntimeDirectory(Context{OS: "windows", Arch: "amd64"}, folders)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	version := filepath.Join(runtimeDirectory, "nvim.sync", "versions", hex.EncodeToString(nonce[:]))
	if err := os.MkdirAll(filepath.Dir(version), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(version, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(version); err != nil {
			t.Error(err)
		}
	})
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "source")
	nested := strings.Repeat("nested-managed-files/", 16) + "proof.txt"
	writeConfigFixture(t, filepath.Join(source, filepath.FromSlash(nested)), "locked long-file proof\n")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	run := func(args ...string) ([]byte, error) {
		prefix := []string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.hooksPath=/dev/null", "-c", "core.longpaths=true", "-c", "core.autocrlf=false", "-c", "init.templateDir="}
		cmd := exec.CommandContext(ctx, git, append(prefix, args...)...)
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_COUNT=0", "GIT_CONFIG_PARAMETERS=", "GIT_TEMPLATE_DIR=", "GIT_TERMINAL_PROMPT=0")
		cmd.Dir = base
		var output boundedCommandOutput
		output.limit = 8 << 10
		cmd.Stdout, cmd.Stderr = &output, &output
		err := cmd.Run()
		return output.data.Bytes(), err
	}
	checked := func(args ...string) string {
		t.Helper()
		out, err := run(args...)
		if err != nil {
			t.Fatalf("native default Git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	checked("-C", source, "init", "--quiet")
	checked("-C", source, "add", nested)
	checked("-C", source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture")
	commit := checked("-C", source, "rev-parse", "HEAD")
	branch := checked("-C", source, "symbolic-ref", "--short", "HEAD")
	for _, name := range []string{"lazy.nvim", "mason-tool-installer.nvim"} {
		target := filepath.Join(version, "payload", "lazy", name)
		stage := target + ".stage." + strconv.Itoa(os.Getpid())
		if err := os.MkdirAll(stage, 0700); err != nil {
			t.Fatal(err)
		}
		checked("-C", stage, "init", "--quiet")
		checked("-C", stage, "remote", "add", "origin", source)
		checked("-C", stage, "fetch", "--depth", "1", "--filter=blob:none", "origin", commit)
		checked("-C", stage, "checkout", "--detach", commit)
		ref := "refs/remotes/origin/" + branch
		checked("-C", stage, "update-ref", ref, commit)
		checked("-C", stage, "symbolic-ref", "refs/remotes/origin/HEAD", ref)
		if head := checked("-C", stage, "rev-parse", "HEAD"); head != commit {
			t.Fatal("staged identity differs", head, commit)
		}
		if err := os.Rename(stage, target); err != nil {
			t.Fatal(err)
		}
		if checked("-C", target, "rev-parse", "HEAD") != commit || checked("-C", target, "status", "--porcelain=v1", "--untracked-files=all") != "" || checked("-C", target, "symbolic-ref", "refs/remotes/origin/HEAD") != ref {
			t.Fatal("published identity/cleanliness differs")
		}
		checked("-C", target, "diff", "--quiet", "HEAD", "--")
		file := filepath.Join(target, filepath.FromSlash(nested))
		if data, err := os.ReadFile(file); err != nil || string(data) != "locked long-file proof\n" || len(file) <= 320 {
			t.Fatal("long published file bytes/boundary", len(file), string(data), err)
		}
		t.Logf("NvimGitNativeDefault complete name=%s stage=%d target=%d config=%d nested=%d", name, len(stage), len(target), len(filepath.Join(target, ".git", "config")), len(file))
	}

	// Retain the exact failing physical-root boundary as a strict refusal proof.
	stage := filepath.Join(base, strings.Repeat("g", 254-len(base)-1))
	if len(stage) != 254 {
		t.Fatal("unsupported-root proof lost exact length", len(stage))
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	out, err := run("-C", stage, "init", "--quiet")
	if err == nil || !strings.Contains(string(out), "Filename too long") {
		t.Fatal("expected explicit native root refusal", err, string(out))
	}
	t.Logf("NvimGitNativeDefault unsupported stage=254 config=266 explicitly refused: %s", out)
}
