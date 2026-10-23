package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// legacyCatalogFixture adds the real setup catalog so replacement
// disclosures read an actual (empty) normal selection.
func legacyCatalogFixture(t *testing.T) (*legacyRetirementDriver, string) {
	t.Helper()
	d, home := legacyStepFixture(t)
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	d.Normal.Catalog, d.Normal.Context = catalog, Context{OS: "linux", Arch: "amd64"}
	d.Platform.Context = d.Normal.Context
	return d, home
}

func legacyDotfilesClone(t *testing.T, directory, origin string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	legacyGitFixture(t, directory, "init", "-q")
	legacyGitFixture(t, directory, "remote", "add", "origin", origin)
	writeConfigFixture(t, filepath.Join(directory, ".chezmoiroot"), "home\n")
	writeConfigFixture(t, filepath.Join(directory, "home", "dot_zshrc"), "# released source\n")
	legacyGitFixture(t, directory, "add", ".")
	legacyGitFixture(t, directory, "commit", "-q", "-m", "fixture")
}

func runLegacySteps(t *testing.T, d *legacyRetirementDriver, steps []legacyRetirementStep) *legacyRetirementIntent {
	t.Helper()
	intent := legacyTestIntent(t, d, steps...)
	for i := range steps {
		if err := d.executeStep(context.Background(), intent, i, false); err != nil {
			t.Fatal(steps[i].Label, err)
		}
	}
	return intent
}

// Git itself is the oracle for the data parser: each configuration is read by
// `git config --local --get`, and the parser must agree or refuse.
func TestLegacyGitConfigParserAgreesWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("the contributor gate requires Git:", err)
	}
	for name, config := range map[string]string{
		"plain":        "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = https://github.com/luisgui1757/dotfiles.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n",
		"cased":        "[REMOTE \"origin\"]\n  URL = \"https://github.com/luisgui1757/dotfiles\" ; comment\n",
		"continuation": "[remote \"origin\"] url = https://github.com/\\\nluisgui1757/dotfiles\n",
		"quoted":       "[remote \"origin\"]\n\turl = \"a # b\" # comment\n",
		"escaped":      "[remote \"origin\"]\n\turl = a\\tb\\\"c\\\\d  \n",
		"deprecated":   "[remote.origin]\n\turl = deprecated-subsection\n",
		"subsection":   "[remote \"Origin\"]\n\turl = only-for-Origin\n",
		"crlf":         "[remote \"origin\"]\r\n\turl = crlf\r\n",
		"spacing":      "[remote \"origin\"]\n\turl =   internal   spacing   \n",
	} {
		t.Run(name, func(t *testing.T) {
			top := t.TempDir()
			legacyGitFixture(t, top, "init", "-q")
			writeConfigFixture(t, filepath.Join(top, ".git", "config"), config)
			command := exec.Command("git", "-C", top, "config", "--local", "--get", "remote.origin.url")
			command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
			output, gitErr := command.Output()
			value, err := legacyGitOrigin(top)
			switch {
			case gitErr == nil && (err != nil || value != strings.TrimSuffix(string(output), "\n")):
				t.Fatalf("parser read %q (%v); Git read %q", value, err, output)
			case gitErr != nil && err == nil:
				t.Fatalf("parser accepted %q where Git found no value: %v", value, gitErr)
			}
		})
	}
	for name, config := range map[string]string{
		"bad escape":   "[remote \"origin\"]\n\turl = a\\qb\n",
		"unterminated": "[remote \"origin\"]\n\turl = \"open\n",
		"repeated":     "[remote \"origin\"]\n\turl = a\n\turl = b\n",
		"no section":   "url = a\n",
	} {
		top := t.TempDir()
		legacyGitFixture(t, top, "init", "-q")
		writeConfigFixture(t, filepath.Join(top, ".git", "config"), config)
		if value, err := legacyGitOrigin(top); err == nil {
			t.Errorf("%s: ambiguous or invalid configuration accepted as %q", name, value)
		}
	}
}

