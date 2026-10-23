package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func homebrewWaitingProcess(t *testing.T, seconds int) HomebrewLocation {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew's POSIX process boundary")
	}
	program := filepath.Join(t.TempDir(), "brew")
	if err := os.WriteFile(program, []byte(fmt.Sprintf("#!/bin/sh\nexec /bin/sleep %d\n", seconds)), 0755); err != nil {
		t.Fatal(err)
	}
	return HomebrewLocation{Program: program}
}

func TestHomebrewInspectionRetainsDeadlineAndCancellationCauses(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			location := homebrewWaitingProcess(t, 60)
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				want = context.DeadlineExceeded
			} else {
				timer := time.AfterFunc(100*time.Millisecond, cancel)
				defer timer.Stop()
			}
			defer cancel()
			started := time.Now()
			_, err := location.query(ctx, false, "brew", nil, "linkage", "--test", "fixture")
			if !errors.Is(err, want) {
				t.Fatal("inspection lost its structured cancellation cause", err, want)
			}
			if time.Since(started) > 5*time.Second {
				t.Fatal("long linkage budget ignored the caller's shorter cancellation")
			}
		})
	}
}

func TestHomebrewLinkageTimeoutCannotAuthorizeReinstallation(t *testing.T) {
	for _, scope := range []string{"batch", "individual"} {
		for _, cause := range []error{context.DeadlineExceeded, context.Canceled} {
			t.Run(scope+"/"+cause.Error(), func(t *testing.T) {
				d, resource, receipt := brewMutationFixture(t, nil)
				query, run := d.Query, d.Run
				individuals, repairs := 0, 0
				d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
					if len(args) >= 3 && args[0] == "linkage" {
						if len(args) == 3 {
							individuals++
							if scope == "batch" {
								return nil, nil
							}
							return []byte("partial inspection"), fmt.Errorf("native inspection: %w", cause)
						}
						if scope == "batch" {
							return nil, fmt.Errorf("native inspection: %w", cause)
						}
						return nil, errors.New("genuine batch linkage failure")
					}
					return query(ctx, privileged, program, input, args...)
				}
				d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
					if command.Arguments[0] == "reinstall" {
						repairs++
						return nil, errors.New("fixture refuses repair without broken-linkage evidence")
					}
					return run(ctx, command)
				}
				_, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
				if repairs != 0 {
					t.Fatal("query interruption authorized a native reinstall", repairs, err)
				}
				if !errors.Is(err, cause) {
					t.Fatal("maintenance lost its query cancellation cause", err)
				}
				if scope == "batch" && individuals != 0 || scope == "individual" && individuals != 1 {
					t.Fatal("maintenance continued inspection after cancellation", individuals)
				}
				intent, err := d.intent(receipt.OperationID)
				if err != nil || intent.Complete || len(intent.Commands) != 1 {
					t.Fatal("interrupted inspection published completion or saved repair authority", intent, err)
				}
			})
		}
	}
}

func TestHomebrewSubstantiveBatchFailureIsNotClearedByHealthyIndividualChecks(t *testing.T) {
	d, resource, receipt := brewMutationFixture(t, nil)
	query := d.Query
	d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if len(args) > 3 && args[0] == "linkage" {
			return []byte("batch-native-diagnostic"), errors.New("substantive linkage failure")
		}
		return query(ctx, privileged, program, input, args...)
	}
	_, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err == nil || !strings.Contains(err.Error(), "substantive linkage failure") || !strings.Contains(err.Error(), "batch-native-diagnostic") {
		t.Fatal("healthy individual checks hid the original substantive batch failure", err)
	}
}

func TestHomebrewRootInspectionTimeoutCannotOfferReinstallation(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			d, resource, _ := brewMutationFixture(t, nil)
			program := filepath.Join(d.Cellar, "first", "1.0", "bin", "first")
			d.Checks = map[string][]string{resource.ID: {program, "--version"}}
			query, interrupted := d.Query, false
			d.Query = func(ctx context.Context, privileged bool, command string, input []byte, args ...string) ([]byte, error) {
				if command == program {
					if interrupted {
						return nil, fmt.Errorf("native command inspection: %w", cause)
					}
					return []byte("first 1.0"), nil
				}
				return query(ctx, privileged, command, input, args...)
			}
			home := filepath.Dir(d.Directory)
			c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "first", Name: "First", Capability: true, Requires: []string{resource.ID}}, resource}}, Context: Context{OS: "darwin", Arch: "arm64"}, Source: "query-timeout-fixture", Home: home, StatePath: filepath.Join(home, "state.json"), Driver: d}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			interrupted = true
			plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			if !errors.Is(err, cause) {
				t.Fatal("incomplete root inspection could offer repair", plan, err)
			}
		})
	}
}

func TestHomebrewLinkageQueryMayExceedOrdinaryInspectionBudget(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_HOMEBREW_LINKAGE_TIMEOUT") != "1" {
		t.Skip("set DOTFILES_TEST_HOMEBREW_LINKAGE_TIMEOUT=1 for a real 31-second read-only process")
	}
	location := homebrewWaitingProcess(t, 31)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, err := location.query(ctx, false, "brew", nil, "linkage", "--test", "first", "outside"); err != nil {
		t.Fatal("bounded affected-set linkage was cut off at the ordinary 30-second query limit", err)
	}
}
