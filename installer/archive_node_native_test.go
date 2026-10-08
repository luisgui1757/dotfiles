package installer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Foreign archives are decoded and hash-verified only; their programs never run.
func TestNativeNodeGypAllPinnedSources(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NODE_PIN_SOURCES") != "1" {
		t.Skip("opt in to checksum-verified all-target source proof")
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"-"+target.Arch, func(t *testing.T) {
			pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
			if err != nil {
				t.Fatal(err)
			}
			pin := pins["tool.node"]
			replacement := pin.Replacements[0]
			root, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			payload := filepath.Join(root, "payload")
			// Foreign executable mode semantics are not native runtime proof.
			// Keep checksum/layout/replacement checks; require the source itself.
			sourcePin := pin
			sourcePin.Commands, sourcePin.BinDirs = nil, nil
			sourcePin.RequiredFiles = []string{replacement.File}
			if err := downloadArchive(t.Context(), nil, sourcePin, payload); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(payload, filepath.FromSlash(replacement.File)))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(data), replacement.After) != 1 {
				t.Fatal("prepared gyp import boundary missing")
			}
			if target.OS == "windows" {
				if _, err := os.Stat(filepath.Join(payload, "include", "node", "common.gypi")); !os.IsNotExist(err) {
					t.Fatal("revisit Windows direct-gyp fixture: header contract changed", err)
				}
			}
			original := strings.Replace(string(data), replacement.After, replacement.Before, 1)
			t.Logf("archive=%s file=%s original=%x prepared=%x", pin.SHA256, replacement.File, sha256.Sum256([]byte(original)), sha256.Sum256(data))
		})
	}
}

// This changes only disposable private archives/projects. POSIX Node archives
// include headers for a real offline npm/node-gyp install. Windows ZIPs omit
// headers, so that lane executes the same vendored gyp import entrypoint directly.
func TestNativeNodeConsumerPreservesPayloadAndRemoval(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LANGUAGE_PREPARATION") != "1" {
		t.Skip("opt in to private pinned Node/Python consumer proof")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	c, d, _ := archiveController(t)
	d.Pins["tool.shared"], d.Client = pins["tool.node"], nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	pythonRoot := filepath.Join(c.Home, "python")
	if err := downloadArchive(t.Context(), nil, pins["tool.python"], pythonRoot); err != nil {
		t.Fatal(err)
	}
	pythonName := "python3"
	if runtime.GOOS == "windows" {
		pythonName = "python"
	}
	python := filepath.Join(pythonRoot, filepath.FromSlash(pins["tool.python"].Commands[pythonName]))
	personal := filepath.Join(c.Home, "personal-python.py")
	personalBytes := []byte("raise RuntimeError('personal interactive startup stays untouched')\n")
	writeConfigFixture(t, personal, string(personalBytes))
	t.Setenv("PYTHONSTARTUP", personal)
	project := filepath.Join(c.Home, "ordinary npm project")
	writeConfigFixture(t, filepath.Join(project, "package.json"), `{"name":"private-gyp-proof","version":"1.0.0","private":true}`)
	writeConfigFixture(t, filepath.Join(project, "binding.gyp"), `{"targets":[{"target_name":"proof","type":"none"}]}`)
	before, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	node, err := d.CommandPath("tool.shared", "node")
	if err != nil {
		t.Fatal(err)
	}
	npm := filepath.Join(payload, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	cmd := exec.CommandContext(ctx, node, npm, "install", "--offline", "--no-audit", "--no-fund", "--nodedir="+payload)
	if runtime.GOOS == "windows" {
		gyp := filepath.Join(payload, "node_modules", "npm", "node_modules", "node-gyp", "gyp", "gyp_main.py")
		cmd = exec.CommandContext(ctx, python, gyp, "--help")
	}
	cmd.Dir = project
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "PYTHON") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	for _, name := range []string{"HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "TEMP", "TMP"} {
		cmd.Env = append(cmd.Env, name+"="+c.Home)
	}
	cmd.Env = append(cmd.Env, "PATH="+filepath.Dir(node)+string(os.PathListSeparator)+filepath.Dir(python)+string(os.PathListSeparator)+os.Getenv("PATH"), "PYTHON="+python, "npm_config_cache="+filepath.Join(c.Home, "npm-cache"), "npm_config_userconfig="+filepath.Join(c.Home, "empty-user-npmrc"), "npm_config_globalconfig="+filepath.Join(c.Home, "empty-global-npmrc"), "npm_config_update_notifier=false")
	var output boundedCommandOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("Node consumer: %v\n%s", err, output.data.String())
	}
	after, err := snapshotTree(payload, maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		t.Fatal("ordinary consumer changed immutable Node", before, after, err)
	}
	t.Logf("consumer=%s Node snapshot=%s unchanged; output=%s", map[bool]string{true: "gyp entrypoint", false: "offline npm install"}[runtime.GOOS == "windows"], before.Hash, output.data.String())
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("consumer invalidated archive check", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Stat(payload); !os.IsNotExist(err) {
		t.Fatal("owned Node survived removal", err)
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != string(personalBytes) || os.Getenv("PYTHONSTARTUP") != personal {
		t.Fatal("personal Python settings changed", fmt.Sprintf("%q", data), err)
	}
}
