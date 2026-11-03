package installer

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
)

func TestZenLauncherKeepsProfileSelectionAndPayloadStable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux browser preparation")
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	pin := pins["tool.zen"]
	for _, generation := range []string{"first install", "replacement install"} {
		root := filepath.Join(t.TempDir(), generation)
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		// This fixture is the upstream process boundary, not a mocked provider.
		for _, name := range []string{"zen", "zen-bin"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nprintf '%s\\n' \"$MOZ_LEGACY_PROFILES\" \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := prepareZen(pin, root); err != nil {
			t.Fatal(err)
		}
		policy, err := os.ReadFile(filepath.Join(root, "distribution", "policies.json"))
		if err != nil || string(policy) != "{\"policies\":{\"DisableAppUpdate\":true}}\n" {
			t.Fatal("missing native updater policy", string(policy), err)
		}
		before, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(filepath.Join(root, "zen"), "https://example.com/a b?c=$d").CombinedOutput()
		if err != nil || string(out) != "1\nhttps://example.com/a b?c=$d\n" {
			t.Fatal(string(out), err)
		}
		// nsProfileLock's documented O_CREAT|O_TRUNC operation must leave the
		// sealed content/mode fingerprint unchanged, under any caller umask.
		lock, err := os.OpenFile(filepath.Join(root, ".parentlock"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		after, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil || before != after {
			t.Fatal("normal startup changed the payload", before, after, err)
		}
		if err := os.WriteFile(filepath.Join(root, ".parentlock"), []byte("personal change"), 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := snapshotTree(root, maxPackageBytes, maxPackageEntries)
		if err != nil || changed == before {
			t.Fatal("lock changes must remain protected", err)
		}
	}
}

func TestMacZenLauncherPreservesVendorBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX application launcher")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "bundle ü with spaces")
	vendor := filepath.Join(root, "Zen.app")
	for _, name := range []string{"Contents/MacOS", "Contents/Resources"} {
		if err := os.MkdirAll(filepath.Join(vendor, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vendor, "Contents/Info.plist"), []byte(`<plist version="1.0"><dict><key>CFBundleExecutable</key><string>zen</string></dict></plist>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendor, "Contents/MacOS/zen"), []byte("#!/bin/sh\nprintf '%s\\n' \"$0\" \"$MOZ_LEGACY_PROFILES\" \"$PWD\" \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(vendor, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareZen(pins["tool.zen"], root); err != nil {
		t.Fatal(err)
	}
	working := filepath.Join(root, "personal work")
	if err := os.Mkdir(working, 0700); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(root, "Published Zen.app")
	if err := os.Symlink(filepath.Join(root, "DotfilesZen.app"), published); err != nil {
		t.Fatal(err)
	}
	// Cocoa locates Mozilla's XPCOM libraries through the executable's bundle.
	// Passing an executable path through the wrapper bundle's ../ components
	// made the real vendor executable fail before startup on macOS.
	want := filepath.Join(vendor, "Contents", "MacOS", "zen") + "\n1\n" + working + "\nhttps://example.com/a b\nrelative note.txt\n"
	for _, bundle := range []string{filepath.Join(root, "DotfilesZen.app"), published} {
		command := exec.Command(filepath.Join(bundle, "Contents/MacOS/zen"), "https://example.com/a b", "relative note.txt")
		command.Dir = working
		out, err := command.CombinedOutput()
		if err != nil || string(out) != want {
			t.Fatal(string(out), err)
		}
	}
	after, err := snapshotTree(vendor, maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		t.Fatal("signed vendor bundle changed", before, after, err)
	}
}

func TestMacZenPreparationsKeepVersionedLauncherBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX application launcher")
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	for revision, want := range []string{
		`#!/bin/sh
set -eu
zen_dir=$(CDPATH= cd -- "$(/usr/bin/dirname "$0")" && pwd -P)
export MOZ_LEGACY_PROFILES=1
exec "$zen_dir/../../../Zen.app/Contents/MacOS/zen" "$@"
`,
		`#!/bin/sh
set -eu
zen_dir=$(CDPATH= cd -- "$(/usr/bin/dirname "$0")" && pwd -P)
zen_dir=$(CDPATH= cd -- "$zen_dir/../../../Zen.app/Contents/MacOS" && pwd -P)
export MOZ_LEGACY_PROFILES=1
exec "$zen_dir/zen" "$@"
`,
	} {
		t.Run(strconv.Itoa(revision), func(t *testing.T) {
			pin := pins["tool.zen"]
			pin.ZenProfileRevision = revision // Older saved pins omitted this field.
			root := t.TempDir()
			writeConfigFixture(t, filepath.Join(root, "Zen.app", "Contents", "Info.plist"), "original metadata")
			if err := prepareZen(pin, root); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "DotfilesZen.app", "Contents", "MacOS", "zen"))
			if err != nil || string(got) != want {
				t.Fatal("versioned preparation recipe changed", string(got), err)
			}
			info, err := os.ReadFile(filepath.Join(root, "DotfilesZen.app", "Contents", "Info.plist"))
			if err != nil || string(info) != "original metadata" {
				t.Fatal("saved launcher metadata changed", string(info), err)
			}
		})
	}
}

func TestMacZenLauncherDeclaresSupportedArchitecture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX application launcher")
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeConfigFixture(t, filepath.Join(root, "Zen.app", "Contents", "Info.plist"), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>zen</string><key>LSEnvironment</key><dict><key>MallocNanoZone</key><string>0</string></dict></dict></plist>`)
	if err := prepareZen(pins["tool.zen"], root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "DotfilesZen.app", "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>zen</string><key>LSEnvironment</key><dict><key>MallocNanoZone</key><string>0</string></dict><key>LSArchitecturePriority</key><array><string>arm64</string></array>
</dict></plist>`
	if string(data) != want {
		t.Fatalf("saved revision-2 metadata changed: %s", data)
	}
	var info struct {
		Dict struct {
			Keys   []string `xml:"key"`
			Arrays []struct {
				Architectures []string `xml:"string"`
			} `xml:"array"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(info.Dict.Keys, []string{"CFBundleExecutable", "LSEnvironment", "LSArchitecturePriority"}) || len(info.Dict.Arrays) != 1 || !slices.Equal(info.Dict.Arrays[0].Architectures, []string{"arm64"}) {
		t.Fatalf("launcher must declare Apple Silicon instead of script-only Rosetta default: %s", data)
	}
}

func TestMacZenLauncherMetadataRejectsUnexpectedVendorShape(t *testing.T) {
	for _, source := range []string{"", "invalid", "<plist><dict>", "<plist><array/></plist>", "<plist><dict/></plist>", "<plist><dict></dict></plist><plist/>", "<plist><dict/><dict/></plist>", "<plist><dict><key>LSArchitecturePriority</key><array><string>x86_64</string></array></dict></plist>"} {
		if _, err := macZenLauncherInfo([]byte(source)); err == nil {
			t.Fatal("unexpected vendor metadata accepted", source)
		}
	}
}

func TestZenPreparationRevisionBoundToPlatform(t *testing.T) {
	for _, target := range []NativePlatform{{Context: linux, Libc: "glibc"}, {Context: Context{OS: "darwin", Arch: "arm64"}}} {
		pins, err := DefaultArchivePins(target)
		if err != nil {
			t.Fatal(err)
		}
		for revision := 0; revision <= 3; revision++ {
			pin := pins["tool.zen"]
			pin.ZenProfileRevision = revision
			err := validateZenPin(pin)
			want := revision <= 1 || (revision == 2 && target.OS == "darwin")
			if (err == nil) != want {
				t.Fatal(target, revision, err)
			}
		}
	}
}
