package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestApplyLogRecordsStepsBesideState(t *testing.T) {
	c, d := engineFixture(t)
	progress := &recordedProgress{}
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(progress.context(), c, linux, "source", d.home, d.statePath, request, d)
	if err != nil || result.Status != "ready" || filepath.Dir(result.Log) != filepath.Join(filepath.Dir(d.statePath), "logs") {
		t.Fatal(result, err)
	}
	data, err := os.ReadFile(result.Log)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range progress.steps {
		if !strings.Contains(string(data), " "+step+"\n") {
			t.Fatalf("log lacks displayed step %q:\n%s", step, data)
		}
	}
	if info, err := os.Stat(result.Log); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("log is not private", info, err)
	}
}

func TestApplyLogKeepsOnlyTheNewestLogs(t *testing.T) {
	c, d := engineFixture(t)
	directory := filepath.Join(filepath.Dir(d.statePath), "logs")
	if err := prepareStateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	old := []string{}
	for i := range keptApplyLogs + 5 {
		name := fmt.Sprintf("apply-2026010%dT0000%02dZ-1.log", i/10, i%100)
		old = append(old, name)
		writeConfigFixture(t, filepath.Join(directory, name), "old")
	}
	writeConfigFixture(t, filepath.Join(directory, "personal notes.log"), "keep")
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := applyFixture(c, d, request)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(old)
	want := append(append([]string{}, old[len(old)-keptApplyLogs+1:]...), filepath.Base(result.Log), "personal notes.log")
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Fatal("log retention removed the wrong files", names)
	}
}

func TestUnavailableApplyLogIsANoteNotAFailure(t *testing.T) {
	c, d := engineFixture(t)
	if err := prepareStateDirectory(filepath.Dir(d.statePath)); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(filepath.Dir(d.statePath), "logs"), "not a directory")
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, d)
	if err != nil || result.Status != "ready" || result.Log != "" || !strings.Contains(result.Message, "Apply log issue: ") {
		t.Fatal(result, err)
	}
}
