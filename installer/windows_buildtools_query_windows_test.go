//go:build windows

package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const buildToolsQueryFixtureArgument = "--buildtools-query-fixture"

func buildToolsQueryFixtureArgs(mode, directory string) []string {
	return []string{"-test.run=^TestWindowsBuildToolsQueryHelper$", "--", buildToolsQueryFixtureArgument, mode, directory}
}

func TestWindowsBuildToolsQueryContainsOnlyItsOwnProcessTree(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "compiler query ü % literal")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(root, "query helper.exe")
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, data, 0700); err != nil {
		t.Fatal(err)
	}
	outside := exec.Command(program, buildToolsQueryFixtureArgs("hold", root)...)
	if err := outside.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := outside.Process.Kill(); err != nil {
			t.Error("unrelated fixture process was not preserved", err)
		}
		outside.Wait()
	})
	outsideHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(outside.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(outsideHandle)
	for _, mode := range []string{"success", "failure", "cancel", "stdout-bound", "stderr-bound"} {
		t.Run(mode, func(t *testing.T) {
			directory := filepath.Join(root, mode)
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			input := bytes.Repeat([]byte("Unicode α ü percent % and spaces\n"), 4096)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			type result struct {
				output []byte
				err    error
			}
			done := make(chan result, 1)
			collected := false
			defer func() {
				cancel()
				if !collected {
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						t.Error("failed fixture inspection did not finish cleanup")
					}
				}
			}()
			go func() {
				output, err := queryWindowsBuildTools(ctx, false, program, input, buildToolsQueryFixtureArgs(mode, directory)...)
				done <- result{output, err}
			}()
			pidFile := filepath.Join(directory, "child.pid")
			if err := waitBuildToolsQueryFixtureFile(ctx, pidFile); err != nil {
				t.Fatal(err)
			}
			pidText, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.ParseUint(string(pidText), 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			// Hold the exact child handle before allowing its parent to exit. The
			// regression cannot confuse a recycled PID with this owned process.
			child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(child)
			defer func() {
				if status, _ := windows.WaitForSingleObject(child, 0); status == uint32(windows.WAIT_TIMEOUT) {
					// Keep the failed-before fixture disposable too.
					windows.TerminateProcess(child, 1)
					windows.WaitForSingleObject(child, 5000)
				}
			}()
			if mode == "cancel" {
				cancel()
			} else if err := os.WriteFile(filepath.Join(directory, "release"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			var got result
			select {
			case got = <-done:
				collected = true
			case <-time.After(10 * time.Second):
				t.Fatal("inspection did not reap its child and inherited output pipes")
			}
			switch mode {
			case "success":
				if got.err != nil || !bytes.Equal(got.output, input) {
					t.Fatalf("stdin/stdout or Unicode path changed: %d bytes: %v", len(got.output), got.err)
				}
			case "failure":
				var nativeErr *nativeCommandError
				if !errors.As(got.err, &nativeErr) || nativeErr.ExitCode == nil || *nativeErr.ExitCode != 23 || !strings.Contains(got.err.Error(), "fixture stderr diagnostic") {
					t.Fatalf("native exit status or stderr was lost: %v", got.err)
				}
			case "cancel":
				if !errors.Is(got.err, context.Canceled) {
					t.Fatalf("cancellation was lost: %v", got.err)
				}
			default:
				if got.err == nil || !strings.Contains(got.err.Error(), "output exceeds 1048576 bytes") {
					t.Fatalf("inspection output was not bounded: %v", got.err)
				}
			}
			if status, err := windows.WaitForSingleObject(child, 0); err != nil || status != windows.WAIT_OBJECT_0 {
				// Preserve the immediate quiescence failure. The bounded diagnostic
				// distinguishes final process-object rundown from an uncontained
				// child without changing the assertion or terminating another PID.
				var before, after uint32
				beforeErr := windows.GetExitCodeProcess(child, &before)
				started := time.Now()
				later, waitErr := windows.WaitForSingleObject(child, 5000)
				afterErr := windows.GetExitCodeProcess(child, &after)
				t.Fatalf("inspection returned with its child still running: initial wait=%d error=%v exit=%d error=%v; diagnostic wait=%d error=%v elapsed=%s exit=%d error=%v", status, err, before, beforeErr, later, waitErr, time.Since(started), after, afterErr)
			}
			if status, err := windows.WaitForSingleObject(outsideHandle, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
				t.Fatal("inspection affected an unrelated process", status, err)
			}
		})
	}
}

func TestWindowsBuildToolsQueryRejectsUnsafeOrUnavailableProgram(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		program    string
		privileged bool
	}{{program, true}, {"relative.exe", false}, {filepath.Join(t.TempDir(), "absent.exe"), false}} {
		if _, err := queryWindowsBuildTools(context.Background(), sample.privileged, sample.program, nil); err == nil {
			t.Fatal("unsafe or unavailable inspection executable accepted", sample)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryWindowsBuildTools(ctx, false, program, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("already canceled inspection was started", err)
	}
}

func waitBuildToolsQueryFixtureFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWindowsBuildToolsQueryHelper(t *testing.T) {
	index := -1
	for i, value := range os.Args {
		if value == buildToolsQueryFixtureArgument {
			index = i
		}
	}
	if index < 0 {
		return
	}
	if len(os.Args) != index+3 {
		t.Fatal("fixture arguments changed", os.Args)
	}
	mode, directory := os.Args[index+1], os.Args[index+2]
	if mode == "hold" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if mode == "exit-on-release" {
		if err := os.WriteFile(filepath.Join(directory, "ready"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := waitBuildToolsQueryFixtureFile(ctx, filepath.Join(directory, "release")); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(program, buildToolsQueryFixtureArgs("hold", directory)...)
	// Deliberately inherit output handles: waiting only for the root or for
	// pipe EOF would hang even after this parent exits.
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "child.pid"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitBuildToolsQueryFixtureFile(ctx, filepath.Join(directory, "release")); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "success":
		if _, err := os.Stdout.Write(input); err != nil {
			t.Fatal(err)
		}
	case "failure":
		fmt.Fprint(os.Stderr, "fixture stderr diagnostic")
		os.Exit(23)
	case "stdout-bound", "stderr-bound":
		file := os.Stdout
		if mode == "stderr-bound" {
			file = os.Stderr
		}
		file.Write(bytes.Repeat([]byte("x"), (1<<20)+1))
	default:
		t.Fatal("unsupported fixture mode", mode)
	}
	os.Exit(0)
}

// A child created after the captured membership cannot be silently covered by
// the earlier HANDLE proof. Exercise the real job boundary, without test hooks.
func TestWindowsBuildToolsQueryRejectsChangedMembership(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(windows.TerminateJobObject(job, 1), windows.CloseHandle(job)); err != nil {
			t.Error(err)
		}
	})
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	application, err := windows.UTF16PtrFromString(program)
	if err != nil {
		t.Fatal(err)
	}
	commandLine := windows.ComposeCommandLine(append([]string{program}, buildToolsQueryFixtureArgs("hold", t.TempDir())...))
	start := func() windows.Handle {
		t.Helper()
		// CreateProcess may modify its command-line buffer.
		arguments, err := windows.UTF16PtrFromString(commandLine)
		if err != nil {
			t.Fatal(err)
		}
		startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
		var process windows.ProcessInformation
		if err := windows.CreateProcess(application, arguments, nil, nil, false, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW, nil, nil, &startup, &process); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if status, err := windows.WaitForSingleObject(process.Process, 0); err != nil {
				t.Error(err)
			} else if status == uint32(windows.WAIT_TIMEOUT) {
				if err := windows.TerminateProcess(process.Process, 1); err != nil {
					t.Error(err)
				}
				if status, err := windows.WaitForSingleObject(process.Process, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Error("fixture child cleanup failed", status, err)
				}
			}
			if err := errors.Join(windows.CloseHandle(process.Thread), windows.CloseHandle(process.Process)); err != nil {
				t.Error(err)
			}
		})
		if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
			t.Fatal(err)
		}
		if _, err := windows.ResumeThread(process.Thread); err != nil {
			t.Fatal(err)
		}
		return process.Process
	}
	first := start()
	handles, total, err := captureBuildToolsQueryProcesses(job)
	defer func() {
		for _, handle := range handles {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		}
	}()
	if err != nil || len(handles) != 1 || total != 1 {
		t.Fatal("initial exact membership capture failed", len(handles), total, err)
	}
	second := start()
	err = terminateBuildToolsQueryProcesses(job, handles, total, time.Now().Add(5*time.Second))
	if err == nil || !strings.Contains(err.Error(), "membership changed during termination") {
		t.Fatal("late child was accepted under an earlier HANDLE proof", err)
	}
	for _, handle := range []windows.Handle{first, second} {
		if status, err := windows.WaitForSingleObject(handle, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal("private job did not terminate its exact member", status, err)
		}
	}
}

