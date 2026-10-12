package installer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Query the executable vnode path, as Apple's libproc proc_pidpath does. ps
// comm retains the invocation alias and cannot establish payload ownership.
// libproc/XNU define PIDINFO=2, PIDPATHINFO=11 and MAXSIZE=4*MAXPATHLEN.
func desktopDarwinOwnedProcesses(ctx context.Context, root string) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := resolveConfigPath(root)
	if err != nil {
		return nil, err
	}
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Getuid())
	if err != nil {
		return nil, fmt.Errorf("enumerate current-user desktop processes: %w", err)
	}
	ids := []int{}
	for _, process := range processes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid := int(process.Proc.P_pid)
		var path [4096]byte
		_, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, 2, uintptr(pid), 11, 0, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
		if errno == unix.ESRCH || errno == unix.ENOENT {
			// Exited processes and removed executable vnodes have no live path.
			// Neither can grant ownership or permission to signal a process.
			continue
		}
		if errno != 0 {
			return nil, fmt.Errorf("inspect desktop process %d executable: %w", pid, errno)
		}
		end := bytes.IndexByte(path[:], 0)
		if end <= 0 || !filepath.IsAbs(string(path[:end])) {
			return nil, fmt.Errorf("invalid desktop process %d executable path", pid)
		}
		if strings.HasPrefix(string(path[:end]), root+string(filepath.Separator)) {
			ids = append(ids, pid)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func TestDesktopDarwinInventoryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := desktopOwnedProcesses(ctx, "", t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled inventory did not preserve its cause", err)
	}
}

func TestDesktopDarwinProcessFixture(t *testing.T) {
	if os.Getenv("DOTFILES_DESKTOP_PROCESS_FIXTURE") != "1" {
		return
	}
	fmt.Println("ready")
	time.Sleep(time.Minute)
}

func TestDesktopDarwinInventoryResolvesPublishedAliases(t *testing.T) {
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "physical ü payload", strings.Repeat("long-", 25), strings.Repeat("path-", 25))
	outside := root + "-outside"
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "private-process"), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	published := filepath.Join(base, "published ü application")
	if err := os.Symlink(root, published); err != nil {
		t.Fatal(err)
	}
	outsideAlias := filepath.Join(root, "outside alias")
	if err := os.Symlink(outside, outsideAlias); err != nil {
		t.Fatal(err)
	}
	owned := desktopDarwinStartProcess(t, filepath.Join(published, "private-process"))
	unrelated := desktopDarwinStartProcess(t, filepath.Join(outsideAlias, "private-process"))
	removedPath := filepath.Join(outside, "removed-process")
	if err := os.WriteFile(removedPath, data, 0700); err != nil {
		t.Fatal(err)
	}
	removed := desktopDarwinStartProcess(t, removedPath)
	if err := os.Remove(removedPath); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ids, err := desktopOwnedProcesses(ctx, "", root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids, owned.Process.Pid) {
		t.Errorf("published alias process missing: got %v; want PID %d", ids, owned.Process.Pid)
	}
	for _, process := range []*exec.Cmd{unrelated, removed} {
		if slices.Contains(ids, process.Process.Pid) {
			t.Errorf("outside executable acquired through its invocation path: PID %d", process.Process.Pid)
		}
	}
	if t.Failed() {
		return
	}
	// Exercise the existing fixture cleanup through the same authoritative
	// inventory, and prove it does not kill the outside process.
	desktopStopOwned(t, "", root)
	for _, process := range []*exec.Cmd{unrelated, removed} {
		if err := process.Process.Signal(syscall.Signal(0)); err != nil {
			t.Fatal("outside process did not survive owned cleanup", err)
		}
	}
}

func desktopDarwinStartProcess(t *testing.T, executable string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	command := exec.CommandContext(ctx, executable, "-test.run=^TestDesktopDarwinProcessFixture$")
	command.Env = append(os.Environ(), "DOTFILES_DESKTOP_PROCESS_FIXTURE=1")
	output, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected private process reply %q", line)
		}
		ready <- err
		done <- command.Wait()
	}()
	t.Cleanup(func() {
		cancel()
		err := <-done
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Error("reap private process", err)
		}
	})
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal("start private process", err)
		}
	case <-ctx.Done():
		t.Fatal("private process readiness", ctx.Err())
	}
	return command
}
