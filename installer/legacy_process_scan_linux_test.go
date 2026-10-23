//go:build linux

package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type legacyProcessScanFixture struct {
	root, entry, fd string
	ctx             context.Context
}

func newLegacyProcessScanFixture(t *testing.T, mode string) legacyProcessScanFixture {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "process.c")
	writeConfigFixture(t, source, `#include <pthread.h>
#include <unistd.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
static pid_t owner;
static void check_owner(void) { if (getppid() != owner) _exit(0); }
static void linger(void) { for (;;) { check_owner(); usleep(1000); } }
static void path(char *out, const char *root, const char *name) {
	if (snprintf(out, 4096, "%s/%s", root, name) >= 4096) exit(90);
}
static void mark(const char *root, const char *name, int value) {
	char file[4096]; path(file, root, name);
	char pending[4096];
	if (snprintf(pending, sizeof(pending), "%s.pending", file) >= sizeof(pending)) exit(90);
	FILE *out = fopen(pending, "w");
	if (!out || fprintf(out, "%d\n", value) < 0 || fclose(out)) exit(91);
	// Publish only after closing: this marker can reuse the descriptor under test.
	if (rename(pending, file)) exit(91);
}
static void await(const char *root, const char *name) {
	char file[4096]; path(file, root, name);
	for (int n=0; n<5000; n++) { check_owner(); if (!access(file, F_OK)) return; usleep(1000); }
	exit(92);
}
static char **args;
static void *handover(void *unused) {
	(void)unused;
	await(args[1], "exec");
	char *next[] = {args[0], args[1], "execd", args[3], 0};
	execv(args[0], next);
	_exit(98);
}
static void *churn(void *unused) {
	(void)unused;
	usleep(1000);
	check_owner();
	char *next[] = {args[0], args[1], "stress-again", args[3], 0};
	execv(args[0], next);
	_exit(99);
}
static void *hold(void *unused) { (void)unused; linger(); return 0; }
int main(int argc, char **argv) {
	if (argc != 4) return 93;
	char *end = 0;
	long parsed = strtol(argv[3], &end, 10);
	if (!end || *end || parsed < 1 || (pid_t)parsed != parsed) return 103;
	owner = (pid_t)parsed;
	check_owner();
	args = argv;
	if (!strcmp(argv[2], "execd")) { mark(argv[1], "execd", 3); linger(); }
	char held[4096]; path(held, argv[1], "nix/held");
	int stress = !strncmp(argv[2], "stress", 6);
	int fd = !strcmp(argv[2], "stress-again") ? 3 : open(!strcmp(argv[2], "handover") || stress ? held : "/dev/null", O_RDONLY);
	if (stress) {
		if (fd != 3 || fcntl(fd, F_GETFD) != 0) return 100;
		char log[4096]; path(log, argv[1], "generations");
		int out = open(log, O_WRONLY | O_CREAT | O_APPEND | O_CLOEXEC, 0600);
		if (out < 0 || write(out, "x", 1) != 1 || close(out)) return 101;
		if (!strcmp(argv[2], "stress")) mark(argv[1], "ready", fd);
		pthread_t worker;
		if (pthread_create(&worker, 0, churn, 0)) return 102;
		linger();
	}
	if (fd < 0) return 94;
	if (!strcmp(argv[2], "leader") || !strcmp(argv[2], "handover")) {
		pthread_t worker;
		if (pthread_create(&worker, 0, !strcmp(argv[2], "handover") ? handover : hold, 0)) return 95;
		mark(argv[1], "ready", fd);
		await(argv[1], "release");
		pthread_exit(0);
	}
	mark(argv[1], "ready", fd);
	if (!strcmp(argv[2], "steady")) { linger(); }
	await(argv[1], "close");
	if (close(fd)) return 96;
	mark(argv[1], "closed", fd);
	await(argv[1], "reopen");
	if (open("/dev/null", O_RDONLY) != fd) return 97;
	mark(argv[1], "reopened", fd);
	linger();
}
`)
	program := filepath.Join(root, "process")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	if output, err := exec.CommandContext(ctx, "cc", "-pthread", "-o", program, source).CombinedOutput(); err != nil {
		t.Fatalf("compile native process fixture: %s: %v", output, err)
	}
	writeConfigFixture(t, filepath.Join(root, "nix", "held"), "held data")
	child := exec.Command(program, root, mode, strconv.Itoa(os.Getpid()))
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
	pid := strconv.Itoa(child.Process.Pid)
	entry := filepath.Join(root, pid)
	if err := os.Symlink("/proc/"+pid, entry); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
	for {
		data, err := os.ReadFile(filepath.Join(root, "ready"))
		if err == nil && len(data) > 0 {
			fd, parseErr := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return legacyProcessScanFixture{root: root, entry: entry, fd: filepath.Join(entry, "fd", strconv.FormatUint(fd, 10)), ctx: ctx}
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal("process fixture did not become ready", ctx.Err())
		}
		time.Sleep(time.Millisecond)
	}
}

