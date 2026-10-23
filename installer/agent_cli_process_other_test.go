//go:build !windows

package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

func startAgentCLI(ctx context.Context, cmd *exec.Cmd) (_ *agentCLIProcess, result error) {
	var pipes [3][2]*os.File
	defer func() {
		if result != nil {
			for _, pair := range pipes {
				for _, file := range pair {
					if file != nil {
						result = errors.Join(result, file.Close())
					}
				}
			}
		}
	}()
	for i := range pipes {
		var err error
		pipes[i][0], pipes[i][1], err = os.Pipe()
		if err != nil {
			return nil, err
		}
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = pipes[0][0], pipes[1][1], pipes[2][1]
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	for _, index := range [][2]int{{0, 0}, {1, 1}, {2, 1}} {
		file := pipes[index[0]][index[1]]
		pipes[index[0]][index[1]] = nil
		if err := file.Close(); err != nil {
			return nil, errors.Join(err, cmd.Process.Kill(), cmd.Wait())
		}
	}
	return &agentCLIProcess{input: pipes[0][1], output: pipes[1][0], diagnostic: pipes[2][0], wait: func(stop bool) error {
		if !stop {
			return cmd.Wait()
		}
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) && exit.ProcessState.ExitCode() == -1 {
			waitErr = nil
		}
		return errors.Join(ctx.Err(), killErr, waitErr)
	}}, nil
}
