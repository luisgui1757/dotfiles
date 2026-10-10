package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This is separate from the ordinary native formula fixture: it requires an
// disposable Apple Silicon machine with an actually absent Homebrew prefix.
// The separate runner setup preserves its preinstalled prefix elsewhere before
// this fixture starts. The production driver never does that. Homebrew, CLT and the
// upstream paths.d entry remain as retained infrastructure on that runner.
func TestNativeHomebrewBootstrapFreshLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" || os.Getenv("GITHUB_ACTIONS") != "true" ||
		os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_HOMEBREW_BOOTSTRAP") != "1" {
		t.Skip("requires explicitly enabled disposable fresh Apple Silicon GitHub host")
	}
	if _, err := os.Lstat("/opt/homebrew"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("bootstrap fixture requires actual /opt/homebrew absence; prepare the dedicated disposable runner first", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	if existing, err := DiscoverHomebrew(ctx); err != nil || existing != nil {
		t.Fatal("bootstrap fixture requires no existing discovered manager", existing, err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := newNativeSession(filepath.Join(home, "state", "worker"))
	session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(session.Directory) }
	platform := NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}
	d, err := configureHomebrewBootstrap(platform, ConfigFolders{Home: home}, session)
	if err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: "infra.homebrew", Name: "Homebrew", Action: "native", Scope: "machine", Retain: true, ReplanAfter: true}
	before, err := d.Observe(ctx, r, Receipt{})
	if err != nil || before.Present || before.Unknown || before.Pending != "" {
		t.Fatal("native prefix is not genuinely absent", before, err)
	}
	apply := func(action, letter string) Observation {
		t.Helper()
		release, err := session.acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		o, applyErr := d.Apply(ctx, r, Operation{Action: action}, bootstrapReceipt(letter))
		if err := errors.Join(applyErr, release()); err != nil {
			t.Fatal(action, err)
		}
		if !o.Healthy || o.CompletedOperation != strings.Repeat(letter, 64) {
			t.Fatal("unproved native operation", action, o)
		}
		return o
	}
	installed := apply("install", "a")
	location, err := inspectHomebrew(ctx, "/opt/homebrew/bin/brew")
	if err != nil || location.Prefix != "/opt/homebrew" {
		t.Fatal("fresh manager failed process-boundary rediscovery", location, err)
	}
	platform.Homebrew = location
	d, err = configureHomebrewBootstrap(platform, ConfigFolders{Home: home}, session)
	if err != nil {
		t.Fatal(err)
	}
	check, err := d.Observe(ctx, r, bootstrapReceipt("a"))
	if err != nil || !check.Healthy || !sameArtifact(installed, check) {
		t.Fatal("fresh manager failed rediscovered check", check, err)
	}
	homebrewBootstrapRetainedPlan(t, r, check, "update", "keep")
	homebrewBootstrapRetainedPlan(t, r, check, "apply", "retain")
	inventory, err := brewInventory(ctx, d.Query)
	if err != nil || len(inventory) != 0 {
		t.Fatal("infrastructure bootstrap installed native packages", inventory, err)
	}
	t.Log("actual fresh prefix installed through pinned native worker, rediscovered, checked, and planned retained on tool update and removal")
}
