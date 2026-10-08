package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopVSCodeWitnessRequiresFreshLaunchEvidence(t *testing.T) {
	home := t.TempDir()
	folders := ConfigFolders{Home: home}
	first := desktopVSCodeWitness(t, folders)
	previous := []byte(`{"Theme":"Rosé Pine","Font":"Hack Nerd Font","TerminalFont":"Hack Nerd Font"}`)
	if err := os.WriteFile(first, previous, 0600); err != nil {
		t.Fatal(err)
	}
	// Match the existing per-launch extension cleanup while deliberately retaining
	// the first launch's witness. It must not authorize the next application's use.
	extension := filepath.Join(home, ".vscode", "extensions", "dotfiles-fixture.native-settings-1.0.0")
	if err := os.RemoveAll(extension); err != nil {
		t.Fatal(err)
	}
	second := desktopVSCodeWitness(t, folders)
	if first == second {
		t.Fatal("second launch reused the first settings witness")
	}
	if _, err := os.Lstat(second); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("second launch already has settings evidence before host activation", err)
	}
	if got, err := os.ReadFile(first); err != nil || !bytes.Equal(got, previous) {
		t.Fatal("fresh launch damaged prior witness bytes", err)
	}
}
