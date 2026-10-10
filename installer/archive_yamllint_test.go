package installer

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func yamllintPreparationFixture(t *testing.T) (*ArchiveDriver, *[]nativeCommand) {
	t.Helper()
	d, add := preparationFixture(t)
	pin := add([]byte("yamllint wheel"), "file")
	pin.Version, pin.File = "1.38.0", "yamllint-1.38.0-py3-none-any.whl"
	binary := "bin/yamllint"
	if runtime.GOOS == "windows" {
		binary = "Scripts/yamllint.exe"
	}
	pin.Commands, pin.BinDirs = map[string]string{"yamllint": binary}, []string{filepath.ToSlash(filepath.Dir(binary))}
	pathspec := add([]byte("pathspec wheel"), "file")
	pathspec.Version, pathspec.File = "1.1.1", "pathspec-1.1.1-py3-none-any.whl"
	pathspec.RequiredFiles = []string{pathspec.File}
	pyyaml := add([]byte("PyYAML wheel"), "file")
	pyyaml.Version, pyyaml.File = "6.0.3", "pyyaml-6.0.3-cp313-cp313-macosx_11_0_arm64.whl"
	pyyaml.RequiredFiles = []string{pyyaml.File}
	pin.Yamllint = &YamllintPin{Pathspec: pathspec, PyYAML: pyyaml, PythonPinID: strings.Repeat("a", 64)}
	d.Pins["tool.shared"], d.Python = pin, filepath.Join(d.Directory, "python", "current", "python")
	commands := []nativeCommand{}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		commands = append(commands, command)
		if slices.Contains(command.Arguments, "venv") {
			payload := command.Arguments[len(command.Arguments)-1]
			for _, name := range []string{binary, filepath.ToSlash(filepath.Join(filepath.Dir(binary), "python"))} {
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

func TestArchiveYamllintPreparationUsesOnlyPinnedOfflineWheels(t *testing.T) {
	d, commands := yamllintPreparationFixture(t)
	receipt := Receipt{OperationID: strings.Repeat("b", 64), Status: "in-progress"}
	r := Resource{ID: "tool.shared"}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt)
	if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID || len(*commands) != 6 {
		t.Fatal(after, err, len(*commands))
	}
	for _, command := range (*commands)[1:4] {
		for _, flag := range []string{"-I", "-B", "--isolated", "--no-index", "--require-hashes", "--no-deps", "--only-binary=:all:", "--no-cache-dir"} {
			if !slices.Contains(command.Arguments, flag) {
				t.Fatal("lost offline wheel control", flag)
			}
		}
		data, err := os.ReadFile(command.Arguments[len(command.Arguments)-1])
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(data))
		if len(fields) != 4 || fields[1] != "@" || !strings.HasPrefix(fields[3], "--hash=sha256:") {
			t.Fatal("unpinned requirement", string(data))
		}
		local, err := url.Parse(fields[2])
		if err != nil || local.Scheme != "file" || local.Host != "" {
			t.Fatal("online requirement", string(data), err)
		}
	}
	for _, command := range *commands {
		if !slices.Contains(command.Environment, "PIP_CONFIG_FILE="+os.DevNull) || !slices.Contains(command.Environment, "PYTHONPATH=") {
			t.Fatal("preparation inherited Python/pip configuration")
		}
	}
	if !slices.Contains((*commands)[4].Arguments, "checked-hash") {
		t.Fatal("ordinary imports would change owned bytecode")
	}
	if observed, err := d.Observe(context.Background(), r, receipt); err != nil || !observed.Healthy || len(*commands) != 6 {
		t.Fatal("check rebuilt the tool", observed, err)
	}
}

func TestArchiveYamllintRequiresEveryDependencyChecksumBeforeExecution(t *testing.T) {
	d, commands := yamllintPreparationFixture(t)
	pin := d.Pins["tool.shared"]
	pin.Yamllint.PyYAML.SHA256 = strings.Repeat("c", 64)
	d.Pins["tool.shared"] = pin
	r := Resource{ID: "tool.shared"}
	before, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, Receipt{OperationID: strings.Repeat("b", 64)}); err == nil || len(*commands) != 0 {
		t.Fatal("unchecked dependency reached Python", err, len(*commands))
	}
}

