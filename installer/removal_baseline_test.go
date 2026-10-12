package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleasedBaselineRequiresCompletedUnchangedRemoval(t *testing.T) {
	for _, change := range []string{"none", "ready", "retained", "in-progress", "missing-status", "uncertain-owner", "changed-baseline", "changed-result", "changed-artifact", "pending", "unknown", "unknown-baseline"} {
		t.Run(change, func(t *testing.T) {
			state, observed := installed()
			baseline := observed["plugins"]
			receipt := Receipt{Ownership: "reused", Status: "removed", Before: baseline, After: baseline}
			switch change {
			case "ready", "retained", "in-progress":
				receipt.Status = change
			case "missing-status":
				receipt.Status = ""
			case "uncertain-owner":
				receipt.Ownership = "uncertain"
			case "changed-baseline":
				receipt.Before.Fingerprint = "different-baseline"
			case "changed-result":
				receipt.After.Fingerprint = "different-result"
			case "changed-artifact":
				baseline.Fingerprint = "later-user-edit"
			case "pending":
				baseline.Pending = "saved evidence is incomplete"
			case "unknown":
				baseline = Observation{Unknown: true, Pending: "cannot inspect personal artifact"}
			case "unknown-baseline":
				receipt.Before.Unknown = true
			}
			state.Receipts["plugins"], observed["plugins"] = receipt, baseline
			// Older schema-1 completed receipts may omit an operation ID. Their
			// saved status and exact before/after artifacts remain required.
			data, err := json.Marshal(state)
			if err != nil || Decode(data, &state) != nil {
				t.Fatal("cannot reload saved removal", err)
			}
			plan, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"node"}}, observed, "source")
			if err != nil {
				t.Fatal(err)
			}
			wantIntegration, wantRuntime := "retain", "retain"
			if change == "none" {
				wantIntegration, wantRuntime = "forget", "remove"
			}
			if op := operation(t, plan, "plugins"); op.Action != wantIntegration {
				t.Fatal("incorrect restored-artifact boundary", op)
			}
			if op := operation(t, plan, "node"); op.Action != wantRuntime {
				t.Fatal("incorrect dependency protection", op)
			}
		})
	}
}

func jsonSettingsRuntimeController(t *testing.T) (Controller, *ProfileDriver, string, string) {
	t.Helper()
	c, d, path := jsonSettingsController(t)
	dependency := filepath.Join(c.Home, "runtime fixture.ps1")
	c.Catalog.Resources[1].Requires = []string{"profile.runtime"}
	c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "profile.runtime", Name: "Runtime", Action: "profile", Scope: "user", Shared: true})
	d.Targets["profile.runtime"] = []ProfileTarget{{dependency, "# runtime fixture"}}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	return c, d, path, dependency
}

func TestJSONRemovalRecoveryReleasesRestoredSettingsPrerequisites(t *testing.T) {
	c, _, path, dependency := jsonSettingsRuntimeController(t)
	const original = `{"theme":"personal","fontSize":16}`
	writeConfigFixture(t, path, original)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"profile.test"}})

	// An actual application lock interrupts publication after the durable
	// removal intent is saved, without replacing the provider or its methods.
	if err := os.Mkdir(path+".lock", 0700); err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"profile.runtime"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatal("application lock did not interrupt removal", err)
	}
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatal(err)
	}
	retry := Request{Schema: 1, Mode: "apply", Retry: true}
	plan, err = c.Preview(context.Background(), retry)
	if err != nil || plan.ResourceResume == nil || len(plan.Operations) != 1 || plan.Operations[0].Resource != "profile.test" {
		t.Fatal("recovery is not the saved settings removal", plan, err)
	}
	retry.ExpectedPlan = plan.ID
	if result, err := c.Dispatch(context.Background(), retry); err != nil || result.Status != "needs-action" {
		t.Fatal("single-resource recovery did not complete", result, err)
	}
	retry.ExpectedPlan = ""
	plan, err = c.Preview(context.Background(), retry)
	if err != nil {
		t.Fatal(err)
	}
	if op := operation(t, plan, "profile.test"); op.Action != "forget" {
		t.Fatal("restored personal settings became an installed integration", op)
	}
	if op := operation(t, plan, "profile.runtime"); op.Action != "remove" {
		t.Fatal("completed settings removal retained its runtime", op)
	}
	dispatchApproved(t, c, retry)
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatal("remaining removal changed restored personal settings", string(data), err)
	}
	if _, err := os.Lstat(dependency); !os.IsNotExist(err) {
		t.Fatal("approved runtime removal did not complete", err)
	}
}

