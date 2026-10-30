package installer

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// sentinelProcesses fakes git and the Sentinel installer at the process
// boundary. Fake git checks out sentinelFakeFiles and reports head as the
// commit of a new checkout.
type sentinelProcesses struct {
	t        *testing.T
	head     string
	check    int
	commands []nativeCommand
	// install runs when the installer writes. A non-nil error stops it.
	install func(nativeCommand) error
}

const fakeSentinelGit, fakeSentinelShell = "/fake/git", "/fake/shell"

var sentinelFakeFiles = map[string]string{"tools/install": "#!/bin/sh\n", "tools/install.ps1": "# install\n", "rules/AGENTS.md": "# Working rules\n"}

func gitBlob(data []byte) string {
	sum := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(data))), data...))
	return hex.EncodeToString(sum[:])
}

func (f *sentinelProcesses) run(_ context.Context, command nativeCommand) ([]byte, error) {
	f.t.Helper()
	f.commands = append(f.commands, command)
	if command.Program == fakeSentinelGit {
		at := slices.Index(command.Arguments, "-C")
		directory, arguments := command.Arguments[at+1], command.Arguments[at+2:]
		switch arguments[0] {
		case "init":
			return nil, os.MkdirAll(filepath.Join(directory, ".git"), 0700)
		case "checkout":
			for path, contents := range sentinelFakeFiles {
				writeConfigFixture(f.t, filepath.Join(directory, filepath.FromSlash(path)), contents)
				if path == "tools/install" {
					if err := os.Chmod(filepath.Join(directory, "tools", "install"), 0755); err != nil {
						return nil, err
					}
				}
			}
			return nil, os.WriteFile(filepath.Join(directory, ".git", "commit"), []byte(f.head), 0600)
		case "rev-parse":
			return os.ReadFile(filepath.Join(directory, ".git", "commit"))
		case "ls-tree":
			listing := ""
			for path, contents := range sentinelFakeFiles {
				mode := "100644"
				if path == "tools/install" {
					mode = "100755"
				}
				listing += mode + " blob " + gitBlob([]byte(contents)) + "\t" + path + "\x00"
			}
			return []byte(listing), nil
		case "hash-object":
			hashes := ""
			for _, path := range strings.Fields(string(command.Input)) {
				data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(path)))
				if err != nil {
					return nil, err
				}
				hashes += gitBlob(data) + "\n"
			}
			return []byte(hashes), nil
		}
		return nil, nil
	}
	switch command.Arguments[len(command.Arguments)-1] {
	case "--check", "-Check":
		if f.check != 0 {
			code := f.check
			return []byte("ok:        /home/AGENTS.md\nDRIFT:     /home/.agents/skills/council\n"), &nativeCommandError{Message: "check failed", ExitCode: &code}
		}
	case "--remove", "-Remove":
	default:
		if f.install != nil {
			return nil, f.install(command)
		}
	}
	return nil, nil
}

func (f *sentinelProcesses) installers() []nativeCommand {
	return slices.DeleteFunc(slices.Clone(f.commands), func(c nativeCommand) bool { return c.Program == fakeSentinelGit })
}

func sentinelFixture(t *testing.T, target Context) (*SentinelDriver, *sentinelProcesses) {
	t.Helper()
	base := t.TempDir()
	processes := &sentinelProcesses{t: t, head: sentinelCommit}
	d := &SentinelDriver{Target: target, Home: filepath.Join(base, "home"), State: filepath.Join(base, "state"), Repository: sentinelRepository, Commit: sentinelCommit,
		Git: fakeSentinelGit, Shell: fakeSentinelShell, Run: processes.run, Query: processes.run}
	return d, processes
}

var sentinelOperation = strings.Repeat("a", 64)

func sentinelApply(t *testing.T, d *SentinelDriver) (Observation, error) {
	t.Helper()
	return d.Apply(context.Background(), Resource{ID: sentinelResource, Name: "Sentinel installer"}, Operation{Action: "install"}, Receipt{OperationID: sentinelOperation})
}

