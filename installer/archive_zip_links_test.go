package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func archiveLinkZIP(t *testing.T, entries []archiveFixtureEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		mode, contents := os.FileMode(0644)|os.FileMode(entry.Mode), entry.Text
		if entry.Link != "" {
			mode, contents = os.ModeSymlink|0777, entry.Link
		}
		header.SetMode(mode)
		member, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(member, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestArchiveZIPPreservesFrameworkLinksAfterWritingPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS framework ZIP layout requires POSIX symbolic links")
	}
	entries := []archiveFixtureEntry{
		{Name: "Framework/Library", Link: "Versions/Current/Library"},
		{Name: "Framework/Versions/Current", Link: "A"},
		{Name: "Framework/Versions/A/Library", Text: "verified library", Mode: 0755},
	}
	pin, client, _ := archivePinFor(t, archiveLinkZIP(t, entries), "zip")
	pin.Commands["tool"] = "Framework/Library"
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(base, "payload")
	if err := downloadArchive(context.Background(), client, pin, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "Framework", "Library"))
	if err != nil || string(data) != "verified library" {
		t.Fatal("framework link did not resolve to its payload", err)
	}
	info, err := os.Lstat(filepath.Join(destination, "Framework", "Versions", "Current"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("framework link was flattened", err)
	}
}

func TestArchiveZIPRejectsUnsafeOrAmbiguousLinks(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []archiveFixtureEntry
	}{
		{"escape", []archiveFixtureEntry{{Name: "link", Link: "../personal"}}},
		{"absolute", []archiveFixtureEntry{{Name: "link", Link: "/personal"}}},
		{"dangling", []archiveFixtureEntry{{Name: "link", Link: "missing"}}},
		{"cycle", []archiveFixtureEntry{{Name: "one", Link: "two"}, {Name: "two", Link: "one"}}},
		{"write through link", []archiveFixtureEntry{{Name: "redirect", Link: "target"}, {Name: "target/kept", Text: "keep"}, {Name: "redirect/changed", Text: "bad"}}},
		{"duplicate", []archiveFixtureEntry{{Name: "link", Link: "tool"}, {Name: "link", Text: "replacement"}}},
		{"oversized", []archiveFixtureEntry{{Name: "link", Link: strings.Repeat("a", 4097)}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			personal := filepath.Join(base, "personal")
			if err := os.WriteFile(personal, []byte("keep personal"), 0600); err != nil {
				t.Fatal(err)
			}
			entries := append([]archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}}, test.entries...)
			pin, client, _ := archivePinFor(t, archiveLinkZIP(t, entries), "zip")
			if err := downloadArchive(context.Background(), client, pin, filepath.Join(base, "payload")); err == nil {
				t.Fatal("unsafe ZIP link was published")
			}
			if data, err := os.ReadFile(personal); err != nil || string(data) != "keep personal" {
				t.Fatal("ZIP changed an entry outside its payload", err)
			}
		})
	}
}
