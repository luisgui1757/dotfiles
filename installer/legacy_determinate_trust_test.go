package installer

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyDeterminateReleaseAuthenticatesWritableBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ownership")
	}
	path := filepath.Join(t.TempDir(), "nix-installer")
	data := "untrusted executable bytes must match the reviewed release"
	writeConfigFixture(t, path, data)
	if err := os.Chmod(path, 0777); err != nil {
		t.Fatal(err)
	}
	pin := legacyDeterminateRelease{Asset: "nix-installer-aarch64-darwin", Size: int64(len(data)), SHA256: legacyHash([]byte(data))}
	proof, problem, err := legacyDeterminateBinaryTrust(path, "", "3.23.2", &pin)
	if err != nil || problem != "" || proof.Trust != "release:v3.23.2/"+pin.Asset || proof.SHA256 != pin.SHA256 || int(proof.Owner) != os.Geteuid() {
		t.Fatal(proof, problem, err)
	}
	if _, problem, err := legacyDeterminateBinaryTrust(path, "", "3.23.2", nil); err != nil || !strings.Contains(problem, "no reviewed release digest") {
		t.Fatal("unreviewed writable executable accepted", problem, err)
	}
	writeConfigFixture(t, path, data+"changed")
	if _, problem, err := legacyDeterminateBinaryTrust(path, "", "3.23.2", &pin); err != nil || !strings.Contains(problem, "reviewed release digest") {
		t.Fatal("modified binary accepted", problem, err)
	}
	if _, problem, err := legacyDeterminateBinaryTrust(path, filepath.Dir(path), "3.23.2", &pin); os.Geteuid() != 0 && (err != nil || !strings.Contains(problem, "root-owned")) {
		t.Fatal("release pin exempted a replaceable parent", problem, err)
	}
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, problem, err := legacyDeterminateBinaryTrust(link, "", "3.23.2", &pin); err != nil || !strings.Contains(problem, "plain file") {
		t.Fatal("release pin exempted a symlink", problem, err)
	}
}

func TestLegacyDeterminateReleasePinsMatchReviewedTargets(t *testing.T) {
	for version, pin := range legacyDeterminateMacReleases {
		parts, ok := legacyDeterminateVersionParts(version)
		if !ok || legacyVersionBefore(legacyDeterminateNewest, parts) || pin.Asset != "nix-installer-aarch64-darwin" || pin.Size <= 0 || pin.Size > 512<<20 || len(pin.SHA256) != 64 || pin.SHA256 != strings.ToLower(pin.SHA256) {
			t.Fatal("invalid reviewed release pin", version, pin)
		}
		if _, err := hex.DecodeString(pin.SHA256); err != nil {
			t.Fatal("invalid release digest", version, err)
		}
	}
	if pin := legacyDeterminateMacReleases["3.23.2"]; pin.SHA256 != "136f40a09ac56e3638530d57099d748c1023677911eb5afd45a4ba438986b648" || pin.Size != 61498576 {
		t.Fatal("native fixture no longer authenticates the independently verified vendor release", pin)
	}
}

func TestLegacyDeterminateReleasePinsReproduceOfficialAssets(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LEGACY_DETERMINATE_PINS") != "1" {
		t.Skip("set DOTFILES_TEST_LEGACY_DETERMINATE_PINS=1 to query official metadata and hash older assets")
	}
	if output, err := exec.Command("python3", "../scripts/generate-determinate-release-pins.py", "--check").CombinedOutput(); err != nil {
		t.Fatalf("official pin reproduction failed: %v\n%s", err, output)
	}
}

func TestLegacyDeterminateRootOwnedGateKeepsHistoricalShape(t *testing.T) {
	const historical = `{"receipt":"receipt-hash","installer":"binary-hash","binary":"100000:1:2:0"}`
	var observed legacyDeterminateObserved
	if err := json.Unmarshal([]byte(historical), &observed); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(observed)
	if err != nil || string(data) != historical {
		t.Fatal("protected root-owned approval identity changed", string(data), err)
	}
	observed.InstallerOwner, observed.InstallerTrust = 501, "release:v3.23.2/nix-installer-aarch64-darwin"
	data, err = json.Marshal(observed)
	if err != nil || string(data) == historical || !strings.Contains(string(data), `"installer_owner":501`) || !strings.Contains(string(data), `"installer_trust":"release:`) {
		t.Fatal("release authentication reused root-only approval", string(data), err)
	}
}

// legacyDeterminateExecutable proves a root-owned regular file that no other
// account can replace. It is bound by inode and SHA-256.
func legacyDeterminateExecutable(path, parent string) (identity, sum, problem string, err error) {
	proof, problem, err := legacyDeterminateBinaryTrust(path, parent, "", nil)
	return proof.Identity, proof.SHA256, problem, err
}
