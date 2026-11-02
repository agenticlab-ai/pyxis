# Argus ledger lint — the two arms' design shape — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The stage-3 design session for the Argus ledger-lint's two arms: how the history arm compares merged entries at a finer grain than file-level `check-immutable`, how the cross-document arm enumerates the corpus and resolves an in-scope `delivery:` citation into a journal-vs-derived region, and where both compose under the `docgate gate` registry. |
| **Relevant-when** | designing or changing the Argus ledger-lint checks (history arm / cross-document arm), their diff-granularity or citation-resolution mechanics, their corpus enumeration, or their `docgate gate` composition; checking prior design rulings or refused shapes about entry-grain history diffing or citation-target-legality scanning over the Argus ledger classes |
| **Basis** | `prd:argus-ledger-lint` (merged ledger — `docs/prds/argus-ledger-lint/effective.md`, FR-1..FR-9 + NR-1..NR-3 + UB-1..UB-3; frozen at PR #185) |
| **Authority** | Argus (AI Engineer — stage-3 decision principal under authority-contract/v1, tier-2; every ruling below is an attributed, legality-tested assumption anchored to its `delivery:argus-ledger-lint#E-n` journal entry, never a human turn — the same principal regime the stage-1 record ran under) |
| **Date(s)** | 2026-07-11 |
| **Status** | gated: ready-for-design — signed by Argus at tier-2 (`sign-gate-verdict` granted; delivery:argus-ledger-lint#E-27) |
| **Tracking** | — *(the design's introducing PR, once filed)* |
| **Feeds** | docs/tech-design/argus-ledger-lint.md |
| **Prior records** | brainstorm:argus-ledger-lint (the frozen stage-1 record this design serves) · design-brainstorm:argus-tech-design (the frozen design brainstorm behind argus-artifacts, whose D-4 fixed the ledger classes and Q-1's staging) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/argus-ledger-lint/effective.md | the merged basis — FR-1..FR-9 (the two arms, report-only, self-registered, base-requiring degradation, FR-9 recognition boundary), NR-1..NR-3, UB-1..UB-3, §5 intro (what stays design-stage: the history arm's append-vs-edit grain; the anchor-into-region resolution mechanics), Q1/Q2, P-1..P-3, WR-1 |
| brainstorm:argus-ledger-lint | the stage-1 record — D-1..D-9 (settled decisions I never re-decide), G-2 (the history-arm grain, non-blocking fog now a design question), G-1 (the third structure arm, WR-1), P-1..P-3 (parked technical constraints conveyed to this stage), T-1/T-2 (the two glossary terms, already landed) |
| docs/tech-design/argus-artifacts.md | the DINV registry the arms enforce — §3.5 DINV-1 (`A-n` append-only forever), DINV-2 (delivery journal append-only, closes once), DINV-6 (derived block regenerable, never cited); §3.2 delivery-ledger region structure (Derived state vs Journal); §3.4 grammar (`delivery:<slug>#`, anchors resolve by grep); §3.8 docgate boundaries (classes outside `check-immutable` until the Q-1 lint); §3.7 injection posture; §6 A-7 corpus |
| docs/tech-design/docgate-v1.md | the check architecture — `internal/docgate/gate` verbs-as-functions → defect lists; the `gate` umbrella (AP-11, one predicate two call sites); `check-immutable` shelling `git diff --name-status <merge-base>`; DINV-3 (verbs never mutate); the fixture/golden layout (§3.9) |
| docs/tech-design/distributed-check-registration.md | the composition contract — the descriptor (verb · help · applicability flag "needs a history base" · run over one context); assembly ordered by verb; base-free skip; DINV-2 (single-file contribution, no shared edit); A-4 (real-corpus byte-identical verdict) |
| docs/tech-design/docgate-v2.md | the per-class precedent — `lint-tech-designs` in `internal/docgate/techdesigns`; the merge-base-needing (`check-governing`, incidents append) vs base-free split; class rules as registries in `internal/docgate/schema` |
| docs/tech-design/tech-design-conventions.md + .pyxis/templates/tech-design-template.md | the class rules the authored design obeys — one decision per document (RW-5), the closed header, mandatory §Observability + §Security & privacy, `DINV-n`/`A-n` binding elements, `## Principles` |
| docs/tech-design/design-brainstorm-record-model.md + template | this record's own contract — DINV-1..8 (authority-anchored rulings, one panel-log line per DQ, ≥2 scored ALT on a fired panel, park-or-bounce), the coverage checklist (core four) |
| docs/architecture-principles.md | the principle IDs the ALT-n and the design cite — AP-10/11/13/14/17/19/23/27, RW-5 |
| the docgate Go codebase (`internal/docgate/gate/{registry.go, reg_check_immutable.go, reg_lint_tech_designs.go, reg_check_links.go, gate.go}`, `schema/schema.go`, `techdesigns/techdesigns.go`, `fold/testdata/`) | the concrete structures the design cites — the live `Check` descriptor + registry, check-immutable's file-granular diff, the base-free repo-wide scan precedent (`check-links`, filesystem walk), the `Defect{Verb,File,Op,Reason,Remedy}` struct, the per-class package shape, the fixture layout — verified in code so the frozen design states no false path |

**Overlaps surfaced:** none that decide the two arms. `check-immutable` (docgate-v1 §3.1) is the nearest live design and the history arm's *model*, but it deliberately governs a **disjoint** set (PRD ledger entries + tech-designs, `cfg.PRDs`/`cfg.TechDesigns`) and is file-granular — it does not decide, and must not be widened onto, the Argus ledger classes (NR-2/NG-3, argus-artifacts §3.8; confirmed in code: Argus ledger paths are absent from its diff scope). `distributed-check-registration` decides *how a check composes* (the descriptor + registry) but explicitly **not what any check verifies** (its §3.2 "Not solved here") — so it is the composition input, not a covering decision. `docgate-v2`'s `lint-tech-designs` is the per-class precedent, not a decision about these classes. Q-1 in argus-artifacts *is* this feature's charter (open question, `Blocks: nothing`), a staging note, not a covering capability. No live design decides the append-vs-edit grain or the citation-into-region resolution; both are genuinely open and are this session's `DQ-1`/`DQ-2`.

## Design questions

- **DQ-1**: How does the **history arm** distinguish a *legal append* (a new, higher-numbered `A-n`/`E-n` entry — FR-2) from an *illegal edit/removal* of a previously-merged entry (FR-1)? A ledger append modifies the same file, so a file-level `git diff --name-status` verdict (what `check-immutable` uses) reports status `M` for both the legal and the illegal case — the arm needs a comparison at a finer grain than the file. What is that grain, and what is compared against what?
- **DQ-2**: How does the **cross-document arm** (a) enumerate its corpus and (b) resolve an in-scope `delivery:<concrete-slug>#<anchor>` citation (FR-9) into one of {journal `E-n` → legal (FR-4), derived-state region → defect (FR-3), neither → unresolvable-anchor defect (UB-2)}? Two sub-seams: the corpus enumeration (which files are scanned) and the delivery-ledger region model (how a resolved anchor is classified into Journal vs Derived state).
- **DQ-3**: At which composition point do the two arms attach — the repo-wide registered-check registry (`Checks()`, iterated by the `gate` umbrella, like `check-links`/`lint-tech-designs`), or the per-ledger-directory `dirs` loop (like `lint-structure`)?
- **DQ-4**: Package placement — do the two arms share one per-class package (`internal/docgate/<class>`) or take a package each, and where do the class's parse registries (the delivery-ledger region delimiters, the citation grammar) live (per docgate-v2's "class rules in `internal/docgate/schema`" precedent)?
- **DQ-5**: Does the third **structure arm** (WR-1 / PRD Q2 / brainstorm G-1 — the DINV-3/4/5 per-file structure checks) ship *with* these two arms, *after* them, or as its *own* feature? (The PRD hands this scoping decision explicitly to "the technical design or a follow-on brainstorm.")

## Panel log

- **DQ-1**: opens/replaces — **fired** — the append-vs-edit grain opens a genuinely new comparison mechanic (the file-granular `check-immutable` verdict cannot serve it; a legal append and an illegal edit are both file-status `M`), and the choice of grain (structural entry-block identity vs line-level text diff vs full-file re-parse) is a real interface into how the arm reads a ledger — ≥ 2 orthogonal shapes exist. Fired: ALT-1, ALT-2, ALT-3.
- **DQ-2**: opens/replaces — **fired** — the corpus-enumeration sub-seam is a real interface choice (a filesystem walk like `check-links` vs the tracked-set `git ls-files` vs a shared umbrella-provided corpus) with a correctness-affecting difference (untracked scratch files), and the region-classification sub-seam opens the delivery-ledger parse boundary — ≥ 2 orthogonal shapes. Fired: ALT-4, ALT-5, ALT-6 (enumeration) and the region model is decided within D-5 as an extension of the settled closed-parse-registry pattern (logged under DQ-2's fired line; the enumeration is the structurally-distinct axis).
- **DQ-3**: extends — **skipped** — the registered-check registry is the settled seam (distributed-check-registration §3.1, live in `registry.go` with 16 checks); a repo-wide base-requiring check (the history arm) and a repo-wide base-free check (the cross-document arm) are exactly what `check-immutable` and `check-links` already are — both fit the existing `Check` descriptor and the `Checks()` iteration without opening any new interface. Placement (repo-wide registry vs per-directory `dirs` loop) is decided in D-6 as a use of the existing composition points, not a new seam.
- **DQ-4**: extends — **skipped** — the per-class package `internal/docgate/<class>` and class-rules-as-schema-registries are the settled shape (docgate-v2 `lint-tech-designs`/`internal/docgate/techdesigns`; NFR-1 pins it); one more per-class package extends it. The one-vs-two-package choice is an assembly refinement decided in D-7, opening no interface a consumer sees.
- **DQ-5**: (not a design seam) — **skipped** — this is a scope-boundary ruling, not a mechanism choice; it opens no interface. Resolved in D-8 by the PRD's own NG-4/WR-1 boundary and RW-5 (one decision per document). Logged here for DINV-7 completeness (every `DQ-n` carries a panel-log line whatever the action).

## Decisions

### D-1 — The design's one decision (the direction)

- **Question**: What single, independently-revisable decision does the serving technical design record?
- **Answer**: **How the two Argus ledger-lint arms read a ledger and compose into `docgate gate`** — the history arm's entry-grain append-vs-edit comparison against the merge base, the cross-document arm's corpus scan + citation-into-region resolution under the FR-9 recognition boundary, both as self-registered repo-wide checks in the existing registry, in one per-class package, report-only. This is one decision (RW-5): the two arms' internal mechanics and their shared parse of the ledger classes change together and are superseded together; their composition is a *use* of the settled registry, not a second decision.
- **Reasoning**: The PRD froze the *what* (two arms, DINV-anchored, report-only, self-registered, FR-9 boundary); §5 intro + Q1 + P-1..P-3 explicitly hand the *how* (the diff grain, the anchor-into-region mechanics, the corpus-read assembly) to this design. Bundling both arms in one design honors the coupling rule: they share the ledger parse and the same acceptance spine (the A-7 corpus) and would only ever be revised together; splitting them would manufacture two designs that must co-approve. The third structure arm (D-8) is a *separate*, independently-revisable decision and is out.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-21
- **Status**: live

### D-2 — The history arm's comparison grain (resolves DQ-1)

- **Question**: What grain does the history arm compare at, and against what, to tell a legal append from an illegal edit/removal?
- **Answer**: **Entry-block identity keyed by anchor.** The arm parses both the merge-base revision and the working tree of each in-scope ledger file into a map of `anchor → entry-block bytes` (the `A-n`/`E-n` sequence, each block being the entry from its `### <anchor>` heading to the next delimiter). A previously-merged anchor is: **present-and-byte-identical** → legal (untouched); **present-but-changed** → edit defect (FR-1); **absent** → removal defect (FR-1). An anchor present *only* in the working tree and strictly higher-numbered is a **legal append** (FR-2). The comparison is the set of *merged* anchors against their working-tree counterparts; new higher anchors are invisible to it. (ALT-1 chosen; ALT-2, ALT-3 rejected.)
- **Reasoning**: A ledger append is a same-file `M` in `git diff --name-status`, so `check-immutable`'s file-granular verdict cannot serve FR-1+FR-2 together — it would red every legal append (violating FR-2, the §3.3 legal-append-false-red guardrail, R1). Entry-block-by-anchor is the finest grain that is *structural* (robust to whitespace re-wraps being classified deterministically as changes, which is correct — DINV-1/2 forbid editing a merged entry at all), reuses the ledger classes' existing anchor grammar (`A-n`/`E-n`, argus-artifacts §3.4, dense-from-1), and needs the merge base already available to the base-requiring path (the same `git diff`/`git show` shell family `check-immutable` uses). It is the entry-grain the PRD's §5 intro named as design-stage and R1 flagged as fog.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-22
- **Status**: live

### D-3 — Renumbering and presence are edits; the arm keys on anchor, not position (extends DQ-1)

- **Question**: How are renumbering and re-ordering of merged entries classified, given the arm keys on anchor?
- **Answer**: **A merged anchor that is renumbered, moved, or removed is an edit/removal defect** — because keying on the anchor makes "merged anchor `E-3` no longer present, unchanged, at its merged content" the defect condition regardless of what replaced it. Renumbering a merged `E-3` to `E-3a` (or deleting it and shifting the tail) presents as `E-3` **absent** → removal defect; changing its body presents as `E-3` **changed** → edit defect. FR-1's "body, anchor, numbering, or presence" is thereby covered by a single mechanic: the merged anchor→bytes map must survive verbatim into the working tree.
- **Reasoning**: This is not a new seam — it is D-2's mechanic applied to the three sub-cases FR-1 enumerates, showing one comparison covers all of them. It matches DINV-1 ("no `A-n` is ever edited, renumbered, or removed") and DINV-2 verbatim. Keying on anchor (not line position) is what makes the arm robust to a legal append shifting nothing about the merged prefix.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-22
- **Status**: live

### D-4 — The cross-document arm's corpus enumeration (resolves DQ-2, enumeration sub-seam)

- **Question**: Which files does the cross-document arm scan for `delivery:` references?
- **Answer**: **The tracked corpus via `git ls-files`**, filtered to text/markdown, scanning every tracked document for `delivery:<slug>#<anchor>` occurrences. (ALT-5 chosen; ALT-4 (`check-links`' filesystem walk) and ALT-6 (umbrella-provided shared corpus) rejected for this arm now.)
- **Reasoning**: FR-9 fixes the corpus as "every tracked document in the repo"; `git ls-files` *is* the tracked set, so it matches the requirement exactly and excludes gitignored scratch and build artifacts that a raw filesystem walk would sweep in — an untracked scratch file cannot be a real citation source and scanning it risks a spurious finding on regenerable content (an own-corpus false-red vector, R7). `check-links`' `filepath.WalkDir` over `docs/` (ALT-4) is narrower than "every tracked document" (it would miss a tracked citation outside `docs/`, e.g. in a top-level doc) and wider in the wrong axis (untracked files under `docs/`). The shared-corpus memoization (ALT-6, distributed-check-registration §3.2 Q2, PRD P-2/NFR-2) is a real future optimization but is an *assembly detail* the composition may provide later; it is not a contract and is parked as the design's own `Q-n` (G-1), not decided now — the arm's contract is "the tracked corpus," however that read is memoized.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-23
- **Status**: live

### D-5 — The delivery-ledger region model + anchor resolution (resolves DQ-2, region sub-seam)

- **Question**: How does a resolved in-scope citation get classified into journal `E-n` (legal) vs derived-state region (defect) vs unresolvable (UB-2 defect)?
- **Answer**: **A version-pinned closed parse of the delivery-ledger/v1 shape**, delimiting exactly two regions by their section headings: the **`## Derived state`** block and the **`## Journal`** section (whose entries are `### E-n …`). Resolution of `delivery:<concrete-slug>#<anchor>`: (1) the `<concrete-slug>` must name an existing `docs/problems/<concrete-slug>/delivery.md` — else it is **not an in-scope citation** (FR-9), left untouched; (2) if the anchor names a `## Journal` `E-n` entry → **legal** (FR-4); (3) if the anchor resolves into the `## Derived state` region (a derived-block field) → **DINV-6 defect** (FR-3); (4) if the anchor resolves to **neither** a journal `E-n` nor a derived-state field → **unresolvable-anchor defect** (UB-2), a distinct `Op` from the DINV-6 finding. The parse is pinned to `delivery-ledger/v1` (couples the arm lightly to that template shape — P-3), living as a registry in `internal/docgate/schema` like every other closed-parse registry.
- **Reasoning**: This extends the settled closed-parse-registry pattern (docgate-v1 `schema`, docgate-v2 "class rules as registries in schema") to the delivery-ledger shape — not a new seam, an instance of the existing one, so DQ-2's structurally-distinct axis was the enumeration (D-4), not this. Two regions (Derived state, Journal) is exactly argus-artifacts §3.2's structure; keying resolution on the section heading is deterministic and matches "anchors resolve by grep" (§3.4). The four-way branch makes UB-2 a real predicate (a recognized-but-unresolvable anchor is a false-green hole, per E-17's disposition), not a silent pass. The problem-ledger (`A-n`, `## Authority entries`) is not a citation *target* for this arm — the `delivery:` namespace resolves only into `delivery.md`; `problem:` is a disjoint namespace and out of the cross-document arm's DINV-6 scope (only the delivery ledger has a derived-state block).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-24
- **Status**: live

### D-6 — Composition: both arms are repo-wide registered checks (resolves DQ-3)

- **Question**: Do the arms attach to the repo-wide `Checks()` registry or the per-ledger-directory `dirs` loop?
- **Answer**: **Both arms are repo-wide self-registered checks** in the `Checks()` registry, each contributed from its own `reg_*.go` file via `Register()` at `init()`. The **history arm** declares `NeedsMergeBase: true` (skipped at the base-free desk path, run on pre-push/CI — FR-5, UB-1, parity with `check-immutable`); the **cross-document arm** declares `NeedsMergeBase: false` (runs everywhere — FR-3/FR-4 need no base). Neither uses the per-directory `dirs` loop.
- **Reasoning**: The `dirs` loop (gate.go) is for checks scoped to *one PRD ledger directory at a time* (`lint-structure`, fold, verify-rebase over `docs/prds/<feature>`); both Argus arms are **repo-wide** (the history arm scans all `docs/problems/*/` ledgers against the base; the cross-document arm scans the whole tracked corpus), which is exactly the shape `check-immutable` and `check-links` already take as registered checks. Using the existing descriptor gives FR-6 (self-registered, no shared-line edit — AP-23/AP-27), FR-8 (one umbrella verb, desk/CI parity — AP-11), and FR-5's base-free skip *for free* — the umbrella's `NeedsMergeBase && mergeBase==""` skip already implements it. This is a use of the settled seam, not a new one (DQ-3 skipped).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-25
- **Status**: live

### D-7 — Package placement: one per-class package, two check files, region registry in schema (resolves DQ-4)

- **Question**: One package or two; where do the parse registries live?
- **Answer**: **One per-class package** `internal/docgate/argusledger` holding the shared ledger parse (the entry-block-by-anchor reader D-2 needs and the delivery-ledger region model D-5 needs), with the **two arms' descriptors contributed from two separate `reg_*.go` files** in `internal/docgate/gate` (each a single-file `Register()` contribution, so the two additions never collide — AP-27) that delegate into the shared package. The delivery-ledger/v1 region delimiters + the `A-n`/`E-n` anchor grammar live as a **closed-parse registry in `internal/docgate/schema`**, version-pinned, matching docgate-v2's "class rules as registries in schema."
- **Reasoning**: The two arms share one parse of the same three classes (both read entry blocks; the cross-document arm additionally reads regions) — a shared package prevents two divergent ledger parsers (AP-11, one predicate one place). Two `reg_*.go` descriptor files (not one) keep each arm's *registration* a disjoint single-file addition, so parallel authorship of the two arms — or a later third arm (D-8) — never contends on a shared line (AP-27/AP-23, FR-6/NR-3). This extends the settled `internal/docgate/<class>` + schema-registry shape (NFR-1); the one-vs-two-package choice opens no consumer-visible interface (DQ-4 skipped).
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-25
- **Status**: live

### D-8 — The third structure arm ships as its own later feature (resolves DQ-5)

- **Question**: Does the DINV-3/4/5 structure arm (WR-1) ship with these two, after, or separately?
- **Answer**: **Separately — as its own future feature/design, not in this design.** This design records exactly the two non-structural arms; the structure arm (DINV-3 attribution present, DINV-4 tier-in-ladder / no bare pointer flip, DINV-5 no-grant-of-never-class, plus the templates' checklist appendices) is deferred, carried as the design's own open question with the WR-1 pointer.
- **Reasoning**: RW-5 (one decision per document) is decisive: the structure arm is *independently revisable* — it reads a single file (no merge base, no cross-document scan), enforces a different DINV set, and could be built, superseded, or scoped entirely on its own timeline without touching the two arms' mechanics. Bundling it would be two §2 decisions in one design (the conventions' explicit anti-pattern). The PRD's NG-4 refuses authority-contract content validation "beyond what the two arms touch" and WR-1 parks the structure arm as a *separate* waiting-room item; keeping it out honors that frozen boundary (channel: no contract moves). This is not a *seam* choice (DQ-5 not-a-seam), so no panel; it is a scope ruling that the design surfaces as a `Q-n`.
- **Anchor**: 2026-07-11 · delivery:argus-ledger-lint#E-26
- **Status**: live

## Alternatives

### ALT-1 — Entry-block identity keyed by anchor (history arm grain)

- **Shape**: Parse merge-base and working-tree ledger into `anchor → block-bytes` maps; a merged anchor must survive present-and-identical; changed → edit, absent → removal; a new higher anchor is an invisible legal append. Structural boundaries are the `### <anchor>` headings.
- **Trade-offs**: Wins — robust to legal appends (they add a new key, touching no merged key); one mechanic covers FR-1's body/anchor/numbering/presence quartet; reuses the classes' own dense anchor grammar; deterministic (byte-compare per block). Costs — needs a real (if minimal) parse of the ledger's entry structure (a closed parser pinned to the template shape), so it couples to `delivery-ledger/v1` / `problem-ledger/v1` block shape; a malformed ledger needs a named parse defect (UB-3) rather than a crash.
- **Principles**: AP-11 — honored (one comparison predicate over merged-vs-working, shared by the classes); AP-10 — honored (the append-only invariant becomes code, not a review checklist); AP-6 — honored (closed parse of a pinned shape, additive).
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11 — the registry holds no entry bearing on ledger diffing; I1/I2 concern stacked-PR/base-branch mis-merges, unrelated to entry-grain comparison).
- **Verdict**: chosen — the finest *structural* grain that satisfies FR-1 and FR-2 together, which the file-granular alternative cannot.
- **From**: DQ-1 · panel

### ALT-2 — Reuse check-immutable's file-granular `git diff --name-status` verdict

- **Shape**: Add the Argus ledger paths (`docs/problems/*/`) to `check-immutable`'s governed set (or a clone of it), flagging any merged ledger file with status `M`/`D`/`R` vs the merge base.
- **Trade-offs**: Wins — zero new parse; the exact shell `check-immutable` already runs. Costs — **fatal**: a legal ledger append is a same-file `M`, so this reds every legal append (violates FR-2, trips the §3.3 legal-append-false-red guardrail and R1); and widening `check-immutable`'s set violates NG-3/NR-2 and argus-artifacts §3.8's deliberate disjointness (the classes must stay outside `check-immutable`).
- **Principles**: AP-17 — strained (it fails *closed* on additive-and-legal change, exactly inverting the additive-fail-open intent); AP-11 — misapplied (it conflates two predicates — "is a merged design/PRD entry immutable" and "is a merged ledger entry immutable-but-appendable" — under one file-granular verdict that cannot express the second).
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11 — none bear on this).
- **Verdict**: rejected — cannot tell a legal append from an edit at file grain, and widens `check-immutable` against a frozen boundary.
- **From**: DQ-1 · panel

### ALT-3 — Line-level text diff of the ledger file (hunk classification)

- **Shape**: Run a line/hunk diff of the ledger file vs the merge base; classify a hunk as "append" if it only adds lines *after* the last merged entry, else as an "edit."
- **Trade-offs**: Wins — no structural parse of entries; operates on raw diff hunks. Costs — brittle: "after the last merged entry" is a positional heuristic that a reformatting, a moved derived-state block (which *does* legally change — DINV-6, it regenerates on every append), or an interleaved edit defeats; the delivery ledger's `## Derived state` block legally changes on every append, so a line-diff arm must special-case it or false-red every append (the derived block is *supposed* to change), re-introducing the FR-2 failure by another door. Entangles the append check with the derived-block-regeneration reality.
- **Principles**: AP-10 — strained (a positional heuristic is the fragile prompt-like rule codification is meant to avoid); AP-6 — strained (operates on unstructured hunks, not the classes' contractual entry shape).
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11).
- **Verdict**: rejected — the legally-mutating derived-state block makes positional line-diffing false-red legal appends; ALT-1's anchor-keyed structural compare sidesteps this by ignoring the derived region entirely and comparing only merged `A-n`/`E-n` blocks.
- **From**: DQ-1 · panel

### ALT-4 — Corpus via `check-links`' filesystem walk (`filepath.WalkDir docs/` + AGENTS.md + templates)

- **Shape**: Reuse the enumeration `check-links` already performs — walk `docs/`, add `AGENTS.md` and the templates dir — as the cross-document arm's corpus.
- **Trade-offs**: Wins — an existing, proven enumeration in the same package; no new git shell. Costs — mismatched to FR-9's "every tracked document": it **misses** a tracked citation outside the walked roots (a future top-level doc, a tracked file in a dir not walked) and **includes** untracked scratch under `docs/` (including any untracked WIP), risking a spurious finding on regenerable/untracked content (an own-corpus false-red vector, R7).
- **Principles**: AP-13 — strained (scanning untracked scratch verifies against non-artifact bytes); AP-11 — neutral (a different corpus predicate than "the tracked set").
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11).
- **Verdict**: rejected — the corpus axis is wrong on both edges vs FR-9; `git ls-files` matches "every tracked document" exactly. (Its walk remains the right precedent for `check-links`' own narrower job.)
- **From**: DQ-2 · panel

### ALT-5 — Corpus via `git ls-files` (the tracked set)

- **Shape**: Enumerate the corpus as `git ls-files` (tracked, text-filtered), scanning each for `delivery:<slug>#<anchor>`; recognize an in-scope citation by resolving `<slug>` to an existing `docs/problems/<slug>/delivery.md` (FR-9).
- **Trade-offs**: Wins — matches FR-9's "every tracked document in the repo" precisely; excludes gitignored scratch by construction; a single git invocation, in the same shell family as the history arm's base read. Costs — a git subprocess (already the norm — `check-immutable` shells git); a check now depends on being inside a git work tree (true for every gate path — desk pre-push and CI both run in the clone).
- **Principles**: AP-13 — honored (scans only tracked artifacts, never scratch); AP-11 — honored (one corpus predicate: the tracked set); AP-6 — honored (in-scope-hood decided by target resolvability, a closed rule).
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11 — none bear on corpus enumeration).
- **Verdict**: chosen — the only enumeration whose set *is* FR-9's stated corpus.
- **From**: DQ-2 · panel

### ALT-6 — Umbrella-provided shared corpus read (memoized across repo-wide checks)

- **Shape**: The gate composition reads the tracked corpus once and hands it to every repo-wide check (the cross-document arm, `check-links`, future scanners) via the `Ctx`, so no check re-reads the tree.
- **Trade-offs**: Wins — bounds NFR-2's repo-wide cost; the memoization distributed-check-registration §3.2 Q2 anticipates. Costs — it is an *assembly/optimization* concern, not a contract; deciding it now over-commits the composition shape (the `Ctx` would grow a corpus field affecting every check) ahead of a demonstrated cost problem; it is orthogonal to *what* the arm verifies.
- **Principles**: AP-19 — honored in spirit (steady-state cost bound); AP-11 — neutral (a shared read, not a shared predicate).
- **Incidents**: none checked (docs/observability/incidents.md, 2026-07-11).
- **Verdict**: hybridized into the design's Q-n — the arm's *contract* is "the tracked corpus" (ALT-5); *how* that read is memoized/shared is deferred to the design's own open question (G-1), decidable at implementation against the shared-read assembly without changing the arm's behavior. Not decided now.
- **From**: DQ-2 · panel

## Seam map

- **S-1**: the history-arm entry-block comparison (merge-base vs working tree, keyed by `A-n`/`E-n` anchor) — **opens** — behind it: a closed structural parse of the ledger classes' entry sequence + a git base read (the `check-immutable` shell family) — feeds: the design's Scope ("the history arm's append-vs-edit grain") and `Serves: prd:argus-ledger-lint/FR-1 · /FR-2 · /FR-5`.
- **S-2**: the cross-document-arm corpus scan + citation-into-region resolution — **opens** — behind it: `git ls-files` corpus enumeration (S-2a) + a version-pinned delivery-ledger region parse (Derived state vs Journal) with FR-9 target-resolvability recognition (S-2b) — feeds: Scope ("the cross-document arm's scan + the FR-9 recognition boundary") and `Serves: prd:argus-ledger-lint/FR-3 · /FR-4 · /FR-9 · /UB-2`.
- **S-3**: the two arms' registration into `docgate gate` — **extends** — behind it: the settled `Check` descriptor + `Checks()` registry (`registry.go`), two disjoint `reg_*.go` single-file contributions, the `NeedsMergeBase` base-free-skip — feeds: Scope ("composed under the existing `docgate gate` umbrella") and `Serves: prd:argus-ledger-lint/FR-6 · /FR-8 · /NR-1 · /NR-3`.
- **S-4**: the shared per-class parse package + schema registry — **extends** — behind it: `internal/docgate/argusledger` (shared ledger parse) + a `delivery-ledger/v1` region/anchor registry in `internal/docgate/schema` — feeds: Scope ("per-class package under `internal/docgate/`") and `Serves: prd:argus-ledger-lint/NFR-1`.

## Facts

- **F-1**: The `Check` descriptor is live in code as `internal/docgate/gate/registry.go`: `type Check struct { Verb string; Help string; NeedsMergeBase bool; Run func(Ctx) Defects }`, with `Register(c)` called from each check file's `init()` and `Checks()` returning the registry sorted by `Verb`. `Ctx{ Root, MergeBase string; Cfg config.Config }`. Sixteen checks self-register today; adding a check is one new file, zero shared-line edits. — source: `internal/docgate/gate/registry.go` (verified in code)
- **F-2**: `check-immutable` (`internal/docgate/gate/reg_check_immutable.go`, `NeedsMergeBase: true`) shells `git diff --name-status <mergeBase> -- <cfg.PRDs> <cfg.TechDesigns>`, classifies a governed path with status `M`/`D`/`R` (skipping `A`) as a defect, and its governed set is **PRD ledger entries** (`^\d{3}-[a-z0-9-]+\.md$` under `cfg.PRDs`) + **tech-design `.md`** (minus `index`). The Argus ledger paths (`docs/problems/*/`) are **not** in its diff scope. — source: `internal/docgate/gate/reg_check_immutable.go` (verified in code)
- **F-3**: A ledger append is a same-file `M` in `git diff --name-status`; the delivery ledger's `## Derived state` block *legally changes on every append* (DINV-6, regenerated). So neither a file-granular verdict (ALT-2) nor a positional line-diff (ALT-3) can tell a legal append from an illegal edit without special-casing the derived block. — source: argus-artifacts §3.2/§3.5 DINV-6; `reg_check_immutable.go` mechanics (verified)
- **F-4**: The repo-wide base-free corpus-scan precedent is `check-links` (`internal/docgate/gate/reg_check_links.go`, `NeedsMergeBase: false`), which enumerates via `filepath.WalkDir` over `docs/` + `AGENTS.md` + the templates dir (a **filesystem walk**, sorted for determinism), not `git ls-files`. A separately-named `doc-link-integrity` check does **not** exist; `check-links` is the actual relative-link resolver. — source: `internal/docgate/gate/reg_check_links.go` (verified in code; corrects the PRD's F-7 `doc-link-integrity` reference to the real check name)
- **F-5**: The `gate` umbrella (`internal/docgate/gate/gate.go`) iterates `Checks()`, skips `chk.NeedsMergeBase && mergeBase==""`, then runs each; a **separate** per-ledger-directory loop over `dirs` runs the directory-scoped checks (`lint-structure`, fold, verify-rebase). Repo-wide checks and per-directory checks are two distinct composition points. — source: `internal/docgate/gate/gate.go` (verified in code)
- **F-6**: The per-class package precedent is `lint-tech-designs` (`internal/docgate/gate/reg_lint_tech_designs.go`, `NeedsMergeBase: false`) delegating into `internal/docgate/techdesigns`, which `LoadDesigns` reads via `os.ReadDir` over `docs/tech-design/`. Class rules live as registries in `internal/docgate/schema`. — source: `internal/docgate/gate/reg_lint_tech_designs.go`, `internal/docgate/techdesigns/techdesigns.go` (verified in code)
- **F-7**: The defect type is `schema.Defect{ Verb, File, Op, Reason, Remedy string }` (all JSON-tagged), rendered human on stderr and one JSON line per defect on `--defects-json`. The `Op` field is the region/category discriminator — the natural place to distinguish a "cited-derived-block" (DINV-6) finding from an "unresolvable-anchor" (UB-2) finding at the same file. — source: `internal/docgate/schema/schema.go` (verified in code)
- **F-8**: Check fixtures follow the ledger-directory pattern under `internal/docgate/*/testdata/<case>/` (ledger files + `expect/` goldens + a red-case defect assertion), with a `MANIFEST.md` mapping fit-criteria → cases and `writeLedger()` temp-dir helpers in tests. — source: `internal/docgate/fold/testdata/` (verified in code)
- **F-9**: The Argus ledger classes are on disk at `docs/problems/<slug>/{problem.md, delivery.md}`; `delivery.md` carries a `## Derived state` block and a `## Journal` section of `### E-n` entries; `problem.md` carries `## Idea` and `## Authority entries` (`### A-n`). The `delivery:` grammar resolves into `delivery.md` only; `problem:` is a disjoint namespace, and only the delivery ledger has a derived-state block. — source: `docs/problems/argus-ledger-lint/{problem.md, delivery.md}`; argus-artifacts §3.1/§3.2/§3.4 (verified)
- **F-10**: The tech-design index (`docs/tech-design/{index.md, index.json}`) is generated by `docgate gen-index` (→ `techdesigns.GenIndex`), sole-writer, byte-verified by `verify-index`; a new design under `docs/tech-design/` must be added and then the index regenerated. `argus-ledger-lint` is currently **absent** from the index (the design is not yet authored). — source: `docs/tech-design/index.{md,json}`, `internal/docgate/gate/gate_v2.go` (verified)

## Fog

- **G-1** (non-blocking — blocks: the design's own `Q-n` / implementation): the memoization/sharing of the cross-document arm's repo-wide corpus read across repo-wide checks (distributed-check-registration §3.2 Q2, PRD P-2/NFR-2). The arm's *contract* is "the tracked corpus" (D-4); how that read is shared is an assembly detail decidable at implementation without changing behavior — rides the authored design as a `Q-n`, not a blocker.
- **G-2** (non-blocking — blocks: the design's own `Q-n` / implementation): the exact text-normalization used for entry-block byte comparison (D-2) — whether the arm reuses docgate's existing fold normalization (CRLF→LF, trailing-space strip, single final newline; docgate-v1 §3.6) before comparing blocks, or compares raw bytes. Reusing the normalization avoids flagging a pure line-ending change as an edit; the choice mirrors `check-immutable`'s own normalization posture and does not gate the direction — a `Q-n` on the authored design.

## Out of scope

- **O-1**: The third **structure arm** (DINV-3 attribution present / DINV-4 tier-in-ladder + no bare pointer flip / DINV-5 no-grant-of-never-class, plus the templates' checklist appendices) — **not in this design**. — reason: RW-5 one-decision-per-document — it is independently revisable (single-file, no base, no cross-document scan, a different DINV set) and would be a second §2; the PRD parks it as the separate WR-1 waiting-room item and NG-4 refuses authority-contract content validation beyond the two arms. (from D-8)
- **O-2**: No widening of `check-immutable`'s governed set — the Argus ledger classes stay disjoint from it; the history arm is a separate check. — reason: argus-artifacts §3.8 disjointness + PRD NG-3/NR-2; confirmed in code (F-2 shows the classes absent from its diff scope). (from D-2, D-6)
- **O-3**: No fold or projection for the ledger classes, and no closed-parse *runtime* reader for engine consumption. — reason: PRD NG-1 (they take no fold) and NG-2 (the runtime reader is Q-1's engine-era other half, the anti-segment). (from D-1)
- **O-4**: The `problem:` citation namespace and the problem ledger's `A-n` / `## Authority entries` are not a DINV-6 *target* for the cross-document arm — only the delivery ledger has a derived-state block. — reason: the `delivery:` namespace resolves only into `delivery.md`; DINV-6 is a delivery-ledger property (F-9). (from D-5)

## Candidate glossary terms

- **T-1**: entry-block comparison — the history arm's mechanic: parse a ledger into `anchor → block-bytes` at merge base and working tree, and require every *merged* anchor to survive present-and-identical (changed → edit, absent → removal), while a new higher-numbered anchor is an invisible legal append. · avoid: "diff check" (too broad; this is anchor-keyed structural, not line-level), "immutability check" (reserved for `check-immutable`'s disjoint file-granular set).
- **T-2**: in-scope citation (Argus grammar) — a `delivery:<concrete-slug>#<anchor>` reference the cross-document arm classifies, recognized only when `<concrete-slug>` names an existing `docs/problems/<concrete-slug>/delivery.md` (FR-9) — never a literal-`<slug>` metavariable illustration or an absent-ledger mention. · avoid: "link" (`check-links` owns relative-link resolution; this is citation-target legality). *(Note: the two arm names — **history arm**, **cross-document arm** — are already landed in docs/wiki/glossary.md via the PRD's introducing PR; these two are the finer design-coined terms this session adds.)*

## Parked product gaps

*None. The merged basis (FR-1..FR-9, NR-1..NR-3, UB-1..UB-3, Q1/Q2, P-1..P-3, WR-1) is admissible and sufficient for the design: every design question resolves against a frozen requirement (the grain/mechanics FR-1/FR-2/FR-3/FR-4/FR-9/UB-2 name and §5/Q1/P-1..P-3 explicitly hand to design) plus the settled precedents — no requirement was found missing, contradictory, or in need of an amendment. No `P-n` bounce.*

- none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-11 |
| **Authority** | Argus (AI Engineer, authority-contract/v1 tier-2 — signed, not requested; `sign-gate-verdict` granted at tier-2; delivery:argus-ledger-lint#E-27) |
| **Covering design** | — (not a `covered` verdict — no live design decides these arms; see Overlaps) |
| **Coverage check** | pass — core four covered, zero blocking fog (walk below) |
| **Amend route** | — (not a `needs-more-product` verdict — the basis is admissible; no `P-n` gap) |

**Coverage walk (Appendix B), for `ready-for-design`:**

| tech-design-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2 Decision — the direction | ≥ 1 live `D-n` choosing the shape, its `DQ-n` panel-logged | ✅ D-1 (the one decision), grounded in D-2 (DQ-1 fired), D-4/D-5 (DQ-2 fired), D-6 (DQ-3 logged), D-7 (DQ-4 logged) — every `DQ-n` carries a panel-log line |
| §4 Alternatives considered | the `ALT-n` set — ≥ 2 entries where the panel fired, every entry verdicted | ✅ ALT-1/2/3 (DQ-1 fired, 3 shapes) + ALT-4/5/6 (DQ-2 fired, 3 shapes); each carries Principles + Incidents + a Verdict |
| header `Serves` + `Scope` | the `S-n` seam map + the `Basis` anchors, enumerated | ✅ S-1..S-4 map to the FR/NR/UB/NFR anchors; Basis = prd:argus-ledger-lint (FR-1..FR-9 enumerated per seam) |
| §3.2 Boundaries and non-goals | the `O-n` set | ✅ O-1..O-4 |

**Gate condition:** zero `blocking` fog — satisfied (G-1, G-2 both non-blocking; no `P-n`). All four coverage rows checked. Verdict `ready-for-design`; signed by Argus at tier-2.
