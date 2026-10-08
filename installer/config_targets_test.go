package installer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestConfigurationManifestCoversEveryConfigResourceWithCanonicalSources(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	m, err := DefaultConfigManifest(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range m.Targets {
		if strings.Contains(target.Source, "chezmoi") || strings.HasPrefix(target.Source, "home/") {
			t.Fatal("native configuration uses a mirrored compatibility source", target)
		}
		info, err := os.Stat(filepath.Join("..", target.Source))
		if err != nil || info.IsDir() != target.Directory {
			t.Fatal("missing or mistyped canonical configuration source", target, err)
		}
	}
}

func TestWindowsConfigurationUsesIndependentlyRedirectedKnownFolders(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	m, err := DefaultConfigManifest(c)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	folders := ConfigFolders{Home: filepath.Join(root, "profile"), Config: filepath.Join(root, "configuration"), LocalAppData: filepath.Join(root, "redirected local"), AppData: filepath.Join(root, "redirected roaming"), Documents: filepath.Join(root, "redirected documents")}
	for resource, destinations := range map[string][]string{
		"config.nvim":    {filepath.Join(folders.LocalAppData, "nvim")},
		"config.herdr":   {filepath.Join(folders.AppData, "herdr", "config.toml")},
		"config.lazygit": {filepath.Join(folders.LocalAppData, "lazygit", "config.yml")},
	} {
		targets, err := m.Resolve(c, Context{OS: "windows", Arch: "amd64"}, folders, filepath.Join(root, "source"), resource)
		if err != nil || len(targets) != len(destinations) {
			t.Fatal(resource, targets, err)
		}
		for _, target := range targets {
			wantMode := "copy"
			if resource == "config.nvim" {
				wantMode = "junction"
			}
			if !slices.Contains(destinations, target.Destination) || target.Mode != wantMode {
				t.Fatal("known folder was guessed or Windows requires symlink privileges", target)
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("configuration discovery created directories", entries, err)
	}
}

func TestConfigurationResolutionRejectsMissingFoldersAndSourceOverlap(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	m, err := DefaultConfigManifest(c)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, folders := range []ConfigFolders{{}, {Home: root, Config: filepath.Join(root, "source")}} {
		if _, err := m.Resolve(c, Context{OS: "linux", Arch: "amd64"}, folders, filepath.Join(root, "source"), "config.nvim"); err == nil {
			t.Fatal("invalid configuration destination was accepted", folders)
		}
	}
	folders := ConfigFolders{Home: filepath.Join(root, "home"), Config: filepath.Join(root, "home", ".config")}
	if _, err := m.Resolve(c, Context{OS: "linux", Arch: "amd64"}, folders, filepath.Join(root, "source"), "config.ghostty"); err != nil {
		t.Fatal("explicit Linux GUI selection failed without session flags", err)
	}
}

func TestConfigurationManifestRejectsEscapeDuplicateAndUnownedTargets(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ConfigManifest){
		func(m *ConfigManifest) { m.Targets[0].Source = "../outside" },
		func(m *ConfigManifest) { m.Targets[0].Path = "../outside" },
		func(m *ConfigManifest) { m.Targets[0].Path = "C:/outside" },
		func(m *ConfigManifest) { m.Targets[0].Path = "tmux/../../outside" },
		func(m *ConfigManifest) { m.Targets = append(m.Targets, m.Targets[0]) },
		func(m *ConfigManifest) { m.Targets[0].Resource = "tool.node" },
		func(m *ConfigManifest) { m.Targets[0].Folder = "guessed_documents" },
		func(m *ConfigManifest) { m.Schema = 2 },
	} {
		m, err := DefaultConfigManifest(c)
		if err != nil {
			t.Fatal(err)
		}
		change(&m)
		if err := m.Validate(c); err == nil {
			t.Fatal("unsafe mapping accepted", m.Targets[0])
		}
	}
}
