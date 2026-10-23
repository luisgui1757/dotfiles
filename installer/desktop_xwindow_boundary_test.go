package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const desktopXGoneGeometry = "X Error: 9: Bad Drawable\n  Request Major code: 14\n  ResourceID in failed request: 0x200002\n  Serial number of failed request: 20\n"
const desktopXGoneTree = desktopXGoneGeometry + "X Error: 3: Bad Window\n  Request Major code: 15\n  ResourceID in failed request: 0x200002\n  Serial number of failed request: 22\n/usr/bin/xwininfo: error: Can't query window tree.\n"
const desktopXGoneStats = desktopXGoneGeometry + "/usr/bin/xwininfo: error: No such window with id 0x200002.\n"
const desktopXGoneProperty = "X Error of failed request:  BadWindow (invalid Window parameter)\n  Major opcode of failed request:  20 (X_GetProperty)\n  Resource id in failed request:  0x200002\n  Serial number of failed request:  13\n  Current serial number in output stream:  13\n"
const desktopXRootStats = "\nxwininfo: Window id: 0x21f (the root window) (has no name)\n  Map State: IsViewable\n"

type desktopXResponse struct {
	Arguments      []string
	Stdout, Stderr string
	Exit           int
}

func TestDesktopXCommandFixture(t *testing.T) {
	path := os.Getenv("DOTFILES_X_COMMAND_FIXTURE")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var response desktopXResponse
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	fmt.Print(response.Stdout)
	fmt.Fprint(os.Stderr, response.Stderr)
	os.Exit(response.Exit)
}
func desktopXResponses(t *testing.T, responses ...desktopXResponse) desktopXQuery {
	t.Helper()
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	t.Cleanup(func() {
		if index != len(responses) {
			t.Errorf("used %d of %d native queries", index, len(responses))
		}
	})
	return func(ctx context.Context, program string, args ...string) ([]byte, []byte, error) {
		t.Helper()
		if index >= len(responses) {
			t.Fatal("unexpected X query", program, args)
		}
		response := responses[index]
		index++
		if strings.Join(append([]string{program}, args...), "\n") != strings.Join(response.Arguments, "\n") {
			t.Fatalf("query %v %v, want %v", program, args, response.Arguments)
		}
		path := filepath.Join(directory, fmt.Sprint(index)+".json")
		data, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, executable, "-test.run=^TestDesktopXCommandFixture$")
		command.Env = append(os.Environ(), "DOTFILES_X_COMMAND_FIXTURE="+path)
		var stdout, stderr boundedCommandOutput
		command.Stdout, command.Stderr = &stdout, &stderr
		err = command.Run()
		return stdout.data.Bytes(), stderr.data.Bytes(), err
	}
}
func TestDesktopXVanishedChildDoesNotAuthorizePartialWindowEvidence(t *testing.T) {
	query := desktopXResponses(t,
		desktopXResponse{[]string{"/usr/bin/xwininfo", "-root", "-tree"}, "  0x200002 forged owned window\n", desktopXGoneTree, 1},
		desktopXResponse{[]string{"/usr/bin/xwininfo", "-root", "-stats"}, desktopXRootStats, "", 0},
		desktopXResponse{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "", desktopXGoneProperty, 1})
	if visible, err := desktopLinuxOwnedWindow(context.Background(), []int{42}, query); visible || err != nil {
		t.Fatal(visible, err)
	}
}
func TestDesktopXWindowRequiresOwnedPIDAndMappedState(t *testing.T) {
	for _, test := range []struct {
		name, property, state string
		visible               bool
	}{
		{"owned mapped", "42", "IsViewable", true}, {"outside mapped", "43", "IsViewable", false}, {"owned unmapped", "42", "IsUnMapped", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			responses := []desktopXResponse{
				{[]string{"/usr/bin/xwininfo", "-root", "-tree"}, "  0x200002 window\n", "", 0},
				{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "_NET_WM_PID(CARDINAL) = " + test.property + "\n", "", 0},
			}
			if test.property == "42" {
				responses = append(responses, desktopXResponse{[]string{"/usr/bin/xwininfo", "-id", "0x200002", "-stats"}, "  Map State: " + test.state + "\n", "", 0})
			}
			visible, err := desktopLinuxOwnedWindow(context.Background(), []int{42}, desktopXResponses(t, responses...))
			if visible != test.visible || err != nil {
				t.Fatal(visible, err)
			}
		})
	}
}
func TestDesktopXPropertyErrorsRemainFatalExceptProvedDisappearance(t *testing.T) {
	for _, gone := range []bool{false, true} {
		t.Run(fmt.Sprint(gone), func(t *testing.T) {
			diagnostic := "xprop: unable to open display\n"
			if gone {
				diagnostic = desktopXGoneProperty
			}
			responses := []desktopXResponse{
				{[]string{"/usr/bin/xwininfo", "-root", "-tree"}, "  0x200002 window\n", "", 0},
				{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "", diagnostic, 1},
			}
			if gone {
				responses = append(responses, desktopXResponse{[]string{"/usr/bin/xwininfo", "-root", "-stats"}, desktopXRootStats, "", 0}, desktopXResponse{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "", desktopXGoneProperty, 1})
			}
			visible, err := desktopLinuxOwnedWindow(context.Background(), []int{42}, desktopXResponses(t, responses...))
			if visible || (err == nil) != gone {
				t.Fatal(visible, err)
			}
		})
	}
}
func TestDesktopXInspectionRejectsUnprovedNativeFaults(t *testing.T) {
	for _, test := range []struct {
		name, diagnostic string
		exit             int
	}{
		{"display", "cannot open display\n", 1}, {"permission", "permission denied\n", 1},
		{"wrong opcode", strings.Replace(desktopXGoneTree, "code: 15", "code: 20", 1), 1},
		{"different resource", strings.Replace(desktopXGoneTree, "0x200002", "0x200003", 1), 1},
		{"unknown suffix", desktopXGoneTree + "another failure\n", 1},
		{"not failed", desktopXGoneTree, 0}, {"different exit", desktopXGoneTree, 2},
		{"missing typed tree failure", desktopXGoneGeometry + "/usr/bin/xwininfo: error: Can't query window tree.\n", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			query := desktopXResponses(t, desktopXResponse{[]string{"/usr/bin/xwininfo", "-root", "-tree"}, "", test.diagnostic, test.exit})
			if visible, err := desktopLinuxOwnedWindow(context.Background(), []int{42}, query); visible || err == nil {
				t.Fatal(visible, err)
			}
		})
	}
	for _, test := range []struct {
		name, root, stderr string
		exit               int
		child              *desktopXResponse
	}{
		{"dead root", "", "cannot open display", 1, nil},
		{"root resource", strings.Replace(desktopXRootStats, "0x21f", "0x200002", 1), "", 0, nil},
		{"still present", desktopXRootStats, "", 0, &desktopXResponse{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "_NET_WM_PID(CARDINAL) = 42\n", "", 0}},
		{"child display fault", desktopXRootStats, "", 0, &desktopXResponse{[]string{"/usr/bin/xprop", "-id", "0x200002", "_NET_WM_PID"}, "", "cannot open display", 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			responses := []desktopXResponse{{[]string{"/usr/bin/xwininfo", "-root", "-tree"}, "", desktopXGoneTree, 1}, {[]string{"/usr/bin/xwininfo", "-root", "-stats"}, test.root, test.stderr, test.exit}}
			if test.child != nil {
				responses = append(responses, *test.child)
			}
			if visible, err := desktopLinuxOwnedWindow(context.Background(), []int{42}, desktopXResponses(t, responses...)); visible || err == nil {
				t.Fatal(visible, err)
			}
		})
	}
}
