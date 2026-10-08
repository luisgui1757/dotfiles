package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestShellProfilesHaveOneOwnerAcrossAllSelections(t *testing.T) {
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	// These constructors bind real filesystem paths. The hosted native matrix
	// exercises all three OSes; a Windows volume is not a valid POSIX PATH entry.
	// Include discovered libc/distribution facts required by archive selection.
	platform := nativePlatform(t)
	for _, target := range []Context{platform.Context} {
		t.Run(target.OS, func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Documents: filepath.Join(home, "redirected documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming"), Zsh: home}
			c, err := NewNativeController(repository, "test", filepath.Join(home, "state"), platform, folders)
			if err != nil {
				t.Fatal(err)
			}
			d := c.Driver.(*NativeDriver)
			owners := map[string]string{}
			record := func(path, id string) {
				t.Helper()
				if old, ok := owners[path]; ok {
					t.Fatalf("profile has competing owners: %s: %s and %s", path, old, id)
				}
				owners[path] = id
			}
			for _, resource := range c.Catalog.Resources {
				if !resource.Available(target) {
					continue
				}
				if resource.Action == "config" {
					targets, err := d.Configurations.Manifest.Resolve(c.Catalog, target, folders, repository, resource.ID)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range targets {
						record(entry.Destination, resource.ID)
					}
				}
				for _, entry := range d.Profiles.Targets[resource.ID] {
					record(entry.Path, resource.ID)
				}
			}
			if target.OS == "windows" {
				for _, directory := range []string{"PowerShell", "WindowsPowerShell"} {
					if owners[filepath.Join(folders.Documents, directory, "profile.ps1")] != "integration.shells" {
						t.Fatal("all-host profile missed actual Documents folder")
					}
				}
			} else if owners[filepath.Join(folders.Zsh, ".zshrc")] != "integration.shells" {
				t.Fatal("actual ZDOTDIR not used")
			}
		})
	}
}

func TestSharedShellEnvironmentIsQuietAndQuotesPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell process test")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home space '$ ` * [x]")
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config")}
	c, err := NewNativeController(repository, "test", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	// Exercise the generated loader as an actual noninteractive login shell.
	// The engine's ownership/publication behavior has separate lifecycle tests.
	for _, entry := range d.Profiles.Targets["integration.shells"] {
		writeConfigFixture(t, entry.Path, entry.Script+"\n")
	}
	local := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(local, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bash", "zsh"} {
		shell, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		profile := ".bash_profile"
		if name == "zsh" {
			profile = ".zprofile"
		}
		script := ". " + shellLiteral(filepath.Join(home, profile)) + "; . " + shellLiteral(filepath.Join(home, profile)) + "; printf '%s\\n' \"$PATH\""
		cmd := exec.Command(shell, "-c", script)
		cmd.Env = append(os.Environ(), "HOME="+home, "PATH=/usr/bin:/bin")
		output, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != local+":/usr/bin:/bin" {
			t.Fatalf("quiet %s path setup: %q %v", name, output, err)
		}
	}
}

func TestConcurrentSelectionsDoNotMutateSharedShellRecipes(t *testing.T) {
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	c, err := NewNativeController(repository, "test", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	profiles := c.Driver.(*NativeDriver).Profiles
	before, err := digest(profiles.Targets)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan string, 12)
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			selected := []string{"starship"}
			if i%2 == 1 {
				selected = []string{"zoxide"}
			}
			plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: selected})
			if err != nil {
				t.Error(err)
				return
			}
			for _, op := range plan.Operations {
				if op.Resource == "integration.shells" {
					results <- selected[0] + ":" + op.Observed.Desired
					return
				}
			}
			t.Error("missing shared shell operation")
		}(i)
	}
	group.Wait()
	close(results)
	hashes := map[string]string{}
	for result := range results {
		id, value, _ := strings.Cut(result, ":")
		if old := hashes[id]; old != "" && old != value {
			t.Fatal("same selection had unstable recipe")
		}
		hashes[id] = value
	}
	if len(hashes) != 2 || hashes["starship"] == hashes["zoxide"] {
		t.Fatal("selection did not bind its own generated setup", hashes)
	}
	after, err := digest(profiles.Targets)
	if err != nil || before != after {
		t.Fatal("preview mutated shared provider state", err)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("preview wrote to home", entries, err)
	}
}
