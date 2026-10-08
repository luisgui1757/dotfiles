package installer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// Driver is the external system boundary. Production adapters must probe real
// consumption, publish with their scoped recovery contract, and verify writes.
type Driver interface {
	Observe(context.Context, Resource, Receipt) (Observation, error)
	Apply(context.Context, Resource, Operation, Receipt) (Observation, error)
	Remove(context.Context, Resource, Receipt) (Observation, error)
}

// SelectionDriver binds generated configuration to the resolved graph for this
// request. It returns an independent adapter without writing to the filesystem.
// State is the already-loaded request snapshot and must not be mutated.
type SelectionDriver interface {
	ForSelection([]string, State) (Driver, error)
}

func driverForSelection(c *Catalog, target Context, state State, request Request, driver Driver) (Driver, error) {
	bound, ok := driver.(SelectionDriver)
	if !ok || request.Mode == "restore" || request.Mode == "abandon" {
		return driver, nil
	}
	effective, err := effectiveRequest(state, request)
	if err != nil {
		return nil, err
	}
	wanted, err := c.Closure(append(append([]string{}, effective.Selected...), effective.Keep...), target)
	if err != nil {
		return nil, err
	}
	return bound.ForSelection(wanted, state)
}

func mutating(action string) bool {
	return action == "install" || action == "repair" || action == "update" || action == "remove" || action == "adopt"
}

// InventoryDriver may discover additional managed members before planning.
// Inventory hashes in either its results or ordinary resource observations bind
// protected provider state to approval. VerifyInventory checks that state before
// each mutation and at finalization, allowing only provider-proved changes.
type InventoryDriver interface {
	ObserveInventory(context.Context, *Catalog, Context, State) (map[string]Observation, error)
	VerifyInventory(context.Context, *Catalog, Context, State, Plan) error
}

// MutationGuard protects providers whose processes can survive the controller.
// It runs before observation or journal changes, including explicit abandonment.
// Acquire it after the engine lock, and release it after all journal writes.
type MutationGuard interface {
	AcquireMutation(context.Context) (func() error, error)
}

// TransactionDriver seals provider-side completion before the core clears its
// intent. Recovery may consist entirely of keep/forget operations, without
// invoking Apply again. This hook runs only after approval, under both guards.
type TransactionDriver interface {
	FinishTransaction(context.Context, Plan, map[string]Receipt) (map[string][]string, error)
}

type Result struct {
	Status  string `json:"status"`
	Plan    Plan   `json:"plan"`
	Message string `json:"message,omitempty"`
}

func Observe(ctx context.Context, c *Catalog, target Context, state State, request Request, driver Driver) (map[string]Observation, error) {
	request, err := effectiveRequest(state, request)
	if err != nil {
		return nil, err
	}
	if request.Mode == "abandon" {
		return map[string]Observation{}, nil
	}
	if request.Mode == "restore" {
		provider, ok := driver.(ResourceRestoreDriver)
		if !ok || state.Transaction.InFlight == "" {
			return nil, errors.New("this interrupted provider has no file restoration action")
		}
		id := state.Transaction.InFlight
		o, err := provider.ObserveRestore(ctx, id, state.Receipts[id])
		return map[string]Observation{id: o}, err
	}
	driver, err = driverForSelection(c, target, state, request, driver)
	if err != nil {
		return nil, err
	}
	return observeResources(ctx, c, target, state, request, driver)
}

