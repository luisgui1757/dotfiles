package installer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

var pythonToolVersion = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)+$`)

// pip compares PIP_CONFIG_FILE with Python's os.devnull using exact strings.
// Python spells the Windows device "nul", whereas Go spells it "NUL".
func pipConfigNullDevice() string { return strings.ToLower(os.DevNull) }

type privatePythonRequirement struct {
	Name, File, Hash string
	Source           bool
}

// Keep historical console layouts readable. New pins expose only the declared
// console, never a private environment's Python, pip or activation commands.
func privatePythonConsolePaths(pin ArchivePin, name string) (string, string, error) {
	target := pin.Commands[name]
	source := ""
	switch target {
	case "bin/" + name, "commands/" + name:
		source = "bin/" + name
	case "Scripts/" + name + ".exe", "commands/" + name + ".exe":
		source = "Scripts/" + name + ".exe"
	default:
		return "", "", fmt.Errorf("%s requires its fixed private console command", name)
	}
	if len(pin.Commands) != 1 || !slices.Equal(pin.BinDirs, []string{path.Dir(target)}) {
		return "", "", fmt.Errorf("%s has an unexpected command layout", name)
	}
	return source, target, nil
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
	environment := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + home, "LOCALAPPDATA=" + home, "TMPDIR=" + temporary, "TEMP=" + temporary, "TMP=" + temporary, "PIP_CONFIG_FILE=" + pipConfigNullDevice(), "PYTHONPATH=", "PYTHONHOME=", "PYTHONDONTWRITEBYTECODE=1"}
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
	base, target := d.Python, payload
	if runtime.GOOS == "windows" {
		base, target = windowsExtendedPath(base), windowsExtendedPath(target)
	}
	if err := run(base, "-I", "-B", "-m", "venv", target); err != nil {
		return err
	}
	python := filepath.Join(payload, "bin", "python")
	if runtime.GOOS == "windows" {
		python = windowsExtendedPath(filepath.Join(payload, "Scripts", "python.exe"))
	}
	for _, requirement := range requirements {
		// These are compiled names and already checksum-verified local files.
		if !resourceID.MatchString(requirement.Name) || !filepath.IsAbs(requirement.File) || !operationID.MatchString(requirement.Hash) {
			return errors.New("invalid fixed private Python requirement")
		}
		path := requirement.File
		if runtime.GOOS == "windows" {
			path = windowsExtendedPath(path)
		}
		path = filepath.ToSlash(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		local := &url.URL{Scheme: "file", Path: path}
		if runtime.GOOS == "windows" {
			// Pip normalizes literal drive colons and drops the device prefix.
			// Escaping the colon preserves the exact extended local path.
			local.RawPath = strings.ReplaceAll(local.EscapedPath(), ":", "%3A")
		}
		requirementFile := filepath.Join(payload, "."+requirement.Name+"-requirements.txt")
		if err := os.WriteFile(requirementFile, []byte(fmt.Sprintf("%s @ %s --hash=sha256:%s\n", requirement.Name, local.String(), requirement.Hash)), 0600); err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			requirementFile = windowsExtendedPath(requirementFile)
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
	source, console, err := privatePythonConsolePaths(intent.Pin, name)
	if err != nil {
		return err
	}
	if source != console {
		if err := os.Mkdir(filepath.Join(payload, "commands"), 0700); err != nil {
			return err
		}
		// Pip's generated console embeds the absolute private interpreter. Copy
		// its bytes exclusively; moving the console does not move that runtime.
		target := filepath.Join(payload, filepath.FromSlash(console))
		if err := copyConfigPayload(filepath.Join(payload, filepath.FromSlash(source)), target); err != nil {
			return err
		}
		if err := os.Chmod(target, 0755); err != nil {
			return err
		}
	}
	if err := run(python, pythonCompileArguments(payload, payload)...); err != nil {
		return err
	}
	return run(python, append([]string{"-I", "-B", "-c", probe}, probeArguments...)...)
}
