# Argus ledger docgate lint — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A two-arm docgate lint that gives the Argus ledger classes (problem-ledger, delivery-ledger, authority-contract) their Q-1 mechanical guard — a git-history arm catching edited append-only entries and a cross-document arm catching citations of the regenerable derived-state block. |
| **Relevant-when** | brainstorming or contracting mechanical enforcement of the Argus ledger/authority-contract invariants (DINV-1/2/6 of argus-artifacts); resolving argus-artifacts Q-1; deciding how a history-based or cross-document docgate check composes; checking prior rulings about pulling the ledger-lint forward from the engine era |
| **Idea** | The Argus ledger classes — problem-ledger, delivery-ledger, and authority-contract — have no mechanical lint yet; their design left it as open question Q-1. Two of their invariants can't be caught by a per-file structure check: an edited append-only entry (that needs git history to see) and a downstream document citing a ledger's regenerable derived-state block (that needs a cross-document scan). I want a docgate lint that catches these two — a history arm and a cross-document arm — so the ledger classes get the same gate protection every other document class already has. |
| **Operator** | Argus (AI Engineer, decision principal under authority-contract/v1, tier floor) — every decision below is an attributed assumption anchored to its `delivery:argus-ledger-lint#E-n` journal entry, never a human turn (feature-brainstorm FR-16/FR-17) |
| **Date(s)** | 2026-07-11 |
| **Status** | gated: recommendation only — the gate verdict is requested from the human (see Gate) |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | brainstorm:argus-tech-design (the frozen design brainstorm behind argus-artifacts, whose D-4 fixed the ledger classes and Q-1's staging) |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/tech-design/argus-artifacts.md | the idea names its Q-1; §3.5 DINV-1/2/6, §3.6 (lint deferred), §3.8 (docgate boundaries: classes outside `check-immutable` "until the Q-1 lint gives them their own mechanical guard"), §7 Q-1 owner "engine era" |
| docs/tech-design/argus-agent-v1.md | the drive protocol and assumption regime this session runs under (decision-principal, legality tests) |
| docs/tech-design/docgate-v1.md | the check architecture the arms compose into: check-as-function→defect-list, the `docgate gate` umbrella, `check-immutable` shelling `git diff --name-status <merge-base>` (the history-arm precedent), desk+CI parity |
| docs/tech-design/distributed-check-registration.md | how a new check composes: self-registered descriptor (verb · help · applicability flag "needs a history base" · run behavior over one context carrying repo root, resolved history base, config); umbrella skips base-requiring checks when no base is present |
| docs/tech-design/docgate-v2.md | the `lint-*` verb family precedent (`lint-tech-designs`) and its `internal/docgate/<class>` package shape |
| .pyxis/authoring/feature-brainstorm.md + .pyxis/templates/brainstorm-record-template.md | the stage-1 discipline and the record contract this file instantiates |
| docs/prds/feature-brainstorm/002-argus-principal-seam.md | FR-16/FR-17/NG-8/EVAL-6 — the landed seam making a granted agent a legal stage-1 principal whose decisions are recorded, attributed, journal-anchored assumptions |
| docs/wiki/glossary.md | confirms `problem:` / `delivery:` are the Argus grammar namespaces and the decision-principal / reasonable-assumption / legality-test vocabulary |
| docs/prds/argus/000-base.md | §6.5 graduated-floor rollout; appetite posture ("bootstrap increment only"); the authority ladder this drive is bound by |

**Overlaps surfaced:** none that cover the idea. The Contract Compiler (brainstorm:contract-compiler) is also a cross-document concern but a different one — cross-*stage* assembly and executability (pipeline stage 6), not enforcement of a single class's append-only/derived-block invariants. The artifacts design's Q-1 *is* this idea, but as an open question (`Blocks: nothing`, status `open`) with a default "engine-era" trigger — a staging note, not a covering capability; resolving it earlier is this session's D-9. No merged capability, NG/WR row, or prior gate refuses it.

## Framings

- **Registry-composed checks (adopted)** — the feature *is* two docgate checks that self-register per distributed-check-registration's descriptor model and run under the existing `docgate gate` umbrella: a base-requiring history arm and a base-free cross-document arm. Implication: reuses the entire settled check architecture and composition model; the only novelty is the two check bodies and their fixtures; the ledger classes reach byte-for-byte the same umbrella/desk/CI treatment every other class has, with zero shared-line edits (AP-23). Opens: the pattern for future per-class history/cross-document checks.
- **A bespoke ledger linter (rejected)** — a standalone tool or verb dedicated to Argus ledgers, outside the docgate umbrella. Implication: a second gate surface to wire into desk+CI, a second invocation shape — violates docgate-v1's AP-11 "one predicate, one invocation shape" and duplicates the umbrella. No reason the ledger classes need their own gate when the umbrella already composes heterogeneous checks.
- **Codify the templates' structure checklists only (rejected as insufficient)** — turn the existing per-file structure-checklist appendices into a structure-lint verb and stop there. Implication: catches the structure-checkable DINVs (attribution present, tier-in-ladder) but *by construction cannot* catch the two invariants the idea names — an edited history entry and a cross-document citation are exactly the non-per-file properties a structure check misses. This is a valuable *third* arm but not the feature the idea asks for; parked as fog G-1.

**Destination:** Registry-composed checks — two self-registered docgate checks (a git-history arm + a cross-document arm), composed under the existing `docgate gate` umbrella, giving the Argus ledger classes their Q-1 mechanical guard now. Rationale (assumption, delivery:argus-ledger-lint#E-2): the mechanism is fully settled by docgate-v1 and distributed-check-registration, the idea names exactly two non-per-file invariants, and this framing delivers them with the least new surface and full parity with every other class.

## Decisions

### D-1 — What the feature is (the Destination)

- **Question**: What is the feature — which of the three framings (registry-composed checks · bespoke linter · structure-checklist codification) is the thesis?
- **Answer**: Two self-registered docgate checks (history arm + cross-document arm) under the existing `docgate gate` umbrella; the ledger classes reach parity with every other document class.
- **Reasoning**: The mechanism is settled (docgate-v1 check-as-function + umbrella; distributed-check-registration descriptors); the idea names exactly two non-per-file invariants; this framing adds the least new surface (two check bodies + fixtures) and no shared line (AP-23). Bespoke linter duplicates the umbrella (fails AP-11); structure-only codification cannot catch either named invariant.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-2
- **Status**: live

### D-2 — The problem framing

- **Question**: What precisely is broken today that this fixes?
- **Answer**: The ledger classes' only guard is the templates' per-file structure checklists at review time; that check cannot see two invariants the classes already declare — (a) an *edited* previously-merged append-only entry (`A-n`/`E-n`), because append-only-ness lives in git history not the current file; (b) a downstream document that *cites* a delivery ledger's regenerable derived-state block, because that is a cross-document relationship and the block is defined never-citable (DINV-6). Both are silent, unenforced corruption paths.
- **Reasoning**: A faithful read of the frozen argus-artifacts §3.5 (DINV-1 `A-n` append-only forever, DINV-2 delivery journal append-only, DINV-6 derived block never cited/authoritative) and §3.6/§3.8, which state review is the only guard "until the Q-1 lint gives them their own mechanical guard." Names the gap Q-1 names; asserts no new invariant.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-3
- **Status**: live

### D-3 — Users and segments

- **Question**: Who is this for, and who is explicitly not?
- **Answer**: Primary segment = whoever writes or reviews an Argus ledger — Argus on every drive, the human reviewer at each PR, a resuming session trusting the journal. Anti-segment = the engine-era *runtime consumer* of these files (the checkpoint-walk verb, a future compiler read).
- **Reasoning**: The lint guards authorship integrity at the gate; it does not build the closed-parse runtime reader, which is Q-1's *other* half (§3.6 "closed parse, DINV checks" — this feature takes the DINV-checks-at-gate portion). Keeping the runtime consumer out of segment is what bounds scope to the two arms.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-4
- **Status**: live

### D-4 — Jobs

- **Question**: What jobs must the feature do?
- **Answer**: Two, one per arm. (1) History arm: "when a push edits or removes a previously-merged `A-n` or `E-n` entry — body, anchor, numbering, or presence — tell me before it lands." (2) Cross-document arm: "when any document cites `delivery:<slug>#` into the derived-state region rather than a journal `E-n`, tell me — that block is never citable (DINV-6)."
- **Reasoning**: Both jobs are the idea verbatim, grounded in the frozen DINVs (F-1, F-2). The history arm models `check-immutable`'s `git diff --name-status <merge-base>` shell (F-3), extended to line/entry grain within the two ledger files; the cross-document arm is a repo-wide citation scan (F-4).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-5
- **Status**: live

### D-5 — Goals direction: the bet and its observable success

- **Question**: What is the bet, and how is success observed?
- **Answer**: Bet — the ledger classes reach parity: a mechanical gate guard, not review-only discipline, so the two silent-corruption paths cannot merge unnoticed. Success — a per-DINV defect fixture is red at exactly its arm and green on legal appends, and the gate's verdict on non-ledger trees is byte-identical before and after.
- **Reasoning**: Reuses the artifacts design's own A-7 defect corpus (edited `A-n`, second `delivered`, unattributed entry, out-of-ladder tier, bare pointer flip, cited derived block) as the acceptance spine, honoring AP-13/AP-14 (verified artifacts, golden fixtures) and distributed-check-registration A-4 (real-corpus byte-identical verdict).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-6
- **Status**: live

### D-6 — Scope: exactly two arms, DINV-anchored, composed as registered checks

- **Question**: What is in scope, precisely?
- **Answer**: Exactly the two arms the idea names. History arm → enforces DINV-1 + DINV-2; a base-requiring check (applicability flag "needs a history base"). Cross-document arm → enforces DINV-6; a repo-wide scan, base-free. Both contributed as self-registered descriptors adding no line to any shared umbrella/dispatch/help. The structure-checkable DINVs (DINV-3/4/5) are a candidate *third* structure arm, parked as non-blocking fog, not in this feature.
- **Reasoning**: The two-arm scope is the idea verbatim; the third-arm restraint honors the idea's stated boundary ("two of their invariants … can't be caught by a per-file structure check") and defers rather than decides. Composition-as-descriptors follows distributed-check-registration DINV-2 (single-file contribution, no shared edit).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-7
- **Status**: live

### D-7 — Non-goals

- **Question**: What does the feature deliberately refuse?
- **Answer**: (a) no fold/projection for these classes (§3.8 "no fold"); (b) no closed-parse *runtime* reader for engine consumption (Q-1's other half, engine-era, the anti-segment); (c) no change to `check-immutable`'s governed set — the ledger classes stay outside it, the history arm is a *separate* check governing them; (d) no authority-contract content validation beyond what the two arms touch (malformed-tier / bare-pointer-flip is DINV-4 structure territory — the parked third arm).
- **Reasoning**: Each refusal quotes a frozen boundary (§3.8) or the idea's own two-arm framing; each is an NG/WR seed (see Out of scope). Keeping the history arm separate from `check-immutable` preserves the artifacts design's deliberate disjointness (§3.8).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-7 (scope) · delivery:argus-ledger-lint#E-8 (non-goals)
- **Status**: live

### D-8 — Appetite

- **Question**: What is this problem worth in spend/scope?
- **Answer**: One docgate increment — two self-registered checks under `internal/docgate/`, the per-DINV defect fixtures (reusing artifacts A-7), and desk+CI wiring through the existing `docgate gate` umbrella. No engine-hosted runtime, no new binary, no org-governance change.
- **Reasoning**: Bounded well under a feature: it reuses the settled check architecture and composition model; the novelty is two check bodies and their fixtures. Mirrors the argus PRD's "bootstrap increment only" posture and the repo's Go-first / stdlib-first rules.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-9
- **Status**: live

### D-9 — Q-1 timing: build the lint now rather than at the engine-era default trigger

- **Question**: The artifacts design stages this lint for the engine era, "on volume evidence." Is building it now legal, or does it override a recorded decision?
- **Answer**: Legal to build now; recorded as an assumption, not an escalation.
- **Reasoning**: Q-1 is an **open question** (`Blocks: nothing`, status `open`) — not a frozen Decision — so resolving it earlier than its default trigger contradicts no frozen prose; the frozen §3.8 text is the *transition itself* ("outside its governed set **until** the Q-1 lint gives them their own mechanical guard"), which this feature performs. It moves through the normal design channel (a successor tech-design answering the open question), so no standing content moves through a wrong channel. It is additive and report-only, so no unamended contract breaks — a previously-review-only guard becoming mechanical strengthens the class. The narrower alternative (park as fog, recommend `needs-investigation` pending an operator timing ruling) was rejected: the idea is an explicit operator request to build it now, the mechanism is fully settled, and the precedent classes (brainstorm-record Q1, the design index) each graduated their guard the same way — a legal assumption remains, so the gap is not breaking.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-10
- **Status**: live

## Facts

- **F-1**: The ledger invariants are frozen as argus-artifacts §3.5 — DINV-1 (`A-n` append-only forever), DINV-2 (delivery journal append-only, closes exactly once), DINV-6 (derived-state block regenerable, never cited, never authoritative). — source: docs/tech-design/argus-artifacts.md §3.5
- **F-2**: The classes' current guard is review-time structure checklists only; §3.8 states they "live outside [`check-immutable`'s] governed set until the Q-1 lint gives them their own mechanical guard; until then the templates' checklists and review are the guard." — source: docs/tech-design/argus-artifacts.md §3.6, §3.8
- **F-3**: docgate already has a history-based check: `check-immutable` shells `git diff --name-status <merge-base>`, isolated in `internal/docgate/gate`; the pre-commit hook runs only tree-local checks because "history checks need a merge base." — source: docs/tech-design/docgate-v1.md §3.1; CLAUDE.md doc-system rule 2
- **F-4**: A new check composes by self-registering a descriptor (verb · one-line help · applicability flag "needs a history base" · run behavior over one context carrying repo root, resolved history base, config); the umbrella skips base-requiring checks when no base is present, and adds no shared enumeration/dispatch/help line (single-file contribution, DINV-2). — source: docs/tech-design/distributed-check-registration.md §3.1, §3.3
- **F-5**: The per-DINV defect fixtures this feature needs already exist as named acceptance in the artifacts design — A-7: edited `A-n`, second `delivered` entry, unattributed entry, out-of-ladder tier state, bare pointer flip, cited derived block, "each red at exactly its row." — source: docs/tech-design/argus-artifacts.md §6 A-7
- **F-6**: `problem:` and `delivery:` are established Argus citation namespaces, disjoint from every other grammar token, anchors resolved by grep; the cross-document arm scans for `delivery:<slug>#` references. — source: docs/wiki/glossary.md (Argus section); docs/tech-design/argus-artifacts.md §3.4
- **F-7**: The `lint-*` verb family and per-class package shape (`internal/docgate/<class>`) already exist — e.g. `lint-tech-designs` in `internal/docgate/techdesigns`. — source: docs/tech-design/docgate-v2.md

## Fog

- **G-1** (non-blocking — blocks: a downstream phase): a third, per-file *structure* arm codifying the structure-checkable DINVs (DINV-3 attribution present, DINV-4 tier-in-ladder / no bare pointer flip, DINV-5 no content granting a never-class act) and the templates' checklist appendices into a `lint-structure`-style verb over these classes. The idea scopes exactly the two non-structural arms; whether the structure arm ships with them, after them, or as its own feature is a scoping decision for the technical design or a follow-on brainstorm, not this session.
- **G-2** (non-blocking — blocks: a downstream phase): the exact granularity and matching rule of the history arm — line-grain diff of a merged entry's body vs. entry-block identity vs. anchor/numbering stability, and how it distinguishes a legal *append* (a new higher-numbered entry, or the one legal mutation of a governing doc's `Links:` line, if any analog applies) from an illegal *edit*. A design-stage concern (it mirrors `check-immutable`'s own diff-classification choices); does not gate the destination.

## Out of scope

- **O-1**: No fold or projection for the ledger classes — they take none by contract. — reason: argus-artifacts §3.8 ("these classes take no fold, are not PRD-ledger entries"). (from D-7)
- **O-2**: No closed-parse runtime reader for engine consumption — that is Q-1's other half and engine-era. — reason: the anti-segment (D-3); §3.6 splits closed-parse-for-runtime from DINV-checks-at-gate. (from D-3, D-7)
- **O-3**: No widening of `check-immutable`'s governed set — the ledger classes stay outside it; the history arm is a distinct check. — reason: preserves the artifacts design's deliberate disjointness (§3.8) so its appends never trip `check-immutable`. (from D-7)
- **O-4**: No authority-contract content validation beyond the two arms — malformed-tier / bare-pointer-flip is DINV-4 structure territory (the parked third arm). — reason: keeps the feature to exactly the two named non-per-file arms. (from D-7)

## Candidate glossary terms

- **T-1**: history arm — a docgate check that requires a git merge-base and compares merged content against the working tree to catch an edited/removed previously-merged entry; the append-only enforcer for the Argus ledger classes. · avoid: "immutability check" (overloaded with `check-immutable`, which governs a different, disjoint set).
- **T-2**: cross-document arm — a base-free docgate check that scans the whole corpus for a citation pattern resolving into a forbidden region — here, a `delivery:<slug>#` reference landing in the regenerable derived-state block rather than a journal `E-n` (DINV-6). · avoid: "link check" (doc-link-integrity owns relative-link resolution; this is a citation-target-legality check).

## Parked technical constraints

- **P-1**: The history arm is a base-requiring check — it must degrade exactly like `check-immutable`: the pre-commit/desk path without a merge base cannot run it (history checks need a merge base); CI and the pre-push path (which have `@{push}`/`origin/main`) do. The applicability flag "needs a history base" (distributed-check-registration §3.1) is the seam that expresses this. — conveyed to stages 3–4.
- **P-2**: Both arms are report-only and must not mutate (docgate-v1#DINV-3); the cross-document arm is repo-wide, so its cost and the memoization of the corpus read (shared with other repo-wide checks) is an assembly detail for the design (distributed-check-registration §3.2, Q2). — conveyed to stages 3–4.
- **P-3**: The cross-document arm must resolve a `delivery:<slug>#anchor` citation and decide whether the anchor names a journal `E-n` (legal) or a derived-state field (illegal) — i.e. it needs the delivery-ledger's section structure (Derived state vs Journal) to classify the target. This couples it lightly to the delivery-ledger/v1 template shape (version-pinned, like every other closed-parse registry). — conveyed to stages 3–4.

## Gate

*Recorded as a recommendation only. Signing a stage gate is the action class `sign-gate-verdict`, which authority-contract/v1 places in tier-2 — ungranted at the current tier (floor). Per the drive guide's ungranted-act path, Argus does not self-sign; it journals the request (delivery:argus-ledger-lint#E-11) and presents the recommended verdict for the human to sign. The row below is the recommendation and the request, not a signed verdict.*

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd — **recommended, not signed** (requested from the human; `sign-gate-verdict` is ungranted at floor — delivery:argus-ledger-lint#E-11) |
| **Date** | 2026-07-11 (recommendation recorded) |
| **Operator** | Argus (recommendation) — human signature pending |
| **Covering source** | — |
| **Coverage check** | recommended pass — all six decision-shaped inputs covered by live decisions, zero blocking fog (both `G-n` are non-blocking); see the coverage walk below. Binding on the human's signature. |

**Coverage walk (Appendix B), for the recommended `ready-for-prd`:**

| prd-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2.1 problem statement | ≥ 1 live `D-n` fixing the problem framing (the Destination's thesis) | ✅ D-1, D-2 |
| §2.2 users and segments | ≥ 1 live `D-n` naming primary segment + anti-segment | ✅ D-3 |
| §2.3 jobs | ≥ 1 live `D-n` per claimed primary job | ✅ D-4 (both arms) |
| §3.1 goals direction | ≥ 1 live `D-n` stating the bet and its observable success | ✅ D-5 |
| §3.4 non-goals | the `O-n` set | ✅ O-1..O-4 (from D-7) |
| header appetite | ≥ 1 live `D-n` bounding spend/scope | ✅ D-8 |

**Gate condition:** zero `blocking` fog entries remain — satisfied (G-1, G-2 both non-blocking). The recommended verdict is therefore `ready-for-prd`; the human's signature converts the recommendation into the gate.
