package installer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// YamllintPin is the fixed wheel-only closure: yamllint, pathspec and PyYAML.
// PythonPinID is bound to the selected managed interpreter before validation.
// No user/system Python environment, PyPI resolver or source build is involved.
type YamllintPin struct {
	Pathspec    ArchivePin `json:"pathspec"`
	PyYAML      ArchivePin `json:"pyyaml"`
	PythonPinID string     `json:"python_pin_id"`
}

var yamllintPyYAMLWheel = regexp.MustCompile(`^pyyaml-[0-9]+(?:\.[0-9]+)+-(cp[0-9]+)-(cp[0-9]+)-(macosx_11_0_arm64|manylinux2014_x86_64\.manylinux_2_17_x86_64\.manylinux_2_28_x86_64|manylinux2014_aarch64\.manylinux_2_17_aarch64\.manylinux_2_28_aarch64|win_amd64)\.whl$`)

func validateYamllintPin(pin ArchivePin) error {
	recipe := pin.Yamllint
	if recipe == nil || pin.Format != "file" || pin.File != "yamllint-"+pin.Version+"-py3-none-any.whl" || !pythonToolVersion.MatchString(pin.Version) || !operationID.MatchString(recipe.PythonPinID) {
		return errors.New("yamllint requires its pinned wheel and Python archive identity")
	}
	binary, directory := pin.Commands["yamllint"], "bin"
	if binary == "Scripts/yamllint.exe" {
		directory = "Scripts"
	} else if binary != "bin/yamllint" {
		return errors.New("yamllint requires its fixed private console command")
	}
	if !maps.Equal(pin.Commands, map[string]string{"yamllint": binary}) || !slices.Equal(pin.BinDirs, []string{directory}) {
		return errors.New("yamllint has an unexpected command layout")
	}
	if recipe.Pathspec.File != "pathspec-"+recipe.Pathspec.Version+"-py3-none-any.whl" {
		return errors.New("yamllint requires the pinned pathspec wheel")
	}
	match := yamllintPyYAMLWheel.FindStringSubmatch(recipe.PyYAML.File)
	if len(match) != 4 || match[1] != match[2] || !strings.HasPrefix(recipe.PyYAML.File, "pyyaml-"+recipe.PyYAML.Version+"-") {
		return errors.New("yamllint requires a supported PyYAML wheel")
	}
	for _, wheel := range []ArchivePin{recipe.Pathspec, recipe.PyYAML} {
		if !pythonToolVersion.MatchString(wheel.Version) || wheel.Format != "file" || len(wheel.Commands) != 0 || len(wheel.BinDirs) != 0 || !slices.Equal(wheel.RequiredFiles, []string{wheel.File}) ||
			len(wheel.RustComponents) != 0 || wheel.RustTarget != "" || wheel.Latex2text != nil || wheel.Yamllint != nil || wheel.PythonStdlib != "" || wheel.PortableGit || len(wheel.ExcludedFiles) != 0 || len(wheel.Replacements) != 0 {
			return errors.New("yamllint dependencies must be checksum-pinned wheels without nested preparation")
		}
		if err := wheel.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func bindYamllintPython(pin, python ArchivePin, platform string) (ArchivePin, error) {
	tags := map[string]string{
		"darwin/arm64":  "macosx_11_0_arm64",
		"linux/amd64":   "manylinux2014_x86_64.manylinux_2_17_x86_64.manylinux_2_28_x86_64",
		"linux/arm64":   "manylinux2014_aarch64.manylinux_2_17_aarch64.manylinux_2_28_aarch64",
		"windows/amd64": "win_amd64",
	}
	parts := strings.Split(python.Version, ".")
	if pin.Yamllint == nil || len(parts) < 2 || !pythonToolVersion.MatchString(python.Version) || tags[platform] == "" {
		return pin, errors.New("yamllint requires its selected supported Python archive")
	}
	abi := "cp" + parts[0] + parts[1]
	recipe := *pin.Yamllint
	if recipe.PyYAML.File != "pyyaml-"+recipe.PyYAML.Version+"-"+abi+"-"+abi+"-"+tags[platform]+".whl" {
		return pin, fmt.Errorf("yamllint PyYAML wheel does not match Python %s on %s", python.Version, platform)
	}
	recipe.PythonPinID = archivePinID(python)
	pin.Yamllint = &recipe
	return pin, nil
}

func (d *ArchiveDriver) prepareYamllint(ctx context.Context, intent archiveIntent, payload string) error {
	pin := intent.Pin
	if err := validateYamllintPin(pin); err != nil {
		return err
	}
	requirements := []privatePythonRequirement{}
	for _, dependency := range []struct {
		name string
		pin  ArchivePin
	}{{"pathspec", pin.Yamllint.Pathspec}, {"pyyaml", pin.Yamllint.PyYAML}} {
		directory := filepath.Join(payload, "."+dependency.name)
		if err := downloadArchive(ctx, d.Client, dependency.pin, directory); err != nil {
			return err
		}
		requirements = append(requirements, privatePythonRequirement{dependency.name, filepath.Join(directory, dependency.pin.File), dependency.pin.SHA256, false})
	}
	requirements = append(requirements, privatePythonRequirement{"yamllint", filepath.Join(payload, pin.File), pin.SHA256, false})
	probe := "import importlib.metadata,sys; from yamllint import linter; from yamllint.config import YamlLintConfig; assert [importlib.metadata.version(p) for p in ('yamllint','pathspec','pyyaml')] == sys.argv[1:]; assert any(p.rule == 'key-duplicates' for p in linter.run('key: 1\\nkey: 2\\n', YamlLintConfig('{extends: relaxed}')))"
	return d.preparePrivatePython(ctx, intent, payload, "yamllint", requirements, probe, pin.Version, pin.Yamllint.Pathspec.Version, pin.Yamllint.PyYAML.Version)
}
