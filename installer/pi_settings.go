package installer

import "path/filepath"

func configurePiSettings(d *ProfileDriver, folders ConfigFolders) {
	if d.JSONFields == nil {
		d.JSONFields = map[string][]string{}
	}
	d.JSONFields["integration.pi"] = []string{"theme"}
	d.Targets["integration.pi"] = []ProfileTarget{{
		Path: filepath.Join(folders.Home, ".pi", "agent", "settings.json"), Script: `{"theme":"rose-pine"}`,
	}}
}
