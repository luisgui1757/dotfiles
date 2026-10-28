package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Agent consumers use an isolated home and no authentication or provider routing.
// Even help/configuration commands must never load a runner's personal account.
func agentCLIEnvironment(home string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "T3CODE_") || strings.HasPrefix(key, "ANTHROPIC_") || strings.HasPrefix(key, "CLAUDE_") || strings.HasPrefix(key, "OPENAI_") || strings.HasPrefix(key, "CODEX_") ||
			strings.HasPrefix(key, "KIRO_") || strings.HasPrefix(key, "Q_") || strings.HasPrefix(key, "COPILOT_") || key == "GH_TOKEN" || key == "GITHUB_TOKEN" {
			continue
		}
		environment = append(environment, entry)
	}
	// The managed shell's update switches, as the generated environment sets them.
	return append(environment, "HOME="+home, "USERPROFILE="+home,
		"CODEX_HOME="+filepath.Join(home, ".codex"), "CLAUDE_CONFIG_DIR="+filepath.Join(home, ".claude"),
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"), "DISABLE_UPDATES=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "COPILOT_AUTO_UPDATE=false")
}

func exerciseAgentCLI(t *testing.T, command, product, home string) {
	t.Helper()
	run := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir, cmd.Env, cmd.WaitDelay = home, agentCLIEnvironment(home), 2*time.Second
		output, err := agentCLIOutput(ctx, cmd)
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", product, args, err, output)
		}
		return string(output)
	}
	if output := run("--help"); !strings.Contains(output, product) {
		t.Fatal("agent command help is unavailable", product, output)
	}
	if product == "T3 Code" {
		exerciseT3Server(t, command, home)
		return
	}
	if product == "Claude Code" {
		if output := run("--safe-mode", "mcp", "list"); !strings.Contains(output, "No MCP servers configured") {
			t.Fatal("standalone Claude configuration command failed", output)
		}
		return
	}
	// Initialize the actual app-server protocol without invoking a model. This
	// proves the installed binary starts its consumer runtime without credentials.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, "app-server")
	cmd.Dir, cmd.Env, cmd.WaitDelay = home, agentCLIEnvironment(home), 2*time.Second
	if err := agentCodexProtocol(ctx, cancel, cmd); err != nil {
		t.Fatal(err)
	}
}

func agentCodexProtocol(ctx context.Context, cancel context.CancelFunc, cmd *exec.Cmd) error {
	process, err := startAgentCLI(ctx, cmd)
	if err != nil {
		return err
	}
	stdin, stdout := process.input, process.output
	stderr := boundedCommandOutput{}
	completed := make(chan error, 2)
	go func() { _, err := io.Copy(&stderr, process.diagnostic); completed <- err }()
	_, writeErr := io.WriteString(stdin, "{\"id\":1,\"method\":\"initialize\",\"params\":{\"clientInfo\":{\"name\":\"dotfiles-acceptance\",\"version\":\"1\"}}}\n")
	acknowledged := make(chan error, 1)
	go func() {
		// Keep consuming after the acknowledgement; root exit and stream EOF
		// are distinct events when an app-server launches background helpers.
		defer func() { _, err := io.Copy(io.Discard, stdout); completed <- err }()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var response struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
				acknowledged <- err
				return
			}
			if response.ID == 1 {
				if len(response.Result) == 0 || len(response.Error) != 0 {
					acknowledged <- errors.New("Codex rejected app-server initialization: " + scanner.Text())
				} else {
					acknowledged <- nil
				}
				return
			}
		}
		acknowledged <- errors.Join(errors.New("Codex did not acknowledge app-server initialization"), scanner.Err())
	}()
	protocolErr := writeErr
	if protocolErr == nil {
		select {
		case protocolErr = <-acknowledged:
		case <-ctx.Done():
			protocolErr = ctx.Err()
		}
	}
	if protocolErr == nil {
		_, protocolErr = io.WriteString(stdin, "{\"method\":\"initialized\",\"params\":{}}\n")
	}
	protocolErr = errors.Join(protocolErr, stdin.Close())
	if protocolErr != nil {
		cancel()
	}
	waitErr := process.wait(false)
	drainErr := agentCLIDrain(process, completed, 2)
	if err := errors.Join(protocolErr, waitErr, drainErr); err != nil {
		return fmt.Errorf("Codex app-server startup: %w%s", err, nativeDiagnostic(stderr.data.Bytes()))
	}
	return nil
}

