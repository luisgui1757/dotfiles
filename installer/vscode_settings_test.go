package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func vscodeSettingsController(t *testing.T) (Controller, *ProfileDriver, string) {
	t.Helper()
	c, d, _ := profileController(t)
	c.Catalog.Resources[0].Requires = []string{vscodeSettingsResource}
	c.Catalog.Resources[1].ID = vscodeSettingsResource
	folders := ConfigFolders{Home: c.Home, Config: filepath.Join(c.Home, "redirected-config"), AppData: filepath.Join(c.Home, "roaming")}
	d.Targets = map[string][]ProfileTarget{}
	configureVSCodeSettings(d, c.Context, folders)
	return c, d, d.Targets[vscodeSettingsResource][0].Path
}

func TestVSCodeJSONCSettingsLifecyclePreservesPersonalCommentsAndFields(t *testing.T) {
	for _, prior := range []string{"", `"workbench.colorTheme" : "personal", /* after theme */`} {
		t.Run(prior, func(t *testing.T) {
			c, d, path := vscodeSettingsController(t)
			original := "\xef\xbb\xbf// personal header\r\n{\r\n" + prior + "\r\n" +
				`"url":"https://example.org/a/*b*/", // inline` + "\r\n" +
				`"nested":{"workbench.colorTheme":"keep",},"huge":9007199254740993,` + "\r\n}\r\n"
			writeConfigFixture(t, path, original)
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if prior != "" {
				preview, err := c.Preview(context.Background(), request)
				if err != nil || operation(t, preview, vscodeSettingsResource).Action != "pending" {
					t.Fatal("personal theme must require adoption", preview, err)
				}
				request.Adopt = []string{vscodeSettingsResource}
			}
			dispatchApproved(t, c, request)
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte(`"workbench.colorTheme":"Rosé Pine"`)) && !bytes.Contains(data, []byte(`"workbench.colorTheme" : "Rosé Pine"`)) {
				t.Fatal("desired theme missing", string(data), err)
			}
			data = bytes.Replace(data, []byte("9007199254740993"), []byte("9007199254740995"), 1)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			d.Targets[vscodeSettingsResource][0].Script = strings.ReplaceAll(d.Targets[vscodeSettingsResource][0].Script, "Rosé Pine", "Rosé Pine Moon")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err = os.ReadFile(path)
			want := strings.Replace(original, "9007199254740993", "9007199254740995", 1)
			if err != nil || string(data) != want {
				t.Fatalf("personal JSONC changed: got %q, want %q: %v", data, want, err)
			}
		})
	}
}

func TestVSCodeJSONCRemovalPreservesAddedComment(t *testing.T) {
	for _, comment := range []string{"", "// personal note\n"} {
		c, _, path := vscodeSettingsController(t)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeConfigFixture(t, path, comment+string(data))
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		data, err = os.ReadFile(path)
		if comment == "" && !os.IsNotExist(err) || comment != "" && (err != nil || !bytes.Contains(data, []byte(comment))) {
			t.Fatal("removal lost comment or left a newly empty document", string(data), err)
		}
	}
}

