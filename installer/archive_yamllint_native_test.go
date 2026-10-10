package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// All downloads, venvs, commands and cleanup stay in the private test home.
// This opt-in never invokes a native package manager or system/user pip.
func TestNativeYamllintPreparationLifecycleAndStablePython(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LANGUAGE_PREPARATION") != "1" {
		t.Skip("set DOTFILES_TEST_LANGUAGE_PREPARATION=1 for private pinned Python tools")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	latest, ok := pins["tool.yamllint"]
	if !ok {
		t.Fatal("yamllint pin is missing")
	}
	c, d, _ := archiveController(t)
	home := c.Home
	pythonPin := pins["tool.python"]
	pythonPayload := filepath.Join(home, "python-v1")
	if err := downloadArchive(ctx, nil, pythonPin, pythonPayload); err != nil {
		t.Fatal(err)
	}
	if err := preparePythonArchive(ctx, pythonPin, pythonPayload); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(home, "python-current")
	if err := createDirectoryLink(pythonPayload, stable); err != nil {
		t.Fatal(err)
	}
	pythonName := "python3"
	if runtime.GOOS == "windows" {
		pythonName = "python"
	}
	d.Python = filepath.Join(stable, filepath.FromSlash(pythonPin.Commands[pythonName]))
	personalPip := filepath.Join(home, "personal-pip.conf")
	personalBytes := []byte("[global]\nindex-url = https://invalid.invalid/should-never-be-contacted\n")
	if err := os.WriteFile(personalPip, personalBytes, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PIP_CONFIG_FILE", personalPip)
	worker, err := workerFixture(filepath.Join(home, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	d.Run, d.Client = worker.run, nil
	old := latest
	old.Version, old.File = "1.37.1", "yamllint-1.37.1-py3-none-any.whl"
	old.URL = "https://files.pythonhosted.org/packages/dd/b9/be7a4cfdf47e03785f657f94daea8123e838d817be76c684298305bd789f/yamllint-1.37.1-py3-none-any.whl"
	old.SHA256 = "364f0d79e81409f591e323725e6a9f4504c8699ddf2d7263d8d2b539cd66a583"
	d.Pins["tool.shared"] = old
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	command, err := d.CommandPath("tool.shared", "yamllint")
	if err != nil {
		t.Fatal(err)
	}
	if got := nativePreparedCommand(t, home, command, "", "--version"); strings.TrimSpace(got) != "yamllint 1.37.1" {
		t.Fatal(got)
	}
	oldPayload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	d.Pins["tool.shared"] = latest
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	if got := nativePreparedCommand(t, home, command, "", "--version"); strings.TrimSpace(got) != "yamllint "+latest.Version {
		t.Fatal(got)
	}
	if _, err := os.Lstat(oldPayload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("obsolete unmodified venv survived update", err)
	}
	good, bad := filepath.Join(home, "valid YAML 'quoted'.yml"), filepath.Join(home, "duplicate.yml")
	if err := os.WriteFile(good, []byte("key: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("key: one\nkey: two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	nativePreparedCommand(t, home, command, "", "-d", "{extends: relaxed}", good)
	invalid := exec.CommandContext(ctx, command, "-d", "{extends: relaxed}", bad)
	invalid.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "PYTHONPATH=", "PYTHONHOME=")
	if output, err := invalid.CombinedOutput(); err == nil || !strings.Contains(string(output), "key-duplicates") {
		t.Fatal("native lint did not reject duplicate YAML keys", err, string(output))
	}
	if check, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "ready" {
		t.Fatal("runtime use changed owned bytes", check, err)
	}
	if err := os.Remove(d.currentLink("tool.shared")); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
	nativePreparedCommand(t, home, command, "", "-d", "{extends: relaxed}", good)
	moved := filepath.Join(home, "python-v2")
	if err := os.Rename(pythonPayload, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stable); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(moved, stable); err != nil {
		t.Fatal(err)
	}
	nativePreparedCommand(t, home, command, "", "-d", "{extends: relaxed}", good)
	if check, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "ready" {
		t.Fatal("stable Python switch broke yamllint", check, err)
	}
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned yamllint survived removal", err)
	}
	if data, err := os.ReadFile(personalPip); err != nil || string(data) != string(personalBytes) {
		t.Fatal("private preparation changed personal pip config", err)
	}
	t.Log("private yamllint installed, updated 1.37.1 to current pin, linted good/bad YAML, repaired entrypoint, survived stable Python switch, checked and removed")
}
