// Package config parses the executor's block on the pyxis-config surface
// (`.pyxis/config.yaml`) — the closed-schema policy that governs backend
// routing and the caps (docs/tech-design/implement-now.md §3.8;
// design-brainstorm:implement-now-tech-design#D-4/#D-6). Four concerns live
// here, all repo-configured data with sensible defaults + per-repo override,
// never model-judged (AP-8 closed-schema config; AP-16/AP-19/AP-21 mechanical
// enforcement downstream):
//
//   - Routing (#D-4): a repo-configured default backend plus an optional
//     per-repo table mapping a task's routing category to a backend. The
//     resolver is ResolveBackend — a table match, else an explicit repo default,
//     else the invocation host's declared ambient backend. When all are absent,
//     implement-now discovers the first compatible authenticated provider. Model
//     and effort are then selected from that provider's built-in catalog using
//     the task's persisted semantic demand; this package never handles native
//     model IDs.
//   - Native routing (native-routing-policy-v1 §3.5): at most one complete,
//     operator-authorized harness-role catalog replacement reference. Selection
//     validates its artifacts as one unit; this parser never field-merges them.
//   - Caps (#D-6): the worker turn bound, the CI self-heal attempt cap, and the
//     parallel-dispatch width. Exposed as typed accessors; enforced mechanically
//     by the Worker/Dispatcher (never here).
//   - Gate (#D-6, FR-8): the repo's CI-mirroring local-gate commands — the same
//     deterministic checks a required CI check runs, executed over a workdir so
//     the drive-to-green loop and the truncated-but-green publish decision verify
//     locally before a push (DINV-4, UB-5). An absent gate is permitted: the gate
//     then vacuously passes and CI remains the authoritative oracle (DINV-3).
//     Exposed as GateChecks; consumed by the drive-to-green wiring (never here).
//
// The block parses CLOSED: an unknown key anywhere under `executor:` is a parse
// error (AP-6, config parses closed — unknown keys are errors). Because the
// surface is shared with other consumers (e.g. docgate's `paths:`/`plan:`
// blocks), this parser owns only the `executor:` subtree — sibling top-level
// blocks belong to other consumers and are skipped, not rejected. An absent
// `executor:` block, or an absent file, yields Defaults so a repo that has not
// configured the executor still runs with sensible bounds.
//
// The parser is a hand-rolled closed reader over a two-space-indented YAML
// subset — stdlib only, matching the house closed-config style
// (internal/docgate/config). It imports only internal/executor/port (for
// port.Task) and the vendor-neutral native-routing policy types, with no
// adapter or runtime package, so config stays a leaf of the hexagon (AP-1).
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const defaultRunLogDir = ".pyxis/logs/implmentor"

// DefaultWorkerBudget is the uniform per-worker wall-clock bound. Both the
// implementation engine and a one-shot prototype use this base policy.
const DefaultWorkerBudget = time.Hour

// Seed defaults. Each is a data value an operator overrides per repo, not a
// code constant to re-decide (#D-6, §7 Q2 — tuned against fixtures):
//
//   - workerTurnsSeed is 0: NO turn cap by default. A Worker is bounded by its
//     wall-clock budget instead (default 1h), which is the bound an operator
//     actually sets. The old seed of 60 silently clamped the claude adapter's own
//     raise to 150 and cut a task off mid-work with budget still to spend
//     (agenticlab-ai/pyxis#997). Set a positive `executor.caps.worker_turns` to
//     reinstate a turn cap; absence now means unbounded turns, not 60.
//   - ciSelfHealSeed bounds the drive-to-green CI self-heal retries before the
//     task fails closed and escalates (AP-16).
//   - parallelWidthSeed matches the Dispatcher's DefaultWidth so config and the
//     Dispatcher agree on the sweep's concurrency bound.
const (
	workerTurnsSeed   = 0
	ciSelfHealSeed    = 3
	parallelWidthSeed = 4
	reviewWidthSeed   = 2
)

