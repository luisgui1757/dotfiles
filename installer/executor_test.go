package installer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Real filesystem operations form the system boundary in these engine tests.
// Actual package management still requires native provider/entrypoint tests.
type fileDriver struct {
	dir, statePath, home string
	fail, pending        string
	leaveOnRemove        bool
	unhealthy            string
	absentPending        bool
	calls                []string
	preserved            map[string][]string
}

func (d *fileDriver) Observe(_ context.Context, r Resource, receipt Receipt) (Observation, error) {
	path := filepath.Join(d.dir, r.ID)
	var preserved []string
	for _, kept := range d.preserved[r.ID] {
		if _, err := os.Stat(kept); err == nil {
			preserved = append(preserved, kept)
		} else if !os.IsNotExist(err) {
			return Observation{}, err
		}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Observation{Preserved: preserved}, nil
	}
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: path, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(data)), Preserved: preserved}
	proof, proofErr := os.ReadFile(path + ".completion")
	if proofErr == nil {
		var completed Observation
		if err := Decode(proof, &completed); err != nil {
			return Observation{}, err
		}
		if receipt.OperationID != "" && completed.CompletedOperation == receipt.OperationID && sameArtifact(completed, o) {
			o.CompletedOperation = completed.CompletedOperation
		}
	} else if !errors.Is(proofErr, os.ErrNotExist) {
		return Observation{}, proofErr
	}
	if r.ID == d.unhealthy || r.ID == "plugins" && d.unhealthy == "node" {
		o.Healthy = false
	}
	if r.ID == d.pending {
		o.Healthy, o.Pending = false, "restart the host"
	}
	return o, nil
}

func (d *fileDriver) Apply(ctx context.Context, r Resource, _ Operation, receipt Receipt) (Observation, error) {
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		return Observation{}, err
	}
	if state.Transaction == nil || state.Transaction.InFlight != r.ID || state.Receipts[r.ID].Status != "in-progress" {
		return Observation{}, errors.New("mutation began without durable intent")
	}
	d.calls = append(d.calls, "apply:"+r.ID)
	if d.absentPending && r.ID == "node" {
		return Observation{Pending: "restart before the provider publishes its artifact"}, nil
	}
	if err := os.WriteFile(filepath.Join(d.dir, r.ID), []byte("installed"), 0600); err != nil {
		return Observation{}, err
	}
	if r.ID == d.fail {
		return Observation{}, errors.New("provider failed after publication")
	}
	if r.ID == d.unhealthy {
		d.unhealthy = ""
	}
	o, err := d.Observe(ctx, r, receipt)
	if err != nil {
		return Observation{}, err
	}
	o.CompletedOperation = receipt.OperationID
	data, err := json.Marshal(o)
	if err != nil {
		return Observation{}, err
	}
	if err := os.WriteFile(filepath.Join(d.dir, r.ID)+".completion", data, 0600); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *fileDriver) Remove(ctx context.Context, r Resource, _ Receipt) (Observation, error) {
	d.calls = append(d.calls, "remove:"+r.ID)
	if d.leaveOnRemove {
		return Observation{}, nil
	}
	if err := os.Remove(filepath.Join(d.dir, r.ID)); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, Receipt{})
}

func engineFixture(t *testing.T) (*Catalog, *fileDriver) {
	t.Helper()
	home := t.TempDir()
	d := &fileDriver{dir: home, home: home, statePath: filepath.Join(home, "ledger", "state.json")}
	return testCatalog(t), d
}

func approve(t *testing.T, c *Catalog, d *fileDriver, request Request) Request {
	t.Helper()
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(context.Background(), c, linux, state, request, d)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanChanges(c, linux, state, request, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	return request
}

func applyFixture(c *Catalog, d *fileDriver, req Request) (Result, error) {
	return Execute(context.Background(), c, linux, "source", d.home, d.statePath, req, d)
}

func TestInstallThenRemoveSharedConsumerLifecycle(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor", "agent"}}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "ready" {
		t.Fatalf("install: %+v %v", result, err)
	}
	if !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
		t.Fatalf("order: %v", d.calls)
	}
	request.Selected = []string{"agent"}
	request.RemoveShared = []string{"node"}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d.dir, "node")); err != nil {
		t.Fatal("removed a shared runtime", err)
	}
	request.Selected = []string{}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d.dir, "node")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("last-consumer cleanup: %v", err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Transaction != nil || len(state.Receipts) != 0 || state.Generation != 3 {
		t.Fatalf("final ledger: %+v %v", state, err)
	}
}

