package installer

import (
	"errors"
	"fmt"
	"os"
)

type legacyDeterminateRelease struct {
	Asset  string
	Size   int64
	SHA256 string
}

type legacyDeterminateBinary struct {
	Identity, SHA256, Trust string
	Owner                   uint32
}

// The installed parent always remains root-owned and protected. A binary
// writable by another account needs independent release authentication, and
// neither trust route executes the source file during privileged removal.
func legacyDeterminateBinaryTrust(path, parent, version string, pin *legacyDeterminateRelease) (proof legacyDeterminateBinary, problem string, err error) {
	if path == "" {
		return proof, "the installed executable path is empty", nil
	}
	for _, item := range []struct {
		path      string
		directory bool
	}{{parent, true}, {path, false}} {
		if item.path == "" {
			continue
		}
		info, err := os.Lstat(item.path)
		if errors.Is(err, os.ErrNotExist) {
			return proof, item.path + " is missing", nil
		}
		if err != nil {
			return proof, "", err
		}
		if item.directory != info.IsDir() || !item.directory && !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return proof, item.path + " is not the expected plain file or directory", nil
		}
		uid, _, mode, err := legacyFileOwner(item.path)
		if err != nil {
			return proof, "", err
		}
		protected := uid == 0 && mode&0022 == 0
		if item.directory {
			if !protected {
				return proof, item.path + " is not root-owned and protected from other writers", nil
			}
			continue
		}
		if info.Size() > 512<<20 {
			return proof, path + " exceeds its review bound", nil
		}
		proof.Owner = uid
		if !protected && pin == nil {
			return proof, fmt.Sprintf("%s (owner uid %d) is not root-owned and protected from other writers, and no reviewed release digest exists for this version and target", path, uid), nil
		}
		if proof.SHA256, err = legacyFileSHA256(path); err != nil {
			return proof, "", err
		}
		proof.Trust = "root-owned"
		if !protected {
			if proof.SHA256 != pin.SHA256 || info.Size() != pin.Size {
				return proof, fmt.Sprintf("%s (owner uid %d) does not match the reviewed release digest and size for v%s/%s", path, uid, version, pin.Asset), nil
			}
			proof.Trust = "release:v" + version + "/" + pin.Asset
		}
	}
	proof.Identity, err = legacyPathIdentity(path)
	return proof, "", err
}
