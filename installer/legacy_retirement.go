package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Runtime retirement is the explicit second migration phase. It removes earlier
// dotfiles infrastructure only from approved native evidence. Receipts record a
// completed historical operation, never ownership of packages or personal data.
// Every step is journaled before it runs, its saved native result is consulted
// before any precondition on recovery, destructive steps recheck consumers
// immediately beforehand, and an unknown native outcome is never repeated.

const (
	LegacyRetirementPreparation = "preparation"
	LegacyRetirementRuntime     = "runtime"
	legacyReplacements          = "retire.replacements"
	legacyStepLimit             = 1024
	legacyAttemptLimit          = 8
)

type LegacyRetirement struct {
	Controller Controller
	driver     *legacyRetirementDriver
}

type legacyRuntimeEnvironment struct {
	Executable, Username, UID string
	// Cache is the observed XDG cache root (or its POSIX default).
	Cache    string
	Root     bool
	Programs map[string]string
}

type legacyRetirementDriver struct {
	Directory       string
	Repository      string
	Normal          Controller
	Platform        NativePlatform
	Folders         ConfigFolders
	Environment     legacyRuntimeEnvironment
	Authenticate    func(context.Context) error
	Elevation       func(context.Context) (func() error, error)
	Query           func(context.Context, nativeCommand) ([]byte, error)
	Run             func(context.Context, nativeCommand) ([]byte, error)
	WorkerDirectory string
	session         *nativeSession
	// systemRoot relocates fixed native system paths for hermetic fixtures.
	// Empty is the host root; production never sets it.
	systemRoot string
}

type legacyRetirementFile struct {
	Path     string         `json:"path"`
	Snapshot configSnapshot `json:"snapshot"`
}

// Blocked is a hard refusal that authorizes no step. Retained artifacts are
// disclosed and keep migration incomplete without blocking verified steps.
// Notices are volatile display text (such as running processes); they are
// excluded from the approved identity and from saved intent.
type legacyRetirementScope struct {
	ReplacementState string                 `json:"replacement_state,omitempty"`
	Blocked          string                 `json:"blocked,omitempty"`
	Native           string                 `json:"native,omitempty"`
	Detail           []string               `json:"detail,omitempty"`
	Retained         []string               `json:"retained,omitempty"`
	Steps            []legacyRetirementStep `json:"steps,omitempty"`
	Notices          []string               `json:"-"`
}

type legacyRetirementIntent struct {
	Schema    int                   `json:"schema"`
	Resource  string                `json:"resource"`
	Operation string                `json:"operation"`
	Before    Observation           `json:"before"`
	Approval  *Plan                 `json:"approval,omitempty"`
	Scope     legacyRetirementScope `json:"scope"`
	Attempts  []int                 `json:"attempts"`
	Next      int                   `json:"next"`
	Complete  bool                  `json:"complete,omitempty"`
	Preserved []string              `json:"preserved,omitempty"`
}

func legacyRetirementCatalog() *Catalog {
	posix := []string{"darwin", "linux"}
	ready := []string{legacyReplacements}
	resources := []Resource{
		{ID: legacyReplacements, Name: "Verified replacement setup", Description: "Normal setup has no unfinished operation and every selected replacement passes its read-only check.", Action: "retire", Scope: "user", Retain: true},
		{ID: "retire.homebrew", Name: "Convert Nix-managed Homebrew", Description: "Before setup, restore Homebrew's own repository with the pinned official installer. Every installed formula, cask and tap is kept and verified unchanged.", Capability: true, Platforms: []string{"darwin"}, Action: "retire", Scope: "machine", Retain: true},
		{ID: "retire.chezmoi", Name: "Uninstall chezmoi", Description: "Remove the chezmoi executable through the installer that provided it. Its configuration, state and cache are moved aside, never deleted or purged.", Capability: true, Requires: ready, Action: "retire", Scope: "user", Retain: true},
		{ID: "retire.gh-dash", Name: "Remove the old gh-dash extension", Description: "Move the verified released dlvhdr/gh-dash extension aside (GitHub CLI no longer lists it); other extensions stay.", Capability: true, Requires: ready, Action: "retire", Scope: "user", Retain: true},
		{ID: "retire.nix", Name: "Uninstall Nix, Home Manager and nix-darwin", Description: "Permanently remove the Nix store, daemon, build accounts and their system integrations after consumer, login-shell and replacement checks. Configuration is moved aside, not deleted.", Capability: true, Platforms: posix, Requires: ready, PlatformRequires: map[string][]string{"darwin": {"retire.homebrew"}}, Action: "retire", Scope: "machine", Retain: true},
		{ID: "retire.pi", Name: "Remove the retired Pi CLI", Description: "Move the exact released Pi packages aside from the active npm package names and remove their command links or npm shims. ~/.pi data stays.", Capability: true, Requires: ready, Action: "retire", Scope: "user", Retain: true},
		{ID: "retire.sentinel-policy", Name: "Remove released agent policy for OpenCode and Pi", Description: "Delete OpenCode and Pi policy files that hold exactly the Sentinel 0.1.2 policy the released setup wrote. Files with any other text stay; dotfiles supports neither agent.", Capability: true, Requires: ready, Action: "retire", Scope: "user", Retain: true},
		{ID: "retire.tools", Name: "Remove replaced legacy tools", Description: "Remove direct binaries proved by released provenance markers and exact clean pinned plugin checkouts. Session snapshots and personal files stay.", Capability: true, Platforms: posix, Requires: ready, Action: "retire", Scope: "user", Retain: true},
	}
	return &Catalog{Schema: 1, Resources: append(resources, legacyManagerDeltaResources()...)}
}

