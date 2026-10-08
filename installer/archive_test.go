package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

type archiveFixtureEntry struct {
	Name, Text, Link string
	Mode             int64
}

func archiveTar(t *testing.T, entries []archiveFixtureEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tarball := tar.NewWriter(gz)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.Name, Size: int64(len(entry.Text)), Mode: entry.Mode, Typeflag: tar.TypeReg}
		if entry.Link != "" {
			header.Typeflag, header.Linkname, header.Size = tar.TypeSymlink, entry.Link, 0
		}
		if err := tarball.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if entry.Link == "" {
			if _, err := io.WriteString(tarball, entry.Text); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := errors.Join(tarball.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func archivePinFor(t *testing.T, data []byte, format string) (ArchivePin, *http.Client, *atomic.Int32) {
	t.Helper()
	count := &atomic.Int32{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if _, err := w.Write(data); err != nil {
			t.Errorf("fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	hash := sha256.Sum256(data)
	return ArchivePin{Version: "1.0", URL: server.URL + "/tool", SHA256: hex.EncodeToString(hash[:]), Format: format, Commands: map[string]string{"tool": "tool"}, BinDirs: []string{"."}}, server.Client(), count
}

func TestArchiveRejectsCorruptDownloadBeforeExtraction(t *testing.T) {
	data := archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}})
	pin, client, _ := archivePinFor(t, data, "tar.gz")
	pin.SHA256 = strings.Repeat("0", 64)
	destination := filepath.Join(t.TempDir(), "payload")
	if err := downloadArchive(context.Background(), client, pin, destination); err == nil {
		t.Fatal("accepted incorrect checksum")
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("extracted bytes before checksum verification", err)
	}
}

func TestArchiveRejectsNonExecutableDeclaredCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use POSIX execute bits")
	}
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "not executable", Mode: 0644}}), "tar.gz")
	if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil {
		t.Fatal("accepted a declared command that cannot execute")
	}
}

func TestArchiveRejectsUnsafeAndAmbiguousPaths(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a/../../escape", `a\escape`, "a:stream", "NUL.txt", "a./tool", "COM1", "a//tool"} {
		t.Run(name, func(t *testing.T) {
			pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: name, Text: "bad", Mode: 0755}}), "tar.gz")
			if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
	for _, entries := range [][]archiveFixtureEntry{
		{{Name: "tool", Text: "one"}, {Name: "tool", Text: "two"}},
		{{Name: "tool", Link: "../outside"}},
		{{Name: "dir", Link: "inside"}, {Name: "dir/tool", Text: "redirected"}},
	} {
		pin, client, _ := archivePinFor(t, archiveTar(t, entries), "tar.gz")
		if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil {
			t.Fatal("accepted duplicate or redirecting member", entries)
		}
	}
}

func TestArchiveExtractsZIPAndTarWithVerifiedCommands(t *testing.T) {
	for _, format := range []string{"zip", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			data := archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}})
			if format == "zip" {
				var buffer bytes.Buffer
				writer := zip.NewWriter(&buffer)
				header := &zip.FileHeader{Name: "tool"}
				header.SetMode(0755)
				member, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(member, "program"); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				data = buffer.Bytes()
			}
			pin, client, _ := archivePinFor(t, data, format)
			destination := filepath.Join(t.TempDir(), "payload")
			if err := downloadArchive(context.Background(), client, pin, destination); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(destination, "tool"))
			if err != nil || string(got) != "program" {
				t.Fatal("extracted command differs", string(got), err)
			}
		})
	}
}

func archiveController(t *testing.T) (Controller, *ArchiveDriver, *atomic.Int32) {
	t.Helper()
	home := t.TempDir()
	home, err := resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "first", Name: "First", Capability: true, Requires: []string{"tool.shared"}}, {ID: "second", Name: "Second", Capability: true, Requires: []string{"tool.shared"}}, {ID: "tool.shared", Name: "Shared archive", Action: "archive"}}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	pin, client, requests := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "version one", Mode: 0755}}), "tar.gz")
	d := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: map[string]ArchivePin{"tool.shared": pin}, Client: client}
	return Controller{Catalog: catalog, Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Source: "archive-fixture", Home: home, StatePath: filepath.Join(home, "state", "state.json"), Driver: d}, d, requests
}

func TestPrivateArchiveLifecycleAndSharedRemoval(t *testing.T) {
	for _, retained := range []string{"first", "second"} {
		t.Run(retained, func(t *testing.T) {
			c, d, requests := archiveController(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first", "second"}})
			payload, err := d.PayloadPath("tool.shared")
			if err != nil {
				t.Fatal(err)
			}
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" || requests.Load() != 1 {
				t.Fatal("check mutated or failed", check, err, requests.Load())
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{retained}})
			if _, err := os.Stat(filepath.Join(payload, "tool")); err != nil {
				t.Fatal("removed shared package", err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("last consumer left package behind", err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			if requests.Load() != 2 {
				t.Fatal("reinstall did not republish from verified bytes")
			}
		})
	}
}

func TestPrivateArchiveUpdateRetiresOnlyUnchangedOldVersion(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	old, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	entrypoint, err := d.CommandPath("tool.shared", "tool")
	if err != nil {
		t.Fatal(err)
	}
	pin, client, requests := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "version two", Mode: 0755}}), "tar.gz")
	pin.Version = "2.0"
	d.Pins["tool.shared"], d.Client = pin, client
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	if requests.Load() != 1 {
		t.Fatal("update did not download reviewed version")
	}
	if _, err := os.Lstat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("obsolete private version remains", err)
	}
	active, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(active, "tool"))
	if err != nil || string(data) != "version two" {
		t.Fatal("update failed", string(data), err)
	}
	stable, err := d.CommandPath("tool.shared", "tool")
	if err != nil || stable != entrypoint {
		t.Fatal("update changed the command path", stable, entrypoint, err)
	}
	data, err = os.ReadFile(entrypoint)
	if err != nil || string(data) != "version two" {
		t.Fatal("existing terminal entrypoint was broken by update", string(data), err)
	}
}

