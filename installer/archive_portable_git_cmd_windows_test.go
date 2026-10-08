package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This exercises real CMD using only private fixture files, with no vendor
// download or installation. The archive lifecycle below mocks only extraction
// and the runtime process; its post-install process is the actual interpreter.
func TestPortableGitPostInstallKeepsCMDInput(t *testing.T) {
	const script = "@echo running post-install\r\n@rmdir etc\\post-install\r\n@DEL post-install.bat\r\n"
	cmd := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	writeScript := func(payload string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(payload, "etc", "post-install"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(payload, "post-install.bat"), []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runScript := func(ctx context.Context, payload, name string) ([]byte, error) {
		command := exec.CommandContext(ctx, cmd, "/d", "/c", name)
		command.Dir = payload
		return command.CombinedOutput()
	}
	baseline := filepath.Join(t.TempDir(), "upstream input ü")
	writeScript(baseline)
	output, err := runScript(t.Context(), baseline, "post-install.bat")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), "running post-install") {
		t.Fatal("expected CMD to fail after its executing input deletes itself", err, string(output))
	}
	if _, err := os.Lstat(filepath.Join(baseline, "post-install.bat")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("baseline did not reach upstream self-deletion", err, string(output))
	}
	_, d, _ := portableGitFixtureDriver(t)
	original := d.Run
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		payload := filepath.Dir(command.Program)
		if filepath.Base(command.Program) == "git-bash.exe" {
			return runScript(ctx, payload, strings.TrimPrefix(command.Arguments[len(command.Arguments)-1], "--command="))
		}
		output, err := original(ctx, command)
		if filepath.Base(command.Program) == "portable-git.exe" {
			writeScript(payload)
		}
		return output, err
	}
	payload := filepath.Join(t.TempDir(), "private staged input ü")
	intent := archiveIntent{Operation: strings.Repeat("a", 64), Pin: d.Pins["tool.shared"]}
	if err := d.preparePayload(t.Context(), intent, payload); err != nil {
		t.Fatal("unchanged sibling input must let CMD reach a successful end", err)
	}
	for _, name := range []string{"post-install.bat", ".prepare-post-install.bat", "etc/post-install"} {
		if _, err := os.Lstat(filepath.Join(payload, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("successful preparation left an interpreter input", name, err)
		}
	}
}
