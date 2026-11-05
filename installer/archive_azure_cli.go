package installer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The POSIX archives hold only the Python packages. AzureCLIPin adds the
// interpreter that their compiled extensions require.
type AzureCLIPin struct {
	Python ArchivePin `json:"python"`
}

const azureCLIPackages = "libexec/lib/python3.14/site-packages"
const azureCLIPython = "bin/python3.14"
const azureCLICommand = "commands/az"

const azureCLILauncher = `#!/bin/sh
set -eu
az_root=$(CDPATH= cd -- "$(/usr/bin/dirname -- "$0")/.." && pwd -P)
PYTHONPATH=$az_root/` + azureCLIPackages + `
AZ_INSTALLER=TARBALL
export PYTHONPATH AZ_INSTALLER
exec "$az_root/python/` + azureCLIPython + `" -B -s -W ignore::SyntaxWarning -m azure.cli "$@"
`

func validateAzureCLIPin(pin ArchivePin) error {
	if pin.Format != "tar.gz" || pin.StripComponents != 0 || !maps.Equal(pin.Commands, map[string]string{"az": azureCLICommand}) || !slices.Equal(pin.BinDirs, []string{"commands"}) || !slices.Equal(pin.RequiredFiles, []string{azureCLIPackages + "/azure/cli/__main__.py"}) || len(pin.ExcludedFiles) != 0 || len(pin.Replacements) != 0 {
		return errors.New("Azure CLI preparation requires its official archive layout")
	}
	python := pin.AzureCLI.Python
	plain := ArchivePin{Version: python.Version, URL: python.URL, SHA256: python.SHA256, Format: "tar.gz", StripComponents: 1, Commands: map[string]string{}, BinDirs: []string{}, RequiredFiles: []string{azureCLIPython}}
	if !strings.HasPrefix(python.Version, "3.14.") || archivePinID(python) != archivePinID(plain) {
		return errors.New("Azure CLI requires its plain pinned Python 3.14 archive")
	}
	return python.Validate()
}

func (d *ArchiveDriver) prepareAzureCLI(ctx context.Context, pin ArchivePin, payload string) (result error) {
	if err := validateAzureCLIPin(pin); err != nil {
		return err
	}
	if err := downloadArchive(ctx, d.Client, pin.AzureCLI.Python, filepath.Join(payload, "python")); err != nil {
		return err
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	if err := root.Mkdir(filepath.Dir(azureCLICommand), 0755); err != nil {
		return err
	}
	file, err := root.OpenFile(azureCLICommand, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	_, err = file.WriteString(azureCLILauncher)
	if err := errors.Join(err, file.Sync(), file.Close()); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(payload, "python", azureCLIPython), "-I", "-B", "-m", "compileall", "-q", "-f", "-j", "1", "--invalidation-mode", "checked-hash", "-o", "0", "python/lib/python3.14")
	cmd.Dir = payload
	cmd.WaitDelay = 2 * time.Second
	var output boundedCommandOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("prepare verified Azure CLI bytecode: %w\n%s", err, output.data.String())
	}
	return nil
}
