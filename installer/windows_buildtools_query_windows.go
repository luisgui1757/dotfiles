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
	if err := setBuildToolsQueryJobLimits(job); err != nil {
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
// await those handles within one shared reap bound. Newly added membership cannot
// authorize a success claim from an incomplete snapshot; natural exits can.
func stopBuildToolsQueryJob(job windows.Handle) (result error) {
	deadline := time.Now().Add(5 * time.Second)
	handles, capture, captureErr := captureBuildToolsQueryProcesses(job, deadline)
	defer func() {
		for _, handle := range handles {
			result = errors.Join(result, windows.CloseHandle(handle))
		}
	}()
	return errors.Join(captureErr, terminateBuildToolsQueryProcesses(job, handles, capture, deadline))
}

type buildToolsJobAccounting struct {
	Times      [4]int64
	PageFaults uint32
	Total      uint32
	Active     uint32
	Terminated uint32
}

var (
	buildToolsGetJobInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("QueryInformationJobObject")
	buildToolsSetJobInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetInformationJobObject")
)

// x/sys's generated QueryInformationJobObject wrapper takes uintptr without
// uintptrescapes. Keep a typed pointer until LazyProc.Call's annotated boundary,
// so Go retains and does not move the output buffer while the kernel writes it.
func queryBuildToolsJobInformation(job windows.Handle, class int32, data unsafe.Pointer, size uint32) error {
	ok, _, err := buildToolsGetJobInformation.Call(uintptr(job), uintptr(class), uintptr(data), uintptr(size), 0)
	if ok == 0 {
		return err
	}
	return nil
}

// Keep the fixed query-job limit buffer at the same annotated syscall boundary.
func setBuildToolsQueryJobLimits(job windows.Handle) error {
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	ok, _, err := buildToolsSetJobInformation.Call(uintptr(job), uintptr(windows.JobObjectExtendedLimitInformation), uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		return err
	}
	return nil
}

func readBuildToolsJobAccounting(job windows.Handle) (buildToolsJobAccounting, error) {
	var info buildToolsJobAccounting
	err := queryBuildToolsJobInformation(job, windows.JobObjectBasicAccountingInformation, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)))
	return info, err
}

type buildToolsJobMembers struct {
	Assigned uint32
	Listed   uint32
	PIDs     [256]uintptr
}

// Keep both accounting snapshots for failure evidence only. The original
// baseline still controls the lifetime-membership guard.
type buildToolsQueryCapture struct {
	Before, After buildToolsJobAccounting
	AfterError    error
}

// Query only the private job, never a machine-wide PID/name inventory. The fixed
// buffer bounds this compiler inspection; overflow is an error, not truncation.
func captureBuildToolsQueryProcesses(job windows.Handle, deadline time.Time) ([]windows.Handle, buildToolsQueryCapture, error) {
	before, err := readBuildToolsJobAccounting(job)
	if err != nil {
		return nil, buildToolsQueryCapture{Before: before}, err
	}
	return captureBuildToolsQueryProcessesAfterAccounting(job, before, deadline)
}

// Accounting and the PID list are separate snapshots. An earlier Active count
// may include a member that has since exited; only the list's own counts prove
// completeness. The lifetime Total is checked again after exact HANDLE waits.
func captureBuildToolsQueryProcessesAfterAccounting(job windows.Handle, before buildToolsJobAccounting, deadline time.Time) ([]windows.Handle, buildToolsQueryCapture, error) {
	capture := buildToolsQueryCapture{Before: before}
	members, err := completeBuildToolsQueryMembers(deadline, func(members *buildToolsJobMembers) error {
		return queryBuildToolsJobInformation(job, windows.JobObjectBasicProcessIdList, unsafe.Pointer(members), uint32(unsafe.Sizeof(*members)))
	})
	if err != nil {
		return nil, capture, err
	}
	handles := make([]windows.Handle, 0, members.Listed)
	isProcessInJob := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	for _, pid := range members.PIDs[:members.Listed] {
		if pid == 0 || pid > 1<<32-1 {
			return handles, capture, errors.New("invalid Build Tools inspection member PID")
		}
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return handles, capture, fmt.Errorf("open Build Tools inspection member: %w", err)
		}
		var belongs int32
		ok, _, callErr := isProcessInJob.Call(uintptr(handle), uintptr(job), uintptr(unsafe.Pointer(&belongs)))
		if ok == 0 || belongs == 0 {
			closeErr := windows.CloseHandle(handle)
			if ok == 0 {
				return handles, capture, errors.Join(fmt.Errorf("verify Build Tools inspection membership: %w", callErr), closeErr)
			}
			return handles, capture, errors.Join(errors.New("Build Tools inspection member changed before capture"), closeErr)
		}
		handles = append(handles, handle)
	}
	capture.After, capture.AfterError = readBuildToolsJobAccounting(job)
	return handles, capture, nil
}

