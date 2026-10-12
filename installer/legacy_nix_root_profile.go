package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/scanner"
)

// These are the three NIX_INSTALLED_* paths in the checksum-verified 2.34.0
// install-multi-user scripts used by the released prerequisite helper. Names
// alone cannot distinguish the installer runtime from root's own packages.
var legacyNixRootPackages = map[string][]string{
	"linux/amd64":  {"1kxmbqsah0bszi95w1ii633vzciqnanx-nix-2.34.0", "k5i9jidynqr8kwylgpqf5j8z8pfkr6fl-nix-manual-2.34.0-man", "cv910ahrajv1p2lvy6phasy2b1d9nxcd-nss-cacert-3.117"},
	"linux/arm64":  {"xs9inr2d17npz3y05y4kqvk5qikq5yp6-nix-2.34.0", "aw051x2542w3wnlc79ab54v12kf33lcq-nix-manual-2.34.0-man", "2g4zfwsrkydpisqk3lz42cf9ak2lfvnc-nss-cacert-3.117"},
	"darwin/arm64": {"m5d4xy2qnlg2gzvw1k95fqpp7n73zhdq-nix-2.34.0", "xvyr4n1lkhbq8gskipbzbfnnj4i9f1bn-nix-manual-2.34.0-man", "a5rxxcslmcvm987j9acr8b1bwx7yfmvh-nss-cacert-3.117"},
}

const legacyNixRootProfileProof = "var/nix/profiles/per-user/root#installer-profile=v1"

var legacyRootProfileGeneration = regexp.MustCompile(`^profile-[1-3]-link$`)
var legacyNixStorePackage = regexp.MustCompile(`^[0-9abcdfghijklmnpqrsvwxyz]{32}-(.+)$`)
var legacyNixBootstrapCA = regexp.MustCompile(`^nss-cacert-3\.[0-9]{1,4}(\.[0-9]{1,4})?$`)
var legacyNixBootstrapUpstream = regexp.MustCompile(`^nix-2\.[0-9]{1,4}\.[0-9]{1,4}$`)

// Tokenize a bounded literal manifest, never evaluate Nix. Comparing the exact
// serialized records below rejects expressions, interpolation, extra fields,
// duplicate fields and different output paths. Quoted whitespace is preserved.
func legacyNixManifestTokens(data []byte) ([]string, error) {
	var lex scanner.Scanner
	lex.Init(bytes.NewReader(data))
	lex.Mode = scanner.ScanIdents | scanner.ScanStrings
	var invalid error
	lex.Error = func(_ *scanner.Scanner, message string) { invalid = errors.New(message) }
	tokens := []string{}
	for token := lex.Scan(); token != scanner.EOF; token = lex.Scan() {
		if invalid != nil {
			return nil, invalid
		}
		if len(tokens) == 512 || len(lex.TokenText()) > 4096 {
			return nil, errors.New("Nix default-profile manifest exceeds its inspection bound")
		}
		tokens = append(tokens, lex.TokenText())
	}
	return tokens, invalid
}

func legacyNixManifestRecord(name, path string, system bool) []string {
	field := ""
	if system {
		field = `system = "unknown"; `
	}
	// All strings are validated absolute paths or package names, so Go's
	// quoted representation is also the literal serialization emitted by Nix.
	data := fmt.Sprintf(`{ meta = { }; name = %q; out = { outPath = %q; }; outPath = %q; outputs = [ "out" ]; %stype = "derivation"; }`, name, path, path, field)
	tokens, err := legacyNixManifestTokens([]byte(data))
	if err != nil {
		panic(err) // Fixed records and previously bounded names/paths only.
	}
	return tokens
}

