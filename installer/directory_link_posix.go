//go:build darwin || linux

package installer

import (
	"os"
	"path/filepath"
)

func createDirectoryLink(target, link string) error { return os.Symlink(target, link) }
func resolveConfigPath(path string) (string, error) { return filepath.EvalSymlinks(path) }
