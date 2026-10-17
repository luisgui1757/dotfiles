package installer

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestLinuxAPTNativeProtectionRetainsItsDependencyClosure(t *testing.T) {
	for _, field := range []string{"essential", "protected"} {
		t.Run(field, func(t *testing.T) {
			flags := "yes\t"
			if field == "protected" {
				flags = "\tyes"
			}
			rows := "root:amd64\t1\tinstall\tinstalled\tok\troot\t\tservice\t\t\t\n" +
				"service:amd64\t1\tinstall\tinstalled\tok\tservice\t\tshared\t\t" + flags + "\n" +
				"shared:amd64\t1\tinstall\tinstalled\tok\tshared\t\t\t\t\t\n"
			installed, err := parseLinuxAPTInventory([]byte(rows), []byte("service\nshared\n"))
			if err != nil {
				t.Fatal(err)
			}
			got, protected, err := nativeRemovalCandidates([]string{"root:amd64"}, []string{"service:amd64", "shared:amd64"}, nil, installed)
			if err != nil || !slices.Equal(got, []string{"root:amd64"}) || !slices.Equal(protected["shared:amd64"], []string{"service:amd64"}) {
				t.Fatal("native protected dependency was offered for removal", got, protected, err)
			}
		})
	}
}

func TestLinuxAPTLegacySavedRemovalCannotBypassCurrentNativeProtection(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	r := aptFixtureResource("first")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("initial")); err != nil {
		t.Fatal(err)
	}
	// This saved inventory is the legacy shape: no native protection field.
	before, err := json.Marshal(f.installed)
	if err != nil || strings.Contains(string(before), "Protected") {
		t.Fatal("fixture must exercise the legacy saved package shape", string(before), err)
	}
	intent := linuxAPTIntent{Schema: 1, Resource: r.ID, Operation: aptFixtureReceipt("remove").OperationID, Package: "first", Action: "remove", Remove: []string{"first:amd64", "shared:amd64"}}
	if err := Decode(before, &intent.Before); err != nil {
		t.Fatal(err)
	}
	command, err := f.driver.command(intent, 0, "remove", nil, []string{"--remove", "first:amd64", "shared:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	query := f.driver.Query
	f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		data, err := query(ctx, privileged, program, input, args...)
		if program == "dpkg-query" && len(args) == 2 && args[1] == "-f="+linuxAPTInventoryFormat {
			lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			for i, line := range lines {
				if strings.HasPrefix(line, "shared:amd64\t") {
					fields := strings.Split(line, "\t")
					// Overwrite only current native observation, not saved intent.
					if len(fields) == 11 {
						fields[10] = "yes"
					} else {
						fields = append(fields, "", "yes")
					}
					lines[i] = strings.Join(fields, "\t")
				}
			}
			data = []byte(strings.Join(lines, "\n") + "\n")
		}
		return data, err
	}
	if err := f.driver.validateDispatch(context.Background(), intent, linuxAPTCommand{Kind: "remove", Command: command}); err == nil || !strings.Contains(err.Error(), "changed before exact removal") {
		t.Fatal("legacy removal did not recheck current native protection", err)
	}
}

func TestLinuxAPTLegacyPendingInstallCompletesWithCurrentProtection(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.installed["essential:amd64"] = nativePackage{Name: "essential:amd64", Version: "1", Source: "essential", Healthy: true}
	f.lostReply = true
	f.approve()
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("legacy-protection")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture did not lose completed reply")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil || intent.Before["essential:amd64"].Protected {
		t.Fatal("fixture must start with the missing legacy protection flag", err)
	}
	pkg := f.installed["essential:amd64"]
	pkg.Protected = true
	f.installed[pkg.Name] = pkg
	calls := f.runs
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || !got.Healthy || got.CompletedOperation != receipt.OperationID || f.runs != calls {
		t.Fatal("legacy completed worker could not publish ownership", got, err)
	}
}
