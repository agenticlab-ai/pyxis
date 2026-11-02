// Package claude is the Claude Code AgentRuntime adapter. It self-registers as
// the "claude" backend and drives one task per fresh, non-interactive `claude`
// process — no session is reused across tasks (DINV-11, the charter-bleed
// guard). Completion is read from the process exit; the CLI's JSON output is
// parsed for token/turn telemetry. Native Claude model identifiers come from
// the harness-config catalog (AP-1, AP-21).
//
// The verified headless invocation is
//
//	claude -p --permission-mode bypassPermissions --model <m> --effort <e> \
//	    --output-format json --max-turns <N>
//
// with the assembled brief piped to stdin (design-brainstorm:implement-now-tech-design#F-8,
// §3.7). The exec is injected (see commander), so command construction, JSON
// parsing, native routing, and the auth-degrade path are
// all unit-tested with no real `claude` binary.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// harnessID is this adapter's registry key.
const harnessID = entity.Claude

// binary is the Claude Code CLI executable.
const binary = "claude"

// defaultMaxTurns is the turn bound wired into `--max-turns`. Zero means NO turn
// cap: the flag is omitted entirely and the Worker's wall-clock budget is the
// only bound on a launch (bound.go's awaitBounded, default 1h).
//
// The cap was 60, then 150 — each raise for the same reason, and each still a
// cliff. A turn cap cannot tell "looping uselessly" from "steadily working a big
// task": both spend turns. Wall-clock cannot either, but it at least bounds the
// thing an operator actually cares about, and it bounds it whatever the turns
// cost. Two bounds on one runaway only means the tighter one decides, and the
// tighter one here fired at 38 minutes on a task with 6 minutes of budget left,
// shipping half a harness (agenticlab-ai/pyxis#997).
//
// A launch is therefore still never unbounded — the bound moved from turns to
// time. Set a positive `executor.caps.worker_turns` to reinstate a turn cap.
const defaultMaxTurns = 0

func init() {
	runtime.Register(
		harnessID,
		func() (port.AgentRuntime, error) { return New() },
		func(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
			return New(WithProfiles(profiles))
		},
	)
}

// Adapter implements port.AgentRuntime over the Claude Code CLI.
type Adapter struct {
	cmd         commander
	maxTurns    int
	authProbe   func(ctx context.Context) error // nil → defaultAuthProbe
	fence       *runtime.AdapterFence
	authority   *runtime.FenceAuthority
	profiles    map[nativerouting.Role][]nativerouting.ProfileSpec
	profilesSet bool
}

var _ port.AgentRuntime = (*Adapter)(nil)
var _ port.FencedRuntime = (*Adapter)(nil)

// Option customizes an Adapter. WithMaxTurns is the config seam; the exec and
// auth-probe seams are unexported and used only by tests.
type Option func(*Adapter)

// WithProfiles installs the model catalog. Production passes the merged
// repository configuration; tests that omit it load that same configuration.
func WithProfiles(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) Option {
	return func(a *Adapter) {
		a.profiles = nativerouting.CloneProfiles(profiles)
		a.profilesSet = true
	}
}

// WithMaxTurns sets the `--max-turns` bound handed to each launch. A value <= 0
// means no turn cap: the flag is omitted and the Worker's wall-clock budget is
// the only bound. Both cases are honored, so a config that asks for no cap is
// not silently overridden by the built-in default.
func WithMaxTurns(n int) Option {
	return func(a *Adapter) {
		if n < 0 {
			n = 0
		}
		a.maxTurns = n
	}
}

// withCommander injects a fake process spawner (tests only).
func withCommander(c commander) Option { return func(a *Adapter) { a.cmd = c } }

// WithFenceAuthority joins this adapter to the runtime service's shared atomic
// generation fence. Supplying nil fails construction closed.
func WithFenceAuthority(authority *runtime.FenceAuthority) Option {
	return func(a *Adapter) { a.authority = authority }
}

// withAuthProbe injects a fake auth preflight (tests only).
func withAuthProbe(f func(ctx context.Context) error) Option {
	return func(a *Adapter) { a.authProbe = f }
}

