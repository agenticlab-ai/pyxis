// Package cursor is the Cursor AgentRuntime adapter. It self-registers as the
// "cursor" backend and drives one task per fresh, non-interactive `cursor-agent`
// process — no session is reused across tasks (DINV-11, the charter-bleed
// guard). The CLI refuses to run without a controlling TTY, so each process is
// spawned under a pseudo-terminal via the POSIX `script` shim (design:
// implement-now#3.7 R6; see pty.go). Completion is read from the process exit;
// the CLI's JSON output is parsed for token telemetry. Native Cursor model
// parameter forms come from the harness-config catalog (AP-1, AP-21).
//
// The verified headless invocation is
//
//	cursor-agent -p --output-format json --model <m> --force <brief>
//
// run under a pty (design-brainstorm:implement-now-tech-design#F-9, §3.7). The
// brief is a positional argument, not stdin: under a pty the line discipline
// echoes anything piped to stdin back into the captured output, which would
// pollute the result JSON, so Cursor departs from the claude adapter's stdin
// delivery here (a task charter is well within the OS argument limit). The exec
// is injected (see commander), so command construction (including the pty wrap),
// JSON parsing, and the auth-degrade path are all unit-tested with no
// real `cursor-agent` binary and no real pty.
package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// harnessID is this adapter's registry key.
const harnessID = entity.Cursor

// binary is the Cursor CLI executable.
const binary = "cursor-agent"

func init() {
	runtime.Register(
		harnessID,
		func() (port.AgentRuntime, error) { return New() },
		func(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
			return New(WithProfiles(profiles))
		},
	)
}

// Adapter implements port.AgentRuntime over the Cursor CLI.
type Adapter struct {
	cmd         commander
	goos        string                          // host OS, selects the `script` grammar; "" → goruntime.GOOS
	authProbe   func(ctx context.Context) error // nil → a.defaultAuthProbe
	fence       *runtime.AdapterFence
	authority   *runtime.FenceAuthority
	profiles    map[nativerouting.Role][]nativerouting.ProfileSpec
	profilesSet bool
}

var _ port.AgentRuntime = (*Adapter)(nil)
var _ port.FencedRuntime = (*Adapter)(nil)

// Option customizes an Adapter. The exec, OS, and auth-probe seams are
// unexported and used only by tests.
type Option func(*Adapter)

// withCommander injects a fake process spawner (tests only).
func withCommander(c commander) Option { return func(a *Adapter) { a.cmd = c } }

// WithProfiles installs the model catalog. Production passes the merged
// repository configuration; tests that omit it load that same configuration.
func WithProfiles(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) Option {
	return func(a *Adapter) {
		a.profiles = nativerouting.CloneProfiles(profiles)
		a.profilesSet = true
	}
}

func WithFenceAuthority(authority *runtime.FenceAuthority) Option {
	return func(a *Adapter) { a.authority = authority }
}

// withGOOS pins the host-OS branch of the pty wrap (tests only), so the
// util-linux grammar is exercised from a non-Linux box.
func withGOOS(goos string) Option { return func(a *Adapter) { a.goos = goos } }

// withAuthProbe injects a fake auth preflight (tests only).
func withAuthProbe(f func(ctx context.Context) error) Option {
	return func(a *Adapter) { a.authProbe = f }
}

// New builds a Cursor adapter with production defaults, applying any options.
func New(opts ...Option) (*Adapter, error) {
	authority, err := runtime.DefaultFenceAuthority()
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		cmd: osCommander{}, goos: goruntime.GOOS, authority: authority,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.profiles, err = harnessconfig.ResolveProfiles(harnessID, a.profilesSet, a.profiles)
	if err != nil {
		return nil, err
	}
	if a.goos == "" {
		a.goos = goruntime.GOOS
	}
	a.fence, err = runtime.NewAdapterFence(a.authority, harnessID)
	if err != nil {
		return nil, fmt.Errorf("cursor: configure launch fence: %w", err)
	}
	return a, nil
}

