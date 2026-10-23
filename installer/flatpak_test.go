package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func flatpakFixture(t *testing.T) (*FlatpakDriver, Resource, Receipt, string) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := &FlatpakDriver{Directory: filepath.Join(root, "flatpak"), WorkerDirectory: filepath.Join(root, "worker"), Home: root, Arch: "x86_64"}
	r := Resource{ID: "tool.obsidian", Name: "Obsidian", Action: "package", Bindings: map[string]Binding{"linux": {Provider: "flatpak", Package: "md.obsidian.Obsidian"}}}
	commit := strings.Repeat("b", 64)
	location := filepath.Join(d.store(r.ID), "app", "md.obsidian.Obsidian", d.Arch, "stable", commit)
	for _, path := range []string{location, filepath.Join(d.store(r.ID), "repo")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(location, "metadata"), []byte("test native deployment"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.store(r.ID), "repo", "config"), []byte("verified remote"), 0600); err != nil {
		t.Fatal(err)
	}
	// Mock only the native process boundary. These responses retain the exact
	// documented output shape: `list --columns=ref` omits the app/ prefix.
	d.Query = func(_ context.Context, c nativeCommand) ([]byte, error) {
		if slices.Contains(c.Arguments, "--show-runtime") || slices.Contains(c.Arguments, "--runtime") {
			return []byte("org.freedesktop.Platform/x86_64/24.08\n"), nil
		}
		for flag, value := range map[string]string{"--columns=ref": "md.obsidian.Obsidian/x86_64/stable", "--show-origin": "flathub", "--show-commit": commit, "--show-location": location} {
			if slices.Contains(c.Arguments, flag) {
				return []byte(value + "\n"), nil
			}
		}
		return nil, errors.New("unexpected native query")
	}
	in := flatpakIntent{Schema: 1, Operation: strings.Repeat("a", 64), Resource: r.ID, Action: "install"}
	in.Commands = d.steps(r, in)
	if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
		t.Fatal(err)
	}
	for _, c := range in.Commands {
		hash, err := digest(c)
		if err != nil {
			t.Fatal(err)
		}
		zero := 0
		if err := saveDocument(filepath.Join(d.WorkerDirectory, "commands", c.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &zero}}); err != nil {
			t.Fatal(err)
		}
	}
	id, err := homebrewBootstrapDirectory(d.store(r.ID))
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(d.ownerPath(r.ID), flatpakOwner{Schema: 1, CreatedBy: in.Operation, Directory: id}); err != nil {
		t.Fatal(err)
	}
	o, err := d.Observe(t.Context(), r, Receipt{})
	if err != nil || !o.Present || !o.Healthy {
		t.Fatal(o, err)
	}
	in.Complete, in.Fingerprint = true, o.Fingerprint
	if err := saveDocument(d.intentPath(in.Operation), in); err != nil {
		t.Fatal(err)
	}
	return d, r, Receipt{Ownership: "created", Status: "complete", OperationID: in.Operation, After: o}, location
}

func TestFlatpakNativeRefOutputCompletesSavedInstall(t *testing.T) {
	d, r, receipt, _ := flatpakFixture(t)
	before, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	o, err := d.Observe(t.Context(), r, receipt)
	if err != nil || !o.Healthy || !o.Present || o.CompletedOperation != receipt.OperationID {
		t.Fatal(o, err)
	}
	after, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		t.Fatal("Check changed native state", err)
	}
}

func TestFlatpakChangedDeploymentIsProtected(t *testing.T) {
	d, r, receipt, location := flatpakFixture(t)
	if err := os.WriteFile(filepath.Join(location, "personal"), []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := d.Observe(t.Context(), r, receipt)
	if err != nil || o.Healthy || o.CompletedOperation != "" || sameArtifact(receipt.After, o) {
		t.Fatal(o, err)
	}
}

func TestFlatpakCommandsNeverRemoveDataOrTouchOtherInstallations(t *testing.T) {
	d, r, _, _ := flatpakFixture(t)
	for _, action := range []string{"install", "update", "repair", "remove"} {
		in := flatpakIntent{PrivateDependencies: true, Operation: strings.Repeat("c", 64), Action: action, Directory: "owned"}
		for _, command := range d.steps(r, in) {
			if !slices.Contains(command.Environment, "FLATPAK_USER_DIR="+d.store(r.ID)) || !slices.Contains(command.Arguments, "--user") {
				t.Fatal(command)
			}
			for _, setting := range []string{"FLATPAK_SYSTEM_DIR=" + d.store(r.ID), "FLATPAK_CONFIG_DIR=" + filepath.Join(d.store(r.ID), "config")} {
				if !slices.Contains(command.Environment, setting) {
					t.Fatal("shared dependency source remains enabled", command)
				}
			}
			for _, arg := range command.Arguments {
				if slices.Contains([]string{"--delete-data", "--unused", "--system", "--all"}, arg) {
					t.Fatal("destructive Flatpak command", command)
				}
			}
		}
	}
}

func TestFlatpakUnknownWorkerOutcomeCannotBeReplayed(t *testing.T) {
	d, r, receipt, _ := flatpakFixture(t)
	in, err := d.intent(r, receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	command := in.Commands[1]
	hash, err := digest(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
		t.Fatal(err)
	}
	err = d.result(command)
	var failed *flatpakCommandFailure
	if err == nil || errors.As(err, &failed) {
		t.Fatal("uncertain native command became retryable", err)
	}
}

func TestFlatpakMissingPrivateRuntimeNeedsRepair(t *testing.T) {
	d, r, receipt, _ := flatpakFixture(t)
	query := d.Query
	d.Query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if slices.Contains(command.Arguments, "--runtime") {
			return nil, nil
		}
		return query(ctx, command)
	}
	o, err := d.Observe(t.Context(), r, receipt)
	if err != nil || !o.Present || o.Healthy || !strings.Contains(o.HealthIssue, "runtime is missing") {
		t.Fatal(o, err)
	}
}

func TestFlatpakUpdateAndRemovalUseTheLastVerifiedArtifact(t *testing.T) {
	d, r, receipt, _ := flatpakFixture(t)
	query := d.Query
	removed := false
	d.Query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if removed && slices.Contains(command.Arguments, "--columns=ref") {
			return nil, nil
		}
		return query(ctx, command)
	}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		if slices.Contains(command.Arguments, "uninstall") {
			removed = true
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		zero := 0
		return nil, saveDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &zero}})
	}
	// Before is the permanent original absence, not the state before each
	// later operation. Preserve that uninstall baseline throughout the lifecycle.
	if receipt.Before.Present {
		t.Fatal("fixture lost its original absence")
	}
	receipt.OperationID, receipt.Status = strings.Repeat("c", 64), "in-progress"
	o, err := d.Apply(t.Context(), r, Operation{Action: "update", Observed: receipt.After}, receipt)
	if err != nil || !o.Healthy || o.CompletedOperation != receipt.OperationID {
		t.Fatal(o, err)
	}
	receipt.After = o
	receipt.OperationID = strings.Repeat("d", 64)
	o, err = d.Remove(t.Context(), r, receipt)
	if err != nil || o.Present || o.CompletedOperation != receipt.OperationID {
		t.Fatal(o, err)
	}
	if _, err := os.Stat(d.store(r.ID)); err != nil {
		t.Fatal("retained native infrastructure disappeared", err)
	}
}
