package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const maxProfileBytes = 256 << 10

type ProfileTarget struct{ Path, Script string }

// ProfileDriver owns marked shell/policy blocks or explicit JSON properties,
// never entire settings files. They share metadata-preserving publication.
// Targets are explicit native paths. A profile redirect is not write authority.
type ProfileDriver struct {
	Directory  string
	Targets    map[string][]ProfileTarget
	Windows    bool
	JSONFields map[string][]string
}

type profileBaselineEntry struct {
	Path   string `json:"path"`
	Block  []byte `json:"block,omitempty"`
	Absent bool   `json:"absent"`
}
type profileBaseline struct {
	Schema   int                    `json:"schema"`
	Resource string                 `json:"resource"`
	Before   Observation            `json:"before"`
	Entries  []profileBaselineEntry `json:"entries"`
	Fields   []string               `json:"fields,omitempty"`
}
type profilePublication struct {
	Path      string         `json:"path"`
	Workspace string         `json:"workspace"`
	Before    configSnapshot `json:"before"`
	After     configSnapshot `json:"after"`
	Content   []byte         `json:"content,omitempty"`
	Block     []byte         `json:"block,omitempty"`
	Staged    bool           `json:"staged"`
	Moved     bool           `json:"moved"`
	Published bool           `json:"published"`
}
type profileJournal struct {
	Schema                                int `json:"schema"`
	Resource, Operation, Recovery, Action string
	Entries                               []profilePublication `json:"entries"`
	Complete                              bool                 `json:"complete"`
	Sealed                                bool                 `json:"sealed"`
	Cancelled                             bool                 `json:"cancelled,omitempty"`
	Restoring                             bool                 `json:"restoring,omitempty"`
	Restoration                           []profilePublication `json:"restoration,omitempty"`
	Preserved                             []string             `json:"preserved,omitempty"`
	Fields                                []string             `json:"fields,omitempty"`
}

func readProfile(path string) ([]byte, configSnapshot, error) {
	bound, err := bindConfigDestination(path)
	if err != nil || path != bound {
		return nil, configSnapshot{}, errors.Join(errors.New("profile parent moved or was redirected"), err)
	}
	snapshot, err := snapshotTree(path, maxProfileBytes, 1)
	if err != nil || snapshot.Kind == "absent" {
		return nil, snapshot, err
	}
	if snapshot.Kind != "file" {
		return nil, snapshot, errors.New("profile is not a regular file; preserve its redirect and use explicit migration")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, snapshot, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxProfileBytes+1))
	err = errors.Join(err, f.Close())
	if err != nil || len(data) > maxProfileBytes {
		return nil, snapshot, errors.Join(errors.New("profile exceeds its read bound"), err)
	}
	if err := verifyConfigSnapshot(path, snapshot); err != nil {
		return nil, snapshot, err
	}
	_, _, err = profileEncoding(data)
	return data, snapshot, err
}

func (d *ProfileDriver) validate(id string) error {
	targets, ok := d.Targets[id]
	if !ok || !resourceID.MatchString(id) || !filepath.IsAbs(d.Directory) || len(targets) == 0 || len(targets) > 8 {
		return errors.New("invalid profile resource or target list")
	}
	seen := map[string]bool{}
	if err := validateJSONFields(d.JSONFields[id]); err != nil {
		return err
	}
	for _, target := range targets {
		if !filepath.IsAbs(target.Path) || filepath.Clean(target.Path) != target.Path || seen[target.Path] {
			return errors.New("invalid or duplicate profile path")
		}
		seen[target.Path] = true
		if _, err := renderProfileBlock(nil, id, target.Script, d.JSONFields[id]...); err != nil {
			return err
		}
	}
	return nil
}

func profileIdentity(paths, fields []string) (string, error) {
	if len(fields) == 0 {
		return digest(paths)
	}
	return digest(struct{ Paths, Fields []string }{paths, fields})
}

func (d *ProfileDriver) journalPath(operation string) string {
	return filepath.Join(d.Directory, "profiles", "operations", operation+".json")
}
func (d *ProfileDriver) baselinePath(recovery string) string {
	return filepath.Join(d.Directory, recovery, "profile.json")
}

