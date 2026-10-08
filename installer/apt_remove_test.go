package installer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestAPTRefusedRemovalRestoresSelectionsWithoutTouchingConsumers(t *testing.T) {
	for _, scenario := range []string{"refused", "partial", "changed-version", "held", "restore-fails"} {
		t.Run(scenario, func(t *testing.T) {
			packages := []string{"first:amd64", "shared:amd64"}
			state := map[string]aptPackageStatus{}
			for _, name := range append(slices.Clone(packages), "outside:amd64", "orphan:amd64") {
				state[name] = aptPackageStatus{name, "1.0", "install", "installed", "ok"}
			}
			if scenario == "held" {
				pkg := state["shared:amd64"]
				pkg.Want = "hold"
				state[pkg.Name] = pkg
			}
			removes, restores := 0, 0
			run := func(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
				switch {
				case program == "dpkg-query":
					if privileged {
						t.Fatal("read-only package query requested elevation")
					}
					var output strings.Builder
					for _, name := range []string{"first:amd64", "shared:amd64", "outside:amd64", "orphan:amd64"} {
						if pkg, ok := state[name]; ok {
							fmt.Fprintf(&output, "%s\t%s\t%s\t%s\t%s\n", pkg.Name, pkg.Version, pkg.Want, pkg.Status, pkg.Error)
						}
					}
					return []byte(output.String()), nil
				case program == "dpkg" && privileged && slices.Equal(args, append([]string{"--remove"}, packages...)):
					removes++
					for _, name := range packages {
						pkg := state[name]
						pkg.Want = "deinstall"
						state[name] = pkg
					}
					if scenario == "partial" {
						delete(state, packages[0])
					}
					if scenario == "changed-version" {
						pkg := state[packages[1]]
						pkg.Version = "2.0"
						state[pkg.Name] = pkg
					}
					return []byte("dpkg: dependency problems prevent removal"), errors.New("exit status 1")
				case program == "dpkg" && privileged && slices.Equal(args, []string{"--set-selections"}):
					restores++
					if scenario == "restore-fails" {
						return nil, errors.New("package database lock is held")
					}
					for _, line := range strings.Split(strings.TrimSpace(string(input)), "\n") {
						parts := strings.Fields(line)
						if len(parts) != 2 || !slices.Contains(packages, parts[0]) || parts[1] != "install" {
							t.Fatal("selection restoration exceeded exact removal candidates", line)
						}
						pkg, exists := state[parts[0]]
						if !exists {
							t.Fatal("restoration recreated a successfully removed package")
						}
						pkg.Want = parts[1]
						state[pkg.Name] = pkg
					}
					return nil, nil
				default:
					t.Fatalf("unexpected process boundary: %s %v", program, args)
					return nil, errors.New("unexpected command")
				}
			}
			_, err := removeAPTPackages(context.Background(), run, packages)
			if err == nil {
				t.Fatal("refused native removal reported success")
			}
			for _, name := range []string{"outside:amd64", "orphan:amd64"} {
				if state[name] != (aptPackageStatus{name, "1.0", "install", "installed", "ok"}) {
					t.Fatal("outside native state changed")
				}
			}
			switch scenario {
			case "refused", "partial":
				if restores != 1 || state[packages[1]].Want != "install" {
					t.Fatal("refused candidate retained an unsafe deinstall selection", state)
				}
			case "held":
				if removes != 0 || restores != 0 || state[packages[1]].Want != "hold" {
					t.Fatal("held package was mutated")
				}
			case "changed-version":
				if restores != 0 || !strings.Contains(err.Error(), "changed version") {
					t.Fatal("outside version change was overwritten", err)
				}
			case "restore-fails":
				if !strings.Contains(err.Error(), "restore refused package selections") {
					t.Fatal("failed selection restoration was hidden", err)
				}
			}
		})
	}
}
