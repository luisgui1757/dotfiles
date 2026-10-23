package terminal

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublicMachineExitChild(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_MACHINE_EXIT_CHILD") != "1" {
		return
	}
	fmt.Print(`{"status":"needs-action","message":"tool.rg: missing entrypoint"}`)
	os.Exit(1)
}

func TestPublicMachineDecodesExpectedUnhealthyExit(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPublicMachineExitChild$")
	cmd.Env = append(os.Environ(), "DOTFILES_TEST_MACHINE_EXIT_CHILD=1")
	data, exit := cmd.Output()
	if result, err := decodePublicMachine(data, exit); err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, "tool.rg") {
		t.Fatal("expected unhealthy Check result was discarded", result, err)
	}
	for _, scenario := range []struct {
		name, data string
		exit       error
		valid      bool
	}{
		{"ready", `{"status":"ready"}`, nil, true},
		{"preview", `{"status":"preview"}`, nil, true},
		{"failed", `{"status":"failed"}`, exit, true},
		{"false success", string(data), nil, false},
		{"unexpected exit", `{"status":"ready"}`, exit, false},
		{"transport", string(data), os.ErrPermission, false},
		{"malformed", `not JSON`, exit, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := decodePublicMachine([]byte(scenario.data), scenario.exit); (err == nil) != scenario.valid {
				t.Fatal("exit/status protocol mismatch", err)
			}
		})
	}
}

type irregularLinkInfo struct{ os.FileInfo }

func (i irregularLinkInfo) Mode() os.FileMode { return os.ModeIrregular | i.FileInfo.Mode().Perm() }

func TestPublicArchiveRecordsWindowsJunctionMetadata(t *testing.T) {
	file := filepath.Join(t.TempDir(), "metadata")
	if err := os.WriteFile(file, []byte("metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(file)
	if err != nil {
		t.Fatal(err)
	}
	// Windows' filesystem boundary reports junctions as ModeIrregular. Exercise
	// that portable archive encoding even when this test runs on POSIX.
	header, err := publicArchiveHeader(irregularLinkInfo{info}, "outside-target")
	if err != nil || header.Typeflag != tar.TypeSymlink || header.Linkname != "outside-target" || header.PAXRecords["DOTFILES.link-kind"] != "junction" {
		t.Fatal("junction metadata cannot be archived without following it", header, err)
	}
}

func TestPublicEvidenceDoesNotFollowNativeDirectoryLinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	var err error
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	outside, err = filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "never-follow.txt"), []byte("outside data"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "a-current")
	if runtime.GOOS == "windows" {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; Import-Module ([IO.Path]::Combine($PSHOME,'Modules','Microsoft.PowerShell.Management','Microsoft.PowerShell.Management.psd1')); New-Item -ItemType Junction -Path "+quote(link)+" -Target "+quote(outside)+" | Out-Null")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatal("create real native junction", err, string(output))
		}
	} else if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "z-state.json"), []byte("original journal"), 0600); err != nil {
		t.Fatal(err)
	}
	entries := publicTree(t, root)
	if len(entries) != 3 || entries["a-current"].Content != outside {
		t.Fatal("snapshot followed or lost the native link", entries)
	}
	archive := filepath.Join(t.TempDir(), "state.tar.gz")
	archivePublicJourney(t, root, archive)
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := compressed.Close(); err != nil {
			t.Error(err)
		}
	}()
	reader := tar.NewReader(compressed)
	names := []string{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
		if header.Name == "a-current" && (header.Typeflag != tar.TypeSymlink || header.Linkname != outside) {
			t.Fatal("archive followed or lost native link", header)
		}
		if header.Name == "z-state.json" {
			data, err := io.ReadAll(reader)
			if err != nil || string(data) != "original journal" {
				t.Fatal("archive lost journal after the link", err)
			}
		}
	}
	if strings.Join(names, ",") != ".,a-current,z-state.json" {
		t.Fatal("archive omitted state or followed an outside target", names)
	}
}