// Sentinel errors — part of the package contract, matched with errors.Is.
var (
	// ErrUnknownKey is a closed-schema violation: a key that is not in the
	// executor block's fixed grammar (AP-6).
	ErrUnknownKey = errors.New("executor config: unknown key")
	// ErrMalformed is a structural defect: bad indentation, a tab in leading
	// whitespace, or a line that is neither a mapping key nor a list the grammar
	// expects.
	ErrMalformed = errors.New("executor config: malformed")
	// ErrInvalidHarness is a harness value outside the closed set
	// (claude/codex/cursor/opencode).
	ErrInvalidHarness = errors.New("executor config: invalid harness")
	// ErrInvalidCap is a cap value that is not a positive integer.
	ErrInvalidCap = errors.New("executor config: invalid cap")
	// ErrInvalidGate is a malformed gate entry: an empty command, or a duplicate
	// check name.
	ErrInvalidGate = errors.New("executor config: invalid gate check")
	// ErrInvalidNativeRouting is a structurally incomplete, incompatible, or
	// injected complete-catalog replacement declaration.
	ErrInvalidNativeRouting = errors.New("executor config: invalid native routing override")
	// ErrInvalidRoutingEvidence is a malformed activation boundary. A present
	// block is the all-path production cutover, so partial declarations are
	// rejected rather than interpreted as an inactive or legacy mode.
	ErrInvalidRoutingEvidence = errors.New("executor config: invalid routing evidence activation")
	// ErrNoDefaultHarness is returned by ResolveHarness when the Config carries
	// no default harness — a zero-value Config never obtained from
	// Load/Parse/Defaults. Routing fails closed rather than returning "".
	ErrNoDefaultHarness = errors.New("executor config: no default harness")
)

// GateCheck is one local-gate command mirrored from a required CI check: Name is
// the check it mirrors (carried for diagnostics), Argv is the command run in the
// task's workdir. It is a plain data value — the drive-to-green wiring maps it to
// its own gate check type, so this package imports no drivetogreen (config stays
// a leaf, AP-1).
type GateCheck struct {
	Name string
	Argv []string
}

// RoutingEvidenceActivation is the exact, human-merged production cutover.
// Private signing material is deliberately absent: the composition root loads
// the repository-specific producer bundle from the operating-system credential
// store. CheckpointEvent identifies the canonical activation_checkpoint that
// authorizes the policy/ref pair.
type RoutingEvidenceActivation struct {
	EvidenceRef       string
	ProducerPolicyRef string
	CheckpointEvent   string
}

// Config is the parsed executor block. Fields are unexported: a Config is
// obtained from Load, Parse, or Defaults (never a struct literal), and its
// routing, caps, and gate are read through ResolveHarness and the typed
// accessors.
type Config struct {
	// defaultHarness is the repo default target; empty only in a zero-value
	// Config. Empty means the composition root must supply ambient identity.
	defaultHarness entity.Harness
	// routes maps a task's routing category (its charter harness slot) to a
	// harness — the optional per-repo table (#D-4). Nil/empty means every task
	// takes the default. The key is the charter slot, which may name a category
	// rather than a harness; the value is always a known harness.
	routes map[string]entity.Harness
	// The impl-review invocation seam (#556; impl-review-v1 §3.4): an absent
	// block keeps the seam uninstalled; a present block may name one explicit
	// harness or inherit each task's selected implementation harness.
	reviewHarness                entity.Harness
	reviewBudgetMin              int
	reviewEvidenceRef            string
	reviewProcessEvidenceRef     string
	reviewPublicationEvidenceRef string
	reviewConfigured             bool
	// The pre-publish review loop (worker/prepublish.go): how many
	// review→remediate rounds run before the task escalates, and whether the
	// loop is disabled (review then runs only post-green, as before).
	reviewRemediationRounds int
	reviewPrePublishOff     bool
	// feedbackTrusted are extra code-host logins whose proposal comments may
	// wake a feedback round even without a trusted author association. The
	// executor.feedback_trusted setting is independent of review installation;
	// the legacy review.feedback_trusted spelling takes precedence when present.
	// Comments from owner/member/collaborator associations always qualify.
	feedbackTrusted []string

	// The task session continuity switch (worker/session.go, tasksession):
	// on by default; `session: { continuity: false }` launches every process
	// as a fresh conversation, as before.
	sessionContinuityOff bool
	// The four caps (#D-6), each non-negative or positive as its accessor states.
	workerTurns        int
	ciSelfHealAttempts int
	parallelWidth      int
	reviewWidth        int
	// gateChecks are the CI-mirroring local-gate commands (#D-6, FR-8), in
	// declaration order. Nil/empty means no local gate — vacuously green, CI is
	// authoritative (DINV-3).
	gateChecks []GateCheck
	// nativeRouting is nil unless the repository declares one complete
	// harness-role catalog replacement. Its referenced artifacts are loaded and
	// selected by the native-routing composition boundary.
	nativeRouting *nativerouting.CatalogOverride
	// routingEvidence is nil until the repository carries the subsequent
	// human-merged activation block. The implementation can therefore land and
	// collect proof without creating mixed production semantics.
	routingEvidence *RoutingEvidenceActivation
	// runLogDir is the repository-relative or absolute directory for the simple,
	// human-readable per-feature implement-now log.
	runLogDir string
}

