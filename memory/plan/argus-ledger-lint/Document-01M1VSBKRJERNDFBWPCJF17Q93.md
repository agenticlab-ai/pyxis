# Argus ledger lint — the two-arm mechanical guard for the ledger classes — plan 001

| Field | Value |
|---|---|
| **Feature** | argus-ledger-lint |
| **PRD** | docs/prds/argus-ledger-lint |
| **Design** | docs/tech-design/argus-ledger-lint.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Implement the Argus ledger-lint checks — the history arm, the cross-document arm, and their docgate-gate composition |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The shared ledger parse and its closed-parse registries

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 120–230 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/argusledger, internal/docgate/schema |

**Goal:** A single per-class parse turns an Argus ledger into an anchor-keyed map of entry blocks and, for a delivery ledger, its two delimited regions — the one predicate both arms read — and names a defect on a ledger it cannot delimit rather than crashing or passing.

**Current behavior:** No Argus-ledger parse exists. The gate can read a document as bytes but has no notion of an `A-n`/`E-n` entry block, of a delivery ledger's derived-state versus journal regions, or of the anchor grammar and region delimiters as versioned data; a malformed ledger would either crash a consumer or pass unexamined.

**Desired behavior:** A shared parse maps a problem or delivery ledger to `anchor → entry-block bytes`, where a block runs from its dense `A-n`/`E-n` heading to the next entry delimiter or the region or file end; for a delivery ledger it also delimits the derived-state region and the journal region by section heading. The anchor grammar and the version-pinned delivery-ledger region delimiters live as closed-parse registries, additive by version bump, so a template-shape change reviews the registry in the same edit. When a ledger's entries or its derived-state region cannot be delimited, the parse yields a named defect stating which region could not be parsed, never a crash and never a false pass.

**Scope:**
- the anchor-keyed entry-block parse over the problem and delivery ledger classes
- the delivery-ledger two-region delimitation (derived-state versus journal) by section heading
- the version-pinned anchor grammar and region-delimiter registries as closed-parse data
- the named parse defect for a ledger whose entries or derived-state region cannot be delimited

**Out of scope:**
- either arm's comparison or citation logic — this task is the parse only (T2, T3)
- any registration into the gate or any desk/CI wiring (T2, T3, T4)
- the authority contract, which carries no `A-n`/`E-n` anchors and is not in scope for these arms

**Acceptance criteria:**
- [ ] a well-formed delivery ledger parses to its anchor-keyed entry blocks with the derived-state and journal regions delimited
- [ ] a problem ledger parses to its `A-n` entry blocks
- [ ] a ledger whose region cannot be delimited yields a named parse defect identifying the region, with no crash
- [ ] the region-delimiter registry is version-pinned and rejects an unknown shape rather than parsing it leniently

**Key interfaces:**
- a ledger parse producing the anchor-keyed entry-block map and, for a delivery ledger, its two delimited regions
- the closed-parse anchor grammar and delivery-ledger region-delimiter registries, version-pinned
- `schema.Defect` — the named parse defect the parse emits for an undelimitable ledger

**Traces:** prd:argus-ledger-lint/FR-1 · prd:argus-ledger-lint/FR-3 · prd:argus-ledger-lint/UB-3 · prd:argus-ledger-lint/NFR-1 · design:argus-ledger-lint#3.1 · design:argus-ledger-lint#3.2 · design:argus-ledger-lint#3.4 · design:argus-ledger-lint#DINV-5

### T2 — The history arm: entry-block comparison keyed by anchor

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 110–210 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | the two arms add two disjoint self-registered descriptor files sharing only the T1 parse, no shared body (design §3.3, §3.4, AP-27); T3 is a parallel sibling |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/argusledger |

**Goal:** A base-requiring gate check flags a previously-merged `A-n`/`E-n` entry that was edited or removed, stays silent on a legal higher-numbered append, and is skipped entirely when no merge base is present.

**Current behavior:** The ledger classes' append-only-ness is guarded only by a review-time structure checklist that cannot see git history, so an edited or deleted merged entry reads as valid structure and merges unnoticed; no check governs these files under a merge base.

**Desired behavior:** Using the T1 parse, the arm builds an anchor-keyed entry-block map of each in-scope ledger at the merge base and at the working tree and, for every anchor present at the base, requires it to survive present and byte-identical: a changed block is an edit defect, an absent one a removal defect, each naming the ledger and the disturbed anchor. An anchor present only in the working tree, higher-numbered than every merged anchor of its family, is a legal append and raises nothing — even though the delivery ledger's derived-state region, which the comparison ignores, changed. The arm declares itself base-requiring, so the umbrella skips it on the base-free path exactly as it skips the file-granular immutability check, and it mutates no file.