func newLegacyRetirement(normal Controller, platform NativePlatform, folders ConfigFolders, repository string, environment legacyRuntimeEnvironment) (*LegacyRetirement, error) {
	if err := normal.validate(); err != nil {
		return nil, err
	}
	if platform.Context != normal.Context {
		return nil, errors.New("legacy retirement and setup must share one target context")
	}
	for _, path := range []string{normal.StatePath, folders.Home, repository, environment.Executable} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return nil, errors.New("legacy retirement requires explicit canonical native paths")
		}
	}
	if environment.Username == "" || strings.ContainsAny(environment.Username, "/:\x00\r\n") {
		return nil, errors.New("legacy retirement requires the observed account name")
	}
	stateDirectory := filepath.Dir(normal.StatePath)
	directory := filepath.Join(stateDirectory, "migration", "retirement")
	session := newNativeSession(filepath.Join(stateDirectory, "native", "worker"))
	driver := &legacyRetirementDriver{Directory: directory, Repository: repository, Normal: normal, Platform: platform, Folders: folders,
		Environment: environment, Query: queryLegacyRuntime, Run: session.run, WorkerDirectory: session.Directory, session: session}
	controller := Controller{Catalog: legacyRetirementCatalog(), Context: platform.Context, Source: normal.Source, Home: folders.Home,
		StatePath: filepath.Join(directory, "state.json"), Driver: driver}
	if err := controller.validate(); err != nil {
		return nil, err
	}
	return &LegacyRetirement{Controller: controller, driver: driver}, nil
}

// SetAuthentication supplies foreground credential refresh after approval.
// Native workers use sudo -n and never read a terminal password.
func (m *LegacyRetirement) SetAuthentication(authenticate func(context.Context) error) {
	m.driver.Authenticate = authenticate
}

// SetElevation authenticates once for an approved privileged retirement and
// keeps that credential fresh while it runs; see ElevationDriver.
func (m *LegacyRetirement) SetElevation(elevation func(context.Context) (func() error, error)) {
	m.driver.Elevation = elevation
}

// Mutation holds normal setup out of the whole replacement-check and
// retirement interval; the shared native worker lock serializes providers.
func (d *legacyRetirementDriver) AcquireMutation(ctx context.Context) (func() error, error) {
	normal, err := Lock(filepath.Dir(d.Normal.StatePath))
	if err != nil {
		return nil, err
	}
	native, err := d.session.acquire(ctx)
	if err != nil {
		return nil, errors.Join(err, normal())
	}
	return func() error { return errors.Join(native(), normal()) }, nil
}

func (d *legacyRetirementDriver) AcquirePreview(ctx context.Context) (func() error, error) {
	retirement, err := lockExisting(d.Directory)
	if err != nil {
		return nil, err
	}
	guard, ok := d.Normal.Driver.(PreviewGuard)
	if !ok {
		normal, err := lockExisting(filepath.Dir(d.Normal.StatePath))
		if err != nil {
			return nil, errors.Join(err, retirement())
		}
		return func() error { return errors.Join(normal(), retirement()) }, nil
	}
	normal, err := guard.AcquirePreview(ctx)
	if err != nil {
		return nil, errors.Join(err, retirement())
	}
	return func() error { return errors.Join(normal(), retirement()) }, nil
}

