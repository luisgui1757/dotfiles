package installer

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
)

func validateZenPin(pin ArchivePin) error {
	if pin.ZenProfileRevision < 0 || pin.ZenProfileRevision > 2 {
		return errors.New("unsupported Zen preparation revision")
	}
	if pin.Format == "dmg" && pin.AppBundle == "Zen.app" && pin.StripComponents == 0 && len(pin.Commands) == 0 && len(pin.BinDirs) == 0 && slices.Contains(pin.RequiredFiles, "Zen.app/Contents/MacOS/zen") && slices.Contains(pin.RequiredFiles, "DotfilesZen.app/Contents/Info.plist") {
		return nil
	}
	if pin.ZenProfileRevision > 1 || pin.Format != "tar.xz" || pin.StripComponents != 1 || !maps.Equal(pin.Commands, map[string]string{"zen": "zen"}) || !slices.Equal(pin.BinDirs, []string{"."}) || !slices.Contains(pin.RequiredFiles, "zen-bin") {
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
	if pin.Format == "dmg" {
		return prepareMacZen(root, pin.ZenProfileRevision)
	}
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
	if pin.ZenProfileRevision == 1 {
		// The updater writes inside Linux installation directories even when
		// no new version is available. This managed installation is updated by
		// the installer; do not weaken its complete payload fingerprint.
		if err := root.Mkdir("distribution", 0755); err != nil {
			return err
		}
		return root.WriteFile("distribution/policies.json", []byte("{\"policies\":{\"DisableAppUpdate\":true}}\n"), 0644)
	}
	return nil
}

// Publish an outer launcher bundle without changing the signed vendor bundle.
// Its original metadata/resources preserve LaunchServices URL handling and the
// icon; only the outer executable supplies the supported profile environment.
func prepareMacZen(root *os.Root, revision int) error {
	for _, name := range []string{"DotfilesZen.app", "DotfilesZen.app/Contents", "DotfilesZen.app/Contents/MacOS"} {
		if err := root.Mkdir(name, 0755); err != nil {
			return err
		}
	}
	info, err := root.ReadFile("Zen.app/Contents/Info.plist")
	if err != nil {
		return err
	}
	if revision == 2 {
		info, err = macZenLauncherInfo(info)
		if err != nil {
			return err
		}
	}
	if err := root.WriteFile("DotfilesZen.app/Contents/Info.plist", info, 0644); err != nil {
		return err
	}
	if err := root.Symlink("../../Zen.app/Contents/Resources", "DotfilesZen.app/Contents/Resources"); err != nil {
		return err
	}
	launcher := strings.Replace(zenLauncher, `/usr/bin/dirname --`, `/usr/bin/dirname`, 1)
	if revision == 0 {
		// Saved preparations retain their original bytes for exact recovery.
		launcher = strings.Replace(launcher, `$zen_dir/zen-bin`, `$zen_dir/../../../Zen.app/Contents/MacOS/zen`, 1)
	} else {
		// Mozilla's macOS BinaryPath::Get strips "./" substrings, including
		// those inside "../". Resolve the vendor directory before exec so its
		// XPCOM lookup sees the canonical bundle path. The subshell preserves
		// the caller's working directory and relative file/URL arguments.
		launcher = strings.Replace(launcher, "export MOZ_LEGACY_PROFILES=1", "zen_dir=$(CDPATH= cd -- \"$zen_dir/../../../Zen.app/Contents/MacOS\" && pwd -P)\nexport MOZ_LEGACY_PROFILES=1", 1)
		launcher = strings.Replace(launcher, `$zen_dir/zen-bin`, `$zen_dir/zen`, 1)
	}
	return root.WriteFile("DotfilesZen.app/Contents/MacOS/zen", []byte(launcher), 0755)
}

// Apple launches script-only bundles under Rosetta unless their architecture is
// declared. Change only our outer launcher's plist; keep the signed vendor app
// intact and every unrelated metadata byte. This installer supports arm64 only.
func macZenLauncherInfo(info []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(info))
	depth, insertion, roots := 0, -1, 0
	for {
		offset := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				roots++
				if roots != 1 {
					return nil, errors.New("Zen Info.plist must contain one document")
				}
			}
			if (depth == 1 && value.Name.Local != "plist") || (depth == 2 && value.Name.Local != "dict") {
				return nil, errors.New("unexpected Zen Info.plist root")
			}
			if depth == 3 && value.Name.Local == "key" {
				var key string
				if err := decoder.DecodeElement(&key, &value); err != nil {
					return nil, err
				}
				depth--
				if key == "LSArchitecturePriority" {
					return nil, errors.New("Zen vendor architecture metadata changed; review launcher preparation")
				}
			}
		case xml.EndElement:
			if depth == 2 {
				if insertion != -1 || !bytes.HasPrefix(info[offset:], []byte("</dict")) {
					return nil, errors.New("Zen Info.plist must contain one root dictionary")
				}
				insertion = offset
			}
			depth--
		}
	}
	if insertion == -1 {
		return nil, errors.New("Zen Info.plist has no root dictionary")
	}
	architecture := []byte("<key>LSArchitecturePriority</key><array><string>arm64</string></array>\n")
	return bytes.Join([][]byte{info[:insertion], architecture, info[insertion:]}, nil), nil
}
