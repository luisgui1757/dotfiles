package installer

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

type brewInstalledKeg struct {
	Name, Version, Path string
}

// Homebrew's documented install badge is printed by FormulaInstaller#finish
// after the build, while holding its package locks. Capture that child's
// output with a per-operation badge; timestamps and global inventory deltas do
// not identify the installing actor. A failed link still prints the badge: the
// caller must verify the successful command result, native keg receipt,
// required activation and pre-state before completing package installation.
func brewInstallEvidence(output []byte, cellar, operation string) ([]brewInstalledKeg, error) {
	absolute := path.IsAbs(cellar) || filepath.IsAbs(filepath.FromSlash(cellar))
	if !operationID.MatchString(operation) || len(output) > 8<<20 || !absolute || path.Clean(cellar) != cellar || strings.ContainsAny(cellar, "\\\x00\r\n") {
		return nil, errors.New("invalid Homebrew operation evidence boundary")
	}
	marker := "dotfiles-operation-" + operation + "  "
	result := []brewInstalledKeg{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, marker) {
			continue
		}
		keg, summary, ok := strings.Cut(strings.TrimPrefix(line, marker), ": ")
		if !ok || summary == "" || !strings.HasPrefix(keg, cellar+"/") || path.Clean(keg) != keg {
			return nil, errors.New("invalid operation-marked Homebrew keg summary")
		}
		parts := strings.Split(strings.TrimPrefix(keg, cellar+"/"), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(keg, "\x00\r\n") || seen[parts[0]] {
			return nil, errors.New("ambiguous operation-marked Homebrew package")
		}
		seen[parts[0]] = true
		result = append(result, brewInstalledKeg{parts[0], parts[1], keg})
	}
	return result, nil
}
