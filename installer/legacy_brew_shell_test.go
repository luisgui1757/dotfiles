package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The fixture relocates the native system into a temporary root. getent and
// chsh are small programs over a local passwd file and a separate
// directory-service file; Homebrew is a program over a real Cellar; the
// /etc/shells edit runs the real edit script. Every query and command crosses
// the real process boundary. Nothing outside the temporary directory changes.

type legacyBrewShellHost struct {
	t                  *testing.T
	d                  *legacyRetirementDriver
	home, root, prefix string
	shells             string // the original /etc/shells
}

const legacyBrewShellGetent = `#!/bin/sh
set -eu
root=$(dirname "$(dirname "$(dirname "$0")")")
directory=1
if [ "$1" = -s ]; then
	[ "$2" = files ] || exit 64
	directory=0
	shift 2
fi
[ "$1" = passwd ] || exit 64
shift
records() {
	cat "$root/etc/passwd"
	if [ "$directory" = 1 ] && [ -f "$root/etc/directory-passwd" ]; then cat "$root/etc/directory-passwd"; fi
}
if [ "$#" -eq 0 ]; then
	records
	exit 0
fi
records | awk -F: -v name="$1" '$1 == name { print; found = 1; exit } END { exit !found }' || exit 2
`

// chsh changes only local records, like the native command.
const legacyBrewShellChsh = `#!/bin/sh
set -eu
root=$(dirname "$(dirname "$(dirname "$0")")")
[ "$#" -eq 3 ] && [ "$1" = -s ] || exit 64
db=$root/etc/passwd
awk -F: -v name="$3" '$1 == name { found = 1 } END { exit !found }' "$db" || { echo "chsh: user '$3' does not exist in /etc/passwd" >&2; exit 1; }
awk -F: -v OFS=: -v name="$3" -v shell="$2" '$1 == name { $7 = shell } { print }' "$db" > "$db.new"
mv "$db.new" "$db"
printf '%s %s\n' "$3" "$2" >> "$root/chsh.log"
`

const legacyBrewShellBrew = `#!/bin/sh
set -eu
prefix=$(dirname "$(dirname "$0")")
case "$*" in
"info --json=v2 --installed")
	if [ -d "$prefix/Cellar/zsh" ]; then
		echo '{"formulae":[{"name":"zsh","full_name":"zsh","linked_keg":"5.9.2","installed":[{"version":"5.9.2","installed_on_request":true,"runtime_dependencies":[]}]}],"casks":[]}'
	else
		echo '{"formulae":[],"casks":[]}'
	fi ;;
"uninstall --formula --force zsh")
	rm -rf "$prefix/Cellar/zsh" "$prefix/opt/zsh" "$prefix/bin/zsh" "$prefix/bin/zsh-5.9" ;;
*)
	echo "unexpected brew $*" >&2
	exit 64 ;;
esac
`

func newLegacyBrewShellHost(t *testing.T, installed bool, selected ...string) *legacyBrewShellHost {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux login-shell and Linuxbrew process boundary")
	}
	d, home := legacyStepFixture(t)
	h := &legacyBrewShellHost{t: t, d: d, home: home, root: filepath.Join(home, "system"), prefix: filepath.Join(home, "linuxbrew")}
	d.systemRoot = h.root
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "linux", Arch: "amd64"}
	d.Normal.Catalog, d.Normal.Context = catalog, target
	d.Platform = NativePlatform{Context: target, Homebrew: &HomebrewLocation{Program: filepath.Join(h.prefix, "bin", "brew"), Prefix: h.prefix, Cellar: filepath.Join(h.prefix, "Cellar")}}
	if err := SaveState(d.Normal.StatePath, State{Schema: 1, Context: target, Target: home, Source: "fixture", Generation: 1, Selected: append([]string{}, selected...), Receipts: map[string]Receipt{}}); err != nil {
		t.Fatal(err)
	}
	h.program("/usr/bin/getent", legacyBrewShellGetent)
	h.program("/usr/bin/chsh", legacyBrewShellChsh)
	h.program("/bin/bash", "#!/bin/sh\necho 'GNU bash, version 5.2.37(1)-release (x86_64-pc-linux-gnu)'\n")
	h.program("/usr/bin/zsh", "#!/bin/sh\necho 'zsh 5.9 (x86_64-debian-linux-gnu)'\n")
	h.executable(d.Platform.Homebrew.Program, legacyBrewShellBrew)
	if installed {
		h.executable(filepath.Join(h.prefix, "Cellar", "zsh", "5.9.2", "bin", "zsh"), "#!/bin/sh\n")
		h.link(filepath.Join(h.prefix, "bin", "zsh"), "../Cellar/zsh/5.9.2/bin/zsh")
		h.link(filepath.Join(h.prefix, "opt", "zsh"), "../Cellar/zsh/5.9.2")
	}
	h.write("/etc/passwd", "root:x:0:0:root:/root:"+h.sys("/bin/bash")+"\nfixture:x:1000:1000:Fixture:"+home+":"+h.login()+"\n")
	h.shells = "# /etc/shells: valid login shells\n/bin/sh\n" + h.sys("/bin/bash") + "\n" + h.sys("/usr/bin/zsh") + "\n" + h.login() + "\n/usr/bin/tmux\n"
	h.write("/etc/shells", h.shells)
	return h
}

