package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

type jsonProperty struct {
	name            string
	key, value, end int
}

func validateJSONFields(fields []string) error {
	if len(fields) > 64 {
		return errors.New("too many managed JSON fields")
	}
	for i, field := range fields {
		if field == "" || len(field) > 256 || !utf8.ValidString(field) || strings.ContainsAny(field, "\x00\r\n") || slices.Contains(fields[:i], field) {
			return errors.New("invalid or duplicate managed JSON field")
		}
	}
	return nil
}

// Decode values with the standard parser, retaining their byte offsets. Only
// owned field values and necessary separators change; unrelated bytes survive.
// Duplicate keys and non-object roots cannot supply an unambiguous baseline.
func jsonProperties(data []byte) ([]jsonProperty, error) {
	if !utf8.Valid(data) || len(data) > maxProfileBytes {
		return nil, errors.New("JSON settings must be bounded UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, errors.New("settings must contain a JSON object")
	}
	properties := []jsonProperty{}
	seen := map[string]bool{}
	for decoder.More() {
		key := int(decoder.InputOffset())
		for key < len(data) && strings.ContainsRune(" \t\r\n,", rune(data[key])) {
			key++
		}
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return nil, errors.New("settings contain an invalid or duplicate JSON property")
		}
		seen[name] = true
		value := int(decoder.InputOffset())
		for value < len(data) && strings.ContainsRune(" \t\r\n:", rune(data[value])) {
			value++
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		properties = append(properties, jsonProperty{name, key, value, int(decoder.InputOffset())})
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return nil, errors.New("settings contain an incomplete JSON object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("settings contain trailing content")
	}
	return properties, nil
}

func extractJSONFields(data []byte, fields []string) ([]byte, error) {
	if err := validateJSONFields(fields); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	properties, err := jsonProperties(data)
	if err != nil {
		return nil, err
	}
	values := map[string]json.RawMessage{}
	for _, property := range properties {
		if slices.Contains(fields, property.name) {
			values[property.name] = bytes.Clone(data[property.value:property.end])
		}
	}
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func replaceJSONFields(data []byte, fields []string, replacement []byte) ([]byte, []byte, error) {
	before, err := extractJSONFields(data, fields)
	if err != nil {
		return nil, nil, err
	}
	values := map[string]json.RawMessage{}
	if len(replacement) != 0 {
		properties, err := jsonProperties(replacement)
		if err != nil {
			return nil, nil, err
		}
		for _, property := range properties {
			if !slices.Contains(fields, property.name) {
				return nil, nil, errors.New("replacement contains an unowned JSON field")
			}
			values[property.name] = replacement[property.value:property.end]
		}
	}
	if len(data) == 0 && len(values) == 0 {
		return nil, before, nil
	}
	result := bytes.Clone(data)
	if len(result) == 0 {
		result = []byte("{}\n")
	}
	for _, field := range fields {
		properties, err := jsonProperties(result)
		if err != nil {
			return nil, nil, err
		}
		index := slices.IndexFunc(properties, func(p jsonProperty) bool { return p.name == field })
		value, keep := values[field]
		start, end := 0, 0
		if index >= 0 {
			property := properties[index]
			start, end = property.value, property.end
			if !keep {
				start = property.key
				if index+1 < len(properties) {
					end = properties[index+1].key
				} else if index > 0 {
					start = properties[index-1].end
				}
			}
		} else if keep {
			name, err := json.Marshal(field)
			if err != nil {
				return nil, nil, err
			}
			value = append(append(name, ':'), value...)
			if len(properties) > 0 {
				start = properties[len(properties)-1].end
				value = append([]byte{','}, value...)
			} else {
				start = bytes.IndexByte(result, '{') + 1
			}
			end = start
		} else {
			continue
		}
		result = append(append(bytes.Clone(result[:start]), value...), result[end:]...)
	}
	if _, err := jsonProperties(result); err != nil {
		return nil, nil, err
	}
	return result, before, nil
}
