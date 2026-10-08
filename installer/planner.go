package installer

import (
	"fmt"
	"slices"
	"sort"
)

type Observation struct {
	// Unknown means inspection could not establish presence or identity. Pending
	// must explain the resource-local problem; it can never authorize mutation.
	Unknown     bool   `json:"unknown,omitempty"`
	Present     bool   `json:"present"`
	Healthy     bool   `json:"healthy"`
	HealthIssue string `json:"health_issue,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Version     string `json:"version,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	// Desired binds mutable configuration source bytes to approval without
	// confusing the desired payload with the currently owned target's identity.
	Desired string `json:"desired,omitempty"`
	// ApplyBlocked explains unavailable inputs without blocking restoration of
	// an already owned target from its independent recovery baseline.
	ApplyBlocked string `json:"apply_blocked,omitempty"`
	// Preserved reports user edits retained independently of the uninstall
	// baseline. These paths are informational and never authorize deletion.
	Preserved []string `json:"preserved,omitempty"`
	// External applications may consume verified native dependencies without
	// being exercised themselves. This disclosure grants no mutation authority.
	UnverifiedApplications []string `json:"unverified_applications,omitempty"`
	// Consumers lists external or uncertain consumers. Dependencies proved to
	// belong to this graph are resolved by the planner, not counted twice here.
	Consumers  []string `json:"consumers,omitempty"`
	Pending    string   `json:"pending,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Privileged bool     `json:"privileged,omitempty"`
	// Adoptable means this provider can preserve and restore the exact current
	// baseline. It is an offered operation, never implicit ownership or consent.
	Adoptable bool `json:"adoptable,omitempty"`
	// CompletedOperation is returned only after a driver verifies its durable
	// provider transaction against this exact observed artifact. Existence, a
	// matching path, or a before/after inventory difference is not evidence.
	CompletedOperation string `json:"completed_operation,omitempty"`
	// RemovalDeferred proves the resource root was released by CompletedOperation
	// into the provider-owned pool. Physical presence remains truthful; Preserved
	// must disclose the retained package. It never authorizes external ownership.
	RemovalDeferred bool `json:"removal_deferred,omitempty"`
	// RestoredOperation proves only cancellation/restoration of saved intent;
	// it never grants ownership or completion of the original installation.
	RestoredOperation string `json:"restored_operation,omitempty"`
	// Restoring means the saved operation has committed to restoration. Until
	// it is archived, the UI must not offer forward resume or abandonment.
	Restoring bool `json:"restoring,omitempty"`
	// Inventory binds provider-owned members and protected inputs to approval,
	// including unselected members. It is separate from this resource's artifact
	// identity and may change through a provider-proved operation.
	Inventory      string          `json:"inventory,omitempty"`
	ResourceResume *ResourceResume `json:"resource_resume,omitempty"`
}

type Receipt struct {
	Ownership   string      `json:"ownership"`
	Before      Observation `json:"before"`
	After       Observation `json:"after"`
	Status      string      `json:"status"`
	Reason      string      `json:"reason,omitempty"`
	Recovery    string      `json:"recovery,omitempty"`
	OperationID string      `json:"operation_id,omitempty"`
	Adopted     bool        `json:"adopted,omitempty"`
}

type Request struct {
	Schema       int      `json:"schema"`
	Mode         string   `json:"mode"`
	Selected     []string `json:"selected"`
	Keep         []string `json:"keep"`
	RemoveShared []string `json:"remove_shared,omitempty"`
	Adopt        []string `json:"adopt,omitempty"`
	ExpectedPlan string   `json:"expected_plan,omitempty"`
	Retry        bool     `json:"retry,omitempty"`
}

type Operation struct {
	Resource   string      `json:"resource"`
	Action     string      `json:"action"`
	Reason     string      `json:"reason"`
	RequiredBy []string    `json:"required_by,omitempty"`
	Observed   Observation `json:"observed"`
	Ownership  string      `json:"ownership"`
	Privileged bool        `json:"privileged,omitempty"`
}

type Plan struct {
	Schema            int             `json:"schema"`
	ID                string          `json:"id"`
	Mode              string          `json:"mode,omitempty"`
	Retry             bool            `json:"retry,omitempty"`
	RemoveShared      []string        `json:"remove_shared,omitempty"`
	Adopt             []string        `json:"adopt,omitempty"`
	Source            string          `json:"source"`
	Target            string          `json:"target"`
	Catalog           string          `json:"catalog"`
	State             string          `json:"state"`
	Inventories       []string        `json:"inventories,omitempty"`
	Context           Context         `json:"context"`
	Generation        uint64          `json:"generation"`
	Selected          []string        `json:"selected"`
	Keep              []string        `json:"keep"`
	Operations        []Operation     `json:"operations"`
	ResourceResume    *ResourceResume `json:"resource_resume,omitempty"`
	InterruptedSource string          `json:"interrupted_source,omitempty"`
}

// PlanChanges is pure: probes, prompts, IO and provider mutation live outside it.
func PlanChanges(c *Catalog, ctx Context, state State, request Request, observed map[string]Observation, source string) (Plan, error) {
	for id, o := range observed {
		if o.Unknown && (o.Pending == "" || o.Present || o.CompletedOperation != "" || o.RestoredOperation != "") {
			return Plan{}, fmt.Errorf("%s has an invalid unknown observation", id)
		}
	}
	request, err := effectiveRequest(state, request)
	if err != nil {
		return Plan{}, err
	}
	if request.Retry && (state.Transaction.Plan.Source == "" || state.Transaction.Plan.Source != source) {
		return Plan{}, fmt.Errorf("resume requires the authenticated installer from original source revision %q; preserve this transaction and use that revision", state.Transaction.Plan.Source)
	}
	manifest, err := digest(c)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Schema: 1, Mode: request.Mode, Retry: request.Retry, RemoveShared: sortedUnique(request.RemoveShared), Adopt: sortedUnique(request.Adopt), Source: source, Target: state.Target, Catalog: manifest, Context: ctx, Generation: state.Generation}
	if request.Mode == "check" && state.Transaction != nil {
		plan.InterruptedSource = state.Transaction.Plan.Source
		if plan.InterruptedSource == "" {
			plan.InterruptedSource = "unknown legacy revision"
		}
	}
	plan.State, err = digest(state)
	if err != nil {
		return plan, err
	}
	if request.Schema != 1 {
		return plan, fmt.Errorf("unsupported request schema %d", request.Schema)
	}
	if request.Mode != "plan" && request.Mode != "apply" && request.Mode != "repair" && request.Mode != "update" && request.Mode != "check" && request.Mode != "abandon" && request.Mode != "restore" {
		return plan, fmt.Errorf("unknown mode %q", request.Mode)
	}
	if err := ctx.Validate(); err != nil {
		return plan, err
	}
	if state.Schema != 0 && state.Context != ctx {
		return plan, fmt.Errorf("state belongs to a different target context; review migration first")
	}
	if request.Mode == "abandon" {
		plan.Selected, plan.Keep = sortedUnique(state.Selected), sortedUnique(state.Keep)
		plan.ID, err = digest(plan)
		return plan, err
	}
	if request.Mode == "restore" {
		restored, found, err := planResourceResume(plan, state, observed)
		if !found && err == nil {
			err = ErrNoResourceRestore
		}
		return restored, err
	}
	if request.Retry {
		if resumed, found, err := planResourceResume(plan, state, observed); found || err != nil {
			return resumed, err
		}
	}
	for _, id := range request.Selected {
		r, ok := c.Resource(id)
		if !ok || !r.Capability {
			return plan, fmt.Errorf("unknown capability %q", id)
		}
	}
	plan.Selected = sortedUnique(request.Selected)
	plan.Keep = sortedUnique(request.Keep)
	for _, id := range plan.Keep {
		if _, ok := c.Resource(id); !ok {
			return plan, fmt.Errorf("unknown retained resource %q", id)
		}
		if _, ok := state.Receipts[id]; !ok {
			return plan, fmt.Errorf("cannot keep unrecorded resource %q", id)
		}
	}
	for _, id := range request.RemoveShared {
		if _, ok := state.Receipts[id]; !ok {
			return plan, fmt.Errorf("cannot remove unrecorded resource %q", id)
		}
		if slices.Contains(plan.Keep, id) {
			return plan, fmt.Errorf("%s cannot be both kept and removed", id)
		}
	}
	wanted, err := c.Closure(append(append([]string{}, plan.Selected...), plan.Keep...), ctx)
	if err != nil {
		return plan, err
	}
	for _, id := range plan.Adopt {
		r, known := c.Resource(id)
		o := observed[id]
		receipt := state.Receipts[id]
		completed := request.Retry && recoverCompletion(receipt, o).Ownership == "created"
		allowedMode := request.Mode == "apply" || (request.Mode == "update" || request.Mode == "repair") && receipt.Ownership == "created"
		if !allowedMode || !known || !slices.Contains(wanted, id) || r.Retain || !o.Present || !o.Adoptable && !completed || o.Pending != "" ||
			!request.Retry && receipt.Ownership == "created" && sameArtifact(receipt.After, o) {
			return plan, fmt.Errorf("%s cannot be adopted in this selection or provider state", id)
		}
	}
	requiredBy := map[string][]string{}
	for _, root := range plan.Selected {
		closure, err := c.Closure([]string{root}, ctx)
		if err != nil {
			return plan, err
		}
		for _, id := range closure {
			requiredBy[id] = append(requiredBy[id], root)
		}
	}
	for _, id := range wanted {
		r, _ := c.Resource(id)
		if r.Action == "" {
			continue
		}
		o, ok := observed[id]
		if !ok {
			return plan, fmt.Errorf("missing observation for %s", id)
		}
		receipt, recorded := state.Receipts[id]
		if request.Retry {
			receipt = recoverCompletion(receipt, o)
		}
		ownership := "unrecorded"
		if recorded {
			ownership = receipt.Ownership
		}
		op := Operation{Resource: id, Action: "keep", Reason: "already satisfies selection", RequiredBy: requiredBy[id], Observed: o, Ownership: ownership, Privileged: o.Privileged || r.Scope == "machine" || o.Scope == "machine"}
		owned := recorded && receipt.Ownership == "created" && sameArtifact(receipt.After, o)
		switch {
		case o.Pending != "":
			op.Action, op.Reason = "pending", o.Pending
		case o.ApplyBlocked != "":
			op.Action, op.Reason = "pending", o.ApplyBlocked
		case !o.Present:
			op.Action, op.Reason = "install", "required by selection"
			if o.HealthIssue != "" {
				op.Reason += ": " + o.HealthIssue
			}
		case o.Healthy && receipt.After.RemovalDeferred && removalCompleted(receipt, o):
			op.Action, op.Reason = "install", "reclaim the previously released owned native root"
		case slices.Contains(plan.Adopt, id) && !owned:
			op.Action, op.Reason = "adopt", "explicitly preserve the current baseline and manage this configuration"
			if receipt.Ownership == "created" {
				op.Reason = "preserve your edits separately and apply the selected configuration; keep the original uninstall baseline"
			}
		case !o.Healthy && !owned:
			op.Action, op.Reason = "pending", "pre-existing, uncertain or changed resource requires explicit adoption"
			if o.HealthIssue != "" {
				op.Reason += ": " + o.HealthIssue
			}
		case !o.Healthy:
			op.Action, op.Reason = "repair", "selected resource failed its check"
			if o.HealthIssue != "" {
				op.Reason += ": " + o.HealthIssue
			}
		case request.Mode == "update":
			if recorded && receipt.Ownership == "created" && !owned {
				op.Action, op.Reason = "pending", "changed resource requires adoption before update"
			} else if request.Retry && completedRetryOperation(state, id, receipt, o) {
				op.Reason = "the original approved update already completed"
			} else if owned && o.Desired != "" && o.Desired == receipt.After.Desired {
				op.Reason = "verified resource already matches the release's pinned inputs"
			} else if owned {
				op.Action, op.Reason = "update", "explicit update of an owned resource"
			}
		}
		if r.ReplanAfter && (op.Action == "install" || op.Action == "repair" || op.Action == "update") {
			op.Reason += "; bootstrap then review newly available provider inventory"
		}
		plan.Operations = append(plan.Operations, op)
	}
	// A reboot or host-side action blocks the whole consumer chain, including
	// resources already present whose health would otherwise look sufficient.
	blocked := map[string]string{}
	for i := range plan.Operations {
		op := &plan.Operations[i]
		r, _ := c.Resource(op.Resource)
		closure, err := c.Closure(r.Dependencies(ctx), ctx)
		if err != nil {
			return plan, err
		}
		for _, dependency := range closure {
			if reason, waiting := blocked[dependency]; waiting {
				op.Action, op.Reason = "pending", "waiting for "+dependency+": "+reason
				break
			}
		}
		if op.Action == "pending" {
			blocked[op.Resource] = op.Reason
		}
	}
	old := make([]string, 0, len(state.Receipts))
	for id := range state.Receipts {
		old = append(old, id)
	}
	for id, observation := range observed {
		r, known := c.Resource(id)
		if !known || !r.Available(ctx) {
			return plan, fmt.Errorf("invalid or unmapped observation %q", id)
		}
		if observation.Inventory != "" {
			if !operationID.MatchString(observation.Inventory) {
				return plan, fmt.Errorf("invalid inventory identity for %s", id)
			}
			plan.Inventories = append(plan.Inventories, observation.Inventory)
		}
		if observation.Present && r.Action != "" {
			old = append(old, id)
		}
	}
	plan.Inventories = sortedUnique(plan.Inventories)
	oldOrder, err := c.Closure(old, ctx)
	if err != nil {
		return plan, fmt.Errorf("installed state needs migration: %w", err)
	}
	// Integration cleanup must run before removing its runtime prerequisites.
	slices.Reverse(oldOrder)
	for _, id := range oldOrder {
		if slices.Contains(wanted, id) {
			continue
		}
		receipt, recorded := state.Receipts[id]
		r, _ := c.Resource(id)
		if r.Action == "" {
			continue
		}
		o, ok := observed[id]
		if !recorded && (!ok || !o.Present) {
			continue
		}
		if !ok {
			return plan, fmt.Errorf("missing removal observation for %s", id)
		}
		if !recorded {
			receipt = Receipt{Ownership: "reused", Before: o, After: o, Status: "retained"}
		}
		op := Operation{Resource: id, Action: "remove", Reason: "no remaining selected consumer", Observed: o, Ownership: receipt.Ownership, Privileged: o.Privileged || r.Scope == "machine" || o.Scope == "machine"}
		switch {
		case receipt.After.RemovalDeferred && removalCompleted(receipt, o):
			op.Action, op.Reason = "forget", "native root already released; retained package remains provider-owned"
		case request.Mode == "update" || request.Mode == "repair" || request.Mode == "check":
			op.Action, op.Reason = "retain", "selection changes and removal require a separate apply request"
		case o.Pending != "":
			op.Action, op.Reason = "retain", o.Pending
		case !o.Present && receipt.Ownership == "created" && receipt.Adopted && receipt.Before.Present && receipt.After.Provider == o.Provider && receipt.After.Identity == o.Identity:
			op.Reason = "restore the preserved original configuration even though the installed target was deleted"
		case !o.Present:
			op.Action, op.Reason = "forget", "already absent"
		case r.Retain:
			op.Action, op.Reason = "retain", "retained infrastructure or personal data"
		case receipt.Ownership != "created":
			op.Action, op.Reason = "retain", "pre-existing or uncertain ownership"
		case receipt.After.Fingerprint != o.Fingerprint || receipt.After.Provider != o.Provider || receipt.After.Identity != o.Identity:
			op.Action, op.Reason = "retain", "changed since the installer last verified it"
		case len(o.Consumers) > 0:
			op.Action, op.Reason = "retain", "another consumer still depends on this resource"
		case (r.Shared || r.Scope == "machine" || o.Scope == "machine") && !slices.Contains(request.RemoveShared, id):
			op.Action, op.Reason = "retain", "external use cannot be excluded; explicit per-item removal required"
		}
		plan.Operations = append(plan.Operations, op)
	}
	// A retained application/integration still needs its prerequisites. A
	// removed checkbox is not permission to break an artifact we preserved.
	retained := []string{}
	for _, op := range plan.Operations {
		if op.Action == "retain" {
			retained = append(retained, op.Resource)
		}
	}
	retainedClosure, closureErr := c.Closure(retained, ctx)
	if closureErr != nil {
		return plan, closureErr
	}
	for i := range plan.Operations {
		op := &plan.Operations[i]
		if op.Action == "remove" && slices.Contains(retainedClosure, op.Resource) {
			op.Action, op.Reason = "retain", "required by another retained resource"
		}
	}
	plan.ID = ""
	plan.ID, err = digest(plan)
	return plan, err
}

func effectiveRequest(state State, request Request) (Request, error) {
	if request.Mode == "apply" && !request.Retry && request.Selected == nil {
		return request, fmt.Errorf("apply requires an explicit selected list; use an empty list only for deliberate deselection")
	}
	if request.Mode == "abandon" || request.Mode == "restore" {
		if state.Transaction == nil {
			return request, fmt.Errorf("no unfinished transaction to %s", request.Mode)
		}
		if request.Retry || request.Selected != nil || request.Keep != nil || len(request.RemoveShared) != 0 || len(request.Adopt) != 0 {
			return request, fmt.Errorf("%s does not accept new selection or ownership choices", request.Mode)
		}
		return request, nil
	}
	if request.Retry {
		if state.Transaction == nil {
			return request, fmt.Errorf("no unfinished transaction to retry")
		}
		previous := state.Transaction.Plan
		if previous.Mode == "" {
			return request, fmt.Errorf("legacy transaction has no operation mode; preserve it and explicitly abandon before a new request")
		}
		if request.Mode != previous.Mode {
			return request, fmt.Errorf("retry must preserve the interrupted operation mode")
		}
		if request.Selected == nil {
			request.Selected = previous.Selected
		}
		if request.Keep == nil {
			request.Keep = previous.Keep
		}
		if request.RemoveShared == nil {
			request.RemoveShared = previous.RemoveShared
		}
		if request.Adopt == nil {
			request.Adopt = previous.Adopt
		}
		if !slices.Equal(sortedUnique(request.Selected), sortedUnique(previous.Selected)) ||
			!slices.Equal(sortedUnique(request.Keep), sortedUnique(previous.Keep)) ||
			!slices.Equal(sortedUnique(request.RemoveShared), sortedUnique(previous.RemoveShared)) ||
			!slices.Equal(sortedUnique(request.Adopt), sortedUnique(previous.Adopt)) {
			return request, fmt.Errorf("retry must preserve the interrupted selection and removal choices; abandon before changing them")
		}
	} else if state.Transaction != nil && request.Mode != "plan" && request.Mode != "check" {
		return request, fmt.Errorf("unfinished transaction: explicitly choose retry or abandon before applying")
	}
	if request.Mode == "update" || request.Mode == "repair" || request.Mode == "check" {
		if request.Selected != nil && !slices.Equal(sortedUnique(request.Selected), sortedUnique(state.Selected)) || request.Keep != nil && !slices.Equal(sortedUnique(request.Keep), sortedUnique(state.Keep)) || len(request.RemoveShared) != 0 || request.Mode == "check" && len(request.Adopt) != 0 {
			return request, fmt.Errorf("%s preserves the recorded selection and Keep choices; use apply to change them", request.Mode)
		}
		request.Selected, request.Keep = state.Selected, state.Keep
	} else if request.Keep == nil {
		request.Keep = state.Keep
	}
	return request, nil
}

// Fingerprints describe this resource's own content, not dependency health.
// Adapters report health separately so an approved prerequisite repair may
// make its consumers healthy without invalidating their ownership evidence.
func sameArtifact(a, b Observation) bool {
	return a.Unknown == b.Unknown && a.Present == b.Present && a.Provider == b.Provider && a.Identity == b.Identity && a.Fingerprint == b.Fingerprint && a.Version == b.Version && a.Scope == b.Scope && a.Privileged == b.Privileged
}

// Only an explicit retry can recover interrupted mutation ownership. Older
// receipts lack OperationID and therefore cannot acquire authority from this
// path. Abandonment preserves their uncertain outcome for explicit adoption.
func recoverCompletion(receipt Receipt, observed Observation) Receipt {
	if observed.RemovalDeferred && (receipt.Status == "in-progress" || receipt.Status == "needs-action") && removalCompleted(receipt, observed) {
		return removedReceipt(receipt, observed)
	}
	if receipt.OperationID == "" || observed.CompletedOperation != receipt.OperationID || !observed.Present ||
		(receipt.Status != "in-progress" && receipt.Status != "needs-action") ||
		(receipt.Ownership != "created" && (receipt.Ownership != "uncertain" || receipt.Before.Present && !receipt.Adopted)) {
		return receipt
	}
	receipt.Ownership, receipt.After = "created", observed
	receipt.Status, receipt.Reason = "ready", ""
	if observed.Pending != "" {
		receipt.Status, receipt.Reason = "needs-action", observed.Pending
	}
	return receipt
}

// Keep the original removal identity until the entire transaction is sealed.
// A later finalization failure must still be able to retry RemoveShared.
func removedReceipt(receipt Receipt, after Observation) Receipt {
	receipt.Ownership, receipt.Status, receipt.Reason = "reused", "removed", "removal restored its baseline"
	receipt.After = after
	if after.RemovalDeferred {
		receipt.Reason = "native root released; package retained in the owned provider pool"
	}
	return receipt
}

func sortedUnique(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return slices.Compact(result)
}
