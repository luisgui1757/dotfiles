package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLegacyNixReenabledLinkSurvivesSecondInterruptedDeletion(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	h.installLinuxDaemonIntegrations()
	unit := "nix-daemon.service"
	storeUnit := "/nix/store/aaa-nix-2.34.0/lib/systemd/system/" + unit
	h.file(storeUnit, "[Unit]\n")
	// Lose a worker reply after deleting only a different store package. The
	// current profile's unit remains available for systemctl reenable.
	run := h.d.Run
	h.d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if !slices.Contains(command.Arguments, legacyNixDeletionScript) {
			return run(ctx, command)
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		if err := saveDocument(filepath.Join(h.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
			return nil, err
		}
		if err := os.RemoveAll(h.path("/nix/store/bbb-bash-5.2")); err != nil {
			return nil, err
		}
		return nil, errors.New("fixture worker was lost after deleting another package")
	}
	interruptedLegacyNixDeletion(t, h)
	h.d.Run, h.die = run, nil
	link := "/etc/systemd/system/multi-user.target.wants/" + unit
	if err := os.Remove(h.path(link)); err != nil {
		t.Fatal(err)
	}
	h.link(link, h.path(storeUnit))
	intent := h.approved(h.inspect())
	for !intent.Scope.Steps[intent.Next].Irreversible {
		if err := h.execute(intent, false); err != nil {
			t.Fatal(err)
		}
	}
	command, err := h.d.stepCommand(*intent, intent.Next)
	if err != nil {
		t.Fatal(err)
	}
	h.die = func(dispatched nativeCommand) bool { return dispatched.Operation == command.Operation }
	if err := h.execute(intent, false); err == nil {
		t.Fatal("second interrupted deletion reported success")
	}
	if _, err := os.Lstat(h.path(link)); !os.IsNotExist(err) {
		t.Fatal("recovery deleted the resolution source before its activation link", err)
	}
	h.runPrefixes(h.approved(h.inspect()), nil)
	if after := h.inspect(); after.Blocked != "" || len(after.Steps) != 0 {
		t.Fatal("double-interruption recovery did not converge", after)
	}
}

func TestLegacyUnprovedStoreLinksGiveActionableRefusal(t *testing.T) {
	for _, shape := range []string{"upgraded", "dangling"} {
		t.Run(shape, func(t *testing.T) {
			h := newLegacyNixHost(t, "linux")
			h.installLinuxStore()
			h.installLinuxDaemonIntegrations()
			unit := "nix-daemon.service"
			old := "/nix/store/old-nix/lib/systemd/system/" + unit
			h.file(old, "[Unit]\n")
			h.file("/nix/store/aaa-nix-2.34.0/lib/systemd/system/"+unit, "[Unit]\n")
			link := "/etc/systemd/system/multi-user.target.wants/" + unit
			if err := os.Remove(h.path(link)); err != nil {
				t.Fatal(err)
			}
			h.link(link, h.path(old))
			if shape == "dangling" {
				if err := os.RemoveAll(h.path("/nix")); err != nil {
					t.Fatal(err)
				}
			}
			before, err := legacyFile(h.path(link))
			if err != nil {
				t.Fatal(err)
			}
			_, vendorErr := h.d.determinateUnitFiles(legacyDeterminateReceipt{Init: "Systemd", Units: []string{unit}})
			_, upstreamReason, err := h.d.linuxNixUnitLinks()
			if err != nil || vendorErr == nil {
				t.Fatal("unproved link did not refuse", vendorErr, err)
			}
			for _, reason := range []string{vendorErr.Error(), upstreamReason} {
				if !strings.Contains(reason, h.path(link)) || !strings.Contains(reason, "sudo rm -- "+h.path(link)) || !strings.Contains(reason, "confirming") {
					t.Fatal("refusal lacks ownership-qualified recovery advice", reason)
				}
			}
			after, err := legacyFile(h.path(link))
			if err != nil || before != after {
				t.Fatal("preview changed an unproved link", err)
			}
		})
	}
}

func TestLegacyNonLinkActivationPathHasPreservationExplanation(t *testing.T) {
	for _, directory := range []bool{false, true} {
		d, root := legacyStepFixture(t)
		d.systemRoot = filepath.Join(root, "system")
		path := d.sys("/etc/systemd/system/multi-user.target.wants/nix-daemon.service")
		if directory {
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			writeConfigFixture(t, path, "personal\n")
		}
		_, err := d.determinateUnitFiles(legacyDeterminateReceipt{Init: "Systemd", Units: []string{"nix-daemon.service"}})
		if err == nil || !strings.Contains(err.Error(), "preserve and inspect it manually") {
			t.Fatal("non-link refusal lacks preservation explanation", err)
		}
	}
}

func TestLegacyPrimaryLinksNeverSuggestActivationRemoval(t *testing.T) {
	for _, path := range []string{"/etc/systemd/system/nix-daemon.service", "/etc/systemd/system/nix-daemon.socket", "/etc/tmpfiles.d/nix-daemon.conf"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			h := newLegacyNixHost(t, "linux")
			h.installLinuxDaemonIntegrations()
			if err := os.Remove(h.path(path)); err != nil {
				t.Fatal(err)
			}
			h.link(path, h.path("/nix/store/foreign/"+filepath.Base(path)))
			before, err := legacyFile(h.path(path))
			if err != nil {
				t.Fatal(err)
			}
			scope := h.inspect()
			if !strings.Contains(scope.Blocked, h.path(path)) || !strings.Contains(scope.Blocked, "preserve and inspect it manually") || strings.Contains(scope.Blocked, "sudo rm") || strings.Contains(scope.Blocked, "activation link") {
				t.Fatal("primary integration received activation-link removal advice", scope.Blocked)
			}
			after, err := legacyFile(h.path(path))
			if err != nil || before != after {
				t.Fatal("preview changed the primary link", err)
			}
		})
	}
}

