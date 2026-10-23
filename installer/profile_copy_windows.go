package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var copyProfileFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("CopyFileW")

// CopyFileW preserves file attributes and security resource properties, but not
// the DACL. Copy and verify ordinary ownership/permissions explicitly before
// publishing; insufficient authority leaves the original profile untouched.
func copyProfileMetadata(ctx context.Context, from, to string) (copyErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := windows.UTF16PtrFromString(windowsExtendedPath(from))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(windowsExtendedPath(to))
	if err != nil {
		return err
	}
	result, _, callErr := copyProfileFile.Call(uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(target)), 1)
	if result == 0 {
		return callErr
	}
	created, err := os.Lstat(to)
	if err != nil {
		return err
	}
	defer func() {
		if copyErr != nil {
			copyErr = errors.Join(copyErr, removeFailedProfileCopy(to, created))
		}
	}()
	const parts = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION | windows.DACL_SECURITY_INFORMATION
	original, err := windows.GetNamedSecurityInfo(windowsExtendedPath(from), windows.SE_FILE_OBJECT, parts)
	if err != nil || original == nil {
		return errors.Join(errors.New("cannot read original profile permissions"), err)
	}
	copy, err := windows.GetNamedSecurityInfo(windowsExtendedPath(to), windows.SE_FILE_OBJECT, parts)
	if err != nil || copy == nil {
		return errors.Join(errors.New("cannot read staged profile permissions"), err)
	}
	want := original.String()
	if want == "" {
		return errors.New("cannot encode original profile permissions")
	}
	// Setting an already equivalent inherited DACL can change Windows' control
	// flags. Preserve an exact copy without an unnecessary descriptor rewrite.
	if copy.String() == want {
		return nil
	}
	owner, _, err := original.Owner()
	if err != nil {
		return err
	}
	group, _, err := original.Group()
	if err != nil {
		return err
	}
	acl, _, err := original.DACL()
	if err != nil {
		return err
	}
	control, _, err := original.Control()
	if err != nil {
		return err
	}
	flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	copiedOwner, _, err := copy.Owner()
	if err != nil {
		return err
	}
	copiedGroup, _, err := copy.Group()
	if err != nil {
		return err
	}
	if !owner.Equals(copiedOwner) {
		flags |= windows.OWNER_SECURITY_INFORMATION
	}
	if !group.Equals(copiedGroup) {
		flags |= windows.GROUP_SECURITY_INFORMATION
	}
	if err := windows.SetNamedSecurityInfo(windowsExtendedPath(to), windows.SE_FILE_OBJECT, flags, owner, group, acl, nil); err != nil {
		if errors.Is(err, windows.ERROR_INVALID_OWNER) {
			identity := owner.String()
			guidance := "Explicitly resolve ownership outside setup before retrying; setup never takes ownership automatically."
			if owner.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
				identity = "BUILTIN\\Administrators (" + identity + ")"
				guidance = "To retry with administrator authority, open elevated PowerShell as the same Windows account, use the same checkout, and recover only this saved resource (Resume, or finish its saved restoration). Review that one-resource plan, then return to normal PowerShell for remaining work. Alternatively, explicitly resolve ownership outside setup before retrying; setup never takes ownership automatically."
			}
			return fmt.Errorf("cannot preserve owner %s: %w; retrying with the same permissions will fail again. %s", identity, err, guidance)
		}
		return err
	}
	verified, err := windows.GetNamedSecurityInfo(windowsExtendedPath(to), windows.SE_FILE_OBJECT, parts)
	if err != nil || verified == nil || verified.String() != want {
		return errors.Join(errors.New("staged profile permissions differ from the original"), err)
	}
	return nil
}
