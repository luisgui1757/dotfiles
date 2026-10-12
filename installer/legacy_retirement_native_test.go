package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Disposable-runner acceptance for runtime retirement. Each test installs the
// published v0.4.4 setup, including its checksum-pinned upstream Nix
// prerequisite helper, into the runner account, then migrates through the
// installed entrypoint and machine protocol. Never run these on a workstation.

func requireLegacyRetirementRunner(t *testing.T, goos string) string {
	t.Helper()
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_LEGACY_RETIREMENT") != "1" {
		t.Skip("set DOTFILES_TEST_NATIVE_PACKAGES=1 and DOTFILES_TEST_LEGACY_RETIREMENT=1 on a disposable runner")
	}
	if runtime.GOOS != goos {
		t.Skip("this acceptance targets " + goos)
	}
	if os.Geteuid() == 0 {
		t.Fatal("the released helper requires a non-root account with sudo")
	}
	if _, err := os.Lstat("/nix"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("legacy retirement acceptance requires a host that never had Nix", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func legacyRun(t *testing.T, directory string, environment []string, program string, arguments ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), environment...)
	output := boundedCommandOutput{limit: 8 << 20}
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		// A released installer can report a dependency error long before its
		// final summary. Preserve all bounded fixture output on failure, while
		// retaining terminal-control filtering and redacting CI credentials
		// before chunking (a split token would evade the hosted log masker).
		diagnostic := output.data.Bytes()
		for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if value := os.Getenv(key); value != "" {
				diagnostic = bytes.ReplaceAll(diagnostic, []byte(value), []byte("[redacted]"))
			}
		}
		for chunk := range slices.Chunk(diagnostic, 8192) {
			t.Logf("native fixture output:%s", nativeDiagnostic(chunk))
		}
		t.Fatalf("%s %v: %v", program, arguments, err)
	}
	return output.data.String()
}

// installReleasedDotfiles runs the documented first-run path of the release.
func installReleasedDotfiles(t *testing.T, helperOnly bool) string {
	t.Helper()
	released := filepath.Join(t.TempDir(), "dotfiles-pre-overhaul")
	legacyRun(t, "", nil, "git", "clone", "--no-checkout", "--branch", "v0.4.4", "--single-branch", "https://github.com/luisgui1757/dotfiles.git", released)
	legacyRun(t, released, nil, "git", "checkout", "--detach", legacyStableCommit)
	if head := strings.TrimSpace(legacyRun(t, released, nil, "git", "rev-parse", "HEAD")); head != legacyStableCommit {
		t.Fatal("released clone differs from the reviewed commit", head)
	}
	if helperOnly {
		legacyRun(t, released, nil, filepath.Join(released, "scripts", "install-nix-prerequisite.sh"), "--install")
	} else {
		prepareLegacyARMCode(t)
		legacyRun(t, released, nil, filepath.Join(released, "setup.sh"), "--all", "--skip-nvim", "--skip-agents")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	nix := legacyInstalledNixProgram(t, home)
	if version := strings.TrimSpace(legacyRun(t, "", nil, nix, "--version")); version != legacyNixVersion {
		t.Fatal("the released helper did not install upstream Nix 2.34.0", version)
	}
	return released
}

// The released Linux installer accepts an existing Code CLI, but otherwise
// selects Snap on hosted Ubuntu. Code's stable Snap has no arm64 build. Supply
// the real supported Microsoft DEB as pre-existing software, without changing
// the immutable release or hiding an unsuccessful installation.
func prepareLegacyARMCode(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "arm64" {
		return
	}
	if _, err := exec.LookPath("code"); err == nil {
		return
	} else if !errors.Is(err, exec.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("snap"); errors.Is(err, exec.ErrNotFound) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	pin, err := defaultLinuxCodePin(runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	data := legacyPinnedDownload(t, pin.URL, pin.SHA256)
	if int64(len(data)) != pin.Size {
		t.Fatal("the pinned Code package has an unexpected size")
	}
	file := filepath.Join(t.TempDir(), "code.deb")
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	op, err := digest("legacy arm64 Code fixture")
	if err != nil {
		t.Fatal(err)
	}
	input, err := newLinuxCodeInput(pin, op)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.cleanup(); err != nil {
			t.Error(err)
		}
	})
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/env", "DEBIAN_FRONTEND=noninteractive",
		"DEBCONF_DB_OVERRIDE=File{filename:"+filepath.Join(input.Directory, "answer.dat")+" accept_name:^code/add-microsoft-repo$ mode:0644 backup:no}",
		"/usr/bin/apt-get", "install", "--yes", "--no-install-recommends", file)
	version := legacyRun(t, "", nil, "/usr/bin/code", "--version")
	if !strings.HasPrefix(version, strings.SplitN(pin.Version, "-", 2)[0]+"\n") {
		t.Fatal("the real pre-existing Code CLI did not run", version)
	}
	t.Cleanup(func() {
		if after := legacyRun(t, "", nil, "/usr/bin/code", "--version"); after != version {
			t.Error("retirement changed pre-existing Code", after)
		}
		if output := legacyRun(t, "", nil, "/usr/bin/dpkg", "--verify", "code"); strings.TrimSpace(output) != "" {
			t.Error("retirement changed pre-existing Code package files", output)
		}
	})
}

