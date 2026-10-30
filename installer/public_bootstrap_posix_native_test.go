//go:build darwin || linux

package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Unlike installedMachineFixture, every invocation enters the public launcher:
// a fresh cache must download and verify Go, extract it and build cmd/dotfiles.
// No native package is selected; all writes stay in this home.
func TestNativePOSIXPublicBootstrapSetupLifecycle(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_TEST_PUBLIC_POSIX_BOOTSTRAP") != "1" {
		t.Skip("verified public bootstrap requires an explicitly opted-in disposable GitHub-hosted POSIX runner")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "private home ü with spaces")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"HOME": home, "XDG_CACHE_HOME": filepath.Join(home, ".cache"),
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"), "XDG_DATA_HOME": filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME": filepath.Join(home, ".local", "state"), "CODEX_HOME": filepath.Join(home, ".codex"),
		"CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude"), "ZDOTDIR": home,
	} {
		t.Setenv(key, value)
	}
	cache := filepath.Join(home, ".cache", "dotfiles", "bootstrap")
	t.Cleanup(func() { cleanPublicBootstrapModules(t, cache) })
	if _, err := os.Lstat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("public bootstrap requires an absent private cache", err)
	}
	hashFile := func(path string) string {
		t.Helper()
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		if err := errors.Join(copyErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(hash.Sum(nil))
	}
	var executableDigest string
	invoke := func(entrypoint string, request Request) Result {
		t.Helper()
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		// The ordinary public-wrapper subprocess regression observes the final
		// exec's umask directly, on cold and warm paths. This real engine keeps
		// the same caller setting while using its explicit private file modes.
		cmd := exec.CommandContext(ctx, "bash", "-c", `umask 0027; exec bash "$1" machine`, "public-bootstrap", filepath.Join(repository, entrypoint+".sh"))
		cmd.Dir, cmd.Stdin, cmd.WaitDelay = home, bytes.NewReader(data), 2*time.Second
		output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{limit: 64 << 10}
		cmd.Stdout, cmd.Stderr = &output, &diagnostic
		if err := cmd.Run(); err != nil {
			t.Fatalf("public %s %s: %v (context: %v)%s", entrypoint, request.Mode, err, ctx.Err(), nativeDiagnostic(append(output.data.Bytes(), diagnostic.data.Bytes()...)))
		}
		var result Result
		if err := Decode(output.data.Bytes(), &result); err != nil {
			t.Fatalf("public machine response: %v%s", err, nativeDiagnostic(output.data.Bytes()))
		}
		if executableDigest != "" && result.Plan.Source != executableDigest {
			t.Fatal("public invocation did not bind its plan to the retained executable SHA-256", result.Plan.Source, executableDigest)
		}
		t.Logf("real public %s %s: status=%s executable_sha256=%s", entrypoint, request.Mode, result.Status, result.Plan.Source)
		return result
	}
	initial := invoke("setup", Request{Schema: 1, Mode: "check"})
	if initial.Status != "ready" {
		t.Fatal("public setup did not accept the isolated empty state", initial)
	}
	binaries, err := filepath.Glob(filepath.Join(cache, "dotfiles-*.sha256"))
	if err != nil || len(binaries) != 1 {
		t.Fatal("fresh bootstrap did not publish exactly one authenticated engine", binaries, err)
	}
	binary := strings.TrimSuffix(binaries[0], ".sha256")
	executableDigest = hashFile(binary)
	sidecar, err := os.ReadFile(binaries[0])
	if err != nil || strings.TrimSpace(string(sidecar)) != executableDigest || initial.Plan.Source != executableDigest {
		t.Fatal("bootstrap artifact, integrity sidecar and public plan identity differ", err)
	}
	pins, err := os.ReadFile(filepath.Join(repository, "installer", "bootstrap-toolchain.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(pins))
	reader.Comma = '\t'
	reader.Comment = '#'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, pin := range records[1:] {
		if len(pin) != 5 || pin[1] != runtime.GOOS+"-"+runtime.GOARCH {
			continue
		}
		info, err := buildinfo.ReadFile(binary)
		if err != nil || info.GoVersion != pin[0] || hashFile(filepath.Join(cache, pin[2])) != pin[3] {
			t.Fatal("native build or cached toolchain archive differs from the pinned Go release", err)
		}
		matched = true
		t.Logf("real pinned Go download/extraction/build: %s %s sha256=%s", pin[0], pin[1], pin[3])
	}
	if !matched {
		t.Fatal("native target has no toolchain evidence")
	}

	assertBytes := func(path, wanted string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != wanted {
			t.Fatalf("personal or preserved bytes changed at %s: %v", path, err)
		}
	}
	approve := func(entrypoint string, request Request) Result {
		t.Helper()
		preview := invoke(entrypoint, request)
		if preview.Status != "preview" || preview.Plan.ID == "" {
			t.Fatal("public launcher did not return an approval plan", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		result := invoke(entrypoint, request)
		if result.Status != "ready" {
			t.Fatal("approved public operation did not complete", result)
		}
		return result
	}
	// ripgrep is a private archive with the shared shell integration. It
	// needs no native package, and its profile block shares the personal file.
	profile := filepath.Join(home, ".zshrc")
	personal := "# personal profile; preserve\nexport PERSONAL=value\n"
	writeConfigFixture(t, profile, personal)
	approve("setup", Request{Schema: 1, Mode: "apply", Selected: []string{"ripgrep"}})
	installed, err := os.ReadFile(profile)
	if err != nil || !bytes.HasPrefix(installed, []byte(personal)) || !bytes.Contains(installed, []byte("# >>> dotfiles:")) {
		t.Fatal("public setup did not add its block after the personal profile", string(installed), err)
	}
	approve("setup", Request{Schema: 1, Mode: "update"})
	if result := invoke("setup", Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("public check did not verify completed setup", result)
	}
	approve("setup", Request{Schema: 1, Mode: "apply", Selected: []string{}})
	assertBytes(profile, personal)
	for _, pattern := range []string{"lock", ".prepare.*"} {
		paths, err := filepath.Glob(filepath.Join(cache, pattern))
		if err != nil || len(paths) != 0 {
			t.Fatal("bootstrap retained its lock or extracted compiler staging", paths, err)
		}
	}
	info, err := os.Stat(cache)
	if err != nil || info.Mode().Perm() != 0700 || hashFile(binary) != executableDigest {
		t.Fatal("warm public launches changed cache privacy or executable identity", err)
	}
	t.Log("public setup cold and warm lifecycle passed: real pinned Go; exact executable SHA-256 approval; setup apply/update/check/removal; personal restoration; requested caller umask 0027 and private cache")
}

// Go module contents are read-only. Let the executing, pinned host Go remove
// only this fixture's module cache before t.TempDir attempts its normal cleanup.
func cleanPublicBootstrapModules(t *testing.T, cache string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	env := append(os.Environ(), "GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local",
		"GOPATH="+filepath.Join(cache, "gopath"), "GOMODCACHE="+filepath.Join(cache, "gopath", "pkg", "mod"),
		"GOCACHE="+filepath.Join(cache, "go-build"), "HOME="+filepath.Join(cache, "compiler-home"),
		"XDG_CONFIG_HOME="+filepath.Join(cache, "compiler-config"))
	// Disable collection in this private config before cleanup: Go otherwise
	// starts a telemetry child that can recreate directories during TempDir removal.
	for _, args := range [][]string{{"telemetry", "off"}, {"clean", "-modcache"}} {
		cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("private bootstrap cleanup %v: %v%s", args, err, nativeDiagnostic(output))
			return
		}
	}
}

func TestPublicBootstrapModuleCleanupPreservesInheritedCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "bootstrap cache ü")
	personal := filepath.Join(t.TempDir(), "personal modules")
	writeConfigFixture(t, filepath.Join(personal, "preserve"), "personal module bytes")
	t.Setenv("GOMODCACHE", personal)
	module := filepath.Join(cache, "gopath", "pkg", "mod", "example.invalid", "fixture@v1.0.0")
	writeConfigFixture(t, filepath.Join(module, "fixture.go"), "package fixture\n")
	if err := os.Chmod(module, 0555); err != nil {
		t.Fatal(err)
	}
	// Register recovery before the assertion so even a regression does not leave
	// this test's read-only fixture behind.
	t.Cleanup(func() { cleanPublicBootstrapModules(t, cache) })
	if os.Geteuid() != 0 { // Root can remove read-only directories by design.
		if err := os.RemoveAll(module); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("read-only module did not reproduce native TempDir cleanup failure: %v", err)
		}
	}
	cleanPublicBootstrapModules(t, cache)
	telemetry := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "telemetry")
	telemetry.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local",
		"HOME="+filepath.Join(cache, "compiler-home"), "XDG_CONFIG_HOME="+filepath.Join(cache, "compiler-config"))
	if output, err := telemetry.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "off" {
		t.Fatalf("private cleanup can still start telemetry writers: %v%s", err, nativeDiagnostic(output))
	}
	if _, err := os.Lstat(filepath.Join(cache, "gopath", "pkg", "mod")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("private module cache remains after Go cleanup", err)
	}
	data, err := os.ReadFile(filepath.Join(personal, "preserve"))
	if err != nil || string(data) != "personal module bytes" || os.Getenv("GOMODCACHE") != personal {
		t.Fatal("cleanup changed the inherited module cache or caller environment", err)
	}
}
