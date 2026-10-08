package installer

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func renameConfigNoReplace(from *os.File, old string, to *os.File, next string) (result error) {
	name, err := windows.NewNTUnicodeString(old)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(from.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var source windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&source, windows.DELETE|windows.SYNCHRONIZE, &attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(source)) }()
	encoded, err := windows.UTF16FromString(next)
	if err != nil {
		return err
	}
	// FILE_RENAME_INFORMATION, ReplaceIfExists=false. The destination is a
	// single name relative to the pinned directory, never an absolute string.
	// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information
	type renameInformation struct {
		Replace uint32
		Root    windows.Handle
		Length  uint32
		Name    [1]uint16
	}
	var header renameInformation
	length := (len(encoded) - 1) * 2
	buffer := make([]byte, int(unsafe.Offsetof(header.Name))+length+2)
	info := (*renameInformation)(unsafe.Pointer(&buffer[0]))
	info.Root, info.Length = windows.Handle(to.Fd()), uint32(length)
	copy(unsafe.Slice(&info.Name[0], len(encoded)), encoded)
	return windows.NtSetInformationFile(source, &status, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation)
}