// The released helper uses the system profile for daemon installs and the
// user's profile for single-user installs. Bind the actual executable before
// an operation so a preservation check cannot switch to another profile.
func legacyInstalledNixProgram(t *testing.T, home string) string {
	t.Helper()
	program := "/nix/var/nix/profiles/default/bin/nix"
	if _, err := os.Stat(program); errors.Is(err, os.ErrNotExist) {
		return filepath.Join(home, ".nix-profile", "bin", "nix")
	} else if err != nil {
		t.Fatal(err)
	}
	return program
}

type legacyInstalled struct {
	t                        *testing.T
	binary, repository, home string
}

func newLegacyInstalled(t *testing.T, home string) legacyInstalled {
	t.Helper()
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "dotfiles")
	if output, err := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/dotfiles").CombinedOutput(); err != nil {
		t.Fatalf("build installed entrypoint: %s %v", output, err)
	}
	return legacyInstalled{t: t, binary: binary, repository: repository, home: home}
}

func (l legacyInstalled) command(request Request, entrypoint, phase string) (*exec.Cmd, *boundedCommandOutput, *boundedCommandOutput) {
	l.t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		l.t.Fatal(err)
	}
	command := exec.Command(l.binary, "machine")
	command.Env = append(os.Environ(), "HOME="+l.home, "DOTFILES_CHECKOUT="+l.repository, "DOTFILES_ENTRYPOINT="+entrypoint, "TERM=xterm-256color")
	if phase != "" {
		command.Env = append(command.Env, "DOTFILES_MIGRATION_PHASE="+phase)
	}
	command.Stdin = bytes.NewReader(data)
	output, diagnostic := &boundedCommandOutput{limit: 8 << 20}, &boundedCommandOutput{limit: 1 << 20}
	command.Stdout, command.Stderr = output, diagnostic
	return command, output, diagnostic
}

