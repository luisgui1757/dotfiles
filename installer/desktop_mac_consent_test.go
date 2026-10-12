package installer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This test-only helper uses the hosted runner's existing Accessibility grant.
// It never requests a grant or writes policy/quarantine. The exact dialog shape
// was observed on macOS 26.6.2/25G83 in native job 118261425211.
const desktopMacZenOpenSource = `import ApplicationServices

struct ConsentNode: Codable {
	var role, title, value, identifier, description: String
	var enabled: Bool
	var actions: [String]
}
struct ConsentDialog: Codable {
	var role, subrole: String
	var children: [ConsentNode]
}
struct ConsentInput: Codable { var quarantine: String; var dialog: ConsentDialog }
func consentError(_ text: String) -> NSError {
	NSError(domain: "DotfilesNativeZenConsent", code: 1, userInfo: [NSLocalizedDescriptionKey: text])
}
func quarantineFields(_ text: String, approved: Bool) throws -> [String] {
	let fields = text.components(separatedBy: ";")
	guard fields.count == 4, let flags = UInt32(fields[0], radix: 16),
		flags & 1 != 0, (flags & 0x40 != 0) == approved,
		UInt64(fields[1], radix: 16) != nil, fields[2] == "Homebrew\\x20Cask",
		UUID(uuidString: fields[3]) != nil else { throw consentError("Unexpected Zen quarantine record") }
	return fields
}
func consentButton(_ dialog: ConsentDialog) -> Int? {
	guard dialog.role == "AXWindow", dialog.subrole == "AXSystemDialog", dialog.children.count == 6 else { return nil }
	var question = 0, origin = 0, image = 0, help = 0, cancel = 0
	var open: Int?
	for (index, node) in dialog.children.enumerated() {
		guard node.enabled else { return nil }
		switch node.role {
		case "AXStaticText":
			if node.value == "“Zen” is an app downloaded from the Internet. Are you sure you want to open it?" {
				question += 1
			} else {
				let text = node.value.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
				let pattern = #"^Homebrew Cask downloaded this file (today|yesterday) at ([1-9]|1[0-2]):[0-5][0-9] (AM|PM)\. Apple checked it for malicious software and none was detected\.$"#
				guard text.range(of: pattern, options: .regularExpression) != nil else { return nil }
				origin += 1
			}
		case "AXImage":
			guard node.description == "alert" else { return nil }; image += 1
		case "AXButton":
			guard node.actions == ["AXPress"] else { return nil }
			if node.title == "Open", node.identifier == "action-button-1", open == nil { open = index }
			else if node.title == "Cancel", node.identifier == "action-button-3" { cancel += 1 }
			else if node.title.isEmpty, node.description == "Help" { help += 1 }
			else { return nil }
		default: return nil
		}
	}
	return question == 1 && origin == 1 && image == 1 && help == 1 && cancel == 1 ? open : nil
}
func consentAttribute(_ element: AXUIElement, _ name: String) throws -> CFTypeRef? {
	var value: CFTypeRef?
	let status = AXUIElementCopyAttributeValue(element, name as CFString, &value)
	switch status {
	case .success: return value
	case .noValue, .attributeUnsupported: return nil // Optional attributes on the observed leaf elements.
	default: throw consentError("AX read \(name) failed: \(status.rawValue)")
	}
}
func consentText(_ element: AXUIElement, _ name: String) throws -> String {
	guard let value = try consentAttribute(element, name) else { return "" }
	guard let text = value as? String else { throw consentError("Non-text AX attribute \(name)") }
	return text
}
func consentWindow(_ window: AXUIElement) throws -> (ConsentDialog, [AXUIElement]) {
	let role = try consentText(window, kAXRoleAttribute)
	let subrole = try consentText(window, kAXSubroleAttribute)
	let raw = try consentAttribute(window, kAXChildrenAttribute)
	guard let raw = raw else { return (ConsentDialog(role: role, subrole: subrole, children: []), []) }
	guard let children = raw as? [AXUIElement] else { throw consentError("Invalid consent-window children") }
	// XProtect may show a progress window first. Inspect only the observed flat
	// six-child prompt; every other shape waits without action until timeout.
	guard children.count == 6 else { return (ConsentDialog(role: role, subrole: subrole, children: []), []) }
	var nodes: [ConsentNode] = []
	for child in children {
		var actions: CFArray?
		let status = AXUIElementCopyActionNames(child, &actions)
		guard status == .success, let names = actions as? [String], names.count <= 8 else {
			throw consentError("Cannot inspect consent actions: \(status.rawValue)")
		}
		nodes.append(try ConsentNode(role: consentText(child, kAXRoleAttribute), title: consentText(child, kAXTitleAttribute),
			value: consentText(child, kAXValueAttribute), identifier: consentText(child, "AXIdentifier"),
			description: consentText(child, kAXDescriptionAttribute), enabled: consentAttribute(child, kAXEnabledAttribute) as? Bool ?? false, actions: names))
	}
	return (ConsentDialog(role: role, subrole: subrole, children: nodes), children)
}
struct ConsentProcess: Equatable { var pid: pid_t; var seconds, micros: UInt64 }
final class ZenFirstOpen {
	let bundle = "/Applications/Zen.app"
	let executable = "/Applications/Zen.app/Contents/MacOS/zen"
	let agentExecutable = "/System/Library/CoreServices/CoreServicesUIAgent.app/Contents/MacOS/CoreServicesUIAgent"
	let original: [String]
	var held: ConsentProcess?
	var pressed = false
	var completed: NSRunningApplication?
	var inspection: Error?
	init(_ path: String) throws {
		let environment = ProcessInfo.processInfo.environment
		guard path == bundle, environment["DOTFILES_NATIVE_DESKTOP"] == "1",
			environment["GITHUB_ACTIONS"] == "true", environment["RUNNER_ENVIRONMENT"] == "github-hosted" else {
			throw consentError("Zen consent requires the explicit disposable hosted fixture")
		}
		let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: false] as CFDictionary
		guard AXIsProcessTrustedWithOptions(options) else { throw consentError("No existing Accessibility grant; no permission requested") }
		original = try quarantineFields(Self.quarantine(path), approved: false)
		guard try processes().isEmpty, try windows().isEmpty else { throw consentError("Pre-existing Zen process or consent-agent window") }
	}
	static func quarantine(_ path: String) throws -> String {
		let size = getxattr(path, "com.apple.quarantine", nil, 0, 0, XATTR_NOFOLLOW)
		guard size > 0, size <= 1024 else { throw consentError("Cannot read bounded Zen quarantine: \(errno)") }
		var bytes = [UInt8](repeating: 0, count: size)
		guard getxattr(path, "com.apple.quarantine", &bytes, bytes.count, 0, XATTR_NOFOLLOW) == size,
			let text = String(bytes: bytes, encoding: .utf8) else { throw consentError("Zen quarantine changed or is invalid") }
		return text
	}
	func windows() throws -> [AXUIElement] {
		let agents = NSWorkspace.shared.runningApplications.filter { $0.bundleIdentifier == "com.apple.coreservices.uiagent" }
		guard agents.count <= 1 else { throw consentError("Multiple consent agents") }
		guard let agent = agents.first else { return [] }
		guard agent.executableURL?.path == agentExecutable else { throw consentError("Unexpected consent-agent executable") }
		let element = AXUIElementCreateApplication(agent.processIdentifier)
		guard AXUIElementSetMessagingTimeout(element, 1) == .success,
			let windows = try consentAttribute(element, kAXWindowsAttribute) as? [AXUIElement], windows.count <= 8 else {
			throw consentError("Cannot inspect bounded consent-agent windows")
		}
		return windows
	}
	func processes() throws -> [ConsentProcess] {
		var ids = [pid_t](repeating: 0, count: 16384)
		let size = ids.count * MemoryLayout<pid_t>.size
		let bytes = proc_listpids(UInt32(PROC_UID_ONLY), getuid(), &ids, Int32(size))
		guard bytes > 0, bytes < size, Int(bytes) % MemoryLayout<pid_t>.size == 0 else { throw consentError("Incomplete native process inventory") }
		var found: [ConsentProcess] = []
		for pid in ids.prefix(Int(bytes) / MemoryLayout<pid_t>.size) where pid > 0 {
			var path = [CChar](repeating: 0, count: 4096)
			let count = proc_pidpath(pid, &path, UInt32(path.count))
			if count <= 0 {
				if errno == ESRCH || errno == ENOENT { continue }
				throw consentError("Cannot inspect native process \(pid): \(errno)")
			}
			guard String(cString: path) == executable else { continue }
			var info = proc_bsdinfo()
			let needed = MemoryLayout<proc_bsdinfo>.size
			guard proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, Int32(needed)) == needed else { throw consentError("Cannot bind Zen process lifetime") }
			found.append(ConsentProcess(pid: pid, seconds: info.pbi_start_tvsec, micros: info.pbi_start_tvusec))
		}
		return found
	}
	func tick() throws -> NSRunningApplication? {
		if let application = completed {
			guard pressed, let held = held, application.processIdentifier == held.pid,
				application.executableURL?.path == executable, application.executableArchitecture == 16777228,
				!application.isTerminated, try processes().contains(held) else { throw consentError("Launch completed without matching consent/process") }
			guard try windows().isEmpty else { return nil }
			let after = try quarantineFields(Self.quarantine(bundle), approved: true)
			guard Array(after.dropFirst()) == Array(original.dropFirst()) else { throw consentError("Zen quarantine provenance changed") }
			print("Zen ordinary Open consent: AXPress once through existing runner Accessibility grant; same PID \(held.pid); macOS user-approved flag observed")
			return application
		}
		if pressed { return nil }
		let candidates = try processes()
		guard candidates.count <= 1 else { throw consentError("Multiple candidate Zen processes") }
		guard let candidate = candidates.first else { return nil }
		if let prior = held, prior != candidate { throw consentError("Zen process lifetime changed before consent") }
		held = candidate
		let window: AXUIElement
		let dialog: ConsentDialog
		let children: [AXUIElement]
		do {
			let observed = try windows()
			guard observed.count == 1 else { return nil }
			window = observed[0]
			(dialog, children) = try consentWindow(window)
		} catch {
			// A progress window can disappear during an AX read. No action has
			// happened; keep the first error for the bounded timeout diagnostic.
			if inspection == nil { inspection = error }
			return nil
		}
		guard let index = consentButton(dialog) else { return nil } // Never act on a progress, security or permission dialog.
		guard let inventory = CGWindowListCopyWindowInfo(.optionOnScreenOnly, kCGNullWindowID) as? [[String: Any]] else { throw consentError("Cannot inspect held Zen windows") }
		guard !inventory.contains(where: { ($0[kCGWindowOwnerPID as String] as? Int) == Int(candidate.pid) && ($0[kCGWindowLayer as String] as? Int) == 0 }) else { throw consentError("Zen already has a window before first consent") }
		let currentWindows = try windows()
		guard currentWindows.count == 1, try processes() == [candidate],
			try Self.quarantine(bundle) == original.joined(separator: ";") else { throw consentError("Consent target changed before action") }
		let current = currentWindows[0]
		guard CFEqual(current, window) else { throw consentError("Consent window changed before action") }
		let (currentDialog, currentChildren) = try consentWindow(current)
		guard consentButton(currentDialog) == index, CFEqual(children[index], currentChildren[index]) else { throw consentError("Consent button changed before action") }
		pressed = true
		let status = AXUIElementPerformAction(currentChildren[index], kAXPressAction as CFString)
		guard status == .success else { throw consentError("Ordinary Open AXPress failed: \(status.rawValue); no retry") }
		print("Zen normal Open AXPress returned success for held PID \(candidate.pid); awaiting native completion")
		return nil
	}
}
`

