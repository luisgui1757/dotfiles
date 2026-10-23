package installer

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InspectLegacyRootStatus is the private, read-only subprocess boundary for
// roots hidden behind another account's directory permissions. Only ENOENT
// proves absence; dangling links are present and all other failures propagate.
// No configuration, state, package manager or profile is loaded.
func InspectLegacyRootStatus(path string, output io.Writer) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || strings.ContainsRune(path, 0) {
		return errors.New("legacy root inspection requires a bounded absolute path")
	}
	_, err := os.Lstat(path)
	status := "present\n"
	if errors.Is(err, os.ErrNotExist) {
		status = "absent\n"
	} else if err != nil {
		return err
	}
	_, err = io.WriteString(output, status)
	return err
}
