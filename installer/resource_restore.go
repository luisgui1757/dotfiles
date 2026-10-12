package installer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// Restoration is deliberately separate from forward retry: it consumes only
// supported saved intent, without trusting or requiring today's source/catalog.
type ResourceRestoreDriver interface {
	ObserveRestore(context.Context, string, Receipt) (Observation, error)
	RestoreResource(context.Context, string, Observation, Receipt) error
}

var ErrNoResourceRestore = errors.New("no started file publication can be restored; use abandonment for unstarted intent")

func executeResourceRestore(ctx context.Context, path string, state *State, plan Plan, driver Driver) (Result, error) {
	result := Result{Status: "failed", Plan: plan}
	provider, ok := driver.(ResourceRestoreDriver)
	if !ok || plan.ResourceResume == nil || len(plan.Operations) != 1 {
		return result, errors.New("provider cannot restore this saved file operation")
	}
	op := plan.Operations[0]
	receipt := state.Receipts[op.Resource]
	before, err := provider.ObserveRestore(ctx, op.Resource, receipt)
	if err != nil || before.ResourceResume == nil || *before.ResourceResume != *plan.ResourceResume {
		return result, errors.Join(errors.New("recovery files changed after approval"), err)
	}
	if err := provider.RestoreResource(ctx, op.Resource, before, receipt); err != nil {
		return result, err
	}
	verified, err := provider.ObserveRestore(ctx, op.Resource, receipt)
	if err != nil || verified.RestoredOperation != receipt.OperationID {
		return result, errors.Join(errors.New("file restoration lacks saved-operation proof"), err)
	}
	// Do not replace the old ownership fingerprint with whatever was restored.
	// A preserved user edit must still require explicit adoption next time.
	receipt.After.Preserved = sortedUnique(append(receipt.After.Preserved, verified.Preserved...))
	state.Receipts[op.Resource] = receipt
	// Save the disclosure before archiving, so another interruption cannot
	// strand the user's retained paths outside the ordinary state history.
	if err := SaveState(path, *state); err != nil {
		return result, err
	}
	archive, err := archiveTransaction(filepath.Dir(path), *state.Transaction)
	if err != nil {
		return result, err
	}
	state.PastTransactions = sortedUnique(append(state.PastTransactions, archive))
	receipt.Status, receipt.Reason = "needs-action", "interrupted files restored; review the next selection"
	if verified.Healthy && sameArtifact(receipt.After, verified) {
		receipt.Status, receipt.Reason = "ready", ""
	}
	state.Receipts[op.Resource] = receipt
	state.Transaction = nil
	state.Generation++
	if err := SaveState(path, *state); err != nil {
		return result, err
	}
	result.Status, result.Message = "ready", "interrupted files recovered and transaction archived; review a new selection before continuing"
	if verified.Pending != "" {
		result.Status, result.Message = "needs-action", verified.Pending
	}
	for _, saved := range verified.Preserved {
		result.Message += "\nPreserved file: " + saved
	}
	result.Message = strings.TrimSpace(result.Message)
	return result, nil
}
