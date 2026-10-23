package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type boundedCommandOutput struct {
	data  bytes.Buffer
	limit int
}

func (output *boundedCommandOutput) Write(data []byte) (int, error) {
	limit := output.limit
	if limit == 0 {
		limit = 1 << 20
	}
	remaining := limit - output.data.Len()
	if len(data) > remaining {
		n, err := output.data.Write(data[:remaining])
		return n, errors.Join(fmt.Errorf("native command output exceeds %d bytes", limit), err)
	}
	return output.data.Write(data)
}

// Python's ordinary imports populate __pycache__. Prepare every supported
// optimization level before taking the package fingerprint, so using the runtime
// does not look like a personal edit. Checked hashes do not depend on extraction
// timestamps. Source and bytecode edits remain subject to the full archive check.
func preparePythonArchive(ctx context.Context, pin ArchivePin, payload string) error {
	if pin.PythonStdlib == "" {
		return nil
	}
	if err := pin.Validate(); err != nil {
		return err
	}
	command := pin.Commands["python3"]
	if command == "" {
		command = pin.Commands["python"]
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// POSIX writes relative to the pinned cwd even after recovery renames it.
	// Windows locks that cwd against renaming while the child is running, so
	// its extended absolute writes cannot reach a replacement stage either.
	args := pythonCompileArguments(filepath.Join(payload, pin.PythonStdlib), pin.PythonStdlib)
	cmd := exec.CommandContext(ctx, filepath.Join(payload, filepath.FromSlash(command)), args...)
	cmd.Dir = payload
	cmd.WaitDelay = 2 * time.Second
	var output boundedCommandOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("prepare verified Python bytecode: %w\n%s", err, output.data.String())
	}
	return nil
}

// Extended Windows paths avoid MAX_PATH for compileall's temporary pyc files.
// Keep the previous spelling in code objects, so prepared bytecode is unchanged.
func pythonCompileArguments(physical, recorded string) []string {
	args := []string{"-I", "-B", "-m", "compileall", "-q", "-f", "-j", "1", "--invalidation-mode", "checked-hash", "-o", "0", "-o", "1", "-o", "2"}
	if runtime.GOOS == "windows" {
		return append(args, "-d", recorded, windowsExtendedPath(physical))
	}
	return append(args, recorded)
}
