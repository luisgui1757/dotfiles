//go:build darwin || linux

package installer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Writes the generated POSIX profiles into a fresh home. With the handoff
// selected, the observed zsh is replaced by a fixture executable in that home
// (its path contains a space, proving the quoting), so the bash side is proved
// without depending on the system zsh.
func handoffShellHome(t *testing.T, selected []string, fixture string) (home, zsh string) {
	t.Helper()
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	zsh = filepath.Join(home, "fixture zsh")
	writeConfigFixture(t, zsh, fixture)
	if err := os.Chmod(zsh, 0700); err != nil {
		t.Fatal(err)
	}
	platform := nativePlatform(t)
	pins, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	d := &ProfileDriver{Directory: filepath.Join(home, "state"), Targets: map[string][]ProfileTarget{}}
	archives := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}
	if err := configureShellProfiles(d, platform.Context, ConfigFolders{Home: home, Zsh: home}, archives, filepath.Join(home, "starship.toml"), selected); err != nil {
		t.Fatal(err)
	}
	observed := shellLiteral("/usr/bin/zsh")
	if platform.OS == "darwin" {
		observed = shellLiteral("/bin/zsh")
	}
	handoff := slices.Contains(selected, "integration.zsh-handoff")
	found := false
	for _, entry := range d.Targets["integration.shells"] {
		script := entry.Script
		if filepath.Base(entry.Path) == "init.sh" {
			found = true
			if handoff != strings.Contains(script, "DOTFILES_ZSH_HANDOFF") || handoff != strings.Contains(script, zshHandoffGhostty) {
				t.Fatalf("handoff selection and generated initialization disagree:\n%s", script)
			}
			if handoff {
				if strings.Count(script, observed) != 4 {
					t.Fatalf("handoff must name the observed zsh exactly four times:\n%s", script)
				}
				script = strings.ReplaceAll(script, observed, shellLiteral(zsh))
			}
		}
		writeConfigFixture(t, entry.Path, script)
		if filepath.Base(entry.Path) == "init.sh" {
			if output, err := exec.Command("bash", "-n", entry.Path).CombinedOutput(); err != nil {
				t.Fatalf("shared initialization must parse in Bash: %s: %v", output, err)
			}
		}
	}
	if !found {
		t.Fatal("generated profiles have no shared initialization")
	}
	return home, zsh
}

type handoffBash struct {
	args  []string
	shell string // empty leaves SHELL unset
	env   []string
	tty   bool
	input string
}

// Runs an actual bash with the generated profiles: on a pseudo-terminal that is
// its controlling terminal, or with piped input. The typed input ends a bash that
// was not handed over; a handed-over process ignores it.
func runHandoffBash(t *testing.T, home string, run handoffBash) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, run.args...)
	cmd.Env = append([]string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "LC_ALL=C", "BASH_SILENCE_DEPRECATION_WARNING=1"}, run.env...)
	if run.shell != "" {
		cmd.Env = append(cmd.Env, "SHELL="+run.shell)
	}
	var mu sync.Mutex
	var output bytes.Buffer
	copied := make(chan struct{})
	if run.tty {
		master, slave, err := openShellTestPTY()
		if err != nil {
			t.Fatal(err)
		}
		defer master.Close()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
		startErr := cmd.Start()
		if err := errors.Join(startErr, slave.Close()); err != nil {
			t.Fatal(err)
		}
		go func() {
			defer close(copied)
			buffer := make([]byte, 4096)
			for {
				n, err := master.Read(buffer)
				mu.Lock()
				output.Write(buffer[:max(n, 0)])
				mu.Unlock()
				if err != nil {
					return
				}
			}
		}()
		if _, err := io.WriteString(master, run.input); err != nil {
			t.Fatal(err)
		}
	} else {
		close(copied)
		cmd.Stdin = strings.NewReader(run.input)
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	waitErr := cmd.Wait()
	select {
	case <-copied:
	case <-time.After(5 * time.Second):
	}
	if waitErr != nil || ctx.Err() != nil {
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("bash %v: %v %v\n%s", run.args, waitErr, ctx.Err(), output.String())
	}
}

