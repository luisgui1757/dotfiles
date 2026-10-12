//go:build !windows

package installer

import (
	"errors"
	"os"
	"syscall"
)

// rmdir(2) removes only an empty directory and refuses a link in its place.
func removeEmptyDirectory(dir string) (bool, error) {
	err := syscall.Rmdir(dir)
	switch {
	case err == nil, errors.Is(err, syscall.ENOENT):
		return true, nil
	case errors.Is(err, syscall.ENOTEMPTY), errors.Is(err, syscall.EEXIST), errors.Is(err, syscall.ENOTDIR):
		return false, nil
	}
	return false, err
}

// removeOwned is os.Remove for an entry of the installer's own verified
// scratch. POSIX directory permissions do not prevent their own removal.
func removeOwned(path string) error {
	return os.Remove(path)
}
