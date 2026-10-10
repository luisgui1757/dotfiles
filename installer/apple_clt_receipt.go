package installer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const appleCLTNoSelectionText = "xcode-select: error: Unable to get active developer directory. Use `sudo xcode-select --switch path/to/Xcode.app` to set one (or see `man xcode-select`)"
const appleCLTNoSelectionDiagnostic = appleCLTNoSelectionText + "\n"

var errAppleCLTNoSelection = errors.New("Apple developer directory is not selected")
var appleCLTReceiptIdentity = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*@[1-9][0-9]*$`)

func appleCLTSelectionMissing(err error, stdout, stderr []byte) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 2 && len(stdout) == 0 && string(stderr) == appleCLTNoSelectionDiagnostic
}

func parseAppleCLTReceipt(data []byte) (string, error) {
	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || fields[key] != "" || value == "" || strings.ContainsAny(value, "\x00\r\n") {
			return "", errors.New("invalid Apple CLT package receipt")
		}
		fields[key] = value
	}
	identity := fields["version"] + "@" + fields["install-time"]
	if len(fields) != 5 || fields["package-id"] != "com.apple.pkg.CLTools_Executables" || fields["volume"] != "/" || fields["location"] != "/" || !validAppleCLTReceipt(identity) || identity == "none" {
		return "", errors.New("invalid Apple CLT package receipt identity or installation root")
	}
	return identity, nil
}

func validAppleCLTReceipt(identity string) bool {
	if identity == "none" {
		return true
	}
	if len(identity) > 160 || !appleCLTReceiptIdentity.MatchString(identity) {
		return false
	}
	version, timestamp, _ := strings.Cut(identity, "@")
	if len(timestamp) > 18 || len(version) > 140 {
		return false
	}
	n, err := strconv.ParseUint(timestamp, 10, 63)
	return err == nil && n > 0
}

func (d *AppleCLTDriver) nativeReceipt(ctx context.Context) (string, error) {
	data, err := d.Query(ctx, false, "/usr/sbin/pkgutil", nil, "--pkgs")
	if err != nil {
		return "", err
	}
	if !slices.Contains(strings.Split(string(data), "\n"), "com.apple.pkg.CLTools_Executables") {
		return "none", nil
	}
	data, err = d.Query(ctx, false, "/usr/sbin/pkgutil", nil, "--pkg-info=com.apple.pkg.CLTools_Executables")
	if err != nil {
		return "", err
	}
	return parseAppleCLTReceipt(data)
}

func (d *AppleCLTDriver) selection(ctx context.Context) (string, error) {
	data, err := d.Query(ctx, false, "/usr/bin/xcode-select", nil, "--print-path")
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if errors.Is(err, errAppleCLTNoSelection) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return macOSDeveloperPath(data)
}

func appleCLTDisclosure(baseline string) string {
	if baseline == "none" {
		return "install missing Apple CLT payload (no prior receipt); select it globally; retain installed tools"
	}
	version, timestamp, _ := strings.Cut(baseline, "@")
	return fmt.Sprintf("reinstall missing Apple CLT payload (historical receipt %s, install-time %s); Apple updates receipts; select CLT globally; retain installed tools", version, timestamp)
}

func appleCLTReceiptChanged(before, after string) bool {
	if !validAppleCLTReceipt(before) || !validAppleCLTReceipt(after) || after == "none" {
		return false
	}
	if before == "none" {
		return true
	}
	_, oldTime, _ := strings.Cut(before, "@")
	_, newTime, _ := strings.Cut(after, "@")
	return oldTime != newTime
}
