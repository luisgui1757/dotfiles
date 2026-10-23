package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedProgress struct {
	mutex  sync.Mutex
	steps  []string
	output bytes.Buffer
}

func (p *recordedProgress) Write(data []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.output.Write(data)
}

func (p *recordedProgress) text() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.output.String()
}

func (p *recordedProgress) context() context.Context {
	return WithProgress(context.Background(), Progress{Step: func(step string) {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		p.steps = append(p.steps, step)
	}, Output: p})
}

func TestExecuteNumbersEachApprovedChangeAndItsOutcome(t *testing.T) {
	c, d := engineFixture(t)
	progress := &recordedProgress{}
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(progress.context(), c, linux, "source", d.home, d.statePath, request, d)
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	want := []string{"Checking the system against the approved plan", "[1/2] Installing Node", "[1/2] Node done in", "[2/2] Installing Plugins", "[2/2] Plugins done in", "Verifying the resulting installation"}
	if len(progress.steps) != len(want) {
		t.Fatal("unexpected progress", progress.steps)
	}
	for i, step := range progress.steps {
		if !strings.HasPrefix(step, want[i]) {
			t.Fatal("unexpected progress", progress.steps)
		}
	}
	c, d = engineFixture(t)
	d.fail = "node"
	progress = &recordedProgress{}
	request = approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
	if _, err := Execute(progress.context(), c, linux, "source", d.home, d.statePath, request, d); err == nil {
		t.Fatal("fixture did not fail")
	}
	if !slices.ContainsFunc(progress.steps, func(step string) bool { return strings.HasPrefix(step, "[1/1] Node failed after") }) {
		t.Fatal("failed change was not named", progress.steps)
	}
}

func TestControllerBackReturnsToMainMenuWithDraftSelection(t *testing.T) {
	c, d := controllerFixture(t)
	// First run opens the tools; back shows the menu. Going back from the
	// review reaches the menu again, and the tools reopen with the draft.
	u := &scriptedInteraction{t: t, accept: true, reviewBack: 1, answers: [][]string{goBack, {"apply"}, {"editor"}, {"apply"}, {"editor"}}}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "ready" || len(u.answers) != 0 {
		t.Fatal(result, err, u.answers)
	}
	tools, menu := "Choose the tools you want installed", "What would you like to do?"
	if !slices.Equal(u.titles, []string{tools, menu, tools, menu, tools}) {
		t.Fatal("back did not return to the main menu", u.titles)
	}
	if !slices.Equal(u.initials[4], []string{"editor"}) {
		t.Fatal("tool selection lost its draft", u.initials)
	}
	if len(u.plans) != 2 || !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
		t.Fatal("going back from the review applied changes", len(u.plans), d.calls)
	}
}

func TestControllerCheckReturnsToMainMenu(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"apply"}, []string{"editor"})
	u := &scriptedInteraction{t: t, answers: [][]string{goBack, {"check"}, {"exit"}}}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "cancelled" || len(u.answers) != 0 || len(u.reports) != 1 || u.reports[0].Plan.Mode != "check" {
		t.Fatal("check did not return to the menu", result, err, u.reports)
	}
	if len(u.titles) != 3 || len(d.calls) != 2 {
		t.Fatal("menu back or check changed something", u.titles, d.calls)
	}
}

func TestNativeWorkerStreamsOutputWhileTheCommandRuns(t *testing.T) {
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
	progress := &recordedProgress{}
	done := make(chan error, 1)
	var output []byte
	go func() {
		var err error
		output, err = client.run(progress.context(), workerCommand(root, "stream"))
		done <- err
	}()
	deadline := time.Now().Add(20 * time.Second)
	for !strings.Contains(progress.text(), "first line") {
		select {
		case err := <-done:
			t.Fatal("command finished before its output was displayed", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("running command output was not displayed", progress.text())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.WriteFile(filepath.Join(root, "release"), []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil || string(output) != "first line\nsecond line\n" || progress.text() != "first line\nsecond line\n" {
		t.Fatalf("evidence or display changed: %q %q %v", output, progress.text(), err)
	}
	// A saved result is replayed without running the command again; the
	// display says so instead of implying a new execution.
	request := workerCommand(root, "once")
	request.Operation = strings.Repeat("d", 64)
	for range 2 {
		progress = &recordedProgress{}
		if output, err := client.run(progress.context(), request); err != nil || string(output) != "installed once" {
			t.Fatal(string(output), err)
		}
	}
	if !strings.Contains(progress.text(), "already completed") || strings.Contains(progress.text(), "installed once") {
		t.Fatal("replayed command display", progress.text())
	}
	data, err := readDocument(filepath.Join(root, "commands", request.Operation+".json"))
	if err != nil || bytes.Contains(data, []byte(`"stream"`)) {
		t.Fatal("live display output entered durable evidence", string(data), err)
	}
}

func TestDownloadDisplaysOnlyVerifiedCompletion(t *testing.T) {
	data := archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}})
	pin, client, _ := archivePinFor(t, data, "tar.gz")
	progress := &recordedProgress{}
	if err := downloadArchive(progress.context(), client, pin, filepath.Join(t.TempDir(), "payload")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(progress.text(), "Downloaded tool (") || !strings.HasSuffix(progress.text(), "SHA-256 matches the reviewed pin\n") {
		t.Fatalf("download completion was not displayed: %q", progress.text())
	}
	pin.SHA256 = strings.Repeat("0", 64)
	progress = &recordedProgress{}
	if err := downloadArchive(progress.context(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil || strings.Contains(progress.text(), "matches") {
		t.Fatalf("corrupt download displayed as verified: %q %v", progress.text(), err)
	}
}
