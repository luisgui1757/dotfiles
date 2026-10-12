package installer

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestT3UsesStableStandaloneArchives(t *testing.T) {
	stable := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	version := ""
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		pin := pins["tool.t3"]
		if !stable.MatchString(pin.Version) {
			t.Fatalf("T3 must use a fixed stable release on %s/%s, got %q", target.OS, target.Arch, pin.Version)
		}
		if version != "" && version != pin.Version {
			t.Fatal("T3 platforms disagree on the stable release", version, pin.Version)
		}
		version = pin.Version
		if !strings.HasPrefix(pin.URL, "https://github.com/pingdotgg/t3code/releases/download/v"+version+"/t3-"+version+"-") {
			t.Fatal("T3 must use the official standalone release", pin.URL)
		}
	}
}

func TestCodexNativePackagePreservesUpstreamMetadata(t *testing.T) {
	for target, triple := range map[Context]string{
		{OS: "linux", Arch: "amd64"}:   "x86_64-unknown-linux-musl",
		{OS: "linux", Arch: "arm64"}:   "aarch64-unknown-linux-musl",
		{OS: "darwin", Arch: "arm64"}:  "aarch64-apple-darwin",
		{OS: "windows", Arch: "amd64"}: "x86_64-pc-windows-msvc",
	} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			prefix := "package/vendor/" + triple + "/"
			suffix := ""
			if target.OS == "windows" {
				suffix = ".exe"
			}
			metadata := `{"name":"@openai/codex","license":"Apache-2.0"}`
			entries := []archiveFixtureEntry{
				{Name: "package/package.json", Text: metadata, Mode: 0644},
				{Name: "package/README.md", Text: "Retain upstream package documentation\n", Mode: 0644},
				{Name: prefix + "bin/codex" + suffix, Text: "native CLI fixture", Mode: 0755},
				{Name: prefix + "bin/codex-code-mode-host" + suffix, Text: "native runtime fixture", Mode: 0755},
				{Name: prefix + "codex-path/rg" + suffix, Text: "bundled search fixture", Mode: 0755},
				{Name: prefix + "codex-package.json", Text: "{}", Mode: 0644},
			}
			if target.OS == "linux" {
				entries = append(entries, archiveFixtureEntry{Name: prefix + "codex-resources/bwrap", Text: "sandbox fixture", Mode: 0755})
			}
			if target.OS == "windows" {
				for _, name := range []string{"codex-command-runner.exe", "codex-windows-sandbox-setup.exe"} {
					entries = append(entries, archiveFixtureEntry{Name: prefix + "codex-resources/" + name, Text: "sandbox fixture", Mode: 0755})
				}
			}
			archive := archiveTar(t, entries)
			transport, client, _ := archivePinFor(t, archive, "tar.gz")
			pin := pins["tool.codex"]
			pin.URL, pin.SHA256 = transport.URL, transport.SHA256
			payload := filepath.Join(t.TempDir(), "payload")
			if err := downloadArchive(context.Background(), client, pin, payload); err != nil {
				t.Fatal("native package layout is not extractable", err)
			}
			if got, err := os.ReadFile(filepath.Join(payload, "package.json")); err != nil || string(got) != metadata {
				t.Fatal("native extraction discarded upstream metadata", string(got), err)
			}
			if got, err := os.ReadFile(filepath.Join(payload, "README.md")); err != nil || string(got) != "Retain upstream package documentation\n" {
				t.Fatal("native extraction discarded upstream documentation", string(got), err)
			}
		})
	}
}

