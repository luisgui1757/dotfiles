package installer

import (
	"errors"
	"fmt"
	"slices"
)

// Native managers provide these observations. Dependencies name installed
// runtime consumers' actual requirements, including virtual/capability
// resolution where the manager uses them. This is not a package version solver.
type nativePackage struct {
	Name         string
	Version      string
	Automatic    bool
	Held         bool
	Healthy      bool
	Source       string
	Dependencies []string
}

// nativeRemovalCandidates narrows an already approved attributable pool. The
// native manager still refuses dependency-breaking removal at execution: this
// snapshot never authorizes dependency-check bypass, cascading removal or
// autoremove. Homebrew's formula-only --force means all installed versions;
// its separate --ignore-dependencies bypass is forbidden.
// Keep includes selected roots even when another operation introduced them as
// incidental dependencies. A manually promoted incidental is no longer ours.
func nativeRemovalCandidates(roots, pool, keep []string, installed map[string]nativePackage) ([]string, map[string][]string, error) {
	candidates, protected, err := nativePoolCandidates(roots, pool, keep, installed)
	if err != nil {
		return nil, protected, err
	}
	for _, root := range roots {
		if _, present := installed[root]; present && !slices.Contains(candidates, root) {
			return nil, protected, fmt.Errorf("%s is held, kept, or required by another installed package; preserve it", root)
		}
	}
	return candidates, protected, nil
}

// nativePoolCandidates also permits retained roots: callers must prove their
// original manual classification and retain ownership of protected packages.
func nativePoolCandidates(roots, pool, keep []string, installed map[string]nativePackage) ([]string, map[string][]string, error) {
	wanted := map[string]bool{}
	for _, name := range append(slices.Clone(pool), roots...) {
		pkg, present := installed[name]
		if !present {
			continue
		}
		if pkg.Name != name || name == "" || pkg.Version == "" {
			return nil, nil, errors.New("native package inventory has an invalid identity")
		}
		if slices.Contains(keep, name) || pkg.Held || !pkg.Automatic && !slices.Contains(roots, name) {
			continue
		}
		wanted[name] = true
	}
	// Iteration handles chains and cycles without maintaining reference counts.
	// Removing one candidate from the set may protect its dependencies too.
	protected := map[string][]string{}
	for changed := true; changed; {
		changed = false
		for name, pkg := range installed {
			if pkg.Name != name || name == "" || pkg.Version == "" {
				return nil, nil, fmt.Errorf("invalid native package identity %q", name)
			}
			if wanted[name] {
				continue
			}
			for _, dependency := range pkg.Dependencies {
				if wanted[dependency] {
					delete(wanted, dependency)
					protected[dependency] = append(protected[dependency], name)
					changed = true
				}
			}
		}
	}
	// Report every immediate outside consumer, independent of map iteration order.
	for name := range protected {
		consumers := []string{}
		for consumer, pkg := range installed {
			if !wanted[consumer] && slices.Contains(pkg.Dependencies, name) {
				consumers = append(consumers, consumer)
			}
		}
		protected[name] = sortedUnique(consumers)
	}
	result := make([]string, 0, len(wanted))
	for name := range wanted {
		result = append(result, name)
	}
	return sortedUnique(result), protected, nil
}
