//go:build !windows

package installer

import (
	"context"
	"errors"
)

func queryWindowsBuildTools(context.Context, bool, string, []byte, ...string) ([]byte, error) {
	return nil, errors.New("Build Tools inspection requires native Windows")
}
