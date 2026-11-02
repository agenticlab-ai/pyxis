// Package opencode is the OpenCode AgentRuntime adapter. It self-registers as
// the "opencode" backend and drives one task per fresh, non-interactive
// `opencode` process — no session is reused across tasks (DINV-11, the
// charter-bleed guard). Completion is read from the process exit; the CLI's JSON
// output is parsed for token/turn telemetry. OpenCode's `provider/model`
// identifiers come from the harness-config catalog (AP-1, AP-21).
//
// The headless invocation is
//
//	opencode run --format json --model <provider/model> <brief>
//
// with the assembled brief delivered as the run message (the `run [message..]`
// positional), cwd = the task's Workspace (design-brainstorm:implement-now-tech-design#F-9,
// §3.7). OpenCode has no native turn cap (no `--max-turns` like claude/cursor),
// so the Worker enforces a uniform wall-clock/budget bound + Kill for this
// backend, keeping the cap uniform across CLIs (§3.7, §7 Q3).
//
// The `--format json` output is newline-delimited JSON. Current releases emit
// per-event records — `step_start`, `step_finish`, `text`, `tool_use`, `error` —
// carrying `sessionID`, a `part` with per-step `tokens`, and tool/error
// signalling; older releases emit a single terminal result object with `steps`
// and a top-level `tokens` map. parseResult accepts both, reporting tokens,
// turns, and tool calls with an observed flag per number, and it reads cached
// tokens and survives incomplete traces. The hermetic unit tests + the shared
// conformance suite are the coverage; the live availability gate
// (TestLive_OpenCodeAvailable) skips cleanly where the binary is absent.
//
// The exec is injected (see commander), so command construction, JSON parsing,
// routing catalog, typed launch capture, and auth-degrade path are all
// unit-tested with no real `opencode` binary (docs/tech-design/implement-now.md
// §3.10).
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// harnessID is this adapter's registry key.
const harnessID = entity.OpenCode

// binary is the OpenCode CLI executable.
const binary = "opencode"

func init() {
	runtime.Register(
		harnessID,
		func() (port.AgentRuntime, error) { return New() },
		func(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
			return New(WithProfiles(profiles))
		},
	)
}

// Adapter implements port.AgentRuntime over the OpenCode CLI.
type Adapter struct {
	cmd         commander
	authProbe   func(ctx context.Context) error // nil → defaultAuthProbe
	fence       *runtime.AdapterFence
	authority   *runtime.FenceAuthority
	profiles    map[nativerouting.Role][]nativerouting.ProfileSpec
	profilesSet bool
}

var _ port.AgentRuntime = (*Adapter)(nil)
var _ port.FencedRuntime = (*Adapter)(nil)

// Option customizes an Adapter. The exec and auth-probe seams are unexported
// and used only by tests. Unlike the claude adapter, opencode exposes no
// native turn-cap flag, so there is no WithMaxTurns seam — the turn bound is
// enforced uniformly by the Worker for this backend (§3.7, §7 Q3).
type Option func(*Adapter)

// WithProfiles installs the model catalog. Production passes the merged
// repository configuration; tests that omit it load that same configuration.
func WithProfiles(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) Option {
	return func(a *Adapter) {
		a.profiles = nativerouting.CloneProfiles(profiles)
		a.profilesSet = true
	}
}

// withCommander injects a fake process spawner (tests only).
func withCommander(c commander) Option { return func(a *Adapter) { a.cmd = c } }

func WithFenceAuthority(authority *runtime.FenceAuthority) Option {
	return func(a *Adapter) { a.authority = authority }
}

// withAuthProbe injects a fake auth preflight (tests only).
func withAuthProbe(f func(ctx context.Context) error) Option {
	return func(a *Adapter) { a.authProbe = f }
}

// New builds an OpenCode adapter with production defaults, applying any options.
func New(opts ...Option) (*Adapter, error) {
	authority, err := runtime.DefaultFenceAuthority()
	if err != nil {
		return nil, err
	}
	a := &Adapter{cmd: osCommander{}, authority: authority}
	for _, opt := range opts {
		opt(a)
	}
	a.profiles, err = harnessconfig.ResolveProfiles(harnessID, a.profilesSet, a.profiles)
	if err != nil {
		return nil, err
	}
	if err := validateProfileModels(a.profiles); err != nil {
		return nil, err
	}
	a.fence, err = runtime.NewAdapterFence(a.authority, harnessID)
	if err != nil {
		return nil, fmt.Errorf("opencode: configure launch fence: %w", err)
	}
	return a, nil
}