func (d *legacyRetirementDriver) nixRootManifest(data []byte) (bool, error) {
	tokens, err := legacyNixManifestTokens(data)
	if err != nil || len(tokens) < 3 || tokens[0] != "[" || tokens[len(tokens)-1] != "]" {
		return false, nil
	}
	store := d.sys("/nix/store")
	var vendor *legacyDeterminateReceipt
	if receipt, _, err := readLegacyFile(d.sys(legacyDeterminateReceiptPath), legacyDeterminateReceiptLimit); err != nil {
		return false, err
	} else if receipt != nil {
		parsed, err := parseLegacyDeterminateReceipt(receipt)
		if err != nil {
			return false, err
		}
		vendor = &parsed
	}
	seen, hasNix := map[string]bool{}, false
	for remaining := tokens[1 : len(tokens)-1]; len(remaining) != 0; {
		if remaining[0] != "{" || len(seen) == 3 {
			return false, nil
		}
		depth, end := 0, 0
		for i, token := range remaining {
			if token == "{" {
				depth++
			}
			if token == "}" {
				depth--
			}
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end == 0 {
			return false, nil
		}
		entry := remaining[:end]
		remaining = remaining[end:]
		// The exact record comparison still checks these fields' names and
		// positions. Decode only the two string slots needed to construct it.
		if len(entry) < 17 {
			return false, nil
		}
		name, nameErr := strconv.Unquote(entry[8])
		path, pathErr := strconv.Unquote(entry[15])
		if nameErr != nil || pathErr != nil || seen[name] || filepath.Dir(path) != store || strings.ContainsAny(path, "\x00\r\n") {
			return false, nil
		}
		if !slices.Equal(entry, legacyNixManifestRecord(name, path, false)) && !slices.Equal(entry, legacyNixManifestRecord(name, path, true)) {
			return false, nil
		}
		if vendor == nil {
			if !slices.Contains(legacyNixRootPackages[d.Platform.OS+"/"+d.Platform.Arch], filepath.Base(path)) || filepath.Base(path)[33:] != name {
				return false, nil
			}
			hasNix = hasNix || name == "nix-2.34.0"
		} else {
			// The vendor's SetupDefaultProfile installs its Nix and CA bundle.
			// Its receipt has no store-path fields. Require the exact serialized
			// two-package vocabulary and matching store path names;
			// arbitrary extra packages remain another account's consumer.
			match := legacyNixStorePackage.FindStringSubmatch(filepath.Base(path))
			if len(match) != 2 || match[1] != name {
				return false, nil
			}
			nix := vendor.Determinate && name == "determinate-nix-"+vendor.Version || !vendor.Determinate && legacyNixBootstrapUpstream.MatchString(name)
			if !nix && !legacyNixBootstrapCA.MatchString(name) {
				return false, nil
			}
			hasNix = hasNix || nix
		}
		seen[name] = true
	}
	return hasNix, nil
}

func (d *legacyRetirementDriver) nixRootDefaultProfile(directory string, children []os.DirEntry, approvedRemainder bool) (bool, error) {
	store := d.sys("/nix/store")
	current, generations := "", map[string]bool{}
	for _, child := range children {
		path := filepath.Join(directory, child.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return false, err
		}
		if child.Name() == "profile" && legacyRootProfileGeneration.MatchString(target) {
			current = target
			continue
		}
		if !legacyRootProfileGeneration.MatchString(child.Name()) || filepath.Dir(target) != store {
			return false, nil
		}
		manifest, err := filepath.EvalSymlinks(filepath.Join(path, "manifest.nix"))
		if errors.Is(err, os.ErrNotExist) {
			if approvedRemainder {
				continue
			}
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !strings.HasPrefix(manifest, store+"/") {
			return false, nil
		}
		data, _, err := readLegacyFile(manifest, 64<<10)
		if err != nil {
			return false, err
		}
		if data == nil && approvedRemainder {
			continue
		}
		known, err := d.nixRootManifest(data)
		if err != nil || !known {
			return false, err
		}
		generations[child.Name()] = true
	}
	if approvedRemainder {
		return true, nil
	}
	if !generations[current] {
		return false, nil
	}
	// Only the runtime profile linked by the installer is exempt.
	defaultProfile, err := filepath.EvalSymlinks(d.sys("/nix/var/nix/profiles/default"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	rootProfile, err := filepath.EvalSymlinks(filepath.Join(directory, "profile"))
	return defaultProfile == rootProfile, err
}

// A partial deletion may remove a manifest before its root-profile links. In
// that case only an intact earlier deletion approval can exempt the remainder.
// The root inode and every surviving member must still belong to that approval.
func (d *legacyRetirementDriver) nixApprovedProfileRemainder(ctx context.Context, directory string, children []os.DirEntry) (bool, error) {
	if d.Platform.OS != "linux" {
		return false, nil
	}
	deletions, _, err := d.nixInterruptedDeletions(d.sys("/nix"))
	if err != nil || len(deletions) == 0 {
		return false, err
	}
	records, err := d.nixMembership(ctx, d.sys("/nix"))
	if err != nil || len(records) == 0 {
		return false, err
	}
	for _, deletion := range deletions {
		approved, err := d.loadNixMembership(deletion.Approval)
		if err != nil {
			return false, err
		}
		if approved[legacyNixRootProfileProof] && legacyNixOutside(records, approved) == "" {
			return d.nixRootDefaultProfile(directory, children, true)
		}
	}
	return false, nil
}
