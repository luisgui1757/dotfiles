package terminal

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	engine "github.com/luisgui1757/dotfiles/installer"
	"golang.org/x/term"
)

func publicSetupCommand(ctx context.Context, repository string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		return exec.CommandContext(ctx, powershell, append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(repository, "setup.ps1")}, args...)...)
	}
	return exec.CommandContext(ctx, "bash", append([]string{filepath.Join(repository, "setup.sh")}, args...)...)
}

// screens splits terminal output at full-screen clears. POSIX passes the
// console's "\x1b[H\x1b[2J" through; ConPTY repaints each cleared frame as
// "\x1b[2J\x1b[H" plus its content. Both contain the clear itself.
func (s *nativeSession) screens() []string {
	return strings.Split(s.text(), "\x1b[2J")
}

// nextScreen waits for a complete redraw after the first seen ones, whose
// newest screen satisfies want, and returns it with the redraw count.
func (s *nativeSession) nextScreen(t *testing.T, seen int, want func(string) bool, timeout time.Duration) (string, int) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		screens := s.screens()
		latest := screens[len(screens)-1]
		if len(screens)-1 > seen && strings.Contains(latest, "quit") && want(latest) {
			return latest, len(screens) - 1
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal did not show the expected screen; latest: %q", latest)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// chooseMenu waits for a main menu after the first seen ones and moves from
// its actual initial entry to label. It returns the menus seen so far.
func (s *nativeSession) chooseMenu(t *testing.T, seen int, label string) int {
	t.Helper()
	labels := []string{"Choose tools", "Update tools", "Check installation", "Exit"}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		menus := strings.Count(s.text(), "What would you like to do?")
		screens := s.screens()
		latest := screens[len(screens)-1]
		current := slices.IndexFunc(labels, func(entry string) bool { return strings.Contains(latest, "> "+entry) })
		if menus > seen && strings.Contains(latest, "What would you like to do?") && strings.Contains(latest, "Ctrl+C: quit") && current >= 0 {
			target := slices.Index(labels, label)
			s.send(t, strings.Repeat("j", (target-current+len(labels))%len(labels))+"\r")
			return menus
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal did not show the main menu; latest: %q", latest)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func testPublicSetupChild(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_PUBLIC_UI") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("public UI child requires its disposable hosted acceptance lane")
	}
	before, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := publicSetupCommand(ctx, os.Getenv("DOTFILES_TEST_UI_CHECKOUT"))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal("public interactive setup", err)
	}
	fmt.Println("Ready for ordinary input")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ordinary input" {
		t.Fatal("public setup stole subsequent input", err)
	}
	after, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("public setup did not restore terminal state", err)
	}
	fmt.Println("public-setup-complete; terminal-restored=true")
}