func TestNativeAgentCLIInstalledApplicationLifecycle(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_AGENT_CLIS") != "1" {
		t.Skip("agent CLI application lifecycle requires an explicitly opted-in disposable native runner")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Codex refuses helper aliases in the OS temporary directory. Keep this
	// isolated disposable home under the runner account, as a real user would.
	home, err := os.MkdirTemp(userHome, ".dotfiles-agent-cli-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(home); err != nil {
			t.Error(err)
		}
	})
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	invoke := installedMachineFixture(t, repository, home, 15*time.Minute)
	apply := func(request Request) Result {
		t.Helper()
		preview := invoke(request)
		if preview.Status != "preview" {
			t.Fatal("agent lifecycle preview", preview)
		}
		request.ExpectedPlan = preview.Plan.ID
		result := invoke(request)
		if result.Status != "ready" {
			t.Fatal("agent lifecycle mutation", result)
		}
		return result
	}
	witnesses := map[string]string{
		filepath.Join(home, ".t3", "userdata", "personal-witness.txt"):          "keep T3 personal data\n",
		filepath.Join(home, "personal-project", "notes.txt"):                    "keep project data\n",
		filepath.Join(home, ".codex", "history.jsonl"):                          "{\"personal\":\"keep Codex history\"}\n",
		filepath.Join(home, ".claude", "projects", "personal", "session.jsonl"): "{\"personal\":\"keep Claude session\"}\n",
	}
	for path, contents := range witnesses {
		writeConfigFixture(t, path, contents)
	}
	// Exercise the independent choices incrementally. Each public request keeps
	// its existing deadline rather than combining two large fresh downloads.
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"t3-code"}})
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"t3-code", "claude-code"}})
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"t3-code", "claude-code", "codex"}})
	platform := nativePlatform(t)
	if platform.OS == "linux" {
		platform.APT, err = DiscoverLinuxAPT(context.Background())
	} else if platform.OS == "darwin" {
		platform.Homebrew, err = DiscoverHomebrew(context.Background())
	}
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: home}
	stateDirectory := filepath.Join(home, ".local", "state", "dotfiles")
	if platform.OS == "windows" {
		folders, err = DiscoverConfigFolders()
		if err != nil {
			t.Fatal(err)
		}
		stateDirectory, err = DiscoverStateDirectory(platform.Context, folders)
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewNativeController(repository, "agent-inspection", stateDirectory, platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	commands := map[string]string{}
	for id, product := range map[string]string{"t3": "T3 Code", "claude": "Claude Code", "codex": "Codex"} {
		commands[id], err = d.Archives.CommandPath("tool."+id, id)
		if err != nil {
			t.Fatal(err)
		}
		exerciseAgentCLI(t, commands[id], product, home)
	}
	agents := []string{"tool.t3", "tool.claude", "tool.codex", "tool.copilot"}
	selected := []string{"t3-code", "claude-code", "codex", "copilot-cli"}
	apply(Request{Schema: 1, Mode: "apply", Selected: selected})
	commands["copilot"] = exerciseInstalledCopilotCLI(t, d, home)
	removeShared := []string{"tool.git"}
	if platform.OS == "windows" {
		// Copilot's PowerShell 7 prerequisite is shared; this disposable state
		// must end empty for the later public migration fixture.
		removeShared = append(removeShared, "tool.powershell")
	} else {
		// Kiro CLI is offered on macOS and Linux only.
		commands["kiro"] = exerciseInstalledKiroCLI(t, d, home, witnesses, apply, selected)
		agents = append(agents, "tool.kiro", kiroSettingsResource)
	}
	before, err := LoadState(filepath.Join(stateDirectory, "state.json"), folders.Home)
	if err != nil {
		t.Fatal(err)
	}
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("agent use changed the managed payload", result)
	}
	update := apply(Request{Schema: 1, Mode: "update"})
	for _, id := range agents {
		if action := operation(t, update.Plan, id).Action; action != "keep" {
			t.Fatal("unchanged agent update was not idempotent", id, action)
		}
	}
	for _, agent := range []struct{ command, product string }{{"claude", "Claude Code"}, {"t3", "T3 Code"}} {
		if err := os.Remove(d.Archives.currentLink("tool." + agent.command)); err != nil {
			t.Fatal("damage the fixture-owned entrypoint", agent.command, err)
		}
		apply(Request{Schema: 1, Mode: "repair"})
		exerciseAgentCLI(t, commands[agent.command], agent.product, home)
	}
	// Preserve the actual database (and any SQLite sidecars), not only fixtures.
	databases, err := filepath.Glob(filepath.Join(home, ".t3", "userdata", "statev2.sqlite*"))
	if err != nil || len(databases) == 0 {
		t.Fatal("T3 database witness unavailable", err)
	}
	for _, path := range databases {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		witnesses[path] = string(contents)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"codex"}, RemoveShared: []string{"tool.git"}})
	exerciseAgentCLI(t, commands["codex"], "Codex", home)
	for _, id := range []string{"claude", "t3"} {
		if _, err := os.Lstat(commands[id]); !os.IsNotExist(err) {
			t.Fatal("removed agent entrypoint remains", id, err)
		}
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: removeShared})
	for id, command := range commands {
		if _, err := os.Lstat(command); !os.IsNotExist(err) {
			t.Fatal("removed agent entrypoint remains", id, err)
		}
	}
	for path, contents := range witnesses {
		if actual, err := os.ReadFile(path); err != nil || string(actual) != contents {
			t.Fatal("agent removal changed personal data", path, err)
		}
	}
	for id, receipt := range before.Receipts {
		if id == "tool.git" && receipt.Ownership == "reused" {
			resource, _ := c.Catalog.Resource(id)
			if after, err := d.Observe(context.Background(), resource, receipt); err != nil || !sameArtifact(after, receipt.After) {
				t.Fatal("agent removal changed pre-existing Git", after, err)
			}
		}
	}
}

