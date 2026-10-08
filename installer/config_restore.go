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

type configRecoveryArtifact struct {
	Snapshot configSnapshot `json:"snapshot"`
	Parent   string         `json:"parent"`
	Problem  string         `json:"problem,omitempty"`
}

// The journal contains the original physical targets and baseline slots. Read
// it independently of the current manifest, checkout and baseline document.
func (d *ConfigDriver) readRecoveryJournal(resource string, receipt Receipt) (j configJournal, err error) {
	if !filepath.IsAbs(d.Directory) || !resourceID.MatchString(resource) || !operationID.MatchString(receipt.OperationID) ||
		!filepath.IsLocal(receipt.Recovery) || filepath.Clean(receipt.Recovery) != receipt.Recovery || filepath.Dir(filepath.Dir(receipt.Recovery)) != "recovery" {
		return j, errors.New("invalid saved configuration recovery identity")
	}
	defer func() {
		if err != nil && !errors.Is(err, ErrNoResourceRestore) {
			err = d.journalError(receipt, err)
		}
	}()
	data, err := readDocument(d.journalPath(receipt.OperationID))
	if errors.Is(err, os.ErrNotExist) {
		return j, ErrNoResourceRestore
	}
	if err != nil {
		return j, err
	}
	if err := Decode(data, &j); err != nil {
		return j, err
	}
	if j.Schema != 1 || j.Resource != resource || j.Operation != receipt.OperationID || j.Recovery != receipt.Recovery || len(j.Entries) == 0 ||
		!slices.Contains([]string{"install", "adopt", "update", "repair", "remove"}, j.Action) {
		return j, errors.New("configuration recovery journal differs from saved engine intent")
	}
	destinations := []string{}
	for i, entry := range j.Entries {
		destination := entry.State.Target.Destination
		backup := filepath.Dir(entry.Restore.Backup)
		prefix, suffix := ".dotfiles-config-", fmt.Sprintf("-%d", i)
		name := filepath.Base(backup)
		identity := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if !filepath.IsAbs(destination) || filepath.Clean(destination) != destination || entry.Workspace != configWorkspace(destination, j.Operation, i) ||
			entry.Restore.Destination != destination || filepath.Base(entry.Restore.Backup) != "previous" || filepath.Dir(backup) != filepath.Dir(destination) ||
			name != prefix+identity+suffix || !operationID.MatchString(identity) ||
			!slices.Contains([]string{"", "moving", "moved", "publishing", "published"}, entry.Phase) {
			return j, errors.New("configuration recovery has invalid paths or publication phase")
		}
		for _, previous := range destinations {
			if configPathsOverlap(previous, destination) {
				return j, errors.New("overlapping configuration recovery targets require explicit migration")
			}
		}
		destinations = append(destinations, destination)
	}
	return j, nil
}

func configPathsOverlap(left, right string) bool {
	for _, pair := range [][2]string{{left, right}, {right, left}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && (relative == "." || filepath.IsLocal(relative)) {
			return true
		}
	}
	return false
}

func configRecoveryPaths(entry configPublication) []string {
	return sortedUnique([]string{entry.State.Target.Destination, filepath.Join(entry.Workspace, "previous"), filepath.Join(entry.Workspace, "next"),
		filepath.Join(entry.Workspace, "partial"), filepath.Join(entry.Workspace, "displaced"), entry.Restore.Backup})
}

func inspectConfigRecovery(path string) configRecoveryArtifact {
	result := configRecoveryArtifact{}
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		result.Problem = errors.Join(errors.New("recorded recovery parent is unavailable or redirected"), err).Error()
		return result
	}
	file, err := openConfigParent(path)
	if errors.Is(err, os.ErrNotExist) {
		result.Parent = "absent"
	} else if err != nil {
		result.Problem = err.Error()
		return result
	} else {
		result.Parent, err = configDirectoryIdentity(file)
		err = errors.Join(err, file.Close())
		if err != nil {
			result.Problem = err.Error()
			return result
		}
	}
	result.Snapshot, err = snapshotConfig(path)
	if err != nil {
		result.Problem = err.Error()
	}
	return result
}

func configRecoveryFiles(j configJournal) map[string]configRecoveryArtifact {
	files := map[string]configRecoveryArtifact{}
	for _, entry := range j.Entries {
		for _, path := range configRecoveryPaths(entry) {
			files[path] = inspectConfigRecovery(path)
		}
	}
	return files
}

