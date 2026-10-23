package installer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// This extends the existing opted-in native agent lane. Every mutation uses the
// installed command and its exact preview; discovery never sends a model turn.
func exerciseInstalledSentinelLifecycle(t *testing.T, c Controller, home string, commands map[string]string, invoke, apply func(Request) Result) {
	t.Helper()
	d := c.Driver.(*NativeDriver)
	names, err := sentinelSkillNames()
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{filepath.Join(c.Home, ".agents", "skills"), filepath.Join(home, ".claude", "skills")}
	personal := map[string]configSnapshot{}
	for _, root := range roots {
		path := filepath.Join(root, "dotfiles-acceptance-personal", "SKILL.md")
		if _, err := os.Lstat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("personal skill fixture collision", path, err)
		}
		writeConfigFixture(t, path, "---\nname: dotfiles-acceptance-personal\ndescription: Personal acceptance witness.\n---\nPreserve my own skill.\n")
		if err := os.Chmod(path, 0640); err != nil {
			t.Fatal(err)
		}
		personal[path], err = snapshotConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		// Only this exclusively created fixture is removed, after proving exact
		// contents and native permissions. Other skills and recovery stay intact.
		t.Cleanup(func() {
			if err := verifyConfigSnapshot(path, personal[path]); err != nil {
				t.Error(err)
				return
			}
			if err := os.Remove(path); err != nil {
				t.Error(err)
				return
			}
			if err := os.Remove(filepath.Dir(path)); err != nil {
				t.Error(err)
			}
		})
	}
	selected := []string{"t3-code", "claude-code", "codex"}
	withSkills := append(slices.Clone(selected), "sentinel-skills")
	apply(Request{Schema: 1, Mode: "apply", Selected: withSkills})
	node, err := d.Archives.CommandPath("tool.node", "node")
	if err != nil {
		t.Fatal(err)
	}
	// Windows' native configuration home is its observed known folder. Keep
	// authentication/configuration private while discovering that real skill root.
	environment := append(agentCLIEnvironment(home), "HOME="+c.Home, "USERPROFILE="+c.Home,
		"PATH="+strings.Join([]string{filepath.Dir(commands["codex"]), filepath.Dir(commands["claude"]), filepath.Dir(node), os.Getenv("PATH")}, string(os.PathListSeparator)),
		"IMPECCABLE_HOME="+filepath.Join(home, "personal-impeccable-cache"))
	verify := func() {
		t.Helper()
		for _, root := range roots {
			for _, name := range names {
				expected, err := snapshotConfig(filepath.Join(d.SentinelSkills.Repository, "vendor", "sentinel", "skills", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := verifyConfigSnapshot(filepath.Join(root, name), expected); err != nil {
					t.Fatal("installed upstream skill differs", name, err)
				}
			}
		}
		for path, expected := range personal {
			if err := verifyConfigSnapshot(path, expected); err != nil {
				t.Fatal("personal skill changed", path, err)
			}
		}
		exerciseInstalledSkillDiscovery(t, home, environment, commands, roots[0], names)
	}
	verify()
	// Exercise the actual installed model-discovery helper against both local
	// host catalogs. These responses explicitly do not verify provider access.
	for _, host := range []string{"codex", "claude"} {
		output := runInstalledSkillHelper(t, node, environment, home, 45*time.Second,
			filepath.Join(roots[0], "setup-helix", "scripts", "discover-models.mjs"), host)
		var catalog struct {
			Host, Source   string
			AccessVerified bool
			Models         []json.RawMessage
		}
		if err := json.Unmarshal(output, &catalog); err != nil || catalog.Host != host || catalog.Source != "installed host catalog" || catalog.AccessVerified || len(catalog.Models) == 0 {
			t.Fatal("installed model discovery did not return a local catalog", host, err)
		}
	}
	engine := filepath.Join(roots[0], "impeccable", "scripts", "sentinel-engine.mjs")
	runInstalledSkillHelper(t, node, environment, home, 3*time.Minute, engine, "--setup")
	if output := runInstalledSkillHelper(t, node, environment, home, 30*time.Second, engine, "--help"); !strings.Contains(strings.ToLower(string(output)), "impeccable") {
		t.Fatal("installed Impeccable helper did not execute its verified engine")
	}
	cache, err := snapshotTree(filepath.Join(home, "personal-impeccable-cache"), maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshotTree(filepath.Dir(c.StatePath), maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("skill use changed installed health", result)
	}
	if after, err := snapshotTree(filepath.Dir(c.StatePath), maxPackageBytes, maxPackageEntries); err != nil || state != after {
		t.Fatal("skill Check changed state/runtime", err)
	}
	// Add a later personal edit and retain its native permission baseline.
	for path := range personal {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := io.WriteString(file, "Later personal instructions.\n")
		if err := errors.Join(writeErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		personal[path], err = snapshotConfig(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	// A real owned-file edit requires explicit replacement approval. The saved
	// edit must remain inspectable through removal and reinstallation.
	edited := filepath.Join(roots[0], names[0], "SKILL.md")
	data, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\nAcceptance edit to the managed skill.\n")...)
	if err := os.WriteFile(edited, data, 0600); err != nil {
		t.Fatal(err)
	}
	repaired := apply(Request{Schema: 1, Mode: "repair", Adopt: []string{"skills.sentinel-agents"}})
	var preserved []string
	for _, op := range repaired.Plan.Operations {
		if op.Resource == "skills.sentinel-agents" {
			preserved = op.Observed.Preserved
		}
	}
	// Read the completed receipt because publication adds preservation evidence.
	completed, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	preserved = append(preserved, completed.Receipts["skills.sentinel-agents"].After.Preserved...)
	if len(preserved) == 0 {
		t.Fatal("skill repair did not disclose preserved edits")
	}
	saved := map[string]configSnapshot{}
	found := false
	for _, path := range sortedUnique(preserved) {
		saved[path], err = snapshotConfig(path)
		if err != nil || saved[path].Kind == "absent" {
			t.Fatal("skill repair evidence unavailable", path, err)
		}
		err = filepath.WalkDir(path, func(p string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type().IsRegular() && entry.Name() == "SKILL.md" {
				contents, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				found = found || string(contents) == string(data)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("skill repair did not preserve the actual edited file")
	}
	verify()
	for cycle := 0; cycle < 2; cycle++ {
		apply(Request{Schema: 1, Mode: "apply", Selected: selected, RemoveShared: []string{"tool.node"}})
		for _, root := range roots {
			for _, name := range names {
				if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("removed skill remains", name, err)
				}
			}
		}
		for path, expected := range personal {
			if err := verifyConfigSnapshot(path, expected); err != nil {
				t.Fatal(err)
			}
		}
		for path, expected := range saved {
			if err := verifyConfigSnapshot(path, expected); err != nil {
				t.Fatal("repair evidence changed", err)
			}
		}
		if after, err := snapshotTree(filepath.Join(home, "personal-impeccable-cache"), maxPackageBytes, maxPackageEntries); err != nil || cache != after {
			t.Fatal("removal changed personal helper cache", err)
		}
		if cycle == 0 {
			apply(Request{Schema: 1, Mode: "apply", Selected: withSkills})
			verify()
		}
	}
	t.Log("installed Sentinel: exact upstream copies, both host discoveries, local model helpers, verified Impeccable engine, read-only Check, approved Repair, Remove/Reinstall, personal modes/cache and repair evidence preserved")
}

func runInstalledSkillHelper(t *testing.T, node string, environment []string, home string, timeout time.Duration, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Dir, cmd.Env = filepath.Join(home, "personal-project"), environment
	process, err := startAgentCLI(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	output, diagnostic := boundedCommandOutput{limit: 1 << 20}, boundedCommandOutput{}
	completed := agentCLIStreams(process, &output, &diagnostic)
	inputErr := process.input.Close()
	waitErr := process.wait(false)
	drainErr := agentCLIDrain(process, completed, 2)
	if err := errors.Join(inputErr, waitErr, drainErr); err != nil {
		t.Fatalf("installed skill helper: %v%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes()
}

func exerciseInstalledSkillDiscovery(t *testing.T, home string, environment []string, commands map[string]string, agentsRoot string, names []string) {
	t.Helper()
	project := filepath.Join(home, "personal-project")
	initialize := `{"id":1,"method":"initialize","params":{"clientInfo":{"name":"dotfiles-acceptance","version":"1"}}}`
	listing, err := json.Marshal(map[string]any{"id": 2, "method": "skills/list", "params": map[string]any{"cwds": []string{project}, "forceReload": true}})
	if err != nil {
		t.Fatal(err)
	}
	codex := installedDiscoveryReply(t, commands["codex"], []string{"app-server"}, environment, project,
		[]string{initialize, `{"method":"initialized"}`, string(listing)}, "codex")
	var response struct {
		Result struct {
			Data []struct {
				Cwd    string
				Errors []json.RawMessage
				Skills []struct{ Name, Path string }
			}
		}
		Error json.RawMessage
	}
	if err := json.Unmarshal(codex, &response); err != nil || len(response.Error) != 0 || len(response.Result.Data) != 1 || len(response.Result.Data[0].Errors) != 0 {
		t.Fatal("Codex skill discovery failed", err)
	}
	for _, name := range append(slices.Clone(names), "dotfiles-acceptance-personal") {
		matches := 0
		for _, skill := range response.Result.Data[0].Skills {
			if skill.Name == name {
				matches++
				got, err := resolveConfigPath(skill.Path)
				want, wantErr := resolveConfigPath(filepath.Join(agentsRoot, name, "SKILL.md"))
				if err != nil || wantErr != nil || got != want {
					t.Fatal("Codex discovered an unexpected skill copy", name, err, wantErr)
				}
			}
		}
		if matches != 1 {
			t.Fatal("Codex must discover each installed/personal skill exactly once", name, matches)
		}
	}
	claude := installedDiscoveryReply(t, commands["claude"], []string{"--print", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--no-session-persistence", "--tools", "", "--no-chrome", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--settings", `{"disableAllHooks":true,"autoMemoryEnabled":false}`}, environment, project,
		[]string{`{"type":"control_request","request_id":"skills","request":{"subtype":"initialize"}}`}, "claude")
	var control struct {
		Response struct {
			Subtype  string
			Response struct{ Commands []struct{ Name string } }
		}
	}
	if err := json.Unmarshal(claude, &control); err != nil || control.Response.Subtype != "success" {
		t.Fatal("Claude skill discovery failed", err)
	}
	for _, name := range append(slices.Clone(names), "dotfiles-acceptance-personal") {
		matches := 0
		for _, command := range control.Response.Response.Commands {
			if command.Name == name {
				matches++
			}
		}
		if matches != 1 {
			t.Fatal("Claude must discover each installed/personal skill exactly once", name, matches)
		}
	}
}

func installedDiscoveryReply(t *testing.T, executable string, args, environment []string, project string, requests []string, host string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir, cmd.Env = project, environment
	process, err := startAgentCLI(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := boundedCommandOutput{}
	completed := make(chan error, 2)
	go func() { _, err := io.Copy(&diagnostic, process.diagnostic); completed <- err }()
	type reply struct {
		data []byte
		err  error
	}
	response := make(chan reply, 1)
	go func() {
		defer func() { _, err := io.Copy(io.Discard, process.output); completed <- err }()
		scanner := bufio.NewScanner(process.output)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		initialized := false
		for count := 0; scanner.Scan(); count++ {
			if count >= 128 {
				response <- reply{err: errors.New("skill discovery exceeded message bound")}
				return
			}
			var message struct {
				ID       int
				Type     string
				Method   string
				Error    json.RawMessage
				Response struct {
					RequestID string `json:"request_id"`
				}
			}
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
				response <- reply{err: err}
				return
			}
			if host == "codex" && message.ID == 1 && message.Method == "" {
				if initialized || len(message.Error) != 0 {
					response <- reply{err: errors.New("Codex rejected or duplicated initialization")}
					return
				}
				// The protocol requires the initialize response before subsequent
				// requests; writing all three lines at once can race host setup.
				initialized = true
				for _, request := range requests[1:] {
					if _, err := io.WriteString(process.input, request+"\n"); err != nil {
						response <- reply{err: err}
						return
					}
				}
				continue
			}
			if host == "codex" && message.ID == 2 && message.Method == "" || host == "claude" && message.Type == "control_response" && message.Response.RequestID == "skills" {
				if host == "codex" && !initialized {
					response <- reply{err: errors.New("Codex listed skills before initialization")}
					return
				}
				response <- reply{data: slices.Clone(scanner.Bytes())}
				return
			}
		}
		response <- reply{err: errors.Join(errors.New("host exited without skill discovery"), scanner.Err())}
	}()
	var result reply
	for _, request := range requests[:1] {
		if _, err := io.WriteString(process.input, request+"\n"); err != nil {
			result.err = err
			break
		}
	}
	if result.err == nil {
		select {
		case result = <-response:
		case <-ctx.Done():
			result.err = ctx.Err()
		}
	}
	closeErr := process.input.Close()
	if result.err != nil || closeErr != nil {
		cancel()
	}
	waitErr := process.wait(false)
	drainErr := agentCLIDrain(process, completed, 2)
	if err := errors.Join(result.err, closeErr, waitErr, drainErr); err != nil {
		t.Fatal(fmt.Errorf("%s skill discovery: %w%s", host, err, nativeDiagnostic(diagnostic.data.Bytes())))
	}
	return result.data
}
