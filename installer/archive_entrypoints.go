package installer

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func archivePinID(pin ArchivePin) string {
	// ArchivePin contains only JSON-serializable fields.
	id, err := digest(pin)
	if err != nil {
		panic(err)
	}
	return id
}

// A launcher recipe changes only when its reviewed revision changes. The
// operation/version still records the exact copied executable hash, and the
// payload snapshot verifies those bytes. Unrelated installer builds must not
// invalidate an already verified Rust generation.
func archiveDesiredID(pin ArchivePin) string {
	if pin.WindowsRustLauncher != nil {
		launcher := *pin.WindowsRustLauncher
		launcher.SHA256 = ""
		pin.WindowsRustLauncher = &launcher
	}
	return archivePinID(pin)
}

func (d *ArchiveDriver) currentLink(id string) string {
	return filepath.Join(d.resourceDirectory(id), "current")
}
func (d *ArchiveDriver) linkWorkspace(intent archiveIntent, kind string) string {
	return filepath.Join(d.resourceDirectory(intent.Resource), "."+kind+"-"+intent.Operation)
}

func archiveLinkTarget(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
		return "", errors.New("private command entrypoint is not a directory link")
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		return "", errors.New("private command entrypoint is not an absolute link")
	}
	return target, nil
}

// Every move is no-replace, with intent already durable. The next run derives
// progress from these three link targets; it cannot overwrite an outside link.
func (d *ArchiveDriver) switchLink(intent archiveIntent, remove bool) error {
	current := d.currentLink(intent.Resource)
	previous := d.linkWorkspace(intent, "previous")
	next := d.linkWorkspace(intent, "next")
	want := filepath.Join(d.versionDirectory(intent.Resource, intent.Generation), "payload")
	actual, err := archiveLinkTarget(current)
	if err != nil {
		return err
	}
	saved, err := archiveLinkTarget(previous)
	if err != nil {
		return err
	}
	if saved != "" && saved != intent.PreviousTarget {
		return errors.New("saved command entrypoint changed during publication")
	}
	if !remove && actual == want {
		return nil
	}
	if actual != "" && actual != intent.PreviousTarget {
		return errors.New("command entrypoint changed after approval")
	}
	if actual != "" {
		if saved != "" {
			return errors.New("both original command entrypoints exist; preserve them for recovery")
		}
		if err := moveConfigExclusive(current, previous); err != nil {
			return err
		}
	} else if intent.PreviousTarget != "" && saved == "" {
		return errors.New("original command entrypoint disappeared; preserve the package for inspection")
	}
	if remove {
		return nil
	}
	staged, err := archiveLinkTarget(next)
	if err != nil {
		return err
	}
	if staged == "" {
		if err := createDirectoryLink(want, next); err != nil {
			return err
		}
		if err := syncConfigDirectory(filepath.Dir(next)); err != nil {
			return err
		}
	} else if staged != want {
		return errors.New("staged command entrypoint changed")
	}
	return moveConfigExclusive(next, current)
}

func (d *ArchiveDriver) cleanLinks(id string) ([]string, error) {
	directory := d.resourceDirectory(id)
	files, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	preserved := []string{}
	for _, file := range files {
		kind, operation := "", ""
		for _, prefix := range []string{"previous", "next"} {
			if suffix, ok := strings.CutPrefix(file.Name(), "."+prefix+"-"); ok {
				kind, operation = prefix, suffix
				break
			}
		}
		if !operationID.MatchString(operation) {
			continue
		}
		path := filepath.Join(directory, file.Name())
		intent, err := d.readIntent(id, operation)
		if err != nil {
			preserved = append(preserved, path, d.intentPath(id, operation))
			continue
		}
		want := intent.PreviousTarget
		if kind == "next" {
			want = filepath.Join(d.versionDirectory(id, intent.Generation), "payload")
		}
		actual, err := archiveLinkTarget(path)
		if err != nil || actual != "" && actual != want {
			preserved = append(preserved, path)
			continue
		}
		if actual == "" {
			continue
		}
		// Removing a link never recursively deletes its package referent.
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	return preserved, nil
}

func (d *ArchiveDriver) CommandPath(id, command string) (string, error) {
	pin, err := d.validate(id)
	if err != nil {
		return "", err
	}
	name, ok := pin.Commands[command]
	if !ok {
		return "", errors.New("unknown private package command")
	}
	return filepath.Join(d.currentLink(id), filepath.FromSlash(name)), nil
}

func (d *ArchiveDriver) BinaryDirectories(id string) ([]string, error) {
	pin, err := d.validate(id)
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(pin.BinDirs))
	for i, dir := range pin.BinDirs {
		paths[i] = filepath.Join(d.currentLink(id), filepath.FromSlash(dir))
	}
	return paths, nil
}

// RequiredFilePath exposes only a declared, verified non-command entry file.
func (d *ArchiveDriver) RequiredFilePath(id, name string) (string, error) {
	pin, err := d.validate(id)
	if err != nil {
		return "", err
	}
	if !slices.Contains(pin.RequiredFiles, name) {
		return "", errors.New("unknown private package entry file")
	}
	return filepath.Join(d.currentLink(id), filepath.FromSlash(name)), nil
}
