package installer

import (
	"golang.org/x/sys/unix"
	"os"
)

func renameConfigNoReplace(from *os.File, old string, to *os.File, next string) error {
	return unix.RenameatxNp(int(from.Fd()), old, int(to.Fd()), next, unix.RENAME_EXCL)
}
