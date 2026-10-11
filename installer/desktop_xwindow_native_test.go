package installer

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This real X client is fixture-only, enabled solely in a disposable Xvfb
// session. Python's standard ctypes calls the existing libX11 runtime; neither
// the installer nor its acceptance probe gains another runtime dependency.
const desktopXClient = `import ctypes as c, os, sys
x=c.CDLL('libX11.so.6')
x.XOpenDisplay.restype=c.c_void_p
x.XDefaultRootWindow.argtypes=[c.c_void_p]; x.XDefaultRootWindow.restype=c.c_ulong
x.XCreateSimpleWindow.argtypes=[c.c_void_p,c.c_ulong,c.c_int,c.c_int,c.c_uint,c.c_uint,c.c_uint,c.c_ulong,c.c_ulong]; x.XCreateSimpleWindow.restype=c.c_ulong
x.XInternAtom.argtypes=[c.c_void_p,c.c_char_p,c.c_int]; x.XInternAtom.restype=c.c_ulong
x.XChangeProperty.argtypes=[c.c_void_p,c.c_ulong,c.c_ulong,c.c_ulong,c.c_int,c.c_int,c.c_void_p,c.c_int]
for name in ['XMapWindow','XUnmapWindow','XDestroyWindow']: getattr(x,name).argtypes=[c.c_void_p,c.c_ulong]
x.XSync.argtypes=[c.c_void_p,c.c_int]; x.XCloseDisplay.argtypes=[c.c_void_p]
d=x.XOpenDisplay(None)
if not d: raise RuntimeError('cannot open disposable display')
r=x.XDefaultRootWindow(d)
w=x.XCreateSimpleWindow(d,r,0,0,100,100,0,0,0)
pid=c.c_ulong(os.getpid())
x.XChangeProperty(d,w,x.XInternAtom(d,b'_NET_WM_PID',0),6,32,0,c.byref(pid),1)
x.XMapWindow(d,w); x.XSync(d,0)
print(hex(w),flush=True)
for line in sys.stdin:
 action=line.strip()
 if action=='destroy': x.XDestroyWindow(d,w)
 elif action=='unmap': x.XUnmapWindow(d,w)
 elif action=='map': x.XMapWindow(d,w)
 else: raise RuntimeError('unknown fixture command')
 x.XSync(d,0); print('done',flush=True)
x.XCloseDisplay(d)
`

func TestNativeDesktopXWindowBoundary(t *testing.T) {
	if os.Getenv("DOTFILES_NATIVE_XWINDOW") != "1" || runtime.GOOS != "linux" {
		t.Skip("requires an explicitly disposable Linux Xvfb session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "/usr/bin/python3", "-c", desktopXClient)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr boundedCommandOutput
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			t.Errorf("X client: %v: %s", err, stderr.data.String())
		}
	})
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() {
		t.Fatal("missing X window identity", scanner.Err())
	}
	window := scanner.Text()
	if _, err := strconv.ParseUint(strings.TrimPrefix(window, "0x"), 16, 32); err != nil {
		t.Fatal(err)
	}
	change := func(action string) {
		t.Helper()
		if _, err := io.WriteString(input, action+"\n"); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() || scanner.Text() != "done" {
			t.Fatal("X client acknowledgement", scanner.Err())
		}
	}
	check := func(ids []int, want bool) {
		t.Helper()
		got, err := desktopOwnedWindow(ctx, "", ids)
		if err != nil || got != want {
			t.Fatal(got, err)
		}
	}
	check([]int{command.Process.Pid}, true)
	check([]int{-1}, false)
	change("unmap")
	check([]int{command.Process.Pid}, false)
	change("map")
	// Destroy the actual server window after the successful tree reply but before
	// the actual xprop request: the native BadWindow must be proved absent.
	destroyed := false
	query := func(ctx context.Context, program string, args ...string) ([]byte, []byte, error) {
		if program == "/usr/bin/xprop" && len(args) > 1 && args[1] == window && !destroyed {
			change("destroy")
			destroyed = true
		}
		return desktopXCommand(ctx, program, args...)
	}
	if got, err := desktopLinuxOwnedWindow(ctx, []int{command.Process.Pid}, query); got || err != nil || !destroyed {
		t.Fatal(got, err, destroyed)
	}
	t.Log("native mapped owned PID proved; outside/unmapped excluded; vanished child proved against live root")
	t.Setenv("DISPLAY", ":65530")
	if got, err := desktopLinuxOwnedWindow(ctx, []int{command.Process.Pid}, desktopXCommand); got || err == nil {
		t.Fatal("real unavailable display accepted", got, err)
	}
	t.Log("native display failure remains fatal")
}

func TestDesktopXQueryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if visible, err := desktopLinuxOwnedWindow(ctx, []int{42}, desktopXCommand); visible || !errors.Is(err, context.Canceled) {
		t.Fatal(visible, err)
	}
}
