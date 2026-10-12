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

func TestNativeConfigurationReadoptionRetainsTheFirstUninstallBaseline(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "initially-absent", true: "original-user-file"}[existing], func(t *testing.T) {
			c, _, destination := nativeConfigFixture(t, "copy")
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if existing {
				writeConfigFixture(t, destination, "original before any install")
				request.Adopt = []string{"config.test"}
			}
			dispatchApproved(t, c, request)
			original, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			preserved := map[string]string{}
			for i, edit := range []string{"first user edit", "second user edit"} {
				writeConfigFixture(t, destination, edit)
				request.Adopt = []string{"config.test"}
				result := dispatchApproved(t, c, request)
				state, err := LoadState(c.StatePath, c.Home)
				before, after := original.Receipts["config.test"], state.Receipts["config.test"]
				if err != nil || before.Recovery != after.Recovery || before.Adopted != after.Adopted || !sameArtifact(before.Before, after.Before) {
					t.Fatal("replacing edited configuration reset the original uninstall baseline", before, after, err)
				}
				if len(after.After.Preserved) != i+1 {
					t.Fatal("replacement lost earlier user edits", after.After.Preserved)
				}
				for _, path := range after.After.Preserved {
					if !strings.Contains(result.Message, path) {
						t.Fatal("saved edits were not disclosed", result.Message, path)
					}
					if _, known := preserved[path]; !known {
						preserved[path] = edit
					}
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err := os.ReadFile(destination)
			if existing && (err != nil || string(data) != "original before any install") || !existing && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("uninstall did not return to the original baseline", string(data), err)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || len(preservedPaths(state)) != 2 {
				t.Fatal("uninstall forgot retained user edits", state, err)
			}
			for path, expected := range preserved {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != expected {
					t.Fatal("uninstall removed or changed preserved user edits", path, string(data), err)
				}
			}
		})
	}
}

func TestNativeConfigurationUpdateAndRepairOfferToPreserveEditedCopies(t *testing.T) {
	for _, mode := range []string{"update", "repair"} {
		t.Run(mode, func(t *testing.T) {
			c, d, destination := nativeConfigFixture(t, "copy")
			d.Manifest.Targets[0].Directory, d.Manifest.Targets[0].Source = true, "nvim"
			writeConfigFixture(t, filepath.Join(d.Repository, "nvim", "lazy-lock.json"), "repository plugin lock")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			writeConfigFixture(t, filepath.Join(destination, "lazy-lock.json"), "lock changed by Lazy update")
			result := runController(t, c, []string{mode}, []string{"config.test"})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || len(preservedPaths(state)) != 1 {
				t.Fatal("maintenance lost edited lockfile", state, err)
			}
			path := preservedPaths(state)[0]
			data, err := os.ReadFile(filepath.Join(path, "lazy-lock.json"))
			if err != nil || string(data) != "lock changed by Lazy update" || !strings.Contains(result.Message, path) {
				t.Fatal("maintenance failed to preserve and disclose edited directory", string(data), result, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil || len(preservedPaths(state)) != 1 || preservedPaths(state)[0] != path {
				t.Fatal("reinstallation lost the saved-edit reference", state, err)
			}
		})
	}
}

func TestNativeConfigurationInterruptedMetadataWriteDoesNotBlockFinalization(t *testing.T) {
	c, d, _ := nativeConfigFixture(t, "copy")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	path := filepath.Join(d.Directory, "configuration", "operations", ".state-123456789")
	writeConfigFixture(t, path, "{\"incomplete\":")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "{\"incomplete\":" {
		t.Fatal("uncommitted metadata was read as intent or destroyed", string(data), err)
	}
}

func TestNativeConfigurationCanUpdateAfterSourceCheckoutMoves(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	next := d.Repository + " relocated 日本"
	if err := os.Rename(d.Repository, next); err != nil {
		t.Fatal(err)
	}
	d.Repository = next
	writeConfigFixture(t, filepath.Join(next, "config.json"), "new source location")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "new source location" {
		t.Fatal(string(data), err)
	}
}

func TestNativeConfigurationUnreadableSourceDoesNotPreventRestoration(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "original user settings")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	if err := os.RemoveAll(d.Repository); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, d.Repository, "the source root is now a file")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "original user settings" {
		t.Fatal(string(data), err)
	}
}

func TestNativeConfigurationRedirectedFolderIsReportedWithoutBlockingOtherResources(t *testing.T) {
	c, d, _ := nativeConfigFixture(t, "copy")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	d.Folders.Config = filepath.Join(filepath.Dir(d.Folders.Config), "redirected")
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" {
		t.Fatal("one moved target broke discovery", check, err)
	}
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil || plan.Operations[0].Action != "retain" {
		t.Fatal("moved target was forgotten or removed", plan, err)
	}
	addIndependentConfigFixture(t, c, d)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"other"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}

