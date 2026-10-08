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

// Failure-only paired transfer evidence. Each download uses a fresh private
// file and the same reviewed URL/hash; neither probe populates the real cache
// or turns the original public-lifecycle failure into a pass.
func diagnoseWindowsBootstrapDownload(t *testing.T, powershell, repository string) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_TEST_PUBLIC_WINDOWS_MIGRATION") != "1" {
		t.Fatal("bootstrap download diagnostics require the disposable native fixture")
	}
	for _, preference := range []string{"Continue", "SilentlyContinue"} {
		directory := t.TempDir()
		archive := filepath.Join(directory, "go.zip")
		script := `$ErrorActionPreference='Stop'
$ProgressPreference=` + desktopPSQuote(preference) + `
[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12
$rows=@(Get-Content -LiteralPath ` + desktopPSQuote(filepath.Join(repository, "installer", "bootstrap-toolchain.tsv")) + ` | Where-Object { -not $_.StartsWith('#') } | ConvertFrom-Csv -Delimiter "` + "`t" + `" | Where-Object platform -CEQ 'windows-amd64')
if ($rows.Count -ne 1) { throw 'Diagnostic requires exact checked-in Windows pin' }
$pin=$rows[0]
if ($pin.filename -cnotmatch '^go[0-9]+\.[0-9]+\.[0-9]+\.windows-amd64\.zip$' -or $pin.sha256 -cnotmatch '^[a-f0-9]{64}$') { throw 'Invalid diagnostic pin' }
$archive=` + desktopPSQuote(archive) + `
$timer=[Diagnostics.Stopwatch]::StartNew()
Invoke-WebRequest -UseBasicParsing -Uri "https://go.dev/dl/$($pin.filename)" -OutFile $archive
if ((Get-Item -LiteralPath $archive).Length -ne [long]$pin.size -or (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -cne $pin.sha256) { throw 'Diagnostic checksum or size mismatch' }
@{ progress=$ProgressPreference.ToString(); elapsed_ms=$timer.ElapsedMilliseconds; bytes=(Get-Item -LiteralPath $archive).Length; verified=$true } | ConvertTo-Json -Compress`
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		start := time.Now()
		command := exec.CommandContext(ctx, powershell, windowsVendorArguments(script)...)
		var output nativeOutput
		command.Stdout, command.Stderr = &output, &output
		err := command.Run()
		contextErr := ctx.Err()
		cancel()
		var bytes int64
		if info, statErr := os.Stat(archive); statErr == nil {
			bytes = info.Size()
		} else if !os.IsNotExist(statErr) {
			t.Log("bootstrap diagnostic file inspection:", statErr)
		}
		t.Logf("bootstrap download probe progress=%s elapsed=%s bytes=%d err=%v context=%v output=%s", preference, time.Since(start), bytes, err, contextErr, strings.TrimSpace(nativeDiagnostic(output.data.Bytes())))
	}
}
