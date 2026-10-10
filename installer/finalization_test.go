package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// This boundary fixture publishes real resource/completion files and a separate
// provider journal. It models a controller failure after provider completion;
// native provider fixtures separately exercise their actual recovered journals.
type finalizingFileDriver struct {
	*fileDriver
	interrupt, failFinalization bool
}

func (d *finalizingFileDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if err := os.WriteFile(filepath.Join(d.dir, "provider-journal"), []byte("incomplete"), 0600); err != nil {
		return Observation{}, err
	}
	o, err := d.fileDriver.Apply(ctx, r, op, receipt)
	if err == nil && d.interrupt {
		d.interrupt = false
		return Observation{}, errors.New("controller stopped after proved provider completion")
	}
	return o, err
}

func (d *finalizingFileDriver) FinishTransaction(_ context.Context, plan Plan, _ map[string]Receipt) (map[string][]string, error) {
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Transaction == nil {
		return nil, errors.Join(errors.New("core intent was cleared before provider finalization"), err)
	}
	if unlock, err := Lock(filepath.Dir(d.statePath)); err == nil {
		return nil, errors.Join(errors.New("provider finalization ran outside the mutation lock"), unlock())
	}
	if plan.ID == "" || d.failFinalization {
		return nil, errors.New("provider journal publication failed")
	}
	return nil, os.WriteFile(filepath.Join(d.dir, "provider-journal"), []byte("complete"), 0600)
}

func TestRecoveredTransactionSealsProviderBeforeClearingCoreIntent(t *testing.T) {
	for _, action := range []string{"retry", "abandon", "failed-finalization"} {
		t.Run(action, func(t *testing.T) {
			c, files := engineFixture(t)
			d := &finalizingFileDriver{fileDriver: files, interrupt: true}
			request := approve(t, c, files, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
			if _, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, d); err == nil {
				t.Fatal("fixture did not interrupt after publication")
			}
			request = Request{Schema: 1, Mode: "apply", Retry: true}
			if action == "abandon" {
				request = Request{Schema: 1, Mode: "abandon"}
			}
			d.failFinalization = action == "failed-finalization"
			request = approve(t, c, files, request)
			result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, d)
			state, stateErr := LoadState(d.statePath, d.home)
			if stateErr != nil {
				t.Fatal(stateErr)
			}
			if d.failFinalization {
				if err == nil || result.Status == "ready" || state.Transaction == nil {
					t.Fatalf("provider publication failure discarded recoverable intent: %+v %v", result, err)
				}
				return
			}
			if err != nil || result.Status != "ready" || state.Transaction != nil {
				t.Fatalf("recovery failed: %+v %v", result, err)
			}
			if action == "abandon" {
				for id, receipt := range state.Receipts {
					if receipt.Status == "in-progress" {
						t.Fatal("abandon left a receipt in progress", id, receipt)
					}
				}
			}
			journal, err := os.ReadFile(filepath.Join(d.dir, "provider-journal"))
			if err != nil || string(journal) != "complete" {
				t.Fatalf("core reported recovery while provider intent remained incomplete: %q %v", journal, err)
			}
		})
	}
}
