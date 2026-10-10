package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maxArchiveBytes int64 = 512 << 20
const maxPackageBytes int64 = 2 << 30
const maxPackageEntries = 100000

// ArchivePin describes upstream bytes and bounded preparation metadata.
// It never supplies an arbitrary command or a remote script.
// Paths are portable, relative to the extracted payload after StripComponents.
type ArchivePin struct {
	Version             string                  `json:"version"`
	URL                 string                  `json:"url"`
	SHA256              string                  `json:"sha256"`
	Format              string                  `json:"format"`
	StripComponents     int                     `json:"strip_components,omitempty"`
	File                string                  `json:"file,omitempty"`
	Commands            map[string]string       `json:"commands"`
	BinDirs             []string                `json:"bin_dirs"`
	RequiredFiles       []string                `json:"required_files,omitempty"`
	ExcludedFiles       []string                `json:"excluded_files,omitempty"`
	Replacements        []ArchiveReplacement    `json:"replacements,omitempty"`
	Libc                string                  `json:"libc,omitempty"`
	PythonStdlib        string                  `json:"python_stdlib,omitempty"`
	RustTarget          string                  `json:"rust_target,omitempty"`
	RustComponents      []ArchivePin            `json:"rust_components,omitempty"`
	WindowsRustLauncher *WindowsRustLauncherPin `json:"windows_rust_launcher,omitempty"`
	Latex2text          *Latex2textPin          `json:"latex2text,omitempty"`
	Yamllint            *YamllintPin            `json:"yamllint,omitempty"`
	PortableGit         bool                    `json:"portable_git,omitempty"`
	GhosttyLibraries    bool                    `json:"ghostty_libraries,omitempty"`
}

// Corrections are reviewed data in the pin, never commands. Apply only to the
// new checksum-verified payload, with the exact reviewed count (one by default).
type ArchiveReplacement struct {
	File   string `json:"file"`
	Before string `json:"before"`
	After  string `json:"after"`
	Count  int    `json:"count,omitempty"`
}

func (p ArchivePin) Validate() error {
	if p.PythonStdlib != "" && (!portableArchivePath(p.PythonStdlib) || p.Commands["python"] == "" && p.Commands["python3"] == "") {
		return errors.New("Python bytecode preparation requires a declared interpreter and relative standard library")
	}
	if p.Libc != "" && p.Libc != "glibc" && p.Libc != "musl" {
		return errors.New("unsupported archive libc requirement")
	}
	u, err := url.Parse(p.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || !operationID.MatchString(p.SHA256) || p.Version == "" {
		return errors.New("archive requires a version, HTTPS URL and SHA-256 pin")
	}
	if !slices.Contains([]string{"tar.gz", "zip", "gzip", "file", "deb"}, p.Format) || p.StripComponents < 0 || p.StripComponents > 4 || (len(p.Commands) == 0 && len(p.RequiredFiles) == 0) || (len(p.Commands) > 0 && len(p.BinDirs) == 0) {
		return errors.New("invalid archive format, layout or command list")
	}
	if (p.Format == "gzip" || p.Format == "file") && (!portableArchivePath(p.File) || p.StripComponents != 0) {
		return errors.New("single-file archive requires a safe output filename")
	}
	for name, file := range p.Commands {
		if !resourceID.MatchString(name) || !portableArchivePath(file) {
			return errors.New("invalid archive command mapping")
		}
	}
	for _, file := range p.RequiredFiles {
		if !portableArchivePath(file) {
			return errors.New("invalid required archive file")
		}
	}
	for i, file := range p.ExcludedFiles {
		if !portableArchivePath(file) || slices.Contains(p.ExcludedFiles[:i], file) || slices.Contains(p.RequiredFiles, file) {
			return errors.New("invalid, repeated or required archive exclusion")
		}
		for _, command := range p.Commands {
			if file == command {
				return errors.New("archive exclusion removes a declared command")
			}
		}
	}
	for _, replacement := range p.Replacements {
		if !portableArchivePath(replacement.File) || slices.Contains(p.ExcludedFiles, replacement.File) || replacement.Before == "" || replacement.Before == replacement.After || replacement.Count < 0 || len(replacement.Before) > 1<<20 || len(replacement.After) > 1<<20 {
			return errors.New("invalid reviewed archive replacement")
		}
	}
	for _, dir := range p.BinDirs {
		if dir != "." && !portableArchivePath(dir) {
			return errors.New("invalid archive binary directory")
		}
	}
	return validateArchivePreparation(p)
}

// Apply Windows filename constraints everywhere so a pin has one interpretation
// on all hosts (including reserved devices, alternate streams and trailing dots).
func portableArchivePath(name string) bool {
	if !canonicalRelative(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, ". ") != part || strings.ContainsAny(part, "<>\"|?*\x00") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if slices.Contains([]string{"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$"}, base) || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return false
		}
	}
	return true
}

