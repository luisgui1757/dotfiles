package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLegacyPylatexencPublishedReleaseLifecycle(t *testing.T) {
	for _, backend := range []string{"83.0.0", "84.0.0"} {
		t.Run(backend, func(t *testing.T) {
			d, home := legacyStepFixture(t)
			root := filepath.Join(home, "python-tools", "pylatexenc")
			site := filepath.Join(root, "lib", "python3.15", "site-packages")
			writeConfigFixture(t, filepath.Join(root, "pyvenv.cfg"), "include-system-site-packages = false\n")
			for _, name := range []string{"pip-26.2.1", "pylatexenc-2.11", "setuptools-" + backend} {
				writeConfigFixture(t, filepath.Join(site, name+".dist-info", "METADATA"), "Name: "+name+"\n")
			}
			writeConfigFixture(t, filepath.Join(root, "bin", "latex2text"), "released console")
			scope := legacyRetirementScope{}
			found, err := d.planPylatexencVenv(&scope, root)
			if err != nil || !found || len(scope.Retained) != 0 {
				t.Fatal("published environment was not recognized", found, scope, err)
			}
			intent := legacyTestIntent(t, d, scope.Steps...)
			for i := range scope.Steps {
				if err := d.executeStep(context.Background(), intent, i, false); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("retired environment remains", err)
			}
		})
	}
}

func TestLegacyPylatexencPreservesAdditionalDistribution(t *testing.T) {
	d, home := legacyStepFixture(t)
	root := filepath.Join(home, "python-tools", "pylatexenc")
	writeConfigFixture(t, filepath.Join(root, "pyvenv.cfg"), "include-system-site-packages = false\n")
	for _, name := range []string{"pip-26.2.1", "pylatexenc-2.11", "setuptools-84.0.0", "personal-1.0"} {
		writeConfigFixture(t, filepath.Join(root, "lib", "python3.15", "site-packages", name+".dist-info", "METADATA"), "fixture")
	}
	scope := legacyRetirementScope{}
	if found, err := d.planPylatexencVenv(&scope, root); err != nil || found || len(scope.Steps) != 0 || len(scope.Retained) == 0 {
		t.Fatal("personal environment was offered for retirement", found, scope, err)
	}
}

func TestLegacyChezmoiSourceSettingFormats(t *testing.T) {
	for _, test := range []struct{ name, data, want string }{
		{"chezmoi.toml", "sourceDir = '/a checkout/home'\n[data]\nsourceDir = 'unrelated'\n", "/a checkout/home"},
		{"chezmoi.json", `{"sourceDir":"/a checkout/home","data":{"sourceDir":"unrelated"}}`, "/a checkout/home"},
		{"chezmoi.jsonc", "// setting\n{\"sourceDir\":\"/a checkout/home\"}", "/a checkout/home"},
		{"chezmoi.yaml", "sourceDir: '/a checkout/home'\ndata:\n  sourceDir: unrelated\n", "/a checkout/home"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, found, err := legacyChezmoiSourceSetting(test.name, []byte(test.data))
			if err != nil || !found || value != test.want {
				t.Fatal(value, found, err)
			}
		})
	}
}

func TestLegacyChezmoiAmbiguousSourceIsRefused(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"chezmoi.json", `{"sourceDir":"/a","SourceDir":"/b"}`},
		{"chezmoi.toml", "sourceDir = '/a'\nSourceDir = '/b'\n"},
		{"chezmoi.yaml", "sourceDir: /a\nsourceDir: /b\n"},
		{"chezmoi.json", `{"sourceDir":42}`},
	} {
		if _, _, err := legacyChezmoiSourceSetting(test.name, []byte(test.data)); err == nil {
			t.Fatal("ambiguous source accepted", test)
		}
	}
}

func TestLegacyChezmoiTOMLUnicodeScalarBoundaries(t *testing.T) {
	for _, test := range []struct{ escape, want string }{
		{`\uD7FF`, "\uD7FF"}, {`\uE000`, "\uE000"}, {`\U0010FFFF`, "\U0010FFFF"},
		{`\uD800`, ""}, {`\uDFFF`, ""}, {`\U00110000`, ""},
		{`\U7FFFFFFF`, ""}, {`\U80000000`, ""}, {`\UFFFFFFFF`, ""},
	} {
		t.Run(test.escape, func(t *testing.T) {
			data := []byte(`sourceDir = "/checkout/` + test.escape + `"`)
			value, found, err := legacyChezmoiSourceSetting("chezmoi.toml", data)
			if test.want == "" {
				if err == nil {
					t.Fatal("an invalid Unicode scalar was accepted", value, found)
				}
			} else if err != nil || !found || value != "/checkout/"+test.want {
				t.Fatal("a valid Unicode scalar was not preserved", value, found, err)
			}
		})
	}
}

func TestLegacyMacApplicationInspectionFailureIsNotAbsence(t *testing.T) {
	d := &legacyRetirementDriver{Environment: legacyRuntimeEnvironment{Root: true}}
	d.Query = func(context.Context, nativeCommand) ([]byte, error) {
		code := 1
		return nil, &nativeCommandError{Message: "automation unavailable", ExitCode: &code}
	}
	if state, err := d.macApplicationState(context.Background(), "com.example.fixture"); err == nil || state != "" {
		t.Fatal("native inspection failure became permission to remove a running app", state, err)
	}
}