// Windows can return a successful but short PID list while members exit, even
// with ample buffer capacity. Only accept a complete snapshot. Re-query that
// narrow state within the original shared reap deadline; never rebase the
// lifetime accounting or retry API, capacity, PID or HANDLE failures.
func completeBuildToolsQueryMembers(deadline time.Time, query func(*buildToolsJobMembers) error) (buildToolsJobMembers, error) {
	var members buildToolsJobMembers
	for attempts := 0; ; attempts++ {
		if !time.Now().Before(deadline) {
			return members, fmt.Errorf("Build Tools inspection membership capture did not complete within five seconds; attempts=%d assigned=%d listed=%d", attempts, members.Assigned, members.Listed)
		}
		members = buildToolsJobMembers{}
		if err := query(&members); err != nil {
			return members, fmt.Errorf("capture Build Tools inspection members: %w", err)
		}
		if members.Assigned > uint32(len(members.PIDs)) || members.Listed > members.Assigned {
			return members, fmt.Errorf("invalid Build Tools inspection membership counts; assigned=%d listed=%d capacity=%d", members.Assigned, members.Listed, len(members.PIDs))
		}
		if members.Listed == members.Assigned && time.Now().Before(deadline) {
			return members, nil
		}
		time.Sleep(min(time.Millisecond, max(0, time.Until(deadline))))
	}
}

func terminateBuildToolsQueryProcesses(job windows.Handle, handles []windows.Handle, capture buildToolsQueryCapture, deadline time.Time) error {
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
	if after.Total != capture.Before.Total || after.Active != 0 {
		return fmt.Errorf("Build Tools inspection membership changed during termination; completion is unproved; %s", buildToolsQueryMembershipDiagnostic(job, capture, after, len(handles)))
	}
	return nil
}

// Failure-only evidence from the private job. Keep the refusal authoritative even
// if this later snapshot is unavailable or incomplete. No paths or environment
// values are queried, and at most the existing 256-member bound can be printed.
func buildToolsQueryMembershipDiagnostic(job windows.Handle, capture buildToolsQueryCapture, after buildToolsJobAccounting, captured int) string {
	before := capture.Before
	detail := fmt.Sprintf("baseline(total=%d active=%d terminated=%d) final(total=%d active=%d terminated=%d) captured_handles=%d", before.Total, before.Active, before.Terminated, after.Total, after.Active, after.Terminated, captured)
	if capture.AfterError != nil {
		detail += fmt.Sprintf("; after_capture unavailable: %v", capture.AfterError)
	} else {
		detail += fmt.Sprintf("; after_capture(total=%d active=%d terminated=%d)", capture.After.Total, capture.After.Active, capture.After.Terminated)
	}
	var members buildToolsJobMembers
	if err := queryBuildToolsJobInformation(job, windows.JobObjectBasicProcessIdList, unsafe.Pointer(&members), uint32(unsafe.Sizeof(members))); err != nil {
		return fmt.Sprintf("%s; current_members unavailable: %v", detail, err)
	}
	if members.Listed > uint32(len(members.PIDs)) || members.Listed != members.Assigned {
		return fmt.Sprintf("%s; current_members assigned=%d listed=%d incomplete", detail, members.Assigned, members.Listed)
	}
	return fmt.Sprintf("%s; current_members assigned=%d listed=%d pids=%v", detail, members.Assigned, members.Listed, members.PIDs[:members.Listed])
}
