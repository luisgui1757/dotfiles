package installer

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Diagnostic-only instrumentation of the same bounded query. Labels contain no
// inventory, environment or personal data; errors retain the original status.
func traceBuildToolsNativeQuery(t *testing.T, query func(context.Context, bool, string, []byte, ...string) ([]byte, error)) func(context.Context, bool, string, []byte, ...string) ([]byte, error) {
	t.Helper()
	return func(ctx context.Context, privileged bool, program string, input []byte, arguments ...string) ([]byte, error) {
		script := shortcutPowerShellScript(t, arguments)
		for _, point := range []struct{ source, label string }{
			{"$signature = Get-AuthenticodeSignature", "authenticode"},
			{"if (-not $chain.Build($signature.SignerCertificate))", "certificate-chain"},
			{"$json = & $locator", "locator-inventory"},
			{"foreach ($item in $registered)", "package-inventory"},
			{"foreach ($name in $files.Keys)", "tool-file-hashes"},
			{"$consumers = @(Get-CimInstance", "process-consumers"},
			{"$boot = (Get-CimInstance", "boot-identity"},
			{"$state = Get-BuildToolsState", "registered-state"},
			{"Assert-MicrosoftSignature $compiler", "compiler-signature"},
			{"Assert-MicrosoftSignature $linker", "linker-signature"},
			{"$null = & $cmd /d /s /c $command", "compile-and-run"},
			{"$state.healthy = $true", "compiler-completed"},
			{"$state | ConvertTo-Json", "result-serialization"},
			{"$output = & (Join-Path ([Environment]::SystemDirectory) 'cmd.exe')", "compiler-environment"},
		} {
			count := strings.Count(script, point.source)
			if count > 1 {
				t.Fatalf("ambiguous native timing point %s", point.label)
			}
			if count == 1 {
				script = strings.Replace(script, point.source, "[Console]::Error.WriteLine('Build Tools stage "+point.label+" elapsed-ms=' + $dotfilesQueryTimer.ElapsedMilliseconds)\n"+point.source, 1)
			}
		}
		script = "$dotfilesQueryTimer=[Diagnostics.Stopwatch]::StartNew()\n" + script
		started := time.Now()
		output, err := query(ctx, privileged, program, input, windowsVendorArguments(script)...)
		t.Logf("bounded native Build Tools query completed after %s: %v", time.Since(started), err)
		return output, err
	}
}
