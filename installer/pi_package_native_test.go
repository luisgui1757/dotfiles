package installer

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Use actual Pi/Node/npm and native Git, with local registry/repository transport
// fixtures. This proves package features, not native Git provisioning. No
// third-party extension code, personal Git configuration or credentials run.
func TestNativeArchivePiPackageLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("requires verified upstream Pi and Node archives")
	}
	platform := nativePlatform(t)
	pins, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	driver := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}
	catalog := &Catalog{Schema: 1, Resources: []Resource{
		{ID: "runtime", Name: "Pi package runtime", Capability: true, Requires: []string{"tool.pi", "tool.node"}},
		{ID: "tool.pi", Name: "Pi", Action: "archive"},
		{ID: "tool.node", Name: "Node", Action: "archive"},
	}}
	c := Controller{Catalog: catalog, Context: platform.Context, Source: "pi-package-fixture", Home: home, StatePath: filepath.Join(home, "state.json"), Driver: driver}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"runtime"}})
	command, err := driver.CommandPath("tool.pi", "pi")
	if err != nil {
		t.Fatal(err)
	}
	node, err := driver.CommandPath("tool.node", "node")
	if err != nil {
		t.Fatal(err)
	}
	const name = "dotfiles-package-proof"
	const source = "npm:" + name + "@1.0.0"
	manifest := `{"name":"dotfiles-package-proof","version":"1.0.0","pi":{"extensions":["extension.ts"]}}`
	extension := `export default function (pi) {
	pi.registerCommand("dotfiles-package-proof", {
		description: "Bundled extension loader is operational",
		handler: async () => {},
	});
}
`
	tarball := archiveTar(t, []archiveFixtureEntry{
		{Name: "package/package.json", Text: manifest, Mode: 0644},
		{Name: "package/extension.ts", Text: extension, Mode: 0644},
	})
	hash := sha512.Sum512(tarball)
	var metadataRequests, tarballRequests atomic.Int32
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected registry mutation", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/" + name:
			metadataRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			metadata := map[string]any{"name": name, "dist-tags": map[string]string{"latest": "1.0.0"}, "versions": map[string]any{"1.0.0": map[string]any{"name": name, "version": "1.0.0", "dist": map[string]string{"tarball": server.URL + "/package.tgz", "integrity": "sha512-" + base64.StdEncoding.EncodeToString(hash[:])}}}}
			if err := json.NewEncoder(w).Encode(metadata); err != nil {
				t.Error(err)
			}
		case "/package.tgz":
			tarballRequests.Add(1)
			if _, err := w.Write(tarball); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	})
	server.Start()
	defer server.Close()
	environment := append(piFixtureEnvironment(home, filepath.Dir(node)),
		"NPM_CONFIG_REGISTRY="+server.URL, "NPM_CONFIG_CACHE="+filepath.Join(home, ".npm"),
		"NPM_CONFIG_USERCONFIG="+filepath.Join(home, "npm-user-config"), "NPM_CONFIG_GLOBALCONFIG="+filepath.Join(home, "npm-global-config"),
		"NPM_CONFIG_AUDIT=false", "NPM_CONFIG_FUND=false", "NPM_CONFIG_UPDATE_NOTIFIER=false")
	run := func(arguments ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, command, arguments...)
		cmd.Dir, cmd.Env = home, environment
		if len(arguments) > 0 && arguments[0] == "update" {
			// Offline mode intentionally skips package updates upstream. Allow
			// the explicit update to use our local registry/repository transport;
			// ordinary runtime startup stays offline.
			cmd.Env = append(append([]string{}, environment...), "PI_OFFLINE=0")
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Pi package command %v failed: %s %v", arguments, output, err)
		}
	}
	run("install", source)
	if metadataRequests.Load() == 0 || tarballRequests.Load() == 0 {
		t.Fatal("Pi did not execute npm against the fixture registry")
	}
	exercisePiRPCCommands(t, command, home, environment, nil, name, true)
	run("update", "--extensions")
	exercisePiRPCCommands(t, command, home, environment, nil, name, true)
	run("remove", source)
	exercisePiRPCCommands(t, command, home, environment, nil, name, false)

	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("package-consumer fixture requires native Git", err)
	}
	repository := filepath.Join(home, "git source")
	writeConfigFixture(t, filepath.Join(repository, "package.json"), manifest)
	writeConfigFixture(t, filepath.Join(repository, "extension.ts"), extension)
	// Rewrite only the fixture URL in subprocess-local Git configuration. Clone
	// and fetch still run through real Git without contacting an external host.
	uriPath := filepath.ToSlash(repository)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath}).String()
	const gitURL = "https://dotfiles-package.invalid/fixtures/proof"
	environment = append(environment,
		"PATH="+filepath.Dir(node)+string(os.PathListSeparator)+filepath.Dir(git),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(home, "git-global-config"),
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=url."+uri+".insteadOf", "GIT_CONFIG_VALUE_0="+gitURL)
	runGit := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, git, args...)
		cmd.Dir, cmd.Env = repository, environment
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git package fixture %v: %s %v", args, output, err)
		}
	}
	commit := func() {
		runGit("add", ".")
		runGit("-c", "user.name=Package fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Package fixture")
	}
	runGit("init", "-b", "main")
	commit()
	run("install", "git:"+gitURL)
	exercisePiRPCCommands(t, command, home, environment, nil, name, true)
	writeConfigFixture(t, filepath.Join(repository, "extension.ts"), strings.ReplaceAll(extension, name, name+"-updated"))
	commit()
	run("update", "--extensions")
	exercisePiRPCCommands(t, command, home, environment, nil, name+"-updated", true)
	run("remove", "git:"+gitURL)
	exercisePiRPCCommands(t, command, home, environment, nil, name+"-updated", false)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}