func readHandoffRecord(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Owner report 2026-10-23: a company directory account keeps bash as its login
// shell, which chsh cannot change, so Ghostty and Herdr (which starts $SHELL)
// opened bash. The selected handoff makes interactive terminal bash become zsh
// with SHELL set; every other bash stays bash.
func TestZshHandoffFromInteractiveTerminalBash(t *testing.T) {
	selected := []string{"config.zsh", "integration.zsh-handoff"}
	const recorder = `#!/bin/sh
printf '%s\n' "args=$*" "shell=$SHELL" "marker=${DOTFILES_ZSH_HANDOFF-}" > "$HOME/handoff"
`
	const exit = "exit\n"
	for _, test := range []struct {
		name     string
		selected []string
		run      handoffBash
		args     string // the expected zsh arguments; "-" means no handoff
	}{
		{"terminal bash hands over", selected, handoffBash{args: []string{"-i"}, shell: "/bin/bash", tty: true, input: exit}, ""},
		{"login bash keeps login semantics", selected, handoffBash{args: []string{"-l", "-i"}, shell: "/bin/bash", tty: true, input: exit}, "-l"},
		{"zsh account starting bash stays", selected, handoffBash{args: []string{"-i"}, shell: "/opt/local/bin/zsh", tty: true, input: exit}, "-"},
		{"bash inside handed-over zsh stays", selected, handoffBash{args: []string{"-i"}, shell: "/bin/bash", env: []string{"DOTFILES_ZSH_HANDOFF=2"}, tty: true, input: exit}, "-"},
		{"bash -c stays", selected, handoffBash{args: []string{"-i", "-c", "true"}, shell: "/bin/bash", tty: true}, "-"},
		{"input without a terminal stays", selected, handoffBash{args: []string{"-i"}, shell: "/bin/bash", input: exit}, "-"},
		{"unselected stays", []string{"config.zsh"}, handoffBash{args: []string{"-i"}, shell: "/bin/bash", tty: true, input: exit}, "-"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home, zsh := handoffShellHome(t, test.selected, recorder)
			runHandoffBash(t, home, test.run)
			want := ""
			if test.args != "-" {
				want = "args=" + test.args + "\nshell=" + zsh + "\nmarker=1\n"
			}
			if got := readHandoffRecord(t, filepath.Join(home, "handoff")); got != want {
				t.Fatalf("handoff record %q, want %q", got, want)
			}
		})
	}
}

// A zsh that cannot be executed must not strand the terminal: interactive bash
// keeps running with its original SHELL and no handoff marker.
func TestZshHandoffFailureKeepsBash(t *testing.T) {
	home, _ := handoffShellHome(t, []string{"config.zsh", "integration.zsh-handoff"}, "\x7fELF\x02\x01\x01\x00not an executable\x00")
	after := filepath.Join(home, "after")
	runHandoffBash(t, home, handoffBash{args: []string{"-i"}, shell: "/bin/bash", tty: true,
		input: "printf '%s|%s|' \"${SHELL-unset}\" \"${DOTFILES_ZSH_HANDOFF-unset}\" > " + shellLiteral(after) + "; shopt -q execfail && echo on >> " + shellLiteral(after) + " || echo off >> " + shellLiteral(after) + "; exit\n"})
	// The caller's execfail setting (off by default) is restored as well.
	if got := readHandoffRecord(t, after); got != "/bin/bash|unset|off\n" {
		t.Fatalf("failed handoff left SHELL, marker and execfail %q", got)
	}
}

// Ghostty injects its integration only into the shell it starts. The zsh that
// received the handoff loads Ghostty's documented zsh integration once; nested
// shells keep the marker and do not.
func TestZshHandoffLoadsGhosttyIntegrationOnce(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal("zsh shell tests require zsh to be provisioned:", err)
	}
	for _, test := range []struct {
		name, marker, want string
		ghostty, loaded    bool
	}{
		{"handed-over zsh in Ghostty", "1", "2", true, true},
		{"handed-over zsh elsewhere", "1", "2", false, false},
		{"nested zsh", "2", "2", true, false},
		{"zsh login shell", "", "", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			resources := filepath.Join(home, "ghostty resources")
			writeConfigFixture(t, filepath.Join(resources, "shell-integration", "zsh", "ghostty-integration"), "print -r -- loaded >> \"$HOME/ghostty\"\n")
			cmd := exec.Command(zsh, "-f", "-c", zshHandoffGhostty+`print -rn -- "${DOTFILES_ZSH_HANDOFF-}"`)
			cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
			if test.marker != "" {
				cmd.Env = append(cmd.Env, "DOTFILES_ZSH_HANDOFF="+test.marker)
			}
			if test.ghostty {
				cmd.Env = append(cmd.Env, "GHOSTTY_RESOURCES_DIR="+resources)
			}
			output, err := cmd.CombinedOutput()
			loaded := readHandoffRecord(t, filepath.Join(home, "ghostty"))
			if err != nil || string(output) != test.want || (loaded == "loaded\n") != test.loaded || (loaded != "" && loaded != "loaded\n") {
				t.Fatalf("marker %q, Ghostty integration %q: %v", output, loaded, err)
			}
		})
	}
}
