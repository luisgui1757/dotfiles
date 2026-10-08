package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func desktopFixture(t *testing.T) (Controller, *DesktopDriver, string) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "space ü $quote' root")
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "code", Text: "#!/bin/sh\nexit 0\n", Mode: 0755}, {Name: "icon.png", Text: "icon", Mode: 0644}}), "tar.gz")
	pin.Commands = map[string]string{"code": "code"}
	pin.RequiredFiles = []string{"code", "icon.png", "resources/app/resources/linux/code.png"}
	// Fixed recipe, including the declared upstream icon location.
	data := archiveTar(t, []archiveFixtureEntry{{Name: "code", Text: "#!/bin/sh\nexit 0\n", Mode: 0755}, {Name: "icon.png", Text: "icon", Mode: 0644}, {Name: "resources/app/resources/linux/code.png", Text: "icon", Mode: 0644}})
	pin, client, _ = archivePinFor(t, data, "tar.gz")
	pin.Commands = map[string]string{"code": "code"}
	pin.RequiredFiles = []string{"code", "resources/app/resources/linux/code.png"}
	state := filepath.Join(root, "state")
	a := &ArchiveDriver{Directory: filepath.Join(state, "packages"), Pins: map[string]ArchivePin{"tool.vscode": pin}, Client: client}
	ac := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"tool.vscode"}}, {ID: "tool.vscode", Name: "Code", Action: "archive"}}}, Context: Context{OS: "linux", Arch: "arm64"}, Source: "desktop-archive", Home: filepath.Join(root, "archive-home"), StatePath: filepath.Join(root, "archive-state.json"), Driver: a}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	d := &DesktopDriver{Target: ac.Context, Folders: ConfigFolders{Home: filepath.Join(root, "home"), Data: filepath.Join(root, "redirected data ü")}, Directory: state, Archives: a}
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"desktop.vscode"}}, {ID: "desktop.vscode", Name: "Code launcher", Action: "desktop"}}}, Context: ac.Context, Source: "desktop-fixture", Home: d.Folders.Home, StatePath: filepath.Join(state, "state.json"), Driver: d}
	return c, d, filepath.Join(d.Folders.Data, "applications", "dotfiles-vscode.desktop")
}

func TestDesktopPublicationUsesStablePayloadAndRestoresPersonalLauncher(t *testing.T) {
	c, d, destination := desktopFixture(t)
	writeConfigFixture(t, destination, "personal launcher")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"app"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil || plan.Operations[0].Action != "pending" {
		t.Fatal(plan, err)
	}
	request.Adopt = []string{"desktop.vscode"}
	dispatchApproved(t, c, request)
	data, err := os.ReadFile(destination)
	// Desktop-entry escaping doubles native backslashes twice. Normalize those
	// separators only for this stable-generation assertion on Windows fixtures.
	portable := strings.ReplaceAll(string(data), `\\\\`, "/")
	if err != nil || !strings.Contains(portable, "/current/code") || !strings.Contains(string(data), "Exec=\"") {
		t.Fatal(string(data), err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err = os.ReadFile(destination)
	if err != nil || string(data) != "personal launcher" {
		t.Fatal(string(data), err)
	}
	_ = d
}

func TestDesktopChangedLauncherIsPreserved(t *testing.T) {
	c, _, destination := desktopFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	writeConfigFixture(t, destination, "user edit")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "user edit" {
		t.Fatal(string(data), err)
	}
}
func TestDesktopPreviewDoesNotPrepareFilesOrRequireInstalledArchive(t *testing.T) {
	_, d, _ := desktopFixture(t)
	d.Directory = filepath.Join(filepath.Dir(d.Directory), "fresh-state")
	d.Archives = &ArchiveDriver{Directory: filepath.Join(d.Directory, "packages"), Pins: d.Archives.Pins}
	r := Resource{ID: "desktop.vscode", Name: "Code", Action: "desktop"}
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil || o.Present || o.ApplyBlocked != "" || o.Desired == "" {
		t.Fatal(o, err)
	}
	if _, err := os.Stat(d.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview mutated", err)
	}
}
func TestDesktopEntryEscapesFieldCodesAndShellMetacharacters(t *testing.T) {
	root := t.TempDir()
	entry, err := desktopEntry("App", filepath.Join(root, `ü $x %f "x" \ prog`), filepath.Join(root, `icon\x.png`))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`%%f`, `\\$x`, `\\"x\\"`, `\\\\ prog`, `icon\\x.png`} {
		if !strings.Contains(entry, expected) {
			t.Fatalf("missing %q in %s", expected, entry)
		}
	}
	if _, err := desktopEntry("App", filepath.Join(root, "x=y"), filepath.Join(root, "icon")); err == nil {
		t.Fatal("accepted forbidden Exec program =")
	}
}
func TestDesktopRejectsMissingKnownProgramsFolder(t *testing.T) {
	_, d, _ := desktopFixture(t)
	d.Target = Context{OS: "windows", Arch: "amd64"}
	pin := d.Archives.Pins["tool.vscode"]
	pin.RequiredFiles = append(pin.RequiredFiles, "Code.exe")
	d.Archives.Pins["tool.vscode"] = pin
	o, err := d.Observe(context.Background(), Resource{ID: "desktop.vscode", Name: "Code"}, Receipt{})
	if err == nil && !o.Unknown {
		t.Fatal("invented a Start menu known folder")
	}
}
