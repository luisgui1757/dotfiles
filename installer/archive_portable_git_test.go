package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func windowsArchiveRecipePins(t *testing.T) map[string]ArchivePin {
	t.Helper()
	data, err := os.ReadFile("testdata/windows-archive-recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	var pins map[string]ArchivePin
	if err := Decode(data, &pins); err != nil {
		t.Fatal(err)
	}
	return pins
}

func TestPortableGitRequiresCompleteOfficialRecipe(t *testing.T) {
	for _, pin := range windowsArchiveRecipePins(t) {
		if err := pin.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, change := range []func(*ArchivePin){
		func(p *ArchivePin) { p.URL = strings.Replace(p.URL, "github.com", "example.org", 1) },
		func(p *ArchivePin) { p.URL = strings.Replace(p.URL, "PortableGit", "MinGit", 1) },
		func(p *ArchivePin) { p.URL += "?channel=latest" },
		func(p *ArchivePin) { p.Format = "zip" },
		func(p *ArchivePin) { p.File = "other.exe" },
		func(p *ArchivePin) { delete(p.Commands, "bash") },
		func(p *ArchivePin) {
			p.RequiredFiles = slices.DeleteFunc(p.RequiredFiles, func(s string) bool { return s == "usr/bin/msys-2.0.dll" })
		},
		func(p *ArchivePin) { p.ExcludedFiles = []string{"README.portable"} },
		func(p *ArchivePin) { p.PythonStdlib = "lib"; p.Commands["python"] = "python.exe" },
		func(p *ArchivePin) { p.RustComponents = []ArchivePin{{}} },
	} {
		pin := windowsArchiveRecipePins(t)["tool.git"]
		change(&pin)
		if err := pin.Validate(); err == nil {
			t.Fatal("unsafe or reduced PortableGit recipe accepted", pin)
		}
	}
}

func TestPortableGitOptionalFieldPreservesLegacyPinIdentity(t *testing.T) {
	pin := windowsArchiveRecipePins(t)["tool.make"]
	data, err := json.Marshal(pin)
	if err != nil || bytes.Contains(data, []byte("portable_git")) {
		t.Fatal("ordinary archive persisted shape changed", err)
	}
	var restored ArchivePin
	if err := Decode(data, &restored); err != nil || archivePinID(restored) != archivePinID(pin) {
		t.Fatal("ordinary archive identity changed", err)
	}
}

type portableGitTransport func(*http.Request) (*http.Response, error)

func (f portableGitTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func portableGitFixtureDriver(t *testing.T) (Controller, *ArchiveDriver, *[]nativeCommand) {
	t.Helper()
	c, d, _ := archiveController(t)
	pin := windowsArchiveRecipePins(t)["tool.git"]
	data := []byte("verified fixture self-extractor")
	hash := sha256.Sum256(data)
	pin.SHA256 = hex.EncodeToString(hash[:])
	d.Pins["tool.shared"] = pin
	d.Client = &http.Client{Transport: portableGitTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != pin.URL {
			t.Fatal("unexpected download URL")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header), Request: request}, nil
	})}
	commands := []nativeCommand{}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		if err := command.validate(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
		if filepath.Base(command.Program) == pin.File {
			payload := filepath.Dir(command.Program)
			output := payload
			if runtime.GOOS == "windows" {
				output = windowsExtendedPath(payload)
			}
			if !slices.Equal(command.Arguments, []string{"-y", "-o" + output}) {
				t.Fatal("unreviewed SFX arguments", command.Arguments)
			}
			for _, file := range pin.RequiredFiles {
				path := filepath.Join(payload, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("runtime fixture"), 0700); err != nil {
					t.Fatal(err)
				}
			}
		}
		return []byte("portable-git-ready\n"), nil
	}
	return c, d, &commands
}

