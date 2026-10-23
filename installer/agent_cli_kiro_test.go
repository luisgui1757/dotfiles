package installer

import (
	"context"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestKiroCLIIsAPinnedMacAndLinuxSelection(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	version := ""
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			offered := slices.ContainsFunc(catalog.Capabilities(target), func(r Resource) bool { return r.ID == "kiro-cli" })
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			pin, pinned := pins["tool.kiro"]
			if target.OS == "windows" {
				// Kiro ships only a machine-wide, self-updating MSI for Windows.
				if offered || pinned {
					t.Fatal("Kiro CLI is offered on Windows", offered, pinned)
				}
				return
			}
			if !offered || !pinned {
				t.Fatal("Kiro CLI is missing from a supported target", offered, pinned)
			}
			closure, err := catalog.Closure([]string{"kiro-cli"}, target)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"tool.kiro", kiroSettingsResource, "tool.git", "integration.shells"} {
				if !slices.Contains(closure, id) {
					t.Fatal("Kiro CLI lacks its pinned command, update switch or prerequisite", closure, id)
				}
			}
			for _, id := range closure {
				resource, _ := catalog.Resource(id)
				if resource.Capability && id != "kiro-cli" || slices.Contains([]string{"tool.node", "tool.fd", "tool.rg"}, id) {
					t.Fatal("Kiro CLI selected an unrelated capability or runtime", closure, id)
				}
			}
			if setting, _ := catalog.Resource(kiroSettingsResource); setting.Action != "profile" || !slices.Equal(setting.Requires, []string{"tool.kiro"}) || setting.Shared || setting.Scope != "user" {
				t.Fatal("the update switch must be a user-scoped field owned only with Kiro CLI", setting)
			}
			if version != "" && pin.Version != version {
				t.Fatal("Kiro CLI targets disagree on the reviewed release", version, pin.Version)
			}
			version = pin.Version
			u, err := url.Parse(pin.URL)
			if err != nil || u.Host != "prod.download.cli.kiro.dev" || !strings.HasPrefix(u.Path, "/stable/"+version+"/") {
				t.Fatal("Kiro CLI must use the vendor's fixed-version stable download", pin.URL, err)
			}
			for _, command := range []string{"kiro-cli", "kiro-cli-chat", "kiro-cli-term"} {
				if pin.Commands[command] == "" || !slices.Contains(pin.BinDirs, path.Dir(pin.Commands[command])) {
					t.Fatal("Kiro CLI helper is not a sibling on PATH", command, pin.Commands, pin.BinDirs)
				}
			}
			if target.OS == "darwin" {
				if pin.Format != "dmg" || pin.AppBundle != "Kiro CLI.app" || pin.DMGRevision != 1 || path.Base(u.Path) != "Kiro CLI.dmg" ||
					!strings.HasPrefix(pin.Commands["kiro-cli"], "Kiro CLI.app/Contents/MacOS/") || !slices.Contains(pin.RequiredFiles, "Kiro CLI.app/Contents/Info.plist") {
					t.Fatal("macOS Kiro CLI must run from its private vendor bundle", pin)
				}
				return
			}
			arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[target.Arch]
			if pin.Format != "zip" || pin.Libc != "glibc" || path.Base(u.Path) != "kirocli-"+arch+"-linux.zip" || pin.StripComponents != 1 ||
				!slices.Equal(pin.ExcludedFiles, []string{"bin/q", "bin/qchat"}) {
				t.Fatal("Linux Kiro CLI must use the glibc archive without its ~/.local/bin aliases", pin)
			}
		})
	}
	// The glibc archive needs an observed glibc; absent or musl discovery is not one.
	for _, libc := range []string{"", "musl"} {
		pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: libc})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := pins["tool.kiro"]; ok {
			t.Fatal("Kiro CLI glibc archive selected without observed glibc", libc)
		}
	}
}

