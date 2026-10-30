package installer

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWindowsPrivatePythonUsesIsolatedPipEnvironment(t *testing.T) {
	d, commands := yamllintPreparationFixture(t)
	r := Resource{ID: "tool.shared"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64)}
	before, err := d.Observe(t.Context(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(t.Context(), r, Operation{Action: "install", Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	if len(*commands) != 6 {
		t.Fatal("preparation did not execute all phases", len(*commands))
	}
	for _, command := range *commands {
		if !slices.Contains(command.Environment, "PIP_CONFIG_FILE=nul") {
			t.Fatal("preparation inherited external pip configuration", command.Environment)
		}
	}
}

func TestWindowsPrivatePythonRequirementURLsRetainExtendedPaths(t *testing.T) {
	d, commands := yamllintPreparationFixture(t)
	d.Directory = filepath.Join(d.Directory, "space % # λ")
	r := Resource{ID: "tool.shared"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64)}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	for _, command := range (*commands)[1:4] {
		data, err := os.ReadFile(command.Arguments[len(command.Arguments)-1])
		if err != nil {
			t.Fatal(err)
		}
		uri := strings.Fields(string(data))[2]
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "//?/") ||
			!strings.HasPrefix(uri, "file:////%3F/") || !strings.Contains(uri, "%3A/") || !strings.Contains(uri, "space%20%25%20%23%20%CE%BB") {
			t.Fatal("local URI lost the device path or filename escaping", uri, err)
		}
		if _, err := os.Stat(filepath.FromSlash(parsed.Path)); err != nil {
			t.Fatal("local URI does not identify the verified wheel", uri, err)
		}
	}
}

// Use the actual LocalAppData prefix and the production generation layout.
// Testing only t.TempDir homes on a provisioned runner misses default Windows'
// path limit, particularly the temporary suffix added to each bytecode file.
func TestNativeArchiveWindowsPythonDefaultPaths(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("enable verified native Python downloads on a disposable Windows host")
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(localAppData) {
		t.Fatal("LOCALAPPDATA must identify the actual Windows account")
	}
	root, err := os.MkdirTemp(localAppData, "dotfiles-python-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	pin := pins["tool.python"]
	payload := filepath.Join(root, "packages", "tool.python", "versions", strings.Repeat("a", 64), "payload")
	if err := os.MkdirAll(filepath.Dir(payload), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := downloadArchive(ctx, nil, pin, payload); err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(payload, filepath.FromSlash(pin.Commands["python"]))

	t.Run("working directory prevents recovery rename", func(t *testing.T) {
		// The interpreter is outside this stage: failure must be caused by its
		// working directory, not by an executable image inside the directory.
		stage := filepath.Join(root, "interrupted-stage")
		working := filepath.Join(stage, "payload")
		if err := os.MkdirAll(working, 0700); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(root, "child-ready")
		child := exec.CommandContext(ctx, python, "-I", "-B", "-c", "import pathlib,sys; pathlib.Path(sys.argv[1]).write_text('ready'); sys.stdin.read(1)", marker)
		child.Dir = working
		input, err := child.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		stopped := false
		stop := func() {
			if stopped {
				return
			}
			stopped = true
			if err := input.Close(); err != nil {
				t.Error(err)
			}
			if err := child.Wait(); err != nil {
				t.Error(err)
			}
		}
		defer stop()
		for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(marker); err == nil {
				break
			} else if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
				t.Fatal("Python did not enter its working directory", err)
			}
		}
		backup := stage + ".interrupted"
		if err := moveConfigExclusive(stage, backup); err == nil {
			t.Fatal("an active Python cwd allowed recovery to reuse its absolute pathname")
		}
		if _, err := os.Stat(working); err != nil {
			t.Fatal("failed recovery move changed the active stage", err)
		}
		if _, err := os.Lstat(backup); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed recovery move published a backup", err)
		}
		// Closing stdin ends this exact child. Reap it before the successful
		// retry; no process-name-wide termination or registry changes are used.
		stop()
		if err := moveConfigExclusive(stage, backup); err != nil {
			t.Fatal("recovery move remained blocked after child exit", err)
		}
	})

	t.Run("checked bytecode and runtime", func(t *testing.T) {
		if err := preparePythonArchive(ctx, pin, payload); err != nil {
			t.Fatal(err)
		}
		// The historical relative recipe still works at this control path. Its
		// complete bytecode output must match, not just one co_filename probe.
		reference := filepath.Join(root, "reference")
		if err := downloadArchive(ctx, nil, pin, reference); err != nil {
			t.Fatal(err)
		}
		old := exec.CommandContext(ctx, filepath.Join(reference, filepath.FromSlash(pin.Commands["python"])), "-I", "-B", "-m", "compileall", "-q", "-f", "-j", "1", "--invalidation-mode", "checked-hash", "-o", "0", "-o", "1", "-o", "2", pin.PythonStdlib)
		old.Dir = reference
		if output, err := old.CombinedOutput(); err != nil {
			t.Fatalf("historical bytecode control: %s %v", output, err)
		}
		compared := 0
		if err := filepath.WalkDir(reference, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".pyc") {
				return nil
			}
			relative, err := filepath.Rel(reference, path)
			if err != nil {
				return err
			}
			previous, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			current, err := os.ReadFile(filepath.Join(payload, relative))
			if err != nil {
				return err
			}
			if !bytes.Equal(previous, current) {
				t.Errorf("base bytecode identity changed: %s", relative)
			}
			compared++
			return nil
		}); err != nil || compared < 1000 {
			t.Fatal("incomplete historical bytecode comparison", compared, err)
		}
		t.Log("byte-identical historical caches:", compared)
		before, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
		if err != nil {
			t.Fatal(err)
		}
		probe := `import ctypes, importlib.util, marshal, os, pathlib, struct, sys
policy = ctypes.WinDLL('ntdll').RtlAreLongPathsEnabled
policy.restype = ctypes.c_ubyte
enabled = bool(policy())
if sys.argv[2] == '1':
	assert not enabled, 'default-path acceptance requires long paths disabled'
source = os.path.join('Lib', 'site-packages', 'pip', '_internal', 'resolution', 'resolvelib', 'found_candidates.py')
for level in ('', '1', '2'):
	cache = importlib.util.cache_from_source(os.path.join(sys.argv[1], source), optimization=level)
	data = pathlib.Path('\\\\?\\' + cache).read_bytes()
	assert struct.unpack('<I', data[4:8])[0] == 3, 'bytecode must use checked hashes'
	assert marshal.loads(data[16:]).co_filename == source, 'recorded filename changed'
print('long paths enabled:', enabled)
print('resolved Python prefix:', sys.prefix)
`
		command := exec.CommandContext(ctx, python, "-I", "-B", "-c", probe, payload, os.Getenv("DOTFILES_TEST_PYTHON_DEFAULT_WINDOWS"))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("native bytecode inspection: %s %v", output, err)
		} else {
			t.Log(string(output))
		}
		for _, optimization := range []string{"-O", "-OO", "-B"} {
			command := exec.CommandContext(ctx, python, "-I", optimization, "-c", "import ssl, sqlite3, venv, ensurepip; import pip._internal.resolution.resolvelib.found_candidates")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("native runtime use: %s %v", output, err)
			}
		}
		after, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
		if err != nil || after != before {
			t.Fatal("ordinary Python use changed the prepared runtime", err)
		}
	})
}
