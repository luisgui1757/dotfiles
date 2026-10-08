package installer

import "path/filepath"

func configureVSCodeSettings(d *ProfileDriver, target Context, folders ConfigFolders) {
	root := filepath.Join(folders.Config, "Code")
	if target.OS == "darwin" {
		root = filepath.Join(folders.Home, "Library", "Application Support", "Code")
	} else if target.OS == "windows" {
		root = filepath.Join(folders.AppData, "Code")
	}
	if d.JSONFields == nil {
		d.JSONFields = map[string][]string{}
	}
	d.JSONFields["integration.vscode"] = []string{
		"workbench.colorTheme", "workbench.preferredDarkColorTheme", "workbench.preferredLightColorTheme",
		"window.autoDetectColorScheme", "editor.fontFamily", "terminal.integrated.fontFamily",
	}
	d.Targets["integration.vscode"] = []ProfileTarget{{Path: filepath.Join(root, "User", "settings.json"), Script: `{
		"workbench.colorTheme":"Rosé Pine",
		"workbench.preferredDarkColorTheme":"Rosé Pine",
		"workbench.preferredLightColorTheme":"Rosé Pine",
		"window.autoDetectColorScheme":false,
		"editor.fontFamily":"'Hack Nerd Font', Consolas, monospace",
		"terminal.integrated.fontFamily":"'Hack Nerd Font', Consolas, monospace"
	}`}}
}
