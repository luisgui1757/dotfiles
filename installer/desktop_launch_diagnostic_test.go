package installer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Use the public LaunchServices completion API once, through the published
// application URL. Unlike open(1), it retains the underlying spawn error.
const desktopMacLaunchSource = `import AppKit
import Darwin

func report(_ error: NSError) {
	var chain: [[String: String]] = []
	var current: NSError? = error
	while let value = current, chain.count < 8 {
		var row = ["domain": value.domain, "code": String(value.code),
			"description": String(value.localizedDescription.prefix(1024))]
		for key in [NSLocalizedFailureReasonErrorKey, NSDebugDescriptionErrorKey, NSFilePathErrorKey] {
			if let detail = value.userInfo[key] as? String { row[key] = String(detail.prefix(1024)) }
		}
		chain.append(row)
		current = value.userInfo[NSUnderlyingErrorKey] as? NSError
	}
	do {
		let data = try JSONSerialization.data(withJSONObject: chain, options: [.sortedKeys])
		FileHandle.standardError.write(data)
		FileHandle.standardError.write(Data("\n".utf8))
	} catch {
		FileHandle.standardError.write(Data("Cannot encode native launch error: \(error)\n".utf8))
	}
}

guard CommandLine.arguments.count == 2 else { exit(2) }
do {
	let environment = try JSONDecoder().decode([String: String].self,
		from: FileHandle.standardInput.readDataToEndOfFile())
	let configuration = NSWorkspace.OpenConfiguration()
	configuration.createsNewApplicationInstance = true
	configuration.environment = environment
	NSWorkspace.shared.openApplication(at: URL(fileURLWithPath: CommandLine.arguments[1]),
		configuration: configuration) { application, error in
		if let error = error { report(error as NSError); exit(1) }
		guard let application = application else { exit(3) }
		print("LaunchServices started PID \(application.processIdentifier)")
		exit(0)
	}
	RunLoop.main.run(until: Date(timeIntervalSinceNow: 30))
	FileHandle.standardError.write(Data("LaunchServices completion timed out\n".utf8))
	exit(4)
} catch {
	report(error as NSError)
	exit(2)
}
`

func desktopMacLauncher(t *testing.T, ctx context.Context) string {
	t.Helper()
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "launch.swift"), filepath.Join(dir, "launch")
	if err := os.WriteFile(source, []byte(desktopMacLaunchSource), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "/usr/bin/xcrun", "swiftc", "-o", binary, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile LaunchServices fixture: %v%s", err, nativeDiagnostic(output))
	}
	return binary
}

func desktopMacLaunchEnvironment(t *testing.T, environment []string) io.Reader {
	t.Helper()
	values := map[string]string{}
	for _, value := range environment {
		key, data, ok := strings.Cut(value, "=")
		if !ok {
			t.Fatal("invalid desktop fixture environment")
		}
		values[key] = data
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReader(string(data))
}

func TestDesktopMacLaunchDiagnosticBuildsAndRejectsMissingInput(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS public application-launch API")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// No application URL is supplied: this validates the helper without opening
	// an application or changing desktop state on a contributor's workstation.
	command := exec.CommandContext(ctx, desktopMacLauncher(t, ctx))
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatal("missing launch URL accepted", string(output))
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatal("unexpected helper failure", err, string(output))
		}
	}
}

func desktopFailureLogs(t *testing.T, folders ConfigFolders, id string) {
	t.Helper()
	if !t.Failed() || id != "vscode" {
		return
	}
	if runtime.GOOS == "linux" {
		t.Logf("native desktop XDG_RUNTIME_DIR=%q DISPLAY=%q", os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("DISPLAY"))
		for _, path := range []string{"/proc/sys/kernel/apparmor_restrict_unprivileged_userns", "/proc/sys/kernel/unprivileged_userns_clone"} {
			data, err := os.ReadFile(path)
			t.Logf("native desktop %s: %q (%v)", path, data, err)
		}
	}
	profiles := &ProfileDriver{Targets: map[string][]ProfileTarget{}}
	configureVSCodeSettings(profiles, Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, folders)
	settings := profiles.Targets["integration.vscode"][0].Path
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(settings)), "logs", "*", "main.log"))
	if err != nil {
		t.Log("Code startup log discovery:", err)
		return
	}
	if len(paths) == 0 {
		t.Log("Code did not create a private main.log")
	}
	if len(paths) > 3 {
		paths = paths[len(paths)-3:]
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Log("Code startup log:", err)
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 64<<10))
		t.Logf("Code startup log %s: %v%s", filepath.Base(filepath.Dir(path)), errors.Join(readErr, file.Close()), nativeDiagnostic(data))
	}
}
