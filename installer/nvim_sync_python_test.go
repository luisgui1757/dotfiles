package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestNvimSyncSuppressesPythonBytecodeForEveryPhase(t *testing.T) {
	d, _, receipt, _ := nvimSyncFixture(t)
	t.Setenv("PYTHONDONTWRITEBYTECODE", "")
	for step := 0; step < 3; step++ {
		for _, check := range []bool{false, true} {
			command := d.command(nvimSyncIntent{Operation: receipt.OperationID}, step, check)
			if !slices.Contains(command.Environment, "PYTHONDONTWRITEBYTECODE=1") {
				t.Fatal("Python descendants may write into immutable package sources", step, check)
			}
		}
	}
	if os.Getenv("PYTHONDONTWRITEBYTECODE") != "" {
		t.Fatal("sync changed the caller's Python setting")
	}
}

// Run the real Python entrypoint shipped within the pinned npm/node-gyp payload.
// Its imports are the exact consumer that changed Node's immutable publication.
func nativeNvimGypPreservesNode(t *testing.T, d *NvimSyncDriver, intent nvimSyncIntent, nodePayload, python string) {
	t.Helper()
	moduleRoot := filepath.Join(nodePayload, "lib", "node_modules")
	if runtime.GOOS == "windows" {
		moduleRoot = filepath.Join(nodePayload, "node_modules")
	}
	gyp := filepath.Join(moduleRoot, "npm", "node_modules", "node-gyp", "gyp", "gyp_main.py")
	before, err := snapshotTree(nodePayload, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		command := d.command(intent, 2, check)
		command.Program, command.Arguments = python, []string{gyp, "--help"}
		if output, err := queryNvimSync(t.Context(), command); err != nil {
			t.Fatalf("vendored gyp invocation: %v\n%s", err, output)
		}
		after, err := snapshotTree(nodePayload, maxPackageBytes, maxPackageEntries)
		if err != nil || before != after {
			t.Fatal("vendored gyp changed the immutable Node archive", check, before, after, err)
		}
	}
}

func TestNativeNvimSyncVendoredGypPreservesNode(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NVIM_SYNC") != "1" {
		t.Skip("set DOTFILES_TEST_NVIM_SYNC=1 for checksum-pinned Python and Node consumer proof")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	d, _, receipt, _ := nvimSyncFixture(t)
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tool.node", "tool.python"} {
		if err := downloadArchive(context.Background(), nil, pins[name], filepath.Join(home, name)); err != nil {
			t.Fatal(err)
		}
	}
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	t.Setenv("PYTHONDONTWRITEBYTECODE", "")
	nativeNvimGypPreservesNode(t, d, nvimSyncIntent{Operation: receipt.OperationID}, filepath.Join(home, "tool.node"), filepath.Join(home, "tool.python", filepath.FromSlash(pins["tool.python"].Commands[python])))
}
