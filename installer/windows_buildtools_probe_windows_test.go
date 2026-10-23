package installer

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestWindowsBuildToolsProbeRequiresExecutionWitness(t *testing.T) {
	program, err := DiscoverWindowsPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, command string
		valid         bool
	}{
		{"zero without execution", "exit /b 0", false},
		{"wrong output", "echo unrelated", false},
		{"token followed by failure", "echo dotfiles-cxx-probe-ok& exit /b 9", false},
		{"duplicate token", "echo dotfiles-cxx-probe-ok& echo dotfiles-cxx-probe-ok", false},
		{"completed runtime", "echo compiler diagnostics& echo dotfiles-cxx-probe-ok", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			// Only the external process is a fixture. The production capture and
			// validation execute unchanged through the real Windows command shell.
			script := "$ErrorActionPreference='Stop'; $cmd=Join-Path ([Environment]::SystemDirectory) 'cmd.exe'; $command=" + powershellLiteral(test.command) + "; " + windowsBuildToolsProbeRun + "; Write-Output 'verified-probe'"
			command := exec.CommandContext(ctx, program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
			output, err := command.CombinedOutput()
			if ctx.Err() != nil || test.valid != (err == nil) || test.valid && strings.TrimSpace(string(output)) != "verified-probe" {
				t.Fatalf("probe validation valid=%t: %v (context %v)\n%s", test.valid, err, ctx.Err(), output)
			}
		})
	}
}