**Scope:**
- the base-map versus working-map anchor-keyed comparison over the in-scope ledgers
- classification of each merged anchor as untouched, edited, or removed, with the derived-state region excluded
- legal-append silence for a new strictly-higher-numbered anchor
- declaring the arm base-requiring so the umbrella skips it with no merge base

**Out of scope:**
- the cross-document citation arm and its recognition or region resolution (T3)
- widening the existing file-granular immutability check's governed set — this arm is a separate guard
- the whole-corpus non-regression and desk/CI parity proofs (T4)

**Acceptance criteria:**
- [ ] on the edited-entry and removed-or-renumbered-entry fixtures the arm reports a defect naming the ledger and the disturbed anchor, red at this arm only
- [ ] on the legal-append fixture the arm reports zero defects even though the derived-state region changed
- [ ] with no merge base the arm is skipped by the umbrella and neither runs nor errors
- [ ] across the fixture runs the working tree is left unchanged by the arm

**Key interfaces:**
- a self-registered gate descriptor for the history arm, base-requiring, delegating to the T1 parse
- the anchor-keyed base-versus-working comparison producing the edit and removal defects
- `schema.Defect` — the edit and removal findings, and the merged-entry survival predicate

**Traces:** prd:argus-ledger-lint/FR-1 · prd:argus-ledger-lint/FR-2 · prd:argus-ledger-lint/FR-5 · prd:argus-ledger-lint/FR-7 · prd:argus-ledger-lint/UB-1 · design:argus-ledger-lint#3.1 · design:argus-ledger-lint#3.3 · design:argus-ledger-lint#DINV-1 · design:argus-ledger-lint#DINV-2 · design:argus-ledger-lint#DINV-4

### T3 — The cross-document arm: recognition boundary and citation-into-region resolution

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | the two arms add two disjoint self-registered descriptor files sharing only the T1 parse, no shared body (design §3.3, §3.4, AP-27); T2 is a parallel sibling |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/argusledger |

**Goal:** A base-free gate check scans the tracked corpus, recognizes a `delivery:` reference as an in-scope citation only through a two-stage boundary, and flags a recognized citation that lands in the derived-state region — while staying green on its own citation-saturated governing prose.

**Current behavior:** A delivery ledger's derived-state region is defined never-citable, but that is prose-only; no cross-document scan exists, so a downstream document that cites the regenerable derived-state block is invisible to any single-file check and can merge as a load-bearing citation of a projection.

**Desired behavior:** The arm enumerates the tracked corpus and finds `delivery:` references, then recognizes an in-scope citation through two stages: a citation-context stage that excludes any occurrence inside a fenced block or an inline-code span, matching the sibling link scanner's corpus convention; then a target-resolvability stage that admits only a reference whose slug names an existing delivery ledger. A reference dropped at either stage — a resolvable slug shown as a code-span example, a literal metavariable placeholder, or a mention of an absent ledger — is not a citation and raises nothing, so the feature's own governing prose stays green. For a recognized citation the arm resolves the anchor against the T1 regions: a journal entry is legal, a derived-state field is a derived-block defect, and an anchor resolving to neither is a distinct unresolvable-anchor defect that is never conflated with the derived-block finding. The arm mutates no file.

**Scope:**
- the tracked-corpus scan for `delivery:` references, bounded to one read of the tracked set
- the two-stage recognition boundary: citation-context exclusion of code spans and fenced blocks, then target resolvability into an existing ledger
- anchor-into-region resolution classifying a recognized citation as legal, derived-block defect, or unresolvable-anchor defect with a distinct discriminator
- silence on every non-citation, including the repo's own citation-bearing prose

**Out of scope:**
- the history arm and its merge-base comparison (T2)
- the whole-corpus byte-identical non-regression and desk/CI parity proofs (T4)
- resolving `problem:` references or any authority-contract target — this arm resolves only into a delivery ledger

**Acceptance criteria:**
- [ ] on the cited-derived-block fixture the arm reports a defect naming the citing document and the derived-region target, red at this arm only
- [ ] a bare-prose citation into a seeded ledger's journal entry is silent, and a resolvable-slug reference shown inside a code span or fenced block raises nothing
- [ ] a recognized citation whose anchor resolves to neither region yields an unresolvable-anchor defect distinct from the derived-block finding
- [ ] on the repo's own citation-bearing prose the arm reports zero defects

**Key interfaces:**
- a self-registered gate descriptor for the cross-document arm, base-free, over the tracked corpus
- the two-stage recognition boundary and the anchor-into-region resolver delegating to the T1 regions
- `schema.Defect` — the derived-block finding and the distinct unresolvable-anchor finding