func sentinelEnvironment(t *testing.T, command nativeCommand, name string) string {
	t.Helper()
	for _, value := range command.Environment {
		if key, rest, _ := strings.Cut(value, "="); key == name {
			return rest
		}
	}
	t.Fatal("the installer command does not set", name, command.Environment)
	return ""
}

func TestSentinelFetchesOnlyThePinnedCommit(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	o, err := sentinelApply(t, d)
	if err != nil || !o.Present || !o.Healthy || o.CompletedOperation != sentinelOperation {
		t.Fatal("the pinned commit was not installed", o, err)
	}
	fetched := slices.ContainsFunc(processes.commands, func(c nativeCommand) bool {
		return slices.Equal(c.Arguments[len(c.Arguments)-7:], []string{"fetch", "--quiet", "--depth", "1", "--no-tags", sentinelRepository, sentinelCommit})
	})
	if !fetched || len(processes.installers()) != 2 {
		t.Fatal("setup did not fetch the pin and run the installer", processes.commands)
	}

	d, processes = sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	processes.head = strings.Repeat("1", 40)
	if _, err := sentinelApply(t, d); err == nil || !strings.Contains(err.Error(), "not the pinned commit") {
		t.Fatal("setup accepted a different commit", err)
	}
	entries, err := os.ReadDir(filepath.Join(d.State, "sentinel"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "record.json" || len(processes.installers()) != 0 {
		t.Fatal("a refused commit left a checkout or ran its installer", entries, err, processes.commands)
	}
}

func TestSentinelInstallerCommandFollowsPlatformAndHosts(t *testing.T) {
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS, func(t *testing.T) {
			d, processes := sentinelFixture(t, target)
			config := filepath.Join(d.Home, "xdg")
			kiro := filepath.Join(d.Home, "work kiro")
			d.Locations = SentinelLocations{CodexHome: filepath.Join(d.Home, "codex home"), ConfigHome: config, KiroHome: kiro, Installed: []string{"claude"}}
			d.Selected = []string{"tool.kiro"}
			writeConfigFixture(t, filepath.Join(d.Locations.CodexHome, "config.toml"), "")
			writeConfigFixture(t, filepath.Join(config, "opencode", "opencode.json"), "{}")
			if _, err := sentinelApply(t, d); err != nil {
				t.Fatal(err)
			}
			checkout := filepath.Join(d.State, "sentinel", "checkout")
			want := []string{filepath.Join(checkout, "tools", "install"), "--global", "--hosts", "codex,claude,kiro,opencode", "--skills", "core,ui"}
			if target.OS == "windows" {
				want = []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "tools", "install.ps1"),
					"-Global", "-Hosts", "codex,claude,kiro,opencode", "-Skills", "core,ui"}
			}
			install := processes.installers()[0]
			if install.Program != fakeSentinelShell || !slices.Equal(install.Arguments, want) {
				t.Fatal("unexpected installer command", install)
			}
			// The installer gets each folder variable unchanged.
			for name, value := range map[string]string{"HOME": d.Home, "CODEX_HOME": d.Locations.CodexHome, "XDG_CONFIG_HOME": config, "COPILOT_HOME": "", "CLAUDE_CONFIG_DIR": "", "KIRO_HOME": kiro} {
				if got := sentinelEnvironment(t, install, name); got != value {
					t.Fatal("the installer does not get the folder variable unchanged", name, got)
				}
			}
		})
	}
}

