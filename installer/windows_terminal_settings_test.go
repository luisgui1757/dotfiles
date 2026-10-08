package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func windowsTerminalController(t *testing.T) (Controller, *ProfileDriver, string) {
	t.Helper()
	c, d, _ := profileController(t)
	c.Catalog.Resources[0].Requires = []string{windowsTerminalResource}
	c.Catalog.Resources[1].ID = windowsTerminalResource
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	d.Targets = map[string][]ProfileTarget{}
	folders := ConfigFolders{Home: c.Home, LocalAppData: filepath.Join(c.Home, "native local data")}
	if err := configureWindowsTerminal(d, Context{OS: "windows"}, folders, repository, filepath.Join(c.Home, "managed pwsh", "pwsh.exe")); err != nil {
		t.Fatal(err)
	}
	return c, d, d.Targets[windowsTerminalResource][0].Path
}

func terminalSetting(t *testing.T, path, field string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, present, err := windowsTerminalGet(data, field)
	if err != nil || !present {
		t.Fatal("missing terminal setting", field, err)
	}
	return string(value)
}

func TestWindowsTerminalSettingsPreservePersonalNestedFieldsAndAllOtherEntries(t *testing.T) {
	c, d, path := windowsTerminalController(t)
	original := `// personal header
{
	"defaultProfile":"{my-shell}",
	"profiles":{"personal":true,"defaults":{"font":{"face":"Personal Font","weight":"bold"},"cursorShape":"bar"},"list":[{"guid":"{personal}","name":"Personal"},{"guid":"` + windowsTerminalProfileGUID + `","name":"Previous","icon":"personal.ico"}]},
	"actions":[{"id":"personal","command":"personal"}],
	"keybindings":[{"keys":["ctrl+alt+p","alt+f9"],"id":"personal"},{"keys":"ctrl+c","id":"old","name":"My copy label"}],
	"schemes":[{"name":"personal","red":"#abcdef"},{"name":"rose-pine","red":"#123456","personal":true}],
	"themes":[{"name":"personal","window":{}},{"name":"rose-pine","window":{"myPreference":true}}],
	"huge":9007199254740993, // keep exact
}
`
	writeConfigFixture(t, path, original)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, windowsTerminalResource).Action != "pending" {
		t.Fatal("overlapping personal settings did not require adoption", preview, err)
	}
	request.Adopt = []string{windowsTerminalResource}
	dispatchApproved(t, c, request)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`"defaultProfile":"{my-shell}"`, `"weight":"bold"`, `"cursorShape":"bar"`, `"icon":"personal.ico"`, `"name":"My copy label"`, `"myPreference":true`, `9007199254740993`, `// keep exact`, `"keys":["ctrl+alt+p","alt+f9"]`} {
		if !bytes.Contains(data, []byte(kept)) {
			t.Fatal("lost personal settings", kept, string(data))
		}
	}
	if !bytes.Contains(data, []byte(`"historySize":32767`)) || !bytes.Contains(data, []byte("managed pwsh")) {
		t.Fatal("managed settings missing", string(data))
	}
	writeConfigFixture(t, path, strings.Replace(string(data), `"weight":"bold"`, `"weight":"light","features":{"ss01":1}`, 1))
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("unowned nested preference invalidated ownership", check, err)
	}
	p, err := decodeWindowsTerminalProjection([]byte(d.Targets[windowsTerminalResource][0].Script))
	if err != nil {
		t.Fatal(err)
	}
	p.Values["profiles/defaults/font/size"] = []byte("13")
	updated, err := windowsTerminalProjectionData(p)
	if err != nil {
		t.Fatal(err)
	}
	d.Targets[windowsTerminalResource][0].Script = string(updated)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`"face":"Personal Font"`, `"weight":"light"`, `"features":{"ss01":1}`, `"name":"Previous"`, `"id":"old"`, `"red":"#123456"`, `"defaultProfile":"{my-shell}"`, `"myPreference":true`, `9007199254740993`} {
		if !bytes.Contains(data, []byte(kept)) {
			t.Fatal("removal lost baseline or later personal setting", kept, string(data))
		}
	}
	if bytes.Contains(data, []byte(`"historySize"`)) || bytes.Contains(data, []byte(`"size":13`)) {
		t.Fatal("removal retained owned setting", string(data))
	}
}

