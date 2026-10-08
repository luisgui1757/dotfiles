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

// ConfigDriver applies the bounded canonical mapping. It runs under the engine
// lock and starts no surviving writers. User baselines stay on the destination
// volume so restoration preserves native permissions, ACLs and symbolic links.
type ConfigDriver struct {
	Catalog               *Catalog
	Manifest              ConfigManifest
	Target                Context
	Folders               ConfigFolders
	Repository, Directory string
	// Fixed native integrations may still be unfinished after their file phase.
	allowCompleteRestore bool
}

type configBaselineTarget struct {
	Destination string         `json:"destination"`
	Original    configSnapshot `json:"original"`
	Backup      string         `json:"backup"`
}

type configBaseline struct {
	Schema    int                    `json:"schema"`
	Resource  string                 `json:"resource"`
	Operation string                 `json:"operation"`
	Before    Observation            `json:"before"`
	Targets   []configBaselineTarget `json:"targets"`
}

type configPublication struct {
	State      configTargetState        `json:"state"`
	Workspace  string                   `json:"workspace"`
	Restore    configBaselineTarget     `json:"restore"`
	Phase      string                   `json:"phase,omitempty"`
	Discarding bool                     `json:"discarding,omitempty"` // Legacy flag grants no deletion authority.
	Cleanup    map[string][]configEntry `json:"cleanup,omitempty"`
}

var errConfigJournalAbsent = errors.New("configuration operation has no journal")

type configJournal struct {
	Schema           int                 `json:"schema"`
	Resource         string              `json:"resource"`
	Operation        string              `json:"operation"`
	Action           string              `json:"action"`
	Recovery         string              `json:"recovery"`
	Before           Observation         `json:"before"`
	Entries          []configPublication `json:"entries"`
	Staged           bool                `json:"staged"`
	Complete         bool                `json:"complete"`
	Cancelled        bool                `json:"cancelled,omitempty"`
	Sealed           bool                `json:"sealed,omitempty"`
	PreservePrevious bool                `json:"preserve_previous,omitempty"`
	Restoring        bool                `json:"restoring,omitempty"`
	Preserved        []string            `json:"preserved,omitempty"`
}

func (d *ConfigDriver) journalPath(operation string) string {
	return filepath.Join(d.Directory, "configuration", "operations", operation+".json")
}

func (d *ConfigDriver) journalError(receipt Receipt, err error) error {
	paths := []string{d.journalPath(receipt.OperationID)}
	if filepath.IsLocal(receipt.Recovery) && filepath.Clean(receipt.Recovery) == receipt.Recovery && filepath.Dir(filepath.Dir(receipt.Recovery)) == "recovery" {
		paths = append(paths, d.baselinePath(receipt.Recovery))
	}
	return fmt.Errorf("cannot use saved configuration evidence; preserve and inspect %s: %w", strings.Join(paths, ", "), err)
}

func (d *ConfigDriver) validate(r Resource, receipt Receipt) error {
	if d.Catalog == nil || r.Action != "config" || !filepath.IsAbs(d.Directory) || !filepath.IsAbs(d.Repository) {
		return errors.New("configuration provider requires canonical source and state directories")
	}
	if receipt.Recovery != "" && (!filepath.IsLocal(receipt.Recovery) || filepath.Clean(receipt.Recovery) != receipt.Recovery || filepath.Dir(filepath.Dir(receipt.Recovery)) != "recovery") {
		return errors.New("configuration recovery reference is outside the engine recovery directory")
	}
	if receipt.OperationID != "" && !operationID.MatchString(receipt.OperationID) {
		return errors.New("invalid configuration operation identity")
	}
	return d.Manifest.Validate(d.Catalog)
}