// The chezmoi source is recognized from Git metadata alone: no Git executable
// is configured, so retiring an earlier Git cannot change the result.
func TestLegacyChezmoiSourceRecognitionNeedsNoGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture; the native Windows lane covers Windows paths")
	}
	d, home := legacyCatalogFixture(t)
	d.Repository = filepath.Join(home, "active checkout")
	d.Folders.Config, d.Folders.Data = filepath.Join(home, ".config"), filepath.Join(home, ".local", "share")
	d.Environment.Cache = filepath.Join(home, ".cache")
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(home, "no-system-config"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(home, "no-system-data"))
	source := filepath.Join(home, ".local", "share", "chezmoi")
	legacyDotfilesClone(t, source, "git@github.com:luisgui1757/dotfiles.git")
	config := d.chezmoiConfigDirectory()
	writeConfigFixture(t, filepath.Join(config, "chezmoistate.boltdb"), "state")
	delete(d.Environment.Programs, "git")

	scope := legacyRetirementScope{}
	if err := d.inspectChezmoi(context.Background(), &scope); err != nil || scope.Blocked != "" {
		t.Fatal("released default source was not recognized without Git", scope, err)
	}
	if !slices.ContainsFunc(scope.Detail, func(line string) bool { return strings.Contains(line, "the dotfiles checkout "+source) }) {
		t.Fatal("the source checkout was not disclosed as preserved", scope.Detail)
	}
	for _, step := range scope.Steps {
		if step.Path == source || strings.HasPrefix(source, step.Path+string(filepath.Separator)) {
			t.Fatal("retirement would move the source checkout", step)
		}
	}
	if !slices.ContainsFunc(scope.Steps, func(step legacyRetirementStep) bool { return step.Kind == "move" && step.Path == config }) {
		t.Fatal("chezmoi configuration and state were not moved aside", scope.Steps)
	}

	// A linked worktree's gitfile and commondir lead to the same origin.
	worktree := filepath.Join(home, "worktree ü")
	legacyGitFixture(t, source, "worktree", "add", "-q", "--detach", worktree)
	writeConfigFixture(t, filepath.Join(config, "chezmoi.toml"), "sourceDir = '"+filepath.Join(worktree, "home")+"'\n[data]\nsourceDir = 'ignored'\n")
	scope = legacyRetirementScope{}
	if err := d.inspectChezmoiSource(context.Background(), &scope, config); err != nil || scope.Blocked != "" {
		t.Fatal("worktree home source was not recognized", scope, err)
	}

	// An unrelated chezmoi source remains a hard block.
	other := filepath.Join(home, "personal source")
	legacyDotfilesClone(t, other, "https://github.com/example/dotfiles")
	writeConfigFixture(t, filepath.Join(config, "chezmoi.toml"), "sourceDir = \""+other+"\"\n")
	scope = legacyRetirementScope{}
	if err := d.inspectChezmoi(context.Background(), &scope); err != nil || scope.Blocked == "" || len(scope.Steps) != 0 {
		t.Fatal("an independent chezmoi source did not block retirement", scope, err)
	}

	// The active checkout's home directory is this checkout.
	writeConfigFixture(t, filepath.Join(d.Repository, "home", "dot_zshrc"), "active\n")
	writeConfigFixture(t, filepath.Join(config, "chezmoi.toml"), "sourceDir = \""+filepath.Join(d.Repository, "home")+"\"\n")
	scope = legacyRetirementScope{}
	if err := d.inspectChezmoiSource(context.Background(), &scope, config); err != nil || scope.Blocked != "" || !strings.Contains(strings.Join(scope.Detail, "\n"), "this dotfiles checkout") {
		t.Fatal("the active checkout was not recognized", scope, err)
	}

	// A source inside the configuration that retirement would move blocks.
	inside := filepath.Join(config, "source")
	legacyDotfilesClone(t, inside, "https://github.com/luisgui1757/dotfiles")
	writeConfigFixture(t, filepath.Join(config, "chezmoi.toml"), "sourceDir = \""+inside+"\"\n")
	scope = legacyRetirementScope{}
	if err := d.inspectChezmoiSource(context.Background(), &scope, config); err != nil || !strings.Contains(scope.Blocked, "lies inside") {
		t.Fatal("a source inside the moved configuration was not refused", scope, err)
	}
}

func writeLegacyGhDash(t *testing.T, data, tag string) string {
	t.Helper()
	directory := filepath.Join(data, "gh", "extensions", "gh-dash")
	writeConfigFixture(t, filepath.Join(directory, "gh-dash"), "released gh-dash binary")
	if err := os.Chmod(filepath.Join(directory, "gh-dash"), 0755); err != nil {
		t.Fatal(err)
	}
	// The bytes gh's yaml.v3 Marshal writes for binManifest.
	writeConfigFixture(t, filepath.Join(directory, "manifest.yml"), "owner: dlvhdr\nname: gh-dash\nhost: github.com\ntag: "+tag+"\nispinned: true\npath: "+filepath.Join(directory, "gh-dash")+"\n")
	return directory
}

