package installer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLinuxAPTPoolClaimRecoveryRequiresCompletedOriginalOperation(t *testing.T) {
	for _, stage := range []string{"before-ledger", "after-ledger", "incomplete-original"} {
		t.Run(stage, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := c.Catalog.Resource("library.libxcb-util1")
			receipt := state.Receipts[r.ID]
			intent, err := f.driver.intent(receipt.OperationID)
			if err != nil || intent.Claim == "" {
				t.Fatal("missing durable pool claim", intent, err)
			}
			intent.Complete = false
			if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
				t.Fatal(err)
			}
			if stage != "after-ledger" {
				ledger, err := f.driver.ledger()
				if err != nil {
					t.Fatal(err)
				}
				owned := ledger.Roots[r.ID]
				ledger.Pool[owned.Name] = owned
				delete(ledger.Roots, r.ID)
				if err := saveDocument(filepath.Join(f.driver.Directory, "packages.json"), ledger); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "incomplete-original" {
				original, err := f.driver.intent(intent.Claim)
				if err != nil {
					t.Fatal(err)
				}
				original.Complete = false
				if err := saveDocument(f.driver.intentPath(original.Operation), original); err != nil {
					t.Fatal(err)
				}
			}
			receipt.Status, receipt.Ownership = "in-progress", "uncertain"
			before, err := f.driver.Observe(context.Background(), r, receipt)
			if err != nil || before.ResourceResume == nil {
				t.Fatal("interrupted claim cannot be reviewed", before, err)
			}
			f.approve()
			got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
			if f.runs != 2 {
				t.Fatal("metadata claim reran native installation", f.runs)
			}
			if stage == "incomplete-original" {
				if err == nil {
					t.Fatal("incomplete original operation authorized a claim", got)
				}
				return
			}
			if err != nil || got.CompletedOperation != receipt.OperationID || !got.Healthy {
				t.Fatal("saved claim did not recover", got, err)
			}
		})
	}
}

func TestLinuxAPTRetainedRootRecoveryBindsExactPoolAndRemoval(t *testing.T) {
	for _, stage := range []string{"before-ledger", "after-ledger", "pool-tamper", "completion-tamper"} {
		t.Run(stage, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}, RemoveShared: []string{"library.libxcb-util1"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := c.Catalog.Resource("library.libxcb-util1")
			receipt := state.Receipts[r.ID]
			intent, err := f.driver.intent(receipt.OperationID)
			if err != nil || intent.Deferred == nil {
				t.Fatal("missing retained-root intent", intent, err)
			}
			ledger, err := f.driver.ledger()
			if err != nil {
				t.Fatal(err)
			}
			runs := f.runs
			if stage == "pool-tamper" {
				delete(ledger.Pool, intent.Deferred.Name)
				if err := saveDocument(filepath.Join(f.driver.Directory, "packages.json"), ledger); err != nil {
					t.Fatal(err)
				}
				got, err := f.driver.Observe(context.Background(), r, receipt)
				if err == nil && (got.RemovalDeferred || got.CompletedOperation == receipt.OperationID) {
					t.Fatal("missing pool ownership proved removal", got)
				}
				return
			}
			if stage == "completion-tamper" {
				intent.Commands = nil
				if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
					t.Fatal(err)
				}
				if _, err := f.driver.Observe(context.Background(), r, receipt); err == nil {
					t.Fatal("retained root completed without saved remove command proof")
				}
				return
			}
			intent.Complete = false
			if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
				t.Fatal(err)
			}
			if stage == "before-ledger" {
				ledger.Roots[r.ID] = *intent.Deferred
				delete(ledger.Pool, intent.Deferred.Name)
				if err := saveDocument(filepath.Join(f.driver.Directory, "packages.json"), ledger); err != nil {
					t.Fatal(err)
				}
			}
			receipt.Status, receipt.Ownership = "in-progress", "created"
			f.approve()
			got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "remove"}, receipt)
			if err != nil || !removalCompleted(receipt, got) || !got.Present || !got.RemovalDeferred || f.runs != runs {
				t.Fatal("retained-root recovery failed or repeated native work", got, err, f.runs)
			}
		})
	}
}

func TestRemovalDeferredRequiresExactCompletionAndLegacyAbsence(t *testing.T) {
	var legacy Observation
	if err := Decode([]byte(`{"present":true,"healthy":true,"provider":"apt","identity":"apt:library","fingerprint":"same"}`), &legacy); err != nil || legacy.RemovalDeferred {
		t.Fatal("legacy observation gained removal authority", legacy, err)
	}
	receipt := Receipt{Before: Observation{}, After: legacy, OperationID: "approved-remove"}
	for _, change := range []string{"no-proof", "foreign-proof", "no-disclosure", "changed-artifact", "pre-existing", "pending", "valid"} {
		t.Run(change, func(t *testing.T) {
			r := receipt
			after := legacy
			after.RemovalDeferred, after.CompletedOperation, after.Preserved = true, receipt.OperationID, []string{"apt:library:amd64"}
			switch change {
			case "no-proof":
				after.CompletedOperation = ""
			case "foreign-proof":
				after.CompletedOperation = "another-remove"
			case "no-disclosure":
				after.Preserved = nil
			case "changed-artifact":
				after.Fingerprint = "personal"
			case "pre-existing":
				r.Before = legacy
				after.Fingerprint = "overlay"
			case "pending":
				after.Pending = "native failure"
			}
			if removalCompleted(r, after) != (change == "valid") {
				t.Fatal("incorrect retained-root removal authority", change, after)
			}
		})
	}
}

