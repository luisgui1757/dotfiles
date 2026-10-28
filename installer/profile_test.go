package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	return profileControllerAt(t, home)
}

func profileControllerAt(t *testing.T, home string) (Controller, *ProfileDriver, string) {
	t.Helper()
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

func TestProfileEditedMarkersNameTheirFile(t *testing.T) {
	c, _, path := profileController(t)
	writeConfigFixture(t, path, "# personal\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	finish := []byte("# <<< dotfiles:profile.test <<<")
	if err := os.WriteFile(path, bytes.Replace(data, finish, append(finish, " edited"...), 1), 0600); err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if report := fmt.Sprintf("%+v", check); err != nil || check.Status == "ready" || !strings.Contains(report, path+": profile.test: profile block markers were edited") {
		t.Fatal("edited markers did not name their file", report, err)
	}
}

func TestProfileBlockOpeningTheFileMayLoseOnlyItsLeadingNewline(t *testing.T) {
	for _, bom := range [][]byte{nil, {0xef, 0xbb, 0xbf}, {0xff, 0xfe}, {0xfe, 0xff}} {
		for _, nl := range []string{"\n", "\r\n"} {
			encode, _, err := profileEncoding(bom)
			if err != nil {
				t.Fatal(err)
			}
			block := encode(nl + "# >>> dotfiles:profile.test >>>" + nl + "# script" + nl + "# <<< dotfiles:profile.test <<<" + nl)
			opening := block[len(encode(nl)):]
			personal := encode("# personal" + nl)
			formatted := append(append(bytes.Clone(bom), opening...), personal...)
			content, got, err := replaceProfileBlock(formatted, "profile.test", nil)
			if err != nil || !bytes.Equal(got, block) || !bytes.Equal(content, append(bytes.Clone(bom), personal...)) {
				t.Fatalf("%x %q: opening block was not recognized as written: %q %q %v", bom, nl, got, content, err)
			}
			edited := append(append(append(bytes.Clone(bom), encode("x")...), opening...), personal...)
			if _, _, err := replaceProfileBlock(edited, "profile.test", nil); err == nil || !strings.Contains(err.Error(), "markers were edited") {
				t.Fatalf("%x %q: text before the opening marker was accepted: %v", bom, nl, err)
			}
		}
	}
}

func TestProfileBlockOpeningTheFileSurvivesFormatterAndLifecycle(t *testing.T) {
	c, d, path := profileController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("# >>> dotfiles:profile.test >>>")
	at := bytes.Index(data, marker)
	opening := bytes.TrimSuffix(bytes.TrimSuffix(data[:at], []byte("\n")), []byte("\r"))
	if at < 0 || len(opening) == at || len(bytes.TrimPrefix(opening, []byte{0xef, 0xbb, 0xbf})) != 0 {
		t.Fatalf("fixture block does not open a new profile: %q", data)
	}
	// shfmt, which Neovim runs on save, drops a file's leading blank lines.
	formatted := append(append(bytes.Clone(opening), data[at:]...), "# personal\n"...)
	if err := os.WriteFile(path, formatted, 0600); err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("a formatter's removed leading line broke block ownership", check, err)
	}
	d.Targets["profile.test"][0].Script = "# second version"
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	updated, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(updated, []byte("# second version")) || !bytes.HasSuffix(updated, []byte("# personal\n")) || bytes.Index(updated, marker) == len(opening) {
		t.Fatalf("update did not rewrite the opening block in place: %q %v", updated, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	removed, err := os.ReadFile(path)
	if err != nil || string(removed) != string(opening)+"# personal\n" {
		t.Fatalf("removal did not leave only the personal bytes: %q %v", removed, err)
	}
}

func TestProfileMixedNewlineOutsideEditsPreserveBlockLifecycle(t *testing.T) {
	for _, encoding := range []struct {
		name string
		bom  []byte
	}{{"utf8", nil}, {"utf8-bom", []byte{0xef, 0xbb, 0xbf}}, {"utf16-le", []byte{0xff, 0xfe}}, {"utf16-be", []byte{0xfe, 0xff}}} {
		for _, ending := range []struct{ name, owned, outside string }{{"lf", "\n", "\r\n"}, {"crlf", "\r\n", "\n"}} {
			t.Run("shell/"+encoding.name+"/"+ending.name, func(t *testing.T) {
				c, d, path := profileController(t)
				selected, id := "test", "profile.test"
				oldScript, newScript := "# first version", "# second version"
				encode, _, err := profileEncoding(encoding.bom)
				if err != nil {
					t.Fatal(err)
				}
				original := append(bytes.Clone(encoding.bom), encode("# personal café"+ending.owned)...)
				writeConfigFixture(t, path, string(original))
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{selected}})
				installed, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				outside := encode("# later personal edit 🌍" + ending.outside)
				edited := append(bytes.Clone(installed), outside...)
				if err := os.WriteFile(path, edited, 0600); err != nil {
					t.Fatal(err)
				}
				check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
				if err != nil || check.Status != "ready" {
					t.Fatal("outside line endings invalidated the managed block", check.Status, err)
				}
				preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update"})
				if err != nil || len(preview.Operations) != 1 || preview.Operations[0].Action != "keep" {
					t.Fatal("outside edit proposed a spurious repair", preview, err)
				}
				d.Targets[id][0].Script = strings.Replace(d.Targets[id][0].Script, oldScript, newScript, 1)
				dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
				updated, err := os.ReadFile(path)
				want := bytes.Replace(edited, encode(oldScript), encode(newScript), 1)
				if err != nil || !bytes.Equal(updated, want) {
					t.Fatal("update changed bytes outside its script", err)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
				restored, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(restored, append(bytes.Clone(original), outside...)) {
					t.Fatal("removal changed personal bytes", err)
				}
			})
		}
	}
}

func TestProfileUTF16FragmentsDoNotRelaxFullFileValidation(t *testing.T) {
	for _, id := range []string{"profile.test"} {
		for _, bom := range [][]byte{{0xff, 0xfe}, {0xfe, 0xff}} {
			encode, _, err := profileEncoding(bom)
			if err != nil {
				t.Fatal(err)
			}
			block := encode("\n# >>> dotfiles:profile.test >>>\n# saved script\n# <<< dotfiles:profile.test <<<\n")
			_, saved, err := replaceProfileBlock(block, id, nil)
			if err != nil || !bytes.Equal(saved, block) {
				t.Fatal("schema-1 encoded fragment was not readable", err)
			}
			path := filepath.Join(t.TempDir(), "profile.ps1")
			if err := os.WriteFile(path, block, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readProfile(path); err == nil {
				t.Fatal("accepted a full UTF-16 profile without its BOM")
			}
			for _, bad := range [][]byte{
				encode("# unmarked UTF-16"),
				append(encode("personal text\n"), block...),
				append(bytes.Clone(block), encode("trailing personal text")...),
				append(bytes.Clone(block), 0),
				append(bytes.Clone(block), block...),
			} {
				if _, _, err := replaceProfileBlock(bad, id, nil); err == nil {
					t.Fatal("accepted unmarked, incomplete or ambiguous saved UTF-16")
				}
			}
		}
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

// Missing parent folders belong to an absent original. Removal restores that
// absence unless a person has used a folder since; older baselines without the
// record keep their folders.
func TestProfileRemovalRestoresCreatedFolders(t *testing.T) {
	for _, variant := range []string{"unused", "personal", "older-baseline"} {
		t.Run(variant, func(t *testing.T) {
			c, d, _ := profileController(t)
			root := filepath.Join(c.Home, "agent config")
			steering := filepath.Join(root, "steering")
			d.Targets["profile.test"] = []ProfileTarget{{filepath.Join(steering, "AGENTS.md"), "# managed policy"}}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			switch variant {
			case "personal":
				writeConfigFixture(t, filepath.Join(root, "personal.txt"), "keep")
			case "older-baseline":
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				path := d.baselinePath(state.Receipts["profile.test"].Recovery)
				var baseline profileBaseline
				data, err := os.ReadFile(path)
				if err != nil || Decode(data, &baseline) != nil || !slices.Equal(baseline.Entries[0].Created, []string{steering, root}) {
					t.Fatal("first baseline did not record the created folders", baseline, err)
				}
				baseline.Entries[0].Created = nil
				if err := saveDocument(path, baseline); err != nil {
					t.Fatal(err)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			_, steeringErr := os.Lstat(steering)
			_, rootErr := os.Lstat(root)
			switch variant {
			case "unused":
				if !errors.Is(steeringErr, os.ErrNotExist) || !errors.Is(rootErr, os.ErrNotExist) {
					t.Fatal("removal left folders the publication created", steeringErr, rootErr)
				}
			case "personal":
				data, err := os.ReadFile(filepath.Join(root, "personal.txt"))
				if !errors.Is(steeringErr, os.ErrNotExist) || err != nil || string(data) != "keep" {
					t.Fatal("removal changed a folder a person used", steeringErr, err)
				}
			case "older-baseline":
				if steeringErr != nil || rootErr != nil {
					t.Fatal("an older baseline gained folder deletion authority", steeringErr, rootErr)
				}
			}
		})
	}
}

// Folder cleanup is cosmetic: a folder that cannot be removed is disclosed
// and finalization still completes, so Retry and Abandon stay available.
func TestProfileFolderCleanupFailureIsDisclosedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions enforced for an ordinary user")
	}
	c, d, _ := profileController(t)
	outer := filepath.Join(c.Home, "outer")
	root := filepath.Join(outer, "agent")
	d.Targets["profile.test"] = []ProfileTarget{{filepath.Join(root, "AGENTS.md"), "# managed policy"}}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	if err := os.Chmod(outer, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outer, 0700) })
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(root); err != nil || !strings.Contains(result.Message, "Preserved files for inspection: "+root) {
		t.Fatal("an unremovable created folder was not kept and disclosed", result.Message, err)
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