// The installer writes to KIRO_HOME as it is. Kiro ignores a relative value,
// a filesystem root and the home folder path, so setup refuses those values.
func TestSentinelRefusesAKiroHomeThatKiroIgnores(t *testing.T) {
	for _, value := range []string{"relative kiro", string(filepath.Separator)} {
		d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
		d.Locations = SentinelLocations{KiroHome: value}
		d.Selected = []string{"tool.kiro"}
		o, err := d.Observe(context.Background(), Resource{ID: sentinelResource}, Receipt{})
		if err != nil || !strings.Contains(o.ApplyBlocked, "Kiro ignores KIRO_HOME") {
			t.Fatal("an ignored KIRO_HOME does not block Sentinel", value, o, err)
		}
		if _, err := sentinelApply(t, d); err == nil || len(processes.commands) != 0 {
			t.Fatal("Sentinel ran with an ignored KIRO_HOME", value, err, processes.commands)
		}
	}
}

func TestSentinelChecksOnlyAVerifiedCheckoutOfThePin(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".claude", "settings.json"), "{}")
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: sentinelResource}
	for _, code := range []int{0, 1, 2} {
		processes.check = code
		o, err := d.Observe(context.Background(), r, Receipt{})
		switch code {
		case 0:
			if err != nil || !o.Present || !o.Healthy || o.CompletedOperation != sentinelOperation {
				t.Fatal("a passing check is not healthy", o, err)
			}
		case 1:
			if err != nil || !o.Present || o.Healthy || !strings.Contains(o.HealthIssue, "DRIFT") || strings.Contains(o.HealthIssue, "ok:") {
				t.Fatal("drift is not reported as a repairable health issue", o, err)
			}
		default:
			if err == nil {
				t.Fatal("an installer error was reported as health", o)
			}
		}
	}
	processes.check = 0
	checks := len(processes.installers())
	checkout := filepath.Join(d.State, "sentinel", "checkout")
	for name, damage := range map[string]func() error{
		"other commit": func() error {
			return os.WriteFile(filepath.Join(checkout, ".git", "commit"), []byte(strings.Repeat("2", 40)), 0600)
		},
		"edited file": func() error {
			return os.WriteFile(filepath.Join(checkout, "tools", "install"), []byte("#!/bin/sh\nexit 0\n"), 0755)
		},
		"extra file": func() error { return os.WriteFile(filepath.Join(checkout, "tools", "extra"), nil, 0600) },
	} {
		if _, err := d.Remove(context.Background(), r, Receipt{OperationID: sentinelOperation}); err != nil {
			t.Fatal(err)
		}
		if _, err := sentinelApply(t, d); err != nil {
			t.Fatal(err)
		}
		checks = len(processes.installers())
		if err := damage(); err != nil {
			t.Fatal(err)
		}
		o, err := d.Observe(context.Background(), r, Receipt{})
		if err != nil || o.Healthy || o.HealthIssue == "" || len(processes.installers()) != checks {
			t.Fatal("Observe ran Sentinel from a checkout that is not exactly the pin", name, o, err)
		}
		fetches := func() int {
			return len(slices.DeleteFunc(slices.Clone(processes.commands), func(c nativeCommand) bool { return !slices.Contains(c.Arguments, "fetch") }))
		}
		before := fetches()
		if _, err := d.Remove(context.Background(), r, Receipt{OperationID: sentinelOperation}); err != nil {
			t.Fatal(err)
		}
		removal := processes.installers()[len(processes.installers())-1]
		if fetches() != before+1 {
			t.Fatal("removal did not fetch the pin again before it ran Sentinel", name)
		}
		if removal.Arguments[len(removal.Arguments)-1] != "--remove" || len(processes.installers()) != checks+1 {
			t.Fatal("removal did not run from a fresh checkout of the pin", name, removal)
		}
		if _, err := os.Lstat(filepath.Join(d.State, "sentinel")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("removal kept the checkout or the record", name, err)
		}
	}
}