// Defaults is the executor policy a repo with no `executor:` block runs under:
// no fixed backend, an empty routing table, and the seed caps. The composition
// root may supply ambient identity or discover an authenticated provider.
func Defaults() Config {
	return Config{
		routes:                       map[string]entity.Harness{},
		workerTurns:                  workerTurnsSeed,
		ciSelfHealAttempts:           ciSelfHealSeed,
		parallelWidth:                parallelWidthSeed,
		reviewWidth:                  reviewWidthSeed,
		reviewEvidenceRef:            "refs/pyxis/implement-now-review-state",
		reviewProcessEvidenceRef:     "refs/pyxis/impl-review-process-state",
		reviewPublicationEvidenceRef: "refs/pyxis/impl-review-publication-state",
		runLogDir:                    defaultRunLogDir,
	}
}

// Load merges root/.pyxis/config.local.yaml over root/.pyxis/config.yaml and
// parses the effective executor block. Both files are optional; an unreadable
// or malformed layer is an error. The effective document is parsed closed.
func Load(root string) (Config, error) {
	b, err := configlayer.Load(root)
	if err != nil {
		return Defaults(), fmt.Errorf("executor config: load configuration: %w", err)
	}
	return Parse(b)
}

// Parse validates the closed executor grammar over cfg bytes and returns the
// resulting Config, starting from Defaults so an omitted key keeps its default
// (partial override). The first defect is returned, wrapped over a sentinel and
// annotated with its 1-based line number, so the offending line is unambiguous.
//
// Grammar (two-space indent steps; `#` comments and blank lines ignored):
//
//	executor:
//	  feedback_trusted: <login>, <login>  # independent of local review installation
//	  routing:
//	    default: <backend>
//	    table:
//	      <category>: <backend>
//	  native_routing:
//	    schema_version: native-routing-catalog-override/v1
//	    resolver_compatibility: native-routing-resolver/v1
//	    harness: <backend>
//	    role: implementation|review
//	    catalog_ref: <repository-relative JSON path>
//	    qualifications_ref: <repository-relative JSON path>
//	    authorization:
//	      kind: repository_operator
//	      repository: <owner/name>
//	      operator_id: <stable operator identity>
//	      evidence_digest: <lowercase SHA-256>
//	      authorized_at: <RFC3339 timestamp>
//	  routing_evidence:
//	    evidence_ref: refs/pyxis/<name>
//	    producer_policy_ref: <repository-relative canonical JSON path>
//	    activation_checkpoint: <lowercase 64-hex event ID>
//	  caps:
//	    worker_turns: <n>
//	    ci_self_heal_attempts: <n>
//	    parallel_width: <n>
//	    review_width: <n>
//	  review:
//	    backend: <backend>        # omit to inherit the implementation backend
//	    budget_minutes: <n>
//	    evidence_ref: refs/pyxis/<name>
//	    process_evidence_ref: refs/pyxis/<name>
//	    publication_evidence_ref: refs/pyxis/<name>
//	  logging:
//	    directory: .pyxis/logs/implmentor
//	  gate:
//	    <check-name>: <command>   # whitespace-split into argv; no shell
//
// Top-level keys other than `executor:` are other consumers' blocks on the
// shared surface and are skipped. Any unknown key *within* the executor block
// is ErrUnknownKey.
func Parse(cfg []byte) (Config, error) {
	c := Defaults()

	// Parse state. inExecutor gates the whole block; section selects
	// routing/caps; sub selects the routing table's nested entries.
	const (
		secNone = iota
		secRouting
		secCaps
		secReview
		secSession
		secGate
		secNativeRouting
		secRoutingEvidence
		secLogging
	)
	inExecutor, seenExecutor := false, false
	section := secNone
	inTable, inNativeAuthorization, seenNativeRouting, seenRoutingEvidence := false, false, false, false
	seenReviewFeedbackTrusted := false
	nativeFields, nativeAuthorizationFields := map[string]bool{}, map[string]bool{}
	routingEvidenceFields := map[string]bool{}

	for i, raw := range strings.Split(strings.ReplaceAll(string(cfg), "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineNo := i + 1

		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.ContainsRune(lead, '\t') {
			return c, fmt.Errorf("%w: line %d: tab in indentation (use spaces)", ErrMalformed, lineNo)
		}
		indent := len(lead)

		// Top-level keys choose the block. Only `executor:` is ours; every
		// sibling belongs to another consumer of the shared surface, and its
		// whole subtree is skipped without structural checks (so a foreign
		// list or deeper nesting never trips this closed parser).
		if indent == 0 {
			key, val, ok := splitKV(trimmed)
			if !ok {
				return c, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformed, lineNo, trimmed)
			}
			inExecutor, section, inTable, inNativeAuthorization = false, secNone, false, false
			if key != "executor" {
				continue
			}
			if seenExecutor {
				return c, fmt.Errorf("%w: line %d: duplicate executor block", ErrMalformed, lineNo)
			}
			if val != "" {
				return c, fmt.Errorf("%w: line %d: executor is a block, not a scalar", ErrMalformed, lineNo)
			}
			inExecutor, seenExecutor = true, true
			continue
		}

		// Indented lines matter only inside the executor block; anything under a
		// foreign top-level block is skipped before any structural check.
		if !inExecutor {
			continue
		}
		if indent%2 != 0 || indent > 6 {
			return c, fmt.Errorf("%w: line %d: unexpected indentation %d (two-space steps, max 6)", ErrMalformed, lineNo, indent)
		}
		key, val, ok := splitKV(trimmed)
		if !ok {
			return c, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformed, lineNo, trimmed)
		}

		switch indent {
		case 2:
			inTable, inNativeAuthorization = false, false
			switch {
			case key == "feedback_trusted":
				section = secNone
				// Existing nested settings, including local overlays, override
				// the shared independent setting regardless of mapping order.
				if !seenReviewFeedbackTrusted {
					c.feedbackTrusted = feedbackTrustedLogins(val)
				}
			case key == "routing" && val == "":
				section = secRouting
			case key == "caps" && val == "":
				section = secCaps
			case key == "gate" && val == "":
				section = secGate
			case key == "review" && val == "":
				section = secReview
				c.reviewConfigured = true
			case key == "session" && val == "":
				section = secSession
			case key == "native_routing" && val == "":
				if seenNativeRouting {
					return c, fmt.Errorf("%w: line %d: duplicate native_routing block", ErrMalformed, lineNo)
				}
				seenNativeRouting, section = true, secNativeRouting
				c.nativeRouting = &nativerouting.CatalogOverride{}
			case key == "routing_evidence" && val == "":
				if seenRoutingEvidence {
					return c, fmt.Errorf("%w: line %d: duplicate routing_evidence block", ErrMalformed, lineNo)
				}
				seenRoutingEvidence, section = true, secRoutingEvidence
				c.routingEvidence = &RoutingEvidenceActivation{}
			case key == "logging" && val == "":
				section = secLogging
			default:
				return c, fmt.Errorf("%w: line %d: %q under executor (expected feedback_trusted, routing, caps, gate, review, session, logging, native_routing, or routing_evidence)", ErrUnknownKey, lineNo, key)
			}

		case 4:
			inTable, inNativeAuthorization = false, false
			switch section {
			case secRouting:
				switch {
				case key == "default":
					if err := setHarness(&c.defaultHarness, val, lineNo); err != nil {
						return c, err
					}
				case key == "table" && val == "":
					inTable = true
				default:
					return c, fmt.Errorf("%w: line %d: %q under routing", ErrUnknownKey, lineNo, key)
				}
			case secCaps:
				dst, known := map[string]*int{
					"worker_turns":          &c.workerTurns,
					"ci_self_heal_attempts": &c.ciSelfHealAttempts,
					"parallel_width":        &c.parallelWidth,
					"review_width":          &c.reviewWidth,
				}[key]
				if !known {
					return c, fmt.Errorf("%w: line %d: %q under caps", ErrUnknownKey, lineNo, key)
				}
				if err := setCap(dst, val, key, lineNo); err != nil {
					return c, err
				}
			case secReview:
				switch key {
				case "backend":
					if err := setHarness(&c.reviewHarness, val, lineNo); err != nil {
						return c, err
					}
				case "budget_minutes":
					if err := setCap(&c.reviewBudgetMin, val, key, lineNo); err != nil {
						return c, err
					}
				case "evidence_ref":
					if err := setEvidenceRef(&c.reviewEvidenceRef, val, lineNo); err != nil {
						return c, err
					}
				case "process_evidence_ref":
					if err := setEvidenceRef(&c.reviewProcessEvidenceRef, val, lineNo); err != nil {
						return c, err
					}
				case "publication_evidence_ref":
					if err := setEvidenceRef(&c.reviewPublicationEvidenceRef, val, lineNo); err != nil {
						return c, err
					}
				case "feedback_trusted":
					seenReviewFeedbackTrusted = true
					c.feedbackTrusted = feedbackTrustedLogins(val)
				case "remediation_rounds":
					if err := setCap(&c.reviewRemediationRounds, val, key, lineNo); err != nil {
						return c, err
					}
				case "pre_publish":
					on, err := parseBool(val)
					if err != nil {
						return c, fmt.Errorf("%w: line %d: pre_publish = %q: %v", ErrMalformed, lineNo, val, err)
					}
					c.reviewPrePublishOff = !on
				default:
					return c, fmt.Errorf("%w: line %d: %q under review (expected backend, budget_minutes, evidence_ref, process_evidence_ref, publication_evidence_ref, remediation_rounds, pre_publish, or feedback_trusted)", ErrUnknownKey, lineNo, key)
				}
			case secSession:
				switch key {
				case "continuity":
					on, err := parseBool(val)
					if err != nil {
						return c, fmt.Errorf("%w: line %d: continuity = %q: %v", ErrMalformed, lineNo, val, err)
					}
					c.sessionContinuityOff = !on
				default:
					return c, fmt.Errorf("%w: line %d: %q under session (expected continuity)", ErrUnknownKey, lineNo, key)
				}
			case secLogging:
				if key != "directory" {
					return c, fmt.Errorf("%w: line %d: %q under logging (expected directory)", ErrUnknownKey, lineNo, key)
				}
				if strings.TrimSpace(val) == "" {
					return c, fmt.Errorf("%w: line %d: logging.directory must not be empty", ErrMalformed, lineNo)
				}
				c.runLogDir = val
			case secGate:
				if key == "" {
					return c, fmt.Errorf("%w: line %d: empty gate check name", ErrMalformed, lineNo)
				}
				if val == "" {
					return c, fmt.Errorf("%w: line %d: gate check %q has an empty command", ErrInvalidGate, lineNo, key)
				}
				for _, g := range c.gateChecks {
					if g.Name == key {
						return c, fmt.Errorf("%w: line %d: duplicate gate check %q", ErrInvalidGate, lineNo, key)
					}
				}
				c.gateChecks = append(c.gateChecks, GateCheck{Name: key, Argv: strings.Fields(val)})
			case secNativeRouting:
				if nativeFields[key] {
					return c, fmt.Errorf("%w: line %d: duplicate native_routing key %q", ErrMalformed, lineNo, key)
				}
				nativeFields[key] = true
				switch key {
				case "schema_version":
					c.nativeRouting.SchemaVersion = val
				case "resolver_compatibility":
					c.nativeRouting.ResolverCompatibility = val
				case "harness":
					c.nativeRouting.Harness = entity.Harness(val)
				case "role":
					c.nativeRouting.Role = nativerouting.Role(val)
				case "catalog_ref":
					c.nativeRouting.CatalogReference = val
				case "qualifications_ref":
					c.nativeRouting.QualificationReference = val
				case "authorization":
					if val != "" {
						return c, fmt.Errorf("%w: line %d: authorization is a block, not a scalar", ErrInvalidNativeRouting, lineNo)
					}
					inNativeAuthorization = true
				default:
					return c, fmt.Errorf("%w: line %d: %q under native_routing", ErrUnknownKey, lineNo, key)
				}
			case secRoutingEvidence:
				if routingEvidenceFields[key] {
					return c, fmt.Errorf("%w: line %d: duplicate routing_evidence key %q", ErrMalformed, lineNo, key)
				}
				routingEvidenceFields[key] = true
				switch key {
				case "evidence_ref":
					if err := setEvidenceRef(&c.routingEvidence.EvidenceRef, val, lineNo); err != nil {
						return c, fmt.Errorf("%w: %v", ErrInvalidRoutingEvidence, err)
					}
				case "producer_policy_ref":
					if !safeRepositoryArtifact(val) {
						return c, fmt.Errorf("%w: line %d: producer_policy_ref must be a safe repository-relative JSON path", ErrInvalidRoutingEvidence, lineNo)
					}
					c.routingEvidence.ProducerPolicyRef = val
				case "activation_checkpoint":
					if !lowerHexDigest(val) {
						return c, fmt.Errorf("%w: line %d: activation_checkpoint must be lowercase 64-hex", ErrInvalidRoutingEvidence, lineNo)
					}
					c.routingEvidence.CheckpointEvent = val
				default:
					return c, fmt.Errorf("%w: line %d: %q under routing_evidence", ErrUnknownKey, lineNo, key)
				}
			default:
				return c, fmt.Errorf("%w: line %d: %q outside routing, caps, or gate", ErrUnknownKey, lineNo, key)
			}

		case 6:
			if section == secNativeRouting && inNativeAuthorization {
				if nativeAuthorizationFields[key] {
					return c, fmt.Errorf("%w: line %d: duplicate authorization key %q", ErrMalformed, lineNo, key)
				}
				nativeAuthorizationFields[key] = true
				switch key {
				case "kind":
					c.nativeRouting.Authorization.Kind = nativerouting.AuthorizationKind(val)
				case "repository":
					c.nativeRouting.Authorization.Repository = val
				case "operator_id":
					c.nativeRouting.Authorization.OperatorID = val
				case "evidence_digest":
					c.nativeRouting.Authorization.EvidenceDigest = val
				case "authorized_at":
					at, err := time.Parse(time.RFC3339, val)
					if err != nil {
						return c, fmt.Errorf("%w: line %d: authorized_at must be RFC3339: %v", ErrInvalidNativeRouting, lineNo, err)
					}
					c.nativeRouting.Authorization.AuthorizedAt = at
				default:
					return c, fmt.Errorf("%w: line %d: %q under native_routing.authorization", ErrUnknownKey, lineNo, key)
				}
				continue
			}
			if section != secRouting || !inTable {
				return c, fmt.Errorf("%w: line %d: %q is only valid as a routing table entry", ErrMalformed, lineNo, key)
			}
			if key == "" {
				return c, fmt.Errorf("%w: line %d: empty routing table key", ErrMalformed, lineNo)
			}
			if _, dup := c.routes[key]; dup {
				return c, fmt.Errorf("%w: line %d: duplicate routing table key %q", ErrMalformed, lineNo, key)
			}
			harness, err := entity.ParseHarness(val)
			if err != nil {
				return c, fmt.Errorf("%w: line %d: routing %q → %q; want one of %s", ErrInvalidHarness, lineNo, key, val, harnessList())
			}
			c.routes[key] = harness
		}
	}
	if seenNativeRouting {
		if err := validateNativeRoutingOverride(*c.nativeRouting); err != nil {
			return c, err
		}
	}
	if seenRoutingEvidence && (c.routingEvidence.EvidenceRef == "" || c.routingEvidence.ProducerPolicyRef == "" || c.routingEvidence.CheckpointEvent == "") {
		return c, fmt.Errorf("%w: evidence_ref, producer_policy_ref, and activation_checkpoint are all required", ErrInvalidRoutingEvidence)
	}
	if seenRoutingEvidence && !c.reviewConfigured {
		return c, fmt.Errorf("%w: routing_evidence requires the configured review path so implementation and review activate together", ErrInvalidRoutingEvidence)
	}
	return c, nil
}