// The executor supplies its already approved selection for final verification.
// It must not reinterpret the newly written journal as a fresh recovery request.
func observeResources(ctx context.Context, c *Catalog, target Context, state State, request Request, driver Driver) (map[string]Observation, error) {
	roots := append(append([]string{}, request.Selected...), request.Keep...)
	for id := range state.Receipts {
		roots = append(roots, id)
	}
	inventory := map[string]Observation{}
	if provider, ok := driver.(InventoryDriver); ok {
		var err error
		inventory, err = provider.ObserveInventory(ctx, c, target, state)
		if err != nil {
			return nil, fmt.Errorf("inspect complete provider inventory: %w", err)
		}
		for id, o := range inventory {
			r, exists := c.Resource(id)
			if !exists || r.Action == "" || !r.Available(target) || !operationID.MatchString(o.Inventory) {
				return nil, fmt.Errorf("invalid or unmapped provider member %q", id)
			}
			if o.Present {
				roots = append(roots, id)
			}
		}
	}
	ids, err := c.Closure(roots, target)
	if err != nil {
		return nil, err
	}
	// Absent, unselected inventory members still bind the provider snapshot to
	// approval; their unused native prerequisites are outside this discovery.
	observed := maps.Clone(inventory)
	for _, id := range ids {
		r, _ := c.Resource(id)
		if r.Action == "" {
			continue
		}
		o, inventoried := inventory[id]
		if !inventoried {
			o, err = driver.Observe(ctx, r, state.Receipts[r.ID])
			if err != nil {
				return nil, fmt.Errorf("inspect %s: %w", r.Name, err)
			}
		}
		if o.Present && (o.Fingerprint == "" || o.Identity == "" || o.Provider == "") {
			return nil, fmt.Errorf("%s probe returned incomplete ownership evidence", r.Name)
		}
		o.Consumers = sortedUnique(o.Consumers)
		observed[id] = o
	}
	return observed, nil
}

