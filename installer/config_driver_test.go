package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func nativeConfigFixture(t *testing.T, mode string) (Controller, *ConfigDriver, string) {
	t.Helper()
	c, manifest, folders, repository := configInspectionFixture(t)
	c.Resources = append(c.Resources, Resource{ID: "test", Name: "Test tool", Capability: true, Requires: []string{"config.test"}})
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "windows", Arch: "amd64"}
	if mode == "link" {
		target.OS = "linux"
	}
	d := &ConfigDriver{Catalog: c, Manifest: manifest, Target: target, Folders: folders, Repository: repository, Directory: filepath.Join(filepath.Dir(repository), "state")}
	controller := Controller{Catalog: c, Context: target, Source: "config-test-source", Home: folders.Home, StatePath: filepath.Join(d.Directory, "state.json"), Driver: d}
	states, _, err := inspectConfiguration(c, manifest, target, folders, repository, "config.test")
	if err != nil {
		t.Fatal(err)
	}
	return controller, d, states[0].Target.Destination
}

func writeConfigFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNativeConfigurationInstallUpdateRemoveReinstall(t *testing.T) {
	for _, mode := range []string{"copy", "link"} {
		if mode == "link" && runtime.GOOS == "windows" {
			continue
		}
		t.Run(mode, func(t *testing.T) {
			c, d, destination := nativeConfigFixture(t, mode)
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if _, err := c.Preview(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(d.Folders.Home); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("preview created the target", err)
			}
			for cycle := 0; cycle < 2; cycle++ {
				dispatchApproved(t, c, request)
				writeConfigFixture(t, filepath.Join(d.Repository, "config.json"), "updated configuration")
				dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "updated configuration" {
					t.Fatal(string(data), err)
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil || state.Receipts["config.test"].Ownership != "created" {
					t.Fatal(state, err)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
				if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("uninstall retained managed configuration", err)
				}
				files, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".dotfiles-config-*"))
				if err != nil || len(files) != 0 {
					t.Fatal("uninstall left adjacent recovery workspaces", files, err)
				}
			}
		})
	}
}

func TestNativeConfigurationAdoptionKeepsOriginalBaselineAcrossUpdates(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "original user settings")
	original, err := snapshotConfig(destination)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || preview.Operations[0].Action != "pending" {
		t.Fatal("pre-existing settings silently adopted", preview, err)
	}
	request.Adopt = []string{"config.test"}
	dispatchApproved(t, c, request)
	for _, payload := range []string{"second", "third"} {
		writeConfigFixture(t, filepath.Join(d.Repository, "config.json"), payload)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err := verifyConfigSnapshot(destination, original); err != nil {
		t.Fatal("uninstall did not restore the first baseline", err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".dotfiles-config-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("restoration left recovery workspaces", files, err)
	}
}

func TestNativeConfigurationChangedTargetIsPreservedOnUninstall(t *testing.T) {
	c, _, destination := nativeConfigFixture(t, "copy")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	writeConfigFixture(t, destination, "user changed the installed configuration")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "user changed the installed configuration" {
		t.Fatal("uninstall destroyed a user edit", string(data), err)
	}
}

func TestNativeConfigurationUninstallDoesNotRequireItsSourceCheckout(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "original settings")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	if err := os.RemoveAll(d.Repository); err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" {
		t.Fatal("missing source was not explained by check", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "original settings" {
		t.Fatal("uninstall depended on source availability", string(data), err)
	}
}

func TestNativeConfigurationChangedBaselinePreventsUpdateAndRemoval(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "original settings")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := d.Catalog.Resource("config.test")
	baseline, err := d.readBaseline(r, state.Receipts[r.ID])
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, baseline.Targets[0].Backup, "edited backup")
	for _, request := range []Request{{Schema: 1, Mode: "update"}, {Schema: 1, Mode: "apply", Selected: []string{}}} {
		plan, err := c.Preview(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Operations[0].Action != "pending" && plan.Operations[0].Action != "retain" {
			t.Fatal("changed backup allowed a destructive operation", plan)
		}
	}
}

