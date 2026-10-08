package installer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The public command deliberately rejects a changed plan. A disposable native
// fixture records a second read-only preview for diagnosis, never approval or
// an automatic retry. It may differ from the observation at the failed instant.
func nativeNeovimPlanDiagnostic(t *testing.T, repository, home string, request Request, reviewed Plan) {
	t.Helper()
	logPlan := func(label string, plan Plan) {
		data, err := json.Marshal(plan)
		if err != nil {
			t.Logf("%s Neovim plan diagnostic: %v", label, err)
			return
		}
		if len(data) > 64<<10 {
			t.Logf("%s Neovim plan diagnostic exceeds 64 KiB: %d bytes", label, len(data))
			return
		}
		t.Logf("%s Neovim plan diagnostic: %s", label, data)
	}
	logPlan("reviewed", reviewed)
	request.ExpectedPlan = ""
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fresh, err := installedMachineRequest(ctx, filepath.Join(home, "dotfiles.exe"), repository, home, request)
	if err != nil {
		t.Logf("fresh read-only Neovim plan diagnostic failed: %v", err)
		return
	}
	logPlan("fresh read-only", fresh.Plan)
}

func TestNativeNeovimPlanDiagnosticOnlyPreviewsOnce(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, "fixture.go")
	// Replace only the external executable. The real test request/environment
	// boundary must clear approval before the diagnostic reaches this process.
	program := `package main
import("encoding/json";"os";"path/filepath")
func main(){
	var request struct{Mode string;ExpectedPlan string ` + "`json:\"expected_plan\"`" + `}
	if len(os.Args)!=2 || os.Args[1]!="machine" {os.Exit(11)}
	if json.NewDecoder(os.Stdin).Decode(&request)!=nil || request.ExpectedPlan!="" {os.Exit(12)}
	file,err:=os.OpenFile(filepath.Join(os.Getenv("HOME"),"preview.json"),os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
	if err!=nil {os.Exit(13)}
	if json.NewEncoder(file).Encode(request)!=nil || file.Close()!=nil {os.Exit(14)}
	os.Stdout.WriteString("{\"status\":\"preview\",\"plan\":{\"schema\":1,\"id\":\"fresh\"}}")
}`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(home, "dotfiles.exe"), source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process boundary fixture: %s: %v", output, err)
	}
	request := Request{Schema: 1, Mode: "apply", ExpectedPlan: "reviewed"}
	nativeNeovimPlanDiagnostic(t, home, home, request, Plan{Schema: 1, ID: "reviewed"})
	data, err := os.ReadFile(filepath.Join(home, "preview.json"))
	if err != nil {
		t.Fatal("diagnostic did not make its bounded preview", err)
	}
	var received struct {
		Mode         string
		ExpectedPlan string `json:"expected_plan"`
	}
	if err := Decode(data, &received); err != nil || received.Mode != "apply" || received.ExpectedPlan != "" || request.ExpectedPlan != "reviewed" {
		t.Fatal("diagnostic changed operation semantics or carried approval", received, err)
	}
}