// Launch spawns a fresh, non-interactive `cursor-agent` process for one task in
// the task's worktree, wrapped in a pty, and returns a handle to await or kill.
// The assembled brief is the process's only instruction (prd:implement-now/FR-19),
// delivered as a positional argument; no session is reused (DINV-11).
func (a *Adapter) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	if strings.TrimSpace(spec.Model) == "" || strings.TrimSpace(spec.Effort) == "" {
		return nil, fmt.Errorf("cursor: launch needs a selected model parameter and decoded effort (got model=%q effort=%q)", spec.Model, spec.Effort)
	}
	brief, err := runtime.AssembleBriefContext(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("cursor: assemble brief: %w", err)
	}
	// Wrap the verified invocation in the `script` pty shim (R6). The wrap is
	// argv, so the injected commander records it and the production path stays a
	// plain os/exec child.
	innerArgs := a.launchArgs(spec, brief)
	capture, err := a.decodeLaunchCapture(innerArgs)
	if err != nil {
		return nil, err
	}
	if capture.NativeEffort != spec.Effort {
		return nil, fmt.Errorf("cursor: launch model parameter %q decodes to effort %q, not selected effort %q", spec.Model, capture.NativeEffort, spec.Effort)
	}
	name, args := ptyCommand(a.goos, binary, innerArgs)
	proc, err := a.cmd.start(ctx, commandSpec{
		Access: runtime.LaunchAccess(spec),
		Name:   name,
		Args:   args,
		Dir:    spec.Workdir,
		Env:    runtime.LaunchEnvironment(spec, "CURSOR_API_KEY", "CURSOR_AUTH_TOKEN"),
	})
	if err != nil {
		return nil, fmt.Errorf("cursor: launch fresh process in %q: %w", spec.Workdir, err)
	}
	return &handle{proc: proc, capture: capture, observe: port.ObserveInvocation(ctx, harnessID, spec.Purpose, capture)}, nil
}

func (a *Adapter) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	return a.fence.Activate(attempt, generation, proof)
}

func (a *Adapter) LaunchFenced(ctx context.Context, spec port.LaunchSpec, capability port.LaunchCapability) (port.Handle, error) {
	return a.fence.Launch(ctx, capability, func() (port.Handle, error) { return a.Launch(ctx, spec) })
}

func (a *Adapter) FenceAndObserve(ctx context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	return a.fence.FenceAndObserve(ctx, attempt, generation)
}

func (a *Adapter) AuthorizeTerminal(capability port.LaunchCapability) error {
	return a.fence.AuthorizeTerminal(capability)
}

// launchArgs builds the verified headless invocation (before the pty wrap). The
// brief is the final positional argument (see the package doc for why Cursor
// uses a positional and not stdin). No `--resume`/`--continue` session flag is
// ever added: a fresh process per task is the whole point (DINV-11). Cursor has
// no native turn-bound flag, so the uniform cap is enforced by the Worker via
// Kill (§7 Q3); and no `--effort` flag exists — effort is folded into the
// selected model-parameter form, so only `--model` is emitted.
func (a *Adapter) launchArgs(spec port.LaunchSpec, brief string) []string {
	args := []string{
		"-p",
		"--output-format", "json",
		"--model", spec.Model,
	}
	if spec.Purpose == port.LaunchPurposeReview {
		args[2] = "stream-json" // retain native tool/subagent evidence, not just the terminal claim
	}
	if spec.Purpose != port.LaunchPurposeReview {
		args = append(args, "--force")
	} else {
		// The shared runtime denies source and Git writes. Keep ordinary tools
		// available without layering a second sandbox over the process guard.
		args = append(args, "--force", "--sandbox", "disabled", "--trust")
	}
	return append(args, brief)
}

// Await blocks until the launched process finishes and reports its token usage,
// parsed from the CLI's JSON. A non-nil error is an operational failure (the
// process could not be run or awaited, or the context was cancelled); a nil
// error means the process finished — whether it produced a usable artifact is
// decided separately, by verification (DINV-3, AP-13). Context cancellation
// kills the process and returns promptly.
func (a *Adapter) Await(ctx context.Context, h port.Handle) (result port.RunResult, awaitErr error) {
	defer func() {
		if !errors.Is(awaitErr, port.ErrNotQuiescent) {
			a.fence.MarkTerminal(h)
		}
	}()
	hh, ok := h.(*handle)
	if !ok {
		return port.RunResult{}, fmt.Errorf("cursor: await: foreign handle %T", h)
	}

	defer func() {
		if hh.observe != nil && !errors.Is(awaitErr, port.ErrNotQuiescent) {
			hh.observe(result, awaitErr)
		}
	}()

	type waited struct {
		stdout []byte
		code   int
		err    error
	}
	done := make(chan waited, 1)
	go func() {
		out, code, err := hh.proc.wait()
		done <- waited{stdout: out, code: code, err: err}
	}()

	// Prefer cancellation deterministically: if the context is already done, do
	// not gamble on select's random choice between a ready result and a ready
	// ctx.Done — kill the child, drain the waiter, and report the cancellation.
	if err := ctx.Err(); err != nil {
		if killErr := hh.proc.kill(); killErr != nil {
			return port.RunResult{}, errors.Join(err, fmt.Errorf("%w: %w", port.ErrNotQuiescent, killErr))
		}
		<-done
		return port.RunResult{}, fmt.Errorf("cursor: await cancelled: %w", err)
	}

	select {
	case <-ctx.Done():
		// Cancellation: terminate the child and drain the waiter (no leak), then
		// report the cancellation as the operational failure.
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, errors.Join(ctx.Err(), fmt.Errorf("%w: %w", port.ErrNotQuiescent, err))
		}
		<-done
		return port.RunResult{}, fmt.Errorf("cursor: await cancelled: %w", ctx.Err())
	case w := <-done:
		// A clean direct-child exit is not proof that the agent session is gone:
		// CLIs may leave MCP or shell descendants in their process group. Quiesce
		// the whole group before the Worker can publish evidence or a reviewer can
		// receive fresh attribution material (DINV-2).
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, fmt.Errorf("cursor: %w: %w", port.ErrNotQuiescent, err)
		}
		if w.err != nil {
			return port.RunResult{}, fmt.Errorf("cursor: await process: %w", w.err)
		}
		res, err := parseResult(w.stdout)
		if err != nil {
			return port.RunResult{}, fmt.Errorf("cursor: parse result JSON (exit %d): %w", w.code, err)
		}
		if w.code != 0 && !res.Incomplete {
			return port.RunResult{}, fmt.Errorf("cursor: process exited %d", w.code)
		}
		return res, nil
	}
}