func TestNativeConfigurationCopiesDirectoryAndRestoresOriginalTree(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	d.Manifest.Targets[0].Directory, d.Manifest.Targets[0].Source = true, "tree"
	writeConfigFixture(t, filepath.Join(d.Repository, "tree", "nested", "settings"), "new tree")
	writeConfigFixture(t, filepath.Join(destination, "user", "settings"), "original tree")
	original, err := snapshotConfig(destination)
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	data, err := os.ReadFile(filepath.Join(destination, "nested", "settings"))
	if err != nil || string(data) != "new tree" {
		t.Fatal(string(data), err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err := verifyConfigSnapshot(destination, original); err != nil {
		t.Fatal(err)
	}
}

func TestNativeConfigurationRestoresUserLinkWithoutReadingItsReferent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX link preservation; Windows directory copies need no link privilege")
	}
	c, _, destination := nativeConfigFixture(t, "link")
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-user-referent", destination); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	link, err := os.Readlink(destination)
	if err != nil || link != "missing-user-referent" {
		t.Fatal("restoration followed or changed the original user link", link, err)
	}
}

func TestConfigExclusiveMoveNeverReplacesAnUnexpectedTarget(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	from, to := filepath.Join(root, "from"), filepath.Join(root, "to")
	writeConfigFixture(t, from, "approved")
	writeConfigFixture(t, to, "appeared after approval")
	if err := moveConfigExclusive(from, to); err == nil {
		t.Fatal("publication overwrote a newly appeared target")
	}
	for path, expected := range map[string]string{from: "approved", to: "appeared after approval"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != expected {
			t.Fatal(path, string(data), err)
		}
	}
}

// These are persisted crash shapes produced at native rename boundaries. Normal
// publication creates the payload and original backup; the fixture rewinds only
// publication position, using real native moves rather than mocking internals.
// Controller retry/approval semantics are exercised in resource_resume_test.go.
func configInterruptedFixture(t *testing.T, cut string) (*ConfigDriver, Resource, Receipt, configJournal) {
	return configInterruptedFixtureMode(t, cut, "copy")
}

