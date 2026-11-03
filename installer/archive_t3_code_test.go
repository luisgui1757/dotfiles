package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestT3CodeLauncherStartsTheAppWithoutTheSandboxAndTheUpdater(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the T3 Code app runs from this launcher only on Linux")
	}
	root := t.TempDir()
	payload := filepath.Join(root, "versions", "1")
	writeConfigFixture(t, filepath.Join(payload, filepath.FromSlash(t3CodeBinary)), "#!/bin/sh\nprintf '%s\\n' \"$T3CODE_DISABLE_AUTO_UPDATE\" \"$T3CODE_DESKTOP_LAUNCHER_PATH\" \"$@\"\n")
	if err := os.Chmod(filepath.Join(payload, filepath.FromSlash(t3CodeBinary)), 0755); err != nil {
		t.Fatal(err)
	}
	pin := ArchivePin{Format: "deb", RequiredFiles: []string{"t3-code", t3CodeBinary}, T3CodeLauncher: true}
	if err := prepareT3CodeLauncher(pin, payload); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current")
	if err := os.Symlink(filepath.Join("versions", "1"), current); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(current, "t3-code")
	output, err := exec.Command(launcher, "t3code://open?path=a b").Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if want := []string{"1", launcher, "--no-sandbox", "t3code://open?path=a b"}; !slices.Equal(got, want) {
		t.Fatalf("launcher started %q, want %q", got, want)
	}
}
