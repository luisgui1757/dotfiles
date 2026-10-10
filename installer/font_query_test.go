package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// This child reproduces PowerShell's separate success and diagnostic streams;
// it never registers fonts or calls native font APIs.
func TestFontQueryProcessFixture(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "font-query-fixture" {
		return
	}
	role := os.Args[len(os.Args)-1]
	var input struct{ Faces []fontFace }
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var err error
	switch role {
	case "progress", "failure", "malformed":
		_, err = fmt.Fprint(os.Stderr, "#< CLIXML\n<Objs Version=\"1.1.0.1\" xmlns=\"http://schemas.microsoft.com/powershell/2004/04\"><Obj S=\"progress\"><MS><PR N=\"Record\"><AV>Preparing modules for first use.</AV></PR></MS></Obj></Objs>")
	case "stdout-bound":
		_, err = fmt.Fprint(os.Stdout, strings.Repeat("x", (1<<20)+1))
	case "stderr-bound":
		_, err = fmt.Fprint(os.Stderr, strings.Repeat("x", (1<<20)+1))
	case "combined-bound":
		_, err = fmt.Fprint(os.Stdout, strings.Repeat("x", 600<<10))
		if err == nil {
			_, err = fmt.Fprint(os.Stderr, strings.Repeat("x", 600<<10))
		}
	default:
		os.Exit(2)
	}
	if err != nil {
		os.Exit(2)
	}
	if role == "malformed" {
		_, err = fmt.Fprint(os.Stdout, "# malformed protocol output\n")
	}
	if err == nil {
		err = json.NewEncoder(os.Stdout).Encode(make([]fontNativeFace, len(input.Faces)))
	}
	if err != nil {
		os.Exit(2)
	}
	if role == "failure" {
		fmt.Fprint(os.Stderr, "\nfont inspection fixture failure\x1b\n")
		os.Exit(23)
	}
	os.Exit(0)
}

func fontQueryBoundaryFixture(t *testing.T, role string) (*FontDriver, []string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := &FontDriver{Target: Context{OS: "windows", Arch: "amd64"}, PowerShell: program}
	d.Query = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		// Replace only the OS process, keeping the real query's input, output
		// handling, error propagation and JSON validation.
		command.Arguments = []string{"-test.run=^TestFontQueryProcessFixture$", "--", "font-query-fixture", role}
		return runIntegrationQuery(ctx, command)
	}
	return d, make([]string, len(hackFontFaces()))
}

func TestFontQuerySeparatesJSONFromPowerShellProgress(t *testing.T) {
	d, paths := fontQueryBoundaryFixture(t, "progress")
	faces, err := d.query(context.Background(), paths)
	if err != nil || len(faces) != len(hackFontFaces()) {
		t.Fatal("successful font JSON was contaminated by PowerShell progress", err)
	}
	for _, face := range faces {
		if face != (fontNativeFace{}) {
			t.Fatal("absence query changed its result", face)
		}
	}
}

func TestFontQueryPreservesFailedProcessDiagnostic(t *testing.T) {
	d, paths := fontQueryBoundaryFixture(t, "failure")
	_, err := d.query(context.Background(), paths)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 || !strings.Contains(err.Error(), "font inspection fixture failure") || strings.ContainsRune(err.Error(), '\x1b') {
		t.Fatal("native font failure or diagnostic was lost", err)
	}
}

func TestFontQueryRejectsMalformedStdout(t *testing.T) {
	d, paths := fontQueryBoundaryFixture(t, "malformed")
	if _, err := d.query(context.Background(), paths); err == nil {
		t.Fatal("query stripped invalid protocol output")
	}
}

func TestFontQueryBoundsBothProcessStreams(t *testing.T) {
	for _, role := range []string{"stdout-bound", "stderr-bound", "combined-bound"} {
		t.Run(role, func(t *testing.T) {
			d, paths := fontQueryBoundaryFixture(t, role)
			if _, err := d.query(context.Background(), paths); err == nil || !strings.Contains(err.Error(), "inspection exceeds 1 MiB") {
				t.Fatal("query exceeded its total output bound", err)
			}
		})
	}
}

// Encoded PowerShell progress is CLIXML on stderr even when success output is
// plain JSON. This read-only probe also runs against Windows PowerShell 5.1 in CI.
func TestFontQueryPowerShellProgress(t *testing.T) {
	name := "pwsh"
	if runtime.GOOS == "windows" {
		name = "powershell.exe"
	}
	program, err := exec.LookPath(name)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("optional read-only PowerShell probe; portable process regression always runs")
	}
	script := `$ErrorActionPreference='Stop';Write-Progress -Activity 'Preparing modules for first use.' -Status 'Loading' -PercentComplete 10;[Console]::Out.Write('{"font_query":true}')`
	output, err := runIntegrationQuery(context.Background(), nativeCommand{Program: program, Arguments: windowsVendorArguments(script)})
	var result struct {
		FontQuery bool `json:"font_query"`
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := Decode(output, &result); err != nil || !result.FontQuery {
		t.Fatal("encoded PowerShell progress contaminated protocol output", err)
	}
}
