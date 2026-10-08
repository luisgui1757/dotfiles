//go:build !windows

package installer

import (
	"os/exec"
	"syscall"
)

func configureNativeProcess(command *exec.Cmd, worker bool) {
	if worker {
		// A terminal interrupt targets the foreground process group. Keep the
		// worker and its children outside it so a surviving privileged child
		// cannot lose its lock when the controller handles Ctrl-C or exits.
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}
