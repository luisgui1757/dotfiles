package installer

import (
	"context"
	"debug/pe"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func windowsTerminalPin(t *testing.T) ArchivePin {
	t.Helper()
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "windows", Arch: "amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := pins["tool.windows-terminal"]
	if !ok {
		t.Fatal("Windows Terminal requires its official unpackaged archive pin")
	}
	return pin
}

func TestWindowsTerminalPinUsesTheUnpackagedStableLayout(t *testing.T) {
	pin := windowsTerminalPin(t)
	if pin.Format != "zip" || pin.StripComponents != 1 || pin.Commands["wt"] != "wt.exe" || !slices.Contains(pin.RequiredFiles, "WindowsTerminal.exe") {
		t.Fatal("Windows Terminal pin no longer exposes the reviewed unpackaged entrypoints", pin)
	}
}

// This opt-in downloads/extracts official bytes in a temporary directory. It
// never runs a Windows executable, registers a package or changes any profile.
func TestWindowsTerminalOfficialPayload(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_WINDOWS_TERMINAL_ARCHIVE") != "1" {
		t.Skip("set DOTFILES_TEST_WINDOWS_TERMINAL_ARCHIVE=1 for the official pinned payload")
	}
	pin := windowsTerminalPin(t)
	destination := filepath.Join(t.TempDir(), "payload")
	if err := downloadArchive(context.Background(), nil, pin, destination); err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"wt.exe"}, pin.RequiredFiles...) {
		if info, err := os.Stat(filepath.Join(destination, name)); err != nil || !info.Mode().IsRegular() {
			t.Fatal("required payload missing", name, err)
		}
	}
	for _, name := range []string{"wt.exe", "WindowsTerminal.exe", "OpenConsole.exe", "TerminalApp.dll"} {
		file, err := pe.Open(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		machine := file.Machine
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			t.Fatal("unexpected native Windows architecture", name, machine)
		}
	}
	if _, err := os.Lstat(filepath.Join(destination, ".portable")); !os.IsNotExist(err) {
		t.Fatal("payload unexpectedly redirects mutable settings into its generation", err)
	}
	// References must resolve to actions shipped by these exact pinned bytes.
	defaults, err := os.ReadFile(filepath.Join(destination, "defaults.json"))
	if err != nil {
		t.Fatal(err)
	}
	clean, _, err := jsoncMask(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var builtin struct{ Actions []struct{ ID string } }
	if err := json.Unmarshal(clean, &builtin); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, action := range builtin.Actions {
		ids[action.ID] = true
	}
	_, driver, _ := windowsTerminalController(t)
	projection, err := decodeWindowsTerminalProjection([]byte(driver.Targets[windowsTerminalResource][0].Script))
	if err != nil {
		t.Fatal(err)
	}
	for field, raw := range projection.Values {
		if !strings.HasPrefix(field, "binding:") {
			continue
		}
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			t.Fatal(err)
		}
		if id != "Dotfiles.CloseTab" && !ids[id] {
			t.Fatal("fixed keybinding references an action absent from the pinned payload", field, id)
		}
	}
}
