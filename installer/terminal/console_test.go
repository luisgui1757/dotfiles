package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/term"
)

func TestTerminalChild(t *testing.T) {
	mode := os.Getenv("DOTFILES_TEST_TERMINAL_CHILD")
	if mode == "" {
		return
	}
	before, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if mode == "pi-theme" {
		testPiThemeChild(t)
		return
	}
	if strings.HasPrefix(mode, "workflow-") {
		testWorkflowChild(t, mode, before)
		return
	}
	console, err := Open(os.Stdin, os.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if mode == "deadline" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Second)
		defer cancel()
	}
	options := []Option{
		{ID: "neovim", Label: "Neovim", Detail: "Editor and its required tools"},
		{ID: "pi", Label: "Pi", Detail: "Coding agent and shared Node runtime"},
		{ID: "github", Label: "GitHub CLI", Detail: "Repository commands"},
	}
	choices, choiceErr := console.Choose(ctx, "Choose tools", options, nil, true)
	closeErr := console.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	// Exercise the next cooked-input consumer, too. Darwin sets the transient
	// PENDIN bit when restoring ICANON; the next read processes that pending
	// state. Comparing before this read mistakes kernel state for a lost mode.
	fmt.Println("Ready for ordinary input")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || strings.TrimRight(line, "\r\n") != "ordinary input" {
		t.Fatalf("cooked input after selector: %q, %v", line, err)
	}
	after, stateErr := term.GetState(int(os.Stdin.Fd()))
	if closeErr != nil || stateErr != nil {
		t.Fatalf("restore: %v %v", closeErr, stateErr)
	}
	if !reflect.DeepEqual(before, after) {
		fmt.Printf("terminal state before=%#v after=%#v\n", before, after)
	}
	result := struct {
		Choices   []string `json:"choices"`
		Cancelled bool     `json:"cancelled"`
		Deadline  bool     `json:"deadline"`
		Restored  bool     `json:"restored"`
	}{choices, errors.Is(choiceErr, ErrCancelled), errors.Is(choiceErr, context.DeadlineExceeded), reflect.DeepEqual(before, after)}
	if choiceErr != nil && !result.Cancelled && !result.Deadline {
		t.Fatal(choiceErr)
	}
	fmt.Printf("\nchoices=[%s]\ncancelled=%v\ndeadline=%v\nrestored=%v\n", strings.Join(result.Choices, ","), result.Cancelled, result.Deadline, result.Restored)
}

type nativeSession struct {
	input           *os.File
	output          *os.File
	done            chan error
	stop            func() error
	finish          func() error
	resize          func(int, int) error
	mu              sync.Mutex
	data            []byte
	readErr         error
	cookedInputSent bool
}

func watchSession(t *testing.T, s *nativeSession) *nativeSession {
	t.Helper()
	go func() {
		var chunk [4096]byte
		for {
			n, err := s.output.Read(chunk[:])
			s.mu.Lock()
			if len(s.data)+n <= 1024*1024 {
				s.data = append(s.data, chunk[:n]...)
			} else {
				s.readErr = errors.New("terminal output exceeded 1 MiB")
			}
			if err != nil && !normalTerminalEnd(err) {
				s.readErr = errors.Join(s.readErr, err)
			}
			s.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		if !t.Failed() {
			if err := s.finish(); err != nil {
				t.Errorf("native terminal child exit: %v", err)
			}
		}
		if err := s.stop(); err != nil {
			t.Errorf("close native terminal session: %v", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.readErr != nil {
			t.Errorf("read native terminal: %v", s.readErr)
		}
	})
	return s
}

func (s *nativeSession) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.data)
}

func (s *nativeSession) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		output := s.text()
		if !s.cookedInputSent && strings.Contains(output, "Ready for ordinary input") {
			s.cookedInputSent = true
			s.send(t, "ordinary input\r")
		}
		if strings.Contains(output, want) {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("native terminal did not produce %q; output: %q", want, s.text())
		case <-tick.C:
		}
	}
}

func (s *nativeSession) send(t *testing.T, input string) {
	t.Helper()
	if _, err := io.WriteString(s.input, input); err != nil {
		t.Fatal(err)
	}
}

func TestNativeTerminalSelectsToolsWithArrowKeys(t *testing.T) {
	s := startNativeSession(t, "select")
	s.waitFor(t, "Choose tools")
	s.send(t, " \x1b[B \r")
	s.waitFor(t, "choices=[neovim,pi]")
	s.waitFor(t, "restored=true")
}

func TestNativeTerminalAllThenNoneIsExplicitEmptySelection(t *testing.T) {
	s := startNativeSession(t, "select")
	s.waitFor(t, "Choose tools")
	s.send(t, "an\r")
	s.waitFor(t, "choices=[]")
	s.waitFor(t, "cancelled=false")
	s.waitFor(t, "restored=true")
}

func TestNativeTerminalCancellationRestoresInput(t *testing.T) {
	for _, input := range []string{"q", "\x03", "\x04", "\x1b"} {
		t.Run(fmt.Sprintf("key-%x", []byte(input)), func(t *testing.T) {
			s := startNativeSession(t, "select")
			s.waitFor(t, "Choose tools")
			s.send(t, input)
			s.waitFor(t, "cancelled=true")
			s.waitFor(t, "restored=true")
		})
	}
}

func TestNativeTerminalDeadlineDoesNotLeaveRawInput(t *testing.T) {
	s := startNativeSession(t, "deadline")
	s.waitFor(t, "deadline=true")
	s.waitFor(t, "restored=true")
}

func TestNativeTerminalResizeKeepsSelectionUsable(t *testing.T) {
	s := startNativeSession(t, "select")
	s.waitFor(t, "Choose tools")
	if err := s.resize(32, 10); err != nil {
		t.Fatal(err)
	}
	s.send(t, "jj \r")
	s.waitFor(t, "choices=[github]")
	s.waitFor(t, "restored=true")
}

func TestRedirectedInputIsRejectedBeforeReading(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if console, err := Open(input, os.Stdout); err == nil {
		if closeErr := console.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("redirected input accepted an implicit interactive selection")
	}
}
