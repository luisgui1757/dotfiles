package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeSessionStartsOnlyUnderMutationAndReleasesBothLocks(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "native", "worker")
	starts := 0
	s := &nativeSession{Directory: directory, Start: func(context.Context) (*nativeWorkerClient, error) {
		starts++
		return workerFixture(directory)
	}}
	d := &NativeDriver{StatePath: filepath.Join(root, "state.json"), session: s}
	preview, err := d.AcquirePreview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := preview(); err != nil {
		t.Fatal(err)
	}
	request := workerCommand(directory, "echo")
	request.Input = []byte("native output")
	if _, err := s.run(context.Background(), request); err == nil {
		t.Fatal("unguarded command was accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 || starts != 0 {
		t.Fatal("preview initialized native state", entries, err, starts)
	}
	engine, err := Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine(); err != nil {
			t.Error(err)
		}
	})
	guard, err := d.AcquireMutation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := guard(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) || starts != 0 {
		t.Fatal("non-native mutation initialized the worker", err, starts)
	}
	guard, err = d.AcquireMutation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := guard(); err != nil {
			t.Error(err)
		}
	})
	for range 2 {
		output, err := s.run(context.Background(), request)
		if err != nil || string(output) != "native output" {
			t.Fatal("worker handoff did not execute the command", string(output), err)
		}
	}
	if starts != 1 {
		t.Fatal("commands did not share their lifetime guard", starts)
	}
	if release, err := Lock(directory); err == nil {
		release()
		t.Fatal("session released the native lock between commands")
	}
	if release, err := d.AcquirePreview(context.Background()); err == nil {
		release()
		t.Fatal("preview entered an active controller")
	}
	if err := guard(); err != nil {
		t.Fatal(err)
	}
	release, err := Lock(directory)
	if err != nil {
		t.Fatal("completed worker retained the provider lock", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.run(context.Background(), request); err == nil {
		t.Fatal("closed session accepted more commands")
	}
}
