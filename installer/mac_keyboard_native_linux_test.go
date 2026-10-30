package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Linux input event codes (include/uapi/linux/input-event-codes.h).
var linuxKeyCodes = map[string]uint16{
	"Esc": 1, "1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10, "0": 11, "Minus": 12, "Equal": 13,
	"Backspace": 14, "Tab": 15, "Q": 16, "W": 17, "E": 18, "R": 19, "T": 20, "Y": 21, "U": 22, "I": 23, "O": 24, "P": 25,
	"LeftBrace": 26, "RightBrace": 27, "Enter": 28, "LeftCtrl": 29, "A": 30, "S": 31, "D": 32, "F": 33, "G": 34, "H": 35,
	"J": 36, "K": 37, "L": 38, "Semicolon": 39, "Apostrophe": 40, "Grave": 41, "LeftShift": 42, "Backslash": 43, "Z": 44,
	"X": 45, "C": 46, "V": 47, "B": 48, "N": 49, "M": 50, "Comma": 51, "Dot": 52, "Slash": 53, "RightShift": 54,
	"LeftAlt": 56, "Space": 57, "F1": 59, "F2": 60, "F3": 61, "F4": 62, "F5": 63, "F6": 64, "F7": 65, "F8": 66, "F9": 67,
	"F10": 68, "102nd": 86, "F11": 87, "F12": 88, "RightCtrl": 97, "SysRq": 99, "RightAlt": 100, "Home": 102, "Up": 103,
	"PageUp": 104, "Left": 105, "Right": 106, "End": 107, "Down": 108, "PageDown": 109, "Insert": 110, "Delete": 111,
	"LeftMeta": 125, "RightMeta": 126, "F24": 194,
}

var linuxModifiers = map[uint16]string{29: "Ctrl", 97: "Ctrl", 42: "Shift", 54: "Shift", 56: "Alt", 100: "Alt", 125: "Super", 126: "Super"}

func linuxKeyName(code uint16) string {
	for name, value := range linuxKeyCodes {
		if value == code {
			return name
		}
	}
	return fmt.Sprintf("code%d", code)
}

// The physical keys of a Mac-layout keyboard in its Linux mode.
var macPhysical = map[string]string{"cmd": "LeftAlt", "opt": "LeftMeta", "ropt": "RightMeta", "ctrl": "LeftCtrl", "shift": "LeftShift"}

type uinputKeyboard struct {
	file  *os.File
	delay time.Duration
}

func newUinputKeyboard(t *testing.T, name string) *uinputKeyboard {
	t.Helper()
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal("open /dev/uinput:", err)
	}
	const uiSetEvbit, uiSetKeybit, uiDevCreate = 0x40045564, 0x40045565, 0x5501
	fd := int(f.Fd())
	if err := errors.Join(unix.IoctlSetInt(fd, uiSetEvbit, 1), unix.IoctlSetInt(fd, uiSetEvbit, 0)); err != nil {
		t.Fatal(err)
	}
	for code := 1; code < 249; code++ {
		if err := unix.IoctlSetInt(fd, uiSetKeybit, code); err != nil {
			t.Fatal(err)
		}
	}
	// Legacy struct uinput_user_dev: name, input_id, ff_effects_max, 4x64 absolute axes.
	device := make([]byte, 80+8+4+4*64*4)
	copy(device, name)
	binary.LittleEndian.PutUint16(device[80:], 0x03) // BUS_USB
	binary.LittleEndian.PutUint16(device[82:], 0x1209)
	binary.LittleEndian.PutUint16(device[84:], 0x0001)
	binary.LittleEndian.PutUint16(device[86:], 1)
	if _, err := f.Write(device); err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetInt(fd, uiDevCreate, 0); err != nil {
		t.Fatal(err)
	}
	k := &uinputKeyboard{file: f, delay: 15 * time.Millisecond}
	t.Cleanup(func() { _ = unix.IoctlSetInt(fd, 0x5502, 0); _ = f.Close() })
	return k
}

func (k *uinputKeyboard) emit(t *testing.T, code uint16, value int32) {
	t.Helper()
	event := make([]byte, 48)
	binary.LittleEndian.PutUint16(event[16:], 1) // EV_KEY
	binary.LittleEndian.PutUint16(event[18:], code)
	binary.LittleEndian.PutUint32(event[20:], uint32(value))
	// EV_SYN/SYN_REPORT follows in the second 24-byte record.
	if _, err := k.file.Write(event); err != nil {
		t.Fatal(err)
	}
	time.Sleep(k.delay)
}

