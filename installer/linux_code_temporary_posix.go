//go:build darwin || linux

package installer

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func readLinuxCodeDatabase(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	// Debconf can atomically replace the caller-created file while running as
	// root. Mode 0644 remains private beneath the caller-owned 0700 directory,
	// while letting that caller verify a root-rewritten file after completion.
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0777 != 0644 || stat.Uid != 0 && int(stat.Uid) != os.Geteuid() || stat.Size != int64(len(linuxCodeDebconf)) {
		return nil, errors.Join(errors.New("Code database has changed owner, mode or size"), file.Close())
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(len(linuxCodeDebconf))+1))
	return data, errors.Join(err, file.Close())
}

func linuxCodeTemporaryIdentity(path string) (string, error) {
	bound, err := bindConfigDestination(path + "/answer.dat")
	if err != nil || bound != path+"/answer.dat" {
		return "", errors.Join(errors.New("Code temporary directory was redirected"), err)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return "", err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0777 != 0700 || int(stat.Uid) != os.Geteuid() {
		return "", errors.New("Code temporary directory has an unexpected owner or permissions")
	}
	return configDirectoryIdentity(file)
}
