//go:build !windows

package installer

import (
	"os/exec"
	"syscall"
	"testing"
)

func enableNativeControllerInterrupt() error { return nil }

func TestNativeWorkerRetainsLockAfterTerminalInterrupt(t *testing.T) {
	testNativeControllerLoss(t, func(command *exec.Cmd) {
		// Only our isolated fixture group receives the signal, never the test
		// runner's terminal or another user's process.
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}, func(command *exec.Cmd) error {
		return syscall.Kill(-command.Process.Pid, syscall.SIGINT)
	})
}
