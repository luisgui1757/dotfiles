package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// X's tree enumeration is not a snapshot: a child can disappear before its
// geometry/property request. Keep diagnostics separate from window names and
// accept absence only after typed error/resource checks and fresh native proof.
type desktopXQuery func(context.Context, string, ...string) ([]byte, []byte, error)

func desktopXCommand(ctx context.Context, program string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, program, args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr boundedCommandOutput
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.data.Bytes(), stderr.data.Bytes(), errors.Join(err, ctx.Err())
}

func desktopLinuxOwnedWindow(ctx context.Context, ids []int, query desktopXQuery) (bool, error) {
	output, diagnostic, err := query(ctx, "/usr/bin/xwininfo", "-root", "-tree")
	if err != nil || len(diagnostic) != 0 {
		return false, desktopXFailure(ctx, query, "tree", "", diagnostic, err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "0x") {
			continue
		}
		window := fields[0]
		if _, err := strconv.ParseUint(window[2:], 16, 32); err != nil {
			return false, fmt.Errorf("invalid X window identity %q", window)
		}
		property, diagnostic, err := query(ctx, "/usr/bin/xprop", "-id", window, "_NET_WM_PID")
		if err != nil || len(diagnostic) != 0 {
			if err := desktopXFailure(ctx, query, "property", window, diagnostic, err); err != nil {
				return false, err
			}
			continue
		}
		_, value, ok := strings.Cut(string(property), "=")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || !slices.Contains(ids, pid) {
			continue
		}
		stats, diagnostic, err := query(ctx, "/usr/bin/xwininfo", "-id", window, "-stats")
		if err != nil || len(diagnostic) != 0 {
			if err := desktopXFailure(ctx, query, "stats", window, diagnostic, err); err != nil {
				return false, err
			}
			continue
		}
		if desktopXViewable.Match(stats) {
			return true, nil
		}
	}
	return false, nil
}

var (
	desktopXRoot     = regexp.MustCompile(`^\s*xwininfo: Window id: (0x[0-9a-f]+) `)
	desktopXViewable = regexp.MustCompile(`(?m)^  Map State: IsViewable$`)
	// xwininfo/dsimple.c Print_X_Error emits these native numeric fields.
	desktopXError = regexp.MustCompile(`^X Error: (3|9): [^\n]+\n  Request Major code: ([0-9]+)(?: \([^\n]+\))?\n  ResourceID in failed request: (0x[0-9a-f]+)\n  Serial number of failed request: [0-9]+\n`)
	// xprop uses Xlib's default error handler instead of xwininfo's XCB printer.
	desktopXPropertyError = regexp.MustCompile(`^X Error of failed request:  BadWindow \(invalid Window parameter\)\n  Major opcode of failed request:  20 \(X_GetProperty\)\n  Resource id in failed request:  (0x[0-9a-f]+)\n  Serial number of failed request:  [0-9]+\n  Current serial number in output stream:  [0-9]+\n$`)
)

func desktopXVanishedID(kind, window string, diagnostic []byte) string {
	if kind == "property" {
		m := desktopXPropertyError.FindSubmatch(diagnostic)
		if m != nil && string(m[1]) == window {
			return window
		}
		return ""
	}
	rest, id, last, count := string(diagnostic), "", "", 0
	for {
		m := desktopXError.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		if !(m[1] == "9" && m[2] == "14" || kind == "tree" && m[1] == "3" && m[2] == "15") || id != "" && id != m[3] {
			return ""
		}
		id, rest, last, count = m[3], rest[len(m[0]):], m[1]+"/"+m[2], count+1
	}
	ending := "/usr/bin/xwininfo: error: Can't query window tree.\n"
	if kind == "stats" {
		ending = "/usr/bin/xwininfo: error: No such window with id " + window + ".\n"
	}
	if id == "" || rest != ending || window != "" && id != window ||
		kind == "tree" && (last != "3/15" || count > 2) || kind == "stats" && count != 1 {
		return ""
	}
	return id
}

func desktopXFailure(ctx context.Context, query desktopXQuery, kind, window string, diagnostic []byte, cause error) error {
	if cause == nil {
		cause = errors.New("unexpected X diagnostic")
	}
	failure := fmt.Errorf("inspect disposable X %s: %w: %s", kind, errors.Join(cause, ctx.Err()), diagnostic)
	var exited *exec.ExitError
	if ctx.Err() != nil || !errors.As(cause, &exited) || exited.ExitCode() != 1 {
		return failure
	}
	id := desktopXVanishedID(kind, window, diagnostic)
	if id == "" {
		return failure
	}
	root, stderr, err := query(ctx, "/usr/bin/xwininfo", "-root", "-stats")
	match := desktopXRoot.FindSubmatch(root)
	if err != nil || len(stderr) != 0 || match == nil || string(match[1]) == id {
		return errors.Join(failure, fmt.Errorf("verify live X root: %v: %s", err, stderr))
	}
	// Require Window-specific BadWindow from GetProperty, not drawable absence.
	// A successful lookup, unrecognized diagnostic, changed resource, or display
	// failure cannot prove disappearance. Do not reuse the failed tree's stdout.
	_, stderr, err = query(ctx, "/usr/bin/xprop", "-id", id, "_NET_WM_PID")
	if ctx.Err() != nil || !errors.As(err, &exited) || exited.ExitCode() != 1 || desktopXVanishedID("property", id, stderr) != id {
		return errors.Join(failure, fmt.Errorf("verify vanished X child %s: %v: %s", id, err, stderr))
	}
	return nil
}
