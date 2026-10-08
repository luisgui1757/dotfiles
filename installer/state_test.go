package installer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredPrototypeBatchStateIsRejectedAndPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data := []byte(`{"schema":1,"target":"home","transaction":{"in_flight":"nix-grow","in_flight_resources":["node"]}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path, "home"); err == nil {
		t.Fatal("retired prototype batch silently entered single-resource recovery")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("unsupported prototype state was modified", err)
	}
}

func TestStateReplacementWhilePreviewReaderIsOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state, _ := installed()
	state.Target, state.Generation = dir, 1
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := root.Open("state.json")
	if err != nil {
		if closeErr := root.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		t.Fatal(err)
	}
	state.Generation = 2
	writeErr := SaveState(path, state)
	oldData, readErr := io.ReadAll(reader)
	readerErr, rootErr := reader.Close(), root.Close()
	if writeErr != nil || readErr != nil || readerErr != nil || rootErr != nil {
		t.Fatalf("publication during preview: %v; read %v; reader %v; root %v", writeErr, readErr, readerErr, rootErr)
	}
	var old State
	if err := Decode(oldData, &old); err != nil || old.Generation != 1 {
		t.Fatalf("open preview lost its complete old snapshot: generation %d, error %v", old.Generation, err)
	}
	got, err := LoadState(path, dir)
	if err != nil || got.Generation != 2 {
		t.Fatalf("published state: %+v %v", got, err)
	}
}

func TestRejectedStatePublicationPreservesPreviousDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state, _ := installed()
	state.Target = "home"
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state.Source = strings.Repeat("x", 8*1024*1024)
	if err := SaveState(path, state); err == nil {
		t.Fatal("published an unreadably large ledger")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected publication changed the old ledger", err)
	}
}

func TestStateRejectsTargetMismatchAndEscapingRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state, _ := installed()
	state.Target = "original-home"
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path, "other-home"); err == nil {
		t.Fatal("another target accepted the ledger")
	}
	receipt := state.Receipts["node"]
	receipt.Recovery = filepath.Join("..", "outside")
	state.Receipts["node"] = receipt
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path, "original-home"); err == nil {
		t.Fatal("escaping recovery reference accepted")
	}
}

func TestNewerSchemaWithNewFieldsHasUsefulDiagnostic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema":2,"future_field":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path, "home"); err == nil || !strings.Contains(err.Error(), "unsupported state schema 2") {
		t.Fatalf("newer engine diagnostic: %v", err)
	}
}