func TestArchiveYamllintInterruptedStageUsesExistingArchiveRecovery(t *testing.T) {
	d, commands := yamllintPreparationFixture(t)
	run, failed := d.Run, false
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if !failed && slices.Contains(command.Arguments, "pip") {
			failed = true
			return nil, errors.New("interrupted offline pip")
		}
		return run(ctx, command)
	}
	r, receipt := Resource{ID: "tool.shared"}, Receipt{OperationID: strings.Repeat("b", 64), Status: "in-progress"}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err == nil {
		t.Fatal("interrupted stage published")
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || observed.ResourceResume == nil {
		t.Fatal(observed, err)
	}
	after, err := d.ResumeResource(context.Background(), r, Operation{Action: "install", Observed: observed}, receipt)
	if err != nil || !after.Healthy || len(after.Preserved) != 1 || len(*commands) != 7 {
		t.Fatal(after, err, len(*commands))
	}
	if (*commands)[0].Operation == (*commands)[1].Operation {
		t.Fatal("fresh stage replayed old venv result")
	}
}

func TestArchiveYamllintRejectsIncompleteOrExecutableRecipes(t *testing.T) {
	mutations := []func(*ArchivePin){
		func(p *ArchivePin) { p.File = "yamllint.tar.gz" },
		func(p *ArchivePin) { p.Yamllint.PythonPinID = "" },
		func(p *ArchivePin) { p.Yamllint.Pathspec.File = "pathspec.tar.gz" },
		func(p *ArchivePin) { p.Yamllint.PyYAML.File = "pyyaml-6.0.3.tar.gz" },
		func(p *ArchivePin) { p.Yamllint.PyYAML.Yamllint = &YamllintPin{} },
		func(p *ArchivePin) { p.Yamllint.PyYAML.Commands = map[string]string{"unreviewed": "unreviewed"} },
		func(p *ArchivePin) { p.PortableGit = true },
		func(p *ArchivePin) { p.Latex2text = &Latex2textPin{} },
		func(p *ArchivePin) { p.Commands["extra"] = "bin/extra" },
	}
	for i, mutate := range mutations {
		d, _ := yamllintPreparationFixture(t)
		pin := d.Pins["tool.shared"]
		mutate(&pin)
		if err := pin.Validate(); err == nil {
			t.Fatal("invalid recipe accepted", i)
		}
	}
}

func TestArchiveYamllintPinsMatchAllTargetsAndSelectedPython(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(target, err)
		}
		pin, python := pins["tool.yamllint"], pins["tool.python"]
		if err := pin.Validate(); err != nil || pin.Yamllint.PythonPinID != archivePinID(python) {
			t.Fatal(target, err)
		}
		original := archivePinID(pin)
		python.SHA256 = strings.Repeat("b", 64)
		changed, err := bindYamllintPython(pin, python, target.OS+"/"+target.Arch)
		if err != nil || archivePinID(changed) == original || archivePinID(pin) != original {
			t.Fatal("Python change not bound without mutating old recipe", err)
		}
		python.Version = "3.14.0"
		if _, err := bindYamllintPython(pin, python, target.OS+"/"+target.Arch); err == nil {
			t.Fatal("mismatched Python ABI accepted", target)
		}
	}
}

func TestArchiveYamllintLegacyPinsKeepTheirIdentity(t *testing.T) {
	_, add := preparationFixture(t)
	pin := add([]byte("legacy tool"), "file")
	pin.File, pin.RequiredFiles = "tool", []string{"tool"}
	data, err := json.Marshal(pin)
	if err != nil || strings.Contains(string(data), "yamllint") {
		t.Fatal("new recipe changed legacy shape", err)
	}
	var restored ArchivePin
	if err := Decode(data, &restored); err != nil || restored.Validate() != nil || archivePinID(restored) != archivePinID(pin) {
		t.Fatal("legacy recipe changed", err)
	}
}