func TestVSCodeManagedUpdatesPreservePersonalSetting(t *testing.T) {
	for _, prior := range []string{"", `"update.mode":"manual",`} {
		t.Run(prior, func(t *testing.T) {
			c, _, path := vscodeSettingsController(t)
			original := "// personal update preference\n{" + prior + `"editor.fontSize":17}` + "\n"
			writeConfigFixture(t, path, original)
			if err := os.Chmod(path, 0640); err != nil {
				t.Fatal(err)
			}
			// Windows exposes its native writable/read-only mode, not POSIX 0640.
			// Preserve the actual original contents and native mode on every OS.
			baseline, err := snapshotConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if prior != "" {
				preview, err := c.Preview(context.Background(), request)
				if err != nil || operation(t, preview, vscodeSettingsResource).Action != "pending" {
					t.Fatal("personal update setting must require adoption", preview, err)
				}
				request.Adopt = []string{vscodeSettingsResource}
			}
			dispatchApproved(t, c, request)
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte(`"update.mode":"none"`)) {
				t.Fatal("managed application can still update its sealed payload", string(data), err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err = os.ReadFile(path)
			snapshotErr := verifyConfigSnapshot(path, baseline)
			if err != nil || string(data) != original || snapshotErr != nil {
				t.Fatalf("personal settings or native mode changed on removal: got %q, want %q; read: %v; snapshot: %v", data, original, err, snapshotErr)
			}
		})
	}
}

func TestVSCodeUpdatePolicyHandsOverOldSettingsReceipt(t *testing.T) {
	for _, fixture := range []struct {
		name, personal string
		changed        bool
	}{
		{"unchanged", "", false}, {"edited", "", true},
		{"adopted-theme", `"workbench.colorTheme":"My original theme",`, false},
		{"personal-update", `"update.mode":"manual",`, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			changed := fixture.changed
			c, d, path := vscodeSettingsController(t)
			c.Catalog.Resources[0].Requires = []string{"integration.vscode"}
			c.Catalog.Resources[1].ID = "integration.vscode"
			original := "// original personal settings\n{" + fixture.personal + "\"editor.fontSize\":17}\n"
			writeConfigFixture(t, path, original)
			if err := os.Chmod(path, 0640); err != nil {
				t.Fatal(err)
			}
			// Windows exposes its native writable/read-only mode, not POSIX 0640.
			// Preserve the actual original contents and native mode on every OS.
			baseline, err := snapshotConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			oldRequest := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if fixture.name == "adopted-theme" {
				oldRequest.Adopt = []string{"integration.vscode"}
			}
			dispatchApproved(t, c, oldRequest)
			old, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				old = bytes.ReplaceAll(old, []byte("Rosé Pine"), []byte("Personal theme"))
				writeConfigFixture(t, path, string(old))
			}
			c.Catalog.Resources[0].Requires = []string{vscodeSettingsResource}
			c.Catalog.Resources[1].ReplacedBy = vscodeSettingsResource
			c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: vscodeSettingsResource, Name: "Managed settings", Action: "profile", Scope: "user"})
			c.Driver = &NativeDriver{Catalog: c.Catalog, Context: c.Context, Profiles: d, StatePath: c.StatePath}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			preview, err := c.Preview(context.Background(), request)
			if err != nil || operation(t, preview, vscodeSettingsResource).Action != "pending" {
				t.Fatal("successor can publish before old baseline restoration", preview, err)
			}
			if fixture.personal != "" && (!strings.Contains(operation(t, preview, vscodeSettingsResource).Reason, "Abandon") || !strings.Contains(operation(t, preview, vscodeSettingsResource).Reason, "Choose tools")) {
				t.Fatal("handover guidance would strand a required new adoption", operation(t, preview, vscodeSettingsResource))
			}
			if changed {
				if operation(t, preview, "integration.vscode").Action != "retain" {
					t.Fatal("changed settings did not require restoration approval", preview)
				}
				request.RemoveShared = []string{"integration.vscode"}
			}
			_, result, err := executeApproved(t, c, request)
			if err != nil || result.Status != "needs-action" {
				t.Fatal(result, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original {
				t.Fatal("old baseline was not restored before handover", string(data), err)
			}
			if fixture.personal == "" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			} else {
				_, resumed, err := executeApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
				if err != nil || resumed.Status != "needs-action" {
					t.Fatal("resume silently approved new personal settings", resumed, err)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{vscodeSettingsResource}})
			}
			data, err = os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte(`"update.mode":"none"`)) || !bytes.Contains(data, []byte(`"workbench.colorTheme":"Rosé Pine"`)) {
				t.Fatal("new single owner did not install both settings", string(data), err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err = os.ReadFile(path)
			snapshotErr := verifyConfigSnapshot(path, baseline)
			if err != nil || string(data) != original || snapshotErr != nil {
				t.Fatalf("handover lost original settings or native mode: got %q, want %q; read: %v; snapshot: %v", data, original, err, snapshotErr)
			}
			{
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				preserved := false
				for _, saved := range state.Receipts["integration.vscode"].After.Preserved {
					b, err := os.ReadFile(saved)
					if err == nil && bytes.Equal(b, old) {
						preserved = true
					}
				}
				if !preserved {
					t.Fatal("changed legacy settings were not preserved separately")
				}
			}
		})
	}
}

