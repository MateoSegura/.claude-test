package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ContainerOpts holds configuration for creating a Docker container.
type ContainerOpts struct {
	Image     string
	ConfigDir string
	Env       map[string]string
	WorkDir   string
}

// Manager manages Docker container lifecycle operations using os/exec.
type Manager struct {
	Image              string
	execCommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// NewManager returns a Manager configured with the given image
// and exec.CommandContext as the default command factory.
func NewManager(image string) *Manager {
	return &Manager{
		Image:              image,
		execCommandContext: exec.CommandContext,
	}
}

// CreateContainer builds and runs a "docker create" command from the given opts.
// It returns the container ID (stdout trimmed) or an error.
func (m *Manager) CreateContainer(ctx context.Context, opts ContainerOpts) (string, error) {
	image := opts.Image
	if image == "" {
		image = m.Image
	}

	args := []string{"create"}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}

	if opts.ConfigDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", opts.ConfigDir, opts.ConfigDir))
	}

	args = append(args, image)

	cmd := m.execCommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker create failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CopyToContainer runs "docker cp src containerID:dst".
func (m *Manager) CopyToContainer(ctx context.Context, containerID, src, dst string) error {
	cmd := m.execCommandContext(ctx, "docker", "cp", src, fmt.Sprintf("%s:%s", containerID, dst))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker cp failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// ExecInContainer runs "docker exec id cmd..." and returns stdout, stderr readers,
// the process exit code, and an error only for system-level failures.
// A non-zero exit code from the executed command is returned as the exitCode
// with a nil error, since non-zero exits are expected during evaluation.
func (m *Manager) ExecInContainer(ctx context.Context, id string, command []string) (stdout io.Reader, stderr io.Reader, exitCode int, err error) {
	args := append([]string{"exec", id}, command...)
	cmd := m.execCommandContext(ctx, "docker", args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return &stdoutBuf, &stderrBuf, exitErr.ExitCode(), nil
		}
		return &stdoutBuf, &stderrBuf, 0, fmt.Errorf("docker exec failed: %w", runErr)
	}

	return &stdoutBuf, &stderrBuf, 0, nil
}

// RemoveContainer runs "docker rm -f id" to forcefully remove a container.
func (m *Manager) RemoveContainer(ctx context.Context, id string) error {
	cmd := m.execCommandContext(ctx, "docker", "rm", "-f", id)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker rm failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// IsAvailable runs "docker info" and returns nil if Docker is accessible.
func (m *Manager) IsAvailable(ctx context.Context) error {
	cmd := m.execCommandContext(ctx, "docker", "info")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	return nil
}