func (h *legacyBrewShellHost) sys(path string) string { return filepath.Join(h.root, path) }

// login is the released installer's chsh target, `command -v zsh`.
func (h *legacyBrewShellHost) login() string { return filepath.Join(h.prefix, "bin", "zsh") }

func (h *legacyBrewShellHost) write(path, content string) {
	h.t.Helper()
	writeConfigFixture(h.t, h.sys(path), content)
}

func (h *legacyBrewShellHost) read(path string) string {
	h.t.Helper()
	data, err := os.ReadFile(h.sys(path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.t.Fatal(err)
	}
	return string(data)
}

func (h *legacyBrewShellHost) executable(path, content string) {
	h.t.Helper()
	writeConfigFixture(h.t, path, content)
	if err := os.Chmod(path, 0755); err != nil {
		h.t.Fatal(err)
	}
}

func (h *legacyBrewShellHost) program(path, content string) { h.executable(h.sys(path), content) }

func (h *legacyBrewShellHost) link(path, target string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		h.t.Fatal(err)
	}
}

// account replaces one local account record.
func (h *legacyBrewShellHost) account(name, record string) {
	h.t.Helper()
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(h.read("/etc/passwd"), "\n"), "\n") {
		if !strings.HasPrefix(line, name+":") {
			lines = append(lines, line)
		}
	}
	if record != "" {
		lines = append(lines, record)
	}
	h.write("/etc/passwd", strings.Join(lines, "\n")+"\n")
}

func (h *legacyBrewShellHost) shell(name string) string {
	h.t.Helper()
	for _, line := range strings.Split(h.read("/etc/passwd"), "\n") {
		if fields := strings.Split(line, ":"); len(fields) == 7 && fields[0] == name {
			return fields[6]
		}
	}
	h.t.Fatal("no local account", name)
	return ""
}

func (h *legacyBrewShellHost) inspect() legacyRetirementScope {
	h.t.Helper()
	scope, err := h.d.inspect(context.Background(), "retire.delta.zsh")
	if err != nil {
		h.t.Fatal(err)
	}
	return scope
}

func (h *legacyBrewShellHost) intent(scope legacyRetirementScope) *legacyRetirementIntent {
	h.t.Helper()
	return legacyTestIntent(h.t, h.d, scope.Steps...)
}

func (h *legacyBrewShellHost) run(intent *legacyRetirementIntent, index int) error {
	return h.d.executeStep(context.Background(), intent, index, false)
}

