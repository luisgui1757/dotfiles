package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNeovimWindowsFormattersUseMatchingStandaloneArchives(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "nvim", "mason-registry", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry []struct {
		Name   string
		Source struct{ ID string }
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "windows", Arch: "amd64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "darwin", Arch: "arm64"}} {
		closure, err := catalog.Closure([]string{"neovim"}, target)
		if err != nil {
			t.Fatal(err)
		}
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"gersemi", "clang-format"} {
			id := "tool." + name
			pin, pinned := pins[id]
			if slices.Contains(closure, id) != (target.OS == "windows") || pinned != (target.OS == "windows") {
				t.Fatalf("%s must use private archives only on Windows: %s closure=%v pinned=%v", name, target.OS, closure, pinned)
			}
			if !pinned {
				continue
			}
			resource, ok := catalog.Resource(id)
			if !ok || resource.Capability || resource.Bindings["windows"].Provider != "archive" || pin.Commands[name] == "" {
				t.Fatal("formatter lacks a hidden connected archive command", id, resource, pin)
			}
			if slices.Contains(resource.Dependencies(target), "tool.vcredist") != (name == "clang-format") || len(pin.Commands) != 1 || len(pin.BinDirs) != 1 {
				t.Fatal("standalone formatter runtime or command surface differs", name, resource, pin)
			}
			index := slices.IndexFunc(registry, func(p struct {
				Name   string
				Source struct{ ID string }
			}) bool {
				return p.Name == name
			})
			if index < 0 || registry[index].Source.ID != "pkg:pypi/"+name+"@"+pin.Version {
				t.Fatal("Windows and Mason formatter versions differ", name, pin.Version)
			}
			if !strings.HasSuffix(pin.Commands[name], ".exe") || pin.PythonStdlib != "" || pin.WindowsVenvRevision != 0 {
				t.Fatal("standalone formatter unexpectedly needs Python preparation", name, pin)
			}
		}
	}
}
