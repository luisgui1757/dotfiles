package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Every installer mutation below goes through the compiled public machine protocol. This
// fixture must run in its own disposable job: Windows uses real known folders,
// macOS registers user fonts, and Linux can provision native APT prerequisites.
func TestNativeDesktopInstalledApplicationLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_NATIVE_DESKTOP") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("requires explicit disposable GitHub-hosted desktop acceptance")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home = filepath.Join(home, "desktop home ü with spaces")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	target := nativePlatform(t)
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Data: filepath.Join(home, ".local", "share"), Zsh: home}
	stateDir := filepath.Join(home, ".local", "state", "dotfiles")
	if runtime.GOOS == "windows" {
		folders, err = DiscoverConfigFolders()
		if err != nil {
			t.Fatal(err)
		}
		stateDir, err = DiscoverStateDirectory(target.Context, folders)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(stateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("public desktop acceptance requires fresh installer state", stateDir, err)
	}
	// Keep the native folder identity on Windows. GUI apps do not read the
	// installer shell's PowerShell profile, nor invented APPDATA overrides.
	environment := []string{"HOME=" + folders.Home, "USERPROFILE=" + folders.Home, "XDG_CONFIG_HOME=" + folders.Config, "XDG_DATA_HOME=" + folders.Data, "XDG_STATE_HOME=" + filepath.Dir(stateDir), "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "ZDOTDIR=" + folders.Home, "BASH_ENV=", "ENV=", "NODE_OPTIONS=", "ELECTRON_RUN_AS_NODE=", "VSCODE_PORTABLE=", "GHOSTTY_RESOURCES_DIR=", "LD_LIBRARY_PATH="}
	if runtime.GOOS == "windows" {
		environment = append(environment, "APPDATA="+folders.AppData, "LOCALAPPDATA="+folders.LocalAppData)
	}
	invoke := desktopMachineFixture(t, repository, home, environment)
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(target)
	if err != nil {
		t.Fatal(err)
	}
	archives := &ArchiveDriver{Directory: filepath.Join(stateDir, "packages"), Pins: pins}
	var powerShell string
	if runtime.GOOS == "windows" {
		powerShell, err = exec.LookPath("powershell.exe")
		if err != nil {
			t.Fatal(err)
		}
	}
	published := &DesktopDriver{Target: target.Context, Folders: folders, Directory: stateDir, Archives: archives, PowerShell: powerShell}
	selected := []string{"vscode"}
	if runtime.GOOS == "windows" {
		selected = append(selected, "windows-terminal")
	}
	if runtime.GOOS != "windows" {
		selected = append(selected, "ghostty")
	}
	if runtime.GOOS == "darwin" {
		selected = append(selected, "aerospace")
	}
	before := map[string]configSnapshot{}
	for _, id := range selected {
		desktopID := "desktop." + id
		if id == "ghostty" {
			desktopID = "integration.ghostty"
		}
		recipe, err := published.recipe(desktopID)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(recipe.Folder, filepath.FromSlash(recipe.Destination))
		snapshot, err := snapshotConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if id == "vscode" && snapshot.Kind == "absent" {
			writeConfigFixture(t, path, "personal launcher fixture, preserve exact bytes\n")
			snapshot, err = snapshotConfig(path)
			if err != nil {
				t.Fatal(err)
			}
		}
		before[path] = snapshot
	}

	manifest, err := DefaultConfigManifest(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"config.ghostty", "config.aerospace"} {
		r, ok := catalog.Resource(id)
		if !ok || !r.Available(target.Context) {
			continue
		}
		paths, err := manifest.Resolve(catalog, target.Context, folders, repository, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			before[path.Destination], err = snapshotConfig(path.Destination)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	// A sibling personal file must survive setup, app use, update and removal.
	personal := filepath.Join(folders.Config, "dotfiles-native-personal.txt")
	if _, err := os.Lstat(personal); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("personal fixture collision", err)
	}
	writeConfigFixture(t, personal, "keep this unrelated desktop preference\n")
	apply := func(request Request) {
		t.Helper()
		for attempt := 0; attempt < 5; attempt++ {
			request.ExpectedPlan = ""
			state, err := LoadState(filepath.Join(stateDir, "state.json"), folders.Home)
			if err != nil {
				t.Fatal(err)
			}
			request.Retry = state.Transaction != nil
			preview := invoke(request)
			if preview.Status != "preview" || preview.Plan.ID == "" {
				t.Fatal("public desktop preview missing", preview)
			}
			var adopt []string
			for _, op := range preview.Plan.Operations {
				if op.Action == "pending" && op.Observed.Adoptable {
					adopt = append(adopt, op.Resource)
				}
			}
			if len(adopt) > 0 {
				request.Adopt = sortedUnique(append(request.Adopt, adopt...))
				continue
			}
			request.ExpectedPlan = preview.Plan.ID
			result := invoke(request)
			if result.Status == "ready" {
				return
			}
			if result.Status != "needs-action" {
				t.Fatal("public desktop operation failed", result)
			}
			t.Logf("public desktop prerequisite/recovery transition %d: %s", attempt+1, result.Message)
		}
		t.Fatal("desktop selection did not complete its bounded prerequisite transitions")
	}
	sharedRemoval := func() []string {
		t.Helper()
		state, err := LoadState(filepath.Join(stateDir, "state.json"), folders.Home)
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for id, receipt := range state.Receipts {
			r, ok := catalog.Resource(id)
			if ok && receipt.Ownership == "created" && !r.Retain && (r.Shared || r.Scope == "machine" || receipt.After.Scope == "machine") {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		return ids
	}
	// Singleton first, then the complete desktop group: dependency omissions must
	// not be hidden by another selected checkbox or a preprovisioned application.
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
	if !t.Run("vscode-singleton", func(t *testing.T) {
		desktopConsumePublished(t, published, archives, "vscode", home, environment)
	}) {
		t.FailNow()
	}
	apply(Request{Schema: 1, Mode: "update"})
	if check := invoke(Request{Schema: 1, Mode: "check"}); check.Status != "ready" {
		t.Fatal("desktop singleton check after use/update", check)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: selected})
	for _, id := range selected {
		if !t.Run(id, func(t *testing.T) {
			desktopConsumePublished(t, published, archives, id, home, environment)
		}) {
			t.FailNow()
		}
	}
	apply(Request{Schema: 1, Mode: "update"})
	if check := invoke(Request{Schema: 1, Mode: "check"}); check.Status != "ready" {
		t.Fatal("complete desktop check after use/update", check)
	}
	apply(Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: sharedRemoval()})
	for path, snapshot := range before {
		if err := verifyConfigSnapshot(path, snapshot); err != nil {
			t.Fatal("desktop removal did not restore personal configuration", path, err)
		}
	}

	data, err := os.ReadFile(personal)
	if err != nil || string(data) != "keep this unrelated desktop preference\n" {
		t.Fatal("personal sibling was not preserved", err)
	}
}

func desktopMachineFixture(t *testing.T, repository, home string, environment []string) func(Request) Result {
	t.Helper()
	binary := filepath.Join(home, "dotfiles")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/dotfiles")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build public desktop entrypoint: %v\n%s", err, output)
	}
	return func(request Request) Result {
		t.Helper()
		input, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, binary, "machine")
		command.Env = append(append(os.Environ(), environment...), "DOTFILES_CHECKOUT="+repository, "DOTFILES_ENTRYPOINT=setup", "TERM=xterm-256color")
		command.Stdin = bytes.NewReader(input)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, runErr := command.Output()
		var result Result
		if err := Decode(output, &result); err != nil {
			t.Fatalf("public desktop machine protocol: %v / %v\n%s\n%s", runErr, err, output, stderr.String())
		}
		if runErr != nil && result.Status != "needs-action" {
			t.Fatalf("public desktop command: %v\n%s", runErr, stderr.String())
		}
		return result
	}
}

