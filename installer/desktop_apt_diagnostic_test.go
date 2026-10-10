package installer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// Temporary failure-only evidence on the disposable diagnostic branch. This
// performs read-only provider queries; it never retries or approves an action.
func temporaryDesktopAPTDiagnostic(t *testing.T, directory, home, arch string) {
	if !t.Failed() || runtime.GOOS != "linux" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	state, err := LoadState(filepath.Join(directory, "state.json"), home)
	if err != nil || state.Transaction == nil {
		t.Log("APT failure state", err, state.Transaction)
		return
	}
	transaction := state.Transaction
	if transaction.Next >= len(transaction.Plan.Operations) {
		t.Log("APT failure transaction has no next operation", transaction.Next)
		return
	}
	op := transaction.Plan.Operations[transaction.Next]
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Log("APT failure catalog", err)
		return
	}
	location, err := DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Log("APT failure discovery", err)
		return
	}
	driver, err := configureLinuxAPT(location, catalog, newNativeSession(filepath.Join(directory, "native", "worker")), arch)
	if err != nil {
		t.Log("APT failure provider", err)
		return
	}
	r, ok := catalog.Resource(op.Resource)
	if !ok || r.Bindings["linux"].Provider != "apt" {
		t.Log("APT failure next operation", op.Resource, op.Action)
		return
	}
	fresh, observeErr := driver.Observe(ctx, r, state.Receipts[r.ID])
	installed, inventoryErr := linuxAPTInventory(ctx, driver.Query)
	ledger, ledgerErr := driver.ledger()
	identity, identityErr := linuxAPTIdentity(r.Bindings["linux"].Package, installed)
	consumers := map[string]nativePackage{}
	ownedConsumers := map[string]bool{}
	for name, pkg := range installed {
		if slices.Contains(pkg.Dependencies, identity) {
			consumers[name] = pkg
			ownedConsumers[name] = driver.ownedConsumer(ledger, pkg)
		}
	}
	payload := struct {
		Next           int
		Approved       Operation
		Fresh          Observation
		Package        nativePackage
		Consumers      map[string]nativePackage
		OwnedConsumers map[string]bool
		Ledger         linuxAPTLedger
		Receipts       map[string]Receipt
	}{transaction.Next, op, fresh, installed[identity], consumers, ownedConsumers, ledger, state.Receipts}
	data, marshalErr := json.Marshal(payload)
	t.Logf("APT failure read-only errors: observe=%v inventory=%v ledger=%v identity=%v marshal=%v", observeErr, inventoryErr, ledgerErr, identityErr, marshalErr)
	if len(data) > 64<<10 {
		t.Log("APT failure diagnostic exceeds 64 KiB", len(data))
		return
	}
	t.Logf("APT failure read-only evidence (not necessarily the failed instant): %s", data)
}
