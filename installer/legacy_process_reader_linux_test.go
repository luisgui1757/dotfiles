//go:build linux

package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func legacyReaderProcRoot(t *testing.T, pid int) string {
	t.Helper()
	root := t.TempDir()
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
	if err := os.Symlink("/proc/"+strconv.Itoa(pid), filepath.Join(root, strconv.Itoa(pid))); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLegacyLinuxReaderInspectsItself(t *testing.T) {
	root, nix := legacyReaderProcRoot(t, os.Getpid()), t.TempDir()
	var output strings.Builder
	if err := InspectLegacyLinuxProcesses(context.Background(), root, nix, &output); err != nil || output.Len() != 0 {
		t.Fatalf("the scanner's own inspection descriptors blocked its query: %s: %v", output.String(), err)
	}
}

func TestLegacyLinuxReaderReportsOpenData(t *testing.T) {
	nix := t.TempDir()
	file, err := os.Create(filepath.Join(nix, "held"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	child := exec.Command("/bin/sleep", "30")
	child.ExtraFiles = []*os.File{file}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		var exit *exec.ExitError
		if err := child.Wait(); err != nil && !errors.As(err, &exit) {
			t.Error(err)
		}
	})
	root := legacyReaderProcRoot(t, child.Process.Pid)
	var output strings.Builder
	err = InspectLegacyLinuxProcesses(context.Background(), root, nix, &output)
	if err != nil || output.String() != strconv.Itoa(child.Process.Pid)+"\topen path in /nix\n" {
		t.Fatalf("an actual open data descriptor was omitted: %s: %v", output.String(), err)
	}
}

func TestLegacyLinuxReaderHandleWaitsForAllThreads(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "leader")
	pid, err := strconv.Atoi(filepath.Base(fixture.entry))
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	p := newLegacyLinuxProc(fixture.ctx, filepath.Join(fixture.root, "nix"))
	if exited, err := p.exited(fd, pid); exited || err != nil {
		t.Fatal("a live process was reported exited", exited, err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	requireLegacyExitedLeader(t, fixture)
	if exited, err := p.exited(fd, pid); exited || err != nil {
		t.Fatal("main-thread exit hid a live worker", exited, err)
	}
	directory, err := unix.Open(fixture.entry, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(directory); err != nil {
			t.Error(err)
		}
	})
	if gone, _ := p.noUserspace(directory, fd, pid); gone {
		t.Fatal("main-thread exit hid a live worker")
	}
	if err := unix.Kill(pid, unix.SIGKILL); err != nil {
		t.Fatal(err)
	}
	// The killed process stays unreaped until the fixture's cleanup, so it keeps
	// its PID: readiness no longer proves exit, but its status does.
	deadline := time.Now().Add(5 * time.Second)
	for {
		exited, err := p.noUserspace(directory, fd, pid)
		if exited && err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("whole-process exit was not observed")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestLegacyLinuxReaderReportsDescriptorAfterHandover(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "handover")
	p := newLegacyLinuxProc(fixture.ctx, filepath.Join(fixture.root, "nix"))
	observed := false
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		if !observed && name == "fd/"+filepath.Base(fixture.fd) {
			legacyMark(t, fixture, "release")
			requireLegacyExitedLeader(t, fixture)
			n, err := unix.Readlinkat(directory, name, buffer)
			if err == nil {
				t.Fatal("fixture did not expose the dead leader's failed descriptor read")
			}
			legacyMark(t, fixture, "exec")
			legacyAwait(t, fixture, "execd")
			observed = true
			return n, err
		}
		return unix.Readlinkat(directory, name, buffer)
	}
	output, err := legacyScan(t, p, fixture.root)
	if err != nil || !observed || output != filepath.Base(fixture.entry)+"\topen path in /nix\n" {
		t.Fatal("handover hid a continuously held descriptor", output, err)
	}
}

func TestLegacyLinuxReaderRefusesIncompleteListingAcrossHandover(t *testing.T) {
	for _, handover := range []bool{false, true} {
		t.Run(strconv.FormatBool(handover), func(t *testing.T) {
			fixture := newLegacyProcessScanFixture(t, "handover")
			p := newLegacyLinuxProc(fixture.ctx, filepath.Join(fixture.root, "nix"))
			observed := false
			p.getdents = func(fd int, buffer []byte) (int, error) {
				if !observed {
					legacyMark(t, fixture, "release")
					requireLegacyExitedLeader(t, fixture)
					observed = true
				}
				n, err := unix.Getdents(fd, buffer)
				if handover && (n == 0 || err != nil) {
					legacyMark(t, fixture, "exec")
					legacyAwait(t, fixture, "execd")
				}
				return n, err
			}
			if output, err := legacyScan(t, p, fixture.root); err == nil || !observed || !strings.Contains(err.Error(), "cannot inspect open files") {
				t.Fatal("incomplete listing became a clean inventory", output, err)
			}
		})
	}
}

func TestLegacyLinuxReaderRefusesTornMappingView(t *testing.T) {
	for _, afterChunk := range []bool{false, true} {
		t.Run(strconv.FormatBool(afterChunk), func(t *testing.T) {
			root := t.TempDir()
			nix := filepath.Join(root, "nix")
			writeConfigFixture(t, filepath.Join(nix, "held"), strings.Repeat("x", 4096))
			program := filepath.Join(root, "process")
			successor := filepath.Join(nix, "process")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, "cc", "-o", program, "testdata/legacy-maps-exec.c").CombinedOutput(); err != nil {
				t.Fatal(string(output), err)
			}
			data, err := os.ReadFile(program)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(successor, data, 0700); err != nil {
				t.Fatal(err)
			}
			child := exec.Command(program, root, filepath.Join(nix, "held"), successor)
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					t.Error(err)
				}
				var exit *exec.ExitError
				if err := child.Wait(); err != nil && !errors.As(err, &exit) {
					t.Error(err)
				}
			})
			fixture := legacyProcessScanFixture{root: root, entry: "/proc/" + strconv.Itoa(child.Process.Pid), ctx: ctx}
			legacyAwait(t, fixture, "ready")
			proc := legacyReaderProcRoot(t, child.Process.Pid)
			p := newLegacyLinuxProc(ctx, nix)
			observed := false
			p.read = func(fd int, buffer []byte) (int, error) {
				path, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
				if err != nil {
					t.Fatal(err)
				}
				if !observed && strings.HasSuffix(path, "/maps") {
					var n int
					if afterChunk {
						n, err = unix.Read(fd, buffer[:min(len(buffer), 512)])
						if err != nil || n == 0 || strings.Contains(string(buffer[:n]), nix) {
							t.Fatal("fixture did not precede its store mapping", n, err)
						}
					}
					legacyMark(t, fixture, "exec")
					legacyAwait(t, fixture, "execd")
					observed = true
					if afterChunk {
						return n, err
					}
				}
				return unix.Read(fd, buffer)
			}
			if output, err := legacyScan(t, p, proc); err == nil || !observed || !strings.Contains(err.Error(), "cannot inspect mappings") {
				t.Fatal("torn old mapping view became a clean inventory", output, err)
			}
			if target, err := os.Readlink(filepath.Join(fixture.entry, "exe")); err != nil || target != successor {
				t.Fatal("new process did not retain a store executable", target, err)
			}
		})
	}
}

