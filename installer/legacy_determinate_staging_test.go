package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func legacyDeterminateStagingParent(t *testing.T, root string) string {
	t.Helper()
	parent := filepath.Join(root, "staging")
	if err := os.Mkdir(parent, 0700|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	return parent
}

func TestLegacyDeterminateExecutesOnlyVerifiedPrivateCopies(t *testing.T) {
	d, root := legacyStepFixture(t)
	parent := legacyDeterminateStagingParent(t, root)
	installer, receipt := filepath.Join(root, "nix-installer"), filepath.Join(root, "receipt.json")
	witness := filepath.Join(root, "executed")
	writeConfigFixture(t, receipt, "approved receipt\n")
	writeConfigFixture(t, installer, "#!/bin/sh\nprintf changed > '"+installer+"'\nprintf changed > '"+receipt+"'\nprintf '%s\\n' \"$0\" \"$3\" > '"+witness+"'\ncat \"$3\" >> '"+witness+"'\n")
	if err := os.Chmod(installer, 0700); err != nil {
		t.Fatal(err)
	}
	command := legacyDeterminateUninstallCommand(installer, legacyFileHash(t, installer), receipt, legacyFileHash(t, receipt))
	command.Arguments = append(command.Arguments, parent)
	step := legacyRetirementStep{Kind: "command", Label: "verified private vendor copy", Command: &command}
	if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], parent+string(filepath.Separator)) || lines[0] == installer || filepath.Dir(lines[0]) != filepath.Dir(lines[1]) || lines[1] == receipt || lines[2] != "approved receipt" {
		t.Fatal("the vendor did not execute with private verified copies", string(data))
	}
	for _, path := range []string{filepath.Dir(lines[0]), lines[0]} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("staging is not private", path, err)
		}
		uid, _, _, err := legacyFileOwner(path)
		if err != nil || int(uid) != os.Geteuid() {
			t.Fatal("staging is not owned by the executing account", uid, err)
		}
	}
}

func TestLegacyDeterminateRejectsNonStickyStagingParent(t *testing.T) {
	d, root := legacyStepFixture(t)
	installer, receipt := filepath.Join(root, "nix-installer"), filepath.Join(root, "receipt.json")
	writeConfigFixture(t, installer, "#!/bin/sh\nexit 99\n")
	writeConfigFixture(t, receipt, "approved receipt\n")
	command := legacyDeterminateUninstallCommand(installer, legacyFileHash(t, installer), receipt, legacyFileHash(t, receipt))
	command.Arguments = append(command.Arguments, root)
	step := legacyRetirementStep{Kind: "command", Label: "refuse unsafe stage", Command: &command}
	if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err == nil || !strings.Contains(err.Error(), "exit 121") {
		t.Fatal("unsafe staging parent was accepted", err)
	}
	if entries, err := filepath.Glob(filepath.Join(root, "dotfiles-determinate.*")); err != nil || len(entries) != 0 {
		t.Fatal("unsafe staging parent was changed", entries, err)
	}
}

func TestLegacyDeterminateSavedOriginalCommandRemainsReadable(t *testing.T) {
	d, root := legacyStepFixture(t)
	installer, receipt := filepath.Join(root, "nix-installer"), filepath.Join(root, "receipt.json")
	witness := filepath.Join(root, "executed")
	writeConfigFixture(t, installer, "#!/bin/sh\nprintf '%s\\n' \"$0\" \"$3\" > '"+witness+"'\n")
	if err := os.Chmod(installer, 0700); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, receipt, "historically approved receipt\n")
	command := legacyDeterminateUninstallCommand(installer, legacyFileHash(t, installer), receipt, legacyFileHash(t, receipt))
	for i, arg := range command.Arguments {
		if arg == legacyDeterminateStagedUninstallScript {
			command.Arguments[i] = legacyDeterminateUninstallScript
		}
	}
	step := legacyRetirementStep{Kind: "command", Label: "saved original vendor recipe", Command: &command}
	if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
		t.Fatal("saved literal command was reinterpreted", err)
	}
	if data, err := os.ReadFile(witness); err != nil || string(data) != installer+"\n"+receipt+"\n" {
		t.Fatal("saved original invocation changed", string(data), err)
	}
}
