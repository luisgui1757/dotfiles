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
			d.Locations = SentinelLocations{CodexHome: filepath.Join(d.Home, "codex home"), ConfigHome: config, KiroHome: "relative kiro", Installed: []string{"claude"}}
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
			// The installer gets each folder variable unchanged, even a value
			// that Kiro itself ignores.
			for name, value := range map[string]string{"HOME": d.Home, "CODEX_HOME": d.Locations.CodexHome, "XDG_CONFIG_HOME": config, "COPILOT_HOME": "", "CLAUDE_CONFIG_DIR": "", "KIRO_HOME": "relative kiro"} {
				if got := sentinelEnvironment(t, install, name); got != value {
					t.Fatal("the installer does not get the folder variable unchanged", name, got)
				}
			}
		})
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

// Index flags can hide a change from git status. The verification compares the
// files with the commit tree, so it finds the change. This uses the real git.
func TestSentinelVerificationIgnoresGitIndexFlags(t *testing.T) {
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
	for _, name := range []string{"assumed.md", "skipped.md"} {
		writeConfigFixture(t, filepath.Join(repository, name), "# pinned\n")
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "fixture")
	commit := git("rev-parse", "HEAD")
	d := &SentinelDriver{Target: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Git: program}
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

// sentinelEarlierCopies copies two skill folders the way dotfiles 1.0 did:
// through the real configuration journal. It returns the skills root.
func sentinelEarlierCopies(t *testing.T, d *SentinelDriver) string {
	t.Helper()
	repository := filepath.Join(filepath.Dir(d.State), "repository")
	root := filepath.Join(d.Home, ".agents", "skills")
	targets := []ConfigTarget{}
	for _, name := range []string{"council", "doc-view"} {
		writeConfigFixture(t, filepath.Join(repository, "skills", name, "SKILL.md"), "# "+name+"\n")
		targets = append(targets, ConfigTarget{Source: "skills/" + name, Folder: "home", Path: name, Directory: true, Mode: "copy"})
	}
	target := Context{OS: runtime.GOOS, Arch: runtime.GOARCH}
	copies, r, err := integrationConfiguration(repository, target, ConfigFolders{Home: root}, Resource{ID: "skills.sentinel-agents", Name: "Earlier skills", Scope: "user"}, targets)
	if err != nil {
		t.Fatal(err)
	}
	copies.Directory = d.State
	catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "earlier", Name: "Earlier skills", Capability: true, Platforms: []string{target.OS}, Requires: []string{r.ID}}, r}}
	c := Controller{Catalog: catalog, Context: target, Source: "earlier-fixture", Home: d.Home, StatePath: filepath.Join(d.State, "state.json"), Driver: copies}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"earlier"}})
	state, err := LoadState(c.StatePath, d.Home)
	if err != nil {
		t.Fatal(err)
	}
	d.Earlier = map[string]Receipt{r.ID: state.Receipts[r.ID]}
	return root
}

func TestSentinelHandoffDeletesOnlyUnchangedDotfilesCopies(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	root := sentinelEarlierCopies(t, d)
	d.Selected = []string{sentinelResource}
	earlier := Resource{ID: "skills.sentinel-agents"}
	if o := d.observeEarlier(earlier, d.Earlier[earlier.ID]); o.Present || o.Pending == "" {
		t.Fatal("the earlier copies were released before Sentinel took them over", o)
	}
	processes.install = func(nativeCommand) error {
		for _, name := range []string{"council", "doc-view"} {
			if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
				t.Error("an earlier copy still exists when the Sentinel installer runs", name, err)
			}
		}
		writeConfigFixture(t, filepath.Join(root, "council", ".sentinel-owned"), "")
		return nil
	}
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	if o := d.observeEarlier(earlier, d.Earlier[earlier.ID]); o.Present || o.Pending != "" {
		t.Fatal("handed-over copies were not released", o)
	}
}

func TestSentinelHandoffLeavesAChangedCopyAndReportsIt(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	root := sentinelEarlierCopies(t, d)
	changed := filepath.Join(root, "council")
	writeConfigFixture(t, filepath.Join(changed, "SKILL.md"), "# my council\n")
	d.Selected = []string{sentinelResource}
	o, err := d.Observe(context.Background(), Resource{ID: sentinelResource}, Receipt{})
	if err != nil || !strings.Contains(o.ApplyBlocked, changed) {
		t.Fatal("the changed copy does not block the Sentinel install", o, err)
	}
	if _, err := sentinelApply(t, d); err == nil || !strings.Contains(err.Error(), changed) {
		t.Fatal("apply did not report the changed copy", err)
	}
	for path, contents := range map[string]string{filepath.Join(changed, "SKILL.md"): "# my council\n", filepath.Join(root, "doc-view", "SKILL.md"): "# doc-view\n"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != contents {
			t.Fatal("the handoff deleted or changed a copy although one copy changed", path, err)
		}
	}
	if len(processes.commands) != 0 {
		t.Fatal("git or the installer ran before the changed copy was moved away", processes.commands)
	}
	if o := d.observeEarlier(Resource{ID: "skills.sentinel-agents"}, d.Earlier["skills.sentinel-agents"]); !strings.Contains(o.Pending, changed) {
		t.Fatal("the earlier receipt does not report the changed copy", o)
	}
}