**Traces:** prd:argus-ledger-lint/FR-3 · prd:argus-ledger-lint/FR-4 · prd:argus-ledger-lint/FR-7 · prd:argus-ledger-lint/FR-9 · prd:argus-ledger-lint/UB-2 · prd:argus-ledger-lint/NFR-2 · design:argus-ledger-lint#3.2 · design:argus-ledger-lint#3.3 · design:argus-ledger-lint#DINV-3 · design:argus-ledger-lint#DINV-4 · design:argus-ledger-lint#DINV-5

### T4 — The acceptance corpus and additive, verdict-preserving composition

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 90–190 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/argusledger, .github/workflows, .githooks |

**Goal:** The full per-invariant fixture corpus proves every defect class red at exactly its arm and every legal append and citation green, and the two arms compose additively — leaving every non-ledger tree's verdict byte-identical, the existing immutability check untouched, and the umbrella's shared enumeration unedited, on both the desk and CI paths.

**Current behavior:** The two arms exist and pass their own per-arm fixtures (T2, T3), but nothing yet proves the cross-arm properties: that each corruption class is red at exactly one arm, that the whole non-ledger corpus verdict is unchanged, that the immutability check's governed set is untouched, and that both arms run under the identical umbrella verb on desk and CI differing only in merge-base source.

**Desired behavior:** The acceptance corpus assembles the per-invariant defect fixtures — an edited entry, a removed or renumbered entry, a cited derived block, a dangling in-scope anchor — plus the legal-append and legal-citation green cases and the recognition-boundary set, and demonstrates each defect is red at exactly its arm while legal cases are green. A whole-corpus check shows the gate's verdict on every non-ledger tree is byte-identical with and without the two arms, the file-granular immutability check's governed set is unchanged and a legal ledger append trips zero of its defects, and the umbrella's shared enumeration, dispatch, and help gained no hand-edited line — the arms appear solely via their descriptors. Both arms are shown to run under the identical umbrella verb on the desk pre-push path and the CI path, differing only in merge-base source, with the base-free path skipping the history arm.

**Scope:**
- the per-invariant acceptance corpus with each fixture's expected arm and verdict declared, including the own-corpus non-regression case
- the byte-identical non-ledger-tree verdict proof and the untouched-immutability-check proof
- confirmation that the umbrella enumeration, dispatch, and help carry no hand-edited line for either arm and list both from their descriptors
- confirmation that both arms run under the identical umbrella verb on desk and CI, differing only in merge-base source, with the history arm skipped base-free

**Out of scope:**
- the arms' own comparison and resolution logic (T2, T3) and the parse (T1)
- the third structure arm, deferred to its own future design

**Acceptance criteria:**
- [ ] every per-invariant defect fixture is red at exactly its arm and every legal-append and legal-citation fixture is green
- [ ] the gate's verdict on the non-ledger corpus is byte-identical with and without the two arms, and the immutability check's governed set is unchanged with zero of its defects on a legal append
- [ ] the umbrella's shared enumeration, dispatch, and help carry no hand-edited line for either arm and list both from their descriptors
- [ ] both arms run under the identical umbrella verb on desk and CI differing only in merge-base source, with the history arm skipped when no merge base is present

**Key interfaces:**
- the golden-fixture corpus directory layout with per-case expected-arm and expected-verdict manifests
- the desk pre-push hook and the CI workflow — the two call sites of the identical umbrella verb
- the umbrella check registry — the derived aggregate the two descriptors register into with no shared-line edit

**Traces:** prd:argus-ledger-lint/FR-6 · prd:argus-ledger-lint/FR-8 · prd:argus-ledger-lint/NR-1 · prd:argus-ledger-lint/NR-2 · prd:argus-ledger-lint/NR-3 · design:argus-ledger-lint#3.3 · design:argus-ledger-lint#3.10 · design:argus-ledger-lint#DINV-6 · design:argus-ledger-lint#A-5

## Coverage

| Requirement | Tasks |
|---|---|
| prd:argus-ledger-lint/FR-1 | T1, T2 |
| prd:argus-ledger-lint/FR-2 | T2 |
| prd:argus-ledger-lint/FR-3 | T1, T3 |
| prd:argus-ledger-lint/FR-4 | T3 |
| prd:argus-ledger-lint/FR-5 | T2 |
| prd:argus-ledger-lint/FR-6 | T4 |
| prd:argus-ledger-lint/FR-7 | T2, T3 |
| prd:argus-ledger-lint/FR-8 | T4 |
| prd:argus-ledger-lint/FR-9 | T3 |
| prd:argus-ledger-lint/UB-1 | T2 |
| prd:argus-ledger-lint/UB-2 | T3 |
| prd:argus-ledger-lint/UB-3 | T1 |
