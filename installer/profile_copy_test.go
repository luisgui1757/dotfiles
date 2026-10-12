package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileMetadataCopyNeverReplacesAnExistingFile(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "source"), filepath.Join(root, "existing")
	writeConfigFixture(t, from, "source")
	writeConfigFixture(t, to, "personal staging file")
	if err := copyProfileMetadata(context.Background(), from, to); err == nil {
		t.Fatal("accepted preexisting copy destination")
	}
	data, err := os.ReadFile(to)
	if err != nil || string(data) != "personal staging file" {
		t.Fatal("overwrote existing staging file", string(data), err)
	}
}

func TestCancelledProfileMetadataCopyLeavesNoDestination(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "source"), filepath.Join(root, "next")
	writeConfigFixture(t, from, "personal profile")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyProfileMetadata(ctx, from, to); err == nil {
		t.Fatal("cancelled copy succeeded")
	}
	if _, err := os.Lstat(to); !os.IsNotExist(err) {
		t.Fatal("cancelled copy left bytes", err)
	}
}
