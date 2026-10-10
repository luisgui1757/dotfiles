package installer

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type offlineDesktopTransport struct{}

func (offlineDesktopTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("desktop checks and cleanup must work offline")
}

func TestNativeDesktopArchiveLifecycles(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_DESKTOP_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_DESKTOP_ARCHIVES=1 for private pinned desktop payloads")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ ID, Command, Bundle string }{
		{"tool.vscode", "code", "Visual Studio Code.app"},
		{"font.hack", "", ""},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases, struct{ ID, Command, Bundle string }{"tool.ghostty", "ghostty", "Ghostty.app"})
	}
	if runtime.GOOS == "darwin" {
		cases = append(cases, struct{ ID, Command, Bundle string }{"tool.aerospace", "aerospace", "AeroSpace.app"})
	}
	for _, test := range cases {
		t.Run(test.ID, func(t *testing.T) {
			pin, ok := pins[test.ID]
			if !ok {
				t.Fatal("native desktop pin is missing")
			}
			c, d, _ := archiveController(t)
			d.Pins["tool.shared"], d.Client = pin, nil
			if runtime.GOOS == "linux" {
				d.DpkgDeb, err = exec.LookPath("dpkg-deb")
				if err != nil || !filepath.IsAbs(d.DpkgDeb) {
					t.Fatal("native desktop fixture requires the observed absolute dpkg-deb", err)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			payload, err := d.PayloadPath("tool.shared")
			if err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(c.Home, "private-profile")
			if err := os.Mkdir(home, 0700); err != nil {
				t.Fatal(err)
			}
			environment := []string{"HOME=" + home, "USERPROFILE=" + home, "BASH_ENV=", "ENV=", "NODE_OPTIONS=", "VSCODE_PORTABLE=", "GHOSTTY_RESOURCES_DIR="}
			environment = append(environment, "LD_LIBRARY_PATH=")
			if runtime.GOOS != "windows" {
				environment = append(environment, "PATH=/usr/bin:/bin:/usr/sbin:/sbin")
			}
			for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP", "TMPDIR"} {
				path := filepath.Join(home, name)
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				environment = append(environment, name+"="+path)
			}
			run := func(program string, args ...string) string {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				command := exec.CommandContext(ctx, program, args...)
				command.Dir = home
				command.Env = append(os.Environ(), environment...)
				var output boundedCommandOutput
				command.Stdout, command.Stderr = &output, &output
				if err := command.Run(); err != nil {
					t.Fatalf("private desktop command %s: %s: %v", filepath.Base(program), output.data.Bytes(), err)
				}
				return output.data.String()
			}
			if runtime.GOOS == "darwin" && test.Bundle != "" {
				run("/usr/bin/codesign", "--verify", "--deep", "--strict", filepath.Join(payload, test.Bundle))
			}
			if test.Command != "" {
				program, err := d.CommandPath("tool.shared", test.Command)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"--version"}
				if test.Command == "code" {
					args = append(args, "--user-data-dir", filepath.Join(home, "code-data"), "--extensions-dir", filepath.Join(home, "code-extensions"))
					if runtime.GOOS == "windows" {
						// Match the upstream code.cmd entrypoint without a shell.
						program = filepath.Join(payload, "Code.exe")
						args = append([]string{desktopCodeCLI(t, payload, pin)}, args...)
						environment = append(environment, "ELECTRON_RUN_AS_NODE=1")
					}
				}
				if output := run(program, args...); !strings.Contains(output, pin.Version) {
					t.Fatal("unexpected pinned desktop version", output)
				}
				if test.Command == "code" {
					theme, ok := pins["vscode.rose-pine"]
					if !ok {
						t.Fatal("native Rose Pine pin is missing")
					}
					themeController, themeDriver, _ := archiveController(t)
					themeDriver.Pins["tool.shared"], themeDriver.Client = theme, nil
					dispatchApproved(t, themeController, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
					themePayload, err := themeDriver.PayloadPath("tool.shared")
					if err != nil {
						t.Fatal(err)
					}
					extensions := filepath.Join(home, "code-extensions")
					if err := os.MkdirAll(extensions, 0700); err != nil {
						t.Fatal(err)
					}
					link := filepath.Join(extensions, "dotfiles.rose-pine")
					if err := createDirectoryLink(filepath.Join(themePayload, "extension"), link); err != nil {
						t.Fatal(err)
					}
					list := []string{"--list-extensions", "--show-versions", "--user-data-dir", filepath.Join(home, "code-data"), "--extensions-dir", extensions}
					if runtime.GOOS == "windows" {
						list = append([]string{desktopCodeCLI(t, payload, pin)}, list...)
					}
					if output := run(program, list...); !strings.Contains(output, "mvllow.rose-pine@"+theme.Version) {
						t.Fatal("Code did not discover the stable theme directory link", output)
					}
					if err := os.Remove(link); err != nil {
						t.Fatal(err)
					}
					themeDriver.Client = &http.Client{Transport: offlineDesktopTransport{}}
					dispatchApproved(t, themeController, Request{Schema: 1, Mode: "apply", Selected: []string{}})
				}
				if runtime.GOOS == "linux" {
					binary := program
					if test.Command == "code" {
						binary = filepath.Join(payload, "code")
					}
					lddArgs := []string{"/usr/bin/ldd", binary}
					if pin.GhosttyLibraries {
						lddArgs = []string{"LD_LIBRARY_PATH=" + filepath.Join(payload, "usr", "lib"), "/usr/bin/ldd", filepath.Join(payload, "usr", "bin", "ghostty-bin")}
					}
					if output := run("/usr/bin/env", lddArgs...); strings.Contains(output, "not found") {
						t.Fatal("private desktop executable has unresolved shared libraries", output)
					}
					switch test.Command {
					case "ghostty":
						if output := run(program, "+list-themes"); !strings.Contains(strings.ToLower(output), "rose pine") {
							t.Fatal("private Ghostty cannot find its bundled themes", output)
						}
					}
					if os.Getenv("DOTFILES_TEST_DESKTOP_GUI") == "1" {
						switch test.Command {
						case "ghostty":
							desktopNativeWindow(t, home, environment, program, "--config-default-files=false", "--gtk-single-instance=false", "--title=dotfiles-desktop-proof", "-e", "/bin/sh", "-c", "exec sleep 60")
						}
					}
				}
			} else {
				for _, name := range pin.RequiredFiles {
					if !strings.HasSuffix(name, ".ttf") {
						continue
					}
					data, err := os.ReadFile(filepath.Join(payload, name))
					if err != nil || len(data) < 12 || string(data[:4]) != "\x00\x01\x00\x00" {
						t.Fatal("missing TrueType font payload", name, err)
					}
				}
			}
			d.Client = &http.Client{Transport: offlineDesktopTransport{}}
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("native use changed the pinned desktop payload", check, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("offline desktop removal left its owned payload", err)
			}
		})
	}
}