func TestKiroCLILinuxArchiveExposesOnlyItsNativeCommands(t *testing.T) {
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	layout := func(aliases bool) []archiveFixtureEntry {
		entries := []archiveFixtureEntry{
			{Name: "kirocli/BUILD-INFO", Text: "BUILD_VERSION=fixture\n", Mode: 0644},
			{Name: "kirocli/README", Text: "vendor readme\n", Mode: 0644},
			{Name: "kirocli/install.sh", Text: "#!/bin/sh\nexit 1\n", Mode: 0755},
			{Name: "kirocli/bin/kiro-cli", Text: "cli fixture", Mode: 0755},
			{Name: "kirocli/bin/kiro-cli-chat", Text: "chat fixture", Mode: 0755},
			{Name: "kirocli/bin/kiro-cli-term", Text: "pty fixture", Mode: 0755},
		}
		if aliases {
			entries = append(entries,
				archiveFixtureEntry{Name: "kirocli/bin/q", Text: "#!/bin/sh\n\"$HOME/.local/bin/kiro-cli\" --show-legacy-warning \"$@\"\n", Mode: 0755},
				archiveFixtureEntry{Name: "kirocli/bin/qchat", Text: "#!/bin/sh\n\"$HOME/.local/bin/kiro-cli\" --show-legacy-warning chat \"$@\"\n", Mode: 0755})
		}
		return entries
	}
	transport, client, _ := archivePinFor(t, archiveLinkZIP(t, layout(true)), "zip")
	pin := pins["tool.kiro"]
	pin.URL, pin.SHA256 = transport.URL, transport.SHA256
	payload := filepath.Join(t.TempDir(), "payload")
	if err := downloadArchive(context.Background(), client, pin, payload); err != nil {
		t.Fatal("vendor layout is not extractable", err)
	}
	for _, name := range []string{"BUILD-INFO", "install.sh", "bin/kiro-cli", "bin/kiro-cli-chat", "bin/kiro-cli-term"} {
		if _, err := os.Stat(filepath.Join(payload, filepath.FromSlash(name))); err != nil {
			t.Fatal("vendor file missing from the private payload", name, err)
		}
	}
	for _, alias := range []string{"q", "qchat"} {
		// These aliases run $HOME/.local/bin/kiro-cli, which this install never creates.
		if _, err := os.Lstat(filepath.Join(payload, "bin", alias)); !os.IsNotExist(err) {
			t.Fatal("a broken legacy alias would shadow other commands on PATH", alias, err)
		}
	}
	transport, client, _ = archivePinFor(t, archiveLinkZIP(t, layout(false)), "zip")
	pin.URL, pin.SHA256 = transport.URL, transport.SHA256
	err = downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "changed"))
	if err == nil || !strings.Contains(err.Error(), "exclusion is absent") {
		t.Fatal("a changed vendor layout passed the reviewed exclusions", err)
	}
}

func TestKiroCLIBindsSettingsSkillsAndPolicyToKiroHome(t *testing.T) {
	platform := nativePlatform(t)
	if platform.OS == "windows" {
		t.Skip("Kiro CLI is offered on macOS and Linux")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(home, "profiles", "work kiro")
	platform.Sentinel = SentinelLocations{KiroHome: work}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: home}
	c, err := NewNativeController(repository, "kiro-home", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	closure, err := c.Catalog.Closure([]string{"kiro-cli", "sentinel", "sentinel-skills"}, platform.Context)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := c.Driver.(*NativeDriver).ForSelection(closure, State{})
	if err != nil {
		t.Fatal(err)
	}
	driver := bound.(*NativeDriver)
	if settings := driver.Profiles.Targets[kiroSettingsResource]; len(settings) != 1 || settings[0].Path != filepath.Join(work, "settings", "cli.json") {
		t.Fatal("Kiro CLI settings ignore KIRO_HOME", settings)
	}
	if !slices.ContainsFunc(driver.Profiles.Targets[sentinelResource], func(target ProfileTarget) bool { return target.Path == filepath.Join(work, "steering", "AGENTS.md") }) {
		t.Fatal("Sentinel policy does not follow the selected Kiro CLI to KIRO_HOME", driver.Profiles.Targets[sentinelResource])
	}
	consumer, _, err := driver.SentinelSkills.conditional(sentinelKiroSkills)
	if err != nil || !consumer.Needed {
		t.Fatal("selecting Kiro CLI does not make Kiro present for skills", consumer, err)
	}
	if _, _, root, err := driver.SentinelSkills.configuration(Resource{ID: sentinelKiroSkills}); err != nil || root != filepath.Join(work, "skills") {
		t.Fatal("Kiro skills ignore KIRO_HOME", root, err)
	}
	if c.Driver.(*NativeDriver).SentinelSkills.Selected != nil {
		t.Fatal("binding a selection changed the shared skills driver")
	}
	if _, err := os.Lstat(work); !os.IsNotExist(err) {
		t.Fatal("binding created KIRO_HOME", err)
	}
}
