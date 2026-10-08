//go:build darwin || linux

package installer

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Reserve the destination exclusively before any native metadata copy. The
// original stays active until a complete staged copy has passed verification.
func copyProfileMetadata(ctx context.Context, from, to string) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	var original unix.Stat_t
	if err := unix.Lstat(from, &original); err != nil {
		return err
	}
	if original.Mode&unix.S_IFMT != unix.S_IFREG {
		return errors.New("profile source is not a regular file")
	}
	output, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	created, statErr := output.Stat()
	if statErr != nil {
		return errors.Join(statErr, output.Close())
	}
	defer func() {
		result = errors.Join(result, output.Close())
		if result != nil {
			result = errors.Join(result, removeFailedProfileCopy(to, created))
		}
	}()
	if err := copyNativeProfile(ctx, from, to, output, original); err != nil {
		return err
	}
	var copied unix.Stat_t
	if err := unix.Stat(to, &copied); err != nil {
		return err
	}
	if original.Uid != copied.Uid || original.Gid != copied.Gid {
		return fmt.Errorf("cannot preserve profile owner/group for %s; original profile is unchanged", from)
	}
	current, err := os.Lstat(to)
	if err != nil || !os.SameFile(created, current) {
		return errors.Join(errors.New("profile staging identity changed"), err)
	}
	return errors.Join(ctx.Err(), output.Sync())
}
