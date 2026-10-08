package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Two native contracts share one deliberately small fixture: an isolated local
// tap, fixed source archive, explicit trust, and cleanup of its unique names.
type nativeBrewFixture struct {
	t                                                 *testing.T
	ctx                                               context.Context
	root, nonce, prefix, tap, tapRoot, cellar, source string
	program, sourceHash                               string
	env, names, casks                                 []string
	before                                            []byte
	beforeCasks                                       []byte
	developer                                         []byte
}

func newNativeBrewFixture(t *testing.T) *nativeBrewFixture {
	t.Helper()
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || runtime.GOOS != "darwin" {
		t.Skip("requires an explicitly opted-in disposable macOS package host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &nativeBrewFixture{t: t, ctx: ctx, root: root, nonce: strings.ToLower(rand.Text()[:10])}
	f.prefix, f.tap = "dotfiles-fixture-"+f.nonce, "dotfiles/contract-"+f.nonce
	f.program, err = exec.LookPath("brew")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.HasPrefix(key, "HOMEBREW_") && key != "LC_ALL" && key != "XDG_CONFIG_HOME" {
			f.env = append(f.env, value)
		}
	}
	f.env = append(f.env, brewProcessControls...)
	f.env = append(f.env, "XDG_CONFIG_HOME="+filepath.Join(root, "brew-config"))
	f.before = f.require("", "list", "--formula", "--versions")
	f.beforeCasks = f.require("", "list", "--cask", "--versions")
	f.developer = f.require("", "developer", "state")
	repository := strings.TrimSpace(string(f.require("", "--repository")))
	f.cellar = strings.TrimSpace(string(f.require("", "--cellar")))
	if !filepath.IsAbs(repository) || !filepath.IsAbs(f.cellar) {
		t.Fatal("Homebrew paths must be absolute")
	}
	f.tapRoot = filepath.Join(repository, "Library", "Taps", "dotfiles", "homebrew-contract-"+f.nonce)
	if err := os.MkdirAll(filepath.Dir(f.tapRoot), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.tapRoot, 0755); err != nil {
		t.Fatal("fixture must own a fresh unique tap", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if len(f.casks) != 0 {
			command := exec.CommandContext(cleanup, f.program, append([]string{"uninstall", "--cask"}, f.casks...)...)
			command.Env = f.env
			if output, err := command.CombinedOutput(); err != nil {
				t.Errorf("native cask cleanup: %v\n%s", err, output)
			}
		}
		if len(f.names) != 0 {
			// Formula-only --force removes all versions of these unique fixture
			// names while retaining Homebrew's outside-dependency protection.
			command := exec.CommandContext(cleanup, f.program, append([]string{"uninstall", "--formula", "--force"}, f.names...)...)
			command.Env = f.env
			if output, err := command.CombinedOutput(); err != nil {
				t.Errorf("native formula cleanup: %v\n%s", err, output)
			}
		}
		if err := os.RemoveAll(f.tapRoot); err != nil {
			t.Error(err)
		}
		command := exec.CommandContext(cleanup, f.program, "list", "--formula", "--versions")
		command.Env = f.env
		after, err := command.Output()
		if err != nil || !bytes.Equal(f.before, after) {
			t.Error("native fixture changed unrelated installed formulae", err)
		}
		command = exec.CommandContext(cleanup, f.program, "list", "--cask", "--versions")
		command.Env = f.env
		afterCasks, err := command.Output()
		if err != nil || !bytes.Equal(f.beforeCasks, afterCasks) {
			t.Error("native fixture changed unrelated installed casks", err)
		}
		command = exec.CommandContext(cleanup, f.program, "developer", "state")
		command.Env = f.env
		developer, err := command.CombinedOutput()
		if err != nil || !bytes.Equal(f.developer, developer) {
			t.Error("native linkage inspection changed Homebrew's developer preference", err)
		}
	})
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "fixture/data.txt", Mode: 0644, Size: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("fixture")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	f.source, f.sourceHash = filepath.Join(root, "fixture.tar.gz"), fmt.Sprintf("%x", sha256.Sum256(archive.Bytes()))
	if err := os.WriteFile(f.source, archive.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *nativeBrewFixture) run(marker string, args ...string) ([]byte, error) {
	command := exec.CommandContext(f.ctx, f.program, args...)
	command.Env = append(slices.Clone(f.env), "HOMEBREW_INSTALL_BADGE="+marker)
	return command.CombinedOutput()
}

func (f *nativeBrewFixture) require(marker string, args ...string) []byte {
	f.t.Helper()
	output, err := f.run(marker, args...)
	if err != nil {
		f.t.Fatalf("brew %v: %v\n%s", args, err, output)
	}
	return output
}

func (f *nativeBrewFixture) query(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || program != "brew" || len(input) != 0 {
		return nil, errors.New("invalid native fixture query")
	}
	command := exec.CommandContext(ctx, f.program, args...)
	command.Env = slices.Clone(f.env)
	if len(args) > 0 && args[0] == "linkage" {
		command.Env = append(command.Env, "HOMEBREW_DEV_CMD_RUN=1")
	}
	return command.Output()
}

func (f *nativeBrewFixture) formula(name, version, dependency, body string) {
	f.t.Helper()
	var class strings.Builder
	for _, part := range strings.Split(name, "-") {
		class.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	definition := fmt.Sprintf("class %s < Formula\n  desc %q\n  homepage %q\n  url %q\n  version %q\n  sha256 %q\n%s  def install\n%s\n  end\nend\n", class.String(), "Disposable dotfiles contract fixture", "https://example.invalid", "file://"+f.source, version, f.sourceHash, dependency, body)
	if err := os.WriteFile(filepath.Join(f.tapRoot, name+".rb"), []byte(definition), 0644); err != nil {
		f.t.Fatal(err)
	}
	if !slices.Contains(f.names, name) {
		f.names = append(f.names, name)
	}
}
