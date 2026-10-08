package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type windowsTerminalOffline struct{}

func (windowsTerminalOffline) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("Windows Terminal checks/removal must work offline")
}

// Terminal obtains LocalAppData through SHGetKnownFolderPath, not its process
// environment. Use a genuinely disposable hosted user and refuse an existing
// unpackaged settings directory; never borrow a developer's personal profile.
func TestNativeWindowsTerminalConsumesScopedSettingsAndExits(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_WINDOWS_TERMINAL") != "1" {
		t.Skip("requires disposable GitHub Windows runner and DOTFILES_TEST_WINDOWS_TERMINAL=1")
	}
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	settingsDirectory := filepath.Join(folders.LocalAppData, "Microsoft", "Windows Terminal")
	if _, err := os.Lstat(settingsDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native Terminal fixture requires an absent unpackaged settings directory; preserve the existing directory", err)
	}
	if err := os.MkdirAll(settingsDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(settingsDirectory); err != nil {
			t.Error(err)
		}
	})
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	pwshPin, ok := pins["tool.powershell"]
	if !ok {
		t.Fatal("native fixture requires managed PowerShell")
	}
	powerShellController, powerShell, _ := archiveController(t)
	powerShell.Pins["tool.shared"], powerShell.Client = pwshPin, nil
	dispatchApproved(t, powerShellController, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	pwsh, err := powerShell.CommandPath("tool.shared", "pwsh")
	if err != nil {
		t.Fatal(err)
	}
	c, archive, _ := archiveController(t)
	archive.Pins["tool.shared"], archive.Client = windowsTerminalPin(t), nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	terminal, err := archive.RequiredFilePath("tool.shared", "WindowsTerminal.exe")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := archive.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	profiles, driver, _ := windowsTerminalController(t)
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if err := configureWindowsTerminal(driver, Context{OS: "windows"}, folders, repository, pwsh); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, profiles, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	path := driver.Targets[windowsTerminalResource][0].Path
	beforeNative, _, err := readProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	windowsTerminalNativeWindow(t, terminal, pwsh, c.Home, path)
	check, err := profiles.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		afterNative, _, readErr := readProfile(path)
		if readErr != nil {
			t.Log("cannot read native settings for owned diff", readErr)
		} else if diff, diffErr := windowsTerminalManagedDiff(beforeNative, afterNative); diffErr != nil {
			t.Log("cannot extract native owned settings diff", diffErr)
		} else {
			t.Log("native settings owned differences:", strings.Join(diff, "\n"))
		}
		t.Fatal("Terminal's own settings serialization changed managed behavior", check, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]string{"profiles/defaults/font/weight": `"bold"`, "defaultProfile": `"{later-personal}"`} {
		data, err = windowsTerminalSet(data, field, []byte(value))
		if err != nil {
			t.Fatal(err)
		}
	}
	collection := windowsTerminalCollections()[0]
	array, _, err := windowsTerminalGet(data, collection.Path)
	if err != nil {
		t.Fatal(err)
	}
	entry, index, err := windowsTerminalEntry(array, collection)
	if err != nil {
		t.Fatal(err)
	}
	entry, err = windowsTerminalSet(entry, "icon", []byte(`"later-personal.ico"`))
	if err != nil {
		t.Fatal(err)
	}
	array, err = windowsTerminalReplaceElement(array, index, entry)
	if err != nil {
		t.Fatal(err)
	}
	data, err = windowsTerminalSet(data, collection.Path, array)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, string(data))
	dispatchApproved(t, profiles, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, profiles, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, retained := range []string{"bold", "later-personal.ico", "{later-personal}"} {
		if !bytes.Contains(data, []byte(retained)) {
			t.Fatal("removal lost later personal preference", retained, string(data))
		}
	}
	archive.Client, powerShell.Client = &http.Client{Transport: windowsTerminalOffline{}}, &http.Client{Transport: windowsTerminalOffline{}}
	check, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("runtime changed immutable Terminal payload", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	dispatchApproved(t, powerShellController, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("offline removal retained owned Terminal payload", err)
	}
}

func windowsTerminalNativeWindow(t *testing.T, terminal, pwsh, home, settings string) {
	t.Helper()
	witness, release := filepath.Join(home, "terminal-witness.json"), filepath.Join(home, "terminal-release")
	script := filepath.Join(home, "terminal-witness.ps1")
	content := `$ErrorActionPreference='Stop'
@{ profile=$env:WT_PROFILE_ID; executable=[Diagnostics.Process]::GetCurrentProcess().MainModule.FileName } | ConvertTo-Json -Compress | Set-Content -LiteralPath $env:DOTFILES_TERMINAL_WITNESS -Encoding utf8
$deadline=[DateTime]::UtcNow.AddSeconds(120)
while (-not (Test-Path -LiteralPath $env:DOTFILES_TERMINAL_RELEASE)) { if ([DateTime]::UtcNow -gt $deadline) { exit 9 }; Start-Sleep -Milliseconds 100 }
exit 0
`
	if err := os.WriteFile(script, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(terminal, "--window", "new", "new-tab", "--profile", windowsTerminalProfileGUID, "--inheritEnvironment", "--appendCommandLine", "--", "-NoLogo", "-NoProfile", "-File", script)
	command.Dir = home
	command.Env = append(os.Environ(), "DOTFILES_TERMINAL_WITNESS="+witness, "DOTFILES_TERMINAL_RELEASE="+release)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	finished := false
	t.Cleanup(func() {
		if err := os.WriteFile(release, nil, 0600); err != nil {
			t.Error(err)
		}
		if !finished {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				// The handle belongs to the exact process started above. Never
				// terminate a process by image name or any pre-existing instance.
				if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					t.Error(err)
				}
				<-done
			}
		}
	})
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			finished = true
			t.Fatal("owned Terminal exited before proving its configured profile", err)
		case <-deadline.C:
			t.Fatal("Terminal did not launch its configured managed PowerShell")
		case <-ticker.C:
			data, err := os.ReadFile(witness)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			var actual struct{ Profile, Executable string }
			if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &actual); err != nil {
				continue
			}
			resolved, err := resolveConfigPath(pwsh)
			if err != nil {
				t.Fatal("resolve configured managed PowerShell", pwsh, err)
			}
			if !strings.EqualFold(actual.Profile, windowsTerminalProfileGUID) || (!strings.EqualFold(actual.Executable, resolved) && !strings.EqualFold(actual.Executable, pwsh)) {
				t.Fatal("Terminal did not consume the configured exact managed profile", actual, resolved)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			query := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "[Diagnostics.Process]::GetProcessById([int]$env:DOTFILES_TERMINAL_PID).MainWindowHandle.ToInt64()")
			query.Env = append(os.Environ(), "DOTFILES_TERMINAL_PID="+strconv.Itoa(command.Process.Pid))
			output, err := query.Output()
			cancel()
			if err != nil {
				t.Fatal("cannot inspect owned Terminal window", err)
			}
			window, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
			if err != nil || window == 0 {
				t.Fatal("owned Terminal has no native window", string(output), err)
			}
			windowsTerminalNativeSave(t, command.Process.Pid, window, settings)
			if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				finished = true
				if err != nil {
					t.Fatal("Terminal did not exit cleanly after its own shell", err)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("owned Terminal did not exit after shell completion")
			}
			return
		}
	}
}