func (d *legacyRetirementDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *legacyRetirementDriver) readIntent(id string, receipt Receipt) (legacyRetirementIntent, error) {
	var intent legacyRetirementIntent
	if receipt.OperationID == "" {
		return intent, os.ErrNotExist
	}
	if !operationID.MatchString(receipt.OperationID) {
		return intent, errors.New("invalid legacy retirement operation")
	}
	data, err := readDocument(d.intentPath(receipt.OperationID))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	approved, err := digest(intent.Scope)
	if err != nil {
		return intent, err
	}
	approvalMatches := approved == intent.Before.Desired
	if intent.Approval != nil {
		plan := *intent.Approval
		planID := plan.ID
		plan.ID = ""
		actual, err := digest(plan)
		if err != nil || actual != planID {
			return intent, errors.Join(errors.New("invalid saved retirement approval"), err)
		}
		operation, err := digest(struct{ Plan, Resource, Action string }{planID, id, "install"})
		if err != nil || operation != intent.Operation {
			return intent, errors.Join(errors.New("retirement approval differs from its operation"), err)
		}
		approvalMatches = false
		for _, op := range plan.Operations {
			if op.Resource == id && op.Action == "install" && !op.Observed.Present && !op.Observed.Unknown && op.Observed.Desired == approved {
				approvalMatches = true
			}
		}
	}
	// The saved scope is exactly the approved, re-observed Desired identity.
	if intent.Schema != 1 || intent.Resource != id || intent.Operation != receipt.OperationID || !sameArtifact(intent.Before, receipt.Before) ||
		intent.Before.Desired != receipt.Before.Desired || !approvalMatches || intent.Scope.Blocked != "" ||
		id != "retire.homebrew" && !operationID.MatchString(intent.Scope.ReplacementState) ||
		len(intent.Scope.Steps) == 0 || len(intent.Scope.Steps) > legacyStepLimit || len(intent.Attempts) != len(intent.Scope.Steps) ||
		intent.Next < 0 || intent.Next > len(intent.Scope.Steps) || intent.Complete && intent.Next != len(intent.Scope.Steps) || len(intent.Preserved) > 4096 {
		return intent, errors.New("legacy retirement intent differs from its approved receipt")
	}
	for i, step := range intent.Scope.Steps {
		if err := d.validateStep(step); err != nil {
			return intent, err
		}
		if intent.Attempts[i] < 0 || intent.Attempts[i] >= legacyAttemptLimit || i > intent.Next && intent.Attempts[i] != 0 {
			return intent, errors.New("invalid legacy retirement attempt record")
		}
	}
	for _, path := range intent.Preserved {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return intent, errors.New("invalid preserved legacy path")
		}
	}
	return intent, nil
}

func (d *legacyRetirementDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "legacy-retirement", Identity: r.ID, Scope: r.Scope}
	if r.ID == legacyReplacements {
		return d.observeReplacements(ctx, o)
	}
	intent, err := d.readIntent(r.ID, receipt)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Damaged history is preserved and disclosed; it authorizes nothing.
		o.Unknown = true
		o.Pending = "saved legacy retirement evidence is unreadable; preserve " + d.intentPath(receipt.OperationID) + ": " + err.Error()
		o.Preserved = slices.Clone(receipt.After.Preserved)
		return o, nil
	}
	if err == nil {
		o.Preserved = slices.Clone(intent.Preserved)
		if !intent.Complete && receipt.Status == "in-progress" {
			return d.observeInterrupted(ctx, o, intent)
		}
		// An abandoned intent is historical evidence; fresh native inspection
		// decides whether a new, separately approved operation is needed.
	}
	scope, err := d.inspect(ctx, r.ID)
	if err != nil {
		o.Unknown, o.Pending = true, "legacy inspection failed; nothing was changed: "+err.Error()
		return o, nil
	}
	o.Desired, err = digest(scope)
	if err != nil {
		return Observation{}, err
	}
	o.ApplyDetail = legacyScopeSummary(scope)
	switch {
	case scope.Blocked != "":
		o.ApplyBlocked = scope.Blocked
	case len(scope.Steps) == 0 && len(scope.Retained) != 0:
		kept, err := d.keptScope(r.ID, scope)
		if err != nil {
			o.Unknown, o.Pending = true, err.Error()
			return o, nil
		}
		if kept {
			o.Present, o.Healthy, o.Fingerprint = true, true, o.Desired
			o.ApplyDetail += "; kept by your choice"
			break
		}
		o.ApplyBlocked = "earlier installation remains but cannot be retired from verified evidence: " + strings.Join(scope.Retained, "; ")
	case len(scope.Steps) == 0:
		o.Present, o.Healthy, o.Fingerprint = true, true, o.Desired
	default:
		o.Privileged = slices.ContainsFunc(scope.Steps, legacyStepNeedsAdministrator)
	}
	if intent.Complete && len(scope.Steps) == 0 {
		// A completed operation remains history, while fresh blockers are still
		// visible. Newly detected actionable artifacts need a separate approval.
		o.Present, o.Healthy, o.CompletedOperation = true, true, intent.Operation
		o.Fingerprint = intent.Operation
	}
	return o, nil
}

