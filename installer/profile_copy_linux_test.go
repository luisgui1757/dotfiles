package installer

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinuxProfileCopyPreservesACLAndExtendedAttributes(t *testing.T) {
	for _, withACL := range []bool{false, true} {
		t.Run(map[bool]string{false: "discard-inherited-acl", true: "copy-source-acl"}[withACL], func(t *testing.T) {
			root := t.TempDir()
			from := filepath.Join(root, "source")
			parent := filepath.Join(root, "workspace")
			writeConfigFixture(t, from, "profile content")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			// Linux UAPI: posix_acl_xattr.h version 2, little-endian header
			// and (tag,permissions,id) entries; posix_acl.h defines the tags.
			// https://github.com/torvalds/linux/blob/master/include/uapi/linux/posix_acl_xattr.h
			acl := new(bytes.Buffer)
			if err := binary.Write(acl, binary.LittleEndian, uint32(2)); err != nil {
				t.Fatal(err)
			}
			for _, entry := range []struct {
				Tag, Permissions uint16
				ID               uint32
			}{{1, 6, ^uint32(0)}, {2, 4, 65534}, {4, 0, ^uint32(0)}, {16, 4, ^uint32(0)}, {32, 0, ^uint32(0)}} {
				if err := binary.Write(acl, binary.LittleEndian, entry); err != nil {
					t.Fatal(err)
				}
			}
			if err := unix.Setxattr(parent, "system.posix_acl_default", acl.Bytes(), 0); err != nil {
				t.Fatal(err)
			}
			if withACL {
				if err := unix.Setxattr(from, "system.posix_acl_access", acl.Bytes(), 0); err != nil {
					t.Fatal(err)
				}
			}
			if err := unix.Setxattr(from, "user.dotfiles-copy", []byte("preserve exact metadata"), 0); err != nil {
				t.Fatal(err)
			}
			to := filepath.Join(parent, "next")
			if err := copyProfileMetadata(context.Background(), from, to); err != nil {
				t.Fatal(err)
			}
			attributes := func(path string) map[string][]byte {
				file, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				attrs, err := profileAttributes(int(file.Fd()))
				if closeErr := file.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if err != nil {
					t.Fatal(err)
				}
				return attrs
			}
			want, got := attributes(from), attributes(to)
			if len(want) != len(got) {
				t.Fatal("attribute count changed", want, got)
			}
			for name, value := range want {
				if !bytes.Equal(got[name], value) {
					t.Fatal("attribute changed", name)
				}
			}
			data, err := os.ReadFile(to)
			if err != nil || string(data) != "profile content" {
				t.Fatal("profile bytes changed", err)
			}
		})
	}
}