// press holds the given physical keys in order, then releases them in reverse.
func (k *uinputKeyboard) press(t *testing.T, keys ...string) {
	t.Helper()
	codes := []uint16{}
	for _, key := range keys {
		if physical, ok := macPhysical[key]; ok {
			key = physical
		}
		code, ok := linuxKeyCodes[key]
		if !ok {
			t.Fatal("unknown key", key)
		}
		codes = append(codes, code)
		k.emit(t, code, 1)
	}
	for i := len(codes) - 1; i >= 0; i-- {
		k.emit(t, codes[i], 0)
	}
}

type evdevEvent struct {
	Type, Code uint16
	Value      int32
}

// outputEvents reads the remapper's virtual keyboard.
func outputEvents(t *testing.T, path string) <-chan evdevEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	events := make(chan evdevEvent, 4096)
	go func() {
		record := make([]byte, 24)
		for {
			if _, err := io.ReadFull(f, record); err != nil {
				close(events)
				return
			}
			events <- evdevEvent{binary.LittleEndian.Uint16(record[16:]), binary.LittleEndian.Uint16(record[18:]), int32(binary.LittleEndian.Uint32(record[20:]))}
		}
	}()
	return events
}

// combos turns output key events into the shortcuts an application receives:
// each non-modifier press with the modifiers then held, and Super taps. A tap
// names the modifiers held when Super went down: GNOME opens its overview only
// for a bare tap:Super.
func combos(events []evdevEvent) ([]string, map[uint16]bool) {
	held, result := map[uint16]bool{}, []string{}
	names := func() []string {
		names := []string{}
		for _, name := range []string{"Ctrl", "Alt", "Shift", "Super"} {
			for code := range held {
				if linuxModifiers[code] == name && !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
		}
		return names
	}
	tap := ""
	for _, e := range events {
		if e.Type != 1 || e.Value == 2 {
			continue
		}
		modifier, isModifier := linuxModifiers[e.Code]
		switch {
		case isModifier && e.Value == 1:
			tap = ""
			if modifier == "Super" {
				tap = "tap:" + strings.Join(append(names(), "Super"), "+")
			}
			held[e.Code] = true
		case isModifier:
			delete(held, e.Code)
			if modifier == "Super" && tap != "" {
				result = append(result, tap)
			}
			tap = ""
		case e.Value == 1:
			tap = ""
			result = append(result, strings.Join(append(names(), linuxKeyName(e.Code)), "+"))
		}
	}
	return result, held
}

func drain(events <-chan evdevEvent, quiet time.Duration) []evdevEvent {
	collected := []evdevEvent{}
	for {
		select {
		case e, ok := <-events:
			if !ok {
				return collected
			}
			collected = append(collected, e)
		case <-time.After(quiet):
			return collected
		}
	}
}

func linuxInputEventNode(t *testing.T, name *regexp.Regexp, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile("/proc/bus/input/devices")
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(data), "\n\n") {
			current, handlers := "", ""
			scanner := bufio.NewScanner(strings.NewReader(block))
			for scanner.Scan() {
				line := scanner.Text()
				if value, ok := strings.CutPrefix(line, "N: Name="); ok {
					current = strings.Trim(value, `"`)
				}
				if value, ok := strings.CutPrefix(line, "H: Handlers="); ok {
					handlers = value
				}
			}
			if name.MatchString(current) {
				for _, handler := range strings.Fields(handlers) {
					if strings.HasPrefix(handler, "event") {
						return "/dev/input/" + handler
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no input device named %s:\n%s", name, data)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestNativeMacKeyboardLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NATIVE_KEYBOARD") != "1" {
		t.Skip("set DOTFILES_TEST_NATIVE_KEYBOARD=1 on a disposable Linux runner with Xvfb, as root")
	}
	if os.Geteuid() != 0 || os.Getenv("DISPLAY") == "" {
		t.Fatal("the opted-in fixture creates a virtual keyboard and a udev rule: run it as root under Xvfb")
	}
	tools := map[string]string{}
	for _, name := range []string{"xterm", "xdotool", "xwininfo", "gio", "udevadm"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal("the opted-in fixture requires", name, err)
		}
		tools[name] = path
	}
	if _, err := os.Lstat(keyboardAccessRules); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the disposable host already has a keyboard access rule", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Minute)
	defer cancel()
	platform := nativePlatform(t)
	var err error
	if platform.APT, err = DiscoverLinuxAPT(ctx); err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Data: filepath.Join(home, "data"), Zsh: home}
	c, err := NewNativeController(repository, "native-mac-keyboard", filepath.Join(home, "state"), platform, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	d.session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(d.session.Directory) }
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"mac-keyboard"}})

	// The rule: exact operation-named bytes, root-owned, accepted by udev.
	state, err := LoadState(c.StatePath, home)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(keyboardAccessRules)
	uid, _, mode, ownerErr := fileOwner(keyboardAccessRules)
	if err != nil || ownerErr != nil || uid != 0 || mode != 0644 || !bytes.Equal(data, keyboardAccessContent(state.Receipts["system.keyboard-access"].OperationID)) {
		t.Fatal("the keyboard access rule is not the exact root-owned recipe", string(data), uid, mode, err, ownerErr)
	}
	if output, err := exec.CommandContext(ctx, tools["udevadm"], "verify", keyboardAccessRules).CombinedOutput(); err != nil {
		t.Fatal("udev rejects the rule", err, nativeDiagnostic(output))
	}
	// The configuration is a live link to the checkout and valid for the pinned binary.
	config := filepath.Join(folders.Config, "xremap", "config.yml")
	if target, err := os.Readlink(config); err != nil || target != filepath.Join(repository, "xremap", "config.yml") {
		t.Fatal("xremap configuration is not the checkout's live link", target, err)
	}
	xremap, err := d.Archives.CommandPath("tool.xremap", "xremap")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, xremap, "--validate-config", config).CombinedOutput(); err != nil {
		t.Fatal("the pinned xremap rejects the configuration", err, nativeDiagnostic(output))
	}
	autostart := filepath.Join(folders.Config, "autostart", "dotfiles-mac-keyboard.desktop")
	if _, err := os.Stat(autostart); err != nil {
		t.Fatal("xremap does not start at login", err)
	}

	// Windows that identify as the applications the rules name.
	focus := map[string]string{}
	for _, window := range []struct{ instance, class string }{
		{"ghostty", "com.mitchellh.ghostty"}, {"Navigator", "zen"}, {"org.gnome.Nautilus", "org.gnome.Nautilus"}, {"gnome-text-editor", "org.gnome.TextEditor"},
		{"gnome-terminal-server", "Gnome-terminal"}, {"obsidian", "obsidian"},
	} {
		xterm := exec.CommandContext(ctx, tools["xterm"], "-name", window.instance, "-class", window.class, "-e", "sleep", "900")
		if err := xterm.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = xterm.Process.Kill(); _, _ = xterm.Process.Wait() })
		output, err := exec.CommandContext(ctx, tools["xdotool"], "search", "--sync", "--class", "^"+regexp.QuoteMeta(window.class)+"$").Output()
		if err != nil {
			t.Fatal("window did not appear", window.class, err)
		}
		focus[window.class] = strings.Fields(string(output))[0]
	}
	root, err := exec.CommandContext(ctx, tools["xwininfo"], "-root").Output()
	match := regexp.MustCompile(`Window id: (0x[0-9a-f]+)`).FindSubmatch(root)
	if err != nil || match == nil {
		t.Fatal("cannot identify the root window", err)
	}
	focus[""] = string(match[1])

	// The kernel console also reads the remapper's output: Alt+F1...F12 change
	// the virtual terminal and Ctrl+Alt+Delete restarts the machine. Turn its
	// keyboard off during the test, as a display server does.
	if console, err := os.OpenFile("/dev/tty0", os.O_RDWR, 0); err == nil {
		const kdgkbmode, kdskbmode, keyboardOff = 0x4B44, 0x4B45, 0x04
		fd := int(console.Fd())
		mode, err := unix.IoctlGetInt(fd, kdgkbmode)
		if err == nil {
			err = unix.IoctlSetInt(fd, kdskbmode, keyboardOff)
		}
		if err != nil {
			t.Fatal("cannot turn off the console keyboard", err)
		}
		t.Cleanup(func() { _ = unix.IoctlSetInt(fd, kdskbmode, mode); _ = console.Close() })
	}
	keyboard := newUinputKeyboard(t, "dotfiles-test-keyboard")
	linuxInputEventNode(t, regexp.MustCompile(`^dotfiles-test-keyboard$`), 10*time.Second)
	// Start xremap exactly as the login session does.
	// xremap outlives gio and keeps its output open: log to a file, not a pipe.
	logPath := filepath.Join(t.TempDir(), "xremap.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	// A stand-in for Resources, the Ubuntu app that Cmd+Option+Esc opens.
	apps := t.TempDir()
	forceQuit := filepath.Join(apps, "force-quit")
	if err := os.WriteFile(filepath.Join(apps, "resources"), []byte("#!/bin/sh\necho opened >> '"+forceQuit+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launch := exec.CommandContext(ctx, tools["gio"], "launch", autostart)
	launch.Env = append(slices.DeleteFunc(os.Environ(), func(v string) bool {
		return strings.HasPrefix(v, "XDG_SESSION_TYPE=") || strings.HasPrefix(v, "PATH=")
	}), "XDG_SESSION_TYPE=x11", "PATH="+apps+":"+os.Getenv("PATH"))
	launch.Stdout, launch.Stderr = log, log
	launchErr := launch.Run()
	t.Cleanup(func() {
		_ = exec.Command("/usr/bin/pkill", "-f", "^"+regexp.QuoteMeta(xremap)+" ").Run()
		_ = log.Close()
		if t.Failed() {
			output, _ := os.ReadFile(logPath)
			t.Logf("xremap output:%s", nativeDiagnostic(output))
		}
	})
	if launchErr != nil {
		t.Fatal("the autostart entry failed", launchErr)
	}
	events := outputEvents(t, linuxInputEventNode(t, regexp.MustCompile(`^xremap( pid=\d+)?$`), 30*time.Second))
	time.Sleep(time.Second)
	drain(events, 300*time.Millisecond)

	drive := func(window, label string, input func()) []string {
		t.Helper()
		if output, err := exec.CommandContext(ctx, tools["xdotool"], "windowfocus", "--sync", focus[window]).CombinedOutput(); err != nil {
			t.Fatal("cannot focus", window, err, nativeDiagnostic(output))
		}
		drain(events, 100*time.Millisecond)
		input()
		got, held := combos(drain(events, 300*time.Millisecond))
		if len(held) != 0 {
			t.Fatalf("%s %s left keys held: %v", window, label, held)
		}
		return got
	}
	run := func(window string, keys []string) []string {
		t.Helper()
		return drive(window, strings.Join(keys, "+"), func() { keyboard.press(t, keys...) })
	}
	check := func(window string, keys []string, want ...string) {
		t.Helper()
		if got := run(window, keys); !slices.Equal(got, want) {
			t.Errorf("%s %v produced %q, want %q", window, keys, got, want)
		}
	}
	editor, browser, files, terminal := "org.gnome.TextEditor", "zen", "org.gnome.Nautilus", "com.mitchellh.ghostty"
	k := func(keys ...string) []string { return keys }
	// Apps: Cmd is Ctrl, Option moves by word, Ctrl is unchanged.
	check(editor, k("cmd", "C"), "Ctrl+C")
	check(editor, k("cmd", "shift", "Z"), "Ctrl+Shift+Z")
	check(editor, k("cmd", "Left"), "Home")
	check(editor, k("cmd", "shift", "Left"), "Shift+Home")
	check(editor, k("cmd", "Up"), "Ctrl+Home")
	check(editor, k("opt", "Left"), "Ctrl+Left")
	check(editor, k("opt", "shift", "Right"), "Ctrl+Shift+Right")
	check(editor, k("opt", "Backspace"), "Ctrl+Backspace")
	check(editor, k("cmd", "Backspace"), "Shift+Home", "Backspace")
	check(editor, k("cmd", "opt", "Down"), "Ctrl+PageDown")
	check(editor, k("cmd", "shift", "RightBrace"), "Ctrl+PageDown")
	check(editor, k("ctrl", "C"), "Ctrl+C")
	check(editor, k("opt", "C"), "Alt+C")
	check(editor, k("cmd", "Space"), "tap:Super")
	check(editor, k("ctrl", "Up"), "tap:Super")
	check(editor, k("ctrl", "Down"), "tap:Super")
	check(terminal, k("ctrl", "Down"), "tap:Super")
	check(editor, k("ctrl", "cmd", "Space"), "Super+Dot")
	check(editor, k("ctrl", "Left"), "Super+PageUp")
	check(editor, k("cmd", "shift", "3"), "Shift+SysRq")
	check(editor, k("cmd", "shift", "4"), "SysRq")
	check(editor, k("ctrl", "cmd", "Q"), "Super+L")
	check(editor, k("cmd", "H"), "Super+H")
	check(editor, k("ctrl", "cmd", "F"), "F11")
	// Option+Space is a space, not the GNOME window menu (Alt+Space).
	check(editor, k("opt", "Space"), "Space")
	check(terminal, k("opt", "Space"), "Space")
	// The right Option key is Option too. As Super, a tap would open the
	// overview and Right Option+L would lock the screen.
	check(editor, k("ropt"))
	check(editor, k("ropt", "L"), "Alt+L")
	check(editor, k("ropt", "Left"), "Ctrl+Left")
	check(terminal, k("ropt", "C"), "Alt+C")
	// Holding Cmd keeps Super held across Tabs, so GNOME's application
	// switcher (Super+Tab, also in Ubuntu) stays open.
	cmd, tab := linuxKeyCodes["LeftAlt"], linuxKeyCodes["Tab"]
	var held []evdevEvent
	switching := drive(editor, "cmd+tab+tab", func() {
		keyboard.emit(t, cmd, 1)
		for range 2 {
			keyboard.emit(t, tab, 1)
			keyboard.emit(t, tab, 0)
		}
		held = drain(events, 300*time.Millisecond)
		keyboard.emit(t, cmd, 0)
	})
	if pressed, _ := combos(held); !slices.Equal(pressed, []string{"Super+Tab", "Super+Tab"}) || len(switching) != 0 {
		t.Errorf("Cmd+Tab+Tab produced %q then %q, want Super held across two Tabs", pressed, switching)
	}
	for _, e := range held {
		if e.Type == 1 && e.Code == linuxKeyCodes["LeftMeta"] && e.Value == 0 {
			t.Error("Super was released while Cmd was still held")
		}
	}
	check(editor, k("C"), "C")
	check(browser, k("cmd", "LeftBrace"), "Alt+Left")
	check(browser, k("cmd", "opt", "Up"), "Ctrl+PageUp")
	check(browser, k("cmd", "T"), "Ctrl+T")
	check(browser, k("cmd", "opt", "I"), "Ctrl+Shift+I")
	// Browsers: the Ctrl text keys of a Mac text field. Ctrl+Tab and the
	// Ctrl+Option chords stay. Other apps keep Linux Ctrl.
	check(browser, k("ctrl", "A"), "Home")
	check(browser, k("ctrl", "E"), "End")
	check(browser, k("ctrl", "K"), "Shift+End", "Delete")
	check(browser, k("ctrl", "N"), "Down")
	check(browser, k("ctrl", "Tab"), "Ctrl+Tab")
	check(browser, k("ctrl", "opt", "F"), "Ctrl+Alt+F")
	check(editor, k("ctrl", "A"), "Ctrl+A")
	check("obsidian", k("ctrl", "A"), "Home")
	check("obsidian", k("ctrl", "K"), "Shift+End", "Delete")
	check("obsidian", k("cmd", "A"), "Ctrl+A")
	check(files, k("cmd", "Backspace"), "Delete")
	check(files, k("cmd", "opt", "Backspace"), "Shift+Delete")
	check(files, k("cmd", "Up"), "Alt+Up")
	check(files, k("cmd", "LeftBrace"), "Alt+Left")
	check(files, k("cmd", "shift", "Dot"), "Ctrl+H")
	// Terminals: Cmd is the terminal's own shortcut; Option is Meta.
	check(terminal, k("cmd", "C"), "Ctrl+Shift+C")
	check(terminal, k("cmd", "V"), "Ctrl+Shift+V")
	check(terminal, k("cmd", "T"), "Ctrl+Shift+T")
	check(terminal, k("cmd", "D"), "Ctrl+Shift+O")
	check(terminal, k("cmd", "shift", "D"), "Ctrl+Shift+E")
	check(terminal, k("cmd", "Left"), "Ctrl+A")
	check(terminal, k("cmd", "K"), "Ctrl+L")
	check(terminal, k("cmd", "3"), "Alt+3")
	check(terminal, k("cmd", "shift", "RightBrace"), "Ctrl+PageDown")
	// Ghostty's Mac keys for splits, prompts, scrolling and full screen.
	check(terminal, k("cmd", "W"), "Ctrl+Super+W")
	check(terminal, k("cmd", "opt", "W"), "Ctrl+Shift+W")
	check(terminal, k("cmd", "shift", "W"), "Alt+F4")
	check(terminal, k("cmd", "LeftBrace"), "Ctrl+Super+LeftBrace")
	check(terminal, k("cmd", "opt", "Left"), "Ctrl+Alt+Super+Left")
	check(terminal, k("ctrl", "cmd", "Left"), "Ctrl+Shift+Super+Left")
	check(terminal, k("cmd", "Up"), "Ctrl+Shift+PageUp")
	check(terminal, k("cmd", "Home"), "Shift+Home")
	check(terminal, k("ctrl", "cmd", "F"), "Ctrl+Enter")
	// Another terminal does not have these Ghostty keybindings. It would get
	// Ctrl+Super+W as ^W and Ctrl+Enter as Return, so it keeps safe keys.
	otherTerminal := "Gnome-terminal"
	check(otherTerminal, k("cmd", "W"), "Ctrl+Shift+W")
	check(otherTerminal, k("ctrl", "cmd", "F"), "F11")
	check(otherTerminal, k("cmd", "Enter"))
	check(otherTerminal, k("cmd", "D"))
	check(otherTerminal, k("cmd", "LeftBrace"))
	check(otherTerminal, k("cmd", "Up"))
	check(terminal, k("cmd", "Z"))
	check(terminal, k("opt", "C"), "Alt+C")
	check(terminal, k("opt", "Left"), "Alt+Left")
	check(terminal, k("ctrl", "C"), "Ctrl+C")
	// Navigation mirrors AeroSpace: Option(+Shift)+digit is Super(+Shift)+digit,
	// GNOME's workspace keys as the README sets them up, and Ctrl+Option+L/H
	// cycles windows like AeroSpace's focus right/left. No Alt or Ctrl stays
	// pressed into the GNOME shortcut. Cmd+digit keeps its meaning, Option+Tab
	// stays Alt+Tab and Ctrl+Option+J/K stay unmapped.
	for _, window := range []string{editor, terminal} {
		check(window, k("opt", "1"), "Super+1")
		check(window, k("opt", "9"), "Super+9")
		check(window, k("opt", "shift", "1"), "Shift+Super+1")
		check(window, k("opt", "shift", "9"), "Shift+Super+9")
		check(window, k("ctrl", "opt", "L"), "Alt+Esc")
		check(window, k("ctrl", "opt", "H"), "Alt+Shift+Esc")
		check(window, k("opt", "Tab"), "Alt+Tab")
		check(window, k("ctrl", "opt", "J"), "Ctrl+Alt+J")
	}
	check(terminal, k("cmd", "1"), "Alt+1")
	check(editor, k("cmd", "1"), "Ctrl+1")
	// GNOME keys that do nothing on a Mac do nothing: no closed window, text
	// console, terminal, log out dialog or workspace change.
	for _, window := range []string{editor, terminal} {
		check(window, k("opt", "F4"))
		check(window, k("opt", "Esc"))
		check(window, k("ctrl", "opt", "F3"))
		check(window, k("cmd", "opt", "F3"))
		check(window, k("ctrl", "opt", "T"))
		check(window, k("ctrl", "opt", "Delete"))
		check(window, k("ctrl", "opt", "Left"))
		check(window, k("ctrl", "shift", "opt", "R"))
	}
	// Cmd+Option+Esc opens the app that ends other apps, like Force Quit.
	check(editor, k("cmd", "opt", "Esc"))
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if data, _ := os.ReadFile(forceQuit); string(data) == "opened\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Error("Cmd+Option+Esc did not open Resources")
			break
		}
	}
	// An unidentified window keeps plain Linux keys.
	check("", k("cmd", "C"), "Alt+C")
	check("", k("opt", "C"), "Super+C")
	check("", k("ropt", "C"), "Super+C")
	check("", k("opt", "1"), "Super+1")
	check("", k("opt", "shift", "1"), "Shift+Super+1")
	check("", k("ctrl", "opt", "L"), "Ctrl+Super+L")

	// No Cmd combination may reach a terminal as a bare control character.
	// Ctrl+Super chords are Ghostty keybindings.
	allowed := []string{"Ctrl+A", "Ctrl+E", "Ctrl+U", "Ctrl+L", "Ctrl+Enter", "Ctrl+Comma", "Ctrl+Equal", "Ctrl+Minus", "Ctrl+0", "Ctrl+PageUp", "Ctrl+PageDown",
		"Ctrl+Super+W", "Ctrl+Super+LeftBrace", "Ctrl+Super+RightBrace"}
	sweep := []string{"Esc", "Minus", "Equal", "Backspace", "Tab", "LeftBrace", "RightBrace", "Enter", "Semicolon", "Apostrophe", "Grave", "Backslash",
		"Comma", "Dot", "Slash", "Space", "102nd", "Home", "Up", "PageUp", "Left", "Right", "End", "Down", "PageDown", "Insert", "Delete"}
	for _, key := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		sweep = append(sweep, string(key))
	}
	for i := 1; i <= 12; i++ {
		sweep = append(sweep, fmt.Sprintf("F%d", i))
	}
	for _, key := range sweep {
		for _, keys := range [][]string{{"cmd", key}, {"cmd", "shift", key}} {
			for _, combo := range run(terminal, keys) {
				if strings.HasPrefix(combo, "Ctrl+") && !strings.Contains(combo, "Shift") && !slices.Contains(allowed, combo) {
					t.Errorf("terminal received control shortcut %s from %v", combo, keys)
				}
			}
		}
	}

	// No Mac chord may reach a GNOME shortcut that the chord does not mean on
	// the Mac. These are the Ubuntu 26.04 defaults of GNOME 50, its dock and
	// tiling extensions, and the IBus emoji keys, as combos() names them.
	gnome := map[string]bool{"tap:Super": true}
	for _, binding := range strings.Fields(`Alt+Space Alt+F2 Alt+F4 Alt+F5 Alt+F6 Alt+Shift+F6 Alt+F7 Alt+F8 Alt+F10
		Alt+Esc Alt+Shift+Esc Alt+Tab Alt+Shift+Tab Alt+Grave Alt+Shift+Grave Alt+SysRq SysRq Shift+SysRq
		Ctrl+Alt+Esc Ctrl+Alt+Shift+Esc Ctrl+Alt+Tab Ctrl+Alt+Shift+Tab Ctrl+Alt+Delete Ctrl+Alt+T Ctrl+Alt+D Ctrl+Alt+Shift+R
		Ctrl+Alt+Up Ctrl+Alt+Down Ctrl+Alt+Left Ctrl+Alt+Right Ctrl+Alt+Shift+Up Ctrl+Alt+Shift+Down Ctrl+Alt+Shift+Left Ctrl+Alt+Shift+Right
		Super+Tab Shift+Super+Tab Super+Grave Shift+Super+Grave Super+Space Shift+Super+Space Super+Esc Shift+Super+Esc
		Super+A Super+D Super+H Super+L Super+M Super+N Super+O Super+P Super+Q Super+S Super+V Super+F1 Super+Dot Super+Semicolon
		Ctrl+Super+D Ctrl+Super+0 Alt+Super+8 Alt+Super+Equal Alt+Super+Minus Alt+Super+S
		Super+Up Super+Down Super+Left Super+Right Super+Home Super+End Super+PageUp Super+PageDown
		Shift+Super+Up Shift+Super+Down Shift+Super+Left Shift+Super+Right Shift+Super+Home Shift+Super+End Shift+Super+PageUp Shift+Super+PageDown
		Alt+Super+Up Alt+Super+Down Alt+Super+Left Alt+Super+Right Alt+Shift+Super+Left Alt+Shift+Super+Right`) {
		gnome[binding] = true
	}
	intended := map[string]string{
		"cmd+Tab": "Super+Tab", "cmd+shift+Tab": "Shift+Super+Tab", "cmd+Grave": "Super+Grave", "cmd+shift+Grave": "Shift+Super+Grave",
		"cmd+Space": "tap:Super", "ctrl+Up": "tap:Super", "ctrl+Down": "tap:Super", "ctrl+Left": "Super+PageUp", "ctrl+Right": "Super+PageDown",
		"cmd+H": "Super+H", "cmd+M": "Super+H", "ctrl+cmd+Q": "Super+L", "ctrl+cmd+Space": "Super+Dot",
		"cmd+shift+3": "Shift+SysRq", "cmd+shift+4": "SysRq", "cmd+shift+5": "SysRq", "cmd+shift+W": "Alt+F4",
		"ctrl+opt+L": "Alt+Esc", "ctrl+opt+H": "Alt+Shift+Esc", "opt+Tab": "Alt+Tab", "opt+shift+Tab": "Alt+Shift+Tab",
	}
	for i := 1; i <= 12; i++ {
		gnome[fmt.Sprintf("Ctrl+Alt+F%d", i)] = true
	}
	for i := 1; i <= 9; i++ {
		for _, binding := range []string{"Super+%d", "Shift+Super+%d", "Ctrl+Super+%d"} {
			gnome[fmt.Sprintf(binding, i)] = true
		}
		intended[fmt.Sprintf("opt+%d", i)] = fmt.Sprintf("Super+%d", i)
		intended[fmt.Sprintf("opt+shift+%d", i)] = fmt.Sprintf("Shift+Super+%d", i)
	}
	chords := [][]string{}
	for set := 1; set < 16; set++ {
		modifiers := []string{}
		for bit, name := range []string{"ctrl", "opt", "cmd", "shift"} {
			if set&(1<<bit) != 0 {
				modifiers = append(modifiers, name)
			}
		}
		if set == 8 {
			continue // Shift alone
		}
		for _, key := range sweep {
			chords = append(chords, append(slices.Clone(modifiers), key))
		}
	}
	// The right Option key gets the same chords as the left one.
	for _, keys := range slices.Clone(chords) {
		if i := slices.Index(keys, "opt"); i >= 0 {
			right := slices.Clone(keys)
			right[i] = "ropt"
			chords = append(chords, right)
		}
	}
	// Each chord ends at a pass-through key, so no quiet period is needed.
	sentinel := linuxKeyCodes["F24"]
	keyboard.delay = 2 * time.Millisecond
	for _, window := range []string{editor, browser, files, terminal, otherTerminal} {
		if output, err := exec.CommandContext(ctx, tools["xdotool"], "windowfocus", "--sync", focus[window]).CombinedOutput(); err != nil {
			t.Fatal("cannot focus", window, err, nativeDiagnostic(output))
		}
		drain(events, 100*time.Millisecond)
		for _, keys := range chords {
			keyboard.press(t, keys...)
			keyboard.emit(t, sentinel, 1)
			keyboard.emit(t, sentinel, 0)
			got := []evdevEvent{}
			for done := false; !done; {
				select {
				case e, ok := <-events:
					if !ok {
						t.Fatal("the remapper closed its keyboard")
					}
					if e.Type == 1 && e.Code == sentinel {
						done = e.Value == 0
					} else {
						got = append(got, e)
					}
				case <-time.After(5 * time.Second):
					t.Fatal(window, keys, "did not reach the sentinel key")
				}
			}
			chord := strings.Join(keys, "+")
			combos, held := combos(got)
			if len(held) != 0 {
				t.Errorf("%s %s left keys held: %v", window, chord, held)
			}
			for _, combo := range combos {
				if gnome[combo] && intended[strings.Replace(chord, "ropt", "opt", 1)] != combo {
					t.Errorf("%s: %s reaches the GNOME shortcut %s", window, chord, combo)
				}
			}
		}
	}
	keyboard.delay = 15 * time.Millisecond
	if t.Failed() {
		return
	}

	// Updating the checkout replaces the keymap file (git unlinks it and creates
	// a new one). The running remapper must reload it without a new login.
	source := filepath.Join(repository, "xremap", "config.yml")
	before, _ := os.ReadFile(logPath)
	original, err := os.ReadFile(source)
	var st unix.Stat_t
	if err != nil || unix.Stat(source, &st) != nil {
		t.Fatal("cannot read the checkout keymap", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(os.WriteFile(source, original, os.FileMode(st.Mode&0o777)), os.Chown(source, int(st.Uid), int(st.Gid))); err != nil {
		t.Fatal("cannot restore the checkout keymap", err)
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if output, _ := os.ReadFile(logPath); len(output) > len(before) && strings.Contains(string(output[len(before):]), "Config Reloaded") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the remapper did not reload a checkout update of its keymap")
		}
	}
	check(editor, k("opt", "1"), "Super+1")

	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, path := range []string{keyboardAccessRules, autostart, config} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("removal left", path, err)
		}
	}
}