func (h *legacyBrewShellHost) installed() bool {
	h.t.Helper()
	_, err := os.Lstat(filepath.Join(h.prefix, "Cellar", "zsh"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.t.Fatal(err)
	}
	return err == nil
}

func legacyStepGates(step legacyRetirementStep) []string {
	kinds := []string{}
	for _, gate := range step.Gates {
		kinds = append(kinds, gate.Kind)
	}
	return kinds
}

// The released v0.4.4 installer left the account on Linuxbrew's zsh. Its first
// retirement, and a retirement already finished by an earlier release, both
// change that login to a healthy registered native shell first: the system
// bash, or setup's zsh when it is selected. Only the released registration
// leaves /etc/shells.
func TestLegacyLinuxbrewZshRetirementPreservesWorkingLogin(t *testing.T) {
	for _, installed := range []bool{true, false} {
		for _, selected := range [][]string{{}, {"zsh"}} {
			name := map[bool]string{true: "installed", false: "already removed"}[installed] + map[bool]string{true: " to setup zsh", false: " to system bash"}[len(selected) != 0]
			t.Run(name, func(t *testing.T) {
				h := newLegacyBrewShellHost(t, installed, selected...)
				replacement := h.sys("/bin/bash")
				if len(selected) != 0 {
					replacement = h.sys("/usr/bin/zsh")
				}
				scope := h.inspect()
				want := 2
				if installed {
					want = 3
				}
				if scope.Blocked != "" || len(scope.Retained) != 0 || len(scope.Steps) != want {
					t.Fatal("the stranded login is not repaired before and with the formula's retirement", legacyStepLabels(scope.Steps), scope.Retained, scope.Blocked)
				}
				switchStep := scope.Steps[0]
				if switchStep.Command == nil || !switchStep.Privileged || !slices.Equal(legacyFakeArgv(*switchStep.Command), []string{h.sys("/usr/bin/chsh"), "-s", replacement, "fixture"}) ||
					!slices.Equal(legacyStepGates(switchStep), []string{"account", "local-account", "native-login-shell"}) {
					t.Fatal("the login switch is not an exact chsh bound to the account and replacement", switchStep)
				}
				if installed && (!slices.Contains(scope.Steps[1].Command.Arguments, "uninstall") || !slices.Contains(legacyStepGates(scope.Steps[1]), "brew-login-shells")) {
					t.Fatal("Homebrew's uninstall does not follow the switch with a fresh login recheck", scope.Steps[1])
				}
				last := scope.Steps[len(scope.Steps)-1]
				if last.Kind != "edit" || last.Path != h.sys("/etc/shells") || !slices.Contains(legacyStepGates(last), "brew-login-shells") {
					t.Fatal("the released registration is not removed after the login no longer uses it", last)
				}
				intent := h.intent(scope)
				for i := range scope.Steps {
					if err := h.run(intent, i); err != nil {
						t.Fatal(scope.Steps[i].Label, err)
					}
				}
				if h.shell("fixture") != replacement || h.shell("root") != h.sys("/bin/bash") || h.read("/chsh.log") != "fixture "+replacement+"\n" {
					t.Fatal("only this account's login shell should change", h.read("/etc/passwd"), h.read("/chsh.log"))
				}
				if h.installed() || h.read("/etc/shells") != strings.Replace(h.shells, h.login()+"\n", "", 1) {
					t.Fatal("the formula or its exact registration remains, or another line changed", h.read("/etc/shells"))
				}
				if after := h.inspect(); after.Blocked != "" || len(after.Steps) != 0 || len(after.Retained) != 0 {
					t.Fatal("retirement remains unfinished", legacyStepLabels(after.Steps), after.Retained, after.Blocked)
				}
			})
		}
	}
}

// Another account whose login reaches the formula, here through its own link,
// keeps the formula installed and is never changed. Once the formula is gone,
// this account is still repaired while the other account is disclosed.
func TestLegacyLinuxbrewZshOtherAccountIsNeverChanged(t *testing.T) {
	for _, installed := range []bool{true, false} {
		t.Run(map[bool]string{true: "installed", false: "already removed"}[installed], func(t *testing.T) {
			h := newLegacyBrewShellHost(t, installed)
			personal := filepath.Join(h.home, "other", "zsh")
			h.link(personal, h.login())
			h.write("/etc/directory-passwd", "other:x:2000:2000:Other:/home/other:"+personal+"\n")
			scope := h.inspect()
			retained := strings.Join(scope.Retained, "; ")
			if !strings.Contains(retained, "other") {
				t.Fatal("the other account's login was not disclosed", scope.Retained)
			}
			if installed {
				if len(scope.Steps) != 0 || !strings.Contains(retained, "preserved") {
					t.Fatal("the formula another account logs into was planned for removal", legacyStepLabels(scope.Steps))
				}
				return
			}
			if len(scope.Steps) != 1 || scope.Steps[0].Command == nil {
				t.Fatal("this account's repair must not remove a registration another account names", legacyStepLabels(scope.Steps))
			}
			if err := h.run(h.intent(scope), 0); err != nil {
				t.Fatal(err)
			}
			if h.shell("fixture") != h.sys("/bin/bash") || !strings.Contains(h.read("/etc/directory-passwd"), personal) || h.read("/etc/shells") != h.shells {
				t.Fatal("another account or the registration it names changed", h.read("/etc/directory-passwd"), h.read("/etc/shells"))
			}
		})
	}
}

// Approval binds the account, its original shell and the replacement; the
// final login recheck runs immediately before Homebrew's uninstall.
func TestLegacyLinuxbrewZshChangesAfterPreviewAreRefused(t *testing.T) {
	for _, change := range []string{"account identity", "replacement registration", "replacement executable", "outside login"} {
		t.Run(change, func(t *testing.T) {
			h := newLegacyBrewShellHost(t, true)
			scope := h.inspect()
			if len(scope.Steps) != 3 {
				t.Fatal(legacyStepLabels(scope.Steps), scope.Retained)
			}
			intent := h.intent(scope)
			step := 0
			switch change {
			case "account identity":
				h.account("fixture", "fixture:x:1001:1000:Fixture:"+h.home+":"+h.login())
			case "replacement registration":
				h.write("/etc/shells", strings.Replace(h.shells, h.sys("/bin/bash")+"\n", "", 1))
			case "replacement executable":
				h.program("/bin/bash", "#!/bin/sh\necho 'GNU bash, version 5.3.0(1)-release (x86_64-pc-linux-gnu)'\n")
			case "outside login":
				if err := h.run(intent, 0); err != nil {
					t.Fatal(err)
				}
				h.account("other", "other:x:2000:2000:Other:/home/other:"+h.login())
				step = 1
			}
			if err := h.run(intent, step); err == nil {
				t.Fatal("a change after approval was not refused", scope.Steps[step].Label)
			}
			if !h.installed() || h.read("/etc/shells") == strings.Replace(h.shells, h.login()+"\n", "", 1) {
				t.Fatal("the formula or its registration was removed after a refused gate")
			}
			if change != "outside login" && (h.read("/chsh.log") != "" || h.shell("fixture") != h.login()) {
				t.Fatal("the login changed after a refused gate", h.read("/chsh.log"))
			}
		})
	}
}

// No replacement is installed or registered merely to repair a login, and a
// directory account is not changed through the local file: the formula and
// the stranded login are disclosed instead.
func TestLegacyLinuxbrewZshUnusableReplacementIsDisclosed(t *testing.T) {
	cases := map[string]func(*legacyBrewShellHost){
		"unregistered": func(h *legacyBrewShellHost) {
			h.write("/etc/shells", strings.Replace(h.shells, h.sys("/bin/bash")+"\n", "", 1))
		},
		"not executable": func(h *legacyBrewShellHost) {
			if err := os.Chmod(h.sys("/bin/bash"), 0644); err != nil {
				h.t.Fatal(err)
			}
		},
		"wrong program": func(h *legacyBrewShellHost) { h.program("/bin/bash", "#!/bin/sh\necho 'dash'\n") },
		"failed start":  func(h *legacyBrewShellHost) { h.program("/bin/bash", "#!/bin/sh\nexit 126\n") },
		"selected zsh not installed": func(h *legacyBrewShellHost) {
			if err := os.Remove(h.sys("/usr/bin/zsh")); err != nil {
				h.t.Fatal(err)
			}
		},
		"directory account": func(h *legacyBrewShellHost) {
			h.account("fixture", "")
			h.write("/etc/directory-passwd", "fixture:x:1000:1000:Fixture:"+h.home+":"+h.login()+"\n")
		},
	}
	for name, prepare := range cases {
		for _, installed := range []bool{true, false} {
			t.Run(name+map[bool]string{true: " installed", false: " already removed"}[installed], func(t *testing.T) {
				selected := []string{}
				if name == "selected zsh not installed" {
					selected = []string{"zsh"}
				}
				h := newLegacyBrewShellHost(t, installed, selected...)
				prepare(h)
				scope := h.inspect()
				if len(scope.Steps) != 0 || len(scope.Retained) == 0 || !strings.Contains(strings.Join(scope.Retained, "; "), "this account's login shell "+h.login()) {
					t.Fatal("an unusable replacement or unchangeable account was not refused and disclosed", legacyStepLabels(scope.Steps), scope.Retained)
				}
				if h.installed() != installed {
					t.Fatal("inspection changed the formula")
				}
			})
		}
	}
}

// An account inventory that cannot be read completely is an inspection
// failure, never proof that no login uses the formula.
func TestLegacyLinuxbrewZshIncompleteAccountInventoryFails(t *testing.T) {
	for _, installed := range []bool{true, false} {
		h := newLegacyBrewShellHost(t, installed)
		h.write("/etc/directory-passwd", "broken:x:2000\n")
		if scope, err := h.d.inspect(context.Background(), "retire.delta.zsh"); err == nil {
			t.Fatal("a malformed account record was treated as absence", installed, legacyStepLabels(scope.Steps))
		}
	}
}
