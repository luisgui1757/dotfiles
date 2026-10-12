package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestInstalledLegacyRootStatusDistinguishesAbsenceFromInspectionFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX retirement helper")
	}
	root := t.TempDir()
	installed := newLegacyInstalled(t, root)
	file := filepath.Join(root, "personal")
	writeConfigFixture(t, file, "unchanged\n")
	dangling := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(root, "missing"), dangling); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join(root, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, path, want string
	}{
		{"missing", filepath.Join(root, "missing", "profile-1-link"), "absent\n"},
		{"file", file, "present\n"},
		{"dangling-link", dangling, "present\n"},
		{"not-a-directory", filepath.Join(file, "child"), ""},
		{"link-loop", filepath.Join(loop, "child"), ""},
		{"relative", "relative/profile", ""},
		{"empty", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := exec.Command(installed.binary, "internal-legacy-root-status", test.path).Output()
			if string(output) != test.want || (err == nil) != (test.want != "") {
				t.Fatalf("status = %q, error = %v; want %q", output, err, test.want)
			}
		})
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "unchanged\n" {
		t.Fatal("inspection changed the file", err)
	}
}

func TestLegacyInaccessibleRootNeedsPrivilegedAbsenceProof(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the denied directory permissions")
	}
	h := newLegacyNixHost(t, "darwin")
	private := filepath.Join(h.base, "private")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(private, "profile-1-link")
	h.link("/nix/var/nix/gcroots/auto/scratch", target)
	if err := os.Chmod(private, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(private, 0700); err != nil {
			t.Error(err)
		}
	})
	h.d.Environment.Root = false
	for _, test := range []struct {
		name, output string
		queryError   error
		blocked      bool
		failed       bool
	}{
		{"stale", "absent\n", nil, false, false},
		{"live", "present\n", nil, true, false},
		{"invalid", "", nil, false, true},
		{"no-credentials", "", errors.New("sudo requires credentials"), false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			h.d.Query = func(_ context.Context, command nativeCommand) ([]byte, error) {
				called = true
				argv := legacyFakeArgv(command)
				if command.Program != "/usr/bin/sudo" || command.Arguments[0] != "-n" || !slices.Equal(argv, []string{h.d.Environment.Executable, "internal-legacy-root-status", target}) {
					t.Fatal("unexpected privileged inspection", command)
				}
				return []byte(test.output), test.queryError
			}
			problems, err := h.d.nixOutsideRoots(context.Background())
			if !called || (err != nil) != test.failed || (len(problems) != 0) != test.blocked {
				t.Fatal("wrong root disposition", called, problems, err)
			}
			if test.blocked && !strings.Contains(problems[0], target) {
				t.Fatal("missing outside-consumer identity", problems)
			}
		})
	}
}
