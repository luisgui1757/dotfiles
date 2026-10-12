package installer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Captured from the actual Windows 11 all-tools failure. Exercise the pure
// resolver in real PowerShell; never fabricate or edit machine registry keys.
func TestWindowsRuntimeRegistrationReconciliation(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" && runtime.GOOS == "windows" {
		program = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	}
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell is not available")
		}
	}
	data, err := os.ReadFile(filepath.Join("testdata", "windows-runtime-mixed-views.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, change string
		refused      bool
		consumer     string
	}{
		{"absent", "$inputData.Registry=@(); $inputData.Files=@()", false, ""},
		{"captured-mixed-views", "", false, "Older VC++ x64 runtime registration in Registry32: 14.44.35211.0"},
		{"reverse-views", "$inputData.Registry[0].View='Registry32'; $inputData.Registry[3].View='Registry64'", false, "Older VC++ x64 runtime registration in Registry64: 14.44.35211.0"},
		{"equivalent-versions", "$inputData.Registry[3].Version='14.51.36247'", false, ""},
		{"single-view", "$inputData.Registry=@($inputData.Registry | Where-Object {$_.View -ne 'Registry32' -or $_.Kind -ne 'runtime'})", false, ""},
		{"unverified-newer-value", "$inputData.Registry[3].Version='14.52.40000.0'", true, ""},
		{"newer-revision", "$inputData.Registry[3].Version='14.51.36247.1'", true, ""},
		{"missing-bundle", "$inputData.Registry=@($inputData.Registry | Where-Object {$_.Kind -ne 'bundle'})", true, ""},
		{"second-bundle", "$other=$inputData.Registry[4] | ConvertTo-Json | ConvertFrom-Json; $other.ID='{11111111-1111-1111-1111-111111111111}'; $inputData.Registry+= $other", true, ""},
		{"bundle-view-disagreement", "$other=$inputData.Registry[4] | ConvertTo-Json | ConvertFrom-Json; $other.DisplayVersion='14.44.35211'; $inputData.Registry+= $other", true, ""},
		{"single-version-bundle-disagreement", "$inputData.Registry[0].Version='14.44.35211'", true, ""},
		{"wrong-publisher", "$inputData.Registry[4].Publisher='Other publisher'", true, ""},
		{"different-msi-id", "$inputData.Registry[5].ID='{11111111-1111-1111-1111-111111111111}'", true, ""},
		{"different-msi-version", "$inputData.Registry[5].Version='14.44.35211'", true, ""},
		{"missing-msi-version", "$inputData.Registry[5].Version=''", true, ""},
		{"missing-msi-part", "$inputData.Registry=@($inputData.Registry | Where-Object {$_.Part -ne 'Additional'})", true, ""},
		{"missing-dll", "$inputData.Files=@($inputData.Files | Where-Object {$_.Name -ne 'msvcp140.dll'})", true, ""},
		{"different-dll-version", "$inputData.Files[2].Version='14.44.35211.0'", true, ""},
		{"outside-dependent", "$inputData.Registry[1].Dependents+= 'outside-consumer'", false, "Older VC++ x64 runtime registration in Registry32: 14.44.35211.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			script := "$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; " + windowsRuntimeVersionFunction + windowsRuntimeRegistrationFunctions + `
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
` + tc.change + `
$files = @{}
foreach ($file in $inputData.Files) { $files[$file.Name] = $file.Version }
Resolve-RuntimeRegistration $inputData.Registry $files | ConvertTo-Json -Depth 4 -Compress
`
			command := exec.CommandContext(ctx, program, windowsVendorArguments(script)...)
			command.Stdin = strings.NewReader(string(data))
			output, err := command.CombinedOutput()
			if tc.refused {
				if err == nil || !strings.Contains(strings.ToLower(string(output)), "runtime") {
					t.Fatalf("unproved registry reconciliation accepted: %s: %v", output, err)
				}
				return
			}
			var got struct {
				Version   string            `json:"version"`
				Bundle    string            `json:"bundle_id"`
				Packages  map[string]string `json:"packages"`
				Consumers []string          `json:"consumers"`
			}
			if err != nil {
				t.Fatalf("corroborated runtime rejected: %s: %v", output, err)
			}
			version, bundle, packages := "14.51.36247.0", windowsVendorNativePin().BundleID, 2
			if tc.name == "absent" {
				version, bundle, packages = "", "", 0
			}
			if err := json.Unmarshal(output, &got); err != nil || got.Version != version || got.Bundle != bundle || len(got.Packages) != packages {
				t.Fatalf("resolved identity changed: %+v: %v", got, err)
			}
			want := []string{}
			if tc.consumer != "" {
				want = append(want, tc.consumer)
			}
			if tc.name == "outside-dependent" {
				want = append(want, "outside-consumer")
			}
			if strings.Join(got.Consumers, "\n") != strings.Join(want, "\n") {
				t.Fatalf("outside registrations/dependents changed: %v; want %v", got.Consumers, want)
			}
		})
	}
}
