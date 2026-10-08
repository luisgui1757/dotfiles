package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

func windowsTerminalProperties(data []byte) ([]jsonProperty, error) {
	clean, _, err := jsoncMask(data)
	if err != nil {
		return nil, err
	}
	return jsonProperties(clean)
}

// The pinned Terminal owns migration of its historical combined action/keys
// schema. Do not guess generated IDs or overwrite an overlapping old binding.
func rejectWindowsTerminalLegacyBindings(data []byte) error {
	array, present, err := windowsTerminalGet(data, "actions")
	if err != nil || !present {
		return err
	}
	for _, collection := range windowsTerminalCollections() {
		if collection.Path != "keybindings" {
			continue
		}
		legacy := collection
		legacy.Path = "actions"
		entry, _, err := windowsTerminalEntry(array, legacy)
		if err != nil || entry != nil {
			return errors.New("Windows Terminal has an overlapping legacy combined action/key binding; preserve settings, launch the supported unpackaged stable Terminal once to migrate its settings, then retry")
		}
	}
	return nil
}

// These path helpers are private to the compiled Terminal recipe. Projection
// validation rejects every path not declared by that recipe.
func windowsTerminalGet(data []byte, path string) ([]byte, bool, error) {
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

func windowsTerminalSet(data []byte, path string, value []byte) ([]byte, error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 2 {
		child, present, err := windowsTerminalGet(data, parts[0])
		if err != nil {
			return nil, err
		}
		if !present {
			if value == nil {
				return data, nil
			}
			child = []byte("{}")
		}
		value, err = windowsTerminalSet(child, parts[1], value)
		if err != nil {
			return nil, err
		}
	}
	clean, _, err := jsoncMask(data)
	if err != nil {
		return nil, err
	}
	properties, err := jsonProperties(clean)
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(properties, func(p jsonProperty) bool { return p.name == parts[0] })
	if index >= 0 && value != nil {
		p := properties[index]
		// Do not rewrite equal values: comments/formatting in a personal value
		// need not change when the desired setting itself has not changed.
		a, err := windowsTerminalValue(data[p.value:p.end])
		if err != nil {
			return nil, err
		}
		b, err := windowsTerminalValue(value)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(a, b) {
			return data, nil
		}
		return append(append(bytes.Clone(data[:p.value]), value...), data[p.end:]...), nil
	}
	if value == nil {
		result, _, err := replaceJSONCFields(data, []string{parts[0]}, nil)
		return result, err
	}
	// The shared writer adds the key/separator. Replace the placeholder value
	// afterwards so restoring a JSONC leaf also restores its own comments.
	result, _, err := replaceJSONCFields(data, []string{parts[0]}, []byte("{"+string(mustTerminalKey(parts[0]))+":null}"))
	if err != nil {
		return nil, err
	}
	properties, err = windowsTerminalProperties(result)
	if err != nil {
		return nil, err
	}
	index = slices.IndexFunc(properties, func(p jsonProperty) bool { return p.name == parts[0] })
	if index < 0 {
		return nil, errors.New("Windows Terminal field publication lost its key")
	}
	p := properties[index]
	return append(append(bytes.Clone(result[:p.value]), value...), result[p.end:]...), nil
}

func mustTerminalKey(value string) []byte {
	// All keys come from the fixed ASCII recipe, never external JSON paths.
	return []byte(`"` + value + `"`)
}

type windowsTerminalElement struct{ start, end int }

func windowsTerminalElements(data []byte) ([]windowsTerminalElement, []byte, error) {
	clean, lexical, err := jsoncMask(data)
	if err != nil {
		return nil, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(clean))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('[') {
		return nil, nil, errors.New("Windows Terminal collection must be an array")
	}
	var result []windowsTerminalElement
	for decoder.More() {
		start := int(decoder.InputOffset())
		for start < len(clean) && bytes.ContainsRune([]byte(" \t\r\n,"), rune(clean[start])) {
			start++
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, nil, err
		}
		result = append(result, windowsTerminalElement{start, int(decoder.InputOffset())})
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
		return nil, nil, errors.New("incomplete Windows Terminal collection")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, nil, errors.New("trailing Windows Terminal collection content")
	}
	return result, lexical, nil
}

func windowsTerminalIdentityEqual(selector, actual, expected string) bool {
	if selector == "keys" {
		a, b := strings.Split(strings.ToLower(actual), "+"), strings.Split(strings.ToLower(expected), "+")
		return slices.Equal(sortedUnique(a), sortedUnique(b))
	}
	if selector == "guid" {
		return strings.EqualFold(actual, expected)
	}
	return strings.EqualFold(actual, expected)
}

func windowsTerminalEntry(array []byte, collection windowsTerminalCollection) ([]byte, int, error) {
	elements, _, err := windowsTerminalElements(array)
	if err != nil {
		return nil, -1, err
	}
	index := -1
	var result []byte
	for i, element := range elements {
		entry := array[element.start:element.end]
		value, present, err := windowsTerminalGet(entry, collection.Key)
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

func windowsTerminalReplaceElement(array []byte, index int, value []byte) ([]byte, error) {
	elements, lexical, err := windowsTerminalElements(array)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		if value == nil {
			return array, nil
		}
		at := bytes.IndexByte(lexical, '[') + 1
		if len(elements) > 0 {
			at = elements[len(elements)-1].end
			value = append([]byte{','}, value...)
		}
		return append(append(bytes.Clone(array[:at]), value...), array[at:]...), nil
	}
	start, end := elements[index].start, elements[index].end
	if value == nil {
		right := len(lexical)
		if index+1 < len(elements) {
			right = elements[index+1].start
		}
		comma := bytes.IndexByte(lexical[end:right], ',')
		if comma >= 0 {
			comma += end
		} else if index > 0 {
			left := elements[index-1].end
			comma = left + bytes.IndexByte(lexical[left:start], ',')
		}
		if comma >= 0 {
			array = append(bytes.Clone(array[:comma]), array[comma+1:]...)
			if comma < start {
				start--
				end--
			}
		}
	}
	return append(append(bytes.Clone(array[:start]), value...), array[end:]...), nil
}

func windowsTerminalEmpty(data []byte, array bool) (bool, error) {
	_, lexical, err := jsoncMask(data)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(lexical, data) {
		return false, nil
	}
	if array {
		items, _, err := windowsTerminalElements(data)
		return len(items) == 0, err
	}
	properties, err := windowsTerminalProperties(data)
	return len(properties) == 0, err
}