func (d *ConfigDriver) readJournal(r Resource, receipt Receipt) (configJournal, error) {
	var j configJournal
	if receipt.OperationID == "" {
		return j, errConfigJournalAbsent
	}
	data, err := readDocument(d.journalPath(receipt.OperationID))
	if errors.Is(err, os.ErrNotExist) {
		return j, errConfigJournalAbsent
	}
	if err != nil {
		return j, d.journalError(receipt, err)
	}
	if err := Decode(data, &j); err != nil {
		return j, d.journalError(receipt, err)
	}
	if j.Schema != 1 || j.Resource != r.ID || j.Operation != receipt.OperationID || j.Recovery != receipt.Recovery || len(j.Entries) == 0 ||
		!slices.Contains([]string{"install", "adopt", "update", "repair", "remove"}, j.Action) {
		return j, d.journalError(receipt, errors.New("configuration journal differs from durable engine intent"))
	}
	if j.Cancelled {
		return d.readRecoveryJournal(r.ID, receipt)
	}
	baseline, err := d.readBaseline(r, receipt)
	if err != nil {
		return j, d.journalError(receipt, err)
	}
	if len(j.Entries) != len(baseline.Targets) {
		return j, d.journalError(receipt, errors.New("configuration journal differs from original baseline targets"))
	}
	for i, entry := range j.Entries {
		if entry.Workspace != configWorkspace(entry.State.Target.Destination, j.Operation, i) || entry.Restore != baseline.Targets[i] || entry.Restore.Destination != entry.State.Target.Destination ||
			!slices.Contains([]string{"", "moving", "moved", "publishing", "published"}, entry.Phase) {
			return j, d.journalError(receipt, errors.New("configuration publication has invalid recovery paths or phase"))
		}
	}
	return j, nil
}

func configWorkspace(destination, operation string, index int) string {
	return filepath.Join(filepath.Dir(destination), fmt.Sprintf(".dotfiles-config-%s-%d", operation, index))
}

func (d *ConfigDriver) baselinePath(recovery string) string {
	return filepath.Join(d.Directory, recovery, "configuration.json")
}

func (d *ConfigDriver) readBaseline(r Resource, receipt Receipt) (configBaseline, error) {
	var baseline configBaseline
	if receipt.Recovery == "" {
		return baseline, os.ErrNotExist
	}
	data, err := readDocument(d.baselinePath(receipt.Recovery))
	if err != nil {
		return baseline, err
	}
	if err := Decode(data, &baseline); err != nil {
		return baseline, err
	}
	if baseline.Schema != 1 || baseline.Resource != r.ID || !operationID.MatchString(baseline.Operation) || !sameArtifact(baseline.Before, receipt.Before) || len(baseline.Targets) == 0 {
		return baseline, errors.New("configuration baseline differs from the original receipt")
	}
	for i, target := range baseline.Targets {
		if target.Backup != filepath.Join(configWorkspace(target.Destination, baseline.Operation, i), "previous") || !filepath.IsAbs(target.Destination) {
			return baseline, errors.New("configuration baseline contains invalid adjacent recovery storage")
		}
	}
	return baseline, nil
}

func (d *ConfigDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := errors.Join(ctx.Err(), d.validate(r, receipt)); err != nil {
		return Observation{}, err
	}
	states, o, err := inspectConfiguration(d.Catalog, d.Manifest, d.Target, d.Folders, d.Repository, r.ID)
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	o.Preserved = slices.Clone(receipt.After.Preserved)
	j, err := d.readJournal(r, receipt)
	if errors.Is(err, errConfigJournalAbsent) {
		if receipt.Ownership == "created" && receipt.Status != "in-progress" && receipt.OperationID != "" {
			return configObservationProblem(receipt, errors.New("recorded configuration operation is missing; preserve the target and recovery data"))
		}
		return o, nil
	}
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	o.Preserved = sortedUnique(append(o.Preserved, j.Preserved...))
	if len(states) != len(j.Entries) {
		return configObservationProblem(receipt, errors.New("configuration targets changed; use explicit migration to preserve the original targets"))
	}
	for i, state := range states {
		previous := j.Entries[i].State.Target
		if j.Complete {
			previous.Source = state.Target.Source
		}
		if state.Target != previous {
			return configObservationProblem(receipt, fmt.Errorf("configuration target moved or changed shape; preserve recorded target %s and use explicit migration", j.Entries[i].State.Target.Destination))
		}
	}
	if j.Restoring && (!j.Complete || receipt.Status == "in-progress") {
		o.Healthy, o.Adoptable, o.Restoring = false, false, true
		o.Pending = "configuration restoration is unfinished; continue restoring the saved files"
		return o, nil
	}
	if j.Complete {
		if j.PreservePrevious && !j.Cancelled {
			for _, entry := range j.Entries {
				if entry.State.Current.Kind != "absent" {
					o.Preserved = append(o.Preserved, filepath.Join(entry.Workspace, "previous"))
				}
			}
			o.Preserved = sortedUnique(o.Preserved)
		}
		if j.Action != "remove" && !j.Cancelled || receipt.Ownership == "created" && j.Cancelled {
			baseline, err := d.readBaseline(r, receipt)
			if err != nil {
				return configObservationProblem(receipt, err)
			}
			for _, target := range baseline.Targets {
				if err := verifyConfigSnapshot(target.Backup, target.Original); err != nil {
					o.Healthy, o.Adoptable, o.Pending = false, false, "configuration recovery baseline changed; preserve both current and recovery files"
					return o, nil
				}
			}
		}
		matches := true
		for i, state := range states {
			matches = matches && state.Current == configPublicationResult(j, j.Entries[i])
		}
		if matches && j.Action != "remove" && !j.Cancelled {
			o.CompletedOperation = j.Operation
		}
		return o, nil
	}
	// Include every recovery artifact in approval, not just the partially
	// published target. Editing a saved baseline invalidates a retry too.
	recovery := []configSnapshot{}
	for _, entry := range j.Entries {
		for _, path := range []string{filepath.Join(entry.Workspace, "partial"), filepath.Join(entry.Workspace, "next"), filepath.Join(entry.Workspace, "previous"), entry.Restore.Backup} {
			snapshot, err := snapshotConfig(path)
			if err != nil {
				return o, err
			}
			recovery = append(recovery, snapshot)
		}
	}
	token, err := digest(struct {
		Journal  configJournal
		Actual   []configTargetState
		Recovery []configSnapshot
	}{j, states, recovery})
	if err != nil {
		return o, err
	}
	o.Healthy, o.Adoptable, o.Pending = false, false, "configuration publication interrupted; resume the saved operation"
	o.ResourceResume = &ResourceResume{Operation: j.Operation, Token: token}
	return o, nil
}

