package installer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativeBrewAppSignaturePreservesFinderMetadataAndRejectsTampering(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || runtime.GOOS != "darwin" {
		t.Skip("requires an explicitly opted-in disposable macOS package host")
	}
	for _, kind := range []string{"finder-metadata", "executable", "resource", "outside-symlink", "extra-root-file", "unsigned-nested-code"} {
		t.Run(kind, func(t *testing.T) {
			root, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			app := filepath.Join(root, "Fixture.app")
			writeConfigFixture(t, filepath.Join(app, "Contents", "Info.plist"), `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>org.dotfiles.signature-fixture</string><key>CFBundleExecutable</key><string>fixture</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`)
			binary, err := os.ReadFile("/bin/echo")
			if err != nil {
				t.Fatal(err)
			}
			program := filepath.Join(app, "Contents", "MacOS", "fixture")
			if err := os.MkdirAll(filepath.Dir(program), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(program, binary, 0755); err != nil {
				t.Fatal(err)
			}
			resource := filepath.Join(app, "Contents", "Resources", "personal-style.txt")
			writeConfigFixture(t, resource, "sealed resource")
			if kind == "outside-symlink" {
				outside := filepath.Join(root, "outside")
				writeConfigFixture(t, outside, "outside consumer")
				if err := os.Symlink(outside, filepath.Join(app, "Contents", "Resources", "outside")); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", app).CombinedOutput(); err != nil {
				t.Fatal(string(out), err)
			}
			switch kind {
			case "finder-metadata":
				// The actual notarized Zen DMG carries these Finder coordinates.
				if out, err := exec.Command("/usr/bin/xattr", "-wx", "com.apple.FinderInfo", "000000000000000000000000ffffffff00000000000000000000000000000000", program).CombinedOutput(); err != nil {
					t.Fatal(string(out), err)
				}
			case "executable":
				binary[len(binary)/2] ^= 1
				if err := os.WriteFile(program, binary, 0755); err != nil {
					t.Fatal(err)
				}
			case "resource":
				writeConfigFixture(t, resource, "unsigned replacement")
			case "extra-root-file":
				writeConfigFixture(t, filepath.Join(app, "extra"), "unsealed extra file")
			case "unsigned-nested-code":
				nested := filepath.Join(app, "Contents", "Frameworks", "unsigned.dylib")
				if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(nested, binary, 0755); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("/usr/bin/codesign", "--remove-signature", nested).CombinedOutput(); err != nil {
					t.Fatal(string(out), err)
				}
			}
			d := BrewDriver{Query: (HomebrewLocation{}).query}
			s := brewCaskSpec{Requirement: `identifier "org.dotfiles.signature-fixture"`}
			// The documented option must differ only for harmless sideband data:
			// every tamper case remains rejected by both strict policies.
			if out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", "-R", "="+s.Requirement, app).CombinedOutput(); err == nil {
				t.Fatal("all-strict unexpectedly accepted the adversarial fixture", string(out))
			}
			present, healthy, err := d.caskIdentity(context.Background(), s, app)
			if err != nil || !present || healthy != (kind == "finder-metadata") {
				t.Fatal("native signature policy", present, healthy, err)
			}
		})
	}
}

