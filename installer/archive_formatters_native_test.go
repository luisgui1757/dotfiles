package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeArchiveWindowsFormatters(t *testing.T) {
	if runtime.GOOS != "windows" || os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("requires verified archive downloads on a disposable Windows host with the VC runtime")
	}
	nativeWindowsFormatterLifecycle(t)
}

// Also called by the fresh VC-runtime container after its prerequisite installs.
// Ordinary archive tests reuse the runtime; they never install system packages.
func nativeWindowsFormatterLifecycle(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; $key=Get-ItemProperty 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\FileSystem'; [int]$key.LongPathsEnabled")
	policy, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("cannot inspect Windows path policy: %s %v", policy, err)
	}
	t.Log("LongPathsEnabled:", strings.TrimSpace(string(policy)))
	if os.Getenv("DOTFILES_TEST_PYTHON_DEFAULT_WINDOWS") == "1" && strings.TrimSpace(string(policy)) != "0" {
		t.Fatal("default-policy acceptance requires long paths disabled")
	}
	local := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(local) {
		t.Fatal("formatter proof requires the actual LocalAppData prefix")
	}
	root, err := os.MkdirTemp(local, "dotfiles-formatters-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gersemi", "clang-format"} {
		t.Run(name, func(t *testing.T) {
			id := "tool." + name
			c, archives, _ := archiveController(t)
			for i := range c.Catalog.Resources {
				resource := &c.Catalog.Resources[i]
				if resource.ID == "tool.shared" {
					resource.ID = id
				} else {
					resource.Requires = []string{id}
				}
			}
			archives.Directory = filepath.Join(root, "packages")
			archives.Pins, archives.Client = map[string]ArchivePin{id: pins[id]}, nil
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			payload, err := archives.PayloadPath(id)
			if err != nil {
				t.Fatal(err)
			}
			entrypoint, err := archives.CommandPath(id, name)
			if err != nil {
				t.Fatal(err)
			}
			before, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
			if err != nil {
				t.Fatal(err)
			}
			version := nativePreparedCommand(t, c.Home, entrypoint, "", "--version")
			if !strings.Contains(version, pins[id].Version) {
				t.Fatal("formatter runtime version differs", version)
			}
			if name == "gersemi" {
				fixture := filepath.Join(c.Home, "CMakeLists.txt")
				writeConfigFixture(t, fixture, "cmake_minimum_required( VERSION 3.20 )\nproject( acceptance )\n")
				nativePreparedCommand(t, c.Home, entrypoint, "", "--in-place", fixture)
				data, err := os.ReadFile(fixture)
				if err != nil || string(data) != "cmake_minimum_required(VERSION 3.20)\nproject(acceptance)\n" {
					t.Fatal("CMake formatting differs", string(data), err)
				}
			} else {
				output := nativePreparedCommand(t, c.Home, entrypoint, "int main(){return 0;}\n", "--style=LLVM")
				if strings.TrimSpace(output) != "int main() { return 0; }" {
					t.Fatal("C/C++ formatting differs", output)
				}
			}
			after, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
			if err != nil || before != after {
				t.Fatal("formatter execution changed its immutable payload", err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			if current, err := archives.PayloadPath(id); err != nil || current != payload {
				t.Fatal("unchanged-pin update replaced the formatter", current, err)
			}
			if err := os.Remove(archives.currentLink(id)); err != nil {
				t.Fatal(err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
			if output := nativePreparedCommand(t, c.Home, entrypoint, "", "--version"); !strings.Contains(output, pins[id].Version) {
				t.Fatal("repaired formatter cannot run", output)
			}
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("formatter check failed", check, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Lstat(payload); !os.IsNotExist(err) {
				t.Fatal("owned formatter survived removal", err)
			}
			t.Log(name, "formatted input, retained immutable bytes, updated, repaired and removed at production generation depth")
		})
	}
}
