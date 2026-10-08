package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The system decoder receives only the already verified open archive. It emits
// tar bytes to the existing bounded extractor; it never installs a package or
// runs its maintainer scripts. dpkg-deb processes --fsys-tarfile sequentially and
// supports "-" for stdin: https://manpages.debian.org/trixie/dpkg/dpkg-deb.1.en.html
func readDebianArchive(ctx context.Context, file *os.File, decoder string, readTar func(io.Reader) error) error {
	if !filepath.IsAbs(decoder) || filepath.Clean(decoder) != decoder || strings.ContainsAny(decoder, "\x00\r\n") {
		return errors.New("Debian archive requires the observed absolute dpkg-deb decoder")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, decoder, "--fsys-tarfile", "-")
	command.Stdin = file
	command.Dir = filepath.Dir(file.Name())
	command.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "HOME=" + command.Dir, "TMPDIR=" + command.Dir}
	command.WaitDelay = 2 * time.Second
	var diagnostic boundedCommandOutput
	command.Stderr = &diagnostic
	stream, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return errors.Join(err, stream.Close())
	}
	readErr := readTar(stream)
	if readErr != nil {
		cancel()
	}
	closeErr := stream.Close()
	// Always reap the decoder, including malformed tar and cancellation paths.
	waitErr := command.Wait()
	if waitErr != nil {
		waitErr = fmt.Errorf("decode verified Debian archive: %w%s", waitErr, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return errors.Join(readErr, closeErr, waitErr)
}
