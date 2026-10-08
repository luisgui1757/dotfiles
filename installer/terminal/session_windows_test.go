package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func normalTerminalEnd(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, windows.ERROR_BROKEN_PIPE)
}

func startNativeSession(t *testing.T, mode string) *nativeSession {
	t.Helper()
	t.Setenv("DOTFILES_TEST_TERMINAL_CHILD", mode)
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(errors.Join(err, inputRead.Close(), inputWrite.Close()))
	}
	var console windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: 80, Y: 24}, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), 0, &console); err != nil {
		t.Fatal(errors.Join(err, inputRead.Close(), inputWrite.Close(), outputRead.Close(), outputWrite.Close()))
	}
	// Drain output before CreateProcess and throughout ClosePseudoConsole. The
	// latter can emit a final frame and deadlock an undrained synchronous pipe.
	s := &nativeSession{input: inputWrite, output: outputRead}
	var process windows.ProcessInformation
	s.finish = func() error {
		result, err := windows.WaitForSingleObject(process.Process, 10000)
		if err != nil {
			return err
		}
		if result != windows.WAIT_OBJECT_0 {
			return fmt.Errorf("ConPTY child did not finish: %d", result)
		}
		var code uint32
		if err := windows.GetExitCodeProcess(process.Process, &code); err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("ConPTY child exited with code %d", code)
		}
		return nil
	}
	s.stop = func() error {
		windows.ClosePseudoConsole(console)
		var waitErr error
		if process.Process != 0 {
			result, err := windows.WaitForSingleObject(process.Process, 10000)
			waitErr = err
			if err == nil && result != windows.WAIT_OBJECT_0 {
				waitErr = fmt.Errorf("ConPTY child did not exit: %d", result)
			}
			waitErr = errors.Join(waitErr, windows.CloseHandle(process.Process), windows.CloseHandle(process.Thread))
		}
		return errors.Join(waitErr, inputWrite.Close(), outputRead.Close())
	}
	s.resize = func(width, height int) error {
		return windows.ResizePseudoConsole(console, windows.Coord{X: int16(width), Y: int16(height)})
	}
	watchSession(t, s)
	// These ends are no longer needed after CreateProcess; also close on any
	// failure before that point. ConPTY retains its own references.
	defer func() {
		if err := errors.Join(inputRead.Close(), outputWrite.Close()); err != nil {
			t.Error(err)
		}
	}()
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		t.Fatal(err)
	}
	defer attributes.Delete()
	// This attribute takes HPCON itself as lpValue, unlike the other attributes
	// which take a pointer to a value. Pass it as uintptr at the syscall boundary
	// rather than fabricate a Go unsafe.Pointer from a Windows handle.
	update := windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
	ok, _, callErr := update.Call(uintptr(unsafe.Pointer(attributes.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(console), unsafe.Sizeof(console), 0, 0)
	if ok == 0 {
		t.Fatal(callErr)
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// Explicit null standard handles let ConPTY initialize console handles.
	// Omitting this flag inherits a CI parent's redirected pipes, even though
	// the child has attached to the new console. Microsoft/node-pty uses this
	// same STARTF_USESTDHANDLES + null-handle combination in PtyConnect.
	startup.Flags = windows.STARTF_USESTDHANDLES
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{os.Args[0], "-test.run=^TestTerminalChild$"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CreateProcess(nil, command, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT, nil, nil, &startup.StartupInfo, &process); err != nil {
		t.Fatal(err)
	}
	return s
}
