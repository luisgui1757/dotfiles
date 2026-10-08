package installer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (d *ProfileDriver) complete(ctx context.Context, r Resource, receipt Receipt, j *profileJournal) (Observation, error) {
	for i := range j.Entries {
		if err := d.publishProfile(ctx, j, &j.Entries[i]); err != nil {
			return Observation{}, err
		}
	}
	j.Complete = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// Forward installation and scoped restoration use the same saved publication
// steps. Each phase is durable in the parent journal before the next move.
func (d *ProfileDriver) publishProfile(ctx context.Context, j *profileJournal, e *profilePublication) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !e.Staged {
		if err := verifyConfigSnapshot(e.Path, e.Before); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(e.Path), 0700); err != nil {
			return err
		}
		if err := os.Mkdir(e.Workspace, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		bound, err := bindConfigDestination(filepath.Join(e.Workspace, "next"))
		if err != nil || bound != filepath.Join(e.Workspace, "next") {
			return errors.Join(errors.New("profile staging directory redirected"), err)
		}
		if e.After.Kind != "absent" {
			next := filepath.Join(e.Workspace, "next")
			_, err := os.Lstat(next)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil {
				// Without durable staging proof, matching bytes do not prove
				// that a copy/write restored its original metadata before death.
				preserved := filepath.Join(e.Workspace, "unfinished-"+rand.Text())
				j.Preserved = sortedUnique(append(j.Preserved, preserved))
				if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
					return err
				}
				if err := moveConfigExclusive(next, preserved); err != nil {
					return err
				}
			}
			if e.Before.Kind == "file" {
				if err := copyProfileMetadata(ctx, e.Path, next); err != nil {
					return fmt.Errorf("cannot stage profile %s at %s: %w; original profile remains active; resolve the reported file access or ownership/permissions problem, then retry the saved operation", e.Path, next, err)
				}
			} else {
				file, err := os.OpenFile(next, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				if err := file.Close(); err != nil {
					return err
				}
			}
			if err := writeStagedProfile(next, e.Content); err != nil {
				return err
			}
			e.After, err = snapshotTree(next, maxProfileBytes, 1)
			if err != nil {
				return err
			}
		}
		e.Staged = true
		// The durable staged file is now the recovery payload. Retaining
		// another full profile copy in historical JSON can retain secrets.
		e.Content = nil
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return err
		}
	}
	previous := filepath.Join(e.Workspace, "previous")
	if !e.Moved {
		if e.Before.Kind != "absent" {
			saved, err := snapshotTree(previous, maxProfileBytes, 1)
			if err != nil {
				return err
			}
			if saved.Kind == "absent" {
				if err := verifyConfigSnapshot(e.Path, e.Before); err != nil {
					return err
				}
				if err := moveConfigExclusive(e.Path, previous); err != nil {
					return err
				}
			}
			if err := verifyConfigSnapshot(previous, e.Before); err != nil {
				return err
			}
		}
		e.Moved = true
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return err
		}
	}
	if !e.Published {
		current, err := snapshotTree(e.Path, maxProfileBytes, 1)
		if err != nil {
			return err
		}
		if e.After.Kind != "absent" && current.Kind == "absent" {
			next := filepath.Join(e.Workspace, "next")
			if err := verifyConfigSnapshot(next, e.After); err != nil {
				return err
			}
			if err := moveConfigExclusive(next, e.Path); err != nil {
				return err
			}
		} else if e.After.Kind != "absent" {
			// A matching file which appeared while our staged file still
			// exists is not evidence that this operation published it.
			next, err := snapshotTree(filepath.Join(e.Workspace, "next"), maxProfileBytes, 1)
			if err != nil || next.Kind != "absent" {
				return errors.Join(errors.New("another profile appeared before publication; preserve both files"), err)
			}
		}
		if err := verifyConfigSnapshot(e.Path, e.After); err != nil {
			return err
		}
		e.Published = true
		if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
			return err
		}
	}
	return nil
}