// The public wrappers, real catalog, providers and native terminal are connected
// here. Machine calls only inspect; every mutation is approved by actual keys.
func TestNativePublicInteractiveLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_PUBLIC_UI") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("requires the explicitly opted-in disposable hosted UI lane")
	}
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	evidence := os.Getenv("DOTFILES_TEST_UI_EVIDENCE")
	if !filepath.IsAbs(evidence) {
		t.Fatal("public UI requires an absolute evidence directory")
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(evidence, "private home ü")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal("fixture home must be fresh", err)
	}
	private, err = filepath.EvalSymlinks(private)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		t.Setenv("HOME", private)
	}
	for key, value := range map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(private, ".config"), "XDG_DATA_HOME": filepath.Join(private, ".local", "share"),
		"XDG_STATE_HOME": filepath.Join(private, ".local", "state"), "XDG_CACHE_HOME": filepath.Join(private, ".cache"),
		"CODEX_HOME": filepath.Join(private, ".codex"), "CLAUDE_CONFIG_DIR": filepath.Join(private, ".claude"),
		"ZDOTDIR": private, "DOTFILES_TEST_UI_CHECKOUT": repository, "TERM": "xterm-256color",
	} {
		t.Setenv(key, value)
	}
	folders, err := engine.DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := engine.DiscoverPlatform()
	if err != nil {
		t.Fatal(err)
	}
	stateDirectory, err := engine.DiscoverStateDirectory(platform.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDirectory, "state.json")
	if _, err := os.Lstat(stateDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("public first-run acceptance requires an absent state directory", err)
	}
	// Always preserve the original operation journals/payloads. The public Go
	// compiler cache is excluded explicitly; it is not installer ownership state.
	journalRoot := stateDirectory
	defer func() { archivePublicJourney(t, journalRoot, filepath.Join(evidence, "installer-state.tar.gz")) }()
	personal := filepath.Join(private, "personal document ü.txt")
	if err := os.WriteFile(personal, []byte("one\nacceptance needle\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	personalBefore := publicTree(t, personal)
	invoke := func(t *testing.T, request engine.Request) engine.Result {
		t.Helper()
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := publicSetupCommand(ctx, repository, "machine")
		cmd.Stdin = bytes.NewReader(data)
		var output, diagnostic bytes.Buffer
		cmd.Stdout, cmd.Stderr, cmd.WaitDelay = &output, &diagnostic, 2*time.Second
		runErr := cmd.Run()
		result, err := decodePublicMachine(output.Bytes(), runErr)
		if err != nil {
			t.Fatalf("public inspection: %v\n%s\n%s", err, output.String(), diagnostic.String())
		}
		return result
	}
	selected := []string{"ripgrep"}
	initial := engine.Request{Schema: 1, Mode: "apply", Selected: selected}
	preview := invoke(t, initial)
	if preview.Status != "preview" {
		t.Fatal("initial public preview", preview)
	}
	before := publicTree(t, stateDirectory)
	// Every interaction is a separate process/session. Subtest cleanup requires
	// its successful exit before the following state or consumer assertion.
	journey := func(name, action string, request engine.Request, approve bool) {
		t.Helper()
		if !t.Run(name, func(t *testing.T) {
			expected := invoke(t, request)
			if request.Mode == "check" {
				if expected.Status != "ready" {
					t.Fatal(expected)
				}
			} else if expected.Status != "preview" {
				t.Fatal(expected)
			}
			s := startNativeSession(t, "public-setup")
			t.Cleanup(func() {
				if err := os.WriteFile(filepath.Join(evidence, name+".terminal.txt"), []byte(s.text()), 0600); err != nil {
					t.Error("preserve terminal evidence", err)
				}
			})
			if err := s.resize(300, 180); err != nil {
				t.Fatal(err)
			}
			menus := 0
			if _, err := os.Stat(statePath); err == nil {
				// Removal is unticking in Choose tools; Update also repairs.
				label := map[string]string{"choose": "Choose tools", "remove": "Choose tools", "update": "Update tools", "check": "Check installation"}[action]
				if label == "" {
					t.Fatal("unknown action", action)
				}
				menus = s.chooseMenu(t, menus, label)
			}
			if action == "choose" {
				s.waitForWithin(t, "Choose the tools you want installed", 5*time.Minute)
				catalog, err := engine.DefaultCatalog()
				if err != nil {
					t.Fatal(err)
				}
				options := []Option{}
				for _, resource := range catalog.Capabilities(platform.Context) {
					options = append(options, Option{ID: resource.ID, Label: resource.Name, Category: resource.Category})
				}
				rows := selectionRows(options, true)
				// All rows fit in this terminal. Assert actual platform filtering,
				// then select explicit independent children from the real catalog.
				last := rows[len(rows)-1].Label
				s.waitFor(t, last)
				for _, row := range rows {
					if !strings.Contains(s.text(), row.Label) {
						t.Fatal("real selector omitted capability/category", row.Label)
					}
				}
				if runtime.GOOS != "windows" && strings.Contains(s.text(), "Windows Terminal") {
					t.Fatal("Windows Terminal leaked onto POSIX")
				}
				if runtime.GOOS == "darwin" && !strings.Contains(s.text(), "AeroSpace") {
					t.Fatal("Mac selector omitted AeroSpace")
				}
				s.send(t, "n")
				cursor := 0
				for _, id := range selected {
					index := slices.IndexFunc(rows, func(row selectionRow) bool { return row.ID == id })
					if index < 0 {
						t.Fatal("missing requested tool", id)
					}
					key := "j"
					distance := index - cursor
					if distance < 0 {
						key, distance = "k", -distance
					}
					s.send(t, strings.Repeat(key, distance)+" ")
					s.waitFor(t, ">   [x] "+rows[index].Label)
					cursor = index
				}
				s.send(t, "\r")
			} else if action == "remove" {
				s.waitForWithin(t, "Choose the tools you want installed", 5*time.Minute)
				s.send(t, "n\r")
			}
			if request.Mode == "check" {
				s.waitFor(t, "All selected resources passed verification.")
				s.send(t, "\r")
				s.chooseMenu(t, menus, "Exit")
			} else {
				confirmation := func(screen string) bool {
					return strings.Contains(screen, "Apply these changes?") && !strings.Contains(screen, "Updating the plan...")
				}
				screen, seen := s.nextScreen(t, 0, confirmation, 5*time.Minute)
				catalog, err := engine.DefaultCatalog()
				if err != nil {
					t.Fatal(err)
				}
				// Optional decisions are checkboxes on the confirmation. Tick
				// each requested replacement and wait for its revised plan.
				for _, id := range request.Adopt {
					resource, ok := catalog.Resource(id)
					if !ok {
						t.Fatal("unknown adoption", id)
					}
					for step := 0; !strings.Contains(screen, "> [x] "+resource.Name); step++ {
						if step > 60 {
							t.Fatal("confirmation did not offer the replacement", id, screen)
						}
						if strings.Contains(screen, "> [ ] "+resource.Name) {
							s.send(t, " ")
						} else {
							s.send(t, "k")
						}
						screen, seen = s.nextScreen(t, seen, confirmation, 5*time.Minute)
					}
				}
				s.send(t, "d")
				s.waitFor(t, "Review every change")
				s.waitFor(t, "Plan: "+expected.Plan.ID)
				s.waitFor(t, "Source: "+expected.Plan.Source)
				for _, op := range expected.Plan.Operations {
					resource, ok := catalog.Resource(op.Resource)
					if !ok {
						t.Fatal("unknown preview resource", op.Resource)
					}
					s.waitFor(t, strings.ToUpper(op.Action)+": "+resource.Name+" ("+op.Resource+")")
				}
				for _, summary := range planSummary(catalog, expected.Plan) {
					s.waitFor(t, summary)
				}
				// Leave the details. A ticked replacement keeps the cursor on
				// its checkbox; move down to Apply before approving.
				s.send(t, "\x1b")
				if approve {
					screen, seen = s.nextScreen(t, seen, confirmation, time.Minute)
					for step := 0; !strings.Contains(screen, "> Apply changes"); step++ {
						if step > 60 {
							t.Fatal("confirmation has no Apply action", screen)
						}
						s.send(t, "j")
						screen, seen = s.nextScreen(t, seen, confirmation, time.Minute)
					}
					s.send(t, "\r")
				} else {
					s.nextScreen(t, seen, confirmation, time.Minute)
					s.send(t, "\x1b")
					s.chooseMenu(t, menus, "Exit")
				}
			}
			s.waitForWithin(t, "public-setup-complete; terminal-restored=true", 5*time.Minute)
		}) {
			t.FailNow()
		}
	}
	journey("01-back", "choose", initial, false)
	if !reflect.DeepEqual(before, publicTree(t, stateDirectory)) {
		t.Fatal("default Back changed ownership state")
	}
	journey("02-install", "choose", initial, true)
	controller, err := engine.NewNativeController(repository, "inspection-only", stateDirectory, platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	archives := controller.Driver.(*engine.NativeDriver).Archives
	command, err := archives.CommandPath("tool.rg", "rg")
	if err != nil {
		t.Fatal(err)
	}
	consume := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, command, "--no-config", "--fixed-strings", "acceptance needle", personal).CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != "acceptance needle" {
			t.Fatal("installed ripgrep did not search the actual personal file", err, string(output))
		}
		state, err := engine.LoadState(statePath, folders.Home)
		if err != nil || state.Transaction != nil || !slices.Equal(state.Selected, selected) {
			t.Fatal("interactive selection did not reach Ready", err, state.Selected)
		}
	}
	consume()
	check := func(name string) {
		t.Helper()
		before := publicTree(t, stateDirectory)
		journey(name, "check", engine.Request{Schema: 1, Mode: "check"}, false)
		if !reflect.DeepEqual(before, publicTree(t, stateDirectory)) {
			t.Fatal("public Check changed state, journals or managed payload")
		}
	}
	check("03-check")
	update := invoke(t, engine.Request{Schema: 1, Mode: "update"})
	for _, op := range update.Plan.Operations {
		if op.Action != "keep" {
			t.Fatal("unchanged Update is not a no-op", op.Resource, op.Action)
		}
	}
	journey("04-update", "update", engine.Request{Schema: 1, Mode: "update"}, true)
	current := filepath.Join(archives.Directory, "tool.rg", "current")
	if err := os.Remove(current); err != nil {
		t.Fatal("damage only the fixture-owned published pointer", err)
	}
	broken := invoke(t, engine.Request{Schema: 1, Mode: "check"})
	if broken.Status != "needs-action" || !strings.Contains(broken.Message, "tool.rg") {
		t.Fatal("actual missing entrypoint was not detected", broken)
	}
	// The menu's repair path is Update, which repairs failed checks.
	journey("05-repair", "update", engine.Request{Schema: 1, Mode: "update"}, true)
	consume()
	for cycle := 0; cycle < 2; cycle++ {
		journey(fmt.Sprintf("%d-remove", 6+cycle*2), "remove", engine.Request{Schema: 1, Mode: "apply", Selected: []string{}}, true)
		if _, err := os.Lstat(command); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("removed command remains", err)
		}
		if !reflect.DeepEqual(personalBefore, publicTree(t, personal)) {
			t.Fatal("personal contents or native permissions changed")
		}
		if cycle == 0 {
			journey("07-reinstall", "choose", initial, true)
			consume()
		}
	}
	check("09-final-check")
	if runtime.GOOS == "windows" {
		// Later public fixtures also require a fresh native known folder. After
		// this exclusive fixture has completed removal, retain its whole state
		// unchanged beside the original path (same volume), then archive it.
		// Never relocate a failed or unfinished journey to force progress.
		before := publicTree(t, stateDirectory)
		journalRoot = retainCompletedPublicJourney(t, stateDirectory, folders.Home)
		if !reflect.DeepEqual(before, publicTree(t, journalRoot)) {
			t.Fatal("completed public fixture evidence changed during relocation")
		}
	}
	t.Log("public keyboard journey passed: real bootstrap/catalog/approval/provider/consumer; Back, Check, unchanged Update, meaningful Repair, Remove/Reinstall; personal bytes/modes and original journals preserved")
}

