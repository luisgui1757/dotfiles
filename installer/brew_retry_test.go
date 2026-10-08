package installer

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBrewFailedUpgradeCannotFinishWithRootOnlyNoop(t *testing.T) {
	d, resource, receipt := brewMutationFixture(t, nil)
	after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
	run, query, attempt, maintained := d.Run, d.Query, 0, false
	d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "linkage" && slices.Contains(args[2:], "outside") && !maintained {
			return []byte("missing fixture library"), errors.New("linkage failed")
		}
		return query(ctx, privileged, program, input, args...)
	}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		attempt++
		output, err := run(ctx, command)
		if err != nil {
			return output, err
		}
		code, failure := 0, ""
		if attempt == 1 {
			code, failure = 1, "fixture-dependent-rebuild-refused"
		} else {
			maintained = slices.Equal(command.Arguments, []string{"reinstall", "--formula", "outside"})
			if !maintained {
				// The root is already current, so a repeated upgrade exits zero
				// without revisiting the dependent repair which failed earlier.
				output = []byte("first 1.0 already installed")
			}
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		if err := saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{Output: output, Error: failure, ExitCode: &code}}); err != nil {
			return output, err
		}
		if failure != "" {
			return output, errors.New(failure)
		}
		return output, nil
	}
	if _, err := d.Apply(context.Background(), resource, Operation{Action: "update"}, receipt); err == nil {
		t.Fatal("initial native maintenance failure was hidden")
	}
	observed, err := d.ResumeResource(context.Background(), resource, Operation{Action: "update"}, receipt)
	if err != nil || !maintained || observed.CompletedOperation != receipt.OperationID {
		t.Fatal("retry did not complete the previously attempted native maintenance", maintained, observed, err)
	}
	intent, err := d.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	intent.Commands[len(intent.Commands)-1].Arguments = append(intent.Commands[len(intent.Commands)-1].Arguments, "unrelated")
	if err := saveDocument(d.intentPath(receipt.OperationID), intent); err != nil {
		t.Fatal(err)
	}
	if _, err := d.intent(receipt.OperationID); err == nil {
		t.Fatal("saved maintenance gained authority over an unrelated formula")
	}
}

func TestBrewInitialFailureRecoversConsumerAfterMissingRoot(t *testing.T) {
	rootInstalled, maintained := false, false
	d, resource, receipt := brewMutationFixture(t, func(installed map[string]nativePackage) {
		if !rootInstalled {
			delete(installed, "first")
		}
	})
	run, query, attempt := d.Run, d.Query, 0
	d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "linkage" && slices.Contains(args[2:], "outside") && !maintained {
			return []byte("missing fixture library"), errors.New("linkage failed")
		}
		return query(ctx, privileged, program, input, args...)
	}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		attempt++
		expected := []string{"install", "--formula", "first"}
		if attempt == 3 {
			expected = []string{"reinstall", "--formula", "outside"}
		}
		if !slices.Equal(command.Arguments, expected) {
			t.Fatalf("unexpected recovery step %d: %v", attempt, command.Arguments)
		}
		rootInstalled = attempt >= 2
		maintained = attempt == 3
		output, err := run(ctx, command)
		if err != nil {
			return output, err
		}
		if !rootInstalled {
			_, remaining, ok := strings.Cut(string(output), "\n")
			if !ok {
				t.Fatal("fixture lacks its separate root/dependency records")
			}
			output = []byte(remaining)
		}
		code, failure := 0, ""
		if attempt == 1 {
			code, failure = 1, "root build failed after dependency upgrade"
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		if err := saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{Output: output, Error: failure, ExitCode: &code}}); err != nil {
			return output, err
		}
		if failure != "" {
			return output, errors.New(failure)
		}
		if attempt == 3 {
			return nil, errors.New("lost response after completed dependency maintenance")
		}
		return output, nil
	}
	if _, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("initial failed root build was hidden")
	}
	if _, err := d.ResumeResource(context.Background(), resource, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "lost response") {
		t.Fatal("did not reach completed dependency maintenance", err)
	}
	observed, err := d.ResumeResource(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err != nil || !observed.Healthy || observed.CompletedOperation != receipt.OperationID || attempt != 3 {
		t.Fatal("recovery repeated completed maintenance or lost root installation", attempt, observed, err)
	}
}

func TestBrewBrokenConsumerCannotBeIgnoredOrRepairedRepeatedly(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(map[bool]string{false: "repair-still-broken", true: "pinned"}[pinned], func(t *testing.T) {
			d, resource, receipt := brewMutationFixture(t, func(installed map[string]nativePackage) {
				pkg := installed["outside"]
				pkg.Held = pinned
				installed["outside"] = pkg
			})
			query, run, repairs := d.Query, d.Run, 0
			d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
				if len(args) >= 3 && args[0] == "linkage" && slices.Contains(args[2:], "outside") {
					return []byte("outside still cannot load its library"), errors.New("linkage failed")
				}
				return query(ctx, privileged, program, input, args...)
			}
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				if slices.Equal(command.Arguments, []string{"reinstall", "--formula", "outside"}) {
					repairs++
				}
				return run(ctx, command)
			}
			_, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
			if err == nil || !strings.Contains(err.Error(), "outside") || repairs > 1 || pinned && repairs != 0 {
				t.Fatal("broken consumer was ignored, repeatedly repaired or unpinned", repairs, err)
			}
		})
	}
}