// Execute requires a freshly recomputed plan with the exact approved identity.
// A single OS lock covers reobservation, journal writes and every mutation.
func Execute(ctx context.Context, c *Catalog, target Context, source, home, statePath string, request Request, driver Driver) (result Result, resultErr error) {
	result = Result{Status: "failed"}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if request.ExpectedPlan == "" {
		return result, errors.New("apply requires the exact reviewed plan identity")
	}
	if request.Mode == "plan" || request.Mode == "check" {
		return result, errors.New("read-only request cannot apply changes")
	}
	unlock, err := Lock(filepath.Dir(statePath))
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, unlock())
		if resultErr != nil {
			result.Status = "failed"
		}
	}()
	if provider, ok := driver.(MutationGuard); ok {
		release, err := provider.AcquireMutation(ctx)
		if err != nil {
			return result, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, release())
			if resultErr != nil {
				result.Status = "failed"
			}
		}()
	}
	state, err := LoadState(statePath, home)
	if err != nil {
		return result, err
	}
	journalStarted := false
	defer func() {
		if resultErr != nil && journalStarted && state.Transaction != nil {
			state.Transaction.Error = resultErr.Error()
			resultErr = errors.Join(resultErr, SaveState(statePath, state))
		}
	}()
	driver, err = driverForSelection(c, target, state, request, driver)
	if err != nil {
		return result, err
	}
	observed, err := Observe(ctx, c, target, state, request, driver)
	if err != nil {
		return result, err
	}
	plan, err := PlanChanges(c, target, state, request, observed, source)
	if err != nil {
		return result, err
	}
	result.Plan = plan
	if plan.ID != request.ExpectedPlan {
		return result, errors.New("machine or selection changed; review a fresh plan before applying")
	}
	if plan.ResourceResume != nil {
		journalStarted = true
		if plan.Mode == "restore" {
			return executeResourceRestore(ctx, statePath, &state, plan, driver)
		}
		return executeResourceResume(ctx, c, statePath, &state, plan, driver)
	}
	state.Schema, state.Context, state.Target, state.Source = 1, target, home, source
	if state.Receipts == nil {
		state.Receipts = map[string]Receipt{}
	}
	if state.Transaction != nil {
		archive, err := archiveTransaction(filepath.Dir(statePath), *state.Transaction)
		if err != nil {
			return result, err
		}
		state.PastTransactions = sortedUnique(append(state.PastTransactions, archive))
	}
	if request.Mode == "abandon" {
		preserved, err := finalizeTransaction(ctx, driver, plan, &state)
		if err != nil {
			return result, err
		}
		for id, receipt := range state.Receipts {
			if receipt.Status == "in-progress" {
				receipt.Status, receipt.Reason = "needs-action", "interrupted operation abandoned; inspect preserved resources before taking ownership"
				state.Receipts[id] = receipt
			}
		}
		state.Transaction = nil
		state.Generation++
		if err := SaveState(statePath, state); err != nil {
			return result, err
		}
		result.Status, result.Message = "ready", "transaction archived; resources and ownership receipts preserved"
		for _, path := range preserved {
			result.Message += "\nPreserved file: " + path
		}
		return result, nil
	}
	state.Transaction = &Transaction{Plan: plan}
	if err := SaveState(statePath, state); err != nil {
		return result, err
	}
	journalStarted = true
	pending := false
	blocked := map[string]string{}
	removed := map[string]Receipt{}
	for index := 0; index < len(plan.Operations); index++ {
		op := plan.Operations[index]
		if err := ctx.Err(); err != nil {
			return result, err
		}
		r, _ := c.Resource(op.Resource)
		if op.Action == "install" || op.Action == "repair" || op.Action == "update" || op.Action == "adopt" || op.Action == "keep" {
			dependencies, err := c.Closure(r.Dependencies(target), target)
			if err != nil {
				return result, err
			}
			for _, id := range dependencies {
				if reason, waiting := blocked[id]; waiting {
					op.Action, op.Reason = "pending", "waiting for "+id+": "+reason
					break
				}
			}
		}
		receipt, exists := state.Receipts[r.ID]
		if request.Retry {
			receipt = recoverCompletion(receipt, op.Observed)
		}
		if !exists {
			receipt = Receipt{Ownership: "reused", Before: op.Observed, After: op.Observed, Status: "ready"}
		}
		switch op.Action {
		case "retain", "keep":
			receipt.Status, receipt.Reason = "retained", op.Reason
			if op.Action == "keep" {
				receipt.Status = "ready"
			}
			state.Receipts[r.ID] = receipt
		case "pending":
			pending = true
			blocked[r.ID] = op.Reason
			receipt.Status, receipt.Reason = "needs-action", op.Reason
			if exists || op.Observed.Present {
				state.Receipts[r.ID] = receipt
			}
		case "forget":
			state.Receipts[r.ID] = removedReceipt(receipt, op.Observed)
		case "install", "repair", "update", "adopt", "remove":
			if provider, ok := driver.(InventoryDriver); ok {
				if err := provider.VerifyInventory(ctx, c, target, state, plan); err != nil {
					return result, fmt.Errorf("provider inventory changed before %s: %w", r.Name, err)
				}
			}
			fresh, err := driver.Observe(ctx, r, receipt)
			if err != nil {
				return result, err
			}
			fresh.Consumers = sortedUnique(fresh.Consumers)
			introduced := introducedByEarlierOperation(plan, state, index, op, fresh)
			if !sameArtifact(op.Observed, fresh) && !introduced || !slices.Equal(op.Observed.Consumers, fresh.Consumers) || op.Action != "remove" && op.Observed.Desired != fresh.Desired {
				return result, fmt.Errorf("%s changed after approval; review a new plan", r.Name)
			}
			if op.Action != "remove" && fresh.Pending != "" {
				pending = true
				blocked[r.ID] = fresh.Pending
				break
			}
			if op.Action != "remove" && fresh.ApplyBlocked != "" {
				return result, fmt.Errorf("%s cannot be applied: %s", r.Name, fresh.ApplyBlocked)
			}
			if op.Action == "repair" && fresh.Healthy {
				receipt.After, receipt.Status, receipt.Reason = fresh, "ready", "prerequisite repair restored health"
				state.Receipts[r.ID] = receipt
				break
			}
			if op.Action == "adopt" && !fresh.Adoptable {
				return result, errors.New("provider can no longer preserve the baseline for adoption")
			}
			replacingOwned := op.Action == "adopt" && receipt.Ownership == "created" && receipt.Recovery != "" && receipt.After.Provider == fresh.Provider
			reclaiming := op.Action == "install" && receipt.After.RemovalDeferred && removalCompleted(receipt, fresh)
			acquiringOwnership := reclaiming || op.Action == "adopt" && !replacingOwned || (!fresh.Present || introduced) && receipt.Ownership != "created"
			if acquiringOwnership {
				before := fresh
				if reclaiming {
					before = receipt.Before
				}
				if introduced {
					before = op.Observed
				}
				receipt = Receipt{Ownership: "uncertain", Before: before, After: Observation{Preserved: slices.Clone(fresh.Preserved)}, Adopted: op.Action == "adopt"}
			}
			if receipt.Recovery == "" {
				receipt.Recovery = filepath.Join("recovery", plan.ID, r.ID)
			}
			if op.Action == "remove" {
				// Restoration can be required even after the user deletes the
				// installed overlay. Bind removal to the just-approved target.
				receipt.After = fresh
			}
			receipt.OperationID, err = digest(struct{ Plan, Resource, Action string }{plan.ID, r.ID, op.Action})
			if err != nil {
				return result, err
			}
			receipt.Status = "in-progress"
			state.Receipts[r.ID] = receipt
			state.Transaction.Next, state.Transaction.InFlight = index, r.ID
			if err := SaveState(statePath, state); err != nil {
				return result, err
			}
			var after Observation
			if op.Action == "remove" {
				after, err = driver.Remove(ctx, r, receipt)
				if err == nil {
					after, err = driver.Observe(ctx, r, receipt)
				}
				if err == nil && !removalCompleted(receipt, after) {
					err = fmt.Errorf("%s removal did not restore its baseline or prove absence", r.Name)
				}
			} else {
				after, err = driver.Apply(ctx, r, op, receipt)
			}
			if err == nil && op.Action != "remove" && after.Pending == "" && (!after.Present || !after.Healthy) {
				err = fmt.Errorf("%s did not pass post-install verification", r.Name)
			}
			if err == nil && after.Present && (after.Identity == "" || after.Provider == "" || after.Fingerprint == "") {
				err = fmt.Errorf("%s returned incomplete post-operation evidence", r.Name)
			}
			if err == nil && op.Action != "remove" && after.Present && after.CompletedOperation != receipt.OperationID {
				err = fmt.Errorf("%s lacks provider-bound completion evidence for the approved operation", r.Name)
			}
			if err != nil {
				return result, fmt.Errorf("%s: %w; recovery journal retained", r.Name, err)
			}
			if op.Action == "remove" {
				removed[r.ID] = receipt
				state.Receipts[r.ID] = removedReceipt(receipt, after)
			} else {
				if acquiringOwnership && after.Present {
					receipt.Ownership = "created"
				}
				receipt.After, receipt.Status, receipt.Reason = after, "ready", ""
				if after.Pending != "" {
					pending = true
					blocked[r.ID] = after.Pending
					receipt.Status, receipt.Reason = "needs-action", after.Pending
				}
				state.Receipts[r.ID] = receipt
			}
		default:
			return result, fmt.Errorf("unknown planned action %q", op.Action)
		}
		state.Transaction.Next, state.Transaction.InFlight = index+1, ""
		state.Transaction.Pending = blocked
		if err := SaveState(statePath, state); err != nil {
			return result, err
		}
		if r.ReplanAfter && (op.Action == "install" || op.Action == "repair" || op.Action == "update") && blocked[r.ID] == "" {
			// Preserve original selections and intent. The bootstrap receipt is
			// complete, but newly available inventory has not been approved and
			// the whole requested installation has not yet been verified.
			result.Status = "needs-action"
			result.Message = r.Name + " is ready. Resume to review the newly available provider inventory before continuing."
			return result, nil
		}
	}
	// Earlier operations can affect later checks (PATH, providers, runtimes).
	// Read-only verification must prove the resulting selection before readiness.
	final, err := observeResources(ctx, c, target, state, Request{Selected: plan.Selected, Keep: plan.Keep}, driver)
	if err != nil {
		return result, err
	}
	wanted, err := c.Closure(append(append([]string{}, plan.Selected...), plan.Keep...), target)
	if err != nil {
		return result, err
	}
	for _, id := range wanted {
		r, _ := c.Resource(id)
		if r.Action == "" || blocked[id] != "" {
			continue
		}
		o := final[id]
		if o.Pending != "" {
			pending = true
			blocked[id] = o.Pending
			receipt := state.Receipts[id]
			receipt.Status, receipt.Reason = "needs-action", o.Pending
			state.Receipts[id] = receipt
		} else if !o.Present || !o.Healthy {
			return result, fmt.Errorf("%s failed final verification; recovery journal retained", r.Name)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(removed)) {
		r, _ := c.Resource(id)
		receipt := removed[id]
		after, err := driver.Observe(ctx, r, receipt)
		if err != nil || !removalCompleted(receipt, after) {
			receipt.Status, receipt.Reason = "failed", "removed resource reappeared or could not be verified"
			state.Receipts[id] = receipt
			return result, errors.Join(fmt.Errorf("%s failed final removal verification", r.Name), err)
		}
		state.Receipts[id] = removedReceipt(receipt, after)
	}
	// A successful consumer can still have damaged another resource. Verify
	// identities for every retained/kept artifact, including unselected ones,
	// and recheck the post-operation identity of every completed mutation.
	for _, op := range plan.Operations {
		if blocked[op.Resource] != "" {
			continue
		}
		expected := op.Observed
		switch op.Action {
		case "keep", "retain":
		case "install", "repair", "update", "adopt":
			expected = state.Receipts[op.Resource].After
		default:
			continue
		}
		after, found := final[op.Resource]
		if !found || !sameArtifact(expected, after) || expected.Healthy && !after.Healthy || expected.Pending == "" && after.Pending != "" {
			receipt := state.Receipts[op.Resource]
			receipt.Status, receipt.Reason = "failed", "final preservation check found an unapproved resource change"
			state.Receipts[op.Resource] = receipt
			return result, fmt.Errorf("%s failed final preservation verification; recovery journal retained", op.Resource)
		}
	}
	if provider, ok := driver.(InventoryDriver); ok {
		if err := provider.VerifyInventory(ctx, c, target, state, plan); err != nil {
			return result, fmt.Errorf("final provider inventory verification failed; recovery journal retained: %w", err)
		}
	}
	// A later removal may collect a dependency retained by an earlier one.
	// Refresh disclosures from the final provider observation, without changing
	// ownership or discarding a report when the artifact cannot be verified.
	for id, receipt := range state.Receipts {
		if after, ok := final[id]; ok && receipt.Status == "removed" && !after.Unknown {
			if receipt.After.RemovalDeferred && removalCompleted(receipt, after) {
				state.Receipts[id] = removedReceipt(receipt, after)
			} else if sameArtifact(receipt.After, after) {
				receipt.After.Preserved = slices.Clone(after.Preserved)
				state.Receipts[id] = receipt
			}
		}
	}
	preserved, err := finalizeTransaction(ctx, driver, plan, &state)
	if err != nil {
		return result, fmt.Errorf("provider finalization failed; recovery journal retained: %w", err)
	}
	state.Selected, state.Keep = plan.Selected, plan.Keep
	state.Generation++
	if pending {
		state.Transaction.Pending = blocked
	} else {
		state.Transaction = nil
		for id, receipt := range state.Receipts {
			if receipt.Status == "removed" && !receipt.After.Present && len(receipt.After.Preserved) == 0 {
				delete(state.Receipts, id)
			}
		}
	}
	if err := SaveState(statePath, state); err != nil {
		return result, err
	}
	result.Status = "ready"
	if pending {
		reasons := []string{}
		for id, reason := range blocked {
			reasons = append(reasons, id+": "+reason)
		}
		for id, receipt := range state.Receipts {
			if receipt.Status == "needs-action" {
				reasons = append(reasons, id+": "+receipt.Reason)
			}
		}
		result.Status, result.Message = "needs-action", strings.Join(sortedUnique(reasons), "; ")
	}
	for _, path := range sortedUnique(append(preserved, preservedPaths(state)...)) {
		result.Message += "\nPreserved files for inspection: " + path
	}
	unverified := []string{}
	for _, observation := range final {
		unverified = append(unverified, observation.UnverifiedApplications...)
	}
	for _, name := range sortedUnique(unverified) {
		result.Message += "\nExternal application not exercised: " + name
	}
	result.Message = strings.TrimSpace(result.Message)
	return result, nil
}

