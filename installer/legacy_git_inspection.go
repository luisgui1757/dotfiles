package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const legacyGitPartialClonePattern = `^(remote\..*\.promisor|extensions\.partialclone)$`

func (d *legacyRetirementDriver) refuseGitSettings(ctx context.Context, path, pattern, reason string) error {
	settings, err := d.gitInspect(ctx, path, "config", "--includes", "--name-only", "--get-regexp", pattern)
	if code, ok := legacyExitCode(err); err != nil && !(ok && code == 1 && settings == "") {
		return err
	}
	if settings != "" {
		return fmt.Errorf("%s has %s; preserve and inspect it manually", path, reason)
	}
	return nil
}

// Git inspection must not execute repository filters, fetch promised objects,
// or recurse into independently configured submodules. The process boundary
// isolates system/global settings; query the effective local AND worktree
// configuration, including their includes, before inspecting worktree bytes.
func (d *legacyRetirementDriver) checkGitInspection(ctx context.Context, path string, allowUninitializedSubmodules bool) error {
	for _, rule := range []struct{ pattern, reason string }{
		{`^filter\..*\.(clean|process)$`, "executable Git filters"},
		{legacyGitPartialClonePattern, "partial-clone fetches"},
	} {
		if err := d.refuseGitSettings(ctx, path, rule.pattern, rule.reason); err != nil {
			return err
		}
	}
	index, err := d.git(ctx, path, "ls-files", "--stage", "-z")
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(string(index), "\x00") {
		if strings.HasPrefix(entry, "160000 ") {
			if !allowUninitializedSubmodules {
				return fmt.Errorf("%s has a submodule; preserve and inspect it manually", path)
			}
			_, relative, ok := strings.Cut(entry, "\t")
			if !ok || !filepath.IsLocal(relative) {
				return fmt.Errorf("%s has an invalid submodule path", path)
			}
			gitdir, common, err := legacyGitRepository(path)
			if err != nil {
				return err
			}
			// Released plugin clones did not initialize test submodules. An
			// empty worktree alone is insufficient: deinit leaves cached Git
			// data under modules, which may contain personal commits/settings.
			for _, directory := range slices.Compact([]string{filepath.Join(path, filepath.FromSlash(relative)), filepath.Join(gitdir, "modules"), filepath.Join(common, "modules")}) {
				entries, err := readPlainDirectory(directory)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("submodule data at %s cannot be inspected: %w", directory, err)
				}
				if len(entries) != 0 {
					return fmt.Errorf("%s has populated or cached submodule data; preserve and inspect it manually", path)
				}
			}
		}
	}
	return nil
}
