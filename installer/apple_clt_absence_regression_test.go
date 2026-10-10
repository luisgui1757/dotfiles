package installer

import (
	"context"
	"errors"
	"testing"
)

func TestAppleCLTHistoricalReceiptAllowsMissingPayloadRegression(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.installed = true
	o, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err != nil || o.Unknown || o.Present || o.Pending != "" {
		t.Fatal("historical receipt incorrectly vetoes missing payload", o, err)
	}
}
func TestAppleCLTSelectionQueryFailureIsNotAbsenceRegression(t *testing.T) {
	f := newAppleCLTFixture(t)
	q := f.d.Query
	f.d.Query = func(ctx context.Context, p bool, program string, input []byte, args ...string) ([]byte, error) {
		if program == "/usr/bin/xcode-select" {
			return nil, errors.New("unexpected native error")
		}
		return q(ctx, p, program, input, args...)
	}
	o, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err == nil {
		t.Fatal("native query failure treated as absence", o)
	}
}
