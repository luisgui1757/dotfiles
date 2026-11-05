package installer

import (
	"errors"
	"slices"
)

const sentinelEarlierRules = "integration.sentinel"

var sentinelEarlierSkills = []string{"skills.sentinel-agents", "skills.sentinel-claude", "skills.sentinel-kiro", "skills.sentinel-copilot"}

func (d *SentinelDriver) observeEarlier(r Resource, receipt Receipt) (Observation, error) {
	o := Observation{Provider: "sentinel-handoff", Identity: r.ID, Scope: "user"}
	if receipt.Ownership == "" || receipt.Ownership == "reused" {
		return o, nil
	}
	record, err := d.readRecord()
	switch {
	case err != nil:
		o.Pending = err.Error()
		return o, nil
	case record != nil && record.Complete:
		return o, nil
	}
	step := "This installer no longer hands the Sentinel files that dotfiles 1.0 copied to the Sentinel installer. Check out " + supportFloor + ", open setup and select Sentinel rules and skills in Choose tools. Then update this checkout and choose Update tools"
	if !slices.Contains(d.Selected, sentinelResource) {
		o.Pending = step + "."
		return o, nil
	}
	return o, errors.New(step)
}
