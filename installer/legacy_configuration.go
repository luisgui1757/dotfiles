package installer

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The inventory is generated from immutable release objects. A listed path is
// discovery evidence, never permission to overwrite a personal regular file.
//
//go:embed legacy-config-targets.json
var legacyConfigurationData []byte

func releasedConfigurationLinks(target Context, folders ConfigFolders, profiles []legacyProfile) ([]legacyProfile, error) {
	var inventory struct {
		Schema   int
		Releases []struct {
			Targets []struct {
				Root, Path, Source, Form string
				Platforms                []string
			}
		}
	}
	if err := json.Unmarshal(legacyConfigurationData, &inventory); err != nil {
		return nil, err
	}
	if inventory.Schema != 1 {
		return nil, errors.New("unsupported released configuration inventory")
	}
	roots := map[string]string{"home": folders.Home, "documents": folders.Documents, "local_app_data": folders.LocalAppData, "roaming_app_data": folders.AppData}
	seen := map[string]bool{}
	pathKey := func(path string) string {
		bound, err := bindConfigDestination(path)
		if err != nil {
			// A deduplication key grants no IO authority. Observation handles
			// unavailable parents without aborting unrelated migration items.
			bound = filepath.Clean(path)
		}
		if target.OS == "windows" {
			bound = strings.ToLower(bound)
		}
		return bound
	}
	for _, profile := range profiles {
		root := folders.Home
		if profile.Folder == "documents" {
			root = folders.Documents
		} else if profile.Folder == "config" {
			root = folders.Config
		}
		key := pathKey(filepath.Join(root, filepath.FromSlash(profile.Path)))
		seen[key] = true
	}
	for _, release := range inventory.Releases {
		for _, item := range release.Targets {
			if item.Form != "source-file" || !slices.Contains(item.Platforms, target.OS) {
				continue
			}
			root, known := roots[item.Root]
			if !known || !canonicalRelative(item.Path) || !canonicalRelative(item.Source) {
				return nil, errors.New("invalid released configuration link evidence")
			}
			if root == "" {
				continue // A pure fixture may omit unused Windows known folders.
			}
			path := filepath.Join(root, filepath.FromSlash(item.Path))
			key := pathKey(path)
			if seen[key] {
				continue
			}
			seen[key] = true
			folder := item.Root
			if folder == "roaming_app_data" {
				folder = "app_data"
			}
			id, err := digest(struct{ Root, Path string }{item.Root, item.Path})
			if err != nil {
				return nil, err
			}
			profiles = append(profiles, legacyProfile{ID: "legacy.config." + id[:16], Kind: "config", Folder: folder, Path: item.Path, Source: item.Source})
		}
	}
	return profiles, nil
}

func (d *legacyMigrationDriver) releasedLink(item legacyProfile) (bool, error) {
	path := d.destination(item)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false, err
	}
	link, err := os.Readlink(path)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	if item.Kind == "git" && filepath.Clean(link) == filepath.Join(d.checkout, "git", "config") {
		return true, nil
	}
	// A release-layout link can have acquired personal settings through the
	// old write-through behavior. Preserve all readable bytes, including edits.
	return strings.HasSuffix(filepath.ToSlash(filepath.Clean(link)), "/"+item.Source), nil
}
