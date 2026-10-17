package installer

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The Linux option uses a private fontconfig file, home, data and cache directory;
// it never updates the caller's font configuration or font cache. macOS/Windows
// activation is permitted only in explicitly opted-in disposable hosted jobs.
func TestNativeFontInstallConsumeRemove(t *testing.T) {
	privateLinux := runtime.GOOS == "linux" && os.Getenv("DOTFILES_NATIVE_FONT_PRIVATE_LINUX") == "1"
	hosted := os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted" && os.Getenv("DOTFILES_NATIVE_DESKTOP_FONTS") == "1"
	if !privateLinux && !hosted {
		t.Skip("requires isolated Linux fontconfig fixture or explicit disposable hosted native font opt-in")
	}
	target := NativePlatform{Context: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Libc: "glibc"}
	pins, err := DefaultArchivePins(target)
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := pins["font.hack"]
	if !ok {
		t.Fatal("missing canonical font pin")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "native fonts space ü")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		folders = ConfigFolders{Home: filepath.Join(root, "home"), Data: filepath.Join(root, "data")}
		config := filepath.Join(root, "fonts.conf")
		xml := `<?xml version="1.0"?><!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd"><fontconfig><dir>` + filepath.Join(folders.Data, "fonts") + `</dir><cachedir>` + filepath.Join(root, "cache") + `</cachedir></fontconfig>`
		if err := os.WriteFile(config, []byte(xml), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("FONTCONFIG_FILE", config)
		t.Setenv("FONTCONFIG_PATH", root)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
		t.Setenv("HOME", folders.Home)
	}
	state := filepath.Join(root, "state")
	archives := &ArchiveDriver{Directory: filepath.Join(state, "packages"), Pins: map[string]ArchivePin{"font.hack": pin}}
	if archive := os.Getenv("DOTFILES_NATIVE_FONT_ARCHIVE"); archive != "" {
		archives.Client = &http.Client{Transport: fontFixtureTransport(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != pin.URL {
				return nil, errors.New("unexpected font fixture URL")
			}
			file, err := os.Open(archive)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Body: file, Header: http.Header{}, Request: request}, nil
		})}
	}

	ac := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "font", Name: "Font archive", Capability: true, Requires: []string{"font.hack"}}, {ID: "font.hack", Name: "Hack", Action: "archive"}}}, Context: target.Context, Source: "native-font-archive", Home: root, StatePath: filepath.Join(root, "archive-state.json"), Driver: archives}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	session := &nativeSession{Directory: filepath.Join(state, "native", "worker")}
	session.Start = func(context.Context) (*nativeWorkerClient, error) { return workerFixture(session.Directory) }
	release, err := session.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	d := &FontDriver{Target: target.Context, Folders: folders, Directory: state, Archives: archives, Run: session.run, FontMatch: "/usr/bin/fc-match", FontCache: "/usr/bin/fc-cache"}
	if runtime.GOOS == "windows" {
		d.PowerShell, err = exec.LookPath("powershell.exe")
		if err != nil {
			t.Fatal(err)
		}
	}
	r := Resource{ID: "tool.font", Name: "Hack Nerd Font", Action: "font"}
	_, _, paths, _, err := d.configuration(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("fresh prerequisite not established: destination already exists", path, err)
		}
	}
	before, err := d.Observe(ctx, r, Receipt{})
	if err != nil || before.Unknown || before.Present || before.Inventory != emptyFontInventory() {
		t.Fatalf("fresh font absence not established: %+v %v", before, err)
	}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "font", Name: "Font", Capability: true, Requires: []string{"tool.font"}}, r}}, Context: target.Context, Source: "native-font", Home: folders.Home, StatePath: filepath.Join(state, "state.json"), Driver: d}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"font"}})
	probe, err := d.query(ctx, paths)
	if err != nil || !d.nativeOwned(probe, paths) {
		t.Fatal("native engine did not consume each exact font face", probe, err)
	}
	nativeFontRender(t, ctx, d, filepath.Join(root, "Nerd glyph.png"))
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("owned native font file remained", path, err)
		}
	}
	after, err := d.query(ctx, paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, face := range after {
		if face.Registration != "" {
			t.Fatal("owned native registration remained", face)
		}
	}
	dispatchApproved(t, ac, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}

func nativeFontRender(t *testing.T, ctx context.Context, d *FontDriver, output string) {
	t.Helper()
	var command *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		renderer, err := exec.LookPath("pango-view")
		if err != nil {
			t.Fatal("native font acceptance requires pango-view", err)
		}
		command = exec.CommandContext(ctx, renderer, "--no-display", "--font=Hack Nerd Font Mono 24", "--text=\uf120", "--output="+output)
		command.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	case "darwin":
		script := `ObjC.import('AppKit'); var font=$.NSFont.fontWithNameSize($('HackNFM-Regular'),32);if(!font)throw Error('Missing exact NSFont');var image=$.NSImage.alloc.initWithSize($.NSMakeSize(160,64));image.lockFocus;$.NSColor.whiteColor.set;$.NSRectFill($.NSMakeRect(0,0,160,64));var attrs=$.NSMutableDictionary.alloc.init;attrs.setObjectForKey(font,$.NSFontAttributeName);attrs.setObjectForKey($.NSColor.blackColor,$.NSForegroundColorAttributeName);$(String.fromCharCode(0xf120)).drawAtPointWithAttributes($.NSMakePoint(8,8),attrs);image.unlockFocus;var rep=$.NSBitmapImageRep.imageRepWithData(image.TIFFRepresentation);if(!rep.representationUsingTypeProperties($.NSPNGFileType,$.NSDictionary.dictionary).writeToFileAtomically($(` + jsString(output) + `),true))throw Error('Cannot save native glyph rendering');`
		command = exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script)
	case "windows":
		script := `$ErrorActionPreference='Stop';Add-Type -AssemblyName System.Drawing;$image=[Drawing.Bitmap]::new(160,64);$graphics=[Drawing.Graphics]::FromImage($image);$font=[Drawing.Font]::new('Hack Nerd Font Mono',32);try{$graphics.Clear([Drawing.Color]::White);$graphics.DrawString([string][char]0xf120,$font,[Drawing.Brushes]::Black,8,8);$image.Save(` + desktopPSQuote(output) + `,[Drawing.Imaging.ImageFormat]::Png)}finally{$font.Dispose();$graphics.Dispose();$image.Dispose()}`
		command = exec.CommandContext(ctx, d.PowerShell, windowsVendorArguments(script)...)
	}
	data, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native font rendering: %v\n%s", err, data)
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() < 100 {
		t.Fatal("native font consumer did not render the Nerd glyph", err)
	}
}
func jsString(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", `\r`, "\n", `\n`).Replace(value) + `"`
}

type fontFixtureTransport func(*http.Request) (*http.Response, error)

func (f fontFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
