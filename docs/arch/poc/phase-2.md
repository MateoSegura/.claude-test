# Phase 2: Container Execution

> **Status:** `NOT_STARTED`
> **Depends on:** Phase 1
> **Produces:** `internal/docker/runner.go`, `internal/metrics/parser.go`, evaluation execution

## Overview

This phase implements the core execution loop: running Claude CLI inside a Docker container, parsing the stream-json output into SDK types, extracting metrics, and evaluating the result. This is the most critical path — if the runner can produce a valid `RunResult` from a container execution, everything downstream works.

## Packages

### 2.1 `internal/docker/runner.go`

Executes Claude inside a container and produces a `RunResult`:

```go
type RunConfig struct {
    ContainerID string
    Prompt      string
    MaxTurns    int
    Timeout     time.Duration
    WorkDir     string
    SetupCmds   []string
    Evaluation  corpus.Evaluation
}

type Runner struct {
    executor Executor
}

func NewRunner(executor Executor) *Runner
func (r *Runner) Run(ctx context.Context, cfg RunConfig) (*metrics.RunResult, error)
```

> **Note:** The Runner takes an `Executor` interface (defined in Stage 2.2), not a `*Manager` directly. This enables testing with `MockExecutor`.

The runner:
1. Runs setup commands (git clone, go mod download, etc.)
2. Executes `docker exec <id> claude --print --output-format stream-json --verbose --dangerously-skip-permissions --max-turns N <prompt>`
3. Reads stdout line-by-line, parsing each as `claude.StreamMessage`
4. Feeds messages to the metrics parser and tool tracker
5. Runs evaluation command after Claude finishes
6. Returns assembled `RunResult`

### 2.2 `internal/metrics/parser.go`

Extracts metrics from a stream of `claude.StreamMessage`:

```go
type StreamParser struct {
    tracker *Tracker
}

func NewStreamParser() *StreamParser
func (p *StreamParser) ProcessMessage(msg *claude.StreamMessage)
func (p *StreamParser) Result() ParserResult

type ParserResult struct {
    InitTools   []string
    ToolCalls   []ToolCall
    Summary     map[string]int
    CostUSD     float64
    DurationMS  int64
    NumTurns    int
    Usage       *claude.Usage
    FinalText   string
}
```

### 2.3 Evaluation execution

Evaluation runs inside the container after Claude finishes:

```go
func (r *Runner) evaluate(ctx context.Context, containerID string, eval corpus.Evaluation) (bool, float64, error)
```

| Method | Implementation |
|--------|---------------|
| `command` | `docker exec <id> sh -c "<command>"`, check exit code |
| `test_passes` | Same as command, but score = tests_passed / tests_total |
| `file_contains` | `docker exec <id> grep -q "<contains>" "<file_path>"` |

## Stages

### Stage 2.1: Stream Parser Core

**What to build:**

- File: `internal/metrics/parser.go`
- Types: `StreamParser`, `ParserResult`
- Functions: `NewStreamParser()`, `(p *StreamParser) ProcessMessage(msg *claude.StreamMessage)`, `(p *StreamParser) Result() ParserResult`

