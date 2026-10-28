package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type transferLog struct {
	mu          sync.Mutex
	requests    []*http.Request
	connections atomic.Int32
}

func (l *transferLog) record(r *http.Request) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, r.Clone(context.Background()))
	return len(l.requests)
}

func (l *transferLog) all() []*http.Request {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]*http.Request(nil), l.requests...)
}

func transferServer(t *testing.T, http2 bool, respond func(attempt int, w http.ResponseWriter, r *http.Request)) (*httptest.Server, *transferLog) {
	t.Helper()
	log := &transferLog{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(log.record(r), w, r)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			log.connections.Add(1)
		}
	}
	server.EnableHTTP2 = http2
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, log
}

func transferData(size int) []byte {
	data := make([]byte, size)
	random := rand.New(rand.NewPCG(1, 2))
	for i := range data {
		data[i] = byte(random.Uint32())
	}
	return data
}

// serveRanges implements Range and If-Range through the standard library.
func serveRanges(w http.ResponseWriter, r *http.Request, data []byte) {
	w.Header().Set("ETag", `"fixture"`)
	http.ServeContent(w, r, "tool", time.Unix(1, 0), bytes.NewReader(data))
}

// cutAfter ends the connection after limit body bytes, like a reset stream.
type cutAfter struct {
	http.ResponseWriter
	left int
}

func (w *cutAfter) Write(data []byte) (int, error) {
	if len(data) >= w.left {
		_, _ = w.ResponseWriter.Write(data[:w.left])
		w.ResponseWriter.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}
	n, err := w.ResponseWriter.Write(data)
	w.left -= n
	return n, err
}

// stallAfter sends a full-length response header and size bytes, then waits
// for the client to abandon the request.
func stallAfter(w http.ResponseWriter, r *http.Request, data []byte, size int, ranges bool) {
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("ETag", `"fixture"`)
	if ranges {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data[:size])
	w.(http.Flusher).Flush()
	select {
	case <-r.Context().Done():
	case <-time.After(10 * time.Second):
	}
}

var testArchiveTransfer = archiveTransfer{limit: 10 * time.Second, window: 100 * time.Millisecond, minimum: 1 << 20, attempts: 3, backoff: 10 * time.Millisecond}

func fetchForTest(t *testing.T, policy archiveTransfer, client *http.Client, url string, display io.Writer) ([]byte, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "download-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	digest := sha256.New()
	n, err := policy.fetch(t.Context(), client, url, file, digest, display, "tool")
	if err != nil {
		return nil, err
	}
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(written)) != n || hex.EncodeToString(digest.Sum(nil)) != sha256Hex(written) {
		t.Fatalf("returned length %d and digest disagree with %d written bytes", n, len(written))
	}
	return written, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// A reset in the middle of the body (HTTP/2 INTERNAL_ERROR on hosted macOS,
// 2026-10-28) used to fail the download; it now resumes with the kept bytes.
func TestArchiveDownloadResumesAfterConnectionLoss(t *testing.T) {
	data := archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: string(transferData(256 << 10)), Mode: 0755}})
	half := len(data) / 2
	server, log := transferServer(t, false, func(attempt int, w http.ResponseWriter, r *http.Request) {
		if attempt == 1 {
			serveRanges(&cutAfter{ResponseWriter: w, left: half}, r, data)
			return
		}
		serveRanges(w, r, data)
	})
	pin := ArchivePin{Version: "1.0", URL: server.URL + "/tool.tar.gz", SHA256: sha256Hex(data), Format: "tar.gz", Commands: map[string]string{"tool": "tool"}, BinDirs: []string{"."}}
	destination := filepath.Join(t.TempDir(), "payload")
	if err := downloadArchive(context.Background(), server.Client(), pin, destination); err != nil {
		t.Fatal(err)
	}
	requests := log.all()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want the cut transfer and one resume", len(requests))
	}
	if got := requests[1].Header.Get("Range"); got != "bytes="+strconv.Itoa(half)+"-" {
		t.Fatalf("resume Range = %q", got)
	}
	if got := requests[1].Header.Get("If-Range"); got != `"fixture"` {
		t.Fatalf("resume If-Range = %q", got)
	}
	if _, err := os.Stat(filepath.Join(destination, "tool")); err != nil {
		t.Fatal(err)
	}
}

// A slow HTTP/2 attempt is abandoned and resumed on a new connection: reusing
// the pooled connection would stay on the slow path.
func TestArchiveTransferReconnectsAnAttemptBelowTheMinimum(t *testing.T) {
	data := transferData(256 << 10)
	server, log := transferServer(t, true, func(attempt int, w http.ResponseWriter, r *http.Request) {
		if attempt == 1 {
			stallAfter(w, r, data, 1024, true)
			return
		}
		serveRanges(w, r, data)
	})
	display := &bytes.Buffer{}
	written, err := fetchForTest(t, testArchiveTransfer, server.Client(), server.URL+"/tool", display)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, data) {
		t.Fatal("resumed bytes differ from the served file")
	}
	requests := log.all()
	if len(requests) != 2 || requests[0].ProtoMajor != 2 || requests[1].Header.Get("Range") != "bytes=1024-" {
		t.Fatalf("requests = %d, first protocol %d, resume Range %q", len(requests), requests[0].ProtoMajor, requests[len(requests)-1].Header.Get("Range"))
	}
	if got := log.connections.Load(); got != 2 {
		t.Fatalf("connections = %d, want a new connection for the resume", got)
	}
	if !strings.Contains(display.String(), "archive transfer too slow") || !strings.Contains(display.String(), "reconnecting and resuming at 1.0 KiB") {
		t.Fatalf("reconnection was not displayed: %q", display.String())
	}
}

