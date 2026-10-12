package installer

import (
	"reflect"
	"strings"
	"testing"
)

func TestAPTArchitectureIndependentInstallationUsesDPKGIdentity(t *testing.T) {
	operation := strings.Repeat("b", 64)
	for _, architecture := range []string{"amd64", "arm64"} {
		t.Run(architecture, func(t *testing.T) {
			history := "Start-Date: 2026-10-10  06:41:40\nCommandline: apt-get -o Dotfiles::Operation=" + operation + " install fixture\nInstall: fixture:" + architecture + " (1.0)\nEnd-Date: 2026-10-10  06:41:40\n"
			log := "2026-10-10 06:41:40 install fixture:all <none> 1.0\n2026-10-10 06:41:40 status installed fixture:all 1.0\n"
			got, err := aptInstallEvidence([]byte(history), []byte(log), operation)
			if err != nil || !reflect.DeepEqual(got, []aptInstalledPackage{{"fixture:all", "1.0", false}}) {
				t.Fatal("native architecture-independent transaction was not reconciled", got, err)
			}
			for _, invalid := range []string{
				strings.ReplaceAll(log, ":all", ":ppc64el"),
				log + "2026-10-10 06:41:40 install fixture:" + architecture + " <none> 2.0\n",
				strings.ReplaceAll(log, "status installed", "status unpacked"),
			} {
				if _, err := aptInstallEvidence([]byte(history), []byte(invalid), operation); err == nil {
					t.Fatal("foreign, ambiguous or unfinished dpkg identity was accepted", invalid)
				}
			}
		})
	}
}

func TestAPTInstallationEvidenceRequiresActualOperationCompletion(t *testing.T) {
	operation := strings.Repeat("a", 64)
	history := "\nStart-Date: 2026-10-10  01:02:03\nCommandline: apt-get -o Dotfiles::Operation=" + operation + " install first\nInstall: first:amd64 (1.2-3), libshared:arm64 (2:3.4-5, automatic)\nEnd-Date: 2026-10-10  01:02:04\n"
	log := "2026-10-10 01:02:03 startup archives unpack\n" +
		"2026-10-10 01:02:03 install first:amd64 <none> 1.2-3\n" +
		"2026-10-10 01:02:03 status unpacked first:amd64 1.2-3\n" +
		"2026-10-10 01:02:03 install libshared:arm64 <none> 2:3.4-5\n" +
		"2026-10-10 01:02:04 status installed first:amd64 1.2-3\n" +
		"2026-10-10 01:02:04 status installed libshared:arm64 2:3.4-5\n"
	want := []aptInstalledPackage{{"first:amd64", "1.2-3", false}, {"libshared:arm64", "2:3.4-5", true}}
	got, err := aptInstallEvidence([]byte(history), []byte(log), operation)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	for _, mutation := range []string{"planned-only", "failed-with-end-date", "unfinished", "different-operation", "two-transactions", "different-version", "unconfigured", "truncated", "unrelated-removal", "empty-item", "duplicate-package", "duplicate-dpkg-install", "changed-after-configuration"} {
		t.Run(mutation, func(t *testing.T) {
			h, l := history, log
			switch mutation {
			case "planned-only":
				l = ""
			case "failed-with-end-date":
				h = strings.Replace(h, "End-Date:", "Error: Sub-process dpkg returned an error code (1)\nEnd-Date:", 1)
			case "unfinished":
				h = h[:strings.Index(h, "End-Date:")]
			case "different-operation":
				h = strings.ReplaceAll(h, operation, strings.Repeat("b", 64))
			case "two-transactions":
				h += history
			case "different-version":
				l = strings.ReplaceAll(l, "1.2-3", "1.2-4")
			case "unconfigured":
				l = strings.ReplaceAll(l, "status installed", "status unpacked")
			case "truncated":
				l += "2026-10-10 01:02:04 status installed first:amd64"
			case "unrelated-removal":
				h = strings.Replace(h, "End-Date:", "Remove: outside:amd64 (1.0)\nEnd-Date:", 1)
			case "empty-item":
				h = strings.Replace(h, ", automatic)\n", ", automatic), \n", 1)
			case "duplicate-package":
				h = strings.Replace(h, "libshared:arm64", "first:amd64", 1)
			case "duplicate-dpkg-install":
				l += "2026-10-10 01:02:04 install first:amd64 <none> 1.2-3\n"
			case "changed-after-configuration":
				l += "2026-10-10 01:02:04 status unpacked first:amd64 1.2-3\n"
			}
			if _, err := aptInstallEvidence([]byte(h), []byte(l), operation); err == nil {
				t.Fatal("unproved installation acquired ownership evidence")
			}
		})
	}
}