func (d *ProfileDriver) readBaseline(id string, receipt Receipt) (profileBaseline, error) {
	var baseline profileBaseline
	if !validProfileRecovery(receipt.Recovery) {
		return baseline, errors.New("invalid profile baseline reference")
	}
	data, err := readDocument(d.baselinePath(receipt.Recovery))
	if err != nil {
		return baseline, err
	}
	if err := Decode(data, &baseline); err != nil {
		return baseline, err
	}
	if baseline.Schema != 1 || !slices.Equal(baseline.Fields, d.JSONFields[id]) || baseline.Resource != id || !sameArtifact(baseline.Before, receipt.Before) || len(baseline.Entries) != len(d.Targets[id]) {
		return baseline, errors.New("profile baseline differs from receipt")
	}
	paths, blocks := []string{}, [][]byte{}
	present := false
	for i, e := range baseline.Entries {
		_, block, err := replaceProfileBlock(e.Block, id, nil, baseline.Fields...)
		if e.Path != d.Targets[id][i].Path || len(e.Block) > maxProfileBytes || err != nil || !bytes.Equal(block, e.Block) || e.Absent && len(e.Block) != 0 {
			return baseline, errors.Join(errors.New("profile baseline has invalid targets or blocks"), err)
		}
		paths, blocks = append(paths, e.Path), append(blocks, e.Block)
		blockPresent, err := profileBlockPresent(id, e.Block)
		if err != nil {
			return baseline, err
		}
		present = present || blockPresent
	}
	identity, identityErr := profileIdentity(paths, baseline.Fields)
	fingerprint, fingerprintErr := profileBlockFingerprint(id, blocks)
	if err := errors.Join(identityErr, fingerprintErr); err != nil {
		return baseline, err
	}
	if id == windowsTerminalResource {
		full, err := digest(blocks)
		if err != nil || full != baseline.Before.Inventory || full != receipt.Before.Inventory {
			return baseline, errors.Join(errors.New("Windows Terminal baseline bookkeeping differs from its receipt"), err)
		}
	}
	if baseline.Before.Present != present || baseline.Before.Identity != identity || baseline.Before.Fingerprint != fingerprint {
		return baseline, errors.New("profile baseline bytes differ from the original receipt")
	}
	return baseline, nil
}

func (d *ProfileDriver) inspect(id string) (Observation, error) {
	if err := d.validate(id); err != nil {
		return Observation{}, err
	}
	paths, blocks, desired := []string{}, [][]byte{}, [][]byte{}
	healthy, present := true, false
	for _, target := range d.Targets[id] {
		data, snapshot, err := readProfile(target.Path)
		if err != nil {
			return Observation{}, err
		}
		if len(d.JSONFields[id]) > 0 && snapshot.Kind == "file" && len(data) == 0 {
			return Observation{}, errors.New("existing JSON settings are empty; preserve and correct the file before continuing")
		}
		_, block, err := replaceProfileBlock(data, id, nil, d.JSONFields[id]...)
		if err != nil {
			return Observation{}, err
		}
		blockPresent, err := profileBlockPresent(id, block)
		if err != nil {
			return Observation{}, err
		}
		present = present || blockPresent
		want, err := renderProfileBlock(data, id, target.Script, d.JSONFields[id]...)
		if err != nil {
			return Observation{}, err
		}
		blockHealthy, err := profileBlockHealthy(id, block, want)
		if err != nil {
			return Observation{}, err
		}
		healthy = healthy && blockHealthy
		paths, blocks, desired = append(paths, target.Path), append(blocks, block), append(desired, want)
	}
	identity, err := profileIdentity(paths, d.JSONFields[id])
	if err != nil {
		return Observation{}, err
	}
	fingerprint, err := profileBlockFingerprint(id, blocks)
	if err != nil {
		return Observation{}, err
	}
	want, err := digest(desired)
	o := Observation{Present: present, Healthy: healthy, Provider: "profile-block", Identity: identity, Fingerprint: fingerprint, Desired: want, Scope: "user", Adoptable: present && (len(d.JSONFields[id]) > 0 || id == sentinelResource)}
	if id == sentinelResource {
		o.UnverifiedApplications = sentinelDisclosures(d.Targets[id])
	}
	if id == windowsTerminalResource {
		o.Inventory, err = digest(blocks)
	}
	return o, err
}

