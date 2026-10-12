package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The provider has its own process-lifetime lock because its child can outlive
// the engine. A real lock is the boundary, including for journal-only abandon.
type leasedFileDriver struct {
	*fileDriver
	leaseDirectory string
}

func (d *leasedFileDriver) AcquireMutation(context.Context) (func() error, error) {
	return Lock(d.leaseDirectory)
}

func TestAbandonCannotBypassActiveProvider(t *testing.T) {
	c, d := engineFixture(t)
	d.fail = "node"
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
	if _, err := applyFixture(c, d, request); err == nil {
		t.Fatal("fixture did not leave an interrupted operation")
	}
	request = approve(t, c, d, Request{Schema: 1, Mode: "abandon"})
	before, err := os.ReadFile(d.statePath)
	if err != nil {
		t.Fatal(err)
	}
	guarded := &leasedFileDriver{d, filepath.Join(d.home, "provider-lease")}
	release, err := Lock(guarded.leaseDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, guarded); err == nil {
		t.Error("abandon archived a transaction while its provider still held the lease")
	}
	after, err := os.ReadFile(d.statePath)
	if err != nil || string(before) != string(after) {
		t.Errorf("blocked abandonment changed durable intent: %v", err)
	}
}
