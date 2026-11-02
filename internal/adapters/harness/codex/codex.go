// Package codex is the Codex CLI AgentRuntime adapter. It self-registers as the
// "codex" backend and drives one task per fresh, non-interactive `codex exec`
// process — no session is reused across tasks (DINV-11, the charter-bleed
// guard). Completion is read from the process exit; the CLI's `--json` event
// stream is parsed for token/turn telemetry. Native Codex model identifiers come
// from the harness-config catalog (AP-1, AP-21).
//
// The verified headless invocation (codex-cli, `codex exec --help`, 2026-07-16)
// is
//
//	codex --no-daemon exec --json -m <model> \
//	    -c model_reasoning_effort="<effort>" \
//	    --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check -
//
// with the assembled brief piped to stdin (the `-` prompt reads instructions
// from stdin), cwd = the task's worktree (design-brainstorm:implement-now-tech-design#F-9,
// §3.7). Codex has no native turn cap — unlike claude's `--max-turns`, the
// uniform turn/wall-clock bound is Worker-enforced via Kill (§3.7, §7 Q3);
// reasoning effort has no native flag either, so it is set through the closed
// `model_reasoning_effort` config key. The exec is injected (see commander), so
// command construction, JSONL parsing, and the auth-degrade path are
// all unit-tested with no real `codex` binary.
package codex

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
const harnessID = entity.Codex

// binary is the Codex CLI executable.
const binary = "codex"

// eventTurnCompleted is the `codex exec --json` thread event that closes one
// agent turn and carries its token usage. Completion of the whole run is the
// process exit; this event is telemetry, not the done signal (AP-13, DINV-3).
const eventTurnCompleted = "turn.completed"

func init() {
	runtime.Register(
		harnessID,
		func() (port.AgentRuntime, error) { return New() },
		func(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
			return New(WithProfiles(profiles))
		},
	)
}

// Adapter implements port.AgentRuntime over the Codex CLI. It carries no native
// turn bound: Codex exposes no `--max-turns`, so the uniform cap is enforced by
// the Worker (Kill), keeping it consistent with the backends that do have a
// native flag (§3.7, §7 Q3).
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
// and used only by tests.
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

// withAuthProbe injects a fake auth preflight (tests only).
func withAuthProbe(f func(ctx context.Context) error) Option {
	return func(a *Adapter) { a.authProbe = f }
}

// New builds a Codex adapter with production defaults, applying any options.
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
	a.fence, err = runtime.NewAdapterFence(a.authority, harnessID)
	if err != nil {
		return nil, fmt.Errorf("codex: configure launch fence: %w", err)
	}
	return a, nil
}

