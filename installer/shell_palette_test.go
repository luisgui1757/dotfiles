package installer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSelectedPOSIXPaletteWorksWithoutZsh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("actual POSIX shell boundary")
	}
	for _, fixture := range []struct {
		name     string
		selected []string
		override string
		want     string
	}{
		{"lsd-default", []string{"integration.lsd"}, "", "di=38;2;246;193;119"},
		{"zsh-default-in-bash", []string{"config.zsh"}, "", "di=38;2;246;193;119"},
		{"shared-default", []string{"config.zsh", "integration.lsd"}, "", "di=38;2;246;193;119"},
		{"explicit-override", []string{"integration.lsd"}, "di=custom:*.txt=custom", "di=custom:*.txt=custom"},
		{"unselected-preserves-personal", nil, "", "di=personal"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			platform := nativePlatform(t)
			pins, err := DefaultArchivePins(platform)
			if err != nil {
				t.Fatal(err)
			}
			d := &ProfileDriver{Directory: filepath.Join(home, "state"), Targets: map[string][]ProfileTarget{}}
			archives := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}
			if err := configureShellProfiles(d, platform.Context, ConfigFolders{Home: home}, archives, filepath.Join(home, "starship.toml"), fixture.selected); err != nil {
				t.Fatal(err)
			}
			init := filepath.Join(d.Directory, "shells", "init.sh")
			for _, entry := range d.Targets["integration.shells"] {
				if entry.Path == init {
					writeConfigFixture(t, init, entry.Script)
				}
			}
			if output, err := exec.Command("bash", "-n", init).CombinedOutput(); err != nil {
				t.Fatalf("shared initialization must parse in Bash: %s: %v", output, err)
			}
			bin := filepath.Join(home, "bin")
			writeConfigFixture(t, filepath.Join(bin, "lsd"), "#!/bin/sh\nexit 0\n")
			if err := os.Chmod(filepath.Join(bin, "lsd"), 0700); err != nil {
				t.Fatal(err)
			}
			stderrPath := filepath.Join(home, "initialization.stderr")
			script := "set -e; . " + shellLiteral(init) + " 2>" + shellLiteral(stderrPath) + "; test \"$__DOTFILES_SHELL_READY\" = 1; "
			if slices.Contains(fixture.selected, "integration.lsd") {
				script += "test \"$(alias ls)\" = \"alias ls='lsd'\"; "
			}
			script += "printf '%s' \"$LS_COLORS\""
			// Isolate initialization stderr from Bash's own non-TTY startup notice.
			// The profile still runs in an actual interactive Bash process.
			cmd := exec.Command("bash", "--noprofile", "--norc", "+m", "-ic", script)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			for _, variable := range os.Environ() {
				key, _, _ := strings.Cut(variable, "=")
				if key != "HOME" && key != "LS_COLORS" && key != "DOTFILES_LS_COLORS" && key != "__DOTFILES_SHELL_READY" && key != "BASH_ENV" && key != "ENV" {
					cmd.Env = append(cmd.Env, variable)
				}
			}
			cmd.Env = append(cmd.Env, "HOME="+home, "LS_COLORS=di=personal", "DOTFILES_LS_COLORS="+fixture.override, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := cmd.Output()
			initStderr, readErr := os.ReadFile(stderrPath)
			if err != nil || readErr != nil || len(initStderr) != 0 || !strings.Contains(string(output), fixture.want) {
				t.Fatalf("active selected palette: %q; initialization stderr %q; process stderr %q; want %q: %v (stderr read: %v)", output, initStderr, stderr.String(), fixture.want, err, readErr)
			}
			if fixture.override != "" || fixture.selected == nil {
				if string(output) != fixture.want {
					t.Fatalf("explicit or unselected palette changed: %q", output)
				}
			}
		})
	}
}
