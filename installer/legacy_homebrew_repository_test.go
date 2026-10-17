package installer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func legacyHomebrewRecordsFixture(t *testing.T) (*legacyRetirementDriver, string) {
	t.Helper()
	prefix, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"Cellar/git/2.56.0/INSTALL_RECEIPT.json":                     `{"installed_on_request":true}`,
		"Cellar/git/2.56.0/.brew/git.rb":                             "class Git < Formula; end\n",
		"Caskroom/firefox/.metadata/145/20261017/Casks/firefox.json": `{"artifacts":[{"command_wrapper":{"name":"firefox"}}]}`,
	} {
		path := filepath.Join(prefix, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	d := &legacyRetirementDriver{Folders: ConfigFolders{Home: prefix}, Platform: NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"},
		Homebrew: &HomebrewLocation{Program: filepath.Join(prefix, "nonfunctional-brew"), Prefix: prefix, Cellar: filepath.Join(prefix, "Cellar")}}}
	return d, prefix
}

func TestLegacyHomebrewPreservationDoesNotNeedOldRuby(t *testing.T) {
	d, _ := legacyHomebrewRecordsFixture(t)
	identity, err := d.brewPreservationIdentity(context.Background())
	if err != nil || !operationID.MatchString(identity) {
		t.Fatal("newer cask metadata must remain inspectable without old Brew", identity, err)
	}
	// Changing only the manager executable cannot change installed records.
	d.Platform.Homebrew.Program = filepath.Join(d.Platform.Homebrew.Prefix, "restored-brew")
	if after, err := d.brewPreservationIdentity(context.Background()); err != nil || after != identity {
		t.Fatal("manager conversion changed package identity", after, err)
	}
}

func TestLegacyHomebrewPreservationBindsInstallationRecords(t *testing.T) {
	for _, name := range []string{
		"Cellar/git/2.56.0/INSTALL_RECEIPT.json",
		"Cellar/git/2.56.0/.brew/git.rb",
		"Caskroom/firefox/.metadata/145/20261017/Casks/firefox.json",
	} {
		t.Run(name, func(t *testing.T) {
			d, prefix := legacyHomebrewRecordsFixture(t)
			before, err := d.brewPreservationIdentity(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(prefix, filepath.FromSlash(name)), []byte("changed installation record"), 0644); err != nil {
				t.Fatal(err)
			}
			if after, err := d.brewPreservationIdentity(context.Background()); err != nil || after == before {
				t.Fatal("changed installation record was not detected", after, err)
			}
		})
	}
}

func TestLegacyHomebrewPreservationBindsEmptyVersionMembership(t *testing.T) {
	d, prefix := legacyHomebrewRecordsFixture(t)
	before, err := d.brewPreservationIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(prefix, "Cellar", "git", "2.57.0"), 0755); err != nil {
		t.Fatal(err)
	}
	if after, err := d.brewPreservationIdentity(context.Background()); err != nil || after == before {
		t.Fatal("a newly introduced version was not detected", after, err)
	}
}

func legacyHomebrewTapFixture(t *testing.T) (*legacyRetirementDriver, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew Git inspection uses the POSIX process boundary")
	}
	d, prefix := legacyHomebrewRecordsFixture(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	d.Environment.Programs = map[string]string{"git": git}
	d.Query = queryLegacyRuntime
	tap := filepath.Join(prefix, "Library", "Taps", "example", "homebrew-personal")
	if err := os.MkdirAll(tap, 0755); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "--template=", "--quiet"},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "--quiet", "-m", "Fixture"},
	} {
		if _, err := d.git(context.Background(), tap, arguments...); err != nil {
			t.Fatal(err)
		}
	}
	return d, tap
}

func TestLegacyHomebrewPreservationBindsUntrackedTapContent(t *testing.T) {
	d, tap := legacyHomebrewTapFixture(t)
	path := filepath.Join(tap, "personal.rb")
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := d.brewPreservationIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if after, err := d.brewPreservationIdentity(context.Background()); err != nil || after == before {
		t.Fatal("same-name untracked tap changes were not detected", after, err)
	}
}

func TestLegacyHomebrewPreservationRefusesRedirectedTaps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew uses POSIX symlinks")
	}
	for _, name := range []string{"Library/Taps/example", "Library/Taps/example/homebrew-personal"} {
		t.Run(name, func(t *testing.T) {
			d, prefix := legacyHomebrewRecordsFixture(t)
			path := filepath.Join(prefix, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("/nix/store/personal-tap", path); err != nil {
				t.Fatal(err)
			}
			if _, err := d.brewPreservationIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "redirected") {
				t.Fatal("a tap dependent on the retired store must block conversion", err)
			}
			if target, err := os.Readlink(path); err != nil || target != "/nix/store/personal-tap" {
				t.Fatal("inspection changed a personal tap", target, err)
			}
		})
	}
}