// gh-dash retires without GitHub CLI: the exact released extension directory
// moves aside intact, and other extensions stay listed.
func TestLegacyGhDashRetiresReleasedExtensionWithoutGitHubCLI(t *testing.T) {
	d, home := legacyCatalogFixture(t)
	data := filepath.Join(home, "xdg data")
	t.Setenv("XDG_DATA_HOME", data)
	delete(d.Environment.Programs, "gh")
	directory := writeLegacyGhDash(t, data, "v4.25.2")
	writeConfigFixture(t, filepath.Join(data, "gh", "extensions", "gh-other", "gh-other"), "personal extension")

	scope := legacyRetirementScope{}
	if err := d.inspectGhDash(context.Background(), &scope); err != nil || len(scope.Steps) != 1 || len(scope.Retained) != 0 {
		t.Fatal("the released extension was not planned", scope, err)
	}
	step := scope.Steps[0]
	if step.Kind != "move" || step.Path != directory || !slices.ContainsFunc(step.Gates, func(gate legacyRetirementGate) bool { return gate.Kind == "tree" }) {
		t.Fatal("the extension move is not bound to its recorded bytes", step)
	}
	runLegacySteps(t, d, scope.Steps)
	if !legacyAbsent(directory) {
		t.Fatal("gh still lists the released extension")
	}
	if moved, err := os.ReadFile(filepath.Join(step.Backup, "gh-dash")); err != nil || string(moved) != "released gh-dash binary" {
		t.Fatal("the moved extension was not preserved intact", err)
	}
	if strings.HasPrefix(filepath.Base(step.Backup), "gh-") {
		t.Fatal("the preserved copy would still be listed as an extension", step.Backup)
	}
	if _, err := os.Stat(filepath.Join(data, "gh", "extensions", "gh-other", "gh-other")); err != nil {
		t.Fatal("another extension was disturbed", err)
	}
	scope = legacyRetirementScope{}
	if err := d.inspectGhDash(context.Background(), &scope); err != nil || len(scope.Steps)+len(scope.Retained) != 0 {
		t.Fatal("retired extension still reported", scope, err)
	}
}

func TestLegacyGhDashRefusesUnreleasedShapes(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, directory string){
		"tag": func(t *testing.T, directory string) {
			writeConfigFixture(t, filepath.Join(directory, "manifest.yml"), "owner: dlvhdr\nname: gh-dash\nhost: github.com\ntag: v4.27.0\nispinned: true\npath: x/gh-dash\n")
		},
		"owner": func(t *testing.T, directory string) {
			writeConfigFixture(t, filepath.Join(directory, "manifest.yml"), "owner: someone\nname: gh-dash\nhost: github.com\ntag: v4.26.0\nispinned: true\npath: x/gh-dash\n")
		},
		"unpinned": func(t *testing.T, directory string) {
			writeConfigFixture(t, filepath.Join(directory, "manifest.yml"), "owner: dlvhdr\nname: gh-dash\nhost: github.com\ntag: v4.26.0\nispinned: false\npath: x/gh-dash\n")
		},
		"extra file": func(t *testing.T, directory string) {
			writeConfigFixture(t, filepath.Join(directory, "notes.txt"), "personal")
		},
		"duplicate key": func(t *testing.T, directory string) {
			writeConfigFixture(t, filepath.Join(directory, "manifest.yml"), "owner: dlvhdr\nowner: dlvhdr\nname: gh-dash\nhost: github.com\ntag: v4.26.0\nispinned: true\npath: x/gh-dash\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, home := legacyCatalogFixture(t)
			data := filepath.Join(home, "data")
			t.Setenv("XDG_DATA_HOME", data)
			directory := writeLegacyGhDash(t, data, "v4.26.0")
			prepare(t, directory)
			scope := legacyRetirementScope{}
			if err := d.inspectGhDash(context.Background(), &scope); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
				t.Fatal("an unreleased extension shape was offered or silently omitted", scope, err)
			}
		})
	}
	d, home := legacyCatalogFixture(t)
	t.Setenv("XDG_DATA_HOME", "relative/data")
	writeLegacyGhDash(t, filepath.Join(home, ".local", "share"), "v4.26.0")
	scope := legacyRetirementScope{}
	if err := d.inspectGhDash(context.Background(), &scope); err != nil || len(scope.Retained) != 1 {
		t.Fatal("an unlocatable extension directory was treated as absence", scope, err)
	}
}

