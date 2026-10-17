package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A CIM PID is only a candidate. Hold its process object and validate that
// object's executable before signalling it, so PID reuse cannot acquire an
// outside process between inventory and cleanup.
func desktopStopProcess(ctx context.Context, root string, id int) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id <= 0 || uint64(id) > 1<<32-1 {
		return errors.New("invalid desktop cleanup PID")
	}
	root, err := resolveConfigPath(root)
	if err != nil {
		return err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(id))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		// The enumerated process exited before a handle could be acquired.
		// The existing outer inventory must still prove the root is empty.
		return nil
	}
	if err != nil {
		return fmt.Errorf("open desktop process %d: %w", id, err)
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(handle)) }()
	var image [32768]uint16
	size := uint32(len(image))
	if err := windows.QueryFullProcessImageName(handle, 0, &image[0], &size); err != nil {
		cause := fmt.Errorf("inspect desktop process %d image: %w", id, err)
		// Image inspection can fail during or after process rundown. Its
		// error code does not prove liveness. Wait only; never signal a
		// process with unproved ownership.
		return desktopWaitWindowsProcess(ctx, handle, cause)
	}
	path, err := resolveConfigPath(windows.UTF16ToString(image[:size]))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(path), strings.ToLower(root+string(filepath.Separator))) {
		return fmt.Errorf("desktop process %d is outside the private application root", id)
	}
	if err := desktopTerminateWindowsProcess(ctx, handle); err != nil {
		return fmt.Errorf("stop desktop process %d: %w", id, err)
	}
	return nil
}

func desktopTerminateWindowsProcess(ctx context.Context, handle windows.Handle) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("desktop process cleanup requires a deadline")
	}
	terminateErr := windows.TerminateProcess(handle, 1)
	if terminateErr != nil && !errors.Is(terminateErr, windows.ERROR_ACCESS_DENIED) {
		return terminateErr
	}
	return desktopWaitWindowsProcess(ctx, handle, terminateErr)
}

// Termination can report ACCESS_DENIED and image inspection can fail while the
// exact object finishes rundown. Use only the existing shared deadline; a
// still-live/inaccessible object retains its original error and cannot pass.
func desktopWaitWindowsProcess(ctx context.Context, handle windows.Handle, cause error) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(cause, err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.Join(cause, errors.New("desktop process cleanup requires a deadline"))
	}
	milliseconds := uint32(max(0, time.Until(deadline)) / time.Millisecond)
	status, waitErr := windows.WaitForSingleObject(handle, milliseconds)
	if waitErr != nil {
		return errors.Join(cause, waitErr)
	}
	if status != windows.WAIT_OBJECT_0 {
		return errors.Join(cause, fmt.Errorf("desktop process completion unproved: wait=%d", status))
	}
	return nil
}