// Cleanup can discover edited recovery files that must be retained. Persist
// and disclose that information before clearing the transaction's intent.
func finalizeTransaction(ctx context.Context, driver Driver, plan Plan, state *State) ([]string, error) {
	provider, ok := driver.(TransactionDriver)
	if !ok {
		return nil, nil
	}
	report, err := provider.FinishTransaction(ctx, plan, state.Receipts)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for id, saved := range report {
		paths = append(paths, saved...)
		if receipt, recorded := state.Receipts[id]; recorded {
			receipt.After.Preserved = sortedUnique(append(receipt.After.Preserved, saved...))
			state.Receipts[id] = receipt
		}
	}
	return sortedUnique(paths), nil
}

// Corrupt historical evidence is preserved, not global mutation authority.
// Evidence for an in-flight or just-completed operation still gates completion.
func journalRequired(plan Plan, receipts map[string]Receipt, operation string) bool {
	for id, receipt := range receipts {
		if receipt.OperationID != operation {
			continue
		}
		if receipt.Status == "in-progress" {
			return true
		}
		for _, op := range plan.Operations {
			if op.Resource != id || receipt.Status != "ready" && receipt.Status != "removed" {
				continue
			}
			if op.Observed.CompletedOperation == operation {
				return true
			}
			expected, err := digest(struct{ Plan, Resource, Action string }{plan.ID, id, op.Action})
			if err == nil && expected == operation {
				return true
			}
		}
	}
	return false
}

