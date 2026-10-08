package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// configureWindowsTerminal binds only the pinned unpackaged stable instance.
// It is pure with respect to installed settings; the ordinary ProfileDriver
// approval/adoption and durable publication lifecycle governs all writes.
func configureWindowsTerminal(d *ProfileDriver, target Context, folders ConfigFolders, repository, pwsh string) error {
	if target.OS != "windows" {
		return nil
	}
	for _, path := range []string{folders.LocalAppData, repository, pwsh} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("Windows Terminal requires observed LocalAppData and canonical repository/managed PowerShell paths")
		}
	}
	data, err := readDocument(filepath.Join(repository, "windows-terminal", "settings.fragment.jsonc"))
	if err != nil {
		return err
	}
	projection, err := extractWindowsTerminal(data)
	if err != nil {
		return err
	}
	remainder, _, err := replaceWindowsTerminal(data, nil, []string{windowsTerminalField})
	if err != nil {
		return err
	}
	properties, err := windowsTerminalProperties(remainder)
	if err != nil || len(properties) != 0 {
		return errors.Join(errors.New("Windows Terminal source has settings outside its reviewed fixed recipe"), err)
	}
	p, err := decodeWindowsTerminalProjection(projection)
	if err != nil {
		return err
	}
	p.Values["profile/commandline"], err = json.Marshal(`"` + pwsh + `"`)
	if err != nil {
		return err
	}
	projection, err = windowsTerminalProjectionData(p)
	if err != nil {
		return err
	}
	path, err := bindConfigDestination(filepath.Join(folders.LocalAppData, "Microsoft", "Windows Terminal", "settings.json"))
	if err != nil {
		return err
	}
	if d.Targets == nil {
		d.Targets = map[string][]ProfileTarget{}
	}
	if d.JSONFields == nil {
		d.JSONFields = map[string][]string{}
	}
	d.Targets[windowsTerminalResource] = []ProfileTarget{{path, string(projection)}}
	d.JSONFields[windowsTerminalResource] = []string{windowsTerminalField}
	return d.validate(windowsTerminalResource)
}

func renderWindowsTerminal(data []byte, recipe string, fields []string) ([]byte, error) {
	if !slices.Equal(fields, []string{windowsTerminalField}) {
		return nil, errors.New("Windows Terminal requires its fixed scoped projection")
	}
	p, err := decodeWindowsTerminalProjection([]byte(recipe))
	if err != nil {
		return nil, err
	}
	defaultValue, err := windowsTerminalValue(p.Default)
	var defaultProfile string
	if err != nil || json.Unmarshal(defaultValue, &defaultProfile) != nil || defaultProfile != windowsTerminalProfileGUID {
		return nil, errors.New("Windows Terminal recipe must declare its managed PowerShell startup profile")
	}
	for _, field := range windowsTerminalScalars {
		if _, ok := p.Values[field]; !ok {
			return nil, errors.New("Windows Terminal recipe is missing a fixed setting")
		}
	}
	for _, collection := range windowsTerminalCollections() {
		for _, field := range collection.Fields {
			if _, ok := p.Values[collection.ID+"/"+field]; !ok {
				return nil, errors.New("Windows Terminal recipe is missing a fixed profile, theme or keybinding setting")
			}
		}
	}
	result, _, err := replaceWindowsTerminal(data, []byte(recipe), fields)
	if err != nil {
		return nil, err
	}
	return extractWindowsTerminal(result)
}