// ResolveHarness selects the agent harness for task (#D-4): the per-repo routing
// table is consulted with the task's charter harness slot as the key — the only
// per-task routing signal the MVP charter carries (O-2 defers a richer plan or
// issue-label hint). A table hit yields the mapped harness; every miss —
// including an empty or unset slot — yields the configured default. A task never
// routes itself; the repo config does. The returned name selects the runtime at
// dispatch (runtime.Resolve); model and effort are resolved separately from
// the task's routing contract, not here (AP-21).
//
// Missing or unknown ambient identity with no configured match returns
// ErrNoDefaultHarness. A composition root may use that sentinel to perform
// authenticated provider discovery; this leaf package never probes the machine.
func (c Config) ResolveHarness(task port.Task, ambient string) (entity.Harness, error) {
	if b, ok := c.routes[string(task.Harness)]; ok {
		return b, nil
	}
	if c.defaultHarness != "" {
		return c.defaultHarness, nil
	}
	h, err := entity.ParseHarness(strings.ToLower(ambient))
	if err != nil {
		return "", fmt.Errorf("executor config: resolve harness for task %q: ambient harness %q: %w (declare one of %s)", task.ID, ambient, ErrNoDefaultHarness, harnessList())
	}
	return h, nil
}

// DefaultHarness is the explicit repository default routing target. Empty
// leaves selection to the invocation composition's ambient identity or local
// provider discovery.
func (c Config) DefaultHarness() entity.Harness { return c.defaultHarness }

