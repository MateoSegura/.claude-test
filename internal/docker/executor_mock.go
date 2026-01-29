package docker

import (
	"context"
	"io"
	"strings"
)

// Compile-time check that MockExecutor satisfies the Executor interface.
var _ Executor = (*MockExecutor)(nil)

// MockResponse defines the canned response a MockExecutor returns.
type MockResponse struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// MockExecutor is a test double that implements Executor with configurable
// responses and command recording.
type MockExecutor struct {
	defaultResponse MockResponse
	responses       map[string]MockResponse // keyed by cmd[0]
	commands        [][]string              // records all commands
}

// NewMockExecutor returns a MockExecutor with an empty default response
// (empty stdout/stderr, exit code 0, nil error).
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		responses: make(map[string]MockResponse),
	}
}

// SetDefault sets the response returned for commands that have no
// explicit match registered via OnCommand.
func (m *MockExecutor) SetDefault(resp MockResponse) {
	m.defaultResponse = resp
}

// OnCommand registers a canned response for any command whose first
// element (cmd[0]) equals cmdPrefix.
func (m *MockExecutor) OnCommand(cmdPrefix string, resp MockResponse) {
	m.responses[cmdPrefix] = resp
}

// Exec looks up a response by cmd[0], falling back to the default response.
// It records every command for later inspection via Commands().
func (m *MockExecutor) Exec(_ context.Context, _ string, cmd []string) (io.Reader, io.Reader, int, error) {
	// Record the command.
	copied := make([]string, len(cmd))
	copy(copied, cmd)
	m.commands = append(m.commands, copied)

	// Look up response.
	resp := m.defaultResponse
	if len(cmd) > 0 {
		if r, ok := m.responses[cmd[0]]; ok {
			resp = r
		}
	}

	return strings.NewReader(resp.Stdout), strings.NewReader(resp.Stderr), resp.ExitCode, resp.Err
}

// Commands returns all commands that have been passed to Exec, in order.
func (m *MockExecutor) Commands() [][]string {
	return m.commands
}