// Download before extraction, verify the complete compressed bytes, and never
// execute a fetched installer. The destination must be a fresh private directory.
func downloadArchive(ctx context.Context, client *http.Client, pin ArchivePin, destination string) error {
	return downloadArchiveWithDeb(ctx, client, pin, destination, "")
}

func downloadArchiveWithDeb(ctx context.Context, client *http.Client, pin ArchivePin, destination, dpkgDeb string) (result error) {
	if err := pin.Validate(); err != nil {
		return err
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Minute}
	}
	// Preserve an injected transport (native tests use TLS) but prohibit redirects
	// that weaken transport authentication or introduce URL credentials.
	safe := *client
	safe.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil {
			return errors.New("unsafe archive redirect")
		}
		if client.CheckRedirect != nil {
			return client.CheckRedirect(req, via)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.URL, nil)
	if err != nil {
		return err
	}
	response, err := safe.Do(req)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxArchiveBytes {
		return fmt.Errorf("archive download rejected: HTTP %d, length %d", response.StatusCode, response.ContentLength)
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close(), os.Remove(file.Name())) }()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxArchiveBytes+1))
	if err != nil {
		return err
	}
	if n > maxArchiveBytes || hex.EncodeToString(hash.Sum(nil)) != pin.SHA256 {
		return errors.New("archive size or SHA-256 differs from the reviewed pin")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return extractArchive(ctx, file, n, pin, destination, dpkgDeb)
}