// WorkerTurns is the worker turn bound cap (#D-6) — the number of agent turns a
// single Worker may take before it is stopped and checkpoint-exits (AP-16). The
// Worker enforces it (via a native flag or a uniform wall-clock/turn budget +
// kill across backends); this is only the configured value.
//
// Zero means NO turn cap — the wall-clock budget is then the only bound. It is
// the default (see workerTurnsSeed); a positive value reinstates a turn cap.
func (c Config) WorkerTurns() int { return c.workerTurns }

// ReviewHarness is the explicitly selected impl-review harness. Empty means
// per-task implementation-harness inheritance only when ReviewConfigured is
// true; an absent review block keeps the seam uninstalled.
func (c Config) ReviewHarness() entity.Harness { return c.reviewHarness }

// ReviewConfigured distinguishes an absent block (uninstalled) from an
// installed block whose backend is intentionally inherited.
func (c Config) ReviewConfigured() bool { return c.reviewConfigured }

// ReviewBudget is the wall bound for one reviewer session. It defaults to 15
// minutes when a review backend is configured without an explicit
// budget_minutes.
func (c Config) ReviewBudget() time.Duration {
	if c.reviewBudgetMin > 0 {
		return time.Duration(c.reviewBudgetMin) * time.Minute
	}
	return 15 * time.Minute
}

