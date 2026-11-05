//go:build darwin || linux

package installer

import (
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestManagedGhNamesItsStableCommandPath(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal("zsh shell tests require zsh to be provisioned:", err)
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
				want = filepath.Join(home, "packages", "tool.gh", "current", "bin", "gh")
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
