package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func xremapAutostartFixture(t *testing.T) (desktopRecipe, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(NativePlatform{Context: linux, Libc: "glibc"})
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config with space")
	d := DesktopDriver{Target: linux, Folders: ConfigFolders{Home: root, Config: config}, Archives: &ArchiveDriver{Directory: filepath.Join(root, "packages"), Pins: pins}}
	p, err := d.recipe("desktop.xremap")
	if err != nil {
		t.Fatal(err)
	}
	if p.Folder != config || p.Destination != "autostart/dotfiles-mac-keyboard.desktop" || !strings.Contains(p.Contents, "\nNoDisplay=true\n") {
		t.Fatal("xremap must start at login from the user's XDG autostart folder", p)
	}
	return p, config
}

func TestMacKeyboardAutostartEntryShape(t *testing.T) {
	p, _ := xremapAutostartFixture(t)
	if strings.Count(p.Contents, "\nExec=") != 1 || !strings.Contains(p.Contents, `Exec="/bin/sh" "-c" "case \\"\\$XDG_SESSION_TYPE\\"`) {
		t.Fatal("the session-type selection must be quoted for the Desktop Entry Exec key", p.Contents)
	}
}

// xremap's automatic backend probes X display :0, which is not every session's
// display. The entry picks the backend from the session type at login.
func TestMacKeyboardAutostartRunsXremapForTheSession(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux desktop entry consumer")
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("gio is exercised by the native desktop job")
	}
	p, config := xremapAutostartFixture(t)
	if err := os.MkdirAll(filepath.Dir(p.Command), 0700); err != nil {
		t.Fatal(err)
	}
	// The configuration is a live link into the checkout: xremap must receive
	// the checkout file, whose directory sees git's replacement on update.
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(checkout, "dotfiles checkout", "xremap", "config.yml")
	link := filepath.Join(config, "xremap", "config.yml")
	writeConfigFixture(t, source, "keymap: []\n")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Command, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOTFILES_XREMAP_ARGUMENTS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(t.TempDir(), "autostart.desktop")
	if err := os.WriteFile(entry, []byte(p.Contents), 0600); err != nil {
		t.Fatal(err)
	}
	for session, desktop := range map[string]string{"x11": "x11", "wayland": "gnome", "": "x11"} {
		arguments := filepath.Join(t.TempDir(), "arguments")
		command := exec.Command(gio, "launch", entry)
		command.Env = append(slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, "XDG_SESSION_TYPE=") }),
			"XDG_SESSION_TYPE="+session, "DOTFILES_XREMAP_ARGUMENTS="+arguments)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatal(string(output), err)
		}
		want := strings.Join([]string{"--desktop", desktop, "--watch=config,device", "--no-window-logging", source}, "\n") + "\n"
		deadline := time.Now().Add(3 * time.Second)
		for {
			data, err := os.ReadFile(arguments)
			if err == nil && len(data) > 0 {
				if string(data) != want {
					t.Fatalf("session %q started xremap with %q, want %q", session, data, want)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the autostart entry did not start xremap", session, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
