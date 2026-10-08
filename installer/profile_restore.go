package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (d *ProfileDriver) readRecoveryJournal(id string, receipt Receipt) (j profileJournal, err error) {
	if !filepath.IsAbs(d.Directory) || !resourceID.MatchString(id) || !operationID.MatchString(receipt.OperationID) || !validProfileRecovery(receipt.Recovery) {
		return j, errors.New("invalid saved profile recovery identity")
	}
	defer func() {
		if err != nil && !errors.Is(err, ErrNoResourceRestore) {
			err = d.journalError(id, receipt, err)
		}
	}()
	data, err := readDocument(d.journalPath(receipt.OperationID))
	if errors.Is(err, os.ErrNotExist) {
		return j, ErrNoResourceRestore
	}
	if err != nil {
		return j, err
	}
	j, err = decodeProfileJournal(data, receipt.OperationID)
	if err != nil {
		return j, err
	}
	if j.Resource != id || j.Recovery != receipt.Recovery {
		return j, errors.New("profile recovery differs from saved engine intent")
	}
	return j, nil
}

func profileRecoveryPaths(e profilePublication) []string {
	return []string{e.Path, filepath.Join(e.Workspace, "previous"), filepath.Join(e.Workspace, "next"), filepath.Join(e.Workspace, "displaced")}
}

func profileRecoveryFiles(j profileJournal) map[string]configRecoveryArtifact {
	files := map[string]configRecoveryArtifact{}
	for _, e := range append(append([]profilePublication{}, j.Entries...), j.Restoration...) {
		for _, path := range profileRecoveryPaths(e) {
			files[path] = inspectConfigRecovery(path)
		}
	}
	return files
}

func (d *ProfileDriver) ObserveRestore(ctx context.Context, id string, receipt Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	j, err := d.readRecoveryJournal(id, receipt)
	if err != nil {
		return Observation{}, err
	}
	if id == windowsTerminalResource {
		saved := *d
		saved.Targets = map[string][]ProfileTarget{id: {}}
		for _, entry := range j.Entries {
			saved.Targets[id] = append(saved.Targets[id], ProfileTarget{Path: entry.Path})
		}
		saved.JSONFields = map[string][]string{id: j.Fields}
		if _, err := saved.readBaseline(id, receipt); err != nil {
			return Observation{}, err
		}
	}
	started := j.Restoring
	for _, e := range j.Entries {
		started = started || e.Staged || e.Moved || e.Published
	}
	if !started || j.Complete && !j.Restoring {
		return Observation{}, ErrNoResourceRestore
	}
	files := profileRecoveryFiles(j)
	token, err := digest(struct {
		Journal profileJournal
		Files   map[string]configRecoveryArtifact
	}{j, files})
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "profile-block", Scope: "user", Restoring: j.Restoring, ResourceResume: &ResourceResume{Operation: j.Operation, Token: token}}
	if j.Restoring && j.Cancelled && j.Complete && j.Sealed {
		o.RestoredOperation, o.Healthy = j.Operation, true
	}
	paths, blocks := []string{}, [][]byte{}
	problems := []string{}
	for _, e := range j.Entries {
		target := files[e.Path]
		o.Healthy = o.Healthy && target.Problem == ""
		paths = append(paths, e.Path)
		data, _, readErr := readProfile(e.Path)
		var block []byte
		if readErr == nil {
			_, block, readErr = replaceProfileBlock(data, id, nil, j.Fields...)
		}
		if readErr != nil {
			o.Healthy = false
			problems = append(problems, fmt.Sprintf("%s: %v", e.Path, readErr))
		}
		recovery := profileRecoveryPaths(e)
		if e.Before.Kind != "absent" && files[recovery[1]].Snapshot.Kind == "absent" && target.Snapshot.Kind == "absent" {
			// An empty managed-block fingerprint does not prove restoration
			// when the personal profile and its saved original are both gone.
			o.Healthy = false
		}
		if e.Before.Kind != "absent" && (e.Moved || e.Published) && files[recovery[1]].Snapshot.Kind == "absent" && files[recovery[2]].Snapshot != e.After {
			problems = append(problems, fmt.Sprintf("%s: saved original is unavailable at %s; inspect the active profile before continuing", e.Path, recovery[1]))
		}
		blocks = append(blocks, block)
		blockPresent, blockErr := profileBlockPresent(id, block)
		if blockErr != nil {
			return Observation{}, blockErr
		}
		o.Present = o.Present || blockPresent
		for _, path := range profileRecoveryPaths(e)[1:] {
			if files[path].Problem != "" || files[path].Snapshot.Kind != "absent" {
				o.Preserved = append(o.Preserved, path)
			}
		}
	}
	o.Identity, err = profileIdentity(paths, j.Fields)
	if err != nil {
		return o, err
	}
	o.Fingerprint, err = profileBlockFingerprint(id, blocks)
	prior := receipt.After
	if prior.Provider == "" {
		prior = receipt.Before
	}
	o.Healthy = o.Healthy && sameArtifact(o, prior)
	for _, e := range j.Restoration {
		for _, path := range profileRecoveryPaths(e)[1:] {
			if files[path].Problem != "" || files[path].Snapshot.Kind != "absent" {
				o.Preserved = append(o.Preserved, path)
			}
		}
	}
	o.Preserved = sortedUnique(append(append(o.Preserved, receipt.After.Preserved...), j.Preserved...))
	if !o.Healthy {
		o.Pending = "profile recovery retains changed or unavailable files; inspect these profiles and the preserved copies: " + strings.Join(paths, ", ")
		if len(problems) != 0 {
			o.Pending += "\n" + strings.Join(problems, "\n")
		}
	}
	return o, err
}