func (d *legacyRetirementDriver) observeInterrupted(ctx context.Context, o Observation, intent legacyRetirementIntent) (Observation, error) {
	var err error
	o.Privileged = slices.ContainsFunc(intent.Scope.Steps[intent.Next:], legacyStepNeedsAdministrator)
	o.Desired, err = digest(intent.Scope)
	if err != nil {
		return Observation{}, err
	}
	label := "final verification"
	if intent.Next < len(intent.Scope.Steps) {
		label = intent.Scope.Steps[intent.Next].Label
	}
	token, outcome, err := d.resumeToken(ctx, intent)
	if err != nil {
		o.Pending = "legacy retirement stopped at " + label + " and its current state could not be verified: " + err.Error() + "; preserve " + d.intentPath(intent.Operation)
		return o, nil
	}
	switch outcome.Status {
	case legacyOutcomeUnknown:
		o.Pending = "the native outcome of " + label + " is unknown. Inspect it natively, then abandon this operation and review a fresh retirement preview; it is never repeated automatically. Saved intent: " + d.intentPath(intent.Operation)
		return o, nil
	case legacyOutcomeFailed:
		o.Pending = "legacy retirement step " + label + " failed: " + outcome.Error + "; resolve the cause, then resume to make one new attempt"
	default:
		o.Pending = "legacy retirement stopped at " + label + "; resume the exact saved operation after reviewing current native evidence"
	}
	o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	return o, nil
}

func (d *legacyRetirementDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if r.ID == legacyReplacements || op.Action != "install" || !operationID.MatchString(receipt.OperationID) || receipt.Before.Present || op.Observed.Present {
		return Observation{}, errors.New("legacy retirement requires an explicitly approved new retirement operation")
	}
	if receipt.Ownership == "created" {
		previous := receipt
		previous.OperationID = receipt.After.CompletedOperation
		prior, err := d.readIntent(r.ID, previous)
		if err != nil || !prior.Complete {
			return Observation{}, errors.Join(errors.New("earlier retirement has no intact completion evidence"), err)
		}
	} else if receipt.Ownership != "uncertain" {
		return Observation{}, errors.New("legacy retirement requires an approved absent baseline")
	}
	if _, err := d.readIntent(r.ID, receipt); !errors.Is(err, os.ErrNotExist) {
		return Observation{}, errors.Join(errors.New("saved retirement intent already exists; use explicit recovery"), err)
	}
	scope, err := d.inspect(ctx, r.ID)
	if err != nil {
		return Observation{}, err
	}
	identity, err := digest(scope)
	if err != nil {
		return Observation{}, err
	}
	if identity != op.Observed.Desired || scope.Blocked != "" || len(scope.Steps) == 0 {
		return Observation{}, errors.New("legacy installation changed or is blocked; review a fresh retirement preview")
	}
	intent := legacyRetirementIntent{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Before: receipt.Before, Scope: scope, Attempts: make([]int, len(scope.Steps)),
		Preserved: sortedUnique(append(slices.Clone(receipt.After.Preserved), op.Observed.Preserved...))}
	state, err := LoadState(filepath.Join(d.Directory, "state.json"), d.Folders.Home)
	if err != nil || state.Transaction == nil {
		return Observation{}, errors.Join(errors.New("retirement has no saved approval transaction"), err)
	}
	intent.Approval = &state.Transaction.Plan
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	if _, err := d.readIntent(r.ID, receipt); err != nil {
		return Observation{}, err
	}
	return d.complete(ctx, r, receipt, &intent, false)
}

