package installer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLegacyLaunchdPostconditionWaitsForObservedUnregistration(t *testing.T) {
	d, _ := legacyStepFixture(t)
	queries := 0
	d.Query = func(context.Context, nativeCommand) ([]byte, error) {
		queries++
		if queries == 1 {
			return []byte("state = running\n"), nil
		}
		return nil, legacyFakeExit(113, "service absent")
	}
	step := legacyRetirementStep{Checks: []legacyRetirementGate{{Kind: "launchd", Subject: "system/fixture", Expected: "absent"}}}
	if err := d.postconditions(context.Background(), step); err != nil || queries < 2 {
		t.Fatal("successful bootout was checked before unregistration", err, queries)
	}
}

func TestLegacyLaunchdPostconditionDoesNotHideFailureOrWaitPastDeadline(t *testing.T) {
	d, _ := legacyStepFixture(t)
	queryError := errors.New("launchd inspection failed")
	d.Query = func(context.Context, nativeCommand) ([]byte, error) { return nil, queryError }
	if err := d.waitLaunchdAbsent(context.Background(), "system/fixture"); !errors.Is(err, queryError) {
		t.Fatal("inspection failure was hidden", err)
	}
	d.Query = func(context.Context, nativeCommand) ([]byte, error) { return []byte("state = running\n"), nil }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := d.waitLaunchdAbsent(ctx, "system/fixture"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("a still-loaded service was accepted", err)
	}
	// Preconditions must reject a live service immediately, not wait for an
	// unapproved change to make a stale removal approval appear valid again.
	if err := d.evaluateGates(context.Background(), []legacyRetirementGate{{Kind: "launchd", Subject: "system/fixture", Expected: "absent"}}); err == nil || !strings.Contains(err.Error(), "found \"loaded\"") {
		t.Fatal("a live service passed the approval gate", err)
	}
}
