package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeControllerNeverInheritsTheCallersProfileDestination(t *testing.T) {
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	foreign := t.TempDir()
	t.Setenv("ZDOTDIR", foreign)
	sentinel := filepath.Join(foreign, ".zshrc")
	if err := os.WriteFile(sentinel, []byte("personal shell settings"), 0600); err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	c, err := NewNativeController(repository, "test", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range c.Driver.(*NativeDriver).Profiles.Targets["integration.shells"] {
		relative, err := filepath.Rel(home, target.Path)
		if err != nil || !filepath.IsLocal(relative) {
			t.Fatalf("controller escaped its supplied home through caller environment: %s", target.Path)
		}
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "personal shell settings" {
		t.Fatal("controller changed the caller's profile", err)
	}
}