func addIndependentConfigFixture(t *testing.T, c Controller, d *ConfigDriver) {
	t.Helper()
	r := c.Catalog.Resources[0]
	r.ID, r.Name = "config.other", "Other configuration"
	c.Catalog.Resources = append(c.Catalog.Resources, r, Resource{ID: "other", Name: "Independent tool", Capability: true, Requires: []string{r.ID}})
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	item := d.Manifest.Targets[0]
	item.Resource, item.Path, item.Folder = r.ID, "independent.json", "home"
	d.Manifest.Targets = append(d.Manifest.Targets, item)
}

func TestNativeConfigurationOversizedUserFileDoesNotBlockOtherResources(t *testing.T) {
	c, d, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "user file")
	if err := os.Truncate(destination, maxConfigBytes+1); err != nil {
		t.Fatal(err)
	}
	addIndependentConfigFixture(t, c, d)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"other"}})
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"test", "other"}})
	if err != nil {
		t.Fatal("bounded inspection aborted unrelated discovery", err)
	}
	for _, op := range plan.Operations {
		if op.Resource == "config.test" && (op.Action != "pending" || !op.Observed.Unknown) {
			t.Fatal("uninspectable target was treated as absent", op)
		}
	}
	info, err := os.Stat(destination)
	if err != nil || info.Size() != maxConfigBytes+1 {
		t.Fatal("unrelated installation changed the oversized user file", info, err)
	}
}

func TestNativeConfigurationLiveSourceEditsDoNotInvalidateLinkRecovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX live-link contract")
	}
	d, r, receipt, j := configInterruptedFixtureMode(t, "after-backup", "link")
	writeConfigFixture(t, j.Entries[0].State.Target.Source, "user edited their live source")
	observation, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "adopt", Observed: observation}, receipt)
	if err != nil || !after.Healthy {
		t.Fatal("live source edit stranded the unchanged link operation", after, err)
	}
	data, err := os.ReadFile(j.Entries[0].State.Target.Destination)
	if err != nil || string(data) != "user edited their live source" {
		t.Fatal("recovery clobbered live source edits", string(data), err)
	}
}

func TestNativeConfigurationSealedHistoryDoesNotBlockUnrelatedFinalization(t *testing.T) {
	for _, change := range []string{"edited-backup", "foreign-file", "removed-resource", "missing-baseline"} {
		t.Run(change, func(t *testing.T) {
			c, d, destination := nativeConfigFixture(t, "copy")
			writeConfigFixture(t, destination, "original user configuration")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := d.Catalog.Resource("config.test")
			receipt := state.Receipts[r.ID]
			baseline, err := d.readBaseline(r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "edited-backup":
				writeConfigFixture(t, baseline.Targets[0].Backup, "user edited their recovery copy")
			case "foreign-file":
				writeConfigFixture(t, filepath.Join(filepath.Dir(baseline.Targets[0].Backup), ".DS_Store"), "foreign metadata")
			case "removed-resource":
				d.Catalog = &Catalog{Schema: 1}
				d.Manifest = ConfigManifest{Schema: 1}
				if err := d.Catalog.Validate(); err != nil {
					t.Fatal(err)
				}
			case "missing-baseline":
				if err := os.Remove(d.baselinePath(receipt.Recovery)); err != nil {
					t.Fatal(err)
				}
			}
			// Integrity checks belong to the affected resource's observation.
			// Historical cleanup must not poison another transaction's finalize.
			if _, err := d.FinishTransaction(context.Background(), Plan{}, nil); err != nil {
				t.Fatal("sealed history blocked finalization", err)
			}
		})
	}
}

func TestNativeConfigurationCanAbandonUnstartedPublicationAfterUserEdit(t *testing.T) {
	d, _, _, j := configInterruptedFixture(t, "after-staging")
	writeConfigFixture(t, j.Entries[0].State.Target.Destination, "user edited before any publication")
	if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "abandon"}, nil); err != nil {
		t.Fatal("unstarted operation cannot be abandoned", err)
	}
	data, err := os.ReadFile(j.Entries[0].State.Target.Destination)
	if err != nil || string(data) != "user edited before any publication" {
		t.Fatal("abandon overwrote user edits", string(data), err)
	}
}

func TestNativeConfigurationDeletedAdoptedTargetStillRestoresItsBaseline(t *testing.T) {
	c, _, destination := nativeConfigFixture(t, "copy")
	writeConfigFixture(t, destination, "original user configuration")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "original user configuration" {
		t.Fatal("deselection stranded an adopted baseline after target deletion", string(data), err)
	}
}