// needs-action exits nonzero after printing its JSON result; decode it.
func (l legacyInstalled) run(request Request, entrypoint, phase string) Result {
	l.t.Helper()
	command, output, diagnostic := l.command(request, entrypoint, phase)
	err := command.Run()
	var result Result
	if decodeErr := Decode(output.data.Bytes(), &result); decodeErr != nil {
		l.t.Fatalf("installed entrypoint returned no result: %v %v%s", err, decodeErr, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) || err == nil && (result.Status == "failed" || result.Status == "needs-action") {
		l.t.Fatalf("installed entrypoint exit does not match %s: %v", result.Status, err)
	}
	return result
}

func (l legacyInstalled) approve(request Request, entrypoint, phase string) Result {
	l.t.Helper()
	request.ExpectedPlan = ""
	preview := l.run(request, entrypoint, phase)
	if preview.Status != "preview" {
		l.t.Fatal("expected a preview", preview)
	}
	request.ExpectedPlan = preview.Plan.ID
	return l.run(request, entrypoint, phase)
}

func (l legacyInstalled) migrateProfiles() {
	l.t.Helper()
	check := l.run(Request{Schema: 1, Mode: "check"}, "migrate", "profiles")
	selected := []string{}
	for _, line := range strings.Split(check.Message, "\n") {
		if rest, ok := strings.CutPrefix(line, "Review "); ok {
			id, _, _ := strings.Cut(rest, ":")
			selected = append(selected, id)
		}
	}
	if len(selected) == 0 {
		l.t.Fatal("the released installation has no detachable profiles or links", check)
	}
	if result := l.approve(Request{Schema: 1, Mode: "apply", Selected: selected, Adopt: selected}, "migrate", "profiles"); result.Status != "ready" {
		l.t.Fatal(result)
	}
}

func (l legacyInstalled) setup(selection []string) {
	l.t.Helper()
	request := Request{Schema: 1, Mode: "apply", Selected: selection, Adopt: []string{}}
	for round := 0; round < 6; round++ {
		preview := l.run(request, "setup", "")
		adopt := slices.Clone(request.Adopt)
		for _, op := range preview.Plan.Operations {
			if op.Action == "pending" && op.Observed.Present && op.Observed.Adoptable && op.Observed.Pending == "" {
				adopt = append(adopt, op.Resource)
			}
		}
		if adopt = sortedUnique(adopt); !request.Retry && !slices.Equal(adopt, sortedUnique(request.Adopt)) {
			request.Adopt = adopt
			continue
		}
		request.ExpectedPlan = preview.Plan.ID
		result := l.run(request, "setup", "")
		request.ExpectedPlan = ""
		switch {
		case result.Status == "ready":
			return
		case result.Status == "needs-action" && strings.Contains(result.Message, "Resume to review"):
			request = Request{Schema: 1, Mode: "apply", Retry: true}
		default:
			l.t.Fatal("setup did not complete", result)
		}
	}
	l.t.Fatal("setup did not converge")
}

// Retry the saved runtime operation until its check is ready.
func (l legacyInstalled) finishRuntime() Result {
	l.t.Helper()
	for round := 0; round < 8; round++ {
		check := l.run(Request{Schema: 1, Mode: "check"}, "migrate", "runtime")
		if check.Status == "ready" {
			return check
		}
		l.t.Logf("runtime retirement round %d check (%s):%s", round+1, check.Status, nativeDiagnostic([]byte(check.Message)))
		request := Request{Schema: 1, Mode: "apply", Retry: true}
		if check.Plan.InterruptedSource == "" {
			// A retired manager consumer can make its former dependency eligible
			// only on a fresh preview. Never widen the preceding approval.
			request = Request{Schema: 1, Mode: "apply", Selected: l.runtimeSelection()}
		}
		result := l.approve(request, "migrate", "runtime")
		l.t.Logf("runtime retirement round %d apply (%s):%s", round+1, result.Status, nativeDiagnostic([]byte(result.Message)))
	}
	l.t.Fatal("runtime retirement did not converge")
	return Result{}
}

func (l legacyInstalled) runtimeSelection() []string {
	selection := []string{}
	for _, r := range legacyRetirementCatalog().Capabilities(Context{OS: runtime.GOOS, Arch: runtime.GOARCH}) {
		selection = append(selection, r.ID)
	}
	return selection
}

func legacyFileHash(t *testing.T, path string) string {
	t.Helper()
	sum, err := legacyFileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func requireLegacyAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Error("retired artifact remains:", path, err)
		}
	}
}

func TestNativeLegacyRetirementLinuxReleasedSetup(t *testing.T) {
	legacyLinuxRetirement(t, true)
}

func TestNativeLegacyRetirementLinuxSingleUserReleasedSetup(t *testing.T) {
	legacyLinuxRetirement(t, false)
}