func (d *legacyRetirementDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	intent, err := d.readIntent(r.ID, receipt)
	if err != nil {
		return Observation{}, err
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	token, outcome, err := d.resumeToken(ctx, intent)
	if err != nil || outcome.Status == legacyOutcomeUnknown || op.Observed.ResourceResume == nil || op.Observed.ResourceResume.Token != token {
		return Observation{}, errors.Join(errors.New("legacy retirement recovery changed after approval or has an unknown native outcome"), err)
	}
	return d.complete(ctx, r, receipt, &intent, true)
}

func (d *legacyRetirementDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("completed retirement is historical evidence, not an uninstallable selection")
}

// Recovery approval may make one new attempt for the first remaining step only
// after its saved native result proves a known failure and preconditions hold.
func (d *legacyRetirementDriver) complete(ctx context.Context, r Resource, receipt Receipt, intent *legacyRetirementIntent, recovery bool) (Observation, error) {
	for intent.Next < len(intent.Scope.Steps) {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		step := intent.Scope.Steps[intent.Next]
		if err := d.executeStep(ctx, intent, intent.Next, recovery); err != nil {
			return Observation{}, fmt.Errorf("%s: %w", step.Label, err)
		}
		recovery = false
		intent.Next++
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	}
	scope, err := d.inspect(ctx, r.ID)
	if err != nil {
		return Observation{}, err
	}
	if scope.Blocked != "" || len(scope.Steps) != 0 {
		return Observation{}, fmt.Errorf("legacy retirement remains unfinished after its approved steps: %s", legacyScopeSummary(scope))
	}
	intent.Complete = true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// Approval binds the full normal check, including provider inventories. Final
// artifact preservation excludes the global inventory: retiring an old native
// package changes it without changing any selected replacement. All selected
// observations, health and the complete setup state remain independently bound.
func (d *legacyRetirementDriver) observeReplacements(ctx context.Context, o Observation) (Observation, error) {
	plan, reasons, err := d.replacementCheck(ctx)
	if err != nil {
		o.Unknown, o.Pending = true, "replacement setup could not be verified: "+err.Error()
		return o, nil
	}
	identity := plan
	identity.ID = ""
	identity.Inventories = nil
	identity.Operations = slices.Clone(plan.Operations)
	for i := range identity.Operations {
		identity.Operations[i].Observed.Inventory = ""
	}
	fingerprint, err := digest(identity)
	if err != nil {
		return o, err
	}
	o.Present, o.Fingerprint, o.Desired = true, fingerprint, plan.ID
	o.Healthy = len(reasons) == 0
	if !o.Healthy {
		o.HealthIssue = strings.Join(reasons, "; ")
		o.ApplyBlocked = "finish and verify the replacement setup before retiring earlier installations: " + o.HealthIssue
	}
	return o, nil
}

func (d *legacyRetirementDriver) replacementCheck(ctx context.Context) (Plan, []string, error) {
	state, err := LoadState(d.Normal.StatePath, d.Normal.Home)
	if err != nil {
		return Plan{}, nil, err
	}
	request := Request{Schema: 1, Mode: "check"}
	observed, err := Observe(ctx, d.Normal.Catalog, d.Normal.Context, state, request, d.Normal.Driver)
	if err != nil {
		return Plan{}, nil, err
	}
	plan, err := PlanChanges(d.Normal.Catalog, d.Normal.Context, state, request, observed, d.Normal.Source)
	if err != nil {
		return plan, nil, err
	}
	reasons := []string{}
	if state.Schema == 0 {
		reasons = append(reasons, "choose and apply replacement tools in setup first")
	}
	if state.Transaction != nil {
		reasons = append(reasons, "finish the interrupted setup operation first")
	}
	for _, op := range plan.Operations {
		if op.Action != "keep" && op.Action != "retain" && op.Action != "forget" {
			reasons = append(reasons, op.Resource+": "+op.Reason)
		}
	}
	return plan, reasons, nil
}

func (d *legacyRetirementDriver) replacementState() (string, error) {
	state, err := LoadState(d.Normal.StatePath, d.Normal.Home)
	if err != nil {
		return "", err
	}
	return digest(state)
}

func (d *legacyRetirementDriver) verifyReplacementState(intent legacyRetirementIntent) error {
	if intent.Resource == "retire.homebrew" {
		return nil
	}
	current, err := d.replacementState()
	if err != nil {
		return err
	}
	if current != intent.Scope.ReplacementState {
		return errors.New("replacement setup changed since retirement approval; abandon the saved operation and review the remaining installation again")
	}
	return nil
}

// replacementSelected reports whether the applied setup selection includes a
// replacement resource. It is disclosure only and never mutation authority.
func (d *legacyRetirementDriver) replacementSelected(id string) (bool, error) {
	state, err := LoadState(d.Normal.StatePath, d.Normal.Home)
	if err != nil {
		return false, err
	}
	if _, known := d.Normal.Catalog.Resource(id); !known {
		return false, fmt.Errorf("unknown replacement resource %s", id)
	}
	wanted, err := d.Normal.Catalog.Closure(append(slices.Clone(state.Selected), state.Keep...), d.Normal.Context)
	if err != nil {
		return false, err
	}
	return slices.Contains(wanted, id), nil
}

func legacyScopeSummary(scope legacyRetirementScope) string {
	if scope.Blocked != "" {
		return truncateLegacyText("blocked: "+scope.Blocked, 1024)
	}
	privileged, irreversible := 0, 0
	for _, step := range scope.Steps {
		if legacyStepNeedsAdministrator(step) {
			privileged++
		}
		if step.Irreversible {
			irreversible++
		}
	}
	text := fmt.Sprintf("%d verified steps (%d administrator, %d irreversible)", len(scope.Steps), privileged, irreversible)
	if len(scope.Retained) != 0 {
		text += "; retained: " + strings.Join(scope.Retained, "; ")
	}
	return truncateLegacyText(text, 1024)
}

// A refusal keeps the native error that caused it for diagnosis.
func legacyReason(reason string, err error) string {
	if err == nil {
		return reason
	}
	return reason + ": " + truncateLegacyText(err.Error(), 512)
}

func truncateLegacyText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.ToValidUTF8(text[:limit-16], "") + " [truncated]"
}

func legacyScopeLines(scope legacyRetirementScope) []string {
	lines := slices.Clone(scope.Detail)
	if scope.Native != "" {
		lines = append(lines, "Native identity: "+scope.Native)
	}
	for _, step := range scope.Steps {
		marker := ""
		if legacyStepNeedsAdministrator(step) {
			marker += " [administrator]"
		}
		if step.Irreversible {
			marker += " [irreversible]"
		}
		lines = append(lines, "- "+step.Label+marker)
	}
	for _, item := range scope.Retained {
		lines = append(lines, "Retained: "+item)
	}
	lines = append(lines, scope.Notices...)
	if scope.Blocked != "" {
		lines = append(lines, "Needs action: "+scope.Blocked)
	}
	return lines
}

func legacyPhaseResources(catalog *Catalog, target Context, phase string) []Resource {
	resources := []Resource{}
	for _, r := range catalog.Capabilities(target) {
		if phase == "" || (r.ID == "retire.homebrew") == (phase == LegacyRetirementPreparation) {
			resources = append(resources, r)
		}
	}
	return resources
}

// remaining reports unfinished retirements and separate explanations for
// deliberately kept resources. Kept shared or reused components do not require
// another action; every blocked or interrupted retirement still does.
func (m *LegacyRetirement) remaining(ctx context.Context, phase string) ([]Choice, []Choice, error) {
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return nil, nil, err
	}
	items, kept := []Choice{}, []Choice{}
	for _, r := range legacyPhaseResources(m.Controller.Catalog, m.Controller.Context, phase) {
		receipt := state.Receipts[r.ID]
		if intent, err := m.driver.readIntent(r.ID, receipt); err == nil && !intent.Complete && receipt.Status == "in-progress" {
			items = append(items, Choice{ID: r.ID, Label: r.Name, Detail: "interrupted retirement; resume or abandon it explicitly"})
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			items = append(items, Choice{ID: r.ID, Label: r.Name, Detail: "saved retirement evidence is unreadable: " + err.Error()})
			continue
		}
		scope, err := m.driver.inspect(ctx, r.ID)
		if err != nil {
			items = append(items, Choice{ID: r.ID, Label: r.Name, Detail: "inspection failed: " + err.Error()})
			continue
		}
		if owned, err := m.driver.keptScope(r.ID, scope); err != nil {
			return nil, nil, err
		} else if owned {
			kept = append(kept, Choice{ID: r.ID, Label: r.Name, Detail: legacyKeptHeader + "\n" + strings.Join(legacyScopeLines(scope), "\n")})
		} else if scope.Blocked != "" || len(scope.Steps) != 0 || len(scope.Retained) != 0 {
			items = append(items, Choice{ID: r.ID, Label: r.Name, Detail: strings.Join(legacyScopeLines(scope), "\n")})
		} else if len(scope.Detail) != 0 {
			kept = append(kept, Choice{ID: r.ID, Label: r.Name, Detail: strings.Join(scope.Detail, "\n")})
		}
	}
	return items, kept, nil
}