// ReviewEvidenceRef is the configured canonical fetched Git ref for review
// obligations, attempts, handoffs, and terminals.
func (c Config) ReviewEvidenceRef() string { return c.reviewEvidenceRef }

// ReviewProcessEvidenceRef is the fetched Git ref for review process evidence.
func (c Config) ReviewProcessEvidenceRef() string { return c.reviewProcessEvidenceRef }

// ReviewPublicationEvidenceRef is the fetched Git ref for durable review publication.
func (c Config) ReviewPublicationEvidenceRef() string { return c.reviewPublicationEvidenceRef }

// DefaultIssueEvidenceRef is the reserved canonical ref for admitted issue tasks.
// Issue entry's fixed state namespace is independent of configured review stores.
const DefaultIssueEvidenceRef = "refs/pyxis/implement-now-issue-state"

// IssueEvidenceRef exposes the fixed issue-state namespace through the same
// evidence policy seam as review state; it is not an operator-configurable key.
func (Config) IssueEvidenceRef() string { return DefaultIssueEvidenceRef }

// ReviewRemediationRounds bounds the pre-publish review loop: how many
// review→remediate cycles a task may run before it fails closed and escalates.
// Defaults to 2 when a review block is present without an explicit value.
func (c Config) ReviewRemediationRounds() int {
	if c.reviewRemediationRounds > 0 {
		return c.reviewRemediationRounds
	}
	return 3
}