func configObservationProblem(receipt Receipt, problem error) (Observation, error) {
	return Observation{Unknown: true, Provider: "configuration", Scope: "user", Pending: problem.Error(), Preserved: slices.Clone(receipt.After.Preserved)}, nil
}

func configPublicationResult(j configJournal, entry configPublication) configSnapshot {
	if j.Action == "remove" {
		return entry.Restore.Original
	}
	return entry.State.Desired
}

func (d *ConfigDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, op.Observed, receipt)
}

func (d *ConfigDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt.After, receipt)
}

func (d *ConfigDriver) change(ctx context.Context, r Resource, action string, approved Observation, receipt Receipt) (Observation, error) {
	if err := errors.Join(ctx.Err(), d.validate(r, receipt)); err != nil {
		return Observation{}, err
	}
	if receipt.Recovery == "" || !operationID.MatchString(receipt.OperationID) || receipt.Status != "in-progress" {
		return Observation{}, errors.New("configuration mutation requires saved engine intent")
	}
	if _, err := d.readJournal(r, receipt); !errors.Is(err, errConfigJournalAbsent) {
		return Observation{}, errors.Join(errors.New("configuration operation already exists; explicitly resume it"), err)
	}
	states, observed, err := inspectConfiguration(d.Catalog, d.Manifest, d.Target, d.Folders, d.Repository, r.ID)
	if err != nil || !sameArtifact(approved, observed) || action != "remove" && approved.Desired != observed.Desired {
		return observed, errors.Join(errors.New("configuration changed after approval"), err)
	}
	if action != "remove" && observed.ApplyBlocked != "" {
		return observed, errors.New(observed.ApplyBlocked)
	}
	baseline, err := d.readBaseline(r, receipt)
	if errors.Is(err, os.ErrNotExist) && (action == "install" || action == "adopt") && sameArtifact(receipt.Before, observed) {
		baseline = configBaseline{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Before: receipt.Before}
		for i, state := range states {
			baseline.Targets = append(baseline.Targets, configBaselineTarget{state.Target.Destination, state.Current, filepath.Join(configWorkspace(state.Target.Destination, receipt.OperationID, i), "previous")})
		}
		err = saveDocument(d.baselinePath(receipt.Recovery), baseline)
	}
	if err != nil {
		return observed, err
	}
	if len(states) != len(baseline.Targets) {
		return observed, errors.New("configuration mapping changed; explicit migration is required")
	}
	j := configJournal{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Action: action, Recovery: receipt.Recovery, Before: observed}
	j.PreservePrevious = action == "adopt" && baseline.Operation != j.Operation
	for i, state := range states {
		if state.Target.Destination != baseline.Targets[i].Destination {
			return observed, errors.New("configuration baseline destination changed")
		}
		workspace := configWorkspace(state.Target.Destination, j.Operation, i)
		if _, err := os.Lstat(workspace); !errors.Is(err, os.ErrNotExist) {
			return observed, errors.Join(errors.New("configuration recovery location already exists without operation intent"), err)
		}
		j.Entries = append(j.Entries, configPublication{State: state, Workspace: workspace, Restore: baseline.Targets[i]})
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return observed, err
	}
	return d.complete(ctx, r, receipt, &j)
}

