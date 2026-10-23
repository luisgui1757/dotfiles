//go:build !darwin

package installer

import (
	"context"
	"errors"
	"io"
	"os"
)

func readDMGArchive(context.Context, *os.File, string, int, func(io.Reader) error) error {
	return errors.New("disk-image application extraction requires macOS")
}