func TestLegacyDeterminateUnitLinksRequireSavedProof(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux systemd remainder planning")
	}
	for _, unit := range []string{"nix-daemon.socket", "nix-daemon.service"} {
		for _, layout := range []string{"literal", "upstream"} {
			for _, proof := range []string{"recorded", "historical", "absent"} {
				t.Run(unit+"/"+layout+"/"+proof, func(t *testing.T) {
					d, root := legacyStepFixture(t)
					d.systemRoot = filepath.Join(root, "system")
					d.Platform.Context = Context{OS: "linux", Arch: "amd64"}
					receipt := legacyDeterminateLinuxReceipt
					if layout == "upstream" {
						receipt = legacyDeterminateLinuxUpstreamReceipt
					}
					r, err := parseLegacyDeterminateReceipt([]byte(receipt))
					if err != nil {
						t.Fatal(err)
					}
					target := "sockets.target.wants"
					if strings.HasSuffix(unit, ".service") {
						target = "multi-user.target.wants"
					}
					path := d.sys("/etc/systemd/system/" + target + "/" + unit)
					linkTarget := d.sys("/etc/systemd/system/" + unit)
					if layout == "upstream" {
						linkTarget = d.sys("/nix/store/aaa-nix/lib/systemd/system/" + unit)
						writeConfigFixture(t, linkTarget, "[Unit]\n")
						profile := d.sys("/nix/var/nix/profiles/default")
						if err := os.MkdirAll(filepath.Dir(profile), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(d.sys("/nix/store/aaa-nix"), profile); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					link := func(target string) {
						t.Helper()
						if err := os.Symlink(target, path); err != nil {
							t.Fatal(err)
						}
					}
					if proof != "absent" {
						link(linkTarget)
					}
					files, err := d.determinateUnitFiles(r)
					if err != nil {
						t.Fatal(err)
					}
					if proof == "historical" {
						files = nil // Old records cannot acquire new link-removal authority.
					}
					evidence := legacyDeterminateEvidence{Schema: 1, Receipt: []byte(receipt), Installer: strings.Repeat("a", 64), Files: files}
					step, err := d.determinateEvidenceStep(evidence)
					if err != nil {
						t.Fatal(err)
					}
					if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
						t.Fatal(err)
					}
					if proof == "absent" {
						link(linkTarget)
					}
					// The vendor removes the store and profile before follow-up cleanup.
					if layout == "upstream" {
						if err := os.RemoveAll(d.sys("/nix")); err != nil {
							t.Fatal(err)
						}
					}
					var scope legacyRetirementScope
					if err := d.planDeterminateLeftovers(context.Background(), &scope); err != nil {
						t.Fatal(err)
					}
					if proof != "recorded" {
						if !strings.Contains(scope.Blocked, "not an unchanged file") || len(scope.Steps) != 0 {
							t.Fatal("unproved link was offered for retirement", scope)
						}
						return
					}
					index := slices.IndexFunc(scope.Steps, func(step legacyRetirementStep) bool { return step.Kind == "remove" && step.Path == path })
					if scope.Blocked != "" || index < 0 {
						t.Fatal("recorded activation link was omitted", scope)
					}
					if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "2", scope.Steps[index]), 0, false); err != nil {
						t.Fatal(err)
					}
					link(d.sys("/personal.socket"))
					scope = legacyRetirementScope{}
					if err := d.planDeterminateLeftovers(context.Background(), &scope); err != nil || !strings.Contains(scope.Blocked, "not an unchanged file") {
						t.Fatal("later personal link was not protected", scope, err)
					}
					if _, err := d.determinateUnitFiles(r); err == nil {
						t.Fatal("an unrelated activation target passed a fresh vendor preview")
					}
				})
			}
		}
	}
}

