# tech-design-new — author a technical design

## Draft-born features

For a feature with an open draft, write its tech spec at
`<draft-root>/<slug>/tech-spec.md`, where `<draft-root>` is the configured
`paths.drafts` (here `drafts`). Confirm the draft with
`go run ./cmd/pyxis draft status <slug>` and work on that branch. Use
`prd.md` and any numbered design-brainstorm working file from this same
draft as the feature contract. Read and write the feature's contract only
inside its draft. The scaffolded draft tech-spec template defines the
decision grammar: `### D-n — title`, `**Decision:**`, and
`**Serves:**` with exact PRD IDs. IDs are stable across locked versions.

Write each concrete engineering layout inside the serving decision's
section in `tech-spec.md`: interfaces, data shapes, failure behavior,
observability, security, and verification where relevant. The compiler
stores the entire tech-spec text literally and maps each `D-n` decision
to a typed memory decision. Do not create a separate solution-shape or
`docs/tech-design/` document. Preserve `Builds-on` and `Supersedes` links
as typed decision IDs when they apply. The authoring discipline below binds
a draft's tech spec as well: seam economy, build what the served rows ask
for, and close findings by narrowing first.

If design work exposes a wrong, missing, or unbuildable requirement, append
the next `P-n` to `problems.md` with the affected requirement ID and the
exact defect. If the PRD row exists, compare it before and after this write
and leave it byte-identical. Resume that decision only
after the operator, or Argus under a verified grant, records an attributed
ruling on the same problem and applies its change to `prd.md`. Commit and
push the draft working files on the existing draft branch. The draft
reviewer supplies review evidence in `reviews/NNN.md`; the operator's exact
head approval is checked later at lock.

When the draft contains a `prototype-record/v1`, read every finding while
authoring the tech spec. For each technical challenge the design handles,
write a decision that names the challenge and explains how its layout handles
it, then set the finding's Answer to `design:<slug>#D-n`. A finding the
contract will not handle may be marked `set aside — <reason>`. Product
conflicts must already cite their PRD answer or a set-aside reason. An Answer
still `open` is unresolved and blocks the corresponding review. With no
prototype record, this adds no step.

## Document-pipeline features

The technical-design convention, separate review report, and frozen design
path below continue for features with a merged PRD ledger.

Author a **technical design** — one decision, argued and frozen. The class
rules live in `docs/tech-design/tech-design-conventions.md`; the skeleton
in `docs/templates/tech-design-template.md`. Read both fully first. Input:
the slug (permanent) and what it serves; ask if missing.

## Input: a gated design-brainstorm record (optional)

When the operator names a design-brainstorm record
(`docs/brainstorms/<slug>-tech-design.md` — produced by
`/tech-design-brainstorm`; a solution shape that is genuinely open should
start there, not here):

- **Refuse** a record that is ungated or whose verdict is not
  `ready-for-design` — name the record, its state, and the remedy, and
  write nothing.
