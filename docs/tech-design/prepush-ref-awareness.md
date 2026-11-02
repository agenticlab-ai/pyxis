# Pre-push ref awareness — deletion pushes skip the content gate; everything else fails closed to it

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-09 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-09 |
| **Relevant-when** | changing the push-hook wiring or the `pre-push` verb; triaging a hook verdict on a deletion or mixed push; asking why a branch delete no longer runs the gate |
| **Scope** | The push hook's ref handling: the `pre-push` verb (stdin protocol, deletion classification, delegation to the gate) and the shim's invocation line. **Not**: the gate's checks themselves (unchanged), the pre-commit hook (FR-9, unchanged), resolver mechanics (repo-layout §3.7). |
| **Tracking** | issue #65 |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/repo-layout-and-system-footprint.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-29 |
| **Security/privacy impact** | none — same hermetic binary reading the hook's own stdin; no new surface; the skip narrows to pushes that carry no content by protocol |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The push hook runs `gate --merge-base origin/main` unconditionally. Two
facts make that wrong for ref deletions (issue #65). First, a deletion
push carries no content — there is nothing to gate — yet the hook still
runs content checks. Second, the gate examines the *checkout's* working
tree, so deleting a branch from a checkout that predates recent merges
reads as mass deletion of ledger entries (`check-immutable` red, seven
defects per attempt), forcing `--no-verify` — a bypass habit no gate
should teach. Delete-on-merge is now prescribed practice (incident I2),
so deletion pushes are routine, not rare.

## 2. Decision

**Decision:** The shim delegates to a new **`pre-push` verb** instead of
`gate` directly. The verb reads the hook protocol's ref lines from its
inherited stdin (`<local-ref> <local-sha> <remote-ref> <remote-sha>`);
when **every** parseable line is a deletion — the local SHA is all zeros
— it passes with a one-line notice and runs nothing. In every other case
(any non-deletion ref, empty stdin, unparseable lines) it runs the
merge-base gate exactly as today. Classification lives in Go; the shim
stays a delegate-only one-liner (the language policy's letter and
spirit), changing only its verb word.

The skip is deliberately narrow (AP-17): only a push that is *provably
all deletions by protocol* skips; anything ambiguous fails closed into
the full gate. No branch-name heuristics, no environment sniffing —
the hook's own stdin is the single source (AP-18: observed refs, not
declared intent).

## 3. The design

### 3.1 Mechanics

- **Verb** — `docgate pre-push --merge-base <ref>`: reads stdin to EOF,
  splits lines, takes fields; a line with ≥2 fields whose second field is
  40+ zeros is a deletion. All-lines-deletions ∧ ≥1 line → print
  `pre-push: deletion-only push — no content to gate` on stderr, exit 0.
  Otherwise → `Gate(root, mergeBase, dirs, cfg)`, verdicts unchanged.
- **Shim** — `.pyxis/hooks/pre-push` exec line changes its verb from
  `gate` to `pre-push`; still one exec, still resolver-routed, still
  bash-3.2-clean. The hook-shim test's delegation assertion learns the
  per-hook verb.
- **A recorded delta from docgate-v1 §3.10** — that Decision's wiring
  line reads `gate --merge-base @{push}`; as built, the shim already ran
  `gate --merge-base origin/main` (recorded in the techspec since the
  wiring shipped). This design changes the verb word and nothing about
  the Decision's substance — committed delegate-only shim, bash-3.2
  floor, one documented activation command, desk/CI running the same
  binary. No binding statement of docgate-v1 (its DINVs, acceptance
  rows, or the Q2 substance) becomes false; the invocation string was
  mechanics, and its as-built form already differed.

### 3.2 Boundaries and non-goals

