//go:build !windows

package terminal

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func prepare(input, _ *os.File) (func() error, error) {
	fd := int(input.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return func() error { return term.Restore(fd, state) }, nil
}

func nextByte(ctx context.Context, input *os.File, milliseconds int) (byte, bool, error) {
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		poll := []unix.PollFd{{Fd: int32(input.Fd()), Events: unix.POLLIN}}
		count, err := unix.Poll(poll, milliseconds)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil || count == 0 {
			return 0, false, err
		}
		if poll[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return 0, false, io.EOF
		}
		var data [1]byte
		n, err := unix.Read(int(input.Fd()), data[:])
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		if n == 0 {
			return 0, false, io.EOF
		}
		return data[0], true, nil
	}
}

func readKey(ctx context.Context, input *os.File) (key, error) {
	first, present, err := nextByte(ctx, input, 100)
	if err != nil || !present {
		return keyNone, err
	}
	if first != 27 {
		return characterKey(rune(first)), nil
	}
	second, present, err := nextByte(ctx, input, 75)
	if err != nil {
		return keyNone, err
	}
	if !present {
		return keyBack, nil
	}
	if second != '[' && second != 'O' {
		return keyNone, nil
	}
	// Bound and consume a CSI sequence so unsupported keys/pasted escapes cannot
	// turn their trailing bytes into a selection or approval key.
	for i := 0; i < 16; i++ {
		last, present, err := nextByte(ctx, input, 75)
		if err != nil || !present {
			return keyNone, err
		}
		if last >= 0x40 && last <= 0x7e {
			switch last {
			case 'A':
				return keyUp, nil
			case 'B':
				return keyDown, nil
			default:
				return keyNone, nil
			}
		}
	}
	return keyNone, errors.New("terminal sent an overlong escape sequence")
}