func TestIncidentalIntroductionRequiresEarlierCompletionInExactPlan(t *testing.T) {
	for _, change := range []string{"historical-plan", "future-operation", "not-completed", "different-provider", "pre-existing", "valid"} {
		t.Run(change, func(t *testing.T) {
			plan := Plan{ID: "active", Operations: []Operation{{Resource: "first", Action: "install"}, {Resource: "later", Action: "install", Observed: Observation{Provider: "apt", Identity: "apt:later"}}}}
			id, err := digest(struct{ Plan, Resource, Action string }{plan.ID, "first", "install"})
			if err != nil {
				t.Fatal(err)
			}
			state := State{Receipts: map[string]Receipt{"first": {Status: "ready", OperationID: id, After: Observation{CompletedOperation: id}}}}
			fresh := Observation{Present: true, Healthy: true, Provider: "apt", Identity: "apt:later", CompletedOperation: id}
			index := 1
			op := plan.Operations[index]
			switch change {
			case "historical-plan":
				plan.ID = "different"
			case "future-operation":
				index = 0
			case "not-completed":
				state.Receipts["first"] = Receipt{Status: "in-progress", OperationID: id}
			case "different-provider":
				fresh.Provider = "outside"
			case "pre-existing":
				op.Observed.Present = true
			}
			if introducedByEarlierOperation(plan, state, index, op, fresh) != (change == "valid") {
				t.Fatal("unrelated proof widened approval", change)
			}
		})
	}
}

func TestLinuxAPTAutoPreservationDoesNotPreventMissingOrPartialRepair(t *testing.T) {
	for _, condition := range []string{"missing", "unpacked"} {
		t.Run(condition, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			r, _ := c.Catalog.Resource("library.libxcb-util1")
			if condition == "missing" {
				delete(f.installed, "libxcb-util1:amd64")
			} else {
				pkg := f.installed["libxcb-util1:amd64"]
				pkg.Healthy = false
				f.installed[pkg.Name] = pkg
			}
			// Native boundaries model dpkg's successful configuration of the
			// already unpacked package without requiring a version change.
			run := f.driver.Run
			f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				out, err := run(ctx, command)
				if slices.Contains(command.Arguments, "install") {
					pkg := f.installed["libxcb-util1:amd64"]
					pkg.Healthy = true
					f.installed[pkg.Name] = pkg
				}
				return out, err
			}
			f.approve()
			receipt := aptFixtureReceipt("repair-" + condition)
			got, err := f.driver.Apply(context.Background(), r, Operation{Action: "repair"}, receipt)
			if err != nil || !got.Healthy {
				t.Fatal("repair failed", got, err)
			}
			intent, err := f.driver.intent(receipt.OperationID)
			if err != nil || intent.PreserveAutomatic != (condition == "unpacked") {
				t.Fatal("wrong automatic preservation branch", intent, err)
			}
			ledger, err := f.driver.ledger()
			if err != nil || !f.driver.matchesOwnership(ledger.Roots[r.ID], f.installed["libxcb-util1:amd64"]) {
				t.Fatal("repair lost provable root ownership", ledger, err)
			}
		})
	}
}

func TestLinuxAPTManualIncidentalDoesNotGainRootRemovalAuthority(t *testing.T) {
	f := newLinuxAPTFixture(t)
	run := f.driver.Run
	f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		out, err := run(ctx, command)
		if err != nil || !slices.Contains(command.Arguments, "install") {
			return out, err
		}
		pkg := f.installed["shared:amd64"]
		pkg.Automatic = false
		f.installed[pkg.Name] = pkg
		path := filepath.Join(f.driver.Directory, "logs", command.Operation+".history")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(path, []byte(strings.ReplaceAll(string(data), ", automatic", "")), 0600)
		return out, err
	}
	f.approve()
	if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: "install"}, aptFixtureReceipt("manual-incidental")); err != nil {
		t.Fatal(err)
	}
	f.approve()
	got, err := f.driver.Remove(context.Background(), aptFixtureResource("first"), aptFixtureReceipt("remove-manual-incidental"))
	if err != nil || !slices.Contains(got.Preserved, "apt:shared:amd64") {
		t.Fatal("manual incidental was collected as an explicit root", got, err)
	}
	if _, present := f.installed["shared:amd64"]; !present {
		t.Fatal("removed a native manual incidental")
	}
}
