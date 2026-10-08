package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixture preparation belongs to an explicitly disposable runner image/job.
// This test never deletes Xcode/CLT or hides toolchains to manufacture absence.
// Existing Homebrew must remain installed. CLT remains retained afterwards.
func TestNativeAppleCLTMissingPayloadWithExistingHomebrew(t *testing.T) {
	baseline := appleCLTNativeGuard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := newNativeSession(filepath.Join(home, "state", "worker"))
	session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(session.Directory) }
	d, err := configureAppleCLT(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, ConfigFolders{Home: home}, session)
	if err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: "infra.apple-clt", Name: "Apple developer tools", Action: "native", Scope: "machine", Retain: true, ReplanAfter: true}
	before, err := d.Observe(ctx, r, Receipt{})
	if err != nil || before.Present || before.Unknown || before.Pending != "" {
		t.Fatal("fixture requires actual missing CLT and no usable selected Xcode", before, err)
	}
	historical, err := d.nativeReceipt(ctx)
	if err != nil || historical == "none" {
		t.Fatal("hosted missing-payload fixture requires its historical receipt", historical, err)
	}
	t.Log("approved historical native receipt:", historical)
	verifyAppleCLTNativePreservation(t, ctx, baseline)
	brew := "/opt/homebrew/bin/brew"
	brewBefore, err := snapshotTree(brew, 8<<20, 1)
	if err != nil || brewBefore.Kind != "file" {
		t.Fatal("fixture requires existing Apple Silicon Homebrew", brewBefore.Kind, err)
	}
	metadataBefore, err := macOSPrerequisiteQuery(ctx, false, "/usr/bin/stat", nil, "-f", "%u:%g:%Lp", "/opt/homebrew")
	if err != nil {
		t.Fatal(err)
	}
	release, err := session.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	installed, applyErr := d.Apply(ctx, r, Operation{Action: "install", Observed: before}, bootstrapReceipt("a"))
	if err := errors.Join(applyErr, release()); err != nil {
		t.Fatal(err)
	}
	if !installed.Healthy || installed.CompletedOperation != strings.Repeat("a", 64) {
		t.Fatal("native install lacked completion evidence", installed)
	}
	checked, err := d.Observe(ctx, r, bootstrapReceipt("a"))
	if err != nil || !checked.Healthy || !sameArtifact(installed, checked) {
		t.Fatal("installed developer tools failed readback", checked, err)
	}
	homebrewBootstrapRetainedPlan(t, r, checked, "update", "keep")
	homebrewBootstrapRetainedPlan(t, r, checked, "apply", "retain")
	brewAfter, err := snapshotTree(brew, 8<<20, 1)
	if err != nil || brewAfter.Hash != brewBefore.Hash {
		t.Fatal("CLT provisioning changed the Homebrew executable", err)
	}
	metadataAfter, err := macOSPrerequisiteQuery(ctx, false, "/usr/bin/stat", nil, "-f", "%u:%g:%Lp", "/opt/homebrew")
	if err != nil || string(metadataAfter) != string(metadataBefore) {
		t.Fatal("CLT provisioning changed Homebrew ownership/permissions", err)
	}
	// Verify the new compiler links an actual SDK-using C program. The build
	// and executable are confined to this disposable test directory.
	source, binary := filepath.Join(home, "hello.c"), filepath.Join(home, "hello")
	if err := os.WriteFile(source, []byte("#include <stdio.h>\nint main(void) { puts(\"clt-native-ok\"); return 0; }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "/usr/bin/xcrun", "--no-cache", "--sdk", "macosx", "clang", source, "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatal("installed Apple compiler/SDK link failed", err, nativeDiagnostic(output))
	}
	if output, err := exec.CommandContext(ctx, binary).Output(); err != nil || string(output) != "clt-native-ok\n" {
		t.Fatal("new compiled program failed", err)
	}
	verifyAppleCLTNativePreservation(t, ctx, baseline)
	installedReceipt, err := d.nativeReceipt(ctx)
	if err != nil || installedReceipt == historical {
		t.Fatal("native receipt did not change", installedReceipt, err)
	}
	intent, err := d.intent(bootstrapReceipt("a").OperationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range intent.Commands {
		reply, err := d.result(step.Command)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("Apple advertised/install evidence:", nativeDiagnostic(reply.Output))
	}
	t.Log("historical receipt", historical, "installed receipt", installedReceipt)
	t.Log("installed missing CLT through saved native worker, verified actual compiler/Make/SDK and compiled runtime, retained Apple tools, preserved existing Homebrew")
}
