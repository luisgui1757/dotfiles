package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Prove the production main dispatch, not just separately constructed drivers.
// Windows known folders cannot be redirected through HOME; the separately gated
// Windows public migration fixture uses the actual disposable hosted account.
func TestInstalledMigrationThenSetupKeepsHistoricalDetachment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows migration runs in its separately gated hosted fixture")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacy := "# unknown personal commands remain available in the migration backup\nexport PERSONAL=value\n"
	profile := filepath.Join(home, ".zshrc")
	writeConfigFixture(t, profile, legacy)
	invoke := installedMachineFixture(t, repository, home, time.Minute)
	t.Setenv("DOTFILES_ENTRYPOINT", "migrate")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"legacy.zshrc"}, Adopt: []string{"legacy.zshrc"}}
	preview := invoke(request)
	if preview.Status != "preview" || operation(t, preview.Plan, "legacy.zshrc").Action != "adopt" {
		t.Fatal("main did not route explicit migration", preview)
	}
	data, err := os.ReadFile(profile)
	if err != nil || string(data) != legacy {
		t.Fatal("migration preview mutated personal profile", string(data), err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if result := invoke(request); result.Status != "ready" {
		t.Fatal(result)
	}
	newProfile := "# new shell profile added after migration\n"
	writeConfigFixture(t, profile, newProfile)
	t.Setenv("DOTFILES_ENTRYPOINT", "setup")
	for _, request := range []Request{{Schema: 1, Mode: "apply", Selected: []string{}}, {Schema: 1, Mode: "update"}} {
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal("main did not route setup", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		if result := invoke(request); result.Status != "ready" {
			t.Fatal(result)
		}
	}
	t.Setenv("DOTFILES_ENTRYPOINT", "migrate")
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("historical migration changed after normal setup", result)
	}
	data, err = os.ReadFile(profile)
	if err != nil || string(data) != newProfile {
		t.Fatal("historical migration replayed over new shell content", string(data), err)
	}
}
