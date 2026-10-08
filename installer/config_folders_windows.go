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
	result.Data = os.Getenv("XDG_DATA_HOME")
	if result.Data == "" {
		result.Data = result.LocalAppData
	}
	if !filepath.IsAbs(result.Data) || filepath.Clean(result.Data) != result.Data || strings.ContainsAny(result.Data, "\x00\r\n") {
		return result, fmt.Errorf("XDG_DATA_HOME must be an absolute canonical path")
	}
	return result, nil
}