func (d *ConfigDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if err := errors.Join(ctx.Err(), d.validate(r, receipt)); err != nil {
		return Observation{}, err
	}
	j, err := d.readJournal(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	if j.Action != op.Action || j.Complete || j.Cancelled || j.Restoring {
		return Observation{}, errors.New("configuration recovery action differs from saved intent")
	}
	current, err := d.Observe(ctx, r, receipt)
	if err != nil || current.ResourceResume == nil || op.Observed.ResourceResume == nil || *current.ResourceResume != *op.Observed.ResourceResume {
		return current, errors.Join(errors.New("configuration recovery artifacts changed after approval"), err)
	}
	if j.Action != "remove" && current.Desired != j.Before.Desired {
		return current, errors.New("configuration source changed; restore the originally approved source before resuming")
	}
	return d.complete(ctx, r, receipt, &j)
}

func (d *ConfigDriver) complete(ctx context.Context, r Resource, receipt Receipt, j *configJournal) (Observation, error) {
	if !j.Staged {
		for i := range j.Entries {
			if err := ctx.Err(); err != nil {
				return Observation{}, err
			}
			if err := d.stage(*j, j.Entries[i]); err != nil {
				return Observation{}, err
			}
		}
		j.Staged = true
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return Observation{}, err
		}
	}
	for i := range j.Entries {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		if err := d.publish(j, i); err != nil {
			return Observation{}, err
		}
	}
	j.Complete = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// The durable operation reserves a fresh, identity-named workspace before
// touching targets. No second ownership marker is needed. Unknown contents are
// never deleted; target ownership still requires verified publication proof.
func verifyConfigWorkspace(j configJournal, entry configPublication, initialize bool) error {
	info, err := os.Lstat(entry.Workspace)
	if errors.Is(err, os.ErrNotExist) && initialize {
		bound, bindErr := bindConfigDestination(entry.State.Target.Destination)
		if bindErr != nil || bound != entry.State.Target.Destination {
			return errors.Join(errors.New("configuration parent changed"), bindErr)
		}
		if err := os.MkdirAll(filepath.Dir(entry.Workspace), 0700); err != nil {
			return err
		}
		if err := os.Mkdir(entry.Workspace, 0700); err != nil {
			return err
		}
		info, err = os.Lstat(entry.Workspace)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("configuration workspace is not a private directory")
	}
	return nil
}

func (d *ConfigDriver) stage(j configJournal, entry configPublication) error {
	if err := verifyConfigWorkspace(j, entry, true); err != nil {
		return err
	}
	if j.Action == "remove" {
		return verifyConfigSnapshot(entry.Restore.Backup, entry.Restore.Original)
	}
	next := filepath.Join(entry.Workspace, "next")
	staged, err := snapshotConfig(next)
	if err != nil {
		return err
	}
	if staged == entry.State.Desired {
		return nil
	}
	if staged.Kind != "absent" {
		return errors.New("staged configuration changed; preserve it for recovery")
	}
	input, err := inspectConfigInput(entry.State.Target)
	if err != nil || input != entry.State.Input {
		return errors.Join(errors.New("configuration source changed before staging"), err)
	}
	partial := filepath.Join(entry.Workspace, "partial")
	// Only this private unpublished payload is disposable. A user baseline or
	// installed target is never recursively removed by staging.
	root, err := os.OpenRoot(entry.Workspace)
	if err != nil {
		return err
	}
	err = errors.Join(root.RemoveAll("partial"), root.Close())
	if err != nil {
		return err
	}
	if entry.State.Target.Mode == "link" {
		err = os.Symlink(entry.State.Target.Source, partial)
	} else if entry.State.Target.Mode == "junction" {
		err = createDirectoryLink(entry.State.Target.Source, partial)
	} else {
		err = copyConfigPayload(entry.State.Target.Source, partial)
	}
	if err != nil {
		return err
	}
	if err := verifyConfigSnapshot(partial, entry.State.Desired); err != nil {
		return err
	}
	return moveConfigExclusive(partial, next)
}

func verifyConfigSnapshot(path string, expected configSnapshot) error {
	actual, err := snapshotConfig(path)
	if err != nil || actual != expected {
		return errors.Join(fmt.Errorf("configuration artifact changed at %s; preserve it for recovery", path), err)
	}
	return nil
}

func (d *ConfigDriver) publish(j *configJournal, index int) error {
	entry := &j.Entries[index]
	if err := verifyConfigWorkspace(*j, *entry, false); err != nil {
		return err
	}
	target, previous := entry.State.Target.Destination, filepath.Join(entry.Workspace, "previous")
	result := configPublicationResult(*j, *entry)
	payload := filepath.Join(entry.Workspace, "next")
	if j.Action == "remove" {
		payload = entry.Restore.Backup
	}
	phase := func(next string) error {
		entry.Phase = next
		return saveDocument(d.journalPath(j.Operation), j)
	}
	if entry.Phase == "" {
		if err := verifyConfigSnapshot(target, entry.State.Current); err != nil {
			return err
		}
		if err := verifyConfigSnapshot(previous, configSnapshot{Kind: "absent"}); err != nil {
			return err
		}
		if err := phase("moving"); err != nil {
			return err
		}
	}
	if entry.Phase == "moving" {
		before, err := snapshotConfig(previous)
		if err != nil {
			return err
		}
		if entry.State.Current.Kind != "absent" && before.Kind == "absent" {
			if err := verifyConfigSnapshot(target, entry.State.Current); err != nil {
				return err
			}
			if err := moveConfigExclusive(target, previous); err != nil {
				return err
			}
		}
		if err := verifyConfigSnapshot(previous, entry.State.Current); err != nil {
			return err
		}
		if err := verifyConfigSnapshot(target, configSnapshot{Kind: "absent"}); err != nil {
			return err
		}
		if err := phase("moved"); err != nil {
			return err
		}
	}
	if entry.Phase == "moved" {
		if err := verifyConfigSnapshot(payload, result); err != nil {
			return err
		}
		if err := verifyConfigSnapshot(target, configSnapshot{Kind: "absent"}); err != nil {
			return err
		}
		if err := phase("publishing"); err != nil {
			return err
		}
	}
	if entry.Phase == "publishing" {
		staged, err := snapshotConfig(payload)
		if err != nil {
			return err
		}
		if result.Kind != "absent" && staged == result {
			if err := moveConfigExclusive(payload, target); err != nil {
				return err
			}
		} else if staged.Kind != "absent" {
			return errors.New("saved configuration payload changed during publication")
		}
		if err := verifyConfigSnapshot(target, result); err != nil {
			return err
		}
		if err := phase("published"); err != nil {
			return err
		}
	}
	return verifyConfigSnapshot(target, result)
}

// Seal only complete publications. Unfinished files remain under their exact
// original operation; even explicit abandonment cannot discard a half-move.
func (d *ConfigDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	preserved := map[string][]string{}
	directory := filepath.Join(d.Directory, "configuration", "operations")
	if err := requireCompletedJournals(plan, receipts, "configuration", directory); err != nil {
		return nil, err
	}
	files, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return preserved, nil
	}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if filepath.Ext(file.Name()) != ".json" || !operationID.MatchString(strings.TrimSuffix(file.Name(), ".json")) {
			continue
		}
		path := filepath.Join(directory, file.Name())
		operation := strings.TrimSuffix(file.Name(), ".json")
		data, err := readDocument(path)
		var header configJournal
		if err == nil {
			err = Decode(data, &header)
		}
		if err == nil && (header.Schema != 1 || !resourceID.MatchString(header.Resource) || operation != header.Operation || header.Sealed && !header.Complete) {
			err = errors.New("invalid configuration journal identity or completion")
		}
		if err != nil {
			if journalRequired(plan, receipts, operation) {
				for _, receipt := range receipts {
					if receipt.OperationID == operation {
						return nil, d.journalError(receipt, err)
					}
				}
				return nil, fmt.Errorf("required configuration journal is invalid at %s: %w", path, err)
			}
			preserved[""] = append(preserved[""], path)
			continue
		}
		if header.Sealed {
			preserved[header.Resource] = sortedUnique(append(preserved[header.Resource], header.Preserved...))
			continue
		}
		// Completion is independent of today's source, manifest and baseline document.
		// Those inputs still govern application; they cannot strand completed history.
		receipt := Receipt{Recovery: header.Recovery, OperationID: header.Operation}
		j, err := d.readRecoveryJournal(header.Resource, receipt)
		if err != nil {
			if journalRequired(plan, receipts, operation) {
				return nil, err
			}
			preserved[header.Resource] = append(preserved[header.Resource], path)
			continue
		}
		if !j.Complete {
			if plan.Mode != "abandon" || j.Restoring {
				return nil, errors.New("unfinished configuration operation must be recovered")
			}
			for _, entry := range j.Entries {
				if entry.Phase != "" {
					return nil, errors.New("configuration publication started; recover before abandoning")
				}
			}
			j.Cancelled, j.Complete = true, true
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				return nil, err
			}
		}
		for i := range j.Entries {
			if err := d.cleanWorkspace(&j, i); err != nil {
				return nil, err
			}
		}
		j.Preserved, j.Sealed = sortedUnique(j.Preserved), true
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return nil, err
		}
		preserved[j.Resource] = sortedUnique(append(preserved[j.Resource], j.Preserved...))
	}
	return preserved, nil
}