func TestLegacyManagerDeltaPlatformsAndReplacements(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range legacyManagerDeltas {
		if delta.Replacement != "" {
			if _, ok := catalog.Resource(delta.Replacement); !ok {
				t.Error("unknown replacement", delta.Key, delta.Replacement)
			}
		}
		if delta.Key == "windows-terminal" && !slices.Equal(delta.platforms(), []string{"windows"}) {
			t.Error("Windows Terminal offered outside Windows", delta.platforms())
		}
		if delta.Key == "aerospace" && !slices.Equal(delta.platforms(), []string{"darwin"}) {
			t.Error("AeroSpace offered outside macOS", delta.platforms())
		}
	}
}

func TestLegacyBrewConsumersProtectOutsideAndCaskDependencies(t *testing.T) {
	inventory := map[string]nativePackage{
		"library":  {Name: "library", Source: "library"},
		"outside":  {Name: "outside", Source: "personal/tap/outside", Dependencies: []string{"library"}},
		"cask/app": {Name: "cask/app", Source: "personal/tap/app", Dependencies: []string{"library"}},
	}
	if got := legacyBrewConsumers(inventory, "library", nil); !slices.Equal(got, []string{"cask/app", "outside"}) {
		t.Fatal("outside consumer lost", got)
	}
	if got := legacyBrewConsumers(inventory, "tap:personal/tap", []string{"outside"}); !slices.Equal(got, []string{"cask/app"}) {
		t.Fatal("tap still in use", got)
	}
	if got := legacyBrewIdentity(nativePackage{Name: "library", Version: "1,2", Source: "library", Held: true}); !strings.Contains(got, "versions=1,2") || !strings.Contains(got, "pinned=true") {
		t.Fatal("retirement identity omits versions or pin", got)
	}
}

func legacyReleasedSource(t *testing.T, commit, path string) string {
	t.Helper()
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("git", "-C", repository, "show", commit+":"+path).Output()
	if err != nil {
		t.Fatalf("released evidence requires complete release history: git show %s:%s: %v", commit, path, err)
	}
	return string(output)
}

// Single-quoted `$Name = 'value'` and double-quoted `NAME="value"` assignments.
func legacyReleasedValues(source string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(source, "\n") {
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(strings.TrimPrefix(key, "$")), strings.TrimSpace(value)
		if !ok || key == "" || strings.ContainsAny(key, " \t$(") || len(value) < 2 {
			continue
		}
		quote := value[:1]
		if quote != "'" && quote != `"` {
			continue
		}
		if end := strings.Index(value[1:], quote); end >= 0 {
			if _, seen := values[key]; !seen {
				values[key] = value[1 : end+1]
			}
		}
	}
	return values
}

// The released Windows $Catalog rows: tool -> manager -> package identifier.
func legacyReleasedWindowsCatalog(t *testing.T, source string) map[string]map[string]string {
	t.Helper()
	start := strings.Index(source, "$Catalog = @{")
	if start < 0 {
		t.Fatal("released Windows catalog not found")
	}
	block := source[start : start+strings.Index(source[start:], "\n}\n")]
	row := regexp.MustCompile(`(?m)^\s+'?([A-Za-z0-9-]+)'?\s+= @\{(.*)\}\s*$`)
	field := regexp.MustCompile(`(winget|choco|scoop)\s*=\s*'([^']*)'`)
	catalog := map[string]map[string]string{}
	for _, match := range row.FindAllStringSubmatch(block, -1) {
		catalog[match[1]] = map[string]string{}
		for _, item := range field.FindAllStringSubmatch(match[2], -1) {
			catalog[match[1]][item[1]] = item[2]
		}
	}
	return catalog
}

