package installer

import (
	"context"
	"strings"
	"testing"
)

func TestLinuxAPTResidualConfigFilesInstallationEvidence(t *testing.T) {
	operation := strings.Repeat("a", 64)
	history := "Start-Date: 2026-10-10  00:00:00\nCommandline: apt-get -o Dotfiles::Operation=" + operation + " install first\nInstall: first:amd64 (1.0)\nEnd-Date: 2026-10-10  00:00:01\n"
	for _, previous := range []string{"<none>", "1.0", "0.9"} {
		t.Run(previous, func(t *testing.T) {
			log := "2026-10-10 00:00:00 install first:amd64 " + previous + " 1.0\n2026-10-10 00:00:01 status installed first:amd64 1.0\n"
			introduced, configured, changed, err := linuxAPTAttemptEvidence([]byte(history), []byte(log), operation)
			if err != nil || introduced["first:amd64"].Version != "1.0" || configured["first:amd64"] != "1.0" || changed["first:amd64"] != "1.0" {
				t.Fatal("native install action was not accepted", introduced, configured, changed, err)
			}
			if got, err := aptInstallEvidence([]byte(history), []byte(log), operation); err != nil || len(got) != 1 {
				t.Fatal("successful command evidence disagreed", got, err)
			}
			failed := strings.Replace(history, "End-Date:", "Error: dpkg configuration failed\nEnd-Date:", 1)
			unpacked := strings.Replace(log, "status installed", "status unpacked", 1)
			introduced, configured, _, err = linuxAPTAttemptEvidence([]byte(failed), []byte(unpacked), operation)
			if err != nil || len(introduced) != 1 || configured["first:amd64"] != "" {
				t.Fatal("interrupted installation was not provisional", introduced, configured, err)
			}
			if _, err := aptInstallEvidence([]byte(failed), []byte(unpacked), operation); err == nil {
				t.Fatal("interrupted configuration granted completed evidence")
			}
			if _, _, _, err := linuxAPTAttemptEvidence([]byte(history), []byte(log+log), operation); err == nil {
				t.Fatal("duplicate native install was accepted")
			}
			introduced, _, _, err = linuxAPTAttemptEvidence([]byte(history), []byte(strings.Replace(log, " install ", " upgrade ", 1)), operation)
			if err != nil || len(introduced) != 0 {
				t.Fatal("upgrade incorrectly introduced a package", introduced, err)
			}
		})
	}
}

func TestLinuxAPTResidualConfigFilesRecovery(t *testing.T) {
	for _, mode := range []string{"successful", "configuration-failed", "completed-reply-lost"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			f := newLinuxAPTFixture(t)
			f.configFiles = map[string]string{"first:amd64": "0.9", "shared:amd64": "1.0"}
			r, receipt := aptFixtureResource("first"), aptFixtureReceipt("residual-"+mode)
			if got, err := f.driver.Observe(ctx, r, Receipt{}); err != nil || got.Present {
				t.Fatal("residual configuration was treated as installed payload", got, err)
			}
			f.approve()
			f.partial, f.lostReply = mode == "configuration-failed", mode == "completed-reply-lost"
			got, err := f.driver.Apply(ctx, r, Operation{Action: "install"}, receipt)
			if mode != "successful" {
				if err == nil {
					t.Fatal("fixture interruption did not occur")
				}
				state, ledgerErr := f.driver.ledger()
				if ledgerErr != nil || len(state.Roots) != 0 || len(state.Pool) != 0 {
					t.Fatal("unfinished operation acquired ownership", state, ledgerErr)
				}
				// Resume the existing schema-1 intent with a fresh driver. A saved
				// successful command must finalize without executing it again.
				driver := *f.driver
				f.driver = &driver
				f.approve()
				got, err = driver.ResumeResource(ctx, r, Operation{Action: "install"}, receipt)
			}
			wantRuns := 2
			if mode == "configuration-failed" {
				wantRuns = 4
			}
			if err != nil || !got.Healthy || got.CompletedOperation != receipt.OperationID || f.runs != wantRuns {
				t.Fatal("residual-state installation did not complete safely", got, err, f.runs)
			}
			state, err := f.driver.ledger()
			if err != nil || state.Roots[r.ID].CreatedBy != receipt.OperationID || state.Pool["shared:amd64"].CreatedBy != receipt.OperationID {
				t.Fatal("root or incidental ownership lost exact operation evidence", state, err)
			}
			f.approve()
			if got, err := f.driver.Remove(ctx, r, aptFixtureReceipt("remove-residual-"+mode)); err != nil || got.Present || len(f.installed) != 0 || f.configFiles["first:amd64"] != "1.0" {
				t.Fatal("removal did not preserve residual configuration", got, err)
			}
		})
	}
}
