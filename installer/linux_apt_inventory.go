package installer

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const linuxAPTInventoryFormat = "${Package}:${Architecture}\t${Version}\t${db:Status-Want}\t${db:Status-Status}\t${db:Status-Eflag}\t${source:Package}\t${Pre-Depends}\t${Depends}\t${Provides}\n"

// Resolve every installed alternative conservatively, including virtual
// Provides. dpkg remains the final authority at removal; this is not a version
// solver, and ambiguous alternatives never grant extra removal authority.
var linuxAPTDependency = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]*)(?::([a-z0-9][a-z0-9-]*))?(?:\s+\((?:<<|<=|=|>=|>>)\s+[^()\s]+\))?$`)

func linuxAPTDependencies(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	names := []string{}
	for _, group := range strings.Split(value, ",") {
		for _, alternative := range strings.Split(group, "|") {
			match := linuxAPTDependency.FindStringSubmatch(strings.TrimSpace(alternative))
			if match == nil {
				return nil, fmt.Errorf("unsupported installed APT dependency %q", alternative)
			}
			name := match[1]
			if match[2] != "" && match[2] != "any" && match[2] != "native" {
				name += ":" + match[2]
			}
			names = append(names, name)
		}
	}
	return sortedUnique(names), nil
}

func linuxAPTInventory(ctx context.Context, run nativeCommandRunner) (map[string]nativePackage, error) {
	output, err := run(ctx, false, "dpkg-query", nil, "-W", "-f="+linuxAPTInventoryFormat)
	if err != nil {
		return nil, err
	}
	automatic, err := run(ctx, false, "apt-mark", nil, "showauto")
	if err != nil {
		return nil, err
	}
	return parseLinuxAPTInventory(output, automatic)
}

func parseLinuxAPTInventory(output, automatic []byte) (map[string]nativePackage, error) {
	if len(output) > 8<<20 || len(automatic) > 8<<20 || strings.ContainsRune(string(output), 0) || strings.ContainsRune(string(automatic), 0) {
		return nil, errors.New("invalid or oversized APT inventory")
	}
	auto := map[string]bool{}
	for _, name := range strings.Fields(string(automatic)) {
		if !aptArgument.MatchString(name) || auto[name] {
			return nil, errors.New("invalid or duplicate APT automatic package")
		}
		auto[name] = true
	}
	installed := map[string]nativePackage{}
	requirements, aliases := map[string][]string{}, map[string][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 9 || !aptPackageName.MatchString(fields[0]) {
			return nil, errors.New("invalid APT package database row")
		}
		if fields[3] == "not-installed" || fields[3] == "config-files" {
			continue
		}
		name, _, _ := strings.Cut(fields[0], ":")
		if _, exists := installed[fields[0]]; exists || fields[1] == "" || !aptArgument.MatchString(fields[5]) {
			return nil, errors.New("invalid or duplicate APT package identity")
		}
		pkg := nativePackage{Name: fields[0], Version: fields[1], Source: fields[5],
			Automatic: auto[name] || auto[fields[0]], Held: fields[2] == "hold",
			Healthy: (fields[2] == "install" || fields[2] == "hold") && fields[3] == "installed" && fields[4] == "ok"}
		installed[pkg.Name] = pkg
		aliases[name] = append(aliases[name], pkg.Name)
		aliases[pkg.Name] = append(aliases[pkg.Name], pkg.Name)
		for _, value := range fields[6:8] {
			deps, err := linuxAPTDependencies(value)
			if err != nil {
				return nil, err
			}
			requirements[pkg.Name] = append(requirements[pkg.Name], deps...)
		}
		provides, err := linuxAPTDependencies(fields[8])
		if err != nil {
			return nil, err
		}
		for _, provided := range provides {
			aliases[provided] = append(aliases[provided], pkg.Name)
		}
	}
	for name, pkg := range installed {
		for _, dependency := range requirements[name] {
			pkg.Dependencies = append(pkg.Dependencies, aliases[dependency]...)
		}
		pkg.Dependencies = sortedUnique(pkg.Dependencies)
		installed[name] = pkg
	}
	return installed, nil
}

func linuxAPTIdentity(packageName string, installed map[string]nativePackage) (string, error) {
	identity := ""
	for name := range installed {
		base, _, _ := strings.Cut(name, ":")
		if name == packageName || base == packageName {
			if identity != "" {
				return "", errors.New("APT package has multiple installed architectures; use an exact package identity")
			}
			identity = name
		}
	}
	return identity, nil
}
