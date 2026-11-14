package installer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNeovimLinuxMasonPrerequisitesUseHiddenAPTResources(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	location := &LinuxAPTLocation{APTGet: program, Dpkg: program, DpkgQuery: program, APTMark: program, root: true}
	driver, err := configureLinuxAPT(location, catalog, &nativeSession{Directory: filepath.Join(t.TempDir(), "worker")}, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		ids, err := catalog.Closure([]string{"neovim"}, target)
		if err != nil {
			t.Fatal(err)
		}
		for name, flag := range map[string]string{"curl": "--version", "unzip": "-v"} {
			id := "tool." + name
			resource, found := catalog.Resource(id)
			if !found || resource.Capability || resource.Scope != "machine" || !resource.Shared || resource.Bindings["linux"] != (Binding{Provider: "apt", Package: name}) {
				t.Errorf("missing hidden APT download/unpack prerequisite: %s %+v", id, resource)
			}
			if slices.Contains(ids, id) != (target.OS == "linux") {
				t.Errorf("Mason prerequisite closure on %s/%s: %s %v", target.OS, target.Arch, id, ids)
			}
			if target.OS == "linux" && slices.Index(ids, id) >= slices.Index(ids, "nvim.sync") {
				t.Errorf("Mason prerequisite is ordered after synchronization: %s %v", id, ids)
			}
			if driver.Packages[id] != name || !slices.Equal(driver.Checks[id], []string{"/usr/bin/" + name, flag}) {
				t.Errorf("missing executable health check: %s %v", id, driver.Checks[id])
			}
		}
	}
}

func TestNodeLinuxRuntimeLibraryUsesHiddenAPTResource(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	location := &LinuxAPTLocation{APTGet: program, Dpkg: program, DpkgQuery: program, APTMark: program, root: true}
	driver, err := configureLinuxAPT(location, catalog, &nativeSession{Directory: filepath.Join(t.TempDir(), "worker")}, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	const id = "library.libatomic1"
	if driver.Packages[id] != "libatomic1" || !slices.Equal(driver.Checks[id], []string{program, "--verify", "libatomic1"}) {
		t.Errorf("missing APT package health check: %s %v", id, driver.Checks[id])
	}
	for _, target := range []Context{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		ids, err := catalog.Closure([]string{"neovim"}, target)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(ids, id) != (target.OS == "linux") {
			t.Errorf("Node runtime library closure on %s/%s: %v", target.OS, target.Arch, ids)
		}
		if target.OS == "linux" && slices.Index(ids, id) >= slices.Index(ids, "tool.node") {
			t.Errorf("Node runtime library is ordered after Node: %v", ids)
		}
	}
}
