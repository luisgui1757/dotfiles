package installer

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Native boundaries of manager-delta retirement. The process, User PATH and
// application inventory queries are read-only and run on their own OS. Package
// lifecycles mutate the real package database and run only on explicitly
// opted-in disposable runners.

func legacyNativeDriver(t *testing.T) *legacyRetirementDriver {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "worker")
	d := &legacyRetirementDriver{Directory: filepath.Join(root, "retirement"), WorkerDirectory: worker, Folders: ConfigFolders{Home: root},
		Normal:      Controller{StatePath: filepath.Join(root, "normal", "state.json"), Home: root},
		Platform:    NativePlatform{Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}},
		Environment: legacyRuntimeEnvironment{Username: "fixture", Root: runtime.GOOS != "windows" && os.Geteuid() == 0, Programs: map[string]string{}}, Query: queryLegacyRuntime}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		reply, err := executeNativeCommand(worker, command)
		if err != nil {
			return nil, err
		}
		if !legacyReplySucceeded(reply) {
			return reply.Output, &nativeCommandError{Message: reply.Error, ExitCode: reply.ExitCode}
		}
		return reply.Output, nil
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	d.Environment.Username = account.Username[strings.LastIndex(account.Username, `\`)+1:]
	if runtime.GOOS == "windows" {
		d.Platform.WindowsPowerShell, err = DiscoverWindowsPowerShell()
		if err != nil {
			t.Fatal(err)
		}
	}
	git, err := discoverLegacyGit(d.Normal, d.Platform)
	if err != nil {
		t.Fatal(err)
	}
	if git != "" {
		d.Environment.Programs["git"] = git
	}
	return d
}

func legacyNativePackages(t *testing.T, goos string) {
	t.Helper()
	if runtime.GOOS != goos || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_LEGACY_MANAGERS") != "1" {
		t.Skip("set DOTFILES_TEST_NATIVE_PACKAGES=1 and DOTFILES_TEST_LEGACY_MANAGERS=1 on a disposable " + goos + " runner")
	}
}

func runLegacyNativeSteps(t *testing.T, d *legacyRetirementDriver, scope legacyRetirementScope) {
	t.Helper()
	if scope.Blocked != "" || len(scope.Steps) == 0 {
		t.Fatal("nothing approvable was planned", scope)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	for i, step := range scope.Steps {
		if err := d.executeStep(context.Background(), intent, i, false); err != nil {
			t.Fatal(step.Label, err)
		}
	}
}

// A running copy of a targeted executable blocks retirement, whether it is
// found by its directory or by its image name, and stops blocking on exit.
func TestLegacyWindowsProcessGateObservesRunningExecutables(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows process inventory")
	}
	d := legacyNativeDriver(t)
	system, err := filepath.Abs(filepath.Join(os.Getenv("SystemRoot"), "System32", "PING.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "retired tool ü")
	data, err := os.ReadFile(system)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(directory, "bin", "retired-ping.exe"), string(data))
	child := exec.Command(filepath.Join(directory, "bin", "retired-ping.exe"), "-n", "120", "127.0.0.1")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	}()
	pid := strconv.Itoa(child.Process.Pid)
	for _, request := range [][2][]string{{{directory}, nil}, {nil, {"retired-ping"}}} {
		gate, err := legacyProcessGate(request[0], request[1])
		if err != nil {
			t.Fatal(err)
		}
		value, err := d.gateValue(context.Background(), gate)
		if err != nil || !strings.Contains(value, pid+" ") {
			t.Fatal("the running targeted executable was not reported", request, value, err)
		}
	}
	gate, err := legacyProcessGate([]string{filepath.Join(t.TempDir(), "unrelated")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := d.gateValue(context.Background(), gate); err != nil || value != "" {
		t.Fatal("unrelated or protected system processes blocked retirement", value, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	stopped = true
	gate, _ = legacyProcessGate([]string{directory}, []string{"retired-ping"})
	deadline := time.Now().Add(30 * time.Second)
	for {
		value, err := d.gateValue(context.Background(), gate)
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("an exited process still blocks", value)
		}
		time.Sleep(time.Second)
	}
}

func TestLegacyWindowsUserPathQueryIsReadOnly(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows User PATH")
	}
	d := legacyNativeDriver(t)
	value, err := d.userPathEntry(context.Background(), filepath.Join(t.TempDir(), "never added"))
	if err != nil || value != "absent" {
		t.Fatal("an absent User PATH entry was not reported as absent", value, err)
	}
}

// Read-only native inventories on whichever managers this host has.
func TestLegacyWindowsManagerInventoriesAreReadable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows package managers")
	}
	d := legacyNativeDriver(t)
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	d.Folders.LocalAppData = folders.LocalAppData
	checked := 0
	if d.wingetProgram() != "" {
		if _, err := d.wingetInventory(context.Background(), true); err != nil {
			t.Fatal("winget export could not be read", err)
		}
		checked++
	}
	if root := chocolateyRoot(); root != "" && !legacyAbsent(filepath.Join(root, "choco.exe")) {
		if _, err := legacyChocoInventory(root); err != nil {
			t.Fatal("Chocolatey's library could not be read", err)
		}
		if _, err := legacyChocoEnabledSources(root); err != nil {
			t.Fatal("Chocolatey's sources could not be read", err)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("this host has neither winget nor Chocolatey")
	}
}

// The released `choco install hyperfine -y`, then retirement through Chocolatey's
// own exact uninstall with provenance, consumer and process gates.
func TestLegacyChocolateyDeltaLifecycle(t *testing.T) {
	legacyNativePackages(t, "windows")
	root := chocolateyRoot()
	program := filepath.Join(root, "choco.exe")
	if root == "" || legacyAbsent(program) {
		t.Fatal("the opted-in Windows runner must provide Chocolatey")
	}
	if !legacyAbsent(filepath.Join(root, "lib", "hyperfine")) {
		t.Fatal("the disposable runner already has Chocolatey hyperfine; its ownership cannot be fixture evidence")
	}
	if output, err := exec.Command(program, "install", "hyperfine", "-y", "--no-progress").CombinedOutput(); err != nil {
		t.Fatalf("released Chocolatey install failed: %v %s", err, output)
	}
	d := legacyNativeDriver(t)
	scope := legacyRetirementScope{}
	if found, err := d.planChocoPackage(context.Background(), &scope, deltaByKey(t, "hyperfine")); err != nil || !found || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	if !legacyAbsent(filepath.Join(root, "lib", "hyperfine")) {
		t.Fatal("Chocolatey hyperfine remains after retirement")
	}
}

// The released `winget install --id sharkdp.hyperfine -e`, then retirement through
// winget's exact uninstall of the same identity and scope.
func TestLegacyWingetDeltaLifecycle(t *testing.T) {
	legacyNativePackages(t, "windows")
	d := legacyNativeDriver(t)
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	d.Folders.LocalAppData = folders.LocalAppData
	program := d.wingetProgram()
	if program == "" {
		t.Fatal("the opted-in Windows runner must provide winget")
	}
	if value, err := d.wingetGate(context.Background(), "sharkdp.hyperfine"); err != nil || value != "absent" {
		t.Fatal("the disposable runner already has winget hyperfine; its ownership cannot be fixture evidence", value, err)
	}
	if output, err := exec.Command(program, "install", "--id", "sharkdp.hyperfine", "-e", "--source", "winget", "--accept-source-agreements", "--accept-package-agreements", "--silent", "--disable-interactivity").CombinedOutput(); err != nil {
		t.Fatalf("released winget install failed: %v %s", err, output)
	}
	// The absence precondition populated the short-lived preview inventory.
	// Observe this external installation afresh before planning its retirement.
	if value, err := d.wingetGate(context.Background(), "sharkdp.hyperfine"); err != nil || value == "absent" {
		t.Fatal("winget did not register the installed fixture package", value, err)
	}
	scope := legacyRetirementScope{}
	if found, err := d.planWingetPackage(context.Background(), &scope, deltaByKey(t, "hyperfine")); err != nil || !found || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	if value, err := d.wingetGate(context.Background(), "sharkdp.hyperfine"); err != nil || value != "absent" {
		t.Fatal("winget hyperfine remains after retirement", value, err)
	}
}

func TestLegacyMacApplicationInventoryIsNative(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("DOTFILES_TEST_NATIVE_INVENTORY") != "1" && os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("set DOTFILES_TEST_NATIVE_INVENTORY=1 on macOS to query NSWorkspace")
	}
	d := legacyNativeDriver(t)
	if state, err := d.macApplicationState(context.Background(), "com.apple.finder"); err != nil || state != "running" {
		t.Fatal("Finder was not reported running", state, err)
	}
	if state, err := d.macApplicationState(context.Background(), "invalid.dotfiles.fixture.application"); err != nil || state != "not-running" {
		t.Fatal("an absent application was not reported as not running", state, err)
	}
}

// The released `brew install hyperfine`, then Homebrew's own uninstall of the
// requested formula with identity, consumer and query-Git gates.
func TestLegacyHomebrewFormulaDeltaLifecycle(t *testing.T) {
	legacyNativePackages(t, "darwin")
	d := legacyNativeDriver(t)
	location, err := discoverHomebrew(context.Background(), "/opt/homebrew/bin/brew")
	if err != nil || location == nil {
		t.Fatal("the opted-in macOS runner must provide Homebrew", err)
	}
	d.Platform.Homebrew = location
	if !legacyAbsent(filepath.Join(location.Cellar, "hyperfine")) {
		t.Fatal("the disposable runner already has hyperfine; its ownership cannot be fixture evidence")
	}
	install := exec.Command(location.Program, "install", "--formula", "hyperfine")
	install.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_INSTALL_CLEANUP=1")
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("released Homebrew install failed: %v %s", err, output)
	}
	scope := legacyRetirementScope{}
	if found, err := d.planBrewPackages(context.Background(), &scope, deltaByKey(t, "hyperfine")); err != nil || !found || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	if !legacyAbsent(filepath.Join(location.Cellar, "hyperfine")) {
		t.Fatal("hyperfine remains after retirement")
	}
}

// The released `apt-get install ./wezterm.deb` from the pinned .deb, then
// dpkg's own --remove after version, executable and consumer proof.
func TestLegacyWezTermDebDeltaLifecycle(t *testing.T) {
	legacyNativePackages(t, "linux")
	if runtime.GOARCH != "amd64" {
		t.Skip("the released WezTerm .deb is amd64-only")
	}
	d := legacyNativeDriver(t)
	if identity, _, err := d.debIdentity(context.Background(), "wezterm"); err != nil || identity != "absent" {
		t.Fatal("the disposable runner already has WezTerm; its ownership cannot be fixture evidence", identity, err)
	}
	pin := legacyDebPins[0]
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Get(pin.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 256<<20))
	response.Body.Close()
	if err != nil || legacyHash(data) != pin.Archive {
		t.Fatal("the pinned WezTerm .deb could not be verified", err)
	}
	deb := filepath.Join(t.TempDir(), "wezterm.deb")
	if err := os.WriteFile(deb, data, 0644); err != nil {
		t.Fatal(err)
	}
	install := []string{"apt-get", "install", "-y", deb}
	if !d.Environment.Root {
		install = append([]string{"sudo", "-n"}, install...)
	}
	command := exec.Command(install[0], install[1:]...)
	command.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("released apt-get install failed: %v %s", err, output)
	}
	scope := legacyRetirementScope{}
	if err := d.planDebPackage(context.Background(), &scope, deltaByKey(t, "wezterm")); err != nil || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	runLegacyNativeSteps(t, d, scope)
	if identity, _, err := d.debIdentity(context.Background(), "wezterm"); err != nil || identity != "absent" {
		t.Fatal("WezTerm remains installed after retirement", identity, err)
	}
}

// dpkg's real database through the same read-only queries retirement uses.
func TestLegacyDpkgQueriesReadTheNativeDatabase(t *testing.T) {
	if runtime.GOOS != "linux" || legacyAbsent("/usr/bin/dpkg-query") {
		t.Skip("native dpkg database")
	}
	d := legacyNativeDriver(t)
	states, err := d.dpkgPackages(context.Background(), "dpkg")
	if err != nil || len(states) != 1 || states[0].Package != "dpkg" || !strings.HasPrefix(states[0].Status, "ii") {
		t.Fatal("dpkg's own installed record was not read", states, err)
	}
	if identity, pin, err := d.debIdentity(context.Background(), "dotfiles-fixture-never-installed"); err != nil || identity != "absent" || pin != nil {
		t.Fatal("an unknown package was not absent", identity, err)
	}
	consumers, err := d.debConsumers(context.Background(), "dpkg")
	if err != nil {
		t.Fatal("the native dependency inventory could not be parsed", err)
	}
	t.Log("installed packages depending on dpkg:", len(consumers))
}
