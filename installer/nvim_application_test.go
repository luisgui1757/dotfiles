package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNvimRuntimeDirectoryUsesTargetDataLayout(t *testing.T) {
	root := t.TempDir()
	folders := ConfigFolders{Home: root, LocalAppData: filepath.Join(root, "local"), Data: filepath.Join(root, "xdg")}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "windows", Arch: "amd64"}} {
		name := "nvim"
		if target.OS == "windows" {
			name += "-data"
		}
		path, err := nvimRuntimeDirectory(target, folders)
		if err != nil || path != filepath.Join(folders.Data, name, "dotfiles-runtime") {
			t.Fatal("wrong target data layout", target, path, err)
		}
	}
	folders.Data = "relative"
	if _, err := nvimRuntimeDirectory(linux, folders); err == nil {
		t.Fatal("relative data directory accepted")
	}
}

func TestNvimCompilerEnvironmentIsResolvedAfterPrerequisiteAndPreservesManagedPath(t *testing.T) {
	ready, dispatched := false, false
	discover := func(context.Context) (map[string]string, error) {
		if !ready {
			return nil, errors.New("compiler is not installed")
		}
		return map[string]string{"PATH": "compiler", "INCLUDE": "sdk"}, nil
	}
	command := nativeCommand{Environment: []string{"PATH=managed", "HOME=private"}}
	run := withCompilerEnvironment(discover, func(_ context.Context, got nativeCommand) ([]byte, error) {
		dispatched = true
		if !slices.Contains(got.Environment, "PATH=compiler"+string(os.PathListSeparator)+"managed") || !slices.Contains(got.Environment, "INCLUDE=sdk") {
			t.Fatal("compiler environment lost selected tool paths", got.Environment)
		}
		return nil, nil
	})
	if _, err := run(context.Background(), command); err == nil || dispatched {
		t.Fatal("phase ran before compiler prerequisite")
	}
	ready = true
	if _, err := run(context.Background(), command); err != nil || !dispatched {
		t.Fatal("phase did not resolve newly installed compiler", err)
	}
	if !slices.Equal(command.Environment, []string{"PATH=managed", "HOME=private"}) {
		t.Fatal("compiler binding mutated the shared command")
	}
}

func TestNvimApplicationUsesExplicitDataAndManagedCommandsWithoutWrites(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "foreign data"))
	t.Setenv("PATH", filepath.Join(root, "foreign commands"))
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{nativePlatform(t).Context} {
		t.Run(target.OS, func(t *testing.T) {
			platform := NativePlatform{Context: target, Libc: "glibc"}
			pins, err := DefaultArchivePins(platform)
			if err != nil {
				t.Fatal(err)
			}
			archives := &ArchiveDriver{Directory: filepath.Join(root, target.OS, "packages"), Pins: pins}
			folders := ConfigFolders{Home: filepath.Join(root, target.OS, "home"), Data: filepath.Join(root, target.OS, "data"), LocalAppData: filepath.Join(root, target.OS, "local")}
			state := filepath.Join(root, target.OS, "state")
			session := newNativeSession(filepath.Join(state, "native", "worker"))
			d, err := configureNvimSync(filepath.Join(root, "checkout"), state, platform, folders, catalog, archives, session)
			if err != nil {
				t.Fatal(err)
			}
			name := "nvim"
			if target.OS == "windows" {
				name += "-data"
			}
			if d.RuntimeDirectory != filepath.Join(folders.Data, name, "dotfiles-runtime") {
				t.Fatal("runtime ignored the independent data root", d.RuntimeDirectory)
			}
			if strings.Contains(strings.Join(d.Environment, "\n"), "foreign") {
				t.Fatal("constructor inherited caller paths", d.Environment)
			}
			for _, id := range []string{"tool.node", "tool.python", "tool.cmake", "tool.tree-sitter"} {
				dirs, err := archives.BinaryDirectories(id)
				if err != nil || !strings.Contains(strings.Join(d.Environment, "\n"), dirs[0]) {
					t.Fatal("missing declared Neovim prerequisite command", id, err)
				}
			}
			folders.Data = ""
			d, err = configureNvimSync(filepath.Join(root, "checkout"), state, platform, folders, catalog, archives, session)
			defaultData := filepath.Join(folders.Home, ".local", "share")
			if target.OS == "windows" {
				defaultData = folders.LocalAppData
			}
			if err != nil || d.RuntimeDirectory != filepath.Join(defaultData, name, "dotfiles-runtime") {
				t.Fatal("fixture default inherited caller data", err)
			}
			if _, err := os.Lstat(state); !os.IsNotExist(err) {
				t.Fatal("construction created installer state", err)
			}
		})
	}
}

func TestNativeHomebrewAbsenceAllowsBootstrapPreviewBeforeFormulaInventory(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	d := &NativeDriver{Catalog: catalog, Context: Context{OS: "darwin", Arch: "arm64"}, Homebrew: f.d}
	bootstrap, _ := catalog.Resource("infra.homebrew")
	if !bootstrap.ReplanAfter || !bootstrap.Retain || bootstrap.Capability {
		t.Fatal("Homebrew must be hidden retained infrastructure with a new inventory preview")
	}
	absent, err := d.Observe(context.Background(), bootstrap, Receipt{})
	if err != nil || absent.Present || absent.Pending != "" {
		t.Fatal("fresh infrastructure cannot be planned", absent, err)
	}
	git, _ := catalog.Resource("tool.git")
	pending, err := d.Observe(context.Background(), git, Receipt{})
	if err != nil || !pending.Unknown || pending.Pending == "" {
		t.Fatal("unavailable formula inventory aborted bootstrap preview", pending, err)
	}
	closure, err := catalog.Closure([]string{"tool.git"}, d.Context)
	if err != nil || slices.Index(closure, "infra.homebrew") > slices.Index(closure, "tool.git") {
		t.Fatal("bootstrap must precede formula inventory", closure, err)
	}
	if len(f.commands) != 0 {
		t.Fatal("preview mutated native infrastructure")
	}
}
