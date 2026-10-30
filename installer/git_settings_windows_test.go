package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsGitXDGCheckoutOverlapIsResourceLocal(t *testing.T) {
	c, source, _ := gitSettingsFixture(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(filepath.Dir(source)))
	unsetGitDiscoveryEnvironment(t, "GIT_CONFIG_GLOBAL")
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	d := c.Driver.(*NativeDriver)
	d.shellSelection = nil
	if err := configureGitSettings(d.Profiles, folders, filepath.Dir(filepath.Dir(source))); err != nil {
		t.Fatal(err)
	}
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
	if err != nil {
		t.Fatal("overlap blocked the whole controller", err)
	}
	found := false
	for _, op := range preview.Plan.Operations {
		if op.Resource == gitSettingsResource && op.Action == "pending" && strings.Contains(op.Reason, "inside the dotfiles checkout") {
			found = true
		}
	}
	if !found {
		t.Fatal("checkout Git file was writable", preview)
	}
}

func unsetGitDiscoveryEnvironment(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	t.Cleanup(func() {
		var err error
		if exists {
			err = os.Setenv(key, value)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Error(err)
		}
	})
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsGitDiscoveryMatchesNativeGit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("native Git is required for Windows configuration-path acceptance", err)
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"explicit-home", "drive-and-path", "missing-drive-path", "system-directory", "explicit-xdg"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "Git home ü")
			profile := filepath.Join(root, "profile")
			for _, path := range []string{home, profile} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, key := range []string{"HOME", "GIT_CONFIG_GLOBAL", "XDG_CONFIG_HOME"} {
				unsetGitDiscoveryEnvironment(t, key)
			}
			t.Setenv("USERPROFILE", profile)
			candidate := home
			if scenario == "missing-drive-path" {
				candidate = filepath.Join(root, "does-not-exist")
			} else if scenario == "system-directory" {
				candidate = system
			}
			volume := filepath.VolumeName(candidate)
			t.Setenv("HOMEDRIVE", volume)
			t.Setenv("HOMEPATH", strings.TrimPrefix(candidate, volume))
			if scenario == "explicit-home" {
				t.Setenv("HOME", home)
			} else if scenario == "explicit-xdg" {
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "separate configuration"))
			}
			folders, err := DiscoverConfigFolders()
			if err != nil || folders.GitIssue != "" {
				t.Fatal(folders.GitIssue, err)
			}
			command := exec.Command(git, "var", "GIT_CONFIG_GLOBAL")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatal(string(output), err)
			}
			found := false
			for _, path := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				found = found || strings.EqualFold(filepath.Clean(strings.TrimSpace(path)), folders.GitConfig)
			}
			if !found {
				t.Fatalf("Git reported %q; discovery chose %q", output, folders.GitConfig)
			}
		})
	}
}

func TestWindowsGitOverrideBlocksOnlyGit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "personal-only"))
	folders, err := DiscoverConfigFolders()
	if err != nil || !strings.Contains(folders.GitIssue, "GIT_CONFIG_GLOBAL") || folders.Home == "" || folders.Documents == "" {
		t.Fatal("Git override broke unrelated native folder discovery", folders.GitIssue, err)
	}
}

func TestWindowsGitDiscoveryReportsEveryCause(t *testing.T) {
	unsetGitDiscoveryEnvironment(t, "XDG_CONFIG_HOME")
	t.Setenv("HOME", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "personal-only"))
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	for _, cause := range []string{"Git HOME must resolve to an absolute path", "absolute canonical path", "GIT_CONFIG_GLOBAL"} {
		if !strings.Contains(folders.GitIssue, cause) {
			t.Fatal("Git discovery dropped an earlier cause", cause, folders.GitIssue)
		}
	}
}
