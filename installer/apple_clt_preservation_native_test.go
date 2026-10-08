package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type appleCLTNativePreservation struct {
	Receipts []string                      `json:"receipts"`
	Paths    map[string]appleCLTNativePath `json:"paths"`
}
type appleCLTNativePath struct {
	Destination string            `json:"destination"`
	Metadata    string            `json:"metadata"`
	Link        string            `json:"link,omitempty"`
	Files       map[string]string `json:"files,omitempty"`
}

func appleCLTNativeGuard(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_APPLE_CLT") != "1" {
		t.Skip("requires explicitly enabled disposable Apple Silicon GitHub host")
	}
	base := os.Getenv("RUNNER_TEMP")
	if !filepath.IsAbs(base) {
		t.Fatal("runner temporary directory unavailable")
	}
	return filepath.Join(base, "preinstalled-apple-tools")
}

func appleCLTNativeSnapshot(t *testing.T, ctx context.Context, path, destination string, files []string) appleCLTNativePath {
	t.Helper()
	metadata, err := macOSPrerequisiteQuery(ctx, false, "/usr/bin/stat", nil, "-f", "%u:%g:%Lp:%m", path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	result := appleCLTNativePath{Destination: destination, Metadata: string(metadata)}
	if info.Mode()&os.ModeSymlink != 0 {
		result.Link, err = os.Readlink(path)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, name := range files {
		file := filepath.Join(path, filepath.FromSlash(name))
		info, err := os.Lstat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		// Only the fixed representative file is read, bounded by its observed
		// size. Native compiler binaries can exceed a configuration-size cap;
		// snapshotTree still streams the hash and rejects concurrent changes.
		snapshot, err := snapshotTree(file, info.Size(), 1)
		if err != nil {
			t.Fatal(file, err)
		}
		if result.Files == nil {
			result.Files = map[string]string{}
		}
		result.Files[name] = snapshot.Hash
	}
	return result
}

func appleCLTNativeReceipts(t *testing.T, ctx context.Context) []string {
	t.Helper()
	output, err := macOSPrerequisiteQuery(ctx, false, "/usr/sbin/pkgutil", nil, "--pkgs")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(id, "com.apple.pkg.CLTools") {
			ids = append(ids, id)
		}
	}
	return sortedUnique(ids)
}

// Runs before the fixture moves tools. The real selected tools must be reused
// and direct Apply must refuse them without dispatching a native mutation.
func TestNativeAppleCLTPreinstalledReuse(t *testing.T) {
	baseline := appleCLTNativeGuard(t)
	ctx := context.Background()
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := newNativeSession(filepath.Join(home, "worker"))
	d, err := configureAppleCLT(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, ConfigFolders{Home: home}, session)
	if err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: "infra.apple-clt", Retain: true, ReplanAfter: true}
	observed, err := d.Observe(ctx, r, Receipt{})
	if err != nil || !observed.Healthy || observed.CompletedOperation != "" {
		t.Fatal("fixture requires reusable preinstalled developer tools", observed, err)
	}
	commands := 0
	d.Run = func(context.Context, nativeCommand) ([]byte, error) {
		commands++
		return nil, errors.New("unexpected mutation")
	}
	if _, err := d.Apply(ctx, r, Operation{Action: "install", Observed: observed}, bootstrapReceipt("a")); err == nil || commands != 0 {
		t.Fatal("existing tools were acquired", err, commands)
	}
	saved := appleCLTNativePreservation{Receipts: appleCLTNativeReceipts(t, ctx), Paths: map[string]appleCLTNativePath{}}
	if !slices.Contains(saved.Receipts, "com.apple.pkg.CLTools_Executables") {
		t.Fatal("hosted fixture requires historical CLT receipts")
	}
	applications, err := filepath.Glob("/Applications/Xcode*.app")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range applications {
		saved.Paths[path] = appleCLTNativeSnapshot(t, ctx, path, filepath.Join(baseline, filepath.Base(path)), []string{"Contents/Info.plist", "Contents/_CodeSignature/CodeResources"})
	}
	entries, err := os.ReadDir("/Library/Developer")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 64 || len(applications) > 32 {
		t.Fatal("fixture exceeds bounded developer-directory inventory")
	}
	for _, entry := range entries {
		path := filepath.Join("/Library/Developer", entry.Name())
		destination := path
		var files []string
		if entry.Name() == "CommandLineTools" {
			destination = filepath.Join(baseline, entry.Name())
			files = []string{"usr/bin/clang", "usr/bin/make"}
		}
		saved.Paths[path] = appleCLTNativeSnapshot(t, ctx, path, destination, files)
	}
	saved.Paths["/opt/homebrew"] = appleCLTNativeSnapshot(t, ctx, "/opt/homebrew", "/opt/homebrew", []string{"bin/brew"})
	if err := os.Mkdir(baseline, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(baseline, "preservation.json"), saved); err != nil {
		t.Fatal(err)
	}
	t.Log("reused real selected developer tools without ownership or native dispatch; recorded moved-root metadata and representative payload hashes, sibling metadata and Homebrew executable")
}

func verifyAppleCLTNativePreservation(t *testing.T, ctx context.Context, baseline string) {
	t.Helper()
	var saved appleCLTNativePreservation
	data, err := readDocument(filepath.Join(baseline, "preservation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Decode(data, &saved); err != nil {
		t.Fatal(err)
	}
	after := appleCLTNativeReceipts(t, ctx)
	for _, id := range saved.Receipts {
		if !slices.Contains(after, id) {
			t.Fatal("Apple removed a historical package receipt", id)
		}
	}
	for original, before := range saved.Paths {
		files := make([]string, 0, len(before.Files))
		for name := range before.Files {
			files = append(files, name)
		}
		actual := appleCLTNativeSnapshot(t, ctx, before.Destination, before.Destination, files)
		if !reflect.DeepEqual(actual, before) {
			t.Fatalf("CLT bootstrap changed preserved %s: before=%+v after=%+v", original, before, actual)
		}
	}
}
