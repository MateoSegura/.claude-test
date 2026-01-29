package docker

import (
	"context"
	"io"
)

// Executor abstracts command execution inside a container,
// allowing real Docker execution and mock implementations for testing.
type Executor interface {
	Exec(ctx context.Context, containerID string, cmd []string) (stdout io.Reader, stderr io.Reader, exitCode int, err error)
}