// Only the reserved staged copy becomes writable. The published profile keeps
// its original mode, including the Windows read-only attribute. A crash before
// mode restoration leaves unproved staging bytes for ordinary recovery.
func writeStagedProfile(path string, content []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("staged profile is not a regular file")
	}
	mode := info.Mode().Perm()
	readOnly := mode&0200 == 0
	if readOnly {
		if err := os.Chmod(path, mode|0200); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return errors.Join(err, os.Chmod(path, mode))
	}
	_, writeErr := file.Write(content)
	var modeErr error
	if readOnly {
		modeErr = file.Chmod(mode)
	}
	return errors.Join(writeErr, modeErr, file.Sync(), file.Close())
}

func (d *ProfileDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	report := map[string][]string{}
	directory := filepath.Join(d.Directory, "profiles", "operations")
	if err := requireCompletedJournals(plan, receipts, "profile-block", directory); err != nil {
		return nil, err
	}
	files, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := file.Name()
		if len(name) != 69 || filepath.Ext(name) != ".json" || !operationID.MatchString(name[:64]) {
			continue
		}
		path := filepath.Join(d.Directory, "profiles", "operations", name)
		data, err := readDocument(path)
		var j profileJournal
		if err == nil {
			err = Decode(data, &j)
		}
		if err == nil && (j.Schema != 1 || !resourceID.MatchString(j.Resource) || j.Operation != name[:64] || j.Sealed && !j.Complete) {
			err = errors.New("invalid profile journal identity or completion")
		}
		if err == nil && (!j.Sealed || len(j.Fields) > 0) {
			j, err = decodeProfileJournal(data, name[:64])
		}
		if err != nil {
			if journalRequired(plan, receipts, name[:64]) {
				for id, receipt := range receipts {
					if receipt.OperationID == name[:64] {
						return nil, d.journalError(id, receipt, err)
					}
				}
				return nil, errors.Join(errors.New("required profile journal is invalid at "+path), err)
			}
			report[""] = append(report[""], path)
			continue
		}
		if !j.Complete {
			if j.Restoring {
				return nil, errors.New("profile restoration is unfinished; retry restoration before abandoning")
			}
			if plan.Mode != "abandon" {
				return nil, errors.New("profile publication is unfinished; resume its saved operation")
			}
			for _, e := range j.Entries {
				previous, err := snapshotTree(filepath.Join(e.Workspace, "previous"), maxProfileBytes, 1)
				if err != nil || previous.Kind != "absent" || e.Moved || e.Published {
					return nil, errors.Join(errors.New("profile publication already started; resume it before abandoning"), err)
				}
			}
			j.Complete, j.Cancelled = true, true
			for i := range j.Entries {
				j.Entries[i].Content = nil
			}
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				return nil, err
			}
		}
		if !j.Sealed {
			for _, e := range j.Entries {
				for _, artifact := range []struct {
					Name     string
					Snapshot configSnapshot
				}{{"previous", e.Before}, {"next", e.After}} {
					path := filepath.Join(e.Workspace, artifact.Name)
					current, err := snapshotTree(path, maxProfileBytes, 1)
					if errors.Is(err, os.ErrNotExist) || err == nil && current.Kind == "absent" {
						continue
					}
					if err != nil || current != artifact.Snapshot || artifact.Name == "previous" && j.Action == "adopt" && !j.Cancelled {
						j.Preserved = append(j.Preserved, path)
						continue
					}
					if err := os.Remove(path); err != nil {
						return nil, err
					}
				}
				contents, err := readPlainDirectory(e.Workspace)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					return nil, err
				}
				if len(contents) == 0 {
					if err := os.Remove(e.Workspace); err != nil {
						return nil, err
					}
				} else {
					j.Preserved = append(j.Preserved, e.Workspace)
				}
			}
			j.Preserved = sortedUnique(j.Preserved)
			j.Sealed = true
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				return nil, err
			}
		}
		if err := finishJSONSettingsLocks(d.Directory, j); err != nil {
			return nil, err
		}
		report[j.Resource] = sortedUnique(append(report[j.Resource], j.Preserved...))
	}
	return report, nil
}