// New builds a Claude adapter with production defaults, applying any options.
func New(opts ...Option) (*Adapter, error) {
	authority, err := runtime.DefaultFenceAuthority()
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		cmd:       osCommander{},
		maxTurns:  defaultMaxTurns,
		authority: authority,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.profiles, err = harnessconfig.ResolveProfiles(harnessID, a.profilesSet, a.profiles)
	if err != nil {
		return nil, err
	}
	a.fence, err = runtime.NewAdapterFence(a.authority, harnessID)
	if err != nil {
		return nil, fmt.Errorf("claude: configure launch fence: %w", err)
	}
	return a, nil
}

// Launch spawns a fresh, non-interactive `claude` process for one task in the
// task's worktree and returns a handle to await or kill. The assembled brief is
// the process's only instruction (prd:implement-now/FR-19), piped to stdin; no
// session is reused (DINV-11).
func (a *Adapter) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	if strings.TrimSpace(spec.Model) == "" || strings.TrimSpace(spec.Effort) == "" {
		return nil, fmt.Errorf("claude: launch needs a resolved model and effort (got model=%q effort=%q)", spec.Model, spec.Effort)
	}
	brief, err := runtime.AssembleBriefContext(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("claude: assemble brief: %w", err)
	}
	args := a.launchArgs(spec)
	capture, err := a.decodeLaunchCapture(args)
	if err != nil {
		return nil, fmt.Errorf("claude: capture native launch: %w", err)
	}
	proc, err := a.cmd.start(ctx, commandSpec{
		Access: runtime.LaunchAccess(spec),
		Name:   binary,
		Args:   args,
		Dir:    spec.Workdir,
		Stdin:  brief,
		Env:    runtime.LaunchEnvironment(spec, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CONFIG_DIR"),
	})
	if err != nil {
		return nil, fmt.Errorf("claude: launch fresh process in %q: %w", spec.Workdir, err)
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

// launchArgs builds the verified headless invocation. The prompt is delivered on
// stdin (not a positional arg), so a large charter is never bounded by the OS
// argument limit. No `--resume`/`--continue`/session flag is ever added: a fresh
// process per task is the whole point (DINV-11).
func (a *Adapter) launchArgs(spec port.LaunchSpec) []string {
	args := []string{
		"-p",
		// Source write authority is enforced by the shared process guard;
		// inspection commands must not wait for native approval prompts.
		"--permission-mode", "bypassPermissions",
		"--model", spec.Model,
		"--effort", spec.Effort,
		"--output-format", "json",
	}
	if spec.Purpose == port.LaunchPurposeReview {
		// A repository setting can otherwise enable a second OS sandbox even
		// with bypassPermissions. Pyxis owns this process tree's write guard.
		args = append(args, "--settings", reviewSettings)
	}
	// maxTurns <= 0 means no turn cap: omit the flag rather than pass a sentinel
	// the CLI would read as a real bound. The wall-clock budget still bounds the
	// launch (see defaultMaxTurns).
	if a.maxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(a.maxTurns))
	}
	// Task session continuity: an implementation launch may continue the
	// task's own conversation. A first launch pre-assigns the identity
	// (--session-id) so it is known before the process exists; a later launch
	// of the SAME task resumes it (--resume). A review launch never carries a
	// session (AssembleBrief refuses one), so the reviewer stays fresh.
	if spec.Purpose != port.LaunchPurposeReview && spec.Session != nil && strings.TrimSpace(spec.Session.ID) != "" {
		if spec.Session.Resume {
			args = append(args, "--resume", spec.Session.ID)
		} else {
			args = append(args, "--session-id", spec.Session.ID)
		}
	}
	return args
}

