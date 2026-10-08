package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Fresh Debian/Ubuntu acceptance must reach the existing complete Neovim
// lifecycle with neither download nor ZIP extraction tools already installed.
// It never removes packages to manufacture an absent baseline.
func requireFreshLinuxMasonPrerequisitesAbsent(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_LINUX_FRESH") != "1" {
		return
	}
	location, err := DiscoverLinuxAPT(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := linuxAPTInventory(context.Background(), location.query)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"curl", "unzip"} {
		if identity, err := linuxAPTIdentity(name, inventory); err != nil || identity != "" {
			t.Fatal("fresh Neovim acceptance requires native package absence", name, identity, err)
		}
		if _, err := os.Lstat("/usr/bin/" + name); !os.IsNotExist(err) {
			t.Fatal("fresh Neovim acceptance found an unmanaged prerequisite command", name, err)
		}
		t.Logf("fresh Neovim native prerequisite %s: absent", name)
	}
}

func requireFreshLinuxMasonPrerequisitesOwned(t *testing.T, stateDirectory, home string) {
	t.Helper()
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_LINUX_FRESH") != "1" {
		return
	}
	state, err := LoadState(filepath.Join(stateDirectory, "state.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"curl", "unzip"} {
		receipt := state.Receipts["tool."+name]
		if receipt.Ownership != "created" || receipt.Before.Present || !receipt.After.Present || !receipt.After.Healthy || receipt.After.Provider != "apt" {
			t.Fatal("fresh Mason prerequisite was not installed by the owned APT graph", name, receipt)
		}
		t.Logf("fresh Neovim native prerequisite %s: installed and healthy with created APT receipt", name)
	}
}
