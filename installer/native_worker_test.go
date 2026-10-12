package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeWorkerFixture(t *testing.T) {
	role := os.Getenv("DOTFILES_NATIVE_WORKER_FIXTURE")
	if role == "" {
		return
	}
	root := os.Args[len(os.Args)-1]
	var err error
	switch role {
	case "worker":
		err = RunNativeWorker(root, os.Stdin, os.Stdout)
	case "controller":
		signal.Reset(os.Interrupt)
		if err = enableNativeControllerInterrupt(); err != nil {
			break
		}
		var client *nativeWorkerClient
		client, err = workerFixture(root)
		if err == nil {
			request := workerCommand(root, "hold")
			request.Operation = strings.Repeat("a", 64)
			_, err = client.run(context.Background(), request)
			err = errors.Join(err, client.close())
		}
	case "hold":
		// Model a native/elevated child that survives the controlling terminal's
		// interrupt. The provider lock must survive for exactly this lifetime.
		signal.Ignore(os.Interrupt)
		err = os.WriteFile(filepath.Join(root, "started"), []byte("started"), 0600)
		if err == nil {
			err = awaitNativeFile(filepath.Join(root, "release"))
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(root, "finished"), []byte("finished"), 0600)
		}
		if err == nil {
			_, err = fmt.Fprint(os.Stdout, "operation completed")
		}
	case "once":
		var file *os.File
		file, err = os.OpenFile(filepath.Join(root, "once"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			err = file.Close()
		}
		if err == nil {
			_, err = fmt.Fprint(os.Stdout, "installed once")
		}
	case "exit-code":
		os.Exit(23)
	case "echo":
		_, err = io.Copy(os.Stdout, os.Stdin)
	case "environment":
		_, err = fmt.Fprint(os.Stdout, os.Getenv("DOTFILES_NATIVE_WORKER_VALUE"))
	case "diagnostic":
		fmt.Fprint(os.Stdout, strings.Repeat("progress\n", 1200)+"\x1b[31m")
		err = errors.New("native maintenance refused: fixture dependency")
	case "fail":
		fmt.Fprint(os.Stdout, "partial output")
		err = errors.New("fixture refusal")
	case "large-output":
		_, err = io.Copy(os.Stdout, strings.NewReader(strings.Repeat("x", 2<<20)))
	default:
		err = errors.New("unknown worker fixture")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func workerCommand(root, role string) nativeCommand {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return nativeCommand{Program: executable, Arguments: []string{"-test.run=^TestNativeWorkerFixture$", "--", root}, Environment: []string{"DOTFILES_NATIVE_WORKER_FIXTURE=" + role}}
}

func workerFixture(root string) (*nativeWorkerClient, error) {
	command := workerCommand(root, "worker")
	return startNativeWorker(context.Background(), command.Program, command.Arguments, command.Environment, os.Stderr)
}

func awaitNativeFile(path string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if err == nil {
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", path)
}

func TestNativeWorkerCommandBoundary(t *testing.T) {
	root := t.TempDir()
	client, err := workerFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.close(); err != nil {
			t.Error(err)
		}
	})
	if release, err := Lock(root); err == nil {
		t.Error("ready worker did not hold its native lock")
		release()
	}
	request := workerCommand(root, "echo")
	request.Input = []byte("exact package\tinstall\n")
	if output, err := client.run(context.Background(), request); err != nil || !bytes.Equal(output, request.Input) {
		t.Fatal("native stdin boundary", string(output), err)
	}
	request = workerCommand(root, "environment")
	request.Environment = append(request.Environment, "DOTFILES_NATIVE_WORKER_VALUE=exact value")
	if output, err := client.run(context.Background(), request); err != nil || string(output) != "exact value" {
		t.Fatal("native environment boundary", string(output), err)
	}
	if output, err := client.run(context.Background(), workerCommand(root, "fail")); err == nil || !bytes.Contains(output, []byte("partial output")) {
		t.Fatal("failed native command was hidden", string(output), err)
	}
	if _, err := client.run(context.Background(), workerCommand(root, "exit-code")); err == nil {
		t.Fatal("native exit code lost")
	} else {
		var failure *nativeCommandError
		if !errors.As(err, &failure) || failure.ExitCode == nil || *failure.ExitCode != 23 {
			t.Fatal("native exit code is not structured", err)
		}
	}
	if output, err := client.run(context.Background(), workerCommand(root, "large-output")); err == nil || len(output) != 1<<20 || !strings.Contains(err.Error(), "exceeded") {
		t.Fatal("native output bound", len(output), err)
	}
	request = workerCommand(root, "echo")
	request.Input = []byte("after failure")
	if output, err := client.run(context.Background(), request); err != nil || !bytes.Equal(output, request.Input) {
		t.Fatal("draining failed command lost the worker session", string(output), err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.run(cancelled, workerCommand(root, "hold")); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request started a native command", err)
	}
	if _, err := os.Stat(filepath.Join(root, "started")); !os.IsNotExist(err) {
		t.Fatal("cancelled request reached the process", err)
	}
	if err := client.close(); err != nil {
		t.Fatal(err)
	}
	if release, err := Lock(root); err != nil {
		t.Fatal("finished session left a native lock", err)
	} else if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorkerRetainsLockAfterControllerDeath(t *testing.T) {
	testNativeControllerLoss(t, nil, func(command *exec.Cmd) error { return command.Process.Kill() })
}

func testNativeControllerLoss(t *testing.T, prepare func(*exec.Cmd), stop func(*exec.Cmd) error) {
	t.Helper()
	root := t.TempDir()
	log, err := os.Create(filepath.Join(root, "controller.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	request := workerCommand(root, "controller")
	controller := exec.Command(request.Program, request.Arguments...)
	controller.Env = append(os.Environ(), request.Environment...)
	controller.Stdout, controller.Stderr = log, log
	if prepare != nil {
		prepare(controller)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The child has a bounded fixture lifetime even if this assertion fails.
		if err := os.WriteFile(filepath.Join(root, "release"), []byte("release"), 0600); err != nil {
			t.Error(err)
		}
		if controller.ProcessState == nil {
			if err := controller.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
			if err := controller.Wait(); err == nil {
				t.Error("terminated controller unexpectedly succeeded")
			}
		}
		if _, err := os.Stat(filepath.Join(root, "started")); err == nil {
			if err := awaitNativeFile(filepath.Join(root, "finished")); err != nil {
				t.Error(err)
			}
		}
	})
	if err := awaitNativeFile(filepath.Join(root, "started")); err != nil {
		data, readErr := os.ReadFile(log.Name())
		t.Fatal(err, readErr, string(data))
	}
	if err := stop(controller); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- controller.Wait() }()
	select {
	case err := <-exited:
		if err == nil {
			t.Fatal("controller survived termination")
		}
	case <-time.After(5 * time.Second):
		if err := controller.Process.Kill(); err != nil {
			t.Error(err)
		}
		<-exited
		t.Fatal("controller did not exit after the termination event")
	}
	until := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(until) {
		if release, err := Lock(root); err == nil {
			release()
			t.Fatal("controller death released the lock while its native command was active")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.WriteFile(filepath.Join(root, "release"), []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := awaitNativeFile(filepath.Join(root, "finished")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if release, err := Lock(root); err == nil {
			if err := release(); err != nil {
				t.Fatal(err)
			}
			data, err := readDocument(filepath.Join(root, "commands", strings.Repeat("a", 64)+".json"))
			if err != nil {
				t.Fatal("controller death lost native evidence", err)
			}
			var record nativeCommandRecord
			if err := Decode(data, &record); err != nil || record.Reply == nil || string(record.Reply.Output) != "operation completed" || record.Reply.Error != "" {
				t.Fatal("durable native completion", record, err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("completed native child left a stuck worker lock")
}

func TestNativeWorkerReplaysCompletedCommandWithoutRunningItAgain(t *testing.T) {
	root := t.TempDir()
	request := workerCommand(root, "once")
	request.Operation = strings.Repeat("b", 64)
	for range 2 {
		client, err := workerFixture(root)
		if err != nil {
			t.Fatal(err)
		}
		output, runErr := client.run(context.Background(), request)
		closeErr := client.close()
		if runErr != nil || closeErr != nil || string(output) != "installed once" {
			t.Fatal("completed command was repeated or lost", string(output), runErr, closeErr)
		}
	}
	request.Arguments = append(request.Arguments, "changed")
	if _, err := executeNativeCommand(root, request); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatal("changed request reused another operation's result", err)
	}
}

func TestNativeWorkerUnfinishedIntentCannotRepeatMutation(t *testing.T) {
	root := t.TempDir()
	request := workerCommand(root, "once")
	request.Operation = strings.Repeat("c", 64)
	hash, err := digest(request)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "commands", request.Operation+".json")
	// Missing reply is also the oldest persisted shape. It cannot mean success
	// or absence of side effects, even if the original command never started.
	if err := saveDocument(path, nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
		t.Fatal(err)
	}
	if _, err := executeNativeCommand(root, request); err == nil || !strings.Contains(err.Error(), "unfinished") {
		t.Fatal("uncertain command was repeated", err)
	}
	if _, err := os.Stat(filepath.Join(root, "once")); !os.IsNotExist(err) {
		t.Fatal("uncertain command started", err)
	}
}

// Hide strings.Reader.WriteTo, as os/exec's pipe readers do. An embedded
// bytes.Buffer accidentally promotes ReadFrom and bypasses the Write limit.
func TestPythonPreparationOutputCannotBypassBound(t *testing.T) {
	var output boundedCommandOutput
	count, err := io.Copy(&output, struct{ io.Reader }{strings.NewReader(strings.Repeat("x", 2<<20))})
	if err == nil || count > 1<<20 || output.data.Len() > 1<<20 {
		t.Fatal("Python output exceeded its bound", count, err)
	}
}

func TestNativeWorkerFailureIncludesBoundedDiagnostic(t *testing.T) {
	root := t.TempDir()
	client, err := workerFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.close(); err != nil {
			t.Error(err)
		}
	})
	_, err = client.run(context.Background(), workerCommand(root, "diagnostic"))
	if err == nil || !strings.Contains(err.Error(), "native maintenance refused: fixture dependency") {
		t.Fatal("native failure lost its actionable diagnostic", err)
	}
	if len(err.Error()) > 8400 || strings.ContainsRune(err.Error(), '\x1b') {
		t.Fatal("native failure exposed excessive output or terminal control sequences", len(err.Error()))
	}
	var failure *nativeCommandError
	if !errors.As(err, &failure) || failure.ExitCode == nil || *failure.ExitCode != 1 {
		t.Fatal("diagnostic lost the structured native exit code", err)
	}
}
