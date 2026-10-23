package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// A target's missing ancestors are bounded; deeper creation is refused.
const maxCreatedDirectories = 16

// missingDirectories returns the folders above path that do not exist yet,
// nearest first, up to its first existing ancestor. A publication creates
// exactly these, so the first baseline records them as originally absent.
func missingDirectories(path string) ([]string, error) {
	missing := []string{}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		_, err := os.Lstat(dir)
		if err == nil {
			return missing, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if filepath.Dir(dir) == dir || len(missing) == maxCreatedDirectories {
			return nil, fmt.Errorf("target %s has no existing ancestor within %d folders", path, maxCreatedDirectories)
		}
		missing = append(missing, dir)
	}
}

// Recorded folders must be the target's exact ancestor chain, nearest first.
func validCreatedDirectories(path string, created []string) bool {
	if len(created) > maxCreatedDirectories {
		return false
	}
	dir := path
	for _, recorded := range created {
		if parent := filepath.Dir(dir); parent == dir || recorded != parent {
			return false
		}
		dir = recorded
	}
	return true
}

// removeCreatedDirectories restores an originally absent target's missing
// folders after its removal: nearest first, each only while it is still an
// empty real directory under unredirected parents. A kept folder keeps every
// ancestor; links, junctions and anything a person added stay. The cleanup is
// cosmetic, so an unexpected failure never blocks finalization: the returned
// folder is disclosed as kept instead.
func removeCreatedDirectories(created []string) (kept string) {
	for _, dir := range created {
		if bound, err := bindConfigDestination(dir); err != nil {
			return dir
		} else if bound != dir {
			return "" // A person relocated an ancestor; the tree is theirs now.
		}
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return dir
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return ""
		}
		removed, err := removeEmptyDirectory(dir)
		if err != nil {
			return dir
		}
		if !removed {
			return ""
		}
	}
	return ""
}
