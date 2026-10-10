package installer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This contract test intentionally mutates the disposable runner's package
// database. It never runs as part of ordinary local make ci. Packages contain
// only uniquely named fixture data and have no maintainer scripts or triggers.
func TestNativeAPTTransactionAttributionAndRemovalBoundary(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || runtime.GOOS != "linux" {
		t.Skip("requires an explicitly opted-in disposable Linux package host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	run := func(program string, args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, program, args...)
		command.Env = append(os.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
		return command.CombinedOutput()
	}
	require := func(program string, args ...string) []byte {
		t.Helper()
		output, err := run(program, args...)
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", program, args, err, output)
		}
		return output
	}
	before := require("dpkg-query", "-W", "-f=${binary:Package}\t${Version}\t${db:Status-Abbrev}\n")
	root, err := os.MkdirTemp("/var/tmp", "dotfiles-apt-contract-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	prefix := "dotfiles-fixture-" + strings.ToLower(rand.Text()[:10])
	names := []string{prefix + "-first", prefix + "-second", prefix + "-outside", prefix + "-shared", prefix + "-unrelated"}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, args := range [][]string{append([]string{"-n", "dpkg", "--remove"}, names...), {"-n", "rm", "-rf", "--", root}} {
			output, err := exec.CommandContext(cleanup, "sudo", args...).CombinedOutput()
			if err != nil {
				t.Errorf("fixture cleanup failed: %v\n%s", err, output)
			}
		}
	})
	repository := filepath.Join(root, "repository")
	for _, dir := range []string{repository, filepath.Join(root, "lists/partial"), filepath.Join(root, "archives/partial")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	var index bytes.Buffer
	for _, name := range names {
		control := "Package: " + name + "\nVersion: 1.0\nArchitecture: all\nMaintainer: Dotfiles test <test@example.invalid>\nDescription: Disposable package-manager contract fixture\n"
		if name == names[0] || name == names[1] || name == names[2] {
			control += "Depends: " + names[3] + "\n"
		}
		build := filepath.Join(root, "build", name)
		for _, dir := range []string{"DEBIAN", "usr/share/" + name} {
			if err := os.MkdirAll(filepath.Join(build, dir), 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(build, "DEBIAN/control"), []byte(control), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(build, "usr/share", name, "fixture"), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(repository, name+".deb")
		require("dpkg-deb", "--root-owner-group", "--build", build, archive)
		data, err := os.ReadFile(archive)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&index, "%sFilename: ./%s.deb\nSize: %d\nSHA256: %x\n\n", control, name, len(data), sha256.Sum256(data))
	}
	if err := os.WriteFile(filepath.Join(repository, "Packages"), index.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	// This trusted file repository contains only fixture bytes created above.
	// It is scoped to these invocations, never added to the host sources list.
	sources := filepath.Join(root, "sources.list")
	if err := os.WriteFile(sources, []byte("deb [trusted=yes] file:"+repository+" ./\n"), 0644); err != nil {
		t.Fatal(err)
	}
	apt := []string{"-n", "env", "DEBIAN_FRONTEND=noninteractive", "LC_ALL=C", "apt-get", "-o", "Dir::Etc::sourcelist=" + sources, "-o", "Dir::Etc::sourceparts=-", "-o", "Dir::State::lists=" + filepath.Join(root, "lists"), "-o", "Dir::Cache::archives=" + filepath.Join(root, "archives"), "-o", "APT::Get::List-Cleanup=0"}
	require("sudo", append(append([]string{}, apt...), "update")...)
	install := func(operation, name string) string {
		t.Helper()
		operation, err := digest(operation)
		if err != nil {
			t.Fatal(err)
		}
		log := filepath.Join(root, operation+".history")
		dpkgLog := filepath.Join(root, operation+".dpkg")
		args := append(append([]string{}, apt...), "-o", "Dir::Log::History="+log, "-o", "Dpkg::Options::=--log="+dpkgLog, "-o", "Dotfiles::Operation="+operation, "--yes", "--no-install-recommends", "--no-remove", "install", name)
		require("sudo", args...)
		data, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(dpkgLog)
		if err != nil {
			t.Fatal(err)
		}
		evidence, err := aptInstallEvidence(data, actual, operation)
		if err != nil || len(evidence) == 0 {
			t.Fatalf("actual native installation evidence: %v\nhistory:\n%s\ndpkg:\n%s", err, data, actual)
		}
		return string(data)
	}
	install("preexisting-orphan", names[4])
	require("sudo", "-n", "apt-mark", "auto", names[4])
	first := install("first-operation", names[0])
	if !strings.Contains(first, "Install:") || !strings.Contains(first, names[0]+":") || !strings.Contains(first, names[3]+":") || !strings.Contains(first, "End-Date:") || strings.Contains(first, names[4]+":") {
		t.Fatal("operation-bound install history is incomplete or claims a pre-existing orphan", first)
	}
	second := install("second-operation", names[1])
	if !strings.Contains(second, names[1]+":") || strings.Contains(second, names[3]+":") {
		t.Fatal("second transaction claimed an existing shared dependency", second)
	}
	if again := install("no-op-operation", names[0]); strings.Contains(again, "Install:") {
		t.Fatal("already installed package has new installation authority", again)
	}
	// A consumer appearing after an earlier removal preview must be protected
	// by the actual removal command, not just an earlier solver/dry-run result.
	install("outside-operation", names[2])
	nativeRun := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged {
			args = append([]string{"-n", program}, args...)
			program = "sudo"
		}
		command := exec.CommandContext(ctx, program, args...)
		command.Env = append(os.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
		command.Stdin = bytes.NewReader(input)
		return command.CombinedOutput()
	}
	if output, err := removeAPTPackages(ctx, nativeRun, []string{names[3]}); err == nil || !bytes.Contains(output, []byte("dependency problems")) {
		t.Fatalf("exact native removal failed to refuse a used dependency: %v\n%s", err, output)
	}
	for _, name := range []string{names[2], names[3], names[4]} {
		if output := require("dpkg-query", "-W", "-f=${db:Status-Abbrev}", name); string(output) != "ii " {
			t.Fatal("protected package changed", name, string(output))
		}
	}
	if output, err := removeAPTPackages(ctx, nativeRun, names[:4]); err != nil {
		t.Fatalf("exact successful removal: %v\n%s", err, output)
	}
	if output := require("dpkg-query", "-W", "-f=${db:Status-Abbrev}", names[4]); string(output) != "ii " {
		t.Fatal("exact removal swept a pre-existing orphan", string(output))
	}
	require("sudo", "-n", "dpkg", "--remove", names[4])
	after := require("dpkg-query", "-W", "-f=${binary:Package}\t${Version}\t${db:Status-Abbrev}\n")
	withoutFixtures := func(data []byte) string {
		var result []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, prefix) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n")
	}
	if withoutFixtures(before) != withoutFixtures(after) {
		t.Fatal("contract fixture changed another native package")
	}
	t.Log("operation-local APT and dpkg records prove actual new packages; exact dpkg removal protects consumers and pre-existing orphans")
}
