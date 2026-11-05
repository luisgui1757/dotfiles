package installer

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Mount only the checksum-verified private input, read one declared application
// through the bounded tar extractor, and detach the exact private mount point.
func readDMGArchive(ctx context.Context, file *os.File, bundle string, readTar func(io.Reader) error) (result error) {
	mount, err := os.MkdirTemp(filepath.Dir(file.Name()), ".dmg-mount-")
	if err != nil {
		return err
	}
	attach, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(attach, "/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-noautoopen", "-mountpoint", mount, file.Name())
	var output boundedCommandOutput
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		// An interrupted attach may have mounted the image. Preserve its mount
		// directory instead of recursively removing an uncertain native mount.
		return errors.Join(err, errors.New("disk-image attachment incomplete; inspect "+mount+nativeDiagnostic(output.data.Bytes())))
	}
	defer func() {
		detach, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(detach, "/usr/bin/hdiutil", "detach", mount)
		var output boundedCommandOutput
		command.Stdout, command.Stderr = &output, &output
		if err := command.Run(); err != nil {
			result = errors.Join(result, err, errors.New("cannot detach private image at "+mount+nativeDiagnostic(output.data.Bytes())))
			return
		}
		result = errors.Join(result, os.Remove(mount))
	}()
	info, err := os.Lstat(filepath.Join(mount, bundle))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errors.New("disk image lacks its declared physical application bundle"), err)
	}
	// tar never extracts here. It streams the mounted bundle into the same
	// checked reader used by ZIP/DEB, without executing application code.
	// Apple tar otherwise creates extra ._ files from filesystem metadata.
	// They are not vendor files and invalidate signed resource envelopes.
	return readNativeTar(ctx, file, "/usr/bin/tar", []string{"-cf", "-", "--no-mac-metadata", "-C", mount, bundle}, readTar)
}