- **Author as assembly, don't re-interview**: live `D-n` rulings are
  resolved decisions — never re-ask one (each new decision-question is a
  defect signal, measured by the PRD's M1). §2's Decision comes from the
  direction ruling; §4 materializes from the `ALT-n` ledger with verdicts
  and scores carried; `Serves` and `Scope` derive from the `S-n` seam map
  and the `Basis` anchors; §3.2 boundaries seed from the `O-n` set;
  non-blocking `G-n` fog lands as the design's own `Q-n` rows; `P-n`
  *proceed-anyway* entries become §5 risk candidates.
- **Cite by the record's grammar** (defined in the design-brainstorm-record
  template's Appendix C, never restated here): §2 carries
  `(design-brainstorm:<stem>#D-n)`, every §4 row its `#ALT-n`.
- **Set the record's `Feeds` row** to the design's path, and land the
  record in the same introducing PR as the design.

A recordless invocation follows the steps below unchanged.

## Authoring discipline (both paths)

- **Authoring never invents a design decision.** A genuine gap — something
  the record, the basis, or the conversation neither decides nor parks —
  is named precisely, then either asked **once** (recommendation attached)
  or parked as the design's own §7 `Q-n` row with an owner. Plausible
  filler in a decision's place is how architectures ship that nobody chose.
- **Seam economy.** Prefer existing seams to new ones; the highest seam
  that serves; the fewest that suffice — a new port or interface is a
  design decision to argue (§2/§4), never a reflex. When record-fed, the
  `S-n` seam map is the input; deviating from it is a decision to record.
- **Build what the served rows ask for** (incident I13). Choosing the
  mechanisms that deliver a served row is the design's job — the simplest
  that serve it at the PRD's Today. What a decision may not add is a
  promise the rows do not make: a guarantee, a supported platform, or
  scope, and any mechanism that exists only to uphold one. If you believe
  such a promise is needed, that is a missing requirement — raise it on
  the requirement route (a draft's `P-n`, a ledger's amendment) with a
  rough task count, and let the ruling decide. The standing obligations
  still bind whatever the design does build — the principles it must cite,
  and Observability and Security & privacy for anything that executes —
  but they are met for the mechanisms that exist and are never a reason to
  add one.
- **Close findings by narrowing first.** A review finding against a claim
  you added — a promise no served row asks for — closes by withdrawing or
  narrowing the claim, and that is an applied disposition. Add a mechanism
  only where a served row, or a governing principle applied to something
  the design builds, requires it.
- **The review adjusts; the merge freezes.** The operator review session on
  the draft (step 3) is the *sanctioned* surface for changing direction.
  On a record-fed design an adjustment lands twice in the same change: the
  still-unfrozen record gains the review-anchored ruling (a new or
  superseding `D-n`/`ALT-n` verdict, anchored `YYYY-MM-DD · review`), and
  the design's `(design-brainstorm:<stem>#…)` citations update — provenance
  stays complete until merge freezes both documents.

## Steps

1. **Header** (the closed schema):
   - `Status` — ladder position; **`Approved` before merge** (merge is the
     freezing event; "superseded" is computed by the index, never written).
   - `Serves` — typed requirement joins: `prd:<feature>/FR-3` ·
     `corpus:VNEXT-8` · `design:<slug>[#anchor]` · `doc:<path#anchor>`;
     enumerate anchors explicitly (range shorthand undecided until v2);
     `—` only for pure infrastructure.
   - `Builds-on` (dependency/lineage — never changes the target's
     liveness) vs `Supersedes` (whole-document replacement — retires the
     target). Contradicting a live design **requires** superseding it: two
     live designs never disagree.
   - `Approver(s)`, `Security/privacy impact` (must pair with §3.5),
     `Template version` = `tech-design-template/v1`.
2. **Body** (§1 Problem · §2 Decision · §3 The design · §4 Alternatives ·
   §5 Risks · §6 Acceptance · §7 Open questions · Principles):
   - **Binding elements** are `**Decision:**` callouts, `DINV-n` invariant
     anchors, and `A-n` acceptance rows — implementation obeys these;
     everything else explains. Write them deliberately.
   - **§3 must contain, by name, `Observability` and `Security & privacy`**
     — with substance, or `none — <reason>` (the security one must echo
     the header row). Silent omission fails review (and docgate v2).
   - Acceptance rows: criterion · Verify method · required evidence ·
     traces-to (cite the Serves anchors). Risks and open questions are
     structured tables with owners; questions carry Status.
   - `## Principles`: cite AP/RW IDs honored, and any deviation **with
     reason** — never silently omit a relevant principle.
   - **§0 Read to rule — authored last.** Once §1–§7 and Principles are
     stable, write §0 (before §1) from this design's own content: the
     ranked **Judge these** calls are the `**Decision:**` callouts (§2 and
     load-bearing §3.x) paired with the `#ALT-n` each displaced; the
     numbers are acceptance/DINV bounds; the open questions are §7's
     `Q-n`; the skip map names the downstream backstops (schema → docgate;
     substance/reuse → `tech-design-review`; defects → codegate). Every
     routed item anchors to a §/row that resolves in this document; §0
     fits one screen (≤ 40 source lines, ≤ 7 ranked calls). Required for
     every design (the class has no tiers) — a design that genuinely
     routes nothing carries the whole-section `Skipped: <reason>` line.
     §0 is informative, never normative, and no machine reads it; it is
     refreshed only by a superseding design, never amended. Contract:
     `read-to-rule-v1` §3.1 · design `read-to-rule-tech-design-v1`.
2b. **Run `/tech-design-review` on the post-lint draft** (and its
   solution-shape when one exists; supersessions included) — the unmanned
   reviewer. Apply blocking findings or decline-with-reason (a declined
   blocking finding escalates); its report
   (`docs/reviews/<design-slug>.md`) rides this introducing PR as the
   design's evidence package. *Advisory period: until its ship rule
   promotes adoption, the verdict blocks nothing — findings accumulate as
   measurement.*
3. **Operator review**: present the source doc and proposed edits; apply
   confirmed changes; record the operator's approval in the Status row.
4. **PR**: branch `docs/<slug>-design`, one concern. Until docgate v2
   lints this class, walk the conventions' checklist by hand and say so in
   the Testing section of the PR description
   (`.pyxis/templates/pr-description-template.md`). Merge freezes the document; evolution = a successor doc
   whose `Supersedes` names this one.

## Never

Edit a merged design (supersede it); write "superseded" into your own
header; add a `Serves` ref you don't actually implement; leave §3.4/§3.5
missing rather than explicitly `none — <reason>`; renumber anchors; invent
a design decision the input didn't resolve (ask once, or park it as a
`Q-n`); apply a review-born direction change to a record-fed design
without landing its ruling in the record; open a new seam without arguing
it as a decision; ship a design with no §0 and no honest `Skipped:` line.
