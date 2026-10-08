package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func nvimSyncFixture(t *testing.T) (*NvimSyncDriver, Resource, Receipt, *[]nativeCommand) {
	t.Helper()
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(home, "repository")
	if err := os.MkdirAll(filepath.Join(repository, "nvim"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "nvim", "init.lua"), []byte("-- fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "nvim", "lazy-lock.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commands := []nativeCommand{}
	d, err := NewNvimSyncDriver(NvimSyncOptions{Repository: repository, Directory: filepath.Join(home, "state", "nvim-sync"), RuntimeDirectory: filepath.Join(home, "data", "nvim", "dotfiles-runtime"), Executable: filepath.Join(home, "tools", "nvim"), Environment: []string{"HOME=" + home, "PATH=" + filepath.Join(home, "tools")}})
	if err != nil {
		t.Fatal(err)
	}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		commands = append(commands, command)
		root := ""
		for _, env := range command.Environment {
			if value, ok := strings.CutPrefix(env, "DOTFILES_NVIM_RUNTIME="); ok {
				root = value
			}
		}
		if !strings.HasPrefix(root, home+string(filepath.Separator)) {
			t.Fatal("escaped disposable home", root)
		}
		return nil, os.WriteFile(filepath.Join(root, "owned"), []byte("locked runtime"), 0600)
	}
	d.query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if !slices.Contains(command.Arguments, "--clean") || slices.Contains(command.Arguments, "-u") || !slices.Contains(command.Arguments, "+lua require('util.sync_check').run()") {
			t.Fatal("observation can run install", command)
		}
		return nil, nil
	}
	return d, Resource{ID: "nvim.sync", Action: "nvim-sync"}, Receipt{OperationID: strings.Repeat("a", 64), Ownership: "uncertain", Status: "in-progress"}, &commands
}

