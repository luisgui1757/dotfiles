package installer

import (
	"errors"
	"path/filepath"
	"strings"
)

// Load verified plugin files directly. The installer already owns their updates;
// a second plugin manager would add independent installation/removal authority.
func configureTmuxProfiles(d *ProfileDriver, target Context, folders ConfigFolders, repository string, archives *ArchiveDriver) error {
	if target.OS == "windows" {
		return nil
	}
	if !filepath.IsAbs(folders.Home) {
		return errors.New("tmux integration requires an absolute home directory")
	}
	arguments := []string{shellLiteral(filepath.Join(repository, "tmux", "plugins.sh"))}
	for _, name := range []string{"sensible", "yank", "resurrect", "continuum"} {
		entry, err := archives.RequiredFilePath("tmux."+name, name+".tmux")
		if err != nil {
			return err
		}
		arguments = append(arguments, shellLiteral(filepath.Dir(entry)))
	}
	command := strings.Join(arguments, " ")
	// tmux parses a configuration string, then expands formats, then passes the
	// command to the shell. Quote independently at each boundary.
	quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "#", "##").Replace(command)
	d.Targets["tmux.plugins"] = []ProfileTarget{{filepath.Join(folders.Home, ".tmux.plugins.conf"), "run-shell \"" + quoted + "\""}}
	return nil
}
