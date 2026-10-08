//go:build darwin || linux

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeConfigFolderDiscoveryHonorsAbsentXDGPathWithoutWrites(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "redirected configuration")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	data := filepath.Join(home, "redirected data")
	t.Setenv("XDG_DATA_HOME", data)
	zsh := filepath.Join(home, "redirected zsh")
	t.Setenv("ZDOTDIR", zsh)
	folders, err := DiscoverConfigFolders()
	if err != nil || folders.Home != home || folders.Config != config || folders.Zsh != zsh || folders.Data != data {
		t.Fatal(folders, err)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatal("discovery created a missing configuration directory", err)
	}
	if _, err := os.Stat(zsh); !os.IsNotExist(err) {
		t.Fatal("discovery created the missing zsh directory", err)
	}
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Fatal("discovery created the missing data directory", err)
	}
	t.Setenv("XDG_DATA_HOME", "relative/data")
	if _, err := DiscoverConfigFolders(); err == nil {
		t.Fatal("relative XDG data path accepted")
	}
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("ZDOTDIR", "relative/zsh")
	if _, err := DiscoverConfigFolders(); err == nil {
		t.Fatal("relative ZDOTDIR accepted")
	}
	t.Setenv("ZDOTDIR", "")
	t.Setenv("XDG_CONFIG_HOME", "relative/path")
	if _, err := DiscoverConfigFolders(); err == nil {
		t.Fatal("relative XDG configuration path accepted")
	}
}
