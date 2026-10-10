package installer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var pythonToolVersion = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)+$`)

type privatePythonRequirement struct {
	Name, File, Hash string
	Source           bool
}

// Only the fixed LaTeX and yamllint recipes call this helper. Requirements and
// probes are compiled recipe decisions, never commands supplied by pin data.
// ArchiveDriver owns staging, interruption recovery, publication and removal.
func (d *ArchiveDriver) preparePrivatePython(ctx context.Context, intent archiveIntent, payload, name string, requirements []privatePythonRequirement, probe string, probeArguments ...string) error {
	if !filepath.IsAbs(d.Python) || filepath.Clean(d.Python) != d.Python || strings.ContainsAny(d.Python, "\x00\r\n") || d.Run == nil {
		return fmt.Errorf("%s requires the managed Python command and native session", name)
	}
	home := filepath.Join(payload, ".build-home")
	// Pip owns and cleans its unique temporary build directories. Nesting them
	// below a versioned payload can exceed Windows' child-process cwd limit.
	// Keep the private home and installed environment inside the generation.
	temporary := os.TempDir()
	if err := os.Mkdir(home, 0700); err != nil {
		return err
	}
	environment := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + home, "LOCALAPPDATA=" + home, "TMPDIR=" + temporary, "TEMP=" + temporary, "TMP=" + temporary, "PIP_CONFIG_FILE=" + os.DevNull, "PYTHONPATH=", "PYTHONHOME=", "PYTHONDONTWRITEBYTECODE=1"}
	// An interrupted archive stage is preserved and a fresh one is prepared.
	// Distinct attempt IDs prevent replaying a saved phase into a new directory.
	attempt, phase := rand.Text(), 0
	run := func(program string, args ...string) error {
		operation, err := digest([]string{intent.Operation, attempt, fmt.Sprint(phase)})
		if err != nil {
			return err
		}
		phase++
		output, err := d.Run(ctx, nativeCommand{Operation: operation, Program: program, Arguments: args, Environment: environment})
		if err != nil {
			return fmt.Errorf("%s preparation phase %d: %s", name, phase, nativeErrorDetail(err, output))
		}
		return nil
	}
	if err := run(d.Python, "-I", "-B", "-m", "venv", payload); err != nil {
		return err
	}
	python := filepath.Join(payload, "bin", "python")
	if runtime.GOOS == "windows" {
		python = filepath.Join(payload, "Scripts", "python.exe")
	}
	for _, requirement := range requirements {
		// These are compiled names and already checksum-verified local files.
		if !resourceID.MatchString(requirement.Name) || !filepath.IsAbs(requirement.File) || !operationID.MatchString(requirement.Hash) {
			return errors.New("invalid fixed private Python requirement")
		}
		path := filepath.ToSlash(requirement.File)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		local := (&url.URL{Scheme: "file", Path: path}).String()
		requirementFile := filepath.Join(payload, "."+requirement.Name+"-requirements.txt")
		if err := os.WriteFile(requirementFile, []byte(fmt.Sprintf("%s @ %s --hash=sha256:%s\n", requirement.Name, local, requirement.Hash)), 0600); err != nil {
			return err
		}
		args := []string{"-I", "-B", "-m", "pip", "--isolated", "install", "--disable-pip-version-check", "--no-cache-dir", "--no-index", "--require-hashes", "--no-deps"}
		if requirement.Source {
			args = append(args, "--no-binary="+requirement.Name, "--no-build-isolation")
		} else {
			args = append(args, "--only-binary=:all:")
		}
		args = append(args, "-r", requirementFile)
		if err := run(python, args...); err != nil {
			return err
		}
	}
	if name == "latex2text" {
		if err := run(python, "-I", "-B", "-c", latex2textConsole); err != nil {
			return err
		}
	}
	if err := run(python, "-I", "-B", "-m", "compileall", "-q", "-f", "-j", "1", "--invalidation-mode", "checked-hash", "-o", "0", "-o", "1", "-o", "2", payload); err != nil {
		return err
	}
	return run(python, append([]string{"-I", "-B", "-c", probe}, probeArguments...)...)
}
