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
		if slices.Contains([]string{"git", "config.git", "integration.git"}, resource.ID) {
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

func TestGitLegacyReceiptRequiresRemovalBeforeScopedDefaults(t *testing.T) {
	c, source, target := gitSettingsFixture(t)
	original := "[user]\n\tname = Personal baseline\n"
	writeConfigFixture(t, target, original)
	old := legacyGitController(c)
	dispatchApproved(t, old, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil {
		t.Fatal(err)
	}
	blocked := false
	for _, operation := range preview.Plan.Operations {
		if operation.Resource == gitSettingsResource && (operation.Action == "blocked" || operation.Action == "pending") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatal("new defaults competed with the previous whole-file owner", preview.Plan)
	}
	if runtime.GOOS != "windows" {
		m, err := NewLegacyMigration(filepath.Dir(filepath.Dir(source)), "git-settings-fixture", filepath.Dir(c.StatePath), c.Context, ConfigFolders{Home: c.Home, Config: filepath.Dir(filepath.Dir(target))})
		if err != nil {
			t.Fatal(err)
		}
		r, _ := m.Controller.Catalog.Resource("legacy.git-config")
		observation, err := m.driver.Observe(context.Background(), r, Receipt{})
		if err != nil || observation.ApplyBlocked == "" {
			t.Fatal("legacy migration would detach setup's owned Git link", observation, err)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	got, err := os.ReadFile(target)
	if err != nil || string(got) != original {
		t.Fatal("legacy removal did not restore the first personal baseline", string(got), err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	got, err = os.ReadFile(target)
	if err != nil || string(got) != original {
		t.Fatal("new integration lost the legacy personal baseline", string(got), err)
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

func TestLegacyGitLinkMigrationPreservesIdentityAndCheckout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("released Windows Git configuration used a copy; POSIX live-link migration")
	}
	c, source, target := gitSettingsFixture(t)
	old := "[user]\n\temail = fixture@example.invalid\n[fetch]\n\tprune = false\n"
	legacySource := filepath.Join(filepath.Dir(filepath.Dir(source)), "home", "dot_config", "git", "config")
	writeConfigFixture(t, legacySource, old)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(legacySource, target); err != nil {
		t.Fatal(err)
	}
	folders := c.Driver.(*NativeDriver).Configurations.Folders
	m, err := NewLegacyMigration(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), c.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	result := approveLegacy(t, m, legacyRequest("legacy.git-config"))
	if result.Status != "ready" {
		t.Fatal(result)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "--get", "pull.rebase"); got != "true" {
		t.Fatal("released defaults shadowed the new rebase default", got)
	}
	gitSettingsCommand(t, c.Home, c.Home, "config", "--global", "user.email", "updated@example.invalid")
	for _, mode := range []string{"check", "update"} {
		if mode == "check" {
			result, err = m.Dispatch(context.Background(), Request{Schema: 1, Mode: mode})
			if err != nil || result.Status != "ready" {
				t.Fatal("historical migration replayed over new Git settings", result, err)
			}
		} else {
			dispatchApproved(t, c, Request{Schema: 1, Mode: mode})
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if got := gitSettingsCommand(t, c.Home, c.Home, "config", "user.email"); got != "updated@example.invalid" {
		t.Fatal("migration or removal lost personal identity", got)
	}
	data, err := os.ReadFile(legacySource)
	if err != nil || string(data) != old {
		t.Fatal("migration changed the old checkout", err)
	}
}

func TestLegacyGitMigrationLeavesPersonalSyncLinkAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX personal symlink fixture")
	}
	c, source, target := gitSettingsFixture(t)
	personal := filepath.Join(c.Home, "personal-sync", "git-config")
	writeConfigFixture(t, personal, "[user]\n\tname = Personal sync\n")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(personal, target); err != nil {
		t.Fatal(err)
	}
	folders := c.Driver.(*NativeDriver).Configurations.Folders
	m, err := NewLegacyMigration(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), c.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := m.Controller.Catalog.Resource("legacy.git-config")
	o, err := m.driver.Observe(context.Background(), r, Receipt{})
	if err != nil || o.Present || !o.Healthy {
		t.Fatal("personal sync link was offered as legacy dotfiles", o, err)
	}
	if got, err := os.Readlink(target); err != nil || got != personal {
		t.Fatal("personal sync link changed", got, err)
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
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	parent := filepath.Dir(target)
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

func TestGitRetirementPreservesChangedWholeFileAndFirstBaseline(t *testing.T) {
	for _, ownership := range []string{"created", "uncertain"} {
		t.Run(ownership, func(t *testing.T) {
			c, _, target := gitSettingsFixture(t)
			original := "[user]\n\tname = Original personal baseline\n"
			writeConfigFixture(t, target, original)
			old := legacyGitController(c)
			dispatchApproved(t, old, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
			// Replace the old POSIX link, or edit the Windows copy, as a personal
			// configuration manager might. Preserve this separately from baseline.
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			changed := "[user]\n\temail = later@example.invalid\n"
			writeConfigFixture(t, target, changed)
			if ownership == "uncertain" {
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				receipt := state.Receipts["config.git"]
				receipt.Ownership, receipt.Status = "uncertain", "needs-action"
				state.Receipts["config.git"] = receipt
				if err := SaveState(c.StatePath, state); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
			preview, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range preview.Plan.Operations {
				if op.Resource == "config.git" && op.Action != "retain" {
					t.Fatal("changed file removed without per-item choice", op)
				}
			}
			request.RemoveShared = []string{"config.git"}
			var result Result
			if ownership == "created" {
				result = runController(t, c, []string{"remove"}, []string{"git"}, []string{"config.git"})
			} else {
				result = dispatchApproved(t, c, request)
			}
			actual, err := os.ReadFile(target)
			if err != nil || string(actual) != original {
				t.Fatal("first baseline was not restored", string(actual), result, err)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			preserved := false
			for _, path := range state.Receipts["config.git"].After.Preserved {
				data, err := os.ReadFile(path)
				if err == nil && string(data) == changed {
					preserved = true
				}
			}
			if !preserved {
				t.Fatal("changed file was not preserved separately", state.Receipts["config.git"])
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			actual, err = os.ReadFile(target)
			if err != nil || string(actual) != original {
				t.Fatal("scoped defaults changed original baseline", err)
			}
		})
	}
}

func TestGitLegacyRetirementResumesBeforeProviderPublication(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	original := "[user]\n\tname = Original baseline\n"
	writeConfigFixture(t, target, original)
	old := legacyGitController(c)
	dispatchApproved(t, old, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	id, err := digest(struct{ Plan, Resource, Action string }{plan.ID, "config.git", "remove"})
	if err != nil {
		t.Fatal(err)
	}
	journal := c.Driver.(*NativeDriver).Configurations.journalPath(id)
	// A filesystem publication error occurs after durable engine intent, before
	// any provider journal or target move. Removing the obstacle permits retry.
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not stop publication")
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan, request.Retry = "", true
	result := dispatchApproved(t, c, request)
	actual, err := os.ReadFile(target)
	if err != nil || string(actual) != original || result.Status != "ready" {
		t.Fatal("saved legacy removal could not resume before provider publication", result, string(actual), err)
	}
}
