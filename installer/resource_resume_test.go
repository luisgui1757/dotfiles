package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type singleFileIntent struct {
	Operation, Action, Payload string
	Complete                   bool
}

type resumableSingleFileDriver struct {
	*fileDriver
	builds, resumes int
}

func (d *resumableSingleFileDriver) intentPath() string {
	return filepath.Join(filepath.Dir(d.statePath), "single-file-intent.json")
}

func (d *resumableSingleFileDriver) readIntent() (singleFileIntent, error) {
	var intent singleFileIntent
	data, err := readDocument(d.intentPath())
	if err == nil {
		err = Decode(data, &intent)
	}
	return intent, err
}

func (d *resumableSingleFileDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Observe(ctx, r, receipt)
	if err != nil || r.ID != "plugins" {
		return o, err
	}
	intent, err := d.readIntent()
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil || intent.Complete {
		return o, err
	}
	token, err := digest(struct {
		Intent singleFileIntent
		Actual Observation
	}{intent, o})
	o.Healthy, o.Pending = false, "configuration publication interrupted"
	o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	return o, err
}

func (d *resumableSingleFileDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if r.ID != "plugins" {
		return d.fileDriver.Apply(ctx, r, op, receipt)
	}
	d.builds++
	intent := singleFileIntent{Operation: receipt.OperationID, Action: op.Action, Payload: "originally approved configuration bytes"}
	if err := saveDocument(d.intentPath(), intent); err != nil {
		return Observation{}, err
	}
	if err := os.WriteFile(filepath.Join(d.dir, r.ID), []byte("partially published configuration"), 0600); err != nil {
		return Observation{}, err
	}
	return Observation{}, errors.New("injected interruption after partial publication")
}

func (d *resumableSingleFileDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID != "plugins" {
		return d.fileDriver.Remove(ctx, r, receipt)
	}
	if err := saveDocument(d.intentPath(), singleFileIntent{Operation: receipt.OperationID, Action: "remove"}); err != nil {
		return Observation{}, err
	}
	if err := os.Remove(filepath.Join(d.dir, r.ID)); err != nil {
		return Observation{}, err
	}
	return Observation{}, errors.New("injected interruption after removal")
}

func (d *resumableSingleFileDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	d.resumes++
	intent, err := d.readIntent()
	if err != nil || intent.Operation != receipt.OperationID || intent.Action != op.Action {
		return Observation{}, errors.Join(errors.New("saved operation differs from request"), err)
	}
	if intent.Action != "remove" {
		if err := os.WriteFile(filepath.Join(d.dir, r.ID), []byte(intent.Payload), 0600); err != nil {
			return Observation{}, err
		}
		o, err := d.fileDriver.Observe(ctx, r, receipt)
		if err != nil {
			return Observation{}, err
		}
		o.CompletedOperation = intent.Operation
		if err := saveDocument(filepath.Join(d.dir, r.ID)+".completion", o); err != nil {
			return Observation{}, err
		}
	}
	intent.Complete = true
	if err := saveDocument(d.intentPath(), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func TestResourceRecoveryPreservesExactOperationAndRequiresRemainingWorkReview(t *testing.T) {
	c, files := controllerFixture(t)
	d := &resumableSingleFileDriver{fileDriver: files}
	c.Driver = d
	for _, request := range []Request{
		{Schema: 1, Mode: "apply", Selected: []string{"editor"}},
		{Schema: 1, Mode: "update"},
		{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"node"}},
	} {
		preview, err := c.Dispatch(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = preview.Plan.ID
		if _, err := c.Dispatch(context.Background(), request); err == nil {
			t.Fatal("injected partial publication reported success")
		}
		before, err := LoadState(c.StatePath, c.Home)
		if err != nil || before.Transaction == nil || before.Transaction.InFlight != "plugins" {
			t.Fatal(before, err)
		}
		retry := Request{Schema: 1, Mode: request.Mode, Retry: true}
		preview, err = c.Dispatch(context.Background(), retry)
		if err != nil || preview.Plan.ResourceResume == nil || len(preview.Plan.Operations) != 1 {
			t.Fatal("partial resource has no exact recovery preview", preview, err)
		}
		retry.ExpectedPlan = preview.Plan.ID
		result, err := c.Dispatch(context.Background(), retry)
		if err != nil || result.Status != "needs-action" {
			t.Fatal("recovery skipped remaining-work review", result, err)
		}
		after, err := LoadState(c.StatePath, c.Home)
		if err != nil || after.Transaction == nil || after.Transaction.Plan.ID != before.Transaction.Plan.ID ||
			after.Receipts["plugins"].OperationID != before.Receipts["plugins"].OperationID {
			t.Fatal("recovery replaced original operation intent", after, err)
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: request.Mode, Retry: true})
	}
	if d.builds != 2 || d.resumes != 3 {
		t.Fatal("recovery rebuilt an approved target", d.builds, d.resumes)
	}
}

func TestResourceRecoveryApprovalRejectsChangedPartialFiles(t *testing.T) {
	c, files := controllerFixture(t)
	d := &resumableSingleFileDriver{fileDriver: files}
	c.Driver = d
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("injected failure did not occur")
	}
	retry := Request{Schema: 1, Mode: "apply", Retry: true}
	preview, err = c.Dispatch(context.Background(), retry)
	if err != nil {
		t.Fatal(err)
	}
	retry.ExpectedPlan = preview.Plan.ID
	if err := os.WriteFile(filepath.Join(d.dir, "plugins"), []byte("user changed partial target"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Dispatch(context.Background(), retry); err == nil || d.resumes != 0 {
		t.Fatal("stale recovery approval changed user files", err)
	}
}