// Launch spawns a fresh, non-interactive `opencode` process for one task in the
// task's worktree and returns a handle to await or kill. The assembled brief is
// the process's only instruction (prd:implement-now/FR-19), delivered as the
// `run` message; no session is reused (DINV-11).
func (a *Adapter) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	if strings.TrimSpace(spec.Model) == "" || strings.TrimSpace(spec.Effort) == "" {
		return nil, fmt.Errorf("opencode: launch needs a resolved model and effort (got model=%q effort=%q)", spec.Model, spec.Effort)
	}
	brief, err := runtime.AssembleBriefContext(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("opencode: assemble brief: %w", err)
	}
	args := a.launchArgs(spec, brief)
	capture, err := captureLaunch(args)
	if err != nil {
		return nil, fmt.Errorf("opencode: capture typed launch: %w", err)
	}
	env := runtime.LaunchEnvironment(spec, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "XDG_DATA_HOME")
	if spec.Purpose == port.LaunchPurposeReview {
		env = append(env,
			`OPENCODE_PERMISSION={"*":"allow"}`,
			"OPENCODE_DISABLE_DEFAULT_PLUGINS=true",
			"OPENCODE_AUTO_SHARE=false",
		)
	}
	proc, err := a.cmd.start(ctx, commandSpec{
		Access: runtime.LaunchAccess(spec),
		Name:   binary,
		Args:   args,
		Dir:    spec.Workdir,
		Env:    env,
	})
	if err != nil {
		return nil, fmt.Errorf("opencode: launch fresh process in %q: %w", spec.Workdir, err)
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

// launchArgs builds the headless invocation `opencode run --format json --model
// <provider/model> <brief>`. The brief is the trailing `run [message..]`
// positional (design-brainstorm:implement-now-tech-design#F-9). Only `--model`
// is wired from the resolved route: opencode selects the model by `provider/model`
// and exposes no CLI reasoning-effort flag, so effort is carried as route
// metadata, not an argument. No `--resume`/`--session`/
// `-c` (continue) flag is ever added — a fresh process per task is the whole
// point (DINV-11) — and no `--max-turns`: opencode has no native turn cap
// (§3.7, §7 Q3).
func (a *Adapter) launchArgs(spec port.LaunchSpec, brief string) []string {
	role := nativerouting.RoleImplementation
	if spec.Purpose == port.LaunchPurposeReview {
		role = nativerouting.RoleReview
	}
	return nativeLaunchArgs(port.NativeLaunchSpec{
		NativeProfileID: "resolved-route", NativeModel: spec.Model, NativeEffort: nativeEffortNone,
		Controls: nativeControls(), Role: role,
	}, brief, resumeSession(spec))
}

// resumeSession is the session an implementation launch continues, or "" for a
// fresh conversation. OpenCode mints session identities itself, so only a
// resume (not a pre-assignment) reaches the argv; a review never resumes.
func resumeSession(spec port.LaunchSpec) string {
	if spec.Purpose == port.LaunchPurposeReview || spec.Session == nil || !spec.Session.Resume {
		return ""
	}
	return strings.TrimSpace(spec.Session.ID)
}

func nativeLaunchArgs(spec port.NativeLaunchSpec, brief, session string) []string {
	args := []string{
		"run",
		"--format", spec.Controls[0].StringValue,
		"--model", spec.NativeModel,
	}
	if session != "" {
		// Task session continuity: continue the task's own conversation.
		args = append(args, "--session", session)
	}
	if spec.Role == nativerouting.RoleReview {
		// The shared runtime enforces source immutability; the normal agent can
		// inspect and execute commands using writable private runtime state.
		args = append(args, "--agent", "build")
	}
	return append(args, brief)
}

// captureLaunch decodes the exact native settings from the command that will be
// spawned. Its closed shape prevents an injected flag from masquerading as a
// qualified launch.
func captureLaunch(args []string) (port.LaunchCapture, error) {
	// A resumed implementation launch carries `--session <id>` right after the
	// model; the session identity is not a routing setting, so it is stripped
	// before the closed-shape check rather than refusing attestation.
	if len(args) >= 7 && args[5] == "--session" && strings.TrimSpace(args[6]) != "" {
		args = append(append([]string{}, args[:5]...), args[7:]...)
	}
	if len(args) != 6 && len(args) != 8 {
		return port.LaunchCapture{}, errors.New("unexpected OpenCode native launch shape")
	}
	if args[0] != "run" || args[1] != "--format" || args[2] == "" || args[3] != "--model" || args[4] == "" {
		return port.LaunchCapture{}, errors.New("unexpected OpenCode native launch shape")
	}
	briefIndex := 5
	if len(args) == 8 {
		if args[5] != "--agent" || args[6] != "build" {
			return port.LaunchCapture{}, errors.New("unexpected OpenCode native launch shape")
		}
		briefIndex = 7
	}
	if strings.TrimSpace(args[briefIndex]) == "" {
		return port.LaunchCapture{}, errors.New("unexpected OpenCode native launch shape")
	}
	return port.LaunchCapture{
		NativeModel: args[4], NativeEffort: nativeEffortNone,
		Controls: []nativerouting.NativeControl{{Name: outputFormat, Type: nativerouting.ControlString, StringValue: args[2]}},
	}, nil
}

// LaunchCapture returns the provider/model and controls decoded from the actual
// command associated with a launched handle.
func (a *Adapter) LaunchCapture(h port.Handle) (port.LaunchCapture, error) {
	hh, ok := h.(*handle)
	if !ok {
		return port.LaunchCapture{}, fmt.Errorf("opencode: launch capture: foreign handle %T", h)
	}
	capture := hh.capture
	capture.Controls = append([]nativerouting.NativeControl(nil), capture.Controls...)
	return capture, nil
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
		return port.RunResult{}, fmt.Errorf("opencode: await: foreign handle %T", h)
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
		w := <-done
		return partialUsage(w.stdout), fmt.Errorf("opencode: await cancelled: %w", err)
	}

	select {
	case <-ctx.Done():
		// Cancellation: terminate the child and drain the waiter (no leak), then
		// report the cancellation as the operational failure.
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, errors.Join(ctx.Err(), fmt.Errorf("%w: %w", port.ErrNotQuiescent, err))
		}
		w := <-done
		return partialUsage(w.stdout), fmt.Errorf("opencode: await cancelled: %w", ctx.Err())
	case w := <-done:
		// A clean direct-child exit is not proof that the agent session is gone:
		// CLIs may leave MCP or shell descendants in their process group. Quiesce
		// the whole group before the Worker can publish evidence or a reviewer can
		// receive fresh attribution material (DINV-2).
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, fmt.Errorf("opencode: %w: %w", port.ErrNotQuiescent, err)
		}
		if w.err != nil {
			return partialUsage(w.stdout), fmt.Errorf("opencode: await process: %w", w.err)
		}
		res, err := parseResult(w.stdout)
		if err != nil {
			return port.RunResult{}, fmt.Errorf("opencode: parse result JSON (exit %d): %w", w.code, err)
		}
		if w.code != 0 && !res.Incomplete {
			return port.RunResult{}, fmt.Errorf("opencode: process exited %d", w.code)
		}
		return res, nil
	}
}

