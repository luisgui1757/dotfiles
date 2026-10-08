//go:build darwin || linux

package terminal

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func normalTerminalEnd(err error) bool {
	// Linux reports EIO, Darwin EOF when the last slave closes.
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EIO)
}

func startNativeSession(t *testing.T, mode string) *nativeSession {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		t.Fatal(err)
	}
	resize := func(width, height int) error {
		return unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(width), Row: uint16(height)})
	}
	if err := resize(80, 24); err != nil {
		t.Fatal(errors.Join(err, master.Close(), slave.Close()))
	}
	command := exec.Command(os.Args[0], "-test.run=^TestTerminalChild$")
	command.Env = append(os.Environ(), "DOTFILES_TEST_TERMINAL_CHILD="+mode)
	command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := command.Start(); err != nil {
		t.Fatal(errors.Join(err, master.Close(), slave.Close()))
	}
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	finished := false
	finish := func() error {
		select {
		case err := <-done:
			finished = true
			return err
		case <-time.After(10 * time.Second):
			return errors.New("PTY child did not exit")
		}
	}
	stop := func() error {
		if finished {
			return master.Close()
		}
		select {
		case err := <-done:
			return errors.Join(err, master.Close())
		default:
			killErr := command.Process.Kill()
			waitErr := <-done
			var exited *exec.ExitError
			if errors.As(waitErr, &exited) && exited.ProcessState.Sys().(syscall.WaitStatus).Signal() == syscall.SIGKILL {
				waitErr = nil
			}
			if errors.Is(killErr, os.ErrProcessDone) {
				killErr = nil
			}
			return errors.Join(killErr, waitErr, master.Close())
		}
	}
	return watchSession(t, &nativeSession{input: master, output: master, done: done, resize: resize, finish: finish, stop: stop})
}
