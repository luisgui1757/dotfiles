//go:build darwin || linux

package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Writes the generated POSIX profiles into home and returns the shared
// environment file and the managed nvim path that SUDO_EDITOR may name.
func editorShellEnvironment(t *testing.T, home string, selected []string) (env, nvim string) {
	t.Helper()
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
	if nvim, err = archives.CommandPath("tool.nvim", "nvim"); err != nil {
		t.Fatal(err)
	}
	for _, entry := range d.Targets["integration.shells"] {
		if filepath.Base(entry.Path) == "env.sh" {
			writeConfigFixture(t, entry.Path, entry.Script)
			return entry.Path, nvim
		}
	}
	t.Fatal("generated profiles have no shared environment")
	return "", ""
}

// Owner request 2026-10-24: selected Neovim is the editor everywhere, including
// sudoedit for root-owned files such as /etc/hosts. Personal values stay when
// Neovim is not selected.
func TestNeovimIsTheShellEditor(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal("zsh shell tests require zsh to be provisioned:", err)
	}
	for _, test := range []struct {
		name     string
		selected []string
		space    bool
		want     string // "<nvim>" stands for the managed command path
	}{
		{"selected", []string{"config.zsh", "config.nvim"}, false, "nvim|nvim|<nvim>"},
		{"unselected keeps personal values", []string{"config.zsh"}, false, "nano|nano|nano"},
		// sudo splits SUDO_EDITOR on whitespace and sudo-rs accepts no escape.
		{"whitespace path leaves SUDO_EDITOR", []string{"config.nvim"}, true, "nvim|nvim|nano"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if test.space {
				home = filepath.Join(home, "with space")
			}
			env, nvim := editorShellEnvironment(t, home, test.selected)
			want := strings.ReplaceAll(test.want, "<nvim>", nvim)
			script := ". " + shellLiteral(env) + `; printf '%s|%s|%s' "${EDITOR-unset}" "${VISUAL-unset}" "${SUDO_EDITOR-unset}"`
			for _, shell := range [][]string{{"bash", "--noprofile", "--norc", "-c"}, {zsh, "-f", "-c"}} {
				cmd := exec.Command(shell[0], append(shell[1:], script)...)
				cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "EDITOR=nano", "VISUAL=nano", "SUDO_EDITOR=nano"}
				output, err := cmd.CombinedOutput()
				if err != nil || string(output) != want {
					t.Fatalf("%s: editor environment %q, want %q: %v", shell[0], output, want, err)
				}
			}
		})
	}
}

// Real sudo (Apple's on macOS, sudo-rs on Ubuntu 26.04) must run the generated
// SUDO_EDITOR as the invoking user with their HOME and write the edit back to
// the root-owned file. Only a disposable runner with passwordless sudo opts in.
func TestNativeSudoEditorUsesGeneratedNeovim(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_SUDO_EDITOR") != "1" {
		t.Skip("set DOTFILES_TEST_SUDO_EDITOR=1 on a disposable runner with passwordless sudo")
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env, nvim := editorShellEnvironment(t, home, []string{"config.nvim"})
	record := filepath.Join(home, "editor-record")
	writeConfigFixture(t, nvim, "#!/bin/sh\nprintf '%s\\n' \"uid=$(id -u)\" \"home=$HOME\" \"args=$#\" > "+shellLiteral(record)+"\nprintf 'edited\\n' >> \"$1\"\n")
	if err := os.Chmod(nvim, 0700); err != nil {
		t.Fatal(err)
	}
	sudo := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("sudo", append([]string{"-n"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("sudo %v: %s: %v", args, output, err)
		}
		return string(output)
	}
	// Like /etc/hosts, the file sits where no path component is writable by the
	// user: sudo-rs checks every one (sudo only the parent), refusing /tmp.
	dir := strings.TrimSpace(sudo("mktemp", "-d", "/etc/dotfiles-sudoedit.XXXXXXXX"))
	if name, ok := strings.CutPrefix(dir, "/etc/dotfiles-sudoedit."); !ok || len(name) != 8 || strings.ContainsAny(name, "/ \t") {
		t.Fatalf("unexpected sudo temporary directory %q", dir)
	}
	t.Cleanup(func() { sudo("rm", "-rf", "--", dir) })
	target := dir + "/hosts"
	sudo("sh", "-c", `printf 'original\n' > "$1"`, "sh", target)
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", `. "$1"; exec sudo -n -e "$2"`, "bash", env, target)
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sudoedit with the generated environment: %s: %v", output, err)
	}
	if got := sudo("cat", target); got != "original\nedited\n" {
		t.Fatalf("root-owned file after sudoedit %q", got)
	}
	if got := sudo("find", target, "-user", "0", "-type", "f"); got != target+"\n" {
		t.Fatalf("sudoedit changed the file's owner or type: %q", got)
	}
	want := "uid=" + strconv.Itoa(os.Getuid()) + "\nhome=" + home + "\nargs=1\n"
	if got := readHandoffRecord(t, record); got != want {
		t.Fatalf("sudoedit ran the editor with %q, want %q", got, want)
	}
}
