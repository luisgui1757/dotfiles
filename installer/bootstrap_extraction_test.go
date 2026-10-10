package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Execute the actual bootstrap extraction action, without invoking a download,
// compiler or the known-folder bootstrap cache. Windows uses system PowerShell
// 5.1, not the independently installed PowerShell 7 test tooling.
func TestBootstrapToolchainExtraction(t *testing.T) {
	var powershell string
	var err error
	if runtime.GOOS == "windows" {
		powershell, err = DiscoverWindowsPowerShell()
	} else {
		powershell, err = exec.LookPath("pwsh")
	}
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal("system Windows PowerShell is required:", err)
		}
		t.Skip("PowerShell extraction boundary requires a local engine:", err)
	}
	launcher, err := filepath.Abs(filepath.Join("..", "scripts", "installer-bootstrap.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"literal paths", "invalid ZIP", "escaping entry", "existing file"} {
		t.Run(scenario, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "bootstrap ü ' [literal]")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			archive, destination := filepath.Join(root, "verified.zip"), filepath.Join(root, "toolchain")
			data := []byte("exact pinned input bytes\x00\xff\n")
			files := map[string][]byte{"go/bin/go.exe": data, "go/src/unicode ü/file.txt": []byte("nested data")}
			// The longest entry in the checksum-pinned go1.27.1 Windows ZIP.
			files["go/src/internal/trace/testdata/generators/go122-syscall-steal-proc-gen-boundary-reacquire-new-proc-bare-m.go"] = data
			if scenario == "escaping entry" {
				files = map[string][]byte{"../outside.txt": data}
			}
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for name, contents := range files {
				entry, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write(contents); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			archiveBytes := buffer.Bytes()
			if scenario == "invalid ZIP" {
				archiveBytes = []byte("not a ZIP archive")
			}
			if err := os.WriteFile(archive, archiveBytes, 0600); err != nil {
				t.Fatal(err)
			}
			preserved := filepath.Join(root, "outside.txt")
			if scenario == "existing file" {
				preserved = filepath.Join(destination, "go", "bin", "go.exe")
				if err := os.MkdirAll(filepath.Dir(preserved), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(preserved, []byte("preserved"), 0600); err != nil {
				t.Fatal(err)
			}
			script := `param([string]$Archive, [string]$Toolchain, [string]$Launcher)
$ErrorActionPreference = 'Stop'
# Extraction must not execute per-entry PowerShell progress machinery.
$ProgressPreference = 'Stop'
if ($env:OS -eq 'Windows_NT' -and ($PSVersionTable.PSVersion.Major -ne 5 -or $PSVersionTable.PSVersion.Minor -ne 1)) { throw 'Requires system Windows PowerShell 5.1' }
[Console]::Error.WriteLine("Extraction engine: PowerShell $($PSVersionTable.PSVersion), CLR $([Environment]::Version)")
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($Launcher, [ref]$tokens, [ref]$errors)
if (@($errors).Count -ne 0) { throw 'Bootstrap parse error' }
$actions = @($ast.FindAll({ param($node)
	$node -is [Management.Automation.Language.CommandAst] -and
	$node.GetCommandName() -eq 'Invoke-BootstrapPhase' -and
	$node.CommandElements[1].Value -eq 'toolchain extraction'
}, $true))
if ($actions.Count -ne 1) { throw 'Expected one bootstrap extraction action' }
& $actions[0].CommandElements[2].ScriptBlock.GetScriptBlock()
`
			scriptPath := filepath.Join(root, "extraction.ps1")
			if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, archive, destination, launcher)
			command.Env = append(os.Environ(), "HOME="+root, "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "XDG_CACHE_HOME="+filepath.Join(root, "cache"), "XDG_DATA_HOME="+filepath.Join(root, "data"))
			var stderr bytes.Buffer
			command.Stderr = &stderr
			output, runErr := command.Output()
			t.Log(strings.TrimSpace(stderr.String()))
			if scenario == "literal paths" {
				if runErr != nil || len(output) != 0 {
					t.Fatalf("extraction failed or polluted stdout: %v %s", runErr, output)
				}
				for name, expected := range files {
					actual, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
					if err != nil || !bytes.Equal(actual, expected) {
						t.Fatalf("extracted bytes differ for %s: %v", name, err)
					}
				}
			} else if runErr == nil || ctx.Err() != nil || len(output) != 0 {
				t.Fatalf("expected immediate extraction error, got %v context=%v stdout=%s", runErr, ctx.Err(), output)
			}
			actual, err := os.ReadFile(preserved)
			if err != nil || string(actual) != "preserved" {
				t.Fatalf("extraction changed pre-existing bytes: %q %v", actual, err)
			}
		})
	}
}