func decodePublicMachine(data []byte, runErr error) (engine.Result, error) {
	var result engine.Result
	if err := engine.Decode(data, &result); err != nil {
		return result, errors.Join(err, runErr)
	}
	unhealthy := result.Status == "needs-action" || result.Status == "failed"
	var exit *exec.ExitError
	if unhealthy && errors.As(runErr, &exit) && exit.ExitCode() == 1 || !unhealthy && runErr == nil {
		return result, nil
	}
	return result, fmt.Errorf("machine exit does not match status %s: %v", result.Status, runErr)
}

func retainCompletedPublicJourney(t *testing.T, directory, home string) string {
	t.Helper()
	state, err := engine.LoadState(filepath.Join(directory, "state.json"), home)
	if err != nil || state.Transaction != nil || len(state.Selected) != 0 || len(state.Keep) != 0 {
		t.Fatal("cannot relocate unfinished public fixture", err)
	}
	for id, receipt := range state.Receipts {
		if receipt.Status != "removed" || receipt.After.RemovalDeferred {
			t.Fatal("cannot relocate public fixture with active resources", id, receipt.Status)
		}
	}
	destination := directory + "-public-ui-completed"
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed fixture evidence destination already exists", err)
	}
	if err := os.Rename(directory, destination); err != nil {
		t.Fatal("retain completed public fixture intact", err)
	}
	return destination
}