// A link in a parent can lead a recorded path to identical files that dotfiles
// did not write at that place. The handoff leaves them and reports the path.
func TestSentinelHandoffRefusesACopyBehindALink(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	root := sentinelEarlierCopies(t, d)
	elsewhere := filepath.Join(d.Home, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0700); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Dir(root)
	if err := os.Rename(agents, filepath.Join(elsewhere, ".agents")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(elsewhere, ".agents"), agents); err != nil {
		t.Skip("this system cannot create the link fixture", err)
	}
	if _, err := sentinelApply(t, d); err == nil || !strings.Contains(err.Error(), "a link now leads to it") {
		t.Fatal("the handoff accepted a copy behind a link", err)
	}
	for _, name := range []string{"council", "doc-view"} {
		if _, err := os.Stat(filepath.Join(elsewhere, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Fatal("the handoff deleted a copy behind a link", name, err)
		}
	}
	if len(processes.commands) != 0 {
		t.Fatal("git or the installer ran", processes.commands)
	}
}

// dotfiles 1.0 saved the original rules block when it adopted an existing
// policy. The handoff drops that baseline. The Sentinel installer owns the
// block, and its removal keeps the text outside the block.
func TestSentinelHandoffDropsTheAdoptedRulesBaseline(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	recovery := filepath.Join("recovery", strings.Repeat("c", 64), sentinelEarlierRules)
	writeConfigFixture(t, filepath.Join(d.State, recovery, "profile.json"), "{}\n")
	receipt := Receipt{Ownership: "created", Status: "ready", Adopted: true, Recovery: recovery, Before: Observation{Present: true}}
	d.Earlier = map[string]Receipt{sentinelEarlierRules: receipt}
	d.Selected = []string{sentinelResource}
	rules := Resource{ID: sentinelEarlierRules}
	if o := d.observeEarlier(rules, receipt); o.Pending == "" {
		t.Fatal("the adopted rules were released before the Sentinel installer ran", o)
	}
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(d.State, recovery)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the handoff kept the adopted rules baseline", err)
	}
	if o := d.observeEarlier(rules, receipt); o.Present || o.Pending != "" {
		t.Fatal("the adopted rules were not released after the handoff", o)
	}
	if _, err := d.Remove(context.Background(), Resource{ID: sentinelResource}, Receipt{OperationID: sentinelOperation}); err != nil {
		t.Fatal(err)
	}
	if remove := processes.installers()[len(processes.installers())-1]; remove.Arguments[len(remove.Arguments)-1] != "--remove" {
		t.Fatal("removal does not use the Sentinel removal", remove)
	}
}

func TestSentinelReleasesEarlierRulesOnlyAfterItsInstallerRan(t *testing.T) {
	d, _ := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	rules, receipt := Resource{ID: sentinelEarlierRules}, Receipt{Ownership: "created"}
	if o := d.observeEarlier(rules, receipt); !strings.Contains(o.Pending, "Select Sentinel") {
		t.Fatal("unselected earlier rules were released without a handoff", o)
	}
	d.Selected = []string{sentinelResource}
	if o := d.observeEarlier(rules, receipt); o.Pending == "" {
		t.Fatal("earlier rules were released before the Sentinel installer ran", o)
	}
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	if o := d.observeEarlier(rules, receipt); o.Present || o.Pending != "" {
		t.Fatal("earlier rules were not released after the Sentinel installer ran", o)
	}
}

func TestLoadStateMapsTheRetiredSentinelSkillsCapability(t *testing.T) {
	for _, selected := range [][]string{{"sentinel-skills", "zsh"}, {"sentinel", "sentinel-skills", "zsh"}} {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := SaveState(path, State{Schema: 1, Context: Context{OS: "linux", Arch: "amd64"}, Target: "/home/user", Selected: selected, Receipts: map[string]Receipt{}}); err != nil {
			t.Fatal(err)
		}
		state, err := LoadState(path, "/home/user")
		if err != nil || !slices.Equal(state.Selected, []string{"sentinel", "zsh"}) {
			t.Fatal("a saved Sentinel skills selection did not map to Sentinel", selected, state.Selected, err)
		}
	}
}

func TestSentinelEarlierRulesJournalStaysHistory(t *testing.T) {
	directory := t.TempDir()
	operation := strings.Repeat("b", 64)
	preserved := filepath.Join(directory, "personal", "AGENTS.md")
	journal := profileJournal{Schema: 1, Resource: sentinelEarlierRules, Operation: operation, Recovery: "recovery/" + operation + "/" + sentinelEarlierRules,
		Action: "install", Complete: true, Sealed: true, Preserved: []string{preserved},
		Entries: []profilePublication{{Path: filepath.Join(directory, ".codex", "AGENTS.md"), Block: []byte("<!-- AGENT-RULES:BEGIN do-not-edit-inside-this-block -->\nOld rules\n<!-- AGENT-RULES:END -->\n")}}}
	if err := saveDocument(filepath.Join(directory, "profiles", "operations", operation+".json"), journal); err != nil {
		t.Fatal(err)
	}
	report, err := (&ProfileDriver{Directory: directory}).FinishTransaction(context.Background(), Plan{}, map[string]Receipt{})
	if err != nil || len(report[""]) != 0 || !slices.Equal(report[sentinelEarlierRules], []string{preserved}) {
		t.Fatal("a finished dotfiles 1.0 rules journal is reported as damaged", report, err)
	}
}
