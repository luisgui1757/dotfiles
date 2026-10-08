package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Failure-only diagnostics on the already opted-in disposable runner. Neither
// experiment publishes a payload or changes the original lifecycle result.
func diagnosePortableGitFailure(t *testing.T, failed nativeCommand, pin ArchivePin) {
	t.Helper()
	if runtime.GOOS != "windows" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_NATIVE_WINDOWS_GIT_MAKE") != "1" {
		t.Log("PortableGit native diagnostics refused outside the opted-in hosted fixture")
		return
	}
	archive, err := os.ReadFile(failed.Program)
	if err != nil {
		t.Log("PortableGit diagnostic archive:", err)
		return
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != pin.SHA256 {
		t.Log("PortableGit diagnostic refused changed archive")
		return
	}
	payload := filepath.Dir(failed.Program)
	// Seven-character sibling names preserve the production payload path length.
	prepare := func(name string) (string, []string, error) {
		root := filepath.Join(filepath.Dir(payload), name)
		if len(root) != len(payload) || filepath.Base(payload) != "payload" {
			return "", nil, fmt.Errorf("diagnostic requires unchanged payload path length")
		}
		if err := os.Mkdir(root, 0700); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(filepath.Join(root, "portable-git.exe"), archive, 0700); err != nil {
			return "", nil, err
		}
		home, temp := filepath.Join(root, ".prepare-home"), filepath.Join(root, ".prepare-temp")
		for _, path := range []string{home, temp} {
			if err := os.Mkdir(path, 0700); err != nil {
				return "", nil, err
			}
		}
		return root, append(os.Environ(), portableGitPreparationEnvironment(root, home, temp)...), nil
	}
	// The exact upstream SFX supports this dialog but -y suppresses errors. Accept
	// only its exact prefilled fixture output directory, then capture its own PID's
	// dialog. No keys/clicks are sent to other applications or unknown prompts.
	root, environment, err := prepare("diag-ui")
	if err != nil {
		t.Log("PortableGit dialog fixture:", err)
	} else {
		capturePortableGitDialog(t, root, environment)
	}
	root, environment, err = prepare("diag-xl")
	if err != nil {
		t.Log("PortableGit extended-path fixture:", err)
		return
	}
	if len(filepath.VolumeName(root)) != 2 || strings.HasPrefix(root, `\\`) {
		t.Log("PortableGit extended-path experiment requires an absolute local drive path")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(root, "portable-git.exe"), "-y", "-o"+`\\?\`+root)
	command.Env, command.WaitDelay = environment, 2*time.Second
	var output nativeOutput
	command.Stdout, command.Stderr = &output, &output
	err = command.Run()
	t.Logf("PortableGit extended-path diagnostic output-root=%q unchanged-length=%d exit=%v context=%v output=%s", root, len(root), err, ctx.Err(), nativeDiagnostic(output.data.Bytes()))
	opened, openErr := os.OpenRoot(root)
	if openErr != nil {
		t.Log("PortableGit extended-path payload inspection:", openErr)
		return
	}
	filesErr := checkArchivePayload(opened, pin)
	closeErr := opened.Close()
	t.Logf("PortableGit extended-path required files: %v; close: %v", filesErr, closeErr)
	for _, name := range []string{"post-install.bat", "etc/post-install"} {
		_, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(name)))
		t.Logf("PortableGit extended-path post-install remainder %s: %v", name, statErr)
	}
	if err != nil || filesErr != nil || closeErr != nil {
		return
	}
	probe := exec.CommandContext(ctx, filepath.Join(root, "bin", "bash.exe"), "--noprofile", "--norc", "-p", "-c", portableGitRuntimeProbe, "dotfiles-portable-git", pin.Version)
	probe.Env, probe.WaitDelay = environment, 2*time.Second
	var runtimeOutput nativeOutput
	probe.Stdout, probe.Stderr = &runtimeOutput, &runtimeOutput
	err = probe.Run()
	t.Logf("PortableGit extended-path runtime probe: %v context=%v output=%s", err, ctx.Err(), nativeDiagnostic(runtimeOutput.data.Bytes()))
}

func capturePortableGitDialog(t *testing.T, root string, environment []string) {
	t.Helper()
	powershell, err := DiscoverWindowsPowerShell()
	if err != nil {
		t.Log("PortableGit dialog inspection:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(root, "portable-git.exe"), "-o"+root)
	command.Env, command.WaitDelay = environment, 2*time.Second
	var extractorOutput nativeOutput
	command.Stdout, command.Stderr = &extractorOutput, &extractorOutput
	if err := command.Start(); err != nil {
		t.Log("PortableGit diagnostic start:", err)
		return
	}
	script := portableGitDialogInspection + fmt.Sprintf("\n[PortableGitDialogs]::Capture(%d, %s) | ConvertTo-Json -Compress\n", command.Process.Pid, desktopPSQuote(root))
	query := exec.CommandContext(ctx, powershell, windowsVendorArguments(script)...)
	var stdout, stderr nativeOutput
	query.Stdout, query.Stderr, query.WaitDelay = &stdout, &stderr, 2*time.Second
	queryErr := query.Run()
	t.Logf("PortableGit own-PID dialog capture: %s query=%v context=%v diagnostics=%s", stdout.data.Bytes(), queryErr, ctx.Err(), nativeDiagnostic(stderr.data.Bytes()))
	// Own process only; a failure dialog is intentionally left open for capture.
	killErr := command.Process.Kill()
	waitErr := command.Wait()
	t.Logf("PortableGit diagnostic process stop: kill=%v wait=%v output=%s", killErr, waitErr, nativeDiagnostic(extractorOutput.data.Bytes()))
}

const portableGitDialogInspection = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
public static class PortableGitDialogs {
	public delegate bool Callback(IntPtr window, IntPtr param);
	[DllImport("user32.dll")] static extern bool EnumWindows(Callback callback, IntPtr param);
	[DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr window, Callback callback, IntPtr param);
	[DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint pid);
	[DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr window, StringBuilder text, int capacity);
	[DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern IntPtr SendMessageTimeout(IntPtr window, uint message, UIntPtr wparam, StringBuilder text, uint flags, uint timeout, out UIntPtr result);
	[DllImport("user32.dll")] static extern bool PostMessage(IntPtr window, uint message, IntPtr wparam, IntPtr lparam);
	static string Text(IntPtr window) {
		var text = new StringBuilder(4096); UIntPtr result;
		if (SendMessageTimeout(window, 13, (UIntPtr)4096, text, 2, 200, out result) == IntPtr.Zero) return "<unreadable>";
		return text.ToString();
	}
	static string Class(IntPtr window) { var text = new StringBuilder(256); GetClassName(window, text, text.Capacity); return text.ToString(); }
	public static string[] Capture(int pid, string expectedOutput) {
		var records = new List<string>(); var seen = new HashSet<string>(); bool approved = false;
		Process process;
		try { process = Process.GetProcessById(pid); }
		catch (ArgumentException) { return new string[] { "Extractor exited before dialog observation" }; }
		using (process) {
			var timer = Stopwatch.StartNew();
			while (!process.HasExited && timer.ElapsedMilliseconds < 60000) {
				bool error = false;
				EnumWindows(delegate(IntPtr window, IntPtr unused) {
					uint owner; GetWindowThreadProcessId(window, out owner);
					if (owner != pid || Class(window) != "#32770") return true;
					string title = Text(window); var lines = new List<string>(); bool exactEdit = false; int count = 0;
					EnumChildWindows(window, delegate(IntPtr child, IntPtr ignored) {
						if (++count > 32) return false;
						string text = Text(child), kind = Class(child);
						lines.Add(kind + ": " + text);
						if (kind == "Edit" && text == expectedOutput) exactEdit = true;
						return true;
					}, IntPtr.Zero);
					string record = title + "\n" + String.Join("\n", lines);
					if (record.Length > 8192) record = record.Substring(0, 8192);
					if (seen.Add(record) && records.Count < 8) records.Add(record);
					if (!approved && title == "Portable Git for Windows 64-bit" && exactEdit) {
						if (!PostMessage(window, 0x111, (IntPtr)1, IntPtr.Zero)) throw new InvalidOperationException("Could not accept exact fixture extraction directory");
						approved = true;
					}
					if (title == "Extraction Failed" || title.IndexOf("Error", StringComparison.OrdinalIgnoreCase) >= 0) {
						error = true;
						if (!records.Contains(record)) records.Add(record);
					}
					return true;
				}, IntPtr.Zero);
				if (error) { records.Add("Captured extractor error; no dismissal or fallback"); break; }
				Thread.Sleep(100);
			}
			records.Add("Exact fixture directory approved=" + approved + "; elapsed-ms=" + timer.ElapsedMilliseconds + "; exited=" + process.HasExited);
		}
		return records.ToArray();
	}
}
'@
`
