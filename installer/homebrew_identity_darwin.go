package installer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// st_dev changes across macOS boots. Bind retained ownership to the filesystem
// UUID and directory object instead, using the already opened no-follow handle.
// https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2
func homebrewDirectoryIdentity(file *os.File) (identity string, resultErr error) {
	var info unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &info); err != nil {
		return "", err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", errors.New("Homebrew infrastructure identity requires a directory")
	}
	// The documented volume-attribute contract requires the volume root. Bind
	// that second handle to the prefix's mounted filesystem before querying it.
	var filesystem, mounted unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &filesystem); err != nil {
		return "", err
	}
	fd, err := unix.Open(unix.ByteSliceToString(filesystem.Mntonname[:]), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW_ANY|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, unix.Close(fd)) }()
	if err := unix.Fstatfs(fd, &mounted); err != nil {
		return "", err
	}
	if filesystem.Fsid != mounted.Fsid || filesystem.Mntonname != mounted.Mntonname {
		return "", errors.New("Homebrew filesystem mount changed during identity inspection")
	}
	attributes := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_RETURNED_ATTRS, Volattr: unix.ATTR_VOL_INFO | unix.ATTR_VOL_UUID}
	var result [40]byte // length, returned attribute_set_t, then the volume UUID.
	_, _, errno := unix.Syscall6(unix.SYS_FGETATTRLIST, uintptr(fd), uintptr(unsafe.Pointer(&attributes)),
		uintptr(unsafe.Pointer(&result[0])), uintptr(len(result)), 0, 0)
	runtime.KeepAlive(file)
	if errno != 0 {
		return "", fmt.Errorf("read Homebrew filesystem UUID: %w", errno)
	}
	var empty [16]byte
	if binary.NativeEndian.Uint32(result[:4]) != uint32(len(result)) || binary.NativeEndian.Uint32(result[8:12])&unix.ATTR_VOL_UUID == 0 || [16]byte(result[24:]) == empty {
		return "", errors.New("Homebrew filesystem did not return a complete nonzero UUID")
	}
	return fmt.Sprintf("volume:%x:inode:%d", result[24:], info.Ino), nil
}
