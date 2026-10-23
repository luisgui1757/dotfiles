package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstalledLegacySystemTextIsBoundedAndRejectsLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX root-private vendor configuration")
	}
	root := t.TempDir()
	installed := newLegacyInstalled(t, root)
	path := filepath.Join(root, "synthetic.conf")
	writeConfigFixture(t, path, "personal\nnix\n")
	output, err := exec.Command(installed.binary, "internal-legacy-system-text", path).Output()
	var file legacySystemText
	if err != nil || json.Unmarshal(output, &file) != nil || file.validate() != nil || string(file.Data) != "personal\nnix\n" {
		t.Fatal("private read did not return consistent data", err)
	}
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{link, root, "relative", ""} {
		if output, err := exec.Command(installed.binary, "internal-legacy-system-text", target).Output(); err == nil || len(output) != 0 {
			t.Fatal("invalid private read accepted", target, err)
		}
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxProfileBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(installed.binary, "internal-legacy-system-text", path).Output(); err == nil || len(output) != 0 {
		t.Fatal("oversized private read accepted", err)
	}
}

func TestLegacyPrivateSystemTextKeepsSnapshotAndPermissionGuards(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the fixture's file permissions")
	}
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "synthetic.conf")
	data := []byte("personal\nnix\n")
	writeConfigFixture(t, path, string(data))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0600); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash(data), Mode: 0}})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := json.Marshal(legacySystemText{Data: data, Snapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	d.Environment.Root = false
	d.Environment.Executable, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		calls++
		argv := legacyFakeArgv(command)
		if command.Program != "/usr/bin/sudo" || len(argv) != 3 || argv[1] != "internal-legacy-system-text" || argv[2] != path {
			t.Fatal("unexpected private inspection", command.Program)
		}
		return reply, nil
	}
	step := legacyRetirementStep{Before: []legacyRetirementFile{{Path: path, Snapshot: snapshot}}}
	if err := d.preconditions(context.Background(), step); err == nil || calls != 0 {
		t.Fatal("unprivileged operation acquired administrator inspection", err, calls)
	}
	step.Privileged = true
	if err := d.preconditions(context.Background(), step); err != nil || calls != 1 {
		t.Fatal("private snapshot could not be verified", err, calls)
	}
	if problem := d.determinateInsertProblem(context.Background(), path, "nix\n", ""); problem != "" {
		t.Fatal("private vendor block could not be inspected", problem)
	}
	step.Before[0].Snapshot.Hash = "changed"
	if err := d.preconditions(context.Background(), step); err == nil {
		t.Fatal("a changed private file passed approval")
	}
	reply = []byte(`{"data":"cGVyc29uYWwK","mode":0,"snapshot":{"kind":"absent"}}`)
	if _, _, err := d.readSystemText(context.Background(), path); err == nil {
		t.Fatal("inconsistent privileged reply was accepted")
	}
	d.Query = func(context.Context, nativeCommand) ([]byte, error) {
		return nil, errors.New("credentials unavailable")
	}
	if _, _, err := d.readSystemText(context.Background(), path); err == nil {
		t.Fatal("missing credentials became an absent file")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0 {
		t.Fatal("inspection weakened file permissions", err)
	}
}

func TestLegacyDarwinFinalCleanupReadsPrivateSystemFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the fixture's file permissions")
	}
	d, root := legacyStepFixture(t)
	d.systemRoot = root
	d.Platform.OS = "darwin"
	d.Environment.Root = false
	var err error
	d.Environment.Executable, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		d.sys("/etc/fstab"):          "# personal mount configuration\n",
		d.sys("/etc/synthetic.conf"): "personal\nnix\n",
	}
	replies := map[string][]byte{}
	for path, content := range files {
		writeConfigFixture(t, path, content)
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(path, 0600); err != nil {
				t.Error(err)
			}
		})
		snapshot, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash([]byte(content)), Mode: 0}})
		if err != nil {
			t.Fatal(err)
		}
		replies[path], err = json.Marshal(legacySystemText{Data: []byte(content), Snapshot: snapshot})
		if err != nil {
			t.Fatal(err)
		}
	}
	d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
		argv := legacyFakeArgv(command)
		if len(argv) == 3 && argv[1] == "internal-legacy-system-text" {
			if reply, ok := replies[argv[2]]; ok {
				return reply, nil
			}
		}
		if filepath.Base(argv[0]) == "dscl" {
			return nil, nil // No remaining build accounts after the vendor uninstall.
		}
		t.Fatal("unexpected final inspection", argv)
		return nil, errors.New("unexpected final inspection")
	}
	var scope legacyRetirementScope
	if err := d.planDarwinNixIntegrations(context.Background(), &scope, false); err != nil || scope.Blocked != "" {
		t.Fatal("private post-uninstall files blocked final cleanup", scope.Blocked, err)
	}
	if len(scope.Steps) != 1 || scope.Steps[0].Path != d.sys("/etc/synthetic.conf") || string(scope.Steps[0].Content) != "personal\n" || scope.Steps[0].Mode != 0 {
		t.Fatal("final cleanup did not preserve private personal bytes and mode", scope.Steps)
	}
}