type publicEntry struct {
	Mode    os.FileMode
	Content string
}

func publicTree(t *testing.T, root string) map[string]publicEntry {
	t.Helper()
	result := map[string]publicEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if path == root && errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "bootstrap" && entry.IsDir() {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := publicEntry{Mode: info.Mode()}
		switch {
		case info.Mode().IsRegular():
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(hash, file)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
			value.Content = hex.EncodeToString(hash.Sum(nil))
		case info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0:
			value.Content, err = os.Readlink(path)
			if err != nil {
				return err
			}
		case !info.IsDir():
			return fmt.Errorf("unsupported evidence file %s", path)
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func archivePublicJourney(t *testing.T, root, destination string) {
	t.Helper()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Error(err)
		return
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if path == root && errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "bootstrap" && entry.IsDir() {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		header, err := publicArchiveHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(archive, input)
			return errors.Join(copyErr, input.Close())
		}
		return nil
	})
	if err := errors.Join(err, archive.Close(), compressed.Close(), file.Close()); err != nil {
		t.Error("preserve original public journey evidence", err)
	}
}

func publicArchiveHeader(info os.FileInfo, link string) (*tar.Header, error) {
	if info.Mode()&os.ModeIrregular != 0 {
		return &tar.Header{Typeflag: tar.TypeSymlink, Linkname: link, Mode: int64(info.Mode().Perm()), ModTime: info.ModTime(),
			PAXRecords: map[string]string{"DOTFILES.link-kind": "junction"}}, nil
	}
	return tar.FileInfoHeader(info, link)
}
