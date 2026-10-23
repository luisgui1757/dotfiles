package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	posixpath "path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Determinate Systems' nix-installer records its own uninstall plan in
// /nix/receipt.json and copies itself to /nix/nix-installer. `uninstall`
// reverts, in reverse order, every action whose state is Completed or Progress
// and continues after individual failures. The official v3.0.0 through v3.23.3
// sources have identical revert code and serialized fields, except
// ConfigureInitService.service_src (a UnitSrc since v3.6.0) and the no-op
// Cleanup action (v3.17.0). The receipt is therefore validated as typed data
// against that reviewed vocabulary; any other version, planner, action, field
// or parameter is refused before anything runs. `uninstall --explain` is not a
// preview: it requires root, re-executes a copy of itself and runs the planner
// checks before confirming.
// https://github.com/DeterminateSystems/nix-installer/blob/192535a8336d446a6ddcbb99fbb03fb047a95f80/src/cli/subcommand/uninstall.rs
const (
	legacyDeterminateReceiptPath   = "/nix/receipt.json"
	legacyDeterminateInstallerPath = "/nix/nix-installer"
	legacyDeterminateNixdPath      = "/usr/local/bin/determinate-nixd"
	legacyDeterminateScratch       = "/nix/temp-install-dir"
	legacyDeterminateKeychain      = "/Library/Keychains/System.keychain"
	legacyDeterminateReceiptLimit  = 512 << 10
)

