package installer

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// APT writes its history's Install list before invoking dpkg. An End-Date is
// also written on failure. New ownership therefore needs both a successful
// operation-bound history entry and dpkg's actual install/configuration records.
// The caller must use fresh operation-local files and verify the current package
// database separately; these records do not prove a package is still installed.
type aptInstalledPackage struct {
	Name      string
	Version   string
	Automatic bool
}

var aptPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*:[a-z0-9][a-z0-9-]*$`)
var aptInstallItem = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]*:[a-z0-9][a-z0-9-]*) \(([^ ,()\t\r\n]+)(, automatic)?\)(, |$)`)

func aptInstallEvidence(history, dpkgLog []byte, operation string) ([]aptInstalledPackage, error) {
	if !operationID.MatchString(operation) || len(history) == 0 || len(history) > 8<<20 || len(dpkgLog) > 8<<20 ||
		strings.ContainsRune(string(history), '\x00') || strings.ContainsRune(string(dpkgLog), '\x00') {
		return nil, errors.New("invalid or oversized APT operation evidence")
	}
	fields := map[string]string{}
	ended := false
	for _, line := range strings.Split(string(history), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok || key == "" || ended {
			return nil, errors.New("malformed or multiple APT history transactions")
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, errors.New("duplicate APT history field")
		}
		if len(fields) == 0 && key != "Start-Date" {
			return nil, errors.New("APT history has no transaction start")
		}
		fields[key] = value
		ended = key == "End-Date"
	}
	if !ended || fields["Start-Date"] == "" || fields["End-Date"] == "" {
		return nil, errors.New("APT transaction is incomplete")
	}
	if _, failed := fields["Error"]; failed {
		return nil, errors.New("APT history records a failed transaction")
	}
	marker := "Dotfiles::Operation=" + operation
	matched := 0
	for _, argument := range strings.Fields(fields["Commandline"]) {
		if argument == marker {
			matched++
		}
	}
	if matched != 1 {
		return nil, errors.New("APT transaction does not match the saved operation")
	}
	for _, destructive := range []string{"Remove", "Purge", "Downgrade", "Disappeared"} {
		if fields[destructive] != "" {
			return nil, fmt.Errorf("APT unexpectedly recorded %s; preserve the operation for inspection", destructive)
		}
	}
	packages := []aptInstalledPackage{}
	seen := map[string]bool{}
	for rest := fields["Install"]; rest != ""; {
		match := aptInstallItem.FindStringSubmatch(rest)
		if match == nil || seen[match[1]] || len(match[2]) > 256 {
			return nil, errors.New("invalid or duplicate APT installed package")
		}
		seen[match[1]] = true
		packages = append(packages, aptInstalledPackage{match[1], match[2], match[3] != ""})
		rest = rest[len(match[0]):]
		if rest == "" && match[4] != "" {
			return nil, errors.New("APT installed package list ends with an empty item")
		}
	}
	installed, configured := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(string(dpkgLog), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			if strings.TrimSpace(line) != "" {
				return nil, errors.New("truncated dpkg operation record")
			}
			continue
		}
		switch parts[2] {
		case "install":
			if len(parts) != 6 || !aptPackageName.MatchString(parts[3]) {
				return nil, errors.New("invalid dpkg installation record")
			}
			if _, duplicate := installed[parts[3]]; duplicate {
				return nil, errors.New("multiple dpkg installs require explicit recovery inspection")
			}
			installed[parts[3]] = parts[5]
		case "status":
			if len(parts) != 6 || !aptPackageName.MatchString(parts[4]) {
				return nil, errors.New("invalid dpkg status record")
			}
			delete(configured, parts[4])
			if parts[3] == "installed" {
				configured[parts[4]] = parts[5]
			}
		}
	}
	proven := map[string]bool{}
	for i := range packages {
		pkg := &packages[i]
		// APT resolves Architecture: all through a native-architecture cache
		// entry, but dpkg records the installed identity as name:all. Use that
		// actual identity only when no competing exact-architecture record exists.
		if installed[pkg.Name] == "" && configured[pkg.Name] == "" {
			name, _, _ := strings.Cut(pkg.Name, ":")
			if installed[name+":all"] == pkg.Version && configured[name+":all"] == pkg.Version {
				pkg.Name = name + ":all"
			}
		}
		if installed[pkg.Name] != pkg.Version || configured[pkg.Name] != pkg.Version {
			return nil, fmt.Errorf("%s lacks actual completed dpkg installation evidence", pkg.Name)
		}
		if proven[pkg.Name] {
			return nil, errors.New("multiple APT entries claim the same dpkg identity")
		}
		proven[pkg.Name] = true
	}
	return packages, nil
}
