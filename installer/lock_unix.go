//go:build !windows

package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func Lock(directory string) (func() error, error) {
	if err := prepareStateDirectory(directory); err != nil {
		return nil, err
	}
	return lockFile(directory, true)
}

// Previews use the existing engine lock without creating state directories or
// lock files. Holding it prevents them from interrupting a worker handoff.
func lockFile(directory string, create bool) (func() error, error) {
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if create {
		flags |= unix.O_CREAT
	}
	fd, err := unix.Open(filepath.Join(directory, "mutation.lock"), flags, 0600)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, errors.Join(err, unix.Close(fd))
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 || int(stat.Uid) != os.Getuid() || stat.Mode&0077 != 0 {
		return nil, errors.Join(errors.New("mutation lock must be a private regular file owned by this user"), unix.Close(fd))
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("another installer holds the mutation lock: %w", err), unix.Close(fd))
	}
	return func() error { return unix.Close(fd) }, nil
}

func publishState(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(to))
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
