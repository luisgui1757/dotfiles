package installer

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// NativePlatform holds provider observations, not persisted target identity.
// A session change must not make the installation belong to another machine.
type NativePlatform struct {
	Context
	Distro                     string
	Family                     string
	Libc                       string
	Homebrew                   *HomebrewLocation
	HomebrewIssue              string
	APT                        *LinuxAPTLocation
	APTIssue                   string
	WindowsPowerShell          string
	WindowsVendorIssue         string
	SystemCommandDirectories   []string
	WindowsBuildToolsDirectory string
	WindowsRustLauncher        string
	WindowsRustLauncherSHA256  string
	LinuxClipboard             LinuxClipboardCapability
	Sentinel                   SentinelLocations
	DpkgDeb                    string
}

// DiscoverPlatform observes the native machine without running shell input or
// installing anything. Linux guests use the same discovery as other Linux hosts.
func DiscoverPlatform() (NativePlatform, error) {
	target := NativePlatform{Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}}
	if err := target.Context.Validate(); err != nil {
		return target, err
	}
	if target.OS != "linux" {
		return target, nil
	}
	data, err := readPlatformFile("/etc/os-release")
	if errors.Is(err, os.ErrNotExist) {
		data, err = readPlatformFile("/usr/lib/os-release")
	}
	if err != nil {
		return target, fmt.Errorf("read Linux distribution identity: %w", err)
	}
	target.Distro, target.Family, err = linuxDistribution(data)
	if err != nil {
		return target, err
	}
	target.Libc, err = linuxLibc("/bin/sh")
	if err != nil {
		return target, err
	}
	return target, nil
}

func readPlatformFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(f, 65537))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, err
	}
	if len(data) > 65536 || bytes.ContainsRune(data, 0) {
		return nil, fmt.Errorf("invalid or oversized platform identity at %s", path)
	}
	return data, nil
}

var distroID = regexp.MustCompile(`^[a-z0-9._-]+$`)

// Read the exact distribution ID. ID_LIKE is not a support promise: derivatives
// do not become supported merely because they use APT. Never source os-release.
// https://www.freedesktop.org/software/systemd/man/latest/os-release.html
func linuxDistribution(data []byte) (string, string, error) {
	id := "linux"
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "ID" {
			continue
		}
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if !distroID.MatchString(value) {
			return "", "", errors.New("invalid distribution ID")
		}
		// The specification requires readers to use the last repeated key.
		id = value
	}
	if id != "ubuntu" && id != "debian" {
		return id, "", fmt.Errorf("unsupported Linux distribution %q; this release supports Ubuntu and Debian only", id)
	}
	return id, "apt", nil
}

// Inspect the system shell's actual ELF loader; distro branding and a glibc
// compatibility shim do not turn a musl installation into a glibc target.
func linuxLibc(shell string) (libc string, result error) {
	f, err := elf.Open(shell)
	if err != nil {
		return "", fmt.Errorf("inspect system shell ELF loader: %w", err)
	}
	defer func() { result = errors.Join(result, f.Close()) }()
	for _, program := range f.Progs {
		if program.Type != elf.PT_INTERP {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(program.Open(), 4097))
		if err != nil || len(data) > 4096 || len(data) < 2 || data[len(data)-1] != 0 {
			return "", errors.Join(errors.New("invalid system ELF interpreter"), err)
		}
		return interpreterLibc(string(data[:len(data)-1])), nil
	}
	return "", nil
}

func interpreterLibc(path string) string {
	name := filepath.Base(path)
	if strings.HasPrefix(name, "ld-musl-") {
		return "musl"
	}
	if strings.HasPrefix(name, "ld-linux-") {
		return "glibc"
	}
	return ""
}
