package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (d *NvimSyncDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := d.validate(r, receipt); err != nil {
		return Observation{}, err
	}
	if receipt.Ownership != "created" || !operationID.MatchString(receipt.OperationID) {
		return Observation{}, errors.New("Neovim removal requires created ownership")
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		current, err := archiveLinkTarget(d.current())
		if err != nil || !d.ownsPath(current) {
			return Observation{}, errors.Join(errors.New("Neovim removal cannot prove current runtime"), err)
		}
		fingerprint, err := digest(current)
		if err != nil || fingerprint != receipt.After.Fingerprint {
			return Observation{}, errors.Join(errors.New("Neovim runtime changed after approval"), err)
		}
		previous, err := d.intent(filepath.Base(filepath.Dir(current)))
		if err != nil || !previous.Complete {
			return Observation{}, errors.Join(errors.New("Neovim runtime lacks completed ownership"), err)
		}
		intent = nvimSyncIntent{Schema: 1, Operation: receipt.OperationID, Action: "remove", Desired: previous.Desired, PreviousTarget: current, Preserved: slices.Clone(receipt.After.Preserved)}
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if intent.Action != "remove" {
		return Observation{}, errors.New("Neovim removal differs from saved intent")
	}
	return d.remove(ctx, r, receipt, intent)
}

func (d *NvimSyncDriver) remove(ctx context.Context, r Resource, receipt Receipt, intent nvimSyncIntent) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	publication := archiveIntent{Resource: "nvim.sync", Operation: intent.Operation, PreviousTarget: intent.PreviousTarget}
	if err := d.links().switchLink(publication, true); err != nil {
		return Observation{}, err
	}
	intent.Complete = true
	if err := d.save(intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// Reuse the existing bounded per-entry deletion primitive. The original phase
// manifest remains durable across partial deletion; it never acquires new files.
func (d *NvimSyncDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	report := map[string][]string{}
	receipt, ok := receipts["nvim.sync"]
	if !ok || receipt.Ownership != "created" && receipt.Status != "removed" {
		return report, nil
	}
	current, err := archiveLinkTarget(d.current())
	if err != nil {
		return report, err
	}
	files, err := readPlainDirectory(filepath.Join(d.Directory, "operations"))
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	report["nvim.sync"] = slices.Clone(receipt.After.Preserved)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		operation := strings.TrimSuffix(file.Name(), ".json")
		if !operationID.MatchString(operation) {
			continue
		}
		intent, err := d.intent(operation)
		if err != nil {
			report["nvim.sync"] = append(report["nvim.sync"], d.intentPath(operation), d.payload(operation))
			continue
		}
		report["nvim.sync"] = append(report["nvim.sync"], intent.Preserved...)
		if !intent.Complete {
			continue
		}
		for _, kind := range []string{"previous", "next"} {
			publication := archiveIntent{Resource: "nvim.sync", Operation: operation}
			path := d.links().linkWorkspace(publication, kind)
			actual, err := archiveLinkTarget(path)
			want := intent.PreviousTarget
			if kind == "next" {
				want = d.payload(operation)
			}
			if err != nil || actual != "" && actual != want {
				report["nvim.sync"] = append(report["nvim.sync"], path)
				continue
			}
			if actual != "" {
				if err := os.Remove(path); err != nil {
					return report, err
				}
			}
		}
		if intent.Action == "remove" || d.payload(operation) == current || len(intent.Manifest) == 0 {
			continue
		}
		entries, err := d.readManifest(intent)
		if err != nil {
			return report, err
		}
		kept, cleanupErr := discardRecordedTree(d.payload(operation), entries, maxPackageBytes, maxNvimEntries, intent.Preserved...)
		intent.Preserved = sortedUnique(append(intent.Preserved, kept...))
		report["nvim.sync"] = append(report["nvim.sync"], intent.Preserved...)
		if err := errors.Join(cleanupErr, d.save(intent)); err != nil {
			return report, err
		}
		// Empty generation directories carry no personal state. Nonempty ones
		// contain preserved additions and are left in place without recursion.
		remaining, err := os.ReadDir(filepath.Dir(d.payload(operation)))
		if err == nil && len(remaining) == 0 {
			err = os.Remove(filepath.Dir(d.payload(operation)))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return report, err
		}
	}
	report["nvim.sync"] = sortedUnique(report["nvim.sync"])
	return report, nil
}
