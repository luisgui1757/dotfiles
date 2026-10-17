//go:build windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func enableNativeControllerInterrupt() error {
	// Windows' inherited ignore-Ctrl-C attribute is independent of Go's signal
	// subscriptions. A CI launcher may set it; explicitly enable this private
	// fixture controller without changing the runner's handling.
	if result, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler").Call(0, 0); result == 0 {
		return fmt.Errorf("enable fixture controller Ctrl-C: %w", err)
	}
	return nil
}

func TestNativeWorkerRetainsLockAfterConsoleInterrupt(t *testing.T) {
	testNativeControllerLoss(t, func(command *exec.Cmd) {
		// Create a private console for the fixture. Never signal the runner's
		// console or detach the test process from its own console.
		command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	}, func(command *exec.Cmd) error {
		sender := exec.Command(command.Path, "-test.run=^TestNativeWorkerConsoleSignalSender$")
		sender.Env = append(os.Environ(), "DOTFILES_NATIVE_CONSOLE_SIGNAL="+strconv.Itoa(command.Process.Pid))
		sender.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
		output, err := sender.CombinedOutput()
		if err != nil {
			return fmt.Errorf("signal private fixture console: %w\n%s", err, output)
		}
		return nil
	})
}

func TestNativeWorkerConsoleSignalSender(t *testing.T) {
	value := os.Getenv("DOTFILES_NATIVE_CONSOLE_SIGNAL")
	if value == "" {
		return
	}
	pid, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	// FreeConsole affects only this short-lived helper and also succeeds when
	// it has no console. CREATE_NO_WINDOW must not be used as proof that the
	// process can attach immediately on every supported Windows host.
	if result, _, err := kernel.NewProc("FreeConsole").Call(); result == 0 {
		t.Fatal("detach signal helper", err)
	}
	if result, _, err := kernel.NewProc("AttachConsole").Call(uintptr(pid)); result == 0 {
		t.Fatal("attach private fixture console", err)
	}
	var clients [32]uint32
	count, _, err := kernel.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&clients[0])), uintptr(len(clients)))
	if count == 0 || count > uintptr(len(clients)) {
		t.Fatal("inspect private fixture console clients", count, err)
	}
	found := false
	for _, client := range clients[:count] {
		found = found || client == uint32(pid)
	}
	if !found {
		t.Fatal("signal sender is not attached to the controller's private console")
	}
	if result, _, err := kernel.NewProc("SetConsoleCtrlHandler").Call(0, 1); result == 0 {
		t.Fatal("ignore sender interrupt", err)
	}
	if result, _, err := kernel.NewProc("GenerateConsoleCtrlEvent").Call(windows.CTRL_C_EVENT, 0); result == 0 {
		t.Fatal("send console interrupt", err)
	}
	os.Exit(0)
}
