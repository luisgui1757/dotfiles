//go:build !darwin

package installer

import "os"

// Preserve the existing non-Darwin identity used by the bootstrap fixtures; the
// durable-volume change is specific to macOS.
func homebrewDirectoryIdentity(file *os.File) (string, error) {
	return configDirectoryIdentity(file)
}
