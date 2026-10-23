package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"
)

// Each apply keeps its own complete record of steps and native output beside
// the state, so a cause that scrolled away stays inspectable. Only the newest
// logs are retained. A log is a record for people, never ownership evidence,
// and its failures are reported as notes rather than failing the apply.
const keptApplyLogs = 20

var applyLogName = regexp.MustCompile(`^apply-[0-9]{8}T[0-9]{6}Z-[0-9]+\.log$`)

type applyLog struct {
	mutex sync.Mutex
	file  *os.File
	err   error
}

func (l *applyLog) Write(data []byte) (int, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.err == nil {
		_, l.err = l.file.Write(data)
	}
	return len(data), nil
}

func (l *applyLog) close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return errors.Join(l.err, l.file.Close())
}

// openApplyLog prunes the oldest logs individually, then creates a private
// new one. A prune failure is returned with the usable log.
func openApplyLog(stateDirectory string, now time.Time) (*applyLog, error) {
	directory := filepath.Join(stateDirectory, "logs")
	if err := prepareStateDirectory(directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && applyLogName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	var pruneErr error
	for len(names) >= keptApplyLogs {
		pruneErr = errors.Join(pruneErr, os.Remove(filepath.Join(directory, names[0])))
		names = names[1:]
	}
	file, err := os.CreateTemp(directory, "apply-"+now.UTC().Format("20060102T150405Z")+"-*.log")
	if err != nil {
		return nil, errors.Join(err, pruneErr)
	}
	return &applyLog{file: file}, pruneErr
}

// attach records every step and native output line, and still forwards them
// to any display already carried by the context.
func (l *applyLog) attach(ctx context.Context) context.Context {
	display, _ := ctx.Value(progressKey{}).(Progress)
	output := io.Writer(l)
	if display.Output != nil {
		output = io.MultiWriter(l, display.Output)
	}
	return WithProgress(ctx, Progress{Output: output, Step: func(text string) {
		_, _ = fmt.Fprintf(l, "==> %s %s\n", time.Now().UTC().Format(time.TimeOnly), text)
		if display.Step != nil {
			display.Step(text)
		}
	}})
}
