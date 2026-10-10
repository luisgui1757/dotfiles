package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxConfigBytes = 128 * 1024 * 1024
const maxConfigEntries = 4096

type configSnapshot struct {
	Kind string `json:"kind"`
	Hash string `json:"hash,omitempty"`
}

type configEntry struct {
	Path, Kind, Content string
	Mode                uint32
}

// Windows ReadDir can report ErrNotExist for an existing regular file. Check
// the entry itself before callers interpret that error as absent owned state.
func readPlainDirectory(path string) ([]os.DirEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, fmt.Errorf("expected an unredirected directory at %s", path)
	}
	return os.ReadDir(path)
}

// Snapshot links themselves, never their referents. Configuration backups must
// preserve a user's link without reading or taking ownership of its destination.
// Directories and regular files are bounded and checked for concurrent changes.
func snapshotConfig(path string) (configSnapshot, error) {
	return snapshotTree(path, maxConfigBytes, maxConfigEntries)
}

func snapshotTree(path string, byteLimit int64, entryLimit int) (configSnapshot, error) {
	entries, err := inspectTree(path, byteLimit, entryLimit)
	if err != nil {
		return configSnapshot{}, err
	}
	return snapshotEntries(entries)
}

func snapshotEntries(entries []configEntry) (configSnapshot, error) {
	if len(entries) == 0 {
		return configSnapshot{Kind: "absent"}, nil
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return configSnapshot{}, err
	}
	hash := sha256.Sum256(encoded)
	return configSnapshot{Kind: entries[0].Kind, Hash: hex.EncodeToString(hash[:])}, nil
}

func inspectTree(path string, byteLimit int64, entryLimit int) ([]configEntry, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []configEntry{}
	remaining := byteLimit
	var walk func(string, string, os.FileInfo) error
	walk = func(name, relative string, before os.FileInfo) error {
		if len(entries) >= entryLimit {
			return errors.New("configuration exceeds the bounded entry count; preserve it for explicit migration")
		}
		entry := configEntry{Path: relative, Mode: uint32(before.Mode().Perm())}
		switch {
		case before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0:
			entry.Kind = "link"
			if before.Mode()&os.ModeIrregular != 0 {
				entry.Kind = "junction"
			}
			// Native link creation permissions vary with OS/umask. The managed
			// link contract is its target; retain modes for files/directories.
			entry.Mode = 0
			entry.Content, err = os.Readlink(name)
			if err != nil {
				return err
			}
			if len(entry.Content) > 32768 {
				return errors.New("configuration link exceeds its size bound")
			}
		case before.IsDir():
			entry.Kind = "directory"
		case before.Mode().IsRegular():
			entry.Kind = "file"
			if before.Size() > remaining {
				return fmt.Errorf("tree exceeds %d bytes; preserve it for explicit migration", byteLimit)
			}
			file, err := os.Open(name)
			if err != nil {
				return err
			}
			opened, err := file.Stat()
			if err != nil || !os.SameFile(before, opened) {
				return errors.Join(errors.New("configuration changed while opening"), err, file.Close())
			}
			hash := sha256.New()
			count, copyErr := io.Copy(hash, io.LimitReader(file, remaining+1))
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
			remaining -= count
			if remaining < 0 {
				return errors.New("configuration grew beyond its size bound")
			}
			entry.Content = hex.EncodeToString(hash.Sum(nil))
		default:
			return errors.New("configuration contains a special file; preserve it for explicit migration")
		}
		entries = append(entries, entry)
		if before.IsDir() {
			children, err := os.ReadDir(name)
			if err != nil {
				return err
			}
			for _, child := range children {
				childPath := filepath.Join(name, child.Name())
				childInfo, err := os.Lstat(childPath)
				if err != nil {
					return err
				}
				if err := walk(childPath, filepath.Join(relative, child.Name()), childInfo); err != nil {
					return err
				}
			}
		}
		after, err := os.Lstat(name)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return errors.Join(errors.New("configuration changed during inspection; review a fresh plan"), err)
		}
		return nil
	}
	if err := walk(path, ".", info); err != nil {
		return nil, err
	}
	return entries, nil
}

// Copy an audited payload to an exclusive staging location. Publication and
// baseline movement are separate journaled operations in the config provider.
// Source links/special files are rejected rather than followed into user data.
func copyConfigPayload(source, destination string) error {
	before, err := snapshotConfig(source)
	if err != nil {
		return err
	}
	if before.Kind != "file" && before.Kind != "directory" {
		return errors.New("configuration payload must be a regular file or directory")
	}
	remainingBytes, remainingEntries := int64(maxConfigBytes), maxConfigEntries
	var copyOne func(string, string) error
	copyOne = func(from, to string) error {
		remainingEntries--
		if remainingEntries < 0 {
			return errors.New("configuration payload grew beyond its entry bound")
		}
		info, err := os.Lstat(from)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.Mkdir(to, 0700); err != nil {
				return err
			}
			children, err := os.ReadDir(from)
			if err != nil {
				return err
			}
			for _, child := range children {
				if err := copyOne(filepath.Join(from, child.Name()), filepath.Join(to, child.Name())); err != nil {
					return err
				}
			}
			if err := os.Chmod(to, info.Mode().Perm()); err != nil {
				return err
			}
			return syncConfigDirectory(to)
		}
		if !info.Mode().IsRegular() {
			return errors.New("configuration payload contains a link or special file")
		}
		input, err := os.Open(from)
		if err != nil {
			return err
		}
		opened, err := input.Stat()
		if err != nil || !os.SameFile(info, opened) {
			return errors.Join(errors.New("payload changed while opening"), err, input.Close())
		}
		output, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return errors.Join(err, input.Close())
		}
		count, copyErr := io.Copy(output, io.LimitReader(input, remainingBytes+1))
		err = errors.Join(copyErr, output.Chmod(info.Mode().Perm()), output.Sync(), output.Close(), input.Close())
		remainingBytes -= count
		if remainingBytes < 0 {
			err = errors.Join(err, errors.New("configuration payload grew beyond its size bound"))
		}
		return err
	}
	if err := copyOne(source, destination); err != nil {
		return err
	}
	after, err := snapshotConfig(destination)
	if err != nil || before != after {
		return errors.Join(fmt.Errorf("staged configuration differs from its inspected source"), err)
	}
	return nil
}