// Every package identity, pinned artifact and exclusion is compared with the
// released install-deps.ps1, install-deps.sh and nix-darwin configuration of
// both the pre-overhaul main and the published v0.4.4.
func TestLegacyManagerDeltasMatchReleasedInstallers(t *testing.T) {
	deltas := map[string]legacyManagerDelta{"chezmoi": legacyChezmoiPackages}
	for _, delta := range legacyManagerDeltas {
		deltas[delta.Key] = delta
	}
	// Released Windows catalog tools and the delta that retires each one.
	windowsTools := map[string]string{"nvim": "nvim", "cmake": "cmake", "rg": "ripgrep", "fd": "fd", "fzf": "fzf", "lsd": "lsd", "zoxide": "zoxide", "lazygit": "lazygit",
		"starship": "starship", "node": "node", "python": "python", "shellcheck": "shellcheck", "jq": "jq", "gh": "gh", "hyperfine": "hyperfine", "taplo": "taplo",
		"make": "make", "pwsh": "powershell", "win32yank": "clipboard", "zig": "zig", "code": "vscode", "wezterm": "wezterm", "wt": "windows-terminal", "chezmoi": "chezmoi"}
	// Released packages that stay, each for a stated reason.
	kept := map[string]string{
		"windows/git":       "Git for Windows is shared developer infrastructure",
		"windows/wt/winget": "packaged (MSIX) Windows Terminal stays personal, like Store builds",
		"windows/wt/choco":  "packaged (MSIX) Windows Terminal stays personal, like Store builds",
		"brew/gnu-tar":      "every supported host ships tar, so the presence check never requested it",
		"brew/git":          "a Linux-only delta; macOS reuses the Homebrew formula",
		"brew/tmux":         "a Linux-only delta; macOS reuses the Homebrew formula",
	}
	for _, commit := range []string{legacyReleaseCommit, legacyStableCommit} {
		t.Run(commit[:12], func(t *testing.T) {
			ps1 := legacyReleasedSource(t, commit, "install-deps.ps1")
			sh := legacyReleasedSource(t, commit, "install-deps.sh")
			darwin := legacyReleasedSource(t, commit, "nix/darwin/configuration.nix")
			for tool, managers := range legacyReleasedWindowsCatalog(t, ps1) {
				key, ok := windowsTools[tool]
				if !ok {
					if kept["windows/"+tool] == "" {
						t.Errorf("released Windows tool %s has neither a delta nor a stated exclusion", tool)
					}
					continue
				}
				delta := deltas[key]
				scoop := managers["scoop"]
				if scoop != "" && !strings.Contains(scoop, "/") {
					scoop = "main/" + scoop // `scoop install <app>` resolves in the main bucket.
				}
				for manager, got := range map[string][2]string{"scoop": {scoop, delta.Scoop}, "winget": {managers["winget"], delta.Winget}, "choco": {managers["choco"], delta.Choco}} {
					if got[0] != got[1] && kept["windows/"+tool+"/"+manager] == "" {
						t.Errorf("%s %s: released %q, delta %q", tool, manager, got[0], got[1])
					}
				}
			}
			for _, command := range []string{"scoop install nerd-fonts/Hack-NF", "scoop install psmux/psmux", "winget install psmux ", "choco install psmux -y",
				"Add-ScoopBucketSafe -Name 'psmux' -Url '" + legacyScoopBuckets["psmux"] + "'", "Add-ScoopBucketSafe -Name 'extras'", "Add-ScoopBucketSafe -Name 'nerd-fonts'",
				"Install-Module -Name PSFzf -Scope CurrentUser -Repository PSGallery"} {
				if !strings.Contains(ps1, command) {
					t.Errorf("released Windows installer no longer runs %q", command)
				}
			}
			if deltas["font"].Scoop != "nerd-fonts/Hack-NF" || deltas["psmux"].Scoop != "psmux/psmux" || deltas["psmux"].Choco != "psmux" || deltas["psmux"].Winget != "marlocarlo.psmux" {
				t.Error("psmux or Hack Nerd Font identities differ from the released commands")
			}
			table := sh[strings.Index(sh, "PKG_TABLE=$(cat <<'EOF'\n")+len("PKG_TABLE=$(cat <<'EOF'\n"):]
			for _, row := range strings.Split(table[:strings.Index(table, "\nEOF\n")], "\n") {
				brew := strings.Split(row, "|")[1]
				if brew == "" {
					continue
				}
				if !slices.ContainsFunc(slices.Collect(maps.Values(deltas)), func(delta legacyManagerDelta) bool {
					return delta.Formula == brew || delta.LinuxFormula == brew
				}) && kept["brew/"+brew] == "" {
					t.Errorf("released Homebrew formula %s has neither a delta nor a stated exclusion", brew)
				}
			}
			for _, cask := range []string{"ghostty", "wezterm", "visual-studio-code", "font-hack-nerd-font", "nikitabobko/tap/aerospace"} {
				if !strings.Contains(sh, "brew install --cask "+cask) || !slices.ContainsFunc(legacyManagerDeltas, func(delta legacyManagerDelta) bool { return delta.Cask == cask }) {
					t.Errorf("released cask %s is not both installed by the release and retired here", cask)
				}
			}
			if !strings.Contains(sh, "brew install herdr") || deltas["herdr"].Formula != "herdr" {
				t.Error("released Herdr formula differs")
			}
			for _, declared := range []string{`taps = [ "nikitabobko/tap" ];`, `"wezterm"`, `"aerospace"`, `"herdr"`} {
				if !strings.Contains(darwin, declared) {
					t.Errorf("released nix-darwin Homebrew declaration %s changed", declared)
				}
			}
			if deltas["aerospace"].Tap != "nikitabobko/tap" {
				t.Error("released AeroSpace tap differs")
			}

			windows, posix := legacyReleasedValues(ps1), legacyReleasedValues(sh)
			release := func(releases []legacyReleaseTree, version string) *legacyReleaseTree {
				for i := range releases {
					if slices.Contains(releases[i].Commits, commit) {
						if releases[i].Version != version {
							t.Errorf("%s pin %s differs from released %s", releases[i].URL, releases[i].Version, version)
						}
						return &releases[i]
					}
				}
				t.Fatalf("no pinned release for %s at %s", version, commit)
				return nil
			}
			pins := map[*legacyReleaseTree][2]string{
				release(legacyHerdrWindowsReleases, windows["HerdrWindowsPreviewVersion"]):                         {windows["HerdrWindowsX64Sha256"], "https://github.com/ogulcancelik/herdr/releases/download/" + windows["HerdrWindowsPreviewVersion"] + "/herdr-windows-x86_64.exe"},
				release(legacyTreeSitterWindowsReleases, windows["TreeSitterCliVersion"]):                          {windows["TreeSitterCliWindowsX64Sha256"], "https://github.com/tree-sitter/tree-sitter/releases/download/" + windows["TreeSitterCliVersion"] + "/tree-sitter-cli-windows-x64.zip"},
				release(legacyWindowsTerminalReleases, strings.TrimPrefix(windows["WindowsTerminalVersion"], "v")): {windows["WindowsTerminalX64Sha256"], "https://github.com/microsoft/terminal/releases/download/" + windows["WindowsTerminalVersion"] + "/Microsoft.WindowsTerminal_" + strings.TrimPrefix(windows["WindowsTerminalVersion"], "v") + "_x64.zip"},
				release(legacyHackFontReleases, windows["HackNerdFontVersion"]):                                    {windows["HackNerdFontSha256"], "https://github.com/ryanoasis/nerd-fonts/releases/download/" + windows["HackNerdFontVersion"] + "/Hack.zip"},
			}
			for pin, want := range pins {
				if pin.SHA256 != want[0] || pin.URL != want[1] {
					t.Errorf("%s differs from the released pin %s", pin.URL, want[1])
				}
			}
			if posix["HACK_NERD_FONT_VERSION"] != windows["HackNerdFontVersion"] || posix["HACK_NERD_FONT_SHA256"] != windows["HackNerdFontSha256"] {
				t.Error("released Linux and Windows Hack Nerd Font pins differ")
			}
			if windows["PsmuxPluginsCommit"] != legacyPsmuxPluginsCommit || !slices.Contains(legacyGhDashTags, windows["GhDashVersion"]) || !slices.Contains(legacyGhDashTags, posix["GH_DASH_VERSION"]) ||
				!slices.Contains([]string{"1.0.4", "0.82.1"}, windows["PiCliVersion"]) || windows["PylatexencVersion"] != legacyPylatexencVersion ||
				!slices.Contains([]string{legacySetuptoolsVersion, "83.0.0"}, windows["PylatexencBuildBackendVersion"]) {
				t.Error("released psmux, gh-dash, Pi or converter identity differs")
			}
			wezterm := legacyDebPins[0]
			if wezterm.Version != posix["WEZTERM_VERSION"] || wezterm.Archive != posix["WEZTERM_DEB_AMD64_SHA256"] ||
				wezterm.URL != "https://github.com/wezterm/wezterm/releases/download/"+posix["WEZTERM_VERSION"]+"/wezterm-"+posix["WEZTERM_VERSION"]+".Ubuntu22.04.deb" {
				t.Error("released WezTerm .deb pin differs")
			}
			version := posix["GHOSTTY_UBUNTU_VERSION"]
			for _, target := range []struct{ arch, release, key string }{{"amd64", "24.04", "UBUNTU_AMD64_2404"}, {"arm64", "24.04", "UBUNTU_ARM64_2404"}, {"amd64", "25.10", "UBUNTU_AMD64_2510"},
				{"arm64", "25.10", "UBUNTU_ARM64_2510"}, {"amd64", "trixie", "DEBIAN_AMD64_TRIXIE"}, {"arm64", "trixie", "DEBIAN_ARM64_TRIXIE"}} {
				url := "https://github.com/mkasberg/ghostty-ubuntu/releases/download/" + version + "/ghostty_" + strings.Replace(version, "-0-", "-0.", 1) + "_" + target.arch + "_" + target.release + ".deb"
				if !slices.ContainsFunc(legacyDebPins, func(pin legacyDebPin) bool {
					return pin.Package == "ghostty" && pin.Architecture == target.arch && pin.Version == strings.Replace(version, "-0-", "-0~", 1) && pin.URL == url && pin.Archive == posix["GHOSTTY_"+target.key+"_SHA256"]
				}) {
					t.Errorf("released Ghostty %s/%s .deb pin differs", target.arch, target.release)
				}
			}
		})
	}
}