var (
	legacyDeterminateVersion = regexp.MustCompile(`^3\.(0|[1-9][0-9]{0,3})\.(0|[1-9][0-9]{0,3})$`)
	// The newest vendor release whose uninstall semantics were reviewed.
	legacyDeterminateNewest       = [2]int{23, 3}
	legacyDeterminateDisk         = regexp.MustCompile(`^disk[0-9]{1,3}$`)
	legacyDeterminateRepair       = regexp.MustCompile(`^receipt\.pre-repair\.[0-9]{1,20}\.json$`)
	legacyDeterminateEvidenceName = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)
	// ConfigureShellProfile also edits $GITHUB_PATH when that variable reached
	// the installer; the runner names those files by a lowercase GUID.
	legacyDeterminateGitHubPath = regexp.MustCompile(`^/.+/_runner_file_commands/add_path_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Exact vendor insertions. Revert removes the last occurrence of this text.
const (
	legacyDeterminateShellHook = "\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n"
	legacyDeterminateFishHook  = "\n# Nix\nif test -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\nend\n# End Nix\n\n"
	legacyDeterminateGitHubBin = "/nix/var/nix/profiles/default/bin\n"
)

var (
	legacyDeterminateSSHHooks = []string{
		"\n# Set up Nix only on SSH connections\n# See: https://github.com/DeterminateSystems/nix-installer/pull/714\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ] && [ -n \"${SSH_CONNECTION}\" ] && [ \"${SHLVL}\" -eq 1 ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n",
		"\n# Set up Nix only on SSH connections\n# See: https://github.com/DeterminateSystems/nix-installer/pull/714\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ] && [ -n \"${SSH_CONNECTION:-}\" ] && [ \"${SHLVL:-0}\" -eq 1 ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n",
	}
	legacyDeterminateGitHubBufs = []string{legacyDeterminateGitHubBin, legacyDeterminateGitHubBin + "/home/runner/.nix-profile/bin\n", legacyDeterminateGitHubBin + "/Users/runner/.nix-profile/bin\n"}
	legacyDeterminateShellFiles = []string{"/etc/bashrc", "/etc/profile.d/nix.sh", "/etc/bash.bashrc", "/etc/zshrc", "/etc/zsh/zshrc"}
	legacyDeterminateFishFiles  = []string{"/etc/fish/conf.d/nix.fish", "/usr/local/etc/fish/conf.d/nix.fish", "/opt/homebrew/etc/fish/conf.d/nix.fish",
		"/opt/local/etc/fish/conf.d/nix.fish", "/usr/share/fish/vendor_conf.d/nix.fish", "/usr/local/share/fish/vendor_conf.d/nix.fish"}
	legacyDeterminateStates = []string{"Completed", "Progress", "Uncompleted", "Skipped"}
)

type legacyDeterminateUser struct {
	Name, Comment string
	UID, GID      uint32
	Reverted      bool
}

type legacyDeterminateInsert struct {
	Path, Buf string
	Reverted  bool
}

type legacyDeterminateVolume struct {
	Disk, Name  string
	Determinate bool
	// Deleted is CreateApfsVolume's revert: unmount, then deleteVolume by name.
	Deleted bool
	// Keychain is EncryptApfsVolume's revert, which deletes the password by
	// the recorded name and disk. The vendor skips it if deletion failed.
	Keychain bool
}

// legacyDeterminateReceipt is the reviewed meaning of one vendor receipt:
// exactly what its uninstall would revert.
type legacyDeterminateReceipt struct {
	Version     string
	Planner     string // linux or macos
	Determinate bool   // Determinate Nix with determinate-nixd; otherwise upstream Nix
	Init        string // Systemd, None or Launchd
	StartDaemon bool
	Group       string
	GroupID     uint32
	GroupRevert bool
	Users       []legacyDeterminateUser
	Inserts     []legacyDeterminateInsert
	// Prune lists directories outside /nix that the vendor deletes recursively.
	Prune []string
	// Removed lists files the vendor deletes.
	Removed  []string
	Units    []string // systemd units the vendor stops and disables
	Services []string // launchd labels the vendor boots out
	Nixd     bool     // the vendor removes determinate-nixd
	Root     bool     // the vendor deletes /nix itself (Linux)
	Volume   *legacyDeterminateVolume
}

type legacyDeterminateFields struct {
	where  string
	values map[string]json.RawMessage
}

// Object fields are read exactly once. serde rejects duplicate struct fields,
// so a duplicate here would hide what was validated from what the vendor reads.
func legacyDeterminateObject(raw json.RawMessage, where string) (legacyDeterminateFields, error) {
	fields := legacyDeterminateFields{where: where, values: map[string]json.RawMessage{}}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return fields, fmt.Errorf("%s is not an object", where)
	}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return fields, fmt.Errorf("%s has an invalid field name", where)
		}
		if _, duplicate := fields.values[key]; duplicate {
			return fields, fmt.Errorf("%s repeats field %q", where, key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fields, fmt.Errorf("%s.%s is not valid JSON", where, key)
		}
		fields.values[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return fields, fmt.Errorf("%s is not a complete object", where)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fields, fmt.Errorf("%s has trailing content", where)
	}
	return fields, nil
}

func (f legacyDeterminateFields) exact(keys ...string) error {
	for _, key := range keys {
		if _, ok := f.values[key]; !ok {
			return fmt.Errorf("%s lacks the reviewed field %s", f.where, key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(f.values)) {
		if !slices.Contains(keys, key) {
			return fmt.Errorf("%s has the unreviewed field %s", f.where, key)
		}
	}
	return nil
}

func legacyJSONKind(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	switch trimmed[0] {
	case 'n':
		return "null"
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "bool"
	}
	return "number"
}

func (f legacyDeterminateFields) text(key string) (string, error) {
	var value string
	if legacyJSONKind(f.values[key]) != "string" || json.Unmarshal(f.values[key], &value) != nil {
		return "", fmt.Errorf("%s.%s must be a string", f.where, key)
	}
	return value, nil
}

func (f legacyDeterminateFields) is(key string, allowed ...string) (string, error) {
	value, err := f.text(key)
	if err != nil {
		return "", err
	}
	if !slices.Contains(allowed, value) {
		return "", fmt.Errorf("%s.%s is %q, outside the reviewed values", f.where, key, truncateLegacyText(value, 256))
	}
	return value, nil
}

func (f legacyDeterminateFields) path(key string) (string, error) {
	value, err := f.text(key)
	// Receipts describe a POSIX target even when inspected on Windows.
	if err == nil && (!posixpath.IsAbs(value) || posixpath.Clean(value) != value || value == "/" || strings.ContainsAny(value, "\x00\r\n")) {
		err = fmt.Errorf("%s.%s is not a canonical absolute path", f.where, key)
	}
	return value, err
}

func (f legacyDeterminateFields) flag(key string) (bool, error) {
	var value bool
	if legacyJSONKind(f.values[key]) != "bool" || json.Unmarshal(f.values[key], &value) != nil {
		return false, fmt.Errorf("%s.%s must be a boolean", f.where, key)
	}
	return value, nil
}

func (f legacyDeterminateFields) number(key string) (uint32, error) {
	var value uint32
	if legacyJSONKind(f.values[key]) != "number" || json.Unmarshal(f.values[key], &value) != nil {
		return 0, fmt.Errorf("%s.%s must be an unsigned 32-bit number", f.where, key)
	}
	return value, nil
}

// optional accepts null or one of the listed JSON kinds for fields the vendor
// uninstall never reads.
func (f legacyDeterminateFields) optional(key string, kinds ...string) error {
	if kind := legacyJSONKind(f.values[key]); kind != "null" && !slices.Contains(kinds, kind) {
		return fmt.Errorf("%s.%s has an unreviewed type", f.where, key)
	}
	return nil
}

func (f legacyDeterminateFields) null(key string) error {
	if legacyJSONKind(f.values[key]) != "null" {
		return fmt.Errorf("%s.%s must be null", f.where, key)
	}
	return nil
}

func legacyDeterminateArray(raw json.RawMessage, where string, limit int) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if legacyJSONKind(raw) != "array" || json.Unmarshal(raw, &items) != nil {
		return nil, fmt.Errorf("%s must be an array", where)
	}
	if len(items) > limit {
		return nil, fmt.Errorf("%s exceeds its review bound", where)
	}
	return items, nil
}

// legacyDeterminateAction decodes one StatefulAction. The vendor reverts an
// action only when its parent reverts it and its own state is Completed or
// Progress.
func legacyDeterminateAction(raw json.RawMessage, where string, parent bool) (string, legacyDeterminateFields, bool, error) {
	wrapper, err := legacyDeterminateObject(raw, where)
	if err == nil {
		err = wrapper.exact("action", "state")
	}
	if err != nil {
		return "", wrapper, false, err
	}
	state, err := wrapper.is("state", legacyDeterminateStates...)
	if err != nil {
		return "", wrapper, false, err
	}
	fields, err := legacyDeterminateObject(wrapper.values["action"], where)
	if err != nil {
		return "", fields, false, err
	}
	name, err := fields.text("action_name")
	if err != nil {
		return "", fields, false, err
	}
	fields.where = where + "<" + name + ">"
	return name, fields, parent && (state == "Completed" || state == "Progress"), nil
}

type legacyDeterminateParser struct {
	r     legacyDeterminateReceipt
	nodes int
	// distribution values recorded by nested actions, checked for consistency.
	distributions []string
	storeGID      *uint32
	settings      struct {
		group, prefix string
		gid, count    uint32
		base          uint32
	}
	volumeLabel, rootDisk string
	roots                 []string // top-level Linux directories
}

func legacyDeterminateVersionParts(version string) ([2]int, bool) {
	match := legacyDeterminateVersion.FindStringSubmatch(version)
	if match == nil {
		return [2]int{}, false
	}
	minor, _ := strconv.Atoi(match[1])
	patch, _ := strconv.Atoi(match[2])
	return [2]int{minor, patch}, true
}

func legacyVersionBefore(left, right [2]int) bool {
	return left[0] < right[0] || left[0] == right[0] && left[1] < right[1]
}

// parseLegacyDeterminateReceipt validates the receipt as the reviewed vendor
// data model. The error is a resource-local refusal naming the first
// unreviewed location; nothing about an unrecognized receipt is guessed.
func parseLegacyDeterminateReceipt(data []byte) (legacyDeterminateReceipt, error) {
	p := &legacyDeterminateParser{}
	if len(data) > legacyDeterminateReceiptLimit {
		return p.r, errors.New("the receipt exceeds its review bound")
	}
	top, err := legacyDeterminateObject(data, "receipt")
	if err == nil {
		err = top.exact("version", "actions", "planner")
	}
	if err != nil {
		return p.r, err
	}
	if p.r.Version, err = top.text("version"); err != nil {
		return p.r, err
	}
	version, ok := legacyDeterminateVersionParts(p.r.Version)
	if !ok || legacyVersionBefore(legacyDeterminateNewest, version) {
		return p.r, fmt.Errorf("receipt version %q is outside the reviewed vendor releases 3.0.0 through 3.%d.%d", truncateLegacyText(p.r.Version, 64), legacyDeterminateNewest[0], legacyDeterminateNewest[1])
	}
	if err := p.planner(top.values["planner"]); err != nil {
		return p.r, err
	}
	actions, err := legacyDeterminateArray(top.values["actions"], "receipt.actions", 64)
	if err != nil {
		return p.r, err
	}
	allowed := map[string][]string{
		"linux": {"create_directory", "provision_determinate_nixd", "provision_nix", "create_users_and_group", "configure_nix",
			"configure_determinate_nixd_init_service", "create_upstream_init_service", "cleanup", "remove_directory"},
		"macos": {"provision_determinate_nixd", "create_determinate_nix_volume", "create_nix_volume", "provision_nix", "create_users_and_group",
			"set_tmutil_exclusions", "configure_nix", "configure_remote_building", "create_nix_hook_service",
			"configure_determinate_nixd_init_service", "create_upstream_init_service", "remove_directory"},
	}[p.r.Planner]
	seen := map[string]int{}
	for i, raw := range actions {
		name, fields, reverted, err := legacyDeterminateAction(raw, fmt.Sprintf("actions[%d]", i), true)
		if err != nil {
			return p.r, err
		}
		if !slices.Contains(allowed, name) {
			return p.r, fmt.Errorf("actions[%d] is %q, not a reviewed top-level %s action", i, truncateLegacyText(name, 128), p.r.Planner)
		}
		if seen[name]++; seen[name] > 1 && name != "create_directory" || seen[name] > 2 {
			return p.r, fmt.Errorf("actions[%d] repeats %s", i, name)
		}
		if err := p.visit(name, fields, reverted, "root"); err != nil {
			return p.r, err
		}
	}
	return p.r, p.consistent(seen)
}

func (p *legacyDeterminateParser) planner(raw json.RawMessage) error {
	f, err := legacyDeterminateObject(raw, "planner")
	if err != nil {
		return err
	}
	if p.r.Planner, err = f.text("planner"); err != nil {
		return err
	}
	switch p.r.Planner {
	case "linux":
		if err := f.exact("planner", "settings", "init"); err != nil {
			return err
		}
		init, err := legacyDeterminateObject(f.values["init"], "planner.init")
		if err == nil {
			err = init.exact("init", "start_daemon")
		}
		if err != nil {
			return err
		}
		if p.r.Init, err = init.is("init", "Systemd", "None"); err != nil {
			return err
		}
		if p.r.StartDaemon, err = init.flag("start_daemon"); err != nil {
			return err
		}
	case "macos":
		if err := f.exact("planner", "settings", "encrypt", "case_sensitive", "volume_label", "root_disk", "use_ec2_instance_store"); err != nil {
			return err
		}
		p.r.Init, p.r.StartDaemon = "Launchd", true
		if err := f.optional("encrypt", "bool"); err != nil {
			return err
		}
		if err := f.optional("root_disk", "string"); err != nil {
			return err
		}
		if _, err := f.flag("case_sensitive"); err != nil {
			return err
		}
		if p.volumeLabel, err = f.text("volume_label"); err != nil {
			return err
		}
		if legacyJSONKind(f.values["root_disk"]) == "string" {
			p.rootDisk, _ = f.text("root_disk")
		}
		if ec2, err := f.flag("use_ec2_instance_store"); err != nil || ec2 {
			return errors.Join(errors.New("EC2 instance-store volumes are not a reviewed vendor layout"), err)
		}
	default:
		return fmt.Errorf("the %q planner is not reviewed; only the Linux and macOS planners are supported", truncateLegacyText(p.r.Planner, 64))
	}
	settings, err := legacyDeterminateObject(f.values["settings"], "planner.settings")
	if err != nil {
		return err
	}
	keys := []string{"determinate_nix", "modify_profile", "nix_build_group_name", "nix_build_group_id", "nix_build_user_prefix", "nix_build_user_count",
		"nix_build_user_id_base", "nix_package_url", "proxy", "ssl_cert_file", "extra_conf", "force", "skip_nix_conf"}
	if _, ok := settings.values["prefer_upstream"]; ok {
		keys = append(keys, "prefer_upstream") // added in v3.11.2
	}
	if err := settings.exact(keys...); err != nil {
		return err
	}
	for _, key := range []string{"determinate_nix", "modify_profile", "force", "skip_nix_conf", "prefer_upstream"} {
		if _, ok := settings.values[key]; ok {
			if _, err := settings.flag(key); err != nil {
				return err
			}
		}
	}
	if err := errors.Join(settings.optional("nix_package_url", "string", "object"), settings.optional("proxy", "string"), settings.optional("ssl_cert_file", "string")); err != nil {
		return err
	}
	if legacyJSONKind(settings.values["extra_conf"]) != "array" {
		return errors.New("planner.settings.extra_conf must be an array")
	}
	s := &p.settings
	if s.group, err = settings.text("nix_build_group_name"); err != nil {
		return err
	}
	if s.prefix, err = settings.text("nix_build_user_prefix"); err != nil {
		return err
	}
	for _, item := range []struct {
		key    string
		target *uint32
	}{{"nix_build_group_id", &s.gid}, {"nix_build_user_count", &s.count}, {"nix_build_user_id_base", &s.base}} {
		if *item.target, err = settings.number(item.key); err != nil {
			return err
		}
	}
	return nil
}

func (p *legacyDeterminateParser) child(parent legacyDeterminateFields, key string, reverted bool, name, context string) error {
	actual, fields, effective, err := legacyDeterminateAction(parent.values[key], parent.where+"."+key, reverted)
	if err != nil {
		return err
	}
	if actual != name {
		return fmt.Errorf("%s.%s is %q, not the reviewed %s", parent.where, key, truncateLegacyText(actual, 128), name)
	}
	return p.visit(actual, fields, effective, context)
}

func (p *legacyDeterminateParser) optionalChild(parent legacyDeterminateFields, key string, reverted bool, name, context string) error {
	if legacyJSONKind(parent.values[key]) == "null" {
		return nil
	}
	return p.child(parent, key, reverted, name, context)
}

func (p *legacyDeterminateParser) children(parent legacyDeterminateFields, key string, reverted bool, name, context string, limit int) error {
	items, err := legacyDeterminateArray(parent.values[key], parent.where+"."+key, limit)
	if err != nil {
		return err
	}
	for i, raw := range items {
		where := fmt.Sprintf("%s.%s[%d]", parent.where, key, i)
		actual, fields, effective, err := legacyDeterminateAction(raw, where, reverted)
		if err != nil {
			return err
		}
		if actual != name {
			return fmt.Errorf("%s is %q, not the reviewed %s", where, truncateLegacyText(actual, 128), name)
		}
		if err := p.visit(actual, fields, effective, context); err != nil {
			return err
		}
	}
	return nil
}

func (p *legacyDeterminateParser) remove(reverted bool, paths ...string) {
	if reverted {
		p.r.Removed = append(p.r.Removed, paths...)
	}
}

// visit validates one action by its reviewed vendor type and records what
// its revert does. Context names the reviewed parent position.
func (p *legacyDeterminateParser) visit(name string, f legacyDeterminateFields, reverted bool, context string) error {
	if p.nodes++; p.nodes > 1024 {
		return errors.New("the receipt exceeds its review bound")
	}
	switch name {
	case "create_directory":
		return p.directory(f, reverted, context)
	case "create_or_insert_into_file":
		return p.insert(f, reverted, context)
	case "create_or_merge_nix_config":
		if err := f.exact("action_name", "path", "pending_nix_config", "header", "footer"); err != nil {
			return err
		}
		if _, err := f.is("path", context); err != nil {
			return err
		}
		if _, err := legacyDeterminateObject(f.values["pending_nix_config"], f.where+".pending_nix_config"); err != nil {
			return err
		}
		if _, err := f.text("header"); err != nil {
			return err
		}
		if err := f.optional("footer", "string"); err != nil {
			return err
		}
		p.remove(reverted, context)
		return nil
	case "place_nix_configuration":
		if err := f.exact("action_name", "create_directory", "create_or_merge_standard_nix_config", "create_or_merge_custom_nix_config"); err != nil {
			return err
		}
		if err := p.child(f, "create_directory", reverted, "create_directory", "config"); err != nil {
			return err
		}
		if err := p.optionalChild(f, "create_or_merge_standard_nix_config", reverted, "create_or_merge_nix_config", "/etc/nix/nix.conf"); err != nil {
			return err
		}
		return p.child(f, "create_or_merge_custom_nix_config", reverted, "create_or_merge_nix_config", "/etc/nix/nix.custom.conf")
	case "configure_nix":
		if err := f.exact("action_name", "setup_default_profile", "configure_shell_profile", "place_nix_configuration"); err != nil {
			return err
		}
		if err := p.child(f, "setup_default_profile", reverted, "setup_default_profile", ""); err != nil {
			return err
		}
		if err := p.optionalChild(f, "configure_shell_profile", reverted, "configure_shell_profile", "shell"); err != nil {
			return err
		}
		return p.optionalChild(f, "place_nix_configuration", reverted, "place_nix_configuration", "")
	case "setup_default_profile", "mount_unpacked_nix":
		// Neither revert changes the host; their scratch input is in /nix.
		if err := f.exact("action_name", "unpacked_path"); err != nil {
			return err
		}
		_, err := f.is("unpacked_path", legacyDeterminateScratch)
		return err
	case "remove_directory":
		if err := f.exact("action_name", "path"); err != nil {
			return err
		}
		_, err := f.is("path", legacyDeterminateScratch)
		return err
	case "cleanup":
		if err := f.exact("action_name", "remove_scratch_dir", "original_home_dir"); err != nil {
			return err
		}
		if err := f.optional("original_home_dir", "string"); err != nil {
			return err
		}
		return p.child(f, "remove_scratch_dir", reverted, "remove_directory", "")
	case "configure_shell_profile":
		if err := f.exact("action_name", "locations", "create_directories", "create_or_insert_into_files"); err != nil {
			return err
		}
		if err := legacyDeterminateLocations(f.values["locations"], f.where+".locations"); err != nil {
			return err
		}
		if err := p.children(f, "create_directories", reverted, "create_directory", "shell", 32); err != nil {
			return err
		}
		return p.children(f, "create_or_insert_into_files", reverted, "create_or_insert_into_file", "shell", 32)
	case "provision_nix":
		if err := f.exact("action_name", "nix_store_gid", "fetch_nix", "create_nix_tree", "move_unpacked_nix"); err != nil {
			return err
		}
		gid, err := f.number("nix_store_gid")
		if err != nil {
			return err
		}
		p.storeGID = &gid
		for _, item := range []struct{ key, name string }{{"fetch_nix", "fetch_and_unpack_nix"}, {"create_nix_tree", "create_nix_tree"}, {"move_unpacked_nix", "mount_unpacked_nix"}} {
			if err := p.child(f, item.key, reverted, item.name, ""); err != nil {
				return err
			}
		}
		return nil
	case "fetch_and_unpack_nix":
		if err := f.exact("action_name", "distribution", "url_or_path", "dest", "proxy", "ssl_cert_file"); err != nil {
			return err
		}
		distribution, err := f.is("distribution", "DeterminateNix", "Nix")
		if err != nil {
			return err
		}
		p.distributions = append(p.distributions, distribution)
		if _, err := f.is("dest", legacyDeterminateScratch); err != nil {
			return err
		}
		if err := errors.Join(f.optional("url_or_path", "string", "object"), f.optional("proxy", "string"), f.optional("ssl_cert_file", "string")); err != nil {
			return err
		}
		return nil
	case "create_nix_tree":
		if err := f.exact("action_name", "create_directories"); err != nil {
			return err
		}
		return p.children(f, "create_directories", reverted, "create_directory", "tree", 64)
	case "create_users_and_group":
		return p.users(f, reverted)
	case "provision_determinate_nixd":
		if err := f.exact("action_name", "binary_location"); err != nil {
			return err
		}
		if _, err := f.is("binary_location", legacyDeterminateNixdPath); err != nil {
			return err
		}
		p.distributions = append(p.distributions, "DeterminateNix")
		// Its revert also deletes the daemon-managed /etc/nix/nix.conf.
		p.r.Nixd = p.r.Nixd || reverted
		p.remove(reverted, legacyDeterminateNixdPath, "/etc/nix/nix.conf")
		return nil
	case "configure_determinate_nixd_init_service":
		if err := f.exact("action_name", "init", "configure_init_service"); err != nil {
			return err
		}
		if _, err := f.is("init", p.r.Init); err != nil {
			return err
		}
		p.r.Determinate = true
		return p.child(f, "configure_init_service", reverted, "configure_init_service", "determinate")
	case "create_upstream_init_service":
		if err := f.exact("action_name", "configure_init_service"); err != nil {
			return err
		}
		return p.child(f, "configure_init_service", reverted, "configure_init_service", "upstream")
	case "configure_init_service":
		return p.initService(f, reverted, context == "determinate")
	case "create_determinate_nix_volume", "create_nix_volume":
		return p.volume(f, reverted, name == "create_determinate_nix_volume")
	case "set_tmutil_exclusions":
		// tmutil removeexclusion only clears Time Machine metadata.
		if err := f.exact("action_name", "set_tmutil_exclusions"); err != nil {
			return err
		}
		return p.children(f, "set_tmutil_exclusions", reverted, "set_tmutil_exclusion", "", 4)
	case "set_tmutil_exclusion":
		if err := f.exact("action_name", "path"); err != nil {
			return err
		}
		_, err := f.is("path", "/nix/store", "/nix/var")
		return err
	case "configure_remote_building":
		if err := f.exact("action_name", "create_or_insert_into_file"); err != nil {
			return err
		}
		return p.optionalChild(f, "create_or_insert_into_file", reverted, "create_or_insert_into_file", "remote")
	case "create_nix_hook_service":
		if err := f.exact("action_name", "path", "service_label", "needs_bootout"); err != nil {
			return err
		}
		if _, err := f.is("path", "/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist"); err != nil {
			return err
		}
		if _, err := f.is("service_label", "systems.determinate.nix-installer.nix-hook"); err != nil {
			return err
		}
		if _, err := f.flag("needs_bootout"); err != nil {
			return err
		}
		p.remove(reverted, "/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist")
		return nil
	}
	return fmt.Errorf("%s is not a reviewed vendor action", f.where)
}

func legacyDeterminateLocations(raw json.RawMessage, where string) error {
	locations, err := legacyDeterminateObject(raw, where)
	if err == nil {
		err = locations.exact("fish", "bash", "zsh")
	}
	if err != nil {
		return err
	}
	fish, err := legacyDeterminateObject(locations.values["fish"], where+".fish")
	if err == nil {
		err = fish.exact("confd_suffix", "confd_prefixes", "vendor_confd_suffix", "vendor_confd_prefixes")
	}
	if err != nil {
		return err
	}
	for _, list := range []struct {
		fields legacyDeterminateFields
		key    string
	}{{locations, "bash"}, {locations, "zsh"}, {fish, "confd_prefixes"}, {fish, "vendor_confd_prefixes"}} {
		var values []string
		if legacyJSONKind(list.fields.values[list.key]) != "array" || json.Unmarshal(list.fields.values[list.key], &values) != nil {
			return fmt.Errorf("%s.%s must be a list of paths", list.fields.where, list.key)
		}
	}
	for _, key := range []string{"confd_suffix", "vendor_confd_suffix"} {
		if _, err := fish.text(key); err != nil {
			return err
		}
	}
	return nil
}

// Directory reverts delete recursively when force_prune_on_revert is set and
// otherwise remove only an empty directory.
func (p *legacyDeterminateParser) directory(f legacyDeterminateFields, reverted bool, context string) error {
	if err := f.exact("action_name", "path", "user", "group", "mode", "is_mountpoint", "force_prune_on_revert"); err != nil {
		return err
	}
	path, err := f.path("path")
	if err != nil {
		return err
	}
	for _, key := range []string{"user", "group"} {
		if err := f.null(key); err != nil {
			return err
		}
	}
	if err := f.optional("mode", "number"); err != nil {
		return err
	}
	mountpoint, err := f.flag("is_mountpoint")
	if err != nil {
		return err
	}
	prune, err := f.flag("force_prune_on_revert")
	if err != nil {
		return err
	}
	if mountpoint {
		return fmt.Errorf("%s recorded %s as an existing mount point; its vendor revert would empty that filesystem", f.where, path)
	}
	shell := []string{}
	for _, file := range append(slices.Clone(legacyDeterminateShellFiles), legacyDeterminateFishFiles...) {
		shell = append(shell, posixpath.Dir(file))
	}
	allowed := false
	switch context {
	case "root":
		allowed = p.r.Planner == "linux" && (path == "/nix" && prune || path == "/etc/tmpfiles.d" && !prune) && !slices.Contains(p.roots, path)
		p.roots = append(p.roots, path)
		p.r.Root = p.r.Root || path == "/nix" && reverted
	case "tree":
		allowed = prune && (path == "/nix/var" || strings.HasPrefix(path, "/nix/var/"))
	case "config":
		// settings.force selects pruning of all of /etc/nix.
		allowed = path == "/etc/nix"
	case "shell":
		allowed = !prune && slices.Contains(shell, path)
	}
	if !allowed {
		return fmt.Errorf("%s (%s, prune %t) is outside the reviewed vendor directories", f.where, path, prune)
	}
	if reverted && prune && path != "/nix" && !strings.HasPrefix(path, "/nix/") && !slices.Contains(p.r.Prune, path) {
		p.r.Prune = append(p.r.Prune, path)
	}
	return nil
}

func (p *legacyDeterminateParser) insert(f legacyDeterminateFields, reverted bool, context string) error {
	if err := f.exact("action_name", "path", "user", "group", "mode", "buf", "position"); err != nil {
		return err
	}
	path, err := f.path("path")
	if err != nil {
		return err
	}
	for _, key := range []string{"user", "group"} {
		if err := f.null(key); err != nil {
			return err
		}
	}
	if err := f.optional("mode", "number"); err != nil {
		return err
	}
	buf, err := f.text("buf")
	if err != nil {
		return err
	}
	position, err := f.is("position", "Beginning", "End")
	if err != nil {
		return err
	}
	allowed := false
	switch context {
	case "shell":
		allowed = position == "Beginning" && (slices.Contains(legacyDeterminateShellFiles, path) && buf == legacyDeterminateShellHook ||
			slices.Contains(legacyDeterminateFishFiles, path) && buf == legacyDeterminateFishHook) ||
			position == "End" && legacyDeterminateGitHubPath.MatchString(path) && slices.Contains(legacyDeterminateGitHubBufs, buf)
	case "synthetic":
		allowed = path == "/etc/synthetic.conf" && buf == "nix\n" && position == "End"
	case "remote":
		allowed = path == "/etc/zshenv" && position == "Beginning" && slices.Contains(legacyDeterminateSSHHooks, buf)
	}
	if !allowed {
		return fmt.Errorf("%s edits %s with unreviewed text or position", f.where, truncateLegacyText(path, 256))
	}
	if slices.ContainsFunc(p.r.Inserts, func(insert legacyDeterminateInsert) bool { return insert.Path == path }) {
		return fmt.Errorf("%s repeats an edit of %s", f.where, path)
	}
	p.r.Inserts = append(p.r.Inserts, legacyDeterminateInsert{Path: path, Buf: buf, Reverted: reverted})
	return nil
}

func (p *legacyDeterminateParser) users(f legacyDeterminateFields, reverted bool) error {
	if err := f.exact("action_name", "nix_build_group_name", "nix_build_group_id", "nix_build_user_count", "nix_build_user_prefix",
		"nix_build_user_id_base", "create_group", "create_users", "add_users_to_groups"); err != nil {
		return err
	}
	prefix := map[string]string{"linux": "nixbld", "macos": "_nixbld"}[p.r.Planner]
	group, err := f.is("nix_build_group_name", "nixbld")
	if err != nil {
		return err
	}
	if _, err := f.is("nix_build_user_prefix", prefix); err != nil {
		return err
	}
	gid, err := f.number("nix_build_group_id")
	if err != nil {
		return err
	}
	count, err := f.number("nix_build_user_count")
	if err != nil {
		return err
	}
	base, err := f.number("nix_build_user_id_base")
	if err != nil {
		return err
	}
	s := p.settings
	if count < 1 || count > 32 || base > 1<<31 || s.group != group || s.prefix != prefix || s.gid != gid || s.count != count || s.base != base {
		return fmt.Errorf("%s differs from the planner build-account settings or the reviewed default names", f.where)
	}
	p.r.Group, p.r.GroupID = group, gid
	_, groupFields, groupReverted, err := legacyDeterminateAction(f.values["create_group"], f.where+".create_group", reverted)
	if err == nil {
		err = groupFields.exact("action_name", "name", "gid")
	}
	if err != nil {
		return err
	}
	if _, err := groupFields.is("action_name", "create_group"); err != nil {
		return err
	}
	if _, err := groupFields.is("name", group); err != nil {
		return err
	}
	if value, err := groupFields.number("gid"); err != nil || value != gid {
		return errors.Join(fmt.Errorf("%s.create_group differs from the build group", f.where), err)
	}
	p.r.GroupRevert = groupReverted
	for _, list := range []string{"create_users", "add_users_to_groups"} {
		items, err := legacyDeterminateArray(f.values[list], f.where+"."+list, 32)
		if err != nil {
			return err
		}
		if len(items) != int(count) {
			return fmt.Errorf("%s.%s does not list exactly %d build accounts", f.where, list, count)
		}
		for i, raw := range items {
			index := uint32(i + 1)
			name, fields, effective, err := legacyDeterminateAction(raw, fmt.Sprintf("%s.%s[%d]", f.where, list, i), reverted)
			if err != nil {
				return err
			}
			want := map[string]string{"create_users": "create_user", "add_users_to_groups": "add_user_to_group"}[list]
			keys := []string{"action_name", "name", "uid", "groupname", "gid"}
			if list == "create_users" {
				keys = append(keys, "comment")
			}
			if name != want {
				return fmt.Errorf("%s is %q, not the reviewed %s", fields.where, truncateLegacyText(name, 128), want)
			}
			if err := fields.exact(keys...); err != nil {
				return err
			}
			user := legacyDeterminateUser{Name: prefix + strconv.FormatUint(uint64(index), 10), UID: base + index, GID: gid,
				Comment: "Nix build user " + strconv.FormatUint(uint64(index), 10), Reverted: effective}
			if _, err := fields.is("name", user.Name); err != nil {
				return err
			}
			if _, err := fields.is("groupname", group); err != nil {
				return err
			}
			uid, uidErr := fields.number("uid")
			actualGID, gidErr := fields.number("gid")
			if err := errors.Join(uidErr, gidErr); err != nil || uid != user.UID || actualGID != gid {
				return errors.Join(fmt.Errorf("%s does not have the reviewed build-account identity", fields.where), err)
			}
			if list == "create_users" {
				if _, err := fields.is("comment", user.Comment); err != nil {
					return err
				}
				p.r.Users = append(p.r.Users, user)
			}
			// CreateUsersAndGroups never reverts AddUserToGroup: deleting the
			// account removes its membership.
		}
	}
	return nil
}

// The vendor stops and disables these exact units, or boots out this exact
// launchd label, then deletes the recorded destinations.
func (p *legacyDeterminateParser) initService(f legacyDeterminateFields, reverted, determinate bool) error {
	if err := f.exact("action_name", "init", "start_daemon", "service_src", "service_name", "service_dest", "socket_files"); err != nil {
		return err
	}
	if _, err := f.is("init", p.r.Init); err != nil {
		return err
	}
	if _, err := f.flag("start_daemon"); err != nil {
		return err
	}
	// Revert never reads the source. Before v3.6.0 it was a plain path.
	switch legacyJSONKind(f.values["service_src"]) {
	case "null", "string":
	case "object":
		if err := legacyDeterminateUnitSource(f.values["service_src"], f.where+".service_src"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s.service_src has an unreviewed type", f.where)
	}
	sockets := []string{"nix-daemon.socket"}
	label, plist := "org.nixos.nix-daemon", "/Library/LaunchDaemons/org.nixos.nix-daemon.plist"
	if determinate {
		sockets = append(sockets, "determinate-nixd.socket")
		label, plist = "systems.determinate.nix-daemon", "/Library/LaunchDaemons/systems.determinate.nix-daemon.plist"
	}
	items, err := legacyDeterminateArray(f.values["socket_files"], f.where+".socket_files", 4)
	if err != nil {
		return err
	}
	names := []string{}
	for i, raw := range items {
		socket, err := legacyDeterminateObject(raw, fmt.Sprintf("%s.socket_files[%d]", f.where, i))
		if err == nil {
			err = socket.exact("name", "src", "dest")
		}
		if err != nil {
			return err
		}
		name, err := socket.is("name", sockets...)
		if err != nil {
			return err
		}
		if _, err := socket.is("dest", "/etc/systemd/system/"+name); err != nil {
			return err
		}
		if err := legacyDeterminateUnitSource(socket.values["src"], socket.where+".src"); err != nil {
			return err
		}
		names = append(names, name)
	}
	if !slices.Equal(names, sockets) {
		return fmt.Errorf("%s.socket_files is not the reviewed socket set", f.where)
	}
	switch p.r.Init {
	case "Systemd":
		if _, err := f.is("service_dest", "/etc/systemd/system/nix-daemon.service"); err != nil {
			return err
		}
		if err := f.optional("service_name", "string"); err != nil {
			return err
		}
		if legacyJSONKind(f.values["service_name"]) == "string" {
			if _, err := f.is("service_name", "nix-daemon.service"); err != nil {
				return err
			}
		}
		if reverted {
			p.r.Units = append(slices.Clone(sockets), "nix-daemon.service")
			p.remove(true, "/etc/systemd/system/nix-daemon.service", "/etc/tmpfiles.d/nix-daemon.conf")
			for _, name := range sockets {
				p.remove(true, "/etc/systemd/system/"+name)
			}
		}
	case "None":
		if err := errors.Join(f.null("service_dest"), f.null("service_name")); err != nil {
			return err
		}
		// Without an init the vendor still deletes the recorded socket files.
		for _, name := range sockets {
			p.remove(reverted, "/etc/systemd/system/"+name)
		}
	case "Launchd":
		if _, err := f.is("service_name", label); err != nil {
			return err
		}
		if _, err := f.is("service_dest", plist); err != nil {
			return err
		}
		if reverted {
			p.r.Services = append(p.r.Services, label)
		}
		p.remove(reverted, plist)
	}
	return nil
}

func legacyDeterminateUnitSource(raw json.RawMessage, where string) error {
	source, err := legacyDeterminateObject(raw, where)
	if err != nil {
		return err
	}
	if len(source.values) != 1 {
		return fmt.Errorf("%s is not a single unit source", where)
	}
	for key := range source.values {
		if key != "Path" && key != "Literal" {
			return fmt.Errorf("%s has the unreviewed unit source %s", where, key)
		}
		if _, err := source.text(key); err != nil {
			return err
		}
	}
	return nil
}

// macOS volume reverts locate the APFS volume by its recorded name and the
// keychain password by name and disk, so both are bound natively later.
func (p *legacyDeterminateParser) volume(f legacyDeterminateFields, reverted, determinate bool) error {
	keys := []string{"action_name", "disk", "name", "case_sensitive", "create_or_append_synthetic_conf", "create_synthetic_objects", "unmount_volume",
		"create_volume", "create_fstab_entry", "encrypt_volume", "setup_volume_daemon", "bootstrap_volume", "kickstart_launchctl_service", "enable_ownership"}
	if determinate {
		keys = append(keys, "use_ec2_instance_store", "create_directory")
	} else {
		keys = append(keys, "encrypt")
	}
	if err := f.exact(keys...); err != nil {
		return err
	}
	if p.r.Volume != nil {
		return fmt.Errorf("%s repeats the Nix Store volume", f.where)
	}
	disk, err := f.text("disk")
	if err != nil {
		return err
	}
	name, err := f.text("name")
	if err != nil {
		return err
	}
	caseSensitive, err := f.flag("case_sensitive")
	if err != nil {
		return err
	}
	if !legacyDeterminateDisk.MatchString(disk) || name == "" || len(name) > 255 || strings.ContainsAny(name, "\x00\r\n") ||
		name != p.volumeLabel || p.rootDisk != "" && p.rootDisk != disk {
		return fmt.Errorf("%s does not name the planner's APFS volume and disk", f.where)
	}
	volume := &legacyDeterminateVolume{Disk: disk, Name: name, Determinate: determinate}
	p.r.Volume = volume
	encrypted := true
	label, plist := "systems.determinate.nix-store", "/Library/LaunchDaemons/systems.determinate.nix-store.plist"
	if determinate {
		if ec2, err := f.flag("use_ec2_instance_store"); err != nil || ec2 {
			return errors.Join(fmt.Errorf("%s uses an unreviewed EC2 instance-store volume", f.where), err)
		}
		if err := p.child(f, "create_directory", reverted, "create_directory", "config"); err != nil {
			return err
		}
	} else {
		label, plist = "org.nixos.darwin-store", "/Library/LaunchDaemons/org.nixos.darwin-store.plist"
		if encrypted, err = f.flag("encrypt"); err != nil {
			return err
		}
	}
	if err := p.child(f, "create_or_append_synthetic_conf", reverted, "create_or_insert_into_file", "synthetic"); err != nil {
		return err
	}
	// CreateSyntheticObjects is a unit struct; its revert only asks apfs.util
	// to refresh synthetic firmlinks and ignores the result.
	wrapper, err := legacyDeterminateObject(f.values["create_synthetic_objects"], f.where+".create_synthetic_objects")
	if err == nil {
		err = wrapper.exact("action", "state")
	}
	if err == nil {
		err = wrapper.null("action")
	}
	if err == nil {
		_, err = wrapper.is("state", legacyDeterminateStates...)
	}
	if err != nil {
		return err
	}
	children := []struct {
		key, name string
		extra     []string
	}{
		{"unmount_volume", "unmount_apfs_volume", nil},
		{"create_volume", "create_apfs_volume", []string{"case_sensitive"}},
		{"create_fstab_entry", "create_fstab_entry", nil},
		{"encrypt_volume", "encrypt_apfs_volume", []string{"distribution"}},
		{"setup_volume_daemon", map[bool]string{true: "create_determinate_volume_service", false: "create_volume_service"}[determinate], nil},
		{"bootstrap_volume", "bootstrap_launchctl_service", nil},
		{"kickstart_launchctl_service", "kickstart_launchctl_service", nil},
		{"enable_ownership", "enable_ownership", nil},
	}
	for _, item := range children {
		if item.key == "encrypt_volume" && !encrypted {
			if err := f.null(item.key); err != nil {
				return err
			}
			continue
		}
		actual, child, effective, err := legacyDeterminateAction(f.values[item.key], f.where+"."+item.key, reverted)
		if err != nil {
			return err
		}
		if actual != item.name {
			return fmt.Errorf("%s is %q, not the reviewed %s", child.where, truncateLegacyText(actual, 128), item.name)
		}
		if err := legacyDeterminateVolumeChild(child, item.name, volume, label, plist, caseSensitive, encrypted); err != nil {
			return err
		}
		switch item.key {
		case "create_volume":
			volume.Deleted = effective
		case "encrypt_volume":
			volume.Keychain = effective
		case "setup_volume_daemon":
			p.remove(effective, plist)
		case "bootstrap_volume":
			if effective {
				p.r.Services = append(p.r.Services, label)
			}
		}
	}
	return nil
}

func legacyDeterminateVolumeChild(f legacyDeterminateFields, name string, volume *legacyDeterminateVolume, label, plist string, caseSensitive, encrypted bool) error {
	checks := []error{}
	text := func(key string, want ...string) {
		_, err := f.is(key, want...)
		checks = append(checks, err)
	}
	switch name {
	case "unmount_apfs_volume", "create_apfs_volume":
		keys := []string{"action_name", "disk", "name"}
		if name == "create_apfs_volume" {
			keys = append(keys, "case_sensitive")
			value, err := f.flag("case_sensitive")
			checks = append(checks, err)
			if err == nil && value != caseSensitive {
				checks = append(checks, fmt.Errorf("%s differs from its volume", f.where))
			}
		}
		checks = append(checks, f.exact(keys...))
		text("disk", volume.Disk)
		text("name", volume.Name)
	case "create_fstab_entry":
		checks = append(checks, f.exact("action_name", "apfs_volume_label"))
		text("apfs_volume_label", volume.Name)
	case "encrypt_apfs_volume":
		checks = append(checks, f.exact("action_name", "distribution", "disk", "name"))
		text("distribution", map[bool]string{true: "DeterminateNix", false: "Nix"}[volume.Determinate])
		text("disk", volume.Disk)
		text("name", volume.Name)
	case "create_determinate_volume_service":
		checks = append(checks, f.exact("action_name", "path", "mount_service_label", "needs_bootout", "use_ec2_instance_store"))
		text("path", plist)
		text("mount_service_label", label)
		_, err := f.flag("needs_bootout")
		checks = append(checks, err)
		if ec2, err := f.flag("use_ec2_instance_store"); err != nil || ec2 {
			checks = append(checks, fmt.Errorf("%s uses an unreviewed EC2 instance store", f.where), err)
		}
	case "create_volume_service":
		checks = append(checks, f.exact("action_name", "path", "apfs_volume_label", "mount_service_label", "mount_point", "encrypt", "needs_bootout"))
		text("path", plist)
		text("apfs_volume_label", volume.Name)
		text("mount_service_label", label)
		text("mount_point", "/nix")
		_, err := f.flag("needs_bootout")
		checks = append(checks, err)
		if value, err := f.flag("encrypt"); err != nil || value != encrypted {
			checks = append(checks, fmt.Errorf("%s differs from its volume encryption", f.where), err)
		}
	case "bootstrap_launchctl_service":
		checks = append(checks, f.exact("action_name", "service", "path", "is_present", "is_disabled"))
		text("service", label)
		text("path", plist)
		for _, key := range []string{"is_present", "is_disabled"} {
			_, err := f.flag(key)
			checks = append(checks, err)
		}
	case "kickstart_launchctl_service":
		checks = append(checks, f.exact("action_name", "domain", "service"))
		text("domain", "system")
		text("service", label)
	case "enable_ownership":
		checks = append(checks, f.exact("action_name", "path"))
		text("path", "/nix")
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *legacyDeterminateParser) consistent(seen map[string]int) error {
	for _, name := range []string{"provision_nix", "create_users_and_group", "configure_nix"} {
		if seen[name] != 1 {
			return fmt.Errorf("the receipt lacks its %s action", name)
		}
	}
	if seen["configure_determinate_nixd_init_service"]+seen["create_upstream_init_service"] != 1 {
		return errors.New("the receipt does not have exactly one daemon service action")
	}
	if p.r.Planner == "linux" && !slices.Contains(p.roots, "/nix") {
		return errors.New("the Linux receipt lacks its /nix directory action")
	}
	if p.r.Planner == "macos" && (p.r.Volume == nil || p.r.Volume.Determinate != p.r.Determinate) {
		return errors.New("the macOS receipt's volume action does not match its Nix distribution")
	}
	want := map[bool]string{true: "DeterminateNix", false: "Nix"}[p.r.Determinate]
	for _, distribution := range p.distributions {
		if distribution != want {
			return errors.New("the receipt mixes the Determinate Nix and upstream Nix distributions")
		}
	}
	if p.r.Determinate != (seen["provision_determinate_nixd"] == 1) {
		return errors.New("the receipt's determinate-nixd provisioning does not match its daemon service")
	}
	if p.storeGID == nil || *p.storeGID != p.r.GroupID {
		return errors.New("the Nix store group differs from the build group")
	}
	slices.Sort(p.r.Removed)
	p.r.Removed = slices.Compact(p.r.Removed)
	return nil
}

// legacyDeterminateRootEntry names the /nix entries the vendor itself creates:
// the store, its runtime, its receipt and self-copy, scratch space, repair
// backups and the macOS repair hook logs. Split-receipt phase files are not
// included: they mean a partly executed uninstall that must be inspected.
func legacyDeterminateRootEntry(goos, name string) bool {
	switch name {
	case "store", "var", "receipt.json", "nix-installer", "temp-install-dir":
		return true
	case ".nix-installer-hook.err.log", ".nix-installer-hook.out.log", ".fseventsd", ".Spotlight-V100", ".Trashes", ".TemporaryItems", ".DocumentRevisions-V100":
		return goos == "darwin"
	}
	return legacyDeterminateRepair.MatchString(name)
}

// legacyDeterminateObserved is the native identity bound into the approval and
// observed again immediately before the vendor runs; root then re-verifies the
// receipt and binary bytes itself.
type legacyDeterminateObserved struct {
	Receipt        string                 `json:"receipt"`
	Installer      string                 `json:"installer"`
	Binary         string                 `json:"binary"`
	InstallerOwner uint32                 `json:"installer_owner,omitempty"`
	InstallerTrust string                 `json:"installer_trust,omitempty"`
	Nixd           string                 `json:"nixd,omitempty"`
	Accounts       []string               `json:"accounts,omitempty"`
	Group          string                 `json:"group,omitempty"`
	Volume         string                 `json:"volume,omitempty"`
	VolumeUUID     string                 `json:"volume_uuid,omitempty"`
	Keychain       bool                   `json:"keychain,omitempty"`
	Absent         []string               `json:"absent,omitempty"`
	Kept           []string               `json:"kept,omitempty"`
	Files          []legacyRetirementFile `json:"files,omitempty"`
}

func legacyDeterminateAccount(goos, record string, user legacyDeterminateUser) bool {
	if goos != "darwin" {
		return record == fmt.Sprintf("%s:x:%d:%d:%s:/var/empty:/sbin/nologin", user.Name, user.UID, user.GID, user.Comment)
	}
	// Vendor accounts use their name as RealName and are hidden unless the
	// hide step was killed; adopted upstream accounts keep "Nix build user N".
	fields := strings.Split(record, ":")
	return len(fields) == 7 && fields[0] == user.Name && fields[1] == strconv.FormatUint(uint64(user.UID), 10) &&
		fields[2] == strconv.FormatUint(uint64(user.GID), 10) && (fields[3] == user.Name || fields[3] == user.Comment) &&
		fields[4] == "/var/empty" && fields[5] == "/sbin/nologin" && (fields[6] == "1" || fields[6] == "")
}

// Consumers are accounts outside the receipt whose primary group is the build
// group the vendor deletes.
func (d *legacyRetirementDriver) determinateGroupConsumers(ctx context.Context, r legacyDeterminateReceipt) (string, error) {
	gid := strconv.FormatUint(uint64(r.GroupID), 10)
	primary := map[string]string{}
	if d.Platform.OS == "darwin" {
		var err error
		if primary, err = d.dsclList(ctx, "Users", "PrimaryGroupID"); err != nil {
			return "", err
		}
	} else {
		output, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/getent", "passwd"))
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
			if fields := strings.Split(line, ":"); len(fields) == 7 {
				primary[fields[0]] = fields[3]
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(primary)) {
		if primary[name] == gid && !slices.ContainsFunc(r.Users, func(user legacyDeterminateUser) bool { return user.Name == name }) {
			return "account " + name + " uses the Nix build group as its primary group; it is another consumer", nil
		}
	}
	return "", nil
}

func (d *legacyRetirementDriver) determinateDiskInfo(ctx context.Context, target string) (map[string]legacyPlistNode, error) {
	output, err := d.Query(ctx, d.systemCommand(false, "/usr/sbin/diskutil", "info", "-plist", target))
	if err != nil {
		return nil, err
	}
	return legacyPlist(output)
}

func legacyPlistWalk(node legacyPlistNode, visit func(map[string]legacyPlistNode)) {
	if node.XMLName.Local == "dict" {
		values := map[string]legacyPlistNode{}
		for i := 0; i+1 < len(node.Nodes); i += 2 {
			if node.Nodes[i].XMLName.Local == "key" {
				values[strings.TrimSpace(node.Nodes[i].Text)] = node.Nodes[i+1]
			}
		}
		visit(values)
	}
	for _, child := range node.Nodes {
		legacyPlistWalk(child, visit)
	}
}

// determinateVolume binds the receipt's volume name and disk to the APFS
// volume mounted at /nix. The vendor unmounts and deletes by name, so that
// name must resolve to this volume and to no other volume on any disk.
func (d *legacyRetirementDriver) determinateVolume(ctx context.Context, v legacyDeterminateVolume) (identity, uuid, problem string, err error) {
	mounted, err := d.determinateDiskInfo(ctx, "/nix")
	if err != nil {
		return "", "", "", err
	}
	uuid = legacyPlistString(mounted, "VolumeUUID")
	parent := legacyPlistString(mounted, "ParentWholeDisk")
	if !legacyVolumeUUID.MatchString(uuid) || legacyPlistString(mounted, "MountPoint") != "/nix" || legacyPlistString(mounted, "VolumeName") != v.Name ||
		legacyPlistString(mounted, "FilesystemType") != "apfs" {
		return "", "", "/nix is not the receipt's mounted APFS volume " + strconv.Quote(v.Name), nil
	}
	if parent != v.Disk {
		return "", "", "the /nix volume belongs to " + parent + ", not the receipt's " + v.Disk + "; the vendor locates its keychain password by the recorded disk", nil
	}
	named, err := d.determinateDiskInfo(ctx, v.Name)
	if err != nil {
		return "", "", "", err
	}
	if legacyPlistString(named, "VolumeUUID") != uuid {
		return "", "", "the volume name " + strconv.Quote(v.Name) + " resolves to a different volume than /nix; the vendor deletes by name", nil
	}
	output, err := d.Query(ctx, d.systemCommand(false, "/usr/sbin/diskutil", "list", "-plist"))
	if err != nil {
		return "", "", "", err
	}
	var document struct {
		XMLName xml.Name        `xml:"plist"`
		Dict    legacyPlistNode `xml:"dict"`
	}
	if err := xml.Unmarshal(output, &document); err != nil {
		return "", "", "", err
	}
	devices := []string{}
	legacyPlistWalk(document.Dict, func(values map[string]legacyPlistNode) {
		if strings.EqualFold(legacyPlistString(values, "VolumeName"), v.Name) {
			devices = append(devices, legacyPlistString(values, "DeviceIdentifier"))
		}
	})
	if len(devices) != 1 || devices[0] != legacyPlistString(mounted, "DeviceIdentifier") {
		return "", "", fmt.Sprintf("%d volumes are named %q; the vendor deletes by name, so only the /nix volume may carry it", len(devices), v.Name), nil
	}
	identity = "VolumeName=" + v.Name + ";VolumeUUID=" + uuid + ";FilesystemType=apfs;ParentWholeDisk=" + parent + ";MountPoint=/nix;Device=" + devices[0]
	return identity, uuid, "", nil
}

// security exits 44 when no item matches. The vendor deletes exactly this
// account, service and label; without it the revert fails after deletion.
func (d *legacyRetirementDriver) determinateKeychain(ctx context.Context, v legacyDeterminateVolume) (bool, error) {
	_, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/security", "find-generic-password", "-a", v.Name, "-s", "Nix Store", "-l", v.Disk+" encryption password", legacyDeterminateKeychain))
	if code, ok := legacyExitCode(err); err != nil && ok && code == 44 {
		return false, nil
	}
	return err == nil, err
}

// determinateInsertProblem reports why the vendor's text removal could
// edit something other than its own block. The vendor follows links and
// removes the last occurrence of its text, so a link or a match inside another
// line is refused. A nix-darwin /etc link is acceptable only in preview: the
// same plan restores the displaced file first, and dispatch requires a file.
func (d *legacyRetirementDriver) determinateInsertProblem(ctx context.Context, path, buf, nixDarwinLink string) string {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return path + " could not be inspected: " + err.Error()
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil && nixDarwinLink != "" && target == nixDarwinLink {
			return ""
		}
		return path + " is a link; the vendor uninstall would edit its destination"
	}
	if !info.Mode().IsRegular() {
		return path + " is not a regular file"
	}
	data, _, err := d.readSystemText(ctx, path)
	if err != nil {
		return legacyReason(path+" could not be read before the vendor edit", err)
	}
	if index := bytes.LastIndex(data, []byte(buf)); index > 0 && !strings.HasPrefix(buf, "\n") && data[index-1] != '\n' {
		return "the vendor uninstall would remove " + strconv.Quote(buf) + " from inside another line of " + path + "; review it manually"
	}
	return ""
}

// observeDeterminate checks every identity the receipt names. Accounts and
// files must be absent or exactly the vendor's; the problem text is a
// resource-local refusal. Preview passes nixDarwin while nix-darwin /etc
// links are retired earlier in the same plan; gates never do.
func (d *legacyRetirementDriver) observeDeterminate(ctx context.Context, r legacyDeterminateReceipt, data []byte, nixDarwin bool) (legacyDeterminateObserved, string, error) {
	o := legacyDeterminateObserved{Receipt: legacyHash(data)}
	links, err := d.determinateUnitFiles(r)
	if err != nil {
		return o, err.Error(), nil
	}
	for _, path := range r.Removed {
		if strings.HasPrefix(path, "/etc/nix/") {
			continue // The separate configuration backup preserves these files.
		}
		file, err := legacyFile(path)
		if err != nil {
			return o, "", err
		}
		o.Files = append(o.Files, file)
	}
	o.Files = append(o.Files, links...)
	entries, err := readPlainDirectory("/nix")
	if err != nil {
		return o, "", err
	}
	for _, entry := range entries {
		if !legacyDeterminateRootEntry(d.Platform.OS, entry.Name()) {
			return o, "unrecognized data in the Nix installation: " + filepath.Join("/nix", entry.Name()) + "; the vendor uninstall deletes all of /nix, so preserve and review it", nil
		}
	}
	var pin *legacyDeterminateRelease
	if d.Platform.OS == "darwin" && d.Platform.Arch == "arm64" {
		if release, ok := legacyDeterminateMacReleases[r.Version]; ok {
			pin = &release
		}
	}
	proof, problem, err := legacyDeterminateBinaryTrust(legacyDeterminateInstallerPath, "/nix", r.Version, pin)
	if err != nil {
		return o, "", err
	}
	if problem != "" {
		return o, "the installed vendor uninstaller cannot be trusted: " + problem, nil
	}
	o.Binary, o.Installer = proof.Identity, proof.SHA256
	// Preserve the old root-owned gate identity exactly. A release-authenticated
	// writable source adds its route and owner, so it cannot reuse that approval.
	if proof.Trust != "root-owned" {
		o.InstallerOwner, o.InstallerTrust = proof.Owner, proof.Trust
	}
	if r.Nixd {
		info, err := os.Lstat(legacyDeterminateNixdPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			o.Absent = append(o.Absent, legacyDeterminateNixdPath)
		case err != nil:
			return o, "", err
		default:
			uid, _, mode, err := legacyFileOwner(legacyDeterminateNixdPath)
			if err != nil {
				return o, "", err
			}
			if !info.Mode().IsRegular() || uid != 0 || mode&0o022 != 0 {
				return o, legacyDeterminateNixdPath + " is not the root-owned file the vendor provisioned", nil
			}
			if o.Nixd, err = legacyFileSHA256(legacyDeterminateNixdPath); err != nil {
				return o, "", err
			}
		}
	}
	for _, user := range r.Users {
		record, err := d.accountRecord(ctx, user.Name)
		if err != nil {
			return o, "", err
		}
		switch {
		case record == "absent" && user.Reverted:
			o.Absent = append(o.Absent, "account "+user.Name)
		case record == "absent":
		case !user.Reverted:
			return o, "account " + user.Name + " exists but its receipt action will not remove it; inspect it natively", nil
		case !legacyDeterminateAccount(d.Platform.OS, record, user):
			return o, "account " + user.Name + " does not have the exact vendor build-account shape (" + truncateLegacyText(record, 256) + "); the vendor deletes accounts by name", nil
		default:
			o.Accounts = append(o.Accounts, record)
		}
	}
	if record, err := d.groupRecord(ctx, r.Group); err != nil {
		return o, "", err
	} else if record != "absent" && !r.GroupRevert {
		o.Kept = append(o.Kept, "group "+r.Group)
	} else if record != "absent" {
		if record != fmt.Sprintf("%s:%d:", r.Group, r.GroupID) {
			return o, "the " + r.Group + " group is not the receipt's build group or has other members (" + truncateLegacyText(record, 256) + ")", nil
		}
		o.Group = record
		if consumer, err := d.determinateGroupConsumers(ctx, r); err != nil || consumer != "" {
			return o, consumer, err
		}
	} else if r.GroupRevert {
		o.Absent = append(o.Absent, "group "+r.Group)
	}
	if v := r.Volume; v != nil && v.Deleted {
		if o.Volume, o.VolumeUUID, problem, err = d.determinateVolume(ctx, *v); err != nil || problem != "" {
			return o, problem, err
		}
		if v.Keychain {
			if o.Keychain, err = d.determinateKeychain(ctx, *v); err != nil {
				return o, "", err
			}
			if !o.Keychain {
				return o, "the System keychain lacks the receipt's " + strconv.Quote(v.Disk+" encryption password") + " item; the vendor uninstall would fail after deleting the volume", nil
			}
		}
	}
	for _, insert := range r.Inserts {
		if !insert.Reverted {
			continue
		}
		link := ""
		if relative, ok := strings.CutPrefix(insert.Path, "/etc/"); nixDarwin && ok {
			link = "/etc/static/" + relative
		}
		if problem := d.determinateInsertProblem(ctx, insert.Path, insert.Buf, link); problem != "" {
			return o, problem, nil
		}
	}
	for _, path := range r.Prune {
		if info, err := os.Lstat(path); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return o, path + " is not a plain directory; the vendor would prune it recursively", nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return o, "", err
		}
	}
	return o, "", nil
}

// determinateGateValue is the "determinate" gate: the digest of the current
// receipt's native identity under dispatch rules, "absent" once the receipt
// is gone, or the refusal that now applies.
func (d *legacyRetirementDriver) determinateGateValue(ctx context.Context) (string, error) {
	data, _, err := readLegacyFile(legacyDeterminateReceiptPath, legacyDeterminateReceiptLimit)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "absent", nil
	}
	r, err := parseLegacyDeterminateReceipt(data)
	if err != nil {
		return "refused: " + err.Error(), nil
	}
	observed, problem, err := d.observeDeterminate(ctx, r, data, false)
	if err != nil {
		return "", err
	}
	if problem != "" {
		return "refused: " + problem, nil
	}
	return digest(observed)
}

// determinateDaemonProcess returns the process ID of the running vendor
// daemon only when the init system reports it and the kernel shows that
// process executing the provisioned determinate-nixd. Every other process,
// including its children, stays a consumer. "" excludes nothing.
func (d *legacyRetirementDriver) determinateDaemonProcess(ctx context.Context) (string, error) {
	pid := ""
	switch d.Platform.OS {
	case "linux":
		systemctl, err := legacySystemProgram("/usr/bin/systemctl", "/bin/systemctl")
		if err != nil {
			return "", err
		}
		output, err := d.Query(ctx, d.systemCommand(false, systemctl, "show", "--property=MainPID", "--value", "nix-daemon.service"))
		if err != nil {
			return "", err
		}
		pid = strings.TrimSpace(string(output))
	case "darwin":
		service := "system/systems.determinate.nix-daemon"
		if state, err := d.launchdState(ctx, service); err != nil || state != "loaded" {
			return "", err
		}
		output, err := d.Query(ctx, d.systemCommand(false, "/bin/launchctl", "print", service))
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(output), "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "pid = "); ok {
				if pid != "" {
					return "", errors.New("the Determinate launchd service reports more than one process")
				}
				pid = value
			}
		}
	default:
		return "", nil
	}
	if value, err := strconv.ParseUint(pid, 10, 32); pid == "" || pid == "0" {
		return "", nil
	} else if err != nil || value == 0 {
		return "", errors.New("the Determinate daemon has an invalid process identity")
	}
	if d.Platform.OS == "linux" {
		readlink, err := legacySystemProgram("/usr/bin/readlink", "/bin/readlink")
		if err != nil {
			return "", err
		}
		output, err := d.privilegedQuery(ctx, d.systemCommand(true, readlink, "--", "/proc/"+pid+"/exe"))
		if err != nil || strings.TrimSuffix(string(output), "\n") != legacyDeterminateNixdPath {
			return "", nil // An exited or different process is not excluded.
		}
		return pid, nil
	}
	// lsof lists a process's program file first among its txt mappings.
	output, err := d.privilegedQuery(ctx, d.systemCommand(true, "/usr/sbin/lsof", "-n", "-a", "-p", pid, "-d", "txt", "-F", "fn"))
	if err != nil {
		return "", nil
	}
	for _, line := range strings.Split(string(output), "\n") {
		if name, ok := strings.CutPrefix(line, "n"); ok {
			if name == legacyDeterminateNixdPath {
				return pid, nil
			}
			return "", nil
		}
	}
	return "", nil
}

// Root re-verifies the approved receipt and binary bytes immediately before
// it execs the reviewed vendor invocation from the vendor's own path, where
// its documented copy-and-reexec protection applies. Exit codes 120-123 mean
// the vendor was not started. Telemetry is requested off for this run only:
// the empty endpoint is the vendor's documented opt-out since v3.0.0, and its
// telemetry client honors DETSYS_IDS_TELEMETRY=disabled since nix-installer
// v3.8.6 (detsys-ids-client 0.5.3).
const legacyDeterminateUninstallScript = `set -u
installer=$1 installer_sum=$2 receipt=$3 receipt_sum=$4
file_hash() {
case $(uname -s) in
Darwin) shasum -a 256 "$1" ;;
*) sha256sum "$1" ;;
esac
}
for path in "$installer" "$receipt"; do
	if [ ! -f "$path" ] || [ -L "$path" ]; then exit 120; fi
