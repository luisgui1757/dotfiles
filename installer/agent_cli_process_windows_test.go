package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func agentCLIProtocolFixtureArguments(mode, directory string) []string {
	return []string{"-test.run=^TestAgentCLIWindowsProtocolFixture$", "--", mode, directory}
}
func TestAgentCLIWindowsProtocolFixture(t *testing.T) {
	if len(os.Args) < 4 || os.Args[len(os.Args)-3] != "--" {
		return
	}
	mode, directory := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "hold" {
		for {
			time.Sleep(time.Second)
		}
	}
	switch mode {
	case "failure":
		fmt.Fprintln(os.Stderr, "fixture diagnostic")
		os.Exit(23)
	case "environment":
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stdout, "%s\n%s", os.Getenv("DOTFILES_FIXTURE_VALUE"), cwd)
		os.Exit(0)
	case "oversized":
		if _, err := io.WriteString(os.Stdout, strings.Repeat("x", (1<<20)+1)); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	case "reject":
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(os.Stdout, `{"id":1,"error":{"message":"fixture rejection"}}`)
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	if mode != "root" {
		t.Fatal("unknown fixture mode")
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, `"method":"initialize"`) {
		t.Fatal("missing initialization", err)
	}
	var flags uint32
	ok, _, flagErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation").Call(os.Stderr.Fd(), uintptr(unsafe.Pointer(&flags)))
	if ok == 0 || flags&windows.HANDLE_FLAG_INHERIT == 0 {
		t.Fatal("fixture requires inherited stderr", flags, flagErr)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	application, err := windows.UTF16PtrFromString(program)
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{program}, agentCLIProtocolFixtureArguments("hold", directory)...)))
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Join(directory, ".codex"))
	if err != nil {
		t.Fatal(err)
	}
	null, err := windows.UTF16PtrFromString("NUL")
	if err != nil {
		t.Fatal(err)
	}
	security := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	handle, err := windows.CreateFile(null, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, &security, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Flags: windows.STARTF_USESTDHANDLES, StdInput: handle, StdOutput: handle, StdErr: handle}
	var child windows.ProcessInformation
	// Match Rust std: inheritance enabled, NUL stdio, no handle allowlist.
	// Extra inherited parent pipe handles remain open in the descendant.
	if err := windows.CreateProcess(application, arguments, nil, nil, true, windows.CREATE_NO_WINDOW, nil, cwd, &startup, &child); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(windows.CloseHandle(handle), windows.CloseHandle(child.Thread), windows.CloseHandle(child.Process)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "child.pid.tmp"), []byte(strconv.Itoa(int(child.ProcessId))), 0600); err != nil {
		t.Fatal(err)
	}
	// A rename may publish the name before releasing its DELETE-access handle.
	// Hold that native access until the reader has attempted the published file.
	markerPath, err := windows.UTF16PtrFromString(filepath.Join(directory, "child.pid.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := windows.CreateFile(markerPath, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	publishErr := os.Rename(filepath.Join(directory, "child.pid.tmp"), filepath.Join(directory, "child.pid"))
	deadline := time.Now().Add(10 * time.Second)
	for publishErr == nil {
		if _, err := os.Stat(filepath.Join(directory, "pid-read-attempted")); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			publishErr = err
			break
		}
		if time.Now().After(deadline) {
			publishErr = errors.New("fixture PID reader did not acknowledge publication")
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := errors.Join(publishErr, windows.CloseHandle(marker)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stdout, `{"id":1,"result":{}}`); err != nil {
		t.Fatal(err)
	}
	line, err = reader.ReadString('\n')
	if err != nil || !strings.Contains(line, `"method":"initialized"`) {
		t.Fatal("missing initialized notification", err)
	}
	if rest, err := io.ReadAll(reader); err != nil || len(rest) != 0 {
		t.Fatal("missing input EOF", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "input-closed"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "release-root")); err == nil {
			os.Exit(0)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture root was not released")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestAgentCLIWindowsProtocolReapsInheritedHandles(t *testing.T) {
	base := t.TempDir()
	home, outsideHome := filepath.Join(base, "private agent ü with spaces"), filepath.Join(base, "outside")
	for _, path := range []string{filepath.Join(home, ".codex"), outsideHome} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	outside := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments("hold", outsideHome)...)
	if err := outside.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		killErr := outside.Process.Kill()
		var exit *exec.ExitError
		waitErr := outside.Wait()
		if errors.As(waitErr, &exit) {
			waitErr = nil
		}
		if err := errors.Join(killErr, waitErr); err != nil {
			t.Error(err)
		}
	})
	outsideHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(outside.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.CloseHandle(outsideHandle); err != nil {
			t.Error(err)
		}
	})
	command := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments("root", home)...)
	command.Dir, command.Env, command.WaitDelay = home, agentCLIEnvironment(home), 2*time.Second
	done := make(chan error, 1)
	go func() { done <- agentCodexProtocol(ctx, cancel, command) }()
	await := func(name string) []byte {
		t.Helper()
		for {
			data, err := os.ReadFile(filepath.Join(home, name))
			if name == "child.pid" && (err == nil || errors.Is(err, windows.ERROR_SHARING_VIOLATION)) {
				if ackErr := os.WriteFile(filepath.Join(home, "pid-read-attempted"), nil, 0600); ackErr != nil {
					t.Fatal(ackErr)
				}
			}
			if err == nil {
				return data
			}
			if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				t.Fatal("protocol ended before fixture readiness", err)
			case <-ctx.Done():
				t.Fatalf("fixture readiness %s: %v; last read: %v", name, ctx.Err(), err)
			default:
			}
			time.Sleep(time.Millisecond)
		}
	}
	pid, err := strconv.ParseUint(string(await("child.pid")), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		status, err := windows.WaitForSingleObject(child, 0)
		if err == nil && status == uint32(windows.WAIT_TIMEOUT) {
			err = windows.TerminateProcess(child, 1)
			if err == nil {
				status, err = windows.WaitForSingleObject(child, 5000)
			}
		}
		if err != nil || status != windows.WAIT_OBJECT_0 {
			t.Error("fixture child cleanup", status, err)
		}
		if err := windows.CloseHandle(child); err != nil {
			t.Error(err)
		}
	})
	await("input-closed")
	if status, err := windows.WaitForSingleObject(child, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("fixture child was not live before root exit", status, err)
	}
	if err := os.WriteFile(filepath.Join(home, "release-root"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	protocolErr := <-done
	childState, childErr := windows.WaitForSingleObject(child, 0)
	removeErr := os.RemoveAll(home)
	if errors.Is(protocolErr, exec.ErrWaitDelay) {
		t.Logf("old launcher reproduces both symptoms: %v; held child wait=%d err=%v; directory removal=%v", protocolErr, childState, childErr, removeErr)
	}
	if protocolErr != nil || childErr != nil || childState != windows.WAIT_OBJECT_0 || removeErr != nil {
		t.Fatal("protocol did not finish its owned process tree", protocolErr, childState, childErr, removeErr)
	}
	if status, err := windows.WaitForSingleObject(outsideHandle, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("outside process did not survive", status, err)
	}
}

func TestAgentCLIWindowsConsumerFailuresRemainErrors(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ mode, expected string }{
		{"failure", "exit status 23"}, {"reject", "rejected app-server initialization"},
		{"oversized", "output exceeds"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments(test.mode, t.TempDir())...)
			var output []byte
			var err error
			if test.mode == "reject" {
				err = agentCodexProtocol(ctx, cancel, cmd)
			} else {
				output, err = agentCLIOutput(ctx, cmd)
			}
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatal("consumer failure was lost", err)
			}
			if test.mode == "failure" && !strings.Contains(string(output), "fixture diagnostic") {
				t.Fatal("stderr lost")
			}
		})
	}
}

