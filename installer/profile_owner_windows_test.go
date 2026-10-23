package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func ownerProfileController(t *testing.T, home string) (Controller, *ProfileDriver, string) {
	t.Helper()
	home, err := resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	c, d, path := profileControllerAt(t, home)
	c.Catalog.Resources = append(c.Catalog.Resources,
		Resource{ID: "other", Name: "Other", Capability: true, Requires: []string{"profile.other"}},
		Resource{ID: "profile.other", Name: "Other profile", Action: "profile", Scope: "user"})
	d.Targets["profile.other"] = []ProfileTarget{{Path: filepath.Join(home, "other.ps1"), Script: "# independent integration"}}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	return c, d, path
}

func profileSecurity(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd.String()
}

func TestWindowsOwnerProfileControllerResolvesDirectoryAlias(t *testing.T) {
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "physical home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "home alias")
	if err := createDirectoryLink(home, alias); err != nil {
		t.Fatal(err)
	}
	c, _, path := ownerProfileController(t, alias)
	const personal = "# personal profile\r\n"
	writeConfigFixture(t, path, personal)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if data, err := os.ReadFile(path); err != nil || string(data) != personal {
		t.Fatal("aliased fixture changed personal bytes", string(data), err)
	}
}

func TestWindowsUnassignableOwnerDoesNotRecommendElevation(t *testing.T) {
	if os.Getenv("DOTFILES_UNASSIGNABLE_OWNER_TEST") != "1" {
		runRestrictedProfileProcess(t, t.TempDir(), "TestWindowsUnassignableOwnerDoesNotRecommendElevation", "DOTFILES_UNASSIGNABLE_OWNER_TEST=1")
		return
	}
	// The system executable is read only. Copy only into a private fixture;
	// its TrustedInstaller owner cannot be assigned by an ordinary admin token.
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	from := filepath.Join(system, "cmd.exe")
	before, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	security := profileSecurity(t, from)
	to := filepath.Join(t.TempDir(), "copy.exe")
	err = copyProfileMetadata(context.Background(), from, to)
	if !errors.Is(err, windows.ERROR_INVALID_OWNER) {
		t.Fatal("expected the real system-owner assignment refusal", err)
	}
	if strings.Contains(err.Error(), "elevated PowerShell") || !strings.Contains(err.Error(), "resolve ownership outside setup") {
		t.Fatal("unassignable owner received unsupported elevated recovery advice", err)
	}
	after, readErr := os.ReadFile(from)
	if readErr != nil || !bytes.Equal(before, after) || profileSecurity(t, from) != security {
		t.Fatal("read-only source changed", readErr)
	}
	if _, statErr := os.Lstat(to); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("refused copy was not removed", statErr)
	}
}