done
sum=$(file_hash "$installer") || exit 121
if [ "${sum%% *}" != "$installer_sum" ]; then exit 122; fi
sum=$(file_hash "$receipt") || exit 121
if [ "${sum%% *}" != "$receipt_sum" ]; then exit 123; fi
cd / || exit 121
exec "$installer" uninstall --no-confirm "$receipt"
`

// Saved pre-staging commands retain the literal script above. New commands
// copy and verify both inputs in a private directory before exec, keeping the
// native worker's process lifetime attached to the actual vendor uninstaller.
// The optional parent is a process-test boundary; production always uses /tmp.
const legacyDeterminateStagedUninstallScript = `set -u
installer=$1 installer_sum=$2 receipt=$3 receipt_sum=$4 parent=${5:-/tmp}
umask 077
file_hash() {
case $(uname -s) in
Darwin) shasum -a 256 "$1" ;;
*) sha256sum "$1" ;;
esac
}
for path in "$installer" "$receipt"; do
	if [ ! -f "$path" ] || [ -L "$path" ]; then exit 120; fi
done
if [ ! -d "$parent" ] || [ ! -k "$parent" ] || [ ! -O "$parent" ]; then exit 121; fi
stage=$(mktemp -d "$parent/dotfiles-determinate.XXXXXXXXXX") || exit 121
if [ ! -d "$stage" ] || [ -L "$stage" ] || [ ! -O "$stage" ]; then exit 121; fi
refuse() {
	code=$1
	rm -f -- "$stage/nix-installer" "$stage/receipt.json" || exit 121
	rmdir -- "$stage" || exit 121
	exit "$code"
}
head -c 536870913 -- "$installer" > "$stage/nix-installer" || refuse 121
head -c 524289 -- "$receipt" > "$stage/receipt.json" || refuse 121
chmod 0700 "$stage/nix-installer" || refuse 121
sum=$(file_hash "$stage/nix-installer") || refuse 121
if [ "${sum%% *}" != "$installer_sum" ]; then refuse 122; fi
sum=$(file_hash "$stage/receipt.json") || refuse 121
if [ "${sum%% *}" != "$receipt_sum" ]; then refuse 123; fi
cd / || refuse 121
exec "$stage/nix-installer" uninstall --no-confirm "$stage/receipt.json"
`

func legacyDeterminateUninstallCommand(installer, installerSum, receipt, receiptSum string) nativeCommand {
	return nativeCommand{Program: "/usr/bin/env", Arguments: []string{"-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C",
		"DETSYS_IDS_TELEMETRY=disabled", "NIX_INSTALLER_DIAGNOSTIC_ENDPOINT=", "/bin/sh", "-c", legacyDeterminateStagedUninstallScript, "sh",
		installer, installerSum, receipt, receiptSum}}
}

// The evidence file is written once with the exact approved bytes; an
// existing different file is preserved and refused.
const legacyDeterminateEvidenceScript = `set -u
target=$1 expected=$2
umask 077
file_hash() {
case $(uname -s) in
Darwin) shasum -a 256 "$1" ;;
*) sha256sum "$1" ;;
esac
}
directory=${target%/*}
mkdir -p "$directory" || exit 90
if [ -e "$target" ] || [ -L "$target" ]; then
	if [ ! -f "$target" ] || [ -L "$target" ]; then exit 91; fi
	sum=$(file_hash "$target") || exit 91
	if [ "${sum%% *}" = "$expected" ]; then exit 0; fi
	exit 91
fi
temporary=$(mktemp "$directory/.evidence.XXXXXX") || exit 92
trap 'rm -f "$temporary"' EXIT
cat > "$temporary" || exit 93
sum=$(file_hash "$temporary") || exit 93
if [ "${sum%% *}" != "$expected" ]; then exit 94; fi
ln "$temporary" "$target" || exit 95
`

// legacyDeterminateEvidence preserves the approved receipt and native
// identities outside /nix, which the vendor deletes. Recovery after a failed
// or unknown vendor run plans only from this approved evidence.
type legacyDeterminateEvidence struct {
	Schema     int                    `json:"schema"`
	Receipt    []byte                 `json:"receipt"`
	Installer  string                 `json:"installer"`
	Nixd       string                 `json:"nixd,omitempty"`
	Accounts   []string               `json:"accounts,omitempty"`
	Group      string                 `json:"group,omitempty"`
	Volume     string                 `json:"volume,omitempty"`
	VolumeUUID string                 `json:"volume_uuid,omitempty"`
	Files      []legacyRetirementFile `json:"files,omitempty"`
}

func (d *legacyRetirementDriver) determinateEvidenceDirectory() string {
	return filepath.Join(d.Directory, "determinate")
}

func (d *legacyRetirementDriver) determinateEvidenceStep(evidence legacyDeterminateEvidence) (legacyRetirementStep, error) {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return legacyRetirementStep{}, err
	}
	sum := legacyHash(encoded)
	path := filepath.Join(d.determinateEvidenceDirectory(), sum+".json")
	after, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: sum, Mode: 0o600}})
	if err != nil {
		return legacyRetirementStep{}, err
	}
	command := d.systemCommand(false, "/bin/sh", "-c", legacyDeterminateEvidenceScript, "sh", path, sum)
	command.Input = encoded
	return legacyRetirementStep{Kind: "command", Label: "Record the approved Determinate receipt and identities at " + path, Command: &command,
		After: []legacyRetirementFile{{Path: path, Snapshot: after}}}, nil
}

// determinateEvidence loads every intact evidence file. Each name is the
// SHA-256 of its bytes and its receipt must still validate.
func (d *legacyRetirementDriver) determinateEvidence() ([]legacyDeterminateEvidence, error) {
	directory := d.determinateEvidenceDirectory()
	entries, err := readPlainDirectory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > 64 {
		return nil, errors.New("Determinate uninstall evidence exceeds its review bound")
	}
	result := []legacyDeterminateEvidence{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".evidence.") {
			continue // An interrupted writer's private temporary file.
		}
		path := filepath.Join(directory, entry.Name())
		data, _, err := readLegacyFile(path, 8<<20)
		if err != nil || data == nil || !legacyDeterminateEvidenceName.MatchString(entry.Name()) || legacyHash(data)+".json" != entry.Name() {
			return nil, errors.Join(errors.New("Determinate uninstall evidence "+path+" is not intact"), err)
		}
		var evidence legacyDeterminateEvidence
		if err := Decode(data, &evidence); err != nil || evidence.Schema != 1 {
			return nil, errors.Join(errors.New("Determinate uninstall evidence "+path+" has an unknown shape"), err)
		}
		if _, err := parseLegacyDeterminateReceipt(evidence.Receipt); err != nil {
			return nil, fmt.Errorf("Determinate uninstall evidence %s no longer validates: %w", path, err)
		}
		result = append(result, evidence)
	}
	return result, nil
}

func (d *legacyRetirementDriver) determinateAccountStep(label, name, record string, uid uint32) (legacyRetirementStep, error) {
	command := d.systemCommand(true, "/usr/bin/dscl", ".", "-delete", "/Users/"+name)
	if d.Platform.OS != "darwin" {
		userdel, err := legacySystemProgram("/usr/sbin/userdel")
		if err != nil {
			return legacyRetirementStep{}, err
		}
		command = d.systemCommand(true, userdel, name)
	}
	return legacyRetirementStep{Kind: "command", Label: label + " " + name, Privileged: true, Command: &command,
		Gates:  []legacyRetirementGate{{Kind: "account", Subject: name, Expected: record}, {Kind: "account-processes", Subject: strconv.FormatUint(uint64(uid), 10)}},
		Checks: []legacyRetirementGate{{Kind: "account", Subject: name, Expected: "absent"}}}, nil
}

func (d *legacyRetirementDriver) determinateGroupStep(label, name, record string) (legacyRetirementStep, error) {
	command := d.systemCommand(true, "/usr/bin/dscl", ".", "-delete", "/Groups/"+name)
	if d.Platform.OS != "darwin" {
		groupdel, err := legacySystemProgram("/usr/sbin/groupdel")
		if err != nil {
			return legacyRetirementStep{}, err
		}
		command = d.systemCommand(true, groupdel, name)
	}
	return legacyRetirementStep{Kind: "command", Label: label + " " + name, Privileged: true, Command: &command,
		Gates:  []legacyRetirementGate{{Kind: "group", Subject: name, Expected: record}},
		Checks: []legacyRetirementGate{{Kind: "group", Subject: name, Expected: "absent"}}}, nil
}

// planDeterminateRetirement replaces the earlier path-only planDeterminate.
// The vendor uninstall is one irreversible step bound to the approved receipt,
// binary and native identities; preservation precedes it and exact follow-up
// steps verify what macOS can leave behind.
func (d *legacyRetirementDriver) planDeterminateRetirement(ctx context.Context, scope *legacyRetirementScope) error {
	data, receiptSnapshot, err := readLegacyFile(legacyDeterminateReceiptPath, legacyDeterminateReceiptLimit)
	if err != nil || data == nil {
		scope.Blocked = legacyReason("the Determinate receipt is unreadable; preserve it for native inspection", err)
		return nil
	}
	r, err := parseLegacyDeterminateReceipt(data)
	if err != nil {
		scope.Blocked = "the Determinate receipt is not a reviewed vendor uninstall plan; preserve /nix/receipt.json and uninstall natively: " + truncateLegacyText(err.Error(), 1024)
		return nil
	}
	if want := map[string]string{"linux": "linux", "darwin": "macos"}[d.Platform.OS]; r.Planner != want {
		scope.Blocked = "the Determinate receipt was planned for " + r.Planner + ", not this " + d.Platform.OS + " host"
		return nil
	}
	nixDarwin := false
	if d.Platform.OS == "darwin" {
		system, err := d.nixDarwinSystem()
		if err != nil {
			return err
		}
		nixDarwin = system != ""
	}
	observed, problem, err := d.observeDeterminate(ctx, r, data, nixDarwin)
	if err != nil {
		return err
	}
	if problem != "" {
		scope.Blocked = problem
		return nil
	}
	// A release pin already identifies the exact version. Never execute a
	// user-writable source during preview: it can change after hashing. The
	// protected root-owned route retains its version probe.
	if observed.InstallerTrust == "" {
		output, err := d.Query(ctx, d.systemCommand(false, legacyDeterminateInstallerPath, "--version"))
		if err != nil || strings.TrimSuffix(string(output), "\n") != "nix-installer "+r.Version {
			scope.Blocked = legacyReason("the installed /nix/nix-installer does not report its receipt version "+r.Version+" ("+truncateLegacyText(strings.TrimSpace(string(output)), 128)+")", err)
			return nil
		}
	}
	// The vendor's own pre-uninstall checks, observed without running it.
	if r.Init == "Systemd" && r.StartDaemon {
		if info, err := os.Lstat("/run/systemd/system"); err != nil || !info.IsDir() {
			scope.Blocked = "systemd is not running; the vendor uninstaller requires it for this receipt"
			return nil
		}
	}
	if d.Platform.OS == "darwin" {
		state, err := d.launchdState(ctx, "system/org.nixos.activate-system")
		if err != nil {
			return err
		}
		if state == "loaded" && !nixDarwin {
			scope.Blocked = "nix-darwin's activation service is loaded without an active generation; the vendor uninstaller refuses while it runs"
			return nil
		}
	}
	identity, err := digest(observed)
	if err != nil {
		return err
	}
	distribution := map[bool]string{true: "Determinate Nix", false: "upstream Nix"}[r.Determinate]
	scope.Native = strings.TrimSpace("Determinate nix-installer " + r.Version + " receipt (" + r.Planner + " planner, " + distribution + "); " + scope.Native)
	scope.Detail = append(scope.Detail,
		"This Nix was installed by Determinate Systems' nix-installer "+r.Version+", not by the earlier dotfiles setup, which reused it. Retiring it removes Nix for every account on this machine.")
	trust := "its protected root ownership and matching reported version"
	if observed.InstallerTrust != "" {
		trust = "the reviewed official " + strings.TrimPrefix(observed.InstallerTrust, "release:") + " release digest; preview never executes the user-writable source"
	}
	scope.Detail = append(scope.Detail, fmt.Sprintf("The installed uninstaller has owner uid %d and SHA-256 %s. Authentication uses %s. Root copies and verifies the binary and approved receipt before running either.", observed.InstallerOwner, observed.Installer, trust))
	scope.Detail = append(scope.Detail, "Vendor telemetry is requested off for the uninstall through its documented empty NIX_INSTALLER_DIAGNOSTIC_ENDPOINT and DETSYS_IDS_TELEMETRY=disabled.")
	for _, item := range observed.Absent {
		scope.Detail = append(scope.Detail, "Already absent although the receipt records it: "+item+".")
	}
	if slices.ContainsFunc(observed.Absent, func(item string) bool {
		return strings.HasPrefix(item, "group ") || d.Platform.OS == "linux" && strings.HasPrefix(item, "account ")
	}) {
		scope.Detail = append(scope.Detail, "The vendor uninstaller reports a failure for an already-absent account or group after it reverts everything else; resuming first verifies the approved outcome and runs nothing when it holds.")
	}
	for _, item := range observed.Kept {
		scope.Detail = append(scope.Detail, "The receipt does not remove the pre-existing "+item+"; it stays.")
	}
	edited := []string{}
	for _, insert := range r.Inserts {
		if insert.Reverted {
			edited = append(edited, insert.Path)
		}
	}
	if len(edited) != 0 {
		scope.Detail = append(scope.Detail, "The vendor removes only its exact Nix block from "+strings.Join(edited, ", ")+"; other lines stay.")
	}
	if slices.Contains(r.Prune, "/etc/nix") {
		scope.Detail = append(scope.Detail, "The receipt was installed with --force: the vendor deletes all of /etc/nix.")
	} else if slices.Contains(r.Removed, "/etc/nix/nix.conf") || slices.Contains(r.Removed, "/etc/nix/nix.custom.conf") {
		scope.Detail = append(scope.Detail, "The vendor deletes /etc/nix/nix.conf and /etc/nix/nix.custom.conf; other /etc/nix files stay.")
	}
	for _, item := range []struct{ path, text string }{
		{"/nix/var/determinate", "Determinate Nixd state at /nix/var/determinate, including any FlakeHub login, is deleted with the store."},
		{"/etc/determinate", "Determinate Nixd configuration at /etc/determinate stays unchanged."},
	} {
		if _, err := os.Lstat(item.path); err == nil {
			scope.Detail = append(scope.Detail, item.text)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if v := r.Volume; v != nil && v.Deleted {
		text := "The vendor deletes the APFS volume " + strconv.Quote(v.Name) + " (" + observed.VolumeUUID + ") on " + v.Disk + " by name; it is the only volume with that name and is mounted at /nix."
		if v.Keychain {
			text += " Its System keychain password is deleted too."
		}
		scope.Detail = append(scope.Detail, text, "An empty /nix folder remains until the next restart. Determinate logs under /var/log stay.")
	}
	scope.Detail = append(scope.Detail, "A private administrator-owned directory /tmp/dotfiles-determinate.* retains the verified uninstaller and receipt after execution.")

	evidence, err := d.determinateEvidenceStep(legacyDeterminateEvidence{Schema: 1, Receipt: data, Installer: observed.Installer, Nixd: observed.Nixd,
		Accounts: observed.Accounts, Group: observed.Group, Volume: observed.Volume, VolumeUUID: observed.VolumeUUID, Files: observed.Files})
	if err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, evidence)
	if err := d.planEtcNixCopy(scope, nixDarwin); err != nil || scope.Blocked != "" {
		return err
	}
	if slices.Contains(r.Removed, "/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist") {
		// The hook runs `/nix/nix-installer repair` and its revert only deletes
		// the plist; stop a loaded hook before the receipt changes under it.
		service := "system/systems.determinate.nix-installer.nix-hook"
		state, err := d.launchdState(ctx, service)
		if err != nil {
			return err
		}
		if state == "loaded" {
			command := d.systemCommand(true, "/bin/launchctl", "bootout", service)
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Stop the Determinate repair hook " + service, Privileged: true, Command: &command,
				Gates:  []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "loaded"}},
				Checks: []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "absent"}}})
		}
	}
	consumers := "daemon"
	if r.Determinate {
		consumers = "determinate"
	}
	if r.Init == "None" {
		// This vendor planner installs no service. Exclude no process, and
		// do not query an init system that the host need not have.
		consumers = ""
	}
	gates, err := d.readyGates(ctx, consumers)
	if err != nil {
		return err
	}
	gates = append(gates, legacyRetirementGate{Kind: "determinate", Expected: identity})
	uids := []string{}
	for _, user := range r.Users {
		if user.Reverted {
			uids = append(uids, strconv.FormatUint(uint64(user.UID), 10))
		}
	}
	if len(uids) != 0 {
		gates = append(gates, legacyRetirementGate{Kind: "account-processes", Subject: strings.Join(uids, ",")})
	}
	if d.Platform.OS == "darwin" {
		gates = append(gates, legacyRetirementGate{Kind: "launchd", Subject: "system/org.nixos.activate-system", Expected: "absent"})
	}
	if r.Init != "None" {
		if err := d.planDeterminateDaemonStop(ctx, scope, r, gates); err != nil {
			return err
		}
		// The receipt's daemon is now stopped. No process is exempt from the
		// final kernel scan, including a surviving daemon child or reporter.
		for i := range gates {
			if gates[i].Kind == "nix-consumers" {
				gates[i].Subject = ""
			}
		}
	}
	absent := append([]string{legacyDeterminateReceiptPath, legacyDeterminateInstallerPath, "/nix/store"}, r.Removed...)
	if r.Root {
		absent = append(absent, "/nix")
	}
	if slices.Contains(r.Prune, "/etc/nix") {
		absent = append(absent, "/etc/nix")
	}
	checks := []legacyRetirementGate{}
	if observed.VolumeUUID != "" {
		checks = append(checks, legacyRetirementGate{Kind: "volume", Subject: observed.VolumeUUID, Expected: "absent"})
	}
	for _, label := range r.Services {
		checks = append(checks, legacyRetirementGate{Kind: "launchd", Subject: "system/" + label, Expected: "absent"})
	}
	installer, err := legacyFile(legacyDeterminateInstallerPath)
	if err != nil {
		return err
	}
	command := legacyDeterminateUninstallCommand(legacyDeterminateInstallerPath, observed.Installer, legacyDeterminateReceiptPath, observed.Receipt)
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Run the installed Determinate uninstaller " + r.Version + " on its validated receipt and delete the Nix store",
		Privileged: true, Irreversible: true, Command: &command, Before: []legacyRetirementFile{{Path: legacyDeterminateReceiptPath, Snapshot: receiptSnapshot}, installer},
		Gates: gates, Absent: sortedUnique(absent), Checks: checks})
	firstUnitLink := len(scope.Steps)
	for _, path := range legacyDeterminateUnitLinks(r) {
		if err := addLegacyRemoval(scope, "Remove the retired Nix unit activation link", d.sys(path)); err != nil {
			return err
		}
	}
	reloadUnits := len(scope.Steps) != firstUnitLink
	// macOS can report a successful account deletion it skipped without a
	// graphical login (dscl -14120). Exact follow-ups prove every account.
	for _, record := range observed.Accounts {
		name := strings.SplitN(record, ":", 2)[0]
		user := r.Users[slices.IndexFunc(r.Users, func(user legacyDeterminateUser) bool { return user.Name == name })]
		step, err := d.determinateAccountStep("Delete the Nix build account if the vendor left it:", name, record, user.UID)
		if err != nil {
			return err
		}
		scope.Steps = append(scope.Steps, step)
	}
	if observed.Group != "" {
		step, err := d.determinateGroupStep("Delete the Nix build group if the vendor left it:", r.Group, observed.Group)
		if err != nil {
			return err
		}
		scope.Steps = append(scope.Steps, step)
	}
	if err := d.planDeterminateDropIns(scope, r.Units, reloadUnits); err != nil {
		return err
	}
	home := "/var/root"
	if d.Platform.OS != "darwin" {
		root, err := d.accountRecord(ctx, "root")
		if err != nil {
			return err
		}
		fields := strings.Split(root, ":")
		if len(fields) != 7 || !legacyCanonicalPath(fields[5]) {
			return errors.New("root's home directory could not be identified")
		}
		home = fields[5]
	}
	return d.addRootFiles(scope, home)
}

// Determinate supervises Nix and a crash reporter as separate processes. Stop
// its receipt-owned service before the irreversible consumer scan, rather than
// exempting its descendants. Static outside consumers and active build accounts
// still block this reversible stop; any other live consumer blocks uninstall.
func (d *legacyRetirementDriver) planDeterminateDaemonStop(ctx context.Context, scope *legacyRetirementScope, r legacyDeterminateReceipt, gates []legacyRetirementGate) error {
	first := len(scope.Steps)
	before := slices.Clone(gates)
	for i := range before {
		if before[i].Kind == "nix-consumers" {
			before[i].Kind, before[i].Subject = "nix-consumer-roots", ""
		}
	}
	if r.Init == "Systemd" && len(r.Units) != 0 {
		stop, err := d.systemdStopStep("Stop the receipt's Nix sockets and daemon before checking remaining consumers", r.Units)
		if err != nil {
			return err
		}
		stop.Gates = before
		scope.Steps = append(scope.Steps, stop)
	}
	if r.Init == "Launchd" {
		for _, label := range r.Services {
			if label != "systems.determinate.nix-daemon" && label != "org.nixos.nix-daemon" {
				continue
			}
			service := "system/" + label
			state, err := d.launchdState(ctx, service)
			if err != nil {
				return err
			}
			if state == "loaded" {
				command := d.systemCommand(true, "/bin/launchctl", "bootout", service)
				scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Stop the receipt's Nix daemon before checking remaining consumers", Privileged: true, Command: &command,
					Gates:  append(slices.Clone(before), legacyRetirementGate{Kind: "launchd", Subject: service, Expected: "loaded"}),
					Checks: []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "absent"}}})
			}
		}
	}
	if len(scope.Steps) != first {
		scope.Detail = append(scope.Detail, "The receipt's Nix daemon is stopped before checking for remaining processes. If another process blocks removal, the store stays intact and the daemon remains stopped until retirement is resumed or the service is started manually.")
	}
	return nil
}

// The vendor deletes nix.conf and nix.custom.conf, which can hold personal
// settings or access tokens. Keep an exact copy first. nix-darwin's retirement
// runs earlier in the same plan and would change links it owns in /etc/nix.
func (d *legacyRetirementDriver) planEtcNixCopy(scope *legacyRetirementScope, nixDarwin bool) error {
	entries, err := inspectTree("/etc/nix", maxConfigBytes, maxConfigEntries)
	if err != nil {
		scope.Blocked = legacyReason("/etc/nix could not be recorded before the vendor uninstall", err)
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	for _, entry := range entries {
		if nixDarwin && entry.Kind == "link" && strings.HasPrefix(entry.Content, "/etc/static/") {
			scope.Blocked = "nix-darwin manages /etc/nix/" + entry.Path + ", which the Determinate uninstall also changes; resolve that overlap natively, then review retirement again"
			return nil
		}
	}
	snapshot, err := snapshotEntries(entries)
	if err != nil {
		return err
	}
	config := legacyRetirementFile{Path: "/etc/nix", Snapshot: snapshot}
	identity, err := legacyPathIdentity("/etc/nix")
	if err != nil {
		return err
	}
	backup, err := legacySiblingBackup("/etc/nix", identity)
	if err != nil {
		return err
	}
	existing, err := legacyFile(backup)
	switch {
	case err != nil:
		return err
	case existing.Snapshot == config.Snapshot:
		scope.Detail = append(scope.Detail, "An exact copy of /etc/nix is already kept at "+backup+".")
	case existing.Snapshot.Kind != "absent":
		scope.Blocked = backup + " already exists and differs from /etc/nix; review it before retirement"
	default:
		command := d.systemCommand(true, "/bin/cp", "-pR", "--", "/etc/nix", backup)
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Keep a copy of the system Nix configuration: /etc/nix -> " + backup, Privileged: true,
			Backup: backup, Command: &command, Before: []legacyRetirementFile{config}, After: []legacyRetirementFile{{Path: backup, Snapshot: config.Snapshot}},
			Gates: []legacyRetirementGate{{Kind: "path-identity", Subject: backup, Expected: "absent"}}})
	}
	return nil
}

// systemd drop-ins are personal overrides the vendor never removes. Move them
// aside after the units are gone, then refresh the unit inventory.
func (d *legacyRetirementDriver) planDeterminateDropIns(scope *legacyRetirementScope, units []string, reload bool) error {
	first := len(scope.Steps)
	for _, unit := range units {
		if err := d.addSystemMove(scope, "Preserve the retired Nix unit override", d.sys("/etc/systemd/system/"+unit+".d")); err != nil {
			return err
		}
	}
	if first == len(scope.Steps) && !reload {
		return nil
	}
	systemctl, err := legacySystemProgram("/usr/bin/systemctl", "/bin/systemctl")
	if err != nil {
		return err
	}
	command := d.systemCommand(true, systemctl, "daemon-reload")
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Reload the systemd unit inventory", Privileged: true, Command: &command})
	return nil
}

// planDeterminateLeftovers plans the remainder of an approved vendor uninstall
// after /nix/receipt.json is gone, for example after a failed or unknown vendor
// run. Only artifacts named by intact saved evidence qualify, accounts and the
// group must still match their exact approved records, and the vendor is never
// run again. /etc/nix stays as personal configuration. The generic leftover
// planner keeps /etc/fstab, /etc/synthetic.conf, an empty /nix and user files.
func (d *legacyRetirementDriver) planDeterminateLeftovers(ctx context.Context, scope *legacyRetirementScope) error {
	evidence, err := d.determinateEvidence()
	if err != nil {
		scope.Blocked = legacyReason("saved Determinate uninstall evidence is unreadable; preserve "+d.determinateEvidenceDirectory(), err)
		return nil
	}
	type approved struct {
		r    legacyDeterminateReceipt
		item legacyDeterminateEvidence
	}
	receipts := []approved{}
	for _, item := range evidence {
		r, err := parseLegacyDeterminateReceipt(item.Receipt)
		if err != nil {
			return err
		}
		if want := map[string]string{"linux": "linux", "darwin": "macos"}[d.Platform.OS]; r.Planner != want {
			continue
		}
		if item.VolumeUUID != "" {
			state, err := d.volumeIdentity(ctx, item.VolumeUUID)
			if err != nil {
				return err
			}
			if state != "absent" {
				scope.Blocked = "the approved Nix Store volume " + item.VolumeUUID + " still exists without its receipt; mount it at /nix or inspect it natively"
				return nil
			}
		}
		receipts = append(receipts, approved{r, item})
	}
	if len(receipts) == 0 {
		return nil
	}
	services, units, files := []string{}, []string{}, []string{}
	for _, entry := range receipts {
		r := entry.r
		services = append(services, r.Services...)
		if r.Planner == "macos" {
			services = append(services, "systems.determinate.nix-installer.nix-hook")
		}
		for _, path := range append(slices.Clone(r.Removed), legacyDeterminateUnitLinks(r)...) {
			if strings.HasPrefix(path, "/etc/nix/") {
				continue
			}
			file, err := legacyFile(d.sys(path))
			if err != nil {
				return err
			}
			if file.Snapshot.Kind == "absent" || slices.Contains(files, d.sys(path)) {
				continue
			}
			proved := false
			for _, approved := range receipts {
				proved = proved || slices.Contains(approved.item.Files, file)
			}
			if !proved {
				scope.Blocked = path + " is not an unchanged file from an approved Determinate uninstall; preserve it and inspect it manually"
				return nil
			}
			if path == legacyDeterminateNixdPath {
				sum, err := legacyFileSHA256(d.sys(path))
				if err != nil {
					return err
				}
				if sum != entry.item.Nixd {
					scope.Retained = append(scope.Retained, path+" changed since the approved uninstall; review it manually")
					continue
				}
			}
			files = append(files, d.sys(path))
			if unit, ok := strings.CutPrefix(path, "/etc/systemd/system/"); ok && slices.Contains(r.Units, unit) && !slices.Contains(units, unit) {
				units = append(units, unit)
			}
		}
	}
	for _, label := range sortedUnique(services) {
		service := "system/" + label
		if state, err := d.launchdState(ctx, service); err != nil {
			return err
		} else if state == "loaded" {
			path := d.sys("/Library/LaunchDaemons/" + label + ".plist")
			if !slices.Contains(files, path) {
				scope.Blocked = "the loaded service " + service + " has no unchanged approved vendor file; inspect it natively"
				return nil
			}
			file, err := legacyFile(path)
			if err != nil {
				return err
			}
			command := d.systemCommand(true, "/bin/launchctl", "bootout", service)
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Stop the leftover launch service " + service, Privileged: true, Command: &command,
				Before: []legacyRetirementFile{file},
				Gates:  []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "loaded"}},
				Checks: []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "absent"}}})
		}
	}
	if len(units) != 0 {
		// Only units whose files remain can be stopped and disabled.
		systemctl, err := legacySystemProgram("/usr/bin/systemctl", "/bin/systemctl")
		if err != nil {
			return err
		}
		stop, err := d.systemdStopStep("Stop the leftover Nix daemon units", units)
		if err != nil {
			return err
		}
		disable := d.systemCommand(true, systemctl, append([]string{"disable"}, units...)...)
		before := []legacyRetirementFile{}
		for _, unit := range units {
			file, err := legacyFile(d.sys("/etc/systemd/system/" + unit))
			if err != nil {
				return err
			}
			before = append(before, file)
		}
		stop.Before = before
		scope.Steps = append(scope.Steps, stop,
			legacyRetirementStep{Kind: "command", Label: "Disable the leftover Nix daemon units", Privileged: true, Command: &disable, Before: before})
	}
	for _, path := range files {
		if err := addLegacyRemoval(scope, "Remove the leftover vendor file", path); err != nil {
			return err
		}
	}
	if d.Platform.OS == "linux" {
		all := []string{}
		for _, entry := range receipts {
			all = append(all, entry.r.Units...)
		}
		reloads := len(units) != 0 || slices.ContainsFunc(files, func(path string) bool { return strings.HasPrefix(path, d.sys("/etc/systemd")+"/") })
		if err := d.planDeterminateDropIns(scope, sortedUnique(all), reloads); err != nil {
			return err
		}
	}
	keychain, inserts, accounts, groups := false, []string{}, []string{}, []string{}
	for _, entry := range receipts {
		r, item := entry.r, entry.item
		if v := r.Volume; v != nil && v.Keychain && !keychain {
			keychain = true
			present, err := d.determinateKeychain(ctx, *v)
			if err != nil {
				return err
			}
			if present {
				command := d.systemCommand(true, "/usr/bin/security", "delete-generic-password", "-a", v.Name, "-s", "Nix Store", "-l", v.Disk+" encryption password", legacyDeterminateKeychain)
				scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete the leftover Nix Store volume password from the System keychain", Privileged: true, Command: &command})
			}
		}
		for _, insert := range r.Inserts {
			if insert.Reverted && !slices.Contains(inserts, insert.Path) {
				inserts = append(inserts, insert.Path)
				insert.Path = d.sys(insert.Path)
				if err := d.addDeterminateBlockRemoval(ctx, scope, insert); err != nil || scope.Blocked != "" {
					return err
				}
			}
		}
		for _, record := range item.Accounts {
			name := strings.SplitN(record, ":", 2)[0]
			index := slices.IndexFunc(r.Users, func(user legacyDeterminateUser) bool { return user.Name == name })
			if index < 0 || slices.Contains(accounts, name) {
				continue
			}
			accounts = append(accounts, name)
			current, err := d.accountRecord(ctx, name)
			if err != nil {
				return err
			}
			if current == "absent" {
				continue
			}
			if current != record {
				scope.Blocked = "account " + name + " changed since the approved Determinate uninstall; inspect it natively"
				return nil
			}
			step, err := d.determinateAccountStep("Delete the leftover Nix build account", name, record, r.Users[index].UID)
			if err != nil {
				return err
			}
			scope.Steps = append(scope.Steps, step)
		}
		if item.Group == "" || slices.Contains(groups, r.Group) {
			continue
		}
		groups = append(groups, r.Group)
		current, err := d.groupRecord(ctx, r.Group)
		if err != nil {
			return err
		}
		if current != "absent" && current != item.Group {
			scope.Blocked = "the " + r.Group + " group changed since the approved Determinate uninstall; inspect it natively"
			return nil
		}
		if current != "absent" {
			step, err := d.determinateGroupStep("Delete the leftover Nix build group", r.Group, item.Group)
			if err != nil {
				return err
			}
			scope.Steps = append(scope.Steps, step)
		}
	}
	if _, err := os.Lstat(d.sys("/etc/nix")); err == nil {
		scope.Detail = append(scope.Detail, "System Nix configuration remaining at /etc/nix stays unchanged.")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// addDeterminateBlockRemoval removes the last exact occurrence of a vendor
// insertion, as the vendor revert would, through the reviewed root edit step.
// Every other byte stays and the original is kept beside the file.
func (d *legacyRetirementDriver) addDeterminateBlockRemoval(ctx context.Context, scope *legacyRetirementScope, insert legacyDeterminateInsert) error {
	if problem := d.determinateInsertProblem(ctx, insert.Path, insert.Buf, ""); problem != "" {
		scope.Blocked = problem
		return nil
	}
	data, snapshot, err := d.readSystemText(ctx, insert.Path)
	if err != nil || data == nil {
		return err
	}
	index := bytes.LastIndex(data, []byte(insert.Buf))
	if index < 0 {
		return nil
	}
	content := slices.Concat(data[:index], data[index+len(insert.Buf):])
	uid, gid, mode, err := legacyFileOwner(insert.Path)
	if err != nil {
		return err
	}
	backup, err := legacySiblingBackup(insert.Path, legacyHash(data))
	if err != nil {
		return err
	}
	step := legacyRetirementStep{Kind: "edit", Label: "Remove the leftover vendor Nix block from " + insert.Path, Privileged: true, Path: insert.Path, Backup: backup,
		Identity: legacyHash(data), Mode: mode, Content: content, Owner: strconv.FormatUint(uint64(uid), 10), Group: strconv.FormatUint(uint64(gid), 10),
		Before: []legacyRetirementFile{{Path: insert.Path, Snapshot: snapshot}}, After: []legacyRetirementFile{{Path: backup, Snapshot: snapshot}}}
	if len(content) == 0 {
		step.Absent = []string{insert.Path}
	} else {
		after, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash(content), Mode: mode & 0o777}})
		if err != nil {
			return err
		}
		step.After = append(step.After, legacyRetirementFile{Path: insert.Path, Snapshot: after})
	}
	scope.Steps = append(scope.Steps, step)
	return nil
}
