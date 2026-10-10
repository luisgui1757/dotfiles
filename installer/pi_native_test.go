package installer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Exercise the bundled runtime and TypeScript extension loader without a model,
// credential, external Node process or upstream tool download. Package install
// features separately retain Node/npm and Git in the capability graph.
func exercisePiOfflineExtension(t *testing.T, command, home string) {
	t.Helper()
	extension := filepath.Join(home, "extension with 'quotes'.ts")
	writeConfigFixture(t, extension, `import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
export default function (pi: ExtensionAPI) {
	pi.registerCommand("dotfiles-runtime-proof", {
		description: "Bundled extension loader is operational",
		handler: async () => {},
	});
}


`)
	exercisePiRPCCommands(t, command, home, piFixtureEnvironment(home, t.TempDir()), []string{"--no-extensions", "-e", extension}, "dotfiles-runtime-proof", true)
}

func piFixtureEnvironment(home, path string) []string {
	environment := []string{"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "XDG_DATA_HOME=" + filepath.Join(home, "data"), "PI_CODING_AGENT_DIR=" + filepath.Join(home, ".pi", "agent"), "PI_OFFLINE=1", "PATH=" + path, "NO_COLOR=1"}
	for _, key := range []string{"SystemRoot", "ComSpec", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

func exercisePiRPCCommands(t *testing.T, command, home string, environment, extra []string, expected string, present bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := append([]string{"--mode", "rpc", "--no-session", "--no-skills", "--no-prompt-templates", "--no-mcp"}, extra...)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = home
	cmd.Env = environment
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := boundedCommandOutput{}
	cmd.Stderr = &diagnostic
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited {
			cancel()
			if err := cmd.Wait(); err != nil {
				t.Log("Pi fixture terminated after test failure:", err)
			}
		}
	})
	if _, err := fmt.Fprintln(input, `{"id":"commands","type":"get_commands"}`); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	found, responded := false, false
	names := []string{}
	for scanner.Scan() {
		var response struct {
			ID      string `json:"id"`
			Success bool   `json:"success"`
			Data    struct {
				Commands []struct{ Name, Description string } `json:"commands"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal("Pi emitted invalid RPC JSON", err)
		}
		if response.ID != "commands" {
			continue
		}
		if !response.Success {
			t.Fatal("Pi rejected its command inventory request")
		}
		responded = true
		for _, command := range response.Data.Commands {
			names = append(names, command.Name)
			found = found || command.Name == expected && command.Description == "Bundled extension loader is operational"
		}
		break
	}
	if err := scanner.Err(); err != nil || !responded || found != present {
		cancel()
		waitErr := cmd.Wait()
		waited = true
		t.Fatal("Pi command inventory did not match its installed extensions", expected, present, names, err, waitErr, diagnostic.data.String())
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	// Drain the remaining protocol output before Wait closes the pipe.
	for scanner.Scan() {
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil || strings.Contains(diagnostic.data.String(), "Failed to load extension") {
		t.Fatal("Pi failed its offline extension lifecycle", err, diagnostic.data.String())
	}
}

// This fixture may reuse existing Git, but never approves a native package
// mutation. The real compiled application installs all private Pi dependencies.
func TestNativeBrewPiInstalledApplicationLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("DOTFILES_TEST_NATIVE_INVENTORY") != "1" && os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicit native Homebrew inventory inspection on macOS")
	}
	ctx := context.Background()
	location, err := DiscoverHomebrew(ctx)
	if err != nil || location == nil {
		t.Fatal("fixture requires existing Homebrew", err)
	}
	before, err := brewInventory(ctx, location.query)
	if err != nil || !before["git"].Healthy {
		t.Fatal("read-only native fixture requires pre-existing healthy Git", err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(home, ".pi", "agent", "settings.json")
	original := "{\n  \"theme\" : \"dark\", \"personal\" : 9007199254740993\n}\n"
	writeConfigFixture(t, settings, original)
	session := filepath.Join(home, ".pi", "agent", "sessions", "personal.jsonl")
	writeConfigFixture(t, session, "personal session fixture\n")
	invoke := installedMachineFixture(t, repository, home, 3*time.Minute)
	apply := func(request Request) {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal("Pi application could not produce a preview", preview)
		}
		for _, op := range preview.Plan.Operations {
			if op.Resource == "tool.git" && op.Action != "keep" && op.Action != "retain" {
				t.Fatal("read-only native fixture refuses to mutate Git", op)
			}
		}
		request.ExpectedPlan = preview.Plan.ID
		if result := invoke(request); result.Status != "ready" {
			t.Fatal("Pi application lifecycle failed", result)
		}
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"pi"}, Adopt: []string{"integration.pi"}})
	platform := NativePlatform{Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}}
	pins, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	archives := ArchiveDriver{Directory: filepath.Join(home, ".local", "state", "dotfiles", "packages"), Pins: pins}
	command, err := archives.CommandPath("tool.pi", "pi")
	if err != nil {
		t.Fatal(err)
	}
	exercisePiOfflineExtension(t, command, home)
	data, err := os.ReadFile(settings)
	if err != nil || !strings.Contains(string(data), `"rose-pine"`) {
		t.Fatal("Pi theme was not configured", string(data), err)
	}
	writeConfigFixture(t, settings, strings.Replace(string(data), "9007199254740993", "9007199254740995", 1))
	apply(Request{Schema: 1, Mode: "update"})
	exercisePiOfflineExtension(t, command, home)
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.node"}})
	data, err = os.ReadFile(settings)
	if err != nil || string(data) != strings.Replace(original, "9007199254740993", "9007199254740995", 1) {
		t.Fatal("Pi removal lost prior theme or later personal edits", string(data), err)
	}
	if data, err := os.ReadFile(session); err != nil || string(data) != "personal session fixture\n" {
		t.Fatal("Pi lifecycle changed personal session data", err)
	}
	if _, err := os.Stat(command); !os.IsNotExist(err) {
		t.Fatal("Pi private executable survived removal", err)
	}
	after, err := brewInventory(ctx, location.query)
	if err != nil || len(after) != len(before) {
		t.Fatal("Pi private lifecycle changed the native package set", err)
	}
	for name, prior := range before {
		if now := after[name]; now.Version != prior.Version || brewClassificationOf(now) != brewClassificationOf(prior) {
			t.Fatal("Pi private lifecycle changed a pre-existing native package", name)
		}
	}
}
