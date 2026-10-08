package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeGypPreparationHasExactScopeAndUpdatesLegacyPublication(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"-"+target.Arch, func(t *testing.T) {
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			pin := pins["tool.node"]
			expected := "lib/node_modules/npm/node_modules/node-gyp/gyp/gyp_main.py"
			if target.OS == "windows" {
				expected = strings.TrimPrefix(expected, "lib/")
			}
			if len(pin.Replacements) != 1 {
				t.Fatal("Node requires exactly one scoped gyp preparation")
			}
			replacement := pin.Replacements[0]
			if replacement.File != expected || replacement.Before != "import gyp  # noqa: E402" || replacement.After != "sys.dont_write_bytecode = True\nimport gyp  # noqa: E402" || replacement.Count != 0 {
				t.Fatal("unexpected gyp preparation", replacement)
			}
			legacy := pin
			legacy.Replacements = nil
			if err := legacy.Validate(); err != nil {
				t.Fatal("legacy pin became unreadable", err)
			}
			if archivePinID(legacy) == archivePinID(pin) {
				t.Fatal("preparation did not change desired identity")
			}
			c, d, _ := archiveController(t)
			text := "import sys\n" + replacement.Before + "\n"
			fixture, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "node fixture", Mode: 0755}, {Name: replacement.File, Text: text, Mode: 0644}}), "tar.gz")
			d.Pins["tool.shared"], d.Client = fixture, client
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			old, err := d.PayloadPath("tool.shared")
			if err != nil {
				t.Fatal(err)
			}
			fixture.Replacements = pin.Replacements
			d.Pins["tool.shared"] = fixture
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			payload, err := d.PayloadPath("tool.shared")
			if err != nil || payload == old {
				t.Fatal("update did not replace old generation", err)
			}
			data, err := os.ReadFile(filepath.Join(payload, filepath.FromSlash(replacement.File)))
			if err != nil || string(data) != "import sys\n"+replacement.After+"\n" {
				t.Fatal("rendered preparation differs", string(data), err)
			}
			result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || result.Status != "ready" {
				t.Fatal("prepared publication not ready", result, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Stat(payload); !os.IsNotExist(err) {
				t.Fatal("owned prepared payload survived removal", err)
			}
		})
	}
}
