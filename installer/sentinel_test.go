package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// sentinelProcesses fakes git and the Sentinel installer at the process
// boundary. Fake git creates a checkout and reports head as its commit.
type sentinelProcesses struct {
	t        *testing.T
	head     string
	check    int
	commands []nativeCommand
	// install runs when the installer writes, before it reports success.
	install func(nativeCommand)
}

const fakeSentinelGit, fakeSentinelShell = "/fake/git", "/fake/shell"

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
			return nil, os.MkdirAll(filepath.Join(directory, "tools"), 0700)
		case "rev-parse":
			return []byte(f.head + "\n"), nil
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
			f.install(command)
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
	d := &SentinelDriver{Target: target, Home: filepath.Join(base, "home"), State: filepath.Join(base, "state"), Git: fakeSentinelGit, Shell: fakeSentinelShell,
		Run: processes.run, Query: processes.run}
	return d, processes
}

func sentinelApply(t *testing.T, d *SentinelDriver) (Observation, error) {
	t.Helper()
	return d.Apply(context.Background(), Resource{ID: sentinelResource, Name: "Sentinel installer"}, Operation{Action: "install"}, Receipt{OperationID: strings.Repeat("a", 64)})
}

func TestSentinelFetchesOnlyThePinnedCommit(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	o, err := sentinelApply(t, d)
	if err != nil || !o.Present || !o.Healthy || o.Version != sentinelCommit || o.CompletedOperation != strings.Repeat("a", 64) {
		t.Fatal("the pinned commit was not installed", o, err)
	}
	fetched := slices.ContainsFunc(processes.commands, func(c nativeCommand) bool {
		return slices.Equal(c.Arguments[len(c.Arguments)-7:], []string{"fetch", "--quiet", "--depth", "1", "--no-tags", sentinelRepository, sentinelCommit})
	})
	if !fetched || len(processes.installers()) != 2 {
		t.Fatal("setup did not fetch the pin and run the installer", processes.commands)
	}
	if _, err := os.Stat(filepath.Join(d.State, "sentinel", "checkout", ".git")); err != nil {
		t.Fatal("the verified checkout is not in the cache", err)
	}

	d, processes = sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	writeConfigFixture(t, filepath.Join(d.Home, ".codex", "config.toml"), "")
	processes.head = strings.Repeat("1", 40)
	if _, err := sentinelApply(t, d); err == nil || !strings.Contains(err.Error(), "not the pinned commit") {
		t.Fatal("setup accepted a different commit", err)
	}
	entries, err := os.ReadDir(filepath.Join(d.State, "sentinel"))
	if err != nil || len(entries) != 0 || len(processes.installers()) != 0 {
		t.Fatal("a refused commit left a checkout or ran its installer", entries, err, processes.commands)
	}
}

func TestSentinelInstallerCommandFollowsPlatformAndHosts(t *testing.T) {
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS, func(t *testing.T) {
			d, processes := sentinelFixture(t, target)
			config := filepath.Join(d.Home, "xdg")
			d.Locations = SentinelLocations{CodexHome: filepath.Join(d.Home, "codex home"), ConfigHome: config, Installed: []string{"claude"}}
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
			for _, value := range []string{"HOME=" + d.Home, "CODEX_HOME=" + d.Locations.CodexHome, "XDG_CONFIG_HOME=" + config, "COPILOT_HOME="} {
				if !slices.Contains(install.Environment, value) {
					t.Fatal("the installer does not use the folders that setup checked", install.Environment)
				}
			}
		})
	}
}

func TestSentinelCheckExitCodeIsHealth(t *testing.T) {
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
			if err != nil || !o.Present || !o.Healthy {
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
	last := processes.installers()[len(processes.installers())-1]
	if last.Arguments[len(last.Arguments)-1] != "--check" {
		t.Fatal("Observe did not run the installer check", last)
	}
}

func TestSentinelRemoveRunsInstallerRemovalForEveryInstalledHost(t *testing.T) {
	d, processes := sentinelFixture(t, Context{OS: "linux", Arch: "amd64"})
	codex := filepath.Join(d.Home, ".codex")
	writeConfigFixture(t, filepath.Join(codex, "config.toml"), "")
	writeConfigFixture(t, filepath.Join(d.Home, ".claude", "settings.json"), "{}")
	if _, err := sentinelApply(t, d); err != nil {
		t.Fatal(err)
	}
	// An agent that disappears after the install still gets its files removed.
	if err := os.RemoveAll(codex); err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: sentinelResource}
	o, err := d.Remove(context.Background(), r, Receipt{})
	if err != nil || o.Present {
		t.Fatal("removal left the Sentinel cache", o, err)
	}
	remove := processes.installers()[len(processes.installers())-1]
	if !slices.Equal(remove.Arguments[1:], []string{"--global", "--hosts", "codex,claude", "--skills", "core,ui", "--remove"}) {
		t.Fatal("removal did not run the installer removal for the installed hosts", remove)
	}
	if _, err := os.Lstat(filepath.Join(d.State, "sentinel")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("removal kept the checkout or the host record", err)
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
	processes.install = func(nativeCommand) {
		for _, name := range []string{"council", "doc-view"} {
			if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
				t.Error("an earlier copy still exists when the Sentinel installer runs", name, err)
			}
		}
		writeConfigFixture(t, filepath.Join(root, "council", ".sentinel-owned"), "")
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
	if len(processes.installers()) != 0 {
		t.Fatal("the installer ran before the changed copy was moved away", processes.commands)
	}
	if o := d.observeEarlier(Resource{ID: "skills.sentinel-agents"}, d.Earlier["skills.sentinel-agents"]); !strings.Contains(o.Pending, changed) {
		t.Fatal("the earlier receipt does not report the changed copy", o)
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