// Launch spawns a fresh, non-interactive `codex exec` process for one task in
// the task's worktree and returns a handle to await or kill. The assembled brief
// is the process's only instruction (prd:implement-now/FR-19), piped to stdin;
// no session is reused (DINV-11).
func (a *Adapter) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	if strings.TrimSpace(spec.Model) == "" || strings.TrimSpace(spec.Effort) == "" {
		return nil, fmt.Errorf("codex: launch needs a resolved model and effort (got model=%q effort=%q)", spec.Model, spec.Effort)
	}
	brief, err := runtime.AssembleBriefContext(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("codex: assemble brief: %w", err)
	}
	args := a.launchArgs(spec)
	capture, err := captureNativeLaunch(args)
	if err != nil {
		return nil, fmt.Errorf("codex: capture actual launch: %w", err)
	}
	env := runtime.LaunchEnvironment(spec, "OPENAI_API_KEY", "CODEX_API_KEY", "CODEX_HOME")
	proc, err := a.cmd.start(ctx, commandSpec{
		Access: runtime.LaunchAccess(spec),
		Name:   binary,
		Args:   args,
		Dir:    spec.Workdir,
		Stdin:  brief,
		Env:    env,
	})
	if err != nil {
		return nil, fmt.Errorf("codex: launch fresh process in %q: %w", spec.Workdir, err)
	}
	h := &handle{proc: proc, observe: port.ObserveInvocation(ctx, harnessID, spec.Purpose, capture)}
	if spec.Purpose == port.LaunchPurposeReview {
		h.sessions = sessionsDir(env)
	}
	return h, nil
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
// stdin (the trailing `-`), not a positional arg, so a large charter is never
// bounded by the OS argument limit. No `resume`/`--last` session flag is ever
// added: a fresh process per task is the whole point (DINV-11). Reasoning effort
// rides the closed `model_reasoning_effort` config key because Codex exposes no
// native effort flag; `--dangerously-bypass-approvals-and-sandbox` is the
// autonomy flag (the more restrictive `-s workspace-write` is the alternative),
// and `--skip-git-repo-check` keeps the launch non-interactive regardless of the
// worktree's git state.
func (a *Adapter) launchArgs(spec port.LaunchSpec) []string {
	if spec.Purpose == port.LaunchPurposeReview {
		return []string{
			"-a", "never",
			"--no-daemon",
			"exec",
			"--json",
			"-m", spec.Model,
			"-c", fmt.Sprintf("model_reasoning_effort=%q", spec.Effort),
			// The shared runtime denies source and Git writes for this process
			// tree. A second native sandbox conflicts with that guard on macOS.
			"--sandbox", "danger-full-access",
			// Not ephemeral: Codex must record each lens child's session in
			// this launch's isolated home, where Await reads them. Nothing is
			// resumed; the home is disposable runtime state.
			"--ignore-user-config",
			"--skip-git-repo-check",
			"-",
		}
	}
	// Task session continuity: a later launch of the SAME task resumes its
	// conversation (`exec resume <thread-id> -`, the prompt still on stdin).
	// Codex mints thread identities itself, so a first launch cannot pre-assign
	// one; the adapter reports the minted id in RunResult.SessionID instead.
	if spec.Session != nil && spec.Session.Resume && strings.TrimSpace(spec.Session.ID) != "" {
		return []string{
			"--no-daemon",
			"exec",
			"resume",
			"--json",
			"-m", spec.Model,
			"-c", fmt.Sprintf("model_reasoning_effort=%q", spec.Effort),
			"--dangerously-bypass-approvals-and-sandbox",
			"--skip-git-repo-check",
			spec.Session.ID,
			"-",
		}
	}
	// A shared background server would own shell commands outside the runtime's
	// process and confinement lifetime. This invocation owns its executor.
	return []string{
		"--no-daemon",
		"exec",
		"--json",
		"-m", spec.Model,
		"-c", fmt.Sprintf("model_reasoning_effort=%q", spec.Effort),
		"--dangerously-bypass-approvals-and-sandbox",
		"--skip-git-repo-check",
		"-",
	}
}

// Await blocks until the launched process finishes and reports its token/turn
// usage, parsed from the CLI's `--json` event stream. A non-nil error is an
// operational failure (the process could not be run or awaited, the context was
// cancelled, or the stream carried no JSON at all); a nil error means the process
// finished — whether it produced a usable artifact is decided separately, by
// verification (DINV-3, AP-13). Context cancellation kills the process and
// returns promptly.
func (a *Adapter) Await(ctx context.Context, h port.Handle) (result port.RunResult, awaitErr error) {
	defer func() {
		if !errors.Is(awaitErr, port.ErrNotQuiescent) {
			a.fence.MarkTerminal(h)
		}
	}()
	hh, ok := h.(*handle)
	if !ok {
		return port.RunResult{}, fmt.Errorf("codex: await: foreign handle %T", h)
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
		res, _ := parseResult(w.stdout) // cancellation owns the error; retain available telemetry
		return res, fmt.Errorf("codex: await cancelled: %w", err)
	}

	select {
	case <-ctx.Done():
		// Cancellation: terminate the child and drain the waiter (no leak), then
		// report the cancellation as the operational failure.
		if err := hh.proc.kill(); err != nil {
			return port.RunResult{}, errors.Join(ctx.Err(), fmt.Errorf("%w: %w", port.ErrNotQuiescent, err))
		}
		w := <-done
		res, _ := parseResult(w.stdout) // cancellation owns the error; retain available telemetry
		return res, fmt.Errorf("codex: await cancelled: %w", ctx.Err())
	case w := <-done:
		res, parseErr := parseResult(w.stdout)
		// A clean direct-child exit is not proof that the agent session is gone:
		// CLIs may leave MCP or shell descendants in their process group. Quiesce
		// the whole group before the Worker can publish evidence or a reviewer can
		// receive fresh attribution material (DINV-2).
		if err := hh.proc.kill(); err != nil {
			return res, fmt.Errorf("codex: %w: %w", port.ErrNotQuiescent, err)
		}
		// Current Codex reports no child identity in this event stream. Its
		// session records are complete only now that the process tree is gone.
		if res.ReviewChildren == nil && hh.sessions != "" {
			res.ReviewChildren = sessionReviewChildren(ctx, hh.sessions, res.SessionID)
		}
		if w.err != nil {
			return res, fmt.Errorf("codex: await process: %w", w.err)
		}
		if parseErr != nil {
			return res, fmt.Errorf("codex: parse event stream (exit %d): %w", w.code, parseErr)
		}
		if w.code != 0 && !res.Incomplete {
			return res, fmt.Errorf("codex: process exited %d", w.code)
		}
		return res, nil
	}
}

