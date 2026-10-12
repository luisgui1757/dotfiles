package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

func cleanupPreserved(path string, preserved []string) bool {
	for _, kept := range preserved {
		relative, err := filepath.Rel(kept, path)
		if err == nil && (relative == "." || filepath.IsLocal(relative)) {
			return true
		}
	}
	return false
}

// Deletion authority is a bounded, durable list of entries. Missing entries
// are normal after our own partial cleanup; changed and new entries survive.
// No recursive removal is used, including after a crash or a sharing violation.
func discardRecordedTree(path string, entries []configEntry, byteLimit int64, entryLimit int, kept ...string) ([]string, error) {
	if err := validateCleanupEntries(entries, entryLimit); err != nil {
		return nil, err
	}
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return []string{path}, nil
	}
	actual, err := inspectTree(path, byteLimit, entryLimit)
	if err != nil {
		return nil, err
	}
	wanted := map[string]configEntry{}
	for _, entry := range entries {
		wanted[entry.Path] = entry
	}
	preserved := slices.Clone(kept)
	for _, entry := range actual {
		if want, ok := wanted[entry.Path]; !ok || want != entry {
			preserved = append(preserved, filepath.Join(path, entry.Path))
		}
	}
	for _, entry := range slices.Backward(actual) {
		name := filepath.Join(path, entry.Path)
		if cleanupPreserved(name, preserved) {
			continue
		}
		bound, err := bindConfigDestination(name)
		if err != nil || bound != name {
			preserved = append(preserved, name)
			continue
		}
		// Recheck each leaf at the removal boundary, after inspecting siblings.
		// Directories are removed only when empty, so new children survive.
		current, err := inspectTree(name, byteLimit, entryLimit)
		if err != nil {
			return sortedUnique(preserved), err
		}
		if len(current) == 0 {
			continue
		}
		want := entry
		want.Path = "."
		if current[0] != want {
			preserved = append(preserved, name)
			continue
		}
		if len(current) != 1 {
			// These children survived the planned leaf removals or appeared
			// afterwards. Preserve them individually; never widen a retry's
			// retention to unchanged siblings that were temporarily locked.
			for _, child := range current[1:] {
				preserved = append(preserved, filepath.Join(name, child.Path))
			}
			continue
		}
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return sortedUnique(preserved), fmt.Errorf("cleanup remains at %s; retry after releasing it: %w", name, err)
		}
	}
	return sortedUnique(preserved), nil
}

func validateCleanupEntries(entries []configEntry, limit int) error {
	if len(entries) == 0 || len(entries) > limit || entries[0].Path != "." {
		return errors.New("invalid cleanup entry list")
	}
	seen := map[string]configEntry{}
	for i, entry := range entries {
		if !filepath.IsLocal(entry.Path) || filepath.Clean(entry.Path) != entry.Path || entry.Mode > 0777 {
			return errors.New("cleanup entry has an invalid path or mode")
		}
		if _, exists := seen[entry.Path]; exists {
			return errors.New("duplicate cleanup entry")
		}
		if i > 0 && seen[filepath.Dir(entry.Path)].Kind != "directory" {
			return errors.New("cleanup entry parent is not a recorded directory")
		}
		switch entry.Kind {
		case "file":
			if !operationID.MatchString(entry.Content) {
				return errors.New("invalid cleanup content hash")
			}
		case "directory":
			if entry.Content != "" {
				return errors.New("invalid cleanup directory")
			}
		case "link", "junction":
			if entry.Content == "" || len(entry.Content) > 32768 || entry.Mode != 0 {
				return errors.New("invalid cleanup link")
			}
		default:
			return errors.New("invalid cleanup entry kind")
		}
		seen[entry.Path] = entry
	}
	return nil
}
