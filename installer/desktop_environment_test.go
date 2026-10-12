package installer

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// Clearing a mode variable means removing it, not installing an empty value.
// Electron's Windows entrypoint tests getenv_s's required size, which includes
// the terminator even for an empty value. Keep the caller environment unchanged.
func desktopFixtureEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if runtime.GOOS == "windows" {
			name = strings.ToUpper(name)
		}
		switch name {
		case "BASH_ENV", "ENV", "NODE_OPTIONS", "ELECTRON_RUN_AS_NODE", "VSCODE_PORTABLE", "GHOSTTY_RESOURCES_DIR", "LD_LIBRARY_PATH":
			continue
		}
		result = append(result, item)
	}
	return result
}

func TestDesktopEnvironmentChild(t *testing.T) {
	if os.Getenv("DOTFILES_DESKTOP_ENV_CHILD") != "1" {
		return
	}
	_, present := os.LookupEnv("ELECTRON_RUN_AS_NODE")
	data, err := json.Marshal(struct {
		Present bool
		Home    string
	}{present, os.Getenv("HOME")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestDesktopFixtureRemovesModeVariablesFromRealChild(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"1", ""} {
		t.Run("mode="+value, func(t *testing.T) {
			home := t.TempDir()
			input := append(os.Environ(), "DOTFILES_DESKTOP_ENV_CHILD=1", "HOME="+home, "ELECTRON_RUN_AS_NODE="+value)
			command := exec.Command(executable, "-test.run=^TestDesktopEnvironmentChild$")
			command.Env = desktopFixtureEnvironment(input)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatal("private child failed", string(output), err)
			}
			var observed struct {
				Present bool
				Home    string
			}
			if err := Decode(output, &observed); err != nil || observed.Present || observed.Home != home {
				t.Fatal("cleared mode or private home changed", observed, err)
			}
			if input[len(input)-1] != "ELECTRON_RUN_AS_NODE="+value {
				t.Fatal("caller environment changed")
			}
		})
	}
}