func desktopCodeCLI(t *testing.T, payload string, pin ArchivePin) string {
	t.Helper()
	for _, name := range pin.RequiredFiles {
		if strings.HasSuffix(name, "resources/app/out/cli.js") {
			return filepath.Join(payload, filepath.FromSlash(name))
		}
	}
	t.Fatal("Code pin must declare its actual CLI script layout")
	return ""
}

// Opt-in Linux fixture: a disposable Xvfb session must already be running.
func desktopNativeWindow(t *testing.T, home string, environment []string, program string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir, command.Env = home, append(os.Environ(), environment...)
	command.WaitDelay = 2 * time.Second
	var diagnostic boundedCommandOutput
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("desktop exited before mapping a window: %v: %s", err, diagnostic.data.String())
		case <-ctx.Done():
			err := <-done
			t.Fatalf("desktop failed to map a window: %v: %s", err, diagnostic.data.String())
		case <-ticker.C:
			probe := exec.CommandContext(ctx, "/usr/bin/xwininfo", "-root", "-tree")
			probe.Env = command.Env
			output, err := probe.CombinedOutput()
			if err == nil && strings.Contains(string(output), "dotfiles-desktop-proof") {
				cancel()
				// A signalled exit is expected after observing the mapped window.
				if err := <-done; err != nil {
					var signalled *exec.ExitError
					if !errors.As(err, &signalled) && !errors.Is(err, context.Canceled) {
						t.Fatal("reap mapped desktop application", err)
					}
				}
				return
			}
		}
	}
}
