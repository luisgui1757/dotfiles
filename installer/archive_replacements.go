package installer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func replaceArchiveText(root *os.Root, replacement ArchiveReplacement) error {
	name := filepath.FromSlash(replacement.File)
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return errors.Join(fmt.Errorf("archive replacement requires a bounded regular file: %s", replacement.File), err)
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (16<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	count := replacement.Count
	if count == 0 {
		count = 1
	}
	if len(data) > 16<<20 || !utf8.Valid(data) || strings.Count(string(data), replacement.Before) != count {
		return fmt.Errorf("archive replacement differs from its reviewed source: %s", replacement.File)
	}
	if growth := len(replacement.After) - len(replacement.Before); growth > 0 && count > ((16<<20)-len(data))/growth {
		return fmt.Errorf("archive replacement exceeds text size limit: %s", replacement.File)
	}
	updated := strings.Replace(string(data), replacement.Before, replacement.After, count)
	file, err = root.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, updated)
	return errors.Join(writeErr, file.Sync(), file.Close())
}
