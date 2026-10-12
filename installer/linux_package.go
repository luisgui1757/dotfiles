package installer

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// LinuxPackagePin supplies a reviewed vendor DEB to the existing APT lifecycle.
// Package dependencies, service scripts and removal remain native dpkg work.
type LinuxPackagePin struct {
	Package      string `json:"package"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

//go:embed linux-package-pins.json
var linuxPackagePinData []byte

var linuxPackageVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func (p LinuxPackagePin) validate() error {
	if !linuxPackageVersion.MatchString(p.Version) || !slices.Contains([]string{"amd64", "arm64"}, p.Architecture) || !operationID.MatchString(p.SHA256) || p.Size <= 0 || p.Size > maxArchiveBytes {
		return errors.New("native application requires a fixed version, architecture, size and SHA-256")
	}
	u, err := url.Parse(p.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid vendor package URL")
	}
	want := ""
	switch p.Package {
	case "tailscale":
		want = "https://pkgs.tailscale.com/stable/tailscale_" + p.Version + "_" + p.Architecture + ".deb"
	default:
		return errors.New("unsupported fixed vendor package")
	}
	if p.URL != want {
		return errors.New("native package does not match its official release URL")
	}
	return nil
}

func defaultLinuxPackagePins(arch string) (map[string]LinuxPackagePin, error) {
	var data struct {
		Schema    int                                   `json:"schema"`
		Resources map[string]map[string]LinuxPackagePin `json:"resources"`
	}
	if err := Decode(linuxPackagePinData, &data); err != nil {
		return nil, err
	}
	if data.Schema != 1 || len(data.Resources) != 1 {
		return nil, errors.New("invalid native application pin catalog")
	}
	result := map[string]LinuxPackagePin{}
	for id, pins := range data.Resources {
		for target, pin := range pins {
			resource := "tool." + pin.Package
			if err := pin.validate(); err != nil || id != resource || target != pin.Architecture {
				return nil, errors.Join(errors.New("invalid native application platform pin"), err)
			}
			if target == arch {
				result[id] = pin
			}
		}
	}
	return result, nil
}

func (d *LinuxAPTDriver) packagePayload(pin LinuxPackagePin) string {
	return filepath.Join(d.Directory, "payloads", pin.SHA256, pin.Package+".deb")
}

func (d *LinuxAPTDriver) verifyPackagePayload(pin LinuxPackagePin) error {
	if err := pin.validate(); err != nil {
		return err
	}
	file := d.packagePayload(pin)
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("native package input was redirected"), err)
	}
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() != pin.Size {
		return errors.Join(errors.New("native package input is missing or changed"), err)
	}
	handle, err := os.Open(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, io.LimitReader(handle, pin.Size+1))
	if err := errors.Join(readErr, handle.Close()); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != pin.SHA256 {
		return errors.New("native package differs from its saved SHA-256")
	}
	return nil
}

func (d *LinuxAPTDriver) preparePackage(ctx context.Context, pin LinuxPackagePin) (result error) {
	if err := pin.validate(); err != nil {
		return err
	}
	file := d.packagePayload(pin)
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("native package input was redirected"), err)
	}
	if _, err := os.Lstat(file); errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(filepath.Dir(file))
		if err := os.MkdirAll(parent, 0700); err != nil {
			return err
		}
		stage, err := os.MkdirTemp(parent, ".vendor-download-")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
		archive := ArchivePin{Version: pin.Version, URL: pin.URL, SHA256: pin.SHA256, Format: "file", File: pin.Package + ".deb", RequiredFiles: []string{pin.Package + ".deb"}}
		if err := downloadArchive(ctx, d.Client, archive, filepath.Join(stage, "payload")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, "payload"), filepath.Dir(file)); err != nil {
			return err
		}
	}
	return d.verifyPackagePayload(pin)
}

func (d *LinuxAPTDriver) cleanupPackagePayload(pin LinuxPackagePin) error {
	file := d.packagePayload(pin)
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("native package input was redirected"), err)
	}
	entries, err := os.ReadDir(filepath.Dir(file))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 1 || len(entries) == 1 && entries[0].Name() != pin.Package+".deb" {
		return errors.New("unexpected native package input is preserved")
	}
	if len(entries) == 1 {
		if err := d.verifyPackagePayload(pin); err != nil {
			return err
		}
		if err := os.Remove(file); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Dir(file))
}

// These include vendor-script publications that dpkg --verify cannot inspect.
func linuxPackagePublications(name string) []string {
	switch name {
	case "tailscale":
		return []string{"/usr/bin/tailscale", "/usr/sbin/tailscaled", "/lib/systemd/system/tailscaled.service", "/usr/lib/systemd/system/tailscaled.service"}
	}
	return nil
}

func (d *LinuxAPTDriver) checkPackagePublications(ctx context.Context, name string, absent bool) error {
	var script string
	if absent {
		script = "for p in"
		for _, path := range linuxPackagePublications(name) {
			script += " " + shellLiteral(path)
		}
		script += "; do if [ -e \"$p\" ] || [ -L \"$p\" ]; then printf '%s\\n' \"$p\"; fi; done"
	} else {
		return nil // This vendor's entrypoints are package files, checked by dpkg.
	}
	output, err := d.Query(ctx, false, "/bin/sh", nil, "-c", script)
	if err != nil || strings.TrimSpace(string(output)) != "" {
		return errors.Join(fmt.Errorf("%s has a conflicting or changed native publication; preserve it%s", name, nativeDiagnostic(output)), err)
	}
	return nil
}

func linuxPackageApprovedDowngrade(intent linuxAPTIntent) string {
	if intent.Deb == nil {
		return linuxCodeApprovedDowngrade(intent)
	}
	if intent.Action != "update" && intent.Action != "repair" {
		return ""
	}
	name := intent.Deb.Package + ":" + intent.Deb.Architecture
	before, present := intent.Before[name]
	if !present || before.Version == intent.Deb.Version {
		return ""
	}
	return name + " (" + before.Version + ", " + intent.Deb.Version + ")"
}
