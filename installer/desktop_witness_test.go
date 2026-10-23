package installer

import (
	"bytes"
	"encoding/json"
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
	if err := os.WriteFile(first.Path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	// Match the existing per-launch extension cleanup while deliberately retaining
	// the first launch's witness. It must not authorize the next application's use.
	extension := filepath.Join(home, ".vscode", "extensions", "dotfiles-fixture.native-settings-1.0.0")
	if err := os.RemoveAll(extension); err != nil {
		t.Fatal(err)
	}
	second := desktopVSCodeWitness(t, folders)
	if first.Path == second.Path {
		t.Fatal("second launch reused the first settings witness")
	}
	if _, err := os.Lstat(second.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("second launch already has settings evidence before host activation", err)
	}
	if got, err := os.ReadFile(first.Path); err != nil || !bytes.Equal(got, previous) {
		t.Fatal("fresh launch damaged prior witness bytes", err)
	}
}

func TestDesktopVSCodeWitnessRequiresSavedDocumentEvidence(t *testing.T) {
	for _, scenario := range []string{"saved", "settings-only", "reported-error", "unsaved", "wrong-permissions"} {
		t.Run(scenario, func(t *testing.T) {
			witness := newDesktopCodeWitness(t)
			result := map[string]string{
				"Theme": "Rosé Pine", "Font": "Hack Nerd Font", "TerminalFont": "Hack Nerd Font", "UpdateMode": "none",
				"Before": desktopCodeOriginal, "Reopened": desktopCodeEdited,
			}
			if scenario != "unsaved" {
				if err := os.WriteFile(witness.Document, []byte(desktopCodeEdited), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "settings-only":
				delete(result, "Before")
				delete(result, "Reopened")
			case "reported-error":
				result["Error"] = "Document did not close"
			case "wrong-permissions":
				if err := os.Chmod(witness.Document, witness.Mode.Perm()&^0222); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(witness.Document, witness.Mode.Perm()); err != nil {
						t.Error(err)
					}
				})
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if err := witness.verify(data); (err == nil) != (scenario == "saved") {
				t.Fatalf("unexpected document verification result: %v", err)
			}
		})
	}
}
