package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// One unavailable released location must not stop review or migration of
// another profile. Uninspectable items are reported but never selectable.
func TestLegacyMigrationInspectionErrorsStayItemLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX profile layout; Windows path failures use the same observation route")
	}
	m := legacyMigrationFixture(t, false)
	writeConfigFixture(t, filepath.Join(m.Controller.Home, ".config"), "a regular file where the configuration directory belongs")
	profile := filepath.Join(m.Controller.Home, ".zshrc")
	writeConfigFixture(t, profile, "export PERSONAL=kept\n")
	check, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, "legacy.git-config") {
		t.Fatal("one unavailable location failed the whole migration check", check, err)
	}
	ui := &scriptedInteraction{t: t, accept: true, answers: [][]string{{"legacy.zshrc"}}}
	result, err := m.Run(context.Background(), ui)
	if err != nil || len(ui.menus) != 1 || len(ui.answers) != 0 {
		t.Fatal("unavailable location blocked an independent migration", result, err)
	}
	offered := []string{}
	for _, choice := range ui.menus[0] {
		offered = append(offered, choice.ID)
	}
	if !slices.Equal(offered, []string{"legacy.zshrc"}) {
		t.Fatal("uninspectable items were offered for selection", offered)
	}
	if result.Status != "needs-action" || !strings.Contains(result.Message, "legacy.git-config") {
		t.Fatal("blocked items were not reported after migration", result)
	}
	if receipt := legacyState(t, m).Receipts["legacy.zshrc"]; receipt.Ownership != "created" || receipt.Status != "ready" {
		t.Fatal("independent profile was not migrated", receipt)
	}
}

func TestLegacyMigrationUnavailableConventionalDocumentsIsItemLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX dangling link models an unavailable redirected Documents alias")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, repository := filepath.Join(root, "home"), filepath.Join(root, "checkout")
	for _, directory := range []string{home, repository} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "offline-documents"), filepath.Join(home, "Documents")); err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Documents: filepath.Join(root, "known Documents")}
	m, err := NewLegacyMigration(repository, "trusted-source", filepath.Join(root, "state"), Context{OS: "windows", Arch: "amd64"}, folders)
	if err != nil {
		t.Fatal("unavailable conventional Documents aborted every migration item", err)
	}
	actual := filepath.Join(folders.Documents, "PowerShell", "Microsoft.PowerShell_profile.ps1")
	writeConfigFixture(t, actual, "# personal profile\r\n")
	check, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, "legacy.conventional-powershell") {
		t.Fatal("unavailable conventional profile location was not reported locally", check, err)
	}
	result := approveLegacy(t, m, legacyRequest("legacy.powershell"))
	if receipt := legacyState(t, m).Receipts["legacy.powershell"]; receipt.Ownership != "created" || result.Status != "needs-action" {
		t.Fatal("known-folder profile migration depended on the unavailable alias", receipt, result)
	}
}
