package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func legacyNativePowerShell(t *testing.T, d *legacyRetirementDriver, script string, input any) {
	t.Helper()
	command, err := d.windowsScript(script, input)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := d.Run(context.Background(), command); err != nil {
		t.Fatalf("native PowerShell: %v%s", err, nativeDiagnostic(output))
	}
}

func TestLegacyWindowsScoopDeltaLifecycle(t *testing.T) {
	legacyNativePackages(t, "windows")
	d := legacyNativeDriver(t)
	root := filepath.Join(d.Folders.Home, "scoop")
	t.Setenv("SCOOP", root)
	t.Setenv("SCOOP_GLOBAL", filepath.Join(d.Folders.Home, "global-scoop"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(d.Folders.Home, "config"))
	// Scoop supports a portable config beside apps/. Keep all fixture paths
	// private and use its actual scripts, installer metadata and uninstall.
	writeConfigFixture(t, filepath.Join(root, "config.json"), `{"use_isolated_path":true,"use_sqlite_cache":false}`)
	for _, checkout := range []struct{ path, url, commit string }{
		{filepath.Join(root, "apps", "scoop", "current"), "https://github.com/ScoopInstaller/Scoop", "e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f"},
		{filepath.Join(root, "buckets", "main"), "https://github.com/ScoopInstaller/Main", "bcff7c46a4ad0a7765b9f61260ab75440a3e69ea"},
	} {
		legacyRun(t, "", nil, "git", "clone", "--quiet", checkout.url, checkout.path)
		legacyRun(t, "", nil, "git", "-C", checkout.path, "checkout", "--detach", checkout.commit)
	}
	script := filepath.Join(root, "apps", "scoop", "current", "bin", "scoop.ps1")
	legacyNativePowerShell(t, d, `
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
& ([string]$request.script) install hyperfine --no-update-scoop
exit $LASTEXITCODE
`, struct {
		Script string `json:"script"`
	}{script})
	program := filepath.Join(root, "apps", "hyperfine", "current", "hyperfine.exe")
	if output := legacyRun(t, "", nil, program, "--version"); !strings.HasPrefix(output, "hyperfine ") {
		t.Fatal(output)
	}
	personal := filepath.Join(root, "persist", "hyperfine", "personal.txt")
	writeConfigFixture(t, personal, "preserve persisted data\n")
	scope := legacyRetirementScope{}
	if found, err := d.planScoopApp(context.Background(), &scope, deltaByKey(t, "hyperfine")); err != nil || !found || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	requireLegacyAbsent(t, filepath.Join(root, "apps", "hyperfine"))
	if data, err := os.ReadFile(personal); err != nil || string(data) != "preserve persisted data\n" {
		t.Fatal("Scoop persisted data changed", err)
	}
}

func TestLegacyWindowsPiNpmShimLifecycle(t *testing.T) {
	legacyNativePackages(t, "windows")
	d := legacyNativeDriver(t)
	npm, err := exec.LookPath("npm.cmd")
	if err != nil {
		t.Fatal("native npm is required for the released Pi fixture", err)
	}
	prefix := filepath.Join(d.Folders.Home, "npm-prefix")
	t.Setenv("SCOOP", filepath.Join(d.Folders.Home, "scoop"))
	t.Setenv("npm_config_cache", filepath.Join(d.Folders.Home, "npm-cache"))
	writeConfigFixture(t, filepath.Join(d.Folders.Home, ".npmrc"), "prefix="+prefix+"\n")
	t.Setenv("npm_config_userconfig", filepath.Join(d.Folders.Home, ".npmrc"))
	globalConfig := filepath.Join(d.Folders.Home, "npm-globalrc")
	writeConfigFixture(t, globalConfig, "")
	t.Setenv("npm_config_globalconfig", globalConfig)
	legacyNativePowerShell(t, d, `
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
& ([string]$request.npm) install --global --prefix ([string]$request.prefix) --no-audit --no-fund --registry=https://registry.npmjs.org '@earendil-works/pi-coding-agent@1.0.4'
exit $LASTEXITCODE
`, struct {
		Npm    string `json:"npm"`
		Prefix string `json:"prefix"`
	}{npm, prefix})
	for _, name := range []string{"pi", "pi.cmd", "pi.ps1"} {
		if _, err := os.Stat(filepath.Join(prefix, name)); err != nil {
			t.Fatal("npm did not generate its real command shim", name, err)
		}
	}
	personal := filepath.Join(d.Folders.Home, ".pi", "agent", "personal.txt")
	outside := filepath.Join(prefix, "unrelated-command.cmd")
	writeConfigFixture(t, personal, "personal Pi data\n")
	writeConfigFixture(t, outside, "@echo unrelated\n")
	scope := legacyRetirementScope{}
	if err := d.inspectPi(&scope); err != nil || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	for _, name := range []string{"pi", "pi.cmd", "pi.ps1", "node_modules/@earendil-works/pi-coding-agent"} {
		requireLegacyAbsent(t, filepath.Join(prefix, name))
	}
	for path, want := range map[string]string{personal: "personal Pi data\n", outside: "@echo unrelated\n"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatal("personal or unrelated npm data changed", path, err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(prefix, "node_modules", "@earendil-works", ".dotfiles-retired-*-pi-coding-agent", "package.json"))
	if err != nil || len(backups) != 1 {
		t.Fatal("Pi package was not preserved for inspection", backups, err)
	}
}

func TestLegacyWindowsDirectExecutableLifecycle(t *testing.T) {
	legacyNativePackages(t, "windows")
	d := legacyNativeDriver(t)
	d.Folders.LocalAppData = filepath.Join(d.Folders.Home, "local-app-data")
	release := legacyTreeSitterWindowsReleases[0]
	data := legacyPinnedDownload(t, release.URL, release.SHA256)
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(d.Folders.LocalAppData, "dotfiles", "bin", "tree-sitter.exe")
	found := false
	for _, member := range archive.File {
		if member.Name != "tree-sitter.exe" {
			continue
		}
		if found {
			t.Fatal("duplicate released executable")
		}
		reader, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(reader, 64<<20))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || legacyHash(payload) != release.Files[member.Name] {
			t.Fatal("released member verification failed", readErr, closeErr)
		}
		writeConfigFixture(t, program, string(payload))
		found = true
	}
	if !found {
		t.Fatal("the released archive has no executable")
	}
	if output := legacyRun(t, "", nil, program, "--version"); !strings.HasPrefix(output, "tree-sitter "+strings.TrimPrefix(release.Version, "v")) {
		t.Fatal(output)
	}
	personal := filepath.Join(filepath.Dir(program), "unrelated.txt")
	writeConfigFixture(t, personal, "preserve sibling\n")
	scope := legacyRetirementScope{}
	if err := d.planWindowsDirect(context.Background(), &scope, deltaByKey(t, "tree-sitter")); err != nil || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	requireLegacyAbsent(t, program)
	if data, err := os.ReadFile(personal); err != nil || string(data) != "preserve sibling\n" {
		t.Fatal("unrelated direct-install data changed", err)
	}
}
