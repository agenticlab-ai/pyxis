# docgate fence completion — vision append-only, the corpus freeze, and the gate-policy key guard

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — governance surface) |
| **Date** | 2026-07-08 |
| **Relevant-when** | changing what the merge-base fences cover (governing docs, corpus, gate-policy keys); touching `check-governing` or the corpus half of `check-immutable`; asking what the `Governing-Change` trailer actually proves |
| **Scope** | The fence-coverage deltas of review wave #54: the vision's append-only line check, the abstract-layer purity scan, the placeholder-corpus byte-freeze, and the trailer guard on the gate's own policy keys. **Not**: fence mechanics already decided (docgate-v1 §3.10 wiring, docgate-v2 trailer semantics), lint-content rules (serves-hardening), link integrity (doc-link-integrity). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/docgate-v2.md · docs/tech-design/repo-layout-and-system-footprint.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-18 · prd:docgate/FR-19 · prd:docgate/FR-23 · prd:docgate/NR-3 |
| **Security/privacy impact** | none new — same hermetic binary; the change *narrows* trust exposure (vision gains the append-only check; the gate's exemption keys stop being self-serve); the trailer's trust model is stated plainly in §3.5 |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The 2026-07-08 review (#54) found the AP-7 fence half-built in three ways.
(A2) `check-governing` line-checks only the principles: a commit carrying
the `Governing-Change: operator` trailer can rewrite or delete vision
content, though AP-7 declares the vision append-only from the beginning —
and rule 1's abstract-layer purity (no links to concrete docs, no
requirement-ID citations) had no mechanical half at all. (C5) The flat
placeholder corpus is citable requirement text — `corpus:` anchors target
it, rule 4 promises its IDs are stable — yet its mutability class was
undeclared and unenforced. (D1) The gate reads its own grandfather and
exemption lists from the ungated config surface, so a PR can exempt itself
in the same diff the gate evaluates — the self-unlock pattern INV-19
forbids for risk policy. Amendment 006/008 (PR #56) put the requirements
in the ledger (FR-18, FR-19, FR-23, NR-3); this design decides the
mechanics.

## 2. Decision

**Decision:** The existing fence family widens; no new mechanism is
invented. `check-governing` gains the vision in its append-only loop
(both governing docs are now line-frozen even with the trailer) and gains
an always-on **purity scan** (no relative markdown links, no baseline
requirement-ID citations in either governing doc — bytes-only, no merge
base needed). `check-immutable` gains a third path class: merged flat
corpus files under the configured PRDs path (README exempt, ledger
directories untouched) are add-only, exactly like ledger entries and
designs — a byte-freeze imposing zero content rules. A new merge-base
check, `check-config-policy`, fails any diff to the config surface's
gate-policy keys (`grandfathered_designs`, `precedence_grandfathered`,
`capability_paths`) without the `Governing-Change: operator` trailer —
the same attestation mechanism, because those keys *are* governance.

Reusing the trailer and the fence shapes keeps one predicate per idea
(AP-11): append-only has one implementation with a path list; attestation
has one trailer with a surface list.

## 3. The design

### 3.1 Mechanics

- **Vision append-only** — the existing principles append-only loop
  (`-U0` removed-line walk) runs over both governing paths instead of
  one. Defect wording names the document; the remedy for the vision is
  "append entries; supersede content by new entries, never edits."
- **Purity scan** — over the raw bytes of both governing docs, outside
  fenced code blocks: (a) any relative markdown link (`](` target not
  starting with a URL scheme or `#`) is a defect; (b) any match of the
  baseline ID families `(SCAF|PLAN|IMPL|GATE|FLEET|IDEN|GOV|KNOW|OBS|BENCH|VNEXT)-[0-9]+`
  is a defect. Incident IDs (`I<n>`) and principle IDs (`AP-n`, `RW-n`)
  are deliberately not matched — the principles cite themselves and may
  mention incidents generically; requirement IDs are the banned class
  (rule 1's letter). Runs with or without a merge base.
- **Corpus freeze** — inside `CheckImmutable`'s existing diff walk: a
  third match arm for `M/D/R` of `<cfg.PRDs>/<name>.md` where `<name>.md`
  is directly under the PRDs path (not in a ledger directory) and is not
  `README.md`. Additions pass untouched; content is never read.
- **Gate-policy key guard** — `check-config-policy` diffs the config file
  against the merge base; if any changed line sits under the three
  gate-policy keys, the HEAD commits must carry the trailer (presence
  check identical to `check-governing`'s — one helper, two callers). Key
  detection is structural (the closed config parse of both versions),
  never line-regex guessing: parse base and head, compare the three keys'
  values.

### 3.2 Boundaries and non-goals

- **Not solved here** — who may *truthfully* assert the trailer (§3.5);
  content rules for the corpus (none exist, by NR-3 — deliberately);
  general link checking (doc-link-integrity's decision).
- **Owned by a future design** — identity-backed verification of the
  trailer (engine-era, when the Identity Broker exists); configurability
  of the governing-doc paths themselves (they stay compiled defaults
  until an operand needs otherwise).
- **Implementation must not infer** — that `templates.*` or `paths.*`
  keys need the trailer: template pins ship with goldens in the same PR
  by the amendment model's own rule, and paths are operand tuning — only
  the three **gate-policy** keys are governance (FR-23's closed list).
  And not that the corpus freeze licenses any corpus *lint* — NR-3
  forbids content gating there.
- **A boundary of docgate-v1 evolves here, by the sanctioned path** —
  v1 §3.2 pinned "must remain unchanged: the placeholder corpus … (PRD
  NR-1)". That constraint's anchor was NR-1 as then written; the ledger
  superseded it (006: NR-1 → NR-3) with the operator's merge as the
  ruling. This design honors the surviving constraint (no content
  gating) and implements the new one (byte-freeze). Both documents stay
  simultaneously true read against the current contract — no binding
  statement of v1 (its Decision, DINVs, or acceptance rows) is falsified.

### 3.3 Invariants

- **DINV-1** — the purity scan is bytes-only: no git history, no
  network, no judgment; every defect names file and line.
- **DINV-2** — the corpus fence reads file *names*, never file contents:
  no content rule can grow inside it (NR-3's mechanical form).
- **DINV-3** — the key guard fails only on trailer *absence* over a
  structurally-detected gate-policy change, never on trailer content and
  never on other keys (docgate-v2#DINV-4, inherited: attestation, not
  authorization).
- **DINV-4** — one append-only implementation, one trailer-presence
  helper: `check-governing` and `check-config-policy` share them (AP-11).

### 3.4 Observability

Unchanged from docgate-v1 §3.4 (defects on stderr + `--defects-json`; a
red required context is the alert). New defect reasons join the committed
defect catalog: vision line-edit, purity link, purity requirement-ID,
corpus freeze, unattested gate-policy change — each with its remedy.

### 3.5 Security & privacy

none new — matches the header row; the surface only narrows. Stated
plainly (review finding D2): the `Governing-Change: operator` trailer is
**assertion, not authentication** — any author, including an automated
one, can type it. The machine checks presence; *truth* is anchored in the
human merge authority every PR requires (AGENTS.md: every merge is a
human with write access), which is exactly where review of an asserted
trailer happens. This design extends the assertion's coverage (to the
vision's lines and the gate-policy keys), not its trust level.
Identity-backed attestation is engine-era work (Identity Broker) and is
deliberately not simulated here.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a separate `check-corpus` verb | the corpus freeze is check-immutable's exact shape on a third path class; a new verb is a twin predicate (AP-11) |
| freezing the corpus by moving it into ledger directories | retrofit — the exact thing the operator ruling and NR-3 forbid; placeholders stay placeholders |
| guarding all config keys with the trailer | paths/templates/system are operand tuning and instrument pins with their own safeguards; blanket guarding makes routine changes ceremony and dilutes the trailer |
| line-regex detection of gate-policy key changes | brittle against YAML formatting; the closed parser already exists — compare parsed values (AP-10) |
| CODEOWNERS for the config file | platform-specific, invisible at desk — rejected for the same reasons as docgate-v2's governing check |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the purity scan's ID regex false-positives on future governing-doc prose | operator friction appending legitimate text | the banned class is requirement IDs only; incident/principle IDs excluded by construction; a false positive is appendable prose reworded, or the families list revised by superseding design | fixture: current vision+principles green byte-for-byte |
| R2 | vision append-only blocks legitimate operator restructuring | operator cannot fix vision typos in place | that is AP-7's letter, now enforced; the escape is an operator erratum append (issue #55's pattern) — same posture as principles today | trailered line-edit fixture red |
| R3 | corpus freeze blocks a legitimate correction to a placeholder | stale corpus text persists | extension by new files (the practiced pattern); a correction is a new file superseding by reference; README stays editable for routing | fixture: new-file green |
| R4 | key-guard bypass by restructuring config (e.g. moving lists to a new key) | self-unlock returns via schema change | unknown keys are hard errors (closed parse); adding a key is a code+schema change through normal review | config closed-parse fixtures |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | trailered line-edit of the vision red; trailered append green; untrailered vision diff red — both hook and CI paths | Test | temp-repo fixtures | prd:docgate/FR-18 |
| A-2 | a governing doc containing a relative link or a baseline requirement-ID citation red naming file+line; the real vision and principles green | Test | fixture + corpus run | prd:docgate/FR-18 · DINV-1 |
| A-3 | modify/delete/rename of a merged flat corpus file red; README edit green; new flat file green; ledger dirs unaffected; no content read from corpus files | Test | fixtures + an fs-access guard in the test | prd:docgate/FR-19 · DINV-2 |
| A-4 | untrailered `grandfathered_designs` addition red; trailered green; untrailered `paths`/`templates` edits green; detection survives YAML reformatting | Test | config fixtures | prd:docgate/FR-23 · DINV-3 |
| A-5 | the real repo gates green at introduction | Test | CI run | NR-3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should the governing-doc *paths* become config keys (guarded like gate-policy keys), or stay compiled defaults until an operand needs them? | operator | nothing (compiled defaults work for every current operand) | open | — |

## Principles

- **AP-7 / INV-41 — honored; this is its completion.** Every append-only class named by AP-7 now has a merge-base line check; the corpus joins the frozen classes by explicit ruling (NR-1 → NR-3 lineage), not silent gate creep.
- **AP-10 — honored.** Three prompt-side rules (vision append-only, rule 1 purity, "don't edit placeholders") become checks.
- **AP-11 / INV-30 — honored.** No new predicate shapes: one append-only walker, one trailer helper, one immutability diff walk — each with a wider path list (DINV-4).
- **AP-17 / INV-6 — honored.** Everything fails closed; the only escape is the explicit, visible trailer — whose trust model §3.5 states instead of implying.
- **AP-18 — deviation, stated.** The trailer is a *declared* attestation, not an observed identity — accepted temporarily because the human merge gate is the real authorization today; identity-backed verification is engine-era (§3.5). This inherits and makes explicit the posture docgate-v2 chose.
- **AP-19 — honored.** All new checks are deterministic byte/diff work; zero tokens, sub-second.
- **Deviation:** AP-18 as above; none otherwise.
