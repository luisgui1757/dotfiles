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
	"strconv"
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

func launched(_ application: NSRunningApplication) {
	print("LaunchServices started PID \(application.processIdentifier)")
	print("LaunchServices architecture: \(application.executableArchitecture)")
	print("LaunchServices executable: \((application.executableURL?.path ?? "missing").prefix(4096))")
	print("LaunchServices terminated at completion: \(application.isTerminated)")
	exit(0)
}
// Pure boundary classifier: no app, permission or Accessibility operation.
if CommandLine.arguments == [CommandLine.arguments[0], "--check-zen-consent"] {
	do {
		let input = try JSONDecoder().decode(ConsentInput.self, from: FileHandle.standardInput.readDataToEndOfFile())
		_ = try quarantineFields(input.quarantine, approved: false)
		guard consentButton(input.dialog) != nil else { throw consentError("Not the observed normal Zen prompt") }
		print("Observed normal Zen prompt"); exit(0)
	} catch { report(error as NSError); exit(2) }
}
guard CommandLine.arguments.count == 2 || (CommandLine.arguments.count == 3 && CommandLine.arguments[2] == "--zen-consent") else { exit(2) }
do {
	let environment = try JSONDecoder().decode([String: String].self,
		from: FileHandle.standardInput.readDataToEndOfFile())
	let consent = CommandLine.arguments.count == 3 ? try ZenFirstOpen(CommandLine.arguments[1]) : nil
	let configuration = NSWorkspace.OpenConfiguration()
	configuration.createsNewApplicationInstance = true
	configuration.environment = environment
	NSWorkspace.shared.openApplication(at: URL(fileURLWithPath: CommandLine.arguments[1]),
		configuration: configuration) { application, error in
		DispatchQueue.main.async {
			if let error = error { report(error as NSError); exit(1) }
			guard let application = application else { exit(3) }
			if let consent = consent { consent.completed = application }
			else { launched(application) }
		}
	}
	if let consent = consent {
		_ = Timer.scheduledTimer(withTimeInterval: 0.2, repeats: true) { _ in
			do { if let application = try consent.tick() { launched(application) } }
			catch { report(error as NSError); exit(5) }
		}
	}
	// Zen's first open waits for Gatekeeper and XProtect; degraded hosted
	// runners took about 50 s for it on 2026-10-28.
	RunLoop.main.run(until: Date(timeIntervalSinceNow: 120))
	if let error = consent?.inspection { report(error as NSError) }
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
	if err := os.WriteFile(source, []byte(desktopMacLaunchSource+desktopMacZenOpenSource), 0600); err != nil {
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
	for _, value := range desktopFixtureEnvironment(environment) {
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
	settings := profiles.Targets[vscodeSettingsResource][0].Path
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

// The consent probe is read-only: it never requests permissions or presses UI.
const desktopMacConsentSource = `import AppKit
import ApplicationServices

func emit(_ label: String, _ value: Any) {
	do {
		let data = try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys])
		print("\(label): \(String(decoding: data, as: UTF8.self))")
	} catch { print("Cannot encode \(label): \(error)") }
}
let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: false] as CFDictionary
let trusted = AXIsProcessTrustedWithOptions(options)
print("Existing Accessibility trust: \(trusted)")
guard let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
	print("CGWindow inventory unavailable"); exit(2)
}
let fields = [kCGWindowOwnerName, kCGWindowOwnerPID, kCGWindowLayer, kCGWindowIsOnscreen, kCGWindowBounds]
for window in windows.prefix(128) {
	var row: [String: Any] = [:]
	for key in fields { row[key as String] = window[key as String] }
	emit("Window", row)
}
let agentPath = "/System/Library/CoreServices/CoreServicesUIAgent.app"
guard let identifier = Bundle(path: agentPath)?.bundleIdentifier else {
	print("System consent agent bundle unavailable"); exit(2)
}
let agents = NSWorkspace.shared.runningApplications.filter { $0.bundleIdentifier == identifier }
var remaining = 128
func attribute(_ element: AXUIElement, _ name: String) -> CFTypeRef? {
	var value: CFTypeRef?
	let result = AXUIElementCopyAttributeValue(element, name as CFString, &value)
	if result != .success { print("AX read \(name): \(result.rawValue)"); return nil }
	return value
}
func dump(_ element: AXUIElement, _ depth: Int) {
	guard depth <= 10, remaining > 0 else { return }
	remaining -= 1
	var row = ["depth": String(depth)]
	for key in [kAXRoleAttribute, kAXSubroleAttribute, kAXTitleAttribute, kAXDescriptionAttribute, kAXValueAttribute, "AXIdentifier"] {
		if let value = attribute(element, key) as? String { row[key] = String(value.prefix(1024)) }
	}
	if let enabled = attribute(element, kAXEnabledAttribute) as? Bool { row["enabled"] = String(enabled) }
	var actions: CFArray?
	let actionResult = AXUIElementCopyActionNames(element, &actions)
	row["actionsResult"] = String(actionResult.rawValue)
	if let names = actions as? [String] { row["actions"] = names.prefix(16).joined(separator: ",") }
	emit("AX", row)
	if let children = attribute(element, kAXChildrenAttribute) as? [AXUIElement] {
		for child in children.prefix(128) { dump(child, depth + 1) }
	}
}
for agent in agents {
	print("Consent agent: pid=\(agent.processIdentifier) bundle=\(identifier) executable=\(agent.executableURL?.path ?? "missing")")
	guard trusted else { continue }
	let application = AXUIElementCreateApplication(agent.processIdentifier)
	if let windows = attribute(application, kAXWindowsAttribute) as? [AXUIElement] {
		for window in windows.prefix(8) { dump(window, 0) }
	}
}
print("Consent inventory complete; existing trust=\(trusted); agents=\(agents.count)")
`

func desktopMacZenConsentDiagnostics(t *testing.T, bundle string) {
	t.Helper()
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "consent.swift"), filepath.Join(dir, "consent")
	if err := os.WriteFile(source, []byte(desktopMacConsentSource), 0600); err != nil {
		t.Log("consent diagnostic source:", err)
		return
	}
	for _, args := range [][]string{
		{"/usr/bin/sw_vers"},
		{"/usr/bin/log", "show", "--last", "2m", "--style", "compact", "--predicate", `subsystem == "com.apple.syspolicy.exec" OR (process == "CoreServicesUIAgent" AND subsystem == "com.apple.launchservices")`},
		{"/usr/sbin/spctl", "--status"},
		{"/usr/sbin/spctl", "--assess", "--type", "execute", "-vv", bundle},
		{"/usr/bin/xattr", "-p", "com.apple.quarantine", bundle},
		{"/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", filepath.Join(bundle, "Contents", "Info.plist")},
		{"/usr/bin/xcrun", "swiftc", "-o", binary, source},
		{binary},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var output boundedCommandOutput
		command := exec.CommandContext(ctx, args[0], args[1:]...)
		command.Stdout, command.Stderr = &output, &output
		command.WaitDelay = 2 * time.Second
		err := command.Run()
		cancel()
		t.Logf("Zen consent diagnostic %s: %v%s", filepath.Base(args[0]), err, nativeDiagnostic(output.data.Bytes()))
	}
}

// Compare the signed vendor executable with the published wrapper after a real
// LaunchServices failure. These diagnostics cannot turn the failed GUI check green.
func desktopMacZenFailureLogs(t *testing.T, wrapper, published, packageRoot string, environment []string) {
	t.Helper()
	vendor := filepath.Join(filepath.Dir(wrapper), "Zen.app")
	for _, bundle := range []string{wrapper, vendor} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		profile := t.TempDir()
		command := exec.CommandContext(ctx, filepath.Join(bundle, "Contents", "MacOS", "zen"),
			"--headless", "--no-remote", "--profile", profile, "--screenshot", filepath.Join(profile, "page.png"), "about:blank")
		command.Env = desktopFixtureEnvironment(append(os.Environ(), environment...))
		command.WaitDelay = 2 * time.Second
		var output boundedCommandOutput
		command.Stdout, command.Stderr = &output, &output
		err := command.Run()
		cancel()
		desktopStopOwned(t, "", packageRoot)
		_, imageErr := os.Stat(filepath.Join(profile, "page.png"))
		t.Logf("Zen direct %s: run=%v screenshot=%v%s", filepath.Base(bundle), err, imageErr, nativeDiagnostic(output.data.Bytes()))
	}
	for _, launch := range []struct {
		name string
		args []string
	}{
		{"published", []string{"/usr/bin/open", published}},
		{"vendor", []string{"/usr/bin/open", vendor}},
		{"direct-wrapper", []string{filepath.Join(wrapper, "Contents", "MacOS", "zen"), "--no-remote", "--profile", t.TempDir(), "about:blank"}},
	} {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, launch.args[0], launch.args[1:]...)
			command.Env = desktopFixtureEnvironment(append(os.Environ(), environment...))
			log, err := os.Create(filepath.Join(t.TempDir(), "gui.log"))
			if err != nil {
				t.Fatal(err)
			}
			command.Stdout, command.Stderr = log, log
			err = command.Start()
			if err == nil {
				defer func() {
					desktopStopOwned(t, "", packageRoot)
					cancel()
					t.Logf("Zen %s process wait: %v", launch.name, command.Wait())
				}()
			}
			var ids []int
			var window bool
			var inspectErr error
			deadline := time.Now().Add(15 * time.Second)
			for err == nil && time.Now().Before(deadline) {
				ids, inspectErr = desktopOwnedProcesses(ctx, "", packageRoot)
				if inspectErr != nil {
					break
				}
				window, inspectErr = desktopOwnedWindow(ctx, "", ids)
				if inspectErr != nil || window {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			t.Logf("Zen GUI %s: start=%v owned=%v window=%v inspection=%v", launch.name, err, ids, window, inspectErr)
			if !window && inspectErr == nil && len(ids) > 0 {
				var output boundedCommandOutput
				sample := exec.CommandContext(ctx, "/usr/bin/sample", strconv.Itoa(ids[0]), "1", "10")
				sample.Stdout, sample.Stderr = &output, &output
				sampleErr := sample.Run()
				// sample ends with a long image inventory; keep the first thread
				// stacks, rather than that inventory, within the diagnostic bound.
				data := output.data.Bytes()
				if len(data) > 8192 {
					data = data[:8192]
				}
				t.Logf("Zen %s owned process sample: %v%s", launch.name, sampleErr, nativeDiagnostic(data))
			}
			if err := log.Close(); err != nil {
				t.Error(err)
			}
			output, readErr := readDesktopLauncherOutput(log.Name())
			t.Logf("Zen %s GUI output: %v%s", launch.name, readErr, nativeDiagnostic(output))
		}()
	}
}
