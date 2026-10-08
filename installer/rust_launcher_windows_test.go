//go:build windows

package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsRustLauncherWaitsForChildAfterConsoleInterrupt(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(bin, "rustc.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", launcher, "./cmd/dotfiles")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual launcher: %s %v", output, err)
	}
	buildRustLauncherFixture(t, filepath.Join(bin, rustOriginalCommand("rustc.exe")))
	ready := filepath.Join(root, "ready")
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, testExecutable, "-test.run=^TestWindowsRustLauncherConsoleController$")
	command.Env = append(os.Environ(), "DOTFILES_RUST_INTERRUPT_READY="+ready, "DOTFILES_RUST_CONSOLE_LAUNCHER="+launcher)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	if err := awaitNativeFile(ready); err != nil {
		t.Fatal(err)
	}
	// Reuse the existing bounded sender, which verifies it attached to exactly
	// this fixture's private console before generating a console-wide Ctrl-C.
	sender := exec.CommandContext(ctx, testExecutable, "-test.run=^TestNativeWorkerConsoleSignalSender$")
	sender.Env = append(os.Environ(), "DOTFILES_NATIVE_CONSOLE_SIGNAL="+strconv.Itoa(command.Process.Pid))
	sender.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if result, err := sender.CombinedOutput(); err != nil {
		t.Fatalf("signal private compiler console: %s %v", result, err)
	}
	select {
	case err := <-done:
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 23 || !strings.Contains(output.String(), "child handled interrupt") {
			t.Fatal("launcher lost child interrupt status", err, output.String())
		}
	case <-ctx.Done():
		t.Fatal("launcher did not finish after child interrupt")
	}
}

// The controller changes only its new private console's inherited Ctrl-C
// handling, then starts the real installed-entrypoint binary in that console.
func TestWindowsRustLauncherConsoleController(t *testing.T) {
	launcher := os.Getenv("DOTFILES_RUST_CONSOLE_LAUNCHER")
	if launcher == "" {
		return
	}
	if err := enableNativeControllerInterrupt(); err != nil {
		t.Fatal(err)
	}
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	command := exec.Command(launcher)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			os.Exit(exited.ExitCode())
		}
		t.Fatal(err)
	}
	os.Exit(0)
}