// Kill terminates a launched process (cancellation, or a breached bound — for
// Codex the uniform turn/wall-clock cap is enforced here, there being no native
// flag). It is safe to call after the process has already exited.
func (a *Adapter) Kill(_ context.Context, h port.Handle) error {
	hh, ok := h.(*handle)
	if !ok {
		return fmt.Errorf("codex: kill: foreign handle %T", h)
	}
	if err := hh.proc.kill(); err != nil {
		return fmt.Errorf("codex: kill process: %w", err)
	}
	a.fence.MarkTerminal(h)
	return nil
}

// AuthCheck preflights install and authentication with two zero-cost probes —
// neither makes a model call — so a missing binary or credential degrades loudly
// here and consumes no Worker attempt (prd:implement-now/FR-16, A-11). The error
// states the remedy.
func (a *Adapter) AuthCheck(ctx context.Context) error {
	// Install probe: `codex --version` proves the CLI is present and runnable.
	if _, code, err := a.cmd.run(ctx, commandSpec{Name: binary, Args: []string{"--version"}}); err != nil {
		return fmt.Errorf("codex backend unavailable: `%s` CLI not found or not runnable (%w); install it (https://developers.openai.com/codex/cli) and re-run", binary, err)
	} else if code != 0 {
		return fmt.Errorf("codex backend unavailable: `%s --version` exited %d; reinstall the CLI and re-run", binary, code)
	}

	// Auth probe: a credential must be resolvable without a model call.
	probe := a.authProbe
	if probe == nil {
		probe = defaultAuthProbe
	}
	if err := probe(ctx); err != nil {
		return fmt.Errorf("codex backend unauthenticated: %w; run `%s login` once, or set OPENAI_API_KEY, then re-run", err, binary)
	}
	return nil
}

// handle is one launched `codex` process. The Worker treats it as opaque.
// sessions is set for a review launch only: the directory under the launch's
// isolated Codex home where the CLI records each thread of the run.
type handle struct {
	observe  func(port.RunResult, error)
	proc     process
	sessions string
}

var _ port.Handle = (*handle)(nil)

func (*handle) RuntimeHandle() {}

// codexEvent is the subset of one `codex exec --json` thread event this adapter
// reads. It is parsed OPEN (unknown fields ignored) on purpose: this is
// third-party CLI output that grows fields over time, not one of our own
// schema-versioned config files, so the closed-parse rule (AP-6) does not apply
// here.
type codexEvent struct {
	Type  string      `json:"type"`
	Usage *codexUsage `json:"usage"`
	Item  *struct {
		Type      string                     `json:"type"`
		Text      string                     `json:"text"`
		Tool      string                     `json:"tool"`
		Status    string                     `json:"status"`
		Sender    string                     `json:"sender_thread_id"`
		Receivers []string                   `json:"receiver_thread_ids"`
		States    map[string]codexChildState `json:"agents_states"`
	} `json:"item"`
	// ThreadID rides the `thread.started` event: the conversation identity a
	// later launch of the same task resumes (task session continuity).
	ThreadID string `json:"thread_id"`
}