func TestWindowsTerminalPreservesEveryNonemptyDefaultAndLaterUserChanges(t *testing.T) {
	for _, prior := range []string{"", `""`, `"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}"`, `"{personal}"`} {
		for _, later := range []bool{false, true} {
			c, _, path := windowsTerminalController(t)
			original := "{}"
			if prior != "" {
				original = `{"defaultProfile":` + prior + `}`
			}
			writeConfigFixture(t, path, original)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			want := prior
			if want == "" || want == `""` {
				want = `"` + windowsTerminalProfileGUID + `"`
			}
			if got := terminalSetting(t, path, "defaultProfile"); got != want {
				t.Fatal("existing startup choice changed", got, want)
			}
			if later {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data, err = windowsTerminalSet(data, "defaultProfile", []byte(`"{later-personal}"`))
				if err != nil {
					t.Fatal(err)
				}
				writeConfigFixture(t, path, string(data))
				check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
				if err != nil || check.Status != "ready" {
					t.Fatal("later default choice was treated as owned drift", check, err)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			value, present, err := windowsTerminalGet(data, "defaultProfile")
			if err != nil {
				t.Fatal(err)
			}
			if later {
				if !present || string(value) != `"{later-personal}"` {
					t.Fatal("removal lost later startup preference", string(data))
				}
			} else if prior == "" {
				if present {
					t.Fatal("removal retained created default", string(data))
				}
			} else if !present || string(value) != prior {
				t.Fatal("removal lost earlier startup preference", string(data), prior)
			}
		}
	}
}

func TestWindowsTerminalRemovalRetainsPersonalFieldsAddedToNewManagedEntries(t *testing.T) {
	c, _, path := windowsTerminalController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err = windowsTerminalSet(data, "profiles/defaults/font/weight", []byte(`"bold"`))
	if err != nil {
		t.Fatal(err)
	}
	collection := windowsTerminalCollections()[0]
	array, _, err := windowsTerminalGet(data, collection.Path)
	if err != nil {
		t.Fatal(err)
	}
	entry, index, err := windowsTerminalEntry(array, collection)
	if err != nil {
		t.Fatal(err)
	}
	entry, err = windowsTerminalSet(entry, "icon", []byte(`"my-icon.ico"`))
	if err != nil {
		t.Fatal(err)
	}
	array, err = windowsTerminalReplaceElement(array, index, entry)
	if err != nil {
		t.Fatal(err)
	}
	data, err = windowsTerminalSet(data, collection.Path, array)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, string(data))
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`"weight":"bold"`, `"icon":"my-icon.ico"`, windowsTerminalProfileGUID} {
		if !bytes.Contains(data, []byte(kept)) {
			t.Fatal("removal lost a personal addition", kept, string(data))
		}
	}
	if bytes.Contains(data, []byte("Hack Nerd Font")) || bytes.Contains(data, []byte("commandline")) {
		t.Fatal("removal retained owned profile fields", string(data))
	}
}

