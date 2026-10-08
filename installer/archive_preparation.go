package installer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// These fixed preparations use the archive
// provider's existing generation, publication and ownership lifecycle.
func validateArchivePreparation(pin ArchivePin) error {
	if err := validateWindowsRustLauncherPin(pin); err != nil {
		return err
	}
	if !pin.PortableGit && pin.PortableGitRevision != 0 {
		return errors.New("PortableGit recipe revision requires its fixed preparation")
	}
	preparations := 0
	for _, enabled := range []bool{len(pin.RustComponents) > 0, pin.Latex2text != nil, pin.Yamllint != nil, pin.PythonStdlib != "", pin.PortableGit, pin.GhosttyLibraries} {
		if enabled {
			preparations++
		}
	}
	if preparations > 1 {
		return errors.New("archive preparations cannot be combined")
	}
	if len(pin.RustComponents) > 0 {
		if !slices.Contains([]string{"aarch64-apple-darwin", "x86_64-unknown-linux-gnu", "aarch64-unknown-linux-gnu", "x86_64-pc-windows-msvc"}, pin.RustTarget) {
			return errors.New("unsupported Rust toolchain target")
		}
		suffix := ""
		if pin.RustTarget == "x86_64-pc-windows-msvc" {
			suffix = ".exe"
		}
		commands := map[string]string{}
		for _, name := range []string{"rustc", "cargo", "rustfmt", "cargo-fmt", "cargo-clippy", "clippy-driver"} {
			commands[name] = "bin/" + name + suffix
		}
		if len(pin.RustComponents) != 5 || pin.Format != "tar.gz" || pin.StripComponents != 1 || !maps.Equal(pin.Commands, commands) || !slices.Equal(pin.BinDirs, []string{"bin"}) || !slices.Contains(pin.RequiredFiles, "lib/rustlib/src/rust/library/core/src/lib.rs") {
			return errors.New("Rust preparation requires the fixed component layout")
		}
		expected := rustComponentNames(pin.RustTarget)
		for index, component := range pin.RustComponents {
			if len(component.RustComponents) != 0 || component.RustTarget != "" || component.Latex2text != nil || component.Yamllint != nil || component.PythonStdlib != "" || component.PortableGit || component.GhosttyLibraries {
				return errors.New("nested archive preparations are forbidden")
			}
			if component.Version != pin.Version || component.Format != "tar.gz" || component.StripComponents != 1 || len(component.Commands) != 0 || !slices.Equal(component.RequiredFiles, []string{expected[index] + "/manifest.in"}) || len(component.ExcludedFiles) != 0 || len(component.Replacements) != 0 {
				return fmt.Errorf("invalid fixed Rust component %s", expected[index])
			}
			if err := component.Validate(); err != nil {
				return err
			}
		}
	}
	if len(pin.RustComponents) == 0 && pin.RustTarget != "" {
		return errors.New("Rust target requires its complete component set")
	}
	if pin.Latex2text != nil {
		return validateLatex2textPin(pin)
	}
	if pin.Yamllint != nil {
		return validateYamllintPin(pin)
	}
	if pin.PortableGit {
		return validatePortableGitPin(pin)
	}
	if pin.GhosttyLibraries {
		return validateGhosttyLibrariesPin(pin)
	}
	return nil
}

func (d *ArchiveDriver) preparePayload(ctx context.Context, intent archiveIntent, payload string) (result error) {
	pin := intent.Pin
	var launcher []byte
	if pin.RustTarget == "x86_64-pc-windows-msvc" {
		var err error
		launcher, err = d.readWindowsRustLauncher(pin)
		if err != nil {
			return err
		}
	}
	source := pin
	source.RustComponents, source.Latex2text = nil, nil
	source.Yamllint = nil
	source.RustTarget = ""
	source.WindowsRustLauncher = nil
	source.PortableGit = false
	source.PortableGitRevision = 0
	source.GhosttyLibraries = false
	if pin.GhosttyLibraries {
		source.RequiredFiles = slices.DeleteFunc(slices.Clone(source.RequiredFiles), func(name string) bool { return name == "usr/bin/ghostty-bin" })
	}
	if len(pin.RustComponents) > 0 {
		source.Commands = nil
		source.RequiredFiles = []string{"rustc/manifest.in", "rustc/" + pin.Commands["rustc"]}
	} else if pin.Latex2text != nil || pin.Yamllint != nil || pin.PortableGit {
		source.Commands = nil
		source.RequiredFiles = []string{pin.File}
	}
	if err := downloadArchiveWithDeb(ctx, d.Client, source, payload, d.DpkgDeb); err != nil {
		return err
	}
	switch {
	case len(pin.RustComponents) > 0:
		if err := d.prepareRustArchive(ctx, pin, payload); err != nil {
			return err
		}
		if launcher != nil {
			if err := prepareWindowsRustLaunchers(payload, launcher); err != nil {
				return err
			}
		}
	case pin.Latex2text != nil:
		if err := d.prepareLatex2text(ctx, intent, payload); err != nil {
			return err
		}
	case pin.Yamllint != nil:
		if err := d.prepareYamllint(ctx, intent, payload); err != nil {
			return err
		}
	case pin.PortableGit:
		if err := d.preparePortableGit(ctx, intent, payload); err != nil {
			return err
		}
	case pin.GhosttyLibraries:
		if err := prepareGhosttyLibraries(pin, payload); err != nil {
			return err
		}
	default:
		return preparePythonArchive(ctx, pin, payload)
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	if err := checkArchivePayload(root, pin); err != nil {
		return err
	}
	return syncConfigDirectory(payload)
}

func checkArchivePayload(root *os.Root, pin ArchivePin) error {
	for _, name := range pin.RequiredFiles {
		info, err := root.Stat(filepath.FromSlash(name))
		if err != nil || !info.Mode().IsRegular() {
			return errors.Join(fmt.Errorf("archive lacks required file %s", name), err)
		}
	}
	for _, name := range pin.Commands {
		info, err := root.Stat(filepath.FromSlash(name))
		if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
			return errors.Join(fmt.Errorf("archive lacks executable %s", name), err)
		}
	}
	return nil
}
