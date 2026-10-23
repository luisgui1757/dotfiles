package terminal

import (
	"context"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var readConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

type inputRecord struct {
	EventType    uint16
	Padding      uint16
	KeyDown      int32
	RepeatCount  uint16
	VirtualKey   uint16
	VirtualScan  uint16
	Character    uint16
	ControlState uint32
}

// The whole screen session needs virtual terminal output for its alternate
// screen; raw key input is limited to each reading screen.
func prepareOutput(output *os.File) (func() error, error) {
	out := windows.Handle(output.Fd())
	var before uint32
	if err := windows.GetConsoleMode(out, &before); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(out, before|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, err
	}
	return func() error { return windows.SetConsoleMode(out, before) }, nil
}

func prepareInput(input *os.File) (func() error, error) {
	in := windows.Handle(input.Fd())
	var before uint32
	if err := windows.GetConsoleMode(in, &before); err != nil {
		return nil, err
	}
	// Read native key records, including under ConPTY, without a background
	// stdin reader. Disable QuickEdit so a mouse selection cannot freeze setup.
	mode := before &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_QUICK_EDIT_MODE | windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	mode |= windows.ENABLE_EXTENDED_FLAGS | windows.ENABLE_WINDOW_INPUT
	if err := windows.SetConsoleMode(in, mode); err != nil {
		return nil, err
	}
	return func() error { return windows.SetConsoleMode(in, before) }, nil
}

// discardPending drops key records typed before the current screen appeared.
func discardPending(_ context.Context, input *os.File) error {
	delete(heldEscape, input.Fd())
	return windows.FlushConsoleInputBuffer(windows.Handle(input.Fd()))
}

// ConPTY reads Escape followed quickly by Escape as one Alt+Escape event, the
// VT convention for Alt. Our screens give Alt+Escape no meaning: it is two Back
// presses, held as on POSIX, so a quick double Escape is never lost.
var heldEscape = map[uintptr]bool{}

func readKey(ctx context.Context, input *os.File) (key, error) {
	if err := ctx.Err(); err != nil {
		return keyNone, err
	}
	if heldEscape[input.Fd()] {
		delete(heldEscape, input.Fd())
		return keyBack, nil
	}
	result, err := windows.WaitForSingleObject(windows.Handle(input.Fd()), 100)
	if err != nil {
		return keyNone, err
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		return keyNone, nil
	}
	if result != windows.WAIT_OBJECT_0 {
		return keyNone, errors.New("console input is unavailable")
	}
	var record inputRecord
	var count uint32
	ok, _, callErr := readConsoleInput.Call(input.Fd(), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		return keyNone, callErr
	}
	if count == 0 {
		return keyNone, nil
	}
	if record.EventType == windows.WINDOW_BUFFER_SIZE_EVENT {
		return keyResize, nil
	}
	if record.EventType != windows.KEY_EVENT || record.KeyDown == 0 {
		return keyNone, nil
	}
	switch record.VirtualKey {
	case 0x26:
		return keyUp, nil
	case 0x28:
		return keyDown, nil
	case 0x1b:
		if record.ControlState&(windows.LEFT_ALT_PRESSED|windows.RIGHT_ALT_PRESSED) != 0 {
			heldEscape[input.Fd()] = true
		}
		return keyBack, nil
	default:
		return characterKey(rune(record.Character)), nil
	}
}
