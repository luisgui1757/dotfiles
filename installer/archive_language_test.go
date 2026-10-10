package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func preparationFixture(t *testing.T) (*ArchiveDriver, func([]byte, string) ArchivePin) {
	t.Helper()
	assets := map[string][]byte{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(data); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	directory, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	driver := &ArchiveDriver{Directory: directory, Pins: map[string]ArchivePin{}, Client: server.Client()}
	add := func(data []byte, format string) ArchivePin {
		hash := sha256.Sum256(data)
		digest := hex.EncodeToString(hash[:])
		assets["/"+digest] = data
		return ArchivePin{Version: "1.0", URL: server.URL + "/" + digest, SHA256: digest, Format: format}
	}
	return driver, add
}

func rustPreparationFixture(t *testing.T) (*ArchiveDriver, ArchivePin) {
	t.Helper()
	d, add := preparationFixture(t)
	pin := add(archiveTar(t, []archiveFixtureEntry{{Name: "rust/LICENSE", Text: "retained provenance"}, {Name: "rust/install.sh", Text: "never execute this installer"}, {Name: "rust/rustc/manifest.in", Text: "file:bin/rustc.exe"}, {Name: "rust/rustc/bin/rustc.exe", Text: "rust compiler", Mode: 0755}, {Name: "rust/rustc/bin/rustdoc.exe", Text: "rust documentation", Mode: 0755}}), "tar.gz")
	pin.StripComponents = 1
	pin.RustTarget = "x86_64-pc-windows-msvc"
	d.WindowsRustLauncher = filepath.Join(d.Directory, "installer-launcher.exe")
	launcher := []byte("trusted fixture launcher")
	if err := os.WriteFile(d.WindowsRustLauncher, launcher, 0755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(launcher)
	pin.WindowsRustLauncher = &WindowsRustLauncherPin{Revision: 1, SHA256: hex.EncodeToString(hash[:])}
	pin.Commands = map[string]string{}
	for _, name := range []string{"rustc", "cargo", "rustfmt", "cargo-fmt", "cargo-clippy", "clippy-driver"} {
		pin.Commands[name] = "bin/" + name + ".exe"
	}
	pin.BinDirs = []string{"bin"}
	pin.RequiredFiles = []string{"lib/rustlib/src/rust/library/core/src/lib.rs"}
	for index, path := range []string{"bin/cargo.exe", "lib/rustlib/x86_64-pc-windows-msvc/lib/libstd-hash.rlib", "bin/rustfmt.exe", "bin/cargo-clippy.exe", "lib/rustlib/src/rust/library/core/src/lib.rs"} {
		entries := []archiveFixtureEntry{{Name: "rust/" + rustComponentNames(pin.RustTarget)[index] + "/manifest.in", Text: "file:" + path}, {Name: "rust/" + rustComponentNames(pin.RustTarget)[index] + "/" + path, Text: path, Mode: 0755}}
		if index == 1 {
			entries = append(entries, archiveFixtureEntry{Name: "rust/" + rustComponentNames(pin.RustTarget)[index] + "/lib/rustlib/" + pin.RustTarget + "/lib/libcore-hash.rlib", Text: "core"})
		}
		if index == 2 || index == 3 {
			name := "cargo-fmt"
			if index == 3 {
				name = "clippy-driver"
			}
			entries = append(entries, archiveFixtureEntry{Name: "rust/" + rustComponentNames(pin.RustTarget)[index] + "/bin/" + name + ".exe", Text: name, Mode: 0755})
		}
		component := add(archiveTar(t, entries), "tar.gz")
		component.StripComponents = 1
		component.RequiredFiles = []string{rustComponentNames(pin.RustTarget)[index] + "/manifest.in"}
		pin.RustComponents = append(pin.RustComponents, component)
	}
	d.Pins["tool.shared"] = pin
	return d, pin
}

func TestArchiveRustPreparationAssemblesThenUsesExistingLifecycle(t *testing.T) {
	d, _ := rustPreparationFixture(t)
	r := Resource{ID: "tool.shared"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt)
	if err != nil || !after.Healthy {
		t.Fatal(after, err)
	}
	old, err := d.PayloadPath(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(old, "LICENSE")); err != nil || string(data) != "retained provenance" {
		t.Fatal("lost upstream top-level metadata", err)
	}
	personal := filepath.Join(old, "personal")
	receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
	removed, err := d.Remove(context.Background(), r, receipt)
	if err != nil || removed.Present {
		t.Fatal(removed, err)
	}
	receipt = removedReceipt(receipt, removed)
	if err := os.WriteFile(personal, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := d.FinishTransaction(context.Background(), Plan{}, map[string]Receipt{r.ID: receipt})
	if err != nil || !slices.Contains(report[r.ID], filepath.Dir(old)) {
		t.Fatal(report, err)
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "keep" {
		t.Fatal("lost personal entry", err)
	}
	if _, err := os.Stat(filepath.Join(old, "bin", "rustc.exe")); err != nil {
		t.Fatal("changed generation was partially acquired for cleanup", err)
	}
}

func TestArchiveRustMergeRejectsCollidingLeaves(t *testing.T) {
	directory := t.TempDir()
	source, destination := filepath.Join(directory, "source"), filepath.Join(directory, "destination")
	for _, path := range []string{source, destination} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "shared"), []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := mergeRustDirectory(context.Background(), source, destination); err == nil {
		t.Fatal("collision replaced a leaf")
	}
	if data, err := os.ReadFile(filepath.Join(destination, "shared")); err != nil || string(data) != destination {
		t.Fatal("existing content changed", err)
	}
}

func TestArchivePreparationRejectsNestedRecipesAndMissingComponents(t *testing.T) {
	_, pin := rustPreparationFixture(t)
	pin.RustComponents[0].RustComponents = []ArchivePin{pin.RustComponents[1]}
	if err := pin.Validate(); err == nil {
		t.Fatal("nested preparation accepted")
	}
	pin.RustComponents = pin.RustComponents[:4]
	if err := pin.Validate(); err == nil {
		t.Fatal("incomplete Rust component set accepted")
	}
}

func TestArchiveFixedPreparationsKeepLegacyPinShape(t *testing.T) {
	pin := ArchivePin{Version: "1.0", URL: "https://example.org/tool", SHA256: strings.Repeat("a", 64), Format: "file", File: "tool", RequiredFiles: []string{"tool"}}
	data, err := json.Marshal(pin)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"rust_target", "rust_components", "latex2text", "portable_git", "windows_rust_launcher"} {
		if strings.Contains(string(data), field) {
			t.Fatal("optional preparation changed legacy persisted identity", field)
		}
	}
	var restored ArchivePin
	if err := Decode(data, &restored); err != nil || restored.Validate() != nil || archivePinID(restored) != archivePinID(pin) {
		t.Fatal("legacy pin no longer round trips", err)
	}
}

func latexPreparationFixture(t *testing.T) (*ArchiveDriver, *[]nativeCommand) {
	t.Helper()
	d, add := preparationFixture(t)
	pin := add([]byte("pinned source archive"), "file")
	pin.Version, pin.File = "2.11", "pylatexenc-2.11.tar.gz"
	binary := "bin/latex2text"
	if runtime.GOOS == "windows" {
		binary = "Scripts/latex2text.exe"
	}
	pin.Commands = map[string]string{"latex2text": binary}
	pin.BinDirs = []string{filepath.ToSlash(filepath.Dir(binary))}
	backend := add([]byte("pinned build backend"), "file")
	backend.Version, backend.File = "84.0.0", "setuptools-84.0.0-py3-none-any.whl"
	backend.RequiredFiles = []string{backend.File}
	pin.Latex2text = &Latex2textPin{Backend: backend, PythonPinID: strings.Repeat("c", 64), ConsoleRevision: 1}
	d.Pins["tool.shared"], d.Python = pin, filepath.Join(d.Directory, "python", "current", "python")
	commands := []nativeCommand{}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		commands = append(commands, command)
		if slices.Contains(command.Arguments, "venv") {
			payload := command.Arguments[len(command.Arguments)-1]
			python := "bin/python"
			if runtime.GOOS == "windows" {
				python = "Scripts/python.exe"
			}
			for _, name := range []string{python, binary} {
				path := filepath.Join(payload, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					return nil, err
				}
				if err := os.WriteFile(path, []byte("fixture program"), 0755); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}
	return d, &commands
}

func TestArchiveLatexPreparationKeepsReleasedPinsAndPrivateCommands(t *testing.T) {
	d, commands := latexPreparationFixture(t)
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
	before, err := d.Observe(context.Background(), Resource{ID: "tool.shared"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.Apply(context.Background(), Resource{ID: "tool.shared"}, Operation{Action: "install", Observed: before}, receipt)
	if err != nil || !after.Healthy || len(*commands) != 6 {
		t.Fatal(after, len(*commands), err)
	}
	for _, index := range []int{1, 2} {
		command := (*commands)[index]
		for _, arg := range []string{"--isolated", "--no-index", "--require-hashes", "--no-deps", "--no-cache-dir"} {
			if !slices.Contains(command.Arguments, arg) {
				t.Fatal("lost isolated pinned install", command)
			}
		}
	}
	if !slices.Contains((*commands)[2].Arguments, "--no-binary=pylatexenc") || !slices.Contains((*commands)[2].Arguments, "--no-build-isolation") {
		t.Fatal("source build changed")
	}
	if !slices.Contains((*commands)[5].Arguments, "-B") {
		t.Fatal("probe can create bytecode")
	}
	if _, err := d.Observe(context.Background(), Resource{ID: "tool.shared"}, receipt); err != nil {
		t.Fatal(err)
	}
	if len(*commands) != 6 {
		t.Fatal("check rebuilt the converter")
	}
}

func TestArchiveLatexInterruptedStageUsesFreshNativeCommandIdentities(t *testing.T) {
	d, commands := latexPreparationFixture(t)
	run := d.Run
	failed := false
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if !failed && slices.Contains(command.Arguments, "--no-binary=pylatexenc") {
			failed = true
			return nil, errors.New("interrupted pip")
		}
		return run(ctx, command)
	}
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
	r := Resource{ID: "tool.shared"}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err == nil {
		t.Fatal("failed recipe published")
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || observed.ResourceResume == nil {
		t.Fatal(observed, err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
	if err != nil || !after.Healthy || len(after.Preserved) != 1 {
		t.Fatal(after, err)
	}
	if len(*commands) != 8 || (*commands)[0].Operation == (*commands)[2].Operation {
		t.Fatal("fresh stage replayed previous preparation")
	}
}

func TestArchivePythonBuildUsesNativeTemporaryDirectory(t *testing.T) {
	d, commands := latexPreparationFixture(t)
	// A versioned payload already consumes much of Windows' working-directory
	// allowance. Pip adds a build subdirectory of its own below TEMP/TMP.
	d.Directory = filepath.Join(d.Directory, strings.Repeat("long-home-", 8))
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
	before, err := d.Observe(context.Background(), Resource{ID: "tool.shared"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), Resource{ID: "tool.shared"}, Operation{Action: "install", Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	for _, command := range *commands {
		for _, name := range []string{"TMP=", "TEMP=", "TMPDIR="} {
			if !slices.Contains(command.Environment, name+os.TempDir()) {
				t.Fatal("Python build temporary directory nests beneath the versioned payload", command.Environment)
			}
		}
	}
}
