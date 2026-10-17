package installer

import (
	"context"
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func darwinFontQueryCommand(t *testing.T) nativeCommand {
	t.Helper()
	var command nativeCommand
	d := FontDriver{Target: Context{OS: "darwin", Arch: "arm64"}}
	d.Query = func(_ context.Context, got nativeCommand) ([]byte, error) {
		command = got
		return json.Marshal(make([]fontNativeFace, len(hackFontFaces())))
	}
	if _, err := d.query(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	return command
}

func TestDarwinFontQuerySeparatesScriptAndNames(t *testing.T) {
	command := darwinFontQueryCommand(t)
	if command.Program != "/usr/bin/osascript" || len(command.Arguments) != 5 ||
		!reflect.DeepEqual(command.Arguments[:3], []string{"-l", "JavaScript", "-e"}) {
		t.Fatal("font query must pass its JSON names separately from the static script")
	}
	if command.Operation != "" || len(command.Input) != 0 {
		t.Fatal("font inspection acquired mutation authority or unexpected input")
	}
	var names []string
	if err := json.Unmarshal([]byte(command.Arguments[4]), &names); err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, face := range hackFontFaces() {
		want = append(want, face.PostScript)
		if strings.Contains(command.Arguments[3], face.PostScript) {
			t.Fatal("font name was embedded in executable source")
		}
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatal("native font inspection changed its exact ordered face names")
	}
}

// This is a read-only CoreText consumer. It never registers fonts or writes
// font/configuration files, and needs no hosted mutation opt-in.
func TestDarwinFontQueryNativeDataBoundary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("read-only native CoreText inspection requires macOS")
	}
	command := darwinFontQueryCommand(t)
	if len(command.Arguments) != 5 {
		t.Fatal("font query has no separate data argument")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cases := [][]string{
		{},
		{"font\"; throw new Error('font-codeql-sentinel'); //"},
		{"font'; throw new Error(\"font-codeql-sentinel\"); //", "font ü 日本語 \\ \n \u2028 \u2029"},
	}
	for _, names := range cases {
		data, err := json.Marshal(names)
		if err != nil {
			t.Fatal(err)
		}
		command.Arguments[4] = string(data)
		output, err := runIntegrationQuery(ctx, command)
		if err != nil {
			t.Fatal("font data was interpreted as executable source", err)
		}
		var result []fontNativeFace
		if err := Decode(output, &result); err != nil || len(result) != len(names) {
			t.Fatal("native query did not preserve the input face count", err)
		}
	}
	d := FontDriver{Target: Context{OS: "darwin", Arch: "arm64"}}
	result, err := d.query(ctx, nil)
	if err != nil || len(result) != len(hackFontFaces()) {
		t.Fatal("ordinary CoreText font inspection failed", err)
	}
}