func desktopMacVerifyZenConsentTarget(t *testing.T, ctx context.Context, bundle string) {
	t.Helper()
	if bundle != "/Applications/Zen.app" || os.Getenv("DOTFILES_NATIVE_DESKTOP") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("Zen consent requires the explicit canonical hosted fixture")
	}
	for _, args := range [][]string{
		{"/usr/bin/codesign", "--verify", "--deep", "--strict=symlinks", "-R", "=" + zenCaskSpec("", "").Requirement, bundle},
		{"/usr/bin/codesign", "--verify", "--strict", "-R", `=anchor apple and identifier "com.apple.coreservices.uiagent"`, "/System/Library/CoreServices/CoreServicesUIAgent.app"},
		{"/usr/sbin/spctl", "--status"},
		{"/usr/sbin/spctl", "--assess", "--type", "execute", "-vv", bundle},
	} {
		output, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("Zen consent preflight %s: %v%s", args[0], err, nativeDiagnostic(output))
		}
		if args[1] == "--status" && strings.TrimSpace(string(output)) != "assessments enabled" {
			t.Fatal("Zen consent requires enabled Gatekeeper", string(output))
		}
		if args[1] == "--assess" && (!strings.Contains(string(output), "source=Notarized Developer ID") || strings.Contains(string(output), "override=")) {
			t.Fatal("Zen consent requires native notarization without override", string(output))
		}
		t.Logf("Zen consent preflight %s: %s", filepath.Base(args[0]), output)
	}
}

