//go:build windows

package installer

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// RemoveDirectory removes only an empty directory. The caller has already
// refused reparse points, which RemoveDirectory would unlink as links.
func removeEmptyDirectory(dir string) (bool, error) {
	remove := func() error {
		path, err := windows.UTF16PtrFromString(dir)
		if err != nil {
			return err
		}
		return windows.RemoveDirectory(path)
	}
	err := remove()
	if err != nil {
		err = retryReadOnlyDirectory(dir, err, remove)
	}
	switch {
	case err == nil, errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND):
		return true, nil
	case errors.Is(err, windows.ERROR_DIR_NOT_EMPTY):
		return false, nil
	}
	return false, err
}

// removeOwned is os.Remove for an entry of the installer's own verified
// scratch, such as an emptied operation workspace.
func removeOwned(path string) error {
	err := os.Remove(path)
	if err == nil {
		return nil
	}
	return retryReadOnlyDirectory(path, err, func() error { return os.Remove(path) })
}

// OneDrive marks the folders it syncs read-only. Windows refuses to delete a
// read-only directory, and os.Remove clears that attribute only for files.
// Clear it on a real directory, retry once, and restore it if the directory
// survives. Links, writable directories and other refusals keep the error.
func retryReadOnlyDirectory(path string, err error, remove func() error) error {
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return err
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0200 != 0 {
		return err
	}
	if chmodErr := os.Chmod(path, info.Mode().Perm()|0200); chmodErr != nil {
		return errors.Join(err, chmodErr)
	}
	retryErr := remove()
	if retryErr == nil {
		return nil
	}
	if restoreErr := os.Chmod(path, info.Mode().Perm()); restoreErr != nil {
		return fmt.Errorf("cannot restore the read-only attribute of %s after a failed removal (%v): %w", path, retryErr, restoreErr)
	}
	return retryErr
}
