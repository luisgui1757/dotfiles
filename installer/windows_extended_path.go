package installer

import "strings"

// Windows paths only. Kept pure so fixed Windows recipes can also be checked
// on POSIX hosts; callers supply an already validated absolute native path.
func windowsExtendedPath(path string) string {
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\\?\` + path
}
