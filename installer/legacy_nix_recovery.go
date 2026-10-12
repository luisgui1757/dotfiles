package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Deleting the Nix root is the only irreversible Linux Nix step. Its approval
// binds the root's membership, and the first dispatch persists that membership
// beside the retirement journal before rm starts. A failed or interrupted
// deletion leaves a subset of the approved members. Fresh classification can
// no longer prove the released layout from that remainder, so the saved
// journal and its membership are the evidence: an explicitly approved
// operation may finish deleting only approved members of the same root inode,
// without consumers or a still-running earlier deletion.
//
// The native worker's lock is close-on-exec and records no child identity. A
// worker killed during deletion therefore releases its lock while sudo and rm
// may continue. The deletion holds its own lease instead: an exclusive flock
// on the Nix root, inherited by rm and released only when rm exits. A second
// deletion cannot start while an earlier one still runs.

const legacyNixChunkEntries = 4096

var errLegacyNixMembershipUnrecorded = errors.New("the approved Nix membership was never recorded")

type legacyNixMembershipChunk struct {
	Schema   int      `json:"schema"`
	Approval string   `json:"approval"`
	Index    int      `json:"index"`
	Records  []string `json:"records"`
}

// Exit 86 means another deletion holds the lease; nothing was deleted.
const legacyNixDeletionScript = `set -u
root=$1 flock=$2
if [ ! -e "$root" ] && [ ! -L "$root" ]; then exit 0; fi
exec 9<"$root"
"$flock" -n -E 86 9 || exit $?
exec /bin/rm -rf --one-file-system --preserve-root -- "$root"
`

// The probe opens the root read-only and releases its lock on exit.
const legacyNixLeaseProbe = `set -u
exec 9<"$1"
exec "$2" -n -E 86 9
`

