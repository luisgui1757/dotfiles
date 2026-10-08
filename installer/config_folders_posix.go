//go:build darwin || linux

package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverConfigFolders is read-only. Missing configuration directories are
// legitimate on first run; creation belongs to an approved resource operation.
func DiscoverConfigFolders() (ConfigFolders, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigFolders{}, err
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	zsh := os.Getenv("ZDOTDIR")
	if zsh == "" {
		zsh = home
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	for _, path := range []string{home, config, zsh, data} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return ConfigFolders{}, errors.New("home, XDG_CONFIG_HOME, XDG_DATA_HOME and ZDOTDIR must be absolute canonical paths")
		}
	}
	return ConfigFolders{Home: home, Config: config, Zsh: zsh, Data: data}, nil
}
