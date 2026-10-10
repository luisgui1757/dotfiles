//go:build windows

package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Compiler inspection can spawn a persistent telemetry child. Contain only this
// read-only query in a job before it executes; never apply this cancellation
// policy to the durable mutation worker or to pre-existing compiler consumers.
func queryWindowsBuildTools(ctx context.Context, privileged bool, program string, input []byte, arguments ...string) ([]byte, error) {
	if privileged || !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return nil, errors.New("Build Tools inspection cannot elevate or use a relative executable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(program)
	if err != nil {
		return nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{program}, arguments...)))
	if err != nil {
		return nil, err
	}
	// Only these three pipe handles may cross CreateProcess. In particular the
	// unnamed job handle must remain owned by this supervisor, not its children.
	var pipes [3][2]*os.File
	defer func() {
		for _, pair := range pipes {
			for _, file := range pair {
				if file != nil {
					file.Close()
				}
			}
		}
	}()
	for i := range pipes {
		pipes[i][0], pipes[i][1], err = os.Pipe()
		if err != nil {
			return nil, err
		}
	}
	childFiles := []*os.File{pipes[0][0], pipes[1][1], pipes[2][1]}
	handles := make([]windows.Handle, len(childFiles))
	for i, file := range childFiles {
		handles[i] = windows.Handle(file.Fd())
		if err := windows.SetHandleInformation(handles[i], windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			return nil, err
		}
	}
	attributes, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	if err := attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	// WinBase.h: ProcThreadAttributeValue(13, FALSE, TRUE, FALSE). Job-list
	// assignment is atomic with creation on Windows 10 / Server 2016+, also
	// Go's minimum Windows baseline. A supervisor crash cannot strand a query
	// between creation and assignment. The handle is not inherited.
	const procThreadAttributeJobList = 0x0002000d
	if err := attributes.Update(procThreadAttributeJobList, unsafe.Pointer(&job), unsafe.Sizeof(job)); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES,
		StdInput: handles[0], StdOutput: handles[1], StdErr: handles[2],
	}, ProcThreadAttributeList: attributes.List()}
	var process windows.ProcessInformation
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW | windows.EXTENDED_STARTUPINFO_PRESENT)
	if err := windows.CreateProcess(application, commandLine, nil, nil, true, flags, nil, nil, &startup.StartupInfo, &process); err != nil {
		return nil, err
	}
	defer windows.CloseHandle(process.Process)
	defer windows.CloseHandle(process.Thread)
	for _, file := range childFiles {
		file.Close()
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		return nil, errors.Join(err, stopBuildToolsQueryJob(job))
	}
	output, diagnostic := boundedCommandOutput{limit: 1 << 20}, boundedCommandOutput{}
	completed := make(chan error, 3)
	go func() {
		_, err := io.Copy(pipes[0][1], bytes.NewReader(input))
		completed <- errors.Join(err, pipes[0][1].Close())
	}()
	go func() { _, err := io.Copy(&output, pipes[1][0]); completed <- err }()
	go func() { _, err := io.Copy(&diagnostic, pipes[2][0]); completed <- err }()
	remaining := 3
	var queryErr error
	for queryErr == nil {
		select {
		case <-ctx.Done():
			queryErr = ctx.Err()
		case err := <-completed:
			remaining--
			queryErr = err
		default:
		}
		if queryErr != nil {
			break
		}
		status, err := windows.WaitForSingleObject(process.Process, 25)
		if err != nil {
			queryErr = err
			break
		}
		if status == windows.WAIT_OBJECT_0 {
			var code uint32
			queryErr = windows.GetExitCodeProcess(process.Process, &code)
			if code != 0 {
				exitCode := int(code)
				queryErr = errors.Join(queryErr, &nativeCommandError{Message: fmt.Sprintf("exit status %d", code), ExitCode: &exitCode})
			}
			break
		}
	}
	// The root may exit while vctip still owns a pipe. Reap the job before
	// waiting for pipe EOF; do not confuse root exit with process-tree exit.
	cleanupErr := stopBuildToolsQueryJob(job)
	if cleanupErr != nil {
		for _, pair := range pipes {
			for _, file := range pair {
				file.Close()
			}
		}
	}
	for ; remaining > 0; remaining-- {
		queryErr = errors.Join(queryErr, <-completed)
	}
	if err := errors.Join(queryErr, cleanupErr); err != nil {
		return nil, fmt.Errorf("Build Tools inspection failed: %w%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), nil
}

// TerminateJobObject initiates termination. Confirm every owned process has
// exited before a fresh inventory can observe it; cancellation gets a separate
// five-second reap bound. Never wait for or terminate an unrelated process.
func stopBuildToolsQueryJob(job windows.Handle) error {
	if err := windows.TerminateJobObject(job, 1); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, including its four LARGE_INTEGERs.
		var accounting struct {
			Times      [4]int64
			PageFaults uint32
			Total      uint32
			Active     uint32
			Terminated uint32
		}
		if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
			return err
		}
		if accounting.Active == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("Build Tools inspection process tree did not terminate within five seconds")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
