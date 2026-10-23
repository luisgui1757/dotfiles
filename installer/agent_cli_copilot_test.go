package installer

import (
	"context"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestCopilotCLIIsAPinnedStableSelectionOnEveryTarget(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	stable := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	version := ""
	for target, asset := range map[Context]string{
		{OS: "darwin", Arch: "arm64"}:  "copilot-darwin-arm64.tar.gz",
		linux:                          "copilot-linux-x64.tar.gz",
		{OS: "linux", Arch: "arm64"}:   "copilot-linux-arm64.tar.gz",
		{OS: "windows", Arch: "amd64"}: "copilot-win32-x64.zip",
	} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			if !slices.ContainsFunc(catalog.Capabilities(target), func(r Resource) bool { return r.ID == "copilot-cli" }) {
				t.Fatal("Copilot CLI is missing from the checkboxes")
			}
			closure, err := catalog.Closure([]string{"copilot-cli"}, target)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"tool.copilot", "tool.git", "integration.shells"}
			if target.OS == "windows" {
				// Copilot runs shell commands with PowerShell 6 or later on Windows.
				want = append(want, "tool.powershell")
			}
			for _, id := range want {
				if !slices.Contains(closure, id) {
					t.Fatal("Copilot CLI lacks a native prerequisite", closure, id)
				}
			}
			for _, id := range closure {
				resource, _ := catalog.Resource(id)
				if resource.Capability && id != "copilot-cli" || slices.Contains([]string{"tool.node", "tool.fd", "tool.rg"}, id) {
					t.Fatal("Copilot CLI selected an unrelated capability or runtime", closure, id)
				}
			}
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			pin, ok := pins["tool.copilot"]
			if !ok || !stable.MatchString(pin.Version) {
				t.Fatal("Copilot CLI needs a fixed stable release pin", pin.Version)
			}
			if version != "" && pin.Version != version {
				t.Fatal("Copilot CLI targets disagree on the reviewed release", version, pin.Version)
			}
			version = pin.Version
			u, err := url.Parse(pin.URL)
			if err != nil || u.Host != "github.com" || u.Path != "/github/copilot-cli/releases/download/v"+pin.Version+"/"+asset {
				t.Fatal("Copilot CLI must use its official standalone release asset", pin.URL, err)
			}
			command := "copilot"
			if target.OS == "windows" {
				command = "copilot.exe"
			}
			if pin.Commands["copilot"] != command || !slices.Equal(pin.BinDirs, []string{"."}) || pin.StripComponents != 0 {
				t.Fatal("Copilot CLI is one native executable at the payload root", pin)
			}
			if (pin.Libc == "glibc") != (target.OS == "linux") {
				t.Fatal("only the Linux builds require glibc", pin.Libc)
			}
		})
	}
	for _, libc := range []string{"", "musl"} {
		pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: libc})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := pins["tool.copilot"]; ok {
			t.Fatal("Copilot CLI glibc build selected without observed glibc", libc)
		}
	}
}

func TestCopilotCLIArchivesExtractTheirSingleExecutable(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target})
		if err != nil {
			t.Fatal(err)
		}
		pin := pins["tool.copilot"]
		entries := []archiveFixtureEntry{{Name: pin.Commands["copilot"], Text: "native CLI fixture", Mode: 0755}}
		data := archiveTar(t, entries)
		if pin.Format == "zip" {
			data = archiveLinkZIP(t, entries)
		}
		transport, client, _ := archivePinFor(t, data, pin.Format)
		pin.URL, pin.SHA256 = transport.URL, transport.SHA256
		payload := filepath.Join(t.TempDir(), "payload")
		if err := downloadArchive(context.Background(), client, pin, payload); err != nil {
			t.Fatal("Copilot release layout is not extractable", target, err)
		}
		if got, err := os.ReadFile(filepath.Join(payload, pin.Commands["copilot"])); err != nil || string(got) != "native CLI fixture" {
			t.Fatal("Copilot executable differs after extraction", target, string(got), err)
		}
		if path.Base(pin.Commands["copilot"]) != pin.Commands["copilot"] {
			t.Fatal("Copilot executable must sit at the archive root", pin.Commands)
		}
	}
}

func TestCopilotUpdatesAreDisabledOnlyForSelectedCLI(t *testing.T) {
	platform := nativePlatform(t)
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Data: filepath.Join(home, "data"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming"), Zsh: home}
	c, err := NewNativeController(repository, "copilot-choice", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range [][]string{{"codex"}, {"copilot-cli"}, {"codex", "copilot-cli"}} {
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
			found = found || strings.Contains(target.Script, "COPILOT_AUTO_UPDATE = 'false'") || strings.Contains(target.Script, "export COPILOT_AUTO_UPDATE=false")
		}
		if found != slices.Contains(selected, "copilot-cli") {
			t.Fatal("Copilot update policy does not follow the selection", selected, found)
		}
	}
}
