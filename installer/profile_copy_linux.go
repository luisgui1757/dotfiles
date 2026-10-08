package installer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func copyNativeProfile(ctx context.Context, from, _ string, output *os.File, original unix.Stat_t) (result error) {
	input, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, input.Close()) }()
	var opened, target unix.Stat_t
	if err := unix.Fstat(int(input.Fd()), &opened); err != nil {
		return err
	}
	if opened.Dev != original.Dev || opened.Ino != original.Ino {
		return errors.New("profile changed while opening")
	}
	sourceInfo, err := input.Stat()
	if err != nil {
		return err
	}
	if err := unix.Fstat(int(output.Fd()), &target); err != nil {
		return err
	}
	if original.Uid != target.Uid || original.Gid != target.Gid {
		if err := output.Chown(int(original.Uid), int(original.Gid)); err != nil {
			return err
		}
	}
	if _, err := io.Copy(output, io.LimitReader(input, maxProfileBytes+1)); err != nil {
		return err
	}
	info, err := output.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxProfileBytes {
		return errors.New("profile grew beyond its size bound")
	}
	// Linux stores POSIX ACLs and extended metadata in xattrs. Native calls
	// work on both glibc and musl and avoid a GNU coreutils prerequisite.
	originalAttributes, err := profileAttributes(int(input.Fd()))
	if err != nil {
		return err
	}
	inherited, err := profileAttributes(int(output.Fd()))
	if err != nil {
		return err
	}
	for name := range inherited {
		if _, exists := originalAttributes[name]; !exists {
			if err := unix.Fremovexattr(int(output.Fd()), name); err != nil {
				return err
			}
		}
	}
	for name, value := range originalAttributes {
		if err := unix.Fsetxattr(int(output.Fd()), name, value, 0); err != nil {
			return err
		}
	}
	if err := output.Chmod(sourceInfo.Mode()); err != nil {
		return err
	}
	actual, err := profileAttributes(int(output.Fd()))
	if err != nil {
		return err
	}
	if len(actual) != len(originalAttributes) {
		return errors.New("staged profile attributes differ from the original")
	}
	for name, value := range originalAttributes {
		if actualValue, exists := actual[name]; !exists || !bytes.Equal(actualValue, value) {
			return errors.New("staged profile attribute differs: " + name)
		}
	}
	return ctx.Err()
}

func profileAttributes(fd int) (map[string][]byte, error) {
	result := map[string][]byte{}
	size, err := unix.Flistxattr(fd, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if size > 1<<20 {
		return nil, errors.New("profile attribute list exceeds its bound")
	}
	names := make([]byte, size)
	if size > 0 {
		count, err := unix.Flistxattr(fd, names)
		if err != nil {
			return nil, err
		}
		if count > len(names) {
			return nil, errors.New("profile attribute list changed during inspection")
		}
		names = names[:count]
	}
	for _, name := range strings.Split(string(names), "\x00") {
		if name == "" {
			continue
		}
		size, err := unix.Fgetxattr(fd, name, nil)
		if err != nil {
			return nil, err
		}
		if size > 1<<20 {
			return nil, errors.New("profile attribute exceeds its bound")
		}
		value := make([]byte, size)
		count, err := unix.Fgetxattr(fd, name, value)
		if err != nil {
			return nil, err
		}
		if count > len(value) {
			return nil, errors.New("profile attribute changed during inspection")
		}
		result[name] = value[:count]
	}
	return result, nil
}
