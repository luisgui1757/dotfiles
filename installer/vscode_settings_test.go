package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vscodeSettingsController(t *testing.T) (Controller, *ProfileDriver, string) {
	t.Helper()
	c, d, _ := profileController(t)
	c.Catalog.Resources[0].Requires = []string{"integration.vscode"}
	c.Catalog.Resources[1].ID = "integration.vscode"
	folders := ConfigFolders{Home: c.Home, Config: filepath.Join(c.Home, "redirected-config"), AppData: filepath.Join(c.Home, "roaming")}
	d.Targets = map[string][]ProfileTarget{}
	configureVSCodeSettings(d, c.Context, folders)
	return c, d, d.Targets["integration.vscode"][0].Path
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
				if err != nil || operation(t, preview, "integration.vscode").Action != "pending" {
					t.Fatal("personal theme must require adoption", preview, err)
				}
				request.Adopt = []string{"integration.vscode"}
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
			d.Targets["integration.vscode"][0].Script = strings.ReplaceAll(d.Targets["integration.vscode"][0].Script, "Rosé Pine", "Rosé Pine Moon")
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
		if d.Targets["integration.vscode"][0].Path != want {
			t.Fatal(target, d.Targets)
		}
	}
}