func TestWindowsAdministratorOwnedProfileRecovery(t *testing.T) {
	if home := os.Getenv("DOTFILES_OWNER_PROFILE_HOME"); home != "" {
		token := windows.GetCurrentProcessToken()
		if token.IsElevated() {
			t.Fatal("child must use the restricted token")
		}
		c, d, path := ownerProfileController(t, home)
		action := os.Getenv("DOTFILES_OWNER_PROFILE_ACTION")
		if os.Getenv("DOTFILES_OWNER_PROFILE_PHASE") == "finish" {
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			if state.Transaction != nil {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			}
			before, err := os.ReadFile(c.StatePath)
			if err != nil {
				t.Fatal(err)
			}
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("ordinary Check refused the preserved owner", check, err)
			}
			after, err := os.ReadFile(c.StatePath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("Check changed the saved state", err)
			}
			if action == "install" {
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"test", "other"}}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				for _, op := range plan.Operations {
					if op.Resource == "profile.test" && (op.Action != "keep" || !op.Observed.Healthy || op.Observed.Pending != "") {
						t.Fatal("unrelated selection cannot keep the healthy profile", op)
					}
				}
				dispatchApproved(t, c, request)
			}
			return
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		security := profileSecurity(t, path)
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
		if action == "remove" {
			request.Selected = []string{}
		} else if action == "restore" {
			request = Request{Schema: 1, Mode: "restore"}
		}
		plan, err := c.Preview(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = plan.ID
		_, refusal := c.Dispatch(context.Background(), request)
		if !errors.Is(refusal, windows.ERROR_INVALID_OWNER) {
			t.Fatal("did not preserve the native owner refusal", refusal)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) || profileSecurity(t, path) != security {
			t.Fatal("refusal changed the original file or descriptor", err)
		}
		state, err := LoadState(c.StatePath, c.Home)
		if err != nil || state.Transaction == nil || state.Receipts["profile.test"].Status != "in-progress" {
			t.Fatal("refusal lost the active intent", state, err)
		}
		j, err := d.readRecoveryJournal("profile.test", state.Receipts["profile.test"])
		if err != nil {
			t.Fatal(err)
		}
		e := j.Entries[0]
		if action == "restore" {
			if len(j.Restoration) != 1 {
				t.Fatal("restoration did not save its intent", j)
			}
			e = j.Restoration[0]
		}
		if e.Staged || e.Moved || e.Published {
			t.Fatal("refusal advanced publication", e)
		}
		if _, err := os.Lstat(filepath.Join(e.Workspace, "next")); !os.IsNotExist(err) {
			t.Fatal("refusal left a staged copy", err)
		}
		for _, want := range []string{path, "S-1-5-32-544", "Administrators", "same Windows account", "elevated PowerShell", "saved resource", "normal PowerShell", "ownership"} {
			if !strings.Contains(refusal.Error(), want) {
				t.Errorf("owner refusal lacks recovery guidance %q: %v", want, refusal)
			}
		}
		return
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("requires an elevated disposable Windows test process to create the legacy owner")
	}
	for _, action := range []string{"install", "remove", "restore"} {
		t.Run(action, func(t *testing.T) {
			home := t.TempDir()
			grantProfileFixtureUser(t, home)
			c, d, path := ownerProfileController(t, home)
			const original = "# personal legacy profile\r\n"
			writeConfigFixture(t, path, original)
			admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
			if err != nil {
				t.Fatal(err)
			}
			if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, admin, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
			security := profileSecurity(t, path)
			if !strings.HasPrefix(security, "O:BA") {
				t.Fatal("fixture is not Administrators-owned", security)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			if action == "remove" {
				dispatchApproved(t, c, request)
			} else if action == "restore" {
				c.Driver = interruptedProfileDriver{d}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = plan.ID
				if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "fixture stopped") {
					t.Fatal("fixture did not interrupt the real publication", err)
				}
				c.Driver = d
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
				if err != nil {
					t.Fatal(err)
				}
				j.Complete = false
				if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte("# later personal edit\r\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{"DOTFILES_OWNER_PROFILE_HOME=" + home, "DOTFILES_OWNER_PROFILE_ACTION=" + action}
			runRestrictedProfileProcess(t, home, "TestWindowsAdministratorOwnedProfileRecovery", env...)
			before, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			retry := Request{Schema: 1, Mode: "apply", Retry: true}
			if action == "restore" {
				retry = Request{Schema: 1, Mode: "restore"}
			}
			plan, err := c.Preview(context.Background(), retry)
			if err != nil || len(plan.Operations) != 1 || plan.Operations[0].Resource != "profile.test" {
				t.Fatal("elevated recovery is not scoped to the saved resource", plan, err)
			}
			retry.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), retry)
			if err != nil || action != "restore" && result.Status != "needs-action" {
				t.Fatal("exact elevated recovery failed", result, err)
			}
			if profileSecurity(t, path) != security {
				t.Fatal("recovery changed owner/group/DACL")
			}
			if action != "restore" {
				after, err := LoadState(c.StatePath, c.Home)
				if err != nil || after.Receipts["profile.test"].OperationID != before.Receipts["profile.test"].OperationID {
					t.Fatal("recovery replaced the saved operation", err)
				}
			}
			t.Log("elevated state descriptor:", profileSecurity(t, c.StatePath))
			runRestrictedProfileProcess(t, home, "TestWindowsAdministratorOwnedProfileRecovery", append(env, "DOTFILES_OWNER_PROFILE_PHASE=finish")...)
			if profileSecurity(t, path) != security {
				t.Fatal("ordinary remaining work changed owner/group/DACL")
			}
			if action != "install" {
				want := original
				if action == "restore" {
					want += "# later personal edit\r\n"
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Fatal("personal bytes changed", string(data), err)
				}
			}
		})
	}
}