func TestOtherConsumerRemovalOrderKeepsEditorWorking(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor", "agent"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	request.Selected, request.RemoveShared = []string{"editor"}, []string{"node"}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 2 {
		t.Fatalf("removing agent touched its shared runtime or editor: %v", d.calls)
	}
	request.Selected = []string{}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.calls[2:], []string{"remove:plugins", "remove:node"}) {
		t.Fatalf("unsafe final cleanup: %v", d.calls)
	}
}

func TestInterruptedPublicationIsJournaledAndNeverInventsOwnership(t *testing.T) {
	c, d := engineFixture(t)
	d.fail = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err == nil {
		t.Fatal("provider failure hidden")
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Transaction == nil || state.Transaction.InFlight != "node" || state.Transaction.Error == "" || state.Receipts["node"].Ownership != "uncertain" {
		t.Fatalf("missing failure evidence: %+v %v", state, err)
	}
	d.fail = ""
	request.Retry = true
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(d.calls[1:], "apply:node") {
		t.Fatal("blindly repeated already completed publication")
	}
	state, err = LoadState(d.statePath, d.home)
	if err != nil || len(state.PastTransactions) != 1 {
		t.Fatalf("lost previous transaction: %+v %v", state, err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(d.statePath), state.PastTransactions[0]))
	if err != nil {
		t.Fatal(err)
	}
	var previous Transaction
	if err := Decode(data, &previous); err != nil || previous.InFlight != "node" || previous.Error == "" {
		t.Fatalf("archived failure lost intent: %+v %v", previous, err)
	}
	request.Selected, request.RemoveShared = []string{}, []string{"node"}
	request.Retry = false
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || operation(t, result.Plan, "node").Action != "retain" {
		t.Fatalf("uncertain ownership became removal authority: %+v %v", result, err)
	}
}

func TestNewPendingPrerequisiteDefersConsumersAndResumes(t *testing.T) {
	c, d := engineFixture(t)
	d.pending = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "needs-action" || !slices.Equal(d.calls, []string{"apply:node"}) {
		t.Fatalf("pending result: %+v %v; calls %v", result, err, d.calls)
	}
	d.pending = ""
	request.Retry = true
	result, err = applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "ready" || !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
		t.Fatalf("resume: %+v %v; calls %v", result, err, d.calls)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["plugins"].Ownership != "created" {
		t.Fatalf("deferred installation did not acquire ownership: %+v %v", state, err)
	}
	request.Selected, request.RemoveShared = []string{}, []string{"node"}
	request.Retry = false
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.calls[2:], []string{"remove:plugins", "remove:node"}) {
		t.Fatalf("deferred resources leaked at removal: %v", d.calls)
	}
}

func TestAbsentPendingArtifactNeverClaimsUnprovedOwnership(t *testing.T) {
	c, d := engineFixture(t)
	d.absentPending = true
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "needs-action" {
		t.Fatalf("pending result: %+v %v", result, err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["node"].Ownership != "uncertain" {
		t.Fatalf("absent pending artifact claimed ownership: %+v %v", state, err)
	}
	request.Retry = true
	// Without a provider-bound completion receipt, appearing after a reboot
	// cannot prove who created the artifact. Keep that unresolved case safe.
	d.absentPending = false
	if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("appeared after restart"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	request.Selected, request.RemoveShared = []string{}, []string{"node"}
	request.Retry = false
	result, err = applyFixture(c, d, approve(t, c, d, request))
	if err != nil || operation(t, result.Plan, "node").Action != "retain" {
		t.Fatalf("unproved completion became deletion authority: %+v %v", result, err)
	}
}

func TestRepairCanRestoreConsumerHealthWithoutUnapprovedMutation(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	d.unhealthy = "node"
	request = Request{Schema: 1, Mode: "repair"}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.calls[2:], []string{"apply:node"}) {
		t.Fatalf("healthy consumer unnecessarily changed: %v", d.calls)
	}
}

func TestReinstalledFormerlyExternalResourceGetsFreshOwnership(t *testing.T) {
	c, d := engineFixture(t)
	if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(d.dir, "node")); err != nil {
		t.Fatal(err)
	}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["node"].Ownership != "created" || state.Receipts["node"].Before.Present {
		t.Fatalf("fresh install has stale provenance: %+v %v", state, err)
	}
}

func TestApprovalRejectsObservedDriftWithoutProviderMutation(t *testing.T) {
	c, d := engineFixture(t)
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := applyFixture(c, d, request); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale approval accepted: %v", err)
	}
	if len(d.calls) != 0 {
		t.Fatal("stale plan mutated a provider")
	}
	if _, err := os.Stat(d.statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale plan published a ledger")
	}
}

