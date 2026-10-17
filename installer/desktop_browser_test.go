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

func TestFlatpakDesktopLauncherPreservesSpecialCharacterPaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux desktop entry consumer")
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("gio is exercised by the native desktop job")
	}
	root := t.TempDir()
	store := filepath.Join(root, "quotes\" back\\ dollar$ percent% tick` space")
	d := DesktopDriver{Target: linux, Folders: ConfigFolders{Home: root}, Archives: &ArchiveDriver{}, Flatpak: &FlatpakDriver{Directory: store, Arch: "x86_64"}}
	p, err := d.recipe("desktop.obsidian")
	if err != nil {
		t.Fatal(err)
	}
	// Replace only the external Flatpak process with a harmless environment
	// recorder; consume the actual generated Exec arguments through GIO.
	probe := filepath.Join(root, "flatpak-probe")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\nprintf '%s\\n' \"$FLATPAK_USER_DIR\" \"$FLATPAK_SYSTEM_DIR\" \"$FLATPAK_CONFIG_DIR\" > \"$DOTFILES_DESKTOP_ARGUMENTS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	contents := strings.Replace(p.Contents, "/usr/bin/flatpak run ", probe+" run ", 1)
	launcher, result := filepath.Join(root, "obsidian.desktop"), filepath.Join(root, "result")
	if err := os.WriteFile(launcher, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(gio, "launch", launcher)
	command.Env = append(os.Environ(), "DOTFILES_DESKTOP_ARGUMENTS="+result)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(result)
		if err == nil && len(data) > 0 {
			installation := d.Flatpak.store("tool.obsidian")
			want := strings.Join([]string{installation, installation, filepath.Join(installation, "config"), ""}, "\n")
			if string(data) != want {
				t.Fatalf("desktop parser changed private store paths: got %q want %q", data, want)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("desktop consumer did not execute the Flatpak launcher")
}