// A retiring runtime's Git cannot be removed while checkouts only it can
// prove remain; once they are retired, the runtime is free to go.
func TestLegacyQueryGitProtectsItsDependents(t *testing.T) {
	d, home := legacyStepFixture(t)
	keg := filepath.Join(home, "Cellar", "git")
	writeConfigFixture(t, filepath.Join(keg, "2.51.0", "bin", "git"), "#!/bin/sh\n")
	link := filepath.Join(home, "brew", "bin", "git")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(keg, "2.51.0", "bin", "git"), link); err != nil {
		t.Fatal(err)
	}
	d.Environment.Programs["git"] = link
	checkout := d.legacyCheckouts()[0].Path
	writeConfigFixture(t, filepath.Join(checkout, "tpm"), "released")
	if dependents, err := d.legacyGitRuntimeDependents(keg); err != nil || !slices.Equal(dependents, []string{checkout}) {
		t.Fatal("the checkout depending on the retiring Git was not reported", dependents, err)
	}
	if value, err := d.queryGitGate(filepath.Join(home, "Cellar", "other")); err != nil || value != "" {
		t.Fatal("an unrelated runtime was gated", value, err)
	}
	if err := os.RemoveAll(checkout); err != nil {
		t.Fatal(err)
	}
	if value, err := d.queryGitGate(keg); err != nil || value != "" {
		t.Fatal("a retired dependent still blocks", value, err)
	}
}

func TestLegacyGitDiscoveryPrefersSurvivingSystemGit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux system Git path")
	}
	if info, err := os.Stat("/usr/bin/git"); err != nil || !info.Mode().IsRegular() {
		t.Skip("this host has no /usr/bin/git")
	}
	shadow := t.TempDir()
	writeConfigFixture(t, filepath.Join(shadow, "git"), "#!/bin/sh\nexit 99\n")
	if err := os.Chmod(filepath.Join(shadow, "git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shadow+string(os.PathListSeparator)+os.Getenv("PATH"))
	git, err := discoverLegacyGit(Controller{}, NativePlatform{Context: Context{OS: "linux", Arch: "amd64"}})
	if err != nil || git != "/usr/bin/git" {
		t.Fatal("a PATH Git from a retirable runtime was preferred over the system Git", git, err)
	}
}

func TestLegacyDevilspie2RuleRetiresOnlyReleasedArtifacts(t *testing.T) {
	d, home := legacyStepFixture(t)
	d.Folders.Config = filepath.Join(home, ".config")
	checkout := filepath.Join(home, "old checkout")
	legacyDotfilesClone(t, checkout, "https://github.com/luisgui1757/dotfiles.git")
	rule := filepath.Join(checkout, "linux", "devilspie2", "ghostty-maximize.lua")
	writeConfigFixture(t, rule, "-- released rule\n")
	rules := filepath.Join(d.Folders.Config, "devilspie2")
	if err := os.MkdirAll(rules, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rule, filepath.Join(rules, "ghostty-maximize.lua")); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(rules, "personal.lua"), "-- personal rule\n")
	autostart := filepath.Join(d.Folders.Config, "autostart", "devilspie2.desktop")
	writeConfigFixture(t, autostart, legacyDevilspie2Autostart)

	scope := legacyRetirementScope{}
	if err := d.planDevilspie2Rule(&scope); err != nil || len(scope.Retained) != 0 || len(scope.Steps) != 3 {
		t.Fatal("the released rule artifacts were not planned", scope, err)
	}
	intent := runLegacySteps(t, d, scope.Steps)
	if !legacyAbsent(filepath.Join(rules, "ghostty-maximize.lua")) || !legacyAbsent(autostart) {
		t.Fatal("released rule artifacts remain")
	}
	if data, err := os.ReadFile(filepath.Join(rules, "personal.lua")); err != nil || string(data) != "-- personal rule\n" {
		t.Fatal("a personal rule was disturbed", err)
	}
	if !slices.Contains(intent.Preserved, rules) {
		t.Fatal("the rules directory kept for a personal rule was not disclosed", intent.Preserved)
	}
	if data, err := os.ReadFile(rule); err != nil || string(data) != "-- released rule\n" {
		t.Fatal("the checkout's rule source was touched", err)
	}

	writeConfigFixture(t, autostart, legacyDevilspie2Autostart+"Hidden=true\n")
	if err := os.Symlink(filepath.Join(home, "elsewhere", "linux", "devilspie2", "ghostty-maximize.lua"), filepath.Join(rules, "ghostty-maximize.lua")); err != nil {
		t.Fatal(err)
	}
	scope = legacyRetirementScope{}
	if err := d.planDevilspie2Rule(&scope); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 2 {
		t.Fatal("changed or foreign rule artifacts were offered", scope, err)
	}
}

