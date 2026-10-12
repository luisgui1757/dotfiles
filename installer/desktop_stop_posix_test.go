//go:build !windows

package installer

import (
	"context"
	"errors"
	"os"
)

// Preserve the existing POSIX fixture signal behavior.
func desktopStopProcess(_ context.Context, _ string, id int) (result error) {
	process, err := os.FindProcess(id)
	if err == nil {
		defer func() { result = errors.Join(result, process.Release()) }()
		err = process.Kill()
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