func TestNvimSyncLockedPhasesAndObservationNeverRunMutation(t *testing.T) {
	d, r, receipt, commands := nvimSyncFixture(t)
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil || before.Present {
		t.Fatal(before, err)
	}
	if _, err := os.Stat(d.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview wrote state", err)
	}
	after, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt)
	if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID {
		t.Fatal(after, err)
	}
	if len(*commands) != 3 {
		t.Fatal("wrong phase count", len(*commands))
	}
	phases := []string{"+Lazy! restore", "+lua require('lazy').load({ plugins = { 'nvim-treesitter' } })", "+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')"}
	for i, command := range *commands {
		if !slices.Contains(command.Arguments, phases[i]) || !slices.Contains(command.Arguments, "NONE") || command.Operation == "" {
			t.Fatal("lost existing locked phase", i, command)
		}
		if (i == 1) != slices.Contains(command.Environment, "DOTFILES_TREESITTER_SYNC_INSTALL=1") {
			t.Fatal("parser scope leaked", i)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := d.Observe(context.Background(), r, receipt); err != nil {
			t.Fatal(err)
		}
	}
	if len(*commands) != 3 {
		t.Fatal("check ran mutation")
	}
}
func TestNvimSyncRemovalPreservesPersonalStateAndChangedOwnedFiles(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	after, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	payload := d.payload(receipt.OperationID)
	personal := filepath.Join(filepath.Dir(d.RuntimeDirectory), "shada", "main.shada")
	if err := os.MkdirAll(filepath.Dir(personal), 0700); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{personal: "personal Shada", filepath.Join(payload, "user-session"): "session", filepath.Join(payload, "owned"): "user edits"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
	removed, err := d.Remove(context.Background(), r, receipt)
	if err != nil || removed.Present {
		t.Fatal(removed, err)
	}
	receipt = removedReceipt(receipt, removed)
	report, err := d.FinishTransaction(context.Background(), Plan{}, map[string]Receipt{r.ID: receipt})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{personal, filepath.Join(payload, "user-session"), filepath.Join(payload, "owned")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("lost personal file", path, err)
		}
	}
	if !slices.Contains(report[r.ID], filepath.Join(payload, "owned")) || !slices.Contains(report[r.ID], filepath.Join(payload, "user-session")) {
		t.Fatal("no disclosure", report)
	}
}
func TestNvimSyncUpdateUsesFreshGenerationAndCleansOnlyRecordedFiles(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	after, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	old := d.payload(receipt.OperationID)
	receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
	after, err = d.Apply(context.Background(), r, Operation{Action: "update"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.After = after
	if _, err := d.FinishTransaction(context.Background(), Plan{}, map[string]Receipt{r.ID: receipt}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old generation survived", err)
	}
	if _, err := os.Stat(filepath.Join(d.payload(receipt.OperationID), "owned")); err != nil {
		t.Fatal("new runtime lost", err)
	}
}
func TestNvimSyncFailureResumesOnlySavedPhaseAndRejectsChangedSource(t *testing.T) {
	d, r, receipt, commands := nvimSyncFixture(t)
	run := d.Run
	failed := false
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if slices.Contains(command.Arguments, "+lua require('lazy').load({ plugins = { 'nvim-treesitter' } })") && !failed {
			failed = true
			return []byte("compiler failed"), errors.New("exit 1")
		}
		return run(ctx, command)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("failure reported success")
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || observed.ResourceResume == nil || observed.Present {
		t.Fatal(observed, err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
	if err != nil || !after.Healthy {
		t.Fatal(after, err)
	}
	if len(*commands) != 3 {
		t.Fatal("completed phase repeated", len(*commands))
	}
	d, r, receipt, _ = nvimSyncFixture(t)
	d.Run = func(context.Context, nativeCommand) ([]byte, error) { return nil, errors.New("offline") }
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("offline success")
	}
	if err := os.WriteFile(filepath.Join(d.Repository, "nvim", "init.lua"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	observed, err = d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt); err == nil {
		t.Fatal("recovery changed payload")
	}
}
func TestNvimSyncRejectsUnownedOrRedirectedRuntime(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(d.current()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(outside, d.current()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("acquired foreign runtime")
	}
	if _, err := d.Remove(context.Background(), r, receipt); err == nil {
		t.Fatal("removed unowned runtime")
	}
}

func TestNvimSyncRecoveryTokenBindsPartialRuntimeAndUnchangedSource(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	d.Run = func(context.Context, nativeCommand) ([]byte, error) { return nil, errors.New("interrupted") }
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("failed phase succeeded")
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || observed.ResourceResume == nil {
		t.Fatal(observed, err)
	}
	if err := os.WriteFile(filepath.Join(d.payload(receipt.OperationID), "personal"), []byte("new user file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt); err == nil {
		t.Fatal("stale approval accepted changed partial runtime")
	}
}

func TestNvimSyncLostReservationPreservesInterruptedBytes(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	desired, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	intent := nvimSyncIntent{Schema: 1, Operation: receipt.OperationID, Action: "install", Desired: desired}
	if err := d.save(intent); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(d.payload(receipt.OperationID), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.payload(receipt.OperationID), "personal"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
	if err != nil || !after.Healthy || len(after.Preserved) != 1 {
		t.Fatal(after, err)
	}
	data, err := os.ReadFile(filepath.Join(after.Preserved[0], "personal"))
	if err != nil || string(data) != "keep" {
		t.Fatal("interrupted stage was deleted", err)
	}
}

func TestNvimSyncFailedPostconditionRequiresExplicitRecipeRetry(t *testing.T) {
	d, r, receipt, commands := nvimSyncFixture(t)
	query := d.query
	failed := false
	d.query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if !failed {
			failed = true
			return []byte("missing parser"), errors.New("exit 1")
		}
		return query(ctx, command)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("missing runtime passed installation")
	}
	if len(*commands) != 3 {
		t.Fatal("phase count", len(*commands))
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || observed.Present || observed.ResourceResume == nil {
		t.Fatal(observed, err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
	if err != nil || !after.Healthy {
		t.Fatal(after, err)
	}
	if len(*commands) != 6 || (*commands)[0].Operation == (*commands)[3].Operation {
		t.Fatal("failed postcondition did not rerun the approved recipe")
	}
}

func TestNvimSyncKeepsLazyLockWritesInsidePrivateGeneration(t *testing.T) {
	d, r, receipt, _ := nvimSyncFixture(t)
	source := filepath.Join(d.Repository, "nvim", "lazy-lock.json")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	run := d.Run
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		private := ""
		for _, env := range command.Environment {
			if value, ok := strings.CutPrefix(env, "DOTFILES_NVIM_LOCKFILE="); ok {
				private = value
			}
		}
		if private == source || !strings.HasPrefix(private, d.payload(receipt.OperationID)+string(filepath.Separator)) {
			t.Fatal("Lazy lock escaped private generation", private)
		}
		if err := os.WriteFile(private, []byte("{\"rewritten_by_lazy\":true}\n"), 0600); err != nil {
			return nil, err
		}
		return run(ctx, command)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(before) != string(after) {
		t.Fatal("restore rewrote authoritative lock", err)
	}
}

func TestNvimSyncManifestChunksPreserveBoundedOwnership(t *testing.T) {
	d, _, receipt, _ := nvimSyncFixture(t)
	entries := []configEntry{{Path: ".", Kind: "directory", Mode: 0700}}
	for i := 0; i < 2*nvimManifestChunkEntries; i++ {
		entries = append(entries, configEntry{Path: fmt.Sprintf("file-%05d", i), Kind: "file", Content: strings.Repeat("a", 64), Mode: 0600})
	}
	hashes, err := d.writeManifest(receipt.OperationID, entries)
	if err != nil || len(hashes) != 3 {
		t.Fatal(len(hashes), err)
	}
	restored, err := d.readManifest(nvimSyncIntent{Operation: receipt.OperationID, Manifest: hashes})
	if err != nil || !slices.Equal(restored, entries) {
		t.Fatal("chunk round trip changed ownership", err)
	}
	again, err := d.writeManifest(receipt.OperationID, entries)
	if err != nil || !slices.Equal(hashes, again) {
		t.Fatal("interrupted manifest cannot resume", err)
	}
}

func corruptNvimManifest(t *testing.T, d *NvimSyncDriver, operation, corruption string) {
	t.Helper()
	path := d.manifestPath(operation, 0)
	if corruption == "missing" {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := readDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	var chunk nvimSyncManifestChunk
	if err := Decode(data, &chunk); err != nil {
		t.Fatal(err)
	}
	chunk.Entries[0].Mode = 0755
	if err := saveDocument(path, chunk); err != nil {
		t.Fatal(err)
	}
}

func TestNvimSyncDamagedManifestCannotPublishOrAcquireNewOwnership(t *testing.T) {
	for _, corruption := range []string{"missing", "tampered"} {
		t.Run(corruption, func(t *testing.T) {
			d, r, receipt, commands := nvimSyncFixture(t)
			if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err != nil {
				t.Fatal(err)
			}
			intent, err := d.intent(receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			// Reproduce interruption after manifest persistence but before publication.
			intent.Complete = false
			if err := d.save(intent); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(d.current()); err != nil {
				t.Fatal(err)
			}
			personal := filepath.Join(d.payload(receipt.OperationID), "personal")
			if err := os.WriteFile(personal, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			corruptNvimManifest(t, d, receipt.OperationID, corruption)
			if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
				t.Fatal("damaged ownership published")
			}
			if _, err := os.Lstat(d.current()); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("runtime was published", err)
			}
			if len(*commands) != 3 {
				t.Fatal("damaged ownership restarted the recipe")
			}
			data, err := os.ReadFile(personal)
			if err != nil || string(data) != "keep" {
				t.Fatal("personal data changed", err)
			}
		})
	}
}

func TestNvimSyncDamagedManifestRefusesRemovalAndPreservesObsoleteRuntime(t *testing.T) {
	for _, corruption := range []string{"missing", "tampered"} {
		t.Run(corruption, func(t *testing.T) {
			d, r, receipt, _ := nvimSyncFixture(t)
			after, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
			if err != nil {
				t.Fatal(err)
			}
			previousOperation := receipt.OperationID
			previous := d.payload(previousOperation)
			receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
			after, err = d.Apply(context.Background(), r, Operation{Action: "update"}, receipt)
			if err != nil {
				t.Fatal(err)
			}
			receipt.After = after
			personal := filepath.Join(previous, "personal")
			if err := os.WriteFile(personal, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			corruptNvimManifest(t, d, previousOperation, corruption)
			report, err := d.FinishTransaction(context.Background(), Plan{}, map[string]Receipt{r.ID: receipt})
			if err != nil || !slices.Contains(report[r.ID], previous) {
				t.Fatal("damaged runtime was not disclosed", report, err)
			}
			for _, path := range []string{personal, filepath.Join(previous, "owned")} {
				if _, err := os.Stat(path); err != nil {
					t.Fatal("damaged ownership was used for cleanup", path, err)
				}
			}
			corruptNvimManifest(t, d, receipt.OperationID, corruption)
			receipt.OperationID = strings.Repeat("c", 64)
			if _, err := d.Remove(context.Background(), r, receipt); err == nil {
				t.Fatal("damaged ownership authorized removal")
			}
			if current, err := archiveLinkTarget(d.current()); err != nil || current != d.payload(strings.Repeat("b", 64)) {
				t.Fatal("current runtime changed", current, err)
			}
		})
	}
}

func TestNvimSyncRustHomeRequiresExplicitManagedAbsolutePath(t *testing.T) {
	d, _, receipt, _ := nvimSyncFixture(t)
	t.Setenv("RUSTUP_HOME", filepath.Join(t.TempDir(), "personal"))
	command := d.command(nvimSyncIntent{Operation: receipt.OperationID}, 0, false)
	if !slices.Contains(command.Environment, "RUSTUP_HOME="+filepath.Join(d.payload(receipt.OperationID), ".rustup")) {
		t.Fatal("personal Rust home was inherited")
	}
	options := d.NvimSyncOptions
	options.Environment = append(slices.Clone(options.Environment), "RUSTUP_HOME=relative")
	if _, err := NewNvimSyncDriver(options); err == nil {
		t.Fatal("relative Rust home accepted")
	}
	managed := filepath.Join(d.RuntimeDirectory, "rust")
	options.Environment[len(options.Environment)-1] = "RUSTUP_HOME=" + managed
	explicit, err := NewNvimSyncDriver(options)
	if err != nil {
		t.Fatal(err)
	}
	command = explicit.command(nvimSyncIntent{Operation: receipt.OperationID}, 0, false)
	if slices.Contains(command.Environment, "RUSTUP_HOME="+filepath.Join(d.payload(receipt.OperationID), ".rustup")) || !slices.Contains(command.Environment, "RUSTUP_HOME="+managed) {
		t.Fatal("explicit managed Rust home was lost")
	}
}
