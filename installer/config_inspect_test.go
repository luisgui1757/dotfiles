package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func configInspectionFixture(t *testing.T) (*Catalog, ConfigManifest, ConfigFolders, string) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, home := filepath.Join(root, "source"), filepath.Join(root, "home")
	if err := os.Mkdir(repository, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "config.json"), []byte("original payload"), 0600); err != nil {
		t.Fatal(err)
	}
	c := &Catalog{Schema: 1, Resources: []Resource{{ID: "config.test", Name: "Test configuration", Action: "config"}}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	m := ConfigManifest{Schema: 1, Targets: []ConfigTarget{{Resource: "config.test", Source: "config.json", Folder: "config", Path: "test/config.json"}}}
	return c, m, ConfigFolders{Home: home, Config: filepath.Join(home, ".config")}, repository
}

func TestConfigurationInspectionSeparatesSourceDriftFromOwnedTargetBytes(t *testing.T) {
	c, m, folders, repository := configInspectionFixture(t)
	// Windows copy policy is evaluated using actual files on every test host.
	target := Context{OS: "windows", Arch: "amd64"}
	states, absent, err := inspectConfiguration(c, m, target, folders, repository, "config.test")
	if err != nil || absent.Present || absent.Healthy || absent.Adoptable {
		t.Fatal(absent, err)
	}
	if _, err := os.Lstat(folders.Home); !os.IsNotExist(err) {
		t.Fatal("inspection created target parents", err)
	}
	if err := os.MkdirAll(filepath.Dir(states[0].Target.Destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := copyConfigPayload(states[0].Target.Source, states[0].Target.Destination); err != nil {
		t.Fatal(err)
	}
	_, before, err := inspectConfiguration(c, m, target, folders, repository, "config.test")
	if err != nil || !before.Present || !before.Healthy || !before.Adoptable || before.CompletedOperation != "" {
		t.Fatal("matching pre-existing bytes became ownership proof", before, err)
	}
	if err := os.WriteFile(states[0].Target.Source, []byte("new payload"), 0600); err != nil {
		t.Fatal(err)
	}
	_, after, err := inspectConfiguration(c, m, target, folders, repository, "config.test")
	if err != nil || after.Healthy || !sameArtifact(before, after) || before.Desired == after.Desired {
		t.Fatal("source changes changed the owned artifact identity", before, after, err)
	}
}

func TestPosixConfigurationInspectionRecognizesLiveLinkWithoutClaimingOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX live-link contract; Windows configuration uses copies")
	}
	c, m, folders, repository := configInspectionFixture(t)
	states, _, err := inspectConfiguration(c, m, Context{OS: "linux", Arch: "amd64"}, folders, repository, "config.test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(states[0].Target.Destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(states[0].Target.Source, states[0].Target.Destination); err != nil {
		t.Fatal(err)
	}
	_, observed, err := inspectConfiguration(c, m, Context{OS: "linux", Arch: "amd64"}, folders, repository, "config.test")
	if err != nil || !observed.Present || !observed.Healthy || observed.CompletedOperation != "" {
		t.Fatal(observed, err)
	}
}