func (d *ProfileDriver) readJournal(id string, receipt Receipt) (j profileJournal, err error) {
	if receipt.OperationID == "" {
		return j, os.ErrNotExist
	}
	if !operationID.MatchString(receipt.OperationID) || !validProfileRecovery(receipt.Recovery) {
		return j, errors.New("invalid profile operation or recovery identity")
	}
	defer func() {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			err = d.journalError(id, receipt, err)
		}
	}()
	data, err := readDocument(d.journalPath(receipt.OperationID))
	if err != nil {
		return j, err
	}
	j, err = decodeProfileJournal(data, receipt.OperationID)
	if err != nil {
		return j, err
	}
	if j.Resource != id || j.Recovery != receipt.Recovery || !slices.Equal(j.Fields, d.JSONFields[id]) || len(j.Entries) != len(d.Targets[id]) {
		return j, errors.New("profile journal differs from saved intent")
	}
	for i, e := range j.Entries {
		if e.Path != d.Targets[id][i].Path {
			return j, errors.New("profile publication has invalid paths or size")
		}
	}

	return j, nil
}

// Invalid intent cannot authorize recovery moves. Disclose trustworthy paths
// from the receipt and current targets, never paths decoded from damaged JSON.
func (d *ProfileDriver) journalError(id string, receipt Receipt, err error) error {
	paths := []string{d.journalPath(receipt.OperationID)}
	if validProfileRecovery(receipt.Recovery) {
		paths = append(paths, d.baselinePath(receipt.Recovery))
	}
	for i, target := range d.Targets[id] {
		paths = append(paths, profileWorkspace(target.Path, receipt.OperationID, i))
	}
	return fmt.Errorf("cannot use saved profile evidence; preserve and inspect %s: %w", strings.Join(paths, ", "), err)
}

func validProfileRecovery(path string) bool {
	return canonicalRelative(filepath.ToSlash(path)) && filepath.Dir(filepath.Dir(path)) == "recovery"
}

// Historical cleanup uses only saved publication evidence. A removed catalog
// entry or relocated source cannot invalidate an already completed operation.
func decodeProfileJournal(data []byte, operation string) (profileJournal, error) {
	var j profileJournal
	if err := Decode(data, &j); err != nil {
		return j, err
	}
	if err := validateJSONFields(j.Fields); err != nil {
		return j, err
	}
	if j.Schema != 1 || !resourceID.MatchString(j.Resource) || !operationID.MatchString(operation) || j.Operation != operation || !validProfileRecovery(j.Recovery) || len(j.Entries) == 0 || len(j.Entries) > 8 || !slices.Contains([]string{"install", "adopt", "repair", "update", "remove"}, j.Action) || j.Sealed && !j.Complete || j.Cancelled && !j.Complete {
		return j, errors.New("invalid saved profile publication")
	}
	for i, e := range j.Entries {
		if !filepath.IsAbs(e.Path) || filepath.Clean(e.Path) != e.Path || e.Workspace != profileWorkspace(e.Path, j.Operation, i) || len(e.Content) > maxProfileBytes || j.Complete && !j.Cancelled && (!e.Staged || !e.Moved || !e.Published) {
			return j, errors.New("saved profile publication has invalid paths, size or phase")
		}
		for _, previous := range j.Entries[:i] {
			if configPathsOverlap(previous.Path, e.Path) {
				return j, errors.New("saved profile publication has overlapping targets")
			}
		}
		for index, snapshot := range []configSnapshot{e.Before, e.After} {
			if snapshot.Kind != "absent" && snapshot.Kind != "file" || snapshot.Kind == "absent" && snapshot.Hash != "" || snapshot.Kind == "file" && (index == 0 || e.Staged) && !operationID.MatchString(snapshot.Hash) {
				return j, errors.New("invalid saved profile artifact fingerprint")
			}
		}
		content := e.Content
		if e.Staged || j.Complete && j.Cancelled {
			content = e.Block
		}
		_, block, err := replaceProfileBlock(content, j.Resource, nil, j.Fields...)
		if err != nil || !bytes.Equal(block, e.Block) || e.After.Kind == "absent" && len(e.Content) != 0 {
			return j, errors.Join(errors.New("saved profile bytes differ from its owned block"), err)
		}
	}
	if len(j.Restoration) > len(j.Entries) || !j.Restoring && len(j.Restoration) != 0 {
		return j, errors.New("invalid scoped profile restoration")
	}
	seen := map[string]bool{}
	for _, e := range j.Restoration {
		valid := false
		for _, original := range j.Entries {
			valid = valid || e.Path == original.Path && e.Workspace == filepath.Join(original.Workspace, "restore")
		}
		if !valid || seen[e.Path] || len(e.Content) > maxProfileBytes || e.Before.Kind != "file" || !operationID.MatchString(e.Before.Hash) || e.After.Kind != "file" || e.Staged && !operationID.MatchString(e.After.Hash) || j.Complete && (!e.Staged || !e.Moved || !e.Published) {
			return j, errors.New("invalid scoped profile restoration path or phase")
		}
		seen[e.Path] = true
		content := e.Content
		if e.Staged {
			content = e.Block
		}
		_, block, err := replaceProfileBlock(content, j.Resource, nil, j.Fields...)
		if err != nil || !bytes.Equal(block, e.Block) {
			return j, errors.Join(errors.New("invalid scoped profile restoration content"), err)
		}
	}
	return j, nil
}

