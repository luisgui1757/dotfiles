package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// The catalog declares the recipe revision. The process boundary supplies the
// installed executable's hash, binding preparation and the generation's provenance.
// An unbound catalog template is readable but cannot prepare a Windows toolchain.
type WindowsRustLauncherPin struct {
	Revision int    `json:"revision"`
	SHA256   string `json:"sha256,omitempty"`
}

func validateWindowsRustLauncherPin(pin ArchivePin) error {
	launcher := pin.WindowsRustLauncher
	if launcher != nil && (pin.RustTarget != "x86_64-pc-windows-msvc" || len(pin.RustComponents) == 0 || launcher.Revision != 1 || launcher.SHA256 != "" && !operationID.MatchString(launcher.SHA256)) {
		return errors.New("Windows Rust launcher requires its fixed recipe and executable SHA-256")
	}
	return nil
}

// Verify once before download/preparation, then copy these same verified bytes.
// Historical raw Windows toolchains remain readable for check/update/removal;
// an unfinished old preparation cannot silently change its executable recipe.
func (d *ArchiveDriver) readWindowsRustLauncher(pin ArchivePin) ([]byte, error) {
	launcher := pin.WindowsRustLauncher
	if launcher == nil {
		return nil, errors.New("saved Windows Rust preparation predates long-path launch support; preserve the original operation and payload for explicit recovery")
	}
	if err := validateWindowsRustLauncherPin(pin); err != nil {
		return nil, err
	}
	if !operationID.MatchString(launcher.SHA256) || !filepath.IsAbs(d.WindowsRustLauncher) || filepath.Clean(d.WindowsRustLauncher) != d.WindowsRustLauncher {
		return nil, errors.New("Windows Rust preparation requires the observed installer executable and its SHA-256")
	}
	f, err := os.Open(d.WindowsRustLauncher)
	if err != nil {
		return nil, err
	}
	info, statErr := f.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return nil, errors.Join(errors.New("invalid Rust launcher executable"), statErr, f.Close())
	}
	data, readErr := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if len(data) > 64<<20 || hex.EncodeToString(hash[:]) != launcher.SHA256 {
		return nil, errors.New("installer executable changed after Windows Rust launcher approval")
	}
	return data, nil
}

func prepareWindowsRustLaunchers(payload string, launcher []byte) error {
	for _, name := range []string{"rustc.exe", "rustdoc.exe", "clippy-driver.exe"} {
		command := filepath.Join(payload, "bin", name)
		info, err := os.Lstat(command)
		if err != nil || !info.Mode().IsRegular() {
			return errors.Join(errors.New("Windows Rust lacks a required compiler executable"), err)
		}
		if err := moveConfigExclusive(command, filepath.Join(payload, "bin", rustOriginalCommand(name))); err != nil {
			return err
		}
		file, err := os.OpenFile(command, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(launcher)
		if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
			return err
		}
	}
	return nil
}
