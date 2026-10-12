package installer

import (
	"context"
	"fmt"
	"strings"
)

// systemd reports an unloaded unit as inactive, but stop exits 5 for it.
// Inspect each approved unit at dispatch and verify its state afterwards.
// A unit disappearing between show and stop is accepted only after a fresh
// inactive/failed observation. A failed unit is stopped; stop does not clear
// that state. Other command failures are never treated as success.
const legacySystemdStopScript = `set -u
systemctl=$1; shift
for unit; do
	state=$("$systemctl" show --property=ActiveState --value -- "$unit") || exit 121
	case "$state" in
	inactive|failed) continue ;;
	active|activating|deactivating|reloading|maintenance|refreshing) ;;
	*) exit 121 ;;
	esac
	"$systemctl" stop -- "$unit"
	result=$?
	if [ "$result" -ne 0 ] && [ "$result" -ne 5 ]; then exit "$result"; fi
	state=$("$systemctl" show --property=ActiveState --value -- "$unit") || exit 121
	case "$state" in inactive|failed) ;; *) exit 121 ;; esac
done
`

func (d *legacyRetirementDriver) systemdStopStep(label string, units []string) (legacyRetirementStep, error) {
	systemctl, err := d.program("/usr/bin/systemctl", "/bin/systemctl")
	if err != nil {
		return legacyRetirementStep{}, err
	}
	// Socket activation must be stopped before its service.
	ordered := make([]string, 0, len(units))
	for _, socket := range []bool{true, false} {
		for _, unit := range units {
			if strings.HasSuffix(unit, ".socket") == socket {
				ordered = append(ordered, unit)
			}
		}
	}
	command := d.systemCommand(true, "/bin/sh", append([]string{"-c", legacySystemdStopScript, "sh", systemctl}, ordered...)...)
	step := legacyRetirementStep{Kind: "command", Label: label, Privileged: true, Command: &command}
	for _, unit := range ordered {
		step.Checks = append(step.Checks, legacyRetirementGate{Kind: "systemd-stopped", Subject: unit, Expected: "stopped"})
	}
	return step, nil
}

func (d *legacyRetirementDriver) systemdActiveState(ctx context.Context, unit string) (string, error) {
	systemctl, err := d.program("/usr/bin/systemctl", "/bin/systemctl")
	if err != nil {
		return "", err
	}
	output, err := d.Query(ctx, d.systemCommand(false, systemctl, "show", "--property=ActiveState", "--value", "--", unit))
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(string(output))
	switch state {
	case "inactive", "active", "activating", "deactivating", "reloading", "failed", "maintenance", "refreshing":
		return state, nil
	}
	return "", fmt.Errorf("systemd returned an unknown active state for %s", unit)
}