The `StreamParser` receives already-parsed `claude.StreamMessage` values (it does NOT unmarshal JSON itself -- that is the runner's job). It delegates tool tracking to the `Tracker` from Phase 1.

`ParserResult` fields:

```go
type ParserResult struct {
    InitTools       []string
    ToolCalls       []ToolCall
    Summary         map[string]int
    CostUSD         float64
    DurationMS      int64
    NumTurns        int
    Usage           *claude.Usage
    FinalText       string
}
```

The parser uses SDK helpers directly: `claude.ExtractInitTools()`, `claude.GetAllToolCalls()`, `claude.IsResult()`, `claude.ExtractText()`. It should handle repeated init messages gracefully (last one wins) and ignore messages with unknown types (no panic, no error -- just skip).

**Pass/fail criteria:**

1. `NewStreamParser()` returns a non-nil `*StreamParser` with an internal `Tracker` initialized.
2. Feeding a single init message with `Tools: ["Read", "Write", "Bash"]` results in `Result().InitTools` being `["Read", "Write", "Bash"]`.
3. Feeding an assistant message containing two `tool_use` blocks results in `Result().ToolCalls` having length 2, each with the correct `Name` and `Category`.
4. Feeding a result message with `CostUSD: 0.042`, `DurationMS: 12345`, `NumTurns: 7`, and a non-nil `Usage` populates all four corresponding `ParserResult` fields exactly.
5. Feeding a result message with `Result: "Here is the answer"` sets `ParserResult.FinalText` to `"Here is the answer"`.
6. Feeding zero messages returns a zero-valued `ParserResult` (no panic, empty slices, zero numerics).
7. `ParserResult.Summary` matches `Tracker.Summary()` -- the parser must delegate, not duplicate.

**Test cases (file: `internal/metrics/parser_test.go`):**

- **Test_StreamParser_InitMessage**: "Given a system init message with tools [Read, Write, Bash, Glob, Grep, mcp__github__create_pr, mcp__context7__resolve], the parser should capture all 7 tools in InitTools. The MCP tools should just be strings at this point -- classification happens in ToolCalls, not InitTools."

- **Test_StreamParser_MultipleAssistantMessages**: "Given a stream that has an init message followed by 3 assistant messages -- the first with a single Bash tool_use, the second with parallel Read and Grep tool_use blocks, the third with an mcp__github__create_pr tool_use -- the parser should record 4 total ToolCalls. The summary should be {Bash: 1, Read: 1, Grep: 1, mcp__github__create_pr: 1}. The Bash/Read/Grep calls should be BuiltIn category and the mcp call should be MCP category."

- **Test_StreamParser_ResultMessage**: "Given a result message with cost $0.087, duration 34200ms, 12 turns, and usage {input: 15000, output: 3200, cache_creation: 500, cache_read: 8000}, the parser should capture all of these exactly. FinalText should equal the Result field value."

- **Test_StreamParser_EmptyStream**: "If I create a StreamParser and immediately call Result() without feeding any messages, I should get back zeroed-out ParserResult with nil/empty slices and zero numerics. No panic."

- **Test_StreamParser_IgnoresUnknownTypes**: "Given a message with Type set to 'ping' (which does not exist in the SDK), ProcessMessage should not panic and should not affect the ParserResult in any way. Feed it between two valid messages and verify nothing is corrupted."

- **Test_StreamParser_FullConversation**: "Simulate a realistic 5-message conversation: init with 10 tools including 2 MCP tools, assistant with thinking + Bash tool_use, user with tool_result, assistant with Read + Edit tool_use blocks, and a result message. Verify InitTools has 10 entries, ToolCalls has 3 entries (Bash, Read, Edit), cost/duration/turns all populated from the result, and FinalText is set. This is the integration-style unit test that proves the parser handles a realistic stream end-to-end."

- **Test_StreamParser_RepeatedInit**: "If two init messages arrive (which can happen when Claude restarts mid-session), the second one should overwrite InitTools. Feed init with [Read, Write], then init with [Read, Write, Bash], verify InitTools is [Read, Write, Bash]."

---

### Stage 2.2: Container Executor Interface

**What to build:**

- File: `internal/docker/executor.go`
- Interface: `Executor`
- File: `internal/docker/executor_docker.go` (real implementation wrapping `Manager`)
- File: `internal/docker/executor_mock.go` (mock for testing)

Extract a testable interface from the `Manager` so the runner can be tested without Docker:

```go
type Executor interface {
    Exec(ctx context.Context, containerID string, cmd []string) (stdout io.Reader, stderr io.Reader, exitCode int, err error)
}
```

The real implementation (`DockerExecutor`) wraps `Manager.ExecInContainer()` and passes through all return values (stdout, stderr, exitCode, error). Since both interfaces now return exit codes, `DockerExecutor` is a thin adapter. The mock implementation (`MockExecutor`) returns pre-configured responses keyed by command pattern (exact match on `cmd[0]` or a custom matcher).

The mock must support:
- Returning a specific `io.Reader` as stdout (for stream-json simulation)
- Returning a specific exit code (for evaluation testing)
- Recording all commands that were executed (for assertion)

**Pass/fail criteria:**

1. `Executor` interface is defined with the `Exec` method signature above.
2. `DockerExecutor` satisfies `Executor` (compile-time check: `var _ Executor = (*DockerExecutor)(nil)`).
3. `MockExecutor` satisfies `Executor` (compile-time check: `var _ Executor = (*MockExecutor)(nil)`).
4. `MockExecutor` can be configured to return different stdout content and exit codes for different commands.
5. `MockExecutor.Commands()` returns the list of all commands that were passed to `Exec`, in order.

**Test cases (file: `internal/docker/executor_test.go`):**

- **Test_MockExecutor_ReturnsConfiguredOutput**: "Set up a MockExecutor that returns 'hello world\n' on stdout for any command starting with 'echo'. Call Exec with ['echo', 'hello world']. Verify stdout reads as 'hello world\n' and exit code is 0."

- **Test_MockExecutor_RecordsCommands**: "Run three different commands through a MockExecutor. Verify Commands() returns all three in order. This is how the runner tests will assert that setup commands were executed before claude was invoked."

- **Test_MockExecutor_ExitCodeControl**: "Configure the mock to return exit code 1 for commands starting with 'false' and exit code 0 for everything else. Run both. Verify the exit codes match. This directly supports the evaluation test path."

---

### Stage 2.3: Runner Setup and Execution

**What to build:**

- File: `internal/docker/runner.go`
- Types: `RunConfig`, `Runner`
- Functions: `NewRunner(executor Executor) *Runner`, `(r *Runner) Run(ctx context.Context, cfg RunConfig) (*metrics.RunResult, error)`

```go
type RunConfig struct {
    ContainerID string
    Prompt      string
    MaxTurns    int
    Timeout     time.Duration
    WorkDir     string
    SetupCmds   []string
    Evaluation  corpus.Evaluation
}
```

The `Run` method executes this sequence:
1. Run each setup command via `executor.Exec(ctx, id, ["sh", "-c", cmd])`. If any returns non-zero exit code, return an error immediately with the command that failed.
2. Build the claude command: `["claude", "--print", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions", "--max-turns", strconv.Itoa(maxTurns), prompt]`.
3. Execute the claude command via `executor.Exec`.
4. Read stdout line-by-line with `bufio.Scanner`. For each line, `json.Unmarshal` into `claude.StreamMessage` and feed to `StreamParser.ProcessMessage()`. Skip lines that fail to unmarshal (stderr leakage, non-JSON output).
5. After the stream ends, run evaluation via `r.evaluate()`.
6. Assemble and return `*metrics.RunResult` from `StreamParser.Result()` plus evaluation outcome.

**Pass/fail criteria:**

1. Given a `RunConfig` with two setup commands, the runner calls `executor.Exec` exactly twice before the claude command, in order.
2. If the first setup command fails (exit code != 0), the runner returns an error containing the failed command string. The claude command is never executed.
3. The claude command is constructed with all expected flags in the correct order. `--max-turns` uses the value from `RunConfig.MaxTurns`.
4. Given a mock that returns 5 lines of valid stream-json on stdout for the claude command, the runner produces a `RunResult` with the correct metrics (cost, duration, turns, tool calls).
5. Given a mock that returns some invalid lines (e.g., "INFO: starting up") mixed with valid stream-json, the runner silently skips the invalid lines and still parses the valid ones correctly.
6. The runner respects context cancellation. If the context is cancelled mid-stream, `Run` returns `context.Canceled` or `context.DeadlineExceeded`.

**Test cases (file: `internal/docker/runner_test.go`):**

- **Test_Runner_SetupCommandsExecuteInOrder**: "Configure RunConfig with setup commands ['git clone https://github.com/example/repo /workspace', 'cd /workspace && go mod download']. Use a MockExecutor. After Run completes, verify Commands() shows both setup commands were executed before the claude command. The claude command should be the third Exec call."

- **Test_Runner_SetupFailureAborts**: "First setup command returns exit code 0, second returns exit code 1 with stderr 'permission denied'. The runner should return an error mentioning the failed command. The mock's Commands() list should have exactly 2 entries -- the claude command should never appear."

- **Test_Runner_StreamParsing**: "Mock the claude command to return stdout with these lines: an init message (tools: [Read, Write, Bash]), an assistant message with a Bash tool_use (command: 'go test ./...'), a user message with tool_result, an assistant message with text 'All tests pass', and a result message (cost: 0.034, turns: 3, duration: 8500). Verify the RunResult has InitTools=[Read, Write, Bash], ToolCalls with 1 Bash call, CostUSD=0.034, NumTurns=3, DurationMS=8500."

- **Test_Runner_SkipsGarbageLines**: "Mix valid stream-json lines with lines like '2024-01-15 DEBUG: connecting to API', an empty line, and a line that is `{invalid json`. Verify the parser still extracts the correct data from the valid lines. Zero errors returned -- garbage is silently dropped."

- **Test_Runner_BuildsClaudeCommand**: "Given MaxTurns=15 and Prompt='Fix the nil pointer in handler.go', verify the command passed to executor.Exec is exactly ['claude', '--print', '--output-format', 'stream-json', '--verbose', '--dangerously-skip-permissions', '--max-turns', '15', 'Fix the nil pointer in handler.go']."

- **Test_Runner_ContextCancellation**: "Create a context with a 50ms timeout. Mock the claude command's stdout to be a reader that blocks for 5 seconds. Verify Run returns an error that wraps context.DeadlineExceeded."

---

### Stage 2.4: Evaluation Execution

**What to build:**

- Add to: `internal/docker/runner.go`
- Function: `(r *Runner) evaluate(ctx context.Context, containerID string, eval corpus.Evaluation) (success bool, score float64, err error)`

Three evaluation methods:

| Method | Implementation |
|--------|---------------|
| `command` | `executor.Exec(ctx, id, ["sh", "-c", eval.Command])`. Success = exit code matches `eval.ExpectedExitCode`. Score = 1.0 if success, 0.0 if not. |
| `test_passes` | `executor.Exec(ctx, id, ["sh", "-c", eval.Command])`. Success = exit code == 0. Score = 1.0 if exit 0, 0.0 if not. (Fractional scoring from test output parsing is deferred to a later phase.) |
| `file_contains` | `executor.Exec(ctx, id, ["grep", "-qF", eval.Contains, eval.FilePath])`. Success = exit code == 0. Score = 1.0 if found, 0.0 if not. Note: `-F` (fixed string) avoids regex interpretation of special chars in `eval.Contains`. |

If `eval.Method` is empty or unrecognized, return `(false, 0.0, fmt.Errorf("unknown evaluation method: %q", eval.Method))`.

**Pass/fail criteria:**

1. `command` method: exit code 0 with `ExpectedExitCode: 0` returns `(true, 1.0, nil)`.
2. `command` method: exit code 1 with `ExpectedExitCode: 0` returns `(false, 0.0, nil)`.
3. `command` method: exit code 1 with `ExpectedExitCode: 1` returns `(true, 1.0, nil)` -- intentional failure matching.
4. `test_passes` method: exit code 0 returns `(true, 1.0, nil)`.
5. `test_passes` method: exit code 2 returns `(false, 0.0, nil)`.
6. `file_contains` method: grep finds the string (exit 0) returns `(true, 1.0, nil)`.
7. `file_contains` method: grep does not find the string (exit 1) returns `(false, 0.0, nil)`.
8. Empty method returns an error with the string "unknown evaluation method".
9. `evaluate` never panics on any input combination.

**Test cases (file: `internal/docker/runner_test.go`, in the same file as Stage 2.3):**

- **Test_Evaluate_CommandSuccess**: "Evaluation method is 'command', command is 'go vet ./...', expected exit code 0. The mock returns exit code 0. evaluate should return success=true, score=1.0, err=nil."

- **Test_Evaluate_CommandFailure**: "Same as above but the mock returns exit code 2. evaluate should return success=false, score=0.0, err=nil. Note: this is not an error -- the evaluation ran fine, the code under test just failed the check."

- **Test_Evaluate_CommandExpectsNonZero**: "Some evaluations intentionally expect failure. Method is 'command', expected exit code 1, mock returns exit code 1. evaluate should return success=true, score=1.0. This covers the case where we want to verify that broken code still fails."

- **Test_Evaluate_TestPasses**: "Method is 'test_passes', command is 'npm test -- --grep refresh'. Mock returns exit code 0. Success=true, score=1.0."

- **Test_Evaluate_TestFails**: "Same as above but mock returns exit code 1 (test failure). Success=false, score=0.0. No error -- the test suite ran, it just had failures."

- **Test_Evaluate_FileContainsFound**: "Method is 'file_contains', file_path is '/workspace/project/main.go', contains is 'func handleNilPointer'. Mock is configured so that when ['grep', '-qF', 'func handleNilPointer', '/workspace/project/main.go'] is executed, it returns exit code 0. evaluate returns success=true, score=1.0."

- **Test_Evaluate_FileContainsNotFound**: "Same setup but mock returns exit code 1 for the grep. evaluate returns success=false, score=0.0."

- **Test_Evaluate_UnknownMethod**: "Method is 'llm_judge' (not implemented yet). evaluate should return success=false, score=0.0, and an error whose message contains 'unknown evaluation method'."

- **Test_Evaluate_EmptyMethod**: "Method is empty string. Same behavior as unknown -- returns an error."

---

### Stage 2.5: Runner End-to-End Integration (Unit Tests with Mocks)

**What to build:**

No new production code. This stage writes integration-level tests that exercise `Runner.Run()` from start to finish using `MockExecutor`, verifying the complete pipeline: setup commands, claude execution, stream parsing, evaluation, and `RunResult` assembly.

**Pass/fail criteria:**

1. A full `Run()` call with 2 setup commands, a multi-message stream, and a `command` evaluation produces a `RunResult` where: `Success` matches the evaluation outcome, `Score` is 1.0 or 0.0, `CostUSD`/`DurationMS`/`NumTurns` come from the result message, `InitTools` come from the init message, `ToolCalls` are classified correctly, and `ToolCallSummary` counts match.
2. A `Run()` where Claude produces no tool calls but writes a text answer still returns a valid `RunResult` with empty `ToolCalls` and the `FinalText` populated.
3. A `Run()` where the claude command exits with a non-zero code AND the stream parser captured zero messages (Claude CLI crash) returns an error, not a partial `RunResult`. If the stream contained valid messages before the crash, the runner still returns a `RunResult` with whatever metrics were captured, and sets `ErrorMsg` to indicate the crash.
4. A `Run()` with an MCP-heavy conversation (tools like `mcp__github__create_pr`, `mcp__context7__resolve`, `mcp__playwright__navigate`) correctly classifies all tool calls as `MCP` category, while standard tools like `Read` and `Bash` remain `BuiltIn`.

**Test cases (file: `internal/docker/runner_integration_test.go`):**

- **Test_Runner_FullPipeline_BugFix**: "Simulate a bug-fix task end-to-end. Setup: ['cd /workspace && go mod download']. The claude stream has: init (tools: [Read, Write, Edit, Bash, Glob, Grep]), assistant with Read tool_use (file: 'handler.go'), user with tool_result (file contents), assistant with thinking + Edit tool_use (fixing nil check), user with tool_result (edit applied), assistant with Bash tool_use ('go vet ./...'), user with tool_result (exit 0), assistant with text 'Fixed the nil pointer', result (cost: 0.023, turns: 4, duration: 15000, usage: {input: 8000, output: 1200}). Evaluation: method=command, command='go vet ./...', expected_exit_code=0, mock returns exit 0. Verify RunResult: Success=true, Score=1.0, CostUSD=0.023, NumTurns=4, DurationMS=15000, InitTools has 6 entries, ToolCalls has 3 (Read, Edit, Bash), all BuiltIn category, ToolCallSummary={Read:1, Edit:1, Bash:1}, FinalText='Fixed the nil pointer'."

- **Test_Runner_FullPipeline_WithMCPTools**: "Simulate a task where Claude uses MCP tools. Init tools include [Read, Write, Bash, mcp__github__create_pr, mcp__github__get_issue, Skill, Task]. The stream includes: assistant with mcp__github__get_issue tool_use, assistant with Read tool_use and Edit tool_use (parallel), assistant with mcp__github__create_pr tool_use, result message. Verify: ToolCalls has 4 entries. mcp__github__get_issue and mcp__github__create_pr are MCP category. Read and Edit are BuiltIn. ToolCallSummary has all 4 tools with count 1 each."

- **Test_Runner_FullPipeline_TextOnlyResponse**: "Claude just answers with text, no tool calls. Stream: init, assistant with text 'The answer is 42', result. Evaluation passes. Verify: ToolCalls is empty, ToolCallSummary is empty map, FinalText is 'The answer is 42', Success=true based on evaluation."

- **Test_Runner_FullPipeline_EvaluationFails**: "Everything runs fine but the evaluation command returns exit code 1 (the code Claude wrote does not pass tests). Verify: RunResult.Success=false, Score=0.0, but all the metrics (cost, duration, turns, tool calls) are still fully populated. A failed evaluation does not mean missing metrics."

- **Test_Runner_ClaudeCrash**: "The mock returns exit code 1 for the claude command itself (not the evaluation -- the claude CLI process crashed). Run should return an error. This is different from a failed evaluation -- a crashed CLI is an infrastructure error, not a test result."

## Test Strategy

- **Parser tests:** Feed known stream-json lines (as `claude.StreamMessage` values) and verify extracted metrics match expected values. Pure unit tests, no Docker.
- **Runner tests:** Use mock `Executor` interface to simulate `Exec` returning pre-built stream-json. Verify the full pipeline from raw lines to `RunResult`.
- **Evaluation tests:** Mock `Executor.Exec` with controlled exit codes. Verify success/failure mapping.