func replaceWindowsTerminal(data, replacement []byte, fields []string) ([]byte, []byte, error) {
	if !slices.Equal(fields, []string{windowsTerminalField}) {
		return nil, nil, errors.New("Windows Terminal requires its fixed scoped projection")
	}
	before, err := extractWindowsTerminal(data)
	if err != nil {
		return nil, nil, err
	}
	want, err := decodeWindowsTerminalProjection(replacement)
	if err != nil {
		return nil, nil, err
	}
	// Saved journal/baseline projections are already validated; this branch
	// supports the existing publisher's block-only integrity checks.
	if len(data) > 0 {
		properties, err := windowsTerminalProperties(data)
		if err != nil {
			return nil, nil, err
		}
		if slices.ContainsFunc(properties, func(p jsonProperty) bool { return p.name == "dotfiles_windows_terminal" }) {
			return bytes.Clone(replacement), before, nil
		}
	}
	result := bytes.Clone(data)
	if len(result) == 0 {
		result = []byte("{}\n")
	}
	for _, field := range windowsTerminalScalars {
		result, err = windowsTerminalSet(result, field, want.Values[field])
		if err != nil {
			return nil, nil, err
		}
	}
	for _, collection := range windowsTerminalCollections() {
		array, present, err := windowsTerminalGet(result, collection.Path)
		if err != nil {
			return nil, nil, err
		}
		needed := slices.Contains(want.Containers, collection.ID)
		for _, field := range collection.Fields {
			needed = needed || want.Values[collection.ID+"/"+field] != nil
		}
		if !present {
			if !needed {
				continue
			}
			array = []byte("[]")
		}
		entry, index, err := windowsTerminalEntry(array, collection)
		if err != nil {
			return nil, nil, err
		}
		if entry == nil {
			if !needed {
				continue
			}
			entry, err = json.Marshal(map[string]string{collection.Key: collection.Match})
			if err != nil {
				return nil, nil, err
			}
		}
		for _, field := range collection.Fields {
			entry, err = windowsTerminalSet(entry, field, want.Values[collection.ID+"/"+field])
			if err != nil {
				return nil, nil, err
			}
		}
		for _, path := range []string{"tab", "tabRow", "window"} {
			if collection.ID != "theme" || slices.Contains(want.Containers, collection.ID+"/"+path) {
				continue
			}
			value, present, err := windowsTerminalGet(entry, path)
			if err != nil {
				return nil, nil, err
			}
			if present {
				empty, err := windowsTerminalEmpty(value, false)
				if err != nil {
					return nil, nil, err
				}
				if empty {
					entry, err = windowsTerminalSet(entry, path, nil)
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
		if !needed {
			withoutKey, err := windowsTerminalSet(entry, collection.Key, nil)
			if err != nil {
				return nil, nil, err
			}
			empty, err := windowsTerminalEmpty(withoutKey, false)
			if err != nil {
				return nil, nil, err
			}
			if empty {
				entry = nil
			}
		}
		array, err = windowsTerminalReplaceElement(array, index, entry)
		if err != nil {
			return nil, nil, err
		}
		result, err = windowsTerminalSet(result, collection.Path, array)
		if err != nil {
			return nil, nil, err
		}
	}
	current, present, err := windowsTerminalGet(result, "defaultProfile")
	if err != nil {
		return nil, nil, err
	}
	name := ""
	if present {
		value, err := windowsTerminalValue(current)
		if err != nil || json.Unmarshal(value, &name) != nil {
			return nil, nil, errors.New("Windows Terminal defaultProfile must be a string")
		}
	}
	// Personal nonempty preferences remain independent, including during update.
	// Removal cannot leave our GUID as default after deleting that profile.
	var prior string
	if len(want.Default) > 0 {
		value, err := windowsTerminalValue(want.Default)
		if err != nil || json.Unmarshal(value, &prior) != nil {
			return nil, nil, errors.New("invalid saved Windows Terminal startup preference")
		}
	}
	removedDefault := false
	if strings.EqualFold(name, windowsTerminalProfileGUID) {
		collection := windowsTerminalCollections()[0]
		array, exists, err := windowsTerminalGet(result, collection.Path)
		if err != nil {
			return nil, nil, err
		}
		removedDefault = !exists
		if exists {
			entry, _, err := windowsTerminalEntry(array, collection)
			if err != nil {
				return nil, nil, err
			}
			removedDefault = entry == nil
		}
	}
	if !present || name == "" || strings.EqualFold(name, windowsTerminalProfileGUID) && prior == "" || removedDefault {
		result, err = windowsTerminalSet(result, "defaultProfile", want.Default)
		if err != nil {
			return nil, nil, err
		}
	}
	for _, path := range []string{"profiles/defaults/font", "profiles/defaults", "profiles/list", "profiles", "schemes", "themes", "actions", "keybindings"} {
		if slices.Contains(want.Containers, path) {
			continue
		}
		value, present, err := windowsTerminalGet(result, path)
		if err != nil {
			return nil, nil, err
		}
		if !present {
			continue
		}
		array := slices.Contains([]string{"profiles/list", "schemes", "themes", "actions", "keybindings"}, path)
		empty, err := windowsTerminalEmpty(value, array)
		if err != nil {
			return nil, nil, err
		}
		if empty {
			result, err = windowsTerminalSet(result, path, nil)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if _, err := extractWindowsTerminal(result); err != nil {
		return nil, nil, err
	}
	return result, before, nil
}