func (m *LegacyRetirement) result(ctx context.Context, result Result, phase string) (Result, error) {
	remaining, kept, err := m.remaining(ctx, phase)
	if err != nil {
		return Result{Status: "failed", Plan: result.Plan}, err
	}
	if len(remaining) != 0 && result.Status == "ready" {
		result.Status = "needs-action"
	}
	name := "runtime retirement"
	if phase == LegacyRetirementPreparation {
		name = "Homebrew preparation"
	}
	header := "Migration " + name + " is complete for the earlier installation. Personal data, configuration backups and unrelated packages were preserved."
	if result.Status != "ready" {
		header = "Migration " + name + " is not complete."
	}
	lines := []string{header, result.Message}
	for _, item := range remaining {
		lines = append(lines, "Remaining "+item.ID+":\n"+item.Detail)
	}
	for _, item := range kept {
		lines = append(lines, "Kept "+item.ID+":\n"+item.Detail)
	}
	result.Message = strings.TrimSpace(strings.Join(lines, "\n"))
	return result, nil
}

// Dispatch is the machine boundary of the runtime phase. Preview, retry and
// abandonment use the ordinary engine protocol; the result never reports
// readiness while any earlier artifact remains.
func (m *LegacyRetirement) Dispatch(ctx context.Context, request Request) (Result, error) {
	if !slices.Contains([]string{"apply", "check", "abandon"}, request.Mode) {
		return Result{Status: "failed"}, errors.New("runtime retirement supports apply, check, retry and abandonment of its saved operation")
	}
	result, err := m.Controller.Dispatch(ctx, request)
	if err != nil {
		return result, err
	}
	if result.Status == "preview" {
		return m.describePreview(ctx, result)
	}
	return m.result(ctx, result, "")
}

