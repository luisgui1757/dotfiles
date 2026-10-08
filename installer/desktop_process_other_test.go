//go:build !darwin

package installer

import (
	"context"
	"errors"
)

func desktopDarwinOwnedProcesses(context.Context, string) ([]int, error) {
	return nil, errors.New("macOS desktop inventory is unavailable on this platform")
}
