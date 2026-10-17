package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchivePreparationRequiresExactReviewedContents(t *testing.T) {
	for _, test := range []struct {
		name, contents string
		count          int
		want           string
	}{
		{"unique", "old command\n", 0, "new command\n"},
		{"explicit repeated", "old old\n", 2, "new new\n"},
		{"missing", "changed command\n", 0, ""},
		{"unexpected duplicate", "old old\n", 0, ""},
		{"wrong count", "old\n", 2, ""},
		{"non text", "old\xff", 0, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: test.contents, Mode: 0755}}), "tar.gz")
			pin.Replacements = []ArchiveReplacement{{File: "tool", Before: "old", After: "new", Count: test.count}}
			destination := filepath.Join(t.TempDir(), "payload")
			err := downloadArchive(context.Background(), client, pin, destination)
			if test.want == "" {
				if err == nil {
					t.Fatal("unreviewed source was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(destination, "tool"))
			if err != nil || string(data) != test.want {
				t.Fatal("prepared payload differs", string(data), err)
			}
			info, err := os.Stat(filepath.Join(destination, "tool"))
			if err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
				t.Fatal("preparation lost executable permissions", err)
			}
		})
	}
}

func TestArchivePreparationRejectsReplacementThroughLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX archive symbolic-link layout")
	}
	entries := []archiveFixtureEntry{{Name: "tool", Text: "old", Mode: 0755}, {Name: "alias", Link: "tool"}}
	pin, client, _ := archivePinFor(t, archiveTar(t, entries), "tar.gz")
	pin.Replacements = []ArchiveReplacement{{File: "alias", Before: "old", After: "new"}}
	if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil {
		t.Fatal("replacement followed an archive symbolic link")
	}
}

func TestArchivePreparationBoundsExpandedReplacement(t *testing.T) {
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: strings.Repeat("x", 32), Mode: 0755}}), "tar.gz")
	pin.Replacements = []ArchiveReplacement{{File: "tool", Before: "x", After: strings.Repeat("y", 1<<20), Count: 32}}
	if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatal("text preparation exceeded its expansion bound", err)
	}
}

func TestArchiveExcludesOnlyExactReviewedNonRuntimeMembers(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []archiveFixtureEntry
		wantOK  bool
	}{
		{"regular", []archiveFixtureEntry{{Name: "tests/run", Text: "test only", Mode: 0755}}, true},
		{"dangling submodule link", []archiveFixtureEntry{{Name: "tests/run", Link: "../lib/test-runner"}}, true},
		{"absent", nil, false},
		{"escaping link", []archiveFixtureEntry{{Name: "tests/run", Link: "../../escape"}}, false},
		{"duplicate", []archiveFixtureEntry{{Name: "tests/run", Text: "one"}, {Name: "tests/run", Text: "two"}}, false},
		{"remaining dangling link", []archiveFixtureEntry{{Name: "tests/run", Text: "test"}, {Name: "other", Link: "missing"}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries := append([]archiveFixtureEntry{{Name: "tool", Text: "runtime", Mode: 0755}}, test.entries...)
			pin, client, _ := archivePinFor(t, archiveTar(t, entries), "tar.gz")
			pin.ExcludedFiles = []string{"tests/run"}
			destination := filepath.Join(t.TempDir(), "payload")
			err := downloadArchive(context.Background(), client, pin, destination)
			if !test.wantOK {
				if err == nil {
					t.Fatal("exclusion bypassed archive safety or source validation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(destination, "tests", "run")); !os.IsNotExist(err) {
				t.Fatal("excluded member was published", err)
			}
			data, err := os.ReadFile(filepath.Join(destination, "tool"))
			if err != nil || string(data) != "runtime" {
				t.Fatal("exclusion changed runtime member", err)
			}
		})
	}
}

func TestArchivePreparationValidationAndLegacyPinIdentity(t *testing.T) {
	pin, _, _ := archivePinFor(t, []byte("tool"), "file")
	pin.File = "tool"
	legacy := archivePinID(pin)
	data, err := json.Marshal(pin)
	if err != nil || strings.Contains(string(data), "replacements") || strings.Contains(string(data), "excluded_files") {
		t.Fatal("optional preparation changed a legacy pin's persisted shape", err)
	}
	var restored ArchivePin
	if err := json.Unmarshal(data, &restored); err != nil || archivePinID(restored) != legacy {
		t.Fatal("legacy pin identity changed", err)
	}
	for _, change := range []func(*ArchivePin){
		func(p *ArchivePin) { p.ExcludedFiles = []string{"../outside"} },
		func(p *ArchivePin) { p.ExcludedFiles = []string{"tool"} },
		func(p *ArchivePin) { p.RequiredFiles, p.ExcludedFiles = []string{"config"}, []string{"config"} },
		func(p *ArchivePin) { p.ExcludedFiles = []string{"tests/run", "tests/run"} },
		func(p *ArchivePin) {
			p.Replacements = []ArchiveReplacement{{File: "../outside", Before: "a", After: "b"}}
		},
		func(p *ArchivePin) { p.Replacements = []ArchiveReplacement{{File: "tool", After: "b"}} },
		func(p *ArchivePin) { p.Replacements = []ArchiveReplacement{{File: "tool", Before: "a", After: "a"}} },
		func(p *ArchivePin) {
			p.Replacements = []ArchiveReplacement{{File: "tool", Before: "a", After: "b", Count: -1}}
		},
		func(p *ArchivePin) {
			p.ExcludedFiles = []string{"test"}
			p.Replacements = []ArchiveReplacement{{File: "test", Before: "a", After: "b"}}
		},
	} {
		invalid := pin
		change(&invalid)
		if err := invalid.Validate(); err == nil {
			t.Fatal("invalid preparation accepted", invalid)
		}
	}
	pin.Replacements = []ArchiveReplacement{{File: "tool", Before: "a", After: "b"}}
	if err := pin.Validate(); err != nil || archivePinID(pin) == legacy {
		t.Fatal("reviewed preparation was not part of approval identity", err)
	}
}
