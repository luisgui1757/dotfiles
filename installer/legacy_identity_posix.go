//go:build darwin || linux

package installer

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// legacyPathIdentity names the inode at path without following a final link.
// A rename on one filesystem preserves it, so a saved move can prove itself.
func legacyPathIdentity(path string) (string, error) {
	var info unix.Stat_t
	if err := unix.Lstat(path, &info); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return "absent", nil
		}
		return "", &os.PathError{Op: "lstat", Path: path, Err: err}
	}
	return fmt.Sprintf("%o:%d:%d:%d", uint32(info.Mode)&unix.S_IFMT, uint64(info.Dev), uint64(info.Ino), info.Uid), nil
}

func legacyFileOwner(path string) (uint32, uint32, uint32, error) {
	var info unix.Stat_t
	if err := unix.Lstat(path, &info); err != nil {
		return 0, 0, 0, &os.PathError{Op: "lstat", Path: path, Err: err}
	}
	return info.Uid, info.Gid, uint32(info.Mode) & 07777, nil
}
