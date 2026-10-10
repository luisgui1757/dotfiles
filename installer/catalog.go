package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
)

// Context describes the target, not the machine on which a plan was generated.
type Context struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func (c Context) Validate() error {
	if c.OS != "darwin" && c.OS != "linux" && c.OS != "windows" {
		return fmt.Errorf("unsupported OS %q", c.OS)
	}
	if c.OS == "darwin" && c.Arch != "arm64" {
		return fmt.Errorf("macOS requires Apple Silicon and the native arm64 installer; %s is unsupported", c.Arch)
	}
	if c.Arch != "amd64" && c.Arch != "arm64" {
		return fmt.Errorf("unsupported architecture %q", c.Arch)
	}
	if c.OS == "windows" && c.Arch != "amd64" {
		return fmt.Errorf("unsupported or unaudited target %s/%s", c.OS, c.Arch)
	}
	return nil
}

type Resource struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description,omitempty"`
	Capability       bool                `json:"capability,omitempty"`
	Platforms        []string            `json:"platforms,omitempty"`
	Requires         []string            `json:"requires,omitempty"`
	PlatformRequires map[string][]string `json:"platform_requires,omitempty"`
	Action           string              `json:"action,omitempty"`
	Scope            string              `json:"scope,omitempty"`
	Retain           bool                `json:"retain,omitempty"`
	Shared           bool                `json:"shared,omitempty"`
	Bindings         map[string]Binding  `json:"bindings,omitempty"`
	// ReplanAfter marks retained bootstrap infrastructure. Its verified creation
	// can change which provider inventory is observable; later mutations require
	// a new preview instead of inheriting approval of an unseen inventory.
	ReplanAfter bool `json:"replan_after,omitempty"`
}

// Bindings identify the canonical package plane, separately from prerequisites.
// An infrastructure dependency alone must never imply package ownership.
type Binding struct {
	Provider string `json:"provider"`
	Package  string `json:"package"`
}

type Catalog struct {
	Schema    int        `json:"schema"`
	Resources []Resource `json:"resources"`
}

var resourceID = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,79}$`)

func (r Resource) Available(c Context) bool {
	return len(r.Platforms) == 0 || slices.Contains(r.Platforms, c.OS)
}

func (r Resource) Dependencies(c Context) []string {
	deps := append([]string{}, r.Requires...)
	deps = append(deps, r.PlatformRequires[c.OS]...)
	sort.Strings(deps)
	return slices.Compact(deps)
}

func (c *Catalog) Validate() error {
	if c.Schema != 1 {
		return fmt.Errorf("unsupported catalog schema %d", c.Schema)
	}
	seen := make(map[string]bool, len(c.Resources))
	packageOwners := map[string]string{}
	for _, r := range c.Resources {
		if !resourceID.MatchString(r.ID) || r.Name == "" {
			return fmt.Errorf("invalid resource %q", r.ID)
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate resource %q", r.ID)
		}
		if r.Scope != "" && r.Scope != "user" && r.Scope != "machine" {
			return fmt.Errorf("invalid scope for %s", r.ID)
		}
		if r.ReplanAfter && (!r.Retain || r.Action == "" || r.Capability || len(r.Bindings) != 0) {
			return fmt.Errorf("replan boundary %s must be retained bootstrap infrastructure", r.ID)
		}
		for _, os := range r.Platforms {
			if os != "darwin" && os != "linux" && os != "windows" {
				return fmt.Errorf("invalid platform %q", os)
			}
		}
		for os := range r.PlatformRequires {
			if os != "darwin" && os != "linux" && os != "windows" {
				return fmt.Errorf("invalid dependency platform %q", os)
			}
		}
		for os, binding := range r.Bindings {
			if (os != "darwin" && os != "linux" && os != "windows") || !r.Available(Context{OS: os}) || binding.Package == "" || r.Action == "" {
				return fmt.Errorf("invalid package binding for %s on %s", r.ID, os)
			}
			if binding.Provider != "archive" && binding.Provider != "homebrew-formula" && binding.Provider != "apt" {
				return fmt.Errorf("invalid package provider %q for %s", binding.Provider, r.ID)
			}
			if binding.Provider == "apt" && (os != "linux" || !aptArgument.MatchString(binding.Package)) {
				return fmt.Errorf("invalid APT binding for %s", r.ID)
			}
			if binding.Provider == "homebrew-formula" {
				if _, err := brewShortName(binding.Package); err != nil || os != "darwin" || !slices.Contains(r.Dependencies(Context{OS: os}), "infra.homebrew") {
					return fmt.Errorf("invalid Homebrew formula binding for %s", r.ID)
				}
			}
			key := os + "/" + binding.Provider + "/" + binding.Package
			if previous, exists := packageOwners[key]; exists {
				return fmt.Errorf("package %s is assigned to both %s and %s", key, previous, r.ID)
			}
			packageOwners[key] = r.ID
		}
		seen[r.ID] = true
	}
	for _, os := range []string{"darwin", "linux", "windows"} {
		{
			ctx := Context{OS: os}
			roots := []string{}
			for _, r := range c.Resources {
				if r.Available(ctx) {
					roots = append(roots, r.ID)
				}
			}
			if _, err := c.Closure(roots, ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

// The small fixed catalog is immutable during use. Linear lookup avoids a
// mutable validation cache shared by otherwise read-only previews.
func (c *Catalog) Resource(id string) (Resource, bool) {
	for _, r := range c.Resources {
		if r.ID == id {
			return r, true
		}
	}
	return Resource{}, false
}

func (c *Catalog) Capabilities(ctx Context) []Resource {
	result := []Resource{}
	for _, r := range c.Resources {
		if r.Capability && r.Available(ctx) {
			result = append(result, r)
		}
	}
	return result
}

// Closure returns prerequisites before consumers and rejects an invalid graph.
// It deliberately does not use persisted reference counts.
func (c *Catalog) Closure(roots []string, ctx Context) ([]string, error) {
	result, marks := []string{}, map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		r, ok := c.Resource(id)
		if !ok {
			return fmt.Errorf("unknown resource %q", id)
		}
		if !r.Available(ctx) {
			return fmt.Errorf("%s is unavailable on %s", r.Name, ctx.OS)
		}
		if marks[id] == 1 {
			return fmt.Errorf("dependency cycle at %s", id)
		}
		if marks[id] == 2 {
			return nil
		}
		marks[id] = 1
		for _, dependency := range r.Dependencies(ctx) {
			if err := visit(dependency); err != nil {
				return fmt.Errorf("%s requires %s: %w", id, dependency, err)
			}
		}
		marks[id] = 2
		result = append(result, id)
		return nil
	}
	ordered := append([]string{}, roots...)
	sort.Strings(ordered)
	for _, root := range ordered {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
