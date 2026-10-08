package installer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// The controller holds the engine lock for this entire session. Keep an
// existing native lock while observing, and hand it to a worker only if a
// native command is needed. Archive-only requests never start a worker or
// create native state. The worker accepts no command before acquiring its lock.
type nativeSession struct {
	Directory string
	Start     func(context.Context) (*nativeWorkerClient, error)
	mutex     sync.Mutex
	active    bool
	release   func() error
	worker    *nativeWorkerClient
}

func newNativeSession(directory string) *nativeSession {
	return &nativeSession{Directory: directory, Start: func(ctx context.Context) (*nativeWorkerClient, error) {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		return startNativeWorker(ctx, executable, []string{"internal-native-worker", directory}, nil, io.Discard)
	}}
}

func lockExisting(directory string) (func() error, error) {
	release, err := lockFile(directory, false)
	if errors.Is(err, os.ErrNotExist) {
		return func() error { return nil }, nil
	}
	return release, err
}

func (s *nativeSession) acquire(ctx context.Context) (func() error, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.active || s.Start == nil || !filepath.IsAbs(s.Directory) {
		return nil, errors.New("native session is active or invalid")
	}
	release, err := lockExisting(s.Directory)
	if err != nil {
		return nil, err
	}
	s.active, s.release = true, release
	return s.close, nil
}

func (s *nativeSession) run(ctx context.Context, command nativeCommand) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.active {
		return nil, errors.New("native command requires the controller mutation guard")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.worker == nil {
		// The engine lock remains held across handoff. Other controllers and
		// previews cannot enter; an old surviving worker was excluded above.
		if s.release != nil {
			err := s.release()
			s.release = nil
			if err != nil {
				return nil, err
			}
		}
		worker, err := s.Start(ctx)
		if err != nil {
			return nil, err
		}
		s.worker = worker
	}
	return s.worker.run(ctx, command)
}

func (s *nativeSession) close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	var err error
	if s.worker != nil {
		err = s.worker.close()
		s.worker = nil
	}
	if s.release != nil {
		err = errors.Join(err, s.release())
		s.release = nil
	}
	s.active = false
	return err
}

func (d *NativeDriver) AcquireMutation(ctx context.Context) (func() error, error) {
	if d.session == nil {
		if d.Brew != nil || d.Homebrew != nil || d.AppleCLT != nil || d.APT != nil || d.WindowsVendor != nil || d.BuildTools != nil || d.NvimSync != nil {
			return nil, errors.New("native package providers require their mutation session")
		}
		return func() error { return nil }, ctx.Err()
	}
	release, err := d.session.acquire(ctx)
	if err != nil {
		return nil, err
	}
	if d.Brew != nil {
		if d.Brew.approval == nil {
			return nil, errors.Join(errors.New("Homebrew lacks its approval session"), release())
		}
		d.Brew.approval.expected = ""
	}
	if d.APT != nil {
		if d.APT.approval == nil {
			return nil, errors.Join(errors.New("APT lacks its approval session"), release())
		}
		d.APT.approval.expected = ""
	}
	return release, nil
}

func (d *NativeDriver) AcquirePreview(ctx context.Context) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(d.StatePath) {
		return nil, errors.New("native preview requires its engine state path")
	}
	engine, err := lockExisting(filepath.Dir(d.StatePath))
	if err != nil {
		return nil, err
	}
	if d.session == nil {
		return engine, nil
	}
	native, err := lockExisting(d.session.Directory)
	if err != nil {
		return nil, errors.Join(err, engine())
	}
	return func() error { return errors.Join(native(), engine()) }, nil
}
