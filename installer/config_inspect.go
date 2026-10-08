package installer

import (
	"errors"
	"os"
	"path/filepath"
)

type configTargetState struct {
	Target  ResolvedConfigTarget `json:"target"`
	Current configSnapshot       `json:"current"`
	Input   configSnapshot       `json:"input"`
	Desired configSnapshot       `json:"desired"`
}

// Resolve the deepest existing parent without creating missing directories.
// The canonical destination participates in observation, so redirecting a known
// folder or replacing a parent symlink invalidates the approved target identity.
func bindConfigDestination(path string) (string, error) {
	parent, suffix := filepath.Dir(path), []string{filepath.Base(path)}
	for {
		info, err := os.Lstat(parent)
		if err == nil {
			resolved, err := resolveConfigPath(parent)
			if err != nil {
				return "", err
			}
			info, err = os.Stat(resolved)
			if err != nil || !info.IsDir() {
				return "", errors.Join(errors.New("configuration parent is not a directory"), err)
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(parent) == parent {
			return "", err
		}
		suffix = append(suffix, filepath.Base(parent))
		parent = filepath.Dir(parent)
	}
}

func inspectConfiguration(c *Catalog, manifest ConfigManifest, target Context, folders ConfigFolders, repository, resource string) ([]configTargetState, Observation, error) {
	if !filepath.IsAbs(repository) {
		return nil, Observation{}, errors.New("configuration source root must be absolute")
	}
	resolvedRepository, err := resolveConfigPath(repository)
	if errors.Is(err, os.ErrNotExist) {
		resolvedRepository, err = bindConfigDestination(repository)
	}
	if err != nil {
		return nil, Observation{}, err
	}
	repository = resolvedRepository
	resolved, err := manifest.Resolve(c, target, folders, repository, resource)
	if err != nil {
		return nil, Observation{}, err
	}
	states := []configTargetState{}
	paths := []string{}
	actual := []struct {
		Path     string
		Snapshot configSnapshot
	}{}
	o := Observation{Provider: "configuration", Scope: "user", Healthy: len(resolved) != 0}
	for _, item := range resolved {
		item.Destination, err = bindConfigDestination(item.Destination)
		if err != nil {
			return nil, o, err
		}
		// Repeat the overlap guard after resolving redirected parents.
		for _, pair := range [][2]string{{repository, item.Destination}, {item.Destination, repository}} {
			relative, err := filepath.Rel(pair[0], pair[1])
			if err == nil && (relative == "." || filepath.IsLocal(relative)) {
				return nil, o, errors.New("resolved configuration destination overlaps its source checkout")
			}
		}
		input, inputErr := inspectConfigInput(item)
		if inputErr != nil {
			input = configSnapshot{Kind: "unavailable"}
		}
		inputValid := item.Directory && input.Kind == "directory" || !item.Directory && input.Kind == "file"
		if !inputValid {
			o.ApplyBlocked = "canonical configuration source is unavailable or has the wrong type; restore it before applying"
			if inputErr != nil {
				o.ApplyBlocked += ": " + inputErr.Error()
			}
		}
		current, err := snapshotConfig(item.Destination)
		if err != nil {
			return nil, o, err
		}
		desired := input
		if item.Mode == "link" || item.Mode == "junction" {
			kind := item.Mode
			hash, err := digest([]configEntry{{Path: ".", Kind: kind, Content: item.Source}})
			if err != nil {
				return nil, o, err
			}
			desired = configSnapshot{Kind: kind, Hash: hash}
		}
		states = append(states, configTargetState{item, current, input, desired})
		paths = append(paths, item.Destination)
		actual = append(actual, struct {
			Path     string
			Snapshot configSnapshot
		}{item.Destination, current})
		o.Present = o.Present || current.Kind != "absent"
		o.Healthy = o.Healthy && inputValid && current == desired
	}
	o.Adoptable = o.Present
	o.Identity, err = digest(paths)
	if err != nil {
		return nil, o, err
	}
	o.Fingerprint, err = digest(actual)
	if err != nil {
		return nil, o, err
	}
	// Current bytes are already bound separately; Desired describes only input
	// and destination. Editing a source never grants ownership of an old target.
	desiredStates := append([]configTargetState{}, states...)
	for i := range desiredStates {
		desiredStates[i].Current = configSnapshot{}
	}
	o.Desired, err = digest(desiredStates)
	return states, o, err
}

// A live link promises a location and source type, not frozen source bytes.
// User edits through that link must neither revoke ownership nor deadlock link
// recovery. Copies bind the exact payload because publication replaces bytes.
func inspectConfigInput(target ResolvedConfigTarget) (configSnapshot, error) {
	if target.Mode != "link" && target.Mode != "junction" {
		return snapshotConfig(target.Source)
	}
	info, err := os.Lstat(target.Source)
	if errors.Is(err, os.ErrNotExist) {
		return configSnapshot{Kind: "absent"}, nil
	}
	if err != nil {
		return configSnapshot{}, err
	}
	if info.IsDir() {
		return configSnapshot{Kind: "directory"}, nil
	}
	if info.Mode().IsRegular() {
		return configSnapshot{Kind: "file"}, nil
	}
	return configSnapshot{Kind: "unsupported"}, nil
}
