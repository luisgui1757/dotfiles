package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Resolve vcvars only when the already-approved native phase runs: the compiler
// may be installed earlier in that same plan. Never retain a caller environment.
func withCompilerEnvironment(discover func(context.Context) (map[string]string, error), run func(context.Context, nativeCommand) ([]byte, error)) func(context.Context, nativeCommand) ([]byte, error) {
	return func(ctx context.Context, command nativeCommand) ([]byte, error) {
		environment, err := discover(ctx)
		if err != nil {
			return nil, err
		}
		basePath := ""
		for _, entry := range command.Environment {
			if path, ok := strings.CutPrefix(entry, "PATH="); ok {
				basePath = path
			}
		}
		var keys []string
		for key := range environment {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		command.Environment = slices.Clone(command.Environment)
		for _, key := range keys {
			value := environment[key]
			if key == "PATH" && basePath != "" {
				value += string(os.PathListSeparator) + basePath
			}
			command.Environment = append(command.Environment, key+"="+value)
		}
		return run(ctx, command)
	}
}

// Neovim's data root is independent of its configuration root. Windows appends
// nvim-data even with XDG_DATA_HOME set (Neovim src/nvim/os/stdpaths.c).
func configureNvimSync(repository, stateDirectory string, platform NativePlatform, folders ConfigFolders, catalog *Catalog, archives *ArchiveDriver, session *nativeSession) (*NvimSyncDriver, error) {
	if session == nil {
		return nil, errors.New("Neovim synchronization requires its native session")
	}
	runtimeDirectory, err := nvimRuntimeDirectory(platform.Context, folders)
	if err != nil {
		return nil, err
	}
	command, err := archives.CommandPath("tool.nvim", "nvim")
	if err != nil {
		return nil, err
	}
	closure, err := catalog.Closure([]string{"nvim.sync"}, platform.Context)
	if err != nil {
		return nil, err
	}
	var directories []string
	for _, id := range closure {
		if _, ok := archives.Pins[id]; !ok {
			continue
		}
		paths, err := archives.BinaryDirectories(id)
		if err != nil {
			return nil, err
		}
		directories = append(directories, paths...)
	}
	directories = append(directories, homebrewCommandDirectories(platform, catalog, closure)...)
	directories = append(directories, platform.SystemCommandDirectories...)
	if platform.OS != "windows" {
		directories = append(directories, "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	}
	if platform.WindowsPowerShell != "" {
		directories = append(directories, filepath.Dir(platform.WindowsPowerShell))
	}
	var path []string
	for _, directory := range directories {
		if !filepath.IsAbs(directory) || strings.ContainsAny(directory, "\x00\r\n"+string(os.PathListSeparator)) {
			return nil, errors.New("Neovim command directories must be absolute native paths")
		}
		if !slices.Contains(path, directory) {
			path = append(path, directory)
		}
	}
	return NewNvimSyncDriver(NvimSyncOptions{
		Repository: repository, Directory: filepath.Join(stateDirectory, "nvim-sync"),
		RuntimeDirectory: runtimeDirectory, Executable: command,
		Environment: []string{"PATH=" + strings.Join(path, string(os.PathListSeparator))}, Run: session.run,
	})
}

func nvimRuntimeDirectory(target Context, folders ConfigFolders) (string, error) {
	data, name := folders.Data, "nvim"
	if target.OS == "windows" {
		name = "nvim-data"
		if data == "" {
			data = folders.LocalAppData
		}
	} else if data == "" {
		data = filepath.Join(folders.Home, ".local", "share")
	}
	if !filepath.IsAbs(data) || filepath.Clean(data) != data || strings.ContainsAny(data, "\x00\r\n") {
		return "", errors.New("Neovim data folder must be an absolute canonical path")
	}
	return filepath.Join(data, name, "dotfiles-runtime"), nil
}