func TestRemovalSuccessRequiresObservedAbsenceOrRestoredBaseline(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	d.leaveOnRemove = true
	request.Selected = []string{}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err == nil {
		t.Fatal("provider exit zero was accepted without removal proof")
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["plugins"].Status != "in-progress" || state.Transaction == nil {
		t.Fatalf("failed removal discarded ownership/recovery: %+v %v", state, err)
	}
}

func TestCancelledContextBeforeApplyDoesNotCreateState(t *testing.T) {
	c, d := engineFixture(t)
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Execute(ctx, c, linux, "source", d.home, d.statePath, request, d); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(d.statePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled apply created state")
	}
}

func TestMissingApprovalDoesNotCreateState(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	approve(t, c, d, request)
	if _, err := os.Stat(filepath.Dir(d.statePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview created state")
	}
	if _, err := applyFixture(c, d, request); err == nil {
		t.Fatal("unapproved mutation accepted")
	}
	if _, err := os.Stat(filepath.Dir(d.statePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unapproved request created state")
	}
}

func TestMutationLockExcludesSecondWriterAndReleases(t *testing.T) {
	dir := t.TempDir()
	unlock, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Lock(dir); err == nil {
		if closeErr := second(); closeErr != nil {
			t.Fatal(closeErr)
		}
		t.Fatal("second writer acquired lock")
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	unlocked, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := unlocked(); err != nil {
		t.Fatal(err)
	}
}

func TestMutationLockReleasesOnProcessDeath(t *testing.T) {
	if directory := os.Getenv("DOTFILES_TEST_LOCK_CHILD"); directory != "" {
		unlock, err := Lock(directory)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("locked")
		time.Sleep(30 * time.Second)
		if err := unlock(); err != nil {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMutationLockReleasesOnProcessDeath$")
	child.Env = append(os.Environ(), "DOTFILES_TEST_LOCK_CHILD="+dir)
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "locked\n" {
		cancel()
		waitErr := child.Wait()
		t.Fatalf("child did not acquire lock: %q %v; wait %v", line, err, waitErr)
	}
	if unlock, err := Lock(dir); err == nil {
		closeErr := unlock()
		cancel()
		waitErr := child.Wait()
		t.Fatalf("child lock was not exclusive: close %v; wait %v", closeErr, waitErr)
	}
	if err := child.Process.Kill(); err != nil {
		cancel()
		waitErr := child.Wait()
		t.Fatalf("could not interrupt child: %v; wait %v", err, waitErr)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("expected killed process status")
	}
	unlock, err := Lock(dir)
	if err != nil {
		t.Fatal("dead process retained lock", err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerPathsWithSpacesAndUnicode(t *testing.T) {
	c, d := engineFixture(t)
	d.dir = filepath.Join(d.home, "tools café 日本語")
	d.statePath = filepath.Join(d.home, "state café 日本語", "state.json")
	if err := os.Mkdir(d.dir, 0700); err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
}

func TestModifiedConsumerRetainsItsRuntimeEvenWhenRemovalRequested(t *testing.T) {
	state, observed := installed()
	o := observed["plugins"]
	o.Fingerprint = "user-modified"
	observed["plugins"] = o
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", RemoveShared: []string{"node"}, Selected: []string{}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if operation(t, p, "node").Action != "retain" {
		t.Fatal("preserved a consumer but destroyed its runtime")
	}
}

func TestRemovingManagedOverlayMustRestoreExistingBaseline(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["plugins"]
	receipt.Before = receipt.After
	receipt.Before.Fingerprint = fmt.Sprintf("%x", sha256.Sum256([]byte("original user configuration")))
	state.Receipts["plugins"] = receipt
	if err := SaveState(d.statePath, state); err != nil {
		t.Fatal(err)
	}
	// The provider deletes the overlay instead of restoring the saved original.
	request.Selected = []string{}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err == nil || result.Status == "ready" {
		t.Fatalf("lost user baseline accepted: %+v %v", result, err)
	}
	state, err = LoadState(d.statePath, d.home)
	if err != nil || state.Transaction == nil || state.Receipts["plugins"].Ownership != "created" {
		t.Fatalf("restoration failure lost recovery evidence: %+v %v", state, err)
	}
}

func TestInterruptedTransactionRejectsImplicitResume(t *testing.T) {
	c, d := engineFixture(t)
	d.fail = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err == nil {
		t.Fatal("expected interrupted installation")
	}
	d.fail = ""
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(context.Background(), c, linux, state, request, d)
	if err == nil {
		_, err = PlanChanges(c, linux, state, request, observed, "source")
	}
	if err == nil {
		t.Fatal("unfinished transaction resumed without an explicit recovery choice")
	}
	if len(d.calls) != 1 {
		t.Fatal("discovery mutated an interrupted provider")
	}
}

func TestAbandonPreservesArtifactsAndArchivesFailure(t *testing.T) {
	c, d := engineFixture(t)
	d.fail = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err == nil {
		t.Fatal("expected interrupted publication")
	}
	before, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	request = Request{Schema: 1, Mode: "abandon"}
	result, err := applyFixture(c, d, approve(t, c, d, request))
	if err != nil || result.Status != "ready" || len(result.Plan.Operations) != 0 {
		t.Fatalf("abandon: %+v %v", result, err)
	}
	after, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	for id, receipt := range before.Receipts {
		if receipt.Status == "in-progress" {
			if after.Receipts[id].Status != "needs-action" || after.Receipts[id].Reason == "" {
				t.Fatal("abandon left a receipt active without its transaction")
			}
			receipt.Status, receipt.Reason = after.Receipts[id].Status, after.Receipts[id].Reason
			before.Receipts[id] = receipt
		}
	}
	oldReceipts, _ := digest(before.Receipts)
	newReceipts, _ := digest(after.Receipts)
	if oldReceipts != newReceipts || after.Transaction != nil || len(after.PastTransactions) != 1 || after.Generation != before.Generation+1 || !slices.Equal(before.Selected, after.Selected) {
		t.Fatalf("abandon changed provenance or lost history: %+v", after)
	}
	if len(d.calls) != 1 {
		t.Fatal("abandon invoked a provider mutation")
	}
	if data, err := os.ReadFile(filepath.Join(d.dir, "node")); err != nil || string(data) != "installed" {
		t.Fatalf("abandon removed partial installation: %q %v", data, err)
	}
}

func TestRetryCannotChangeInterruptedSelection(t *testing.T) {
	c, d := engineFixture(t)
	d.fail = "node"
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err == nil {
		t.Fatal("expected interruption")
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	request.Retry, request.Selected = true, []string{"agent"}
	if _, err := PlanChanges(c, linux, state, request, nil, "source"); err == nil {
		t.Fatal("retry silently replaced interrupted selection")
	}
}

func TestRetryRequiresAnUnfinishedTransaction(t *testing.T) {
	c, d := engineFixture(t)
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{
		{Schema: 1, Mode: "apply", Retry: true},
		{Schema: 1, Mode: "abandon"},
	} {
		if _, err := PlanChanges(c, linux, state, request, nil, "source"); err == nil {
			t.Fatalf("recovery without interrupted transaction accepted: %+v", request)
		}
	}
}

type receiptDriver struct {
	*fileDriver
	seen []Receipt
}

func (d *receiptDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	d.seen = append(d.seen, receipt)
	return d.fileDriver.Observe(ctx, r, receipt)
}

func TestProbesReceiveRecordedOwnershipAndRecoveryContext(t *testing.T) {
	c, d := engineFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
	if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	probe := &receiptDriver{fileDriver: d}
	if _, err := Observe(context.Background(), c, linux, state, request, probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.seen) != 1 || probe.seen[0].Ownership != "created" || probe.seen[0].Recovery == "" || !sameArtifact(probe.seen[0].After, state.Receipts["node"].After) {
		t.Fatalf("probe cannot verify its recovery evidence: %+v", probe.seen)
	}
	request.Mode = "update"
	request = approve(t, c, d, request)
	if _, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, probe); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range probe.seen {
		if receipt.Ownership != "created" || receipt.Recovery == "" {
			t.Fatalf("execution probe lost receipt context: %+v", receipt)
		}
	}
}
