package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestAppleCLTHistoricalReceiptAllowsMissingPayload(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.installed = true
	before, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err != nil || before.Present || before.Unknown || before.Pending != "" || !strings.Contains(before.HealthIssue, "historical receipt 26.0.0, install-time 100") {
		t.Fatal(before, err)
	}
	after, err := f.apply()
	if err != nil || !after.Healthy || len(f.commands) != 1 {
		t.Fatal(after, err, len(f.commands))
	}
	intent, err := f.d.intent(bootstrapReceipt("a").OperationID)
	if err != nil || intent.Baseline != "26.0.0@100" || !strings.Contains(strings.Join(f.commands[0].Arguments, "\n"), intent.Baseline) {
		t.Fatal(intent, err)
	}
}

func TestAppleCLTHistoricalReceiptDoesNotAuthorizeOccupiedOrForeignTools(t *testing.T) {
	for _, state := range []string{"directory", "foreign", "broken"} {
		t.Run(state, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			f.receipt = "26.0.0@100"
			switch state {
			case "directory":
				if err := os.Mkdir(f.d.clt, 0755); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				f.selected = "/missing/Xcode.app/Contents/Developer"
			case "broken":
				f.install(f.d.clt)
				f.selected = f.d.clt
				f.unhealthy = true
			}
			before, err := f.d.Observe(context.Background(), f.r, Receipt{})
			if err != nil || !before.Unknown {
				t.Fatal(before, err)
			}
			if _, err := f.apply(); err == nil || len(f.commands) != 0 {
				t.Fatal("overwrote existing tools", err)
			}
		})
	}
}

func TestAppleCLTReceiptDriftRejectsApprovalAndResume(t *testing.T) {
	for _, phase := range []string{"approval", "unsent", "before-install", "after-install"} {
		t.Run(phase, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			f.receipt = "26.0.0@100"
			before, err := f.d.Observe(context.Background(), f.r, Receipt{})
			if err != nil {
				t.Fatal(err)
			}
			if phase != "approval" {
				if phase == "unsent" {
					f.d.Authenticate = func(context.Context) error { return errors.New("not authorized") }
				} else if phase == "before-install" {
					f.fail = "before"
				} else {
					f.fail = "after-install"
				}
				if _, err := f.apply(); err == nil {
					t.Fatal("injected failure hidden")
				}
				f.d.Authenticate = nil
			}
			count := len(f.commands)
			f.receipt = "26.0.0@300"
			if phase == "approval" {
				_, err = f.d.Apply(context.Background(), f.r, Operation{Action: "install", Observed: before}, bootstrapReceipt("a"))
			} else {
				_, err = f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
			}
			if err == nil || len(f.commands) != count {
				t.Fatal("receipt drift dispatched mutation", err, len(f.commands), count)
			}
		})
	}
}

func TestAppleCLTUnchangedReceiptCannotProveCompletion(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.receipt = "26.0.0@100"
	f.unchanged = true
	o, err := f.apply()
	if err == nil || o.CompletedOperation != "" {
		t.Fatal(o, err)
	}
}

func TestAppleCLTLegacyIntentRemainsReadableButCannotResume(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.fail = "before"
	if _, err := f.apply(); err == nil {
		t.Fatal("failure hidden")
	}
	intent, err := f.d.intent(bootstrapReceipt("a").OperationID)
	if err != nil {
		t.Fatal(err)
	}
	intent.Schema = 1
	intent.Baseline = ""
	if err := saveDocument(f.d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.d.intentPath(intent.Operation))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil || !strings.Contains(err.Error(), "schema 1") || len(f.commands) != 1 {
		t.Fatal(err)
	}
	after, err := os.ReadFile(f.d.intentPath(intent.Operation))
	if err != nil || string(after) != string(before) {
		t.Fatal("legacy journal changed", err)
	}
}

func TestAppleCLTReceiptStrictIdentity(t *testing.T) {
	valid := "package-id: com.apple.pkg.CLTools_Executables\nversion: 27.0.0.0.1788430756\nvolume: /\nlocation: /\ninstall-time: 1789093597\n"
	if got, err := parseAppleCLTReceipt([]byte(valid)); err != nil || got != "27.0.0.0.1788430756@1789093597" {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"", valid + "version: 2\n", strings.Replace(valid, "volume: /", "volume: /other", 1), strings.Replace(valid, "location: /", "location: /other", 1), strings.Replace(valid, "1789093597", "0", 1), strings.Replace(valid, "1789093597", "18446744073709551616", 1), strings.Replace(valid, "package-id: com.apple.pkg.CLTools_Executables", "package-id: unrelated", 1), strings.Replace(valid, "version: 27.0.0.0.1788430756", "version: unknown", 1)} {
		if _, err := parseAppleCLTReceipt([]byte(bad)); err == nil {
			t.Fatal("accepted malformed receipt", bad)
		}
	}
	for _, failure := range []string{"info-error", "malformed", "selection-error"} {
		t.Run(failure, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			f.receipt = "26.0.0@100"
			query := f.d.Query
			f.d.Query = func(ctx context.Context, p bool, program string, input []byte, args ...string) ([]byte, error) {
				if failure == "selection-error" && program == "/usr/bin/xcode-select" {
					return nil, errors.New("unexpected native selection failure")
				}
				if program == "/usr/sbin/pkgutil" && args[0] == "--pkg-info=com.apple.pkg.CLTools_Executables" {
					if failure == "info-error" {
						return nil, errors.New("receipt database unavailable")
					}
					return []byte("invalid"), nil
				}
				return query(ctx, p, program, input, args...)
			}
			if _, err := f.apply(); err == nil || len(f.commands) != 0 {
				t.Fatal("query error authorized install", err)
			}
		})
	}
}

func TestAppleCLTSelectionExitBoundary(t *testing.T) {
	if os.Getenv("DOTFILES_APPLE_SELECTION_EXIT_FIXTURE") == "1" {
		os.Exit(2)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestAppleCLTSelectionExitBoundary$")
	command.Env = append(os.Environ(), "DOTFILES_APPLE_SELECTION_EXIT_FIXTURE=1")
	err := command.Run()
	if !appleCLTSelectionMissing(err, nil, []byte(appleCLTNoSelectionDiagnostic)) {
		t.Fatal("exact exit2 and diagnostic not recognized", err)
	}
	for _, diagnostic := range []string{"", strings.TrimSpace(appleCLTNoSelectionDiagnostic), appleCLTNoSelectionDiagnostic + "other error\n", "permission denied\n"} {
		if appleCLTSelectionMissing(err, nil, []byte(diagnostic)) {
			t.Fatal("noncanonical failure treated as absence")
		}
	}
	if appleCLTSelectionMissing(err, []byte("partial output"), []byte(appleCLTNoSelectionDiagnostic)) || appleCLTSelectionMissing(errors.New("exit status 2"), nil, []byte(appleCLTNoSelectionDiagnostic)) {
		t.Fatal("unproved native absence accepted")
	}
}

func TestAppleCLTVersionChangeWithoutNewInstallTimeIsNotCompletion(t *testing.T) {
	if appleCLTReceiptChanged("26.0@100", "27.0@100") {
		t.Fatal("version change reused an old installation timestamp")
	}
}