func TestLegacyMatchingTreeRequiresAnExactRelease(t *testing.T) {
	releases := []legacyReleaseTree{
		{Version: "2", Files: map[string]string{"a/b.dll": "h1", "c.exe": "h2"}},
		{Version: "1", Files: map[string]string{"c.exe": "h3"}},
	}
	tree := []configEntry{{Path: ".", Kind: "directory"}, {Path: "a", Kind: "directory"}, {Path: "a/b.dll", Kind: "file", Content: "h1"}, {Path: "c.exe", Kind: "file", Content: "h2"}}
	if release, _ := legacyMatchingTree(tree, releases); release == nil || release.Version != "2" {
		t.Fatal("the exact release was not matched", release)
	}
	if release, _ := legacyMatchingTree([]configEntry{{Path: ".", Kind: "directory"}, {Path: "c.exe", Kind: "file", Content: "h3"}}, releases); release == nil || release.Version != "1" {
		t.Fatal("the earlier release was not matched", release)
	}
	for name, entries := range map[string][]configEntry{
		"mixed":      {{Path: ".", Kind: "directory"}, {Path: "a", Kind: "directory"}, {Path: "a/b.dll", Kind: "file", Content: "h1"}, {Path: "c.exe", Kind: "file", Content: "h3"}},
		"incomplete": {{Path: ".", Kind: "directory"}, {Path: "c.exe", Kind: "file", Content: "h2"}},
		"personal":   append(slices.Clone(tree), configEntry{Path: "settings.json", Kind: "file", Content: "x"}),
		"link":       {{Path: ".", Kind: "directory"}, {Path: "c.exe", Kind: "link", Content: "h3"}},
	} {
		if release, difference := legacyMatchingTree(entries, releases); release != nil || difference == "" {
			t.Errorf("%s tree matched %v", name, release)
		}
	}
}