func TestRestoredSettingsKeepCompletionAcrossTemporaryRetention(t *testing.T) {
	for _, interruption := range []string{"none", "personal-edit", "missing-journal"} {
		t.Run(interruption, func(t *testing.T) {
			c, d, path, dependency := jsonSettingsRuntimeController(t)
			const original = `{"theme":"personal","fontSize":16}`
			writeConfigFixture(t, path, original)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"profile.test"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			before, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := before.Receipts["profile.test"]
			journal := d.journalPath(receipt.OperationID)
			switch interruption {
			case "personal-edit":
				writeConfigFixture(t, path, `{"theme":"later-personal-choice","fontSize":16}`)
			case "missing-journal":
				if err := os.Rename(journal, journal+".saved"); err != nil {
					t.Fatal(err)
				}
			}
			if interruption != "none" {
				request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"profile.runtime"}}
				if interruption == "missing-journal" {
					request = Request{Schema: 1, Mode: "update"}
				}
				result := dispatchApproved(t, c, request)
				if operation(t, result.Plan, "profile.test").Action != "retain" || operation(t, result.Plan, "profile.runtime").Action != "retain" {
					t.Fatal("unproved personal configuration lost dependency protection", result)
				}
				after, err := LoadState(c.StatePath, c.Home)
				if err != nil || after.Receipts["profile.test"].Status != "removed" || !sameArtifact(after.Receipts["profile.test"].After, receipt.After) {
					t.Fatal("temporary retention erased completed-removal evidence", after.Receipts["profile.test"], err)
				}
			}
			switch interruption {
			case "personal-edit":
				writeConfigFixture(t, path, original)
			case "missing-journal":
				if err := os.Rename(journal+".saved", journal); err != nil {
					t.Fatal(err)
				}
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"profile.runtime"}})
			if operation(t, result.Plan, "profile.test").Action != "forget" || operation(t, result.Plan, "profile.runtime").Action != "remove" {
				t.Fatal("restored evidence did not release the old prerequisites", result)
			}
			if _, err := os.Lstat(dependency); !os.IsNotExist(err) {
				t.Fatal("approved runtime removal did not complete", err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != original {
				t.Fatal("remaining removal changed personal settings", string(data), err)
			}
		})
	}
}

func TestAlreadyAbsentRemovalKeepsItsAccurateReason(t *testing.T) {
	state, observed := installed()
	for id, receipt := range state.Receipts {
		absent := receipt.After
		absent.Present, absent.Healthy, absent.Fingerprint = false, false, ""
		receipt.Before = absent
		state.Receipts[id], observed[id] = removedReceipt(receipt, absent), absent
	}
	plan, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range plan.Operations {
		if op.Action != "forget" || op.Reason != "already absent" {
			t.Fatal("absent resource was described as a preserved personal artifact", op)
		}
	}
}

func TestReselectingRestoredSettingsKeepsRemovalCompletion(t *testing.T) {
	for _, theme := range []string{"personal", "rose-pine"} {
		t.Run(theme, func(t *testing.T) {
			c, _, path, dependency := jsonSettingsRuntimeController(t)
			original := `{"theme":"` + theme + `","fontSize":16}`
			writeConfigFixture(t, path, original)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"profile.test"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Receipts["profile.test"].Status != "removed" {
				t.Fatal("reselection without adoption erased completed removal", state.Receipts["profile.test"], err)
			}
			if theme == "personal" {
				if result.Status != "needs-action" || operation(t, result.Plan, "profile.test").Action != "pending" || state.Transaction == nil || state.Transaction.Pending["profile.test"] == "" {
					t.Fatal("declined adoption lost its pending outcome", result, state.Transaction)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
			} else if result.Status != "ready" || operation(t, result.Plan, "profile.test").Action != "keep" {
				t.Fatal("matching personal settings were not reused", result)
			}
			result = dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"profile.runtime"}})
			if operation(t, result.Plan, "profile.test").Action != "forget" || operation(t, result.Plan, "profile.runtime").Action != "remove" {
				t.Fatal("declined adoption pinned the previously released runtime", result)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != original {
				t.Fatal("reselection changed personal settings", string(data), err)
			}
			if _, err := os.Lstat(dependency); !os.IsNotExist(err) {
				t.Fatal("approved runtime removal did not complete", err)
			}
		})
	}
}

// Model recovery evidence becoming unavailable during a later filesystem
// operation. Observation and finalization still use the real profile provider.
type missingBaselineJournalDriver struct {
	*ProfileDriver
	journal string
}

func (d *missingBaselineJournalDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	after, err := d.ProfileDriver.Apply(ctx, r, op, receipt)
	if err == nil && r.ID == "profile.later" {
		err = os.Rename(d.journal, d.journal+".saved")
	}
	return after, err
}

