package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeWindowsPipConfigurationIsolation(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LANGUAGE_PREPARATION") != "1" {
		t.Skip("enable private pinned Python preparation")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(t.TempDir(), "python")
	if err := downloadArchive(ctx, nil, pins["tool.python"], payload); err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(payload, "python.exe")
	// pip --isolated still reads site and machine configuration unless the
	// null-device spelling exactly matches Python's os.devnull.
	configuration := filepath.Join(payload, "pip.ini")
	writeConfigFixture(t, configuration, "[global]\nrequire-virtualenv = true\n")
	run := func(config string) string {
		t.Helper()
		command := exec.CommandContext(ctx, python, "-I", "-B", "-m", "pip", "--isolated", "config", "list")
		command.Env = append(os.Environ(), "PIP_CONFIG_FILE="+config)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("native pip configuration: %v\n%s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if output := run(""); !strings.Contains(output, "global.require-virtualenv='true'") {
		t.Fatalf("site configuration fixture was not consumed: %q", output)
	}
	if output := run(pipConfigNullDevice()); output != "" {
		t.Fatalf("external pip configuration was not disabled: %q", output)
	}
}