// Await blocks until the launched process finishes and reports its token/turn
// usage, parsed from the CLI's JSON. A non-nil error is an operational failure
// (the process could not be run or awaited, or the context was cancelled); a nil
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
		return port.RunResult{}, fmt.Errorf("claude: await: foreign handle %T", h)
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
		return port.RunResult{}, fmt.Errorf("claude: await cancelled: %w", err)
	}

	select {
	case <-ctx.Done():
		// Cancellation: terminate the child and drain the waiter (no leak), then
		// report the cancellation as the operational failure.
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, errors.Join(ctx.Err(), fmt.Errorf("%w: %w", port.ErrNotQuiescent, err))
		}
		<-done
		return port.RunResult{}, fmt.Errorf("claude: await cancelled: %w", ctx.Err())
	case w := <-done:
		// A clean direct-child exit is not proof that the agent session is gone:
		// CLIs may leave MCP or shell descendants in their process group. Quiesce
		// the whole group before the Worker can publish evidence or a reviewer can
		// receive fresh attribution material (DINV-2).
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, fmt.Errorf("claude: %w: %w", port.ErrNotQuiescent, err)
		}
		if w.err != nil {
			return port.RunResult{}, fmt.Errorf("claude: await process: %w", w.err)
		}
		res, err := parseResult(w.stdout)
		if err != nil {
			return port.RunResult{}, fmt.Errorf("claude: parse result JSON (exit %d): %w", w.code, err)
		}
		if w.code != 0 && !res.Incomplete {
			return port.RunResult{}, fmt.Errorf("claude: process exited %d", w.code)
		}
		return res, nil
	}
}

// Kill terminates a launched process (cancellation, or a breached bound). It is
// safe to call after the process has already exited.
func (a *Adapter) Kill(_ context.Context, h port.Handle) error {
	hh, ok := h.(*handle)
	if !ok {
		return fmt.Errorf("claude: kill: foreign handle %T", h)
	}
	if err := hh.proc.kill(); err != nil {
		return fmt.Errorf("claude: kill process: %w", err)
	}
	a.fence.MarkTerminal(h)
	return nil
}

// AuthCheck preflights install and authentication with two zero-cost probes —
// neither makes a model call — so a missing binary or credential degrades loudly
// here and consumes no Worker attempt (prd:implement-now/FR-16, A-11). The error
// states the remedy.
func (a *Adapter) AuthCheck(ctx context.Context) error {
	// Install probe: `claude --version` proves the CLI is present and runnable.
	if _, code, err := a.cmd.run(ctx, commandSpec{Name: binary, Args: []string{"--version"}}); err != nil {
		return fmt.Errorf("claude backend unavailable: `%s` CLI not found or not runnable (%w); install it (https://docs.claude.com/claude-code) and re-run", binary, err)
	} else if code != 0 {
		return fmt.Errorf("claude backend unavailable: `%s --version` exited %d; reinstall the CLI and re-run", binary, code)
	}

	// Auth probe: a credential must be resolvable without a model call.
	probe := a.authProbe
	if probe == nil {
		probe = defaultAuthProbe
	}
	if err := probe(ctx); err != nil {
		return fmt.Errorf("claude backend unauthenticated: %w; run `%s` once to /login, or set ANTHROPIC_API_KEY, then re-run", err, binary)
	}
	return nil
}

// handle is one launched `claude` process. The Worker treats it as opaque.
type handle struct {
	observe func(port.RunResult, error)
	proc    process
	capture port.LaunchCapture
}

var _ port.Handle = (*handle)(nil)

func (*handle) RuntimeHandle() {}

