package installer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestProfileLifecycleWithRestrictedWindowsToken(t *testing.T) {
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DOTFILES_RESTRICTED_PROFILE_TEST") == "1" {
		token := windows.GetCurrentProcessToken()
		if token.IsElevated() {
			t.Fatal("fixture is still elevated")
		}
		groups, err := token.GetTokenGroups()
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range groups.AllGroups() {
			if group.Sid.Equals(admin) && group.Attributes&windows.SE_GROUP_USE_FOR_DENY_ONLY == 0 {
				t.Fatal("administrator SID still grants access")
			}
		}
		TestProfilePublicationPreservesNativeWindowsPermissions(t)
		TestProfileMetadataCopyNeverReplacesAnExistingFile(t)
		TestProfileInstallUpdateAndRemovePreserveOutsideEdits(t)
		return
	}
	runRestrictedProfileProcess(t, t.TempDir(), "TestProfileLifecycleWithRestrictedWindowsToken", "DOTFILES_RESTRICTED_PROFILE_TEST=1")
}

func runRestrictedProfileProcess(t *testing.T, fixture, test string, env ...string) {
	t.Helper()
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	var original windows.Token
	// CreateRestrictedToken preserves the handle's access rights. Go starts the
	// child through CreateProcessAsUserW, which also requires ASSIGN_PRIMARY.
	// https://learn.microsoft.com/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &original); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	}()
	disabled := windows.SIDAndAttributes{Sid: admin}
	var restricted windows.Token
	// DISABLE_MAX_PRIVILEGE | LUA_TOKEN, plus an explicit deny-only admin SID.
	// https://learn.microsoft.com/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken
	create := windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")
	ok, _, callErr := create.Call(uintptr(original), 5, 1, uintptr(unsafe.Pointer(&disabled)), 0, 0, 0, 0, uintptr(unsafe.Pointer(&restricted)))
	if ok == 0 {
		t.Fatal(callErr)
	}
	defer func() {
		if err := restricted.Close(); err != nil {
			t.Error(err)
		}
	}()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Go's build directory may grant execution only through Administrators.
	// Give this disposable fixture an explicit user ACL before denying that SID.
	grantProfileFixtureUser(t, fixture)
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	binary = filepath.Join(fixture, "restricted-test.exe")
	if err := os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	// A test binary run directly has no default timeout: bound it so a hang
	// prints every goroutine. Its output goes to a file, not a pipe, because a
	// descendant that outlives the child would keep a pipe open forever.
	child := exec.Command(binary, "-test.run=^"+test+"$", "-test.v", "-test.timeout=12m")
	child.Dir = fixture
	child.Env = append(os.Environ(), "TEMP="+fixture, "TMP="+fixture)
	child.Env = append(child.Env, env...)
	child.SysProcAttr = &syscall.SysProcAttr{Token: syscall.Token(restricted)}
	logPath := filepath.Join(fixture, "restricted-test.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	child.Stdout, child.Stderr = log, log
	err = errors.Join(child.Run(), log.Close())
	output, readErr := os.ReadFile(logPath)
	if err != nil || readErr != nil {
		t.Fatalf("restricted lifecycle failed: %v\n%s", errors.Join(err, readErr), output)
	}
	t.Log(string(output))
}

func grantProfileFixtureUser(t *testing.T, fixture string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(fixture, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}
