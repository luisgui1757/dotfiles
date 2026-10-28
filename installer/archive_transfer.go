package installer

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// archiveTransfer bounds one archive download across reconnections. On
// 2026-10-28 GitHub-hosted macOS runners reached nodejs.org through paths that
// alternated between sub-second transfers, trickles below 15 KiB/s and HTTP/2
// stream resets, while the same file took 4 s from a home Mac. A single request
// stayed on its slow path until the 15-minute limit and failed every macOS Node
// lifecycle. Each attempt therefore opens fresh connections, an attempt slower
// than the minimum is abandoned, and a server that offers byte ranges continues
// from the kept bytes. The caller's complete SHA-256 check remains the only
// integrity authority; ranges never decide which bytes are correct.
type archiveTransfer struct {
	limit    time.Duration // whole download, every attempt included
	window   time.Duration // interval over which attempt progress is measured
	minimum  int64         // bytes a resumable attempt must deliver per window
	attempts int           // consecutive attempts that may keep no new bytes
	backoff  time.Duration // delay after the first such attempt, then doubled
}

var defaultArchiveTransfer = archiveTransfer{
	limit:    15 * time.Minute,
	window:   30 * time.Second,
	minimum:  4 << 20,
	attempts: 5,
	backoff:  time.Second,
}

var errArchiveTransferSlow = errors.New("archive transfer too slow")

// archivePermanentError marks a response that a new connection cannot change.
type archivePermanentError struct{ error }

func (e archivePermanentError) Unwrap() error { return e.error }

type archiveTransferState struct {
	file      *os.File
	digest    hash.Hash
	progress  *downloadProgress
	kept      int64  // verified-length bytes written to file and digest
	total     int64  // declared length of the last full response; 0 or -1 is unknown
	validator string // strong ETag or Last-Modified for If-Range
	ranges    bool   // the full response offered byte ranges of an unencoded body
	unhonored bool   // a resume was answered without the requested range
	// Counts for the final error; a machine request has no progress display.
	attempts, resumed, restarted int
}

func (s *archiveTransferState) summary() string {
	return fmt.Sprintf("%d attempts, %d resumed, %d restarted", s.attempts, s.resumed, s.restarted)
}

func (s *archiveTransferState) restart() error {
	s.restarted++
	if err := s.file.Truncate(0); err != nil {
		return archivePermanentError{err}
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return archivePermanentError{err}
	}
	s.digest.Reset()
	s.kept, s.total, s.validator, s.ranges = 0, -1, "", false
	if s.progress != nil {
		s.progress.done, s.progress.total = 0, -1
	}
	return nil
}

// unhonoredResume starts over after a server answered a resume without the
// requested range. Its advertised ranges are not trusted again in this
// transfer: abandoning its slow attempts would discard every restart's bytes.
func (s *archiveTransferState) unhonoredResume() error {
	s.unhonored = true
	return s.restart()
}

func (s *archiveTransferState) Write(data []byte) (int, error) {
	n, err := s.file.Write(data)
	s.digest.Write(data[:n])
	s.kept += int64(n)
	if s.progress != nil {
		_, _ = s.progress.Write(data[:n])
	}
	if err != nil {
		return n, archivePermanentError{err}
	}
	return n, nil
}

// fetch writes the complete response body for rawURL to file and digest and
// returns its length. The caller verifies size and SHA-256.
func (p archiveTransfer) fetch(ctx context.Context, client *http.Client, rawURL string, file *os.File, digest hash.Hash, display io.Writer, name string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, p.limit)
	defer cancel()
	s := &archiveTransferState{file: file, digest: digest, total: -1}
	if display != nil {
		s.progress = &downloadProgress{output: display, name: name, total: -1}
	}
	failures := 0
	for {
		start := s.kept
		err := p.attempt(ctx, client, rawURL, s)
		if err == nil {
			return s.kept, nil
		}
		if ctx.Err() != nil {
			return s.kept, fmt.Errorf("download %s stopped with %s received (%s): %w (last attempt: %v)", name, formatBytes(s.kept), s.summary(), ctx.Err(), err)
		}
		if permanent := (archivePermanentError{}); errors.As(err, &permanent) {
			return s.kept, permanent.error
		}
		if s.ranges && s.kept > start {
			failures = 0
		} else {
			failures++
		}
		if failures >= p.attempts {
			return s.kept, fmt.Errorf("download %s failed %d times without progress (%s): %w", name, failures, s.summary(), err)
		}
		if display != nil {
			action := "restarting"
			if s.ranges && s.kept > 0 {
				action = "resuming at " + formatBytes(s.kept)
			}
			_, _ = fmt.Fprintf(display, "\rDownloading %s: %v; reconnecting and %s\n", name, err, action)
		}
		if failures > 0 {
			delay := min(p.backoff<<(failures-1), 30*time.Second)
			select {
			case <-ctx.Done():
				return s.kept, fmt.Errorf("download %s stopped with %s received (%s): %w (last attempt: %v)", name, formatBytes(s.kept), s.summary(), ctx.Err(), err)
			case <-time.After(delay):
			}
		}
	}
}