- **Not solved here** — the checkout-vs-pushed-refs mismatch for
  *content* pushes: the gate still examines the working tree, not the
  pushed SHAs, so pushing branch X from a checkout on Y still gates Y
  (v1's semantics, kept; Q1). CI remains the authoritative gate either
  way (docgate-v1 DINV-4).
- **Implementation must not infer** — that branch names, remote names,
  or push flags signal deletion (only the zero SHA does); or that a
  mixed push may skip the checks for its deletion refs (the whole push
  gates).
- **Must remain unchanged** — every verdict on a content push; the
  pre-commit hook; the resolver path; CI wiring (CI never sees deletion
  pushes — hooks are the only consumer of this verb).

### 3.3 Invariants

- **DINV-1** — deletion is classified only by the hook protocol's
  zero-SHA field, never by names or flags.
- **DINV-2** — the skip fires only when *every* parseable ref line is a
  deletion and at least one line exists; empty, malformed, or mixed
  input runs the full gate (fail closed).
- **DINV-3** — the shim carries no parsing: stdin flows to the verb via
  the exec'd process; the shim remains one delegate line.

### 3.4 Observability

The skip prints its one-line notice on stderr (visible in the push
output); everything else inherits docgate-v1 §3.4 (defects on stderr,
`--defects-json`, red required context as the alert). A skipped deletion
push leaves no other trace — matching its content: none.

### 3.5 Security & privacy

none — matches the header row: the verb reads the stdin git already
hands the hook; no network, no new data, and the only behavior change
removes a bypass incentive (`--no-verify` stops being the workaround for
routine branch deletion).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a guard clause inside the bash shim (parse stdin in sh) | logic in a shim — the language policy's named failure mode; the predecessor died of exactly this accretion |
| gate the pushed SHAs (git archive each non-deletion ref) instead of the working tree | the real fix for Q1, but a much larger change to v1's wiring semantics — a separate decision when content-push misfires actually bite |
| have the shim skip when `$1`/`$2` (remote args) look like a delete | push args don't carry per-ref deletion info; only stdin does (DINV-1) |
| teach `check-immutable` to ignore deletions | wrong layer — every content check would need the same patch; the push's nature is known before any check runs |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | a future hook manager or git version changes the stdin protocol shape | skip stops firing; pushes fail closed into the gate (annoying, never unsafe) | DINV-2's fail-closed default is the mitigation; the protocol is git's documented pre-push contract | malformed-stdin fixture runs the gate |
| R2 | the verb is invoked interactively without stdin ref lines | hangs reading a TTY | stdin read is EOF-bounded and the hook always provides EOF; interactive misuse reads an empty pipe on CI shells — documented in usage; empty input runs the gate | empty-stdin fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | deletion-only stdin → exit 0, skip notice, zero checks run; mixed and content pushes → full gate verdicts unchanged; empty/malformed stdin → full gate | Test | Go fixtures over the classifier + verb | prd:docgate/FR-29 · DINV-1/2 |
| A-2 | the shim remains delegate-only, one exec line, bash-3.2-clean, resolver-routed — asserted by the hook-shim test with the per-hook verb | Test | `test-docgate-hook.sh` run | §3.1 · DINV-3 |
| A-3 | a real `git push --delete` of an absorbed branch from a stale checkout passes without `--no-verify` | Demo | transcript in the introducing PR | §1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should content pushes gate the *pushed* SHAs (worktree-free) instead of the checkout's tree? | future design | nothing — CI is authoritative today | open | revisit when a content-push misfire bites in practice |

## Principles

- **AP-10 — honored.** The deletion rule is code reading a protocol, not a shim heuristic or a prompt habit.
- **AP-17 / INV-6 — honored.** Only provably-empty pushes skip; every ambiguity fails closed into the full gate (DINV-2).
- **AP-18 — honored.** Classification observes the hook's stdin refs, never declared intent or names (DINV-1).
- **AP-11 / INV-30 — honored.** One gate predicate remains; the verb adds routing before it, not a second predicate.
- **AP-13 — honored.** The A-3 demo is a real deletion push, not a claim.
- **Language policy — honored.** The shim stays one delegate line; the logic lives in Go (DINV-3).
- **Deviation:** none.
