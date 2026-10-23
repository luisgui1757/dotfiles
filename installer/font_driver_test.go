package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fontProcessFixture struct {
	registrations    []string
	active, external bool
	failNext         bool
	mutations        int
	paths            []string
}

func fontFixture(t *testing.T) (Controller, *FontDriver, *fontProcessFixture) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "font space ü")
	entries := []archiveFixtureEntry{}
	names := []string{}
	for _, face := range hackFontFaces() {
		entries = append(entries, archiveFixtureEntry{Name: face.File, Text: "verified face " + face.PostScript, Mode: 0644})
		names = append(names, face.File)
	}
	pin, client, _ := archivePinFor(t, archiveTar(t, entries), "tar.gz")
	pin.Commands = map[string]string{}
	pin.BinDirs = []string{}
	pin.RequiredFiles = names
	state := filepath.Join(root, "state")
	a := &ArchiveDriver{Directory: filepath.Join(state, "packages"), Pins: map[string]ArchivePin{"font.hack": pin}, Client: client}
	ac := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "font", Name: "Font", Capability: true, Requires: []string{"font.hack"}}, {ID: "font.hack", Name: "Font archive", Action: "archive"}}}, Context: Context{OS: "windows", Arch: "amd64"}, Source: "font-archive", Home: filepath.Join(root, "archivehome"), StatePath: filepath.Join(root, "archive-state.json"), Driver: a}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	f := &fontProcessFixture{registrations: make([]string, len(names))}
	d := &FontDriver{Target: ac.Context, Folders: ConfigFolders{Home: filepath.Join(root, "home"), LocalAppData: filepath.Join(root, "redirected local ü")}, Directory: state, Archives: a, PowerShell: filepath.Join(root, "powershell.exe")}
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		var input struct{ Paths []string }
		if err := json.Unmarshal(command.Input, &input); err != nil {
			return nil, err
		}
		f.paths = input.Paths
		result := make([]fontNativeFace, len(names))
		for i, face := range hackFontFaces() {
			result[i].Registration = f.registrations[i]
			if f.active || f.external {
				result[i].Name = face.PostScript
				result[i].Glyph = true
				hash := strings.Repeat("a", 64)
				if !f.external {
					var err error
					hash, err = fontFileHash(input.Paths[i])
					if err != nil {
						return nil, err
					}
				}
				result[i].Hash = hash
			}
		}
		return json.Marshal(result)
	}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.mutations++
		var input struct {
			Paths, Before, Hashes []string
			Install               bool
		}
		if err := json.Unmarshal(command.Input, &input); err != nil {
			return nil, err
		}
		for i := range names {
			if f.registrations[i] != input.Before[i] || input.Before[i] != "" && input.Before[i] != input.Paths[i] {
				return nil, errors.New("changed registry")
			}
		}
		for i := range names {
			if input.Install {
				f.registrations[i] = input.Paths[i]
			} else {
				f.registrations[i] = ""
			}
			if f.failNext {
				f.failNext = false
				return nil, errors.New("native interruption after one exact value")
			}
		}
		f.active = input.Install
		return nil, nil
	}
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "font", Name: "Font", Capability: true, Requires: []string{"tool.font"}}, {ID: "tool.font", Name: "Font activation", Action: "font"}}}, Context: ac.Context, Source: "font-fixture", Home: d.Folders.Home, StatePath: filepath.Join(state, "state.json"), Driver: d}
	return c, d, f
}
func TestFontNativePublicationAndRemovalOwnsOnlyExactFaces(t *testing.T) {
	c, _, f := fontFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	if f.mutations != 1 {
		t.Fatal(f.mutations)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for i, path := range f.paths {
		if f.registrations[i] != "" {
			t.Fatal("registration retained")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(path, err)
		}
	}
}
func TestFontReusesGenuinePreexistingFacesWithoutOwnership(t *testing.T) {
	c, _, f := fontFixture(t)
	f.external = true
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Receipts["tool.font"].Ownership == "created" || f.mutations != 0 {
		t.Fatal(state, err, f.mutations)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if f.mutations != 0 {
		t.Fatal("removed external font")
	}
}
func TestFontChangedRegistrationRetainedOnRemoval(t *testing.T) {
	c, _, f := fontFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	f.registrations[0] = filepath.Join(c.Home, "personal-font.ttf")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if f.mutations != 1 || !strings.HasSuffix(f.registrations[0], "personal-font.ttf") {
		t.Fatal("changed registration mutated", f)
	}
}
func TestFontInterruptedRegistrationResumesOwnedFiles(t *testing.T) {
	c, _, f := fontFixture(t)
	f.failNext = true
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"font"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err = c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("native interruption ignored")
	}
	request.ExpectedPlan = ""
	request.Retry = true
	plan, err = c.Preview(context.Background(), request)
	if err != nil || plan.ResourceResume == nil {
		t.Fatal(plan, err)
	}
	request.ExpectedPlan = plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "needs-action" {
		t.Fatal(result, err)
	}
	request.ExpectedPlan = ""
	request.Retry = true
	dispatchApproved(t, c, request)
	if f.mutations != 2 || !f.active {
		t.Fatal("native activation was not completed", f)
	}
}
func TestFontInterruptedRegistrationCanRestoreExactPartialKeys(t *testing.T) {
	c, d, f := fontFixture(t)
	f.failNext = true
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"font"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err = c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("interruption ignored")
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["tool.font"]
	if _, err := (&ConfigDriver{Directory: d.Directory}).ObserveRestore(context.Background(), "tool.font", receipt); !errors.Is(err, ErrNoResourceRestore) {
		t.Fatal("ordinary completed file journal became restorable", err)
	}
	observed, err := d.ObserveRestore(context.Background(), "tool.font", receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.RestoreResource(context.Background(), "tool.font", observed, receipt); err != nil {
		t.Fatal(err)
	}
	for i, path := range f.paths {
		if f.registrations[i] != "" {
			t.Fatal("partial registration retained")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("partial font retained", path, err)
		}
	}
}
func TestFontMissingInspectionDoesNotInferFreshAbsence(t *testing.T) {
	c, d, _ := fontFixture(t)
	d.Query = func(context.Context, nativeCommand) ([]byte, error) { return nil, os.ErrNotExist }
	r, _ := c.Catalog.Resource("tool.font")
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil || !o.Unknown || o.Pending == "" || o.Present {
		t.Fatal(o, err)
	}
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	if err != nil || plan.Operations[0].Action != "pending" {
		t.Fatal(plan, err)
	}
}
func TestFontChangedOwnedFilePreserved(t *testing.T) {
	c, _, f := fontFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	writeConfigFixture(t, f.paths[0], "personal changed font")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if f.mutations != 1 {
		t.Fatal("changed owned font was unregistered")
	}
	data, err := os.ReadFile(f.paths[0])
	if err != nil || string(data) != "personal changed font" {
		t.Fatal(string(data), err)
	}
}
func TestFontCharsetRequiresActualNerdGlyph(t *testing.T) {
	if !fontCharsetContains("20-7e f000-f200", 0xf120) || fontCharsetContains("20-7e f000-f100", 0xf120) {
		t.Fatal("font charset did not discriminate Nerd glyph")
	}
}

type fontPrerequisiteFixture struct {
	*FontDriver
	prerequisite *fileDriver
}

func (d *fontPrerequisiteFixture) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID == "infra.fontconfig" {
		return d.prerequisite.Observe(ctx, r, receipt)
	}
	return d.FontDriver.Observe(ctx, r, receipt)
}
func (d *fontPrerequisiteFixture) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if r.ID == "infra.fontconfig" {
		return d.prerequisite.Apply(ctx, r, op, receipt)
	}
	return d.FontDriver.Apply(ctx, r, op, receipt)
}
func (d *fontPrerequisiteFixture) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID == "infra.fontconfig" {
		return d.prerequisite.Remove(ctx, r, receipt)
	}
	return d.FontDriver.Remove(ctx, r, receipt)
}
func TestFontInspectionPrerequisiteInstallsBeforeFreshPreviewAndRemainsRemovable(t *testing.T) {
	c, d, _ := fontFixture(t)
	dir := filepath.Join(filepath.Dir(d.Directory), "prerequisite")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	prerequisite := &fileDriver{dir: dir, statePath: c.StatePath, home: c.Home}
	d.Target = Context{OS: "linux", Arch: "amd64"}
	c.Context = d.Target
	d.FontCache = filepath.Join(dir, "fc-cache")
	d.FontMatch = filepath.Join(dir, "fc-match")
	_, _, paths, _, err := d.configuration(Resource{ID: "tool.font", Name: "Font"})
	if err != nil {
		t.Fatal(err)
	}
	active := false
	d.Run = func(context.Context, nativeCommand) ([]byte, error) {
		_, err := os.Stat(paths[0])
		active = err == nil
		return nil, nil
	}
	d.Query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if _, err := os.Stat(filepath.Join(dir, "infra.fontconfig")); err != nil {
			return nil, err
		}
		if !active {
			return []byte("Fallback\n/not-read\n20-7e\n"), nil
		}
		for i, face := range hackFontFaces() {
			style := strings.ReplaceAll(face.Style, "BoldItalic", "Bold Italic")
			if command.Arguments[2] == face.Family+":style="+style {
				return []byte(face.PostScript + "\n" + paths[i] + "\nf000-f200\n"), nil
			}
		}
		return nil, errors.New("unexpected fontconfig query")
	}
	c.Catalog.Resources[1].Requires = []string{"infra.fontconfig"}
	c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "infra.fontconfig", Name: "Fontconfig", Action: "native"})
	c.Driver = &fontPrerequisiteFixture{d, prerequisite}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"font"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil || len(plan.Operations) != 2 || plan.Operations[0].Action != "install" || plan.Operations[1].Action != "pending" {
		t.Fatal(plan, err)
	}
	request.ExpectedPlan = plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "needs-action" {
		t.Fatal(result, err)
	}
	request.ExpectedPlan = ""
	request.Retry = true
	dispatchApproved(t, c, request)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Stat(filepath.Join(dir, "infra.fontconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ordinary prerequisite became permanently retained", err)
	}
}

func TestFontRepairResumesAfterUnregistrationBeforeNewFileJournal(t *testing.T) {
	c, _, f := fontFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"font"}}
	dispatchApproved(t, c, request)
	f.active = false
	f.failNext = true
	plan, err := c.Preview(context.Background(), request)
	if err != nil || plan.Operations[0].Action != "repair" {
		t.Fatal(plan, err)
	}
	request.ExpectedPlan = plan.ID
	if _, err = c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("unregistration interruption ignored")
	}
	request.ExpectedPlan = ""
	request.Retry = true
	plan, err = c.Preview(context.Background(), request)
	if err != nil || plan.ResourceResume == nil {
		t.Fatal(plan, err)
	}
	request.ExpectedPlan = plan.ID
	if result, err := c.Dispatch(context.Background(), request); err != nil || result.Status != "needs-action" {
		t.Fatal(result, err)
	}
	request.ExpectedPlan = ""
	dispatchApproved(t, c, request)
	if !f.active || f.mutations != 4 {
		t.Fatal("native repair did not finish exact registration lifecycle", f)
	}
}