func TestDesktopWindowsProcessFixture(t *testing.T) {
	if os.Getenv("DOTFILES_DESKTOP_PROCESS_FIXTURE") != "1" {
		return
	}
	fmt.Println("ready")
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopWindowsStopRequiresExactCompletionAndPrivateImage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private desktop ü with spaces")
	outside := root + "-outside"
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{root, outside} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "private process.exe"), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	start := func(t *testing.T, directory string) (int, windows.Handle) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		command := exec.CommandContext(ctx, filepath.Join(directory, "private process.exe"), "-test.run=^TestDesktopWindowsProcessFixture$")
		command.Env = append(os.Environ(), "DOTFILES_DESKTOP_PROCESS_FIXTURE=1")
		input, err := command.StdinPipe()
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		output, err := command.StdoutPipe()
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			cancel()
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := input.Close(); err != nil {
				t.Error(err)
			}
			cancel()
			var exit *exec.ExitError
			if err := command.Wait(); err != nil && !errors.As(err, &exit) {
				t.Error(err)
			}
		})
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(command.Process.Pid))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			status, err := windows.WaitForSingleObject(handle, 0)
			if err == nil && status == uint32(windows.WAIT_TIMEOUT) {
				err = windows.TerminateProcess(handle, 1)
				if err == nil {
					status, err = windows.WaitForSingleObject(handle, 5000)
				}
			}
			if err != nil || status != windows.WAIT_OBJECT_0 {
				t.Error("private process fixture cleanup", status, err)
			}
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		})
		if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "ready\n" {
			t.Fatal("private process readiness", line, err)
		}
		return command.Process.Pid, handle
	}
	outsidePID, outsideHandle := start(t, outside)
	for _, name := range []string{"owned", "exited", "terminating", "live-denied", "image-denied", "outside", "canceled"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			pid, handle := start(t, root)
			switch name {
			case "owned":
				if err := desktopStopProcess(ctx, root, pid); err != nil {
					t.Fatal(err)
				}
				if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Fatal("owned cleanup returned before exact completion", status, err)
				}
			case "exited":
				if err := windows.TerminateProcess(handle, 1); err != nil {
					t.Fatal(err)
				}
				if status, err := windows.WaitForSingleObject(handle, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Fatal("fixture did not establish an exited held process", status, err)
				}
				// Reproduce the former fixture's actual os.Process.Kill boundary.
				old, err := os.FindProcess(pid)
				if err != nil {
					t.Fatal(err)
				}
				oldErr := old.Kill()
				if err := old.Release(); err != nil {
					t.Fatal(err)
				}
				if !errors.Is(oldErr, windows.ERROR_ACCESS_DENIED) || errors.Is(oldErr, os.ErrProcessDone) {
					t.Fatal("old cleanup did not reproduce documented post-exit denial", oldErr)
				}
				t.Log("old cleanup rejects exited exact process:", oldErr)
				if err := desktopTerminateWindowsProcess(ctx, handle); err != nil {
					t.Fatal("signaled held process was not accepted", err)
				}
				if err := desktopStopProcess(ctx, root, pid); err != nil {
					t.Fatal("completed exact process failed image inspection", err)
				}
			case "terminating":
				if err := windows.TerminateProcess(handle, 1); err != nil {
					t.Fatal(err)
				}
				status, err := windows.WaitForSingleObject(handle, 0)
				if err != nil {
					t.Fatal(err)
				}
				t.Log("asynchronous termination initial exact wait:", status)
				if err := desktopTerminateWindowsProcess(ctx, handle); err != nil {
					t.Fatal("asynchronously terminating object was not reaped", err)
				}
				if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Fatal("asynchronous cleanup returned before exact completion", status, err)
				}
			case "live-denied", "image-denied":
				limited, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := windows.CloseHandle(limited); err != nil {
						t.Error(err)
					}
				}()
				if name == "live-denied" {
					err = desktopTerminateWindowsProcess(ctx, limited)
				} else {
					var image [32768]uint16
					size := uint32(len(image))
					imageErr := windows.QueryFullProcessImageName(limited, 0, &image[0], &size)
					if !errors.Is(imageErr, windows.ERROR_ACCESS_DENIED) {
						t.Fatal("restricted image query did not establish native denial", imageErr)
					}
					err = desktopWaitWindowsProcess(ctx, limited, imageErr)
				}
				if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || !strings.Contains(err.Error(), "completion unproved") {
					t.Fatal("live termination denial was accepted", err)
				}
			case "outside":
				if err := desktopStopProcess(ctx, root, outsidePID); err == nil || !strings.Contains(err.Error(), "outside the private application root") {
					t.Fatal("outside executable acquired through a candidate PID", err)
				}
			case "canceled":
				cancel()
				if err := desktopStopProcess(ctx, root, pid); !errors.Is(err, context.Canceled) {
					t.Fatal("canceled cleanup lost its cause", err)
				}
			}
			if name == "live-denied" || name == "image-denied" || name == "canceled" {
				if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
					t.Fatal("unapproved live process did not survive", status, err)
				}
			}
			if status, err := windows.WaitForSingleObject(outsideHandle, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
				t.Fatal("outside process did not survive", status, err)
			}
		})
	}
}