func requireLegacyExitedLeader(t *testing.T, fixture legacyProcessScanFixture) {
	t.Helper()
	for {
		status, err := os.ReadFile(filepath.Join(fixture.entry, "status"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(status), "State:\tZ") && strings.Contains(string(status), "Threads:\t2") {
			return
		}
		if fixture.ctx.Err() != nil {
			t.Fatalf("leader did not exit with a live worker: %s: %v", status, fixture.ctx.Err())
		}
		time.Sleep(time.Millisecond)
	}
}

func legacyScan(t *testing.T, p legacyLinuxProc, root string) (string, error) {
	t.Helper()
	var output strings.Builder
	err := p.scan(root, &output)
	return output.String(), err
}

func legacyMark(t *testing.T, fixture legacyProcessScanFixture, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fixture.root, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func legacyAwait(t *testing.T, fixture legacyProcessScanFixture, name string) {
	t.Helper()
	for {
		if _, err := os.Stat(filepath.Join(fixture.root, name)); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if err := fixture.ctx.Err(); err != nil {
			t.Fatal("missing process marker", name, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestLegacyLinuxProcessScanAcceptsExitAfterExecutableRead(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "steady")
	p := newLegacyLinuxProc(fixture.ctx, "/nix")
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		n, err := unix.Readlinkat(directory, name, buffer)
		if name == "exe" {
			pid, parseErr := strconv.Atoi(filepath.Base(fixture.entry))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if err := unix.Kill(pid, unix.SIGKILL); err != nil {
				t.Fatal(err)
			}
			for {
				status, err := os.ReadFile(filepath.Join(fixture.entry, "status"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(status), "State:\tZ") && strings.Contains(string(status), "Threads:\t1") {
					break
				}
				if fixture.ctx.Err() != nil {
					t.Fatal("fixture did not become a single-thread zombie")
				}
				time.Sleep(time.Millisecond)
			}
		}
		return n, err
	}
	if output, err := legacyScan(t, p, fixture.root); err != nil || output != "" {
		t.Fatal("exited process blocked inspection", output, err)
	}
}

func TestLegacyLinuxProcessScanAcceptsReapedProcessDuringRead(t *testing.T) {
	child := exec.Command("/bin/sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	t.Cleanup(func() {
		if reaped {
			return
		}
		if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		var exit *exec.ExitError
		if err := child.Wait(); err != nil && !errors.As(err, &exit) {
			t.Error(err)
		}
	})
	root := legacyReaderProcRoot(t, child.Process.Pid)
	p := newLegacyLinuxProc(context.Background(), "/nix")
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		if name == "exe" {
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			var exit *exec.ExitError
			if err := child.Wait(); !errors.As(err, &exit) {
				t.Fatal("fixture did not exit", err)
			}
			reaped = true
		}
		return unix.Readlinkat(directory, name, buffer)
	}
	if output, err := legacyScan(t, p, root); err != nil || output != "" || !reaped {
		t.Fatal("reaped process blocked inspection", output, err)
	}
}

func TestLegacyLinuxProcessScanRefusesExitedLeaderWithLiveThread(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "leader")
	legacyMark(t, fixture, "release")
	requireLegacyExitedLeader(t, fixture)
	p := newLegacyLinuxProc(fixture.ctx, "/nix")
	if output, err := legacyScan(t, p, fixture.root); err == nil || !strings.Contains(err.Error(), "cannot inspect executable of live process "+filepath.Base(fixture.entry)) {
		t.Fatal("live worker was hidden", output, err)
	}
}

func TestLegacyLinuxProcessScanHandlesNativeDescriptorReuse(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "reuse")
	p := newLegacyLinuxProc(fixture.ctx, "/nix")
	calls := 0
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		if name == "fd/"+filepath.Base(fixture.fd) {
			calls++
			if calls == 1 {
				legacyMark(t, fixture, "close")
				legacyAwait(t, fixture, "closed")
				n, err := unix.Readlinkat(directory, name, buffer)
				if !errors.Is(err, unix.ENOENT) {
					t.Fatal("no real descriptor gap", n, err)
				}
				legacyMark(t, fixture, "reopen")
				legacyAwait(t, fixture, "reopened")
				return n, err
			}
		}
		return unix.Readlinkat(directory, name, buffer)
	}
	if output, err := legacyScan(t, p, fixture.root); err != nil || output != "" || calls != 2 {
		t.Fatal("reopened descriptor was not inspected twice", output, calls, err)
	}
}