func partialUsage(stdout []byte) port.RunResult {
	res, err := parseResult(stdout)
	if err != nil {
		return port.RunResult{} // unparsable usage stays unavailable; Await retains the operational error
	}
	return res
}

// Kill terminates a launched process (cancellation, or a breached bound). It is
// safe to call after the process has already exited.
func (a *Adapter) Kill(_ context.Context, h port.Handle) error {
	hh, ok := h.(*handle)
	if !ok {
		return fmt.Errorf("opencode: kill: foreign handle %T", h)
	}
	if err := hh.proc.kill(); err != nil {
		return fmt.Errorf("opencode: kill process: %w", err)
	}
	a.fence.MarkTerminal(h)
	return nil
}

// AuthCheck preflights install and authentication with zero-cost probes —
// neither makes a model call — so a missing binary or credential degrades loudly
// here and consumes no Worker attempt (prd:implement-now/FR-16, A-11). The error
// states the remedy.
func (a *Adapter) AuthCheck(ctx context.Context) error {
	// Install probe: `opencode --version` proves the CLI is present and runnable.
	if _, code, err := a.cmd.run(ctx, commandSpec{Name: binary, Args: []string{"--version"}}); err != nil {
		return fmt.Errorf("opencode backend unavailable: `%s` CLI not found or not runnable (%w); install it (https://opencode.ai/docs) and re-run", binary, err)
	} else if code != 0 {
		return fmt.Errorf("opencode backend unavailable: `%s --version` exited %d; reinstall the CLI and re-run", binary, code)
	}

	// Auth probe: a credential must be resolvable without a model call.
	probe := a.authProbe
	if probe == nil {
		probe = defaultAuthProbe
	}
	if err := probe(ctx); err != nil {
		return fmt.Errorf("opencode backend unauthenticated: %w; run `%s auth login` to add a provider credential, or set ANTHROPIC_API_KEY, then re-run", err, binary)
	}
	return nil
}

