package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyIrreversibleDispatchIgnoresAtomicWriteStaging(t *testing.T) {
	for _, complete := range []bool{false, true} {
		name := "partial document"
		if complete {
			name = "complete unpublished document"
		}
		t.Run(name, func(t *testing.T) {
			d, _ := legacyStepFixture(t)
			command := d.systemCommand(false, "/bin/sh", "-c", "exit 0")
			intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "fixture", Command: &command, Irreversible: true})
			intent.Resource, intent.Before.Identity = "retire.nix", "retire.nix"
			if err := os.Remove(d.intentPath(intent.Operation)); err != nil {
				t.Fatal(err)
			}
			command, err := d.stepCommand(*intent, 0)
			if err != nil {
				t.Fatal(err)
			}
			if output, err := d.Run(context.Background(), command); err != nil {
				t.Fatalf("fixture worker: %v%s", err, nativeDiagnostic(output))
			}
			data := []byte("{")
			if complete {
				data, err = json.Marshal(intent)
				if err != nil {
					t.Fatal(err)
				}
			}
			staging := filepath.Join(d.Directory, "operations", ".state-fixture")
			if err := os.WriteFile(staging, data, 0600); err != nil {
				t.Fatal(err)
			}
			if dispatched, err := legacyIrreversibleDispatched(d); err != nil || dispatched {
				t.Fatal("unpublished staging was treated as a committed journal", dispatched, err)
			}
			if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
				t.Fatal(err)
			}
			if dispatched, err := legacyIrreversibleDispatched(d); err != nil || !dispatched {
				t.Fatal("committed irreversible dispatch was not observed", dispatched, err)
			}
		})
	}
}

func TestLegacyIrreversibleDispatchRejectsMalformedCommittedJournal(t *testing.T) {
	d, _ := legacyStepFixture(t)
	writeConfigFixture(t, d.intentPath(strings.Repeat("a", 64)), "{")
	if dispatched, err := legacyIrreversibleDispatched(d); err == nil || dispatched {
		t.Fatal("malformed committed journal was ignored", dispatched, err)
	}
}

func TestLegacyIrreversibleDispatchRejectsNonFileCommittedJournal(t *testing.T) {
	d, _ := legacyStepFixture(t)
	if err := os.MkdirAll(d.intentPath(strings.Repeat("a", 64)), 0700); err != nil {
		t.Fatal(err)
	}
	if dispatched, err := legacyIrreversibleDispatched(d); err == nil || dispatched {
		t.Fatal("non-file committed journal was ignored", dispatched, err)
	}
}

func TestLegacyIrreversibleDispatchDuringAtomicPublication(t *testing.T) {
	d, _ := legacyStepFixture(t)
	command := d.systemCommand(false, "/bin/sh", "-c", "exit 0")
	intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "fixture", Command: &command, Irreversible: true})
	intent.Resource, intent.Before.Identity = "retire.nix", "retire.nix"
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		defer close(done)
		for range 1000 {
			if ctx.Err() != nil {
				return
			}
			if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
				done <- err
				return
			}
		}
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error("concurrent publication failed", err)
		}
	}()
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal("concurrent publication failed", err)
			}
			return
		default:
		}
		if dispatched, err := legacyIrreversibleDispatched(d); err != nil || dispatched {
			t.Fatal("concurrent publication without a worker record is not dispatch", dispatched, err)
		}
	}
}