// Both interaction surfaces receive the exact scope of this preview.
// Reinspection must still match the plan; display cannot widen it.
func (m *LegacyRetirement) describePreview(ctx context.Context, result Result) (Result, error) {
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	lines := []string{}
	for _, op := range result.Plan.Operations {
		if op.Resource == legacyReplacements {
			continue
		}
		var scope legacyRetirementScope
		if op.Observed.ResourceResume != nil {
			lines = append(lines, "Recovery state: "+op.Observed.Pending)
			intent, err := m.driver.readIntent(op.Resource, state.Receipts[op.Resource])
			if err != nil {
				return Result{Status: "failed"}, err
			}
			token, _, err := m.driver.resumeToken(ctx, intent)
			if err != nil || token != op.Observed.ResourceResume.Token {
				return Result{Status: "failed"}, errors.Join(errors.New("retirement recovery changed while describing its preview"), err)
			}
			scope = intent.Scope
			scope.Steps = scope.Steps[intent.Next:]
		} else if op.Action == "install" {
			scope, err = m.driver.inspect(ctx, op.Resource)
			if err != nil {
				return Result{Status: "failed"}, err
			}
			identity, err := digest(scope)
			if err != nil || identity != op.Observed.Desired {
				return Result{Status: "failed"}, errors.Join(errors.New("retirement changed while describing its preview; request a fresh preview"), err)
			}
		} else {
			continue
		}
		lines = append(lines, op.Resource+":", strings.Join(legacyScopeLines(scope), "\n"))
	}
	result.Message = strings.Join(lines, "\n")
	return result, nil
}

// Decorate the ordinary recovery controller as well as first-time retirement,
// so every approval shows only the exact remaining, revalidated steps.
type legacyRetirementInteraction struct {
	Interaction
	retirement *LegacyRetirement
}

func (ui legacyRetirementInteraction) Review(ctx context.Context, catalog *Catalog, proposal Proposal, revise Revise) (bool, error) {
	describe := func(ctx context.Context, proposal Proposal) (Proposal, error) {
		preview, err := ui.retirement.describePreview(ctx, Result{Status: "preview", Plan: proposal.Plan})
		proposal.Details = preview.Message
		return proposal, err
	}
	proposal, err := describe(ctx, proposal)
	if err != nil {
		return false, err
	}
	if revise != nil {
		original := revise
		revise = func(ctx context.Context, choices []ReviewChoice) (Proposal, error) {
			next, err := original(ctx, choices)
			if err != nil {
				return next, err
			}
			return describe(ctx, next)
		}
	}
	return ui.Interaction.Review(ctx, catalog, proposal, revise)
}