// FeedbackTrusted is the operator allowlist of code-host logins whose
// proposal comments may wake a feedback round even without a trusted author
// association. Empty by default: association alone gates.
func (c Config) FeedbackTrusted() []string { return append([]string(nil), c.feedbackTrusted...) }

func feedbackTrustedLogins(value string) []string {
	var logins []string
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			logins = append(logins, part)
		}
	}
	return logins
}

// SessionContinuity reports whether implementation-role launches continue the
// task's own harness conversation (on by default; `session: { continuity:
// false }` disables it).
func (c Config) SessionContinuity() bool { return !c.sessionContinuityOff }

// RunLogDirectory is the directory for the simple, human-readable per-feature
// execution log. Relative paths are resolved from the repository root.
func (c Config) RunLogDirectory() string {
	if strings.TrimSpace(c.runLogDir) == "" {
		return defaultRunLogDir
	}
	return c.runLogDir
}

// ReviewPrePublish reports whether the configured review runs as the
// pre-publish loop (review the exact local revision after the local gate,
// remediate on request-changes, publish only an accepted revision). It is on
// by default for a configured review block; `pre_publish: false` keeps review
// post-green only. It is always false when no review block is present.
func (c Config) ReviewPrePublish() bool {
	return c.reviewConfigured && !c.reviewPrePublishOff
}

// RoutingEvidenceActivation returns the immutable production cutover binding.
// The bool is false only when the block is absent; a present partial block can
// never survive Parse.
func (c Config) RoutingEvidenceActivation() (RoutingEvidenceActivation, bool) {
	if c.routingEvidence == nil {
		return RoutingEvidenceActivation{}, false
	}
	return *c.routingEvidence, true
}

// CISelfHealAttempts is the CI self-heal attempt cap (#D-6) — how many times
// drive-to-green may attempt to fix a red remote CI run before the task fails
// closed and escalates (AP-16). Enforced by the Worker.
func (c Config) CISelfHealAttempts() int { return c.ciSelfHealAttempts }

// ParallelWidth is the parallel-dispatch width cap (#D-6) — how many Workers a
// sweep runs concurrently (AP-19). Enforced by the Dispatcher, which clamps it
// to at least 1.
func (c Config) ParallelWidth() int { return c.parallelWidth }

// ReviewWidth is the independent reviewer concurrency cap. It never borrows
// implementation width, so reviewer latency cannot stall task dispatch.
func (c Config) ReviewWidth() int { return c.reviewWidth }

// GateChecks are the repo's CI-mirroring local-gate commands (#D-6, FR-8), in
// declaration order — the checks the drive-to-green loop and the truncated-but-
// green publish decision run over a workdir before a push (DINV-4). An empty
// result means no local gate is configured: the gate vacuously passes and CI is
// the authoritative oracle (DINV-3). The returned slice is a deep copy, so a
// caller can never mutate the Config's checks.
func (c Config) GateChecks() []GateCheck {
	if len(c.gateChecks) == 0 {
		return nil
	}
	out := make([]GateCheck, len(c.gateChecks))
	for i, g := range c.gateChecks {
		argv := make([]string, len(g.Argv))
		copy(argv, g.Argv)
		out[i] = GateCheck{Name: g.Name, Argv: argv}
	}
	return out
}

// NativeRoutingOverride returns the repository's one complete harness-role
// replacement declaration. Nil means released adapter policy remains in
// effect. The returned value is a copy and carries references only; callers
// load the artifacts and pass the populated value to SelectEffectiveCatalog.
func (c Config) NativeRoutingOverride() *nativerouting.CatalogOverride {
	if c.nativeRouting == nil {
		return nil
	}
	out := *c.nativeRouting
	return &out
}

