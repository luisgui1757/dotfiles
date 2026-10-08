//go:build windows

package installer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Direct PS7 -> powershell.exe rewrites module paths. An intermediate Go process
// inherits PS7 paths, so this regression must include that actual boundary.
func TestWindowsPowerShellInheritedModulePath(t *testing.T) {
	if os.Getenv("DOTFILES_MODULE_BOUNDARY_CHILD") != "1" {
		pwsh, err := exec.LookPath("pwsh")
		if err != nil {
			t.Fatal("native compatibility regression requires PowerShell 7", err)
		}
		program, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		script := `if ($PSVersionTable.PSVersion.Major -ne 7) { throw 'Expected PowerShell 7 parent' }; $env:DOTFILES_MODULE_BOUNDARY_CHILD='1'; & $env:DOTFILES_MODULE_BOUNDARY_PROGRAM '-test.run=^TestWindowsPowerShellInheritedModulePath$' '-test.v'; exit $LASTEXITCODE`
		command := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
		command.Env = append(os.Environ(), "DOTFILES_MODULE_BOUNDARY_PROGRAM="+program)
		output, err := command.CombinedOutput()
		t.Log(string(output))
		if err != nil {
			t.Fatal("PS7 intermediary compatibility failed", err)
		}
		return
	}
	powershell, err := DiscoverWindowsPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "native hash ü.txt")
	data := []byte("exact module boundary bytes")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(data))
	script := `$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; (Get-FileHash -LiteralPath ` + desktopPSQuote(path) + ` -Algorithm SHA256).Hash.ToLowerInvariant()`
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	raw := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := raw.CombinedOutput()
	diagnostic, err := windowsPowerShellBaselineDiagnostic(output, err, want)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(diagnostic)
	arguments := windowsVendorArguments(script)
	for _, query := range []struct {
		name string
		run  func() ([]byte, error)
	}{
		{"vendor", func() ([]byte, error) { return queryWindowsVendor(ctx, false, powershell, nil, arguments...) }},
		{"compiler", func() ([]byte, error) { return queryWindowsBuildTools(ctx, false, powershell, nil, arguments...) }},
		{"integration", func() ([]byte, error) {
			return runIntegrationQuery(ctx, nativeCommand{Program: powershell, Arguments: arguments})
		}},
		{"worker", func() ([]byte, error) {
			reply, err := executeNativeCommand(filepath.Join(root, "worker"), nativeCommand{Operation: strings.Repeat("a", 64), Program: powershell, Arguments: arguments})
			if err == nil && reply.Error != "" {
				err = fmt.Errorf("native worker: %s", reply.Error)
			}
			return reply.Output, err
		}},
	} {
		output, err := query.run()
		if err != nil || strings.TrimSpace(string(output)) != want {
			t.Fatalf("%s built-in hash binding failed: %v %s", query.name, err, output)
		}
		t.Log(query.name, "native Utility hash passed")
	}
	// Invoke the actual bootstrap binding without running its cache or installer.
	launcher, err := filepath.Abs(filepath.Join("..", "scripts", "installer-bootstrap.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := `$ErrorActionPreference='Stop'; $tokens=$null; $problems=$null; $ast=[Management.Automation.Language.Parser]::ParseFile(` + desktopPSQuote(launcher) + `,[ref]$tokens,[ref]$problems); if ($problems.Count) { throw 'Bootstrap parse error' }; $functions=@($ast.FindAll({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Import-BootstrapModules'},$true)); if ($functions.Count -ne 1) { throw 'Missing bootstrap module binding' }; & $functions[0].Body.GetScriptBlock(); ` + script
	command := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", bootstrap)
	output, err = command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != want {
		t.Fatalf("bootstrap built-in hash binding failed: %v %s", err, output)
	}
	t.Log("bootstrap native Utility hash passed")
}