func writeLegacyScoopApp(t *testing.T, root, app, bucket, manifest string, current bool) {
	t.Helper()
	install, installed := "scoop-install.json", "scoop-manifest.json"
	if !current {
		install, installed = "install.json", "manifest.json"
	}
	writeConfigFixture(t, filepath.Join(root, "apps", app, "current", install), `{"bucket":"`+bucket+`","architecture":"64bit"}`)
	writeConfigFixture(t, filepath.Join(root, "apps", app, "current", installed), manifest)
}

// Scoop runs strings from the installed manifest. Retirement accepts them only
// when the official bucket's own history recorded exactly those bytes.
func TestLegacyScoopAppRequiresBucketProvenance(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("the contributor gate requires Git:", err)
	}
	d, home := legacyStepFixture(t)
	d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
	d.Environment.Programs["git"] = git
	root := filepath.Join(home, "scoop")
	t.Setenv("SCOOP", root)
	writeConfigFixture(t, filepath.Join(root, "apps", "scoop", "current", "bin", "scoop.ps1"), "# Scoop\n")
	bucket := filepath.Join(root, "buckets", "main")
	if err := os.MkdirAll(bucket, 0700); err != nil {
		t.Fatal(err)
	}
	legacyGitFixture(t, bucket, "init", "-q")
	legacyGitFixture(t, bucket, "remote", "add", "origin", "https://github.com/ScoopInstaller/Main")
	released := "{\n    \"version\": \"1.7.1\",\n    \"post_uninstall\": \"Remove-Item $persist_dir\\\\cache\"\n}\n"
	writeConfigFixture(t, filepath.Join(bucket, "bucket", "jq.json"), released)
	legacyGitFixture(t, bucket, "add", ".")
	legacyGitFixture(t, bucket, "commit", "-q", "-m", "jq 1.7.1")
	writeConfigFixture(t, filepath.Join(bucket, "bucket", "jq.json"), strings.Replace(released, "1.7.1", "1.8.1", 1))
	legacyGitFixture(t, bucket, "commit", "-q", "-am", "jq 1.8.1")
	jq := deltaByKey(t, "jq")

	// A Git for Windows checkout converts the recorded LF bytes to CRLF.
	writeLegacyScoopApp(t, root, "jq", "main", strings.ReplaceAll(released, "\n", "\r\n"), true)
	scope := legacyRetirementScope{}
	if found, err := d.planScoopApp(context.Background(), &scope, jq); err != nil || !found || len(scope.Steps) != 1 || len(scope.Retained) != 0 {
		t.Fatal("a manifest from the official bucket's history was not proved", scope, err)
	}
	gates := []string{}
	for _, gate := range scope.Steps[0].Gates {
		gates = append(gates, gate.Kind)
	}
	if !slices.Equal(gates, []string{"windows-processes", "scoop-app", "scoop-consumers"}) || !slices.Contains(scope.Steps[0].Absent, filepath.Join(root, "apps", "jq")) {
		t.Fatal("the Scoop uninstall is not gated on processes, identity and consumers", scope.Steps[0])
	}

	for _, scenario := range []struct {
		name    string
		prepare func()
	}{
		{"modified manifest", func() {
			writeLegacyScoopApp(t, root, "jq", "main", strings.Replace(released, "cache", "everything", 1), true)
		}},
		{"earlier file names", func() {
			if err := os.RemoveAll(filepath.Join(root, "apps", "jq")); err != nil {
				t.Fatal(err)
			}
			writeLegacyScoopApp(t, root, "jq", "main", released, false)
		}},
		{"foreign origin", func() {
			writeLegacyScoopApp(t, root, "jq", "main", released, true)
			legacyGitFixture(t, bucket, "remote", "set-url", "origin", "https://github.com/someone/Main")
		}},
		{"consumer", func() {
			legacyGitFixture(t, bucket, "remote", "set-url", "origin", "https://github.com/ScoopInstaller/Main.git")
			writeLegacyScoopApp(t, root, "tool", "main", `{"version":"1","depends":"main/jq"}`, true)
		}},
	} {
		scenario.prepare()
		scope := legacyRetirementScope{}
		found, err := d.planScoopApp(context.Background(), &scope, jq)
		wantStep := scenario.name == "earlier file names"
		if err != nil || !found || (len(scope.Steps) == 1) != wantStep || (len(scope.Retained) == 0) != wantStep {
			t.Errorf("%s: %+v %v", scenario.name, scope, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(bucket, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "apps", "tool")); err != nil {
		t.Fatal(err)
	}
	scope = legacyRetirementScope{}
	if _, err := d.planScoopApp(context.Background(), &scope, jq); err != nil || len(scope.Steps) != 0 || !strings.Contains(strings.Join(scope.Retained, "\n"), "scoop update") {
		t.Fatal("a bucket without history proved the manifest", scope, err)
	}
	writeLegacyScoopApp(t, root, "jq", "extras", released, true)
	scope = legacyRetirementScope{}
	if found, err := d.planScoopApp(context.Background(), &scope, jq); err != nil || found || len(scope.Steps) != 0 {
		t.Fatal("an app from another bucket was treated as the released one", scope, err)
	}
}

func deltaByKey(t *testing.T, key string) legacyManagerDelta {
	t.Helper()
	index := slices.IndexFunc(legacyManagerDeltas, func(delta legacyManagerDelta) bool { return delta.Key == key })
	if index < 0 {
		t.Fatal("unknown delta", key)
	}
	return legacyManagerDeltas[index]
}

func writeLegacyChocoPackage(t *testing.T, root, id, version string, dependencies []string, files map[string]string) {
	t.Helper()
	deps := ""
	for _, dependency := range dependencies {
		deps += `<dependency id="` + dependency + `" />`
	}
	nuspec := `<?xml version="1.0"?><package><metadata><id>` + id + `</id><version>` + version + `</version><dependencies>` + deps + `</dependencies></metadata></package>`
	library := filepath.Join(root, "lib", id)
	writeConfigFixture(t, filepath.Join(library, id+".nuspec"), nuspec)
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, data := range map[string]string{id + ".nuspec": nuspec} {
		files[name] = data
	}
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
		if name != id+".nuspec" {
			writeConfigFixture(t, filepath.Join(library, filepath.FromSlash(name)), data)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(library, id+".nupkg"), archive.String())
}

func TestLegacyChocoBootstrapInventoryReadsRetainedArchive(t *testing.T) {
	root := t.TempDir()
	writeLegacyChocoPackage(t, root, "chocolatey", "2.7.4", []string{"personal-runtime"}, map[string]string{})
	library := filepath.Join(root, "lib", "chocolatey")
	if err := os.Remove(filepath.Join(library, "chocolatey.nuspec")); err != nil {
		t.Fatal(err)
	}
	inventory, err := legacyChocoInventory(root)
	if err != nil {
		t.Fatal("the official bootstrap retains only chocolatey.nupkg", err)
	}
	pkg := inventory["chocolatey"]
	if pkg.Version != "2.7.4" || !slices.Equal(legacyChocoConsumers(inventory, "personal-runtime", nil), []string{"chocolatey"}) {
		t.Fatal("bootstrap metadata and dependency protection must survive", inventory)
	}
	writeLegacyChocoPackage(t, root, "chocolatey", "2.7.4", []string{"different-runtime"}, map[string]string{})
	if err := os.Remove(filepath.Join(library, "chocolatey.nuspec")); err != nil {
		t.Fatal(err)
	}
	changed, err := legacyChocoInventory(root)
	if err != nil || changed["chocolatey"].Identity == pkg.Identity {
		t.Fatal("inventory identity must change with the retained archive at the same version", changed, err)
	}
}

func TestLegacyChocoMissingMetadataRemainsAnError(t *testing.T) {
	for _, id := range []string{"chocolatey", "personal-tool"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			writeLegacyChocoPackage(t, root, id, "1.0", nil, map[string]string{})
			library := filepath.Join(root, "lib", id)
			if err := os.Remove(filepath.Join(library, id+".nuspec")); err != nil {
				t.Fatal(err)
			}
			if id == "chocolatey" {
				writeConfigFixture(t, filepath.Join(library, id+".nupkg"), "invalid archive")
			}
			if _, err := legacyChocoInventory(root); err == nil {
				t.Fatal("unreadable bootstrap metadata and damaged ordinary packages must block retirement")
			}
		})
	}
}