func TestLegacyLinuxProcessScanRefusesLeaderExitDuringDescriptorRead(t *testing.T) {
	fixture := newLegacyProcessScanFixture(t, "leader")
	p := newLegacyLinuxProc(fixture.ctx, "/nix")
	observed := false
	p.readlink = func(directory int, name string, buffer []byte) (int, error) {
		if name == "fd/"+filepath.Base(fixture.fd) {
			legacyMark(t, fixture, "release")
			requireLegacyExitedLeader(t, fixture)
			observed = true
		}
		return unix.Readlinkat(directory, name, buffer)
	}
	if output, err := legacyScan(t, p, fixture.root); err == nil || !observed || !strings.Contains(err.Error(), "cannot inspect open files of live process") {
		t.Fatal("exited leader disguised a live worker's open files", output, err)
	}
}

// A private filesystem models kernel status/error responses while a real,
// stable child supplies pidfd liveness and the descriptor-table cursor. No
// internal policy function is replaced to force the expected classification.
func legacySyntheticProc(t *testing.T) (legacyLinuxProc, string, string) {
	t.Helper()
	fixture := newLegacyProcessScanFixture(t, "steady")
	root := t.TempDir()
	entry := filepath.Join(root, "42")
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw\n")
	status, err := os.ReadFile(filepath.Join(fixture.entry, "status"))
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(entry, "status"), string(status))
	writeConfigFixture(t, filepath.Join(entry, "stat"), "42 (fixture) S 2 0 0 0 -1 0 0 0 0\n")
	writeConfigFixture(t, filepath.Join(entry, "maps"), "1000-2000 r-xp 0 00:00 0 /usr/bin/fixture\n")
	for name, target := range map[string]string{"exe": "/usr/bin/fixture", "cwd": "/tmp", "root": "/", "fd": filepath.Join(fixture.entry, "fd")} {
		if err := os.Symlink(target, filepath.Join(entry, name)); err != nil {
			t.Fatal(err)
		}
	}
	pid, err := strconv.Atoi(filepath.Base(fixture.entry))
	if err != nil {
		t.Fatal(err)
	}
	p := newLegacyLinuxProc(fixture.ctx, "/nix")
	p.openPID = func(logical, flags int) (int, error) {
		if logical != 42 {
			t.Fatal("unexpected fixture PID", logical)
		}
		return unix.PidfdOpen(pid, flags)
	}
	return p, root, entry
}

