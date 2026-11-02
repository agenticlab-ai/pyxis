# Typed-citation resolution — the `check-citations` gate — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge is the freezing event; "superseded" is computed by the index)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — document-system / gate surface |
| **Date** | 2026-07-14 |
| **Relevant-when** | designing or changing the typed-citation resolution gate; implementing `check-citations`; triaging a dangling-citation defect or a false-positive on a superseded reference; superseding the check |
| **Scope** | The base-free gate check that resolves **typed cross-document citations** to a real anchor: the grammars it covers, the present-vs-dangling resolution rule, its reconciliation with `gen-index`'s Serves flag, and the code-span/fence exemption. **Not**: relative markdown file links (`check-links` owns them); Argus `delivery:` citations (`check-ledger-citations` owns them); the design-index staleness flags (`gen-index` owns them, unchanged); whether a citation is *semantically* apt (a reviewer judgment, never mechanical). |
| **Tracking** | — |
| **Builds-on** | docs/tech-design/tech-design-conventions.md · docs/tech-design/docgate-v1.md |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | none — a read-only gate check over repository documents; no identity, secret, network, or egress surface. |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The typed cross-document citations that carry the pipeline's traceability — the requirement↔design↔record join a reviewer verifies in the upstream-alignment lens — are only partly gate-checked. `check-links` resolves relative markdown *file* links; `gen-index` resolves design `Serves` anchors into a non-fatal `stale-serves` *flag*; `check-ledger-citations` resolves Argus `delivery:` citations. The grammars a reviewer reads to confirm alignment — a PRD's `Traces`, and `brainstorm:` / `design-brainstorm:` / `corpus:` / `design:#anchor` citations wherever they appear — resolve to a real anchor or they do not, and today **only a human reviewer catches a broken one**: it costs review tokens, and a dangling citation (a typo'd ID that never existed) can merge unnoticed. tech-design-conventions §3.4 already named a "resolution checks" increment for docgate v2; this is that check, for the typed grammars nothing yet enforces. The cheapest moment is now: the reviewer is told to "start where the gate stops," so every citation the gate resolves is one the review no longer spends tokens re-checking.

## 2. Decision

**Decision:** A base-free docgate check, **`check-citations`**, resolves every typed cross-document citation in the governed documents to a real anchor. A citation whose target is **present — live *or* superseded** — resolves and passes; a citation whose target is **absent from its resolvable universe** is a **dangling** defect and fails the gate. It covers the grammars nothing else checks (`prd:`, `brainstorm:`, `design-brainstorm:`, `corpus:`, `design:#anchor`), reusing `check-links`'s fenced-block and inline-code exemption so a citation shown as an example never fires. It **leaves `gen-index`'s `stale-serves` flag intact**: a design serving a *superseded* requirement stays the deliberate non-fatal signal (tech-design-conventions §3.2, the §12 stale-cite posture) — that target still resolves, so `check-citations` passes it; the new fatal class is the *never-existed* target the flag never distinguished.

A reader who stops here should see the shape: one deterministic pass over the doc tree; a citation resolves iff its unit exists somewhere in its projection, record, or index (superseded counts); a dangling target is fatal, a moved-on target stays an advisory flag, and an example in a code span is neither.

## 3. The design

### 3.1 The citation grammars and their resolvable universe

**Decision:** the check parses a **closed set** of typed citation grammars, each with a defined resolvable universe (AP-6 — an unknown `word:`-shaped token is out of scope, never a lenient pass):

| Grammar | Resolves against | Resolves iff |
|---|---|---|
| `prd:<feature>/<ID>` | the feature's PRD projection (`docs/prds/<feature>/`) | `<feature>` is a PRD ledger dir and `<ID>` appears in its folded projection, live or superseded |
| `design:<slug>` · `design:<slug>#<anchor>` | the design index + the target design file | `<slug>` is an indexed design; if `#<anchor>` given, that design contains the anchor (`DINV-n` · `A-n` · `#<section>` · a `**Decision:**` id) |
| `brainstorm:<slug>#<anchor>` | `docs/brainstorms/<slug>.md` | the record exists and contains the anchor (`D-n` · `F-n` · `G-n` · `O-n` · `T-n` · `P-n`) |
| `design-brainstorm:<slug>#<anchor>` | `docs/brainstorms/<slug>-tech-design.md` | the record exists and contains the anchor (`D-n` · `ALT-n` · `S-n` · …) |
| `corpus:<ID>` | the placeholder requirement corpus (the flat `docs/prds/*.md`) | `<ID>` is a corpus requirement ID (`SCAF`/`PLAN`/…/`VNEXT`-n) |

`doc:<path#anchor>` resolves its file like `check-links` does; its fragment is deferred to §7 Q1 (check-links Q1 parity). `problem:` / `delivery:` are out of scope — `check-ledger-citations` owns them (§3.4).

### 3.2 Where citations are scanned

**Decision:** the check walks the same governed corpus `check-links` walks (the `docs/` tree plus the operating manual and templates), and finds citation tokens in prose and table cells. It reuses `check-links`'s line scanner: a token inside a fenced block or an inline code span is an example, not a citation, and is skipped (DINV-3) — the two-stage recognition the cross-document ledger arm already established (a resolvable slug shown as `` `prd:x/FR-1` `` never fires).

### 3.3 The resolution rule and reconciliation with `gen-index`

**Decision:** resolution is **present-or-dangling**, and superseded counts as present. A unit ID that appears anywhere in its projection — a live row *or* a "Superseded & retired" row — resolves; supersede is not retire, and a citation to a superseded unit is legitimate (the stale-cite signal). Only a target **absent entirely** — a typo, a never-existed ID, a deleted record — is dangling and fatal.

This is deliberately **complementary to `gen-index`, not a replacement**: `gen-index` flags a live design whose `Serves` target was *superseded* (`stale-serves` — resolvable, so `check-citations` passes it; the flag is the review prompt to re-anchor). `check-citations` fails a citation whose target *never existed* — the case the flag cannot distinguish from a legitimate stale one. `gen-index`'s flags and computation are **unchanged** (DINV-5); the two run side by side, one advisory, one fatal.

### 3.4 Boundaries and non-goals

- **Not solved here** — relative markdown file links (`check-links`); Argus `delivery:`/`problem:` citations (`check-ledger-citations`); the design-index `stale-serves`/`stale-builds-on` flags (`gen-index`); whether a citation is *semantically* apt (does the cited FR actually support the claim — a reviewer's judgment, never mechanical).
- **Owned by a future design** — `doc:` fragment resolution (§7 Q1); a `problem:` resolver if a gap is found (§7 Q2); adding a grammar to the closed set (a successor increment, never a silent widening).
- **Implementation must not infer** — that a superseded target is a failure (it resolves, §3.3); that a citation in a code span is real (§3.2); that an unknown `word:` token is a citation (§3.1's set is closed).
- **Must remain unchanged** — `gen-index`'s flags and index; `check-links`; `check-ledger-citations`; every existing gate verdict on a tree with no dangling citation.

### 3.5 Observability

- **Events/logs** — one named `schema.Defect` per dangling citation: the citing document, the line, the exact citation token, and the universe it failed to resolve against.
- **Metrics / Alerts / Trace** — none: a build-time gate check with no standing process (AP-19); its result is the umbrella's exit code.
- **Debugging surface** — the defect names the token and what it tried; the fix is to correct or remove the citation.

### 3.6 Security & privacy

none — matches the header row: a read-only pass over repository markdown; no identity, secret, network, or egress surface.

### 3.7 Invariants

- **DINV-1** — A citation whose target is present in its resolvable universe (live or superseded) never fails.
- **DINV-2** — A citation whose target is absent from its universe fails with a named defect identifying the citing document and the token.
- **DINV-3** — A token inside a fenced block or an inline code span is never resolved — it is an example, not a citation.
- **DINV-4** — The check reads only; it writes no file and requires no merge base (base-free; it runs on every umbrella path).
- **DINV-5** — `gen-index`'s `stale-serves`/`stale-builds-on` flags, `check-links`, and `check-ledger-citations` behave identically before and after this check exists.
- **DINV-6** — The citation-grammar set is closed: a `word:`-shaped token outside §3.1 is out of scope, never leniently resolved and never a false defect.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Make every design `Serves` resolution failure fatal (fold the stale flag into a hard error) | breaks tech-design-conventions §3.2's deliberate non-fatal `stale-serves` — a design legitimately serves a superseded requirement (supersede ≠ retire); the stale-cite signal is a review prompt, not a merge block |
| Fold into `check-links` | `check-links` resolves file *paths* against the filesystem; typed citations resolve *anchors* against projections, records, and the index — a different resolver over different universes |
| Resolve `doc:` fragments now | deferred to match `check-links` Q1 (fragment resolution across the corpus is its own increment); files still resolve |
| A single "all citations everywhere" mega-check including `delivery:`/`problem:` | `check-ledger-citations` already owns the Argus-ledger grammars with ledger-specific region rules; duplicating them invites drift — this check covers only the unowned grammars |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | a legitimate citation to a *superseded* unit false-fails | a valid stale-cite blocks the gate; §3.2's posture broken | present-includes-superseded rule (§3.3, DINV-1) | fixture: a `prd:` citation to a superseded ID passes green |
| R2 | a real broken citation in a grammar outside the closed set slips through | a dangling reference merges | the set covers the grammars in live use; a novel grammar is documented out-of-scope and still caught by a human reviewer; adding one is a successor increment (§3.4) | inspection of the grammar set against the glossary's citation grammars |
| R3 | own-corpus false positives on the repo's real citations | the gate reds on merge-day | A-6 own-corpus green gate; red-first fixtures precede it | A-6: `check-citations` green on the current tree |
| R4 | performance — scanning the tree and loading every cited projection | slow gate | one pass; projections and records loaded once and memoized per feature/slug | the docgate gate's runtime stays within its current envelope |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A dangling `prd:` / `brainstorm:` / `design-brainstorm:` / `design:#anchor` / `corpus:` citation reds with a named defect (citing document, line, token, universe) | Test | fixture corpus, red at exactly the dangling row | §3.1 · §3.3 · #DINV-2 |
| A-2 | A citation to a present-but-superseded target passes with no defect | Test | superseded-target fixture green | §3.3 · #DINV-1 |
| A-3 | A citation inside a fenced block or inline code span never fires | Test | code-span/fence fixture | §3.2 · #DINV-3 |
| A-4 | `gen-index` flags, `check-links`, and `check-ledger-citations` produce identical output with and without this check | Test | before/after parity | §3.3 · #DINV-5 |
| A-5 | The check runs base-free under the umbrella and mutates no file | Test | umbrella run + working-tree diff | #DINV-4 |
| A-6 | The current repository tree passes `check-citations` — no false positive on any real citation | Test / Inspection | green run on the live tree | §3.4 · R3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | `doc:` fragment (anchor) resolution — resolve `#anchor` in a `doc:<path#anchor>`, or file-only like `check-links`? | implementation | nothing | open | file-only in v1 (check-links Q1 parity); a fragment increment later |
| Q2 | Does `check-ledger-citations` already cover `problem:` citations, or is that a gap this or a sibling should fill? | implementation | nothing | open | decided at implementation against the existing arm; out of this check's scope regardless |
| Q3 | The `prd:` resolvable universe — read `effective.source.json`, the folded `effective.md` rows, or the base + amendments? | implementation | nothing | open | decided at implementation; the projection is the source of truth, superseded rows included |

## Principles

- **AP-10 — honored.** The reviewer's "citations resolve" check becomes deterministic gate code, not prompt discipline — the review starts where the gate stops.
- **AP-11 / INV-30 — honored.** One more self-registered check under the one umbrella verb — wider fence, one binary, one predicate.
- **AP-26 — honored.** Resolution is against actual content (the projection, the record, the index), never a trusted status or badge.
- **AP-6 / INV-32 — honored.** The citation-grammar set is a closed registry; an unknown grammar is out of scope, never leniently parsed.
- **AP-5 — honored.** Observability is the named defect per dangling citation; nothing else emitted.
- **AP-13 — honored.** "Citations resolve" becomes a verifiable gate artifact, not a review claim taken on trust.
- **AP-19 — honored.** Zero steady-state cost: a build-time check, no standing process.
- **RW-5 — honored.** One decision — the typed-citation resolution check — in one document; the reconciliation with `gen-index` is a boundary, not a second decision.
- **Deviation:** none.