func TestLegacyChocoBootstrapRejectsInvalidArchiveMetadata(t *testing.T) {
	valid := `<package><metadata><id>chocolatey</id><version>2.7.4</version></metadata></package>`
	for _, scenario := range []struct {
		name, file, data string
		copies           int
	}{
		{"duplicate", "chocolatey.nuspec", valid, 2},
		{"missing", "other.nuspec", valid, 1},
		{"nested", "tools/chocolatey.nuspec", valid, 1},
		{"wrong identity", "chocolatey.nuspec", strings.Replace(valid, "chocolatey", "personal-tool", 1), 1},
		{"malformed", "chocolatey.nuspec", "<package>", 1},
		{"oversized", "chocolatey.nuspec", strings.Repeat(" ", 1<<20+1), 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			for range scenario.copies {
				file, err := writer.Create(scenario.file)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(file, scenario.data); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, filepath.Join(root, "lib", "chocolatey", "chocolatey.nupkg"), archive.String())
			if _, err := legacyChocoInventory(root); err == nil {
				t.Fatal("untrusted bootstrap metadata must block retirement")
			}
		})
	}
}

// Chocolatey runs package scripts. They are accepted only from the community
// repository and only while equal to the package's own retained .nupkg.
func TestLegacyChocoPackageRequiresUnmodifiedCommunityScripts(t *testing.T) {
	d, home := legacyStepFixture(t)
	d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
	root := filepath.Join(home, "chocolatey")
	t.Setenv("ChocolateyInstall", root)
	writeConfigFixture(t, filepath.Join(root, "choco.exe"), "choco")
	config := `<?xml version="1.0"?><chocolatey><sources><source id="chocolatey" value="https://community.chocolatey.org/api/v2/" disabled="false" /><source id="old" value="https://example.invalid/feed" disabled="true" /></sources></chocolatey>`
	writeConfigFixture(t, filepath.Join(root, "config", "chocolatey.config"), config)
	writeLegacyChocoPackage(t, root, "python", "3.13.7", []string{"python3"}, map[string]string{})
	writeLegacyChocoPackage(t, root, "python3", "3.13.7", []string{"python313"}, map[string]string{})
	writeLegacyChocoPackage(t, root, "python313", "3.13.7", []string{"chocolatey-compatibility.extension"}, map[string]string{"tools/chocolateyUninstall.ps1": "Uninstall-ChocolateyPackage python\n"})
	writeLegacyChocoPackage(t, root, "chocolatey-compatibility.extension", "1.0.0", nil, map[string]string{"extensions/compatibility.psm1": "function Get-Compatible {}\n"})
	writeConfigFixture(t, filepath.Join(root, "extensions", "chocolatey-compatibility", "compatibility.psm1"), "function Get-Compatible {}\n")
	python := deltaByKey(t, "python")

	scope := legacyRetirementScope{}
	if found, err := d.planChocoPackage(context.Background(), &scope, python); err != nil || !found || len(scope.Retained) != 0 || len(scope.Steps) != 3 {
		t.Fatal("the released Chocolatey Python chain was not planned", scope, err)
	}
	order := []string{}
	for _, step := range scope.Steps {
		order = append(order, filepath.Base(step.Absent[0]))
	}
	if !slices.Equal(order, []string{"python", "python3", "python313"}) {
		t.Fatal("the virtual package chain is not removed top-down", order)
	}
	if !strings.Contains(strings.Join(scope.Detail, "\n"), "chocolatey-compatibility.extension stay installed") {
		t.Fatal("a shared dependency was not disclosed as kept", scope.Detail)
	}

	for _, scenario := range []struct {
		name    string
		prepare func()
	}{
		{"modified script", func() {
			writeConfigFixture(t, filepath.Join(root, "lib", "python313", "tools", "chocolateyUninstall.ps1"), "Remove-Item C:\\ -Recurse\n")
		}},
		{"added script", func() {
			writeLegacyChocoPackage(t, root, "python313", "3.13.7", []string{"chocolatey-compatibility.extension"}, map[string]string{"tools/chocolateyUninstall.ps1": "Uninstall-ChocolateyPackage python\n"})
			writeConfigFixture(t, filepath.Join(root, "lib", "python313", "tools", "chocolateyBeforeModify.ps1"), "personal\n")
		}},
		{"modified extension", func() {
			if err := os.Remove(filepath.Join(root, "lib", "python313", "tools", "chocolateyBeforeModify.ps1")); err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, filepath.Join(root, "extensions", "chocolatey-compatibility", "compatibility.psm1"), "function Get-Compatible { Remove-Item C:\\ }\n")
		}},
		{"other source", func() {
			writeConfigFixture(t, filepath.Join(root, "extensions", "chocolatey-compatibility", "compatibility.psm1"), "function Get-Compatible {}\n")
			writeConfigFixture(t, filepath.Join(root, "config", "chocolatey.config"), strings.Replace(config, `disabled="true"`, `disabled="false"`, 1))
		}},
		{"outside consumer", func() {
			writeConfigFixture(t, filepath.Join(root, "config", "chocolatey.config"), config)
			writeLegacyChocoPackage(t, root, "personal-tool", "1.0", []string{"python313"}, map[string]string{})
		}},
	} {
		scenario.prepare()
		scope := legacyRetirementScope{}
		if found, err := d.planChocoPackage(context.Background(), &scope, python); err != nil || !found || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
			t.Errorf("%s: %+v %v", scenario.name, scope, err)
		}
	}
}

