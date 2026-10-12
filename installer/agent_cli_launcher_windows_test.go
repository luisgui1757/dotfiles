package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Test consumers must reap their own helpers before private home removal.
// Assignment at creation avoids the child-spawn race after Cmd.Start.
func startAgentCLI(ctx context.Context, cmd *exec.Cmd) (_ *agentCLIProcess, result error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return nil, err
	}
	var directory *uint16
	if cmd.Dir != "" {
		directory, err = windows.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return nil, err
		}
	}
	// Cmd.Environ preserves Go's case-insensitive, last-value-wins behavior.
	environment := cmd.Environ()
	for _, entry := range environment {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("agent fixture environment contains NUL")
		}
	}
	sort.Slice(environment, func(i, j int) bool { return strings.ToUpper(environment[i]) < strings.ToUpper(environment[j]) })
	block := utf16.Encode([]rune(strings.Join(environment, "\x00") + "\x00\x00"))
	var pipes [3][2]*os.File
	defer func() {
		if result != nil {
			for _, pair := range pipes {
				for _, file := range pair {
					if file != nil {
						result = errors.Join(result, file.Close())
					}
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
	children := [][2]int{{0, 0}, {1, 1}, {2, 1}}
	handles := make([]windows.Handle, 3)
	for i, index := range children {
		handles[i] = windows.Handle(pipes[index[0]][index[1]].Fd())
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
	defer func() {
		if result != nil {
			result = errors.Join(result, windows.CloseHandle(job))
		}
	}()
	if err := setBuildToolsQueryJobLimits(job); err != nil {
		return nil, err
	}
	const procThreadAttributeJobList = 0x0002000d
	if err := attributes.Update(procThreadAttributeJobList, unsafe.Pointer(&job), unsafe.Sizeof(job)); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES,
		StdInput: handles[0], StdOutput: handles[1], StdErr: handles[2],
	}, ProcThreadAttributeList: attributes.List()}
	var process windows.ProcessInformation
	flags := uint32(windows.CREATE_SUSPENDED | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(application, commandLine, nil, nil, true, flags, &block[0], directory, &startup.StartupInfo, &process); err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, stopBuildToolsQueryJob(job), windows.CloseHandle(process.Process), windows.CloseHandle(process.Thread))
		}
	}()
	for _, index := range children {
		file := pipes[index[0]][index[1]]
		pipes[index[0]][index[1]] = nil
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		return nil, err
	}
	return &agentCLIProcess{input: pipes[0][1], output: pipes[1][0], diagnostic: pipes[2][0], wait: func(stop bool) error {
		var waitErr error
		for {
			if err := ctx.Err(); err != nil {
				waitErr = err
				break
			}
			status, err := windows.WaitForSingleObject(process.Process, 25)
			if err != nil {
				waitErr = err
				break
			}
			if stop {
				if status != uint32(windows.WAIT_TIMEOUT) {
					waitErr = errors.New("agent server exited before intentional stop")
				}
				break
			}
			if status == windows.WAIT_OBJECT_0 {
				var code uint32
				waitErr = windows.GetExitCodeProcess(process.Process, &code)
				if code != 0 {
					waitErr = errors.Join(waitErr, fmt.Errorf("agent fixture exit status %d", code))
				}
				break
			}
			if status != uint32(windows.WAIT_TIMEOUT) {
				waitErr = fmt.Errorf("unexpected agent wait status %d", status)
				break
			}
		}
		// Await exact private job handles, including helpers retaining pipes/CWD.
		cleanupErr := stopBuildToolsQueryJob(job)
		status, rootErr := windows.WaitForSingleObject(process.Process, 2000)
		if rootErr == nil && status != windows.WAIT_OBJECT_0 {
			rootErr = fmt.Errorf("agent root did not exit: %d", status)
		}
		return errors.Join(waitErr, cleanupErr, rootErr, windows.CloseHandle(process.Thread), windows.CloseHandle(process.Process), windows.CloseHandle(job))
	}}, nil
}