// handle is one launched `opencode` process. The Worker treats it as opaque.
type handle struct {
	observe func(port.RunResult, error)
	proc    process
	capture port.LaunchCapture
}

var _ port.Handle = (*handle)(nil)

func (*handle) RuntimeHandle() {}

// opencodeResult is the subset of `opencode run --format json` this adapter
// reads. It carries both the current JSONL-event fields (`type`, `sessionID`,
// `part`) and the legacy terminal-result fields (`steps`, top-level `tokens`).
// It is parsed OPEN (unknown fields ignored) on purpose: this is third-party CLI
// output that grows fields over time, not one of our own schema-versioned config
// files, so the closed-parse rule (AP-6) does not apply here. A new event shape
// is absorbed by this struct and parseResult together, with the golden fixture in
// the test.
type opencodeResult struct {
	Type   string          `json:"type"`
	Error  json.RawMessage `json:"error"`
	Status string          `json:"status"`
	// SessionID rides current OpenCode events: the conversation a later launch
	// of the same task resumes (task session continuity).
	SessionID string `json:"sessionID"`
	// Text covers the terminal-result form; Part covers the raw JSON-event form
	// emitted by current OpenCode releases. The parser retains the last text part
	// as the review broker payload.
	Text string `json:"text"`
	Part *struct {
		Type   string          `json:"type"`
		Text   string          `json:"text"`
		Reason string          `json:"reason"`
		Tokens *opencodeTokens `json:"tokens"`
	} `json:"part"`
	Tokens *opencodeTokens `json:"tokens"`
	// Steps is the agentic step (turn) count of the run.
	Steps *int `json:"steps"`
}

type opencodeTokens struct {
	Input     *int `json:"input"`
	Output    *int `json:"output"`
	Reasoning *int `json:"reasoning"`
	Cache     struct {
		Read  *int `json:"read"`
		Write *int `json:"write"`
	} `json:"cache"`
}

func (u *opencodeTokens) total() (int, bool) {
	if u == nil {
		return 0, false
	}
	total, observed := 0, false
	for _, v := range []*int{u.Input, u.Output, u.Reasoning, u.Cache.Read, u.Cache.Write} {
		if v != nil {
			total += *v
			observed = true
		}
	}
	return total, observed
}

