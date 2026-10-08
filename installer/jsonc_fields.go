package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"unicode/utf8"
)

// Mask JSONC syntax without changing offsets. The standard JSON parser remains
// responsible for values and structure; comment-like text inside strings stays.
func jsoncMask(data []byte) (clean, lexical []byte, err error) {
	if len(data) > maxProfileBytes || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, nil, errors.New("JSONC settings must be bounded UTF-8")
	}
	lexical = bytes.Clone(data)
	if bytes.HasPrefix(lexical, []byte{0xef, 0xbb, 0xbf}) {
		copy(lexical, "   ")
	}
	for i := 0; i < len(lexical); i++ {
		switch lexical[i] {
		case '"':
			for i++; i < len(lexical); i++ {
				if lexical[i] == '\\' {
					i++
				} else if lexical[i] == '"' {
					break
				}
			}
		case '/':
			if i+1 == len(lexical) {
				continue
			}
			start := i
			if lexical[i+1] == '/' {
				for i += 2; i < len(lexical) && lexical[i] != '\n' && lexical[i] != '\r'; i++ {
				}
			} else if lexical[i+1] == '*' {
				end := bytes.Index(lexical[i+2:], []byte("*/"))
				if end < 0 {
					return nil, nil, errors.New("unterminated JSONC comment")
				}
				i += end + 4
			} else {
				continue
			}
			for j := start; j < i; j++ {
				if lexical[j] != '\r' && lexical[j] != '\n' {
					lexical[j] = ' '
				}
			}
			i--
		}
	}
	clean = bytes.Clone(lexical)
	for i := 0; i < len(clean); i++ {
		if clean[i] == '"' {
			for i++; i < len(clean); i++ {
				if clean[i] == '\\' {
					i++
				} else if clean[i] == '"' {
					break
				}
			}
		} else if clean[i] == ',' {
			previous := i - 1
			for previous >= 0 && bytes.ContainsRune([]byte(" \t\r\n"), rune(clean[previous])) {
				previous--
			}
			if previous < 0 || bytes.ContainsRune([]byte("{[:,"), rune(clean[previous])) {
				continue
			}
			j := i + 1
			for j < len(clean) && bytes.ContainsRune([]byte(" \t\r\n"), rune(clean[j])) {
				j++
			}
			if j < len(clean) && (clean[j] == '}' || clean[j] == ']') {
				clean[i] = ' '
			}
		}
	}
	return clean, lexical, nil
}

func replaceJSONCFields(data []byte, fields []string, replacement []byte) ([]byte, []byte, error) {
	clean, _, err := jsoncMask(data)
	if err != nil {
		return nil, nil, err
	}
	before, err := extractJSONFields(clean, fields)
	if err != nil {
		return nil, nil, err
	}
	// Validate the recipe with the same strict boundary used by other settings.
	if _, _, err := replaceJSONFields(nil, fields, replacement); err != nil {
		return nil, nil, err
	}
	values := map[string]json.RawMessage{}
	if len(replacement) != 0 {
		if err := json.Unmarshal(replacement, &values); err != nil {
			return nil, nil, err
		}
	}
	result := bytes.Clone(data)
	if len(result) == 0 {
		if len(values) == 0 {
			return nil, before, nil
		}
		result = []byte("{}\n")
	}
	for _, field := range fields {
		clean, lexical, err := jsoncMask(result)
		if err != nil {
			return nil, nil, err
		}
		properties, err := jsonProperties(clean)
		if err != nil {
			return nil, nil, err
		}
		index := slices.IndexFunc(properties, func(p jsonProperty) bool { return p.name == field })
		value, keep := values[field]
		if index < 0 && !keep {
			continue
		}
		start, end := 0, 0
		if index >= 0 {
			p := properties[index]
			start, end = p.value, p.end
			if !keep {
				start = p.key
				// Remove one separator independently, leaving intervening comments
				// and whitespace byte-for-byte intact.
				right := len(lexical)
				if index+1 < len(properties) {
					right = properties[index+1].key
				}
				comma := bytes.IndexByte(lexical[p.end:right], ',')
				if comma >= 0 {
					comma += p.end
				} else if index > 0 {
					left := properties[index-1].end
					comma = left + bytes.IndexByte(lexical[left:p.key], ',')
				}
				if comma >= 0 {
					result = append(result[:comma], result[comma+1:]...)
					if comma < start {
						start--
						end--
					}
				}
			}
		} else {
			name, err := json.Marshal(field)
			if err != nil {
				return nil, nil, err
			}
			value = append(append(name, ':'), value...)
			start = bytes.IndexByte(clean, '{') + 1
			if len(properties) != 0 {
				start = properties[len(properties)-1].end
				value = append([]byte{','}, value...)
			}
			end = start
		}
		result = append(append(bytes.Clone(result[:start]), value...), result[end:]...)
	}
	clean, _, err = jsoncMask(result)
	if err == nil {
		_, err = jsonProperties(clean)
	}
	return result, before, err
}

func settingsDocumentEmpty(data []byte, resource string) (bool, error) {
	if resource == "integration.vscode" || resource == windowsTerminalResource {
		clean, lexical, err := jsoncMask(data)
		if err != nil {
			return false, err
		}
		properties, err := jsonProperties(clean)
		// A personal comment added to an installer-created document is data.
		return len(properties) == 0 && bytes.Equal(lexical, data), err
	}
	properties, err := jsonProperties(data)
	return len(properties) == 0, err
}
