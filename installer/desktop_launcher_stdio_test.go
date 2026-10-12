package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// GUI descendants may inherit launcher output and live beyond the launcher.
// Regular OS handles let Wait report that launcher's exit without waiting for
// descendant-held copy pipes. The fixture later proves ownership/window/settings
// and stops only its own GUI processes before reading the final bounded log.
func runDesktopLauncher(ctx context.Context, command *exec.Cmd, logPath string) error {
	output, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	command.Stdout, command.Stderr = output, output
	return errors.Join(command.Run(), ctx.Err(), output.Close())
}

func readDesktopLauncherOutput(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	err = errors.Join(err, file.Close())
	if len(data) > 1<<20 {
		return data[:1<<20], errors.Join(err, errors.New("desktop launcher output exceeded 1 MiB"))
	}
	return data, err
}

// The fixture child is bounded even if its parent assertion fails. It writes
// only under the test's private directory and never launches a GUI application.
func TestDesktopLauncherFixtureChild(t *testing.T) {
	role, directory := os.Getenv("DOTFILES_DESKTOP_LAUNCH_ROLE"), os.Getenv("DOTFILES_DESKTOP_LAUNCH_DIRECTORY")
	switch role {
	case "launcher":
		child := exec.Command(os.Args[0], "-test.run=^TestDesktopLauncherFixtureChild$")
		child.Env = append(os.Environ(), "DOTFILES_DESKTOP_LAUNCH_ROLE=child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Println("launcher exited successfully")
		os.Exit(0)
	case "child":
		fmt.Println("live child stdout")
		fmt.Fprintln(os.Stderr, "live child stderr")
		if err := os.WriteFile(filepath.Join(directory, "ready"), []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(directory, "release")); err == nil {
				if err := errors.Join(os.Stdout.Close(), os.Stderr.Close()); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "finished"), []byte("finished"), 0600); err != nil {
					t.Fatal(err)
				}
				os.Exit(0)
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("private child did not receive its release")
	case "nonzero":
		fmt.Fprintln(os.Stderr, "launcher rejected input")
		os.Exit(19)
	case "blocked":
		time.Sleep(8 * time.Second)
		os.Exit(0)
	}
}

func desktopLauncherFixtureCommand(ctx context.Context, role, directory string) *exec.Cmd {
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDesktopLauncherFixtureChild$")
	command.Env = append(os.Environ(), "DOTFILES_DESKTOP_LAUNCH_ROLE="+role, "DOTFILES_DESKTOP_LAUNCH_DIRECTORY="+directory)
	return command
}

func TestDesktopLauncherReturnsWhileLiveChildKeepsOutputOpen(t *testing.T) {
	directory := t.TempDir()
	waitFile := func(name string) bool {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
				return true
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return false
	}
	release := func() {
		if err := os.WriteFile(filepath.Join(directory, "release"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		if !waitFile("finished") {
			t.Fatal("private child did not exit after release")
		}
	}
	t.Cleanup(release)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := desktopLauncherFixtureCommand(ctx, "launcher", directory)
	path := filepath.Join(directory, "launcher.log")
	done := make(chan error, 1)
	go func() { done <- runDesktopLauncher(ctx, command, path) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		// Release the exact private child so the failed-before regression also
		// joins the waiter; it must not leave a goroutine or descendant behind.
		release()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("launcher waiter did not finish after releasing its child")
		}
		t.Fatal("successful launcher waited beyond its deadline for child-held output")
	}
	if command.ProcessState.ExitCode() != 0 || !waitFile("ready") {
		t.Fatal("private launcher/child did not start correctly", command.ProcessState)
	}
	if _, err := os.Stat(filepath.Join(directory, "finished")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("launcher return required child exit", err)
	}
	output, err := readDesktopLauncherOutput(path)
	if err != nil || !bytes.Contains(output, []byte("live child stdout")) || !bytes.Contains(output, []byte("live child stderr")) {
		t.Fatal("file capture lost child output", string(output), err)
	}
}

func TestDesktopLauncherPreservesExitAndContextFailures(t *testing.T) {
	for _, role := range []string{"nonzero", "blocked"} {
		t.Run(role, func(t *testing.T) {
			directory := t.TempDir()
			timeout := 2 * time.Second
			if role == "blocked" {
				timeout = 200 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			path := filepath.Join(directory, "launcher.log")
			err := runDesktopLauncher(ctx, desktopLauncherFixtureCommand(ctx, role, directory), path)
			if role == "blocked" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("lost launcher cancellation", err)
				}
				return
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 19 {
				t.Fatal("lost nonzero launcher exit", err)
			}
			output, readErr := readDesktopLauncherOutput(path)
			if readErr != nil || !bytes.Contains(output, []byte("launcher rejected input")) {
				t.Fatal("lost launcher failure diagnostic", string(output), readErr)
			}
		})
	}
}

func TestDesktopLauncherOutputIsBounded(t *testing.T) {
	for _, size := range []int{0, 1 << 20, (1 << 20) + 1} {
		path := filepath.Join(t.TempDir(), "launcher.log")
		if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, size), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := readDesktopLauncherOutput(path)
		if (err != nil) != (size > 1<<20) || len(data) != min(size, 1<<20) {
			t.Fatal("launcher diagnostic bound differs", size, len(data), err)
		}
	}
}