func TestAgentCLIsAreIndependentNativeSelections(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for capability, command := range map[string]string{"t3-code": "t3", "claude-code": "claude", "codex": "codex"} {
			t.Run(target.OS+"/"+target.Arch+"/"+capability, func(t *testing.T) {
				closure, err := catalog.Closure([]string{capability}, target)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"tool." + command, "tool.git", "integration.shells"} {
					if !slices.Contains(closure, id) {
						t.Fatal("agent selection lacks its native command prerequisite", closure, id)
					}
				}
				for _, id := range closure {
					resource, _ := catalog.Resource(id)
					if resource.Capability && id != capability || slices.Contains([]string{"tool.node", "tool.fd", "tool.rg"}, id) {
						t.Fatal("standalone agent selected an unrelated capability or runtime", closure, id)
					}
				}
				pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
				if err != nil || pins["tool."+command].Commands[command] == "" {
					t.Fatal("agent lacks a pinned native entrypoint", target, command, err)
				}
			})
		}
	}
}

func TestPiHasNoActiveInstallerRoute(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range catalog.Resources {
		if resource.ID == "pi" || strings.HasSuffix(resource.ID, ".pi") {
			t.Fatal("retired agent remains in the active catalog", resource.ID)
		}
	}
	manifest, err := DefaultConfigManifest(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range manifest.Targets {
		if strings.HasPrefix(target.Source, "pi/") || strings.HasPrefix(target.Path, ".pi/") {
			t.Fatal("retired agent configuration remains managed", target)
		}
	}
}

func TestClaudeUpdatesAreDelegatedOnlyForSelectedCLI(t *testing.T) {
	platform := nativePlatform(t)
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Data: filepath.Join(home, "data"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming"), Zsh: home}
	c, err := NewNativeController(repository, "agent-choice", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range [][]string{{"codex"}, {"claude-code"}, {"codex", "claude-code"}} {
		closure, err := c.Catalog.Closure(selected, platform.Context)
		if err != nil {
			t.Fatal(err)
		}
		driver, err := c.Driver.(*NativeDriver).ForSelection(closure, State{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, target := range driver.(*NativeDriver).Profiles.Targets["integration.shells"] {
			found = found || strings.Contains(target.Script, "DISABLE_UPDATES")
		}
		if found != slices.Contains(selected, "claude-code") {
			t.Fatal("Claude update policy does not follow the selection", selected, found)
		}
	}
}

func TestRemovingEitherAgentPreservesTheSelectedAgentAndGit(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		closure, err := catalog.Closure([]string{"claude-code", "codex"}, target)
		if err != nil {
			t.Fatal(err)
		}
		state := State{Schema: 1, Context: target, Selected: []string{"claude-code", "codex"}, Receipts: map[string]Receipt{}}
		observed := map[string]Observation{}
		for _, id := range closure {
			resource, _ := catalog.Resource(id)
			if resource.Action != "" {
				observation := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: id}
				observed[id] = observation
				state.Receipts[id] = Receipt{Ownership: "created", After: observation, Status: "ready"}
			}
		}
		for capability, command := range map[string]string{"claude-code": "claude", "codex": "codex"} {
			plan, err := PlanChanges(catalog, target, state, Request{Schema: 1, Mode: "apply", Selected: []string{capability}, RemoveShared: []string{"tool.git"}}, observed, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"tool." + command, "tool.git", "integration.shells"} {
				if action := operation(t, plan, id).Action; action != "keep" {
					t.Fatal("selected agent lost its command or shared prerequisite", capability, id, action)
				}
			}
			other := "tool.claude"
			if command == "claude" {
				other = "tool.codex"
			}
			if action := operation(t, plan, other).Action; action != "remove" {
				t.Fatal("deselected agent was not removed", other, action)
			}
		}
	}
}

func TestT3LinuxRequiresObservedGlibc(t *testing.T) {
	// Distribution identity alone must not authorize a glibc-only payload when
	// runtime discovery is absent or reports another libc.
	for _, arch := range []string{"amd64", "arm64"} {
		for _, libc := range []string{"", "musl", "glibc"} {
			pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "linux", Arch: arch}, Libc: libc})
			if err != nil {
				t.Fatal(err)
			}
			_, available := pins["tool.t3"]
			if available != (libc == "glibc") {
				t.Errorf("T3 availability on linux/%s with libc %q: %t", arch, libc, available)
			}
		}
	}
}
