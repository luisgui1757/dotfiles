package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const kiroSettingsResource = "integration.kiro"

// Kiro CLI replaces itself in the background on macOS and Linux. Its only
// documented switch there is app.disableAutoupdates in cli.json (the
// KIRO_NO_AUTO_UPDATE variable is Windows-only), so the reviewed pin owns
// exactly that field, as VS Code owns update.mode. Every other setting stays
// personal. Kiro CLI is not offered on Windows.
func configureKiroSettings(d *ProfileDriver, target Context, folders ConfigFolders, locations SentinelLocations) {
	if target.OS == "windows" {
		return
	}
	if d.JSONFields == nil {
		d.JSONFields = map[string][]string{}
	}
	d.JSONFields[kiroSettingsResource] = []string{"app.disableAutoupdates"}
	d.Targets[kiroSettingsResource] = []ProfileTarget{{Path: filepath.Join(kiroHome(folders.Home, locations.KiroHome), "settings", "cli.json"), Script: `{"app.disableAutoupdates":true}`}}
}

// Saved ownership keeps its approved settings file. A different KIRO_HOME
// later is refused for this resource only, never retargeted or abandoned.
func retainKiroSettingsTarget(d *ProfileDriver, state State) error {
	receipt := state.Receipts[kiroSettingsResource]
	if receipt.Recovery == "" || receipt.Ownership == "reused" || len(d.Targets[kiroSettingsResource]) != 1 {
		return nil
	}
	if !validProfileRecovery(receipt.Recovery) {
		return errors.New("invalid saved Kiro CLI settings recovery path")
	}
	data, err := readDocument(d.baselinePath(receipt.Recovery))
	if errors.Is(err, os.ErrNotExist) && receipt.Status == "in-progress" {
		return nil
	}
	if err != nil {
		return err
	}
	var baseline profileBaseline
	if err := Decode(data, &baseline); err != nil {
		return err
	}
	if len(baseline.Entries) != 1 {
		return errors.New("invalid saved Kiro CLI settings target")
	}
	if saved, current := baseline.Entries[0].Path, d.Targets[kiroSettingsResource][0].Path; saved != current {
		return fmt.Errorf("saved Kiro CLI settings target %s is not the current location %s; run setup with the KIRO_HOME used when it was installed to update or remove it", saved, current)
	}
	return nil
}
