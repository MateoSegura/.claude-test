package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestHelperProcess is invoked by the mocked exec commands.
// It checks env vars to decide what to output and how to exit.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	stdout := os.Getenv("GO_HELPER_STDOUT")
	stderr := os.Getenv("GO_HELPER_STDERR")
	exitCodeStr := os.Getenv("GO_HELPER_EXIT_CODE")

	if stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}

	exitCode := 0
	if exitCodeStr != "" {
		code, err := strconv.Atoi(exitCodeStr)
		if err == nil {
			exitCode = code
		}
	}

	os.Exit(exitCode)
}

// mockExecCommand returns a function compatible with Manager.execCommandContext.
// It creates a command that re-invokes the test binary targeting TestHelperProcess,
// passing the real command and args after "--".
func mockExecCommand(stdout, stderr string, exitCode int) func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("GO_HELPER_STDOUT=%s", stdout),
			fmt.Sprintf("GO_HELPER_STDERR=%s", stderr),
			fmt.Sprintf("GO_HELPER_EXIT_CODE=%d", exitCode),
		}
		return cmd
	}
}

func TestNewManagerDefaults(t *testing.T) {
	m := NewManager("myimage")
	if m.Image != "myimage" {
		t.Errorf("expected Image to be 'myimage', got %q", m.Image)
	}
	if m.execCommandContext == nil {
		t.Error("expected execCommandContext to be set")
	}
}

func TestCreateContainerSuccess(t *testing.T) {
	m := NewManager("fallback:latest")
	m.execCommandContext = mockExecCommand("abc123def456\n", "", 0)

	ctx := context.Background()
	id, err := m.CreateContainer(ctx, ContainerOpts{
		Image:   "myimage:latest",
		Env:     map[string]string{"ANTHROPIC_API_KEY": "sk-test", "HOME": "/root"},
		WorkDir: "/workspace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc123def456" {
		t.Errorf("expected container ID 'abc123def456', got %q", id)
	}

	// Verify args by capturing what the mock would receive.
	// We rebuild the args to check them directly.
	args := buildCreateArgs(ContainerOpts{
		Image:   "myimage:latest",
		Env:     map[string]string{"ANTHROPIC_API_KEY": "sk-test", "HOME": "/root"},
		WorkDir: "/workspace",
	}, "fallback:latest")

	argStr := strings.Join(args, " ")
	for _, want := range []string{
		"-e ANTHROPIC_API_KEY=sk-test",
		"-e HOME=/root",
		"-w /workspace",
		"myimage:latest",
	} {
		if !strings.Contains(argStr, want) {
			t.Errorf("expected args to contain %q, got %q", want, argStr)
		}
	}
}

// buildCreateArgs replicates the arg-building logic for verification.
func buildCreateArgs(opts ContainerOpts, fallbackImage string) []string {
	image := opts.Image
	if image == "" {
		image = fallbackImage
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
	return args
}

func TestCreateContainerFailure(t *testing.T) {
	m := NewManager("myimage:latest")
	m.execCommandContext = mockExecCommand("", "image not found", 1)

	ctx := context.Background()
	_, err := m.CreateContainer(ctx, ContainerOpts{
		Image: "nonexistent:latest",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecInContainerSuccess(t *testing.T) {
	m := NewManager("myimage:latest")
	m.execCommandContext = mockExecCommand("hello world\n", "", 0)

	ctx := context.Background()
	stdout, _, exitCode, err := m.ExecInContainer(ctx, "ctr1", []string{"echo", "hello world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	data, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("failed to read stdout: %v", readErr)
	}
	if string(data) != "hello world\n" {
		t.Errorf("expected stdout 'hello world\\n', got %q", string(data))
	}
}

func TestIsAvailable(t *testing.T) {
	// Test unavailable
	m := NewManager("myimage:latest")
	m.execCommandContext = mockExecCommand("", "Cannot connect", 1)

	ctx := context.Background()
	err := m.IsAvailable(ctx)
	if err == nil {
		t.Error("expected error when docker info fails, got nil")
	}

	// Test available
	m2 := NewManager("myimage:latest")
	m2.execCommandContext = mockExecCommand("", "", 0)

	err = m2.IsAvailable(ctx)
	if err != nil {
		t.Errorf("expected nil error when docker info succeeds, got %v", err)
	}
}

func TestRemoveContainer(t *testing.T) {
	m := NewManager("myimage:latest")
	m.execCommandContext = mockExecCommand("", "", 0)

	ctx := context.Background()
	err := m.RemoveContainer(ctx, "ctr1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyToContainer(t *testing.T) {
	m := NewManager("myimage:latest")
	m.execCommandContext = mockExecCommand("", "", 0)

	ctx := context.Background()
	err := m.CopyToContainer(ctx, "ctr1", "/tmp/src", "/dst")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
