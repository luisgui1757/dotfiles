package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Run last in the hosted Windows core job, after the public archive lifecycle
// has removed its selections. HOME does not redirect Windows known folders.
// This test deliberately leaves its historical migration receipt and benign
// personal files in that disposable account; rerunning requires a fresh runner.
func TestNativeWindowsPublicMigrationSetupLifecycle(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_TEST_PUBLIC_WINDOWS_MIGRATION") != "1" {
		t.Skip("actual known-folder writes require an explicitly opted-in disposable GitHub-hosted Windows runner")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Match installedMachineFixture's consumer locations for wrapper invocations.
	// Documents, LocalAppData, the user home and Claude remain actual known paths.
	for key, value := range map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":       filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":      filepath.Join(home, ".local", "state"),
		"CODEX_HOME":          filepath.Join(home, ".codex"),
		"PI_CODING_AGENT_DIR": filepath.Join(home, ".pi", "agent"), "ZDOTDIR": home,
	} {
		t.Setenv(key, value)
	}
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	target := Context{OS: runtime.GOOS, Arch: runtime.GOARCH}
	stateDirectory, err := DiscoverStateDirectory(target, folders)
	if err != nil {
		t.Fatal(err)
	}
	assertEmptySetup := func() {
		t.Helper()
		state, err := LoadState(filepath.Join(stateDirectory, "state.json"), folders.Home)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Selected) != 0 || len(state.Keep) != 0 || state.Transaction != nil {
			t.Fatal("public migration requires empty completed setup state; do not discard existing selections or recovery")
		}
		for id, receipt := range state.Receipts {
			if receipt.Ownership != "reused" || receipt.Status != "removed" && receipt.Status != "ready" {
				t.Fatalf("public migration refuses active or uncertain ownership: %s (%s, %s)", id, receipt.Ownership, receipt.Status)
			}
		}
	}
	assertEmptySetup()
	migration, err := NewLegacyMigration(repository, "inspection", stateDirectory, target, folders)
	if err != nil {
		t.Fatal(err)
	}
	assertAbsent := func(path string) {
		t.Helper()
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("fixture requires an absent target, refusing replacement: %s: %v", path, err)
		}
	}
	// Bootstrap cache and completed setup receipts may exist. Migration evidence
	// and each personal target must be fresh before this fixture writes anything.
	assertAbsent(migration.driver.Directory)
	for _, profile := range migration.driver.profiles {
		assertAbsent(migration.driver.destination(profile))
	}
	platform := NativePlatform{Context: target, Sentinel: SentinelLocations{
		CodexHome: os.Getenv("CODEX_HOME"), ConfigHome: os.Getenv("XDG_CONFIG_HOME"), PiAgentDir: os.Getenv("PI_CODING_AGENT_DIR"),
	}}
	c, err := NewNativeController(repository, "inspection", stateDirectory, platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	driver := c.Driver.(*NativeDriver)
	shellTargets := driver.Profiles.Targets["integration.shells"]
	policyTargets := driver.Profiles.Targets[sentinelResource]
	for _, targets := range [][]ProfileTarget{shellTargets, policyTargets} {
		for _, target := range targets {
			assertAbsent(target.Path)
		}
	}
	starshipTargets, err := driver.Configurations.Manifest.Resolve(c.Catalog, target, folders, repository, "config.starship")
	if err != nil || len(starshipTargets) != 1 {
		t.Fatal("resolve actual Starship destination", err)
	}
	assertAbsent(starshipTargets[0].Destination)

	powershell, err := DiscoverWindowsPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	versionContext, cancelVersion := context.WithTimeout(context.Background(), 10*time.Second)
	version, versionErr := exec.CommandContext(versionContext, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "$PSVersionTable.PSVersion.ToString()").CombinedOutput()
	cancelVersion()
	if versionErr != nil || !strings.HasPrefix(strings.TrimSpace(string(version)), "5.1.") {
		t.Fatalf("public wrappers must run under system Windows PowerShell 5.1: %s %v", version, versionErr)
	}
	invokeWrapper := func(name string, request Request) Result {
		t.Helper()
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(repository, name), "machine")
		cmd.Stdin = bytes.NewReader(data)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			if ctx.Err() != nil && strings.Contains(stderr.String(), "Bootstrap download: started") && !strings.Contains(stderr.String(), "Bootstrap download: completed") {
				diagnoseWindowsBootstrapDownload(t, powershell, repository)
			}
			t.Fatalf("public %s: %s %s %v (context: %v)", name, output, stderr.String(), err, ctx.Err())
		}
		t.Logf("public %s bootstrap phases: %s", name, stderr.String())
		var result Result
		if err := Decode(output, &result); err != nil {
			t.Fatalf("public %s machine response: %s %s %v", name, output, stderr.String(), err)
		}
		return result
	}
	if result := invokeWrapper("setup.ps1", Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("public setup wrapper did not accept empty completed state", result)
	}

	profile := migration.driver.destination(migration.driver.profile("legacy.powershell"))
	legacy := "# personal legacy profile; retained for explicit review\r\n$global:DotfilesMigrationFixture = 'preserve-only'\r\n"
	writeConfigFixture(t, profile, legacy)
	assertBytes := func(path, wanted string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != wanted {
			t.Fatalf("fixture bytes changed at %s: %v", path, err)
		}
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"legacy.powershell"}, Adopt: []string{"legacy.powershell"}}
	preview := invokeWrapper("migrate.ps1", request)
	if preview.Status != "preview" || operation(t, preview.Plan, "legacy.powershell").Action != "adopt" {
		t.Fatal("public migrate wrapper did not request explicit adoption", preview)
	}
	assertBytes(profile, legacy)
	assertAbsent(migration.driver.Directory)

	invoke := installedMachineFixture(t, repository, home, 3*time.Minute)
	approve := func(request Request) Result {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" || preview.Plan.ID == "" {
			t.Fatal("compiled public command did not return an approval plan", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		result := invoke(request)
		if result.Status != "ready" {
			t.Fatal("approved public command did not complete", result)
		}
		return result
	}
	t.Setenv("DOTFILES_ENTRYPOINT", "migrate")
	// Build a fresh approval with this executable's source identity. A bootstrap
	// preview is read-only evidence, not approval for a differently built binary.
	result := approve(request)
	state, err := LoadState(migration.Controller.StatePath, folders.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["legacy.powershell"]
	if receipt.Status != "ready" || receipt.Ownership != "created" || !receipt.Adopted || len(receipt.After.Preserved) != 2 {
		t.Fatal("migration lacks adopted completion and two original preservation records", receipt)
	}
	assertBackups := func() {
		t.Helper()
		for _, saved := range receipt.After.Preserved {
			assertBytes(saved, legacy)
		}
	}
	assertBackups()
	for _, saved := range receipt.After.Preserved {
		if !strings.Contains(result.Message, saved) {
			t.Fatal("migration did not disclose a preserved original", saved)
		}
	}
	detached, err := os.ReadFile(profile)
	if err != nil || !bytes.Contains(detached, []byte("Legacy dotfiles profile preserved")) || bytes.Contains(detached, []byte("$global:DotfilesMigrationFixture")) {
		t.Fatal("migration copied unknown personal commands into the fresh profile", err)
	}
	laterProfile := "# new personal CurrentHost profile after migration\r\n"
	writeConfigFixture(t, profile, laterProfile)
	// The new shell owner uses AllHosts profile.ps1, independently of the
	// released CurrentHost Microsoft.PowerShell_profile.ps1 detached above.
	allHosts, err := bindConfigDestination(filepath.Join(folders.Documents, "PowerShell", "profile.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	personalShell := "# personal AllHosts preferences\r\n"
	writeConfigFixture(t, allHosts, personalShell)
	policy := policyTargets[0].Path
	priorPolicy := "# personal instructions\n<!-- AGENT-RULES:BEGIN -->\nOld policy\n<!-- AGENT-RULES:END -->\n"
	writeConfigFixture(t, policy, priorPolicy)

	t.Setenv("DOTFILES_ENTRYPOINT", "setup")
	approve(Request{Schema: 1, Mode: "apply", Selected: []string{"starship", "sentinel"}, Adopt: []string{sentinelResource}})
	installed, err := LoadState(c.StatePath, folders.Home)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tool.starship", "config.starship", "integration.shells", sentinelResource} {
		if receipt := installed.Receipts[id]; receipt.Status != "ready" || receipt.Ownership != "created" {
			t.Fatal("setup did not complete an owned selection", id, receipt)
		}
	}
	if !installed.Receipts[sentinelResource].Adopted {
		t.Fatal("setup did not record the explicit prior-policy adoption")
	}
	assertInstalled := func() {
		t.Helper()
		for _, target := range shellTargets {
			data, err := os.ReadFile(target.Path)
			if err != nil || !bytes.Contains(data, []byte("# >>> dotfiles:integration.shells >>>")) {
				t.Fatal("scoped shell attachment was not published", target.Path, err)
			}
		}
		for _, target := range policyTargets {
			data, err := os.ReadFile(target.Path)
			if err != nil || !bytes.Contains(data, []byte("Operating Contract")) {
				t.Fatal("actual Sentinel policy was not published", target.Path, err)
			}
		}
		assertBytes(profile, laterProfile)
	}
	assertInstalled()
	laterShell := "# personal AllHosts addition after setup\r\n"
	f, err := os.OpenFile(allHosts, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.WriteString(laterShell)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		t.Fatal(err)
	}
	approve(Request{Schema: 1, Mode: "update"})
	assertInstalled()
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal(result)
	}
	activeShell, err := os.ReadFile(allHosts)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES_ENTRYPOINT", "migrate")
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("historical migration interfered with active setup", result)
	}
	assertBytes(allHosts, string(activeShell))
	assertBytes(profile, laterProfile)
	assertBackups()

	t.Setenv("DOTFILES_ENTRYPOINT", "setup")
	approve(Request{Schema: 1, Mode: "apply", Selected: []string{}})
	assertBytes(allHosts, personalShell+laterShell)
	assertBytes(profile, laterProfile)
	assertBytes(policy, priorPolicy)
	for _, target := range shellTargets {
		if target.Path != allHosts {
			assertAbsent(target.Path)
		}
	}
	for _, target := range policyTargets[1:] {
		assertAbsent(target.Path)
	}
	assertAbsent(starshipTargets[0].Destination)
	starship, err := driver.Archives.CommandPath("tool.starship", "starship")
	if err != nil {
		t.Fatal(err)
	}
	assertAbsent(starship)
	assertEmptySetup()
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("setup removal did not leave healthy restored baselines", result)
	}
	t.Setenv("DOTFILES_ENTRYPOINT", "migrate")
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("historical migration failed after normal removal", result)
	}
	assertBytes(profile, laterProfile)
	assertBytes(allHosts, personalShell+laterShell)
	assertBytes(policy, priorPolicy)
	assertBackups()
}
