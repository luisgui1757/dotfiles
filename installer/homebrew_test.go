package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func homebrewProcessFixture(t *testing.T) (HomebrewLocation, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew's POSIX process boundary")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "native packages with 'quotes'")
	location := HomebrewLocation{Program: filepath.Join(root, "bin", "brew"), Prefix: root, Cellar: filepath.Join(root, "Cellar")}
	log := filepath.Join(root, "calls")
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$*\" >>" + shellLiteral(log) + "\n" + `case "$*" in
--prefix) printf '%s\n' ` + shellLiteral(root) + `;;
--cellar) printf '%s\n' ` + shellLiteral(location.Cellar) + `;;
--version) printf 'Homebrew fixture\n';;
'info --json=v2 --installed') printf '%s\n' '{"formulae":[{"name":"git","full_name":"git","linked_keg":"1.0","installed":[{"version":"1.0","installed_on_request":true,"runtime_dependencies":[]}]}],"casks":[]}';;
'linkage --test git') test "${HOMEBREW_DEV_CMD_RUN-}" = 1; test "${HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK-}" = '';;
*) printf 'unexpected fixture command\n' >&2; exit 2;;
esac
`
	for path, contents := range map[string]string{location.Program: script, filepath.Join(root, "bin", "git"): "#!/bin/sh\nprintf 'git version fixture\\n'\n"} {
		writeConfigFixture(t, path, contents)
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return location, log
}

func TestHomebrewDiscoveryAndInspectionUseActualProcessPaths(t *testing.T) {
	location, _ := homebrewProcessFixture(t)
	got, err := inspectHomebrew(context.Background(), location.Program)
	if err != nil || *got != location {
		t.Fatal("provider paths were inferred or lost quoting", got, err)
	}
	t.Setenv("HOMEBREW_DEV_CMD_RUN", "parent-value")
	t.Setenv("HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK", "1")
	if _, err := location.query(context.Background(), false, "brew", nil, "linkage", "--test", "git"); err != nil {
		t.Fatal("native inspection did not apply child-only maintenance controls", err)
	}
	if os.Getenv("HOMEBREW_DEV_CMD_RUN") != "parent-value" || os.Getenv("HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK") != "1" {
		t.Fatal("inspection changed its caller's environment")
	}
	if _, err := location.query(context.Background(), true, "brew", nil, "--version"); err == nil {
		t.Fatal("inspection allowed elevation")
	}
	if _, err := location.query(context.Background(), false, "brew", []byte("input"), "--version"); err == nil {
		t.Fatal("inspection accepted process input")
	}
}

func TestHomebrewDiscoveryReusesBootstrapBeforeShellPATHRefresh(t *testing.T) {
	location, _ := homebrewProcessFixture(t)
	t.Setenv("PATH", t.TempDir())
	got, err := discoverHomebrew(context.Background(), location.Program)
	if err != nil || got == nil || *got != location {
		t.Fatal("an existing bootstrap was hidden by stale shell PATH", got, err)
	}
	if err := os.Remove(location.Program); err != nil {
		t.Fatal(err)
	}
	if got, err := discoverHomebrew(context.Background(), location.Program); err != nil || got != nil {
		t.Fatal("missing bootstrap candidate was not observed as absent", got, err)
	}
}

func TestHomebrewConnectionKeepsConstructionPureAndSelectionScoped(t *testing.T) {
	location, log := homebrewProcessFixture(t)
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	platform := NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}, Homebrew: &location}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config")}
	c, err := NewNativeController(repository, "homebrew-fixture", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("constructor invoked the package manager", err)
	}
	if _, err := os.Stat(filepath.Join(home, "state")); !os.IsNotExist(err) {
		t.Fatal("constructor initialized install state", err)
	}
	native := c.Driver.(*NativeDriver)
	for _, selected := range [][]string{{"tool.git"}, {"tool.tmux"}, {"tool.starship"}, {}} {
		bound, err := native.ForSelection(selected, State{})
		if err != nil {
			t.Fatal(err)
		}
		want := len(selected) > 0 && selected[0] != "tool.starship"
		loader := bound.(*NativeDriver).Profiles.Targets["integration.shells"][0].Script
		if strings.Contains(loader, shellLiteral(filepath.Join(location.Prefix, "bin"))) != want {
			t.Fatal("native shell command paths ignored selection", selected, loader)
		}
	}
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"gh-dash"}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, operation := range plan.Operations {
		if operation.Resource == "tool.git" {
			found = true
			if operation.Action != "keep" || !operation.Observed.Healthy || operation.Observed.Provider != "homebrew-formula" {
				t.Fatal("pre-existing Git was not checked and reused", operation)
			}
		}
	}
	if !found {
		t.Fatal("dashboard preview did not include native Git")
	}
	if _, err := os.Stat(filepath.Join(home, "state")); !os.IsNotExist(err) {
		t.Fatal("native preview wrote install state", err)
	}
}

func TestHomebrewDiscoveryFailureDoesNotBlockIndependentArchives(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	d := &NativeDriver{Catalog: catalog, Context: Context{OS: "darwin"}, HomebrewIssue: "cannot inspect fixture Homebrew"}
	for _, id := range []string{"tool.git", "tool.tmux", "infra.homebrew"} {
		resource, _ := catalog.Resource(id)
		observed, err := d.Observe(context.Background(), resource, Receipt{After: Observation{Preserved: []string{"personal"}}})
		if err != nil || !observed.Unknown || observed.Pending != d.HomebrewIssue || len(observed.Preserved) != 1 {
			t.Fatal("uninspectable native provider lost its local diagnostic or retained data", id, observed, err)
		}
	}
	starship, _ := catalog.Resource("tool.starship")
	if provider, err := d.provider(starship); err != nil || provider != d.Archives {
		t.Fatal("Homebrew failure changed the independent archive route", err)
	}
}
