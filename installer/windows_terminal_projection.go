package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
)

const windowsTerminalResource = "integration.windows-terminal"
const windowsTerminalField = "windows-terminal-scoped-v1"
const windowsTerminalProfileGUID = "{8a0e8c9b-2b4c-5842-ac1b-29cd17efc89b}"

// Values are the fixed recipe's owned leaves. Containers and Default retain
// restoration facts, not ownership of personal descendants or startup choices.
// Raw leaf bytes retain comments for exact baseline restoration. Inventory binds
// the entire projection; the owned fingerprint canonicalizes only Values.
type windowsTerminalProjection struct {
	Schema     int               `json:"dotfiles_windows_terminal"`
	Values     map[string][]byte `json:"values,omitempty"`
	Containers []string          `json:"containers,omitempty"`
	Default    []byte            `json:"default,omitempty"`
}

type windowsTerminalCollection struct {
	ID, Path, Key, Match string
	Fields               []string
}

var windowsTerminalScalars = []string{
	"$schema", "copyFormatting", "copyOnSelect", "firstWindowPreference", "initialRows", "launchMode", "theme", "useAcrylicInTabRow", "windowingBehavior",
	"profiles/defaults/colorScheme", "profiles/defaults/font/face", "profiles/defaults/font/size", "profiles/defaults/historySize", "profiles/defaults/opacity", "profiles/defaults/useAcrylic", "profiles/defaults/padding", "profiles/defaults/antialiasingMode", "profiles/defaults/scrollbarState",
}

