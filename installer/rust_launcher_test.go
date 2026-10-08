package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRustLauncherPreservesCallerArguments(t *testing.T) {
	directory := t.TempDir()
	for _, test := range []struct {
		name     string
		args     []string
		file     string
		explicit bool
	}{
		{"empty", nil, "", false},
		{"ordinary", []string{"--crate-name", "with space", "α", `a\"b`, ""}, "", false},
		{"separated", []string{"--sysroot", "caller root"}, "", true},
		{"joined", []string{"--sysroot=caller root"}, "", true},
		{"missing-value", []string{"--sysroot"}, "", true},
		{"terminator", []string{"--", "--sysroot=filename"}, "", false},
		{"argfile-crlf", []string{"@FILE"}, "--sysroot\r\ncaller root\r\n", true},
		{"argfile-eof", []string{"@FILE"}, "--sysroot=caller root", true},
		{"argfile-bare-cr", []string{"@FILE"}, "--sysroot\r", false},
		{"argfile-nonrecursive", []string{"@FILE"}, "@missing-file\n", false},
		{"argfile-terminator", []string{"@FILE", "--sysroot=filename"}, "--\n", false},
		{"argfile-unicode", []string{"@FILE"}, "α.rs\n--crate-name\nwith space\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string(nil), test.args...)
			if test.file != "" {
				path := filepath.Join(directory, test.name)
				if err := os.WriteFile(path, []byte(test.file), 0600); err != nil {
					t.Fatal(err)
				}
				args[0] = "@" + path
			}
			before := append([]string(nil), args...)
			got, err := rustLauncherArguments(args, `\\?\C:\managed root`)
			want := args
			if !test.explicit {
				want = append([]string{"--sysroot", `\\?\C:\managed root`}, args...)
			}
			if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(args, before) {
				t.Fatal(got, want, args, before, err)
			}
		})
	}
}

func TestRustLauncherRejectsUnreadableInvalidOrOversizedArgumentFiles(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"missing", nil}, {"invalid", []byte{255}}, {"large", bytes.Repeat([]byte{'x'}, 16<<20+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "args")
			if test.data != nil {
				if err := os.WriteFile(path, test.data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := rustLauncherArguments([]string{"@" + path}, "root"); err == nil {
				t.Fatal("invalid argument file accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "exact-bound")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, 16<<20), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := rustLauncherArguments([]string{"@" + path}, "root"); err != nil {
		t.Fatal("exact bound rejected", err)
	}
}

// A real child process records the received argv and streams. It can also wait
// for an interrupt inside a private Windows console; no internal mock replaces
// process creation, Go's Windows quoting, or exit-status propagation.
func buildRustLauncherFixture(t *testing.T, output string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	const program = `package main
import("encoding/json";"fmt";"io";"os";"os/signal")
func main(){
	if ready:=os.Getenv("DOTFILES_RUST_INTERRUPT_READY");ready!=""{
		interrupts:=make(chan os.Signal,1);signal.Notify(interrupts,os.Interrupt)
		if err:=os.WriteFile(ready,[]byte("ready"),0600);err!=nil{panic(err)}
		<-interrupts;fmt.Fprintln(os.Stdout,"child handled interrupt");os.Exit(23)
	}
	input,err:=io.ReadAll(os.Stdin);if err!=nil{panic(err)}
	if err:=json.NewEncoder(os.Stdout).Encode(struct{Args []string;Input string;Sysroot string}{os.Args[1:],string(input),os.Getenv("SYSROOT")});err!=nil{panic(err)}
	fmt.Fprintln(os.Stderr,"child diagnostic");os.Exit(23)
}`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, source)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build child: %s %v", result, err)
	}
}

func TestRustLauncherForwardsStreamsArgumentsAndExitStatus(t *testing.T) {
	directory := t.TempDir()
	launcher := filepath.Join(directory, "rustc.exe")
	buildRustLauncherFixture(t, filepath.Join(directory, rustOriginalCommand("rustc.exe")))
	args := []string{"α.rs", "", `with spaces`, `a\"b`, `trailing\`}
	var output, diagnostic bytes.Buffer
	code := runRustLauncher(launcher, "managed root", args, strings.NewReader("input α"), &output, &diagnostic)
	var received struct {
		Args  []string
		Input string
	}
	if err := json.Unmarshal(output.Bytes(), &received); err != nil {
		t.Fatal(output.String(), err)
	}
	if code != 23 || received.Input != "input α" || !reflect.DeepEqual(received.Args, append([]string{"--sysroot", "managed root"}, args...)) || diagnostic.String() != "child diagnostic\n" {
		t.Fatal(code, received, diagnostic.String())
	}
	diagnostic.Reset()
	if code := runRustLauncher(filepath.Join(directory, "rustdoc.exe"), "root", nil, nil, &output, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "Dotfiles Rust:") {
		t.Fatal(code, diagnostic.String())
	}
}

func TestRustLauncherPreservesClippyCargoWrapperProtocol(t *testing.T) {
	directory := t.TempDir()
	launcher := filepath.Join(directory, "clippy-driver.exe")
	buildRustLauncherFixture(t, filepath.Join(directory, rustOriginalCommand("clippy-driver.exe")))
	for _, supplied := range []bool{false, true} {
		t.Run(map[bool]string{false: "managed", true: "caller"}[supplied], func(t *testing.T) {
			t.Setenv("SYSROOT", "caller root")
			if !supplied {
				if err := os.Unsetenv("SYSROOT"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{`C:\with space\rustc.exe`, "--crate-name", "example", "source.rs"}
			var output, diagnostic bytes.Buffer
			code := runRustLauncher(launcher, `\\?\managed root`, args, nil, &output, &diagnostic)
			var received struct {
				Args    []string
				Sysroot string
			}
			if err := json.Unmarshal(output.Bytes(), &received); err != nil {
				t.Fatal(output.String(), err)
			}
			wantRoot := `\\?\managed root`
			if supplied {
				wantRoot = "caller root"
			}
			if code != 23 || !reflect.DeepEqual(received.Args, args) || received.Sysroot != wantRoot {
				t.Fatal(code, received, args, wantRoot)
			}
		})
	}
}
