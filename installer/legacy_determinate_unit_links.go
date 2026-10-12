package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// systemd resolves a linked unit into the store when enable is given its /etc
// path. Accept that exact regular unit, never an arbitrary store-path pattern.
func (d *legacyRetirementDriver) nixActivationTarget(unit, target string) (bool, error) {
	sources := []string{d.sys("/etc/systemd/system/" + unit), d.sys("/nix/var/nix/profiles/default/lib/systemd/system/" + unit)}
	for _, source := range sources {
		if target == source {
			return true, nil
		}
	}
	if !strings.HasPrefix(target, d.sys("/nix/store")+string(filepath.Separator)) || filepath.Base(target) != unit || filepath.Clean(target) != target {
		return false, nil
	}
	for _, source := range sources {
		resolved, err := filepath.EvalSymlinks(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if resolved == target {
			info, err := os.Stat(resolved)
			if err != nil {
				return false, err
			}
			return info.Mode().IsRegular(), nil
		}
	}
	return false, nil
}

func (d *legacyRetirementDriver) nixUnitLinkAdvice(path, target string) string {
	unit := filepath.Base(path)
	activation := filepath.Dir(path) == d.sys("/etc/systemd/system/sockets.target.wants") || filepath.Dir(path) == d.sys("/etc/systemd/system/multi-user.target.wants")
	if activation && strings.HasPrefix(target, d.sys("/nix/store")+string(filepath.Separator)) && filepath.Clean(target) == target && filepath.Base(target) == unit {
		return "preserve and inspect it manually; after confirming this is the old Nix activation link, run sudo rm -- " + path + " and then retry migrate"
	}
	return "preserve and inspect it manually"
}

// Resolve-dependent evidence must retire before its source, including during
// recovery if an administrator re-enabled a unit over the remaining store.
// Callers have already validated every link through linuxNixUnitLinks.
func (d *legacyRetirementDriver) planResolvedNixActivationLinks(scope *legacyRetirementScope, links []string) error {
	for _, path := range links {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(target, d.sys("/nix/store")+string(filepath.Separator)) {
			if err := addLegacyRemoval(scope, "Remove the Nix systemd link", path); err != nil {
				return err
			}
		}
	}
	return nil
}

// systemctl enable creates these links for the receipt's service and socket units. If
// a partial vendor revert already removed a unit, disable may leave its link.
// Record the link before approval and retire only those exact bytes afterwards.
func legacyDeterminateUnitLinks(r legacyDeterminateReceipt) []string {
	var paths []string
	if r.Init == "Systemd" {
		for _, unit := range r.Units {
			switch {
			case strings.HasSuffix(unit, ".socket"):
				paths = append(paths, "/etc/systemd/system/sockets.target.wants/"+unit)
			case unit == "nix-daemon.service":
				paths = append(paths, "/etc/systemd/system/multi-user.target.wants/"+unit)
			}
		}
	}
	return paths
}

func (d *legacyRetirementDriver) determinateUnitFiles(r legacyDeterminateReceipt) ([]legacyRetirementFile, error) {
	var files []legacyRetirementFile
	for _, path := range legacyDeterminateUnitLinks(r) {
		path = d.sys(path)
		file, err := legacyFile(path)
		if err != nil {
			return nil, err
		}
		if file.Snapshot.Kind != "absent" {
			target, err := os.Readlink(path)
			unit := filepath.Base(path)
			if err != nil {
				return nil, fmt.Errorf("%s is not the receipt's unit activation link; preserve and inspect it manually: %w", path, err)
			}
			accepted, err := d.nixActivationTarget(unit, target)
			if err != nil {
				return nil, err
			}
			if !accepted {
				return nil, fmt.Errorf("%s is not the receipt's unit activation link; %s", path, d.nixUnitLinkAdvice(path, target))
			}
		}
		files = append(files, file)
	}
	return files, nil
}
