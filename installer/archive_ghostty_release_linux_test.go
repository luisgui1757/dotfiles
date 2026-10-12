package installer

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Debian 13 builds of the same release, pinned before 2026-10-21. They import
// gtk_css_section_get_bytes (GTK 4.16), which Ubuntu 24.04's GTK 4.14 lacks.
var ghosttyTrixieBuilds = map[string][2]string{
	"amd64": {"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_amd64_trixie.deb", "9fda8e418d7a7f58149ba3ba823a255d6b80f8bb5431b3bd7e912ff597715b2e"},
	"arm64": {"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_arm64_trixie.deb", "73f384e62c419d7a7809d686bf579fea5e23f52742b34f70c74d6adf0e72f8ab"},
}

// GTK and libadwaita keep binary compatibility only forward, and Ghostty's
// health check verifies payload bytes rather than symbol resolution. Load the
// prepared payload against the oldest supported Ubuntu's own libraries with
// every symbol bound at startup, as a desktop launch would eventually need.
func TestNativeGhosttyLoadsOnOldestSupportedUbuntu(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_GHOSTTY_RELEASES") != "1" {
		t.Skip("set DOTFILES_TEST_GHOSTTY_RELEASES=1 on a disposable Docker host")
	}
	image := os.Getenv("GHOSTTY_OLDEST_UBUNTU_IMAGE")
	if !regexp.MustCompile(`^ubuntu:24\.04@sha256:[0-9a-f]{64}$`).MatchString(image) {
		t.Fatal("GHOSTTY_OLDEST_UBUNTU_IMAGE must be the digest-pinned Ubuntu 24.04 image", image)
	}
	docker, err := exec.LookPath("docker")
	if err != nil || !filepath.IsAbs(docker) {
		t.Fatal("the opted-in check requires Docker", err)
	}
	dpkgDeb, err := exec.LookPath("dpkg-deb")
	if err != nil || !filepath.IsAbs(dpkgDeb) {
		t.Fatal("the opted-in check requires the observed absolute dpkg-deb", err)
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	current, ok := pins["tool.ghostty"]
	previous := current
	previous.URL, previous.SHA256 = ghosttyTrixieBuilds[runtime.GOARCH][0], ghosttyTrixieBuilds[runtime.GOARCH][1]
	if !ok || !current.GhosttyLibraries || previous.URL == "" || current.SHA256 == previous.SHA256 {
		t.Fatal("Ghostty must use a pinned build other than the rejected Debian 13 build", current)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ghostty, _ := catalog.Resource("tool.ghostty")
	packages := []string{}
	for _, id := range ghostty.PlatformRequires["linux"] {
		library, _ := catalog.Resource(id)
		if binding := library.Bindings["linux"]; strings.HasPrefix(id, "library.") && binding.Provider == "apt" {
			packages = append(packages, binding.Package)
		}
	}
	if len(packages) < 9 {
		t.Fatal("Ghostty's catalog libraries are incomplete", packages)
	}
	payloads := map[string]string{}
	for name, pin := range map[string]ArchivePin{"current": current, "previous": previous} {
		c, d, _ := archiveController(t)
		d.Pins["tool.shared"], d.Client, d.DpkgDeb = pin, nil, dpkgDeb
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
		if payloads[name], err = d.PayloadPath("tool.shared"); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	run := func(args ...string) (string, error) {
		var output bytes.Buffer
		command := exec.CommandContext(ctx, docker, args...)
		command.Stdout, command.Stderr = &output, &output
		err := command.Run()
		return output.String(), err
	}
	container := "dotfiles-ghostty-" + strings.ToLower(rand.Text())
	if output, err := run("run", "--detach", "--name", container,
		"--volume", payloads["current"]+":/current:ro", "--volume", payloads["previous"]+":/previous:ro",
		image, "sleep", "900"); err != nil {
		t.Fatal(output, err)
	}
	t.Cleanup(func() { _, _ = exec.Command(docker, "rm", "--force", container).CombinedOutput() })
	install := "apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends " + strings.Join(packages, " ") + " >/dev/null"
	if output, err := run("exec", container, "sh", "-euc", install); err != nil {
		t.Fatal("Ubuntu 24.04 cannot install Ghostty's catalog libraries", output, err)
	}
	load := func(name string) (string, error) {
		return run("exec", "--env", "LD_BIND_NOW=1", "--env", "HOME=/tmp", container, "/"+name+"/usr/bin/ghostty", "--version")
	}
	if output, err := load("current"); err != nil || !strings.Contains(output, current.Version) {
		t.Fatal("pinned Ghostty does not load against Ubuntu 24.04 libraries", output, err)
	}
	// The failure a desktop launch hit on Ubuntu 24.04 (2026-10-21) must reproduce,
	// or this check could not have caught it.
	if output, err := load("previous"); err == nil || !strings.Contains(output, "undefined symbol: gtk_css_section_get_bytes") {
		t.Fatal("the rejected Debian 13 build did not reproduce its Ubuntu 24.04 symbol failure", output, err)
	}
}
