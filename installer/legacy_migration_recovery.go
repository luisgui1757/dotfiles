package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Published upgrades already have their own authenticated frozen recovery
// source. A new major must not supersede an unfinished transaction or run it.
func (d *legacyMigrationDriver) checkReleasedRecovery() error {
	directory := filepath.Join(filepath.Dir(d.Directory), "migrations")
	entries, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 1024 {
		return errors.New("released migration inventory exceeds its bound; preserve and inspect it")
	}
	for _, entry := range entries {
		recognized := false
		for _, tag := range []string{"v0.2.0", "v0.3.0", "v0.4.0", "v0.4.1", "v0.4.2", "v0.4.3", "v0.4.4"} {
			recognized = recognized || strings.HasPrefix(entry.Name(), "v0.1.0-to-"+tag+".")
		}
		if !recognized {
			continue
		}
		root := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.Join(fmt.Errorf("unsafe released recovery directory: %s", root), err)
		}
		values := map[string]string{}
		for _, name := range []string{"stage", "old-checkout", "new-checkout"} {
			data, _, err := readProfile(filepath.Join(root, name))
			if err != nil {
				return fmt.Errorf("preserve released recovery %s: %w", root, err)
			}
			value := strings.TrimSuffix(string(data), "\n")
			value = strings.TrimSuffix(value, "\r")
			if len(data) == 0 || data[len(data)-1] != '\n' || value == "" || strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("invalid released recovery identity at %s", root)
			}
			values[name] = value
		}
		if !slices.Contains([]string{"accepted", "rolled-back"}, values["stage"]) {
			return fmt.Errorf("released migration is unfinished or unrecognized (%s); use its retained recovery instructions before the new migration: %s", values["stage"], root)
		}
	}
	return nil
}
