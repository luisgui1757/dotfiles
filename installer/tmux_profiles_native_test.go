package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This covers the plugin/configuration layer against a real tmux executable.
// Native provisioning of tmux itself is a separate provider acceptance test.
func TestNativeArchiveTmuxPluginLifecyclePreservesStartupAndSessions(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" || runtime.GOOS == "windows" {
		t.Skip("requires real POSIX archives and tmux")
	}
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal("native tmux integration prerequisite is missing", err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "home with spaces and 'quotes' # $dollars")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	archives := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: pins}
	profiles := &ProfileDriver{Directory: filepath.Join(home, "state"), Targets: map[string][]ProfileTarget{}}
	if err := configureTmuxProfiles(profiles, Context{OS: runtime.GOOS}, ConfigFolders{Home: home}, repository, archives); err != nil {
		t.Fatal(err)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// The test's selected capability is the plugin layer, with tmux supplied by
	// this fixture. Each archive and the real profile publisher still goes
	// through the controller's production ownership and recovery paths.
	fixture := &Catalog{Schema: 1, Resources: []Resource{{ID: "plugins", Name: "Plugin layer", Capability: true, Platforms: []string{"darwin", "linux"}, Requires: []string{"tmux.plugins"}}}}
	for _, id := range []string{"tmux.plugins", "tmux.sensible", "tmux.yank", "tmux.resurrect", "tmux.continuum"} {
		r, ok := catalog.Resource(id)
		if !ok {
			t.Fatal("missing plugin resource", id)
		}
		if id == "tmux.plugins" {
			r.Requires = []string{"tmux.sensible", "tmux.yank", "tmux.resurrect", "tmux.continuum"}
		}
		fixture.Resources = append(fixture.Resources, r)
	}
	state := filepath.Join(home, "state", "state.json")
	driver := &NativeDriver{Catalog: fixture, Context: nativePlatform(t).Context, Archives: archives, Profiles: profiles, StatePath: state}
	c := Controller{Catalog: fixture, Context: driver.Context, Source: "native-tmux-plugins", Home: home, StatePath: state, Driver: driver}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"plugins"}})
	for _, variant := range []string{"main", "moon", "dawn"} {
		data, err := os.ReadFile(filepath.Join(repository, "tmux", "rose-pine."+variant+".conf"))
		if err != nil {
			t.Fatal(err)
		}
		writeConfigFixture(t, filepath.Join(home, ".tmux.rose-pine."+variant+".conf"), string(data))
	}
	startup := filepath.Join(home, "Library", "LaunchAgents", "Tmux.Start.plist")
	writeConfigFixture(t, startup, "personal startup configuration\n")
	commands := filepath.Join(home, "fixture-bin")
	serviceCalls := filepath.Join(home, "service-calls")
	service := filepath.Join(commands, "systemctl")
	writeConfigFixture(t, service, "#!/bin/sh\nprintf '%s\\n' \"$*\" >>"+shellLiteral(serviceCalls)+"\nexit 0\n")
	if err := os.Chmod(service, 0755); err != nil {
		t.Fatal(err)
	}
	socketDir, err := os.MkdirTemp("/tmp", "dotfiles-tmux-native-")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(socketDir, "socket")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	invoke := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, tmux, append([]string{"-S", socket}, args...)...)
		cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "TMUX=", "TERM=xterm-256color", "PATH="+commands+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	t.Cleanup(func() {
		cmd := exec.Command(tmux, "-S", socket, "kill-server")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("stop fixture server: %v\n%s", err, output)
		}
		if err := os.RemoveAll(socketDir); err != nil {
			t.Error(err)
		}
	})
	invoke("-f", filepath.Join(repository, "tmux", "tmux.conf"), "new-session", "-d", "-s", "preserved", "/bin/sh")
	invoke("source-file", filepath.Join(home, ".tmux.plugins.conf"))
	invoke("send-keys", "-t", "preserved", "printf 'saved-pane-marker\\n'", "Enter")
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(invoke("capture-pane", "-p", "-t", "preserved"), "saved-pane-marker") {
		if time.Now().After(deadline) {
			t.Fatal("fixture pane did not produce its saved contents")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if data, err := os.ReadFile(startup); err != nil || string(data) != "personal startup configuration\n" {
		t.Fatal("plugin loading changed personal startup configuration", err)
	}
	save := invoke("show-option", "-gqv", "@resurrect-save-script-path")
	if !strings.Contains(save, "tmux.resurrect") {
		t.Fatal("resurrect did not activate", save)
	}
	if status := invoke("show-option", "-gqv", "status-right"); !strings.Contains(status, "continuum_save.sh") {
		t.Fatal("automatic session saving did not activate", status)
	}
	backupDir := filepath.Join(home, ".local", "share", "tmux", "resurrect")
	for i := 0; i < 7; i++ {
		path := filepath.Join(backupDir, fmt.Sprintf("tmux_resurrect_200001%02dT000000.txt", i+1))
		writeConfigFixture(t, path, "older saved session\n")
		old := time.Now().Add(-time.Duration(100-i) * 24 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	// Re-run the command serialized by tmux's actual save binding through its
	// own configuration parser; do not bypass a broken binding by invoking the
	// script with a separately corrected path.
	command := ""
	for _, line := range strings.Split(invoke("list-keys", "-T", "prefix"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[3] == "C-s" && fields[4] == "run-shell" {
			_, command, _ = strings.Cut(line, "run-shell ")
		}
	}
	if command == "" {
		t.Fatal("session save binding is absent")
	}
	saveBinding := filepath.Join(home, "save-binding.conf")
	writeConfigFixture(t, saveBinding, "run-shell "+command+"\n")
	invoke("source-file", saveBinding)
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "tmux", "resurrect", "last")); err != nil {
		t.Fatal("session save did not create recoverable personal data", err)
	}
	savedSession, err := os.ReadFile(filepath.Join(backupDir, "last"))
	if err != nil || !strings.Contains(string(savedSession), "pane\tpreserved\t") {
		t.Fatal("snapshot lacks the original session", string(savedSession), err)
	}
	for i := 0; i < 7; i++ {
		path := filepath.Join(backupDir, fmt.Sprintf("tmux_resurrect_200001%02dT000000.txt", i+1))
		_, err := os.Stat(path)
		if i < 3 && !os.IsNotExist(err) || i >= 3 && err != nil {
			t.Fatal("backup retention did not preserve the newest five snapshots", i, err)
		}
	}
	serverPID, err := strconv.Atoi(invoke("display-message", "-p", "#{pid}"))
	if err != nil || serverPID <= 0 {
		t.Fatal("cannot identify the fixture server before shutdown", serverPID, err)
	}
	server, err := os.FindProcess(serverPID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Release(); err != nil {
			t.Error(err)
		}
	})
	invoke("kill-server")
	// kill-server acknowledges the request before the daemon necessarily exits.
	// Reconnecting immediately can attach the new-session client to that dying
	// server. Require actual process exit before proving a fresh startup.
	deadline = time.Now().Add(5 * time.Second)
	for {
		err := server.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil || time.Now().After(deadline) {
			t.Fatal("fixture server did not finish shutdown before restart", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	invoke("-f", filepath.Join(repository, "tmux", "tmux.conf"), "new-session", "-d", "-s", "bootstrap", "/bin/sh")
	deadline = time.Now().Add(10 * time.Second)
	for {
		cmd := exec.CommandContext(ctx, tmux, "-S", socket, "has-session", "-t", "preserved")
		if err := cmd.Run(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			restore := invoke("show-option", "-gqv", "@resurrect-restore-script-path")
			restoreLog := filepath.Join(home, "restore.log")
			t.Logf("saved session: %s", savedSession)
			invoke("run-shell", shellLiteral(restore)+" >"+shellLiteral(restoreLog)+" 2>&1")
			data, readErr := os.ReadFile(restoreLog)
			t.Logf("restore diagnosis: %s (%v)", data, readErr)
			t.Fatal("automatic restoration did not recreate the saved session")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for {
		pane := invoke("capture-pane", "-p", "-t", "preserved")
		if strings.Contains(pane, "saved-pane-marker") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restoration lost the saved pane contents", pane)
		}
		time.Sleep(25 * time.Millisecond)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
	invoke("new-session", "-d", "-s", "automatically-saved", "/bin/sh")
	invoke("set-option", "-g", "@continuum-save-last-timestamp", "0")
	status := invoke("show-option", "-gqv", "status-right")
	hook, _, found := strings.Cut(strings.TrimPrefix(status, "#("), ")")
	if !found || !strings.HasPrefix(status, "#(") || !strings.Contains(hook, "continuum_save.sh") {
		t.Fatal("cannot execute the installed automatic-save hook", status)
	}
	invoke("run-shell", hook)
	deadline = time.Now().Add(10 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Join(backupDir, "last"))
		if err == nil && strings.Contains(string(data), "pane\tautomatically-saved\t") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic-save hook did not persist the updated sessions", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(filepath.Join(home, ".tmux.plugins.conf")); !os.IsNotExist(err) {
		t.Fatal("plugin removal left its managed loader", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "tmux", "resurrect", "last")); err != nil {
		t.Fatal("plugin removal deleted personal saved sessions", err)
	}
	invoke("has-session", "-t", "preserved")
	if _, err := os.Stat(serviceCalls); !os.IsNotExist(err) {
		t.Fatal("plugin integration invoked unrelated service management", err)
	}
	if data, err := os.ReadFile(startup); err != nil || string(data) != "personal startup configuration\n" {
		t.Fatal("plugin lifecycle changed personal startup configuration", err)
	}
}