// claudeResult is the subset of `claude --output-format json` this adapter reads.
// It is parsed OPEN (unknown fields ignored) on purpose: this is third-party CLI
// output that grows fields over time, not one of our own schema-versioned config
// files, so the closed-parse rule (AP-6) does not apply here.
type claudeResult struct {
	NumTurns  *int   `json:"num_turns"`
	Subtype   string `json:"subtype"`    // "success", "error_max_turns", "error_during_execution", …
	IsError   bool   `json:"is_error"`   // the CLI's own flag that the run did not end cleanly
	SessionID string `json:"session_id"` // the conversation this run created or resumed
	Result    string `json:"result"`
	Usage     *struct {
		InputTokens              *int `json:"input_tokens"`
		OutputTokens             *int `json:"output_tokens"`
		CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     *int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// parseResult reads token/turn telemetry from the CLI's terminal result object.
// Tokens is the total across every usage category the CLI reported; Turns is the
// agentic loop's turn count. resultObject first isolates that object from either
// output shape the CLI emits (a bare object, or a transcript array — see below).
func parseResult(stdout []byte) (port.RunResult, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return port.RunResult{}, errors.New("empty CLI output")
	}
	obj, err := resultObject(trimmed)
	if err != nil {
		return port.RunResult{}, err
	}
	var r claudeResult
	if err := json.Unmarshal(obj, &r); err != nil {
		return port.RunResult{}, fmt.Errorf("decode claude result: %w", err)
	}
	tokens := 0
	tokensObserved := false
	if r.Usage != nil {
		for _, value := range []*int{r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheCreationInputTokens, r.Usage.CacheReadInputTokens} {
			if value != nil {
				tokens += *value
				tokensObserved = true
			}
		}
	}

	// The run is INCOMPLETE — it stopped before a clean finish — when the CLI
	// flags an error or reports any terminal subtype other than "success" (most
	// commonly "error_max_turns"). An empty subtype is treated as complete, so a
	// CLI that omits the field never yields a false truncation signal. Reason is
	// the failure detail, left empty on a clean run.
	incomplete := r.IsError || (r.Subtype != "" && r.Subtype != "success")
	reason := ""
	if incomplete {
		if reason = r.Subtype; reason == "" {
			reason = "error"
		}
	}
	turns := 0
	if r.NumTurns != nil {
		turns = *r.NumTurns
	}
	return port.RunResult{Tokens: tokens, Turns: turns, TokensObserved: tokensObserved, TurnsObserved: r.NumTurns != nil, Incomplete: incomplete, Reason: reason, Output: r.Result, SessionID: strings.TrimSpace(r.SessionID)}, nil
}

// resultObject isolates the CLI's terminal result object from either shape
// `claude -p --output-format json` emits: a bare result object (older CLI), or a
// JSON array of transcript messages whose final `type:"result"` element carries
// the telemetry (Claude Code CLI 2.x, observed on 2.1.218 — agenticlab-ai/pyxis).
// It scans the array from the end for the result marker and falls back to the
// last element, so the telemetry is still read if the marker is ever renamed.
// Parsed OPEN, like claudeResult itself — third-party CLI output, not our schema.
func resultObject(trimmed []byte) ([]byte, error) {
	switch trimmed[0] {
	case '{':
		return trimmed, nil
	case '[':
		var msgs []json.RawMessage
		if err := json.Unmarshal(trimmed, &msgs); err != nil {
			return nil, fmt.Errorf("decode claude result array: %w", err)
		}
		if len(msgs) == 0 {
			return nil, errors.New("claude result array is empty")
		}
		for _, msg := range slices.Backward(msgs) {
			var probe struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &probe) == nil && probe.Type == "result" {
				return msg, nil
			}
		}
		return msgs[len(msgs)-1], nil
	default:
		return nil, fmt.Errorf("decode claude result: unexpected output (not a JSON object or array)")
	}
}

// defaultAuthProbe resolves a Claude credential without a model call: an env
// token, or the Claude Code on-disk credential store. It covers both API-key and
// OAuth (`/login`) setups, so a genuinely unauthenticated backend degrades
// loudly while an authenticated one preflights clean (FR-16).
func defaultAuthProbe(_ context.Context) error {
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return nil
		}
	}
	if credentialStoreExists() {
		return nil
	}
	return errors.New("no credential found (checked ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, CLAUDE_CODE_OAUTH_TOKEN, and the Claude credential store)")
}

// credentialStoreExists reports whether a Claude Code credential is present
// without a model call (the OAuth-login case): the on-disk credential file, or —
// on macOS, where the CLI stores the credential in the login Keychain rather
// than a file — a Keychain item. Presence only; the secret is never read, so
// this stays a zero-cost preflight (A-11).
func credentialStoreExists() bool {
	dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dir = filepath.Join(home, ".claude")
	}
	if _, err := os.Stat(filepath.Join(dir, ".credentials.json")); err == nil {
		return true
	}
	// On macOS the OAuth credential lives in the login Keychain, not the file
	// probed above, so a logged-in `claude` shows no `.credentials.json`. Fall
	// back to a Keychain presence check there (this was the A-12 dogfood
	// false-negative: a fully logged-in macOS CLI reported "unauthenticated").
	if goruntime.GOOS == "darwin" {
		return keychainCredentialExists()
	}
	return false
}

// keychainCredentialExists reports whether the Claude Code credential item is
// present in the macOS login Keychain. `find-generic-password` without `-w`
// returns exit 0 when the item exists and never emits the secret, so this reads
// presence only. It is a package var so tests stub it — a real login-Keychain
// credential must never leak into a hermetic unit test.
var keychainCredentialExists = func() bool {
	return exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials").Run() == nil
}
