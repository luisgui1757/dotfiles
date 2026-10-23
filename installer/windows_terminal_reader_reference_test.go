package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Frozen uncached reference from cfb54576c627. Keep this separate from the
// production reader: it is the byte/error oracle for inspection-only parse reuse.
func referenceTerminalGet(data []byte, path string) ([]byte, bool, error) {
	for _, part := range strings.Split(path, "/") {
		properties, err := windowsTerminalProperties(data)
		if err != nil {
			return nil, false, err
		}
		index := slices.IndexFunc(properties, func(p jsonProperty) bool { return p.name == part })
		if index < 0 {
			return nil, false, nil
		}
		p := properties[index]
		data = data[p.value:p.end]
	}
	return data, true, nil
}

func referenceTerminalEntry(array []byte, collection windowsTerminalCollection) ([]byte, int, error) {
	elements, _, err := windowsTerminalElements(array)
	if err != nil {
		return nil, -1, err
	}
	index := -1
	var result []byte
	for i, element := range elements {
		entry := array[element.start:element.end]
		value, present, err := referenceTerminalGet(entry, collection.Key)
		if err != nil {
			return nil, -1, err
		}
		if !present {
			continue
		}
		clean, err := windowsTerminalValue(value)
		if err != nil {
			return nil, -1, err
		}
		var key string
		if json.Unmarshal(clean, &key) != nil {
			// A key chord array could overlap our fixed keys and needs explicit
			// user normalization before it can provide an unambiguous baseline.
			if collection.Key == "keys" {
				var keys []string
				if json.Unmarshal(clean, &keys) != nil {
					return nil, -1, errors.New("Windows Terminal action keys must be strings or string arrays")
				}
				if slices.ContainsFunc(keys, func(key string) bool { return windowsTerminalIdentityEqual(collection.Key, key, collection.Match) }) {
					return nil, -1, errors.New("managed Windows Terminal action key appears in a grouped binding; normalize it to an individual binding before adoption")
				}
				continue
			}
			return nil, -1, errors.New("Windows Terminal profile/scheme identity must be a string")
		}
		if collection.Key == "guid" && strings.EqualFold(strings.Trim(key, "{}"), strings.Trim(collection.Match, "{}")) && !strings.EqualFold(key, collection.Match) {
			return nil, -1, errors.New("managed Windows Terminal profile GUID must use its canonical braces before adoption")
		}
		if collection.Key != "guid" && collection.Key != "keys" && strings.EqualFold(key, collection.Match) && key != collection.Match {
			return nil, -1, errors.New("managed Windows Terminal name or action ID differs only by case; normalize the case-sensitive reference before adoption")
		}
		if !windowsTerminalIdentityEqual(collection.Key, key, collection.Match) {
			continue
		}
		if index >= 0 {
			return nil, -1, fmt.Errorf("duplicate managed Windows Terminal %s identity", collection.ID)
		}
		result, index = entry, i
	}
	return result, index, nil
}

func referenceTerminalLegacyBindings(data []byte) error {
	array, present, err := referenceTerminalGet(data, "actions")
	if err != nil || !present {
		return err
	}
	for _, collection := range windowsTerminalCollections() {
		if collection.Path != "keybindings" {
			continue
		}
		legacy := collection
		legacy.Path = "actions"
		entry, _, err := referenceTerminalEntry(array, legacy)
		if err != nil || entry != nil {
			return errors.New("Windows Terminal has an overlapping legacy combined action/key binding; preserve settings, launch the supported unpackaged stable Terminal once to migrate its settings, then retry")
		}
	}
	return nil
}

// These path helpers are private to the compiled Terminal recipe. Projection
// validation rejects every path not declared by that recipe.

func referenceTerminalExtract(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	clean, _, err := jsoncMask(data)
	if err != nil {
		return nil, err
	}
	properties, err := jsonProperties(clean)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(properties, func(p jsonProperty) bool { return p.name == "dotfiles_windows_terminal" }) {
		p, err := decodeWindowsTerminalProjection(data)
		if err != nil {
			return nil, err
		}
		return windowsTerminalProjectionData(p)
	}
	if err := referenceTerminalLegacyBindings(data); err != nil {
		return nil, err
	}
	p := windowsTerminalProjection{Schema: 1, Values: map[string][]byte{}}
	for _, path := range []string{"profiles", "profiles/defaults", "profiles/defaults/font"} {
		value, present, err := referenceTerminalGet(data, path)
		if err != nil {
			return nil, err
		}
		if present {
			if _, err := windowsTerminalProperties(value); err != nil {
				return nil, fmt.Errorf("Windows Terminal %s must be an object: %w", path, err)
			}
			p.Containers = append(p.Containers, path)
		}
	}
	for _, field := range windowsTerminalScalars {
		value, present, err := referenceTerminalGet(data, field)
		if err != nil {
			return nil, err
		}
		if present {
			p.Values[field] = bytes.Clone(value)
		}
	}
	if value, present, err := referenceTerminalGet(data, "defaultProfile"); err != nil {
		return nil, err
	} else if present {
		p.Default = bytes.Clone(value)
	}
	for _, collection := range windowsTerminalCollections() {
		array, present, err := referenceTerminalGet(data, collection.Path)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		p.Containers = append(p.Containers, collection.Path)
		entry, _, err := referenceTerminalEntry(array, collection)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			continue
		}
		p.Containers = append(p.Containers, collection.ID)
		for _, field := range collection.Fields {
			parts := strings.Split(field, "/")
			for i := 1; i < len(parts); i++ {
				path := strings.Join(parts[:i], "/")
				value, present, err := referenceTerminalGet(entry, path)
				if err != nil {
					return nil, err
				}
				if present {
					if _, err := windowsTerminalProperties(value); err != nil {
						return nil, err
					}
					p.Containers = append(p.Containers, collection.ID+"/"+path)
				}
			}
			value, present, err := referenceTerminalGet(entry, field)
			if err != nil {
				return nil, err
			}
			if present {
				p.Values[collection.ID+"/"+field] = bytes.Clone(value)
			}
		}
	}
	encoded, err := windowsTerminalProjectionData(p)
	if err != nil {
		return nil, err
	}
	_, err = decodeWindowsTerminalProjection(encoded)
	return encoded, err
}
