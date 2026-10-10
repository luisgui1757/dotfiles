package installer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

var portableGitVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.windows\.[0-9]+$`)

// PortableGit is the complete upstream distribution, including its interactive
// tools. These are runtime prerequisites, not an alternative reduced Git build.
var portableGitRequiredFiles = []string{
	"cmd/git.exe", "cmd/git-gui.exe", "cmd/gitk.exe", "bin/bash.exe", "bin/sh.exe",
	"git-bash.exe", "git-cmd.exe", "usr/bin/bash.exe", "usr/bin/ssh.exe", "usr/bin/ssh-keygen.exe",
	"usr/bin/msys-2.0.dll", "usr/bin/msys-crypto-3.dll", "usr/bin/msys-z.dll",
	"usr/bin/msys-iconv-2.dll", "usr/bin/msys-intl-8.dll", "usr/bin/msys-readline8.dll", "usr/bin/msys-ncursesw6.dll",
	"ucrt64/bin/git.exe", "ucrt64/bin/git-lfs.exe", "ucrt64/bin/git-credential-manager.exe",
	"ucrt64/bin/libcurl-4.dll", "ucrt64/bin/libpcre2-8-0.dll", "ucrt64/bin/libiconv-2.dll", "ucrt64/bin/libintl-8.dll",
	"ucrt64/bin/libcrypto-3-x64.dll", "ucrt64/bin/libssl-3-x64.dll", "ucrt64/bin/zlib1.dll",
	"ucrt64/bin/tcl86.dll", "ucrt64/bin/tk86.dll", "ucrt64/libexec/git-core/git-gui",
	"etc/gitconfig", "etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", "README.portable",
}

func validatePortableGitPin(pin ArchivePin) error {
	version := strings.ReplaceAll(pin.Version, ".windows.", ".")
	if strings.HasSuffix(pin.Version, ".windows.1") {
		version = strings.TrimSuffix(pin.Version, ".windows.1")
	}
	u, err := url.Parse(pin.URL)
	if !pin.PortableGit || pin.PortableGitRevision < 0 || pin.PortableGitRevision > 1 || !portableGitVersion.MatchString(pin.Version) || pin.Format != "file" || pin.File != "portable-git.exe" || pin.StripComponents != 0 || len(pin.ExcludedFiles) != 0 || len(pin.Replacements) != 0 ||
		err != nil || u.Scheme != "https" || u.Host != "github.com" || u.RawQuery != "" || u.User != nil || u.Fragment != "" || u.Path != "/git-for-windows/git/releases/download/v"+pin.Version+"/PortableGit-"+version+"-64-bit.7z.exe" ||
		pin.Commands["git"] != "cmd/git.exe" || pin.Commands["bash"] != "bin/bash.exe" || pin.Commands["sh"] != "bin/sh.exe" {
		return errors.New("PortableGit requires the pinned complete official amd64 self-extractor and fixed command layout")
	}
	for _, name := range portableGitRequiredFiles {
		if !slices.Contains(pin.RequiredFiles, name) {
			return fmt.Errorf("PortableGit requires runtime file %s", name)
		}
	}
	return nil
}

// Run only the checksum-verified, fixed upstream self-extractor. Its documented
// post-install step creates private MSYS devices/configuration and DLL links;
// manually unpacking and omitting that step does not produce usable Git.
// https://github.com/git-for-windows/build-extra/blob/main/portable/root/README.portable
func (d *ArchiveDriver) preparePortableGit(ctx context.Context, intent archiveIntent, payload string) error {
	pin := intent.Pin
	if err := validatePortableGitPin(pin); err != nil {
		return err
	}
	if pin.PortableGitRevision != 1 {
		return errors.New("saved PortableGit preparation predates long-path support; preserve the original operation and payload for explicit recovery")
	}
	if d.Run == nil {
		return errors.New("PortableGit preparation requires the native session")
	}
	program := filepath.Join(payload, pin.File)
	file, err := os.Open(program)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, io.LimitReader(file, maxArchiveBytes+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != pin.SHA256 {
		return errors.New("PortableGit self-extractor changed after download verification")
	}
	home, temporary := filepath.Join(payload, ".prepare-home"), filepath.Join(payload, ".prepare-temp")
	for _, path := range []string{home, temporary} {
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
	}
	environment := portableGitPreparationEnvironment(payload, home, temporary)
	// A resumed archive publication preserves an incomplete stage and starts a
	// fresh one. Give each attempt distinct worker IDs so no old reply is reused.
	attempt := rand.Text()
	run := func(phase, program string, args ...string) error {
		operation, err := digest([]string{intent.Operation, attempt, phase})
		if err != nil {
			return err
		}
		output, err := d.Run(ctx, nativeCommand{Operation: operation, Program: program, Arguments: args, Environment: environment})
		if err != nil {
			return fmt.Errorf("PortableGit %s: %s", phase, nativeErrorDetail(err, output))
		}
		return nil
	}
	// The upstream Git project's own Windows CI uses these SFX switches. The
	// extractor itself is not long-path aware; an extended output path keeps
	// nested distribution files addressable without shortening our owned paths.
	// https://github.com/git/git/blob/master/ci/install-dependencies.ps1
	outputPath := payload
	if runtime.GOOS == "windows" {
		outputPath = windowsExtendedPath(payload)
	}
	if err := run("extract", program, "-y", "-o"+outputPath); err != nil {
		return err
	}
	var stagedScript string
	var stagedInfo os.FileInfo
	var stagedHash [sha256.Size]byte
	// The SFX does not propagate its child post-install exit status. Complete
	// the upstream manual preparation if its script remains, using an ordinary
	// working-directory spelling rather than the SFX's extended current path.
	// https://github.com/git-for-windows/build-extra/blob/main/post-install.bat
	if info, err := os.Lstat(filepath.Join(payload, "post-install.bat")); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 64<<10 {
			return errors.New("PortableGit post-install script is not a bounded regular file")
		}
		// Its final DEL names post-install.bat literally. Running unchanged bytes
		// from a sibling keeps CMD's input readable after that upstream cleanup;
		// %~dp0 still resolves to the same payload. Never ignore a failed exit.
		source, err := os.Open(filepath.Join(payload, "post-install.bat"))
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(source, (64<<10)+1))
		if err := errors.Join(readErr, source.Close()); err != nil {
			return err
		}
		if len(data) > 64<<10 {
			return errors.New("PortableGit post-install script exceeds its size limit")
		}
		stagedScript = filepath.Join(payload, ".prepare-post-install.bat")
		staged, err := os.OpenFile(stagedScript, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := staged.Write(data)
		stagedInfo, err = staged.Stat()
		if err := errors.Join(writeErr, err, staged.Close()); err != nil {
			return err
		}
		stagedHash = sha256.Sum256(data)
		if err := run("post-install", filepath.Join(payload, "git-bash.exe"), "--no-needs-console", "--hide", "--cd="+payload, "--command=.prepare-post-install.bat"); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, name := range []string{"post-install.bat", "etc/post-install"} {
		if _, err := os.Lstat(filepath.Join(payload, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errors.New("PortableGit post-install has not completed"), err)
		}
	}
	root, err := os.OpenRoot(payload)
	if err != nil {
		return err
	}
	if err := errors.Join(checkArchivePayload(root, pin), root.Close()); err != nil {
		return err
	}
	// Set only this distribution's configuration before its immutable snapshot.
	// The command-line flag also permits opening the long staging config path.
	if err := run("long-paths", filepath.Join(payload, "cmd", "git.exe"), "-c", "core.longpaths=true", "config", "--file", filepath.Join(payload, "etc", "gitconfig"), "core.longpaths", "true"); err != nil {
		return err
	}
	if err := run("runtime", filepath.Join(payload, "bin", "bash.exe"), "--noprofile", "--norc", "-p", "-c", portableGitRuntimeProbe, "dotfiles-portable-git", pin.Version); err != nil {
		return err
	}
	// Retain the execution copy on every failed preparation. Successful cleanup
	// is limited to the exact file and bytes we created, after the child exits.
	if stagedScript != "" {
		current, err := os.Lstat(stagedScript)
		if err != nil || !current.Mode().IsRegular() || !os.SameFile(stagedInfo, current) || current.Size() != stagedInfo.Size() {
			return errors.Join(errors.New("PortableGit post-install execution copy changed"), err)
		}
		data, err := os.ReadFile(stagedScript)
		if err != nil || sha256.Sum256(data) != stagedHash {
			return errors.Join(errors.New("PortableGit post-install execution copy changed"), err)
		}
		if err := os.Remove(stagedScript); err != nil {
			return err
		}
	}
	// All later fingerprint/cleanup work sees only the prepared distribution.
	return os.Remove(program)
}

// Git's Windows access/fopen implementation recognizes /dev/null explicitly;
// Go's uppercase Windows os.DevNull (NUL) is rejected as a config-file path.
// https://github.com/git-for-windows/git/blob/v2.56.0.windows.1/compat/mingw.c
func portableGitPreparationEnvironment(payload, home, temporary string) []string {
	return []string{
		"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + home, "LOCALAPPDATA=" + home,
		"TEMP=" + temporary, "TMP=" + temporary, "TMPDIR=" + temporary,
		"PATH=" + strings.Join([]string{filepath.Join(payload, "cmd"), filepath.Join(payload, "bin"), filepath.Join(payload, "ucrt64", "bin"), filepath.Join(payload, "usr", "bin"), filepath.Join(os.Getenv("SystemRoot"), "System32")}, ";"),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_COUNT=0", "GIT_CONFIG_PARAMETERS=", "GIT_EXEC_PATH=" + filepath.Join(payload, "ucrt64", "libexec", "git-core"),
		"GIT_TEMPLATE_DIR=" + filepath.Join(payload, "ucrt64", "share", "git-core", "templates"),
		"BASH_ENV=", "ENV=", "CDPATH=", "MSYSTEM=UCRT64", "MSYS2_PATH_TYPE=strict", "MSYS2_ENV_CONV_EXCL=", "MSYS_NO_PATHCONV=",
		// MSYS can read this ordinary system-attribute link file without native
		// symlink privileges; Go can fingerprint it without unsupported LX tags.
		// https://github.com/git-for-windows/msys2-runtime/blob/main/winsup/cygwin/environ.cc
		"MSYS=winsymlinks:sys",
	}
}

const portableGitRuntimeProbe = `set -eu
export PATH=/ucrt64/bin:/usr/bin:/bin
printf '%s\n' 'PortableGit runtime: prepared filesystem' >&2
test -d /dev/shm && test -d /dev/mqueue
test "$(readlink /etc/mtab)" = /proc/mounts
test -f /ucrt64/libexec/git-core/dlls-copied
printf '%s\n' 'PortableGit runtime: Git version' >&2
test "$(git --version)" = "git version $1"
test "$(git -c core.longpaths=true config --file /etc/gitconfig --type=bool --get core.longpaths)" = true
printf '%s\n' 'PortableGit runtime: Git LFS' >&2
git lfs version >/dev/null
printf '%s\n' 'PortableGit runtime: Git Credential Manager' >&2
git credential-manager --version >/dev/null
printf '%s\n' 'PortableGit runtime: SSH' >&2
ssh -V
printf '%s\n' portable-git-ready
`
