package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGhosttyPreparationKeepsLegacyPinShape(t *testing.T) {
	legacy := `{"version":"1.0","url":"https://example.org/tool","sha256":"` + strings.Repeat("a", 64) + `","format":"file","file":"tool","commands":null,"bin_dirs":null,"required_files":["tool"]}`
	var pin ArchivePin
	if err := json.Unmarshal([]byte(legacy), &pin); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(pin)
	if err != nil || string(data) != legacy || pin.Validate() != nil {
		t.Fatal("optional Ghostty preparation changed the legacy persisted pin", string(data), err)
	}
}

func ghosttyPreparationFixture(t *testing.T) (ArchivePin, []byte) {
	t.Helper()
	entries := []archiveFixtureEntry{{Name: "usr/bin/ghostty", Text: "#!/bin/sh\nprintf '%s\\n' \"$LD_LIBRARY_PATH\" \"$@\"\n", Mode: 0755}}
	required := []string{"usr/bin/ghostty-bin", "usr/lib/libgtk4-layer-shell.so", "usr/share/terminfo/x/xterm-ghostty", "usr/share/ghostty/themes/Rose Pine"}
	for _, name := range required[1:] {
		entries = append(entries, archiveFixtureEntry{Name: name, Text: "private runtime"})
	}
	return ArchivePin{Version: "1.3.1", Format: "deb", GhosttyLibraries: true, Commands: map[string]string{"ghostty": "usr/bin/ghostty"}, BinDirs: []string{"usr/bin"}, RequiredFiles: required}, debTarFixture(t, entries)
}

func TestGhosttyLibraryPreparationUsesStableCommandAndOwnedLifecycle(t *testing.T) {
	decoder := debDecoderFixture(t, "exec cat")
	pin, data := ghosttyPreparationFixture(t)
	download, client, _ := archivePinFor(t, data, "deb")
	pin.URL, pin.SHA256 = download.URL, download.SHA256
	c, d, _ := archiveController(t)
	d.Pins["tool.shared"], d.Client, d.DpkgDeb = pin, client, decoder
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	program, err := d.CommandPath("tool.shared", "ghostty")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(program, "argument with spaces", "$literal")
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + c.Home, "LD_LIBRARY_PATH=/prior-library-path"}
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "/usr/bin/../lib:/prior-library-path\nargument with spaces\n$literal\n") {
		t.Fatal("Ghostty wrapper lost library path or literal arguments", string(output), err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("Ghostty use modified the managed payload", check, err)
	}
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Stat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Ghostty removal left owned runtime files", err)
	}
}

func TestGhosttyPreparationRejectsUnexpectedLayoutAndExistingBinary(t *testing.T) {
	pin, _ := ghosttyPreparationFixture(t)
	for _, mutate := range []func(*ArchivePin){
		func(p *ArchivePin) { p.Format = "zip" },
		func(p *ArchivePin) { p.RequiredFiles = nil },
		func(p *ArchivePin) { p.PortableGit = true },
		func(p *ArchivePin) { p.StripComponents = 1 },
	} {
		invalid := pin
		mutate(&invalid)
		if err := validateArchivePreparation(invalid); err == nil {
			t.Fatal("accepted an unsupported Ghostty preparation")
		}
	}
	payload := t.TempDir()
	if err := os.MkdirAll(filepath.Join(payload, "usr", "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ghostty", "ghostty-bin"} {
		if err := os.WriteFile(filepath.Join(payload, "usr", "bin", name), []byte(name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := prepareGhosttyLibraries(pin, payload); err == nil {
		t.Fatal("preparation replaced an existing binary")
	}
	data, err := os.ReadFile(filepath.Join(payload, "usr", "bin", "ghostty-bin"))
	if err != nil || string(data) != "ghostty-bin" {
		t.Fatal("preparation changed a conflicting entry", err)
	}
}