// Uses a disposable, locally signed app and real Homebrew. No application is
// launched, and no Gatekeeper/quarantine exception is used for the fixture.
func TestNativeBrewCaskBadgeSignatureAndNoBinaries(t *testing.T) {
	f := newNativeBrewFixture(t)
	name := f.prefix + "-application"
	bundleName := name + ".app"
	build := filepath.Join(f.root, "source")
	app := filepath.Join(build, bundleName)
	writeConfigFixture(t, filepath.Join(app, "Contents", "Info.plist"), `<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>org.dotfiles.acceptance-fixture</string><key>CFBundleExecutable</key><string>fixture</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`)
	binary, err := os.ReadFile("/bin/echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "fixture"), binary, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(f.ctx, "/usr/bin/codesign", "--force", "--sign", "-", app).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	archive := filepath.Join(f.root, "application.tar.gz")
	if out, err := exec.CommandContext(f.ctx, "/usr/bin/tar", "-czf", archive, "-C", build, bundleName).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	payload, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	archiveURL := url.URL{Scheme: "file", Path: archive}
	definition := fmt.Sprintf(`cask %q do
	version "1.0"
	sha256 %q
	url %q
	name "Dotfiles disposable app"
	desc "Real native cask acceptance"
	homepage "https://example.invalid/"
	auto_updates true
	app %q
	command_wrapper %q, executable: "#{appdir}/%s/Contents/MacOS/fixture"
end
`, name, fmt.Sprintf("%x", sha256.Sum256(payload)), archiveURL.String(), bundleName, name, bundleName)
	writeConfigFixture(t, filepath.Join(f.tapRoot, "Casks", name+".rb"), definition)
	f.require("", "trust", "--tap", f.tap)
	appdir := filepath.Join(f.root, "Applications")
	if err := os.Mkdir(appdir, 0755); err != nil {
		t.Fatal(err)
	}
	poison := filepath.Join(f.root, "unapproved-appdir")
	prefix := strings.TrimSpace(string(f.require("", "--prefix")))
	d := &BrewDriver{Environment: slices.Clone(brewProcessControls), Casks: map[string]brewCaskSpec{"fixture": {}}}
	operation, err := digest(f.nonce + "-cask")
	if err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(f.root, "unwritable-appdir")
	// Otherwise a lack of credentials could explain the refusal independently
	// of the command's explicit HOMEBREW_NO_SUDO control.
	if output, err := exec.CommandContext(f.ctx, "/usr/bin/sudo", "-n", "/usr/bin/true").CombinedOutput(); err != nil {
		t.Fatal("native no-sudo fixture requires usable administrator credentials", string(output), err)
	}
	if err := os.Mkdir(locked, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0755); err != nil {
			t.Error(err)
		}
	})
	blockedCtx, cancelBlocked := context.WithTimeout(f.ctx, 30*time.Second)
	blocked := exec.CommandContext(blockedCtx, f.program, "install", "--cask", "--appdir="+locked, "--no-binaries", "--require-sha", f.tap+"/"+name)
	blocked.Env = append(slices.Clone(f.env), d.commandEnvironment("fixture", operation, 4)...)
	blockedOutput, blockedErr := blocked.CombinedOutput()
	deadline := blockedCtx.Err()
	cancelBlocked()
	if blockedErr == nil || deadline != nil || !strings.Contains(string(blockedOutput), "Permission denied") {
		t.Fatal("native cask did not promptly respect the non-writable appdir", string(blockedOutput), blockedErr, deadline)
	}
	if _, err := os.Lstat(filepath.Join(locked, bundleName)); !os.IsNotExist(err) {
		t.Fatal("native cask elevated into the protected appdir", err)
	}
	run := func(verb string) []byte {
		t.Helper()
		arguments := []string{verb, "--cask", "--appdir=" + appdir, "--no-binaries", "--require-sha", f.tap + "/" + name}
		command := exec.CommandContext(f.ctx, f.program, arguments...)
		command.Env = append(append(slices.Clone(f.env), "HOMEBREW_CASK_OPTS=--appdir="+poison, "HOMEBREW_NO_EMOJI=1"), d.commandEnvironment("fixture", operation, 4)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(string(output), err)
		}
		return output
	}
	f.casks = append(f.casks, f.tap+"/"+name)
	for _, verb := range []string{"install", "reinstall"} {
		output := run(verb)
		evidence, err := brewInstallEvidence(output, f.cellar, operation)
		if err != nil || len(evidence) != 1 || evidence[0].Name != "cask/"+name {
			t.Fatal("native cask attribution failed", string(output), evidence, err)
		}
		if _, err := os.Lstat(filepath.Join(prefix, "bin", name)); !os.IsNotExist(err) {
			t.Fatal("disabled wrapper was installed", err)
		}
		if _, err := os.Lstat(poison); !os.IsNotExist(err) {
			t.Fatal("inherited cask options escaped", err)
		}
	}
	location := HomebrewLocation{Program: f.program, Prefix: prefix, Cellar: f.cellar}
	d.Query = location.query
	s := brewCaskSpec{Requirement: `identifier "org.dotfiles.acceptance-fixture"`}
	present, healthy, err := d.caskIdentity(f.ctx, s, filepath.Join(appdir, bundleName))
	if err != nil || !present || !healthy {
		t.Fatal("real signed fixture not verified", present, healthy, err)
	}
	s.Requirement = `anchor apple generic and certificate leaf[subject.OU] = "9V5K9TP787"`
	present, healthy, err = d.caskIdentity(f.ctx, s, filepath.Join(appdir, bundleName))
	if err != nil || !present || healthy {
		t.Fatal("foreign developer accepted", present, healthy, err)
	}
	metadata, err := filepath.Glob(filepath.Join(prefix, "Caskroom", name, ".metadata", "*", "*", "Casks", "*.json"))
	if err != nil || len(metadata) != 1 {
		t.Fatal(metadata, err)
	}
	data, err := os.ReadFile(metadata[0])
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	details, err := json.Marshal(map[string]any{"token": record["token"], "full_token": record["full_token"], "tap": record["tap"], "artifacts": record["artifacts"]})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("installed compact cask metadata:", string(details))
	receiptData, err := os.ReadFile(filepath.Join(prefix, "Caskroom", name, ".metadata", "INSTALL_RECEIPT.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Artifacts []map[string]json.RawMessage `json:"uninstall_artifacts"`
	}
	if err := json.Unmarshal(receiptData, &receipt); err != nil || len(receipt.Artifacts) != 2 {
		t.Fatal("native tab omitted app/wrapper removal recipe", string(receiptData), err)
	}
	t.Log("installed uninstall artifacts:", string(receiptData))
	// Current upstream recipes do not replace the saved uninstall contract.
	outside := filepath.Join(f.root, "outside-witness")
	writeConfigFixture(t, outside, "preserve independently of changed recipe")
	changedRecipe := strings.TrimSuffix(definition, "end\n") + fmt.Sprintf("\tuninstall delete: %q\nend\n", outside)
	writeConfigFixture(t, filepath.Join(f.tapRoot, "Casks", name+".rb"), changedRecipe)
	f.require("", "uninstall", "--cask", f.tap+"/"+name)
	f.casks = nil
	if data, err := os.ReadFile(outside); err != nil || string(data) != "preserve independently of changed recipe" {
		t.Fatal("uninstall used the changed source instead of its recorded artifacts", string(data), err)
	}
	if _, err := os.Lstat(filepath.Join(appdir, bundleName)); !os.IsNotExist(err) {
		t.Fatal("native app removal did not complete", err)
	}
}
