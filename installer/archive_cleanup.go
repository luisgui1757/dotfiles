package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type archiveCleanup struct {
	Schema     int            `json:"schema"`
	Generation string         `json:"generation"`
	Version    archiveVersion `json:"version"`
	Entries    []configEntry  `json:"entries"`
	Preserved  []string       `json:"preserved,omitempty"`
}

func (d *ArchiveDriver) cleanupPath(id, generation string) string {
	return filepath.Join(d.resourceDirectory(id), "cleanup", generation+".json")
}

// A cleanup document keeps the publication proof after version.json itself is
// deleted. The same entry list resumes partial deletion without treating a
// locked file as personal data or granting authority over later user edits.
func (d *ArchiveDriver) cleanGeneration(id, generation string) ([]string, error) {
	path := d.versionDirectory(id, generation)
	if !operationID.MatchString(generation) {
		return []string{path}, nil
	}
	journal := d.cleanupPath(id, generation)
	data, err := readDocument(journal)
	var cleanup archiveCleanup
	if errors.Is(err, os.ErrNotExist) {
		version, err := d.version(id, generation)
		if err != nil {
			return []string{path}, nil
		}
		entries, err := inspectTree(path, maxPackageBytes, maxPackageEntries)
		if err != nil {
			return nil, err
		}
		cleanup = archiveCleanup{Schema: 1, Generation: generation, Version: version, Entries: entries}
		if err := d.validateCleanup(id, cleanup); err != nil {
			return []string{path}, nil
		}
		if err := saveDocument(journal, cleanup); err != nil {
			return nil, err
		}
	} else {
		if err == nil {
			err = Decode(data, &cleanup)
		}
		if err == nil {
			err = d.validateCleanup(id, cleanup)
		}
		if err != nil || cleanup.Generation != generation {
			return []string{path, journal}, nil
		}
	}
	kept, cleanupErr := discardRecordedTree(path, cleanup.Entries, maxPackageBytes, maxPackageEntries, cleanup.Preserved...)
	cleanup.Preserved = sortedUnique(kept)
	return kept, errors.Join(cleanupErr, saveDocument(journal, cleanup))
}

func (d *ArchiveDriver) validateCleanup(id string, cleanup archiveCleanup) error {
	v := cleanup.Version
	if cleanup.Schema != 1 || !operationID.MatchString(cleanup.Generation) || v.Schema != archiveSchema || v.Resource != id || v.Operation != cleanup.Generation {
		return errors.New("invalid obsolete package cleanup identity")
	}
	intent, err := d.readIntent(id, v.Operation)
	if err != nil || intent.Generation != cleanup.Generation || intent.Action == "remove" || archivePinID(intent.Pin) != archivePinID(v.Pin) {
		return errors.Join(errors.New("obsolete package lacks publication intent"), err)
	}
	if err := validateCleanupEntries(cleanup.Entries, maxPackageEntries); err != nil {
		return err
	}
	payload := []configEntry{}
	manifest := false
	for _, entry := range cleanup.Entries {
		switch {
		case entry.Path == ".":
			if entry.Kind != "directory" {
				return errors.New("package generation is not a directory")
			}
		case entry.Path == "version.json":
			if entry.Kind != "file" {
				return errors.New("package manifest is not a file")
			}
			manifest = true
		case entry.Path == "payload":
			entry.Path = "."
			payload = append(payload, entry)
		case strings.HasPrefix(entry.Path, "payload"+string(filepath.Separator)):
			entry.Path = strings.TrimPrefix(entry.Path, "payload"+string(filepath.Separator))
			payload = append(payload, entry)
		default:
			return errors.New("package generation contains unowned entries")
		}
	}
	snapshot, err := snapshotEntries(payload)
	if err != nil || !manifest || snapshot != v.Payload {
		return errors.Join(errors.New("obsolete package differs from its published payload"), err)
	}
	return nil
}