func extractArchive(ctx context.Context, file *os.File, size int64, pin ArchivePin, destination, dpkgDeb string) (result error) {
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	caseSensitive, err := archiveCaseSensitivity(root)
	if err != nil {
		return err
	}
	remaining, entries := maxPackageBytes, 0
	seen := map[string]bool{}
	links := map[string]string{}
	excluded := map[string]bool{}
	write := func(raw string, mode os.FileMode, data io.Reader, link string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxPackageEntries {
			return errors.New("archive exceeds entry limit")
		}
		name := strings.TrimSuffix(strings.TrimPrefix(raw, "./"), "/")
		if name == "." || name == "" {
			if mode.IsDir() {
				return nil
			}
			return errors.New("archive file has no name")
		}
		if !portableArchivePath(name) {
			return fmt.Errorf("unsafe archive path %q", raw)
		}
		parts := strings.Split(name, "/")
		if len(parts) <= pin.StripComponents {
			if mode.IsDir() {
				return nil
			}
			return errors.New("archive stripping would discard a file")
		}
		name = strings.Join(parts[pin.StripComponents:], "/")
		key := name
		if !caseSensitive {
			key = strings.ToLower(name)
		}
		if seen[key] {
			return fmt.Errorf("duplicate archive path %q", name)
		}
		seen[key] = true
		if mode&os.ModeSymlink != 0 && (strings.ContainsAny(link, "\\:\x00\r\n") || path.IsAbs(link) || !portableArchivePath(path.Clean(path.Join(path.Dir(name), link)))) {
			return errors.New("archive link escapes its payload")
		}
		if slices.Contains(pin.ExcludedFiles, name) {
			if !mode.IsRegular() && mode&os.ModeSymlink == 0 {
				return errors.New("archive exclusion must identify an exact regular file or symbolic link")
			}
			n, err := io.Copy(io.Discard, io.LimitReader(data, remaining+1))
			remaining -= n
			if remaining < 0 {
				return errors.Join(err, errors.New("archive exceeds expanded size limit"))
			}
			excluded[name] = true
			return err
		}
		if err := root.MkdirAll(filepath.FromSlash(path.Dir(name)), 0755); err != nil {
			return err
		}
		if mode.IsDir() {
			return root.MkdirAll(filepath.FromSlash(name), 0755)
		}
		if mode&os.ModeSymlink != 0 {
			// Links are published last; they cannot redirect any extraction write.
			links[name] = link
			return nil
		}
		if !mode.IsRegular() {
			return errors.New("archive contains unsupported special files")
		}
		output, err := root.OpenFile(filepath.FromSlash(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644|mode.Perm()&0111)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(output, io.LimitReader(data, remaining+1))
		err = errors.Join(copyErr, output.Sync(), output.Close())
		remaining -= n
		if remaining < 0 {
			return errors.Join(err, errors.New("archive exceeds expanded size limit"))
		}
		return err
	}
	readTar := func(stream io.Reader) error {
		reader := tar.NewReader(stream)
		for {
			header, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if header.Typeflag == tar.TypeXGlobalHeader {
				// Git source archives carry a global commit comment. Go
				// parses these records but does not apply global file
				// attributes: accept comments, reject semantic overrides.
				// https://pkg.go.dev/archive/tar#TypeXGlobalHeader
				entries++
				if entries > maxPackageEntries {
					return errors.New("archive exceeds entry limit")
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				for key := range header.PAXRecords {
					if key != "comment" {
						return fmt.Errorf("unsupported global archive attribute %s", key)
					}
				}
				continue
			}
			if header.Typeflag == tar.TypeLink {
				return errors.New("archive hard links require an explicit supported layout")
			}
			if err := write(header.Name, header.FileInfo().Mode(), reader, header.Linkname); err != nil {
				return err
			}
		}
		// Consume the trailer so truncated streams and bad gzip CRCs fail.
		trailing, err := io.Copy(io.Discard, io.LimitReader(stream, remaining+1))
		if err != nil {
			return err
		}
		if trailing > remaining {
			return errors.New("archive trailer exceeds expanded size limit")
		}
		return nil
	}
	switch pin.Format {
	case "tar.gz", "gzip":
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, gz.Close()) }()
		if pin.Format == "gzip" {
			if err := write(pin.File, 0755, gz, ""); err != nil {
				return err
			}
		} else {
			if err := readTar(gz); err != nil {
				return err
			}
		}
	case "deb":
		if err := readDebianArchive(ctx, file, dpkgDeb, readTar); err != nil {
			return err
		}
	case "zip":
		reader, err := zip.NewReader(file, size)
		if err != nil {
			return err
		}
		for _, member := range reader.File {
			input, err := member.Open()
			if err != nil {
				return err
			}
			link := ""
			if member.Mode()&os.ModeSymlink != 0 {
				// Unix ZIP links store the target as member data. Bound that
				// data before the same deferred publication checks used by tar.
				data, readErr := io.ReadAll(io.LimitReader(input, 4097))
				remaining -= int64(len(data))
				if len(data) > 4096 || remaining < 0 {
					readErr = errors.Join(readErr, errors.New("ZIP symbolic link exceeds its size bound"))
				}
				if readErr != nil {
					return errors.Join(readErr, input.Close())
				}
				link = string(data)
			}
			err = errors.Join(write(member.Name, member.Mode(), input, link), input.Close())
			if err != nil {
				return err
			}
		}
	case "file":
		if err := write(pin.File, 0755, file, ""); err != nil {
			return err
		}
	default:
		return errors.New("unsupported archive format")
	}
	for _, name := range pin.ExcludedFiles {
		if !excluded[name] {
			return fmt.Errorf("reviewed archive exclusion is absent: %s", name)
		}
	}
	for name, link := range links {
		if err := root.Symlink(filepath.FromSlash(link), filepath.FromSlash(name)); err != nil {
			return err
		}
	}
	for name := range links {
		resolved, err := filepath.EvalSymlinks(filepath.Join(destination, filepath.FromSlash(name)))
		relative, relErr := filepath.Rel(destination, resolved)
		if err != nil || relErr != nil || !filepath.IsLocal(relative) {
			return errors.Join(errors.New("archive contains a dangling or escaping link"), err, relErr)
		}
	}
	for _, replacement := range pin.Replacements {
		if err := replaceArchiveText(root, replacement); err != nil {
			return err
		}
	}
	if err := checkArchivePayload(root, pin); err != nil {
		return err
	}
	return syncConfigDirectory(destination)
}

// The extraction directory is new and empty. Probe its filesystem, not the OS:
// Linux can use a case-folding mount and macOS can use case-sensitive APFS.
// Python's Linux terminfo archive legitimately contains case-distinct names.
func archiveCaseSensitivity(root *os.Root) (sensitive bool, result error) {
	const first = ".dotfiles-case-probe-A"
	const second = ".dotfiles-case-probe-a"
	file, err := root.OpenFile(first, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, root.Remove(first)) }()
	if err := file.Close(); err != nil {
		return false, err
	}
	file, err = root.OpenFile(second, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, errors.Join(file.Close(), root.Remove(second))
}
