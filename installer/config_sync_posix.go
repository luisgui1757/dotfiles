//go:build darwin || linux

package installer

import (
	"errors"
	"os"
)

func syncConfigDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(syncConfigHandle(file), file.Close())
}

func syncConfigHandle(file *os.File) error { return file.Sync() }