func TestPortableGitUsesArchivePublicationAndRemoval(t *testing.T) {
	c, d, commands := portableGitFixtureDriver(t)
	d.Directory = filepath.Join(c.Home, "private Git ü path")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	payload, err := d.PayloadPath("tool.shared")
	if err != nil || len(*commands) != 2 {
		t.Fatal("verified extraction and runtime probe were not completed", err, len(*commands))
	}
	for _, command := range *commands {
		if !slices.Contains(command.Environment, "HOME="+filepath.Join(payload, ".prepare-home")) || !slices.Contains(command.Environment, "GIT_CONFIG_GLOBAL=/dev/null") || !slices.Contains(command.Environment, "MSYS=winsymlinks:sys") {
			t.Fatal("preparation can use personal configuration or unsupported link format")
		}
	}
	if _, err := os.Lstat(filepath.Join(payload, ".prepare-post-install.bat")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("post-install execution copy remained in published payload", err)
	}
	if _, err := os.Lstat(filepath.Join(payload, "portable-git.exe")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("self-extractor remained in published payload", err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" || len(*commands) != 2 {
		t.Fatal("check changed payload or lost ownership", check.Status, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned private distribution was not removed", err)
	}
}

func TestPortableGitFailureCannotPublishReadyPayload(t *testing.T) {
	for _, problem := range []string{"checksum", "extract", "post-install", "runtime", "missing-dll"} {
		t.Run(problem, func(t *testing.T) {
			_, d, commands := portableGitFixtureDriver(t)
			pin := d.Pins["tool.shared"]
			if problem == "checksum" {
				pin.SHA256 = strings.Repeat("0", 64)
			}
			original := d.Run
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				output, err := original(ctx, command)
				if filepath.Base(command.Program) == pin.File {
					switch problem {
					case "extract":
						return nil, errors.New("fixture extraction interrupted")
					case "post-install":
						err = os.WriteFile(filepath.Join(filepath.Dir(command.Program), "post-install.bat"), []byte("incomplete"), 0600)
					case "missing-dll":
						err = os.Remove(filepath.Join(filepath.Dir(command.Program), "usr", "bin", "msys-2.0.dll"))
					}
				} else if problem == "runtime" {
					return nil, errors.New("fixture runtime failed")
				}
				return output, err
			}
			intent := archiveIntent{Operation: strings.Repeat("a", 64), Pin: pin}
			if err := d.preparePayload(context.Background(), intent, filepath.Join(t.TempDir(), "stage ü")); err == nil {
				t.Fatal("failed preparation was accepted")
			}
			if problem == "checksum" && len(*commands) != 0 {
				t.Fatal("executed a download before checksum validation")
			}
		})
	}
}

func TestPortableGitFreshRecoveryAttemptHasNewNativeIdentity(t *testing.T) {
	_, d, commands := portableGitFixtureDriver(t)
	intent := archiveIntent{Operation: strings.Repeat("a", 64), Pin: d.Pins["tool.shared"]}
	for i := 0; i < 2; i++ {
		if err := d.preparePayload(context.Background(), intent, filepath.Join(t.TempDir(), "stage")); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for _, command := range *commands {
		if seen[command.Operation] {
			t.Fatal("retry could replay a previous stage's native result")
		}
		seen[command.Operation] = true
	}
}

func TestPortableGitCompletesUpstreamPostInstallAfterExtraction(t *testing.T) {
	for _, outcome := range []string{"complete", "failed", "failed-after-cleanup", "unfinished", "changed-copy", "occupied-copy"} {
		t.Run(outcome, func(t *testing.T) {
			_, d, commands := portableGitFixtureDriver(t)
			original := d.Run
			postInstallCalls := 0
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				output, err := original(ctx, command)
				payload := filepath.Dir(command.Program)
				switch filepath.Base(command.Program) {
				case "portable-git.exe":
					if err := os.WriteFile(filepath.Join(payload, "post-install.bat"), []byte("upstream fixture"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(filepath.Join(payload, "etc", "post-install"), 0700); err != nil {
						t.Fatal(err)
					}
					if outcome == "occupied-copy" {
						if err := os.WriteFile(filepath.Join(payload, ".prepare-post-install.bat"), []byte("preserve occupied path"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				case "git-bash.exe":
					postInstallCalls++
					if !slices.Equal(command.Arguments, []string{"--no-needs-console", "--hide", "--cd=" + payload, "--command=.prepare-post-install.bat"}) {
						t.Fatal("post-install must run an unchanged sibling copy in its ordinary payload directory", command.Arguments)
					}
					if outcome == "failed" {
						return nil, errors.New("upstream post-install failed")
					}
					originalScript, err := os.ReadFile(filepath.Join(payload, "post-install.bat"))
					if err != nil {
						t.Fatal(err)
					}
					copyScript, err := os.ReadFile(filepath.Join(payload, ".prepare-post-install.bat"))
					if err != nil || !bytes.Equal(copyScript, originalScript) {
						t.Fatal("CMD input must remain an exact copy of the verified upstream script", err)
					}
					if outcome == "complete" || outcome == "failed-after-cleanup" || outcome == "changed-copy" {
						for _, name := range []string{"post-install.bat", "etc/post-install"} {
							if err := os.Remove(filepath.Join(payload, filepath.FromSlash(name))); err != nil {
								t.Fatal(err)
							}
						}
					}
					if outcome == "changed-copy" {
						if err := os.WriteFile(filepath.Join(payload, ".prepare-post-install.bat"), []byte("changed input"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if outcome == "failed-after-cleanup" {
						return nil, errors.New("post-install failed after deleting its artifacts")
					}
				}
				return output, err
			}
			intent := archiveIntent{Operation: strings.Repeat("a", 64), Pin: d.Pins["tool.shared"]}
			payload := filepath.Join(t.TempDir(), "stage ü")
			err := d.preparePayload(context.Background(), intent, payload)
			wantCalls := 1
			if outcome == "occupied-copy" {
				wantCalls = 0
			}
			if postInstallCalls != wantCalls || (err == nil) != (outcome == "complete") {
				t.Fatalf("post-install calls=%d error=%v", postInstallCalls, err)
			}
			wantCommands := 2
			if outcome == "complete" || outcome == "changed-copy" {
				wantCommands = 3
			}
			if outcome == "occupied-copy" {
				wantCommands = 1
			}
			if len(*commands) != wantCommands {
				t.Fatal("runtime probe ran before preparation completed", len(*commands))
			}
			copy, copyErr := os.ReadFile(filepath.Join(payload, ".prepare-post-install.bat"))
			if outcome == "complete" {
				if !errors.Is(copyErr, os.ErrNotExist) {
					t.Fatal("successful copy cleanup failed", copyErr)
				}
			} else if copyErr != nil {
				t.Fatal("failed preparation lost interpreter evidence", copyErr)
			} else if outcome == "occupied-copy" && string(copy) != "preserve occupied path" {
				t.Fatal("occupied input was overwritten")
			} else if outcome == "changed-copy" && string(copy) != "changed input" {
				t.Fatal("changed input was removed")
			}
		})
	}
}

// Exercise Git's config boundary with a real executable. Only its explicitly
// documented null spelling works consistently with Git for Windows' path checks.
func TestPortableGitNullConfigIgnoresPersonalSettings(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("invalid personal configuration\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), git, "config", "--global", "--list")
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(variable), "GIT_") {
			command.Env = append(command.Env, variable)
		}
	}
	command.Env = append(command.Env, portableGitPreparationEnvironment(home, home, home)...)
	if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
		t.Fatal("Git's private null configuration must ignore personal settings", err, string(output))
	}
}