// parseResult reads token/turn/tool telemetry from the CLI's JSON. It accepts
// both output forms the CLI has emitted: the current newline-delimited JSONL
// event stream and the legacy single terminal result object.
//
// In the JSONL-event form each completed `step-finish` part contributes a turn
// and its `tokens` — input, output, reasoning, and cache read/write; `tool_use`
// events count tool calls; the last `text` part becomes the review payload; and
// `sessionID` carries the conversation id. Zero tool calls are only reported as
// observed when the trace is complete — the last event is a `step_finish` with
// reason `stop`. An `error` event, a non-null `error` field, or an error/failed
// `status` marks the trace incomplete, leaving tool accounting partial and
// unavailable. The legacy terminal object uses `steps` for turns and a top-level
// `tokens` map.
//
// Tokens, turns, and tool calls each carry an observed flag, so a genuinely
// missing usage record is distinguishable from an observed zero, and an
// incomplete trace still reports the usage it did capture.
func parseResult(stdout []byte) (port.RunResult, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return port.RunResult{}, errors.New("empty CLI output")
	}
	var records []opencodeResult
	for line := range bytes.SplitSeq(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var r opencodeResult
		if err := json.Unmarshal(line, &r); err != nil {
			return port.RunResult{}, fmt.Errorf("decode opencode result: %w", err)
		}
		records = append(records, r)
	}
	if len(records) == 0 {
		return port.RunResult{}, errors.New("no JSON events in opencode output")
	}
	var (
		output         string
		tokens         int
		turns          int
		toolCalls      int
		incomplete     bool
		reason         string
		nonRetryable   bool
		sessionID      string
		tokensObserved bool
		usageMissing   bool
		turnsObserved  bool
	)
	for _, record := range records {
		if record.Type == "tool_use" {
			toolCalls++
		}
		if id := strings.TrimSpace(record.SessionID); id != "" {
			sessionID = id
		}
		if record.Text != "" {
			output = record.Text
		}
		if record.Part != nil && record.Part.Type == "text" && record.Part.Text != "" {
			output = record.Part.Text
		}
		if record.Part != nil && record.Part.Type == "step-finish" {
			turns++
			turnsObserved = true
			value, known := record.Part.Tokens.total()
			tokens += value
			tokensObserved = tokensObserved || known
			usageMissing = usageMissing || !known
		}
		// Retain the legacy terminal-object shape for older OpenCode versions.
		if record.Type == "" && (record.Steps != nil || record.Tokens != nil) {
			if record.Steps != nil {
				turns += *record.Steps
				turnsObserved = true
			}
			value, known := record.Tokens.total()
			tokens += value
			tokensObserved = tokensObserved || known
			usageMissing = usageMissing || !known
		}
		if record.Type == "error" || (len(bytes.TrimSpace(record.Error)) > 0 && string(bytes.TrimSpace(record.Error)) != "null") || strings.EqualFold(record.Status, "error") || strings.EqualFold(record.Status, "failed") {
			incomplete = true
			reason = "opencode_result_error"
			if record.Status != "" {
				reason = record.Status
			}
			if detail, noRetry := nativeErrorDetail(record.Error); detail != "" {
				reason = detail
				nonRetryable = nonRetryable || noRetry
			}
		}
	}
	if turns == 0 && !incomplete {
		return port.RunResult{}, errors.New("no completed step in opencode output")
	}
	last := records[len(records)-1]
	// Only a complete native trace can prove zero tool calls. Legacy terminal
	// objects and truncated/error streams leave tool accounting unavailable or partial.
	toolCallsObserved := !incomplete && last.Type == "step_finish" && last.Part != nil && last.Part.Type == "step-finish" && last.Part.Reason == "stop"
	if !incomplete && last.Type != "" && !toolCallsObserved {
		incomplete, reason = true, "opencode_trace_incomplete: no terminal stop event"
	}
	return port.RunResult{Tokens: tokens, Turns: turns, ToolCalls: toolCalls, TokensObserved: tokensObserved && !usageMissing, TurnsObserved: turnsObserved, ToolCallsObserved: toolCallsObserved, Incomplete: incomplete, NonRetryable: nonRetryable, Reason: reason, Output: output, SessionID: sessionID}, nil
}

// nativeErrorDetail carries the provider's diagnostic and explicit retry policy,
// without copying response headers, bodies, or credentials into engine events.
func nativeErrorDetail(raw json.RawMessage) (string, bool) {
	var native struct {
		Name string `json:"name"`
		Data struct {
			Message     string `json:"message"`
			StatusCode  int    `json:"statusCode"`
			IsRetryable *bool  `json:"isRetryable"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &native) != nil || strings.TrimSpace(native.Data.Message) == "" {
		return "", false // legacy and unknown errors retain the generic native status
	}
	detail := strings.TrimSpace(native.Data.Message)
	if native.Data.StatusCode > 0 {
		detail = fmt.Sprintf("HTTP %d: %s", native.Data.StatusCode, detail)
	}
	if native.Name != "" {
		detail = native.Name + ": " + detail
	}
	return detail, native.Data.IsRetryable != nil && !*native.Data.IsRetryable
}

// defaultAuthProbe resolves an OpenCode credential without a model call: a
// provider env token, or the OpenCode on-disk auth store written by `opencode
// auth login`. The environment keys support Anthropic credentials; the
// auth store covers a credential added for any provider, so a genuinely
// unauthenticated backend degrades loudly while an authenticated one preflights
// clean (FR-16).
func defaultAuthProbe(_ context.Context) error {
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return nil
		}
	}
	if credentialStoreExists() {
		return nil
	}
	return errors.New("no credential found (checked ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, and the opencode auth store)")
}

// credentialStoreExists reports whether the OpenCode on-disk auth file is
// present (the `opencode auth login` case). OpenCode stores it under the XDG
// data directory: $XDG_DATA_HOME/opencode/auth.json, defaulting to
// ~/.local/share/opencode/auth.json.
func credentialStoreExists() bool {
	dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dir = filepath.Join(home, ".local", "share")
	}
	_, err := os.Stat(filepath.Join(dir, "opencode", "auth.json"))
	return err == nil
}
