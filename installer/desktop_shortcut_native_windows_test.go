package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Ordinary Windows coverage uses the real system PowerShell 5.1/COM boundary.
// It creates only private .lnk files, never a real Start menu entry or process.
func TestDesktopShortcutNativePowerShell51COMPublication(t *testing.T) {
	c, d, destination, _, _ := shortcutFixture(t)
	var err error
	d.PowerShell, err = DiscoverWindowsPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		reply, err := executeNativeCommand(filepath.Join(d.Directory, "worker"), command)
		if err == nil && reply.Error != "" {
			err = errors.New(reply.Error)
		}
		return reply.Output, err
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"app"}})
	recipe, err := d.recipe("desktop.vscode")
	if err != nil {
		t.Fatal(err)
	}
	script := `$ErrorActionPreference='Stop';[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$link=(New-Object -ComObject WScript.Shell).CreateShortcut(` + desktopPSQuote(destination) + `);[Console]::Out.Write($link.TargetPath)`
	output, err := runIntegrationQuery(t.Context(), nativeCommand{Program: d.PowerShell, Arguments: windowsVendorArguments(script)})
	if err != nil || string(output) != recipe.Command {
		t.Fatal("published native shortcut lost its exact target", err, string(output))
	}
	if check, err := c.Dispatch(t.Context(), Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "ready" {
		t.Fatal("native shortcut ownership was lost", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("private native shortcut was not removed", err)
	}
}
