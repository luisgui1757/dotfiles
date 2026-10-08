package installer

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type archiveCrashFixture struct {
	Home, Directory, StatePath, Source, Cut string
	Catalog                                 Catalog
	Target                                  Context
	Pin                                     ArchivePin
	Certificate                             []byte
}

type archiveCrashDriver struct {
	*ArchiveDriver
	marker string
}

func (d archiveCrashDriver) stop(o Observation, err error) (Observation, error) {
	if err != nil {
		return o, err
	}
	if err := os.WriteFile(d.marker, []byte("published"), 0600); err != nil {
		return o, err
	}
	// The parent kills this actual process before the controller can record
	// completion. A bounded timer also prevents a stranded test child.
	<-time.After(45 * time.Second)
	return o, errors.New("crash fixture was not terminated by its parent")
}
func (d archiveCrashDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.stop(d.ArchiveDriver.Apply(ctx, r, op, receipt))
}
func (d archiveCrashDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.stop(d.ArchiveDriver.Remove(ctx, r, receipt))
}

func TestArchiveCrashChild(t *testing.T) {
	file := os.Getenv("DOTFILES_ARCHIVE_CRASH_FIXTURE")
	if file == "" {
		return
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var fixture archiveCrashFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(fixture.Certificate)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificate)
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	d := &ArchiveDriver{Directory: fixture.Directory, Pins: map[string]ArchivePin{"tool.shared": fixture.Pin}, Client: client}
	c := Controller{Catalog: &fixture.Catalog, Context: fixture.Target, Source: fixture.Source, Home: fixture.Home, StatePath: fixture.StatePath, Driver: d}
	if fixture.Cut != "download" {
		c.Driver = archiveCrashDriver{d, filepath.Join(fixture.Home, "published")}
	}
	selected := []string{"first"}
	if fixture.Cut == "remove" {
		selected = []string{}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: selected})
	t.Fatal("parent did not interrupt archive publication")
}

func TestPrivateArchiveRecoversAfterActualProcessDeath(t *testing.T) {
	for _, cut := range []string{"download", "publication", "remove"} {
		t.Run(cut, func(t *testing.T) {
			c, d, _ := archiveController(t)
			data := archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "verified package", Mode: 0755}})
			requests := atomic.Int32{}
			started := make(chan struct{}, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := requests.Add(1)
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				if cut == "download" && count == 1 {
					if _, err := w.Write(data[:len(data)/2]); err != nil {
						t.Errorf("start download: %v", err)
						return
					}
					w.(http.Flusher).Flush()
					started <- struct{}{}
					<-r.Context().Done()
					return
				}
				if _, err := w.Write(data); err != nil {
					t.Errorf("complete download: %v", err)
				}
			}))
			t.Cleanup(server.Close)
			hash := sha256.Sum256(data)
			pin := d.Pins["tool.shared"]
			pin.URL, pin.SHA256 = server.URL+"/tool", hex.EncodeToString(hash[:])
			d.Pins["tool.shared"], d.Client = pin, server.Client()
			if cut == "remove" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			}
			fixture := archiveCrashFixture{c.Home, d.Directory, c.StatePath, c.Source, cut, *c.Catalog, c.Context, pin, server.Certificate().Raw}
			encoded, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			fixturePath := filepath.Join(c.Home, "crash-fixture.json")
			if err := os.WriteFile(fixturePath, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(binary, "-test.run=^TestArchiveCrashChild$", "-test.timeout=60s")
			command.Env = append(os.Environ(), "DOTFILES_ARCHIVE_CRASH_FIXTURE="+fixturePath)
			log, err := os.Create(filepath.Join(c.Home, "child.log"))
			if err != nil {
				t.Fatal(err)
			}
			command.Stdout, command.Stderr = log, log
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				if !waited {
					if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
						t.Errorf("stop child: %v", err)
					}
					if err := command.Wait(); err == nil {
						t.Error("uninterrupted child exited successfully")
					}
				}
				if err := log.Close(); err != nil {
					t.Error(err)
				}
			})
			deadline := time.After(15 * time.Second)
			if cut == "download" {
				select {
				case <-started:
				case <-deadline:
					t.Fatal("child did not start its download")
				}
			} else {
				for {
					if _, err := os.Stat(filepath.Join(c.Home, "published")); err == nil {
						break
					} else if !errors.Is(err, os.ErrNotExist) {
						t.Fatal(err)
					}
					select {
					case <-deadline:
						output, readErr := os.ReadFile(log.Name())
						t.Fatalf("child did not publish: %s (%v)", output, readErr)
					case <-time.After(10 * time.Millisecond):
					}
				}
			}
			if err := command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if err := command.Wait(); err == nil {
				t.Fatal("child survived process kill")
			}
			waited = true
			request := Request{Schema: 1, Mode: "apply", Retry: true}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = ""
			if result.Status == "needs-action" {
				result = dispatchApproved(t, c, request)
			}
			if result.Status != "ready" {
				t.Fatal("recovery did not converge", result)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil {
				t.Fatal("recovery left unfinished intent", state.Transaction, err)
			}
			if cut != "remove" {
				if state.Receipts["tool.shared"].Ownership != "created" {
					t.Fatal("recovery lost proved ownership")
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			}
			wantRequests := int32(1)
			if cut == "download" {
				wantRequests = 2
			}
			if requests.Load() != wantRequests {
				t.Fatal("recovery rebuilt a published package", requests.Load())
			}
		})
	}
}
