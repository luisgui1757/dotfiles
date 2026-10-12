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
	"LeftMeta": 125, "RightMeta": 126,
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
var macPhysical = map[string]string{"cmd": "LeftAlt", "opt": "LeftMeta", "ctrl": "LeftCtrl", "shift": "LeftShift"}

type uinputKeyboard struct{ file *os.File }

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
	k := &uinputKeyboard{file: f}
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
	time.Sleep(15 * time.Millisecond)
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
// each non-modifier press with the modifiers then held, and bare Super taps.
func combos(events []evdevEvent) ([]string, map[uint16]bool) {
	held, result := map[uint16]bool{}, []string{}
	superAlone := false
	for _, e := range events {
		if e.Type != 1 || e.Value == 2 {
			continue
		}
		modifier, isModifier := linuxModifiers[e.Code]
		switch {
		case isModifier && e.Value == 1:
			held[e.Code] = true
			superAlone = modifier == "Super"
		case isModifier:
			delete(held, e.Code)
			if modifier == "Super" && superAlone {
				result = append(result, "tap:Super")
			}
			superAlone = false
		case e.Value == 1:
			superAlone = false
			names := []string{}
			for _, name := range []string{"Ctrl", "Alt", "Shift", "Super"} {
				for code := range held {
					if linuxModifiers[code] == name && !slices.Contains(names, name) {
						names = append(names, name)
					}
				}
			}
			result = append(result, strings.Join(append(names, linuxKeyName(e.Code)), "+"))
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
	uid, _, mode, ownerErr := legacyFileOwner(keyboardAccessRules)
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

	keyboard := newUinputKeyboard(t, "dotfiles-test-keyboard")
	linuxInputEventNode(t, regexp.MustCompile(`^dotfiles-test-keyboard$`), 10*time.Second)
	// Start xremap exactly as the login session does.
	// xremap outlives gio and keeps its output open: log to a file, not a pipe.
	logPath := filepath.Join(t.TempDir(), "xremap.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	launch := exec.CommandContext(ctx, tools["gio"], "launch", autostart)
	launch.Env = append(slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, "XDG_SESSION_TYPE=") }), "XDG_SESSION_TYPE=x11")
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
	check(editor, k("ctrl", "Left"), "Super+PageUp")
	check(editor, k("cmd", "shift", "3"), "Shift+SysRq")
	check(editor, k("cmd", "shift", "4"), "SysRq")
	check(editor, k("ctrl", "cmd", "Q"), "Super+L")
	check(editor, k("cmd", "H"), "Super+H")
	// Holding Cmd keeps Alt held across Tabs, so GNOME's switcher stays open.
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
	if pressed, _ := combos(held); !slices.Equal(pressed, []string{"Alt+Tab", "Alt+Tab"}) || len(switching) != 0 {
		t.Errorf("Cmd+Tab+Tab produced %q then %q, want Alt held across two Tabs", pressed, switching)
	}
	for _, e := range held {
		if e.Type == 1 && e.Code == linuxKeyCodes["LeftAlt"] && e.Value == 0 {
			t.Error("Alt was released while Cmd was still held")
		}
	}
	check(editor, k("C"), "C")
	check(browser, k("cmd", "LeftBrace"), "Alt+Left")
	check(browser, k("cmd", "opt", "Up"), "Ctrl+PageUp")
	check(browser, k("cmd", "T"), "Ctrl+T")
	check(files, k("cmd", "Backspace"), "Delete")
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
	// An unidentified window keeps plain Linux keys.
	check("", k("cmd", "C"), "Alt+C")
	check("", k("opt", "C"), "Super+C")
	check("", k("opt", "1"), "Super+1")
	check("", k("opt", "shift", "1"), "Shift+Super+1")
	check("", k("ctrl", "opt", "L"), "Ctrl+Super+L")

	// No Cmd combination may reach a terminal as a bare control character.
	allowed := []string{"Ctrl+A", "Ctrl+E", "Ctrl+U", "Ctrl+L", "Ctrl+Enter", "Ctrl+Comma", "Ctrl+Equal", "Ctrl+Minus", "Ctrl+0", "Ctrl+PageUp", "Ctrl+PageDown"}
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