// Enumerating existing journals alone cannot detect deleted completion proof.
// Unstarted in-progress intent may have no provider journal yet; only completed
// receipts required by this plan must already have one before finalization.
func requireCompletedJournals(plan Plan, receipts map[string]Receipt, provider, directory string) error {
	for _, receipt := range receipts {
		if receipt.OperationID == "" || receipt.Status != "ready" && receipt.Status != "removed" {
			continue
		}
		if receipt.After.Provider != provider && receipt.Before.Provider != provider || !journalRequired(plan, receipts, receipt.OperationID) {
			continue
		}
		path := filepath.Join(directory, receipt.OperationID+".json")
		if _, err := readDocument(path); err != nil {
			return fmt.Errorf("required completion journal unavailable at %s; preserve this transaction: %w", path, err)
		}
	}
	return nil
}

func preservedPaths(state State) []string {
	paths := []string{}
	for _, receipt := range state.Receipts {
		paths = append(paths, receipt.After.Preserved...)
	}
	return sortedUnique(paths)
}

// Deleting an overlay with a pre-existing baseline is data loss, not cleanup.
// Both the immediate check and final verification must prove restoration.
func baselineRestored(before, after Observation) bool {
	return !before.Unknown && !after.Unknown && after.Pending == "" && before.Present == after.Present && (!before.Present || sameArtifact(before, after))
}

