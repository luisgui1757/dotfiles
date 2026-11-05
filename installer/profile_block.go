package installer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Profile blocks own only their delimited bytes. Everything outside a block,
// including the original BOM, line endings and missing final newline, survives.
// UTF-16 profiles are common in Windows PowerShell 5.1.
func profileEncoding(data []byte) (func(string) []byte, string, error) {
	encode := func(s string) []byte { return []byte(s) }
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		var order binary.ByteOrder = binary.LittleEndian
		if data[0] == 0xfe {
			order = binary.BigEndian
		}
		if len(data)%2 != 0 {
			return nil, "", errors.New("profile has incomplete UTF-16 data")
		}
		units := make([]uint16, (len(data)-2)/2)
		for i := range units {
			units[i] = order.Uint16(data[2+i*2:])
		}
		for i := 0; i < len(units); i++ {
			if utf16.IsSurrogate(rune(units[i])) {
				if i+1 == len(units) || units[i] < 0xd800 || units[i] > 0xdbff || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return nil, "", errors.New("profile has invalid UTF-16 data")
				}
				i++
			}
		}
		encode = func(s string) []byte {
			u := utf16.Encode([]rune(s))
			out := make([]byte, 2*len(u))
			for i, value := range u {
				order.PutUint16(out[i*2:], value)
			}
			return out
		}
	} else if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, "", errors.New("profile must be valid UTF-8 or BOM-marked UTF-16; preserve it for explicit conversion")
	}
	newline := "\n"
	if bytes.Contains(data, encode("\r\n")) {
		newline = "\r\n"
	}
	return encode, newline, nil
}

// Saved blocks omit their full file's BOM. Recognize only a marked UTF-16
// fragment here; readProfile still requires a BOM on full UTF-16 files.
func markedProfileEncoding(data []byte, begin string) (func(string) []byte, bool, error) {
	encode, _, err := profileEncoding(data)
	if err == nil {
		return encode, false, nil
	}
	for _, bom := range [][]byte{{0xff, 0xfe}, {0xfe, 0xff}} {
		candidate, _, bomErr := profileEncoding(bom)
		if bomErr != nil {
			return nil, false, bomErr
		}
		for _, separator := range []string{"", "\n", "\r\n"} {
			if bytes.HasPrefix(data, candidate(separator+begin)) {
				encode, _, err = profileEncoding(append(bytes.Clone(bom), data...))
				return encode, true, err
			}
		}
	}
	return nil, false, err
}

// The leading newline belongs to the block. Removing a block from a file which
// originally ended without a newline therefore restores the exact original.
func profileBlock(data []byte, id string) (start, end int, err error) {
	start, end, _, err = profileBlockBounds(data, id)
	return start, end, err
}

// A block that opens its file can lose that leading newline: formatters drop a
// file's leading blank lines (Neovim runs shfmt when saving a zsh profile).
// The block is still found; missing is the lost newline, so the block's bytes
// still equal the ones that were written.
func profileBlockBounds(data []byte, id string) (start, end int, missing []byte, err error) {
	if id == gitSettingsResource {
		if err := validateGitEncoding(data); err != nil {
			return 0, 0, nil, err
		}
	}
	if !resourceID.MatchString(id) {
		return 0, 0, nil, errors.New("invalid profile block identity")
	}
	encode, fragment, err := markedProfileEncoding(data, "# >>> dotfiles:"+id+" >>>")
	if err != nil {
		return 0, 0, nil, err
	}
	begin, finish := "# >>> dotfiles:"+id+" >>>", "# <<< dotfiles:"+id+" <<<"
	if bytes.Count(data, encode(begin)) == 0 && bytes.Count(data, encode(finish)) == 0 {
		return -1, -1, nil, nil
	}
	if bytes.Count(data, encode(begin)) != 1 || bytes.Count(data, encode(finish)) != 1 {
		return 0, 0, nil, errors.New("profile block markers are duplicated or incomplete")
	}
	opening := 0
	for _, bom := range [][]byte{{0xef, 0xbb, 0xbf}, {0xff, 0xfe}, {0xfe, 0xff}} {
		if !fragment && bytes.HasPrefix(data, bom) {
			opening = len(bom)
		}
	}
	for _, nl := range []string{"\r\n", "\n"} {
		left, right := encode(nl+begin+nl), encode(finish+nl)
		start, missing = bytes.Index(data, left), nil
		if start < 0 && !fragment && bytes.HasPrefix(data[opening:], encode(begin+nl)) {
			start, missing, left = opening, encode(nl), encode(begin+nl)
		}
		if start < 0 {
			continue
		}
		closing := bytes.Index(data[start+len(left):], right)
		if closing < 0 {
			continue
		}
		end = start + len(left) + closing + len(right)
		if fragment && (start != 0 || end != len(data)) {
			return 0, 0, nil, errors.New("saved UTF-16 profile fragment must contain only its managed block")
		}
		return start, end, missing, nil
	}
	return 0, 0, nil, errors.New("profile block markers were edited; preserve the file")
}

func renderProfileBlock(data []byte, id, script string, fields ...string) ([]byte, error) {
	if id == gitSettingsResource {
		if err := validateGitEncoding(data); err != nil {
			return nil, err
		}
	}
	if id == windowsTerminalResource {
		return renderWindowsTerminal(data, script, fields)
	}
	if len(fields) != 0 {
		properties, err := jsonProperties([]byte(script))
		if err != nil || len(properties) != len(fields) {
			return nil, errors.Join(errors.New("JSON recipe must declare exactly its managed fields"), err)
		}
		if _, _, err := replaceJSONFields(nil, fields, []byte(script)); err != nil {
			return nil, err
		}
		return extractJSONFields([]byte(script), fields)
	}
	encode, nl, err := profileEncoding(data)
	if err != nil {
		return nil, err
	}
	if !resourceID.MatchString(id) || strings.ContainsAny(script, "\x00\r") || strings.Contains(script, "# >>> dotfiles:") || strings.Contains(script, "# <<< dotfiles:") {
		return nil, errors.New("invalid managed profile script")
	}
	start, end, err := profileBlock(data, id)
	if err != nil {
		return nil, err
	}
	if start >= 0 {
		nl = "\n"
		if bytes.Contains(data[start:end], encode("\r\n")) {
			nl = "\r\n"
		}
	}
	return encode(nl + "# >>> dotfiles:" + id + " >>>" + nl + strings.ReplaceAll(strings.TrimRight(script, "\n"), "\n", nl) + nl + "# <<< dotfiles:" + id + " <<<" + nl), nil
}

func replaceProfileBlock(data []byte, id string, replacement []byte, fields ...string) ([]byte, []byte, error) {
	if id == windowsTerminalResource {
		return replaceWindowsTerminal(data, replacement, fields)
	}
	if len(fields) != 0 {
		if id == vscodeSettingsResource {
			return replaceJSONCFields(data, fields, replacement)
		}
		return replaceJSONFields(data, fields, replacement)
	}
	start, end, missing, err := profileBlockBounds(data, id)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", id, err)
	}
	if start < 0 {
		if id == gitSettingsResource {
			// Git accepts a UTF-8 BOM only at the beginning of the file.
			prefix := 0
			if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
				prefix = 3
			}
			result := append(bytes.Clone(data[:prefix]), replacement...)
			return append(result, data[prefix:]...), nil, nil
		}
		return append(bytes.Clone(data), replacement...), nil, nil
	}
	result := append(bytes.Clone(data[:start]), replacement...)
	result = append(result, data[end:]...)
	return result, append(bytes.Clone(missing), data[start:end]...), nil
}