// Kill terminates a launched process (cancellation, or a breached bound). It is
// safe to call after the process has already exited.
func (a *Adapter) Kill(_ context.Context, h port.Handle) error {
	hh, ok := h.(*handle)
	if !ok {
		return fmt.Errorf("cursor: kill: foreign handle %T", h)
	}
	if err := hh.proc.kill(); err != nil {
		return fmt.Errorf("cursor: kill process: %w", err)
	}
	a.fence.MarkTerminal(h)
	return nil
}

// AuthCheck preflights install and authentication with two zero-cost probes —
// neither makes a model call — so a missing binary or credential degrades loudly
// here and consumes no Worker attempt (prd:implement-now/FR-16, A-11). The error
// states the remedy.
func (a *Adapter) AuthCheck(ctx context.Context) error {
	// Install probe: `cursor-agent --version` proves the CLI is present and
	// runnable. No pty is needed for a metadata subcommand.
	if _, code, err := a.cmd.run(ctx, commandSpec{Name: binary, Args: []string{"--version"}}); err != nil {
		return fmt.Errorf("cursor backend unavailable: `%s` CLI not found or not runnable (%w); install it (https://docs.cursor.com/en/cli) and re-run", binary, err)
	} else if code != 0 {
		return fmt.Errorf("cursor backend unavailable: `%s --version` exited %d; reinstall the CLI and re-run", binary, code)
	}

	// Auth probe: a credential must be resolvable without a model call.
	probe := a.authProbe
	if probe == nil {
		probe = a.defaultAuthProbe
	}
	if err := probe(ctx); err != nil {
		return fmt.Errorf("cursor backend unauthenticated: %w; run `%s login` once, or set CURSOR_API_KEY, then re-run", err, binary)
	}
	return nil
}

// handle is one launched `cursor-agent` process. The Worker treats it as opaque.
type handle struct {
	observe func(port.RunResult, error)
	proc    process
	capture port.LaunchCapture
}

var _ port.Handle = (*handle)(nil)

func (*handle) RuntimeHandle() {}

// LaunchCapture returns the settings observed in the exact inner Cursor argv
// before the platform pty wrapper was applied.
func (*Adapter) LaunchCapture(h port.Handle) (port.LaunchCapture, error) {
	hh, ok := h.(*handle)
	if !ok {
		return port.LaunchCapture{}, fmt.Errorf("cursor: launch capture: foreign handle %T", h)
	}
	return port.LaunchCapture{NativeModel: hh.capture.NativeModel, NativeEffort: hh.capture.NativeEffort, Controls: slices.Clone(hh.capture.Controls)}, nil
}

