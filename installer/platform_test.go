package installer

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestUnsupportedTargetFailsBeforeSelectingPackagesOrConstructingApplication(t *testing.T) {
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	for _, target := range []Context{{OS: "darwin", Arch: "amd64"}, {OS: "windows", Arch: "arm64"}, {OS: "linux", Arch: "386"}} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			platform := NativePlatform{Context: target}
			if _, err := DefaultArchivePins(platform); err == nil {
				t.Error("selected packages for an unsupported target")
			}
			if _, err := NewNativeController(repository, "unsupported-target", filepath.Join(home, "state"), platform, ConfigFolders{Home: home, Config: filepath.Join(home, ".config")}); err == nil {
				t.Error("constructed an application for an unsupported target")
			}
		})
	}
}

func nativePlatform(t *testing.T) NativePlatform {
	t.Helper()
	target, err := DiscoverPlatform()
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestNativePlatformDiscovery(t *testing.T) {
	target := nativePlatform(t)
	if target.OS != runtime.GOOS || target.Arch != runtime.GOARCH {
		t.Fatal("native platform differs", target)
	}
	if target.OS == "linux" && (target.Distro == "" || target.Libc == "") {
		t.Fatal("native runner must have an observed distribution and libc", target)
	}
	t.Logf("observed native target: %+v", target)
}

func TestLinuxSupportRejectsUnselectedDistributions(t *testing.T) {
	for _, data := range []string{
		"ID=fedora\n", "ID=arch\n", "ID=opensuse-tumbleweed\n", "ID=alpine\n",
		"ID=linuxmint\nID_LIKE=\"ubuntu debian\"\n", "ID=unknown\nID_LIKE=debian\n",
		"ID=ubuntu\nID=fedora\n", "NAME=Linux\n",
	} {
		if _, _, err := linuxDistribution([]byte(data)); err == nil {
			t.Errorf("unsupported distribution accepted: %q", data)
		}
	}
}

func TestLinuxDistributionUsesExactSupportedIdentity(t *testing.T) {
	for _, fixture := range []struct{ data, id, family string }{
		{"ID=ubuntu\nID_LIKE=debian\n", "ubuntu", "apt"},
		{"ID='debian'\n", "debian", "apt"},
		{"ID=fedora\nID=ubuntu\n", "ubuntu", "apt"},
		{"ID=ubuntu\nID_LIKE=\"fedora\"\n", "ubuntu", "apt"},
		{"ID=debian\nNAME=\"$(do not execute)\"\n", "debian", "apt"},
	} {
		id, family, err := linuxDistribution([]byte(fixture.data))
		if err != nil || id != fixture.id || family != fixture.family {
			t.Fatal(fixture, id, family, err)
		}
	}
	for _, invalid := range []string{"ID=\"ubuntu", "ID=ubuntu debian", "ID=$(uname)", "ID=", "ID_LIKE=\"debian;touch /tmp/file\""} {
		if _, _, err := linuxDistribution([]byte(invalid)); err == nil {
			t.Fatal("accepted malformed policy input", invalid)
		}
	}
}

func TestArchiveSelectionRequiresObservedLibc(t *testing.T) {
	for _, libc := range []string{"", "musl", "glibc"} {
		pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "linux", Arch: "amd64"}, Libc: libc})
		if err != nil {
			t.Fatal(err)
		}
		_, nvim := pins["tool.nvim"]
		_, starship := pins["tool.starship"]
		if nvim != (libc == "glibc") || !starship {
			t.Fatal("archive selection guessed libc or removed a static tool", libc, nvim, starship)
		}
	}
	for _, fixture := range []struct{ interpreter, libc string }{
		{"/lib64/ld-linux-x86-64.so.2", "glibc"},
		{"/lib/ld-linux-aarch64.so.1", "glibc"},
		{"/lib/ld-musl-x86_64.so.1", "musl"},
		{"/lib/ld-musl-aarch64.so.1", "musl"},
		{"/custom/loader", ""},
	} {
		if actual := interpreterLibc(fixture.interpreter); actual != fixture.libc {
			t.Fatal("wrong libc for system ELF interpreter", fixture, actual)
		}
	}
}

func TestPlatformFactsDoNotChangePersistedContext(t *testing.T) {
	var target Context
	if err := Decode([]byte(`{"os":"linux","arch":"amd64"}`), &target); err != nil {
		t.Fatal(err)
	}
	platform := NativePlatform{Context: target, Distro: "ubuntu", Family: "apt", Libc: "glibc"}
	if platform.Context != target {
		t.Fatal("provider observations changed the target identity", platform)
	}
}
