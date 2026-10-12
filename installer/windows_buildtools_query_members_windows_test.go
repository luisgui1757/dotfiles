//go:build windows

package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Exercise successful short PID lists during real natural exits. The syscall
// boundary stays native; no process creation is allowed after the baseline.
func TestWindowsBuildToolsQueryCompletesNativeExitSnapshots(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	application, err := windows.UTF16PtrFromString(program)
	if err != nil {
		t.Fatal(err)
	}
	totalMismatch := 0
	for round := range 12 {
		t.Run(fmt.Sprint(round), func(t *testing.T) {
			job, err := windows.CreateJobObject(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := errors.Join(windows.TerminateJobObject(job, 1), windows.CloseHandle(job)); err != nil {
					t.Error(err)
				}
			})
			root := t.TempDir()
			var processes []windows.ProcessInformation
			var directories []string
			for index := range 16 {
				directory := filepath.Join(root, fmt.Sprint(index))
				if err := os.Mkdir(directory, 0700); err != nil {
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
							t.Error("child cleanup", status, err)
						}
					}
					if err := errors.Join(windows.CloseHandle(process.Process), windows.CloseHandle(process.Thread)); err != nil {
						t.Error(err)
					}
				})
				if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
					t.Fatal(err)
				}
				if _, err := windows.ResumeThread(process.Thread); err != nil {
					t.Fatal(err)
				}
				processes = append(processes, process)
				directories = append(directories, directory)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			for _, directory := range directories {
				if err := waitBuildToolsQueryFixtureFile(ctx, filepath.Join(directory, "ready")); err != nil {
					t.Fatal(err)
				}
			}
			before, err := readBuildToolsJobAccounting(job)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("before release total=%d active=%d terminated=%d", before.Total, before.Active, before.Terminated)
			released := make(chan error, 1)
			go func() {
				for _, directory := range directories {
					if err := os.WriteFile(filepath.Join(directory, "release"), nil, 0600); err != nil {
						released <- err
						return
					}
				}
				released <- nil
			}()
			deadline := time.Now().Add(5 * time.Second)
			samples, mismatches := 0, 0
			for time.Now().Before(deadline) {
				var members buildToolsJobMembers
				err := queryBuildToolsJobInformation(job, windows.JobObjectBasicProcessIdList, unsafe.Pointer(&members), uint32(unsafe.Sizeof(members)))
				if err != nil {
					t.Fatal(err)
				}
				samples++
				accounting, err := readBuildToolsJobAccounting(job)
				if err != nil {
					t.Fatal(err)
				}
				if members.Listed > uint32(len(members.PIDs)) || members.Assigned != members.Listed {
					mismatches++
					complete, err := completeBuildToolsQueryMembers(deadline, func(members *buildToolsJobMembers) error {
						return queryBuildToolsJobInformation(job, windows.JobObjectBasicProcessIdList, unsafe.Pointer(members), uint32(unsafe.Sizeof(*members)))
					})
					if err != nil || complete.Assigned != complete.Listed || complete.Listed > uint32(len(complete.PIDs)) {
						t.Fatalf("natural exits did not yield a complete snapshot: initial assigned=%d listed=%d; final assigned=%d listed=%d: %v", members.Assigned, members.Listed, complete.Assigned, complete.Listed, err)
					}
				}
				if accounting.Active == 0 {
					break
				}
			}
			if err := <-released; err != nil {
				t.Fatal(err)
			}
			for _, process := range processes {
				if status, err := windows.WaitForSingleObject(process.Process, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Fatal("ordinary root exit", status, err)
				}
			}
			after, err := readBuildToolsJobAccounting(job)
			if err != nil || after.Total != before.Total {
				t.Fatal("unexpected added membership", before, after, err)
			}
			totalMismatch += mismatches
			t.Logf("samples=%d incomplete=%d after total=%d active=%d terminated=%d", samples, mismatches, after.Total, after.Active, after.Terminated)
		})
	}
	t.Logf("native incomplete snapshots recovered=%d", totalMismatch)
}