// winget reports an ambiguous inventory; retirement never chooses one.
func TestLegacyWingetAmbiguousInstallationsAreRetained(t *testing.T) {
	d, home := legacyStepFixture(t)
	d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
	d.Folders.LocalAppData = filepath.Join(home, "AppData", "Local")
	writeConfigFixture(t, filepath.Join(d.Folders.LocalAppData, "Microsoft", "WindowsApps", "winget.exe"), "winget")
	export := `{"Sources":[{"SourceDetails":{"Name":"winget"},"Packages":[{"PackageIdentifier":"jqlang.jq","Version":"1.7.1"}]}]}`
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		index := slices.Index(command.Arguments, "--output")
		if filepath.Base(command.Program) != "winget.exe" || command.Arguments[0] != "export" || index < 0 {
			t.Fatal("unexpected inventory command", command)
		}
		return nil, os.WriteFile(command.Arguments[index+1], []byte(export), 0600)
	}
	jq := deltaByKey(t, "jq")
	scope := legacyRetirementScope{}
	if found, err := d.planWingetPackage(context.Background(), &scope, jq); err != nil || !found || len(scope.Steps) != 1 {
		t.Fatal("the winget package was not planned", scope, err)
	}
	arguments := scope.Steps[0].Command.Arguments
	if !slices.Contains(arguments, "--exact") || !slices.Contains(arguments, "jqlang.jq") || slices.Contains(arguments, "--purge") || slices.Contains(arguments, "--force") {
		t.Fatal("the winget uninstall is not exact", arguments)
	}
	export = `{"Sources":[{"SourceDetails":{"Name":"winget"},"Packages":[{"PackageIdentifier":"jqlang.jq","Version":"1.7.1"},{"PackageIdentifier":"jqlang.jq","Version":"1.8.1"}]}]}`
	if _, err := d.wingetInventory(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	scope = legacyRetirementScope{}
	if found, err := d.planWingetPackage(context.Background(), &scope, jq); err != nil || !found || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
		t.Fatal("an ambiguous winget inventory chose an installation", scope, err)
	}
}

