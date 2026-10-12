package installer

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// Retirement queries outlive the runtimes they retire. Repository identity is
// therefore read from Git metadata as data, and the Git executable used to
// prove pinned checkouts is chosen outside any runtime retirement can remove.

// discoverLegacyGit prefers setup's own Git, then the operating system's, and
// only then the caller's PATH. A Nix or retirable Homebrew Git still works,
// but retirement of that runtime is gated while Git-dependent evidence remains.
func discoverLegacyGit(normal Controller, platform NativePlatform) (string, error) {
	candidates := []string{}
	if native, ok := normal.Driver.(*NativeDriver); ok && native.Archives != nil && platform.OS == "windows" {
		if path, err := native.Archives.CommandPath("tool.git", "git"); err == nil {
			candidates = append(candidates, path)
		}
	}
	switch platform.OS {
	case "linux":
		candidates = append(candidates, "/usr/bin/git")
	case "darwin":
		if platform.Homebrew != nil {
			candidates = append(candidates, filepath.Join(platform.Homebrew.Prefix, "bin", "git"))
		}
		// The CommandLineTools payload itself; /usr/bin/git would offer an
		// installation dialog when the payload is missing.
		candidates = append(candidates, "/Library/Developer/CommandLineTools/usr/bin/git")
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && (platform.OS == "windows" || info.Mode()&0111 != 0) {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	path, err := exec.LookPath("git")
	if errors.Is(err, exec.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("git resolved through a relative PATH entry")
	}
	return filepath.Clean(path), nil
}

// legacyGitRuntimeDependents names released artifacts that only Git can prove
// while the retirement's Git executable lives below runtime. Retiring that
// runtime first would leave them unprovable, so it waits for them instead.
func (d *legacyRetirementDriver) legacyGitRuntimeDependents(runtime string) ([]string, error) {
	program := d.Environment.Programs["git"]
	if program == "" || runtime == "" {
		return nil, nil
	}
	resolved, err := resolveConfigPath(program)
	if errors.Is(err, os.ErrNotExist) {
		resolved, err = program, nil
	}
	if err != nil {
		return nil, err
	}
	inside := func(path string) bool {
		relative, err := filepath.Rel(runtime, path)
		return err == nil && (relative == "." || filepath.IsLocal(relative))
	}
	if !inside(program) && !inside(resolved) {
		return nil, nil
	}
	dependents := []string{}
	if d.Platform.OS != "windows" {
		for _, checkout := range d.legacyCheckouts() {
			if _, err := os.Lstat(checkout.Path); err == nil {
				dependents = append(dependents, checkout.Path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	return dependents, nil
}

func (d *legacyRetirementDriver) queryGitGate(subject string) (string, error) {
	dependents, err := d.legacyGitRuntimeDependents(subject)
	return strings.Join(dependents, ","), err
}

// legacyGitRepository locates a worktree's Git directory and the common
// directory holding its configuration, following gitfiles and commondir.
func legacyGitRepository(top string) (string, string, error) {
	dotgit := filepath.Join(top, ".git")
	info, err := os.Lstat(dotgit)
	if err != nil {
		return "", "", err
	}
	gitdir := dotgit
	switch {
	case info.IsDir() && info.Mode()&os.ModeSymlink == 0:
	case info.Mode().IsRegular():
		data, _, err := readLegacyFile(dotgit, 4096)
		if err != nil {
			return "", "", err
		}
		value, ok := strings.CutPrefix(strings.TrimRight(string(data), "\r\n"), "gitdir: ")
		if !ok || value == "" || strings.ContainsAny(value, "\x00\r\n") {
			return "", "", fmt.Errorf("%s is not a gitfile", dotgit)
		}
		gitdir = filepath.FromSlash(value)
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(top, gitdir)
		}
		gitdir = filepath.Clean(gitdir)
	default:
		return "", "", fmt.Errorf("%s is neither a Git directory nor a gitfile", dotgit)
	}
	if head, err := os.Lstat(filepath.Join(gitdir, "HEAD")); err != nil || !head.Mode().IsRegular() {
		return "", "", errors.Join(fmt.Errorf("%s has no HEAD", gitdir), err)
	}
	common := gitdir
	data, _, err := readLegacyFile(filepath.Join(gitdir, "commondir"), 4096)
	if err != nil {
		return "", "", err
	}
	if data != nil {
		value := filepath.FromSlash(strings.TrimRight(string(data), "\r\n"))
		if value == "" || strings.ContainsAny(value, "\x00\r\n") {
			return "", "", fmt.Errorf("%s has an invalid commondir", gitdir)
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(gitdir, value)
		}
		common = filepath.Clean(value)
	}
	return gitdir, common, nil
}

// legacyGitLocalConfig reads the repository's own configuration file, the
// file `git config --local` reads; includes are not followed there either.
func legacyGitLocalConfig(top string) (map[string][]string, error) {
	_, common, err := legacyGitRepository(top)
	if err != nil {
		return nil, err
	}
	data, _, err := readLegacyFile(filepath.Join(common, "config"), 1<<20)
	if err == nil && data == nil {
		err = errors.New("the repository has no configuration file")
	}
	if err != nil {
		return nil, err
	}
	return parseLegacyGitConfig(data)
}

func legacyGitOrigin(top string) (string, error) {
	values, err := legacyGitLocalConfig(top)
	if err != nil {
		return "", err
	}
	urls := values["remote.origin.url"]
	if len(urls) != 1 {
		return "", fmt.Errorf("the repository configures %d origin URLs", len(urls))
	}
	return urls[0], nil
}

// parseLegacyGitConfig implements git-config(1)'s file syntax as data: section
// and key names are case-insensitive, subsections exact, values unquoted and
// unescaped. Anything Git itself would reject is refused.
func parseLegacyGitConfig(data []byte) (map[string][]string, error) {
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return nil, errors.New("Git configuration is not UTF-8 text")
	}
	text := strings.ReplaceAll(strings.TrimPrefix(string(data), "\ufeff"), "\r\n", "\n")
	values := map[string][]string{}
	section, i := "", 0
	name := func(c byte) bool {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
	}
	for i < len(text) {
		switch c := text[i]; {
		case c == ' ' || c == '\t' || c == '\n':
			i++
			continue
		case c == '#' || c == ';':
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		case c == '[':
			i++
			start := i
			for i < len(text) && (name(text[i]) || text[i] == '.') {
				i++
			}
			if i == start || i == len(text) {
				return nil, errors.New("invalid Git configuration section")
			}
			section = strings.ToLower(text[start:i])
			if text[i] == ' ' || text[i] == '\t' {
				for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
					i++
				}
				if i == len(text) || text[i] != '"' || strings.Contains(section, ".") {
					return nil, errors.New("invalid Git configuration subsection")
				}
				var sub strings.Builder
				for i++; i < len(text) && text[i] != '"'; i++ {
					if text[i] == '\n' {
						return nil, errors.New("unterminated Git configuration subsection")
					}
					if text[i] == '\\' {
						i++
						if i == len(text) || text[i] == '\n' {
							return nil, errors.New("invalid Git configuration subsection escape")
						}
					}
					sub.WriteByte(text[i])
				}
				if i == len(text) {
					return nil, errors.New("unterminated Git configuration subsection")
				}
				i++
				section += "." + sub.String()
			}
			if i == len(text) || text[i] != ']' {
				return nil, errors.New("invalid Git configuration section header")
			}
			i++
			continue
		case !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') || section == "":
			return nil, errors.New("invalid Git configuration line")
		}
		start := i
		for i < len(text) && name(text[i]) {
			i++
		}
		key := section + "." + strings.ToLower(text[start:i])
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i == len(text) || text[i] == '\n' || text[i] == '#' || text[i] == ';' {
			values[key] = append(values[key], "true")
			continue
		}
		if text[i] != '=' {
			return nil, errors.New("invalid Git configuration variable")
		}
		i++
		var value strings.Builder
		pending, quoted := "", false
	value:
		for i < len(text) {
			c := text[i]
			switch {
			case c == '\n':
				if quoted {
					return nil, errors.New("unterminated Git configuration quote")
				}
				i++
				break value
			case !quoted && (c == '#' || c == ';'):
				for i < len(text) && text[i] != '\n' {
					i++
				}
				break value
			case c == '\\':
				i++
				if i == len(text) {
					return nil, errors.New("invalid Git configuration escape")
				}
				replacement, ok := legacyGitEscapes[text[i]]
				if !ok {
					return nil, errors.New("invalid Git configuration escape")
				}
				if text[i] != '\n' {
					value.WriteString(pending + replacement)
					pending = ""
				}
				i++
			case c == '"':
				quoted = !quoted
				value.WriteString(pending)
				pending = ""
				i++
			case !quoted && (c == ' ' || c == '\t'):
				if value.Len() != 0 {
					pending += string(c)
				}
				i++
			default:
				value.WriteString(pending)
				value.WriteByte(c)
				pending = ""
				i++
			}
		}
		if quoted {
			return nil, errors.New("unterminated Git configuration quote")
		}
		values[key] = append(values[key], value.String())
	}
	return values, nil
}

var legacyGitEscapes = map[byte]string{'\n': "", 'n': "\n", 't': "\t", 'b': "\b", '"': `"`, '\\': `\`}

// legacyGitBlobIDs are the object names Git would give these bytes, as stored
// and with a checkout's CRLF conversion undone.
func legacyGitBlobIDs(data []byte, format string) ([]string, error) {
	variants := [][]byte{data}
	if normalized := []byte(strings.ReplaceAll(string(data), "\r\n", "\n")); len(normalized) != len(data) {
		variants = append(variants, normalized)
	}
	ids := []string{}
	for _, variant := range variants {
		header := fmt.Sprintf("blob %d\x00", len(variant))
		switch format {
		case "", "sha1":
			sum := sha1.Sum(append([]byte(header), variant...))
			ids = append(ids, hex.EncodeToString(sum[:]))
		case "sha256":
			sum := sha256.Sum256(append([]byte(header), variant...))
			ids = append(ids, hex.EncodeToString(sum[:]))
		default:
			return nil, fmt.Errorf("unknown Git object format %q", format)
		}
	}
	return ids, nil
}

// legacyBlobInHistory proves that the repository's history recorded exactly
// these bytes at path. Git only reads objects; no hook or filter runs.
func (d *legacyRetirementDriver) legacyBlobInHistory(ctx context.Context, top, path string, data []byte) (bool, error) {
	if err := d.refuseGitSettings(ctx, top, legacyGitPartialClonePattern, "partial-clone fetches"); err != nil {
		return false, err
	}
	values, err := legacyGitLocalConfig(top)
	if err != nil {
		return false, err
	}
	formats := values["extensions.objectformat"]
	if len(formats) > 1 {
		return false, errors.New("the repository declares several object formats")
	}
	format := ""
	if len(formats) == 1 {
		format = strings.ToLower(formats[0])
	}
	ids, err := legacyGitBlobIDs(data, format)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		output, err := legacyMemoized(ctx, "git-blob\x00"+top+"\x00"+path+"\x00"+id, false, func() (string, error) {
			return d.gitInspect(ctx, top, "--no-pager", "log", "-n", "1", "--format=%H", "--find-object="+id, "HEAD", "--", filepath.ToSlash(path))
		})
		if err != nil {
			return false, err
		}
		if operationCommit(output) {
			return true, nil
		}
	}
	return false, nil
}

func operationCommit(text string) bool {
	return (len(text) == 40 || len(text) == 64) && strings.Trim(text, "0123456789abcdef") == ""
}

func legacyRepositoryURL(url string, known ...string) bool {
	url = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	return slices.ContainsFunc(known, func(candidate string) bool { return strings.EqualFold(url, candidate) })
}
