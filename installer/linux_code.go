package installer

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// LinuxCodePin is the fixed official Code DEB input to the APT provider. APT
// still owns installation, dependency resolution, operation evidence and removal.
type LinuxCodePin struct {
	Version      string `json:"version"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Architecture string `json:"architecture"`
}

//go:embed linux-code-pins.json
var linuxCodePinData []byte

var linuxCodeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-[0-9]+$`)
var linuxCodeURLPath = regexp.MustCompile(`^/dbazure/download/stable/[a-f0-9]{40}/code_[0-9]+\.[0-9]+\.[0-9]+-[0-9]+_(amd64|arm64)\.deb$`)

func (pin LinuxCodePin) validate() error {
	u, err := url.Parse(pin.URL)
	if err != nil || u.Scheme != "https" || u.Host != "vscode.download.prss.microsoft.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		!linuxCodeURLPath.MatchString(u.Path) || filepath.Base(u.Path) != "code_"+pin.Version+"_"+pin.Architecture+".deb" ||
		!linuxCodeVersion.MatchString(pin.Version) || !slices.Contains([]string{"amd64", "arm64"}, pin.Architecture) || !operationID.MatchString(pin.SHA256) || pin.Size <= 0 || pin.Size > maxArchiveBytes {
		return errors.New("Code requires an exact official Debian package version, architecture, size and SHA-256")
	}
	return nil
}

func defaultLinuxCodePin(arch string) (LinuxCodePin, error) {
	var data struct {
		Schema int                     `json:"schema"`
		Pins   map[string]LinuxCodePin `json:"pins"`
	}
	if err := Decode(linuxCodePinData, &data); err != nil {
		return LinuxCodePin{}, err
	}
	if data.Schema != 1 || len(data.Pins) != 2 {
		return LinuxCodePin{}, errors.New("invalid Linux Code pins")
	}
	for architecture, pin := range data.Pins {
		if err := pin.validate(); err != nil || architecture != pin.Architecture {
			return LinuxCodePin{}, errors.Join(errors.New("invalid Linux Code platform pin"), err)
		}
	}
	pin, ok := data.Pins[arch]
	if !ok {
		return pin, errors.New("unsupported Linux Code architecture")
	}
	return pin, nil
}

// Debconf's environment grammar cannot encode a filename containing whitespace;
// dpkg also changes maintainer-script cwd to /. Use an exclusive native temporary
// input, never an alias or a shortened package/state/profile path. The writable
// database accepts only this one question; all other package answers stay in the
// normal system database. This also suppresses postrm's repository/key removal.
const linuxCodeDebconf = "Name: code/add-microsoft-repo\nTemplate: code/add-microsoft-repo\nValue: false\nOwners: code\nFlags: seen\n\n"

type linuxCodeInput struct {
	Pin       LinuxCodePin `json:"pin"`
	Operation string       `json:"operation"`
	Directory string       `json:"directory"`
	Identity  string       `json:"identity"`
}

func newLinuxCodeInput(pin LinuxCodePin, operation string) (*linuxCodeInput, error) {
	if err := pin.validate(); err != nil || !operationID.MatchString(operation) {
		return nil, errors.Join(errors.New("invalid Code input operation"), err)
	}
	directory, err := os.MkdirTemp("/var/tmp", "dotfiles-code-debconf-"+operation+"-")
	if err != nil {
		return nil, err
	}
	directory, err = resolveConfigPath(directory)
	if err != nil {
		return nil, err
	}
	identity, err := linuxCodeTemporaryIdentity(directory)
	if err != nil {
		return nil, errors.Join(err, os.Remove(directory))
	}
	input := &linuxCodeInput{Pin: pin, Operation: operation, Directory: directory, Identity: identity}
	file, err := os.OpenFile(filepath.Join(directory, "answer.dat"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, errors.Join(err, os.Remove(directory))
	}
	_, writeErr := io.WriteString(file, linuxCodeDebconf)
	if err := errors.Join(writeErr, file.Chmod(0644), file.Sync(), file.Close()); err != nil {
		return nil, err // Preserve uncertain input for inspection.
	}
	return input, nil
}

func (input linuxCodeInput) validate() error {
	if err := input.Pin.validate(); err != nil {
		return err
	}
	parent, err := resolveConfigPath("/var/tmp")
	if err != nil || filepath.Dir(input.Directory) != parent || !operationID.MatchString(input.Operation) || !strings.HasPrefix(filepath.Base(input.Directory), "dotfiles-code-debconf-"+input.Operation+"-") ||
		strings.ContainsAny(input.Directory, " \t\r\n\x00{}") || input.Identity == "" {
		return errors.Join(errors.New("invalid saved Code temporary input"), err)
	}
	return nil
}

func (input linuxCodeInput) inspect(absentOK bool) error {
	if err := input.validate(); err != nil {
		return err
	}
	identity, err := linuxCodeTemporaryIdentity(input.Directory)
	if absentOK && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || identity != input.Identity {
		return errors.Join(errors.New("Code temporary input changed owner, identity or location"), err)
	}
	entries, err := os.ReadDir(input.Directory)
	if err != nil {
		return err
	}
	found := false
	for _, entry := range entries {
		if !slices.Contains([]string{"answer.dat", "answer.dat-new", "answer.dat-old"}, entry.Name()) {
			return errors.New("unexpected Code temporary input is preserved for inspection")
		}
		path := filepath.Join(input.Directory, entry.Name())
		data, err := readLinuxCodeDatabase(path)
		if err != nil || string(data) != linuxCodeDebconf {
			return errors.Join(errors.New("changed Code temporary database bytes are preserved for inspection"), err)
		}
		found = found || entry.Name() == "answer.dat"
	}
	if !found && !absentOK {
		return errors.New("Code temporary database is missing; preserve the saved operation")
	}
	return nil
}

func (input linuxCodeInput) cleanup() error {
	if err := input.inspect(true); err != nil {
		return err
	}
	for _, name := range []string{"answer.dat", "answer.dat-new", "answer.dat-old"} {
		if err := os.Remove(filepath.Join(input.Directory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(input.Directory); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (d *LinuxAPTDriver) codePayload(pin LinuxCodePin) string {
	return filepath.Join(d.Directory, "payloads", pin.SHA256, "code.deb")
}

func (d *LinuxAPTDriver) prepareCode(ctx context.Context, input linuxCodeInput) (result error) {
	if err := input.inspect(false); err != nil {
		return err
	}
	file := d.codePayload(input.Pin)
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("Code package input was redirected"), err)
	}
	if _, err := os.Lstat(file); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(filepath.Dir(file)), 0700); err != nil {
			return err
		}
		stage, err := os.MkdirTemp(filepath.Dir(filepath.Dir(file)), ".code-download-")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
		pin := ArchivePin{Version: input.Pin.Version, URL: input.Pin.URL, SHA256: input.Pin.SHA256, Format: "file", File: "code.deb", RequiredFiles: []string{"code.deb"}}
		if err := downloadArchive(ctx, d.Client, pin, filepath.Join(stage, "payload")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, "payload"), filepath.Dir(file)); err != nil {
			return err
		}
	}
	return d.verifyCodePayload(input.Pin)
}

