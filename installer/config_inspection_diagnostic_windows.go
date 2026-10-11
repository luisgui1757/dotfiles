//go:build windows

package installer

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func configInspectionIdentityDiagnostic(path string) string {
	name, err := windows.UTF16PtrFromString(windowsExtendedPath(path))
	if err != nil {
		return fmt.Sprintf("UTF16 error=%v", err)
	}
	// Match Go1.27.1 os.fileStat.loadFileId's read-only open flags. SameFile
	// hides this error; report one subsequent probe, never retry the verdict.
	handle, err := windows.CreateFile(name, 0, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Sprintf("CreateFile error=%v", err)
	}
	var info windows.ByHandleFileInformation
	queryErr := windows.GetFileInformationByHandle(handle, &info)
	closeErr := windows.CloseHandle(handle)
	return fmt.Sprintf("volume=%x file_index=%x:%x query_error=%v close_error=%v", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow, queryErr, closeErr)
}
