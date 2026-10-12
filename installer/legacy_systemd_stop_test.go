package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacySystemdStopCompletesWithMissingUnits(t *testing.T) {
	for _, caller := range []string{"upstream", "determinate"} {
		for _, missing := range []string{"all", "socket"} {
			t.Run(caller+"/"+missing, func(t *testing.T) {
				d, program := legacySystemdStopFixture(t)
				if missing == "socket" {
					writeConfigFixture(t, filepath.Join(filepath.Dir(program), "nix-daemon.service"), "active\n")
				}
				var scope legacyRetirementScope
				var err error
				if caller == "upstream" {
					err = d.planLinuxDaemonStop(&scope, nil)
				} else {
					receipt := legacyDeterminateReceipt{Init: "Systemd", Units: []string{"nix-daemon.socket", "nix-daemon.service"}}
					err = d.planDeterminateDaemonStop(context.Background(), &scope, receipt, nil)
				}
				if err != nil || len(scope.Steps) != 1 {
					t.Fatal(scope, err)
				}
				if err := d.executeStep(context.Background(), legacyTestIntent(t, d, scope.Steps...), 0, false); err != nil {
					t.Fatal("an already absent unit blocked retirement", err)
				}
				stopped, err := os.ReadFile(filepath.Join(filepath.Dir(program), "stopped"))
				if missing == "all" && !os.IsNotExist(err) || missing == "socket" && (err != nil || strings.TrimSpace(string(stopped)) != "nix-daemon.service") {
					t.Fatal("stop did not target only the remaining active service", string(stopped), err)
				}
			})
		}
	}
}

func legacySystemdStopFixture(t *testing.T) (*legacyRetirementDriver, string) {
	t.Helper()
	d, root := legacyStepFixture(t)
	d.systemRoot = root
	program := d.sys("/usr/bin/systemctl")
	writeConfigFixture(t, program, `#!/bin/sh
root=$(dirname "$0")
verb=$1; shift
case "$verb" in
show)
	if [ -f "$root/query-error" ]; then exit 7; fi
	if [ -f "$root/query-state" ]; then cat "$root/query-state"; exit 0; fi
	for unit; do :; done
	if [ -f "$root/$unit" ]; then cat "$root/$unit"; else printf 'inactive\n'; fi
	;;
stop)
	if [ "$1" = -- ]; then shift; fi
	for unit; do
		printf '%s\n' "$unit" >> "$root/stopped"
		if [ -f "$root/stop-result" ]; then
			case $(cat "$root/stop-result") in
			disappeared) rm -- "$root/$unit"; exit 5 ;;
			still-active) exit 5 ;;
			other-failure) printf 'inactive\n' > "$root/$unit"; exit 1 ;;
			stopped-failed) printf 'failed\n' > "$root/$unit"; exit 0 ;;
			disappeared-failed) printf 'failed\n' > "$root/$unit"; exit 5 ;;
			esac
		fi
		if [ ! -f "$root/$unit" ]; then exit 5; fi
		if [ "$(cat "$root/$unit")" = failed ]; then continue; fi
		printf 'inactive\n' > "$root/$unit"
	done
	;;
*) exit 99 ;;
esac
`)
	if err := os.Chmod(program, 0700); err != nil {
		t.Fatal(err)
	}
	return d, program
}

func TestLegacySystemdStopAcceptsFailedUnits(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "dispatch", true: "recovery"}[recovery], func(t *testing.T) {
			d, program := legacySystemdStopFixture(t)
			root := filepath.Dir(program)
			writeConfigFixture(t, filepath.Join(root, "nix-daemon.service"), "failed\n")
			if !recovery {
				writeConfigFixture(t, filepath.Join(root, "nix-daemon.socket"), "active\n")
			}
			step, err := d.systemdStopStep("Stop fixture", []string{"nix-daemon.service", "nix-daemon.socket"})
			if err != nil {
				t.Fatal(err)
			}
			if err := d.executeStep(context.Background(), legacyTestIntent(t, d, step), 0, recovery); err != nil {
				t.Fatal("a failed unit blocked retirement", err)
			}
			state, err := d.systemdActiveState(context.Background(), "nix-daemon.service")
			if err != nil || state != "failed" {
				t.Fatal("the fixture must preserve systemd's failed state", state, err)
			}
		})
	}
}

func TestLegacySystemdStopRequiresObservedCompletion(t *testing.T) {
	for _, outcome := range []string{"disappeared", "stopped-failed", "disappeared-failed", "still-active", "other-failure", "query-error", "unknown-state"} {
		t.Run(outcome, func(t *testing.T) {
			d, program := legacySystemdStopFixture(t)
			root := filepath.Dir(program)
			unit := "nix-daemon.service"
			writeConfigFixture(t, filepath.Join(root, unit), "active\n")
			switch outcome {
			case "query-error":
				writeConfigFixture(t, filepath.Join(root, "query-error"), "deny\n")
			case "unknown-state":
				writeConfigFixture(t, filepath.Join(root, "query-state"), "unknown\n")
			default:
				writeConfigFixture(t, filepath.Join(root, "stop-result"), outcome)
			}
			step, err := d.systemdStopStep("Stop fixture", []string{unit})
			if err != nil {
				t.Fatal(err)
			}
			err = d.executeStep(context.Background(), legacyTestIntent(t, d, step), 0, false)
			if (err == nil) != (outcome == "disappeared" || outcome == "stopped-failed" || outcome == "disappeared-failed") {
				t.Fatal("completion did not require an observed stopped state and permitted outcome", outcome, err)
			}
		})
	}
}

func TestLegacySystemdSavedInactiveCheckRemainsStrict(t *testing.T) {
	d, program := legacySystemdStopFixture(t)
	unit := "nix-daemon.service"
	step := legacyRetirementStep{Kind: "command", Checks: []legacyRetirementGate{{Kind: "systemd-stopped", Subject: unit, Expected: "inactive"}}}
	for _, state := range []string{"failed", "inactive"} {
		writeConfigFixture(t, filepath.Join(filepath.Dir(program), unit), state+"\n")
		done, err := d.alreadyRetired(context.Background(), step)
		if err != nil || done != (state == "inactive") {
			t.Fatal("historical completion semantics changed", state, done, err)
		}
		if err := d.postconditions(context.Background(), step); (err == nil) != (state == "inactive") {
			t.Fatal("historical postcondition semantics changed", state, err)
		}
	}
}
