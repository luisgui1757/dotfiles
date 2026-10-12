package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// DONT_VERIFY reads registered paths without creating a missing known folder.
// Each folder is queried independently; redirected folders can span volumes.
func DiscoverConfigFolders() (ConfigFolders, error) {
	var result ConfigFolders
	for _, item := range []struct {
		name string
		id   *windows.KNOWNFOLDERID
		out  *string
	}{
		{"profile", windows.FOLDERID_Profile, &result.Home},
		{"local app data", windows.FOLDERID_LocalAppData, &result.LocalAppData},
		{"roaming app data", windows.FOLDERID_RoamingAppData, &result.AppData},
		{"documents", windows.FOLDERID_Documents, &result.Documents},
		{"programs", windows.FOLDERID_Programs, &result.Programs},
	} {
		value, err := windows.KnownFolderPath(item.id, windows.KF_FLAG_DONT_VERIFY)
		if err != nil {
			return result, fmt.Errorf("resolve current user's %s folder: %w", item.name, err)
		}
		if !filepath.IsAbs(value) {
			return result, fmt.Errorf("current user's %s folder is not absolute", item.name)
		}
		*item.out = filepath.Clean(value)
	}
	result.Config = filepath.Join(result.Home, ".config")
	gitHome, explicitHome := os.LookupEnv("HOME")
	if !explicitHome {
		drive, path := os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH")
		system, err := windows.GetSystemDirectory()
		if err != nil {
			return result, err
		}
		candidate := drive + path
		info, err := os.Stat(candidate)
		if drive != "" && path != "" && err == nil && info.IsDir() && !strings.EqualFold(filepath.Clean(candidate), filepath.Clean(system)) {
			gitHome = candidate
		} else {
			gitHome = os.Getenv("USERPROFILE")
		}
	}
	// Report every Git discovery cause; a later one must not hide an earlier one.
	issues := []string{}
	gitConfig := os.Getenv("XDG_CONFIG_HOME")
	if gitConfig == "" {
		if !filepath.IsAbs(gitHome) {
			issues = append(issues, "Git HOME must resolve to an absolute path.")
		}
		gitConfig = filepath.Join(gitHome, ".config")
	}
	if !filepath.IsAbs(gitConfig) || filepath.Clean(gitConfig) != gitConfig || strings.ContainsAny(gitConfig, "\x00\r\n") {
		issues = append(issues, "Git HOME/XDG_CONFIG_HOME must resolve to an absolute canonical path.")
	}
	result.GitConfig = filepath.Join(gitConfig, "git", "config")
	if _, set := os.LookupEnv("GIT_CONFIG_GLOBAL"); set {
		issues = append(issues, "Unset GIT_CONFIG_GLOBAL before configuring Git defaults; that override bypasses both personal global configuration files.")
	}
	result.GitIssue = strings.Join(issues, " ")
	result.Data = os.Getenv("XDG_DATA_HOME")
	if result.Data == "" {
		result.Data = result.LocalAppData
	}
	if !filepath.IsAbs(result.Data) || filepath.Clean(result.Data) != result.Data || strings.ContainsAny(result.Data, "\x00\r\n") {
		return result, fmt.Errorf("XDG_DATA_HOME must be an absolute canonical path")
	}
	return result, nil
}
