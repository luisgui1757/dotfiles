package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveEditedPayloadReplacementUsesFreshGeneration(t *testing.T) {
	for _, damage := range []string{"edit", "missing-file", "missing-payload"} {
		t.Run(damage, func(t *testing.T) {
			c, d, requests := archiveController(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			old, err := d.PayloadPath("tool.shared")
			if err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "edit":
				writeConfigFixture(t, filepath.Join(old, "tool"), "personal bytes")
			case "missing-file":
				if err := os.Remove(filepath.Join(old, "tool")); err != nil {
					t.Fatal(err)
				}
			case "missing-payload":
				if err := os.RemoveAll(old); err != nil {
					t.Fatal(err)
				}
			}
			before, err := snapshotConfig(filepath.Dir(old))
			if err != nil {
				t.Fatal(err)
			}
			preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "repair"})
			if err != nil {
				t.Fatal(err)
			}
			op := operation(t, preview, "tool.shared")
			if !op.Observed.Adoptable || op.Action != "pending" {
				t.Fatal("missing explicit replacement choice", op)
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "repair", Adopt: []string{"tool.shared"}})
			active, err := d.PayloadPath("tool.shared")
			if err != nil || active == old || requests.Load() != 2 {
				t.Fatal("did not publish fresh generation", active, old, err, requests.Load())
			}
			if !strings.Contains(result.Message, filepath.Dir(old)) {
				t.Fatal("replacement did not disclose saved data", result)
			}
			for _, mode := range []string{"update", "remove", "reinstall"} {
				request := Request{Schema: 1, Mode: "update"}
				if mode != "update" {
					request = Request{Schema: 1, Mode: "apply", Selected: []string{}}
				}
				if mode == "reinstall" {
					request.Selected = []string{"first"}
				}
				dispatchApproved(t, c, request)
				if err := verifyConfigSnapshot(filepath.Dir(old), before); err != nil {
					t.Fatal("saved data changed after", mode, err)
				}
			}
		})
	}
}

func TestArchiveBrokenSavedOperationIsResourceLocalUnknown(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["tool.shared"]
	receipt.Status, receipt.OperationID = "in-progress", strings.Repeat("a", 64)
	writeConfigFixture(t, d.intentPath("tool.shared", receipt.OperationID), "{broken")
	r, _ := c.Catalog.Resource("tool.shared")
	o, err := d.Observe(context.Background(), r, receipt)
	if err != nil || !o.Unknown || o.Present || o.CompletedOperation != "" || o.Pending == "" {
		t.Fatal("invalid unknown observation", o, err)
	}
	state.Receipts[r.ID] = receipt
	if _, err := PlanChanges(c.Catalog, c.Context, state, Request{Schema: 1, Mode: "check"}, map[string]Observation{r.ID: o}, c.Source); err != nil {
		t.Fatal("damaged resource broke the planner", err)
	}
}

func TestArchivePreexistingPayloadCannotBeOfferedForReplacement(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	path, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(path, "tool"), "preexisting bytes")
	r, _ := c.Catalog.Resource("tool.shared")
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil || o.Adoptable {
		t.Fatal("offered ownership of unrelated preexisting data", o, err)
	}
}
