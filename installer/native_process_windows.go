//go:build windows

package installer

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureNativeProcess(command *exec.Cmd, _ bool) {
	// The worker and native console commands use pipes, not the controller's
	// console. Ctrl-C/console-close must not release the worker's lifetime lock.
	// GUI installers retain their normal GUI; this flag applies to console apps.
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}