func legacyPinnedDownload(t *testing.T, url, sha string) []byte {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s: HTTP %d", url, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 512<<20))
	if err != nil {
		t.Fatal(err)
	}
	if legacyHash(data) != sha {
		t.Fatalf("%s no longer has its pinned SHA-256 %s", url, sha)
	}
	return data
}

// Opt-in: download each pinned archive, verify its released SHA-256, recompute
// the member hashes the recognizers trust, and drive the planners with the
// real released bytes.
func TestLegacyReleaseTreesMatchPinnedArchives(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LEGACY_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_LEGACY_ARCHIVES=1 to download every pinned release archive and recompute its member hashes")
	}
	members := map[string]map[string][]byte{}
	for _, group := range [][]legacyReleaseTree{legacyHerdrWindowsReleases, legacyTreeSitterWindowsReleases, legacyWindowsTerminalReleases, legacyHackFontReleases} {
		for _, release := range group {
			data := legacyPinnedDownload(t, release.URL, release.SHA256)
			files := map[string][]byte{}
			if !strings.HasSuffix(release.URL, ".zip") {
				files[path.Base(release.URL)] = data
			} else {
				reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal(err)
				}
				for _, file := range reader.File {
					if file.FileInfo().IsDir() {
						continue
					}
					name, ok := strings.CutPrefix(file.Name, release.Prefix)
					if !ok {
						t.Fatalf("%s has member %s outside the copied %s", release.URL, file.Name, release.Prefix)
					}
					opened, err := file.Open()
					if err != nil {
						t.Fatal(err)
					}
					content, err := io.ReadAll(opened)
					if err := errors.Join(err, opened.Close()); err != nil {
						t.Fatal(err)
					}
					files[name] = content
				}
			}
			hashes := map[string]string{}
			for name, content := range files {
				hashes[name] = legacyHash(content)
			}
			if !maps.Equal(hashes, release.Files) {
				t.Errorf("%s members differ from the pinned member table", release.URL)
			}
			members[release.URL] = files
		}
	}

	for _, release := range legacyHackFontReleases {
		d, home := legacyStepFixture(t)
		d.Folders.Data = filepath.Join(home, "data")
		for name, content := range members[release.URL] {
			writeConfigFixture(t, filepath.Join(d.Folders.Data, "fonts", "HackNerdFont", name), string(content))
		}
		scope := legacyRetirementScope{}
		if err := d.planLinuxHackFont(&scope); err != nil || len(scope.Steps) != 1 || len(scope.Retained) != 0 {
			t.Fatal("the unzipped Linux font release was not recognized", release.Version, scope, err)
		}
		runLegacySteps(t, d, scope.Steps)
		if !legacyAbsent(filepath.Join(d.Folders.Data, "fonts", "HackNerdFont")) {
			t.Fatal("released Linux font directory remains")
		}

		d, home = legacyStepFixture(t)
		d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
		d.Folders.LocalAppData = filepath.Join(home, "AppData", "Local")
		t.Setenv("SCOOP", filepath.Join(home, "scoop"))
		for name, content := range members[release.URL] {
			if strings.HasSuffix(name, ".ttf") {
				writeConfigFixture(t, filepath.Join(d.Folders.LocalAppData, "Microsoft", "Windows", "Fonts", name), string(content))
			}
		}
		scope = legacyRetirementScope{}
		if err := d.planWindowsHackFonts(&scope); err != nil || len(scope.Steps) != 12 || len(scope.Retained) != 0 {
			t.Fatal("the copied Windows font faces were not recognized", release.Version, scope, err)
		}
	}
	for _, release := range legacyWindowsTerminalReleases {
		d, home := legacyStepFixture(t)
		d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
		d.Folders.LocalAppData = filepath.Join(home, "AppData", "Local")
		d.Query = func(context.Context, nativeCommand) ([]byte, error) {
			return []byte("0 " + strings.Repeat("a", 64)), nil
		}
		root := filepath.Join(d.Folders.LocalAppData, "Programs", "WindowsTerminal")
		for name, content := range members[release.URL] {
			writeConfigFixture(t, filepath.Join(root, filepath.FromSlash(name)), string(content))
		}
		scope := legacyRetirementScope{}
		if err := d.planWindowsTerminalPortable(context.Background(), &scope, root); err != nil || len(scope.Steps) != 1 || len(scope.Retained) != 0 || scope.Steps[0].Gates[0].Kind != "windows-processes" {
			t.Fatal("the copied portable Windows Terminal was not recognized", release.Version, scope, err)
		}
		writeConfigFixture(t, filepath.Join(root, "settings.json"), "{}")
		scope = legacyRetirementScope{}
		if err := d.planWindowsTerminalPortable(context.Background(), &scope, root); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
			t.Fatal("a personal file in the portable tree did not preserve it", scope, err)
		}
	}
	for _, release := range legacyTreeSitterWindowsReleases {
		d, home := legacyStepFixture(t)
		d.Platform.OS, d.Platform.WindowsPowerShell = "windows", "powershell.exe"
		d.Folders.LocalAppData = filepath.Join(home, "AppData", "Local")
		d.Query = func(context.Context, nativeCommand) ([]byte, error) {
			return []byte("1 " + strings.Repeat("b", 64)), nil
		}
		writeConfigFixture(t, filepath.Join(d.Folders.LocalAppData, "dotfiles", "bin", "tree-sitter.exe"), string(members[release.URL]["tree-sitter.exe"]))
		scope := legacyRetirementScope{}
		delta := deltaByKey(t, "tree-sitter")
		if err := d.planWindowsDirect(context.Background(), &scope, delta); err != nil || len(scope.Retained) != 0 || len(scope.Steps) != 3 {
			t.Fatal("the released tree-sitter executable was not recognized", release.Version, scope, err)
		}
		if last := scope.Steps[2]; last.Kind != "command" || last.Gates[0].Kind != "path-absent" {
			t.Fatal("the User PATH entry is not removed after its directory", last)
		}
	}

	dpkg, err := exec.LookPath("dpkg-deb")
	if err != nil {
		t.Skip("the .deb member check needs dpkg-deb to read the pinned packages:", err)
	}
	for _, pin := range legacyDebPins {
		file := filepath.Join(t.TempDir(), "package.deb")
		if err := os.WriteFile(file, legacyPinnedDownload(t, pin.URL, pin.Archive), 0600); err != nil {
			t.Fatal(err)
		}
		fields, err := exec.Command(dpkg, "--field", file, "Package", "Architecture", "Version").Output()
		if err != nil || string(fields) != "Package: "+pin.Package+"\nArchitecture: "+pin.Architecture+"\nVersion: "+pin.Version+"\n" {
			t.Fatalf("%s control fields differ: %s %v", pin.URL, fields, err)
		}
		data, err := exec.Command(dpkg, "--fsys-tarfile", file).Output()
		if err != nil {
			t.Fatal(err)
		}
		reader, found := tar.NewReader(bytes.NewReader(data)), false
		for {
			header, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if header.Name == "."+pin.Executable {
				content, err := io.ReadAll(reader)
				if err != nil || legacyHash(content) != pin.SHA256 {
					t.Fatalf("%s %s differs from its pin: %v", pin.URL, pin.Executable, err)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("%s has no %s", pin.URL, pin.Executable)
		}
	}
}

func TestLegacyFontReusedBySetupIsNotRetired(t *testing.T) {
	d, home := legacyStepFixture(t)
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "darwin", Arch: "arm64"}
	d.Normal.Catalog, d.Normal.Context = catalog, target
	d.Platform = NativePlatform{Context: target, Homebrew: &HomebrewLocation{Program: filepath.Join(home, "brew"), Prefix: home, Cellar: filepath.Join(home, "Cellar")}}
	writeConfigFixture(t, filepath.Join(home, "Caskroom", "font-hack-nerd-font", "witness"), "native package")
	writeConfigFixture(t, d.Platform.Homebrew.Program, "#!/bin/sh\ncat <<'JSON'\n{\"formulae\":[],\"casks\":[{\"token\":\"font-hack-nerd-font\",\"full_token\":\"font-hack-nerd-font\",\"installed\":\"3.5.1\",\"depends_on\":{\"formula\":[]}}]}\nJSON\n")
	if err := os.Chmod(d.Platform.Homebrew.Program, 0700); err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: target, Target: home, Source: "fixture", Generation: 1, Selected: []string{"ghostty"}, Receipts: map[string]Receipt{
		"tool.font": {Ownership: "reused", Status: "ready", After: Observation{Provider: "native-font", Present: true, Healthy: true}},
	}}
	if err := SaveState(d.Normal.StatePath, state); err != nil {
		t.Fatal(err)
	}
	scope := legacyRetirementScope{}
	if err := d.inspectManagerDelta(context.Background(), "retire.delta.font", &scope); err != nil || len(scope.Steps) != 0 || !strings.Contains(strings.Join(scope.Detail, " "), "reused") {
		t.Fatal("retirement would remove setup's reused native font", scope, err)
	}
	state.Selected = []string{}
	if err := SaveState(d.Normal.StatePath, state); err != nil {
		t.Fatal(err)
	}
	scope = legacyRetirementScope{}
	if err := d.inspectManagerDelta(context.Background(), "retire.delta.font", &scope); err != nil || len(scope.Steps) != 1 {
		t.Fatal("unselected historical reuse blocked explicit retirement", scope, err)
	}
}