func (d *ProfileDriver) RestoreResource(ctx context.Context, id string, approved Observation, receipt Receipt) (resultErr error) {
	actual, err := d.ObserveRestore(ctx, id, receipt)
	if err != nil || actual.ResourceResume == nil || approved.ResourceResume == nil || *actual.ResourceResume != *approved.ResourceResume {
		return errors.Join(errors.New("profile recovery files changed after approval"), err)
	}
	j, err := d.readRecoveryJournal(id, receipt)
	if err != nil {
		return err
	}
	if j.Cancelled && j.Complete && j.Sealed {
		return nil
	}
	targets := make([]ProfileTarget, len(j.Entries))
	for i, entry := range j.Entries {
		targets[i].Path = entry.Path
	}
	release, err := lockJSONSettings(ctx, d.Directory, j.Operation, targets, j.Fields)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	// A failed inspection has not chosen a recovery direction. Keep the
	// original retry/abandon choices until restoration can actually start.
	for path, file := range profileRecoveryFiles(j) {
		if file.Problem != "" {
			return fmt.Errorf("cannot inspect recovery path %s: %s; preserve the saved operation and resolve this path before restoring", path, file.Problem)
		}
	}
	j.Restoring = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return err
	}
	for _, e := range j.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := d.restoreProfileEntry(ctx, &j, e); err != nil {
			return err
		}
	}
	// No recovery data is deleted. Keep every unconsumed original, staged file
	// and conflict, including unknown bytes, and disclose their physical paths.
	files := profileRecoveryFiles(j)
	for _, e := range append(append([]profilePublication{}, j.Entries...), j.Restoration...) {
		for _, path := range profileRecoveryPaths(e)[1:] {
			if files[path].Problem != "" || files[path].Snapshot.Kind != "absent" {
				j.Preserved = append(j.Preserved, path)
			}
		}
	}
	j.Preserved = sortedUnique(j.Preserved)
	for i := range j.Entries {
		j.Entries[i].Content = nil
	}
	j.Complete, j.Sealed, j.Cancelled = true, true, true
	return saveDocument(d.journalPath(j.Operation), j)
}