func TestFinalPendingObservationKeepsRemovalCompletion(t *testing.T) {
	c, d, path, dependency := jsonSettingsRuntimeController(t)
	const original = `{"theme":"rose-pine","fontSize":16}`
	writeConfigFixture(t, path, original)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"profile.test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	journal := d.journalPath(state.Receipts["profile.test"].OperationID)
	c.Catalog.Resources[0].Requires = []string{"profile.later"}
	c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "profile.later", Name: "Later profile", Action: "profile", Scope: "user", Requires: []string{"profile.test"}})
	d.Targets["profile.later"] = []ProfileTarget{{filepath.Join(c.Home, "later.ps1"), "# later integration"}}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Driver = &missingBaselineJournalDriver{d, journal}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, plan, "profile.test").Action != "keep" {
		t.Fatal("restored settings were not initially healthy", plan, err)
	}
	request.ExpectedPlan = plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err == nil || result.Status != "failed" || !strings.Contains(err.Error(), "required completion journal unavailable") {
		t.Fatal("missing completion journal did not preserve the finalization failure", result, err)
	}
	state, err = LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction == nil || state.Transaction.Error == "" || state.Transaction.Pending["profile.test"] == "" || state.Receipts["profile.test"].Status != "removed" {
		t.Fatal("final pending observation erased completion or pending intent", state.Receipts["profile.test"], state.Transaction, err)
	}
	if err := os.Rename(journal+".saved", journal); err != nil {
		t.Fatal(err)
	}
	c.Driver = d
	dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
	result = dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"profile.runtime"}})
	if operation(t, result.Plan, "profile.test").Action != "forget" || operation(t, result.Plan, "profile.runtime").Action != "remove" {
		t.Fatal("restored journal did not release the runtime", result)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatal("pending recovery changed personal settings", string(data), err)
	}
	if _, err := os.Lstat(dependency); !os.IsNotExist(err) {
		t.Fatal("approved runtime removal did not complete", err)
	}
}

func TestPreservationFailureKeepsCompletedRemovalEvidence(t *testing.T) {
	for _, kind := range []string{"retained", "released"} {
		t.Run(kind, func(t *testing.T) {
			c, d := engineFixture(t)
			initial := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
			if _, err := applyFixture(c, d, initial); err != nil {
				t.Fatal(err)
			}
			state, err := LoadState(d.statePath, d.home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := c.Resource("node")
			path := filepath.Join(d.dir, "node")
			writeConfigFixture(t, path, "personal baseline")
			baseline, err := d.Observe(context.Background(), r, Receipt{})
			if err != nil {
				t.Fatal(err)
			}
			state.Receipts["node"] = Receipt{Ownership: "reused", Status: "removed", Before: baseline, After: baseline}
			if err := SaveState(d.statePath, state); err != nil {
				t.Fatal(err)
			}
			if kind == "retained" {
				writeConfigFixture(t, path, "later personal edit")
			}
			request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &collateralDriver{fileDriver: d})
			if err == nil || result.Status == "ready" || !strings.Contains(err.Error(), "verification") {
				t.Fatal("real collateral change was not refused", result, err)
			}
			state, err = LoadState(d.statePath, d.home)
			if err != nil || state.Transaction == nil || state.Transaction.Error == "" || state.Receipts["node"].Status != "removed" || !sameArtifact(state.Receipts["node"].After, baseline) {
				t.Fatal("preservation failure erased removal history or failure intent", state.Receipts["node"], err)
			}
			writeConfigFixture(t, path, "personal baseline")
			retry := approve(t, c, d, Request{Schema: 1, Mode: "apply", Retry: true})
			if result, err := applyFixture(c, d, retry); err != nil || result.Status != "ready" || operation(t, result.Plan, "node").Action != "forget" {
				t.Fatal("restored evidence could not finish the interrupted plan", result, err)
			}
		})
	}
}

func TestCompletedConfigurationRemovalStaysReleased(t *testing.T) {
	for _, kind := range []string{"json", "configuration", "sentinel"} {
		t.Run(kind, func(t *testing.T) {
			var c Controller
			var path, resource, original string
			selected := "test"
			if kind == "json" {
				c, _, path = jsonSettingsController(t)
				resource, original = "profile.test", `{"theme":"personal","fontSize":16}`
			} else if kind == "sentinel" {
				var d *ProfileDriver
				c, d = sentinelController(t)
				path = d.Targets[sentinelResource][0].Path
				resource, selected = sentinelResource, "sentinel"
				original = "My prose.\n\n<!-- AGENT-RULES:BEGIN -->\nEarlier personal policy.\n" + sentinelEnd + "\nTrailing prose."
			} else {
				c, _, path = nativeConfigFixture(t, "copy")
				resource, original = "config.test", "personal original configuration"
			}
			writeConfigFixture(t, path, original)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{selected}, Adopt: []string{resource}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			for _, mode := range []string{"check", "update", "repair", "apply"} {
				request := Request{Schema: 1, Mode: mode}
				if mode == "apply" {
					request.Selected = []string{}
				}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if op := operation(t, plan, resource); op.Action != "forget" {
					t.Fatal("later lifecycle reactivated restored personal configuration", mode, op)
				}
				if mode != "check" {
					dispatchApproved(t, c, request)
				}
				if data, err := os.ReadFile(path); err != nil || string(data) != original {
					t.Fatal("later lifecycle changed the restored file", mode, string(data), err)
				}
			}
		})
	}
}
