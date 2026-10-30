package installer

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func configDirectoryIdentity(file *os.File) (string, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}

// Windows has no POSIX file ownership.
func fileOwner(path string) (uint32, uint32, uint32, error) {
	return 0, 0, 0, fmt.Errorf("POSIX ownership is unavailable for %s on Windows", path)
}
