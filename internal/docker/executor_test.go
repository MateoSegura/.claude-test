package docker

import (
	"context"
	"io"
	"testing"
)

func Test_MockExecutor_ReturnsConfiguredOutput(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("echo", MockResponse{
		Stdout:   "hello world\n",
		ExitCode: 0,
	})

	stdout, _, exitCode, err := mock.Exec(context.Background(), "container-1", []string{"echo", "hello world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if got := string(data); got != "hello world\n" {
		t.Errorf("stdout = %q, want %q", got, "hello world\n")
	}

	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func Test_MockExecutor_RecordsCommands(t *testing.T) {
	mock := NewMockExecutor()

	cmds := [][]string{
		{"git", "clone", "https://github.com/example/repo.git"},
		{"go", "mod", "download"},
		{"claude", "--print", "explain this code"},
	}

	for _, cmd := range cmds {
		_, _, _, err := mock.Exec(context.Background(), "container-1", cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	recorded := mock.Commands()
	if len(recorded) != 3 {
		t.Fatalf("recorded %d commands, want 3", len(recorded))
	}

	for i, want := range cmds {
		got := recorded[i]
		if len(got) != len(want) {
			t.Errorf("command[%d] has %d args, want %d", i, len(got), len(want))
			continue
		}
		for j := range want {
			if got[j] != want[j] {
				t.Errorf("command[%d][%d] = %q, want %q", i, j, got[j], want[j])
			}
		}
	}
}

func Test_MockExecutor_ExitCodeControl(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("false", MockResponse{ExitCode: 1})

	// "false" should return exit code 1.
	_, _, exitCode, err := mock.Exec(context.Background(), "container-1", []string{"false"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 1 {
		t.Errorf("exitCode for 'false' = %d, want 1", exitCode)
	}

	// "true" should fall back to default (exit code 0).
	_, _, exitCode, err = mock.Exec(context.Background(), "container-1", []string{"true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode for 'true' = %d, want 0", exitCode)
	}
}

func Test_DockerExecutor_SatisfiesInterface(t *testing.T) {
	// Compile-time interface satisfaction check.
	// If this compiles, DockerExecutor implements Executor.
	var _ Executor = (*DockerExecutor)(nil)
}

func Test_MockExecutor_DefaultResponse(t *testing.T) {
	mock := NewMockExecutor()
	mock.SetDefault(MockResponse{
		Stdout:   "default output",
		ExitCode: 42,
	})

	stdout, _, exitCode, err := mock.Exec(context.Background(), "container-1", []string{"anything"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if got := string(data); got != "default output" {
		t.Errorf("stdout = %q, want %q", got, "default output")
	}

	if exitCode != 42 {
		t.Errorf("exitCode = %d, want 42", exitCode)
	}
}
