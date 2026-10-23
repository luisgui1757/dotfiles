package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGeneratedShellKeepsPrivatePythonEnvironmentsOffCommandPath(t *testing.T) {
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	c, err := NewNativeController(repository, "python-command-path", filepath.Join(home, "state"), nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	python, suffix, privateBin := "python3", "", "bin"
	if runtime.GOOS == "windows" {
		python, suffix, privateBin = "python", ".exe", "Scripts"
	}
	writeCommand := func(path string) {
		t.Helper()
		writeConfigFixture(t, path, "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	expected := map[string]string{}
	for command, id := range map[string]string{python: "tool.python", "latex2text": "tool.latex2text", "yamllint": "tool.yamllint"} {
		path, err := d.Archives.CommandPath(id, command)
		if err != nil {
			t.Fatal(err)
		}
		expected[command] = path
		writeCommand(path)
	}
	var privateDirectories []string
	for _, id := range []string{"tool.latex2text", "tool.yamllint"} {
		directory := filepath.Join(d.Archives.currentLink(id), privateBin)
		privateDirectories = append(privateDirectories, directory)
		for _, command := range []string{python, "pip"} {
			writeCommand(filepath.Join(directory, command+suffix))
		}
	}
	// Publish the actual generated profile through its normal owner. Command
	// files are filesystem-boundary fixtures; lookup runs in the real shell.
	c.Catalog = &Catalog{Schema: 1, Resources: []Resource{
		{ID: "fixture", Name: "Fixture", Capability: true, Requires: []string{"integration.shells"}},
		{ID: "integration.shells", Name: "Shell", Action: "profile", Scope: "user"},
	}}
	c.Driver = d.Profiles
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"fixture"}})
	shells := []string{"bash", "zsh"}
	if runtime.GOOS == "windows" {
		shells = []string{"powershell.exe"}
	}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			program, err := exec.LookPath(shell)
			if err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(home, ".bash_profile")
			if shell == "zsh" {
				profile = filepath.Join(home, ".zprofile")
			}
			script := ". " + shellLiteral(profile) + "; . " + shellLiteral(profile) + "; "
			for _, command := range []string{python, "latex2text", "yamllint"} {
				script += "command -v " + command + "; "
			}
			script += "printf '%s\\n' \"$PATH\""
			args := []string{"-c", script}
			path := "/usr/bin:/bin"
			if runtime.GOOS == "windows" {
				profile = filepath.Join(folders.Documents, "WindowsPowerShell", "profile.ps1")
				script = "$ErrorActionPreference = 'Stop'; . " + powershellLiteral(profile) + "; . " + powershellLiteral(profile) + "; "
				for _, command := range []string{python, "latex2text", "yamllint"} {
					script += "(Get-Command " + command + " -CommandType Application | Select-Object -First 1).Source; "
				}
				script += "$env:PATH"
				args = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
				path = filepath.Join(os.Getenv("SystemRoot"), "System32")
			}
			cmd := exec.CommandContext(context.Background(), program, args...)
			cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "PATH="+path)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(string(output), err)
			}
			lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
			if len(lines) != 4 {
				t.Fatal("unexpected shell output", string(output))
			}
			for i, command := range []string{python, "latex2text", "yamllint"} {
				if lines[i] != expected[command] {
					t.Errorf("%s resolves to %s, want %s", command, lines[i], expected[command])
				}
			}
			for _, directory := range privateDirectories {
				for _, entry := range filepath.SplitList(lines[3]) {
					if entry == directory {
						t.Errorf("private Python and pip directory is exposed: %s", directory)
					}
				}
			}
		})
	}
}