func profileWorkspace(path, operation string, index int) string {
	return filepath.Join(filepath.Dir(path), fmt.Sprintf(".dotfiles-profile-%s-%d", operation, index))
}

func (d *ProfileDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	o, err := d.inspect(r.ID)
	if err != nil {
		return Observation{Unknown: true, Provider: "profile-block", Scope: "user", Pending: err.Error()}, nil
	}
	j, err := d.readJournal(r.ID, receipt)
	if errors.Is(err, os.ErrNotExist) {
		if receipt.OperationID != "" && receipt.Status != "in-progress" {
			o.Healthy = false
			o.Pending = "profile publication evidence is missing; preserve the profile"
		}
		return o, nil
	}
	if err != nil {
		o.Healthy = false
		o.Pending = err.Error()
		return o, nil
	}
	o.Preserved = sortedUnique(append(slices.Clone(receipt.After.Preserved), j.Preserved...))
	if j.Restoring && (!j.Complete || receipt.Status == "in-progress") {
		o.Restoring = true
		o.Pending = "profile restoration started; finish its saved restoration before applying another operation"
		return o, nil
	}
	if j.Complete {
		if receipt.Ownership == "created" && receipt.After.Provider == o.Provider {
			if _, err := d.readBaseline(r.ID, receipt); err != nil {
				o.Healthy = false
				o.Pending = d.journalError(r.ID, receipt, err).Error()
				return o, nil
			}
			// Previously owned integrations must retain their first baseline
			// before offering replacement of edited blocks.
			o.Adoptable = o.Present
		}
		matches := true
		for _, e := range j.Entries {
			data, _, err := readProfile(e.Path)
			if err != nil {
				return Observation{}, err
			}
			_, block, err := replaceProfileBlock(data, r.ID, nil, d.JSONFields[r.ID]...)
			if err != nil {
				return Observation{}, err
			}
			matches = matches && bytes.Equal(block, e.Block)
		}
		if matches && !j.Cancelled {
			o.CompletedOperation = j.Operation
		}
		return o, nil
	}
	artifacts := []configSnapshot{}
	for _, e := range j.Entries {
		for _, path := range []string{e.Path, filepath.Join(e.Workspace, "previous"), filepath.Join(e.Workspace, "next")} {
			s, err := snapshotTree(path, maxProfileBytes, 1)
			if err != nil {
				return Observation{}, err
			}
			artifacts = append(artifacts, s)
		}
	}
	token, err := digest(struct {
		Journal   profileJournal
		Artifacts []configSnapshot
	}{j, artifacts})
	if err != nil {
		return Observation{}, err
	}
	o.Pending = "profile publication was interrupted; review and resume its saved operation"
	o.ResourceResume = &ResourceResume{Operation: j.Operation, Token: token}
	return o, nil
}

func (d *ProfileDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, op.Observed, receipt)
}
func (d *ProfileDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt.After, receipt)
}

