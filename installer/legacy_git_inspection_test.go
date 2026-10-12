package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyPreviewNeverExecutesRepositoryFilters(t *testing.T) {
	for _, target := range []string{"tap", "plugin"} {
		for _, config := range []string{"local", "worktree"} {
			t.Run(target+"/"+config, func(t *testing.T) {
				d, repository := legacyHomebrewTapFixture(t)
				for name, content := range map[string]string{".gitattributes": "*.rb filter=witness\n", "personal.rb": "before\n"} {
					writeConfigFixture(t, filepath.Join(repository, name), content)
				}
				legacyGitFixture(t, repository, "add", ".")
				legacyGitFixture(t, repository, "commit", "-q", "-m", "filtered source")
				head := strings.TrimSpace(legacyGitFixture(t, repository, "rev-parse", "HEAD"))
				legacyGitFixture(t, repository, "config", "remote.origin.url", "https://example.invalid/plugin")
				if config == "worktree" {
					legacyGitFixture(t, repository, "config", "extensions.worktreeConfig", "true")
				}
				legacyGitFixture(t, repository, "config", "--"+config, "filter.witness.clean", "touch filter-executed; cat")
				writeConfigFixture(t, filepath.Join(repository, "personal.rb"), "after!\n")
				before, err := snapshotConfig(repository)
				if err != nil {
					t.Fatal(err)
				}
				if target == "tap" {
					if _, err := d.brewPreservationIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "executable Git filters") {
						t.Error("unsafe tap was not refused", err)
					}
				} else {
					var scope legacyRetirementScope
					checkout := legacyCheckout{Path: repository, Commit: head, Origin: "https://example.invalid/plugin"}
					if err := d.planCheckout(context.Background(), &scope, checkout); err != nil || len(scope.Steps) != 0 || !strings.Contains(strings.Join(scope.Retained, "\n"), "executable Git filters") {
						t.Error("unsafe plugin was not preserved", scope, err)
					}
				}
				if _, err := os.Lstat(filepath.Join(repository, "filter-executed")); !os.IsNotExist(err) {
					t.Error("preview executed a personal command", err)
				}
				if after, err := snapshotConfig(repository); err != nil || after != before {
					t.Error("preview changed the repository", err)
				}
			})
		}
	}
}

func TestLegacyHistoryInspectionNeverFetchesPromisedTrees(t *testing.T) {
	d, source := legacyHomebrewTapFixture(t)
	data := []byte("recorded content\n")
	writeConfigFixture(t, filepath.Join(source, "recorded"), string(data))
	legacyGitFixture(t, source, "add", "recorded")
	legacyGitFixture(t, source, "commit", "-q", "-m", "recorded file")
	legacyGitFixture(t, source, "config", "uploadpack.allowFilter", "true")
	partial := filepath.Join(t.TempDir(), "partial")
	legacyGitFixture(t, source, "clone", "--quiet", "--filter=tree:0", "--no-checkout", "file://"+filepath.ToSlash(source), partial)
	before, err := snapshotConfig(partial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.legacyBlobInHistory(context.Background(), partial, "recorded", data); err == nil || !strings.Contains(err.Error(), "partial-clone fetches") {
		t.Error("history inspection did not refuse missing promised trees", err)
	}
	if after, err := snapshotConfig(partial); err != nil || after != before {
		t.Fatal("history inspection fetched objects into the repository", err)
	}
}

func TestLegacyPluginRetirementRecognizesUninitializedSubmodules(t *testing.T) {
	for _, state := range []string{"absent", "empty", "populated", "cached"} {
		t.Run(state, func(t *testing.T) {
			d, repository := legacyHomebrewTapFixture(t)
			initial := strings.TrimSpace(legacyGitFixture(t, repository, "rev-parse", "HEAD"))
			// The released tmux-resurrect pin has this uninitialized test
			// gitlink; install-deps.sh never initialized its submodule.
			legacyGitFixture(t, repository, "update-index", "--add", "--cacheinfo", "160000,"+initial+",lib/tmux-test")
			legacyGitFixture(t, repository, "commit", "-q", "-m", "uninitialized test submodule")
			legacyGitFixture(t, repository, "config", "remote.origin.url", "https://example.invalid/plugin")
			head := strings.TrimSpace(legacyGitFixture(t, repository, "rev-parse", "HEAD"))
			path := filepath.Join(repository, "lib", "tmux-test")
			switch state {
			case "empty":
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "populated":
				writeConfigFixture(t, filepath.Join(path, "personal"), "keep\n")
			case "cached":
				writeConfigFixture(t, filepath.Join(repository, ".git", "modules", "lib", "tmux-test", "personal"), "keep\n")
			}
			before, err := snapshotConfig(repository)
			if err != nil {
				t.Fatal(err)
			}
			var scope legacyRetirementScope
			checkout := legacyCheckout{Path: repository, Commit: head, Origin: "https://example.invalid/plugin"}
			if err := d.planCheckout(context.Background(), &scope, checkout); err != nil {
				t.Fatal(err)
			}
			// Git reports a missing tracked gitlink as deleted. Keep that
			// modified checkout; the released checkout creates an empty dir.
			if state == "absent" && !strings.Contains(legacyGitFixture(t, repository, "status", "--porcelain=v1"), " D lib/tmux-test") {
				t.Fatal("missing-gitlink fixture no longer matches Git's dirty status")
			}
			wantRemoval := state == "empty"
			if wantRemoval && (len(scope.Steps) != 1 || len(scope.Retained) != 0) || !wantRemoval && (len(scope.Steps) != 0 || len(scope.Retained) == 0) {
				t.Fatal("uninitialized/personal submodule ownership was misclassified", state, scope)
			}
			if after, err := snapshotConfig(repository); err != nil || after != before {
				t.Fatal("submodule preview changed repository data", err)
			}
		})
	}
}