func TestLegacyNixResolvedUnitLinkSurvivesInterruptedDeletion(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	h.installLinuxDaemonIntegrations()
	unit := "nix-daemon.service"
	storeUnit := "/nix/store/aaa-nix-2.34.0/lib/systemd/system/" + unit
	h.file(storeUnit, "[Unit]\n")
	link := "/etc/systemd/system/multi-user.target.wants/" + unit
	if err := os.Remove(h.path(link)); err != nil {
		t.Fatal(err)
	}
	h.link(link, h.path(storeUnit))
	// The ordinary developer cannot chown the private fixture's /nix to
	// root, so enter the daemon planner directly; native CI proves discovery.
	var scope legacyRetirementScope
	var err error
	scope.ReplacementState, err = h.d.replacementState()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.d.planUpstreamLinux(context.Background(), &scope, true); err != nil || scope.Blocked != "" {
		t.Fatal("systemd's resolved link blocked preview", scope.Blocked, err)
	}
	intent := h.approved(scope)
	for !intent.Scope.Steps[intent.Next].Irreversible {
		if err := h.execute(intent, false); err != nil {
			t.Fatal(err)
		}
	}
	command, err := h.d.stepCommand(*intent, intent.Next)
	if err != nil {
		t.Fatal(err)
	}
	h.die = func(dispatched nativeCommand) bool { return dispatched.Operation == command.Operation }
	if err := h.execute(intent, false); err == nil {
		t.Fatal("killed deletion reported success")
	}
	if _, err := os.Lstat(h.path(link)); !os.IsNotExist(err) {
		t.Fatal("store-derived activation evidence must retire before deleting its source", err)
	}
	h.runPrefixes(h.approved(h.inspect()), nil)
	if after := h.inspect(); after.Blocked != "" || len(after.Steps) != 0 {
		t.Fatal("resolved-link recovery did not converge", after)
	}
}

func TestLegacyResolvedUnitLinkRequiresItsOwnRegularStoreFile(t *testing.T) {
	for _, shape := range []string{"other-store-file", "outside-store", "directory", "missing", "cycle"} {
		t.Run(shape, func(t *testing.T) {
			d, root := legacyStepFixture(t)
			d.systemRoot = filepath.Join(root, "system")
			unit := "nix-daemon.service"
			source := d.sys("/etc/systemd/system/" + unit)
			target := d.sys("/nix/store/aaa-nix/lib/systemd/system/" + unit)
			if shape == "outside-store" {
				target = d.sys("/personal/" + unit)
			}
			if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
				t.Fatal(err)
			}
			if shape == "directory" {
				if err := os.MkdirAll(target, 0700); err != nil {
					t.Fatal(err)
				}
			} else if shape != "missing" && shape != "cycle" {
				writeConfigFixture(t, target, "[Unit]\n")
			}
			linkTarget := target
			if shape == "other-store-file" {
				linkTarget = d.sys("/nix/store/bbb-personal/lib/systemd/system/" + unit)
				writeConfigFixture(t, linkTarget, "[Unit]\n")
			}
			if shape == "cycle" {
				target = source
			}
			if err := os.Symlink(target, source); err != nil {
				t.Fatal(err)
			}
			link := d.sys("/etc/systemd/system/multi-user.target.wants/" + unit)
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(linkTarget, link); err != nil {
				t.Fatal(err)
			}
			before, err := legacyFile(link)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.determinateUnitFiles(legacyDeterminateReceipt{Init: "Systemd", Units: []string{unit}}); err == nil {
				t.Fatal("unproved activation link passed preview", shape)
			}
			after, err := legacyFile(link)
			if err != nil || before != after {
				t.Fatal("preview changed the link", err)
			}
		})
	}
}