// An incidental package can fulfill a later absent install in this same plan.
// Durable evidence from an unrelated historical operation cannot widen approval.
func introducedByEarlierOperation(plan Plan, state State, index int, op Operation, fresh Observation) bool {
	if op.Action != "install" || op.Observed.Unknown || op.Observed.Present || fresh.Unknown || !fresh.Present || !fresh.Healthy || fresh.Pending != "" || fresh.Provider != op.Observed.Provider || fresh.Identity != op.Observed.Identity || fresh.Scope != op.Observed.Scope || fresh.Privileged != op.Observed.Privileged || fresh.CompletedOperation == "" {
		return false
	}
	for _, earlier := range plan.Operations[:index] {
		if earlier.Action != "install" && earlier.Action != "update" && earlier.Action != "repair" {
			continue
		}
		id, err := digest(struct{ Plan, Resource, Action string }{plan.ID, earlier.Resource, earlier.Action})
		receipt := state.Receipts[earlier.Resource]
		if err == nil && id == fresh.CompletedOperation && receipt.OperationID == id && receipt.Status == "ready" && receipt.After.CompletedOperation == id {
			return true
		}
	}
	return false
}

func removalCompleted(receipt Receipt, after Observation) bool {
	if baselineRestored(receipt.Before, after) {
		return true
	}
	return !receipt.Before.Unknown && !receipt.Before.Present && !after.Unknown && after.Pending == "" && after.RemovalDeferred && after.Present && len(after.Preserved) > 0 && receipt.OperationID != "" && after.CompletedOperation == receipt.OperationID && sameArtifact(receipt.After, after)
}
