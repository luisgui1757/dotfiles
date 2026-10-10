package installer

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
)

// An edited profile is still the user's active file. Undo only the managed
// block, through the same durable publication steps as ordinary profile writes.
// Unmodified or missing files can use the simpler physical inverse moves.
func (d *ProfileDriver) restoreProfileEntry(ctx context.Context, j *profileJournal, e profilePublication) error {
	for i := range j.Restoration {
		if j.Restoration[i].Path == e.Path {
			again, err := d.resumeProfileRestoration(ctx, j, i)
			if err != nil || !again {
				return err
			}
			break
		}
	}
	paths := profileRecoveryPaths(e)
	files := map[string]configRecoveryArtifact{}
	for _, path := range paths {
		files[path] = inspectConfigRecovery(path)
		if files[path].Problem != "" {
			return fmt.Errorf("cannot inspect recovery path %s: %s; restore can be retried after resolving this problem", path, files[path].Problem)
		}
	}
	current := files[e.Path].Snapshot
	// A staged payload still waiting to publish cannot have been edited at
	// the target. Return the original and retain the independently created file.
	if current.Kind != "file" || current == e.Before || current == e.After || !e.Published && files[paths[2]].Snapshot.Kind != "absent" {
		return restorePublicationFiles(paths[0], paths[1], paths[2], paths[3], paths, e.Before, e.After, false)
	}
	data, actual, err := readProfile(e.Path)
	if err != nil {
		fresh := inspectConfigRecovery(e.Path)
		if fresh.Problem != "" || fresh.Snapshot != current {
			return errors.Join(errors.New("profile cannot be inspected consistently; preserve the saved operation and retry"), err)
		}
		j.Preserved = append(j.Preserved, e.Path)
		return nil
	}
	if actual != current {
		return errors.New("profile changed while preparing scoped restoration; retry with a fresh preview")
	}
	var original, beforeBlock []byte
	if e.Before.Kind != "absent" {
		var snapshot configSnapshot
		original, snapshot, err = readProfile(paths[1])
		if err != nil || snapshot != e.Before {
			// The original can still be active or have already returned. A
			// saved published payload distinguishes a physical return from a
			// missing backup; disclose the active file in the latter case.
			if files[paths[1]].Snapshot.Kind == "absent" {
				if files[paths[2]].Snapshot != e.After {
					j.Preserved = append(j.Preserved, e.Path)
				}
				return nil
			}
			j.Preserved = append(j.Preserved, e.Path, paths[1])
			return nil
		}
		_, beforeBlock, err = replaceProfileBlock(original, j.Resource, nil, j.Fields...)
		if err != nil {
			j.Preserved = append(j.Preserved, e.Path, paths[1])
			return nil
		}
	}
	content, block, err := replaceProfileBlock(data, j.Resource, beforeBlock, j.Fields...)
	if err != nil {
		j.Preserved = append(j.Preserved, e.Path)
		return nil
	}
	if len(block) != 0 && !bytes.Equal(block, e.Block) && !bytes.Equal(block, beforeBlock) {
		// A changed owned block is user data too. Preserve it and the saved
		// original; the final observation explains the unresolved ownership.
		j.Preserved = append(j.Preserved, e.Path)
		return nil
	}
	if len(j.Fields) == 0 && len(block) == 0 && len(beforeBlock) != 0 {
		start, _, err := profileBlock(original, j.Resource)
		if err != nil {
			return err
		}
		// Reinsert a removed block at its former boundary only when the
		// preceding personal bytes still identify that boundary. Otherwise
		// preserve both files and report the unresolved block, without guessing
		// an execution order for the user's shell commands.
		if start < 0 || !bytes.HasPrefix(data, original[:start]) {
			j.Preserved = append(j.Preserved, e.Path)
			return nil
		}
		content = append(bytes.Clone(data[:start]), beforeBlock...)
		content = append(content, data[start:]...)
	}
	if bytes.Equal(content, data) {
		return nil
	}
	if len(content) > maxProfileBytes {
		j.Preserved = append(j.Preserved, e.Path)
		return nil
	}
	if _, _, err := replaceProfileBlock(content, j.Resource, nil, j.Fields...); err != nil {
		return err
	}
	j.Restoration = append(j.Restoration, profilePublication{Path: e.Path, Workspace: filepath.Join(e.Workspace, "restore"), Before: current, After: configSnapshot{Kind: "file"}, Content: content, Block: beforeBlock})
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return err
	}
	return d.publishProfile(ctx, j, &j.Restoration[len(j.Restoration)-1])
}

// Rebase an inverse only while its original remains active. After its move,
// preserve a recreated target before publishing the saved inverse. A subsequent
// edit of a published inverse is personal data, not permission to overwrite it.
func (d *ProfileDriver) resumeProfileRestoration(ctx context.Context, j *profileJournal, index int) (bool, error) {
	e := &j.Restoration[index]
	if e.Published {
		return false, nil
	}
	files := profileRecoveryFiles(profileJournal{Entries: []profilePublication{*e}})
	paths := profileRecoveryPaths(*e)
	for path, file := range files {
		if file.Problem != "" {
			return false, fmt.Errorf("cannot inspect recovery path %s: %s", path, file.Problem)
		}
	}
	current, previous, next := files[paths[0]].Snapshot, files[paths[1]].Snapshot, files[paths[2]].Snapshot
	if previous.Kind != "absent" && next.Kind != "absent" && current.Kind != "absent" {
		displaced := paths[3]
		if files[displaced].Snapshot.Kind != "absent" {
			displaced = filepath.Join(e.Workspace, "displaced-"+rand.Text())
		}
		j.Preserved = sortedUnique(append(j.Preserved, displaced))
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return false, err
		}
		if err := verifyConfigSnapshot(e.Path, current); err != nil {
			return false, err
		}
		if err := moveConfigExclusive(e.Path, displaced); err != nil {
			return false, err
		}
		return false, d.publishProfile(ctx, j, e)
	}
	if previous.Kind != "absent" && next.Kind == "absent" && current.Kind == "absent" {
		if err := verifyConfigSnapshot(paths[1], e.Before); err != nil {
			return false, err
		}
		if err := moveConfigExclusive(paths[1], e.Path); err != nil {
			return false, err
		}
		previous.Kind = "absent"
	}
	if previous.Kind == "absent" && (current != e.Before || e.Staged && next.Kind == "absent") {
		j.Preserved = append(j.Preserved, e.Workspace)
		j.Restoration = slices.Delete(j.Restoration, index, index+1)
		return true, nil
	}
	if previous.Kind != "absent" && next.Kind == "absent" && current.Kind != "absent" && current != e.After {
		j.Preserved = append(j.Preserved, e.Workspace)
		j.Restoration = slices.Delete(j.Restoration, index, index+1)
		return false, nil
	}
	return false, d.publishProfile(ctx, j, e)
}