func TestJSONCFieldRemovalPreservesAdjacentComments(t *testing.T) {
	for _, original := range []string{
		`{/*before*/"theme":"old"/*after*/,/*next*/"keep":1,/*tail*/}`,
		`{"keep":1,/*before*/"theme":"old"/*after*/,/*tail*/}`,
		`{/*before*/"theme":"old"/*after*/,/*tail*/}`,
		`{/*before*/"theme":"old"/*after*/}`,
	} {
		result, _, err := replaceJSONCFields([]byte(original), []string{"theme"}, nil)
		if err != nil {
			t.Fatal(original, err)
		}
		for _, comment := range []string{"/*before*/", "/*after*/", "/*next*/", "/*tail*/"} {
			if strings.Contains(original, comment) && !bytes.Contains(result, []byte(comment)) {
				t.Fatal("lost personal comment", string(result))
			}
		}
	}
}

func TestJSONCRejectsAmbiguousAndMalformedSettings(t *testing.T) {
	for _, data := range []string{`{,}`, `{"array":[,]}`, `{"theme":1,"theme":2}`, `{"theme":1,,}`, `{"a":1}true`, `/*open`, `{"a":"bad\"}`, "// invalid \xff\n{}", "{}\x00", `[]`} {
		if _, _, err := replaceJSONCFields([]byte(data), []string{"theme"}, []byte(`{"theme":"new"}`)); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
	if _, _, err := replaceJSONCFields([]byte(`{}`), []string{"theme"}, []byte(`{"personal":1}`)); err == nil {
		t.Fatal("accepted unowned replacement")
	}
}

func TestVSCodeSettingsUseIndependentlyObservedFolders(t *testing.T) {
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, "xdg"), AppData: filepath.Join(home, "redirected-roaming")}
	for target, want := range map[string]string{
		"darwin":  filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json"),
		"linux":   filepath.Join(folders.Config, "Code", "User", "settings.json"),
		"windows": filepath.Join(folders.AppData, "Code", "User", "settings.json"),
	} {
		d := &ProfileDriver{Targets: map[string][]ProfileTarget{}}
		configureVSCodeSettings(d, Context{OS: target}, folders)
		if d.Targets[vscodeSettingsResource][0].Path != want {
			t.Fatal(target, d.Targets)
		}
	}
}

// These six fields are the persisted identity in the approved d9a3993 build.
// Pin the historical contract independently of the current settings recipe.
func TestVSCodeLegacySettingsFieldsRemainCompatible(t *testing.T) {
	_, d, _ := vscodeSettingsController(t)
	expected := []string{"workbench.colorTheme", "workbench.preferredDarkColorTheme", "workbench.preferredLightColorTheme", "window.autoDetectColorScheme", "editor.fontFamily", "terminal.integrated.fontFamily"}
	if !slices.Equal(d.JSONFields["integration.vscode"], expected) {
		t.Fatal("historical profile field identity changed", d.JSONFields["integration.vscode"])
	}
}

func TestVSCodeHandoverMissingEvidencePreservesPersonalSettings(t *testing.T) {
	c, d, path := vscodeSettingsController(t)
	c.Catalog.Resources[0].Requires = []string{"integration.vscode"}
	c.Catalog.Resources[1].ID = "integration.vscode"
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	journal := d.journalPath(state.Receipts["integration.vscode"].OperationID)
	if err := os.Rename(journal, journal+".offline"); err != nil {
		t.Fatal(err)
	}
	c.Catalog.Resources[0].Requires = []string{vscodeSettingsResource}
	c.Catalog.Resources[1].ReplacedBy = vscodeSettingsResource
	c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: vscodeSettingsResource, Name: "Settings", Action: "profile", Scope: "user"})
	c.Driver = &NativeDriver{Catalog: c.Catalog, Context: c.Context, Profiles: d, StatePath: c.StatePath}
	plan, result, err := executeApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	if err != nil || result.Status != "needs-action" || !strings.Contains(operation(t, plan, "integration.vscode").Reason, "evidence is missing") || !strings.Contains(operation(t, plan, vscodeSettingsResource).Reason, "recovery evidence") {
		t.Fatal("missing evidence did not fail closed with useful guidance", plan, result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("missing evidence authorized a profile mutation", err)
	}
}
