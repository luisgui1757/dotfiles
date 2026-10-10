package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// No administrator access, global Git configuration, registry integration or
// installed package manager is used. Each archive is fresh inside the fixture.
func TestNativeWindowsGitAndMakeLifecycle(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_NATIVE_WINDOWS_GIT_MAKE") != "1" {
		t.Skip("requires explicit opt-in on a disposable GitHub-hosted Windows amd64 runner")
	}
	c, d, _ := archiveController(t)
	d.Directory = filepath.Join(c.Home, "private Git and Make ü")
	d.Client = nil
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		versions := filepath.Join(d.Directory, "tool.git", "versions")
		entries, err := os.ReadDir(versions)
		if err != nil {
			t.Log("Git preparation diagnostics:", err)
			return
		}
		for _, entry := range entries {
			payload := filepath.Join(versions, entry.Name(), "payload")
			for _, name := range []string{"portable-git.exe", "git-bash.exe", "post-install.bat", "etc/post-install", "usr/bin/bash.exe", "ucrt64/bin/git.exe"} {
				info, err := os.Stat(filepath.Join(payload, filepath.FromSlash(name)))
				t.Logf("Git preparation diagnostic path-length=%d file=%s info=%v error=%v", len(payload), name, info, err)
			}
		}
	})
	pins := windowsArchiveRecipePins(t)
	latest := pins["tool.git"]
	baseline := latest
	// Published GitHub release digest; the provider verifies downloaded bytes.
	baseline.Version = "2.56.0.windows.1"
	baseline.URL = "https://github.com/git-for-windows/git/releases/download/v2.56.0.windows.1/PortableGit-2.56.0-64-bit.7z.exe"
	baseline.SHA256 = "eceb5e061aa90df2f69ddd3e90f0030e1b8037a7829934bc40e4be1caa1accc1"
	d.Pins = map[string]ArchivePin{"tool.git": baseline, "tool.make": pins["tool.make"]}
	c.Catalog = &Catalog{Schema: 1, Resources: []Resource{
		{ID: "first", Name: "Git and GNU Make", Capability: true, Requires: []string{"tool.git", "tool.make"}},
		{ID: "tool.git", Name: "Git", Action: "archive"}, {ID: "tool.make", Name: "Make", Action: "archive"},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	session := &nativeSession{Directory: filepath.Join(c.Home, "worker")}
	session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(session.Directory) }
	release, err := session.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if filepath.Base(command.Program) == "bash.exe" && len(command.Arguments) == 7 && command.Arguments[4] == portableGitRuntimeProbe {
			// The fixed --version probe never authenticates. Upstream GCM prints
			// its exception stack only with tracing enabled; keep secret tracing off.
			command.Environment = append(command.Environment, "GCM_TRACE=1", "GCM_TRACE_SECRETS=0")
		}
		output, err := session.run(ctx, command)
		if err != nil && filepath.Base(command.Program) == "portable-git.exe" {
			diagnosePortableGitFailure(t, command, d.Pins["tool.git"])
		}
		return output, err
	}
	personal := filepath.Join(c.Home, "personal home ü")
	if err := os.Mkdir(personal, 0700); err != nil {
		t.Fatal(err)
	}
	const sentinel = "[alias]\n\tuntouched = status\n"
	personalConfig := filepath.Join(personal, ".gitconfig")
	if err := os.WriteFile(personalConfig, []byte(sentinel), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", personal)
	t.Setenv("USERPROFILE", personal)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	previous, err := d.PayloadPath("tool.git")
	if err != nil {
		t.Fatal(err)
	}
	d.Pins["tool.git"] = latest
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	gitRoot, err := d.PayloadPath("tool.git")
	if err != nil || gitRoot == previous {
		t.Fatal("changed-version Git update did not publish a new generation", err)
	}
	if _, err := os.Lstat(previous); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unchanged obsolete Git payload was not removed", err)
	}
	makeRoot, err := d.PayloadPath("tool.make")
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(c.Home, "offline repository ü")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	environment := []string{"SystemRoot=" + os.Getenv("SystemRoot"), "WINDIR=" + os.Getenv("SystemRoot"), "COMSPEC=" + filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"),
		"HOME=" + personal, "USERPROFILE=" + personal, "TEMP=" + work, "TMP=" + work, "GIT_CONFIG_GLOBAL=" + personalConfig, "GIT_CONFIG_COUNT=0",
		"PATH=" + strings.Join([]string{filepath.Join(gitRoot, "cmd"), filepath.Join(gitRoot, "bin"), filepath.Join(gitRoot, "usr", "bin"), filepath.Join(gitRoot, "ucrt64", "bin"), filepath.Join(makeRoot, "bin"), filepath.Join(os.Getenv("SystemRoot"), "System32")}, ";")}
	run := func(program, input string, args ...string) string {
		t.Helper()
		call, stop := context.WithTimeout(ctx, 45*time.Second)
		defer stop()
		command := exec.CommandContext(call, program, args...)
		command.Dir, command.Env, command.Stdin = work, environment, strings.NewReader(input)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("private Windows command %s %v: %v\n%s", filepath.Base(program), args, err, output)
		}
		return string(output)
	}
	git := filepath.Join(gitRoot, "cmd", "git.exe")
	if output := run(git, "", "--version"); !strings.Contains(output, latest.Version) {
		t.Fatal("published Git version differs", output)
	}
	if output := run(git, "", "config", "--type=bool", "--get", "core.longpaths"); strings.TrimSpace(output) != "true" {
		t.Fatal("published Git did not consume its private long-path setting", output)
	}
	run(git, "", "init", "--initial-branch=main", ".")
	run(git, "", "config", "--local", "user.name", "Offline fixture")
	run(git, "", "config", "--local", "user.email", "fixture@example.invalid")
	file := filepath.Join(work, "file ü.txt")
	if err := os.WriteFile(file, []byte("first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(git, "", "add", "--", "file ü.txt")
	run(git, "", "commit", "-m", "offline fixture")
	// Keep the physical ownership path and test builtin Git consumers beyond
	// MAX_PATH without a command-line or environment long-path override.
	longRoot := filepath.Join(c.Home, strings.Repeat("managed-generation"+string(filepath.Separator), 18))
	if err := os.MkdirAll(longRoot, 0700); err != nil {
		t.Fatal(err)
	}
	run(git, "", "init", "--initial-branch=main", filepath.Join(longRoot, "new ü"))
	longCheckout := filepath.Join(longRoot, "checkout ü")
	run(git, "", "clone", "--no-local", work, longCheckout)
	commit := strings.TrimSpace(run(git, "", "rev-parse", "HEAD"))
	run(git, "", "-C", longCheckout, "checkout", "--detach", commit)
	run(git, "", "-C", longCheckout, "diff", "--quiet", "HEAD", "--")
	if output := run(git, "", "-C", longCheckout, "rev-parse", "HEAD"); strings.TrimSpace(output) != commit {
		t.Fatal("long-path Git checkout differs from its pinned commit", output)
	}

	if err := os.WriteFile(file, []byte("first\nsecond\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(git, "y\n", "-c", "interactive.singleKey=false", "add", "--patch", "--", "file ü.txt")
	if output := run(git, "", "diff", "--cached"); !strings.Contains(output, "+second") {
		t.Fatal("interactive staging is unavailable", output)
	}
	run(git, "", "lfs", "install", "--local", "--skip-smudge")
	run(git, "", "lfs", "track", "*.bin")
	if err := os.WriteFile(filepath.Join(work, "asset.bin"), []byte("offline LFS bytes\x00\xff"), 0600); err != nil {
		t.Fatal(err)
	}
	run(git, "", "add", ".gitattributes", "asset.bin")
	run(git, "", "commit", "-m", "offline LFS fixture")
	if output := run(git, "", "show", "HEAD:asset.bin"); !strings.Contains(output, "version https://git-lfs.github.com/spec/v1") {
		t.Fatal("LFS clean filter did not run", output)
	}
	run(git, "", "lfs", "fsck")
	run(git, "", "credential-manager", "--version")
	run(filepath.Join(gitRoot, "usr", "bin", "ssh-keygen.exe"), "", "-q", "-t", "ed25519", "-N", "", "-f", filepath.Join(work, "fixture-key"))
	if output := run(filepath.Join(gitRoot, "usr", "bin", "ssh.exe"), "", "-F", "NUL", "-G", "-o", "IdentityFile="+filepath.Join(work, "fixture-key"), "fixture.invalid"); !strings.Contains(output, "hostname fixture.invalid") {
		t.Fatal("bundled SSH could not evaluate its offline connection configuration")
	}
	if output := run(filepath.Join(gitRoot, "bin", "bash.exe"), "", "--noprofile", "--norc", "-c", "printf '%s\\n' bash-ready; test -r /etc/mtab; test -d /dev/shm"); !strings.Contains(output, "bash-ready") {
		t.Fatal("bundled Bash cannot use its prepared MSYS filesystem")
	}
	makefile := "all:\n\t@printf '%s\\n' make-ready > output.txt\n"
	if err := os.WriteFile(filepath.Join(work, "Makefile"), []byte(makefile), 0600); err != nil {
		t.Fatal(err)
	}
	makeCommand := filepath.Join(makeRoot, "bin", "make.exe")
	if output := run(makeCommand, "", "--version"); !strings.Contains(output, "GNU Make 4.4.1") {
		t.Fatal("GNU Make was replaced by another make implementation", output)
	}
	run(makeCommand, "", "SHELL="+filepath.ToSlash(filepath.Join(gitRoot, "bin", "sh.exe")), "all")
	if output, err := os.ReadFile(filepath.Join(work, "output.txt")); err != nil || strings.TrimSpace(string(output)) != "make-ready" {
		t.Fatal("GNU Make could not execute a recipe through the private Git shell", err)
	}
	check, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("runtime usage changed immutable archive payloads", check.Status, err)
	}
	data, err := os.ReadFile(personalConfig)
	if err != nil || string(data) != sentinel {
		t.Fatal("personal Git configuration was changed", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, payload := range []string{gitRoot, makeRoot} {
		if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("private owned package was not removed", err)
		}
	}
}
