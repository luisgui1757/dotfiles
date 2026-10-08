package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			cmd := exec.Command("bash", "--noprofile", "--norc", "-ic", ". "+shellLiteral(init)+"; printf '%s' \"$LS_COLORS\"")
			for _, variable := range os.Environ() {
				key, _, _ := strings.Cut(variable, "=")
				if key != "HOME" && key != "LS_COLORS" && key != "DOTFILES_LS_COLORS" && key != "__DOTFILES_SHELL_READY" && key != "BASH_ENV" && key != "ENV" {
					cmd.Env = append(cmd.Env, variable)
				}
			}
			cmd.Env = append(cmd.Env, "HOME="+home, "LS_COLORS=di=personal", "DOTFILES_LS_COLORS="+fixture.override)
			output, err := cmd.Output()
			if err != nil || !strings.Contains(string(output), fixture.want) {
				t.Fatalf("active selected palette: %q; want %q: %v", output, fixture.want, err)
			}
			if fixture.override != "" || fixture.selected == nil {
				if string(output) != fixture.want {
					t.Fatalf("explicit or unselected palette changed: %q", output)
				}
			}
		})
	}
}