func (d *ConfigDriver) cleanWorkspace(j *configJournal, index int) error {
	entry := &j.Entries[index]
	// An altered baseline is retained and disclosed even when it lives in an
	// earlier operation's workspace. Normal observation flags the affected tool.
	if entry.Restore.Original.Kind != "absent" && j.Action != "remove" {
		if err := verifyConfigSnapshot(entry.Restore.Backup, entry.Restore.Original); err != nil {
			j.Preserved = append(j.Preserved, entry.Restore.Backup)
		}
	}
	bound, err := bindConfigDestination(entry.Workspace)
	if err != nil || bound != entry.Workspace {
		j.Preserved = append(j.Preserved, entry.Workspace)
		return nil
	}
	info, err := os.Lstat(entry.Workspace)
	if errors.Is(err, os.ErrNotExist) {
		if j.Action == "remove" {
			return removeEmptyConfigWorkspace(filepath.Dir(entry.Restore.Backup))
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		j.Preserved = append(j.Preserved, entry.Workspace)
		return nil
	}
	children, err := os.ReadDir(entry.Workspace)
	if err != nil {
		return err
	}
	if entry.Cleanup == nil {
		entry.Cleanup = map[string][]configEntry{}
	}
	for _, child := range children {
		path := filepath.Join(entry.Workspace, child.Name())
		// Once disclosed as personal data, a path cannot become cleanup
		// material on retry, even when another sibling was already removed.
		if cleanupPreserved(path, j.Preserved) {
			continue
		}
		var expected configSnapshot
		switch child.Name() {
		case "previous":
			if path == entry.Restore.Backup || j.PreservePrevious {
				if j.PreservePrevious || verifyConfigSnapshot(path, entry.Restore.Original) != nil {
					j.Preserved = append(j.Preserved, path)
				}
				continue
			}
			expected = entry.State.Current
		case "next":
			if !j.Cancelled {
				j.Preserved = append(j.Preserved, path)
				continue
			}
			expected = entry.State.Desired
		case "partial":
			if !j.Cancelled {
				j.Preserved = append(j.Preserved, path)
				continue
			}
		default:
			j.Preserved = append(j.Preserved, path)
			continue
		}
		entries, recorded := entry.Cleanup[child.Name()]
		if !recorded {
			var err error
			entries, err = inspectTree(path, maxConfigBytes, maxConfigEntries)
			actual, hashErr := snapshotEntries(entries)
			if err != nil || hashErr != nil || expected.Kind != "" && actual != expected || entry.Discarding && child.Name() == "partial" {
				j.Preserved = append(j.Preserved, path)
				continue
			}
			entry.Cleanup[child.Name()] = entries
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				return err
			}
		}
		kept, err := discardRecordedTree(path, entries, maxConfigBytes, maxConfigEntries, j.Preserved...)
		j.Preserved = sortedUnique(append(j.Preserved, kept...))
		// Persist preservation even if another leaf is temporarily locked.
		if saveErr := saveDocument(d.journalPath(j.Operation), j); err != nil || saveErr != nil {
			return errors.Join(err, saveErr)
		}
	}
	if err := removeEmptyConfigWorkspace(entry.Workspace); err != nil {
		return err
	}
	if j.Action == "remove" {
		// The original baseline was restored from a previously sealed
		// workspace. Remove its empty container, preserving foreign contents.
		return removeEmptyConfigWorkspace(filepath.Dir(entry.Restore.Backup))
	}
	return nil
}

func removeEmptyConfigWorkspace(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	if err := os.Remove(path); err != nil {
		remaining, readErr := os.ReadDir(path)
		if readErr == nil && len(remaining) > 0 || errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		return errors.Join(err, readErr)
	}
	return nil
}