// cursorResult is the subset of `cursor-agent --output-format json` this adapter
// reads. It is parsed OPEN (unknown fields ignored) on purpose: this is
// third-party CLI output that grows fields over time, not one of our own
// schema-versioned config files, so the closed-parse rule (AP-6) does not apply
// here.
//
// Current Cursor usage uses camelCase; older releases used Claude-compatible
// snake_case. Read both without counting the same category twice. num_turns is
// not emitted in Cursor's result today; it is captured
// defensively so a forward-compatible addition is read without a code change,
// and defaults to zero (Turns is telemetry, never a success verdict — DINV-3).
type cursorResult struct {
	Type      string                     `json:"type"`
	CallID    string                     `json:"call_id"`
	ToolCall  map[string]json.RawMessage `json:"tool_call"`
	SessionID string                     `json:"session_id"`
	IsError   bool                       `json:"is_error"`
	Subtype   string                     `json:"subtype"`
	Result    string                     `json:"result"`
	NumTurns  *int                       `json:"num_turns"`
	Usage     *struct {
		CurrentInputTokens       *int `json:"inputTokens"`
		CurrentOutputTokens      *int `json:"outputTokens"`
		CurrentCacheWriteTokens  *int `json:"cacheWriteTokens"`
		CurrentCacheReadTokens   *int `json:"cacheReadTokens"`
		InputTokens              *int `json:"input_tokens"`
		OutputTokens             *int `json:"output_tokens"`
		CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     *int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// parseResult reads token/turn telemetry from the CLI's JSON result object.
// Because the process runs under a pty, the captured stream can carry terminal
// artifacts around the JSON (a leading EOT/^D byte, CR line endings, a trailing
// prompt), so the object is located by scanning to the first '{' and decoding a
// single value from there — tolerating both leading noise and trailing bytes.
// Tokens is the total across every usage category the CLI reported; Turns is the
// turn count when present (0 today).
func parseResult(stdout []byte) (port.RunResult, error) {
	start := bytes.IndexByte(stdout, '{')
	if start < 0 {
		return port.RunResult{}, errors.New("no JSON result object in cursor output")
	}
	remaining := stdout[start:]
	var terminal cursorResult
	streamed, completed := false, false
	sessionID := ""
	calls := map[string]bool{}
	toolsComplete := true
	var messages []string
	var children []port.NativeReviewChild
	childCalls := map[string]bool{}
	for len(remaining) > 0 {
		dec := json.NewDecoder(bytes.NewReader(remaining))
		var record cursorResult
		if err := dec.Decode(&record); err != nil {
			return port.RunResult{}, fmt.Errorf("decode cursor result: %w", err)
		}
		if record.SessionID != "" {
			sessionID = record.SessionID
		}
		if record.Type == "tool_call" && record.Subtype == "completed" {
			if raw, ok := record.ToolCall["taskToolCall"]; ok && (record.CallID == "" || !childCalls[record.CallID]) {
				var task struct {
					Result struct {
						Success *struct {
							AgentID           string `json:"agentId"`
							ConversationSteps []struct {
								Assistant struct {
									Text string `json:"text"`
								} `json:"assistantMessage"`
							} `json:"conversationSteps"`
						} `json:"success"`
					} `json:"result"`
				}
				if err := json.Unmarshal(raw, &task); err != nil {
					return port.RunResult{}, fmt.Errorf("decode cursor native child: %w", err)
				}
				child := port.NativeReviewChild{}
				if success := task.Result.Success; success != nil {
					child.ContextID, child.Completed = success.AgentID, true
					for _, step := range success.ConversationSteps {
						child.Output += step.Assistant.Text
					}
				}
				children = append(children, child)
				if record.CallID != "" {
					childCalls[record.CallID] = true
				}
			}
		}
		if record.Type == "tool_call" && record.Subtype == "started" {
			identified := false
			for kind, raw := range record.ToolCall {
				// Native tool_call objects also contain hook annotations and
				// other metadata of arbitrary JSON types. Only tagged tool
				// variants carry call identities; metadata cannot invent one.
				if !strings.HasSuffix(kind, "ToolCall") {
					continue
				}
				var tool struct {
					Args struct {
						ID string `json:"toolCallId"`
					} `json:"args"`
				}
				if err := json.Unmarshal(raw, &tool); err != nil {
					return port.RunResult{}, fmt.Errorf("decode cursor tool %s: %w", kind, err)
				}
				if id := strings.TrimSpace(tool.Args.ID); id != "" {
					calls[id], identified = true, true
				}
			}
			// Older streams expose call_id directly. Prefer nested native IDs
			// when both occur so one event cannot count twice under aliases.
			if !identified && strings.TrimSpace(record.CallID) != "" {
				calls[record.CallID], identified = true, true
			}
			toolsComplete = toolsComplete && identified
		}
		if record.Type == "assistant" {
			var message strings.Builder
			for _, content := range record.Message.Content {
				if content.Type == "text" {
					message.WriteString(content.Text)
				}
			}
			if message.Len() > 0 {
				messages = append(messages, message.String())
			}
		}
		if record.Type == "result" || record.Type == "" {
			terminal, completed = record, true
		} else {
			streamed = true
		}
		remaining = remaining[dec.InputOffset():]
		next := bytes.IndexByte(remaining, '{')
		if next < 0 {
			break
		}
		remaining = remaining[next:]
	}
	if !completed {
		return port.RunResult{Incomplete: true, Reason: "cursor_trace_incomplete: no terminal result", ToolCalls: len(calls), SessionID: sessionID}, nil
	}
	if !streamed {
		sessionID = "" // terminal-only legacy output does not add a continuity capability
	}
	r := terminal
	// Cursor concatenates streamed assistant messages in result without a
	// separator. Restore their boundaries only when their exact concatenation
	// equals the authoritative terminal text; never substitute stale stream
	// content for a different result. Markdown fences must start on their own
	// line, including a final machine handoff following earlier commentary.
	if len(messages) > 1 && strings.Join(messages, "") == r.Result {
		r.Result = strings.Join(messages, "\n\n")
	}
	tokens := 0
	tokensObserved := false
	if r.Usage != nil {
		for _, aliases := range [][2]*int{
			{r.Usage.CurrentInputTokens, r.Usage.InputTokens},
			{r.Usage.CurrentOutputTokens, r.Usage.OutputTokens},
			{r.Usage.CurrentCacheWriteTokens, r.Usage.CacheCreationInputTokens},
			{r.Usage.CurrentCacheReadTokens, r.Usage.CacheReadInputTokens},
		} {
			// Presence, including an explicit zero, wins over a legacy alias.
			value := aliases[0]
			if value == nil {
				value = aliases[1]
			}
			if value != nil {
				tokens += *value
				tokensObserved = true
			}
		}
	}

	reason := ""
	if r.IsError {
		reason = strings.TrimSpace(r.Subtype)
		if reason == "" {
			reason = strings.TrimSpace(r.Result)
		}
		if reason == "" {
			reason = "cursor_result_error"
		}
	}
	turns := 0
	if r.NumTurns != nil {
		turns = *r.NumTurns
	}
	var nativeChildren *port.NativeReviewChildren
	if len(children) > 0 {
		nativeChildren = &port.NativeReviewChildren{Children: children}
	}
	return port.RunResult{ReviewChildren: nativeChildren, Tokens: tokens, Turns: turns, ToolCalls: len(calls), TokensObserved: tokensObserved, TurnsObserved: r.NumTurns != nil, ToolCallsObserved: streamed && !r.IsError && toolsComplete, Incomplete: r.IsError, Reason: reason, Output: r.Result, SessionID: sessionID}, nil
}

// cursorStatus is the subset of `cursor-agent status --format json` the auth
// probe reads — parsed OPEN, third-party output.
type cursorStatus struct {
	IsAuthenticated bool `json:"isAuthenticated"`
}

// defaultAuthProbe resolves a Cursor credential without a model call: an env
// token, or the CLI's own `status --format json`, which reports isAuthenticated
// for a logged-in session (it exits 0 either way, so the flag — not the exit
// code — is authoritative). This covers both API-key and `login` setups, so a
// genuinely unauthenticated backend degrades loudly while an authenticated one
// preflights clean (FR-16).
func (a *Adapter) defaultAuthProbe(ctx context.Context) error {
	for _, key := range []string{"CURSOR_API_KEY", "CURSOR_AUTH_TOKEN"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return nil
		}
	}
	out, _, err := a.cmd.run(ctx, commandSpec{Name: binary, Args: []string{"status", "--format", "json"}})
	if err != nil {
		return fmt.Errorf("run `%s status`: %w", binary, err)
	}
	authed, err := parseAuthenticated(out)
	if err != nil {
		return err
	}
	if !authed {
		return errors.New("no credential found (checked CURSOR_API_KEY, CURSOR_AUTH_TOKEN, and `cursor-agent status`)")
	}
	return nil
}

// parseAuthenticated reads isAuthenticated from `cursor-agent status --format
// json`. The object is located from the first '{' for symmetry with the launch
// path (status is not run under a pty, but the tolerant scan costs nothing).
func parseAuthenticated(out []byte) (bool, error) {
	start := bytes.IndexByte(out, '{')
	if start < 0 {
		return false, errors.New("no JSON object in `cursor-agent status` output")
	}
	dec := json.NewDecoder(bytes.NewReader(out[start:]))
	var s cursorStatus
	if err := dec.Decode(&s); err != nil {
		return false, fmt.Errorf("decode cursor status: %w", err)
	}
	return s.IsAuthenticated, nil
}