func (m *LegacyRetirement) Run(ctx context.Context, ui Interaction, phase string) (Result, error) {
	if ui == nil || phase != LegacyRetirementPreparation && phase != LegacyRetirementRuntime {
		return Result{Status: "failed"}, errors.New("legacy retirement requires an interaction surface and an explicit phase")
	}
	ui = legacyRetirementInteraction{Interaction: ui, retirement: m}
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if state.Transaction != nil {
		result, err := m.Controller.Run(ctx, ui)
		if err != nil || result.Status == "cancelled" {
			return result, err
		}
		return m.finish(ctx, ui, result, phase)
	}
	choices, err := m.choices(ctx, state, phase)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if len(choices) == 0 {
		return m.finish(ctx, ui, Result{Status: "ready"}, phase)
	}
	title := "Retire these earlier installations? Each item lists its exact verified steps. Unchecked items stay unchanged."
	if phase == LegacyRetirementPreparation {
		title = "Convert Nix-managed Homebrew before setup? Installed packages are kept. Unchecked items stay unchanged."
	}
	selected, err := ui.Choose(ctx, title, choices, nil, true)
	if errors.Is(err, ErrCancelled) {
		return Result{Status: "cancelled"}, nil
	}
	if err != nil {
		return Result{Status: "failed"}, err
	}
	if err := validateChoice(selected, choices, true); err != nil {
		return Result{Status: "failed"}, err
	}
	if len(selected) == 0 {
		return m.finish(ctx, ui, Result{Status: "ready"}, phase)
	}
	// Historical retirement never selects a newly found installation. Only the
	// current choices authorize it; retained receipts preserve earlier evidence.
	request := Request{Schema: 1, Mode: "apply", Selected: sortedUnique(selected)}
	preview, err := m.Controller.Dispatch(ctx, request)
	if err != nil {
		return preview, err
	}
	approved, err := ui.Review(ctx, m.Controller.Catalog, Proposal{Plan: preview.Plan}, nil)
	if errors.Is(err, ErrCancelled) || err == nil && !approved {
		return Result{Status: "cancelled"}, nil
	}
	if err != nil {
		return Result{Status: "failed"}, err
	}
	request.ExpectedPlan = preview.Plan.ID
	applying, err := ui.BeginApply(ctx)
	if err != nil {
		return Result{Status: "failed"}, err
	}
	result, err := m.Controller.Dispatch(applying, request)
	if err != nil {
		return result, err
	}
	return m.finish(ctx, ui, result, phase)
}

func (m *LegacyRetirement) choices(ctx context.Context, state State, phase string) ([]Choice, error) {
	choices := []Choice{}
	for _, r := range legacyPhaseResources(m.Controller.Catalog, m.Controller.Context, phase) {
		scope, err := m.driver.inspect(ctx, r.ID)
		if err != nil {
			continue // remaining reports this resource's inspection failure.
		}
		if scope.Blocked != "" || len(scope.Steps) == 0 {
			continue // Reported with the phase result; it is not an approvable choice.
		}
		choices = append(choices, Choice{ID: r.ID, Label: r.Name, Detail: r.Description + "\n" + strings.Join(legacyScopeLines(scope), "\n")})
	}
	return choices, nil
}

// LegacyInstallationHints reports earlier dotfiles evidence with path checks
// only, so setup can disclose it. It runs no command and grants no authority.
func LegacyInstallationHints(platform NativePlatform, folders ConfigFolders) []string {
	hints := []string{}
	exists := func(path string) bool {
		_, err := os.Lstat(path)
		return err == nil
	}
	share := filepath.Join(folders.Home, ".local", "share", "dotfiles")
	for _, item := range []struct{ path, text string }{
		{filepath.Join(share, "provenance"), "released direct-binary provenance markers"},
		{filepath.Join(share, "tmux-plugins"), "released pinned tmux plugin checkouts"},
		{filepath.Join(share, "zsh-plugins"), "released pinned zsh plugin checkouts"},
		{filepath.Join(share, "sentinel"), "released Sentinel source cache"},
		{filepath.Join(folders.Home, ".config", "chezmoi"), "chezmoi configuration and state"},
		{filepath.Join(folders.Home, ".local", "state", "home-manager"), "Home Manager state"},
	} {
		if exists(item.path) {
			hints = append(hints, item.text+" at "+item.path)
		}
	}
	if platform.Homebrew != nil && LegacyNixHomebrewIssue(*platform.Homebrew) != "" {
		hints = append(hints, "Nix-managed Homebrew at "+platform.Homebrew.Prefix)
	}
	if platform.OS == "darwin" && exists("/run/current-system") {
		hints = append(hints, "nix-darwin system activation at /run/current-system")
	}
	if len(hints) != 0 && platform.OS != "windows" && exists("/nix") {
		hints = append(hints, "Nix runtime at /nix")
	}
	return hints
}

// LegacyNixHomebrewIssue identifies a Homebrew prefix still run from the Nix
// store by nix-homebrew. Setup must not reuse it as native infrastructure.
func LegacyNixHomebrewIssue(location HomebrewLocation) string {
	issue := "Homebrew at " + location.Prefix + " is managed by nix-homebrew; run migrate to convert it before setup uses Homebrew"
	for _, path := range []string{filepath.Join(location.Prefix, ".managed_by_nix_darwin"), filepath.Join(location.Prefix, "Library", ".homebrew-is-managed-by-nix")} {
		if _, err := os.Lstat(path); err == nil {
			return issue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
	}
	for _, path := range []string{location.Program, filepath.Join(location.Prefix, "Library", "Homebrew")} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "Homebrew ownership could not be inspected: " + err.Error()
		}
		if strings.HasPrefix(target, "/nix/store/") {
			return issue
		}
	}
	return ""
}