func (d *LinuxAPTDriver) verifyCodePayload(pin LinuxCodePin) error {
	file := d.codePayload(pin)
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("Code package input was redirected"), err)
	}
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() != pin.Size {
		return errors.Join(errors.New("Code package input is missing, redirected or damaged"), err)
	}
	handle, err := os.Open(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, io.LimitReader(handle, pin.Size+1))
	if err := errors.Join(readErr, handle.Close()); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != pin.SHA256 {
		return errors.New("Code package input differs from its saved SHA-256")
	}
	return nil
}

// These are the reviewed DEB's nonshared publications plus the legacy command
// that its postinst removes. Do not let dpkg overwrite a personal installation.
var linuxCodePublications = []string{
	"/usr/share/code", "/usr/bin/code", "/usr/local/bin/code",
	"/usr/share/appdata/com.microsoft.VSCode.appdata.xml",
	"/usr/share/applications/com.microsoft.VSCode.desktop",
	"/usr/share/applications/com.microsoft.VSCode.UrlHandler.desktop",
	"/usr/share/bash-completion/completions/code", "/usr/share/mime/packages/code-workspace.xml",
	"/usr/share/pixmaps/vscode.png", "/usr/share/zsh/vendor-completions/_code",
}

func (d *LinuxAPTDriver) checkCodeCollisions(ctx context.Context, absent bool) error {
	paths := linuxCodePublications
	if !absent {
		paths = []string{"/usr/local/bin/code"}
	}
	script := "for p in"
	for _, path := range paths {
		script += " " + shellLiteral(path)
	}
	script += "; do if [ -e \"$p\" ] || [ -L \"$p\" ]; then printf '%s\\n' \"$p\"; fi; done"
	output, err := d.Query(ctx, false, "/bin/sh", nil, "-c", script)
	if err != nil || len(output) != 0 {
		return errors.Join(fmt.Errorf("existing Code publication must be preserved; move or remove the conflicting personal installation before retrying%s", nativeDiagnostic(output)), err)
	}
	return nil
}

// Package checksum verification does not check this postinst-created symlink.
// Verify the actual canonical executable and trusted parent ownership as well.
const linuxCodeHealth = "set -eu\n" +
	"fail() { printf 'Code native health: %s\\n' \"$*\" >&2; exit 1; }\n" +
	"link=$(readlink /usr/bin/code) || fail '/usr/bin/code target unreadable'\n" +
	"[ \"$link\" = /usr/share/code/bin/code ] || fail \"/usr/bin/code target=$link\"\n" +
	"for p in /usr /usr/share /usr/share/code /usr/share/code/code /usr/share/code/chrome-sandbox; do\n" +
	"  [ ! -L \"$p\" ] || fail \"$p is a symbolic link\"\n" +
	"  owner=$(stat -c %u \"$p\") || fail \"$p owner unreadable\"\n" +
	"  mode=$(stat -c %a \"$p\") || fail \"$p mode unreadable\"\n" +
	"  [ \"$owner\" = 0 ] && [ \"$mode\" = 755 ] || fail \"$p owner=$owner mode=$mode (expected owner=0 mode=755)\"\n" +
	"done\n"

const linuxCodeRemoveLink = "set -eu\n" +
	"if [ -e /usr/bin/code ] || [ -L /usr/bin/code ]; then\n" +
	"  [ -L /usr/bin/code ]\n" +
	"  [ \"$(readlink /usr/bin/code)\" = /usr/share/code/bin/code ]\n" +
	"fi\n"