func configInterruptedFixtureMode(t *testing.T, cut, mode string) (*ConfigDriver, Resource, Receipt, configJournal) {
	t.Helper()
	_, d, destination := nativeConfigFixture(t, mode)
	writeConfigFixture(t, destination, "user baseline")
	r, _ := d.Catalog.Resource("config.test")
	before, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{Ownership: "uncertain", Before: before, Adopted: true, Status: "in-progress", OperationID: strings.Repeat("b", 64), Recovery: filepath.Join("recovery", strings.Repeat("a", 64), r.ID)}
	if _, err := d.Apply(context.Background(), r, Operation{Resource: r.ID, Action: "adopt", Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal(r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	j.Complete = false
	switch cut {
	case "after-publish":
		j.Entries[0].Phase = "publishing"
	case "after-backup":
		if err := moveConfigExclusive(destination, filepath.Join(j.Entries[0].Workspace, "next")); err != nil {
			t.Fatal(err)
		}
		j.Entries[0].Phase = "moving"
	case "after-staging":
		if err := moveConfigExclusive(destination, filepath.Join(j.Entries[0].Workspace, "next")); err != nil {
			t.Fatal(err)
		}
		if err := moveConfigExclusive(j.Entries[0].Restore.Backup, destination); err != nil {
			t.Fatal(err)
		}
		j.Entries[0].Phase = ""
	default:
		t.Fatal("unknown crash cut", cut)
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	return d, r, receipt, j
}

func TestNativeConfigurationResumesSavedRenameBoundaries(t *testing.T) {
	for _, cut := range []string{"after-staging", "after-backup", "after-publish"} {
		t.Run(cut, func(t *testing.T) {
			d, r, receipt, j := configInterruptedFixture(t, cut)
			observation, err := d.Observe(context.Background(), r, receipt)
			if err != nil || observation.ResourceResume == nil || observation.CompletedOperation != "" {
				t.Fatal(observation, err)
			}
			after, err := d.ResumeResource(context.Background(), r, Operation{Resource: r.ID, Action: "adopt", Observed: observation}, receipt)
			if err != nil || after.CompletedOperation != receipt.OperationID || !after.Healthy || after.ResourceResume != nil {
				t.Fatal(after, err)
			}
			if err := verifyConfigSnapshot(j.Entries[0].Restore.Backup, j.Entries[0].Restore.Original); err != nil {
				t.Fatal("resume changed original user baseline", err)
			}
		})
	}
}

func TestNativeConfigurationRecoveryRejectsChangedBackupAndSource(t *testing.T) {
	for _, artifact := range []string{"backup", "source"} {
		t.Run(artifact, func(t *testing.T) {
			d, r, receipt, j := configInterruptedFixture(t, "after-backup")
			observation, err := d.Observe(context.Background(), r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			path := j.Entries[0].Restore.Backup
			if artifact == "source" {
				path = j.Entries[0].State.Target.Source
			}
			writeConfigFixture(t, path, "unapproved change")
			if _, err := d.ResumeResource(context.Background(), r, Operation{Action: "adopt", Observed: observation}, receipt); err == nil {
				t.Fatal("recovery accepted changed artifacts")
			}
			if err := verifyConfigSnapshot(j.Entries[0].State.Target.Destination, configSnapshot{Kind: "absent"}); err != nil {
				t.Fatal("rejected retry still published", err)
			}
		})
	}
}

func TestNativeConfigurationCannotAbandonPartiallyMovedTarget(t *testing.T) {
	d, _, _, j := configInterruptedFixture(t, "after-backup")
	if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "abandon"}, nil); err == nil {
		t.Fatal("abandon lost partial publication intent")
	}
	if err := verifyConfigSnapshot(j.Entries[0].Restore.Backup, j.Entries[0].Restore.Original); err != nil {
		t.Fatal(err)
	}
}

func TestNativeConfigurationResumesInterruptedCleanup(t *testing.T) {
	for _, cut := range []string{"partial-tree", "empty-workspace"} {
		t.Run(cut, func(t *testing.T) {
			c, d, destination := nativeConfigFixture(t, "copy")
			d.Manifest.Targets[0].Directory, d.Manifest.Targets[0].Source = true, "tree"
			for _, name := range []string{"first", "second"} {
				writeConfigFixture(t, filepath.Join(d.Repository, "tree", name), name)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := d.Catalog.Resource("config.test")
			receipt := state.Receipts[r.ID]
			receipt.Status, receipt.OperationID = "in-progress", strings.Repeat("c", 64)
			writeConfigFixture(t, filepath.Join(d.Repository, "tree", "third"), "third")
			observed, err := d.Observe(context.Background(), r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Apply(context.Background(), r, Operation{Action: "update", Observed: observed}, receipt); err != nil {
				t.Fatal(err)
			}
			j, err := d.readJournal(r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := inspectTree(filepath.Join(j.Entries[0].Workspace, "previous"), maxConfigBytes, maxConfigEntries)
			if err != nil {
				t.Fatal(err)
			}
			j.Entries[0].Cleanup = map[string][]configEntry{"previous": entries}
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			workspace := j.Entries[0].Workspace
			if cut == "partial-tree" {
				if err := os.Remove(filepath.Join(workspace, "previous", "first")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.RemoveAll(filepath.Join(workspace, "previous")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := d.FinishTransaction(context.Background(), Plan{}, nil); err != nil {
				t.Fatal("cleanup could not resume", err)
			}
			if _, err := os.Lstat(workspace); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("obsolete payload retained", err)
			}
			if _, err := os.ReadFile(filepath.Join(destination, "third")); err != nil {
				t.Fatal("cleanup removed current configuration", err)
			}
		})
	}
}
