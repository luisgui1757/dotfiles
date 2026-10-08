package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsVendorConnectionIsPureAndUsesTheReviewedBundle(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	platform := NativePlatform{Context: Context{OS: "windows", Arch: "amd64"}, WindowsPowerShell: filepath.Join(root, "missing-system-powershell.exe")}
	provider, err := configureWindowsVendor(platform, newNativeSession(filepath.Join(root, "worker")))
	if err != nil || provider == nil {
		t.Fatal("Windows runtime was not connected", err)
	}
	if provider.Pins["tool.vcredist"] != windowsVendorNativePin() {
		t.Fatal("production runtime pin differs from the verified native fixture")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("provider construction wrote state", entries, err)
	}
	driver := &NativeDriver{Context: platform.Context, WindowsVendor: provider}
	r := Resource{ID: "tool.vcredist"}
	if got, err := driver.provider(r); err != nil || got != provider {
		t.Fatal("Windows runtime route missing", err)
	}
	driver.WindowsVendorIssue = "system PowerShell is unavailable"
	if got, err := driver.Observe(context.Background(), r, Receipt{}); err != nil || !got.Unknown || got.Pending != driver.WindowsVendorIssue {
		t.Fatal("system discovery failure lost its resource-local explanation", got, err)
	}
	if _, err := driver.AcquireMutation(context.Background()); err == nil {
		t.Fatal("Windows vendor mutation accepted a missing native session")
	}
}

func TestWindowsBuildToolsConnectionUsesObservedDedicatedDirectory(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	platform := NativePlatform{Context: Context{OS: "windows", Arch: "amd64"}, WindowsPowerShell: filepath.Join(root, "system", "powershell.exe"), WindowsBuildToolsDirectory: filepath.Join(root, "program files", "DotfilesBuildTools")}
	provider, err := configureWindowsBuildTools(platform, newNativeSession(filepath.Join(root, "worker")))
	if err != nil || provider == nil || provider.InstallDirectory != platform.WindowsBuildToolsDirectory {
		t.Fatal("compiler did not use its observed installation directory", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("compiler construction wrote native state", err)
	}
	driver := &NativeDriver{Context: platform.Context, BuildTools: provider}
	r := Resource{ID: "tool.compiler"}
	if got, err := driver.provider(r); err != nil || got != provider {
		t.Fatal("compiler route missing", err)
	}
	if _, err := driver.AcquireMutation(context.Background()); err == nil {
		t.Fatal("compiler mutation accepted a missing native session")
	}
}
