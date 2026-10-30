package installer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Exercise the real application's Git configuration route without installing
// native packages on a contributor's machine. Git itself is a real subprocess.
func gitSettingsFixture(t *testing.T) (Controller, string, string) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, repository := filepath.Join(root, "home"), filepath.Join(root, "checkout with space ü")
	defaults, err := os.ReadFile(filepath.Join("..", "git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repository, "git", "config")
	writeConfigFixture(t, source, string(defaults))
	if runtime.GOOS == "windows" {
		fragment, err := os.ReadFile(filepath.Join("..", "windows-terminal", "settings.fragment.jsonc"))
		if err != nil {
			t.Fatal(err)
		}
		writeConfigFixture(t, filepath.Join(repository, "windows-terminal", "settings.fragment.jsonc"), string(fragment))
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming"), Documents: filepath.Join(home, "documents")}
	c, err := NewNativeController(repository, "git-settings-fixture", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{Schema: 1}
	for _, resource := range c.Catalog.Resources {
		if slices.Contains([]string{"git", "integration.git"}, resource.ID) {
			if !resource.Capability {
				resource.Requires = nil
			}
			catalog.Resources = append(catalog.Resources, resource)
		}
	}
	c.Catalog = catalog
	return c, source, filepath.Join(folders.Config, "git", "config")
}

func gitSettingsCommand(t *testing.T, home, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") && !slices.Contains([]string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"}, strings.ToUpper(key)) {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
	return strings.TrimSpace(string(output))
}

func TestGitGlobalWritesNeverModifyTheCheckout(t *testing.T) {
	c, source, target := gitSettingsFixture(t)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email", "fixture@example.invalid")
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("git config --global wrote personal settings into the checkout", err)
	}
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("Git's writable global configuration is not a regular file", info, err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("personal Git settings invalidated managed defaults", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email"); got != "fixture@example.invalid" {
		t.Fatal("removal lost personal Git settings", got)
	}
}

func TestGitDefaultsRebasePullAndPruneFetch(t *testing.T) {
	c, _, _ := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	for _, key := range []string{"pull.rebase", "fetch.prune"} {
		if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--bool", "--get", key); got != "true" {
			t.Fatal("missing Git default", key, got)
		}
	}
}

func TestGitRemovalWorksWhenDefaultsSourceIsMissing(t *testing.T) {
	c, source, _ := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email", "fixture@example.invalid")
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email"); got != "fixture@example.invalid" {
		t.Fatal(got)
	}
}

func TestGitLockFinalizationPreservesForeignLockFile(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "config")
	writeConfigFixture(t, target+".lock", "in-progress native Git write")
	journal := profileJournal{Resource: gitSettingsResource, Operation: strings.Repeat("a", 64), Entries: []profilePublication{{Path: target}}}
	if err := finishProfileSettingsLocks(root, journal); err != nil {
		t.Fatal("foreign Git lock prevented finalization", err)
	}
	got, err := os.ReadFile(target + ".lock")
	if err != nil || string(got) != "in-progress native Git write" {
		t.Fatal("finalization changed a foreign Git lock", string(got), err)
	}
}

func TestGitIncludeEditsRequireReviewAndRemainPreserved(t *testing.T) {
	c, source, target := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	personal := filepath.Join(c.Home, "extra.gitconfig")
	writeConfigFixture(t, personal, "[user]\n\temail = include@example.invalid\n")
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "--add", "include.path", personal)
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status == "ready" {
		t.Fatal("edited managed include was not detected", check, err)
	}
	edited, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	retained, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(edited, retained) {
		t.Fatal("removal did not preserve the edited include", err)
	}
	defaults, err := os.ReadFile(source)
	if err != nil || bytes.Contains(defaults, []byte("include@example.invalid")) {
		t.Fatal("include edit wrote into the checkout", err)
	}
}

func TestGitDefaultIncludePreservesPersonalPrecedence(t *testing.T) {
	for _, location := range []string{"xdg", "home"} {
		t.Run(location, func(t *testing.T) {
			c, _, target := gitSettingsFixture(t)
			personal := target
			if location == "home" {
				personal = filepath.Join(c.Home, ".gitconfig")
			}
			original := "[pull]\n\trebase = false\n[fetch]\n\tprune = false\n"
			writeConfigFixture(t, personal, original)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
			for _, key := range []string{"pull.rebase", "fetch.prune"} {
				if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--bool", "--get", key); got != "false" {
					t.Fatal("managed defaults overrode personal Git settings", key, got)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			got, err := os.ReadFile(personal)
			if err != nil || string(got) != original {
				t.Fatal("Git removal changed the original personal configuration", runtime.GOOS, err)
			}
		})
	}
}

func TestGitDivergentPullRebasesWithoutAMergeCommit(t *testing.T) {
	c, _, _ := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	remote, first, second := filepath.Join(c.Home, "remote.git"), filepath.Join(c.Home, "first"), filepath.Join(c.Home, "second")
	git := func(directory string, args ...string) string {
		return gitSettingsCommand(t, c.Home, directory, args...)
	}
	git(c.Home, "init", "--bare", "--initial-branch=main", remote)
	git(c.Home, "clone", remote, first)
	writeConfigFixture(t, filepath.Join(first, "base"), "base\n")
	git(first, "add", "base")
	git(first, "commit", "-m", "base")
	git(first, "push", "origin", "main")
	git(c.Home, "clone", remote, second)
	writeConfigFixture(t, filepath.Join(second, "local"), "local\n")
	git(second, "add", "local")
	git(second, "commit", "-m", "local")
	localBefore := git(second, "rev-parse", "HEAD")
	writeConfigFixture(t, filepath.Join(first, "remote"), "remote\n")
	git(first, "add", "remote")
	git(first, "commit", "-m", "remote")
	remoteHead := git(first, "rev-parse", "HEAD")
	git(first, "push", "origin", "main")
	git(second, "pull")
	if git(second, "rev-parse", "HEAD") == localBefore || git(second, "rev-parse", "HEAD^") != remoteHead || git(second, "rev-list", "--count", "--merges", "HEAD") != "0" {
		t.Fatal("divergent pull did not replay the local commit above the remote tip")
	}
	git(first, "push", "origin", "HEAD:temporary")
	git(second, "fetch")
	git(first, "push", "origin", "--delete", "temporary")
	git(second, "fetch")
	if git(second, "for-each-ref", "--format=%(refname)", "refs/remotes/origin/temporary") != "" {
		t.Fatal("fetch retained the deleted remote branch")
	}
}

func TestGitIncludePreservesBOMAndUnsetRestoresDefault(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	original := "\xef\xbb\xbf# personal\r\n"
	writeConfigFixture(t, target, original)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "pull.rebase", "false")
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--bool", "pull.rebase"); got != "false" {
		t.Fatal("personal override did not win", got)
	}
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "--unset", "pull.rebase")
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--bool", "pull.rebase"); got != "true" {
		t.Fatal("unsetting personal override did not reveal the default", got)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	got, err := os.ReadFile(target)
	if err != nil || !bytes.HasPrefix(got, []byte(original)) {
		t.Fatal("Git integration lost the original BOM or comments", err)
	}
}

