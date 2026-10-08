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

// TerminateJobObject initiates termination, but job accounting and exit codes
// can settle before process HANDLEs signal. Capture exact job members first and
// await those handles within one shared reap bound. A changing membership cannot
// authorize a success claim from an incomplete snapshot.
func stopBuildToolsQueryJob(job windows.Handle) (result error) {
	deadline := time.Now().Add(5 * time.Second)
	handles, total, captureErr := captureBuildToolsQueryProcesses(job)
	defer func() {
		for _, handle := range handles {
			result = errors.Join(result, windows.CloseHandle(handle))
		}
	}()
	return errors.Join(captureErr, terminateBuildToolsQueryProcesses(job, handles, total, deadline))
}

type buildToolsJobAccounting struct {
	Times      [4]int64
	PageFaults uint32
	Total      uint32
	Active     uint32
	Terminated uint32
}

func readBuildToolsJobAccounting(job windows.Handle) (buildToolsJobAccounting, error) {
	var info buildToolsJobAccounting
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil)
	return info, err
}

// Query only the private job, never a machine-wide PID/name inventory. The fixed
// buffer bounds this compiler inspection; overflow is an error, not truncation.
func captureBuildToolsQueryProcesses(job windows.Handle) ([]windows.Handle, uint32, error) {
	before, err := readBuildToolsJobAccounting(job)
	if err != nil {
		return nil, 0, err
	}
	var members struct {
		Assigned uint32
		Listed   uint32
		PIDs     [256]uintptr
	}
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&members)), uint32(unsafe.Sizeof(members)), nil); err != nil {
		return nil, before.Total, fmt.Errorf("capture Build Tools inspection members: %w", err)
	}
	if members.Listed > uint32(len(members.PIDs)) || members.Listed != members.Assigned || members.Assigned != before.Active {
		return nil, before.Total, errors.New("Build Tools inspection membership changed during capture")
	}
	handles := make([]windows.Handle, 0, members.Listed)
	isProcessInJob := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	for _, pid := range members.PIDs[:members.Listed] {
		if pid == 0 || pid > 1<<32-1 {
			return handles, before.Total, errors.New("invalid Build Tools inspection member PID")
		}
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return handles, before.Total, fmt.Errorf("open Build Tools inspection member: %w", err)
		}
		var belongs int32
		ok, _, callErr := isProcessInJob.Call(uintptr(handle), uintptr(job), uintptr(unsafe.Pointer(&belongs)))
		if ok == 0 || belongs == 0 {
			closeErr := windows.CloseHandle(handle)
			if ok == 0 {
				return handles, before.Total, errors.Join(fmt.Errorf("verify Build Tools inspection membership: %w", callErr), closeErr)
			}
			return handles, before.Total, errors.Join(errors.New("Build Tools inspection member changed before capture"), closeErr)
		}
		handles = append(handles, handle)
	}
	return handles, before.Total, nil
}

func terminateBuildToolsQueryProcesses(job windows.Handle, handles []windows.Handle, total uint32, deadline time.Time) error {
	if err := windows.TerminateJobObject(job, 1); err != nil {
		return err
	}
	for _, handle := range handles {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("Build Tools inspection process tree did not terminate within five seconds")
		}
		milliseconds := uint32((remaining + time.Millisecond - 1) / time.Millisecond)
		status, err := windows.WaitForSingleObject(handle, milliseconds)
		if err != nil {
			return err
		}
		if status != windows.WAIT_OBJECT_0 {
			return fmt.Errorf("Build Tools inspection process completion was not signaled within five seconds (wait %d)", status)
		}
	}
	after, err := readBuildToolsJobAccounting(job)
	if err != nil {
		return err
	}
	if after.Total != total || after.Active != 0 {
		return errors.New("Build Tools inspection membership changed during termination; completion is unproved")
	}
	return nil
}
