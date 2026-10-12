package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The actual fixed recipe runs in the real native subprocess worker with only
// Apple's three command boundaries replaced by private filesystem fixtures.
// sudo is removed from this fixture command; no real native package tool runs.
func TestAppleCLTScriptNativeWorkerBoundaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Apple recipe requires POSIX processes")
	}
	for _, scenario := range []string{"fresh", "external-marker", "no-label", "list-fails", "receipt-query-fails", "historical-receipt", "receipt-unchanged-after-install", "receipt-drift", "racing-receipt", "selection-query-fails", "racing-selection", "install-fails", "no-receipt", "missing-tool", "switch-fails", "racing-clt", "foreign-selection", "sentinel-symlink", "replaced-marker"} {
		t.Run(scenario, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			root := filepath.Dir(f.d.clt)
			writeFixture := func(name, body string) string {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, []byte("#!/bin/bash\nset -eu\n"+body), 0755); err != nil {
					t.Fatal(err)
				}
				return path
			}
			softwareupdate := writeFixture("softwareupdate", `
[[ -z "${DOTFILES_CLT_PRIVATE_TEST_SECRET+x}" ]] || exit 98
case "$1" in
	-l)
		[[ -f "$FIXTURE_ROOT/placeholder" ]] || exit 98
		if [[ "$FIXTURE_MODE" == list-fails ]]; then exit 2; fi
		if [[ "$FIXTURE_MODE" == no-label ]]; then printf 'No new software available.\n'; exit 0; fi
		if [[ "$FIXTURE_MODE" == racing-clt ]]; then /bin/mkdir "$FIXTURE_ROOT/CLT"; fi
		: > "$FIXTURE_ROOT/listed"
		printf '%s\n' 'Software Update Tool' '* Label: Command Line Tools for Xcode-16.9' ' Title: Command Line Tools for Xcode, Version: 16.9,' '* Label: unrelated OS upgrade' ' Title: macOS' '* Label: Command Line Tools for Xcode-16.10' ' Title: Command Line Tools for Xcode, Version: 16.10,'
		;;
	-i)
		[[ "$#" == 2 && "$2" == 'Command Line Tools for Xcode-16.10' ]] || exit 98
		printf '%s\n' "$2" > "$FIXTURE_ROOT/installed-label"
		if [[ "$FIXTURE_MODE" == install-fails ]]; then exit 3; fi
		/bin/mkdir -p "$FIXTURE_ROOT/CLT/usr/bin"
		for name in clang clang++ make; do
			if [[ "$FIXTURE_MODE" == missing-tool && "$name" == make ]]; then continue; fi
			printf '#!/bin/sh\nexit 0\n' > "$FIXTURE_ROOT/CLT/usr/bin/$name"
			/bin/chmod 755 "$FIXTURE_ROOT/CLT/usr/bin/$name"
		done
		if [[ "$FIXTURE_MODE" != no-receipt ]]; then : > "$FIXTURE_ROOT/receipt"; fi
		if [[ "$FIXTURE_MODE" == replaced-marker ]]; then
			printf external-marker > "$FIXTURE_ROOT/replacement"
			/bin/mv "$FIXTURE_ROOT/replacement" "$FIXTURE_ROOT/placeholder"
		fi
		;;
	*) exit 99 ;;
esac
`)
			xcodeSelect := writeFixture("xcode-select", `
case "$1" in
	--print-path)
		if [[ "$FIXTURE_MODE" == selection-query-fails ]]; then printf "unexpected native error\n" >&2; exit 2; fi
		if [[ "$FIXTURE_MODE" == foreign-selection || ( "$FIXTURE_MODE" == racing-selection && -f "$FIXTURE_ROOT/listed" ) ]]; then printf '/private/foreign/Xcode.app/Contents/Developer\n'; exit 0; fi
		if [[ -f "$FIXTURE_ROOT/selected" ]]; then /bin/cat "$FIXTURE_ROOT/selected"; else printf '%s\n' '`+appleCLTNoSelectionText+`' >&2; exit 2; fi
		;;
	--switch)
		[[ "$#" == 2 && "$2" == "$FIXTURE_ROOT/CLT" ]] || exit 98
		if [[ "$FIXTURE_MODE" == switch-fails ]]; then exit 4; fi
		printf '%s\n' "$2" > "$FIXTURE_ROOT/selected"
		;;
	*) exit 99 ;;
esac
`)
			pkgutil := writeFixture("pkgutil", `
case "$1" in
	--pkgs=*) exit 1 ;; # pkgutil returns 1 for a regex that has no matches.
	--pkgs)
		if [[ "$FIXTURE_MODE" == receipt-query-fails && -f "$FIXTURE_ROOT/listed" ]]; then exit 7; fi
		printf 'unrelated.package\ncom.apple.pkg.CLTools_Executables.extra\n'
		if [[ -f "$FIXTURE_ROOT/receipt" || "$FIXTURE_MODE" == historical-receipt || "$FIXTURE_MODE" == receipt-unchanged-after-install || "$FIXTURE_MODE" == receipt-drift || ( "$FIXTURE_MODE" == racing-receipt && -f "$FIXTURE_ROOT/listed" ) ]]; then printf 'com.apple.pkg.CLTools_Executables\n'; fi
		;;
	--pkg-info=com.apple.pkg.CLTools_Executables)
		timestamp=100
		if [[ -f "$FIXTURE_ROOT/receipt" && "$FIXTURE_MODE" != receipt-unchanged-after-install ]]; then timestamp=200; fi
		if [[ "$FIXTURE_MODE" == receipt-drift && -f "$FIXTURE_ROOT/listed" ]]; then timestamp=150; fi
		printf 'package-id: com.apple.pkg.CLTools_Executables\nversion: 16.10\nvolume: /\nlocation: /\ninstall-time: %s\n' "$timestamp"
		;;
	*) exit 99 ;;
esac
`)
			baseline := "none"
			if scenario == "historical-receipt" || scenario == "receipt-unchanged-after-install" || scenario == "receipt-drift" {
				baseline = "16.10@100"
			}
			command, err := f.d.command(bootstrapReceipt("a").OperationID, 0, "install", baseline, "", "")
			if err != nil {
				t.Fatal(err)
			}
			script := strings.NewReplacer("/usr/sbin/softwareupdate", softwareupdate, "/usr/bin/xcode-select", xcodeSelect, "/usr/sbin/pkgutil", pkgutil).Replace(string(command.Input))
			if runtime.GOOS != "darwin" {
				stat := writeFixture("stat", `exec /usr/bin/stat -c '%d:%i' "$3"`)
				script = strings.ReplaceAll(script, "/usr/bin/stat", stat)
			}
			command.Input = []byte(script)
			command.Program = "/usr/bin/env"
			command.Arguments = append([]string{"-i", "FIXTURE_ROOT=" + root, "FIXTURE_MODE=" + scenario}, command.Arguments[3:]...)
			t.Setenv("DOTFILES_CLT_PRIVATE_TEST_SECRET", "public-fixture-not-for-inheritance")
			switch scenario {
			case "external-marker":
				if err := os.WriteFile(f.d.placeholder, []byte("external-marker"), 0600); err != nil {
					t.Fatal(err)
				}
			case "sentinel-symlink":
				if err := os.WriteFile(filepath.Join(root, "external-file"), []byte("external-marker"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "external-file"), f.d.placeholder); err != nil {
					t.Fatal(err)
				}
			}
			worker, err := workerFixture(f.d.WorkerDirectory)
			if err != nil {
				t.Fatal(err)
			}
			output, runErr := worker.run(context.Background(), command)
			if err := worker.close(); err != nil {
				t.Fatal(err)
			}
			success := scenario == "fresh" || scenario == "external-marker" || scenario == "historical-receipt"
			if (runErr == nil) != success {
				t.Fatalf("unexpected outcome: %v %s", runErr, output)
			}
			reply, err := f.d.result(command)
			if err != nil {
				t.Fatal(err)
			}
			if appleCLTMarker(reply, "INSTALLED", bootstrapReceipt("a").OperationID) != (success || scenario == "switch-fails" || scenario == "replaced-marker") {
				t.Fatal("incorrect installation attribution", string(output))
			}
			if scenario == "receipt-query-fails" || scenario == "receipt-drift" || scenario == "selection-query-fails" || scenario == "racing-receipt" || scenario == "racing-clt" || scenario == "foreign-selection" || scenario == "racing-selection" {
				if _, err := os.Lstat(filepath.Join(root, "installed-label")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("unproved absence reached native installation", err)
				}
			}
			if scenario == "external-marker" || scenario == "sentinel-symlink" || scenario == "replaced-marker" {
				data, err := os.ReadFile(f.d.placeholder)
				if err != nil || string(data) != "external-marker" {
					t.Fatal("external sentinel changed", err)
				}
			} else if _, err := os.Lstat(f.d.placeholder); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("owned sentinel was not cleaned", err)
			}
			if scenario == "switch-fails" {
				// Operation-attributed package success permits only final selection;
				// the finish mode does not rerun list/install or need its sentinel.
				finish, err := f.d.command(bootstrapReceipt("a").OperationID, 1, "finish", baseline, "", "16.10@200")
				if err != nil {
					t.Fatal(err)
				}
				finish.Program, finish.Input = command.Program, command.Input
				finish.Arguments = append([]string{"-i", "FIXTURE_ROOT=" + root, "FIXTURE_MODE=fresh"}, finish.Arguments[3:]...)
				worker, err := workerFixture(f.d.WorkerDirectory)
				if err != nil {
					t.Fatal(err)
				}
				if output, err := worker.run(context.Background(), finish); err != nil {
					t.Fatal(err, string(output))
				}
				if err := worker.close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
