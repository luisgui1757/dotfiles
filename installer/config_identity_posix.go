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