// Force a real private member to exit between the two independent kernel
// snapshots. Its earlier Active count cannot invalidate the later complete list.
func TestWindowsBuildToolsQueryAcceptsExitedMemberDuringCapture(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(windows.TerminateJobObject(job, 1), windows.CloseHandle(job)); err != nil {
			t.Error(err)
		}
	})
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	application, err := windows.UTF16PtrFromString(program)
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{program}, buildToolsQueryFixtureArgs("exit-on-release", directory)...)))
	if err != nil {
		t.Fatal(err)
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var process windows.ProcessInformation
	if err := windows.CreateProcess(application, arguments, nil, nil, false, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW, nil, nil, &startup, &process); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if status, err := windows.WaitForSingleObject(process.Process, 0); err != nil {
			t.Error(err)
		} else if status == uint32(windows.WAIT_TIMEOUT) {
			if err := windows.TerminateProcess(process.Process, 1); err != nil {
				t.Error(err)
			}
			if status, err := windows.WaitForSingleObject(process.Process, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
				t.Error("fixture child cleanup failed", status, err)
			}
		}
		if err := errors.Join(windows.CloseHandle(process.Thread), windows.CloseHandle(process.Process)); err != nil {
			t.Error(err)
		}
	})
	if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		t.Fatal(err)
	}
	// Startup may include other private members. Hold every ready member's
	// exact HANDLE, then force only this root to exit between the snapshots.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := waitBuildToolsQueryFixtureFile(ctx, filepath.Join(directory, "ready")); err != nil {
		t.Fatal("private fixture did not become ready", err)
	}
	before, err := readBuildToolsJobAccounting(job)
	if err != nil || before.Active == 0 || before.Total == 0 {
		t.Fatal("expected ready private members before capture", before, err)
	}
	ready, total, err := captureBuildToolsQueryProcessesAfterAccounting(job, before)
	defer func() {
		for _, handle := range ready {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		}
	}()
	if err != nil || total != before.Total {
		t.Fatal("ready private member capture failed", err)
	}
	foundRoot := false
	for _, handle := range ready {
		pid, err := windows.GetProcessId(handle)
		if err != nil {
			t.Fatal(err)
		}
		foundRoot = foundRoot || pid == process.ProcessId
		image := make([]uint16, 32768)
		size := uint32(len(image))
		if err := windows.QueryFullProcessImageName(handle, 0, &image[0], &size); err != nil {
			t.Fatal(err)
		}
		// These handles already have proved job membership. The image is a
		// diagnostic, never process selection or termination authority.
		t.Logf("ready private member pid=%d image=%s root=%t", pid, filepath.Base(windows.UTF16ToString(image[:size])), pid == process.ProcessId)
	}
	if !foundRoot {
		t.Fatal("ready capture omitted the exact private root")
	}
	if err := os.WriteFile(filepath.Join(directory, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if status, err := windows.WaitForSingleObject(process.Process, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("private fixture did not exit naturally", status, err)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(process.Process, &exitCode); err != nil || exitCode != 0 {
		t.Fatal("private fixture did not complete its ordinary exit", exitCode, err)
	}
	after, err := readBuildToolsJobAccounting(job)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("natural exit accounting: before active=%d total=%d; after active=%d total=%d; exact HANDLE signaled", before.Active, before.Total, after.Active, after.Total)
	if after.Total != before.Total || after.Active >= before.Active {
		t.Fatal("natural exit did not preserve lifetime membership and reduce active members")
	}
	handles, total, err := captureBuildToolsQueryProcessesAfterAccounting(job, before)
	defer func() {
		for _, handle := range handles {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		}
	}()
	if err != nil || total != before.Total {
		t.Fatal("natural exit invalidated a complete private member list", len(handles), total, err)
	}
	if err := terminateBuildToolsQueryProcesses(job, handles, total, time.Now().Add(5*time.Second)); err != nil {
		t.Fatal("completed private query rejected after natural exit", err)
	}
	for _, handle := range ready {
		if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal("ready private member survived exact completion", status, err)
		}
	}
}
