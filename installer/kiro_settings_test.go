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

func kiroSettingsController(t *testing.T, locations SentinelLocations) (Controller, *ProfileDriver, string) {
	t.Helper()
	c, d, _ := profileController(t)
	c.Catalog.Resources[0].Requires = []string{kiroSettingsResource}
	c.Catalog.Resources[1].ID = kiroSettingsResource
	d.Targets = map[string][]ProfileTarget{}
	// Kiro CLI is offered on macOS and Linux; exercise that recipe on every host.
	configureKiroSettings(d, Context{OS: "linux", Arch: "amd64"}, ConfigFolders{Home: c.Home}, locations)
	return c, d, d.Targets[kiroSettingsResource][0].Path
}

func kiroSettingsJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("Kiro settings are not strict JSON: %q: %v", data, err)
	}
	return values
}

func TestKiroHomeFollowsKirosDocumentedRules(t *testing.T) {
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	fallback := filepath.Join(home, ".kiro")
	work := filepath.Join(base, "profiles", "work")
	root := filepath.VolumeName(base) + string(filepath.Separator)
	for value, want := range map[string]string{
		"":                                 fallback,
		"relative/kiro":                    fallback,
		root:                               fallback,
		home:                               fallback,
		base:                               fallback,
		work:                               work,
		work + string(filepath.Separator):  work,
		filepath.Join(home, "kiro-work"):   filepath.Join(home, "kiro-work"),
		filepath.Join(work, "..", "other"): filepath.Join(base, "profiles", "other"),
	} {
		if got := kiroHome(home, value); got != want {
			t.Errorf("KIRO_HOME %q resolved to %q, want %q", value, got, want)
		}
	}
}

func TestKiroSettingsOnlyOwnTheSelfUpdateSwitch(t *testing.T) {
	for name, original := range map[string]string{
		"absent":   "",
		"personal": "{\n  \"chat.defaultModel\": \"personal\",\n  \"telemetry.enabled\": false\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			c, _, path := kiroSettingsController(t, SentinelLocations{})
			if want := filepath.Join(c.Home, ".kiro", "settings", "cli.json"); path != want {
				t.Fatal("Kiro CLI settings target", path, want)
			}
			if original != "" {
				writeConfigFixture(t, path, original)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			values := kiroSettingsJSON(t, path)
			if values["app.disableAutoupdates"] != true || original != "" && values["chat.defaultModel"] != "personal" {
				t.Fatal("Kiro self-update stays enabled or personal settings changed", values)
			}
			// Kiro rewrites its own file when a setting changes; ownership is the
			// field's value, not the formatting around it.
			writeConfigFixture(t, path, "{\n  \"app.disableAutoupdates\": true,\n  \"chat.greeting.enabled\": false\n}\n")
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("a Kiro rewrite of personal settings invalidated ownership", check, err)
			}
			if original != "" {
				writeConfigFixture(t, path, strings.Replace(original, "{\n", "{\n  \"app.disableAutoupdates\": true,\n", 1))
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err := os.ReadFile(path)
			if original == "" {
				// Removal leaves the personal greeting setting Kiro added.
				if err != nil || strings.Contains(string(data), "disableAutoupdates") || kiroSettingsJSON(t, path)["chat.greeting.enabled"] != false {
					t.Fatal("removal kept the owned field or lost a personal one", string(data), err)
				}
				return
			}
			if err != nil || string(data) != original {
				t.Fatalf("removal did not restore personal settings: got %q, want %q: %v", data, original, err)
			}
		})
	}
}

func TestKiroSettingsRemovalDeletesOnlyTheFoldersItCreated(t *testing.T) {
	c, _, path := kiroSettingsController(t, SentinelLocations{})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	if values := kiroSettingsJSON(t, path); len(values) != 1 || values["app.disableAutoupdates"] != true {
		t.Fatal("fresh Kiro settings own more than the self-update switch", values)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(filepath.Join(c.Home, ".kiro")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("removal left a Kiro folder that setup created", err)
	}
}

func TestKiroSettingsRequireAdoptionOfAPersonalUpdateChoice(t *testing.T) {
	c, _, path := kiroSettingsController(t, SentinelLocations{})
	original := "{\"app.disableAutoupdates\":false,\"chat.defaultModel\":\"personal\"}\n"
	writeConfigFixture(t, path, original)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, kiroSettingsResource).Action != "pending" {
		t.Fatal("a personal update choice was replaced without adoption", preview, err)
	}
	request.Adopt = []string{kiroSettingsResource}
	dispatchApproved(t, c, request)
	if kiroSettingsJSON(t, path)["app.disableAutoupdates"] != true {
		t.Fatal("adopted Kiro settings still update in the background")
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("removal did not restore the adopted personal choice: %q %v", data, err)
	}
}

func TestKiroSettingsFollowKiroHomeAndRefuseItsLaterMove(t *testing.T) {
	work, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, d, path := kiroSettingsController(t, SentinelLocations{KiroHome: work})
	if path != filepath.Join(work, "settings", "cli.json") {
		t.Fatal("Kiro settings ignore KIRO_HOME", path)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	if err := retainKiroSettingsTarget(d, state); err != nil {
		t.Fatal("unchanged KIRO_HOME was refused", err)
	}
	moved := &ProfileDriver{Directory: d.Directory, Targets: map[string][]ProfileTarget{}, Windows: d.Windows}
	configureKiroSettings(moved, Context{OS: "linux", Arch: "amd64"}, ConfigFolders{Home: c.Home}, SentinelLocations{})
	err = retainKiroSettingsTarget(moved, state)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "KIRO_HOME") {
		t.Fatal("a moved KIRO_HOME silently retargeted owned settings", err)
	}
	none := &ProfileDriver{Directory: d.Directory, Targets: map[string][]ProfileTarget{}}
	configureKiroSettings(none, Context{OS: "windows", Arch: "amd64"}, ConfigFolders{Home: c.Home}, SentinelLocations{})
	if len(none.Targets) != 0 || len(none.JSONFields) != 0 {
		t.Fatal("Kiro CLI settings are configured on Windows, where Kiro CLI is not offered")
	}
}
