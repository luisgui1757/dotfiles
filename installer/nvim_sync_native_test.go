package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeNvimSyncCheckNeverBootstrapsOrWritesPersonalState(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NVIM_SYNC") != "1" {
		t.Skip("set DOTFILES_TEST_NVIM_SYNC=1 for checksum-pinned Neovim read-only verification")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	c, archives, _ := archiveController(t)
	archives.Pins["tool.shared"], archives.Client = pins["tool.nvim"], nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	binary, err := archives.CommandPath("tool.shared", "nvim")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	d, _, receipt, _ := nvimSyncFixture(t)
	d.Repository, d.Executable, d.query = repository, binary, queryNvimSync
	d.Environment = append(d.Environment, "PATH="+os.Getenv("PATH"))
	// A missing plugin must fail without running production init.lua, which would
	// bootstrap lazy.nvim and write package state in an ordinary invocation.
	if err := os.MkdirAll(d.payload(receipt.OperationID), 0700); err != nil {
		t.Fatal(err)
	}
	intent := nvimSyncIntent{Operation: receipt.OperationID}
	before, err := snapshotTree(filepath.Dir(filepath.Dir(d.Directory)), maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	output, err := d.query(context.Background(), d.command(intent, 0, true))
	if err == nil || !strings.Contains(string(output), "plugin differs from lock:") {
		t.Fatalf("real checker did not reject missing locked plugins: %s %v", output, err)
	}
	after, err := snapshotTree(filepath.Dir(filepath.Dir(d.Directory)), maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		t.Fatal("check wrote runtime/personal data", before, after, err)
	}
	t.Log("checksum-pinned Neovim rejected missing locked plugins; disposable home stayed byte-identical")
}

// This opt-in recipe proof downloads only declared private archives. The host
// supplies its native Git, make and C compiler; it never installs host packages.
func TestNativeNvimSyncCompleteBootstrap(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NVIM_SYNC_BOOTSTRAP") != "1" {
		t.Skip("set DOTFILES_TEST_NVIM_SYNC_BOOTSTRAP=1 on a disposable native toolchain fixture")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("first recipe proof requires the macOS native compiler fixture")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "tools"), 0700); err != nil {
		t.Fatal(err)
	}
	path := []string{}
	programs := map[string]string{}
	for _, id := range []string{"tool.nvim", "tool.node", "tool.tree-sitter", "tool.python", "tool.cmake"} {
		pin := pins[id]
		payload := filepath.Join(home, "tools", id)
		t.Log("downloading verified private", id)
		if err := downloadArchive(context.Background(), nil, pin, payload); err != nil {
			t.Fatal(id, err)
		}
		if err := preparePythonArchive(context.Background(), pin, payload); err != nil {
			t.Fatal(id, err)
		}
		for _, dir := range pin.BinDirs {
			path = append(path, filepath.Join(payload, filepath.FromSlash(dir)))
		}
		for name, command := range pin.Commands {
			programs[name] = filepath.Join(payload, filepath.FromSlash(command))
		}
	}
	path = append(path, "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	d, err := NewNvimSyncDriver(NvimSyncOptions{Repository: repository, Directory: filepath.Join(home, "state", "nvim-sync"), RuntimeDirectory: filepath.Join(home, "data", "nvim", "dotfiles-runtime"), Executable: programs["nvim"], Environment: []string{"HOME=" + home, "PATH=" + strings.Join(path, string(os.PathListSeparator))}})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := workerFixture(filepath.Join(home, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		t.Log("running locked Neovim phase", command.Arguments[len(command.Arguments)-1])
		return worker.run(ctx, command)
	}
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Ownership: "uncertain", Status: "in-progress"}
	r := Resource{ID: "nvim.sync", Action: "nvim-sync"}
	after, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
	var nativeEntries, nativeBytes int64
	walkErr := filepath.WalkDir(d.payload(receipt.OperationID), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		nativeEntries++
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			nativeBytes += info.Size()
		}
		return nil
	})
	t.Logf("native runtime: %d entries, %d regular-file bytes; inventory error: %v", nativeEntries, nativeBytes, walkErr)
	if err != nil || !after.Healthy {
		t.Fatal(after, err)
	}
	receipt.After, receipt.Ownership = after, "created"
	snapshot, err := snapshotTree(d.payload(receipt.OperationID), maxPackageBytes, maxNvimEntries)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := d.Observe(context.Background(), r, receipt)
	if err != nil || !checked.Healthy {
		t.Fatal(checked, err)
	}
	unchanged, err := snapshotTree(d.payload(receipt.OperationID), maxPackageBytes, maxNvimEntries)
	if err != nil || snapshot != unchanged {
		t.Fatal("healthy check changed the installed runtime", err)
	}
	receipt.OperationID = strings.Repeat("b", 64)
	removed, err := d.Remove(context.Background(), r, receipt)
	if err != nil || removed.Present {
		t.Fatal(removed, err)
	}
	receipt = removedReceipt(receipt, removed)
	if _, err := d.FinishTransaction(context.Background(), Plan{}, map[string]Receipt{r.ID: receipt}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(d.payload(strings.Repeat("a", 64))); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native owned runtime survived removal", err)
	}
	t.Log("real locked plugins, all parsers and Mason tools installed, checked without writes, and removed")
}
