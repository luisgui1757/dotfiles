package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileBlockRoundTripPreservesEncodingAndAllOutsideBytes(t *testing.T) {
	fixtures := [][]byte{nil, []byte("# personal"), []byte("# café\n"), []byte("# personal\r\n"), {0xef, 0xbb, 0xbf, '#', ' ', 'x'}}
	for _, bom := range [][]byte{{0xff, 0xfe}, {0xfe, 0xff}} {
		encode, _, err := profileEncoding(bom)
		if err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, append(bytes.Clone(bom), encode("# café 🌍\r\n")...))
	}
	for _, original := range fixtures {
		block, err := renderProfileBlock(original, "integration.starship", "# prompt\necho 'ready'")
		if err != nil {
			t.Fatal(err)
		}
		installed, old, err := replaceProfileBlock(original, "integration.starship", block)
		if err != nil || len(old) != 0 {
			t.Fatal(err, old)
		}
		again, _, err := replaceProfileBlock(installed, "integration.starship", block)
		if err != nil || !bytes.Equal(again, installed) {
			t.Fatal("repeat changed the profile", err)
		}
		restored, _, err := replaceProfileBlock(installed, "integration.starship", nil)
		if err != nil || !bytes.Equal(restored, original) {
			t.Fatalf("round trip changed bytes: %x / %x: %v", original, restored, err)
		}
	}
}

func TestProfileRejectsMalformedOrAmbiguousInput(t *testing.T) {
	for _, data := range [][]byte{{0xff}, {0xff, 0xfe, 0}, {0xff, 0xfe, 0, 0xd8}, []byte("x\x00y"), []byte("# >>> dotfiles:test >>>\n"), []byte("\n# >>> dotfiles:test >>>\nx\n# <<< dotfiles:test <<<\n\n# >>> dotfiles:test >>>\nx\n# <<< dotfiles:test <<<\n")} {
		if _, _, err := replaceProfileBlock(data, "test", nil); err == nil {
			t.Fatalf("accepted malformed input: %x", data)
		}
	}
}

func profileController(t *testing.T) (Controller, *ProfileDriver, string) {
	t.Helper()
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "profile with space.ps1")
	d := &ProfileDriver{Directory: filepath.Join(home, "state"), Targets: map[string][]ProfileTarget{"profile.test": {{path, "# first version"}}}, Windows: runtime.GOOS == "windows"}
	catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "test", Name: "Test", Capability: true, Requires: []string{"profile.test"}}, {ID: "profile.test", Name: "Profile", Action: "profile", Scope: "user"}}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	return Controller{Catalog: catalog, Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Source: "profile-tests", Home: home, StatePath: filepath.Join(d.Directory, "state.json"), Driver: d}, d, path
}

func TestProfileInstallUpdateAndRemovePreserveOutsideEdits(t *testing.T) {
	c, d, path := profileController(t)
	original := "# personal café\r\n"
	writeConfigFixture(t, path, original)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("# later user edit\r\n")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("outside edit invalidated block ownership", check, err)
	}
	d.Targets["profile.test"][0].Script = "# second version"
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original+"# later user edit\r\n" {
		t.Fatalf("outside edits were changed: %q %v", got, err)
	}
}

func TestProfileRemovalDeletesOnlyNewEmptyProfile(t *testing.T) {
	for _, edited := range []bool{false, true} {
		c, _, path := profileController(t)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		if edited {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, []byte("# keep me")...), 0600); err != nil {
				t.Fatal(err)
			}
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		data, err := os.ReadFile(path)
		if edited {
			if err != nil || !strings.HasSuffix(string(data), "# keep me") {
				t.Fatal(string(data), err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("left a newly created empty profile", err)
		}
	}
}

func TestProfileEditedManagedBlockIsRetained(t *testing.T) {
	c, _, path := profileController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.ReplaceAll(data, []byte("first version"), []byte("user version"))
	if err := os.WriteFile(path, edited, 0600); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, edited) {
		t.Fatal("removed an edited managed block", err)
	}
}

func TestProfileUnchangedUpdateDoesNotRewriteTheFile(t *testing.T) {
	c, _, path := profileController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged update rewrote profile", err)
	}
}

func TestProfilePublicationPreservesReadOnlyMode(t *testing.T) {
	c, d, path := profileController(t)
	writeConfigFixture(t, path, "# personal read-only profile")
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"install", "update", "remove"} {
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
		if mode == "update" {
			d.Targets["profile.test"][0].Script = "# updated integration"
			request = Request{Schema: 1, Mode: "update"}
		} else if mode == "remove" {
			request.Selected = []string{}
		}
		dispatchApproved(t, c, request)
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0200 != 0 {
			t.Fatal("publication failed to retain read-only permissions", mode, info, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "# personal read-only profile" {
		t.Fatal("removal failed to restore personal content", string(data), err)
	}
}

func TestProfileFinishedHistoryDoesNotRequireTheCurrentTargetCatalog(t *testing.T) {
	c, d, _ := profileController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	delete(d.Targets, "profile.test")
	if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "apply"}, nil); err != nil {
		t.Fatal("finished historical profile blocked unrelated work", err)
	}
}
