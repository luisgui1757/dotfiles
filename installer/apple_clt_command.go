package installer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// This is the CLT-only recipe from Homebrew/install at
// 35da6871c4be7d7fdab2fd505fb7fa667926a2a5, install.sh:827-846.
// Apple supplies the compatible signed package through softwareupdate; this is
// not a pinned package version. No Homebrew installer, prefix or package is used.
// Compared with upstream, preserve somebody else's sentinel, fail on errors,
// and report an operation marker only after the exact native install succeeds.
// Bash 3.2 does not apply errexit to a failed [[...]] command: assertions below
// use explicit exits, including inside the shell fixtures exercising this recipe.
const appleCLTScript = `set -eu
operation="$1"
mode="$2"
clt="$3"
placeholder="$4"
baseline="$5"
selected_before="$6"
installed_receipt="$7"
owned_placeholder=""
# Canonical read-only receipt identity. Query failures never mean absence.
receipt_identity() {
	local packages info
	packages="$(/usr/sbin/pkgutil --pkgs)" || return 1
	case $'\n'"$packages"$'\n' in
		*$'\ncom.apple.pkg.CLTools_Executables\n'*) ;;
		*) printf '%s\n' none; return 0 ;;
	esac
	info="$(/usr/sbin/pkgutil --pkg-info=com.apple.pkg.CLTools_Executables)" || return 1
	printf '%s\n' "$info" | /usr/bin/awk '
		{
			if (split($0, a, ": ") != 2 || seen[a[1]]++ || a[2] == "") exit 1
			if (a[1] != "package-id" && a[1] != "version" && a[1] != "volume" && a[1] != "location" && a[1] != "install-time") exit 1
			value[a[1]]=a[2]; count++
		}
		END {
			if (count != 5 || value["package-id"] != "com.apple.pkg.CLTools_Executables" || value["volume"] != "/" || value["location"] != "/" ||
				value["version"] !~ /^[0-9]+(\.[0-9]+)*$/ || value["install-time"] !~ /^[1-9][0-9]*$/ || length(value["install-time"]) > 18 || length(value["version"]) > 140) exit 1
			print value["version"] "@" value["install-time"]
		}'
}
selection() {
	local value status
	if value="$(/usr/bin/xcode-select --print-path 2>&1)"; then printf '%s\n' "$value"; return 0; else status=$?; fi
	if [[ "$status" == 2 && "$value" == $'` + appleCLTNoSelectionText + `' ]]; then return 0; fi
	printf 'Cannot inspect Apple developer selection (exit %s): %s\n' "$status" "$value" >&2
	return 1
}
cleanup() {
	status=$?
	trap - EXIT
	if [[ -n "$owned_placeholder" ]]; then
		if [[ -L "$placeholder" || ! -f "$placeholder" ]] ||
				[[ "$(/usr/bin/stat -f '%d:%i' "$placeholder")" != "$owned_placeholder" ]]; then
			printf '%s\n' 'CLT discovery sentinel changed; preserve it and inspect manually.' >&2
			exit 1
		fi
		/bin/rm "$placeholder" || exit 1
	fi
	exit "$status"
}
trap cleanup EXIT
selected="$(selection)"
if [[ -n "$selected" && "$selected" != "$clt" ]]; then
	printf '%s\n' 'A different Apple developer directory is selected; preserve it and review a new plan.' >&2
	exit 1
fi
if [[ "$mode" == install ]]; then
	if [[ -e "$clt" || -L "$clt" ]]; then
		printf '%s\n' 'CLT appeared before installation; preserve it and review a new plan.' >&2
		exit 1
	fi
	current_receipt="$(receipt_identity)" || exit 1
	if [[ "$selected" != "$selected_before" || "$current_receipt" != "$baseline" ]]; then
		printf '%s\n' 'Apple CLT receipt or selection changed from approval; preserve it and replan.' >&2
		exit 1
	fi
	if [[ -L "$placeholder" || ( -e "$placeholder" && ! -f "$placeholder" ) ]]; then
		printf '%s\n' 'The CLT discovery sentinel is not a regular file; inspect it manually.' >&2
		exit 1
	fi
	if [[ ! -e "$placeholder" ]]; then
		(set -C; : > "$placeholder")
		owned_placeholder="$(/usr/bin/stat -f '%d:%i' "$placeholder")"
	fi
	listing="$(/usr/sbin/softwareupdate -l)"
	label="$(printf '%s\n' "$listing" |
		/usr/bin/grep -B 1 -E 'Command Line Tools' |
		/usr/bin/awk -F'*' '/^ *\*/ {print $2}' |
		/usr/bin/sed -e 's/^ *Label: //' -e 's/^ *//' |
		/usr/bin/sort -V | /usr/bin/tail -n1)"
	if [[ "$label" != Command\ Line\ Tools* || "$label" == *$'\n'* || "$label" == *$'\r'* ]]; then
		printf '%s\n' 'Apple did not advertise a compatible CLT package. Run xcode-select --install in a graphical session, complete Apple setup, then abandon this operation and replan to reuse it.' >&2
		exit 1
	fi
	printf 'DOTFILES_CLT_LABEL:%s\n' "$label"
	# Discovery may take time. Preserve a competing installation that appeared.
	current_receipt="$(receipt_identity)" || exit 1
	selected="$(selection)" || exit 1
	if [[ -e "$clt" || -L "$clt" || "$current_receipt" != "$baseline" || "$selected" != "$selected_before" ]]; then
		printf '%s\n' 'CLT appeared during discovery; preserve it and review a new plan.' >&2
		exit 1
	fi
	/usr/sbin/softwareupdate -i "$label"
	installed_receipt="$(receipt_identity)" || exit 1
	if [[ "$installed_receipt" == none || "${installed_receipt#*@}" == "${baseline#*@}" ]]; then
		printf '%s\n' 'Apple CLT installation did not produce a changed native receipt; completion is unproved.' >&2
		exit 1
	fi
	[[ -x "$clt/usr/bin/clang" && -x "$clt/usr/bin/clang++" && -x "$clt/usr/bin/make" ]] || exit 1
	printf 'DOTFILES_CLT_RECEIPT:%s\n' "$installed_receipt"
	printf 'DOTFILES_CLT_INSTALLED:%s\n' "$operation"
elif [[ "$mode" != finish ]]; then
	exit 1
fi
# Recheck before the global selection change: never replace somebody's Xcode.
selected="$(selection)"
if [[ -n "$selected" && "$selected" != "$clt" ]]; then
	printf '%s\n' 'Apple developer selection changed during installation; preserve it.' >&2
	exit 1
fi
current_receipt="$(receipt_identity)" || exit 1
[[ "$current_receipt" == "$installed_receipt" && "$current_receipt" != none && "$current_receipt" != "$baseline" ]] || exit 1
[[ -x "$clt/usr/bin/clang" && -x "$clt/usr/bin/clang++" && -x "$clt/usr/bin/make" ]] || exit 1
/usr/bin/xcode-select --switch "$clt"
printf 'DOTFILES_CLT_SELECTED:%s\n' "$operation"
`

