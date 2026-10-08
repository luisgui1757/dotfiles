package installer

import (
	"errors"
	"slices"
	"strings"
)

// Classification comes from the original native operation, including explicit
// manual roots later retained in the pool. A user's auto-to-manual promotion
// never turns an incidental dependency into an owned manual root.
func (d *LinuxAPTDriver) matchesOwnership(owned linuxAPTOwnership, pkg nativePackage) bool {
	if pkg.Name != owned.Name || pkg.Source != owned.Source || d.proveOwnership(owned) != nil {
		return false
	}
	intent, err := d.intent(owned.CreatedBy)
	if err != nil {
		return false
	}
	introduced, _, _, err := d.attemptEvidence(intent)
	proof, ok := introduced[owned.Name]
	base, _, _ := strings.Cut(owned.Name, ":")
	return err == nil && ok && pkg.Automatic == proof.Automatic &&
		(pkg.Automatic || intent.Package == owned.Name || intent.Package == base)
}

func (d *LinuxAPTDriver) ownedConsumer(state linuxAPTLedger, pkg nativePackage) bool {
	if pkg.Held || !pkg.Healthy {
		return false
	}
	if owned, ok := state.Pool[pkg.Name]; ok && d.matchesOwnership(owned, pkg) {
		return true
	}
	for _, owned := range state.Roots {
		if owned.Name == pkg.Name && d.matchesOwnership(owned, pkg) {
			return true
		}
	}
	return false
}

func (d *LinuxAPTDriver) removalCandidates(resource string, state linuxAPTLedger, installed map[string]nativePackage) ([]string, error) {
	root, ok := state.Roots[resource]
	if !ok {
		return nil, errors.New("APT removal lacks root ownership")
	}
	roots, pool, keep := []string{}, []string{}, []string{}
	if pkg, present := installed[root.Name]; present {
		if !d.matchesOwnership(root, pkg) {
			return nil, errors.New("APT root changed native ownership before removal")
		}
		roots = append(roots, root.Name)
	}
	for _, name := range d.Keep {
		resolved, err := linuxAPTIdentity(name, installed)
		if err != nil {
			return nil, err
		}
		if resolved != "" {
			keep = append(keep, resolved)
		}
	}
	for id, owned := range state.Roots {
		if id != resource {
			keep = append(keep, owned.Name)
		}
	}
	for name, owned := range state.Pool {
		if err := d.proveOwnership(owned); err != nil {
			return nil, err
		}
		pkg, present := installed[name]
		if !present || !d.matchesOwnership(owned, pkg) {
			continue
		}
		if pkg.Automatic {
			pool = append(pool, name)
		} else {
			roots = append(roots, name)
		}
	}
	candidates, _, err := nativePoolCandidates(roots, pool, keep, installed)
	if err != nil {
		return nil, err
	}
	// A changed or external consumer must stay visible to approval. Demotion is
	// permitted only when direct consumers are still proved provider-owned.
	if _, present := installed[root.Name]; present && !slices.Contains(candidates, root.Name) {
		for _, pkg := range installed {
			if slices.Contains(pkg.Dependencies, root.Name) && !d.ownedConsumer(state, pkg) {
				return nil, errors.New("APT root acquired an outside consumer; review a new plan")
			}
		}
	}
	return candidates, nil
}