type codexChildState struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// codexUsage is the token accounting Codex attaches to a `turn.completed` event.
// total_tokens already equals input_tokens + output_tokens (cached_input_tokens
// and reasoning_output_tokens are subsets of those, so they are deliberately not
// re-added — that would double-count).
type codexUsage struct {
	InputTokens  *int `json:"input_tokens"`
	OutputTokens *int `json:"output_tokens"`
	TotalTokens  *int `json:"total_tokens"`
}

// total is the token count for one turn: the reported total when present, else
// input+output (tolerating a stream that omits total_tokens).
func (u codexUsage) total() (int, bool) {
	if u.TotalTokens != nil {
		return *u.TotalTokens, true
	}
	total := 0
	for _, v := range []*int{u.InputTokens, u.OutputTokens} {
		if v != nil {
			total += *v
		}
	}
	return total, u.InputTokens != nil || u.OutputTokens != nil
}

// parseResult reads token/turn telemetry from the CLI's `--json` (JSONL) event
// stream. Tokens is the usage summed across every `turn.completed` event; Turns
// is the count of those completed turns. Individual non-JSON lines (a stray
// diagnostic on stdout) are skipped rather than fatal — the stream is
// line-delimited JSON and one bad line must not discard the run's telemetry — but
// a stream with no JSON object at all is an operational failure (a broken
// invocation, mirroring an empty result).
func parseResult(stdout []byte) (port.RunResult, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return port.RunResult{}, errors.New("empty CLI output")
	}
	var (
		tokens         int
		turns          int
		tokensObserved bool
		usageMissing   bool
		parsed         bool
		incomplete     bool
		reason         string
		output         string
		sessionID      string
	)
	var children codexReviewChildren
	for line := range bytes.SplitSeq(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		parsed = true
		if ev.Type == "thread.started" && strings.TrimSpace(ev.ThreadID) != "" {
			sessionID = strings.TrimSpace(ev.ThreadID)
		}
		children.observe(ev, sessionID)
		if ev.Type == eventTurnCompleted {
			turns++
			if ev.Usage != nil {
				value, observed := ev.Usage.total()
				tokens += value
				tokensObserved = tokensObserved || observed
				usageMissing = usageMissing || !observed
			} else {
				usageMissing = true
			}
		}
		if ev.Type == "turn.failed" || ev.Type == "error" {
			incomplete, reason = true, ev.Type
		}
		if ev.Type == "item.completed" && ev.Item != nil && ev.Item.Type == "agent_message" {
			output = ev.Item.Text
		}
	}
	if !parsed {
		return port.RunResult{}, errors.New("no JSON events in CLI output")
	}
	res := port.RunResult{ReviewChildren: children.result(), Tokens: tokens, Turns: turns, TokensObserved: tokensObserved && !usageMissing, TurnsObserved: parsed, Incomplete: incomplete, Reason: reason, Output: output, SessionID: sessionID}
	if turns == 0 && !incomplete {
		res.TurnsObserved = false
		return res, errors.New("no completed turn in CLI output")
	}
	return res, nil
}

// defaultAuthProbe resolves a Codex credential without a model call: an env
// token, or the Codex on-disk auth store (`codex login`). It covers both API-key
// and ChatGPT-login setups, so a genuinely unauthenticated backend degrades
// loudly while an authenticated one preflights clean (FR-16).
func defaultAuthProbe(_ context.Context) error {
	for _, key := range []string{"OPENAI_API_KEY", "CODEX_API_KEY"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return nil
		}
	}
	if authStoreExists() {
		return nil
	}
	return errors.New("no credential found (checked OPENAI_API_KEY, CODEX_API_KEY, and the Codex auth store)")
}

// authStoreExists reports whether the Codex on-disk auth file is present (the
// `codex login` case).
func authStoreExists() bool {
	dir := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dir = filepath.Join(home, ".codex")
	}
	_, err := os.Stat(filepath.Join(dir, "auth.json"))
	return err == nil
}
