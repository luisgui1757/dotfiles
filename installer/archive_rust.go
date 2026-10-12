package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func rustComponentNames(target string) []string {
	return []string{"cargo", "rust-std-" + target, "rustfmt-preview", "clippy-preview", "rust-src"}
}

func (d *ArchiveDriver) prepareRustArchive(ctx context.Context, pin ArchivePin, payload string) error {
	if err := validateArchivePreparation(pin); err != nil {
		return err
	}
	components := filepath.Join(payload, ".rust-components")
	if err := os.Mkdir(components, 0700); err != nil {
		return err
	}
	if err := mergeRustComponent(ctx, filepath.Join(payload, "rustc"), payload); err != nil {
		return err
	}
	names := rustComponentNames(pin.RustTarget)
	for index, component := range pin.RustComponents {
		source := filepath.Join(components, names[index])
		if err := downloadArchive(ctx, d.Client, component, source); err != nil {
			return err
		}
		if err := mergeRustComponent(ctx, filepath.Join(source, names[index]), payload); err != nil {
			return err
		}
	}
	for _, pattern := range []string{"libstd-*.rlib", "libcore-*.rlib"} {
		standard, err := filepath.Glob(filepath.Join(payload, "lib", "rustlib", pin.RustTarget, "lib", pattern))
		if err != nil || len(standard) != 1 {
			return errors.Join(errors.New("Rust standard library is incomplete"), err)
		}
	}
	return nil
}

// Component manifests remain as provenance. Only the fixed distribution
// payload subtrees are merged; scripts and unrelated files are never run.
func mergeRustComponent(ctx context.Context, source, payload string) error {
	for _, name := range []string{"bin", "lib", "libexec", "share", "etc"} {
		from := filepath.Join(source, name)
		if _, err := os.Lstat(from); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := mergeRustDirectory(ctx, from, filepath.Join(payload, name)); err != nil {
			return err
		}
	}
	return nil
}

// Merge only verified, newly extracted component directories. Existing leaves
// are collisions and are never replaced, even if their bytes happen to match.
func mergeRustDirectory(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := readPlainDirectory(source)
	if err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if _, err := readPlainDirectory(destination); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		from, to := filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			if err := mergeRustDirectory(ctx, from, to); err != nil {
				return err
			}
		} else {
			if err := moveConfigExclusive(from, to); err != nil {
				return fmt.Errorf("Rust component collision at %s: %w", to, err)
			}
		}
	}
	return os.Remove(source)
}
