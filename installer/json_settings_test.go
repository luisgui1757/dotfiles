package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func jsonSettingsController(t *testing.T) (Controller, *ProfileDriver, string) {
	t.Helper()
	c, d, _ := profileController(t)
	path := filepath.Join(c.Home, "settings.json")
	d.Targets["profile.test"] = []ProfileTarget{{path, `{"theme":"rose-pine"}`}}
	d.JSONFields = map[string][]string{"profile.test": {"theme"}}
	return c, d, path
}

func TestJSONSettingsLifecycleRestoresOnlyTheOwnedProperty(t *testing.T) {
	for _, prior := range []string{"", ", \"theme\" : \"personal\""} {
		t.Run(prior, func(t *testing.T) {
			c, d, path := jsonSettingsController(t)
			original := "{\r\n  \"model\" : {\"limit\":9007199254740993}" + prior + "\r\n}\r\n"
			writeConfigFixture(t, path, original)
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if prior != "" {
				preview, err := c.Preview(context.Background(), request)
				if err != nil || operation(t, preview, "profile.test").Action != "pending" {
					t.Fatal("existing personal theme requires explicit adoption", preview, err)
				}
				request.Adopt = []string{"profile.test"}
			}
			dispatchApproved(t, c, request)
			data, err := os.ReadFile(path)
			var configured struct{ Theme string }
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &configured); err != nil || configured.Theme != "rose-pine" {
				t.Fatal("theme was not configured", string(data), err)
			}
			data = []byte(strings.Replace(string(data), "9007199254740993", "9007199254740995", 1))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			d.Targets["profile.test"][0].Script = `{"theme":"rose-pine-moon"}`
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err = os.ReadFile(path)
			want := strings.Replace(original, "9007199254740993", "9007199254740995", 1)
			if err != nil || string(data) != want {
				t.Fatalf("removal changed unrelated bytes or lost the prior theme: %q, want %q: %v", data, want, err)
			}
		})
	}
}

func TestJSONSettingsRemovalDeletesOnlyNewEmptyDocument(t *testing.T) {
	for _, edited := range []bool{false, true} {
		c, _, path := jsonSettingsController(t)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		if edited {
			writeConfigFixture(t, path, `{"theme":"rose-pine","model":"later"}`)
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		data, err := os.ReadFile(path)
		if !edited && !os.IsNotExist(err) || edited && (err != nil || string(data) != `{"model":"later"}`) {
			t.Fatal("new settings cleanup lost personal fields or left an empty document", string(data), err)
		}
	}
}

func TestJSONSettingsUserThemeChangeIsRetainedOnRemoval(t *testing.T) {
	c, _, path := jsonSettingsController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	personal := `{"theme":"personal","model":"later"}`
	writeConfigFixture(t, path, personal)
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(path)
	if err != nil || string(data) != personal || operation(t, result.Plan, "profile.test").Action != "retain" {
		t.Fatal("removal overwrote a personal theme", string(data), err)
	}
}

func TestJSONSettingsSavedScopeCannotChangeDuringRecovery(t *testing.T) {
	c, d, _ := jsonSettingsController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["profile.test"]
	journal, err := d.readJournal("profile.test", receipt)
	if err != nil {
		t.Fatal(err)
	}
	journal.Fields = []string{"model"}
	data, err := json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeProfileJournal(data, journal.Operation); err == nil {
		t.Fatal("changed JSON ownership scope accepted for historical recovery")
	}
	d.JSONFields["profile.test"] = []string{"model"}
	if _, err := d.readBaseline("profile.test", receipt); err == nil {
		t.Fatal("changed recipe scope acquired another settings property")
	}
}

func TestJSONSettingsRecoversAfterProcessDeath(t *testing.T) {
	for _, recoverMode := range []string{"resume", "restore"} {
		t.Run(recoverMode, func(t *testing.T) {
			c, d, _ := jsonSettingsController(t)
			d.Targets["profile.test"] = nil
			for i := range 8 {
				path := filepath.Join(c.Home, string(rune('a'+i))+".json")
				writeConfigFixture(t, path, `{"model":"personal"}`)
				d.Targets["profile.test"] = append(d.Targets["profile.test"], ProfileTarget{path, `{"theme":"rose-pine"}`})
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			killProfileChildAfterMove(t, c, d, request, "previous", 1)
			if recoverMode == "restore" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			} else {
				retry := Request{Schema: 1, Mode: "apply", Retry: true}
				preview, err := c.Preview(context.Background(), retry)
				if err != nil {
					t.Fatal(err)
				}
				retry.ExpectedPlan = preview.ID
				result, err := c.Dispatch(context.Background(), retry)
				if err != nil || result.Status != "needs-action" {
					t.Fatal("resource recovery must require fresh approval for the remaining transaction", result, err)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			}
			for _, target := range d.Targets["profile.test"] {
				data, err := os.ReadFile(target.Path)
				if err != nil || string(data) != `{"model":"personal"}` {
					t.Fatal("recovery lost personal settings", target.Path, string(data), err)
				}
				if _, err := os.Lstat(target.Path + ".lock"); !os.IsNotExist(err) {
					t.Fatal("recovery stranded an application lock", err)
				}
			}
		})
	}
}

func TestJSONSettingsAbandonmentReleasesOnlyRecordedApplicationLocks(t *testing.T) {
	for _, ours := range []bool{false, true} {
		c, d, path := jsonSettingsController(t)
		original := `{"model":"personal"}`
		writeConfigFixture(t, path, original)
		if err := os.Mkdir(path+".lock", 0700); err != nil {
			t.Fatal(err)
		}
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
		preview, err := c.Preview(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = preview.ID
		if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "busy") {
			t.Fatal("installer ignored the application's active settings lock", err)
		}
		if ours {
			if err := os.Remove(path + ".lock"); err != nil {
				t.Fatal(err)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			// Recreate the saved boundary after lock acquisition but before
			// publication, as if that controller had died there.
			if _, err := lockJSONSettingsTarget(d.Directory, state.Receipts["profile.test"].OperationID, path); err != nil {
				t.Fatal(err)
			}
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			t.Fatal("abandonment modified personal settings", string(data), err)
		}
		_, err = os.Lstat(path + ".lock")
		if ours && !os.IsNotExist(err) || !ours && err != nil {
			t.Fatal("abandonment failed to distinguish installer and application locks", ours, err)
		}
	}
}

func TestJSONSettingsLockDoesNotInferMissingOwnershipFields(t *testing.T) {
	c, d, path := jsonSettingsController(t)
	operation, err := digest("incomplete-lock-owner")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(path+".lock", settingsLockMarker), "{}\n")
	if _, err := lockJSONSettingsTarget(d.Directory, operation, path); err == nil {
		t.Fatal("missing lock identity fields acquired another writer's lock", c.Home)
	}
}