func desktopConsumePublished(t *testing.T, d *DesktopDriver, a *ArchiveDriver, id, home string, environment []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		environment = append(slices.Clone(environment), "PATH="+filepath.Join(os.Getenv("SystemRoot"), "System32"))
	}
	desktopID := "desktop." + id
	if id == "ghostty" {
		desktopID = "integration.ghostty"
	}
	p, err := d.recipe(desktopID)
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(p.Folder, filepath.FromSlash(p.Destination))
	if _, err := os.Stat(launcher); err != nil {
		t.Fatal("published desktop launcher missing", launcher, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	packageRoot := filepath.Join(a.Directory, "tool."+id)
	witness := ""
	if id == "vscode" {
		witness = desktopVSCodeWitness(t, d.Folders, home)
	}
	t.Cleanup(func() { desktopFailureLogs(t, d.Folders, id) })
	args := []string{}
	switch runtime.GOOS {
	case "linux":
		args = []string{"/usr/bin/gio", "launch", launcher}
	case "darwin":
		args = []string{desktopMacLauncher(t, ctx), launcher}
		t.Cleanup(func() {
			if !t.Failed() {
				return
			}
			probe, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			output, err := exec.CommandContext(probe, "/usr/bin/codesign", "--verify", "--deep", "--strict", launcher).CombinedOutput()
			t.Logf("%s published bundle signature: %v %s", id, err, output)
		})
	case "windows":
		script := `$ErrorActionPreference='Stop';$shortcut=(New-Object -ComObject WScript.Shell).CreateShortcut(` + desktopPSQuote(launcher) + `);if($shortcut.TargetPath -cne ` + desktopPSQuote(p.Command) + `){throw 'Published shortcut target differs from managed application'};Start-Process -FilePath ` + desktopPSQuote(launcher) + ` | Out-Null`
		args = append([]string{d.PowerShell}, windowsVendorArguments(script)...)
	}
	before, err := desktopOwnedProcesses(ctx, d.PowerShell, packageRoot)
	if err != nil || len(before) > 0 {
		t.Fatal("native fixture found an already running private application", before, err)
	}
	defer desktopStopOwned(t, d.PowerShell, packageRoot)
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	command.Env = append(os.Environ(), environment...)
	command.Dir = home
	if runtime.GOOS == "darwin" {
		command.Stdin = desktopMacLaunchEnvironment(t, environment)
	}
	var output nativeOutput
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil || output.exceeded {
		t.Fatalf("published %s launcher failed: %v (output exceeded=%v)%s", id, err, output.exceeded, nativeDiagnostic(output.data.Bytes()))
	}
	if output.data.Len() != 0 {
		t.Logf("published %s launcher output:%s", id, nativeDiagnostic(output.data.Bytes()))
	}
	var owned []int
	window := false
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		owned, err = desktopOwnedProcesses(ctx, d.PowerShell, packageRoot)
		if err != nil {
			t.Fatal(err)
		}
		if len(owned) > 0 {
			window, err = desktopOwnedWindow(ctx, d.PowerShell, owned)
			if err != nil {
				t.Fatal(err)
			}
			if window || id == "aerospace" {
				break
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if len(owned) == 0 || !window && id != "aerospace" {
		t.Fatalf("published %s application did not map a native window; owned pids=%v", id, owned)
	}
	run := func(program string, args ...string) string {
		t.Helper()
		call := exec.CommandContext(ctx, program, args...)
		call.Env = append(os.Environ(), environment...)
		output, err := call.CombinedOutput()
		if err != nil {
			t.Fatalf("native %s configuration consumer: %v\n%s", id, err, output)
		}
		return string(output)
	}
	switch id {
	case "ghostty":
		cli, err := a.CommandPath("tool.ghostty", "ghostty")
		if err != nil {
			t.Fatal(err)
		}
		output := run(cli, "+show-config")
		if !strings.Contains(output, "font-family = Hack Nerd Font") || !strings.Contains(output, "theme = Rose Pine") {
			t.Fatal("Ghostty did not consume managed configuration", output)
		}
	case "vscode":
		deadline := time.Now().Add(30 * time.Second)
		for {
			data, err := os.ReadFile(witness)
			if err == nil {
				var result struct{ Theme, Font, TerminalFont string }
				if err := Decode(data, &result); err != nil {
					t.Fatal(err)
				}
				if result.Theme != "Rosé Pine" || !strings.Contains(result.Font, "Hack Nerd Font") || !strings.Contains(result.TerminalFont, "Hack Nerd Font") {
					t.Fatal("Code extension host did not consume managed settings", result)
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
				t.Fatal("Code extension host did not publish settings witness", err)
			}
			time.Sleep(250 * time.Millisecond)
		}
	case "aerospace":
		cli, err := a.CommandPath("tool.aerospace", "aerospace")
		if err != nil {
			t.Fatal(err)
		}
		call := exec.CommandContext(ctx, cli, "config", "--config-path")
		call.Env = append(os.Environ(), environment...)
		output, err := call.CombinedOutput()
		if err != nil {
			// Pinned v0.21.1-Beta initAppBundle.swift waits for Accessibility
			// approval before loading configuration or starting its server.
			// A generic command error is not evidence of that boundary.
			if !strings.Contains(string(output), "Can't connect to AeroSpace server. Is AeroSpace.app running?") {
				t.Fatalf("AeroSpace configuration query failed: %v\n%s", err, output)
			}
			stillRunning, inspectErr := desktopOwnedProcesses(ctx, d.PowerShell, packageRoot)
			if inspectErr != nil || len(stillRunning) == 0 {
				t.Fatal("AeroSpace exited before its consent/server boundary", inspectErr)
			}
			t.Log("AeroSpace Applications-link launch verified; configuration and tiling UNVERIFIED: native server has not started, and pinned startup waits for owner-granted Accessibility approval before loading configuration")
		} else {
			got, resolveErr := resolveConfigPath(strings.TrimSpace(string(output)))
			want, wantErr := resolveConfigPath(filepath.Join(d.Folders.Config, "aerospace", "aerospace.toml"))
			if resolveErr != nil || wantErr != nil || got != want {
				t.Fatal("AeroSpace did not consume the managed configuration", string(output), resolveErr, wantErr)
			}
			t.Log("AeroSpace loaded the managed configuration through its published Applications link; automatic tiling remains unverified")
		}
	}
}

func desktopVSCodeWitness(t *testing.T, folders ConfigFolders, home string) string {
	t.Helper()
	dir := filepath.Join(folders.Home, ".vscode", "extensions", "dotfiles-fixture.native-settings-1.0.0")
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fixture extension collision", err)
	}
	witness := filepath.Join(home, "Code settings witness.json")
	writeConfigFixture(t, filepath.Join(dir, "package.json"), `{"name":"native-settings","publisher":"dotfiles-fixture","version":"1.0.0","engines":{"vscode":"^1.90.0"},"activationEvents":["onStartupFinished"],"main":"./extension.js"}`)
	location, _ := json.Marshal(witness)
	writeConfigFixture(t, filepath.Join(dir, "extension.js"), `exports.activate=()=>{const vscode=require('vscode');const fs=require('fs');const c=vscode.workspace.getConfiguration();fs.writeFileSync(`+string(location)+`,JSON.stringify({Theme:c.get('workbench.colorTheme'),Font:c.get('editor.fontFamily'),TerminalFont:c.get('terminal.integrated.fontFamily')}));};`)
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return witness
}

func desktopOwnedProcesses(ctx context.Context, powerShell, root string) ([]int, error) {
	ids := []int{}
	switch runtime.GOOS {
	case "windows":
		script := `$ErrorActionPreference='Stop';$root=` + desktopPSQuote(root+string(filepath.Separator)) + `;@((Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -and $_.ExecutablePath.StartsWith($root,[StringComparison]::OrdinalIgnoreCase)} | ForEach-Object {[int]$_.ProcessId})) | ConvertTo-Json -Compress`
		output, err := exec.CommandContext(ctx, powerShell, windowsVendorArguments(script)...).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("inspect owned GUI processes: %w: %s", err, output)
		}
		// PowerShell pipeline serialization unwraps a single element unless -InputObject is used.
		text := strings.TrimSpace(string(output))
		if text == "" {
			return ids, nil
		}
		if strings.HasPrefix(text, "[") {
			err = json.Unmarshal(output, &ids)
		} else {
			var id int
			err = json.Unmarshal(output, &id)
			ids = append(ids, id)
		}
		return ids, err
	case "darwin":
		output, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,comm=").Output()
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			pid, path, ok := strings.Cut(line, " ")
			if !ok || !strings.HasPrefix(strings.TrimSpace(path), root+string(filepath.Separator)) {
				continue
			}
			id, err := strconv.Atoi(pid)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
	case "linux":
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			id, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			path, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
					continue
				}
				return nil, err
			}
			if strings.HasPrefix(path, root+string(filepath.Separator)) {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}
func desktopOwnedWindow(ctx context.Context, powerShell string, ids []int) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		names := []string{}
		for _, id := range ids {
			names = append(names, strconv.Itoa(id))
		}
		script := `$ErrorActionPreference='Stop';$found=$false;foreach($id in @(` + strings.Join(names, ",") + `)){$p=Get-Process -Id $id -ErrorAction SilentlyContinue;if($p -and $p.MainWindowHandle -ne 0){$found=$true}};if($found){'visible'}else{'absent'}`
		output, err := exec.CommandContext(ctx, powerShell, windowsVendorArguments(script)...).CombinedOutput()
		return strings.TrimSpace(string(output)) == "visible", err
	case "darwin":
		data, _ := json.Marshal(ids)
		script := `ObjC.import('CoreGraphics');var ids=` + string(data) + `;var windows=ObjC.deepUnwrap(ObjC.castRefToObject($.CGWindowListCopyWindowInfo($.kCGWindowListOptionAll,$.kCGNullWindowID)));JSON.stringify(windows.some(function(w){return ids.indexOf(w.kCGWindowOwnerPID)>=0&&w.kCGWindowLayer===0&&w.kCGWindowIsOnscreen&&w.kCGWindowBounds.Width>10&&w.kCGWindowBounds.Height>10;}));`
		output, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script).CombinedOutput()
		return strings.TrimSpace(string(output)) == "true", err
	case "linux":
		output, err := exec.CommandContext(ctx, "/usr/bin/xwininfo", "-root", "-tree").CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("inspect disposable X display: %w: %s", err, output)
		}
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 || !strings.HasPrefix(fields[0], "0x") {
				continue
			}
			property, err := exec.CommandContext(ctx, "/usr/bin/xprop", "-id", fields[0], "_NET_WM_PID").CombinedOutput()
			if err != nil {
				continue
			}
			_, value, ok := strings.Cut(string(property), "=")
			if !ok {
				continue
			}
			id, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil && slices.Contains(ids, id) {
				return true, nil
			}
		}
	}
	return false, nil
}
func desktopStopOwned(t *testing.T, powerShell, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for attempt := 0; attempt < 20; attempt++ {
		ids, err := desktopOwnedProcesses(ctx, powerShell, root)
		if err != nil {
			t.Error(err)
			return
		}
		if len(ids) == 0 {
			return
		}
		for _, id := range ids {
			process, err := os.FindProcess(id)
			if err != nil {
				if errors.Is(err, os.ErrProcessDone) {
					continue
				}
				t.Error(err)
				continue
			}
			if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Error("fixture-owned GUI processes did not exit before package mutation")
}