func TestLegacyLinuxProcessScanChecksStateAtEveryUnreadableBoundary(t *testing.T) {
	for _, boundary := range []string{"exe", "maps", "fd", "cwd", "root", "fd/3"} {
		for _, state := range []struct {
			name, status string
			accept       bool
		}{
			{"zombie", "State:\tZ (zombie)\nThreads:\t1\n", true},
			{"dead", "State:\tX (dead)\nThreads:\t1\n", true},
			{"released", "State:\tX (dead)\nThreads:\t0\n", false},
			{"released zombie", "State:\tZ (zombie)\nThreads:\t0\n", false},
			{"live", "State:\tS (sleeping)\nThreads:\t1\n", false},
			{"live worker", "State:\tZ (zombie)\nThreads:\t2\n", false},
			{"unknown threads", "State:\tZ (zombie)\n", false},
		} {
			t.Run(boundary+"/"+state.name, func(t *testing.T) {
				p, root, entry := legacySyntheticProc(t)
				status, err := p.statusFromFixture(entry)
				if err != nil {
					t.Fatal(err)
				}
				writeConfigFixture(t, filepath.Join(entry, "status"), state.status+"FDSize:\t"+status["FDSize"]+"\n")
				if boundary == "fd/3" {
					p.readlink = func(fd int, name string, buffer []byte) (int, error) {
						if name == boundary {
							return 0, unix.EACCES
						}
						return unix.Readlinkat(fd, name, buffer)
					}
				} else {
					if err := os.Remove(filepath.Join(entry, boundary)); err != nil {
						t.Fatal(err)
					}
					if boundary == "maps" {
						if err := os.Mkdir(filepath.Join(entry, boundary), 0700); err != nil {
							t.Fatal(err)
						}
					}
				}
				if output, err := legacyScan(t, p, root); (err == nil) != state.accept {
					t.Fatal("unexpected state classification", output, err)
				}
			})
		}
	}
}

func (p legacyLinuxProc) statusFromFixture(entry string) (map[string]string, error) {
	fd, err := unix.Open(entry, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	fields, err := p.status(fd)
	return fields, errors.Join(err, unix.Close(fd))
}

func TestLegacyLinuxProcessScanClassifiesReopenedDescriptor(t *testing.T) {
	for _, target := range []string{"/tmp/personal", "/nix/store/reopened", "unreadable"} {
		t.Run(filepath.Base(target), func(t *testing.T) {
			p, root, _ := legacySyntheticProc(t)
			calls := 0
			p.readlink = func(fd int, name string, buffer []byte) (int, error) {
				if name != "fd/3" {
					return unix.Readlinkat(fd, name, buffer)
				}
				calls++
				if calls == 1 || target == "unreadable" {
					return 0, unix.EACCES
				}
				return copy(buffer, target), nil
			}
			output, err := legacyScan(t, p, root)
			if calls != 2 {
				t.Fatal("descriptor was not read exactly twice", calls)
			}
			switch target {
			case "/tmp/personal":
				if err != nil || output != "" {
					t.Fatal(output, err)
				}
			case "/nix/store/reopened":
				if err != nil || output != "42\topen path in /nix\n" {
					t.Fatal(output, err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), "threads=1") {
					t.Fatal(output, err)
				}
			}
		})
	}
}

// A later PF_EXITING sample cannot prove a failed descriptor read was closed:
// exec can replace the exiting leader before that sample. Require the complete
// descriptor table instead, including a positive, matching capacity.
func TestLegacyLinuxClosedDescriptorRequiresCompleteTable(t *testing.T) {
	for _, capacity := range []string{"actual", "0", "unknown", "128"} {
		t.Run(capacity, func(t *testing.T) {
			p, root, entry := legacySyntheticProc(t)
			if capacity != "actual" {
				writeConfigFixture(t, filepath.Join(entry, "status"), "State:\tS\nThreads:\t1\nFDSize:\t"+capacity+"\n")
			}
			if output, err := legacyScan(t, p, root); (err == nil) != (capacity == "actual") {
				t.Fatal("unverified table accepted", output, err)
			}
		})
	}
}

func TestLegacyLinuxProcessScanRefusesUninspectedLiveProcess(t *testing.T) {
	p, root, entry := legacySyntheticProc(t)
	if err := os.Remove(filepath.Join(entry, "exe")); err != nil {
		t.Fatal(err)
	}
	if output, err := legacyScan(t, p, root); err == nil {
		t.Fatal("uninspected live process was omitted", output)
	}
}

func TestLegacyLinuxProcessScanRefusesHiddenProcesses(t *testing.T) {
	root := t.TempDir()
	writeConfigFixture(t, filepath.Join(root, "self", "mountinfo"), "42 1 0:1 / /proc rw - proc proc rw,hidepid=2\n")
	if output, err := legacyScan(t, newLegacyLinuxProc(context.Background(), "/nix"), root); err == nil {
		t.Fatal("hidden inventory accepted", output)
	}
}

func TestLegacyLinuxKernelThreadWithoutStatusFlag(t *testing.T) {
	p, root, entry := legacySyntheticProc(t)
	if err := os.Remove(filepath.Join(entry, "exe")); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(entry, "status"), "Name:\tkworker\nState:\tI (idle)\n")
	writeConfigFixture(t, filepath.Join(entry, "stat"), "42 (kworker ) name) I 2 0 0 0 -1 2097152 0 0 0\n")
	if output, err := legacyScan(t, p, root); err != nil || output != "" {
		t.Fatal("kernel thread refused", output, err)
	}
}

