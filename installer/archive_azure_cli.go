package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// AzureCLIPin adds the Python interpreter that the compiled extensions of
// the POSIX Azure CLI archives require.
type AzureCLIPin struct {
	Python ArchivePin `json:"python"`
}

const azureCLIPackages = "libexec/lib/python3.14/site-packages"
const azureCLIPython = "bin/python3.14"
const azureCLICommand = "commands/az"
const azureCLIMain = azureCLIPackages + "/azure/cli/__main__.py"

const azureCLILauncher = `#!/bin/sh
set -eu
az_root=$(CDPATH= cd -- "$(/usr/bin/dirname -- "$0")/.." && pwd -P)
PYTHONPATH=$az_root/` + azureCLIPackages + `
AZ_INSTALLER=TARBALL
export PYTHONPATH AZ_INSTALLER
exec "$az_root/python/` + azureCLIPython + `" -B -s -P -m azure.cli "$@"
`

func validateAzureCLIPin(pin ArchivePin) error {
	if pin.Format != "tar.gz" || pin.StripComponents != 0 || !maps.Equal(pin.Commands, map[string]string{"az": azureCLICommand}) || !slices.Equal(pin.BinDirs, []string{"commands"}) || !slices.Equal(pin.RequiredFiles, []string{azureCLIMain + "c"}) || len(pin.ExcludedFiles) != 0 || len(pin.Replacements) != 0 {
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for _, args := range [][]string{
		{"-b", "--invalidation-mode", "unchecked-hash", azureCLIPackages},
		{"--invalidation-mode", "checked-hash", "python/lib/python3.14"},
	} {
		cmd := exec.CommandContext(ctx, filepath.Join(payload, "python", azureCLIPython), append([]string{"-I", "-B", "-W", "ignore", "-m", "compileall", "-q", "-f", "-j", "1", "-o", "0"}, args...)...)
		cmd.Dir = payload
		cmd.WaitDelay = 2 * time.Second
		var output boundedCommandOutput
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("prepare verified Azure CLI bytecode: %w\n%s", err, output.data.String())
		}
	}
	return fs.WalkDir(root.FS(), azureCLIPackages, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() || filepath.Ext(name) != ".py" {
			return err
		}
		if info, err := root.Lstat(name + "c"); err != nil || !info.Mode().IsRegular() {
			return errors.Join(fmt.Errorf("Azure CLI module %s has no bytecode", name), err)
		}
		return root.Remove(name)
	})
}
