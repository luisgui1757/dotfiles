package installer

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

func completedRetryOperation(state State, id string, receipt Receipt, observed Observation) bool {
	if state.Transaction == nil || observed.CompletedOperation == "" || observed.CompletedOperation != receipt.OperationID || receipt.Status != "ready" {
		return false
	}
	for _, op := range state.Transaction.Plan.Operations {
		if op.Resource == id && op.Action == "update" {
			expected, err := digest(struct{ Plan, Resource, Action string }{state.Transaction.Plan.ID, id, op.Action})
			return err == nil && receipt.OperationID == expected
		}
	}
	return false
}

// ResourceResume binds an explicit recovery approval to the original operation
// and the provider's current partial publication. It cannot authorize rebuilding
// from new source bytes or acquiring an unrelated existing artifact.
type ResourceResume struct {
	Operation string `json:"operation"`
	Token     string `json:"token"`
}

type ResourceResumeDriver interface {
	ResumeResource(context.Context, Resource, Operation, Receipt) (Observation, error)
}

func planResourceResume(plan Plan, state State, observed map[string]Observation) (Plan, bool, error) {
	transaction := state.Transaction
	if transaction == nil || transaction.InFlight == "" {
		return plan, false, nil
	}
	id := transaction.InFlight
	o, found := observed[id]
	if found && o.ResourceResume == nil && o.Pending != "" {
		return plan, false, fmt.Errorf("%s: %s; preserve the saved operation, resolve this problem or choose an available recovery action", id, o.Pending)
	}
	if !found || o.ResourceResume == nil {
		return plan, false, nil
	}
	resume := *o.ResourceResume
	receipt, recorded := state.Receipts[id]
	if !operationID.MatchString(resume.Operation) || !operationID.MatchString(resume.Token) || !recorded ||
		receipt.OperationID != resume.Operation || receipt.Status != "in-progress" || plan.Mode != "restore" && o.Pending == "" ||
		plan.Mode != "restore" && (transaction.Plan.Source != plan.Source || transaction.Plan.Catalog != plan.Catalog) || transaction.Plan.Target != plan.Target || transaction.Plan.Context != plan.Context {
		return plan, false, errors.New("resource recovery differs from the original source, target or durable intent")
	}
	if transaction.Next < 0 || transaction.Next >= len(transaction.Plan.Operations) {
		return plan, false, errors.New("resource recovery has no original operation position")
	}
	original := transaction.Plan.Operations[transaction.Next]
	expected, err := digest(struct{ Plan, Resource, Action string }{transaction.Plan.ID, id, original.Action})
	if err != nil || original.Resource != id || !mutating(original.Action) || expected != resume.Operation {
		return plan, false, errors.Join(errors.New("resource recovery operation differs from its approved intent"), err)
	}
	original.Observed, original.Reason = o, "resume the exact saved resource operation; preserve its original baseline"
	if plan.Mode == "restore" {
		original.Action = "restore"
		original.Reason = "restore from saved file intent; preserve conflicting files and archive the interrupted transaction"
	}
	plan.ResourceResume = &resume
	plan.Selected, plan.Keep = slices.Clone(transaction.Plan.Selected), slices.Clone(transaction.Plan.Keep)
	plan.Operations = []Operation{original}
	plan.ID, err = digest(plan)
	return plan, true, err
}

func executeResourceResume(ctx context.Context, c *Catalog, path string, state *State, plan Plan, driver Driver) (Result, error) {
	result := Result{Status: "failed", Plan: plan}
	provider, ok := driver.(ResourceResumeDriver)
	if !ok || len(plan.Operations) != 1 || plan.ResourceResume == nil {
		return result, errors.New("provider cannot resume this saved resource operation")
	}
	op := plan.Operations[0]
	r, _ := c.Resource(op.Resource)
	receipt := state.Receipts[r.ID]
	before, err := driver.Observe(ctx, r, receipt)
	if err != nil || before.ResourceResume == nil || *before.ResourceResume != *plan.ResourceResume || !sameArtifact(before, op.Observed) {
		return result, errors.Join(errors.New("partial publication changed after recovery approval"), err)
	}
	after, err := provider.ResumeResource(ctx, r, op, receipt)
	if err != nil {
		return result, err
	}
	verified, err := driver.Observe(ctx, r, receipt)
	if err != nil || !sameArtifact(after, verified) || verified.Pending != "" || verified.ResourceResume != nil {
		return result, errors.Join(errors.New("resource recovery did not verify a complete publication"), err)
	}
	if op.Action == "remove" {
		if !removalCompleted(receipt, verified) {
			return result, errors.New("resource recovery did not restore its original baseline")
		}
		receipt = removedReceipt(receipt, verified)
	} else {
		if !verified.Present || !verified.Healthy || verified.Provider == "" || verified.Identity == "" || verified.Fingerprint == "" || verified.CompletedOperation != receipt.OperationID {
			return result, errors.New("resource recovery lacks exact operation completion proof")
		}
		if receipt.Ownership == "uncertain" && (!receipt.Before.Present || receipt.Adopted) {
			receipt.Ownership = "created"
		}
		receipt.After, receipt.Status, receipt.Reason = verified, "ready", ""
	}
	state.Receipts[r.ID] = receipt
	state.Transaction.Next++
	state.Transaction.InFlight, state.Transaction.Error = "", ""
	delete(state.Transaction.Pending, r.ID)
	if err := SaveState(path, *state); err != nil {
		return result, err
	}
	result.Status, result.Message = "needs-action", "saved operation recovered; review the remaining work against the verified result"
	return result, nil
}