func TestWindowsTerminalRejectsAmbiguousManagedIdentityAndInvalidContainers(t *testing.T) {
	for _, bad := range []string{`{"profiles":[]}`, `{"profiles":{"defaults":{"font":"wrong"}}}`, `{"schemes":[{"name":"rose-pine"},{"name":"ROSE-PINE"}]}`, `{"profiles":{"list":[{"guid":"` + windowsTerminalProfileGUID + `"},{"guid":"` + windowsTerminalProfileGUID + `"}]}}`, `{"keybindings":[{"keys":["ctrl+c","ctrl+p"],"id":"Terminal.CopyToClipboard"}]}`, `{"defaultProfile":17}`, `{"profiles":{"defaults":{"font":{"face":"one","face":"two"}}}}`} {
		c, _, path := windowsTerminalController(t)
		writeConfigFixture(t, path, bad)
		preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		if err != nil || operation(t, preview, windowsTerminalResource).Action != "pending" {
			t.Fatal("unsafe Terminal settings accepted", bad, preview, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != bad {
			t.Fatal("inspection changed unsafe settings", err)
		}
	}
}

func TestWindowsTerminalRestorationBookkeepingIsBoundToTheFirstReceipt(t *testing.T) {
	for _, changed := range []string{"default", "containers", "rehashed"} {
		c, d, path := windowsTerminalController(t)
		writeConfigFixture(t, path, `{"defaultProfile":"{personal}","profiles":{"defaults":{"font":{}}}}`)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		state, err := LoadState(c.StatePath, c.Home)
		if err != nil {
			t.Fatal(err)
		}
		receipt := state.Receipts[windowsTerminalResource]
		baseline, err := d.readBaseline(windowsTerminalResource, receipt)
		if err != nil {
			t.Fatal(err)
		}
		p, err := decodeWindowsTerminalProjection(baseline.Entries[0].Block)
		if err != nil {
			t.Fatal(err)
		}
		if changed != "containers" {
			p.Default = []byte(`"{tampered}"`)
		} else {
			p.Containers = nil
		}
		baseline.Entries[0].Block, err = windowsTerminalProjectionData(p)
		if err != nil {
			t.Fatal(err)
		}
		if changed == "rehashed" {
			baseline.Before.Inventory, err = digest([][]byte{baseline.Entries[0].Block})
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := saveDocument(d.baselinePath(receipt.Recovery), baseline); err != nil {
			t.Fatal(err)
		}
		if _, err := d.readBaseline(windowsTerminalResource, receipt); err == nil {
			t.Fatal("changed unowned bookkeeping accepted")
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
		if err != nil || check.Status != "needs-action" {
			t.Fatal("baseline tamper did not block lifecycle", check, err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("tamper inspection mutated settings", err)
		}
	}
}

func TestWindowsTerminalFixedProjectionKeepsRawOwnedBaselineValues(t *testing.T) {
	data := []byte(`{"actions":[{"id":"Dotfiles.CloseTab","command":{"action":/* original */"closeTab"}}]}`)
	block, err := extractWindowsTerminal(data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodeWindowsTerminalProjection(block)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(p.Values["close-tab/command"], []byte("/* original */")) {
		t.Fatal("baseline lost owned value comments")
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(block, &raw) != nil {
		t.Fatal("projection is not durable JSON")
	}
}

func TestWindowsTerminalLegacyCombinedBindingsRequireNativeMigration(t *testing.T) {
	for _, keys := range []string{`"ctrl+c"`, `["ctrl+c"]`, `"SHIFT+CTRL+f"`} {
		c, _, path := windowsTerminalController(t)
		original := `{"actions":[{"keys":` + keys + `,"command":"personal"}]}`
		writeConfigFixture(t, path, original)
		preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		if err != nil {
			t.Fatal(err)
		}
		op := operation(t, preview, windowsTerminalResource)
		if op.Action != "pending" || !strings.Contains(op.Reason, "launch the supported unpackaged stable Terminal once") {
			t.Fatal("legacy overlap lacked actionable migration boundary", op)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			t.Fatal("legacy settings changed during discovery", err)
		}
	}
}

func TestWindowsTerminalNativeEquivalentSerializationKeepsOwnership(t *testing.T) {
	c, _, path := windowsTerminalController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("#e0def4"), []byte("#E0DEF4"))
	data = bytes.ReplaceAll(data, []byte("#191724ff"), []byte("#191724"))
	data, err = windowsTerminalSet(data, "profiles/defaults/font/size", []byte("12.0"))
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, string(data))
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("equivalent native serialization invalidated owned settings", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}

func TestWindowsTerminalRejectsEquivalentDuplicateModernKeybindings(t *testing.T) {
	_, err := extractWindowsTerminal([]byte(`{"keybindings":[{"keys":"ctrl+shift+f","id":"one"},{"keys":"SHIFT+CTRL+f","id":"two"}]}`))
	if err == nil {
		t.Fatal("accepted duplicate equivalent managed key chord")
	}
}

func TestWindowsTerminalRejectsCaseMismatchedNamesAndMalformedManagedGUID(t *testing.T) {
	for _, data := range []string{`{"schemes":[{"name":"ROSE-PINE"}]}`, `{"themes":[{"name":"Rose-Pine"}]}`, `{"actions":[{"id":"dotfiles.closetab"}]}`, `{"profiles":{"list":[{"guid":"{` + windowsTerminalProfileGUID + `}"}]}}`} {
		if _, err := extractWindowsTerminal([]byte(data)); err == nil {
			t.Fatal("ambiguous managed identity accepted", data)
		}
	}
}

func TestWindowsTerminalRemovalRestoresDefaultWhenOwnedProfileDisappears(t *testing.T) {
	c, _, path := windowsTerminalController(t)
	writeConfigFixture(t, path, `{"defaultProfile":"{original-personal}"}`)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err = windowsTerminalSet(data, "defaultProfile", []byte(`"`+windowsTerminalProfileGUID+`"`))
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, string(data))
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if got := terminalSetting(t, path, "defaultProfile"); got != `"{original-personal}"` {
		t.Fatal("removal left the default referring to its removed profile", got)
	}
}

func TestWindowsTerminalLaterManagedDefaultSurvivesWhileProfileRemains(t *testing.T) {
	for _, scenario := range []string{"update", "original-profile", "later-personal-field"} {
		t.Run(scenario, func(t *testing.T) {
			c, _, path := windowsTerminalController(t)
			original := `{"defaultProfile":"{original-personal}"}`
			if scenario == "original-profile" {
				original = `{"defaultProfile":"{original-personal}","profiles":{"list":[{"guid":"` + windowsTerminalProfileGUID + `","name":"Original profile"}]}}`
			}
			writeConfigFixture(t, path, original)
			install := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if scenario == "original-profile" {
				install.Adopt = []string{windowsTerminalResource}
			}
			dispatchApproved(t, c, install)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err = windowsTerminalSet(data, "defaultProfile", []byte(`"`+windowsTerminalProfileGUID+`"`))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "later-personal-field" {
				collection := windowsTerminalCollections()[0]
				array, _, err := windowsTerminalGet(data, collection.Path)
				if err != nil {
					t.Fatal(err)
				}
				entry, index, err := windowsTerminalEntry(array, collection)
				if err != nil {
					t.Fatal(err)
				}
				// icon is outside managed fields and therefore retains this entry.
				entry, err = windowsTerminalSet(entry, "icon", []byte(`"personal.ico"`))
				if err != nil {
					t.Fatal(err)
				}
				array, err = windowsTerminalReplaceElement(array, index, entry)
				if err != nil {
					t.Fatal(err)
				}
				data, err = windowsTerminalSet(data, collection.Path, array)
				if err != nil {
					t.Fatal(err)
				}
			}
			writeConfigFixture(t, path, string(data))
			request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
			if scenario == "update" {
				request.Mode = "update"
				request.Selected = nil
			}
			dispatchApproved(t, c, request)
			if got := terminalSetting(t, path, "defaultProfile"); got != `"`+windowsTerminalProfileGUID+`"` {
				t.Fatal("changed a valid later default choice", got)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			array, _, err := windowsTerminalGet(data, "profiles/list")
			if err != nil {
				t.Fatal(err)
			}
			entry, _, err := windowsTerminalEntry(array, windowsTerminalCollections()[0])
			if err != nil || entry == nil {
				t.Fatal("valid retained default has no profile", string(array), err)
			}
		})
	}
}

func TestWindowsTerminalOwnedEditNeedsAdoptionAndRetainsFirstBaseline(t *testing.T) {
	c, _, path := windowsTerminalController(t)
	writeConfigFixture(t, path, `{"profiles":{"defaults":{"font":{"size":9}}}}`)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{windowsTerminalResource}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err = windowsTerminalSet(data, "profiles/defaults/font/size", []byte("14"))
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, string(data))
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, windowsTerminalResource).Action != "pending" {
		t.Fatal("owned user edit was silently overwritten", preview, err)
	}
	request.Adopt = []string{windowsTerminalResource}
	dispatchApproved(t, c, request)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if got := terminalSetting(t, path, "profiles/defaults/font/size"); got != "9" {
		t.Fatal("readoption replaced the first baseline", got)
	}
}
