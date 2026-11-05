package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

func t3SettingsController(t *testing.T) (Controller, string, string) {
	t.Helper()
	c, d, _ := profileController(t)
	c.Catalog.Resources = []Resource{
		{ID: "test", Name: "Test", Capability: true, Requires: []string{t3SettingsResource}},
		{ID: t3SettingsResource, Name: "T3 settings", Action: "profile", Scope: "user", Requires: []string{t3ThemeResource}},
		{ID: t3ThemeResource, Name: "T3 theme", Action: "profile", Scope: "user"},
	}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	d.Targets = map[string][]ProfileTarget{}
	configureT3Settings(d, ConfigFolders{Home: c.Home}, repository)
	if len(d.Issues) != 0 {
		t.Fatal(d.Issues)
	}
	return c, d.Targets[t3SettingsResource][0].Path, d.Targets[t3ThemeResource][0].Path
}

func t3JSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("%s is not strict JSON: %q: %v", path, data, err)
	}
	return values
}

func TestT3SettingsOwnTheirFieldsAndWriteARealThemeFile(t *testing.T) {
	for name, original := range map[string]string{
		"absent":   "",
		"personal": "{\n  \"favorites\": [\n    {\n      \"provider\": \"codex\",\n      \"model\": \"personal\"\n    }\n  ],\n  \"projectSettingsFolded\": true\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			c, settings, theme := t3SettingsController(t)
			if want := filepath.Join(c.Home, ".t3", "userdata", "settings.json"); settings != want {
				t.Fatal("T3 settings target", settings, want)
			}
			if original != "" {
				writeConfigFixture(t, settings, original)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			values := t3JSON(t, settings)
			if values["defaultTheme"] != "rose-pine-moon" || values["pullRequestMergeMethod"] != "squash" || !reflect.DeepEqual(values["storageCleanup"], map[string]any{"worktreeOnMerge": true, "worktreeOnDelete": true}) {
				t.Fatal("T3 settings were not applied", values)
			}
			if original != "" && values["projectSettingsFolded"] != true {
				t.Fatal("personal T3 settings changed", values)
			}
			if info, err := os.Lstat(theme); err != nil || !info.Mode().IsRegular() {
				t.Fatal("T3 ignores a theme that is not a regular file", info, err)
			}
			if t3JSON(t, theme)["name"] != "Rosé Pine Moon" {
				t.Fatal("the T3 theme file is wrong")
			}

			writeConfigFixture(t, settings, `{
	"storageCleanup": {
		"worktreeOnMerge": true,
		"worktreeOnDelete": true
	},
	"enableProviderUpdateChecks": false,
	"projectSettingsFolded": true,
	"sidebarAutoSettleAfterDays": null,
	"idleCompaction": "onReturn",
	"autoResumeLimitedThreads": true,
	"sidebarAutoSettleOnMerge": false,
	"defaultThreadEnvMode": "worktree",
	"branchNamingMode": "semantic",
	"removeAgentCreditsOnMerge": true,
	"pullRequestMergeMethod": "squash",
	"defaultTheme": "rose-pine-moon"
}`)
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("a T3 rewrite of its settings invalidated ownership", check, err)
			}

			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Lstat(theme); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("removal left the T3 theme file", err)
			}
			if values := t3JSON(t, settings); !reflect.DeepEqual(values, map[string]any{"projectSettingsFolded": true}) {
				t.Fatal("removal kept an owned field or lost a personal one", values)
			}
		})
	}
}

func TestT3SettingsRequireAdoptionOfAPersonalChoice(t *testing.T) {
	c, settings, theme := t3SettingsController(t)
	original := "{\n  \"pullRequestMergeMethod\": \"merge\",\n  \"defaultThreadEnvMode\": \"local\"\n}\n"
	writeConfigFixture(t, settings, original)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, t3SettingsResource).Action != "pending" {
		t.Fatal("a personal T3 choice was replaced without adoption", preview, err)
	}
	request.Adopt = []string{t3SettingsResource}
	dispatchApproved(t, c, request)
	if values := t3JSON(t, settings); values["pullRequestMergeMethod"] != "squash" || values["defaultThreadEnvMode"] != "worktree" {
		t.Fatal("adopted T3 settings kept the personal choice", values)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if data, err := os.ReadFile(settings); err != nil || string(data) != original {
		t.Fatalf("removal did not restore the adopted personal choice: %q %v", data, err)
	}
	if _, err := os.Lstat(filepath.Dir(theme)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("removal left the themes folder that setup created", err)
	}
}

func TestT3ThemeIsAValidEnvironmentTheme(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "t3", "rose-pine-moon.json"))
	if err != nil {
		t.Fatal(err)
	}
	var theme struct {
		Version    int               `json:"version"`
		Name       string            `json:"name"`
		Appearance string            `json:"appearance"`
		Colors     map[string]string `json:"colors"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&theme); err != nil {
		t.Fatal(err)
	}
	role := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,63}$`)
	if theme.Version != 1 || theme.Name == "" || len(utf16.Encode([]rune(theme.Name))) > 48 || theme.Appearance != "dark" || len(theme.Colors) == 0 {
		t.Fatal("T3 ignores this theme file", theme.Version, theme.Name, theme.Appearance, len(theme.Colors))
	}
	for key, value := range theme.Colors {
		if !role.MatchString(key) || strings.TrimSpace(value) == "" || len(utf16.Encode([]rune(value))) > 64 {
			t.Fatalf("T3 ignores this theme file because of color %q: %q", key, value)
		}
	}
}
