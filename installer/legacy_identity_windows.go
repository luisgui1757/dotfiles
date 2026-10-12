package installer

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// legacyPathIdentity names the file object at path without following a final
// reparse point. A same-volume rename preserves the file index.
func legacyPathIdentity(path string) (identity string, result error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return "absent", nil
	}
	if err != nil {
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(handle)) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return "", &os.PathError{Op: "inspect", Path: path, Err: err}
	}
	kind := info.FileAttributes & (windows.FILE_ATTRIBUTE_DIRECTORY | windows.FILE_ATTRIBUTE_REPARSE_POINT)
	return fmt.Sprintf("%d:%d:%d:%d", kind, info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}

// Windows retirement has no root-owned system file edits.
func legacyFileOwner(path string) (uint32, uint32, uint32, error) {
	return 0, 0, 0, fmt.Errorf("POSIX ownership is unavailable for %s on Windows", path)
}
