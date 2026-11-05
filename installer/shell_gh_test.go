//go:build darwin || linux

package installer

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestManagedGhNamesItsStableCommandPath(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal("zsh shell tests require zsh to be provisioned:", err)
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		selected []string
	}{
		{"selected", []string{"config.zsh", "tool.gh"}},
		{"unselected keeps the personal value", []string{"config.zsh"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			env, _ := editorShellEnvironment(t, home, test.selected)
			want := "personal"
			if slices.Contains(test.selected, "tool.gh") {
				gh, err := (&ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}).CommandPath("tool.gh", "gh")
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(gh, string(filepath.Separator)+"current"+string(filepath.Separator)) {
					t.Fatal("managed gh path is not the stable current link", gh)
				}
				want = gh
			}
			script := ". " + shellLiteral(env) + `; printf '%s' "${GH_PATH-unset}"`
			for _, shell := range [][]string{{"bash", "--noprofile", "--norc", "-c"}, {zsh, "-f", "-c"}} {
				cmd := exec.Command(shell[0], append(shell[1:], script)...)
				cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "GH_PATH=personal"}
				output, err := cmd.CombinedOutput()
				if err != nil || string(output) != want {
					t.Fatalf("%s: GH_PATH %q, want %q: %v", shell[0], output, want, err)
				}
			}
		})
	}
}
