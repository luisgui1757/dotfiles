package installer

import (
	"os"
	"path/filepath"
)

const (
	t3SettingsResource = "integration.t3-code"
	t3ThemeResource    = "integration.t3-code-theme"
)

func configureT3Settings(d *ProfileDriver, folders ConfigFolders, repository string) {
	userdata := filepath.Join(folders.Home, ".t3", "userdata")
	if d.JSONFields == nil {
		d.JSONFields = map[string][]string{}
	}
	d.JSONFields[t3SettingsResource] = []string{
		"storageCleanup", "enableProviderUpdateChecks", "sidebarAutoSettleAfterDays", "idleCompaction",
		"autoResumeLimitedThreads", "sidebarAutoSettleOnMerge", "defaultThreadEnvMode", "branchNamingMode",
		"removeAgentCreditsOnMerge", "pullRequestMergeMethod", "defaultTheme",
	}
	d.Targets[t3SettingsResource] = []ProfileTarget{{Path: filepath.Join(userdata, "settings.json"), Script: `{
		"storageCleanup":{"worktreeOnMerge":true,"worktreeOnDelete":true},
		"enableProviderUpdateChecks":false,
		"sidebarAutoSettleAfterDays":null,
		"idleCompaction":"onReturn",
		"autoResumeLimitedThreads":true,
		"sidebarAutoSettleOnMerge":false,
		"defaultThreadEnvMode":"worktree",
		"branchNamingMode":"semantic",
		"removeAgentCreditsOnMerge":true,
		"pullRequestMergeMethod":"squash",
		"defaultTheme":"rose-pine-moon"
	}`}}
	theme, err := os.ReadFile(filepath.Join(repository, "t3", "rose-pine-moon.json"))
	if err != nil {
		d.recordIssue(t3ThemeResource, err)
		return
	}
	d.JSONFields[t3ThemeResource] = []string{"version", "name", "appearance", "colors"}
	d.Targets[t3ThemeResource] = []ProfileTarget{{Path: filepath.Join(userdata, "themes", "rose-pine-moon.json"), Script: string(theme)}}
}
