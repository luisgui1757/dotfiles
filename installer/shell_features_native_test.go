package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeArchiveSelectedShellFeaturesLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("enable real upstream native lifecycle")
	}
	t.Run("native-default-shell", func(t *testing.T) { selectedShellLifecycle(t, runtime.GOOS == "windows") })
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("pwsh"); err == nil {
			t.Run("powershell-script-composition", func(t *testing.T) { selectedShellLifecycle(t, true) })
		} else {
			t.Log("PowerShell script composition requires pwsh; native Windows lane covers it")
		}
	}
}

func selectedShellLifecycle(t *testing.T, powershell bool) {

	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	target := nativePlatform(t)
	if powershell {
		target.OS = "windows"
		target.Arch = "amd64"
	}
	c, err := NewNativeController(repository, "feature-lifecycle", filepath.Join(home, "state"), target, folders)
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	if powershell && runtime.GOOS != "windows" {
		// Cross-platform PowerShell script proof on actual host executables.
		// This does not claim Windows filesystem/ACL or terminal verification.
		pins, err := DefaultArchivePins(nativePlatform(t))
		if err != nil {
			t.Fatal(err)
		}
		pins["powershell.psfzf"] = d.Archives.Pins["powershell.psfzf"]
		d.Archives.Pins = pins
	}
	selected := []string{"starship", "fzf", "zoxide"}
	if !powershell {
		selected = append(selected, "lsd")
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: selected})
	verify := func(zoxide bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var cmd *exec.Cmd
		if powershell {
			// This subprocess tests real loaded functions with an explicit
			// invocation fixture. Native terminal interaction is a separate gate.
			profile := filepath.Join(folders.Documents, "PowerShell", "profile.ps1")
			command, err := d.Archives.CommandPath("tool.starship", "starship")
			if err != nil {
				t.Fatal(err)
			}
			script := "$global:DotfilesProfileInvocationContext = @{UserInteractive=$true;HostName='ConsoleHost';CommandLineArgs=@('pwsh','-NoExit');InputRedirected=$false;OutputRedirected=$false;ErrorRedirected=$false;CIValue=''}; . " + powershellLiteral(profile) + "; if ((Get-Command starship).Source -ne " + powershellLiteral(command) + ") { throw 'wrong Starship executable' }; starship --version; fzf --version; Get-Command Invoke-Fzf | Select-Object -ExpandProperty Name; "
			if zoxide {
				script += "Get-Command z | Select-Object -ExpandProperty Name; zoxide --version"
			} else {
				script += "if (Get-Command z -ErrorAction SilentlyContinue) { throw 'removed zoxide hook remains' }"
			}
			cmd = exec.CommandContext(ctx, "pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
		} else {
			script := "starship --version; fzf --version; alias ls; "
			if zoxide {
				script += "type z; zoxide --version"
			} else {
				script += "if declare -F __zoxide_z >/dev/null; then exit 31; fi"
			}
			cmd = exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--rcfile", filepath.Join(home, ".bashrc"), "-ic", script)
		}
		cmd.Dir = home
		cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+folders.Config, "ZDOTDIR="+home, "TERM=xterm-256color")
		output, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(output), "starship "+d.Archives.Pins["tool.starship"].Version) || !strings.Contains(string(output), d.Archives.Pins["tool.fzf"].Version) || zoxide && !strings.Contains(string(output), "zoxide "+d.Archives.Pins["tool.zoxide"].Version) {
			t.Fatalf("selected hooks zoxide=%v: %s %v", zoxide, output, err)
		}
		t.Logf("selected hooks zoxide=%v: %s", zoxide, output)
	}
	verify(true)
	initName, personal := "init.sh", "export DOTFILES_PERSONAL_VALUE=preserved\n"
	if powershell {
		initName, personal = "init.ps1", "$env:DOTFILES_PERSONAL_VALUE = 'preserved'\n"
	}
	edited := filepath.Join(home, "state", "shells", initName)
	data, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edited, append(data, []byte(personal)...), 0600); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
	selected = []string{"starship", "fzf"}
	if !powershell {
		selected = append(selected, "lsd")
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: selected})
	verify(false)
	if _, err := os.Lstat(d.Archives.currentLink("tool.zoxide")); !os.IsNotExist(err) {
		t.Fatal("unused private dependency survived removal", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, id := range []string{"tool.starship", "tool.fzf", "tool.zoxide"} {
		if _, err := os.Lstat(d.Archives.currentLink(id)); !os.IsNotExist(err) {
			t.Fatal("private dependency survived full removal", id, err)
		}
	}
	for _, entry := range d.Profiles.Targets["integration.shells"] {
		if entry.Path == edited {
			data, err := os.ReadFile(edited)
			expected := personal
			if powershell {
				expected = "\ufeff" + expected
			}
			if err != nil || string(data) != expected {
				t.Fatalf("outside shell edits lost: %q %v", data, err)
			}
			continue
		}
		if _, err := os.Lstat(entry.Path); !os.IsNotExist(err) {
			t.Fatal("newly created profile survived full removal", entry.Path, err)
		}
	}
}
