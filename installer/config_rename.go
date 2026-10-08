package installer

import (
	"errors"
	"os"
	"path/filepath"
)

// Configuration publication must never replace a target that appeared after
// review. Both directories are pinned for the native no-replace rename; moving
// the old target first preserves its native ACLs and link identity.
func moveConfigExclusive(from, to string) (result error) {
	left, err := openConfigParent(from)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, left.Close()) }()
	right, err := openConfigParent(to)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, right.Close()) }()
	if err := renameConfigNoReplace(left, filepath.Base(from), right, filepath.Base(to)); err != nil {
		return err
	}
	return errors.Join(syncConfigHandle(left), syncConfigHandle(right))
}

func openConfigParent(path string) (*os.File, error) {
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return nil, errors.Join(errors.New("configuration parent changed; preserve recovery files"), err)
	}
	parent := filepath.Dir(path)
	before, err := os.Lstat(parent)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(errors.New("configuration parent is not a directory"), err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	file, err := root.Open(".")
	if err = errors.Join(err, root.Close()); err != nil {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		return nil, err
	}
	opened, statErr := file.Stat()
	after, pathErr := os.Lstat(parent)
	if statErr != nil || pathErr != nil || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("configuration parent changed while opening"), statErr, pathErr, file.Close())
	}
	return file, nil
}
