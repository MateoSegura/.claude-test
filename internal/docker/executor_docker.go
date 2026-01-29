package docker

import (
	"context"
	"io"
)

// Compile-time check that DockerExecutor satisfies the Executor interface.
var _ Executor = (*DockerExecutor)(nil)

// DockerExecutor implements Executor by delegating to a Manager.
type DockerExecutor struct {
	manager *Manager
}

// NewDockerExecutor returns a DockerExecutor that wraps the given Manager.
func NewDockerExecutor(manager *Manager) *DockerExecutor {
	return &DockerExecutor{manager: manager}
}

// Exec runs a command inside the specified container via the underlying Manager.
func (e *DockerExecutor) Exec(ctx context.Context, containerID string, cmd []string) (io.Reader, io.Reader, int, error) {
	return e.manager.ExecInContainer(ctx, containerID, cmd)
}