func windowsTerminalCollections() []windowsTerminalCollection {
	result := []windowsTerminalCollection{
		{"profile", "profiles/list", "guid", windowsTerminalProfileGUID, []string{"name", "commandline", "startingDirectory"}},
		{"scheme", "schemes", "name", "rose-pine", []string{"background", "foreground", "cursorColor", "selectionBackground", "black", "red", "green", "yellow", "blue", "purple", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightPurple", "brightCyan", "brightWhite"}},
		{"theme", "themes", "name", "rose-pine", []string{"tab/background", "tab/iconStyle", "tab/showCloseButton", "tab/unfocusedBackground", "tabRow/background", "tabRow/unfocusedBackground", "window/applicationTheme", "window/experimental.rainbowFrame", "window/frame", "window/unfocusedFrame", "window/useMica"}},
	}
	result = append(result, windowsTerminalCollection{"close-tab", "actions", "id", "Dotfiles.CloseTab", []string{"command"}})
	for _, key := range []string{"ctrl+c", "ctrl+v", "ctrl+shift+f", "alt+shift+d", "ctrl+shift+w", "ctrl+t", "ctrl+tab", "ctrl+right", "ctrl+shift+tab", "ctrl+left", "ctrl+shift+up", "ctrl+shift+down", "ctrl+shift+right", "ctrl+shift+left", "ctrl+alt+w"} {
		result = append(result, windowsTerminalCollection{"binding:" + key, "keybindings", "keys", key, []string{"id"}})
	}
	return result
}

func windowsTerminalProjectionData(p windowsTerminalProjection) ([]byte, error) {
	if len(p.Values) == 0 && len(p.Containers) == 0 && len(p.Default) == 0 {
		return nil, nil
	}
	p.Schema = 1
	p.Containers = sortedUnique(p.Containers)
	data, err := json.Marshal(p)
	if len(data) > maxProfileBytes {
		return nil, errors.New("Windows Terminal owned projection exceeds its saved document bound")
	}
	return data, err
}

func decodeWindowsTerminalProjection(data []byte) (windowsTerminalProjection, error) {
	p := windowsTerminalProjection{Schema: 1, Values: map[string][]byte{}}
	if len(data) == 0 {
		return p, nil
	}
	if err := Decode(data, &p); err != nil {
		return p, err
	}
	if p.Schema != 1 {
		return p, errors.New("invalid Windows Terminal projection schema")
	}
	allowed := map[string]bool{}
	containers := map[string]bool{"profiles": true, "profiles/defaults": true, "profiles/defaults/font": true}
	for _, field := range windowsTerminalScalars {
		allowed[field] = true
	}
	for _, collection := range windowsTerminalCollections() {
		containers[collection.Path], containers[collection.ID] = true, true
		for _, field := range collection.Fields {
			allowed[collection.ID+"/"+field] = true
			parts := strings.Split(field, "/")
			for i := 1; i < len(parts); i++ {
				containers[collection.ID+"/"+strings.Join(parts[:i], "/")] = true
			}
		}
	}
	for field, value := range p.Values {
		if !allowed[field] {
			return p, errors.New("Windows Terminal projection contains an unowned field")
		}
		if _, err := windowsTerminalValue(value); err != nil {
			return p, err
		}
	}
	seen := map[string]bool{}
	for _, container := range p.Containers {
		if !containers[container] || seen[container] {
			return p, errors.New("Windows Terminal projection has invalid or duplicate containers")
		}
		seen[container] = true
	}
	if len(p.Default) > 0 {
		value, err := windowsTerminalValue(p.Default)
		var name string
		if err != nil || json.Unmarshal(value, &name) != nil {
			return p, errors.New("Windows Terminal defaultProfile must be a string")
		}
	}
	return p, nil
}

func windowsTerminalValue(raw []byte) ([]byte, error) {
	clean, _, err := jsoncMask(raw)
	if err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, clean); err != nil {
		return nil, err
	}
	return compact.Bytes(), nil
}

type windowsTerminalCanonicalValue struct {
	JSON   json.RawMessage
	Number string
}

func windowsTerminalOwned(data []byte) (map[string]windowsTerminalCanonicalValue, error) {
	p, err := decodeWindowsTerminalProjection(data)
	if err != nil {
		return nil, err
	}
	values := map[string]windowsTerminalCanonicalValue{}
	for field, value := range p.Values {
		clean, err := windowsTerminalValue(value)
		if err != nil {
			return nil, err
		}
		value := windowsTerminalCanonicalValue{JSON: clean}
		// Terminal 1.25 constructs CloseTabArgs for the string form and writes
		// {"action":"closeTab"}. Keep the existing string fingerprint and raw
		// restoration bytes; additional arguments still change ownership.
		if field == "close-tab/command" && bytes.Equal(clean, []byte(`{"action":"closeTab"}`)) {
			value.JSON = json.RawMessage(`"closeTab"`)
		}
		if len(clean) > 0 && (clean[0] == '-' || clean[0] >= '0' && clean[0] <= '9') {
			numberText := string(clean)
			bounded := len(numberText) <= 128
			if _, exponent, found := strings.Cut(strings.ToLower(numberText), "e"); found {
				n, err := strconv.Atoi(exponent)
				bounded = bounded && err == nil && n >= -1000 && n <= 1000
			}
			if bounded {
				if number, ok := new(big.Rat).SetString(numberText); ok {
					value = windowsTerminalCanonicalValue{Number: number.RatString()}
				}
			}
		}
		if strings.HasPrefix(field, "scheme/") || strings.HasPrefix(field, "theme/") {
			var color string
			if json.Unmarshal(clean, &color) == nil && strings.HasPrefix(color, "#") {
				color = strings.ToLower(color)
				if len(color) == 9 && strings.HasSuffix(color, "ff") {
					color = color[:7]
				}
				value.JSON, err = json.Marshal(color)
				if err != nil {
					return nil, err
				}
			}
		}
		values[field] = value
	}
	return values, nil
}

func profileBlockFingerprint(id string, blocks [][]byte) (string, error) {
	if id != windowsTerminalResource {
		return digest(blocks)
	}
	values := []map[string]windowsTerminalCanonicalValue{}
	for _, block := range blocks {
		owned, err := windowsTerminalOwned(block)
		if err != nil {
			return "", err
		}
		values = append(values, owned)
	}
	return digest(values)
}

func profileBlockPresent(id string, block []byte) (bool, error) {
	if id != windowsTerminalResource {
		return len(block) != 0, nil
	}
	p, err := decodeWindowsTerminalProjection(block)
	return len(p.Values) > 0, err
}

func profileBlockHealthy(id string, block, want []byte) (bool, error) {
	if id != windowsTerminalResource {
		return bytes.Equal(block, want), nil
	}
	actual, err := decodeWindowsTerminalProjection(block)
	if err != nil {
		return false, err
	}
	desired, err := decodeWindowsTerminalProjection(want)
	if err != nil {
		return false, err
	}
	got, err := profileBlockFingerprint(id, [][]byte{block})
	if err != nil {
		return false, err
	}
	expected, err := profileBlockFingerprint(id, [][]byte{want})
	if err != nil {
		return false, err
	}
	return got == expected && bytes.Equal(actual.Default, desired.Default), nil
}

func extractWindowsTerminal(data []byte) ([]byte, error) {
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
	if err := rejectWindowsTerminalLegacyBindings(data); err != nil {
		return nil, err
	}
	p := windowsTerminalProjection{Schema: 1, Values: map[string][]byte{}}
	for _, path := range []string{"profiles", "profiles/defaults", "profiles/defaults/font"} {
		value, present, err := windowsTerminalGet(data, path)
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
		value, present, err := windowsTerminalGet(data, field)
		if err != nil {
			return nil, err
		}
		if present {
			p.Values[field] = bytes.Clone(value)
		}
	}
	if value, present, err := windowsTerminalGet(data, "defaultProfile"); err != nil {
		return nil, err
	} else if present {
		p.Default = bytes.Clone(value)
	}
	for _, collection := range windowsTerminalCollections() {
		array, present, err := windowsTerminalGet(data, collection.Path)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		p.Containers = append(p.Containers, collection.Path)
		entry, _, err := windowsTerminalEntry(array, collection)
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
				value, present, err := windowsTerminalGet(entry, path)
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
			value, present, err := windowsTerminalGet(entry, field)
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
