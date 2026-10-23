//go:build !windows

package installer

func RunWindowsRustLauncher() (bool, int) { return false, 0 }
