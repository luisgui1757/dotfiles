package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type settingsLockOwner struct{ Directory, Operation, Target string }

const settingsLockMarker = ".dotfiles-owner.json"

func readSettingsLockOwner(path string) (settingsLockOwner, os.FileInfo, error) {
	var owner settingsLockOwner
	info, err := os.Lstat(path)
	if err != nil {
		return owner, nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return owner, nil, errors.New("settings lock is not an ordinary directory")
	}
	data, err := readDocument(filepath.Join(path, settingsLockMarker))
	if err == nil {
		err = Decode(data, &owner)
	}
	return owner, info, err
}

// Pi's settings manager uses proper-lockfile's <file>.lock directory. Publish
// our ownership record with that directory atomically so a killed controller
// can reacquire only its saved lock. The nonempty directory also prevents a
// competing stale-lock cleanup from discarding an interrupted transaction.
// Never remove an unrecorded application lock or infer ownership from its age.
func lockJSONSettings(ctx context.Context, directory, operation string, targets []ProfileTarget, fields []string) (func() error, error) {
	if len(fields) == 0 {
		return func() error { return nil }, nil
	}
	if !operationID.MatchString(operation) || !filepath.IsAbs(directory) {
		return nil, errors.New("JSON settings lock requires saved installer intent")
	}
	releases := []func() error{}
	releaseAll := func() error {
		var result error
		for i := len(releases) - 1; i >= 0; i-- {
			result = errors.Join(result, releases[i]())
		}
		return result
	}
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, releaseAll())
		}
		release, err := lockJSONSettingsTarget(directory, operation, target.Path)
		if err != nil {
			return nil, errors.Join(err, releaseAll())
		}
		releases = append(releases, release)
	}
	return releaseAll, nil
}

func lockJSONSettingsTarget(directory, operation, target string) (func() error, error) {
	lock := target + ".lock"
	stage := target + ".dotfiles-lock-" + operation
	bound, err := bindConfigDestination(lock)
	if err != nil || bound != lock {
		return nil, errors.Join(errors.New("settings lock target was redirected"), err)
	}
	owner := settingsLockOwner{directory, operation, target}
	marker := settingsLockMarker
	verify := func(path string) (os.FileInfo, error) {
		observed, info, err := readSettingsLockOwner(path)
		if err != nil || observed != owner {
			return nil, errors.Join(errors.New("settings lock belongs to another operation"), err)
		}
		return info, nil
	}
	info, err := os.Lstat(lock)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(stage), 0700); err != nil {
			return nil, err
		}
		if err := os.Mkdir(stage, 0700); err == nil {
			if err := saveDocument(filepath.Join(stage, marker), owner); err != nil {
				return nil, fmt.Errorf("preserve incomplete settings lock at %s: %w", stage, err)
			}
		} else if !os.IsExist(err) {
			return nil, err
		}
		if _, err := verify(stage); err != nil {
			return nil, fmt.Errorf("preserve unknown settings lock at %s: %w", stage, err)
		}
		if err := moveConfigExclusive(stage, lock); err != nil {
			return nil, fmt.Errorf("settings are busy at %s; preserve %s and retry after the application finishes: %w", lock, stage, err)
		}
	} else if err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("settings are busy or redirected at %s", lock)
	}
	info, err = verify(lock)
	if err != nil {
		return nil, fmt.Errorf("settings are busy at %s; let the application finish before retrying: %w", lock, err)
	}
	return func() error {
		current, err := verify(lock)
		if err != nil || !os.SameFile(current, info) {
			return errors.Join(fmt.Errorf("settings lock changed; preserve %s", lock), err)
		}
		// Release the public lock path atomically. Cleanup cannot remove a new
		// application lock created after this rename.
		if err := moveConfigExclusive(lock, stage); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(stage, marker)); err != nil {
			return err
		}
		return os.Remove(stage)
	}, nil
}

// Finalization also runs on approved abandonment. A journal is saved before
// acquiring application locks, so even death before the first publication has
// a durable target/operation record from which to release only our own locks.
func finishJSONSettingsLocks(directory string, journal profileJournal) error {
	if len(journal.Fields) == 0 {
		return nil
	}
	for _, entry := range journal.Entries {
		owner := settingsLockOwner{directory, journal.Operation, entry.Path}
		actual, _, err := readSettingsLockOwner(entry.Path + ".lock")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot inspect settings lock %s: %w", entry.Path+".lock", err)
		}
		if err == nil && actual == owner {
			release, err := lockJSONSettingsTarget(directory, journal.Operation, entry.Path)
			if err != nil {
				return err
			}
			if err := release(); err != nil {
				return err
			}
		}
		stage := entry.Path + ".dotfiles-lock-" + journal.Operation
		bound, err := bindConfigDestination(stage)
		if err != nil || bound != stage {
			return errors.Join(fmt.Errorf("settings lock workspace moved; preserve %s", stage), err)
		}
		actual, _, err = readSettingsLockOwner(stage)
		if errors.Is(err, os.ErrNotExist) {
			if _, err := os.Lstat(stage); errors.Is(err, os.ErrNotExist) {
				continue
			}
			// Like publication workspaces, this operation's recorded temporary
			// directory can be removed when empty. Remove refuses a raced-in file.
			if entries, readErr := readPlainDirectory(stage); readErr == nil && len(entries) == 0 {
				if err := os.Remove(stage); err != nil {
					return err
				}
				continue
			}
		}
		if err != nil || actual != owner {
			return errors.Join(fmt.Errorf("preserve unverified settings lock workspace %s", stage), err)
		}
		if err := os.Remove(filepath.Join(stage, settingsLockMarker)); err != nil {
			return err
		}
		if err := os.Remove(stage); err != nil {
			return err
		}
	}
	return nil
}