// Without byte ranges a slow attempt would lose its bytes, so only a complete
// stall abandons it.
func TestArchiveTransferKeepsSlowProgressWithoutRanges(t *testing.T) {
	data := transferData(64 << 10)
	server, log := transferServer(t, false, func(_ int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		for chunk := range slices.Chunk(data, 8<<10) {
			_, _ = w.Write(chunk)
			w.(http.Flusher).Flush()
			time.Sleep(40 * time.Millisecond)
		}
	})
	written, err := fetchForTest(t, testArchiveTransfer, server.Client(), server.URL+"/tool", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, data) || len(log.all()) != 1 {
		t.Fatalf("requests = %d, want one uninterrupted slow transfer", len(log.all()))
	}
}

// A server that advertises ranges but answers a resume with the whole file is
// treated as having no ranges for the rest of the transfer. Abandoning its slow
// attempts would discard every restart's progress.
func TestArchiveTransferKeepsSlowProgressAfterAnIgnoredResume(t *testing.T) {
	data := transferData(64 << 10)
	server, log := transferServer(t, false, func(_ int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"fixture"`)
		w.WriteHeader(http.StatusOK)
		for chunk := range slices.Chunk(data, 8<<10) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(40 * time.Millisecond)
		}
	})
	written, err := fetchForTest(t, testArchiveTransfer, server.Client(), server.URL+"/tool", nil)
	if err != nil {
		t.Fatal(err)
	}
	requests := log.all()
	if !bytes.Equal(written, data) || len(requests) != 2 || requests[1].Header.Get("Range") == "" {
		t.Fatalf("requests = %d, want an abandoned slow attempt and one ignored resume completed", len(requests))
	}
}

func TestArchiveTransferRestartsWhenAResumeIsNotHonored(t *testing.T) {
	data := transferData(128 << 10)
	for _, test := range []struct {
		name     string
		resume   func(w http.ResponseWriter, r *http.Request)
		requests int
	}{
		{"full response", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			_, _ = w.Write(data)
		}, 2},
		{"wrong range", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Range", "bytes 0-"+strconv.Itoa(len(data)-1)+"/"+strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data)
		}, 3},
		{"range refused", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		}, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, log := transferServer(t, false, func(attempt int, w http.ResponseWriter, r *http.Request) {
				switch attempt {
				case 1:
					serveRanges(&cutAfter{ResponseWriter: w, left: 4096}, r, data)
				case 2:
					test.resume(w, r)
				default:
					serveRanges(w, r, data)
				}
			})
			written, err := fetchForTest(t, testArchiveTransfer, server.Client(), server.URL+"/tool", nil)
			if err != nil {
				t.Fatal(err)
			}
			requests := log.all()
			if !bytes.Equal(written, data) || len(requests) != test.requests {
				t.Fatalf("requests = %d, want %d and the complete file", len(requests), test.requests)
			}
			if last := requests[len(requests)-1]; test.requests == 3 && last.Header.Get("Range") != "" {
				t.Fatalf("restart after a bad resume still sent Range %q", last.Header.Get("Range"))
			}
		})
	}
}

func TestArchiveTransferStopsOnlyRetryableFailures(t *testing.T) {
	for _, test := range []struct {
		status, requests int
	}{{http.StatusServiceUnavailable, 3}, {http.StatusTooManyRequests, 3}, {http.StatusNotFound, 1}, {http.StatusForbidden, 1}} {
		t.Run(strconv.Itoa(test.status), func(t *testing.T) {
			server, log := transferServer(t, false, func(_ int, w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			})
			_, err := fetchForTest(t, testArchiveTransfer, server.Client(), server.URL+"/tool", nil)
			// Retried failures report their attempts; a machine request has no display.
			if err == nil || !strings.Contains(err.Error(), "HTTP "+strconv.Itoa(test.status)) || len(log.all()) != test.requests ||
				test.requests > 1 && !strings.Contains(err.Error(), "(3 attempts, 0 resumed, 0 restarted)") {
				t.Fatalf("requests = %d, want %d; error %v", len(log.all()), test.requests, err)
			}
		})
	}
}

func TestArchiveTransferHonorsItsTotalLimit(t *testing.T) {
	data := transferData(64 << 10)
	server, _ := transferServer(t, false, func(_ int, w http.ResponseWriter, r *http.Request) {
		stallAfter(w, r, data, 0, true)
	})
	policy := testArchiveTransfer
	policy.limit, policy.attempts = 700*time.Millisecond, 1000
	started := time.Now()
	_, err := fetchForTest(t, policy, server.Client(), server.URL+"/tool", nil)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 5*time.Second {
		t.Fatalf("total limit not enforced after %s: %v", time.Since(started), err)
	}
}

func TestArchiveTransferAcceptsOnlyTheRequestedRemainder(t *testing.T) {
	for value, want := range map[string]bool{
		"bytes 100-999/1000": true,
		"bytes 100-998/1000": false,
		"bytes 99-999/1000":  false,
		"bytes 100-999/*":    false,
		"bytes 100-999/1001": false,
		"items 100-999/1000": false,
		"bytes 100-999":      false,
		"":                   false,
	} {
		if got := matchesContentRange(value, 100, 1000); got != want {
			t.Errorf("matchesContentRange(%q) = %v, want %v", value, got, want)
		}
	}
	if matchesContentRange("bytes 1000-999/1000", 1000, 1000) {
		t.Error("accepted a resume at the end of the file")
	}
}