func TestLegacyPiWindowsShimsAndPackages(t *testing.T) {
	d, home := legacyStepFixture(t)
	d.Platform.OS = "windows"
	d.Folders.AppData = filepath.Join(home, "AppData", "Roaming")
	t.Setenv("SCOOP", filepath.Join(home, "scoop"))
	prefix := filepath.Join(d.Folders.AppData, "npm")
	namespace := filepath.Join(prefix, "node_modules", "@earendil-works")
	for _, name := range []string{"pi-coding-agent", "pi-ai"} {
		bin := ""
		if name == "pi-coding-agent" {
			bin = `,"bin":{"pi":"dist/cli.js"}`
		}
		writeConfigFixture(t, filepath.Join(namespace, name, "package.json"), `{"name":"@earendil-works/`+name+`","version":"1.0.4"`+bin+`}`)
	}
	// The three files npm's cmd-shim writes for the pi command.
	writeConfigFixture(t, filepath.Join(prefix, "pi.cmd"), "@ECHO off\r\nendLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & \"%_prog%\"  \"%dp0%\\node_modules\\@earendil-works\\pi-coding-agent\\dist\\cli.js\" %*\r\n")
	writeConfigFixture(t, filepath.Join(prefix, "pi.ps1"), "$basedir=Split-Path $MyInvocation.MyCommand.Definition -Parent\n& \"$basedir/node$exe\"  \"$basedir/node_modules/@earendil-works/pi-coding-agent/dist/cli.js\" $args\n& \"node$exe\"  \"$basedir/node_modules/@earendil-works/pi-coding-agent/dist/cli.js\" $args\n")
	writeConfigFixture(t, filepath.Join(prefix, "pi"), "#!/bin/sh\nexec node  \"$basedir/node_modules/@earendil-works/pi-coding-agent/dist/cli.js\" \"$@\"\n")
	writeConfigFixture(t, filepath.Join(prefix, "personal.cmd"), "\"%dp0%\\node_modules\\personal\\cli.js\" %*\r\n")
	writeConfigFixture(t, filepath.Join(home, ".pi", "agent", "settings.json"), "{}")

	scope := legacyRetirementScope{}
	if err := d.inspectPi(&scope); err != nil || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	discarded, moved := []string{}, []string{}
	for _, step := range scope.Steps {
		switch step.Kind {
		case "discard":
			discarded = append(discarded, filepath.Base(step.Path))
		case "move":
			moved = append(moved, filepath.Base(step.Path))
		}
	}
	slices.Sort(discarded)
	slices.Sort(moved)
	if !slices.Equal(discarded, []string{"pi", "pi.cmd", "pi.ps1"}) || !slices.Equal(moved, []string{"pi-ai", "pi-coding-agent"}) {
		t.Fatal("Windows Pi shims or packages were not planned exactly", discarded, moved)
	}
	if !strings.Contains(strings.Join(scope.Detail, "\n"), prefix+" stays on the User PATH") {
		t.Fatal("the npm prefix's PATH entry was not disclosed as kept", scope.Detail)
	}
	for _, step := range scope.Steps {
		if strings.Contains(step.Path, "personal") || strings.HasPrefix(step.Path, filepath.Join(home, ".pi")) {
			t.Fatal("personal npm or Pi data was planned", step)
		}
	}

	writeConfigFixture(t, filepath.Join(prefix, "pi.cmd"), "\"%dp0%\\node_modules\\@earendil-works\\pi-coding-agent\\dist\\cli.js\" & \"%dp0%\\node_modules\\other\\x.js\"\r\n")
	scope = legacyRetirementScope{}
	if err := d.inspectPi(&scope); err != nil || len(scope.Retained) != 1 || slices.ContainsFunc(scope.Steps, func(step legacyRetirementStep) bool { return step.Kind == "discard" }) {
		t.Fatal("a shim that also runs another package was removed", scope, err)
	}
}

func TestLegacyLinuxDebPackageRequiresPinnedBytes(t *testing.T) {
	d, _ := legacyStepFixture(t)
	d.Platform.OS = "linux"
	responses := map[string]string{}
	exits := map[string]int{}
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		key := command.Arguments[len(command.Arguments)-1]
		if strings.HasPrefix(key, "--showformat=${binary:Package}\t${db:Status-Abbrev}") {
			key = "consumers"
		}
		if code, ok := exits[key]; ok {
			return nil, &nativeCommandError{Message: "dpkg-query", ExitCode: &code}
		}
		return []byte(responses[key]), nil
	}
	exits["wezterm"] = 1
	if identity, pin, err := d.debIdentity(context.Background(), "wezterm"); err != nil || identity != "absent" || pin != nil {
		t.Fatal("dpkg-query's no-match status was not absence", identity, err)
	}
	delete(exits, "wezterm")
	responses["wezterm"] = "wezterm\tamd64\t20240203-110809-5046fc22\trc \n"
	if identity, _, err := d.debIdentity(context.Background(), "wezterm"); err != nil || identity != "absent" {
		t.Fatal("a removed package with configuration files was treated as installed", identity, err)
	}
	responses["wezterm"] = "wezterm\tamd64\t20230712-072601-f4abf8fd\tii \n"
	if identity, pin, err := d.debIdentity(context.Background(), "wezterm"); err != nil || pin != nil || !strings.Contains(identity, "version=20230712") {
		t.Fatal("an unpinned version was proved", identity, err)
	}
	responses["ghostty"] = "ghostty:amd64\tamd64\t1.3.1-0~ppa2\tii \nghostty:arm64\tarm64\t1.3.1-0~ppa2\tii \n"
	if identity, pin, err := d.debIdentity(context.Background(), "ghostty"); err != nil || pin != nil || identity != "2 installed architectures" {
		t.Fatal("an ambiguous multiarch installation was proved", identity, err)
	}
	exits["ghostty"] = 2
	if _, _, err := d.debIdentity(context.Background(), "ghostty"); err == nil {
		t.Fatal("a failed dpkg-query became absence")
	}
	responses["consumers"] = "wezterm\tii \t\n" +
		"wezterm-extras\tii \tlibc6 (>= 2.34), wezterm (= 20240203-110809-5046fc22) | foot, \n" +
		"old-plugin\trc \twezterm, \n" +
		"multi:amd64\tii \tlibc6, wezterm:amd64\n"
	if consumers, err := d.debConsumers(context.Background(), "wezterm"); err != nil || !slices.Equal(consumers, []string{"multi:amd64", "wezterm-extras"}) {
		t.Fatal("installed dependents were not reported", consumers, err)
	}
}

