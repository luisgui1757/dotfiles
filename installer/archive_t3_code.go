package installer

import (
	"errors"
	"os"
	"slices"
)

const t3CodeBinary = "opt/T3 Code (Beta)/t3code"

const t3CodeLauncher = `#!/bin/sh
set -eu
t3_code_root=$(CDPATH= cd -- "$(/usr/bin/dirname -- "$0")" && pwd)
T3CODE_DISABLE_AUTO_UPDATE=1
T3CODE_DESKTOP_LAUNCHER_PATH=$t3_code_root/t3-code
export T3CODE_DISABLE_AUTO_UPDATE T3CODE_DESKTOP_LAUNCHER_PATH
exec "$t3_code_root/` + t3CodeBinary + `" --no-sandbox "$@"
`

func validateT3CodeLauncherPin(pin ArchivePin) error {
	if pin.Format != "deb" || pin.StripComponents != 0 || len(pin.Commands) != 0 || len(pin.ExcludedFiles) != 0 || len(pin.Replacements) != 0 || !slices.Contains(pin.RequiredFiles, t3CodeBinary) || !slices.Contains(pin.RequiredFiles, "t3-code") {
		return errors.New("T3 Code launcher preparation requires its fixed Debian package layout")
	}
	return nil
}

func prepareT3CodeLauncher(pin ArchivePin, payload string) (result error) {
	if err := validateT3CodeLauncherPin(pin); err != nil {
		return err
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	file, err := root.OpenFile("t3-code", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	_, err = file.WriteString(t3CodeLauncher)
	return errors.Join(err, file.Sync(), file.Close())
}