func TestLegacyLinuxProcessNameCannotImpersonateKernelThread(t *testing.T) {
	p, root, entry := legacySyntheticProc(t)
	if err := os.Remove(filepath.Join(entry, "exe")); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(entry, "stat"), "42 (fake) I 2 0 0 0 -1 2097152\nactual) S 2 0 0 0 -1 0 0 0 0\n")
	if output, err := legacyScan(t, p, root); err == nil {
		t.Fatal("process name impersonated a kernel thread", output)
	}
}

func TestLegacyLinuxProcessDataConsumers(t *testing.T) {
	for _, link := range []string{"cwd", "root", "fd/3"} {
		t.Run(link, func(t *testing.T) {
			p, root, _ := legacySyntheticProc(t)
			p.readlink = func(fd int, name string, buffer []byte) (int, error) {
				if name == link {
					return copy(buffer, "/nix/store/fixture-data"), nil
				}
				return unix.Readlinkat(fd, name, buffer)
			}
			if output, err := legacyScan(t, p, root); err != nil || output != "42\topen path in /nix\n" {
				t.Fatal("data consumer omitted", output, err)
			}
		})
	}
}

func TestLegacyLinuxProcessFixtureOwner(t *testing.T) {
	if os.Getenv("DOTFILES_LEGACY_FIXTURE_OWNER") != "1" {
		return
	}
	fixture := newLegacyProcessScanFixture(t, "stress")
	fmt.Println(filepath.Base(fixture.entry))
	for {
		time.Sleep(time.Second)
	}
}

func TestLegacyLinuxProcessFixtureExitsWithOwner(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, "-test.run=^TestLegacyLinuxProcessFixtureOwner$")
	cmd.Env = append(os.Environ(), "DOTFILES_LEGACY_FIXTURE_OWNER=1", "TMPDIR="+t.TempDir())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited {
			killErr := cmd.Process.Kill()
			waitErr := cmd.Wait()
			var exit *exec.ExitError
			if errors.As(waitErr, &exit) {
				waitErr = nil
			}
			if err := errors.Join(killErr, waitErr); err != nil {
				t.Error(err)
			}
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal("fixture owner readiness", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p := newLegacyLinuxProc(context.Background(), "/nix")
		gone, err := p.exited(handle, pid)
		if err == nil && !gone {
			err = unix.PidfdSendSignal(handle, unix.SIGKILL, nil, 0)
		}
		if err := errors.Join(err, unix.Close(handle)); err != nil {
			t.Error(err)
		}
	})
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitErr := cmd.Wait()
	waited = true
	var exit *exec.ExitError
	if !errors.As(waitErr, &exit) {
		t.Fatal("owner did not terminate", waitErr)
	}
	p := newLegacyLinuxProc(ctx, "/nix")
	deadline := time.Now().Add(2 * time.Second)
	for {
		gone, err := p.exited(handle, pid)
		if err != nil {
			t.Fatal(err)
		}
		if gone {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("exec stress fixture survived its owner")
		}
		time.Sleep(time.Millisecond)
	}
}
