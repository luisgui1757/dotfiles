package installer

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProfilePublicationPreservesNativeWindowsPermissions(t *testing.T) {
	for _, fixture := range []struct{ protected, readOnly, defaults bool }{{true, false, false}, {true, true, false}, {false, false, false}, {false, true, false}, {false, false, true}, {false, true, true}} {
		name := "inherited-and-explicit-acl"
		if fixture.protected {
			name = "protected-acl"
		}
		if fixture.defaults {
			name = "default-acl"
		}
		if fixture.readOnly {
			name += "-read-only"
		}
		t.Run(name, func(t *testing.T) {
			c, d, path := profileController(t)
			writeConfigFixture(t, path, "# personal profile\r\n")
			user, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")(A;;FR;;;BU)")
			if err != nil {
				t.Fatal(err)
			}
			acl, _, err := descriptor.DACL()
			if err != nil {
				t.Fatal(err)
			}
			flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
			if fixture.protected {
				flags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
			}
			if !fixture.defaults {
				if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, flags, nil, nil, acl, nil); err != nil {
					t.Fatal(err)
				}
			}
			if fixture.readOnly {
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
			}
			security := func() string {
				t.Helper()
				sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION)
				if err != nil {
					t.Fatal(err)
				}
				return sd.String()
			}
			before := security()
			mode, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"install", "update", "remove"} {
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
				if action == "update" {
					d.Targets["profile.test"][0].Script = "# updated integration"
					request = Request{Schema: 1, Mode: "update"}
				} else if action == "remove" {
					request.Selected = []string{}
				}
				dispatchApproved(t, c, request)
				if actual := security(); actual != before {
					t.Fatalf("%s changed native ACL/owner/group: %s -> %s", action, before, actual)
				}
				actual, err := os.Stat(path)
				if err != nil || actual.Mode() != mode.Mode() {
					t.Fatal("publication changed read-only attributes", action, actual, err)
				}
			}
		})
	}
}
