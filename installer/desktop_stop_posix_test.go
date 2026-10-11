//go:build !windows

package installer

import (
	"context"
	"errors"
	"os"
)

// Preserve the existing POSIX fixture signal behavior.
func desktopStopProcess(_ context.Context, _ string, id int) error {
	process, err := os.FindProcess(id)
	if err == nil {
		err = process.Kill()
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