func (d *ConfigDriver) ObserveRestore(ctx context.Context, resource string, receipt Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	j, err := d.readRecoveryJournal(resource, receipt)
	if err != nil {
		return Observation{}, err
	}
	started := j.Restoring
	for _, entry := range j.Entries {
		started = started || entry.Phase != ""
	}
	if !started || j.Complete && !j.Restoring && !d.allowCompleteRestore {
		return Observation{}, ErrNoResourceRestore
	}
	files := configRecoveryFiles(j)
	token, err := digest(struct {
		Journal configJournal
		Files   map[string]configRecoveryArtifact
	}{j, files})
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "configuration", Scope: "user", Restoring: j.Restoring, ResourceResume: &ResourceResume{Operation: j.Operation, Token: token}, Preserved: slices.Clone(receipt.After.Preserved)}
	if j.Restoring && j.Cancelled && j.Complete && j.Sealed {
		o.RestoredOperation, o.Healthy = j.Operation, true
	}
	actual := []struct {
		Path     string
		Snapshot configSnapshot
	}{}
	paths := []string{}
	for _, entry := range j.Entries {
		target := files[entry.State.Target.Destination]
		paths = append(paths, entry.State.Target.Destination)
		actual = append(actual, struct {
			Path     string
			Snapshot configSnapshot
		}{entry.State.Target.Destination, target.Snapshot})
		o.Present = o.Present || target.Problem == "" && target.Snapshot.Kind != "absent"
		o.Healthy = o.Healthy && target.Problem == "" && target.Snapshot == entry.State.Current
		if receipt.Ownership == "created" {
			baseline := files[entry.Restore.Backup]
			o.Healthy = o.Healthy && baseline.Problem == "" && baseline.Snapshot == entry.Restore.Original
		}
		for _, path := range configRecoveryPaths(entry) {
			artifact := files[path]
			if path != entry.State.Target.Destination && (artifact.Problem != "" || artifact.Snapshot.Kind != "absent") {
				o.Preserved = append(o.Preserved, path)
			}
		}
	}
	o.Identity, err = digest(paths)
	if err != nil {
		return o, err
	}
	o.Fingerprint, err = digest(actual)
	o.Preserved = sortedUnique(append(o.Preserved, j.Preserved...))
	if !o.Healthy {
		o.Pending = "saved configuration recovery retains changed or unavailable files; inspect these targets and the preserved copies before applying: " + strings.Join(paths, ", ")
	}
	return o, err
}

func (d *ConfigDriver) RestoreResource(ctx context.Context, resource string, approved Observation, receipt Receipt) error {
	before, err := d.ObserveRestore(ctx, resource, receipt)
	if err != nil || before.ResourceResume == nil || approved.ResourceResume == nil || *before.ResourceResume != *approved.ResourceResume {
		return errors.Join(errors.New("configuration recovery files changed after approval"), err)
	}
	j, err := d.readRecoveryJournal(resource, receipt)
	if err != nil {
		return err
	}
	if j.Cancelled && j.Complete && j.Sealed {
		return nil
	}
	for path, file := range configRecoveryFiles(j) {
		if file.Problem != "" {
			return fmt.Errorf("cannot inspect recovery path %s: %s; preserve the saved operation and resolve this path before restoring", path, file.Problem)
		}
	}
	j.Restoring = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return err
	}
	for _, entry := range j.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := restoreConfigEntry(j, entry); err != nil {
			return err
		}
	}
	// Restoration deliberately deletes no recovery data. Known unused payloads
	// are small and reported, and unknown/user-edited files can never be mistaken
	// for disposable staging. Normal forward cleanup remains separate.
	j.Preserved = nil
	for path, artifact := range configRecoveryFiles(j) {
		if artifact.Problem != "" || artifact.Snapshot.Kind != "absent" {
			isTarget := false
			for _, entry := range j.Entries {
				isTarget = isTarget || path == entry.State.Target.Destination
			}
			if !isTarget {
				j.Preserved = append(j.Preserved, path)
			}
		}
	}
	j.Preserved = sortedUnique(j.Preserved)
	j.Cancelled, j.Complete, j.Sealed = true, true, true
	return saveDocument(d.journalPath(j.Operation), j)
}

func restoreConfigEntry(j configJournal, entry configPublication) error {
	target, previous := entry.State.Target.Destination, filepath.Join(entry.Workspace, "previous")
	payload, displaced := filepath.Join(entry.Workspace, "next"), filepath.Join(entry.Workspace, "displaced")
	if j.Action == "remove" {
		payload = entry.Restore.Backup
	}
	return restorePublicationFiles(target, previous, payload, displaced, configRecoveryPaths(entry), entry.State.Current, configPublicationResult(j, entry), j.Action == "remove")
}

// Both whole configurations and scoped profiles use these physical inverse
// moves. No step overwrites or deletes a file; later user edits are displaced
// only when an original is waiting to return to its recorded path.
func restorePublicationFiles(target, previous, payload, displaced string, paths []string, original, result configSnapshot, returningBaseline bool) error {
	// Each guard is a physical fact, not an inferred inverse phase. A retry
	// sees the completed move and proceeds to the next rule without overwriting.
	read := func() (map[string]configRecoveryArtifact, error) {
		files := map[string]configRecoveryArtifact{}
		for _, path := range paths {
			files[path] = inspectConfigRecovery(path)
			if files[path].Problem != "" {
				return nil, fmt.Errorf("cannot inspect recovery path %s: %s; restore can be retried after resolving this problem", path, files[path].Problem)
			}
		}
		return files, nil
	}
	files, err := read()
	if err != nil {
		return err
	}
	// An absent backup never authorizes taking the active file out of place.
	// The original may already have returned, or it may be unavailable. Leave
	// the target intact; the recovery observation decides whether it is healthy.
	if original.Kind != "absent" && files[previous].Snapshot.Kind == "absent" {
		return nil
	}
	if result.Kind != "absent" && files[target].Snapshot == result && files[payload].Snapshot.Kind == "absent" {
		if err := moveConfigExclusive(target, payload); err != nil {
			return err
		}
	}
	files, err = read()
	if err != nil {
		return err
	}
	returnedEditedBaseline := returningBaseline && result.Kind != "absent" && files[payload].Snapshot.Kind == "absent"
	if !returnedEditedBaseline && files[target].Snapshot.Kind != "absent" && files[previous].Snapshot.Kind != "absent" && files[displaced].Snapshot.Kind == "absent" {
		if err := moveConfigExclusive(target, displaced); err != nil {
			return err
		}
	}
	files, err = read()
	if err != nil {
		return err
	}
	if files[target].Snapshot.Kind == "absent" && files[previous].Snapshot.Kind != "absent" {
		return moveConfigExclusive(previous, target)
	}
	return nil
}