func TestLegacyHomebrewRepositoryPreservesUnrecognizedGitDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew conversion uses the POSIX process boundary")
	}
	d, prefix := legacyHomebrewRecordsFixture(t)
	path := filepath.Join(prefix, ".git")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "config"), []byte("personal repository"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(prefix)
	if err != nil {
		t.Fatal(err)
	}
	_, err = queryLegacyRuntime(context.Background(), d.homebrewRepositoryCommand(prefix, strings.Repeat("a", 64)))
	if code, ok := legacyExitCode(err); !ok || code != 110 {
		t.Fatal("an unknown repository must block source restoration before fetch", err)
	}
	if after, err := snapshotConfig(prefix); err != nil || after != before {
		t.Fatal("source restoration changed an unrecognized repository", after, err)
	}
}

func TestLegacyHomebrewRepositoryRecoveryRejectsChangedConfiguration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Homebrew recovery uses native Darwin directory identity")
	}
	d, prefix := legacyHomebrewRecordsFixture(t)
	repository := filepath.Join(prefix, ".git")
	if err := os.Mkdir(repository, 0755); err != nil {
		t.Fatal(err)
	}
	configuration := []byte("[remote \"origin\"]\n\turl = https://github.com/Homebrew/brew\n")
	identity, err := exec.Command("/usr/bin/stat", "-f", "%d:%i:%u", repository).Output()
	if err != nil {
		t.Fatal(err)
	}
	approval := strings.Repeat("a", 64)
	marker := fmt.Sprintf("%s\n%s\n%x  .git/config\n", approval, strings.TrimSpace(string(identity)), sha256.Sum256(configuration))
	if err := os.WriteFile(filepath.Join(repository, "dotfiles-retirement"), []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	configuration = append(configuration, []byte("[core]\n\tworktree = /outside\n")...)
	if err := os.WriteFile(filepath.Join(repository, "config"), configuration, 0644); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(prefix)
	if err != nil {
		t.Fatal(err)
	}
	_, err = queryLegacyRuntime(context.Background(), d.homebrewRepositoryCommand(prefix, approval))
	if code, ok := legacyExitCode(err); !ok || code != 111 {
		t.Fatal("changed local configuration must block recovery before running Git", err)
	}
	if after, err := snapshotConfig(prefix); err != nil || after != before {
		t.Fatal("refused recovery changed repository files", after, err)
	}
}

func TestLegacyHomebrewPreservationNeverExecutesTapFilters(t *testing.T) {
	d, tap := legacyHomebrewTapFixture(t)
	for name, contents := range map[string]string{".gitattributes": "*.rb filter=witness\n", "personal.rb": "before\n"} {
		if err := os.WriteFile(filepath.Join(tap, name), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "Filtered source"},
		{"config", "filter.witness.clean", "touch filter-executed; cat"},
	} {
		if _, err := d.git(context.Background(), tap, arguments...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tap, "personal.rb"), []byte("after!\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := d.brewPreservationIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "executable Git filters") {
		t.Fatal("executable filters must block read-only tap inspection", err)
	}
	if _, err := os.Lstat(filepath.Join(tap, "filter-executed")); !os.IsNotExist(err) {
		t.Fatal("tap inspection ran a personal command", err)
	}
}

func TestLegacyHomebrewPreservationRefusesTapSubmodules(t *testing.T) {
	d, tap := legacyHomebrewTapFixture(t)
	head, err := d.git(context.Background(), tap, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.git(context.Background(), tap, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(string(head))+",nested"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.brewPreservationIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatal("a tap submodule needs independent inspection before Git status", err)
	}
}

func TestLegacyHomebrewPreservationRefusesLazyFetch(t *testing.T) {
	d, tap := legacyHomebrewTapFixture(t)
	if _, err := d.git(context.Background(), tap, "config", "remote.origin.promisor", "true"); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(tap)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.brewPreservationIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "partial-clone fetches") {
		t.Fatal("read-only tap inspection must not fetch missing objects", err)
	}
	if after, err := snapshotConfig(tap); err != nil || after != before {
		t.Fatal("tap inspection changed the partial repository", after, err)
	}
}
