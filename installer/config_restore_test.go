package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Interrupt at the provider/engine boundary, after real native publication but
// before the engine records completion. Rewound positions below are persisted
// rename cuts; separate subprocess tests exercise actual process termination.
type interruptedConfigDriver struct{ *ConfigDriver }

func (d interruptedConfigDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.ConfigDriver.Apply(ctx, r, op, receipt)
	return o, errors.Join(err, errors.New("test interruption before engine completion"))
}

func (d interruptedConfigDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.ConfigDriver.Remove(ctx, r, receipt)
	return o, errors.Join(err, errors.New("test interruption before engine completion"))
}

func interruptedConfigController(t *testing.T, action, cut string) (Controller, *ConfigDriver, Receipt, configJournal) {
	t.Helper()
	c, d, destination := nativeConfigFixture(t, "copy")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	if action != "install" {
		writeConfigFixture(t, destination, "original user settings")
		request.Adopt = []string{"config.test"}
	}
	if action == "update" || action == "remove" {
		dispatchApproved(t, c, request)
		request = Request{Schema: 1, Mode: "update"}
		writeConfigFixture(t, filepath.Join(d.Repository, "config.json"), "updated managed settings")
		if action == "remove" {
			request = Request{Schema: 1, Mode: "apply", Selected: []string{}}
		}
	}
	c.Driver = interruptedConfigDriver{d}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "test interruption") {
		t.Fatal("fixture did not interrupt real publication", err)
	}
	c.Driver = d
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := c.Catalog.Resource("config.test")
	receipt := state.Receipts[r.ID]
	j, err := d.readJournal(r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if cut == "completed-unsealed" {
		return c, d, receipt, j
	}
	j.Complete = false
	entry := &j.Entries[0]
	entry.Phase = "publishing"
	if cut == "after-backup" || cut == "before-move" {
		payload := filepath.Join(entry.Workspace, "next")
		if action == "remove" {
			payload = entry.Restore.Backup
		}
		if configPublicationResult(j, *entry).Kind != "absent" {
			if err := moveConfigExclusive(destination, payload); err != nil {
				t.Fatal(err)
			}
		}
		entry.Phase = "moving"
		if cut == "before-move" && entry.State.Current.Kind != "absent" {
			if err := moveConfigExclusive(filepath.Join(entry.Workspace, "previous"), destination); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	return c, d, receipt, j
}

func restoreConfigController(t *testing.T, c Controller) Result {
	t.Helper()
	request := Request{Schema: 1, Mode: "restore"}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal("restoration preview failed", err)
	}
	request.ExpectedPlan = plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "ready" && result.Status != "needs-action" {
		t.Fatal("restoration failed", result, err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction != nil || len(state.PastTransactions) != 1 || state.Receipts["config.test"].Status == "in-progress" {
		t.Fatal("restoration did not finish the interrupted transaction", state, err)
	}
	return result
}

func TestNativeConfigurationRestorePreservesConflictsWithoutTheOriginalSource(t *testing.T) {
	for _, action := range []string{"install", "adopt", "update", "remove"} {
		for _, cut := range []string{"before-move", "after-backup", "after-publication"} {
			t.Run(action+"/"+cut, func(t *testing.T) {
				c, d, _, j := interruptedConfigController(t, action, cut)
				entry := j.Entries[0]
				writeConfigFixture(t, entry.State.Target.Destination, "recreated or edited by the application")
				c.Source = "new-installer-source"
				if err := os.RemoveAll(d.Repository); err != nil {
					t.Fatal(err)
				}
				result := restoreConfigController(t, c)
				found := false
				for _, path := range append(configRecoveryPaths(entry), entry.State.Target.Destination) {
					data, err := os.ReadFile(path)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						t.Fatal(err)
					}
					if string(data) == "recreated or edited by the application" {
						found = true
						if path != entry.State.Target.Destination && !strings.Contains(result.Message, path) {
							t.Fatal("retained conflict was not disclosed", path, result)
						}
					}
				}
				if !found {
					t.Fatal("restoration lost the application's later edit")
				}
			})
		}
	}
}

func TestNativeConfigurationRestoreRejectsChangedApproval(t *testing.T) {
	for _, artifact := range []string{"target", "previous", "next", "partial", "displaced", "baseline"} {
		t.Run(artifact, func(t *testing.T) {
			c, _, _, j := interruptedConfigController(t, "update", "after-backup")
			entry := j.Entries[0]
			plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "restore"})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(entry.Workspace, artifact)
			if artifact == "target" {
				path = entry.State.Target.Destination
			} else if artifact == "baseline" {
				path = entry.Restore.Backup
			}
			writeConfigFixture(t, path, "changed after restoration approval")
			before := configRecoveryFiles(j)
			if _, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "restore", ExpectedPlan: plan.ID}); err == nil {
				t.Fatal("stale restoration approval changed files")
			}
			after := configRecoveryFiles(j)
			x, _ := digest(before)
			y, _ := digest(after)
			if x != y {
				t.Fatal("rejected restoration moved files")
			}
		})
	}
}

func TestNativeConfigurationRestoreDoesNotNeedTheBaselineDocumentOrCurrentMapping(t *testing.T) {
	c, d, receipt, j := interruptedConfigController(t, "adopt", "after-backup")
	if err := os.Remove(d.baselinePath(receipt.Recovery)); err != nil {
		t.Fatal(err)
	}
	d.Manifest = ConfigManifest{Schema: 1}
	d.Catalog = &Catalog{Schema: 1}
	c.Catalog = d.Catalog
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Driver = &NativeDriver{Catalog: c.Catalog, Configurations: d, StatePath: c.StatePath}
	result := restoreConfigController(t, c)
	data, err := os.ReadFile(j.Entries[0].State.Target.Destination)
	if err != nil || string(data) != "original user settings" || result.Status != "ready" {
		t.Fatal("restoration depended on current source or an external baseline document", string(data), result, err)
	}
}

func TestNativeConfigurationUnsealedCompletionPreservesChangedRecoveryFiles(t *testing.T) {
	for _, changed := range []string{"baseline", "previous", "next", "missing-baseline-document", "retired-resource"} {
		t.Run(changed, func(t *testing.T) {
			_, d, receipt, j := interruptedConfigController(t, "update", "completed-unsealed")
			entry := j.Entries[0]
			path := filepath.Join(entry.Workspace, changed)
			switch changed {
			case "baseline":
				path = entry.Restore.Backup
			case "missing-baseline-document":
				if err := os.Remove(d.baselinePath(receipt.Recovery)); err != nil {
					t.Fatal(err)
				}
			case "retired-resource":
				d.Catalog = &Catalog{Schema: 1}
				d.Manifest = ConfigManifest{Schema: 1}
				if err := d.Catalog.Validate(); err != nil {
					t.Fatal(err)
				}
			}
			if changed == "baseline" || changed == "previous" || changed == "next" {
				writeConfigFixture(t, path, "user changed a recovery file")
			}
			preserved, err := d.FinishTransaction(context.Background(), Plan{}, nil)
			if err != nil {
				t.Fatal("unsealed completed history blocked finalization", err)
			}
			if (changed == "baseline" || changed == "previous" || changed == "next") && !slices.Contains(preserved[j.Resource], path) {
				t.Fatal("finalization did not report retained user bytes", preserved, path)
			}
			if changed == "baseline" || changed == "previous" || changed == "next" {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "user changed a recovery file" {
					t.Fatal("finalization discarded changed recovery data", string(data), err)
				}
			}
		})
	}
}