func TestDesktopMacZenConsentRejectsUnrelatedPrompts(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native Swift consent classifier; no UI action")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	binary := desktopMacLauncher(t, ctx)
	base := map[string]any{
		"quarantine": `0181;6ad6704d;Homebrew\x20Cask;4F4CF190-1B99-4334-B189-BAD420C126F6`,
		"dialog": map[string]any{"role": "AXWindow", "subrole": "AXSystemDialog", "children": []map[string]any{
			{"role": "AXImage", "description": "alert"},
			{"role": "AXStaticText", "value": "“Zen” is an app downloaded from the Internet. Are you sure you want to open it?"},
			{"role": "AXStaticText", "value": "Homebrew Cask downloaded this file today at 7:32\u202fPM. Apple checked it for malicious software and none was detected."},
			{"role": "AXButton", "description": "Help", "actions": []string{"AXPress"}},
			{"role": "AXButton", "title": "Cancel", "identifier": "action-button-3", "actions": []string{"AXPress"}},
			{"role": "AXButton", "title": "Open", "identifier": "action-button-1", "actions": []string{"AXPress"}},
		}},
	}
	for _, node := range base["dialog"].(map[string]any)["children"].([]map[string]any) {
		for _, key := range []string{"title", "value", "identifier", "description"} {
			if _, ok := node[key]; !ok {
				node[key] = ""
			}
		}
		node["enabled"] = true
		if _, ok := node["actions"]; !ok {
			node["actions"] = []string{}
		}
	}
	for _, name := range []string{"observed", "wrong-app", "wrong-origin", "malware", "permission", "open-anyway", "wrong-window", "wrong-button-id", "disabled", "no-action", "extra-button", "missing-cancel", "approved", "wrong-quarantine-origin", "missing-quarantine"} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var input map[string]any
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			dialog := input["dialog"].(map[string]any)
			nodes := dialog["children"].([]any)
			switch name {
			case "wrong-app":
				nodes[1].(map[string]any)["value"] = "“Other” is an app downloaded from the Internet. Are you sure you want to open it?"
			case "wrong-origin":
				nodes[2].(map[string]any)["value"] = "Safari downloaded this file today at 7:32 PM. Apple checked it for malicious software and none was detected."
			case "malware":
				nodes[2].(map[string]any)["value"] = "Apple could not verify this app is free of malware."
			case "permission":
				nodes[5].(map[string]any)["title"] = "Allow"
			case "open-anyway":
				nodes[5].(map[string]any)["title"] = "Open Anyway"
			case "wrong-window":
				dialog["subrole"] = "AXStandardWindow"
			case "wrong-button-id":
				nodes[5].(map[string]any)["identifier"] = "action-button-3"
			case "disabled":
				nodes[5].(map[string]any)["enabled"] = false
			case "no-action":
				nodes[5].(map[string]any)["actions"] = []string{}
			case "extra-button":
				dialog["children"] = append(nodes, nodes[5])
			case "missing-cancel":
				dialog["children"] = append(nodes[:4], nodes[5:]...)
			case "approved":
				input["quarantine"] = strings.Replace(input["quarantine"].(string), "0181", "01c1", 1)
			case "wrong-quarantine-origin":
				input["quarantine"] = strings.Replace(input["quarantine"].(string), `Homebrew\x20Cask`, "Safari", 1)
			case "missing-quarantine":
				input["quarantine"] = ""
			}
			data, err = json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, binary, "--check-zen-consent")
			command.Stdin = strings.NewReader(string(data))
			output, err := command.CombinedOutput()
			if (err == nil) != (name == "observed") {
				t.Fatalf("consent classification: %v%s", err, nativeDiagnostic(output))
			}
		})
	}
	t.Run("not-hosted", func(t *testing.T) {
		command := exec.CommandContext(ctx, binary, "/Applications/Zen.app", "--zen-consent")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "GITHUB_ACTIONS=") {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "GITHUB_ACTIONS=false")
		command.Stdin = strings.NewReader("{}")
		output, err := command.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "Zen consent requires the explicit disposable hosted fixture") {
			t.Fatalf("ordinary host did not stop before permission/UI inspection: %v%s", err, nativeDiagnostic(output))
		}
	})
}
