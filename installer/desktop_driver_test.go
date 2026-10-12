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
	pin.Commands = map[string]string{"ghostty": "code"}
	pin.RequiredFiles = []string{"code", "icon.png", "usr/share/icons/hicolor/256x256/apps/com.mitchellh.ghostty.png"}
	// Fixed recipe, including the declared upstream icon location.
	data := archiveTar(t, []archiveFixtureEntry{{Name: "code", Text: "#!/bin/sh\nexit 0\n", Mode: 0755}, {Name: "icon.png", Text: "icon", Mode: 0644}, {Name: "usr/share/icons/hicolor/256x256/apps/com.mitchellh.ghostty.png", Text: "icon", Mode: 0644}})
	pin, client, _ = archivePinFor(t, data, "tar.gz")
	pin.Commands = map[string]string{"ghostty": "code"}
	pin.RequiredFiles = []string{"code", "usr/share/icons/hicolor/256x256/apps/com.mitchellh.ghostty.png"}
	state := filepath.Join(root, "state")
	a := &ArchiveDriver{Directory: filepath.Join(state, "packages"), Pins: map[string]ArchivePin{"tool.ghostty": pin}, Client: client}
	ac := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"tool.ghostty"}}, {ID: "tool.ghostty", Name: "Ghostty", Action: "archive"}}}, Context: Context{OS: "linux", Arch: "arm64"}, Source: "desktop-archive", Home: filepath.Join(root, "archive-home"), StatePath: filepath.Join(root, "archive-state.json"), Driver: a}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	d := &DesktopDriver{Target: ac.Context, Folders: ConfigFolders{Home: filepath.Join(root, "home"), Data: filepath.Join(root, "redirected data ü")}, Directory: state, Archives: a}
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "app", Name: "App", Capability: true, Requires: []string{"desktop.ghostty"}}, {ID: "desktop.ghostty", Name: "Ghostty launcher", Action: "desktop"}}}, Context: ac.Context, Source: "desktop-fixture", Home: d.Folders.Home, StatePath: filepath.Join(state, "state.json"), Driver: d}
	return c, d, filepath.Join(d.Folders.Data, "applications", "dotfiles-ghostty.desktop")
}

func TestDesktopPublicationUsesStablePayloadAndRestoresPersonalLauncher(t *testing.T) {
	c, d, destination := desktopFixture(t)
	writeConfigFixture(t, destination, "personal launcher")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"app"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil || plan.Operations[0].Action != "pending" {
		t.Fatal(plan, err)
	}
	request.Adopt = []string{"desktop.ghostty"}
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
	r := Resource{ID: "desktop.ghostty", Name: "Ghostty", Action: "desktop"}
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
	entry, err := desktopEntry("App", filepath.Join(root, `ü $x %f "x" \ prog`), filepath.Join(root, `icon\x.png`), "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`%%f`, `\\$x`, `\\"x\\"`, `\\\\ prog`, `icon\\x.png`} {
		if !strings.Contains(entry, expected) {
			t.Fatalf("missing %q in %s", expected, entry)
		}
	}
	if _, err := desktopEntry("App", filepath.Join(root, "x=y"), filepath.Join(root, "icon"), "", false); err == nil {
		t.Fatal("accepted forbidden Exec program =")
	}
}
func TestDesktopRejectsMissingKnownProgramsFolder(t *testing.T) {
	_, d, _ := desktopFixture(t)
	d.Target = Context{OS: "windows", Arch: "amd64"}
	pin := d.Archives.Pins["tool.ghostty"]
	pin.RequiredFiles = append(pin.RequiredFiles, "Code.exe")
	d.Archives.Pins["tool.vscode"] = pin
	o, err := d.Observe(context.Background(), Resource{ID: "desktop.vscode", Name: "Code"}, Receipt{})
	if err == nil && !o.Unknown {
		t.Fatal("invented a Start menu known folder")
	}
}

// Field report 2026-10-21: without StartupWMClass GNOME showed Zen under a
// generic icon and every launcher click opened another window. The classes are
// Ghostty's GTK application ID and the pinned Zen archive's RemotingName.
// OpenWhispr showed the same generic icon (field report 2026-10-23): its main
// process calls app.setDesktopName("open-whispr.desktop") and Electron derives
// WM_CLASS from that name.
func TestLinuxLaunchersDeclareTheirWindowClass(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	d := DesktopDriver{Target: linux, Folders: ConfigFolders{Home: root}, Archives: &ArchiveDriver{Directory: filepath.Join(root, "packages"), Pins: pins}}
	for id, want := range map[string]string{"integration.ghostty": "com.mitchellh.ghostty", "desktop.zen": "zen", "desktop.openwhispr": "open-whispr"} {
		p, err := d.recipe(id)
		if err != nil {
			t.Fatal(id, err)
		}
		_, class, ok := strings.Cut(p.Contents, "\nStartupWMClass=")
		class, _, _ = strings.Cut(class, "\n")
		if ok != (want != "") || class != want || strings.Count(p.Contents, "StartupWMClass=") > 1 {
			t.Fatalf("%s launcher declares the wrong window class:\n%s", id, p.Contents)
		}
	}
}
