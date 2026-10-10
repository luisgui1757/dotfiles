package installer

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The desktop fixture's PID JSON and visible-window token use the same bounded
// read-only process boundary. Exercise real encoded PowerShell progress without
// launching a GUI or relying on a particular module's first-load state.
func TestDesktopQueryPowerShellProgress(t *testing.T) {
	name := "pwsh"
	if runtime.GOOS == "windows" {
		name = "powershell.exe"
	}
	program, err := exec.LookPath(name)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("optional read-only PowerShell probe; required on Windows")
	}
	for _, protocol := range []string{"[123,456]", "123", "visible"} {
		t.Run(protocol, func(t *testing.T) {
			script := `$ErrorActionPreference='Stop';Write-Progress -Activity 'Preparing modules for first use.' -Status 'Loading' -PercentComplete 10;[Console]::Out.Write(` + desktopPSQuote(protocol) + `)`
			command := nativeCommand{Program: program, Arguments: windowsVendorArguments(script)}
			output, err := runIntegrationQuery(context.Background(), command)
			if err != nil || strings.TrimSpace(string(output)) != protocol {
				t.Fatalf("desktop query protocol was contaminated: %q (%v)", output, err)
			}
		})
	}
	t.Run("failed query", func(t *testing.T) {
		script := `$ErrorActionPreference='Stop';Write-Progress -Activity 'Preparing modules for first use.' -Status 'Loading' -PercentComplete 10;[Console]::Out.Write('visible');[Console]::Error.Write('desktop query fixture failure');exit 23`
		output, err := runIntegrationQuery(context.Background(), nativeCommand{Program: program, Arguments: windowsVendorArguments(script)})
		var exit *exec.ExitError
		if string(output) != "visible" || !errors.As(err, &exit) || exit.ExitCode() != 23 || !strings.Contains(err.Error(), "desktop query fixture failure") {
			t.Fatal("failed query was accepted or lost its exit/diagnostic", string(output), err)
		}
	})
}
