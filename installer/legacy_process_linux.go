//go:build linux

package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// The syscall boundary is explicit so regressions can sequence real owned
// processes between kernel reads, including main-thread exit and exec handover.
type legacyLinuxProc struct {
	ctx      context.Context
	nix      string
	read     func(int, []byte) (int, error)
	readlink func(int, string, []byte) (int, error)
	getdents func(int, []byte) (int, error)
	openPID  func(int, int) (int, error)
	poll     func([]unix.PollFd, int) (int, error)
}

func newLegacyLinuxProc(ctx context.Context, nix string) legacyLinuxProc {
	return legacyLinuxProc{ctx: ctx, nix: nix, read: unix.Read, readlink: unix.Readlinkat,
		getdents: unix.Getdents, openPID: unix.PidfdOpen, poll: unix.Poll}
}

// InspectLegacyLinuxProcesses is a private read-only subprocess boundary. The
// caller supplies the existing system-root paths; normal discovery uses /proc
// and /nix. No package manager, configuration, or installer state is loaded.
func InspectLegacyLinuxProcesses(ctx context.Context, procRoot, nixRoot string, output io.Writer) error {
	if !legacyCanonicalPath(procRoot) || !legacyCanonicalPath(nixRoot) || len(procRoot) > 4096 || len(nixRoot) > 4096 {
		return errors.New("legacy process inspection requires bounded absolute paths")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	p := newLegacyLinuxProc(ctx, nixRoot)
	return p.scan(procRoot, output)
}

func (p legacyLinuxProc) scan(root string, output io.Writer) (err error) {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	mounts, err := p.readAt(int(directory.Fd()), "self/mountinfo", 1<<20)
	if err != nil {
		return err
	}
	found := false
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[4] != "/proc" {
			continue
		}
		found = true
		for _, option := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == ',' }) {
			if strings.HasPrefix(option, "hidepid=") && option != "hidepid=0" && option != "hidepid=off" {
				return errors.New("a restricted proc mount hides processes from legacy inspection")
			}
		}
	}
	if !found {
		return errors.New("the proc mount cannot be verified")
	}
	for {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(256)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		for _, entry := range entries {
			pid, parseErr := strconv.ParseUint(entry.Name(), 10, 31)
			if parseErr != nil || pid == 0 {
				continue
			}
			reason, inspectErr := p.inspect(int(directory.Fd()), entry.Name(), int(pid))
			if inspectErr != nil {
				return inspectErr
			}
			if reason != "" {
				if _, err := fmt.Fprintf(output, "%s\t%s\n", entry.Name(), reason); err != nil {
					return err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
	}
}

func (p legacyLinuxProc) pathUsesNix(path string) bool {
	return path == p.nix || strings.HasPrefix(path, p.nix+"/")
}

func (p legacyLinuxProc) readAt(directory int, name string, limit int) (data []byte, err error) {
	fd, err := unix.Openat(directory, name, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	buffer := make([]byte, 4096)
	for {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		n, readErr := p.read(fd, buffer)
		if readErr != nil {
			return nil, readErr
		}
		if n == 0 {
			return data, nil
		}
		if n < 0 || n > len(buffer) || len(data)+n > limit {
			return nil, errors.New("kernel process data exceeds its read bound")
		}
		data = append(data, buffer[:n]...)
	}
}

func (p legacyLinuxProc) linkAt(directory int, name string) (string, error) {
	if err := p.ctx.Err(); err != nil {
		return "", err
	}
	buffer := make([]byte, unix.PathMax+1)
	n, err := p.readlink(directory, name, buffer)
	if err != nil {
		return "", err
	}
	if n < 0 || n > unix.PathMax {
		return "", errors.New("kernel process path exceeds its read bound")
	}
	return string(buffer[:n]), nil
}

func (p legacyLinuxProc) status(directory int) (map[string]string, error) {
	data, err := p.readAt(directory, "status", 64<<10)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || key != "State" && key != "Threads" && key != "Kthread" && key != "FDSize" {
			continue
		}
		values := strings.Fields(value)
		if _, duplicate := fields[key]; duplicate || len(values) == 0 {
			return nil, errors.New("invalid kernel process status")
		}
		fields[key] = values[0]
	}
	return fields, nil
}

// pidfd readiness alone does not prove exit: a fresh pidfd can poll ready for a
// moment while a worker thread's exec takes over the leader PID (Linux 6.12,
// 2026-10-23: State R and a not-ready re-poll immediately afterwards). Readiness
// only qualifies the PID for a fresh lookup, which must find no thread-group
// leader. A disappearing procfs reference alone is not exit proof either, and an
// unreaped zombie still has its leader; its status proves no userspace instead.
func (p legacyLinuxProc) exited(pidfd, pid int) (bool, error) {
	if err := p.ctx.Err(); err != nil {
		return false, err
	}
	events := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
	n, err := p.poll(events, 0)
	if err != nil {
		return false, err
	}
	if n < 0 || n > 1 || events[0].Revents & ^int16(unix.POLLIN|unix.POLLHUP) != 0 {
		return false, errors.New("invalid kernel process-exit observation")
	}
	if n != 1 || events[0].Revents == 0 {
		return false, nil
	}
	fresh, err := p.openPID(pid, 0)
	if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOENT) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, unix.Close(fresh)
}

func (p legacyLinuxProc) noUserspace(directory, pidfd, pid int) (bool, error) {
	if exited, err := p.exited(pidfd, pid); exited || err != nil {
		return exited, err
	}
	fields, statusErr := p.status(directory)
	if statusErr == nil && (fields["Kthread"] == "1" || (fields["State"] == "Z" || fields["State"] == "X") && fields["Threads"] == "1") {
		return true, nil
	}
	// Older kernels expose PF_KTHREAD only through stat. The task name can
	// contain spaces, newlines, and ')', so parse after the last ') '.
	data, statErr := p.readAt(directory, "stat", 64<<10)
	if statErr == nil {
		text := string(data)
		if end := strings.LastIndex(text, ") "); end >= 0 {
			values := strings.Fields(text[end+2:])
			if len(values) >= 7 {
				flags, parseErr := strconv.ParseUint(values[6], 10, 64)
				if parseErr == nil && flags&(1<<21) != 0 {
					return true, nil
				}
			}
		}
	}
	if exited, err := p.exited(pidfd, pid); exited || err != nil {
		return exited, err
	}
	return false, errors.Join(statusErr, statErr)
}

func (p legacyLinuxProc) refuse(directory, pidfd, number int, pid, reference string, cause error) (string, error) {
	omit, proofErr := p.noUserspace(directory, pidfd, number)
	if omit && proofErr == nil {
		return "", nil
	}
	state, threads := "unknown", "unknown"
	if fields, err := p.status(directory); err == nil {
		if value := fields["State"]; len(value) == 1 && strings.Contains("RSDTtIZX", value) {
			state = value
		}
		if _, err := strconv.ParseUint(fields["Threads"], 10, 32); err == nil {
			threads = fields["Threads"]
		}
	}
	return "", fmt.Errorf("cannot inspect %s of live process %s (state=%s threads=%s): %w", reference, pid, state, threads, errors.Join(cause, proofErr))
}

func (p legacyLinuxProc) mapsUseNix(directory int) (found bool, err error) {
	fd, err := unix.Openat(directory, "maps", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	needle := []byte(" " + p.nix + "/")
	var tail []byte
	buffer := make([]byte, 64<<10)
	for {
		if err := p.ctx.Err(); err != nil {
			return false, err
		}
		n, readErr := p.read(fd, buffer)
		if readErr != nil {
			return false, readErr
		}
		if n < 0 || n > len(buffer) {
			return false, errors.New("invalid kernel mapping read")
		}
		if n == 0 {
			break
		}
		window := append(tail, buffer[:n]...)
		if bytes.Contains(window, needle) {
			return true, nil
		}
		tail = append([]byte(nil), window[max(0, len(window)-len(needle)+1):]...)
	}
	// This open file pins one mm. If exec or main-thread exit destroys that
	// view, reads return EOF without an error. Require the same view to remain
	// readable after the full scan; an mm's user count cannot rise from zero.
	if _, err := unix.Seek(fd, 0, unix.SEEK_SET); err != nil {
		return false, err
	}
	n, err := p.read(fd, buffer[:1])
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, errors.New("kernel mapping enumeration became incomplete")
	}
	return false, nil
}

// proc_readfd_common leaves its cursor at max_fds+2 after a complete table,
// but at the next descriptor+2 if the old leader loses its files during exec.
// A later positive FDSize bounds every continuously held descriptor. Matching
// that boundary proves it was not skipped by an early, successful EOF.
func (p legacyLinuxProc) descriptors(directory, fdDirectory int) ([]string, error) {
	var names []string
	buffer := make([]byte, 32<<10)
	for {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		n, err := p.getdents(fdDirectory, buffer)
		if err != nil {
			return nil, err
		}
		if n < 0 || n > len(buffer) {
			return nil, errors.New("invalid kernel descriptor listing")
		}
		if n == 0 {
			break
		}
		consumed, _, entries := unix.ParseDirent(buffer[:n], -1, nil)
		if consumed != n || len(names)+len(entries) > 1<<20 {
			return nil, errors.New("kernel descriptor listing exceeds its bound")
		}
		for _, name := range entries {
			value, err := strconv.ParseUint(name, 10, 31)
			if err != nil || strconv.FormatUint(value, 10) != name {
				return nil, errors.New("invalid kernel descriptor number")
			}
		}
		names = append(names, entries...)
	}
	position, err := unix.Seek(fdDirectory, 0, unix.SEEK_CUR)
	if err != nil {
		return nil, err
	}
	fields, err := p.status(directory)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseUint(fields["FDSize"], 10, 31)
	if err != nil || size == 0 || position != int64(size)+2 {
		return nil, errors.New("kernel descriptor listing ended before a verified table boundary")
	}
	return names, nil
}

func (p legacyLinuxProc) inspect(root int, name string, pid int) (reason string, err error) {
	pidfd, err := p.openPID(pid, 0)
	// PID is positive and flags are zero. EINVAL on older kernels and ENOENT
	// on newer ones mean this identity no longer has a thread-group leader
	// (a retained process-group ID or reuse by a nonleader), like ESRCH.
	if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOENT) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot hold kernel identity for process %s: %w", name, err)
	}
	defer func() { err = errors.Join(err, unix.Close(pidfd)) }()
	directory, err := unix.Openat(root, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		gone, proofErr := p.exited(pidfd, pid)
		if gone && proofErr == nil {
			return "", nil
		}
		// A worker's exec can briefly hide the leader's procfs entry.
		return "", fmt.Errorf("cannot inspect the directory of live process %s: %w", name, errors.Join(err, proofErr))
	}
	defer func() { err = errors.Join(err, unix.Close(directory)) }()
	exe, err := p.linkAt(directory, "exe")
	if err != nil {
		return p.refuse(directory, pidfd, pid, name, "executable", err)
	}
	if p.pathUsesNix(exe) {
		return exe, nil
	}
	if found, err := p.mapsUseNix(directory); err != nil {
		return p.refuse(directory, pidfd, pid, name, "mappings", err)
	} else if found {
		return "mapped /nix", nil
	}
	for _, reference := range []string{"cwd", "root"} {
		target, err := p.linkAt(directory, reference)
		if err != nil {
			return p.refuse(directory, pidfd, pid, name, "open path ("+reference+")", err)
		}
		if p.pathUsesNix(target) {
			return "open path in /nix", nil
		}
	}
	fdDirectory, err := unix.Openat(directory, "fd", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return p.refuse(directory, pidfd, pid, name, "open files", err)
	}
	// Keep the enumeration handle open through classification. In particular,
	// inspecting this helper must not repeatedly close and recreate its own fd.
	defer func() { err = errors.Join(err, unix.Close(fdDirectory)) }()
	names, err := p.descriptors(directory, fdDirectory)
	if err != nil {
		return p.refuse(directory, pidfd, pid, name, "open files", err)
	}
	var unread []string
	for _, number := range names {
		target, err := p.linkAt(directory, "fd/"+number)
		if err != nil {
			unread = append(unread, number)
		} else if p.pathUsesNix(target) {
			return "open path in /nix", nil
		}
	}
	if len(unread) == 0 {
		return "", nil
	}
	if _, err := unix.Seek(fdDirectory, 0, unix.SEEK_SET); err != nil {
		return p.refuse(directory, pidfd, pid, name, "open files", err)
	}
	again, err := p.descriptors(directory, fdDirectory)
	if err != nil {
		return p.refuse(directory, pidfd, pid, name, "open files", err)
	}
	present := make(map[string]bool, len(again))
	for _, number := range again {
		present[number] = true
	}
	for _, number := range unread {
		if !present[number] {
			continue // A complete second listing proves this descriptor closed.
		}
		target, err := p.linkAt(directory, "fd/"+number)
		if err != nil {
			return p.refuse(directory, pidfd, pid, name, "open path (fd/"+number+")", err)
		}
		if p.pathUsesNix(target) {
			return "open path in /nix", nil
		}
	}
	return "", nil
}