func TestLegacyWingetExportAmbiguityIsRetained(t *testing.T) {
	inventory, err := parseLegacyWingetExport([]byte("\xef\xbb\xbf" + `{"Sources":[
		{"SourceDetails":{"Name":"winget"},"Packages":[{"PackageIdentifier":"jqlang.jq","Version":"1.7.1","Scope":"user"},{"PackageIdentifier":"jqlang.jq","Version":"1.8.0","Scope":"machine"}]},
		{"SourceDetails":{"Name":"msstore"},"Packages":[{"PackageIdentifier":"9NBLGGH4NNS1"}]}]}`))
	if err != nil || len(inventory["jqlang.jq"]) != 2 || len(inventory) != 1 {
		t.Fatal("winget export was not read per source", inventory, err)
	}
	if _, err := parseLegacyWingetExport([]byte(`{"Sources":[{"SourceDetails":{"Name":"winget"},"Packages":[{"PackageIdentifier":"bad id; rm"}]}]}`)); err == nil {
		t.Fatal("an invalid package identity was accepted")
	}
	if _, err := parseLegacyWingetExport([]byte(`{}`)); err == nil {
		t.Fatal("an export without sources was treated as empty")
	}
}

func TestLegacyWindowsProcessGateRequestIsData(t *testing.T) {
	gate, err := legacyProcessGate([]string{`C:\Users\fixture\scoop\apps\neovim`}, []string{"nvim"})
	if err != nil || gate.Kind != "windows-processes" || validateLegacyGate(gate) != nil {
		t.Fatal(gate, err)
	}
	d := &legacyRetirementDriver{Platform: NativePlatform{WindowsPowerShell: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`}}
	var seen nativeCommand
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		seen = command
		return []byte("1234 C:\\Users\\fixture\\scoop\\apps\\neovim\\0.12.6\\bin\\nvim.exe\n"), nil
	}
	value, err := d.windowsProcesses(context.Background(), gate.Subject)
	if err != nil || value != `1234 C:\Users\fixture\scoop\apps\neovim\0.12.6\bin\nvim.exe` {
		t.Fatal(value, err)
	}
	if !slices.Contains(seen.Environment, "DOTFILES_RETIREMENT_INPUT="+gate.Subject) || slices.ContainsFunc(seen.Arguments, func(argument string) bool { return strings.Contains(argument, "neovim") }) {
		t.Fatal("the process request was interpolated into the fixed script", seen)
	}
	if _, err := d.windowsProcesses(context.Background(), "not json"); err == nil {
		t.Fatal("an invalid saved process request was evaluated")
	}
	var native *nativeCommandError
	d.Query = func(context.Context, nativeCommand) ([]byte, error) {
		code := 1
		return nil, &nativeCommandError{Message: "inventory failed", ExitCode: &code}
	}
	if _, err := d.windowsProcesses(context.Background(), gate.Subject); !errors.As(err, &native) {
		t.Fatal("a failed process inventory was treated as no running process", err)
	}
}

// Linuxbrew discovery problems stay local to Homebrew deltas.
func TestLegacyHomebrewIssueDoesNotHideOtherLinuxDeltas(t *testing.T) {
	d, home := legacyCatalogFixture(t)
	d.Platform.HomebrewIssue = "Linuxbrew could not be inspected"
	d.Folders.Config = filepath.Join(home, ".config")
	writeConfigFixture(t, filepath.Join(d.Folders.Config, "autostart", "devilspie2.desktop"), legacyDevilspie2Autostart)
	scope, err := d.inspect(context.Background(), legacyDeltaPrefix+"devilspie2")
	if err != nil || len(scope.Steps) == 0 {
		t.Fatal("an unrelated Homebrew problem hid a non-Homebrew delta", scope, err)
	}
	if _, err := d.inspect(context.Background(), legacyDeltaPrefix+"jq"); err == nil || !strings.Contains(err.Error(), "Linuxbrew could not be inspected") {
		t.Fatal("a Homebrew delta did not report its discovery problem", err)
	}
}
