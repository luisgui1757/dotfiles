package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func legacyGitController(c Controller) Controller {
	old := c
	old.Catalog = &Catalog{Schema: 1, Resources: slices.Clone(c.Catalog.Resources)}
	for i := range old.Catalog.Resources {
		r := &old.Catalog.Resources[i]
		if r.ID == "git" {
			r.Requires = []string{"config.git"}
		}
		if r.ID == "config.git" {
			r.ReplacedBy = ""
		}
	}
	return old
}

func TestGitDeclinedLegacyRestorationStaysRemovable(t *testing.T) {
	c, _, target := gitSettingsFixture(t)
	writeConfigFixture(t, target, "[user]\nname = Original\n")
	dispatchApproved(t, legacyGitController(c), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}, Adopt: []string{"config.git"}})
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	changed := "[user]\nname = Edited personal identity\n"
	writeConfigFixture(t, target, changed)
	ui := &scriptedInteraction{t: t, accept: true, answers: [][]string{{"apply"}, {}, {}}}
	result, err := c.Run(context.Background(), ui)
	if err != nil || result.Status != "ready" || len(ui.answers) != 0 {
		t.Fatal("declining restoration stranded an active transaction", result, err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || slices.Contains(state.Keep, "config.git") || state.Transaction != nil {
		t.Fatal("recovery-only mapping became a retained installation root", state, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != changed {
		t.Fatal("declining restoration changed personal settings", err)
	}
	ui = &scriptedInteraction{t: t, accept: true, answers: [][]string{{"apply"}, {}, {"config.git"}}}
	result, err = c.Run(context.Background(), ui)
	if err != nil || result.Status != "ready" || len(ui.answers) != 0 {
		t.Fatal("restoration was not available on the next run", result, err)
	}
}

func TestGitRecoveryOnlyMappingCannotBeKeptOrAdopted(t *testing.T) {
	c, _, _ := gitSettingsFixture(t)
	dispatchApproved(t, legacyGitController(c), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	for _, adopt := range []bool{false, true} {
		request := Request{Schema: 1, Mode: "apply", Selected: []string{}, Keep: []string{"config.git"}}
		if adopt {
			request.Adopt = []string{"config.git"}
		}
		if _, err := c.Preview(context.Background(), request); err == nil {
			t.Fatal("machine request accepted a recovery-only installation root", request)
		}
	}
}

func TestGitDanglingParentIsResourceLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX dangling directory link")
	}
	for _, recorded := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-selection", true: "recorded-receipt"}[recorded], func(t *testing.T) {
			c, source, target := gitSettingsFixture(t)
			if recorded {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
				if err := os.Rename(filepath.Dir(target), filepath.Dir(target)+"-saved"); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(c.Home, "missing-personal-config"), filepath.Dir(target)); err != nil {
				t.Fatal(err)
			}
			folders := c.Driver.(*NativeDriver).Configurations.Folders
			rebuilt, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
			if err != nil {
				t.Fatal("bad Git path prevented controller construction", err)
			}
			rebuilt.Catalog = c.Catalog
			selected := []string{"git"}
			if recorded {
				selected = []string{}
			}
			preview, err := rebuilt.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: selected})
			if err != nil {
				t.Fatal("bad Git path prevented an independent preview", err)
			}
			op := operation(t, preview, gitSettingsResource)
			if !op.Observed.Unknown || !strings.Contains(op.Observed.Pending, "missing-personal-config") {
				t.Fatal("the real path error was not preserved locally", op)
			}
		})
	}
}

func TestGitLegacyReceiptDoesNotRetainShellIntegration(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Target: "/fixture", Receipts: map[string]Receipt{}}
	observed := map[string]Observation{}
	for _, id := range []string{"config.git", "tool.git", "integration.shells"} {
		o := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: id}
		observed[id] = o
		state.Receipts[id] = Receipt{Ownership: "created", Before: Observation{}, After: o, Status: "ready"}
	}
	r := state.Receipts["config.git"]
	r.Ownership, r.Status = "reused", "removed"
	state.Receipts["config.git"] = r
	plan, err := PlanChanges(c, linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.git"}}, observed, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tool.git", "integration.shells"} {
		if op := operation(t, plan, id); op.Action != "remove" {
			t.Fatal("retired configuration pinned an unrelated active integration", op)
		}
	}
}

func TestGitCancelledFirstPublicationIsNotAMissingBaseline(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "abandoned-before-publication", true: "restored-after-publication"}[restore], func(t *testing.T) {
			cut := "after-staging"
			if restore {
				cut = "after-publish"
			}
			d, r, receipt, journal := configInterruptedFixture(t, cut)
			if restore {
				before, err := d.ObserveRestore(context.Background(), r.ID, receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := d.RestoreResource(context.Background(), r.ID, before, receipt); err != nil {
					t.Fatal(err)
				}
			} else if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "abandon"}, nil); err != nil {
				t.Fatal(err)
			}
			r.ReplacedBy = "integration.git"
			receipt.Status = "needs-action"
			after, err := d.Observe(context.Background(), r, receipt)
			if err != nil || after.Pending != "" || after.RestoredOperation != receipt.OperationID {
				t.Fatal("completed cancellation was mistaken for a lost original", after, err)
			}
			if data, err := os.ReadFile(journal.Entries[0].State.Target.Destination); err != nil || string(data) != "user baseline" {
				t.Fatal("original personal configuration was changed", err)
			}
		})
	}
}

func TestLegacyMigrationUnavailablePathDoesNotAbortOtherItems(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX dangling directory fixture")
	}
	c, source, _ := gitSettingsFixture(t)
	folders := c.Driver.(*NativeDriver).Configurations.Folders
	if err := os.MkdirAll(folders.Config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(c.Home, "missing-dashboard"), filepath.Join(folders.Config, "gh-dash")); err != nil {
		t.Fatal(err)
	}
	for _, gitPath := range []string{"", "relative/git/config"} {
		folders.GitConfig = gitPath
		m, err := NewLegacyMigration(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), c.Context, folders)
		if err != nil {
			t.Fatal("unavailable item aborted migration construction", err)
		}
		check, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
		if err != nil {
			t.Fatal("unavailable item aborted migration check", err)
		}
		if gitPath != "" && (check.Status != "needs-action" || !strings.Contains(check.Message, "canonical Git")) {
			t.Fatal("invalid Git path was silently skipped", check)
		}
	}
}