type appleCLTStep struct {
	Mode             string        `json:"mode"`
	Command          nativeCommand `json:"command"`
	InstalledReceipt string        `json:"installed_receipt,omitempty"`
}

func (d *AppleCLTDriver) command(operation string, index int, mode, baseline, selection, installedReceipt string) (nativeCommand, error) {
	if mode != "install" && mode != "finish" || !validAppleCLTReceipt(baseline) || selection != "" && selection != d.clt || mode == "install" && installedReceipt != "" || mode == "finish" && (!validAppleCLTReceipt(installedReceipt) || installedReceipt == "none" || !appleCLTReceiptChanged(baseline, installedReceipt)) {
		return nativeCommand{}, errors.New("invalid Apple CLT command mode")
	}
	id, err := digest(struct {
		Operation string
		Step      int
	}{operation, index})
	if err != nil {
		return nativeCommand{}, err
	}
	command := nativeCommand{Operation: id, Program: "/usr/bin/sudo", Arguments: []string{
		"-n", "/usr/bin/env", "-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C",
		"/bin/bash", "--noprofile", "--norc", "-s", "--", operation, mode, d.clt, d.placeholder, baseline, selection, installedReceipt,
	}, Input: []byte(appleCLTScript)}
	return command, command.validate()
}

func (d *AppleCLTDriver) result(command nativeCommand) (nativeReply, error) {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash {
		return nativeReply{}, errors.Join(errors.New("Apple CLT command differs from its saved native evidence"), err)
	}
	if record.Reply == nil {
		return nativeReply{}, errors.New("Apple CLT command has an unknown outcome; inspect its package receipt and developer selection, repair if needed, then explicitly abandon and replan to reuse the tools")
	}
	if record.Reply.Ready || record.Reply.ExitCode == nil && record.Reply.Error == "" || len(record.Reply.Output) > 1<<20 {
		return nativeReply{}, errors.New("invalid Apple CLT native command result")
	}
	return *record.Reply, nil
}

func appleCLTMarker(reply nativeReply, kind, operation string) bool {
	for _, line := range strings.Split(string(reply.Output), "\n") {
		if line == "DOTFILES_CLT_"+kind+":"+operation {
			return true
		}
	}
	return false
}

func appleCLTSuccess(reply nativeReply) bool {
	return reply.ExitCode != nil && *reply.ExitCode == 0 && reply.Error == ""
}

func (d *AppleCLTDriver) dispatch(ctx context.Context, command nativeCommand) error {
	if d.Authenticate != nil {
		if err := d.Authenticate(ctx); err != nil {
			return err
		}
	}
	_, err := d.Run(ctx, command)
	if err != nil {
		return errors.Join(errors.New("Apple CLT native action failed; administrator access must be authorized in the foreground; inspect the native diagnostic and retry the saved operation"), err)
	}
	return nil
}
