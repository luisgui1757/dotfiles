package installer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func windowsBuildToolsNativeFixture(t *testing.T) (*WindowsBuildToolsDriver, Resource) {
	t.Helper()
	base, _ := windowsVendorNativeFixture(t)
	return &WindowsBuildToolsDriver{Directory: base.Directory, WorkerDirectory: base.WorkerDirectory, PowerShell: base.PowerShell, Query: queryWindowsBuildTools,
		InstallDirectory: filepath.Join(os.Getenv("ProgramFiles"), "DotfilesBuildToolsFixture"),
		Pin: WindowsBuildToolsPin{Version: "17.14.37710.0", Size: 4478032, SHA256: "985969f472caad75d993a5cb4c35a6a4271460cc12b343e2433b994d173aa990",
			URL: "https://download.visualstudio.microsoft.com/download/pr/bc92e2cb-33de-4a0c-995d-efa817f16b16/985969f472caad75d993a5cb4c35a6a4271460cc12b343e2433b994d173aa990/vs_BuildTools.exe", Components: append([]string{}, windowsBuildToolsComponents...)}}, Resource{ID: "tool.compiler", Scope: "machine", Shared: true}
}

func TestWindowsBuildToolsNativeReuseAndEnvironment(t *testing.T) {
	driver, resource := windowsBuildToolsNativeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	observed, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if !observed.Present {
		t.Skip("no pre-existing compiler; fresh lifecycle fixture supplies its separate proof")
	}
	if !observed.Healthy || observed.CompletedOperation != "" {
		t.Fatalf("pre-existing compiler is not reusable: %+v", observed)
	}
	environment, err := driver.CompilerEnvironment(ctx)
	if err != nil || len(environment) != 8 {
		t.Fatalf("restricted native compiler environment: %v", err)
	}
	for range 2 {
		fresh, err := driver.Observe(ctx, resource, Receipt{})
		if err != nil || !fresh.Healthy || fresh.Fingerprint != observed.Fingerprint || !slices.Equal(fresh.Consumers, observed.Consumers) {
			t.Fatalf("compiler inspection changed its own inventory: before=%+v after=%+v: %v", observed, fresh, err)
		}
	}
}

func TestWindowsBuildToolsNativeOwnedLifecycle(t *testing.T) {
	driver, resource := windowsBuildToolsNativeFixture(t)
	if os.Getenv("DOTFILES_NATIVE_WINDOWS_BUILDTOOLS_INSTALL") != "1" {
		t.Skip("Build Tools installation additionally requires DOTFILES_NATIVE_WINDOWS_BUILDTOOLS_INSTALL=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	observed, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if observed.Present || observed.ApplyBlocked != "" {
		t.Fatal("fresh Build Tools prerequisite failed; preserve existing tools and use an isolated disposable Windows container")
	}
	root := filepath.Dir(driver.Directory)
	unlock, err := Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})
	session := &nativeSession{Directory: driver.WorkerDirectory, Start: func(context.Context) (*nativeWorkerClient, error) { return workerFixture(driver.WorkerDirectory) }}
	native := &NativeDriver{StatePath: filepath.Join(root, "state.json"), session: session}
	release, err := native.AcquireMutation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	driver.Run = session.run
	receipt := Receipt{OperationID: strings.Repeat("1", 64), Ownership: "uncertain", Status: "in-progress"}
	after, err := driver.Apply(ctx, resource, Operation{Action: "install"}, receipt)
	if err != nil || !after.Healthy || after.Pending != "" || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native Build Tools install has not proved readiness: %+v: %v", after, err)
	}
	if _, err := driver.CompilerEnvironment(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		fresh, err := driver.Observe(ctx, resource, receipt)
		if err != nil || !fresh.Healthy || len(fresh.Consumers) != 0 || fresh.Fingerprint != after.Fingerprint {
			t.Fatalf("fresh owned compiler inspection left a consumer or changed inventory: %+v: %v", fresh, err)
		}
	}
	receipt.Ownership = "created"
	for index, action := range []string{"update", "repair", "remove"} {
		receipt.After, receipt.OperationID = after, strings.Repeat(string(rune('2'+index)), 64)
		if action == "remove" {
			after, err = driver.Remove(ctx, resource, receipt)
		} else {
			after, err = driver.Apply(ctx, resource, Operation{Action: action}, receipt)
		}
		if err != nil || after.Pending != "" || after.CompletedOperation != receipt.OperationID || action == "remove" && after.Present || action != "remove" && !after.Healthy {
			t.Fatalf("native Build Tools %s has not verified: %+v: %v", action, after, err)
		}
		if len(after.Consumers) != 0 {
			t.Fatalf("native Build Tools %s left running consumers in the fresh fixture: %+v", action, after)
		}
	}
}
