package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// exerciseInstalledKiroCLI runs the pinned Kiro CLI commands without
// credentials, proves that its one owned setting turns self-update off beside
// personal settings, and that Sentinel skills reach the now-present Kiro. The
// caller's later Check, Update and removal cover the payload and witnesses.
func exerciseInstalledKiroCLI(t *testing.T, d *NativeDriver, home string, witnesses map[string]string, apply func(Request) Result, base []string) string {
	t.Helper()
	kiro := filepath.Join(home, ".kiro")
	settings := filepath.Join(kiro, "settings", "cli.json")
	for path, contents := range map[string]string{
		settings: "{\n  \"chat.greeting.enabled\": false\n}\n",
		filepath.Join(kiro, "settings", "mcp.json"): "{\"mcpServers\":{}}\n",
		filepath.Join(kiro, "steering", "mine.md"):  "Keep my Kiro steering.\n",
	} {
		writeConfigFixture(t, path, contents)
		witnesses[path] = contents
	}
	selected := append(slices.Clone(base), "kiro-cli")
	apply(Request{Schema: 1, Mode: "apply", Selected: append(slices.Clone(selected), "sentinel-skills")})
	pin, ok := d.Archives.Pins["tool.kiro"]
	if !ok {
		t.Fatal("native runner has no Kiro CLI artifact")
	}
	commands := map[string]string{}
	for _, name := range []string{"kiro-cli", "kiro-cli-chat"} {
		command, err := d.Archives.CommandPath("tool.kiro", name)
		if err != nil {
			t.Fatal(err)
		}
		commands[name] = command
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, command, "--version")
		cmd.Dir, cmd.Env, cmd.WaitDelay = home, agentCLIEnvironment(home), 2*time.Second
		output, err := agentCLIOutput(ctx, cmd)
		cancel()
		if err != nil || !strings.Contains(string(output), pin.Version) {
			t.Fatalf("pinned %s did not run from its private payload: %v\n%s", name, err, output)
		}
	}
	data, err := os.ReadFile(settings)
	values := map[string]any{}
	if err == nil {
		err = json.Unmarshal(data, &values)
	}
	if err != nil || values["app.disableAutoupdates"] != true || values["chat.greeting.enabled"] != false {
		t.Fatalf("Kiro CLI can still replace its pinned payload or lost a personal setting: %q %v", data, err)
	}
	names, err := sentinelSkillNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		expected, err := snapshotConfig(filepath.Join(d.SentinelSkills.Repository, "vendor", "sentinel", "skills", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyConfigSnapshot(filepath.Join(kiro, "skills", name), expected); err != nil {
			t.Fatal("Kiro skill is not an exact upstream copy", name, err)
		}
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: selected, RemoveShared: []string{"tool.node"}})
	if _, err := os.Lstat(filepath.Join(kiro, "skills")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("skill removal left the Kiro skills folder setup created", err)
	}
	t.Log("installed Kiro CLI: pinned commands run without credentials, self-update off beside personal settings, Sentinel skills copied to and removed from Kiro")
	return commands["kiro-cli"]
}

// exerciseInstalledCopilotCLI runs the pinned Copilot CLI without credentials,
// with the managed shell's COPILOT_AUTO_UPDATE=false, and returns its
// entrypoint. The caller's Check proves the payload unchanged after use.
func exerciseInstalledCopilotCLI(t *testing.T, d *NativeDriver, home string) string {
	t.Helper()
	pin, ok := d.Archives.Pins["tool.copilot"]
	if !ok {
		t.Fatal("native runner has no Copilot CLI artifact")
	}
	command, err := d.Archives.CommandPath("tool.copilot", "copilot")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, "--version")
	cmd.Dir, cmd.Env, cmd.WaitDelay = home, agentCLIEnvironment(home), 2*time.Second
	output, err := agentCLIOutput(ctx, cmd)
	if err != nil || !strings.Contains(string(output), pin.Version) {
		t.Fatalf("pinned Copilot CLI did not run from its private payload: %v\n%s", err, output)
	}
	return command
}
