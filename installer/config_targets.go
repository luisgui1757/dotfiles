package installer

import (
	_ "embed"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed config-targets.json
var configTargetData []byte

type ConfigTarget struct {
	Resource  string   `json:"resource"`
	Source    string   `json:"source"`
	Folder    string   `json:"folder"`
	Path      string   `json:"path"`
	Platforms []string `json:"platforms,omitempty"`
	Directory bool     `json:"directory,omitempty"`
	Mode      string   `json:"mode,omitempty"`
}

type ConfigManifest struct {
	Schema  int            `json:"schema"`
	Targets []ConfigTarget `json:"targets"`
}

// ConfigFolders holds actual known-folder results, never reconstructed Windows
// paths. Each selected root may be redirected independently, including to a
// different volume. The resolver does not create or traverse destinations.
type ConfigFolders struct {
	Home, Config, LocalAppData, AppData, Documents string
	// Zsh is discovered at the process boundary. An explicit folder fixture must
	// never inherit the caller's ZDOTDIR; empty means Home.
	Zsh string
	// Data is the observed XDG_DATA_HOME (or native default), independently of
	// Config. An empty fixture uses the target OS default, never caller settings.
	Data string
	// Programs is FOLDERID_Programs, observed independently on Windows.
	Programs string
}

type ResolvedConfigTarget struct {
	Source, Destination string
	Directory           bool
	Mode                string
}

func DefaultConfigManifest(c *Catalog) (ConfigManifest, error) {
	var manifest ConfigManifest
	if err := Decode(configTargetData, &manifest); err != nil {
		return manifest, err
	}
	return manifest, manifest.Validate(c)
}

func canonicalRelative(value string) bool {
	return value != "." && !path.IsAbs(value) && path.Clean(value) == value &&
		filepath.IsLocal(filepath.FromSlash(value)) && !strings.ContainsAny(value, "\\\x00\r\n:")
}

func (m ConfigManifest) Validate(c *Catalog) error {
	if m.Schema != 1 {
		return errors.New("unsupported configuration target schema")
	}
	seen := map[string]bool{}
	owners := map[string]bool{}
	for _, target := range m.Targets {
		r, found := c.Resource(target.Resource)
		if !found || r.Action != "config" || !canonicalRelative(target.Source) || !canonicalRelative(target.Path) {
			return fmt.Errorf("invalid configuration mapping for %s", target.Resource)
		}
		if target.Mode != "" && !(target.Mode == "copy" && !target.Directory) && (target.Mode != "junction" || !target.Directory || len(target.Platforms) != 1 || target.Platforms[0] != "windows") {
			return errors.New("explicit configuration modes require a file copy or Windows-only directory junction")
		}
		if !slices.Contains([]string{"home", "config", "local_app_data", "app_data", "documents"}, target.Folder) {
			return fmt.Errorf("unknown configuration folder %s", target.Folder)
		}
		for _, os := range target.Platforms {
			if !slices.Contains([]string{"darwin", "linux", "windows"}, os) || !r.Available(Context{OS: os}) {
				return fmt.Errorf("configuration target has invalid platform %s", os)
			}
		}
		for _, os := range []string{"darwin", "linux", "windows"} {
			if !r.Available(Context{OS: os}) || len(target.Platforms) != 0 && !slices.Contains(target.Platforms, os) {
				continue
			}
			if os != "windows" && slices.Contains([]string{"local_app_data", "app_data", "documents"}, target.Folder) {
				return errors.New("Windows known folder mapped to a POSIX target")
			}
			key := os + "/" + target.Folder + "/" + strings.ToLower(target.Path)
			if seen[key] {
				return fmt.Errorf("duplicate configuration destination %s", key)
			}
			seen[key], owners[os+"/"+r.ID] = true, true
		}
	}
	for _, r := range c.Resources {
		for _, os := range []string{"darwin", "linux", "windows"} {
			if r.Action == "config" && r.Available(Context{OS: os}) && !owners[os+"/"+r.ID] {
				return fmt.Errorf("configuration resource %s has no target on %s", r.ID, os)
			}
		}
	}
	return nil
}

func (m ConfigManifest) Resolve(c *Catalog, target Context, folders ConfigFolders, repository, resource string) ([]ResolvedConfigTarget, error) {
	if err := m.Validate(c); err != nil {
		return nil, err
	}
	r, exists := c.Resource(resource)
	if !exists || r.Action != "config" || !r.Available(target) || !filepath.IsAbs(repository) || filepath.Clean(repository) != repository {
		return nil, errors.New("configuration resolution requires an available resource and canonical source root")
	}
	roots := map[string]string{"home": folders.Home, "config": folders.Config, "local_app_data": folders.LocalAppData, "app_data": folders.AppData, "documents": folders.Documents}
	resolved := []ResolvedConfigTarget{}
	for _, item := range m.Targets {
		if item.Resource != resource || len(item.Platforms) != 0 && !slices.Contains(item.Platforms, target.OS) {
			continue
		}
		root := roots[item.Folder]
		if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.ContainsAny(root, "\x00\r\n") {
			return nil, fmt.Errorf("selected configuration requires the actual %s known folder", item.Folder)
		}
		source := filepath.Join(repository, filepath.FromSlash(item.Source))
		destination := filepath.Join(root, filepath.FromSlash(item.Path))
		// A source nested in its destination would be moved/deleted by adoption.
		for _, pair := range [][2]string{{source, destination}, {destination, source}, {destination, repository}, {repository, destination}} {
			relative, err := filepath.Rel(pair[0], pair[1])
			if err == nil && (relative == "." || filepath.IsLocal(relative)) {
				return nil, fmt.Errorf("source and destination overlap for %s", resource)
			}
		}
		mode := "link"
		if target.OS == "windows" {
			mode = "copy"
		}
		if item.Mode != "" {
			mode = item.Mode
		}
		resolved = append(resolved, ResolvedConfigTarget{source, destination, item.Directory, mode})
	}
	return resolved, nil
}