// sentinelGitFixture makes a real repository with one commit of files. It
// returns the repository, the commit and a git helper without caller GIT_*
// variables.
func sentinelGitFixture(t *testing.T, files map[string]string) (string, string, func(...string) string) {
	t.Helper()
	program, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	git := func(arguments ...string) string {
		t.Helper()
		command := exec.Command(program, append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.autocrlf=false", "-C", repository}, arguments...)...)
		command.Env = slices.DeleteFunc(os.Environ(), func(value string) bool { return strings.HasPrefix(value, "GIT_") })
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(arguments, string(output), err)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	for name, contents := range files {
		writeConfigFixture(t, filepath.Join(repository, filepath.FromSlash(name)), contents)
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "fixture")
	return repository, git("rev-parse", "HEAD"), git
}

func sentinelVerifier(t *testing.T) *SentinelDriver {
	t.Helper()
	program, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	return &SentinelDriver{Target: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Git: program}
}

// Index flags can hide a change from git status. The verification compares the
// files with the commit tree, so it finds the change. This uses the real git.
func TestSentinelVerificationIgnoresGitIndexFlags(t *testing.T) {
	d := sentinelVerifier(t)
	repository, commit, git := sentinelGitFixture(t, map[string]string{"assumed.md": "# pinned\n", "skipped.md": "# pinned\n"})
	if got, err := d.verify(context.Background(), querySentinel, repository); err != nil || got != commit {
		t.Fatal("a clean checkout does not verify", got, err)
	}
	git("update-index", "--assume-unchanged", "assumed.md")
	git("update-index", "--skip-worktree", "skipped.md")
	for _, name := range []string{"assumed.md", "skipped.md"} {
		writeConfigFixture(t, filepath.Join(repository, name), "# edited\n")
		if status := git("status", "--porcelain"); status != "" {
			t.Fatal("the fixture flag does not hide the edit from git status", status)
		}
		if _, err := d.verify(context.Background(), querySentinel, repository); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatal("an edited file with an index flag verified", name, err)
		}
		writeConfigFixture(t, filepath.Join(repository, name), "# pinned\n")
	}
	writeConfigFixture(t, filepath.Join(repository, "extra.md"), "")
	if _, err := d.verify(context.Background(), querySentinel, repository); err == nil || !strings.Contains(err.Error(), "extra.md") {
		t.Fatal("an extra file verified", err)
	}
}

// The local configuration of a checkout can add a clean filter that gives an
// edited file the pinned hash. It can also replace the pinned commit with
// another object. The verification hashes raw bytes and ignores replacement
// objects, so it finds both changes. This uses the real git.
func TestSentinelVerificationIgnoresLocalFiltersAndReplacements(t *testing.T) {
	d := sentinelVerifier(t)
	pinned := "#!/bin/sh\necho pinned\n"
	repository, commit, git := sentinelGitFixture(t, map[string]string{"tools/install": pinned})
	install := filepath.Join(repository, "tools", "install")

	copy := filepath.Join(t.TempDir(), "pinned")
	writeConfigFixture(t, copy, pinned)
	writeConfigFixture(t, filepath.Join(repository, ".git", "info", "attributes"), "tools/install filter=hide\n")
	git("config", "filter.hide.clean", "cat '"+filepath.ToSlash(copy)+"'")
	writeConfigFixture(t, install, "#!/bin/sh\necho changed\n")
	if git("hash-object", "tools/install") != git("rev-parse", commit+":tools/install") {
		t.Fatal("the fixture filter does not give the edited file the pinned hash")
	}
	if _, err := d.verify(context.Background(), querySentinel, repository); err == nil || !strings.Contains(err.Error(), "tools/install") {
		t.Fatal("a clean filter hid an edited file", err)
	}
	git("config", "--unset", "filter.hide.clean")

	writeConfigFixture(t, install, "#!/bin/sh\necho replaced\n")
	git("commit", "--quiet", "-am", "replacement")
	git("replace", commit, git("rev-parse", "HEAD"))
	git("update-ref", "--no-deref", "HEAD", commit)
	if git("ls-tree", commit, "tools/install") == git("--no-replace-objects", "ls-tree", commit, "tools/install") {
		t.Fatal("the fixture replacement does not change the pinned tree")
	}
	if got, err := d.verify(context.Background(), querySentinel, repository); got != commit || err == nil || !strings.Contains(err.Error(), "tools/install") {
		t.Fatal("a replacement object hid an edited file", got, err)
	}
}

func TestSentinelRemovesWithTheFoldersOfTheInstall(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	installed := filepath.Join(d.Home, "claude at install")
	d.Locations = SentinelLocations{ClaudeConfigDir: installed}
	writeConfigFixture(t, filepath.Join(installed, "settings.json"), "{}")
	codex := filepath.Join(d.Home, ".codex")
	writeConfigFixture(t, filepath.Join(codex, "config.toml"), "")
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	// The user moves Claude Code and removes Codex before the removal.
	d.Locations.ClaudeConfigDir = filepath.Join(d.Home, "claude later")
	if err := os.RemoveAll(codex); err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: sentinelResource}
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil || !slices.ContainsFunc(o.Notes, func(note string) bool { return strings.Contains(note, "CLAUDE_CONFIG_DIR="+installed) }) {
		t.Fatal("Observe does not disclose the recorded folders", o, err)
	}
	check := processes.installers()[len(processes.installers())-1]
	if sentinelEnvironment(t, check, "CLAUDE_CONFIG_DIR") != installed {
		t.Fatal("the check does not use the folders of the install", check)
	}
	if o, err = d.Remove(context.Background(), r, Receipt{OperationID: sentinelOperation}); err != nil || o.Present {
		t.Fatal("removal left the Sentinel cache", o, err)
	}
	remove := processes.installers()[len(processes.installers())-1]
	if !slices.Equal(remove.Arguments[1:], []string{"--global", "--hosts", "codex,claude", "--skills", "core,ui", "--remove"}) || sentinelEnvironment(t, remove, "CLAUDE_CONFIG_DIR") != installed {
		t.Fatal("removal did not reach the hosts and folders of the install", remove)
	}
}