func TestAgentCLIWindowsEnvironmentAndDirectory(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "agent ü with spaces")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments("environment", directory)...)
	cmd.Dir = directory
	cmd.Env = append(os.Environ(), "DOTFILES_FIXTURE_VALUE=old", "dotfiles_fixture_value=new ü")
	output, err := agentCLIOutput(ctx, cmd)
	if err != nil || string(output) != "new ü\n"+directory {
		t.Fatal("environment/directory changed", string(output), err)
	}
}

func TestAgentCLIWindowsDeadlineReapsRoot(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments("hold", t.TempDir())...)
	p, err := startAgentCLI(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	done := agentCLIStreams(p, io.Discard, io.Discard)
	closeErr := p.input.Close()
	cancel()
	waitErr := p.wait(false)
	drainErr := agentCLIDrain(p, done, 2)
	if !errors.Is(waitErr, context.Canceled) || closeErr != nil || drainErr != nil {
		t.Fatal("cancellation/cleanup", waitErr, closeErr, drainErr)
	}
}

func TestAgentCLIWindowsIntentionalStop(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, agentCLIProtocolFixtureArguments("hold", t.TempDir())...)
	p, err := startAgentCLI(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	done := agentCLIStreams(p, io.Discard, io.Discard)
	closeErr := p.input.Close()
	waitErr := p.wait(true)
	drainErr := agentCLIDrain(p, done, 2)
	if err := errors.Join(closeErr, waitErr, drainErr); err != nil {
		t.Fatal(err)
	}
}
