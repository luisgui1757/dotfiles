package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestZenDesktopLauncherPassesURLsWithoutConvertingToFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux desktop entry consumer")
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("gio is exercised by the native desktop job")
	}
	root := t.TempDir()
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	d := DesktopDriver{Target: linux, Folders: ConfigFolders{Home: root}, Archives: &ArchiveDriver{Directory: filepath.Join(root, "packages"), Pins: pins}}
	p, err := d.recipe("desktop.zen")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p.Command), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Command, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOTFILES_DESKTOP_ARGUMENTS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	launcher, arguments := filepath.Join(root, "zen.desktop"), filepath.Join(root, "arguments")
	if err := os.WriteFile(launcher, []byte(p.Contents), 0600); err != nil {
		t.Fatal(err)
	}
	const url = "https://example.com/dotfiles?smoke=1"
	command := exec.Command(gio, "launch", launcher, url)
	command.Env = append(os.Environ(), "DOTFILES_DESKTOP_ARGUMENTS="+arguments)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(arguments)
		if err == nil && len(data) > 0 {
			if strings.TrimSpace(string(data)) != url {
				t.Fatalf("URL changed at the desktop boundary: %q", data)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("desktop consumer did not execute the published browser command")
}

// Linux Obsidian's portable build starts without Chromium's sandbox, which
// needs user namespaces that Ubuntu restricts, and still receives its
// obsidian:// links, through a private path with desktop-entry specials.
func TestObsidianDesktopLauncherDisablesChromiumSandboxAndForwardsLinks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux desktop entry consumer")
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("gio is exercised by the native desktop job")
	}
	root := t.TempDir()
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	// GLib checks that the program exists using the Exec value before it expands
	// %%, so GIO cannot start a program whose path contains a literal percent.
	packages := filepath.Join(root, "quotes\" back\\ dollar$ tick` space")
	d := DesktopDriver{Target: linux, Folders: ConfigFolders{Home: root}, Archives: &ArchiveDriver{Directory: packages, Pins: pins}}
	p, err := d.recipe("desktop.obsidian")
	if err != nil {
		t.Fatal(err)
	}
	if p.Command != filepath.Join(packages, obsidianPortable, "current", "obsidian") || p.Destination != "applications/dotfiles-obsidian.desktop" ||
		!strings.Contains(p.Contents, "\nStartupWMClass=md.obsidian.Obsidian\n") || !strings.Contains(p.Contents, "\nMimeType=x-scheme-handler/obsidian;\n") {
		t.Fatalf("unexpected Obsidian launcher %+v\n%s", p, p.Contents)
	}
	// Replace only the vendor executable with a harmless argument recorder and
	// consume the actual generated Exec line through GIO.
	if err := os.MkdirAll(filepath.Dir(p.Command), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Command, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOTFILES_DESKTOP_ARGUMENTS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	launcher, result := filepath.Join(root, "obsidian.desktop"), filepath.Join(root, "result")
	if err := os.WriteFile(launcher, []byte(p.Contents), 0600); err != nil {
		t.Fatal(err)
	}
	const link = "obsidian://open?vault=dotfiles%20notes"
	command := exec.Command(gio, "launch", launcher, link)
	command.Env = append(os.Environ(), "DOTFILES_DESKTOP_ARGUMENTS="+result)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(result)
		if err == nil && len(data) > 0 {
			if want := "--no-sandbox\n" + link + "\n"; string(data) != want {
				t.Fatalf("desktop entry changed Obsidian's arguments: got %q want %q", data, want)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("desktop consumer did not execute the published Obsidian command")
}