// An interruption after the checkout and before completion leaves the saved
// intent. Resume runs the installer again and proves the completion, so the
// installation becomes owned and removable.
func TestSentinelResumesAnInterruptedInstall(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	processes.install = func(nativeCommand) error {
		if _, err := os.Stat(filepath.Join(d.State, "sentinel", "checkout", "tools", "install")); err != nil {
			t.Error("the interruption is not after the checkout", err)
		}
		return errors.New("the process stopped")
	}
	catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "sentinel", Name: "Sentinel rules and skills", Capability: true, Requires: []string{sentinelResource}},
		{ID: sentinelResource, Name: "Sentinel installer", Action: "sentinel", Scope: "user"}}}
	c := Controller{Catalog: catalog, Context: d.Target, Source: "sentinel-fixture", Home: d.Home, StatePath: filepath.Join(d.State, "state.json"), Driver: d}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if result, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("the interrupted install reported success", result)
	}
	processes.install = nil
	retry := Request{Schema: 1, Mode: "apply", Retry: true}
	preview, err = c.Dispatch(context.Background(), retry)
	if err != nil || preview.Plan.ResourceResume == nil {
		t.Fatal("the interrupted install offers no resume", preview, err)
	}
	retry.ExpectedPlan = preview.Plan.ID
	if result, err := c.Dispatch(context.Background(), retry); err != nil || result.Status == "failed" {
		t.Fatal("resume did not finish the install", result, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
	state, err := LoadState(c.StatePath, d.Home)
	if receipt := state.Receipts[sentinelResource]; err != nil || receipt.Ownership != "created" || receipt.Status != "ready" {
		t.Fatal("the resumed install is not owned", receipt, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if remove := processes.installers()[len(processes.installers())-1]; remove.Arguments[len(remove.Arguments)-1] != "--remove" {
		t.Fatal("the resumed install could not be removed", remove)
	}
}
