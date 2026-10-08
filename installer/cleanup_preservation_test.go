package installer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestConfigurationCleanupRetryKeepsPreviouslyPreservedFile(t *testing.T) {
	_, d, _, j := interruptedConfigController(t, "update", "completed-unsealed")
	previous := filepath.Join(j.Entries[0].Workspace, "previous")
	writeConfigFixture(t, previous, "personal edited copy")
	// The first cleanup already disclosed this file, then died after saving
	// its deletion intent and before sealing the journal.
	j.Preserved = append(j.Preserved, previous)
	j.Entries[0].Discarding = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	preserved, err := d.FinishTransaction(context.Background(), Plan{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(previous)
	if err != nil || string(data) != "personal edited copy" {
		t.Fatalf("cleanup deleted a previously preserved file: %q %v", data, err)
	}
	if !slices.Contains(preserved[j.Resource], previous) {
		t.Fatal("lost preserved path", preserved)
	}
}

func TestConfigurationLegacyDiscardingFlagDoesNotAuthorizeChangedFile(t *testing.T) {
	_, d, _, j := interruptedConfigController(t, "update", "completed-unsealed")
	previous := filepath.Join(j.Entries[0].Workspace, "previous")
	j.Entries[0].Discarding = true
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, previous, "new personal edits after interruption")
	preserved, err := d.FinishTransaction(context.Background(), Plan{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(previous)
	if err != nil || string(data) != "new personal edits after interruption" {
		t.Fatalf("cleanup took deletion authority from a boolean: %q %v", data, err)
	}
	if !slices.Contains(preserved[j.Resource], previous) {
		t.Fatal("lost preserved path", preserved)
	}
}