func (d *ProfileDriver) change(ctx context.Context, r Resource, action string, approved Observation, receipt Receipt) (result Observation, resultErr error) {
	if err := d.validate(r.ID); err != nil {
		return Observation{}, err
	}
	if receipt.Status != "in-progress" || !operationID.MatchString(receipt.OperationID) || !validProfileRecovery(receipt.Recovery) {
		return Observation{}, errors.New("profile write requires saved engine intent")
	}
	if _, err := d.readJournal(r.ID, receipt); !errors.Is(err, os.ErrNotExist) {
		return Observation{}, errors.Join(errors.New("profile operation already exists; resume it"), err)
	}
	o, err := d.inspect(r.ID)
	if err != nil || !sameArtifact(o, approved) || action != "remove" && o.Desired != approved.Desired {
		return o, errors.Join(errors.New("profile block changed after approval"), err)
	}
	baseline, err := d.readBaseline(r.ID, receipt)
	fresh := errors.Is(err, os.ErrNotExist) && (!receipt.Before.Present && action == "install" ||
		(len(d.JSONFields[r.ID]) > 0 || r.ID == sentinelResource) && action == "adopt" && receipt.Adopted && receipt.Ownership == "uncertain")
	if fresh {
		baseline = profileBaseline{Schema: 1, Resource: r.ID, Before: receipt.Before, Fields: slices.Clone(d.JSONFields[r.ID])}
	} else if err != nil {
		return o, err
	}
	if action == "adopt" && !fresh && (receipt.Ownership != "created" || receipt.After.Provider != o.Provider) {
		return o, errors.New("profile replacement requires its original owned baseline")
	}
	j := profileJournal{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Recovery: receipt.Recovery, Action: action, Fields: slices.Clone(d.JSONFields[r.ID])}
	for i, target := range d.Targets[r.ID] {
		data, before, err := readProfile(target.Path)
		if err != nil {
			return o, err
		}
		if fresh {
			_, block, err := replaceProfileBlock(data, r.ID, nil, d.JSONFields[r.ID]...)
			if err != nil {
				return o, err
			}
			baseline.Entries = append(baseline.Entries, profileBaselineEntry{target.Path, block, before.Kind == "absent"})
		}
		original := baseline.Entries[i]
		if original.Path != target.Path {
			return o, errors.New("profile baseline target moved")
		}
		if before.Kind == "absent" && d.Windows && len(d.JSONFields[r.ID]) == 0 && r.ID != sentinelResource {
			data = []byte{0xef, 0xbb, 0xbf}
		}
		block := original.Block
		if action != "remove" {
			block, err = renderProfileBlock(data, r.ID, target.Script, d.JSONFields[r.ID]...)
			if err != nil {
				return o, err
			}
		}
		content, _, err := replaceProfileBlock(data, r.ID, block, d.JSONFields[r.ID]...)
		if err != nil {
			return o, err
		}
		if r.ID == windowsTerminalResource {
			_, block, err = replaceProfileBlock(content, r.ID, nil, d.JSONFields[r.ID]...)
			if err != nil {
				return o, err
			}
		}
		if len(content) > maxProfileBytes {
			return o, errors.New("merged profile exceeds its size bound")
		}
		after := configSnapshot{Kind: "file"}
		empty := len(content) == 0 || bytes.Equal(content, []byte{0xef, 0xbb, 0xbf})
		if len(d.JSONFields[r.ID]) > 0 {
			var err error
			empty, err = settingsDocumentEmpty(content, r.ID)
			if err != nil {
				return o, err
			}
		}
		if action == "remove" && original.Absent && empty {
			content = nil
			after.Kind = "absent"
		}
		j.Entries = append(j.Entries, profilePublication{Path: target.Path, Workspace: profileWorkspace(target.Path, j.Operation, i), Before: before, After: after, Content: content, Block: block})
	}
	if fresh {
		if err := saveDocument(d.baselinePath(receipt.Recovery), baseline); err != nil {
			return o, err
		}
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return o, err
	}
	release, err := lockJSONSettings(ctx, d.Directory, receipt.OperationID, d.Targets[r.ID], d.JSONFields[r.ID])
	if err != nil {
		return o, err
	}
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	return d.complete(ctx, r, receipt, &j)
}

func (d *ProfileDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (result Observation, resultErr error) {
	release, err := lockJSONSettings(ctx, d.Directory, receipt.OperationID, d.Targets[r.ID], d.JSONFields[r.ID])
	if err != nil {
		return Observation{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	o, err := d.Observe(ctx, r, receipt)
	if err != nil || o.ResourceResume == nil || op.Observed.ResourceResume == nil || *o.ResourceResume != *op.Observed.ResourceResume {
		return o, errors.Join(errors.New("profile recovery changed after approval"), err)
	}
	j, err := d.readJournal(r.ID, receipt)
	if err != nil {
		return o, err
	}
	if j.Action != op.Action || j.Complete || j.Restoring {
		return o, errors.New("profile recovery action differs")
	}
	return d.complete(ctx, r, receipt, &j)
}
