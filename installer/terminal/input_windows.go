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

func prepare(input, output *os.File) (func() error, error) {
	in, out := windows.Handle(input.Fd()), windows.Handle(output.Fd())
	var beforeIn, beforeOut uint32
	if err := windows.GetConsoleMode(in, &beforeIn); err != nil {
		return nil, err
	}
	if err := windows.GetConsoleMode(out, &beforeOut); err != nil {
		return nil, err
	}
	// Read native key records, including under ConPTY, without a background
	// stdin reader. Disable QuickEdit so a mouse selection cannot freeze setup.
	mode := beforeIn &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_QUICK_EDIT_MODE | windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	mode |= windows.ENABLE_EXTENDED_FLAGS | windows.ENABLE_WINDOW_INPUT
	if err := windows.SetConsoleMode(in, mode); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(out, beforeOut|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, errors.Join(err, windows.SetConsoleMode(in, beforeIn))
	}
	return func() error {
		return errors.Join(windows.SetConsoleMode(in, beforeIn), windows.SetConsoleMode(out, beforeOut))
	}, nil
}

func readKey(ctx context.Context, input *os.File) (key, error) {
	if err := ctx.Err(); err != nil {
		return keyNone, err
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
		return keyBack, nil
	default:
		return characterKey(rune(record.Character)), nil
	}
}
