package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var brewPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9+_.@-]{0,199}$`)

func brewShortName(full string) (string, error) {
	parts := strings.Split(full, "/")
	if len(parts) != 1 && len(parts) != 3 {
		return "", errors.New("invalid Homebrew package identity")
	}
	for _, part := range parts {
		if !brewPackageName.MatchString(part) {
			return "", errors.New("invalid Homebrew package identity")
		}
	}
	return parts[len(parts)-1], nil
}

// Read the manager's installed inventory, including cask consumers. Unknown
// native JSON fields are allowed: this is Homebrew's public v2 format, not our
// persisted schema. Missing runtime metadata is resolved through Homebrew's own
// installed-dependency query, never a guessed current formula dependency list.
func brewInventory(ctx context.Context, run nativeCommandRunner) (map[string]nativePackage, error) {
	data, err := run(ctx, false, "brew", nil, "info", "--json=v2", "--installed")
	if err != nil {
		return nil, fmt.Errorf("inspect installed Homebrew packages: %w", err)
	}
	if len(data) > 8<<20 {
		return nil, errors.New("Homebrew inventory exceeds 8 MiB")
	}
	var document struct {
		Formulae []struct {
			Name      string `json:"name"`
			FullName  string `json:"full_name"`
			Pinned    bool   `json:"pinned"`
			KegOnly   bool   `json:"keg_only"`
			Linked    string `json:"linked_keg"`
			Installed []struct {
				Version      string `json:"version"`
				OnRequest    *bool  `json:"installed_on_request"`
				Dependencies *[]struct {
					FullName string `json:"full_name"`
				} `json:"runtime_dependencies"`
			} `json:"installed"`
		} `json:"formulae"`
		Casks []struct {
			Token     string `json:"token"`
			FullToken string `json:"full_token"`
			Installed string `json:"installed"`
			DependsOn *struct {
				Formula []string `json:"formula"`
			} `json:"depends_on"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if document.Formulae == nil || document.Casks == nil {
		return nil, errors.New("Homebrew omitted installed inventory collections")
	}
	installed := map[string]nativePackage{}
	queryDependencies := func(kind, name string) ([]string, error) {
		// --direct selects declared formula metadata instead of actual runtime
		// dependencies. Keep Homebrew's runtime mode, including transitive edges.
		data, err := run(ctx, false, "brew", nil, "deps", "--installed", "--"+kind, name)
		if err != nil {
			return nil, fmt.Errorf("inspect installed dependencies of %s: %w", name, err)
		}
		if len(data) > 1<<20 {
			return nil, errors.New("Homebrew dependency list exceeds its bound")
		}
		return strings.Fields(string(data)), nil
	}
	add := func(pkg nativePackage) error {
		if _, duplicate := installed[pkg.Name]; duplicate || pkg.Version == "" {
			return errors.New("duplicate or incomplete Homebrew package identity")
		}
		for i, full := range pkg.Dependencies {
			name, err := brewShortName(full)
			if err != nil {
				return err
			}
			pkg.Dependencies[i] = name
		}
		pkg.Dependencies = sortedUnique(pkg.Dependencies)
		installed[pkg.Name] = pkg
		return nil
	}
	for _, formula := range document.Formulae {
		name, err := brewShortName(formula.FullName)
		if err != nil || name != formula.Name || len(formula.Installed) == 0 {
			return nil, errors.New("invalid installed Homebrew formula")
		}
		pkg := nativePackage{Name: name, Source: formula.FullName, Automatic: true, Held: formula.Pinned}
		versions := []string{}
		missing := false
		for _, keg := range formula.Installed {
			if keg.Version == "" || len(keg.Version) > 256 || strings.ContainsAny(keg.Version, "\x00\r\n") {
				return nil, errors.New("invalid Homebrew keg version")
			}
			versions = append(versions, keg.Version)
			pkg.Automatic = pkg.Automatic && keg.OnRequest != nil && !*keg.OnRequest
			if keg.Dependencies == nil {
				missing = true
				continue
			}
			for _, dep := range *keg.Dependencies {
				pkg.Dependencies = append(pkg.Dependencies, dep.FullName)
			}
		}
		if missing {
			deps, err := queryDependencies("formula", formula.FullName)
			if err != nil {
				return nil, err
			}
			pkg.Dependencies = append(pkg.Dependencies, deps...)
		}
		pkg.Version = strings.Join(sortedUnique(versions), ",")
		pkg.Healthy = formula.KegOnly || formula.Linked != "" && slices.Contains(versions, formula.Linked)
		if err := add(pkg); err != nil {
			return nil, err
		}
	}
	for _, cask := range document.Casks {
		name, err := brewShortName(cask.FullToken)
		if err != nil || name != cask.Token || cask.Installed == "" {
			return nil, errors.New("invalid installed Homebrew cask")
		}
		pkg := nativePackage{Name: "cask/" + name, Source: cask.FullToken, Version: cask.Installed, Healthy: true}
		if cask.DependsOn != nil {
			pkg.Dependencies = cask.DependsOn.Formula
		} else {
			pkg.Dependencies, err = queryDependencies("cask", cask.FullToken)
			if err != nil {
				return nil, err
			}
		}
		if err := add(pkg); err != nil {
			return nil, err
		}
	}
	return installed, nil
}
