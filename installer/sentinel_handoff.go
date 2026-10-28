package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// dotfiles 1.0 wrote the Sentinel rules block itself and copied the Sentinel
// skills through its configuration journal. The Sentinel installer replaces the
// rules block in place, because it uses the same markers. It refuses a skill
// folder without its .sentinel-owned marker, so setup first deletes the copies
// that its own receipts prove unchanged. These resources stay in the catalog
// only for this handoff, and their receipts are released after it.
const sentinelEarlierRules = "integration.sentinel"

var sentinelEarlierSkills = []string{"skills.sentinel-agents", "skills.sentinel-claude", "skills.sentinel-kiro", "skills.sentinel-copilot"}

// sentinelCopies sorts the folders of one earlier skills receipt. Unchanged
// folders hold the bytes that dotfiles copied, at the recorded physical path.
// Changed folders hold other bytes, or a link leads to them. Absent folders and
// folders that Sentinel now owns are in neither list. Both lists, and so each
// handoff report, use the physical path that the journal recorded.
type sentinelCopies struct{ Unchanged, Changed []string }

func (d *SentinelDriver) earlierCopies(id string, receipt Receipt) (sentinelCopies, error) {
	var copies sentinelCopies
	if receipt.Ownership == "" || receipt.Ownership == "reused" {
		return copies, nil // dotfiles owned nothing, or a removal already finished.
	}
	if receipt.Ownership != "created" || receipt.Status == "in-progress" {
		return copies, errors.New("an earlier dotfiles operation on these skills did not finish; use dotfiles 1.0 to finish it, or to remove these skills, first")
	}
	j, err := (&ConfigDriver{Directory: d.State}).readRecoveryJournal(id, receipt)
	if err != nil {
		return copies, err
	}
	if !j.Complete || j.Cancelled || j.Restoring || j.Action == "remove" {
		return copies, errors.New("the saved journal does not prove a finished copy")
	}
	for _, entry := range j.Entries {
		// The journal holds the physical path that dotfiles wrote. A link in
		// a parent can lead the same path to a copy that dotfiles never wrote.
		path := entry.State.Target.Destination
		bound, err := bindConfigDestination(path)
		if err != nil {
			return copies, err
		}
		current, err := snapshotConfig(path)
		if err != nil {
			return copies, err
		}
		marker, err := os.Lstat(filepath.Join(path, ".sentinel-owned"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return copies, err
		}
		switch {
		case current.Kind == "absent":
		case bound != path:
			copies.Changed = append(copies.Changed, path)
		case err == nil && marker.Mode().IsRegular():
		case current == configPublicationResult(j, entry):
			copies.Unchanged = append(copies.Unchanged, path)
		default:
			copies.Changed = append(copies.Changed, path)
		}
	}
	return copies, nil
}

// handoff returns the unchanged earlier copies. A changed copy or a receipt
// without proof stops the handoff, and nothing is deleted.
func (d *SentinelDriver) handoff() ([]string, error) {
	unchanged := []string{}
	for _, id := range sentinelEarlierSkills {
		copies, err := d.earlierCopies(id, d.Earlier[id])
		if err != nil {
			return nil, fmt.Errorf("cannot verify the earlier dotfiles copies of %s: %w", id, err)
		}
		if len(copies.Changed) != 0 {
			return nil, fmt.Errorf("%s changed after dotfiles copied it, or a link now leads to it. Dotfiles leaves it in place, and Sentinel does not replace it. Move it to a different folder, then apply again", strings.Join(copies.Changed, ", "))
		}
		unchanged = append(unchanged, copies.Unchanged...)
	}
	return unchanged, nil
}

// handOver deletes the unchanged earlier copies, so that the Sentinel installer
// can write its own copies. It also drops the saved baseline of the earlier
// rules block. That baseline holds a block between the AGENT-RULES markers
// from an earlier Sentinel install. The Sentinel installer now owns the block,
// and its removal keeps the text outside the block, so nothing restores it.
func (d *SentinelDriver) handOver() error {
	unchanged, err := d.handoff()
	if err != nil {
		return err
	}
	for _, path := range unchanged {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	if recovery := d.Earlier[sentinelEarlierRules].Recovery; recovery != "" {
		if !validProfileRecovery(recovery) {
			return fmt.Errorf("invalid saved baseline reference %q of the earlier Sentinel rules", recovery)
		}
		return os.RemoveAll(filepath.Join(d.State, recovery))
	}
	return nil
}

// observeEarlier reports an earlier resource as absent when nothing is left to
// hand over, so that apply releases its receipt. Otherwise Pending keeps the
// receipt: while Sentinel is selected, its apply hands the files over. While
// Sentinel is not selected, the user selects it once to hand them over. The
// rules block is handed over when the Sentinel installer has run once.
func (d *SentinelDriver) observeEarlier(r Resource, receipt Receipt) Observation {
	o := Observation{Provider: "sentinel-handoff", Identity: r.ID, Scope: "user"}
	if receipt.Ownership == "" || receipt.Ownership == "reused" {
		return o
	}
	selected := slices.Contains(d.Selected, sentinelResource)
	handOver := "Select Sentinel rules and skills once to hand these earlier files to the Sentinel installer. Untick it after that to remove them."
	if r.ID == sentinelEarlierRules {
		// A complete record exists after the Sentinel installer ran.
		record, err := d.readRecord()
		switch {
		case !selected:
			o.Pending = handOver
		case err != nil:
			o.Pending = err.Error()
		case record == nil || !record.Complete:
			o.Pending = "The Sentinel installer replaces this rules block when it applies."
		}
		return o
	}
	copies, err := d.earlierCopies(r.ID, receipt)
	switch {
	case err != nil:
		o.Pending = "Cannot verify the earlier dotfiles Sentinel skills: " + err.Error()
	case len(copies.Changed) != 0:
		o.Pending = strings.Join(copies.Changed, ", ") + " changed after dotfiles copied it, or a link now leads to it. Dotfiles leaves it in place. Move it to a different folder, then apply again."
	case len(copies.Unchanged) == 0:
	case selected:
		o.Pending = "The Sentinel installer replaces these unchanged dotfiles copies when it applies."
	default:
		o.Pending = handOver
	}
	return o
}
