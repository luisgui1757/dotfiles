package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsVendorQueryProcessFixture(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "vendor-query-fixture" {
		return
	}
	role := os.Args[len(os.Args)-1]
	fmt.Fprint(os.Stderr, "#< CLIXML\n<Objs><Obj S=\"progress\">Preparing modules for first use.</Obj></Objs>\n")
	if role == "failure" {
		fmt.Fprint(os.Stderr, "native vendor query failed\x1b\n")
		os.Exit(23)
	}
	if role == "malformed" {
		fmt.Fprint(os.Stdout, "# malformed protocol output\n")
	}
	fmt.Fprint(os.Stdout, `{"ready":true}`)
	os.Exit(0)
}

func TestWindowsVendorQueryKeepsProgressSeparateAndRetainsFailures(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"progress", "failure", "malformed"} {
		t.Run(role, func(t *testing.T) {
			output, err := queryWindowsVendor(context.Background(), false, program, nil,
				"-test.run=^TestWindowsVendorQueryProcessFixture$", "--", "vendor-query-fixture", role)
			if role == "failure" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 23 || !strings.Contains(err.Error(), "native vendor query failed") || strings.ContainsRune(err.Error(), '\x1b') {
					t.Fatal("native query discarded its error or diagnostic", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Ready bool }
			if err := Decode(output, &result); role == "malformed" {
				if err == nil {
					t.Fatal("query discarded malformed protocol output")
				}
			} else if err != nil || !result.Ready {
				t.Fatal("successful vendor JSON was contaminated by stderr progress", err)
			}
		})
	}
}

func TestWindowsVendorQueryPowerShellProgress(t *testing.T) {
	name := "pwsh"
	if runtime.GOOS == "windows" {
		name = "powershell.exe"
	}
	program, err := exec.LookPath(name)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("optional PowerShell stream probe; process boundary cases always run")
	}
	script := `Write-Progress -Activity 'Preparing modules for first use.' -Status 'Loading' -PercentComplete 10;[Console]::Out.Write('{"ready":true}')`
	output, err := queryWindowsVendor(context.Background(), false, program, nil, windowsVendorArguments(script)...)
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Ready bool }
	if err := Decode(output, &result); err != nil || !result.Ready {
		t.Fatal("encoded PowerShell progress contaminated vendor JSON", err)
	}
}
