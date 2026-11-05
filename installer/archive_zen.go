package installer

import (
	"errors"
	"maps"
	"os"
	"slices"
)

func validateZenPin(pin ArchivePin) error {
	if pin.ZenProfileRevision != 1 {
		return errors.New("unsupported Zen preparation revision")
	}
	if pin.Format != "tar.xz" || pin.StripComponents != 1 || !maps.Equal(pin.Commands, map[string]string{"zen": "zen"}) || !slices.Equal(pin.BinDirs, []string{"."}) || !slices.Contains(pin.RequiredFiles, "zen-bin") {
		return errors.New("Zen preparation requires its official Linux archive layout")
	}
	return nil
}

// Mozilla explicitly supports MOZ_LEGACY_PROFILES for package managers whose
// installation path changes with each version (nsToolkitProfileService.cpp).
// Keep all personal profiles outside the payload and pass every CLI/desktop
// argument unchanged. The signed upstream executable remains byte-identical.
const zenLauncher = `#!/bin/sh
set -eu
zen_dir=$(CDPATH= cd -- "$(/usr/bin/dirname -- "$0")" && pwd -P)
export MOZ_LEGACY_PROFILES=1
exec "$zen_dir/zen-bin" "$@"
`

func prepareZen(pin ArchivePin, payload string) (result error) {
	if err := validateZenPin(pin); err != nil {
		return err
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	// The two upstream executable entrypoints contain identical verified bytes.
	// Replacing only the public launcher preserves zen-bin and libxul.
	if err := root.Remove("zen"); err != nil {
		return err
	}
	file, err := root.OpenFile("zen", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	_, err = file.WriteString(zenLauncher)
	if err := errors.Join(err, file.Sync(), file.Close()); err != nil {
		return err
	}
	// nsUpdateMutex uses nsProfileLock on the installation directory. Its
	// zero-byte lock is opened with O_TRUNC and never contains personal data.
	// Prepare it before sealing the generation: checks still cover its bytes,
	// type and mode, instead of ignoring a runtime-created file afterwards.
	lock, err := root.OpenFile(".parentlock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := errors.Join(lock.Sync(), lock.Close()); err != nil {
		return err
	}
	// The updater writes inside Linux installation directories even when
	// no new version is available. This managed installation is updated by
	// the installer; do not weaken its complete payload fingerprint.
	if err := root.Mkdir("distribution", 0755); err != nil {
		return err
	}
	return root.WriteFile("distribution/policies.json", []byte("{\"policies\":{\"DisableAppUpdate\":true}}\n"), 0644)
}
