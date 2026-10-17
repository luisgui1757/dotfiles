package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func Lock(directory string) (func() error, error) {
	if err := prepareStateDirectory(directory); err != nil {
		return nil, err
	}
	return lockFile(directory, true)
}

func lockFile(directory string, create bool) (func() error, error) {
	path, err := windows.UTF16PtrFromString(filepath.Join(directory, "mutation.lock"))
	if err != nil {
		return nil, err
	}
	disposition := uint32(windows.OPEN_EXISTING)
	if create {
		disposition = windows.OPEN_ALWAYS
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, disposition, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, errors.Join(err, windows.CloseHandle(handle))
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, errors.Join(errors.New("mutation lock must not be a reparse point"), windows.CloseHandle(handle))
	}
	overlapped := &windows.Overlapped{}
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped); err != nil {
		return nil, errors.Join(fmt.Errorf("another installer holds the mutation lock: %w", err), windows.CloseHandle(handle))
	}
	return func() error { return windows.CloseHandle(handle) }, nil
}

// Root.Rename uses FileRenameInformationEx with POSIX replacement semantics on
// Windows. MoveFileEx cannot replace an open destination even when a preview
// shares delete access. Flush both the staged data (saveDocument) and the
// published file; return every publication/flush failure to the journal caller.
func publishState(from, to string) (result error) {
	if filepath.Dir(from) != filepath.Dir(to) {
		return errors.New("state publication must stay in one directory")
	}
	root, err := os.OpenRoot(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	if err := root.Rename(filepath.Base(from), filepath.Base(to)); err != nil {
		return err
	}
	file, err := root.OpenFile(filepath.Base(to), os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}
