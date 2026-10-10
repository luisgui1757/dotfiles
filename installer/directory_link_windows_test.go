package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsDirectoryJunctionSupportsLongPaths(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, strings.Repeat("a", 120), strings.Repeat("b", 120))
	target := filepath.Join(directory, "payload")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "current")
	if err := createDirectoryLink(target, link); err != nil {
		t.Fatal(err)
	}
	actual, err := resolveConfigPath(link)
	if err != nil || actual != target {
		t.Fatal("native long-path junction resolution", actual, target, err)
	}
	// Managed executables are children of current, not the junction itself.
	// filepath.EvalSymlinks rejects that intermediate mount-point component.
	file := filepath.Join(target, "pwsh.exe")
	writeConfigFixture(t, file, "private path-resolution fixture")
	actual, err = resolveConfigPath(filepath.Join(link, "pwsh.exe"))
	if err != nil || actual != file {
		t.Fatal("native long-path junction child resolution", actual, file, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("junction removal touched its referent", err)
	}
}

func TestWindowsNeovimJunctionPreservesOriginalLinkWithoutReadingItsReferent(t *testing.T) {
	c, d, _ := nativeConfigFixture(t, "copy")
	d.Manifest.Targets[0].Source, d.Manifest.Targets[0].Path = "nvim", "nvim"
	d.Manifest.Targets[0].Directory, d.Manifest.Targets[0].Mode = true, "junction"
	d.Manifest.Targets[0].Platforms = []string{"windows"}
	for i := range c.Catalog.Resources {
		c.Catalog.Resources[i].Platforms = []string{"windows"}
	}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(d.Repository, "nvim", "init.lua"), "-- managed configuration")
	targets, err := d.Manifest.Resolve(d.Catalog, d.Target, d.Folders, d.Repository, "config.test")
	if err != nil {
		t.Fatal(err)
	}
	destination := targets[0].Destination
	original := filepath.Join(filepath.Dir(d.Repository), "original-neovim")
	writeConfigFixture(t, filepath.Join(original, "init.lua"), "-- personal configuration")
	file, err := os.Create(filepath.Join(original, "large-unrelated-file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxConfigBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(original, destination); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(destination)
	if err != nil || before.Kind != "junction" {
		t.Fatal("junction snapshot traversed its referent", before, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"config.test"}})
	link, err := os.Readlink(destination)
	if err != nil || link != filepath.Join(d.Repository, "nvim") {
		t.Fatal("did not publish native junction", link, err)
	}
	writeConfigFixture(t, filepath.Join(destination, "lazy-lock.json"), "{}")
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("runtime lock write invalidated the junction", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	after, err := snapshotConfig(destination)
	if err != nil || after != before {
		t.Fatal("original junction was not restored", before, after, err)
	}
	data, err := os.ReadFile(filepath.Join(original, "init.lua"))
	if err != nil || string(data) != "-- personal configuration" {
		t.Fatal("changed the original link referent", string(data), err)
	}
}

func TestWindowsConfigurationResolvesAJunctionKnownFolder(t *testing.T) {
	c, d, _ := nativeConfigFixture(t, "copy")
	physical := filepath.Join(filepath.Dir(d.Repository), "actual-config")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	redirected := filepath.Join(filepath.Dir(d.Repository), "redirected-config")
	if err := createDirectoryLink(physical, redirected); err != nil {
		t.Fatal(err)
	}
	d.Folders.Config = redirected
	states, _, err := inspectConfiguration(d.Catalog, d.Manifest, d.Target, d.Folders, d.Repository, "config.test")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(physical, filepath.FromSlash(d.Manifest.Targets[0].Path))
	if states[0].Target.Destination != want {
		t.Fatal("junction parent was not physically bound", states[0].Target.Destination, want)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(redirected); err != nil {
		t.Fatal("removed the user's redirected known folder", err)
	}
}
