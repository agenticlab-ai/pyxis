# The /help front door — prompt-walk derivation over live routing surfaces — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(Draft while this PR is open; approval is the introducing PR's operator review; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-23 |
| **Relevant-when** | designing or changing the help skill's derivation mechanics, catalog rendering, problem routing, or first-steps entry; implementing the skill against this design; wiring the installer's welcome pointer; superseding the prompt-walk model |
| **Scope** | How `/help` derives and renders its three answers (catalog, route, first steps): the prompt-walk derivation contract, the invocation grammar, the honesty rules, and the purity proof. **Not**: the welcome work item's creation or transport (installer-v1's), corpus browsing (the reading room's), any enumerator verb or generated index (barred by the basis appetite). |
| **Tracking** | agenticlab-ai/pyxis#636 — the introducing PR |
| **Builds-on** | docs/tech-design/installer-v1.md (the welcome work item this design's entry string plugs into) · docs/tech-design/config-driven-portability-v1.md (target-facing paths resolve from repo config) |
| **Supersedes** | — |
| **Serves** | prd:ecosystem-help/FR-1 · prd:ecosystem-help/FR-2 · prd:ecosystem-help/FR-3 · prd:ecosystem-help/FR-4 · prd:ecosystem-help/FR-5 · prd:ecosystem-help/FR-6 · prd:ecosystem-help/UB-1 · prd:ecosystem-help/UB-2 · prd:ecosystem-help/UB-3 · prd:ecosystem-help/NR-1 · prd:ecosystem-help/NFR-1 |
| **Security/privacy impact** | none — a read-only conversational skill over repository files already readable by the host CLI; no new surface, credential, or egress |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Prompt-walk, no machinery** — the session reads the live surfaces raw at invocation; no enumerator verb, no generated catalog | #ALT-2 (a codified read-only enumerator verb) | The basis appetite must reopen first (`/prd-amend`), then a superseding design → §2, §3.2, `design-brainstorm:ecosystem-help-tech-design#D-1` |
| 2 | **Purity is proven by a conformance test, not runtime enforcement** — a text-level scan of help's own files plus an add-a-skill fixture | A runtime source-attribution check inside the skill | A-4's verification method changes; the test moves from the shipped harness into the skill's own behavior → §3.6, #DINV-1 |
| 3 | **One skill, `start` argument** — the installer's welcome template carries the literal `/help start` | A separate welcome/onboarding skill | The two-door problem reopens against the frozen stage-1 ruling; installer-v1's template content changes → §3.5, `#D-2` |

**Sanity-check the numbers** — the catalog spends ≤ 2 output lines per capability and reads routing surfaces only (#DINV-4, A-1); the routing bar is 100% correct entry naming with zero fabricated commands on every scenario set (A-2); the purity scan tolerates zero per-capability literals in help's own files (A-4).

**The author's open questions** — Q1: when the installer's welcome template gains the `/help start` string (their side of the seam; blocks nothing here) → §7.

**Skip the rest** — schema/structure → docgate's design lint; substance/coherence/reuse → the unmanned `tech-design-review` report riding this PR; implementation defects → the scripts-test harness + downstream gates; rationale → §4 (explanatory).

## 1. Problem

The approved contract (`prd:ecosystem-help`) promises a stateless front door that orients a newcomer, routes a described problem to its entry command, and offers a first-steps path — with every catalog fact derived from live surfaces, never hand-authored. What stage 3 had to decide is the *mechanism*: how the derivation actually happens within the token budget (NFR-1), how the three answers behave at their failure edges (UB-1..3), and where the installer's one-time welcome pointer lands (FR-6, the seam with installer-v1's FR-15 welcome work item). The cheapest moment is now: the skill does not exist yet, and the two candidate mechanisms diverge at the first line of implementation.

## 2. Decision

**Decision:** `/help` is a **prompt-walk skill**: at invocation, the session reads the live routing surfaces raw — the shipped pipeline map whole, plus every installed skill's frontmatter description — and composes its three answers (catalog, route, first steps) in-session, holding no per-capability content of its own and shipping no new binary, verb, or generated artifact. The first-steps path is the same skill's `start` argument; the installer's welcome work item carries the literal string `/help start`. (design-brainstorm:ecosystem-help-tech-design#D-1, #D-2)

This is safe because the surfaces already carry the catalog as data — skill frontmatter is written as when-to-use routing text, and the pipeline map owns stages, entry rules, and built-status — and because the honesty and purity obligations are pinned by binding elements below (#DINV-1..4) with a mechanical conformance test rather than trust. The alternative (a codified enumerator) is contractually barred by the basis appetite; if prompt-walk derivation proves fragile in practice, the sanctioned route is an amendment against the surface, never silent machinery growth (§3.2).

## 3. The design

### 3.1 The derivation walk

On invocation the skill's authoring guide drives exactly this read set:

1. **The shipped pipeline map** (one bounded read) — stages, the skill/mechanism per stage, human gates, built-status legend, and the "where a problem enters" rules.
2. **Every installed skill's frontmatter** (`name` + `description` only — never guide bodies) — the per-skill one-liner and when-to-use text.
3. **On demand only** (routing/first-steps answers that need them): wiki entry routing headers (In-one-line + Relevant-when rows), never entry bodies.

The three answers compose from that set:

- **Catalog** (bare `/help`, FR-1): every installed skill exactly once + every stage row of the map with its built-status, grouped by pipeline position (entry stages · cross-cutting · tooling), ≤ 2 output lines per capability, closing with the pointer to `start` and to the map itself.
- **Route** (`/help <described problem>`, FR-2): the entry command or stage for the problem with the map's why; judgment happens here and only here — the one prompt site (AP-21) — over the derived set, never over memory.
- **First steps** (`/help start` or a where-do-I-start ask, FR-3/FR-6): a two-sentence orientation plus the one concrete first action for the user's situation, routing into catalog/route — never teaching corpus content.

**The degraded walk:** a surface missing or unreadable on the install (a repo without the wiki, a skill without frontmatter) narrows the read set — the answers present what the remaining surfaces yield and name the absence explicitly; entries are never fabricated for an absent surface (#DINV-2).

Paths in the walk resolve per the config-driven-portability binding: the map, the skills tree, and the vendor pointers are located through the repo's configured surface set, never hardcoded absolute layout assumptions.

### 3.2 Boundaries and non-goals

- **Not solved here** — the welcome work item's creation, timing, template, and transport: installer-v1 §3.5/§3.6 owns them; this design only fixes the string its template carries (design-brainstorm:ecosystem-help-tech-design#O-2). Corpus browsing/search/trace: the reading room's charter.
- **Owned by a future design** — any enumerator verb, generated catalog, or index over the derivation surfaces: barred by the basis appetite (prd:ecosystem-help header); admissible only after a `/prd-amend` reopens the appetite, via a design superseding this one (#O-1). The situational advisor (the basis's WR-1) likewise.
- **Implementation must not infer** — that catalog completeness may be satisfied from memory or training data (every line traces to a read performed *this* invocation, #DINV-1); that an unbuilt stage row licenses recommending a command (#DINV-2); that the skill may cache, persist, or mark state anywhere (#DINV-3).
- **Must remain unchanged** — every derivation surface: the map's content, skill frontmatter, wiki headers, and the authoring guides are read, never edited by help (prd:ecosystem-help/NR-1).

### 3.3 Invariants

- **DINV-1** — **Derivation purity.** Help's own files (skill glue + authoring guide) contain no per-capability catalog content: no skill names list, no stage list, no per-feature descriptions. Every catalog line in an answer traces to a live-surface read performed in that invocation; adding or removing a skill on the surface changes help's output with zero help-side edits.
- **DINV-2** — **Routing honesty.** No answer ever names a command or stage absent from the surfaces read; a problem matching no entry gets an explicit refusal plus the map pointer; a stage the map marks unbuilt is presented with its status and the nearest built entry, never as an invocable command; a missing or unreadable surface is named in the answer, and no entry is ever fabricated for it.
- **DINV-3** — **Statelessness.** The skill writes nothing, caches nothing, and holds no cross-invocation state; the one-time welcome moment is the installer's, carried entirely by the template string.
- **DINV-4** — **Budget.** The walk reads routing surfaces only — the map whole, frontmatter blocks, wiki headers on demand — never a corpus document body, a guide body, or a governed document; the catalog answer spends ≤ 2 output lines per capability.

### 3.4 Observability

none — the skill is a conversational surface with no runtime of its own beyond the host CLI session: it emits no events, holds no state, and runs no code. Its "seen working / seen broken" surface is the acceptance evidence (§6): the eval scenario checklist and the purity conformance test, both riding the implementation PR, plus the recorded newcomer-proxy sessions the basis's M1 requires.

### 3.5 Security & privacy

none — matching the header row: the skill performs read-only access to repository files the host CLI already exposes, under the host CLI's own permission model; no new credential, no network egress, no state written, no new attack surface. The welcome-pointer seam adds only a literal string to a template installer-v1 already ships under its own security analysis.

### 3.6 Testing strategy

The seams under test, binned per the template's menu:

- **The derivation surfaces** (map, frontmatter, wiki headers) — *in-process*: real files in the repo (or a sandbox copy); no stand-ins needed.
- **The purity obligation (#DINV-1)** — *in-process, mechanical*: a conformance test on the shipped scripts-test harness with two arms: (a) a text-level scan asserting help's own files contain no per-capability literals (skill names, stage names harvested from the live surfaces at test time — the assertion list is itself derived, never hand-kept, AP-27); (b) an add-a-skill fixture: drop a throwaway skill into a sandbox copy of the surface and assert the catalog answer includes it with zero help-file edits (prd:ecosystem-help/FR-4's fit criterion, verbatim).
- **The routing/honesty obligations (#DINV-2)** — *eval*: the basis's EVAL-1 scenario checklist (canonical entry set + negative set + unbuilt-stage set), run manually until eval machinery exists.
- **The judgment itself** — deliberately not unit-tested: it is the one AP-21 judgment site; the eval checklist is its harness. No port, no mock — one adapter would be a hypothetical seam.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **Codified enumerator verb** — a read-only binary scanning map + frontmatter into a deterministic catalog block the skill consumes (design-brainstorm:ecosystem-help-tech-design#ALT-2) | Contractually barred: the basis appetite explicitly excludes "an enumeration API or generated index over the derivation surfaces"; also a new shipped binary surface and an I3-class misparse obligation. Its genuine wins (AP-10 enumeration, unit-testable completeness) are partially recovered by the derived-assertion conformance test. Revisitable only through a `/prd-amend` + superseding design. |
| **Pure prompt-walk with no mechanical backstop** — trust the guide alone for purity and completeness | The purity promise (FR-4) would rest on prompt discipline — the drift class AP-10 warns against; the conformance test closes it at near-zero cost on the existing harness. (The chosen shape is #ALT-1 *plus* this backstop.) |
| **A separate welcome/onboarding skill as the pointer target** | Reopens the two-door problem the frozen stage-1 record closed (brainstorm:ecosystem-help#D-7); collides with the glossary's "onboarding" reservation; buys nothing — skills cannot self-fire either way. |
| **Generated catalog file refreshed by CI** | Same appetite bar as the enumerator, plus a staleness window between surface change and regeneration — strictly worse than reading live. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | A derivation surface proves too thin or inconsistent to read as-is (the basis's R1) | catalog gaps or wrong routing | each gap is filed as a defect/amendment against the *surface* (its own PR); the appetite bars compensating machinery here | the eval checklist failing names the surface, not a help patch — visible in the implementation PR's evidence |
| R2 | Prompt-walk discipline drifts over time (reads skipped, memory substituted) | stale or fabricated catalog entries — the drift class the feature exists to kill | #DINV-1/#DINV-2 pinned by A-2/A-4; the conformance test runs on every harness run; the eval checklist re-runs at the M1 proxy checkpoints | CI/harness results riding implementation and proxy-run evidence |
| R3 | The first-steps layer accretes curriculum content (the basis's R2) | authored content with a freshness problem | #DINV-1's zero-per-capability-literals scan structurally caps what `start` can hold; review discipline on help-file diffs | the A-4 scan; NR-1's diff-bound inspection |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Bare `/help` on the installed surface lists every installed skill exactly once and every stage row of the shipped map with its built-status, ≤ 2 output lines per capability, grouped by pipeline position | Demo | recorded run transcript in the implementation PR | prd:ecosystem-help/FR-1 · #DINV-4 |
| A-2 | The EVAL-1 scenario checklist clears its bar: 100% correct entry naming on the canonical set; zero fabricated commands or stages on any set; zero commands recommended for unbuilt stages (status stated every time) | Eval | the completed scenario checklist with results | prd:ecosystem-help/FR-2 · UB-2 · UB-3 · #DINV-2 |
| A-3 | `/help start` lands directly on the first-steps path with no interactive navigation, and the same content is reachable conversationally ("where do I start?") | Demo | recorded run transcript | prd:ecosystem-help/FR-3 · FR-6 |
| A-4 | The purity conformance test passes both arms: the text-level scan finds zero per-capability literals in help's own files, and the add-a-skill sandbox fixture changes the catalog with zero help-side edits | Test | harness result in the implementation PR | prd:ecosystem-help/FR-4 · #DINV-1 |
| A-5 | The skill resolves through every vendor pointer present in the repo | Inspection | the implementation PR's pointer diff + resolution check | prd:ecosystem-help/FR-5 |
| A-6 | With a derivation surface removed in a sandbox copy, the answer contains zero entries derived from it and names the absence explicitly | Test | sandbox-run result | prd:ecosystem-help/UB-1 · #DINV-2 |
| A-7 | The implementation diff outside help's own files is limited to vendor pointers, an additive pipeline-map row, and the living-rollup updates the shipped-capability convention requires | Inspection | the implementation PR's diff | prd:ecosystem-help/NR-1 |
| A-8 | Help's own files (skill glue + authoring guide) contain no write, cache, or persistence instruction; the recorded sandbox transcripts (A-1/A-3/A-6) show no file mutations | Inspection | the implementation PR's file review + the transcripts | §2 · #DINV-3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | When does installer-v1's welcome work-item template gain the `/help start` string — with help's implementation PR or the installer's next change? (The string is fixed here; the wiring is the installer side of the S-2 seam. Inherits prd:ecosystem-help/Q1's timing.) | operator | none — help works without the pointer | open | decided when the two implementations land |

## Principles

- **AP-4 / AP-25 — honored.** The skill is authored once on the vendor-neutral surface; vendor directories gain pointers only; it behaves identically on every supported CLI.
- **AP-6 — honored.** #DINV-4 is the token budget as a binding element: routing surfaces only, bounded output; the walk never loads corpus bodies.
- **AP-10 — deviation with reason.** The enumeration rule lives in a prompt, not code — the basis appetite bars the codified alternative (#ALT-2). The mechanical half is recovered where it matters: purity and completeness are enforced by the A-4 conformance test on the shipped harness, so the load-bearing rule is still machine-checked.
- **AP-19 — honored.** The skill fires only on invocation; idle cost is zero; no recurring path exists.
- **AP-21 — honored.** Routing judgment happens once, at the richest-context site (the invocation, with the freshly derived set in view); nothing re-judges downstream.
- **AP-27 — honored.** The catalog is a derived aggregate — no authored center; even the conformance test's assertion list is derived from the live surfaces at test time.
- **RW-5 — honored.** This document decides exactly one independently-revisable question: the derivation mechanism (with its subordinate entry-grammar and honesty mechanics, which only change together with it).
- **Deviation:** AP-10 as above — argued, bounded, and backstopped; no other deviation.
