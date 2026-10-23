//go:build !linux

package installer

import (
	"context"
	"errors"
	"io"
)

func InspectLegacyLinuxProcesses(context.Context, string, string, io.Writer) error {
	return errors.New("Linux process inspection is unavailable on this platform")
}
