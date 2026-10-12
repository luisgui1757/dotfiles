package installer

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Read the actual native system directory, not a caller-supplied PATH or
// SystemRoot environment override. Windows PowerShell is an OS component.
func DiscoverWindowsPowerShell() (string, error) {
	directory, err := windows.GetSystemDirectory()
	if err != nil {
		return "", err
	}
	program := filepath.Join(directory, "WindowsPowerShell", "v1.0", "powershell.exe")
	info, err := os.Stat(program)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", os.ErrInvalid
	}
	return program, nil
}

func DiscoverSystemCommandDirectories() ([]string, error) {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, err
	}
	return []string{system, root, filepath.Join(system, "Wbem")}, nil
}

func DiscoverWindowsBuildToolsDirectory() (string, error) {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFilesX64, windows.KF_FLAG_DONT_VERIFY)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "DotfilesBuildTools"), nil
}