func TestGitPublicationRespectsNativeApplicationLock(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"git"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, target+".lock", "owned by Git")
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("installer ignored Git's application lock")
	}
	data, err := os.ReadFile(target + ".lock")
	if err != nil || string(data) != "owned by Git" {
		t.Fatal("installer changed the native Git lock", err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("installer published while Git held its lock", err)
	}
}

func TestGitRemovalStaysAtApprovedPathAfterEnvironmentChanges(t *testing.T) {
	c, _, original := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email", "fixture@example.invalid")
	driver := c.Driver.(*NativeDriver)
	driver.shellSelection = nil
	moved := filepath.Join(c.Home, "different-xdg", "git", "config")
	driver.Profiles.Targets[gitSettingsResource][0].Path = moved
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, "path changed") {
		t.Fatal("changed discovery path was not explained", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(original)
	if err != nil || bytes.Contains(data, []byte("dotfiles:integration.git")) || !bytes.Contains(data, []byte("fixture@example.invalid")) {
		t.Fatal("removal failed to preserve personal data at its original target", string(data), err)
	}
	if _, err := os.Lstat(moved); !os.IsNotExist(err) {
		t.Fatal("removal wrote a new discovery path", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	data, err = os.ReadFile(moved)
	if err != nil || !bytes.Contains(data, []byte("dotfiles:integration.git")) {
		t.Fatal("explicit removal did not permit a fresh Git target", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(moved); !os.IsNotExist(err) {
		t.Fatal("new target did not retain its independent absent baseline", err)
	}
}

func TestGitRejectsUTF16WithoutChangingTheFile(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	original := []byte{0xff, 0xfe, '[', 0, 'x', 0, ']', 0, '\n', 0}
	writeConfigFixture(t, target, string(original))
	result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Operations) != 1 || result.Plan.Operations[0].Action != "pending" || !strings.Contains(result.Plan.Operations[0].Reason, "UTF-8") {
		t.Fatal("UTF-16 Git configuration was not blocked", result)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(original, got) {
		t.Fatal("invalid Git configuration changed", err)
	}
}

func TestGitNewWindowsConfigurationDoesNotSeedABOM(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	driver := c.Driver.(*NativeDriver)
	driver.shellSelection = nil
	driver.Profiles.Windows = true
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	data, err := os.ReadFile(target)
	if err != nil || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("new Git configuration inherited the PowerShell-only BOM", err)
	}
}

func TestGitRedirectedDirectoryCannotPublishIntoCheckout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory-link layout; Windows XDG overlap is covered separately")
	}
	c, source, target := gitSettingsFixture(t)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(source), filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil {
		t.Fatal(err)
	}
	blocked := false
	for _, op := range preview.Plan.Operations {
		if op.Resource == gitSettingsResource && (op.Action == "pending" || op.Action == "blocked") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatal("checkout redirect was allowed", preview.Plan)
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("checkout changed", err)
	}
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--get", "pull.rebase"); got != "true" {
		t.Fatal(got)
	}
}

func TestGitMissingBaselineDoesNotBlockIndependentSelection(t *testing.T) {
	c, _, _ := gitSettingsFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts[gitSettingsResource]
	d := c.Driver.(*NativeDriver).Profiles
	if err := os.Remove(d.baselinePath(receipt.Recovery)); err != nil {
		t.Fatal(err)
	}
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil {
		t.Fatal("Git evidence prevented an independent preview", err)
	}
	for _, op := range preview.Plan.Operations {
		if op.Resource == gitSettingsResource && (op.Action != "retain" || op.Observed.Pending == "") {
			t.Fatal("missing evidence did not remain local and protected", op)
		}
	}
}

func TestGitHistoricalLocksDoNotBlockAfterParentRedirect(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX redirected parent fixture")
	}
	c, _, target := gitSettingsFixture(t)
	// A personal Git folder pre-exists, so removal keeps it for the redirect.
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	moved := parent + "-moved"
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, parent); err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver).Profiles
	for _, mode := range []string{"apply", "abandon"} {
		if _, err := d.FinishTransaction(context.Background(), Plan{Mode: mode}, nil); err != nil {
			t.Fatal("historical Git journal blocked unrelated finalization", mode, err)
		}
	}
}
