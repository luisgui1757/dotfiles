package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBrewCommandHealthPrecedesOwnership(t *testing.T) {
	d, resource, receipt := brewMutationFixture(t, nil)
	program := filepath.Join(d.Cellar, "first", "1.0", "bin", "first")
	d.Checks = map[string][]string{resource.ID: {program, "--version"}}
	query := d.Query
	d.Query = func(ctx context.Context, privileged bool, command string, input []byte, args ...string) ([]byte, error) {
		if command == program {
			if privileged || len(input) != 0 || !slices.Equal(args, []string{"--version"}) {
				t.Fatal("health command differed from trusted catalog", command, args)
			}
			return []byte("missing shared library fixture"), errors.New("exit status 1")
		}
		return query(ctx, privileged, command, input, args...)
	}
	if _, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "missing shared library fixture") {
		t.Fatal("linked keg metadata replaced actual command health", err)
	}
	if _, err := os.Stat(filepath.Join(d.Directory, "packages.json")); !os.IsNotExist(err) {
		t.Fatal("failed command health published ownership", err)
	}
	observed, err := d.Observe(context.Background(), resource, Receipt{})
	if err != nil || !observed.Present || observed.Healthy || !strings.Contains(observed.HealthIssue, "missing shared library fixture") {
		t.Fatal("inspection hid the actual command failure", observed, err)
	}
}

func TestBrewHealthyCommandAndLegacyObservation(t *testing.T) {
	d, resource, receipt := brewMutationFixture(t, nil)
	program := filepath.Join(d.Cellar, "first", "1.0", "bin", "first")
	d.Checks = map[string][]string{resource.ID: {program, "--version"}}
	query, probes := d.Query, 0
	d.Query = func(ctx context.Context, privileged bool, command string, input []byte, args ...string) ([]byte, error) {
		if command == program {
			probes++
			return []byte("first 1.0"), nil
		}
		return query(ctx, privileged, command, input, args...)
	}
	observed, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err != nil || !observed.Healthy || observed.HealthIssue != "" || probes == 0 {
		t.Fatal("healthy native command was not proved", observed, probes, err)
	}
	var previous Observation
	if err := Decode([]byte(`{"present":true,"healthy":true}`), &previous); err != nil || previous.HealthIssue != "" {
		t.Fatal("older observation requires no health-issue field", previous, err)
	}
}

func TestBrewCompletionRechecksRootAfterDependentMaintenance(t *testing.T) {
	d, resource, receipt := brewMutationFixture(t, nil)
	program := filepath.Join(d.Directory, "first")
	d.Checks = map[string][]string{resource.ID: {program, "--version"}}
	query, run, maintained := d.Query, d.Run, false
	d.Query = func(ctx context.Context, privileged bool, command string, input []byte, args ...string) ([]byte, error) {
		if command == program {
			if maintained {
				return []byte("root failed after native maintenance"), errors.New("exit status 1")
			}
			return []byte("first 1.0"), nil
		}
		if len(args) >= 3 && args[0] == "linkage" && slices.Contains(args[2:], "outside") && !maintained {
			return nil, errors.New("outside needs native repair")
		}
		return query(ctx, privileged, command, input, args...)
	}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if slices.Equal(command.Arguments, []string{"reinstall", "--formula", "outside"}) {
			maintained = true
		}
		return run(ctx, command)
	}
	_, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err == nil || !maintained || !strings.Contains(err.Error(), "root failed after native maintenance") {
		t.Fatal("completion trusted a root check from before dependent maintenance", maintained, err)
	}
	if _, err := os.Stat(filepath.Join(d.Directory, "packages.json")); !os.IsNotExist(err) {
		t.Fatal("failed final command health published ownership", err)
	}
}
