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
			child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
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
				t.Fatal("inspection returned with its child still running", status, err)
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
