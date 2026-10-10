package installer

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type nativeCommandRunner func(context.Context, bool, string, []byte, ...string) ([]byte, error)

var aptArgument = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*(?::[a-z0-9][a-z0-9-]*)?$`)

type aptPackageStatus struct {
	Name, Version, Want, Status, Error string
}

func aptStatuses(ctx context.Context, run nativeCommandRunner, packages []string) ([]aptPackageStatus, error) {
	wanted := map[string]bool{}
	for _, name := range packages {
		if !aptArgument.MatchString(name) || wanted[name] {
			return nil, errors.New("invalid exact APT package argument")
		}
		wanted[name] = true
	}
	format := "${Package}:${Architecture}\t${Version}\t${db:Status-Want}\t${db:Status-Status}\t${db:Status-Eflag}\n"
	// Querying missing names exits nonzero and mixes errors into the output.
	// Read the bounded database once, then select only the exact requested names;
	// absence must remain observable after partial successful removal.
	output, err := run(ctx, false, "dpkg-query", nil, "-W", "-f="+format)
	if err != nil {
		return nil, fmt.Errorf("query exact native package state: %w\n%s", err, output)
	}
	if len(output) > 8<<20 {
		return nil, errors.New("native package status exceeds its bound")
	}
	result := []aptPackageStatus{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		name, _, _ := strings.Cut(fields[0], ":")
		if !wanted[fields[0]] && !wanted[name] {
			continue
		}
		if len(fields) != 5 || !aptPackageName.MatchString(fields[0]) || seen[fields[0]] {
			return nil, errors.New("invalid or duplicate native package status")
		}
		seen[fields[0]] = true
		result = append(result, aptPackageStatus{fields[0], fields[1], fields[2], fields[3], fields[4]})
	}
	return result, nil
}

// dpkg refuses dependency-breaking removal, but still changes the requested
// package's selection from install to deinstall. Restore only refused, still-
// installed, unchanged candidates so a later dselect operation cannot mistake
// this failed removal for a standing removal request. Never purge or autoremove.
func removeAPTPackages(ctx context.Context, run nativeCommandRunner, packages []string) ([]byte, error) {
	if len(packages) == 0 {
		return nil, nil
	}
	before, err := aptStatuses(ctx, run, packages)
	if err != nil {
		return nil, err
	}
	if len(before) != len(packages) {
		return nil, errors.New("native removal requires present, unambiguous package architectures")
	}
	packages = make([]string, len(before))
	for i, pkg := range before {
		if pkg.Want != "install" || pkg.Status != "installed" || pkg.Error != "ok" {
			return nil, fmt.Errorf("%s has held, pending or incomplete native state; review it before removal", pkg.Name)
		}
		packages[i] = pkg.Name
	}
	output, removalErr := run(ctx, true, "dpkg", nil, append([]string{"--remove"}, packages...)...)
	if removalErr == nil {
		return output, nil
	}
	// Finishing this bounded restoration is cleanup of the already attempted
	// operation, including when its cancellation caused the failure.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	after, err := aptStatuses(ctx, run, packages)
	if err != nil {
		return output, errors.Join(removalErr, err)
	}
	previous := map[string]aptPackageStatus{}
	for _, pkg := range before {
		previous[pkg.Name] = pkg
	}
	var restore strings.Builder
	for _, pkg := range after {
		old, known := previous[pkg.Name]
		if !known {
			return output, errors.Join(removalErr, errors.New("native package identity changed during failed removal"))
		}
		if pkg.Status == "installed" && pkg.Want == "deinstall" && pkg.Error == "ok" {
			if pkg.Version != old.Version {
				return output, errors.Join(removalErr, fmt.Errorf("%s changed version during failed removal; its current selection was preserved", pkg.Name))
			}
			fmt.Fprintf(&restore, "%s\t%s\n", pkg.Name, old.Want)
		}
	}
	if restore.Len() == 0 {
		return output, removalErr
	}
	restored, err := run(ctx, true, "dpkg", []byte(restore.String()), "--set-selections")
	if err != nil {
		return output, errors.Join(removalErr, fmt.Errorf("restore refused package selections: %w\n%s", err, restored))
	}
	verified, err := aptStatuses(ctx, run, packages)
	if err != nil {
		return output, errors.Join(removalErr, err)
	}
	for _, pkg := range verified {
		if pkg.Status == "installed" && pkg.Want != previous[pkg.Name].Want {
			return output, errors.Join(removalErr, fmt.Errorf("%s removal selection was not restored", pkg.Name))
		}
	}
	return output, removalErr
}
