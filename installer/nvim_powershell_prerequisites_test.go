package installer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNeovimWindowsPowerShellPrerequisiteAndCommandPath(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"-"+target.Arch, func(t *testing.T) {
			ids, err := catalog.Closure([]string{"neovim"}, target)
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(ids, "tool.powershell") != (target.OS == "windows") {
				t.Fatal("Neovim's Windows language runtime is missing or selected off target", ids)
			}
			if target.OS != "windows" {
				return
			}
			if slices.Index(ids, "tool.powershell") >= slices.Index(ids, "nvim.sync") {
				t.Fatal("PowerShell runtime must exist before synchronization", ids)
			}
			resource, found := catalog.Resource("tool.powershell")
			if !found || resource.Capability || !resource.Shared || resource.Bindings["windows"].Provider != "archive" {
				t.Fatal("Neovim did not reuse the existing hidden PowerShell archive", resource)
			}
			root, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			platform := NativePlatform{Context: target}
			pins, err := DefaultArchivePins(platform)
			if err != nil {
				t.Fatal(err)
			}
			archives := &ArchiveDriver{Directory: filepath.Join(root, "packages"), Pins: pins}
			folders := ConfigFolders{Home: root, LocalAppData: filepath.Join(root, "local")}
			driver, err := configureNvimSync(filepath.Join(root, "checkout"), filepath.Join(root, "state"), platform, folders, catalog, archives, newNativeSession(filepath.Join(root, "worker")))
			if err != nil {
				t.Fatal(err)
			}
			directories, err := archives.BinaryDirectories("tool.powershell")
			if err != nil || len(directories) == 0 {
				t.Fatal("missing pinned PowerShell command directory", err)
			}
			var commands []string
			for _, value := range driver.Environment {
				if path, ok := strings.CutPrefix(value, "PATH="); ok {
					commands = filepath.SplitList(path)
				}
			}
			for _, directory := range directories {
				if !slices.Contains(commands, directory) {
					t.Fatal("declared PowerShell runtime is unreachable by the Neovim consumer", directory, commands)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
				t.Fatal("constructing language command paths wrote installer state", err)
			}
		})
	}
}
