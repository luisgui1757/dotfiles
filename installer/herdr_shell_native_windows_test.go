package installer

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeArchiveHerdrOpensManagedPowerShellPane(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("enable real upstream native lifecycle on a disposable Windows account")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	platform := nativePlatform(t)
	platform.WindowsPowerShell, err = DiscoverWindowsPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewNativeController(repository, "herdr-pane", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"herdr"}})
	pwsh, err := d.Archives.CommandPath("tool.powershell", "pwsh")
	if err != nil {
		t.Fatal(err)
	}
	herdr, err := d.Archives.CommandPath("tool.herdr", "herdr")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := d.Configurations.Manifest.Resolve(c.Catalog, platform.Context, folders, repository, "config.herdr")
	if err != nil || len(paths) != 1 {
		t.Fatal("resolve published Herdr configuration", paths, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	session := "dotfiles-test-" + strings.ToLower(rand.Text())
	environment := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "APPDATA="+folders.AppData, "LOCALAPPDATA="+folders.LocalAppData, "HERDR_CONFIG_PATH="+paths[0].Destination)
	// The real published profile supplies process PATH. Do not put the private
	// runtime into the registry: that would hide the original pane-launch bug.
	profile := filepath.Join(folders.Documents, "PowerShell", "profile.ps1")
	script := ". " + powershellLiteral(profile) + "; & " + powershellLiteral(herdr) + " --session " + powershellLiteral(session) + " server; exit $LASTEXITCODE"
	server := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-Command", script)
	server.Dir, server.Env = home, environment
	process, err := startAgentCLI(ctx, server)
	if err != nil {
		t.Fatal(err)
	}
	ready, done := make(chan struct{}, 1), make(chan error, 2)
	stderrClosed := make(chan struct{})
	go func() { _, err := io.Copy(io.Discard, process.output); done <- err }()
	go func() {
		defer close(stderrClosed)
		scanner := bufio.NewScanner(process.diagnostic)
		remaining := 4096
		for scanner.Scan() {
			if remaining > 0 {
				line := scanner.Text()
				if len(line) > remaining {
					line = line[:remaining]
				}
				t.Logf("Herdr stderr: %s", line)
				remaining -= len(line) + 1
			}
			if strings.HasPrefix(scanner.Text(), "herdr server running;") {
				ready <- struct{}{}
			}
		}
		done <- scanner.Err()
	}()
	stopped := false
	defer func() {
		if err := errors.Join(process.input.Close(), process.wait(!stopped), agentCLIDrain(process, done, 2)); err != nil {
			t.Error("Herdr owned process cleanup", err)
		}
	}()
	select {
	case <-ready:
	case <-stderrClosed:
		t.Fatal("Herdr server closed stderr before readiness; see bounded diagnostics")
	case <-ctx.Done():
		t.Fatal("Herdr server readiness", ctx.Err())
	}
	run := func(arguments ...string) []byte {
		t.Helper()
		command := exec.CommandContext(ctx, herdr, append([]string{"--session", session}, arguments...)...)
		command.Dir, command.Env = home, environment
		output, err := agentCLIOutput(ctx, command)
		if err != nil {
			t.Fatalf("Herdr %v: %v\n%s", arguments, err, output)
		}
		return output
	}
	var created struct {
		Result struct {
			RootPane struct {
				ID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	output := run("workspace", "create", "--cwd", home)
	if err := json.Unmarshal(output, &created); err != nil || created.Result.RootPane.ID == "" {
		t.Fatalf("Herdr did not create a real pane: %s %v", output, err)
	}
	witness := filepath.Join(home, "pane-shell.json")
	run("pane", "run", created.Result.RootPane.ID, "[IO.File]::WriteAllText("+powershellLiteral(witness+".tmp")+", (@{Version=$PSVersionTable.PSVersion.ToString();Executable=(Get-Process -Id $PID).Path} | ConvertTo-Json)); [IO.File]::Move("+powershellLiteral(witness+".tmp")+", "+powershellLiteral(witness)+")")
	var data []byte
	for {
		data, err = os.ReadFile(witness)
		if !errors.Is(err, os.ErrNotExist) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("pane did not execute the shell witness", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	var shell struct{ Version, Executable string }
	if err != nil || json.Unmarshal(data, &shell) != nil || shell.Version != d.Archives.Pins["tool.powershell"].Version || !strings.EqualFold(shell.Executable, pwsh) {
		t.Fatalf("pane did not use managed PowerShell: %s %v", data, err)
	}
	run("server", "stop")
	stopped = true
	t.Logf("actual Herdr pane used PowerShell %s at %s", shell.Version, shell.Executable)
}