func legacyLinuxRetirement(t *testing.T, daemon bool) {
	home := requireLegacyRetirementRunner(t, "linux")
	_, systemdErr := os.Stat("/run/systemd/system")
	if daemon && systemdErr != nil || !daemon && !errors.Is(systemdErr, os.ErrNotExist) {
		t.Fatal("fixture daemon mode differs from observed systemd", daemon, systemdErr)
	}
	installReleasedDotfiles(t, false)
	systemConfig, err := snapshotConfig("/etc/nix")
	if err != nil {
		t.Fatal(err)
	}
	// The released account evidence this migration must retire.
	expected := []string{filepath.Join(home, ".local", "share", "dotfiles", "tmux-plugins", "tpm"),
		filepath.Join(home, ".config", "chezmoi", "chezmoi.toml"), filepath.Join(home, ".local", "bin", "pi")}
	if daemon {
		expected = append(expected, "/etc/systemd/system/nix-daemon.service")
	}
	for _, path := range expected {
		if _, err := os.Lstat(path); err != nil {
			t.Fatal("released setup did not produce", path, err)
		}
	}
	// Use an actual shell from the verified Nix bootstrap closure. The fixture
	// keeps a live login-shell path until migration restores a system shell.
	activation, err := os.ReadFile(filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home", "activate"))
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(activation), "\n")
	nixShell := strings.TrimSpace(strings.TrimPrefix(first, "#!"))
	if !strings.HasPrefix(first, "#!") || !strings.HasPrefix(nixShell, "/nix/store/") || !strings.HasSuffix(nixShell, "/bin/bash") {
		t.Fatal("Home Manager activation does not identify a Nix Bash", first)
	}
	legacyRun(t, "", nil, nixShell, "--version")
	if resolved, err := filepath.EvalSymlinks(nixShell); err != nil || !strings.HasPrefix(resolved, "/nix/store/") {
		t.Fatal("released Home Manager did not provide its package profile", resolved, err)
	}
	piData := filepath.Join(home, ".pi", "agent", "fixture-personal.json")
	writeConfigFixture(t, piData, "{\"keep\":true}\n")
	userConfig := filepath.Join(home, ".config", "nix", "nix.conf")
	writeConfigFixture(t, userConfig, "# personal Nix settings stay\nmax-jobs = 2\n")
	userConfigHash := legacyFileHash(t, userConfig)

	installed := newLegacyInstalled(t, home)
	installed.migrateProfiles()
	installed.setup([]string{"git", "zsh", "starship", "lazygit", "ripgrep", "fd", "fzf", "zoxide", "tmux", "herdr"})
	if check := installed.run(Request{Schema: 1, Mode: "check"}, "setup", ""); check.Status != "ready" {
		t.Fatal(check)
	}
	// The fresh release sets its account shell to Linuxbrew zsh. Exercise
	// that actual login before separately testing a Nix-provided shell.
	if check := installed.run(Request{Schema: 1, Mode: "check"}, "migrate", "runtime"); strings.Contains(check.Message, "Remaining retire.delta.zsh:") {
		result := installed.approve(Request{Schema: 1, Mode: "apply", Selected: []string{"retire.delta.zsh"}}, "migrate", "runtime")
		if result.Status != "ready" && result.Status != "needs-action" {
			t.Fatal("retire the released Linuxbrew login shell", result)
		}
		if check := installed.run(Request{Schema: 1, Mode: "check"}, "migrate", "runtime"); strings.Contains(check.Message, "Remaining retire.delta.zsh:") {
			t.Fatal("the released Linuxbrew zsh remains", check)
		}
		legacyAssertLinuxLogin(t)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/sh", "-c", `printf '%s\n' "$1" >> /etc/shells`, "sh", nixShell)
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/chsh", "-s", nixShell, legacyNativeUsername(t))
	legacyOtherAccountBlocks(t, installed)
	pending := installed.run(Request{Schema: 1, Mode: "check"}, "migrate", "runtime")
	for _, id := range []string{"retire.chezmoi", "retire.nix", "retire.pi", "retire.tools"} {
		if pending.Status != "needs-action" || !strings.Contains(pending.Message, "Remaining "+id) {
			t.Fatal("runtime phase must disclose", id, pending)
		}
	}

	// Kill the controller after the worker has durably started the
	// irreversible store deletion. The worker finishes; recovery reads its
	// saved result instead of repeating or guessing.
	request := Request{Schema: 1, Mode: "apply", Selected: installed.runtimeSelection()}
	preview := installed.run(request, "migrate", "runtime")
	if preview.Status != "preview" || operation(t, preview.Plan, "retire.nix").Action != "install" {
		t.Fatal(preview)
	}
	request.ExpectedPlan = preview.Plan.ID
	controller, output, diagnostic := installed.command(request, "migrate", "runtime")
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- controller.Wait() }()
	state := filepath.Join(home, ".local", "state", "dotfiles")
	probe := &legacyRetirementDriver{Directory: filepath.Join(state, "migration", "retirement"), WorkerDirectory: filepath.Join(state, "native", "worker"),
		Folders: ConfigFolders{Home: home}, Environment: legacyRuntimeEnvironment{Username: legacyNativeUsername(t)}}
	killed := false
	for deadline := time.Now().Add(90 * time.Minute); !killed && time.Now().Before(deadline); {
		select {
		case err := <-done:
			// Wait has returned, so the outputs are complete and no longer written.
			t.Fatalf("controller finished before the irreversible step could be interrupted: %v%s%s", err, nativeDiagnostic(output.data.Bytes()), nativeDiagnostic(diagnostic.data.Bytes()))
		case <-time.After(100 * time.Millisecond):
		}
		var err error
		killed, err = legacyIrreversibleDispatched(probe)
		if err != nil {
			t.Fatal("inspect worker dispatch", err)
		}
	}
	if !killed {
		t.Fatal("the irreversible step was never dispatched")
	}
	if err := controller.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-done
	for deadline := time.Now().Add(30 * time.Minute); ; time.Sleep(time.Second) {
		release, err := lockFile(probe.WorkerDirectory, false)
		if err == nil {
			if err := release(); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the native worker did not finish the started deletion", err)
		}
	}
	installed.finishRuntime()

	requireLegacyAbsent(t, "/nix", "/etc/systemd/system/nix-daemon.service", "/etc/systemd/system/nix-daemon.socket", "/etc/tmpfiles.d/nix-daemon.conf",
		filepath.Join(home, ".local", "bin", "chezmoi"), filepath.Join(home, ".local", "bin", "pi"), filepath.Join(home, ".config", "chezmoi"),
		filepath.Join(home, ".local", "share", "dotfiles", "tmux-plugins", "tpm"), filepath.Join(home, ".local", "share", "dotfiles", "provenance", "chezmoi.env"),
		filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-coding-agent"), filepath.Join(home, ".nix-profile"))
	for _, name := range []string{"nixbld1", "nixbld32"} {
		if output, err := exec.Command("/usr/bin/getent", "passwd", name).CombinedOutput(); err == nil {
			t.Error("Nix build account remains", string(output))
		}
	}
	if output, err := exec.Command("/usr/bin/getent", "group", "nixbld").CombinedOutput(); err == nil {
		t.Error("Nix build group remains", string(output))
	}
	if shell := strings.Split(strings.TrimSpace(legacyRun(t, "", nil, "/usr/bin/getent", "passwd", legacyNativeUsername(t))), ":"); shell[len(shell)-1] != "/bin/bash" {
		t.Error("the Nix login shell was not replaced", shell)
	}
	legacyAssertLinuxLogin(t)
	if shells, err := os.ReadFile("/etc/shells"); err != nil || strings.Contains(string(shells), nixShell) {
		t.Error("the retired Nix shell is still registered", err)
	}
	if data, err := os.ReadFile(piData); err != nil || string(data) != "{\"keep\":true}\n" {
		t.Error("Pi personal data changed", err)
	}
	if legacyFileHash(t, userConfig) != userConfigHash {
		t.Error("personal Nix configuration changed")
	}
	if daemon {
		requireLegacyAbsent(t, "/etc/nix")
		backups, err := filepath.Glob("/etc/.dotfiles-retired-*-nix")
		if err != nil || len(backups) != 1 {
			t.Error("the retired system Nix configuration was not preserved", backups, err)
		} else if err := verifyConfigSnapshot(backups[0], systemConfig); err != nil {
			t.Error("the system Nix configuration backup changed", err)
		}
	} else if err := verifyConfigSnapshot("/etc/nix", systemConfig); err != nil {
		t.Error("single-user retirement changed unrelated system configuration", err)
	}
	if check := installed.run(Request{Schema: 1, Mode: "check"}, "setup", ""); check.Status != "ready" {
		t.Fatal("replacement setup broke after retirement", check)
	}
}

func legacyAssertLinuxLogin(t *testing.T) {
	t.Helper()
	output := legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/su", "--login", legacyNativeUsername(t), "--command", `printf '%s\n' dotfiles-login-ready`)
	if strings.TrimSpace(output) != "dotfiles-login-ready" {
		t.Fatal("the account's configured login shell did not start cleanly", output)
	}
}

func legacyNativeUsername(t *testing.T) string {
	t.Helper()
	account, err := user.Current()
	if err != nil || account.Username == "" {
		t.Fatal("the native fixture account could not be identified", err)
	}
	return account.Username
}

// legacyIrreversibleDispatched reports whether the worker saved its intent for
// the next irreversible Nix step of the saved operation.
func legacyIrreversibleDispatched(probe *legacyRetirementDriver) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(probe.Directory, "operations"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		// Observe only committed journals, as nixInterruptedDeletions does.
		// saveDocument concurrently stages .state-* files in this directory.
		operation, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !operationID.MatchString(operation) {
			continue
		}
		data, err := readDocument(filepath.Join(probe.Directory, "operations", entry.Name()))
		var intent legacyRetirementIntent
		if err != nil {
			// Poll again within the caller's deadline when atomic publication
			// replaces the opened inode. An exact match keeps joined close or
			// other read errors fatal; a failed observation never proves dispatch.
			if err.Error() == "installer document changed while opening; retry discovery" {
				return false, nil
			}
			return false, err
		}
		if err := Decode(data, &intent); err != nil {
			return false, err
		}
		if intent.Resource != "retire.nix" || intent.Complete || intent.Next >= len(intent.Scope.Steps) || !intent.Scope.Steps[intent.Next].Irreversible {
			continue
		}
		command, err := probe.stepCommand(intent, intent.Next)
		if err != nil {
			return false, err
		}
		if _, err := os.Lstat(filepath.Join(probe.WorkerDirectory, "commands", command.Operation+".json")); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func TestNativeLegacyRetirementBlocksAnotherNixAccount(t *testing.T) {
	home := requireLegacyRetirementRunner(t, "linux")
	installReleasedDotfiles(t, true)
	installed := newLegacyInstalled(t, home)
	installed.setup([]string{})
	legacyOtherAccountBlocks(t, installed)
}

func legacyOtherAccountBlocks(t *testing.T, installed legacyInstalled) {
	t.Helper()
	nix := legacyInstalledNixProgram(t, installed.home)
	intents := filepath.Join(installed.home, ".local", "state", "dotfiles", "migration", "retirement", "operations")
	before, err := snapshotTree(intents, 16<<20, 1024)
	if err != nil {
		t.Fatal(err)
	}
	other := "/nix/var/nix/profiles/per-user/fixture-other"
	if _, err := os.Lstat(other); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("outside-account fixture already exists", err)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/mkdir", "-p", other)
	profile, err := filepath.EvalSymlinks(filepath.Join(installed.home, ".nix-profile"))
	if err != nil {
		t.Fatal(err)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/ln", "-s", profile, filepath.Join(other, "profile"))
	result := installed.approve(Request{Schema: 1, Mode: "apply", Selected: []string{"retire.nix"}}, "migrate", "runtime")
	if result.Status != "needs-action" || !strings.Contains(result.Message, "account fixture-other has Nix profiles") {
		t.Fatal("outside Nix account did not block retirement", result)
	}
	if version := strings.TrimSpace(legacyRun(t, "", nil, nix, "--version")); version != legacyNixVersion {
		t.Fatal("blocked retirement changed Nix", version)
	}
	if err := verifyConfigSnapshot(intents, before); err != nil {
		t.Fatal("blocked retirement created or changed mutation intent", err)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rm", "--", filepath.Join(other, "profile"))
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rmdir", "--", other)
	installed.approve(Request{Schema: 1, Mode: "abandon"}, "migrate", "runtime")
}

func TestNativeLegacyRetirementDarwinReleasedSetup(t *testing.T) {
	home := requireLegacyRetirementRunner(t, "darwin")
	systemFiles := map[string]string{}
	for _, path := range []string{"/etc/zshrc", "/etc/bashrc"} {
		systemFiles[path] = legacyFileHash(t, path)
	}
	prepareLegacyDarwinBrew(t)
	installReleasedDotfiles(t, false)
	t.Log("released Nix build-account attributes:", legacyRun(t, "", nil, "/usr/bin/dscl", ".", "-read", "/Users/_nixbld1",
		"UniqueID", "PrimaryGroupID", "RealName", "NFSHomeDirectory", "UserShell", "IsHidden"))
	if target, err := os.Readlink("/opt/homebrew/bin/brew"); err != nil || !strings.HasPrefix(target, "/nix/store/") {
		t.Fatal("released setup did not produce nix-homebrew", target, err)
	}
	uuid, _, err := legacyNixFstabLine()
	if err != nil || uuid == "" {
		t.Fatal("released helper did not record the Nix Store volume", err)
	}
	inventory := func() string {
		d := legacyNativeDriver(t)
		d.Platform.Homebrew, err = DiscoverHomebrew(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		value, err := d.brewPreservationIdentity(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := inventory()
	location, err := DiscoverHomebrew(context.Background())
	if err != nil || location == nil {
		t.Fatal("released Homebrew is unavailable", err)
	}
	fontconfigPath := filepath.Join(location.Cellar, "fontconfig")
	fontconfigBefore, err := snapshotTree(fontconfigPath, 64<<20, 8192)
	if err != nil {
		t.Fatal(err)
	}
	installed := newLegacyInstalled(t, home)
	installed.migrateProfiles()
	converted := installed.approve(Request{Schema: 1, Mode: "apply", Selected: []string{"retire.homebrew"}}, "migrate", "runtime")
	if operation(t, converted.Plan, "retire.homebrew").Action != "install" || strings.Contains(converted.Message, "Remaining retire.homebrew") {
		t.Fatal("Homebrew conversion did not complete", converted)
	}
	if info, err := os.Lstat("/opt/homebrew/bin/brew"); err != nil || !info.Mode().IsRegular() {
		t.Fatal("brew still runs from the Nix store", err)
	}
	if after := inventory(); after != before {
		t.Fatalf("Homebrew package inventory changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	// Brew 6.0.1 cannot load every current runner cask. The file-record
	// equality above proves conversion preserved the released baseline; read
	// both semantic inventories through the converted, supported Brew.
	location, err = DiscoverHomebrew(context.Background())
	if err != nil || location == nil {
		t.Fatal("converted Homebrew is unavailable", err)
	}
	packages, err := brewInventory(context.Background(), location.query)
	if err != nil {
		t.Fatal(err)
	}
	fontconfig, present := packages["fontconfig"]
	consumers := legacyBrewConsumers(packages, "fontconfig", nil)
	if !present || fontconfig.Source != "fontconfig" || fontconfig.Held || fontconfig.Automatic != (len(consumers) != 0) {
		t.Fatal("unsupported released fontconfig baseline; preserve requested shared or unknown packages", fontconfig, consumers)
	}
	t.Log("released fontconfig baseline after verified conversion:", legacyBrewIdentity(fontconfig), "outside consumers:", consumers)
	// `brew ruby` is a developer command and persistently changes Homebrew's
	// update channel. Conversion must use the dedicated runtime bootstrap.
	configuration := legacyRun(t, "", nil, "/usr/bin/git", "config", "--file", "/opt/homebrew/.git/config", "--list")
	if strings.Contains(configuration, "homebrew.devcmdrun=") {
		t.Fatal("Homebrew conversion enabled developer mode", configuration)
	}
	installed.setup([]string{"git", "zsh", "starship", "lazygit", "ripgrep", "fd", "fzf", "zoxide", "tmux", "aerospace"})
	result := installed.approve(Request{Schema: 1, Mode: "apply", Selected: installed.runtimeSelection()}, "migrate", "runtime")
	if result.Status != "ready" {
		installed.finishRuntime()
	}
	location, err = DiscoverHomebrew(context.Background())
	if err != nil || location == nil {
		t.Fatal("converted Homebrew is unavailable", err)
	}
	remaining, err := brewInventory(context.Background(), location.query)
	if err != nil {
		t.Fatal(err)
	}
	if fontconfig.Automatic {
		if !reflect.DeepEqual(fontconfig, remaining["fontconfig"]) || !slices.Equal(consumers, legacyBrewConsumers(remaining, "fontconfig", nil)) {
			t.Fatal("retirement changed shared fontconfig or its outside consumers")
		}
		if err := verifyConfigSnapshot(fontconfigPath, fontconfigBefore); err != nil {
			t.Fatal("retirement changed the shared fontconfig payload", err)
		}
		for _, name := range consumers {
			if !reflect.DeepEqual(packages[name], remaining[name]) {
				t.Fatal("retirement changed an outside fontconfig consumer", name)
			}
		}
		check := installed.run(Request{Schema: 1, Mode: "check"}, "migrate", "runtime")
		if check.Status != "ready" || !strings.Contains(check.Message, "Kept retire.delta.fontconfig:") {
			t.Fatal("automatic shared fontconfig was not truthfully retained", check)
		}
	} else {
		if _, present := remaining["fontconfig"]; present {
			t.Fatal("the requested, unshared legacy fontconfig was not retired")
		}
		requireLegacyAbsent(t, fontconfigPath)
	}
	requireLegacyAbsent(t, "/run/current-system", "/etc/static", "/etc/nix", "/Library/LaunchDaemons/org.nixos.nix-daemon.plist",
		"/Library/LaunchDaemons/org.nixos.darwin-store.plist", "/Library/LaunchDaemons/org.nixos.activate-system.plist", filepath.Join(home, ".nix-profile"))
	for path, hash := range systemFiles {
		if legacyFileHash(t, path) != hash {
			t.Error("nix-darwin did not restore the original", path)
		}
	}
	if output, err := exec.Command("/usr/sbin/diskutil", "info", uuid).CombinedOutput(); err == nil {
		t.Error("the Nix Store volume remains", string(output))
	}
	for _, path := range []string{"/etc/fstab", "/etc/synthetic.conf"} {
		if data, err := os.ReadFile(path); err == nil && (strings.Contains(string(data), uuid) || slices.Contains(strings.Split(string(data), "\n"), "nix")) {
			t.Error("boot-time Nix reference remains in", path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	}
	if users := legacyRun(t, "", nil, "/usr/bin/dscl", ".", "-list", "/Users"); strings.Contains(users, "_nixbld") {
		t.Error("Nix build accounts remain")
	}
	if shell := legacyRun(t, "", nil, "/usr/bin/dscl", ".", "-read", "/Users/"+legacyNativeUsername(t), "UserShell"); strings.Contains(shell, "/nix/") || strings.Contains(shell, "/run/") {
		t.Error("the account still uses a Nix login shell", shell)
	}
	if check := installed.run(Request{Schema: 1, Mode: "check"}, "setup", ""); check.Status != "ready" {
		t.Fatal("replacement setup broke after retirement", check)
	}
	// Removing the old cask must leave the private replacement launchable
	// through its published Applications entry, despite the shared bundle ID.
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	target := nativePlatform(t)
	stateDirectory, err := DiscoverStateDirectory(target.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(target)
	if err != nil {
		t.Fatal(err)
	}
	archives := &ArchiveDriver{Directory: filepath.Join(stateDirectory, "packages"), Pins: pins}
	published := &DesktopDriver{Target: target.Context, Folders: folders, Directory: stateDirectory, Archives: archives}
	desktopConsumePublished(t, published, archives, "aerospace", home, []string{"HOME=" + home})
}
