package installer

import (
	"errors"
	"os"
	"slices"
)

func validateGhosttyLibrariesPin(pin ArchivePin) error {
	if pin.Format != "deb" || pin.StripComponents != 0 || len(pin.Commands) != 1 || pin.Commands["ghostty"] != "usr/bin/ghostty" || !slices.Equal(pin.BinDirs, []string{"usr/bin"}) || len(pin.ExcludedFiles) != 0 || len(pin.Replacements) != 0 {
		return errors.New("Ghostty library preparation requires its fixed Debian package layout")
	}
	for _, name := range []string{"usr/bin/ghostty-bin", "usr/lib/libgtk4-layer-shell.so", "usr/share/terminfo/x/xterm-ghostty", "usr/share/ghostty/themes/Rose Pine"} {
		if !slices.Contains(pin.RequiredFiles, name) {
			return errors.New("Ghostty library preparation lacks its required private runtime")
		}
	}
	return nil
}

// The Trixie package bundles layer-shell but retains a build-directory RUNPATH.
// Keep the upstream ELF bytes intact and supply its adjacent library directory
// for both CLI and desktop launches. Resources remain discoverable in usr/share.
const ghosttyLibraryLauncher = `#!/bin/sh
set -eu
ghostty_bin_dir=$(CDPATH= cd -- "$(/usr/bin/dirname -- "$0")" && pwd -P)
LD_LIBRARY_PATH="$ghostty_bin_dir/../lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export LD_LIBRARY_PATH
exec "$ghostty_bin_dir/ghostty-bin" "$@"
`

func prepareGhosttyLibraries(pin ArchivePin, payload string) (result error) {
	if err := validateGhosttyLibrariesPin(pin); err != nil {
		return err
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	if _, err := root.Lstat("usr/bin/ghostty-bin"); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(errors.New("Ghostty prepared binary already exists"), err)
	}
	if err := root.Rename("usr/bin/ghostty", "usr/bin/ghostty-bin"); err != nil {
		return err
	}
	file, err := root.OpenFile("usr/bin/ghostty", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	_, err = file.WriteString(ghosttyLibraryLauncher)
	return errors.Join(err, file.Sync(), file.Close())
}
