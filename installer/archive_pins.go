package installer

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
)

//go:embed archive-pins.json
var archivePinData []byte

type ArchiveCatalog struct {
	Schema    int                              `json:"schema"`
	Resources map[string]map[string]ArchivePin `json:"resources"`
}

func DefaultArchivePins(target NativePlatform) (map[string]ArchivePin, error) {
	if err := target.Context.Validate(); err != nil {
		return nil, err
	}
	var catalog ArchiveCatalog
	if err := Decode(archivePinData, &catalog); err != nil {
		return nil, err
	}
	if catalog.Schema != 1 {
		return nil, errors.New("unsupported archive catalog schema")
	}
	// A prepared Python tool must be rebuilt when its reviewed interpreter pin
	// or fixed console preparation changes, even if its source has not changed.
	for platform, pin := range catalog.Resources["tool.latex2text"] {
		python, ok := catalog.Resources["tool.python"][platform]
		if !ok || pin.Latex2text == nil {
			return nil, fmt.Errorf("latex2text on %s requires its Python archive", platform)
		}
		pin.Latex2text.PythonPinID = archivePinID(python)
		pin.Latex2text.ConsoleRevision = 1
		catalog.Resources["tool.latex2text"][platform] = pin
	}
	for platform, pin := range catalog.Resources["tool.yamllint"] {
		python, ok := catalog.Resources["tool.python"][platform]
		if !ok {
			return nil, fmt.Errorf("yamllint on %s requires its Python archive", platform)
		}
		bound, err := bindYamllintPython(pin, python, platform)
		if err != nil {
			return nil, err
		}
		catalog.Resources["tool.yamllint"][platform] = bound
	}
	if pin, ok := catalog.Resources["tool.rust"]["windows/amd64"]; ok && pin.WindowsRustLauncher != nil {
		pin.WindowsRustLauncher.SHA256 = target.WindowsRustLauncherSHA256
		catalog.Resources["tool.rust"]["windows/amd64"] = pin
	}
	result := map[string]ArchivePin{}
	for id, platforms := range catalog.Resources {
		if !resourceID.MatchString(id) || len(platforms) == 0 {
			return nil, errors.New("invalid archive catalog resource")
		}
		for platform, pin := range platforms {
			parts := strings.Split(platform, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid archive platform %s", platform)
			}
			if err := (Context{OS: parts[0], Arch: parts[1]}).Validate(); err != nil {
				return nil, fmt.Errorf("invalid archive platform %s: %w", platform, err)
			}
			if err := pin.Validate(); err != nil {
				return nil, fmt.Errorf("archive %s on %s: %w", id, platform, err)
			}
		}
		if pin, ok := platforms[target.OS+"/"+target.Arch]; ok && (pin.Libc == "" || pin.Libc == target.Libc) {
			result[id] = pin
		}
	}
	return result, nil
}
