//go:build !windows

package installer

import "errors"

func DiscoverWindowsPowerShell() (string, error) {
	return "", errors.New("system Windows PowerShell discovery requires native Windows")
}

func DiscoverSystemCommandDirectories() ([]string, error) {
	return []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin"}, nil
}

func DiscoverWindowsBuildToolsDirectory() (string, error) {
	return "", errors.New("Build Tools installation directory requires native Windows")
}