// Consume the bundled frontend and native server, without a provider login or
// background service. The desktop smoke separately verifies browser interaction.
func exerciseT3Server(t *testing.T, command, home string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, "serve", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--base-dir", filepath.Join(home, ".t3"), "--auto-bootstrap-project-from-cwd")
	cmd.Dir, cmd.Env, cmd.WaitDelay = filepath.Join(home, "personal-project"), agentCLIEnvironment(home), 2*time.Second
	// Startup can print a one-time pairing secret. Never export it as diagnostics.
	process, err := startAgentCLI(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	completed := agentCLIStreams(process, io.Discard, io.Discard)
	inputErr := process.input.Close()
	defer func() {
		waitErr := process.wait(true)
		drainErr := agentCLIDrain(process, completed, 2)
		if err := errors.Join(inputErr, waitErr, drainErr); err != nil {
			t.Error(err)
		}
	}()
	client := &http.Client{Timeout: time.Second}
	origin := "http://127.0.0.1:" + strconv.Itoa(port)
	var page []byte
	for {
		response, err := client.Get(origin)
		if err == nil {
			page, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
			closeErr := response.Body.Close()
			if err != nil || closeErr != nil {
				t.Fatal(errors.Join(err, closeErr))
			}
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("T3 did not serve its frontend before the startup deadline")
		case <-time.After(200 * time.Millisecond):
		}
	}
	script := regexp.MustCompile(`src="(/assets/[^" ]+\.js)"`).FindSubmatch(page)
	if len(script) != 2 || !bytes.Contains(page, []byte(`<div id="root"`)) {
		t.Fatal("T3 frontend entrypoint is incomplete")
	}
	response, err := client.Get(origin + string(script[1]))
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err := errors.Join(readErr, response.Body.Close()); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(body) < 1000 || !strings.Contains(response.Header.Get("Content-Type"), "javascript") {
		t.Fatal("T3 frontend bundle is unavailable")
	}
	if _, err := os.Stat(filepath.Join(home, ".t3", "userdata", "statev2.sqlite")); err != nil {
		t.Fatal("T3 did not initialize its conversation database", err)
	}
}
