package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// keyboardAccessFixture runs the fixed recipe against fake system programs with
// GNU argument shapes, recording outcomes like the native worker.
func keyboardAccessFixture(t *testing.T) (*KeyboardAccessDriver, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the keyboard access rule is Linux-only; fixtures use POSIX shell programs")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin, rules := filepath.Join(root, "bin"), filepath.Join(root, "etc", "udev", "rules.d")
	if err := os.MkdirAll(rules, 0755); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		// install -m 0644 -o root -g root -T SOURCE DEST
		"install": `shift 7; cp "$1" "$2" && chmod 0644 "$2"`,
		// ln -T TARGET LINK fails rather than replace LINK.
		"ln":      `if [ -e "$3" ] || [ -e "` + filepath.Join(root, "fail-link") + `" ]; then echo "ln: failed to create hard link" >&2; exit 1; fi; /bin/ln "$2" "$3"`,
		"rm":      `/bin/rm -f -- "$3"`,
		"mv":      `/bin/mv "$2" "$3"`,
		"udevadm": `echo "$*" >> "` + filepath.Join(root, "udevadm.log") + `"`,
	} {
		writeConfigFixture(t, filepath.Join(bin, name), "#!/bin/sh\n"+script+"\n")
		if err := os.Chmod(filepath.Join(bin, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	worker := filepath.Join(root, "worker")
	d := &KeyboardAccessDriver{Directory: filepath.Join(root, "state"), WorkerDirectory: worker, Rules: filepath.Join(rules, "70-dotfiles-mac-keyboard.rules"), Owner: os.Getuid(),
		Programs: keyboardAccessPrograms{Install: filepath.Join(bin, "install"), Link: filepath.Join(bin, "ln"), Remove: filepath.Join(bin, "rm"), Move: filepath.Join(bin, "mv"), Udevadm: filepath.Join(bin, "udevadm")}}
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		output, err := exec.CommandContext(ctx, command.Program, command.Arguments...).CombinedOutput()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			return nil, err
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		return output, saveDocument(filepath.Join(worker, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{Output: output, ExitCode: &code}})
	}
	return d, root
}

var keyboardAccessResource = Resource{ID: "system.keyboard-access", Name: "Keyboard access", Action: "keyboard-access", Scope: "user"}

func TestKeyboardAccessRuleLifecycle(t *testing.T) {
	d, root := keyboardAccessFixture(t)
	ctx := context.Background()
	r := keyboardAccessResource
	if o, err := d.Observe(ctx, r, Receipt{}); err != nil || o.Present || o.Desired == "" || o.Pending != "" || !o.Privileged || o.Scope != "user" {
		t.Fatal("an absent rule must be installable, with administrator rights, and removed with its tool", o, err)
	}
	install := strings.Repeat("a", 64)
	installed, err := d.Apply(ctx, r, Operation{Action: "install"}, Receipt{OperationID: install, Status: "in-progress"})
	if err != nil || !installed.Present || !installed.Healthy {
		t.Fatal(installed, err)
	}
	data, err := os.ReadFile(d.Rules)
	if info, statErr := os.Stat(d.Rules); err != nil || statErr != nil || string(data) != string(keyboardAccessContent(install)) || info.Mode().Perm() != 0644 {
		t.Fatal("the rule must be the exact operation-named content, mode 0644", string(data), err)
	}
	if !strings.Contains(string(data), `TAG+="uaccess"`) || strings.Contains(string(data), "GROUP=") || strings.Contains(string(data), "MODE") {
		t.Fatal("access must follow the seat (uaccess), never a group or open mode", string(data))
	}
	entries, err := os.ReadDir(filepath.Dir(d.Rules))
	if err != nil || len(entries) != 1 {
		t.Fatal("the temporary copy must not remain beside the rule", entries, err)
	}
	if _, err := os.Stat(d.stagePath(install)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the private input must be removed after completion", err)
	}
	log, err := os.ReadFile(filepath.Join(root, "udevadm.log"))
	if err != nil || string(log) != "control --reload-rules\ntrigger --action=change --subsystem-match=misc --sysname-match=uinput\ntrigger --action=change --subsystem-match=input\n" {
		t.Fatal("udev must reload and re-apply seat access", string(log), err)
	}
	owned := Receipt{Ownership: "created", OperationID: install, Status: "ready", After: installed}
	if o, err := d.Observe(ctx, r, owned); err != nil || !o.Healthy || o.CompletedOperation != install {
		t.Fatal(o, err)
	}

	// A rule changed outside setup is preserved, never repaired or removed.
	writeConfigFixture(t, d.Rules, string(data)+"# local\n")
	if o, err := d.Observe(ctx, r, owned); err != nil || o.Healthy || !strings.Contains(o.HealthIssue, "changed after setup installed it") {
		t.Fatal(o, err)
	}
	remove := strings.Repeat("b", 64)
	if _, err := d.Remove(ctx, r, Receipt{Ownership: "created", OperationID: remove, Status: "in-progress", After: installed}); err == nil {
		t.Fatal("removed a changed rule")
	}
	if changed, err := os.ReadFile(d.Rules); err != nil || !strings.HasSuffix(string(changed), "# local\n") {
		t.Fatal("the changed rule was not preserved", err)
	}
	if err := os.WriteFile(d.Rules, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(d.intentPath(remove)); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	removed, err := d.Remove(ctx, r, Receipt{Ownership: "created", OperationID: remove, Status: "in-progress", After: installed})
	if err != nil || removed.Present {
		t.Fatal(removed, err)
	}
	if entries, err := os.ReadDir(filepath.Dir(d.Rules)); err != nil || len(entries) != 0 {
		t.Fatal("removal left the rule or its moved copy", entries, err)
	}
}

func TestKeyboardAccessNeverReplacesAnotherRule(t *testing.T) {
	d, _ := keyboardAccessFixture(t)
	ctx := context.Background()
	r := keyboardAccessResource
	writeConfigFixture(t, d.Rules, "KERNEL==\"uinput\", MODE=\"0666\"\n")
	o, err := d.Observe(ctx, r, Receipt{})
	if err != nil || !o.Present || o.Healthy || !strings.Contains(o.Pending, "never replaces or removes it") {
		t.Fatal(o, err)
	}
	if _, err := d.Apply(ctx, r, Operation{Action: "install"}, Receipt{OperationID: strings.Repeat("c", 64), Status: "in-progress"}); err == nil {
		t.Fatal("installed over an existing rule")
	}
	if data, err := os.ReadFile(d.Rules); err != nil || string(data) != "KERNEL==\"uinput\", MODE=\"0666\"\n" {
		t.Fatal("another rule changed", string(data), err)
	}
	// The same bytes with another operation, or tampered dotfiles bytes, are not ours.
	if _, ok := keyboardAccessOwner(append(keyboardAccessContent(strings.Repeat("d", 64)), '\n')); ok {
		t.Fatal("accepted modified rule bytes")
	}
	d.Issue = "APT is unavailable"
	if o, err := d.Observe(ctx, r, Receipt{}); err != nil || !o.Unknown || o.Pending != "APT is unavailable" {
		t.Fatal(o, err)
	}
}

// A failed hard link (an outside rule appeared after approval, here forced)
// keeps the intent; recovery resumes only the saved operation.
func TestKeyboardAccessResumesItsSavedOperation(t *testing.T) {
	d, root := keyboardAccessFixture(t)
	ctx := context.Background()
	r := keyboardAccessResource
	op := strings.Repeat("e", 64)
	writeConfigFixture(t, filepath.Join(root, "fail-link"), "")
	receipt := Receipt{OperationID: op, Status: "in-progress"}
	if _, err := d.Apply(ctx, r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("a failed link reported success")
	}
	o, err := d.Observe(ctx, r, receipt)
	if err != nil || o.ResourceResume == nil || o.ResourceResume.Operation != op || o.Present {
		t.Fatal("interrupted installation must offer only its saved operation", o, err)
	}
	if err := os.Remove(filepath.Join(root, "fail-link")); err != nil {
		t.Fatal(err)
	}
	resumed, err := d.ResumeResource(ctx, r, Operation{Action: "install"}, receipt)
	if err != nil || !resumed.Healthy {
		t.Fatal(resumed, err)
	}
	if data, err := os.ReadFile(d.Rules); err != nil || string(data) != string(keyboardAccessContent(op)) {
		t.Fatal("recovery did not finish the saved operation", err)
	}
}

// The hosted lifecycle found the rule kept after deselection: machine scope
// means a possibly shared resource. This grant serves only its capability.
func TestKeyboardAccessLeavesWithItsTool(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	closure, err := c.Closure([]string{"mac-keyboard"}, linux)
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Selected: []string{"mac-keyboard"}, Receipts: map[string]Receipt{}}
	observed := map[string]Observation{}
	for _, id := range closure {
		o := Observation{Provider: "fixture", Identity: id, Present: true, Healthy: true, Fingerprint: "installed"}
		if id == "system.keyboard-access" {
			o.Provider, o.Scope, o.Privileged = "keyboard-access", "user", true
		}
		observed[id], state.Receipts[id] = o, Receipt{Ownership: "created", Status: "ready", After: o}
	}
	plan, err := PlanChanges(c, linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if op := operation(t, plan, "system.keyboard-access"); op.Action != "remove" || !op.Privileged {
		t.Fatal("unticking the tool must remove its keyboard grant, with administrator rights", op)
	}
}