func validateNativeRoutingOverride(override nativerouting.CatalogOverride) error {
	if override.SchemaVersion != nativerouting.CatalogOverrideSchema {
		return fmt.Errorf("%w: schema_version %q; want %q", ErrInvalidNativeRouting, override.SchemaVersion, nativerouting.CatalogOverrideSchema)
	}
	if override.ResolverCompatibility != nativerouting.ResolverCompatibility {
		return fmt.Errorf("%w: resolver_compatibility %q; want %q", ErrInvalidNativeRouting, override.ResolverCompatibility, nativerouting.ResolverCompatibility)
	}
	if !entity.Harness(override.Harness).Known() {
		return fmt.Errorf("%w: harness %q; want one of %s", ErrInvalidNativeRouting, override.Harness, harnessList())
	}
	if override.Role != nativerouting.RoleImplementation && override.Role != nativerouting.RoleReview {
		return fmt.Errorf("%w: role %q; want implementation or review", ErrInvalidNativeRouting, override.Role)
	}
	if !nativerouting.ValidPolicyReference(override.CatalogReference) || !nativerouting.ValidPolicyReference(override.QualificationReference) {
		return fmt.Errorf("%w: catalog_ref and qualifications_ref must be non-argument repository-relative paths", ErrInvalidNativeRouting)
	}
	proof := override.Authorization
	if proof.Kind != nativerouting.AuthorizationRepositoryOperator {
		return fmt.Errorf("%w: authorization kind %q; only repository_operator is accepted", ErrInvalidNativeRouting, proof.Kind)
	}
	if proof.Repository == "" || proof.OperatorID == "" || !configDigestLike(proof.EvidenceDigest) || proof.AuthorizedAt.IsZero() {
		return fmt.Errorf("%w: complete repository, operator_id, evidence_digest, and authorized_at proof is required", ErrInvalidNativeRouting)
	}
	return nil
}

func configDigestLike(value string) bool {
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// splitKV splits "key: value" at the first colon. val is scalar-cleaned
// (inline comment and surrounding quotes stripped). ok is false when the line
// carries no colon at all.
func splitKV(s string) (key, val string, ok bool) {
	key, val, ok = strings.Cut(s, ":")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), scalar(val), true
}

// scalar cleans a mapping value: trim surrounding space, drop a trailing
// " #..." inline comment, and strip matching surrounding quotes.
func scalar(v string) string {
	v = strings.TrimSpace(v)
	if j := strings.Index(v, " #"); j >= 0 {
		v = strings.TrimSpace(v[:j])
	}
	return strings.Trim(v, `"'`)
}

// setHarness assigns a validated harness value, or returns ErrInvalidHarness.
func setHarness(dst *entity.Harness, val string, lineNo int) error {
	h, err := entity.ParseHarness(val)
	if err != nil {
		return fmt.Errorf("%w: line %d: %q; want one of %s", ErrInvalidHarness, lineNo, val, harnessList())
	}
	*dst = h
	return nil
}

func setEvidenceRef(dst *string, val string, lineNo int) error {
	if !safeEvidenceRef(val) {
		return fmt.Errorf("%w: line %d: evidence_ref must be a safe refs/pyxis/* ref", ErrMalformed, lineNo)
	}
	*dst = val
	return nil
}

func safeEvidenceRef(ref string) bool {
	if !strings.HasPrefix(ref, "refs/pyxis/") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") || strings.ContainsAny(ref, " \t\r\n~^:?*[\\") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for component := range strings.SplitSeq(ref, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func safeRepositoryArtifact(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Ext(path) != ".json" || strings.Contains(path, "\\") {
		return false
	}
	for component := range strings.SplitSeq(path, "/") {
		if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") && component != ".pyxis" {
			return false
		}
	}
	return true
}

func lowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// setCap assigns a positive-integer cap value, or returns ErrInvalidCap. A
// leading '+', whitespace, or any non-digit run is rejected so "01", "3 " and
// "0x4" never slip through.
func setCap(dst *int, val, name string, lineNo int) error {
	n, err := parsePositiveInt(val)
	if err != nil {
		return fmt.Errorf("%w: line %d: %s = %q: %w", ErrInvalidCap, lineNo, name, val, err)
	}
	*dst = n
	return nil
}

var errNotPositiveInt = errors.New("not a positive integer")

// parseBool accepts exactly "true" or "false" (closed grammar, AP-6).
func parseBool(v string) (bool, error) {
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, errors.New("want true or false")
}

// parsePositiveInt accepts a bare run of decimal digits denoting a value >= 1.
func parsePositiveInt(v string) (int, error) {
	if v == "" {
		return 0, errNotPositiveInt
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0, errNotPositiveInt
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 {
		return 0, errNotPositiveInt
	}
	return n, nil
}

// harnessList renders the closed harness set for error messages.
func harnessList() string {
	names := entity.Harnesses()
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = name.String()
	}
	return strings.Join(out, ", ")
}