// nixMembership lists the approval records of a Nix root. Every entry directly
// below the root, var and var/nix, and every store entry, is bound by inode;
// the profile and garbage-collection trees are bound by content. Records name
// root-relative paths, so a partial deletion leaves a subset and any added
// member is outside the approval. Runtime contents below other bound
// directories (the database, logs and temporary roots) are not recursively
// bound; they belong to the approved directory that holds them. macOS volume
// metadata is deleted with its volume and is not a member. A missing root has
// no members. Nix's garbage collector is never invoked: even its print-roots
// mode can delete stale roots. A changed device needs fresh approval.
func (d *legacyRetirementDriver) nixMembership(ctx context.Context, root string) ([]string, error) {
	identity, err := legacyPathIdentity(root)
	if err != nil || identity == "absent" {
		return nil, err
	}
	records := []string{".=" + identity}
	for _, directory := range []struct {
		name  string
		limit int
	}{{"", 64}, {"var", 1024}, {"var/nix", 1024}, {"store", 131072}} {
		entries, err := readPlainDirectory(filepath.Join(root, directory.name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(entries) > directory.limit {
			return nil, fmt.Errorf("Nix membership of %s exceeds its review bound", filepath.Join(root, directory.name))
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if directory.name == "" && d.Platform.OS == "darwin" && slices.Contains(legacyDarwinVolumeMetadata, entry.Name()) {
				continue
			}
			name := filepath.Join(directory.name, entry.Name())
			identity, err := legacyPathIdentity(filepath.Join(root, name))
			if err != nil {
				return nil, err
			}
			if identity != "absent" {
				records = append(records, name+"="+identity)
			}
		}
	}
	for _, tree := range []string{"var/nix/profiles", "var/nix/gcroots"} {
		entries, err := inspectTree(filepath.Join(root, tree), maxConfigBytes, maxConfigEntries)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			hash, err := digest(entry)
			if err != nil {
				return nil, err
			}
			records = append(records, filepath.Join(tree, entry.Path)+"#content="+hash)
		}
	}
	profile := filepath.Join(root, "var/nix/profiles/per-user/root")
	if children, err := readPlainDirectory(profile); err == nil && len(children) != 0 {
		known, err := d.nixRootDefaultProfile(profile, children, false)
		if err != nil {
			return nil, err
		}
		if known {
			records = append(records, legacyNixRootProfileProof)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	receipt, err := legacyFileSHA256(filepath.Join(root, "receipt.json"))
	if err == nil {
		records = append(records, "receipt.json#content="+receipt)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return records, ctx.Err()
}

func (d *legacyRetirementDriver) nixStoreIdentity(ctx context.Context) (string, error) {
	records, err := d.nixMembership(ctx, d.sys("/nix"))
	if err != nil {
		return "", err
	}
	return digest(records)
}

// nixStoreGate observes a store gate. An empty subject requires the exact
// approved membership. The deletion gate names its root instead: it passes
// while every remaining member was approved and no deletion holds the lease.
func (d *legacyRetirementDriver) nixStoreGate(ctx context.Context, gate legacyRetirementGate) (string, error) {
	if gate.Subject == "" {
		return d.nixStoreIdentity(ctx)
	}
	root := d.sys("/nix")
	if gate.Subject != root || !operationID.MatchString(gate.Expected) {
		return "", errors.New("invalid Nix store deletion gate")
	}
	records, err := d.nixMembership(ctx, root)
	if err != nil || len(records) == 0 {
		return "absent", err
	}
	if active, err := d.nixDeletionActive(ctx, root); err != nil || active {
		return "", errors.Join(fmt.Errorf("an earlier deletion of %s is still running; wait for it to exit, then review retirement again", root), err)
	}
	current, err := digest(records)
	if err != nil {
		return "", err
	}
	if current == gate.Expected {
		// Persist the approved membership before the deletion it authorizes.
		return current, d.saveNixMembership(current, records)
	}
	approved, err := d.loadNixMembership(gate.Expected)
	if errors.Is(err, errLegacyNixMembershipUnrecorded) {
		// Never dispatched: only the exact approved membership may start it.
		return "membership changed after approval; review a fresh retirement preview", nil
	}
	if err != nil {
		return "", err
	}
	if outside := legacyNixOutside(records, approved); outside != "" {
		return "unapproved member " + outside, nil
	}
	return gate.Expected, nil
}

func (d *legacyRetirementDriver) nixDeletionActive(ctx context.Context, root string) (bool, error) {
	flock, err := d.program("/usr/bin/flock", "/bin/flock")
	if err != nil {
		return false, err
	}
	_, err = d.Query(ctx, d.systemCommand(false, "/bin/sh", "-c", legacyNixLeaseProbe, "sh", root, flock))
	if code, ok := legacyExitCode(err); ok && code == 86 {
		return true, nil
	}
	return false, err
}

func (d *legacyRetirementDriver) nixMembershipPath(approval string, index int) string {
	return filepath.Join(d.Directory, "nix-store", approval, fmt.Sprintf("%03d.json", index))
}

// Chunks are content-addressed by the approved digest and never rewritten.
func (d *legacyRetirementDriver) saveNixMembership(approval string, records []string) error {
	for begin := 0; begin < len(records); begin += legacyNixChunkEntries {
		chunk := legacyNixMembershipChunk{Schema: 1, Approval: approval, Index: begin / legacyNixChunkEntries, Records: records[begin:min(begin+legacyNixChunkEntries, len(records))]}
		path := d.nixMembershipPath(approval, chunk.Index)
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			if err := saveDocument(path, chunk); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	approved, err := d.loadNixMembership(approval)
	if err == nil && len(approved) != len(records) {
		err = errors.New("approved Nix membership has repeated records")
	}
	return err
}

func (d *legacyRetirementDriver) loadNixMembership(approval string) (map[string]bool, error) {
	records := []string{}
	for index := 0; ; index++ {
		if index > (131072+2*maxConfigEntries+8)/legacyNixChunkEntries {
			return nil, errors.New("approved Nix membership exceeds its chunk bound")
		}
		data, err := readDocument(d.nixMembershipPath(approval, index))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, err
		}
		var chunk legacyNixMembershipChunk
		if err := Decode(data, &chunk); err != nil {
			return nil, err
		}
		if chunk.Schema != 1 || chunk.Approval != approval || chunk.Index != index || len(chunk.Records) == 0 || len(chunk.Records) > legacyNixChunkEntries ||
			len(records)%legacyNixChunkEntries != 0 {
			return nil, errors.New("approved Nix membership chunk is invalid; preserve " + d.nixMembershipPath(approval, index))
		}
		records = append(records, chunk.Records...)
	}
	if len(records) == 0 {
		return nil, errLegacyNixMembershipUnrecorded
	}
	actual, err := digest(records)
	if err != nil || actual != approval {
		return nil, errors.Join(errors.New("saved Nix membership differs from its approval; preserve "+filepath.Dir(d.nixMembershipPath(approval, 0))), err)
	}
	approved := map[string]bool{}
	for _, record := range records {
		approved[record] = true
	}
	return approved, nil
}

// legacyNixOutside names the first current member that was not approved.
func legacyNixOutside(records []string, approved map[string]bool) string {
	for _, record := range records {
		if !approved[record] {
			name, _, _ := strings.Cut(record, "=")
			name, _, _ = strings.Cut(name, "#")
			return name
		}
	}
	return ""
}

// nixDeletionStep deletes the Nix root under its own lease.
func (d *legacyRetirementDriver) nixDeletionStep(label, approval string) (legacyRetirementStep, error) {
	root := d.sys("/nix")
	flock, err := d.program("/usr/bin/flock", "/bin/flock")
	if err != nil {
		return legacyRetirementStep{}, err
	}
	command := d.systemCommand(true, "/bin/sh", "-c", legacyNixDeletionScript, "sh", root, flock)
	gates := []legacyRetirementGate{{Kind: "replacements"}, {Kind: "nix-homebrew"}, {Kind: "query-git", Subject: root}, {Kind: "nix-store", Subject: root, Expected: approval}, {Kind: "nix-consumers"}}
	return legacyRetirementStep{Kind: "command", Label: label, Privileged: true, Irreversible: true, Command: &command, Gates: gates, Absent: []string{root}}, nil
}

type legacyNixDeletion struct {
	Operation, Approval, Outcome string
}

// nixInterruptedDeletions finds saved operations stopped at a dispatched Nix
// root deletion without recorded success. The journal must prove its own
// approval; the attempts' native records prove the dispatch. Unreadable
// retire.nix journals authorize nothing and are reported.
func (d *legacyRetirementDriver) nixInterruptedDeletions(root string) ([]legacyNixDeletion, []string, error) {
	directory := filepath.Join(d.Directory, "operations")
	entries, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(entries) > 4096 {
		return nil, nil, errors.New("retirement journal inventory exceeds its review bound")
	}
	deletions, unreadable := []legacyNixDeletion{}, []string{}
	for _, entry := range entries {
		operation, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !operationID.MatchString(operation) {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := readDocument(path)
		if err != nil {
			return nil, nil, err
		}
		var intent legacyRetirementIntent
		if err := Decode(data, &intent); err != nil || intent.Resource != "retire.nix" {
			continue // Not attributable to Nix; its own resource reports it.
		}
		if _, err := d.readIntent("retire.nix", Receipt{OperationID: operation, Before: intent.Before}); err != nil {
			unreadable = append(unreadable, path+" ("+truncateLegacyText(err.Error(), 256)+")")
			continue
		}
		if intent.Complete || intent.Next >= len(intent.Scope.Steps) {
			continue
		}
		step := intent.Scope.Steps[intent.Next]
		approval := ""
		for _, gate := range step.Gates {
			if gate.Kind == "nix-store" && gate.Subject == root {
				approval = gate.Expected
			}
		}
		if step.Kind != "command" || !step.Irreversible || approval == "" {
			continue
		}
		outcome := legacyOutcomeNone
		// An attempt that failed and a newer attempt that never reached the
		// worker both leave the root partly deleted.
		for attempt := intent.Attempts[intent.Next]; attempt >= 0 && outcome == legacyOutcomeNone; attempt-- {
			probe := intent
			probe.Attempts = slices.Clone(intent.Attempts)
			probe.Attempts[intent.Next] = attempt
			result, err := d.stepOutcome(probe, intent.Next)
			if err != nil {
				return nil, nil, err
			}
			outcome = result.Status
		}
		if outcome == legacyOutcomeUnknown || outcome == legacyOutcomeFailed {
			deletions = append(deletions, legacyNixDeletion{Operation: operation, Approval: approval, Outcome: outcome})
		}
	}
	return deletions, unreadable, nil
}

// planNixDeletionRecovery plans the remainder of a dispatched deletion of this
// same Nix root. It reports false when no saved operation concerns this root,
// so a later reinstallation is classified normally.
func (d *legacyRetirementDriver) planNixDeletionRecovery(ctx context.Context, scope *legacyRetirementScope) (bool, error) {
	root := d.sys("/nix")
	deletions, unreadable, err := d.nixInterruptedDeletions(root)
	if err != nil {
		return false, err
	}
	for _, path := range unreadable {
		scope.Detail = append(scope.Detail, "Saved retirement journal "+path+" is unreadable and authorizes nothing.")
	}
	if len(deletions) == 0 {
		return false, nil
	}
	records, err := d.nixMembership(ctx, root)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		// The earlier deletion finished after the root was listed; only inert
		// leftovers remain.
		return true, d.planNixLeftovers(ctx, scope, false)
	}
	var chosen *legacyNixDeletion
	chosenSize, reasons := 0, []string{}
	for i := range deletions {
		approved, err := d.loadNixMembership(deletions[i].Approval)
		if err != nil {
			reasons = append(reasons, "operation "+deletions[i].Operation+": "+err.Error())
			continue
		}
		if !approved[records[0]] {
			continue // A different Nix root; that deletion does not concern it.
		}
		if outside := legacyNixOutside(records, approved); outside != "" {
			reasons = append(reasons, filepath.Join(root, outside)+" was not part of the deletion approved in operation "+deletions[i].Operation)
			continue
		}
		if chosen == nil || len(approved) < chosenSize {
			chosen, chosenSize = &deletions[i], len(approved)
		}
	}
	if chosen == nil {
		if len(reasons) == 0 {
			return false, nil
		}
		scope.Blocked = "an earlier retirement started deleting " + root + ", and its approval cannot finish what remains: " + strings.Join(reasons, "; ") + "; preserve it and inspect it manually"
		return true, nil
	}
	if active, err := d.nixDeletionActive(ctx, root); err != nil {
		return true, err
	} else if active {
		scope.Blocked = "the deletion of " + root + " started by operation " + chosen.Operation + " is still running; wait for it to exit, then review retirement again"
		return true, nil
	}
	consumers, err := d.nixConsumers(ctx, false, "")
	if err != nil {
		return true, err
	}
	if consumers != "" {
		scope.Blocked = consumers + "; remove or migrate those consumers before finishing the deletion"
		return true, nil
	}
	if hints, err := d.nixProcessHints(ctx); err != nil {
		return true, err
	} else if hints != "" {
		scope.Notices = append(scope.Notices, "Processes still use Nix ("+hints+"). Close them before the deletion is finished; it is rechecked with administrator rights immediately beforehand.")
	}
	scope.Native = "upstream Nix 2.34.0 deletion approved in operation " + chosen.Operation
	scope.Detail = append(scope.Detail, fmt.Sprintf("Operation %s approved deleting %s and its native outcome is %s. %d of its %d approved members remain and nothing else is present; only those are deleted.",
		chosen.Operation, root, chosen.Outcome, len(records), chosenSize))
	approval, err := digest(records)
	if err != nil {
		return true, err
	}
	for _, plan := range []func(context.Context, *legacyRetirementScope) error{d.planLoginShell, d.planHomeManager} {
		if err := plan(ctx, scope); err != nil || scope.Blocked != "" {
			return true, err
		}
	}
	if err := d.discloseSystemShellHooks(scope); err != nil {
		return true, err
	}
	links, reason, err := d.linuxNixUnitLinks()
	if err != nil || reason != "" {
		scope.Blocked = reason
		return true, err
	}
	if len(links) != 0 {
		// A restart may have reactivated the daemon socket over the remainder.
		if err := d.planLinuxDaemonStop(scope, []legacyRetirementGate{{Kind: "replacements"}, {Kind: "nix-homebrew"}, {Kind: "query-git", Subject: root}, {Kind: "nix-store", Subject: root, Expected: approval}}); err != nil {
			return true, err
		}
		if err := d.planResolvedNixActivationLinks(scope, links); err != nil {
			return true, err
		}
	}
	step, err := d.nixDeletionStep("Finish deleting the partly deleted Nix store and runtime at /nix", approval)
	if err != nil {
		return true, err
	}
	scope.Steps = append(scope.Steps, step)
	if err := d.planLinuxNixIntegrations(ctx, scope); err != nil || scope.Blocked != "" {
		return true, err
	}
	return true, d.discloseNixRemoval(scope)
}
