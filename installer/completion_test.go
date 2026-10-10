package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This boundary returns a healthy artifact but deliberately supplies no
// transaction evidence tying its creation to the approved operation.
type unprovedCompletionDriver struct{ *fileDriver }

func (d *unprovedCompletionDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Apply(ctx, r, op, receipt)
	return Observation{Present: o.Present, Healthy: o.Healthy, Provider: o.Provider, Identity: o.Identity, Fingerprint: o.Fingerprint}, err
}

func pendingCompletionFixture(t *testing.T) (*Catalog, *fileDriver, Request, string) {
	t.Helper()
	c, d := engineFixture(t)
	d.absentPending = true
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "needs-action" {
		t.Fatalf("pending install: %+v %v", result, err)
	}
	// Decode the serialized contract so the same regression can exercise an
	// older executable that did not persist provider operation identities.
	data, err := os.ReadFile(d.statePath)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Receipts map[string]struct {
			OperationID string `json:"operation_id"`
		} `json:"receipts"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	id := saved.Receipts["node"].OperationID
	if len(id) != 64 {
		t.Fatal("pending mutation has no durable operation identity")
	}
	d.absentPending = false
	request.Retry = true
	return c, d, request, id
}

// Model a provider finishing its already journaled transaction after reboot.
// The proof binds the exact artifact, not merely its location or existence.
func completeAfterRestart(t *testing.T, c *Catalog, d *fileDriver, id string) {
	t.Helper()
	path := filepath.Join(d.dir, "node")
	if err := os.WriteFile(path, []byte("provider completed after reboot"), 0600); err != nil {
		t.Fatal(err)
	}
	r, _ := c.Resource("node")
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document["completed_operation"] = id
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".completion", data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProvedRebootCompletionRecoversOwnedResourceForRemoval(t *testing.T) {
	c, d, request, id := pendingCompletionFixture(t)
	completeAfterRestart(t, c, d, id)
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "ready" {
		t.Fatalf("retry: %+v %v", result, err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["node"].Ownership != "created" {
		t.Fatalf("proved completion did not recover ownership: %+v %v", state, err)
	}
	request.Retry, request.Selected, request.RemoveShared = false, []string{}, []string{"node"}
	result, err = applyFixture(c, d, approve(t, c, d, request))
	if err != nil || operation(t, result.Plan, "node").Action != "remove" {
		t.Fatalf("proved completed resource could not be removed: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(d.dir, "node")); !os.IsNotExist(err) {
		t.Fatalf("proved removal left the artifact: %v", err)
	}
}

func TestMismatchedRebootEvidenceCannotRecoverOwnership(t *testing.T) {
	for _, mismatch := range []string{"operation", "content", "provider", "identity"} {
		t.Run(mismatch, func(t *testing.T) {
			c, d, request, id := pendingCompletionFixture(t)
			if mismatch == "operation" {
				id = strings.Repeat("f", 64)
			}
			completeAfterRestart(t, c, d, id)
			if mismatch == "content" {
				if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("external replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if mismatch == "provider" || mismatch == "identity" {
				path := filepath.Join(d.dir, "node.completion")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var document map[string]any
				if err := json.Unmarshal(data, &document); err != nil {
					t.Fatal(err)
				}
				document[mismatch] = "unrelated"
				data, err = json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
				t.Fatal(err)
			}
			request.Retry, request.Selected, request.RemoveShared = false, []string{}, []string{"node"}
			result, err := applyFixture(c, d, approve(t, c, d, request))
			if err != nil || operation(t, result.Plan, "node").Action != "retain" {
				t.Fatalf("mismatched evidence authorized removal: %+v %v", result, err)
			}
		})
	}
}

func TestLegacyJournalWithoutModeOrOperationCanOnlyBeAbandoned(t *testing.T) {
	c, d, _, _ := pendingCompletionFixture(t)
	data, err := os.ReadFile(d.statePath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	delete(document["transaction"].(map[string]any)["plan"].(map[string]any), "mode")
	delete(document["receipts"].(map[string]any)["node"].(map[string]any), "operation_id")
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.statePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Observe(context.Background(), c, linux, state, Request{Schema: 1, Mode: "apply", Retry: true}, d)
	if err == nil || !strings.Contains(err.Error(), "legacy transaction") {
		t.Fatalf("legacy journal was silently reinterpreted: %v", err)
	}
	result, err := applyFixture(c, d, approve(t, c, d, Request{Schema: 1, Mode: "abandon"}))
	if err != nil || result.Status != "ready" || len(d.calls) != 1 {
		t.Fatalf("legacy abandonment: %+v %v", result, err)
	}
	state, err = LoadState(d.statePath, d.home)
	if err != nil || state.Transaction != nil || len(state.PastTransactions) != 1 || state.Receipts["node"].Ownership != "uncertain" {
		t.Fatalf("legacy abandonment lost original evidence: %+v %v", state, err)
	}
}

func TestArtifactAppearanceAloneCannotAcquireRemovalOwnership(t *testing.T) {
	c, d := engineFixture(t)
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &unprovedCompletionDriver{d})
	if err == nil || result.Status == "ready" {
		t.Fatalf("unproved creation was accepted: %+v, %v", result, err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Transaction == nil || state.Receipts["node"].Ownership != "uncertain" {
		t.Fatalf("unproved creation acquired removal authority: %+v, %v", state, err)
	}
}

func TestRetryCannotReinterpretIntentUsingAnotherSourceRevision(t *testing.T) {
	c, d, request, _ := pendingCompletionFixture(t)
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(context.Background(), c, linux, state, request, d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanChanges(c, linux, state, request, observed, "different-source"); err == nil || !strings.Contains(err.Error(), "source revision") {
		t.Fatalf("retry silently reinterpreted an interrupted source: %v", err)
	}
	// Explicit abandonment still lets a current installer preserve the original
	// evidence before the user reviews a separate plan from the new revision.
	if _, err := PlanChanges(c, linux, state, Request{Schema: 1, Mode: "abandon"}, nil, "different-source"); err != nil {
		t.Fatalf("source mismatch blocked explicit safe abandonment: %v", err)
	}
}

func TestCompletedOriginalUpdateIsNotRepeatedByRetry(t *testing.T) {
	c, d := inventoryFixture(t)
	for _, request := range []Request{{Schema: 1, Mode: "apply", Selected: []string{"editor"}}, {Schema: 1, Mode: "update"}} {
		result, err := applyInventoryFixture(t, c, d, request)
		if err != nil || result.Status != "ready" {
			t.Fatal(result.Status, err)
		}
		if request.Mode != "update" {
			continue
		}
		state, err := LoadState(d.statePath, d.home)
		if err != nil {
			t.Fatal(err)
		}
		state.Transaction = &Transaction{Plan: result.Plan, Next: len(result.Plan.Operations)}
		observed, err := Observe(context.Background(), c, linux, state, Request{Schema: 1, Mode: "update", Retry: true}, d)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := PlanChanges(c, linux, state, Request{Schema: 1, Mode: "update", Retry: true}, observed, "source")
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range plan.Operations {
			if op.Resource == "node" || op.Resource == "fzf" {
				if op.Action != "keep" {
					t.Fatal("retry repeated a provider-proved original update", op)
				}
			}
		}
	}
}