func TestLegacyLinuxReaderSubprocessEntryPoint(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "dotfiles")
	if output, err := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/dotfiles").CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	fixture := newLegacyProcessScanFixture(t, "handover")
	command := exec.CommandContext(fixture.ctx, binary, "internal-legacy-linux-processes", fixture.root, filepath.Join(fixture.root, "nix"))
	if output, err := command.CombinedOutput(); err != nil || string(output) != filepath.Base(fixture.entry)+"\topen path in /nix\n" {
		t.Fatal("private command did not report the real consumer", string(output), err)
	}
}

func TestLegacyLinuxReaderWorkerExecStress(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "stress")
	p := newLegacyLinuxProc(fixture.ctx, filepath.Join(fixture.root, "nix"))
	reported, refused := 0, 0
	for range 3000 {
		output, err := legacyScan(t, p, fixture.root)
		if err != nil {
			if !strings.Contains(err.Error(), "cannot inspect") {
				t.Fatal("unexpected scan failure", err)
			}
			refused++
		} else if output == filepath.Base(fixture.entry)+"\topen path in /nix\n" {
			reported++
		} else {
			t.Fatalf("continuous store descriptor silently omitted: %q", output)
		}
	}
	generations, err := os.ReadFile(filepath.Join(fixture.root, "generations"))
	if err != nil || len(generations) < 2 || reported == 0 {
		t.Fatal("fixture did not exercise both handover and normal reporting", len(generations), reported, err)
	}
	t.Logf("3000 scans, %d reported, %d safely refused, %d exec generations", reported, refused, len(generations))
}

// Linux 6.12 measurement (2026-10-23): a fresh pidfd can poll ready for a moment
// while a worker thread's exec takes over the leader PID, and the stress test
// then omitted a live store consumer about once per hundred 3000-scan runs.
// Simulate that readiness on a live process whose executable read fails: a
// fresh lookup still finds its leader, so the scan must refuse, never omit.
func TestLegacyLinuxReaderTransientPidfdReadinessIsNotExit(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "handover")
	p := newLegacyLinuxProc(fixture.ctx, filepath.Join(fixture.root, "nix"))
	polled := false
	p.poll = func(events []unix.PollFd, timeout int) (int, error) {
		if len(events) != 1 || timeout != 0 {
			t.Fatal("invalid pidfd readiness query", events, timeout)
		}
		polled = true
		events[0].Revents = unix.POLLIN
		return 1, nil
	}
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		if name == "exe" {
			return 0, unix.ENOENT
		}
		return unix.Readlinkat(directory, name, buffer)
	}
	output, err := legacyScan(t, p, fixture.root)
	if !polled || output != "" || err == nil || !strings.Contains(err.Error(), "cannot inspect executable of live process") {
		t.Fatal("transient pidfd readiness hid a live store consumer", output, err)
	}
}

// Flags and PID have already been validated by the scanner. Older kernels use
// EINVAL for a reaped TGID retained only as a process-group/session ID; newer
// kernels use ENOENT for a PID now naming a nonleader thread.
func TestLegacyLinuxReaderProcessIdentityRaceErrors(t *testing.T) {
	for _, test := range []struct {
		errno error
		gone  bool
	}{
		{unix.ESRCH, true}, {unix.EINVAL, true}, {unix.ENOENT, true},
		{unix.EPERM, false}, {unix.ENOSYS, false}, {unix.EMFILE, false},
	} {
		t.Run(test.errno.Error(), func(t *testing.T) {
			root := legacyReaderProcRoot(t, os.Getpid())
			p := newLegacyLinuxProc(context.Background(), "/nix")
			called := false
			p.openPID = func(pid, flags int) (int, error) {
				called = true
				if pid != os.Getpid() || flags != 0 {
					t.Fatal("invalid pidfd boundary", pid, flags)
				}
				return -1, test.errno
			}
			output, err := legacyScan(t, p, root)
			if !called || output != "" || (test.gone && err != nil) || (!test.gone && !errors.Is(err, test.errno)) {
				t.Fatal("kernel identity race classification", output, err)
			}
		})
	}
}