func (p archiveTransfer) attempt(ctx context.Context, client *http.Client, rawURL string, s *archiveTransferState) (result error) {
	s.attempts++
	if s.kept > 0 && !s.ranges {
		if err := s.restart(); err != nil {
			return err
		}
	}
	attemptCtx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	// Every attempt gets its own connection pool. Reusing a pooled HTTP/2
	// connection would keep the attempt on the path that just failed.
	c := *client
	if base, ok := transportOrDefault(client.Transport).(*http.Transport); ok {
		fresh := base.Clone()
		defer fresh.CloseIdleConnections()
		c.Transport = fresh
	}
	var received atomic.Int64
	var resumable atomic.Bool
	resumable.Store(s.ranges)
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(p.window)
		defer ticker.Stop()
		var last int64
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := received.Load()
				// Abandoning a slow attempt keeps its bytes only when the server
				// can resume; otherwise only a complete stall starts over.
				if now == last || resumable.Load() && now-last < p.minimum {
					stop(fmt.Errorf("%w: %s in %s", errArchiveTransferSlow, formatBytes(now-last), p.window))
					return
				}
				last = now
			}
		}
	}()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return archivePermanentError{err}
	}
	if s.kept > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", s.kept))
		if s.validator != "" {
			req.Header.Set("If-Range", s.validator)
		}
	}
	response, err := c.Do(req)
	if err != nil {
		return attemptError(attemptCtx, err)
	}
	defer func() { result = errors.Join(result, response.Body.Close()) }()
	switch code := response.StatusCode; {
	case s.kept > 0 && code == http.StatusPartialContent:
		if !matchesContentRange(response.Header.Get("Content-Range"), s.kept, s.total) {
			if err := s.unhonoredResume(); err != nil {
				return err
			}
			return fmt.Errorf("archive server answered a resume with range %q", response.Header.Get("Content-Range"))
		}
		s.resumed++
	case code == http.StatusOK:
		// A full response to a resume means the server ignored the range or
		// the If-Range validator changed: discard the kept bytes.
		if s.kept > 0 {
			if err := s.unhonoredResume(); err != nil {
				return err
			}
		}
		if response.ContentLength > maxArchiveBytes {
			return archivePermanentError{fmt.Errorf("archive download rejected: HTTP %d, length %d", code, response.ContentLength)}
		}
		s.total = response.ContentLength
		s.ranges = !s.unhonored && s.total > 0 && !response.Uncompressed && acceptsByteRanges(response.Header)
		s.validator = rangeValidator(response.Header)
		if s.progress != nil {
			s.progress.total = s.total
		}
		resumable.Store(s.ranges)
	case s.kept > 0 && code == http.StatusRequestedRangeNotSatisfiable:
		if err := s.unhonoredResume(); err != nil {
			return err
		}
		return fmt.Errorf("archive server refused to resume: HTTP %d", code)
	case code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500:
		return fmt.Errorf("archive download: HTTP %d", code)
	default:
		return archivePermanentError{fmt.Errorf("archive download rejected: HTTP %d, length %d", code, response.ContentLength)}
	}
	counted := io.TeeReader(response.Body, archiveByteCounter{&received})
	_, err = io.Copy(s, io.LimitReader(counted, maxArchiveBytes+1-s.kept))
	if s.kept > maxArchiveBytes || s.total > 0 && s.kept > s.total {
		return archivePermanentError{errors.New("archive size or SHA-256 differs from the reviewed pin")}
	}
	if err != nil {
		return attemptError(attemptCtx, err)
	}
	if s.total > 0 && s.kept < s.total {
		return fmt.Errorf("archive body ended after %s of %s", formatBytes(s.kept), formatBytes(s.total))
	}
	return nil
}

// attemptError reports the watchdog's reason rather than the cancellation it caused.
func attemptError(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); errors.Is(cause, errArchiveTransferSlow) {
		return cause
	}
	return err
}

type archiveByteCounter struct{ count *atomic.Int64 }

func (c archiveByteCounter) Write(data []byte) (int, error) {
	c.count.Add(int64(len(data)))
	return len(data), nil
}

func transportOrDefault(transport http.RoundTripper) http.RoundTripper {
	if transport == nil {
		return http.DefaultTransport
	}
	return transport
}

func acceptsByteRanges(header http.Header) bool {
	for _, value := range header.Values("Accept-Ranges") {
		for _, unit := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(unit), "bytes") {
				return true
			}
		}
	}
	return false
}

// rangeValidator returns a validator that If-Range accepts: a strong ETag, or
// Last-Modified. Without one, the final checksum still rejects a changed file.
func rangeValidator(header http.Header) string {
	if tag := header.Get("ETag"); strings.HasPrefix(tag, `"`) {
		return tag
	}
	return header.Get("Last-Modified")
}

// matchesContentRange accepts exactly "bytes start-(total-1)/total".
func matchesContentRange(value string, start, total int64) bool {
	spec, ok := strings.CutPrefix(value, "bytes ")
	if !ok || total <= start {
		return false
	}
	span, size, ok := strings.Cut(spec, "/")
	if !ok {
		return false
	}
	first, last, ok := strings.Cut(span, "-")
	if !ok {
		return false
	}
	parsed := make([]int64, 0, 3)
	for _, field := range []string{first, last, size} {
		number, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return false
		}
		parsed = append(parsed, number)
	}
	return parsed[0] == start && parsed[1] == total-1 && parsed[2] == total
}
