package terminal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	engine "github.com/luisgui1757/dotfiles/installer"
)

// This consumes the actual pinned runtime and canonical theme/keybinding files
// in a PTY/ConPTY. Package provisioning and settings restoration have separate
// controller lifecycle tests; this fixture does not claim those dependencies.
func TestNativeArchivePiThemeInTerminal(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("requires verified upstream archive and native terminal execution")
	}
	platform, err := engine.DiscoverPlatform()
	if err != nil {
		t.Fatal(err)
	}
	pins, err := engine.DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	driver := &engine.ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{
		{ID: "pi", Name: "Pi runtime fixture", Capability: true, Requires: []string{"tool.pi"}},
		{ID: "tool.pi", Name: "Pi", Action: "archive"},
	}}
	c := engine.Controller{Catalog: catalog, Context: platform.Context, Source: "pi-terminal-fixture", Home: home, StatePath: filepath.Join(home, "state.json"), Driver: driver}
	request := engine.Request{Schema: 1, Mode: "apply", Selected: []string{"pi"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil || preview.Status != "preview" {
		t.Fatal("Pi terminal fixture preview", preview, err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if result, err := c.Dispatch(context.Background(), request); err != nil || result.Status != "ready" {
		t.Fatal("Pi terminal fixture installation", result, err)
	}
	command, err := driver.CommandPath("tool.pi", "pi")
	if err != nil {
		t.Fatal(err)
	}
	agent := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(filepath.Join(agent, "themes"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rose-pine", "rose-pine-moon", "rose-pine-dawn", "keybindings"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "pi", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(agent, "themes", name+".json")
		if name == "keybindings" {
			destination = filepath.Join(agent, "keybindings.json")
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(agent, "settings.json"), []byte(`{"theme":"rose-pine","quietStartup":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	extension := filepath.Join(home, "theme-proof.ts")
	if err := os.WriteFile(extension, []byte(`export default function (pi) {
	pi.on("session_start", async (_event, ctx) => {
		ctx.ui.notify("DOTFILES_THEME_READY");
	});
	pi.registerCommand("dotfiles-theme-proof", {
		handler: async (_args, ctx) => {
			if (!ctx.hasUI || ctx.ui.theme.name !== "rose-pine") throw new Error("Initial theme is not active");
			const names = ctx.ui.getAllThemes().map(theme => theme.name);
			for (const name of ["rose-pine", "rose-pine-moon", "rose-pine-dawn"]) {
				if (!names.includes(name) || !ctx.ui.getTheme(name)) throw new Error("Missing theme: " + name);
				if (!ctx.ui.setTheme(name).success || ctx.ui.theme.name !== name) throw new Error("Theme switch failed: " + name);
				const expected = name === "rose-pine-dawn" ? "38;2;87;82;121m" : "38;2;224;222;244m";
				if (!ctx.ui.theme.fg("text", "proof").includes(expected)) throw new Error("Wrong rendered text color: " + name);
			}
			if (!ctx.ui.setTheme("rose-pine").success) throw new Error("Theme restoration failed");
			ctx.ui.notify("DOTFILES_THEME_PASSED");
		},
	});
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES_TEST_PI_COMMAND", command)
	t.Setenv("DOTFILES_TEST_PI_HOME", home)
	s := startNativeSession(t, "pi-theme")
	s.waitFor(t, "DOTFILES_THEME_READY")
	s.send(t, "/dotfiles-theme-proof\r")
	s.waitFor(t, "DOTFILES_THEME_PASSED")
	s.send(t, "\x04")
	s.waitFor(t, "DOTFILES_PI_EXITED")
}

func testPiThemeChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	home := os.Getenv("DOTFILES_TEST_PI_HOME")
	cmd := exec.CommandContext(ctx, os.Getenv("DOTFILES_TEST_PI_COMMAND"), "--no-session", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-mcp", "-e", filepath.Join(home, "theme-proof.ts"))
	cmd.Dir = home
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = []string{"HOME=" + home, "USERPROFILE=" + home, "PI_CODING_AGENT_DIR=" + filepath.Join(home, ".pi", "agent"), "PI_OFFLINE=1", "PATH=" + filepath.Join(home, "empty-bin"), "TERM=xterm-256color", "COLORTERM=truecolor"}
	for _, key := range []string{"SystemRoot", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	if err := cmd.Run(); err != nil {
		t.Fatal("Pi interactive theme lifecycle", err)
	}
	if _, err := os.Stdout.WriteString("\n\nDOTFILES_PI_EXITED\n"); err != nil {
		t.Fatal(err)
	}
}
