package installer

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed vcredist-pin.json
var windowsRuntimePinData []byte

//go:embed buildtools-pin.json
var windowsBuildToolsPinData []byte

func DefaultWindowsBuildToolsPin() (WindowsBuildToolsPin, error) {
	var pin WindowsBuildToolsPin
	if err := Decode(windowsBuildToolsPinData, &pin); err != nil {
		return pin, err
	}
	return pin, pin.Validate()
}

func configureWindowsBuildTools(platform NativePlatform, session *nativeSession) (*WindowsBuildToolsDriver, error) {
	if platform.OS != "windows" || platform.WindowsPowerShell == "" || platform.WindowsBuildToolsDirectory == "" {
		return nil, nil
	}
	if session == nil || !filepath.IsAbs(platform.WindowsPowerShell) || !filepath.IsAbs(platform.WindowsBuildToolsDirectory) {
		return nil, errors.New("Build Tools requires observed system paths and native session")
	}
	pin, err := DefaultWindowsBuildToolsPin()
	if err != nil {
		return nil, err
	}
	return &WindowsBuildToolsDriver{
		Directory: filepath.Join(filepath.Dir(session.Directory), "windows-buildtools"), WorkerDirectory: session.Directory,
		PowerShell: platform.WindowsPowerShell, InstallDirectory: platform.WindowsBuildToolsDirectory,
		Pin: pin, Query: queryWindowsBuildTools, Run: session.run,
	}, nil
}

func DefaultWindowsVendorPin() (WindowsVendorPin, error) {
	var pin WindowsVendorPin
	if err := Decode(windowsRuntimePinData, &pin); err != nil {
		return pin, err
	}
	return pin, pin.Validate()
}

func configureWindowsVendor(platform NativePlatform, session *nativeSession) (*WindowsVendorDriver, error) {
	if platform.OS != "windows" || platform.WindowsPowerShell == "" {
		return nil, nil
	}
	if !filepath.IsAbs(platform.WindowsPowerShell) || filepath.Clean(platform.WindowsPowerShell) != platform.WindowsPowerShell || session == nil {
		return nil, errors.New("Windows vendor provider needs observed system PowerShell and native session")
	}
	pin, err := DefaultWindowsVendorPin()
	if err != nil {
		return nil, err
	}
	return &WindowsVendorDriver{
		Directory: filepath.Join(filepath.Dir(session.Directory), "windows-vendor"), WorkerDirectory: session.Directory,
		PowerShell: platform.WindowsPowerShell, Pins: map[string]WindowsVendorPin{"tool.vcredist": pin},
		Query: queryWindowsVendor, Run: session.run,
	}, nil
}

func queryWindowsVendor(ctx context.Context, privileged bool, program string, input []byte, arguments ...string) ([]byte, error) {
	if privileged || !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return nil, errors.New("Windows vendor inspection cannot elevate or use a relative executable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, arguments...)
	command.Stdin = bytes.NewReader(input)
	output, diagnostic := boundedCommandOutput{limit: 1 << 20}, boundedCommandOutput{}
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("Windows vendor inspection failed: %w%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), nil
}
