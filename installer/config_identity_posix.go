//go:build darwin || linux

package installer

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func configDirectoryIdentity(file *os.File) (string, error) {
	var info unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", info.Dev, info.Ino), nil
}

func fileOwner(path string) (uint32, uint32, uint32, error) {
	var info unix.Stat_t
	if err := unix.Lstat(path, &info); err != nil {
		return 0, 0, 0, &os.PathError{Op: "lstat", Path: path, Err: err}
	}
	return info.Uid, info.Gid, uint32(info.Mode) & 07777, nil
}