func TestPrivateArchiveRepairsDeletedEntrypointWithoutRedownloading(t *testing.T) {
	c, d, requests := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	if err := os.Remove(d.currentLink("tool.shared")); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
	command, err := d.CommandPath("tool.shared", "tool")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(command)
	if err != nil || string(data) != "version one" || requests.Load() != 1 {
		t.Fatal("entrypoint repair rebuilt or failed", string(data), err, requests.Load())
	}
}

func TestPrivateArchiveDoesNotFollowRedirectedVersionsOnRemoval(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	versions := filepath.Join(d.resourceDirectory("tool.shared"), "versions")
	foreign := filepath.Join(c.Home, "moved-user-data")
	if err := os.Rename(versions, foreign); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(foreign, versions); err != nil {
		t.Fatal(err)
	}
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if operation(t, plan, "tool.shared").Action != "retain" {
		t.Fatal("redirected private data was selected for removal")
	}
}

func TestPrivateArchivePreservesEditedPayload(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, "tool"), []byte("user data"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range result.Operations {
		if op.Resource == "tool.shared" && op.Action != "retain" {
			t.Fatal("changed package was not retained", op)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err := os.ReadFile(filepath.Join(payload, "tool"))
	if err != nil || string(data) != "user data" {
		t.Fatal("removed edited private bytes", string(data), err)
	}
}

func TestPrivateArchiveRepeatedUpdateIsReadOnlyWhenPinsAreUnchanged(t *testing.T) {
	c, d, requests := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	before, err := os.ReadFile(filepath.Join(d.resourceDirectory("tool.shared"), "current.json"))
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	after, err := os.ReadFile(filepath.Join(d.resourceDirectory("tool.shared"), "current.json"))
	if err != nil || !bytes.Equal(before, after) || requests.Load() != 1 {
		t.Fatal("unchanged pinned update rewrote or downloaded package", err, requests.Load())
	}
}

func TestArchiveNonCommandPayloadRequiresItsDeclaredEntryFiles(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-module", true: "valid-module"}[present], func(t *testing.T) {
			name := "other.txt"
			if present {
				name = "module.psd1"
			}
			pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: name, Text: "@{}", Mode: 0644}}), "tar.gz")
			pin.Commands = nil
			pin.BinDirs = nil
			pin.RequiredFiles = []string{"module.psd1"}
			err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload"))
			if (err == nil) != present {
				t.Fatal("required module validation", err)
			}
		})
	}
}

func TestArchiveHandlesGitSourceGlobalPAXMetadata(t *testing.T) {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for _, header := range []*tar.Header{
		{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "pinned-git-commit"}},
		{Name: "source/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "source/module.zsh", Typeflag: tar.TypeReg, Mode: 0644, Size: 1},
	} {
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := tw.Write([]byte("#")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := errors.Join(tw.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	pin, client, _ := archivePinFor(t, buffer.Bytes(), "tar.gz")
	pin.Commands = nil
	pin.BinDirs = nil
	pin.RequiredFiles = []string{"module.zsh"}
	pin.StripComponents = 1
	root := filepath.Join(t.TempDir(), "payload")
	if err := downloadArchive(context.Background(), client, pin, root); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 1 || files[0].Name() != "module.zsh" {
		t.Fatal("metadata became an extracted file", files, err)
	}
}

func TestArchiveRejectsGlobalFileAttributeOverrides(t *testing.T) {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "global", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"path": "outside"}}); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(tw.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	pin, client, _ := archivePinFor(t, buffer.Bytes(), "tar.gz")
	if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil || !strings.Contains(err.Error(), "unsupported global archive attribute") {
		t.Fatal("global path override accepted", err)
	}
}

func TestArchiveCaseDistinctFilesFollowDestinationFilesystem(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "case-probe"), []byte("probe"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(parent, "CASE-PROBE"))
	sensitive := errors.Is(err, os.ErrNotExist)
	if err != nil && !sensitive {
		t.Fatal(err)
	}
	entries := []archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}, {Name: "terminfo/2621A", Text: "uppercase", Mode: 0644}, {Name: "terminfo/2621a", Text: "lowercase", Mode: 0644}}
	pin, client, _ := archivePinFor(t, archiveTar(t, entries), "tar.gz")
	destination := filepath.Join(parent, "payload")
	err = downloadArchive(context.Background(), client, pin, destination)
	if !sensitive {
		if err == nil {
			t.Fatal("accepted colliding names on a case-insensitive filesystem")
		}
		return
	}
	if err != nil {
		t.Fatal("case-distinct native files rejected", err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(entry.Name)))
		if err != nil || string(data) != entry.Text {
			t.Fatal("archive member changed", entry.Name, string(data), err)
		}
	}
	files, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatal("extraction left probing artifacts", files)
	}
}
