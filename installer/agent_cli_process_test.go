package installer

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

// These pipes belong to the fixture, so root exit cannot close a reader before
// its final bytes arrive. Windows additionally owns descendants in a private job.
type agentCLIProcess struct {
	input, output, diagnostic *os.File
	wait                      func(bool) error
}

func (p *agentCLIProcess) close() error {
	return errors.Join(p.output.Close(), p.diagnostic.Close())
}

func agentCLIStreams(p *agentCLIProcess, output, diagnostic io.Writer) <-chan error {
	done := make(chan error, 2)
	go func() { _, err := io.Copy(output, p.output); done <- err }()
	go func() { _, err := io.Copy(diagnostic, p.diagnostic); done <- err }()
	return done
}

func agentCLIDrain(p *agentCLIProcess, done <-chan error, remaining int) error {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var result error
	for ; remaining > 0; remaining-- {
		select {
		case err := <-done:
			result = errors.Join(result, err)
		case <-deadline.C:
			result = errors.Join(result, exec.ErrWaitDelay, p.close())
			for ; remaining > 0; remaining-- {
				result = errors.Join(result, <-done)
			}
			return result
		}
	}
	return errors.Join(result, p.close())
}

func agentCLIOutput(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	p, err := startAgentCLI(ctx, cmd)
	if err != nil {
		return nil, err
	}
	output, diagnostic := boundedCommandOutput{limit: 1 << 20}, boundedCommandOutput{}
	done := agentCLIStreams(p, &output, &diagnostic)
	inputErr := p.input.Close()
	waitErr := p.wait(false)
	drainErr := agentCLIDrain(p, done, 2)
	return append(output.data.Bytes(), diagnostic.data.Bytes()...), errors.Join(inputErr, waitErr, drainErr)
}
