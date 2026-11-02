# docgate — the document gate and fold — PRD *(effective view)*

---

**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **018**. Content fingerprint: `sha256:9d7934e3c155499adbf6298d5eee5d4baab5526efcb78fcce9d459206523bb19`. Tool version (recorded, never hashed): `docgate/v1`.

---


| Field | Value |
|---|---|
| **In one line** | The deterministic tool that folds PRD ledgers into projections and gates document validity in CI and at pre-push. |
| **Relevant-when** | planning, building, or reviewing the fold, the gate verbs, or ledger validation; triaging defects in projection generation or gate verdicts; changing anything that parses PRDs, amendments, or templates |
| **Status** | Problem review ✓ · Solution review ✓ (2026-07-06) — launch review pending, post-implementation |
| **Tier** | T2 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | operator (product) · factory agents (implementation) |
| **Informed** | future adapter and skill authors |
| **Date** | 2026-07-06 created · 2026-07-06 last updated |
| **Appetite** | small: ≤2 human review rounds per document; modest worker spend; zero rollout/support burden (repo-internal tool) |
| **Target release** | precondition for the first real per-feature PRD beyond this one — nothing else lands first |
| **Tracking** | introducing PR (filed on approval) |
| **Artifacts** | docs/tech-design/prd-amendment-model.md · docs/tech-design/tech-design-conventions.md · golden fixture corpus (arrives with implementation) |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — Approved PRDs must evolve without trusting any editor's discipline: deterministic generation can still repeat a stale markdown projection and stale source map after an untyped whole-section row change, so artifact agreement needs its own merge-base guard.
- **For whom** — agents authoring amendments and rebase checkpoints, human reviewers of ledger PRs, and CI on every PR touching `docs/prds/`.
- **Why now** — PR #1090 reproduced a same-ID row mutation hidden inside a section supersede; the corpus audit also found historical projection/source-map debt that makes naïve hard adoption false-red.
- **Direction** — one static Go binary: a deterministic fold plus self-registering gate verbs, with projection congruence checking each new ledger boundary against an exact grandfathered baseline; the identical predicate runs in required CI and pre-push.
- **Appetite** — as header; stdlib-only, fixture-first, and no corpus rewrite as the price of adoption.
- **Success** — M1/M2 remain green and M3 records zero newly introduced mismatch accepted by the gate.
- **Guardrail** — no §0 routing row becomes a contract unit; no baseline debt blocks an unchanged amendment; no named repair dead-ends under the next gate.
- **Top risks** — parser inference reading non-binding tables; an ambiguous baseline comparator exchanging defects; a rebase repairing machine lineage while leaving human lineage stale; a partial corpus inventory presenting itself as complete.

## 2. Problem space

### 2.1 Problem statement

An approved PRD is a contract consumed by humans, reviewers, and (soon) the Contract Compiler. It must change as building teaches — but every mechanism that lets LLMs edit documents in place decays the contract. The amendment model solves this structurally: an append-only ledger of typed amendments, folded deterministically into a generated projection. Without the tool, the model is prose: projections would be hand-maintained (the drift the model exists to kill), immutability is prompt-side only, and malformed amendments are discovered by human reviewers late instead of by machines instantly. The cost of doing nothing is that the document system we just built operates on the honor system.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | factory agents authoring amendments in this repo | journeyman | master | a valid amendment on the first try; instant local verdicts | no mechanical feedback until human review | amendment-model review threads |
| P2 | human reviewers of amendment PRs | master | master | review the *effect*, trust the projection unconditionally | must mentally fold ledgers | prd-amendment-model §5 (`diff` rationale) |
| P3 | CI / branch protection | — | — | one deterministic verdict per PR | nothing enforces INV-41's CI half | INV-41; AGENTS.md rule 8 ("prompt-side half") |
| **Anti-segment** | authors of the placeholder corpus (flat `docs/prds/*.md`) | — | — | — | — | operator ruling: exempt, never retrofitted |

### 2.3 Jobs and desired outcomes

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I have authored an amendment | regenerate and verify the projection locally | open a PR that passes on the first try | hand-run the documented fold discipline | prd-amendment-template §0 (transitional note) |
| J2 | I review an amendment PR | see before/after effect, affected IDs, and computed blast radius | judge the change without replaying the ledger | mental fold | prd-amendment-model §5 |
| J3 | a PR touches a ledger | have CI refuse stale projections, edited history, and malformed amendments | trust `effective.md` without reading `docs/prds/<feature>/` history | none | prd-amendment-model §2 |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| Hand-maintained current-state docs drift ("the README that lied for a week") | VNEXT-6; predecessor incident history | High |
| Byte-deterministic document tooling is achievable in this repo | the Go plan renderer is golden-tested and byte-deterministic across runs | High |
| Desk-time + CI running the identical predicate prevents divergence | AP-11 / INV-30; GATE-2 lineage (local CI mirror) | High |
| Agents will attempt to "helpfully" repair generated artifacts | prd-amendment-model §5 actor table; observed LLM behavior during this repo's own tooling work | High |

### 2.5 The journey today

Skipped: the pre-tool journey is fully described in §2.1 — one manual path (hand-fold, attach a diff, hope) with no variation worth mapping.

### 2.6 Opportunity and why now

- **Size** — every future PRD and amendment flows through this gate; it is the trust anchor for stage-6 compilation and the first executable piece of the document system.
- **Alternatives users have** — keep prose discipline (drift, proven by the predecessor); LLM-verified projections (unverifiable — rejected in the amendment model's alternatives).
- **Why us / strategic fit** — RW-4's spirit: substrate first; the factory's first Go code enforces its own document contract.
- **Why now** — the model and templates are merged and exactly one ledger exists (this document): the guarantee can be born-in rather than migrated-to.
- **Cost of doing nothing** — the amendment model remains ceremony; drift returns wearing a process costume.

**Decision:** Problem gate — signed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | 2026-07-06 | approved |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a deterministic fold + byte-verifying gate | projections trusted without reading ledgers | M1 |
| G2 | instant local verdicts (pre-push, identical binary) | amendments passing CI on the first try | M2 |
| G3 *(018)* | a merge-base congruence gate over every newly added amendment and rebase checkpoint | no new projection/source-map disagreement can merge while historical debt remains non-blocking and repairable | M3 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | fixture folds byte-identical across runs/platforms | n/a (no tool) | 100% at "golden suite green in CI" | measurement plan: golden-suite results in CI |
| M2 | G2 | amendment PRs green on first docgate run | n/a | ≥80% at "10 amendments through the required gate" | measurement plan: PR check history via `gh` |
| M3 *(018)* | G3 | newly introduced projection/source-map mismatches accepted by the required gate | no gate; the complete independently reconstructed debt set at gate activation, pinned to the activation merge-base revision and ledger fingerprint | zero across the full defect matrix and the first 10 gated ledger entries; the independent corpus audit remains byte-equal to the reviewed activation golden after only exact-key deletions made by explicit debt-repair proposals | required-check history + independent corpus-audit test |

*(§5.7 tracking plan skipped with reason — no product analytics in a hermetic tool; both metrics are measured from CI and PR records, plans stated above.)*

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| hand-edited projection merged | any single occurrence | incident entry + gate-wiring fix before the next amendment merges |
| false-red rate on valid fixtures | >0 in the golden suite | fold or gate bug — fix before enforcement widens |
| §0 row classified as a contract unit | any | block release; repair parser scope and add the real document as a fixture |
| incomplete baseline-debt golden | any independently discovered omitted record | block release or unwire the gate; the implementation owner regenerates it only in a dedicated reviewed gate-fix proposal against the immutable activation fingerprint |
| named repair fails under the next enforcement layer | any | block release; make the repair executable before emitting it |

**Ship rule (amended):** the existing fold golden suite is 100% green, the complete FR-32..FR-43/UB-4..UB-6/NR-4 matrix is green, the independently reconstructed corpus inventory byte-equals its reviewed activation golden, and every FR-41-addressable-prefix repair plus checkpoint Q-state transition runs end to end → wire `check-projection-congruence` as a hard required check from first release. Any guardrail breach → do not wire, or unwire and repair before the next ledger entry merges. Historical keys in the reviewed golden are non-blocking only while identical; there is no advisory bypass for a new key.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-2 | Semantic conflict detection (an `answer` contradicting a live row, duplicate-in-different-words) | the model's tier boundary: semantic conflicts belong to review and the Contract Compiler (VNEXT-1) |
| NG-3 | Any LLM invocation inside the tool | AP-10; the gate job is hermetic — no network, no model |
| NG-4 | Design-doc verbs and the generated design index | scoped to v2 by the tech-design conventions; v1 ships the approved PRD scope only |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | docgate v2: design-header lint, mandatory-subsection + security-pair checks, `Supersedes`/`Serves` resolution, index generation | after v1 is proven; scope enumerated in tech-design-conventions §3.4 |
| WR-2 | `docgate diff` output posted as a PR comment | when GitHub-adapter work begins |
| WR-3 | absorption into the engine as the Contract Compiler's front door | at that component's design (prd-amendment-model §7 Q4) |

## 4. Solution space

### 4.1 Approach and alternatives considered

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | one Go binary: deterministic fold + self-registering gate verbs; congruence compares each new ledger boundary by canonical mismatch-multiset subtraction and checks rebases as repair boundaries | — |
| B | rewrite or rebase every indebted ledger before enforcement | serializes unrelated work across many contract PRs and spends operator review to repair historical tooling debt before stopping the next regression |
| C | advisory-first gate | observes the next regression without stopping it, contrary to G3; the exact activation golden makes hard adoption safe |
| D | retain base-row kind inference without an ancestry guard | reads §0 routing tables as contract; reproduced false positive on impl-review FR-24 |
| E | retain the design's narrow `supersede` target list and repair excluded live prefixes with `retire` plus fresh `add` | erases `supersedes`/`superseded_by`, conflicts with merged ABUSE/EVT precedent, and can send a cited row to a structurally forbidden retirement |
| F | add the lexical `F` prefix to the addressable table-schema registry in this amendment | newly governs heterogeneous §4.3 flow tables, creates source-map debt, and expands the product beyond projection-congruence repair; a future amendment must define that contract deliberately |
| G | scan entries before the latest checkpoint solely to recover Q answer operations | makes compacted history a hidden live input, contradicts the checkpoint as replay base, and gives current state an identity outside the replayed-ledger fingerprint |
| Do nothing | rely on review to notice untyped same-ID row changes | PR #1090 proves review feedback can introduce exactly this regression |

### 4.2 Scope perimeter

- **In (plan of record)** — every live docgate requirement; specifically for this amendment: `check-projection-congruence`, complete §0-subtree exclusion with existing merged-base row-kind inference and exact payload schemas, per-entry attribution, exact-revision reads, exact-value baseline multiset subtraction, checkpoint congruence, addressable-table-registry and checkpoint-carried Q-state boundaries, lineage-preserving repairs under the closed operation grammar, unchanged-row preservation, complete diagnostics, the reviewed activation-debt golden, required-context + pre-push wiring, and the live Wiring rollup.
- **Next governed result** — a superseding technical design for the fold/parser/checkpoint and section-supersede semantics serving FR-32..FR-43, UB-4..UB-6, and NR-4; it must explicitly supersede the amendment model's narrow `supersede` target list so every live FR-41-addressable ID may be superseded with a fresh same-prefix row except an FR-42-unanswered Q (use `answer`) and NG/WR (use `rescope`), preserve `retire` as removal rather than replacement with the FR-39 citation domain, and define the typed replay-base encoding, restoration, source-map projection, and verification of FR-43 `question_answered`. Implementation remains blocked until it is approved.
- **Future (believed next, not committed)** — WR-1 (additional design-class scope), WR-2, WR-3.
- **Out** — scanning pre-checkpoint history to reconstruct current Q state; registration or source-map governance of `F` flow rows; semantic duplicate detection; automatic mutation of frozen historical entries; an advisory bypass; a new operation verb; hand-authored exemptions or a corpus-wide mandatory repair campaign.

### 4.3 Experience: flows and states

**Flow F1 — the ordinary author loop (serves J1/J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | writes one `NNN-<slug>.md` amendment from the template | — | — | prd-amendment-template |
| 2 | previews, reviews, approves, and folds it | regenerates both projection artifacts; ordinary validation names malformed ops | ok · named defect | prd-amendment-model §4–5 |
| 3 | pushes the exact revision | pre-push and CI evaluate the same added-entry boundary; unchanged merge-base debt subtracts away, each new canonical mismatch key is red against its actual artifact | green · congruence defect · read/replay refusal | FR-32..FR-34, UB-4..UB-5 |
| 4 | reviews the folded effect and gate evidence | merges only with every required check green | accepted · returned for correction | DOD-7 |

**Flow F2 — changing a unit-operation-addressable lineage-less unit (serves J1)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | proposes one semantically identical rebase checkpoint | `verify-rebase` proves no contract change; congruence proves human and machine lineage re-anchor together; every binding row gains checkpoint-origin lineage, and each Q gains exact typed `question_answered` replay-base state | green checkpoint · semantic-change red · lineage-compaction red · Q-state mismatch red | FR-35..FR-37, FR-43 |
| 2 | after that checkpoint merges, proposes the prefix-authorized unit operation in a later amendment | the operation resolves the now-live target; Q routing reads restored checkpoint state plus post-base answer operations, and ordinary congruence checks the resulting boundary | green typed change · Q answered/unanswered · ordinary op defect | FR-32, FR-36, FR-39, FR-42..FR-43 |

**Flow F3 — selecting or refusing a section repair (serves J1)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | replaces the complete owning section for a G/J/M/ABUSE/R row | the fold compares each replacement row with its immediately-before canonical content and lineage: FR-40 preserves every unchanged row exactly; a changed row with no prior live state or source-map lineage alone is refreshed and receives matching visible/machine origin; a changed row with prior live state reds under FR-32 and names `supersede <ID>` with a fresh same-prefix row | green lineage-less section change · unchanged restatement · lineage-bearing mutation red · malformed section | FR-32, FR-34, FR-37, FR-39..FR-41, UB-6 |

*(five-UI-states bullets skipped with reason — a CLI's states are exit codes and named defects; accessibility/localization skipped — terminal tool, English-only repo convention.)*

### 4.4 Scenarios

An agent applies review feedback by replacing a whole section and silently changes lineage-bearing ABUSE-1 under the same ID: the existing live state prevents refresh, the exact pushed-revision gate reds the actual inconsistent artifact and names the boundary, and `supersede ABUSE-1` with fresh ABUSE-9 is green while recording both ABUSE-1 `superseded_by=ABUSE-9` and ABUSE-9 `supersedes=ABUSE-1`. — An ordinary amendment restates a section containing a historical lineage-less R6 byte-for-byte: the fold preserves every projected byte and absent lineage record under FR-40, its exact baseline key subtracts away, and the amendment stays green. — The author must change lineage-less R6: `supersede R6` is rejected with the prefix-correct whole-section remedy, and only R6 receives new matching visible/machine lineage. — A Q row whose Status says `resolved (001)` but has neither a true replay-base value nor a successful post-base answer op is unanswered under FR-42 and routes to `answer`; after that answer, `002-rebase.md` materializes `question_answered: true`, a checkpoint-only replay restores answered state, and another answer reds with the fresh-Q supersede remedy. — The author must change lineage-less NG-5: the diagnostic names a semantic-no-op checkpoint followed by `rescope NG-5`; both proposals pass their own gates. — Workbench-ui and planner-driven-model-routing F1..F5 rows remain non-addressable under FR-41 and create neither source-map records nor repair obligations. — A prd-template/v2 §0 routing table names FR-24 and FR-19/FR-20/FR-26: it contributes zero units and cannot shadow the binding FR-24 row.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked as `fold`, docgate shall emit `effective.md` and `effective.source.json` from the ledger deterministically, with the content fingerprint over normalized replayed-ledger bytes and the tool version recorded separately. | the projection is derived, never authored; identity tracks contract, not tooling | Must | med | byte-identical output across 100 runs on 2 platforms over the fixture corpus; fingerprint stable across a tool-version bump with unchanged ledger | Test | G1, J1 |
| FR-2 | If an operation is invalid (dangling target, double-supersede, ID reuse, schema-violating payload), then docgate shall abort with a defect naming file, op, reason, and remedy — as human text on stderr and as one machine-readable JSON line per defect on a `--defects-json` stream. | fail closed; authors fix fast; CI annotates precisely | Must | med | 100% of fixture defect cases produce the named quadruple in both forms; JSON parses against the defect-schema golden; zero best-effort output | Test | J1, J3, G2 |
| FR-3 | When invoked as `verify-generated`, docgate shall recompute the fold hermetically and byte-compare every generated artifact against the committed set, failing on any difference or fingerprint mismatch. | unforgeable projection | Must | high | hand-edited fixture projection → red naming the artifact; regenerated → green | Test | G1, J3 |
| FR-4 | When invoked as `check-immutable` with a merge base, docgate shall fail on any modify, delete, or rename of an existing ledger file. | INV-41's CI half; merge is the freezing event | Must | high | fixture M/D/R each red; add-only green; rename detected as delete+add | Test | J3 |
| FR-5 | When invoked as `validate-amendments`, docgate shall enforce the closed op vocabulary, `Status: Approved` at merge-target, typed `Learned-from` evidence classes, and a computed blast radius that the declared one may exceed but never understate. | the envelope contract; declarations are checked, not trusted | Must | high | one fixture per violation class, each red with its named defect | Test | J1, J3 |
| FR-6 | When invoked as `diff`, docgate shall render the projection before/after, affected IDs, computed blast radius, and downstream invalidation levels. | reviewers judge the effect, not the mechanics | Must | low | fixture amendment yields all four outputs with expected IDs and levels | Test | J2 |
| FR-7 | When invoked as `verify-rebase`, docgate shall fail any rebase checkpoint that is not semantically identical to the projection immediately preceding it. | rebases launder nothing | Must | high | tampered fixture rebase red; faithful one green | Test | G1 |
| FR-8 | When invoked as `lint-structure`, docgate shall verify every ledger directory has exactly one `000-base.md`, dense monotonic numbering, and headers parsing against their declared template versions. | malformed layout never lands | Must | med | gap, duplicate, missing-base, and unknown-header fixtures each red with named files | Test | J3 |
| FR-9 *(001)* | When a commit is created in a clone with the hook path activated, the pre-commit hook shall run the merge-base-free gate verbs (lint-structure, validate-amendments, verify-generated, verify-rebase) and reject the commit on any defect. | generation must hold with no remote: the projection is committed fresh or not at all | Should | low | hook fixture: a stale projection rejects the commit naming the regenerate remedy; a fresh one passes; absent Go toolchain skips with the two-line notice | Test | G1, J1 |
| FR-20 *(007 ← FR-16)* | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only), reference resolution — with `corpus:` references resolving against requirement IDs actually present in the flat placeholder-corpus files — mandatory-by-name Observability and Security & privacy subsections with the header pair, slug rules, and a Status row recording `Approved`; shall fail any design whose `Serves` cites a `prd:` ledger without at least one requirement-row anchor (FR-n, UB-n, or NFR-n); and, when a merge base is given, shall fail any design added since the merge base whose `Serves` cites a superseded or retired ledger unit. Designs merged before the v2 gate stay grandfathered; the config's precedence grandfather list (I1) exempts from the precedence clause only. | a resolvable stub must not read as a satisfied ordering rule (I1), an unresolvable corpus ref must not read as resolved, and dead intent must not anchor new designs (#54 A4) | Must | med | corpus-ref-to-nonexistent-ID fixture red naming the ID; the real corpus green (`local-work-tracker`'s corpus refs resolve); with a merge base, a new design serving a superseded FR red, a pre-existing design serving one green (flag only); a `Status: Draft` design red; grandfathered designs green; every FR-16 fixture keeps its verdict | Test | G1 |
| FR-12 *(003)* | When invoked as `gen-index`, docgate shall emit the tech-design index (`index.md` + `index.json`) deterministically with liveness computed from `Supersedes` edges, lineage, `Serves` inverse map, and staleness flags; `verify-index` shall byte-reject any committed index differing from the recomputed one. | the class's projection; liveness computed, never written | Must | med | hand-edited index red; regenerated green; fixture graph incl. a supersession chain yields expected liveness and flags | Test | G1 |
| FR-13 *(003)* | When invoked as `lint-wiki` or `lint-rollups`, docgate shall verify the class's metadata header rows are present and well-formed (wiki: In one line, Relevant-when, Authority, Sources, Last reviewed; rollups: In one line, Relevant-when, Authority, Update rule, Seeded). | routing metadata becomes checkable | Must | low | missing-row and malformed-date fixtures red; real corpus green | Test | J3 |
| FR-18 *(006 ← FR-14)* | When invoked as `check-governing`, docgate shall verify the vision and principles documents contain no relative markdown link and no baseline requirement-ID citation (the SCAF/PLAN/IMPL/GATE/FLEET/IDEN/GOV/KNOW/OBS/BENCH/VNEXT families); when additionally given a merge base, it shall fail any diff touching either document whose commits lack the `Governing-Change: operator` trailer, and shall fail line modifications or deletions in either document even with the trailer (append-only). | the trust root's machine backing was half-built: only principles were line-checked, and link-direction purity was prompt-side only (#54 A2/A5) | Must | high | trailered line-edit of the vision red; trailered append to the vision green; a vision fixture containing a relative link or a `VNEXT-1` citation red naming the line; principles behavior unchanged (trailered line-edit still red); the real corpus green | Test | G1, J3 |
| FR-25 *(010 ← FR-15)* | When invoked as `lint-incidents`, docgate shall verify the registry at its configured path is append-only vs the merge base — committed `I<n>` blocks immutable except the sanctioned `Links:`-line growth — and that each entry carries a dense `I<n>` ID, a `YYYY-MM-DD` date, a bracketed tag list of one or more comma-separated kebab-case tokens, and the four named fields (Decision, Broke, Lesson, Links) each present as a `- **<Field>:**`-anchored line within the entry. | tags are the retrieval key and fields are the contract — silhouette checks let malformed entries read as conformant (#54 E7) | Must | med | empty-tag and non-kebab-tag fixtures red naming the entry; a field present only as an unanchored substring red; `I1` and the real registry green; graduation `Links:` growth still green; density and append-only fixtures keep their verdicts | Test | J3 |
| FR-17 *(005)* | When invoked as `check-immutable` with a merge base, docgate shall fail any modify, delete, or rename of a technical-design document present at the merge base under the configured tech-designs path, excluding the generated index artifacts, with the remedy naming supersession. | INV-41's CI half for the design class; merge is the freezing event | Must | high | fixture: modify/delete/rename of a merged design each red naming supersession; a design new in the PR edits green; index.md modification green; the PRD fence unaffected | Test | G1, J3 |
| FR-19 *(006)* | When invoked as `check-immutable` with a merge base, docgate shall fail any modify, delete, or rename of a flat placeholder-corpus file (a `.md` file directly under the configured PRDs path, excluding `README.md` and ledger directories) present at the merge base, while imposing no format, template, or content requirement on corpus files. | the corpus is citable requirement text (`corpus:` anchors, rule-4 stable IDs) with no mechanical stability; byte-freeze without retrofit resolves its undeclared mutability class (#54 C5) | Must | med | fixture: modify and delete of a merged flat corpus file each red naming the freeze; `README.md` edit green; a new flat file green; ledger directories unaffected; no content lint runs on corpus files; the real corpus green | Test | G1, J3 |
| FR-21 *(008)* | When invoked as `check-links`, docgate shall resolve every relative markdown link target in `docs/**/*.md`, `AGENTS.md`, and the configured templates directory to an existing file — skipping fenced code blocks, external URLs, and pure `#fragment` links — and shall fail on any dangling target, naming source file, line, and target. | inbound links to frozen docs never go stale by design; outbound links rot silently when targets move, and nothing checked (#54 A5) | Should | low | a dangling-link fixture red naming file, line, and target; fenced-block, external, and fragment-only links ignored; the real corpus green | Test | G1, J3 |
| FR-22 *(008)* | When invoked with a merge base, docgate shall fail any diff to the configured system-map documents (defaults `docs/domain-model.md` and `docs/system-design.md`) that removes or renumbers an `INV-<n>` invariant-registry row or a `D<n>` open-decision row present at the merge base; a map file absent from the tree is skipped without defect. | registry IDs are permanent citation anchors inside deliberately mutable documents — the one quasi-authoritative class with zero protection (#54 C3) | Must | high | fixtures: removing INV-33 red; renumbering an INV row red; appending a fresh INV row green; editing non-registry prose green; absent `system-design.md` produces no defect; the real corpus green | Test | G1, J3 |
| FR-23 *(008)* | When invoked with a merge base, docgate shall fail any diff modifying the gate-policy keys of the pyxis config surface — `grandfathered_designs`, `precedence_grandfathered`, and `capability_paths` — unless the commits carry the `Governing-Change: operator` trailer. | the gate's exemption surface is policy, and policy changes are never self-serve (INV-19's pattern applied to the doc gate; #54 D1) | Must | high | an untrailered diff adding a slug to `grandfathered_designs` red; the same diff trailered green; untrailered edits to `paths`/`templates`/`system` keys green; the real config green | Test | G1, J3 |
| FR-24 *(009)* | When validating an amendment envelope, docgate shall validate against the schema of the envelope's declared template version from the supported set — `amendment-template/v1` (the seven base fields) and `amendment-template/v2` (v1 plus an optional `Tracking` row carrying an issue or PR reference) — and shall fail any envelope declaring a version outside the set or carrying fields unknown to its declared version. | an amendment whose implementation lands later needs a durable pointer to what tracks that work; version-keyed validation adds it without touching frozen history (#54 A1/C1) | Must | med | a v2 fixture with `Tracking` green; a v2 fixture with an unknown field red; a v1 fixture without `Tracking` green (grandfathered forever); a v1 fixture **with** `Tracking` red (unknown to v1); a declared v3 red; the real ledger green | Test | J1, J3 |
| FR-28 *(013 ← FR-26)* | When invoked as `coverage`, docgate shall compute the requirements-coverage view — for every corpus ID and every live ledger unit, the live designs serving it and the explicit unserved remainder per family — from committed inputs on read and print it (a human table by default, `--json` for the machine face), committing it to no artifact; and `gen-index`/`verify-index` shall carry only the design-plane graph (design liveness and `serves_inverse`), never the coverage join, so that adding or amending a ledger never changes `docs/tech-design/index.*`. | coverage is a cross-plane join; committing it anywhere couples the planes (issue #70). The universe already lives under `docs/prds/`, so computing on read removes the coupling in both directions and `index.*` regenerates only when designs change | Should | med | temp-repo fixtures: `docgate coverage` yields the expected served/unserved partition in both faces; `gen-index` emits no `coverage` key/section and `verify-index` is green over the coverage-free index; **adding a ledger amendment leaves `index.*` byte-identical**; superseding a design still returns its solely-served anchors to unserved in `coverage` output; CI publishes the view as a job summary | Test | G1, J2 |
| FR-27 *(012)* | When invoked with a merge base, docgate shall fail a diff that adds more than one PRD ledger entry — an added `NNN-<slug>.md` file one directory below the configured PRDs root — naming each added entry and the remedy; the generated projection, the flat placeholder corpus (NG-1), and modifications of existing entries (already fenced by FR-4) do not count. | one concern per PR becomes code for the ledger: one amendment, or one new base, per change keeps concurrent claims content-free and reviews honest | Must | low | temp-repo fixtures: a diff adding two entries (one ledger or two) → red naming both and the remedy; a diff adding exactly one amendment → green; a new PRD's lone `000-base.md` → green; a projection-only regeneration with no added entry → green; a flat placeholder is never counted | Test | J3, G1 |
| FR-30 *(014)* | When invoked with a merge base, docgate shall fail a diff that touches a governed contract class — a PRD ledger entry (`NNN-<slug>.md`), an authored technical design (`docs/tech-design/*.md`, excluding the generated `index.*`), or the incident registry — if the same diff also touches implementation/capability paths or a second governed class; the generated projections (`effective.*`, `index.*`) and the live rollups (`docs/techspec.md`, `docs/prd.md`) are exempt, riding with their source or with the code respectively. | one concern per PR becomes code for the governed classes; the contract is reviewed on its own before code builds against it, so FR-16 precedence holds in fact and not only at lint time | Must | med | temp-repo fixtures: an amendment + a design → red (two classes); an amendment + code → red; a design + code → red; incident + code → red; a lone amendment with its regenerated `effective.*` → green; a lone design with its regenerated `index.*` → green; a pure code or rollup change → green | Test | J3, G1 |
| FR-29 *(015)* | When invoked as `pre-push` by the push hook, docgate shall read the hook's ref lines from stdin and, when every pushed ref is a deletion (an all-zeros local SHA), pass without running content checks, saying so; when any non-deletion ref is pushed — or stdin carries no parseable ref lines — it shall run the merge-base gate exactly as today. | a ref deletion carries no content to gate; everything else fails closed to the full gate (issue #65) | Should | low | fixtures: a deletion-only stdin exits 0 with the skip notice; a mixed push runs the gate; a content push keeps today's verdicts; empty or malformed stdin runs the gate; the shim stays delegate-only and bash-3.2-clean | Test | G2, J3 |
| FR-31 *(016)* | When invoked as `check-content-paths`, docgate shall fail any system-footprint path reference (a `.pyxis/` path) in wiki entries or in the flat requirement corpus (its README included), and — when a merge base is given — in any PRD ledger entry or technical design added since the merge base; the live rollups, the root operating manual, generated artifacts, and documents present at the merge base are out of scope (rollups describe the shipped footprint; the root manual owns the pointers; frozen history is never retro-edited). Templates are cited by version pin. | the corpus must survive footprint removal interpretable (repo-layout A-2/DINV-4); the promised grep gate becomes real, scoped forward-only (#54 A5) | Must | low | fixtures: a wiki entry or corpus file containing a footprint path → red naming file and line; a ledger entry or design added since the merge base containing one → red; a pre-existing frozen document containing one → green; rollup and root-manual references → green; the real corpus green after the three known violations are fixed | Test | J3, G1 |
| FR-32 *(018)* | When invoked as `check-projection-congruence` with a merge base, docgate shall evaluate every newly added amendment at its own immediately-before and immediately-after replay boundary and fail each after-only disagreement between a binding projected unit and its source-map record: missing on either face, non-live-but-rendered, live-but-absent, current-location drift, contract-field drift, or disagreement between the ID cell's human-visible lineage and machine lineage. | deterministic generation can repeat two mutually inconsistent artifacts; the gate must prove agreement at the entry that changed them | Must | high | each named mismatch class has an isolated red fixture; a same-ID mutation hidden in a section supersede is red; two added entries attribute only their own delta and name the actual inconsistent artifact plus the causative entry | Test | G3, M3, J3, FR-1, FR-3 |
| FR-33 *(018)* | When a checked revision is supplied, `check-projection-congruence` shall read the complete ledger prefix from that exact revision; when none is supplied, it shall read the working tree through the ordinary ledger reader and produce the same verdict for the same bytes. | another session's dirty shared worktree must not alter the verdict for the revision being pushed, and the merge-target path must not become a twin predicate | Must | high | head-set and head-empty conformance fixtures produce identical defects for identical trees; a dirty worktree mutation cannot alter the head-set verdict | Test | G3, M3, AP-11, AP-13, AP-15 |
| FR-34 *(018)* | Where congruence debt exists immediately before a checked entry, docgate shall grandfather it by multiset subtraction over the key `(unit ID, mismatch class, field, projected value under FR-38, source-map value under FR-38)` and fail every after-key whose identical before-key multiplicity is exhausted. | exact identity makes baseline adoption decidable: unchanged debt stays green, but changing a wrong value or moving the disagreement cannot masquerade as no worse | Must | high | unchanged mismatch and byte-identical section restatement green; value replacement at constant count, field/class movement, and count increase red; count decrease and full repair green; projected units with no baseline lineage participate in the same key rule | Test | G3, M3, J1, J3, FR-38 |
| FR-35 *(018)* | When a semantically identical rebase checkpoint is newly added, `check-projection-congruence` shall evaluate the checkpoint as its own boundary and permit lineage compaction only when the checkpoint re-anchors the human-visible ID annotation and the source-map origin to that checkpoint consistently, introducing no after-only mismatch under FR-34. | ignoring checkpoints lets the named repair create fresh hidden debt; comparing them without defining compaction false-rejects a valid rebase | Must | high | a checkpoint containing both a historically annotated row and a grandfathered lineage-less row passes `verify-rebase` and congruence only when both faces re-anchor consistently; retaining old visible lineage with new machine origin and the inverse are red | Test | G3, M3, FR-7, incident:I6 |
| FR-36 *(018)* | After a rebase checkpoint passes FR-35, docgate shall materialize every binding addressable row as one live checkpoint-origin fold-state and source-map record. | rendered rows without fold state caused the dangling-target remedies and stale source map that motivated this amendment | Must | high | a checkpoint containing at least one row from every FR-41 addressable table-schema prefix yields the same complete ID set in the projection, live fold state, and source map, with checkpoint origin on every record | Test | G3, M3, J1, FR-35, FR-41, incident:I6 |
| FR-37 *(018)* | When discovering projected units for fold state or congruence, docgate shall retain the existing merged-base row-kind inference except that every row whose numbered-section ancestry contains §0 shall be non-addressable. | the reproduced phantoms are human-only §0 rows, while exact-header enforcement would silently de-govern legitimate rows in frozen bases whose cosmetic labels predate the registered payload schemas; as-of-review evidence (2026-08-20): origin/main carried 95 drifted UB, 26 drifted J, and 5 drifted R rows outside §0 | Must | high | the real impl-review §0 and nested §0.1 routing tables plus a typed FR-24 supersede are green; an origin/main corpus audit proves zero non-§0 live source-map record disappears from fold state | Test | G3, M3, NR-4 |
| FR-38 *(018)* | When constructing a mismatch key, docgate shall encode each compared scalar as canonical JSON: a projected cell is the exact UTF-8 string returned after the ledger's line-ending/trailing-line-space normalization plus registered table decoding (outer delimiters removed and leading/trailing cell whitespace trimmed), a source-map string is its exact decoded JSON string, a boolean is literal `true` or `false`, and absence is `null`; it shall apply no further case folding, Unicode normalization, internal-whitespace folding, Markdown-markup normalization, or empty/absent equivalence. | an undefined “canonical value” lets two implementations disagree on whether historical debt changed | Must | high | CRLF versus LF, trailing line whitespace, and delimiter-adjacent whitespace normalize green; case change, internal whitespace change, composed versus decomposed Unicode, Markdown markup change, and empty string versus absent are each red; the baseline golden encodes absence as JSON `null` | Test | G3, M3, FR-34 |
| FR-39 *(018)* | Where an author replaces the canonical content of a binding row, docgate shall accept only the lineage-preserving repair authorized by its immediately-before state: a row with live fold state uses `supersede <ID>` with a complete fresh same-prefix row for every FR-41 addressable prefix except a Q classified unanswered by FR-42, which uses `answer`, and NG/WR, which use `rescope`; a lineage-less FR/NR/UB/NFR/DOD/EVAL/EVT/Q/NG/WR row first uses a semantic-no-op rebase checkpoint and then the same prefix-authorized operation; and a lineage-less G/J/M/ABUSE/R row may use a direct whole-section `supersede` that refreshes and annotates only the canonically changed row with no prior fold-state or source-map lineage. `retire` remains available only for actual removal of a live row for which no live DOD `Source` cell or live row `Traces to` cell contains a resolvable reference, and shall never be emitted as part of a replacement repair. | the current design text lists only FR/NR/NFR/DOD/EVAL/UB as `supersede` targets, while docgate and merged amendments already accept lineage-preserving ABUSE and EVT supersession; `retire` plus `add` discards both source-map lineage links and can be structurally invalid for a cited row | Must | high | a lineage-bearing ABUSE-1 changed inside a section is red under FR-32, then `supersede ABUSE-1` with fresh ABUSE-9 is green with ABUSE-1 `superseded_by=ABUSE-9` and ABUSE-9 `supersedes=ABUSE-1`; existing ABUSE and EVT supersede corpus fixtures stay green; lineage-less R, M, and ABUSE rows change through direct whole-section supersede with matching new visible/machine lineage; FR-42/FR-43 unanswered/answered Q, NG/WR, EVT, and FR each pass only their named branch; a row referenced by a live DOD `Source` or live `Traces to` cell cannot retire; no replacement remedy uses `retire` plus `add` or fails cited-target validation | Test | G3, M3, J1, FR-32, FR-35..FR-36, FR-41..FR-43, incident:I6 |
| FR-40 *(018)* | When a whole-section supersede restates a binding row whose canonical content under FR-38 is unchanged from immediately before the operation, docgate shall preserve that row's exact projected bytes, fold-state presence and value, and source-map-record presence and value, including lineage, and shall not refresh or annotate it. | a byte-identical restatement must not manufacture a content delta or baseline mismatch merely because a neighboring row required repair | Must | high | a mixed-section fixture with one changed lineage-less row, one unchanged lineage-less row, and one unchanged lineage-bearing row refreshes and annotates only the changed row; both unchanged rows remain byte/state/source-map identical, and a wholly byte-identical section restatement stays green under FR-34 | Test | G3, M3, FR-34, FR-38 |
| FR-41 *(018)* | When fold state, source-map projection, checkpoint materialization, or repair routing tests whether an ID prefix is registered and addressable, docgate shall use membership in its version-pinned addressable table-schema registry—the registry represented by `schema.TableSchemas`—and shall not infer addressability from membership in the lexical ID-token registry represented by `schema.Prefixes`. | `F` is intentionally recognized as a citation token but has no addressable table schema; treating the two registries as equivalent invents governance and an impossible repair obligation for flow prose | Must | high | an exact registry fixture proves every table-schema key is prefix-eligible for addressability outside §0; `F1` still parses as a lexical token but the real workbench-ui and planner-driven-model-routing F1..F5 rows remain absent from fold state and source maps and require no DOD-7 repair proof; a non-§0 G/J/M/R/EVT/Q/ABUSE/EVAL/NG/WR row remains addressable | Test | G3, M3, FR-36, FR-37, NR-4 |
| FR-42 *(018)* | At each replay boundary, docgate shall classify a Q as answered if and only if either its validated replay-base checkpoint carries `question_answered: true` under FR-43 or a successful `answer Q<n>` operation targeting that ID occurs after that replay base and at or before the boundary; otherwise it shall classify the Q as unanswered. Repair routing shall not read or interpret the Q row's free-text `Status` cell, and missing or invalid required checkpoint state shall fail under FR-43 rather than default to unanswered. | merged Q status cells contain multiple open-like forms and answer-rendered prose, while operations before the latest checkpoint are outside the replay window; both text classification and replay-window-only classification lose a decidable answer state | Must | high | Q rows whose Status cells contain `open`, qualified open prose, `resolved (001) — …`, or arbitrary text but have neither a true replay-base value nor a post-base successful answer op classify unanswered and route to `answer`; after a successful `answer Q1`, another `answer Q1` and a section mutation are red with the `supersede Q1` fresh-Q remedy; after a semantic-no-op checkpoint, the same Q remains answered solely through `question_answered: true`, and a fresh Q introduced after the checkpoint begins false | Test | G3, M3, FR-35..FR-36, FR-39, FR-43, UB-6 |
| FR-43 *(018)* | When folding a Q, docgate shall maintain a required boolean `question_answered` in its fold-state and `effective.source.json` unit record: a Q introduced without checkpoint-carried state starts false and a successful `answer Q<n>` sets it true; non-Q records omit the field. When docgate creates, validates, or consumes a rebase checkpoint, it shall copy each live Q's immediately-before value into typed replay-base metadata, restore that value before applying later entries, and have `verify-rebase` compare it with the pre-checkpoint fold, failing any missing, invalid, or unequal value. | the checkpoint Markdown re-anchors units but neither its free-text Status cell nor the current source-map lineage fields can preserve an answer operation that the replay window no longer contains | Must | high | a base Q with arbitrary Status and a fresh post-checkpoint Q each emit explicit false; a ledger with `answer Q1` followed by `002-rebase.md` emits and restores Q1 true while unanswered Q2 remains false; folding the full pre-checkpoint history and folding from the checkpoint produce identical Q classifications and source-map booleans; deleting or flipping either checkpoint value reds `verify-rebase`, deleting or flipping the generated source-map value reds `verify-generated`, and a second faithful checkpoint preserves both values | Test | G3, M3, FR-1, FR-3, FR-7, FR-35..FR-36, FR-42 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If ledger numbering has a gap or duplicate | fail lint naming the files and the next free number | named defect + remedy | Must | low | both fixture cases name files and the number | Test | F1, FR-8 |
| UB-2 | If a payload table's columns mismatch the pinned template schema | abort the fold naming the column diff | named defect | Must | med | reordered and missing-column fixtures both named | Test | FR-2 |
| UB-3 | If the working tree contains uncommitted ledger changes during `verify-generated` | verify against the working tree exactly and say so in output | verdict + dirty-tree note | Should | low | dirty-tree fixture produces the note and correct verdict | Test | J1 |
| UB-4 *(018)* | If a named added amendment or rebase checkpoint is not replayed at the checked revision | fail closed without a congruence verdict | a defect naming the missing entry, replay range, and remedy; never a silent pass | Must | high | absent entry and entry outside the replay window each red; no case returns green | Test | FR-2, FR-32, FR-35 |
| UB-5 *(018)* | If the ledger prefix cannot be read without one or more defects | fail closed before congruence evaluation and preserve every underlying defect unchanged | one defect per cause with its original artifact path, operation, reason, and remedy | Must | high | a fixture with multiple read defects returns all defects in stable order; none is rewritten against the added amendment or reduced to reason-only text | Test | FR-2, FR-33 |
| UB-6 *(018)* | If an author applies a unit or section operation whose prefix, canonical-content delta, or immediately-before lineage or Q-answer state requires another repair under FR-39 | fail before mutation and select the authorized branch | a remedy naming rebase→unit operation, changed-lineage-less whole-section repair, or lineage-bearing `supersede <ID>` with a fresh same-prefix row; never a refresh that hides a changed lineage-bearing row or a replacement expressed as `retire` plus `add` | Must | high | `supersede R6`, `supersede M4`, and `supersede ABUSE-5` while lineage-less each red with their owning-section branch; a lineage-bearing ABUSE-1 changed inside section supersede reds with `supersede ABUSE-1` and a fresh ABUSE ID; `answer Q1` after a successful prior answer, including after checkpoint restoration, and direct `rescope NG-5` each red with the correct branch; every emitted remedy passes the next validation layer and a row referenced by a live DOD `Source` or live `Traces to` cell is never sent to `retire` | Test | FR-2, FR-32, FR-39, FR-41..FR-43, incident:I6 |

### 5.3 Permissions and abuse cases

Skipped: repo-local CLI with no roles and no new surface — the gate itself is the abuse control (FR-3/FR-4), and its own change path is governed by this repo's review rules.

### 5.4 Data

Skipped: no state created or owned — inputs are tracked files; outputs are the two generated artifacts whose contract is FR-1/FR-3. No retention, no privacy surface, no egress.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| git (merge base + exact checked revision) | in | immutability and congruence boundaries | merge-base name-status identifies added ledger entries; exact tree reads supply the complete prefix; no shell interpolation | unresolved revision or missing entry → fail closed with original attribution | FR-4, FR-33, UB-4..UB-5 |
| PRD projection pair | in/out | the two faces whose agreement is gated | merged-base inferred rows outside the §0 subtree ↔ versioned source-map unit records, including required per-Q `question_answered`; FR-38 defines exact value equality; deterministic fold remains the sole writer | after-only mismatch → red; missing/altered Q state → red; historical identical key → grandfathered | FR-1, FR-3, FR-32, FR-34, FR-37, FR-38, FR-43 |
| rebase checkpoint machine state | in/out | complete replay-base semantics | each live Q carries required typed `question_answered`; `verify-rebase` compares it with the pre-checkpoint fold and replay restores it before later ops | missing, invalid, or unequal value → red; no fallback to Status or pre-checkpoint history | FR-7, FR-35..FR-36, FR-42..FR-43 |
| CI required context `docs-gate` | out | merge enforcement | the umbrella includes every registered applicable verb, including `check-projection-congruence`; exit code + human defects + `--defects-json` | red blocks merge; unwiring is rollback only after a guardrail breach | FR-2, FR-32..FR-43, UB-4..UB-6 |
| pre-push hook | out | desk-time verdict | the same umbrella binary and registered verb set over the exact pushed revision | AP-11: never a twin predicate; deletion-only behavior remains FR-29 | G2, G3, FR-33 |
| templates + addressable table-schema registry | in | addressable row kinds and op payloads | FR-41 registry membership defines addressability; payload headers remain exact-schema; merged base rows retain their existing inferred kind; §0 ancestry is excluded before row inference | malformed payload schema → red; lexical-only `F` and every §0-subtree row are non-addressable; cosmetic base-header drift remains governed | FR-5, FR-37, FR-41, NR-4 |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Performance | docgate shall verify a 50-entry ledger fast enough for pre-push. | p95 < 1s over the fixture corpus on developer hardware | Test |
| NFR-2 | Operability | docgate shall run hermetically as a single static binary. | zero network syscalls in the gate path; no runtime deps beyond the binary | Analysis |
| NFR-3 | Maintainability | the repo's Go rules apply in full. | gofmt/go vet/golangci-lint clean; `go test -race` green; stdlib-only | Inspection |

### 5.7 Instrumentation — the tracking plan

Runtime telemetry remains skipped: docgate is hermetic. M1/M2 continue to read CI and PR history. M3 reads two repository-verifiable feeds: required-check history for every ledger-entry PR after activation, and the committed activation-debt golden reconstructed independently by the corpus-audit test. The implementation owner generates that golden once from the exact merge-base revision where the verb is first wired; its revision and ledger fingerprint never move with later merges. Ordinary proposals never rewrite it. A proposal whose explicit purpose is to repair grandfathered debt deletes only the exact repaired multiplicity in the same change; no new or altered key may enter. A proven baseline defect requires the verb to be unwired and a dedicated reviewed gate-fix proposal to regenerate the record set from the unchanged activation revision. The first checkpoint is the first 10 gated ledger entries; any accepted new canonical mismatch is both an M3 failure and a gate incident, never an aggregate to discuss away.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..8 | Test | CI golden-suite run | yes |
| DOD-2 | No unwanted-behavior or non-regression row broken | UB-1..3, NR-1 | Test | suite result | yes |
| DOD-3 | Gate wired: required CI context + pre-push hook, same binary, same verbs | §5.5 | Inspection | branch-protection config + hook file | yes |
| DOD-4 | Both templates' example ops parse as golden fixtures | §5.5 templates row | Test | fixture manifest | yes |
| DOD-5 | This ledger's own projection is generated and verified by the shipped tool | FR-1, FR-3 | Test | committed `effective.*` for `docs/prds/docgate/` passing `verify-generated` | yes |
| DOD-6 | Guardrails intact through the first 10 gated amendments | §3.3 | Analysis | PR history at the M2 checkpoint | yes |
| DOD-7 *(018)* | Projection congruence ships with the full FR-32..FR-43/UB-4..UB-6/NR-4 matrix, exact-revision and working-tree conformance, every FR-41-addressable-prefix repair proof, checkpoint Q-state preservation, and the live Wiring rollup naming the verb | FR-32..FR-43, UB-4..UB-6, NR-4 | Test + Inspection | focused red/green tests; full race suite; `docs/techspec.md` Wiring diff; a committed sorted golden at `internal/docgate/gate/testdata/projection-congruence-baseline.json`, generated once by the implementation owner from the gate-activation merge base, carrying that immutable revision and ledger fingerprint plus every `(ledger, unit ID, mismatch class, field, projected value, source-map value)` record, independently reconstructed and byte-compared by a corpus audit whose omission and hidden-mismatch fixtures red; ordinary proposals never regenerate it, an explicit debt-repair proposal may delete only the exact repaired multiplicity, and a proven golden defect requires a dedicated reviewed gate-fix proposal while the verb is unwired | yes |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-3 *(006 ← NR-1)* | Corpus and non-ledger docs are never content-gated, linted, or reformatted — the corpus fence (FR-19) freezes bytes of merged flat corpus files and nothing more; wiki, maps, and rollups keep exactly their own named checks | placeholders are never retrofitted (the surviving spirit of NG-1); gates never widen silently | low | a gate run over the current repo yields zero content findings on flat corpus files; FR-19 fires only on modify/delete/rename of merged flat corpus files; wiki/map/rollup checks unchanged | Test | G1, J3 |
| NR-2 | Conversation-based operator review and Go plan rendering remain independent of docgate | docgate is additive tooling | low | the plan-rendering and document-workflow suites stay green in the introducing PR | Test | §5.5 |
| NR-4 *(018)* | §0 and all of its subsections remain human-only: no docgate contract-unit parser, fold comparison, source-map projection, or gate verdict treats an ID-looking row anywhere in that subtree as an addressable unit | read-to-rule routing prose binds nothing and automated consumers are contractually forbidden to read, cite, or obey it | high | the complete prd-template/v2 §0 fixture, including exact and slash-joined live IDs under §0 and §0.1, contributes zero units and cannot shadow a binding row with the same ID | Test | FR-32, FR-37, prd:read-to-rule/FR-5 |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Existing docgate | preserve the shipped fold and registered gate suite | all live requirements through amendment 017 | current corpus and gates remain green |
| Congruence contract | freeze the observable and adoption boundary | G3, M3, FR-32..FR-43, UB-4..UB-6, NR-4, DOD-7 | this amendment reviewed and operator-approved |
| Congruence design | decide parser refresh, §0-ancestry tracking, addressable-registry scope, section-repair annotation, unchanged-row preservation, checkpoint Q-state encoding/restoration/projection, citation-domain validation, artifact comparison, checkpoint compaction, defect attribution, and the lineage-preserving `supersede` target-domain expansion while preserving merged-base inference and implementing FR-38's fixed equality contract | the next governed design result | technical design review approved; no open DINV or migration question |
| Fixtures first | pin every mismatch transition, FR-41-addressable-prefix repair, unchanged-row invariant, pre-answer→answer→checkpoint→post-checkpoint Q-state transitions, and the complete corpus baseline before production code | FR-32..FR-43, UB-4..UB-6, NR-4 | reviewed golden + all red/green fixtures fail against the pre-change implementation for the intended reason |
| Implement and wire | add the self-registering verb, refresh model semantics as designed, update the living rollup | code + tests + `docs/techspec.md` | DOD-7 complete; full code gate and docs gate green; hard check active with no advisory mode |

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| amendment model + all three templates merged | docs | Fixtures-first | confirmed (#13–#15, #18–#19) | — |
| approved technical design (RW-5) | docs | Fold | next step after this PRD | @mdarabi |
| branch-protection edit rights | approval | Wire | confirmed | @mdarabi |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | markdown-table parsing fragility — the fold's rabbit hole | M | H | closed parser, hard errors, one fixture per op × table kind; no lenient mode exists to accrete around | implementation |
| R2 | template evolution desyncs the goldens | M | M | template PRs ship updated goldens in the same change (prd-amendment-model R3); the goldens fail otherwise | implementation |
| R3 | gate friction drives authors away from ledger directories | L | H | pre-push verdicts in seconds; every defect names file, reason, and remedy | @mdarabi |
| R4 | the self-projection bootstrap (DOD-5) surfaces fold bugs against this very document | M | L | that is the point — this PRD is fixture zero; bugs found here are bugs not found in the field | implementation |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all live Must requirements, including the complete FR-32..FR-43/UB-4..UB-6/NR-4 matrix | hermetic fixture suite |
| Reliability | deterministic fold remains byte-identical; exact revision and working-tree paths conform | full race suite + conformance fixtures |
| Adoption safety | independently reconstructed debt set byte-equals the reviewed activation golden after permitted repair-only deletions; every after-only key reds from first release | corpus audit + required gate |
| Repairability | checkpoint compaction passes both gates; every FR-41-addressable formerly lineage-less row kind follows its authorized repair without a dangling target; Q routing is invariant to Status prose and checkpoint compaction; every live replacement preserves both source-map lineage links | end-to-end FR-39/FR-41/FR-42/FR-43 fixtures |
| Supportability | every defect preserves actual artifact attribution and names an executable remedy | defect catalog + UB-4..UB-6 fixtures |

### 6.5 Rollout and reversibility

The gate launches hard once DOD-7 is complete: the reviewed activation golden and its immutable activation fingerprint are the adoption boundary, identical historical keys subtract away, and any after-only key blocks locally and in CI. Later merge bases do not regenerate the golden; only explicit repairs shrink it, and only a dedicated reviewed gate-fix while unwired corrects a proven oracle defect. There is no advisory phase and no corpus-wide repair prerequisite. A lineage-less row changes only through the prefix branch in F2 or F3. Reversibility is operational, not semantic: a proven false-red guardrail breach may temporarily unwire the registered verb from the umbrella while the code is repaired, but no exemption list or best-effort mode is added and every change merged during the gap is independently audited before rewiring.

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Contract | amendment 018 operator-approved; PRD review converged | @mdarabi | ☐ |
| Design | superseding projection-congruence design serves every new Must row, resolves checkpoint compaction, binds addressability to FR-41, specifies FR-43 checkpoint Q-state encoding/restoration/projection, widens the existing `supersede` target domain as FR-39 requires, and reconciles section versus unit repair semantics | @mdarabi | ☐ |
| Verification | full transition matrix, exact-read conformance, §0 and lexical-F real-document fixtures, pre/post-checkpoint Q-state fixtures, and every FR-39 repair green | implementation | ☐ |
| Adoption | implementation owner generated the activation golden once from the wiring merge base; independent reconstruction byte-equals it and lifecycle fixtures enforce repair-only shrinkage | reviewer | ☐ |
| Docs | `docs/techspec.md` Wiring names the live verb and artifact boundary | implementation | ☐ |
| Support | every defect preserves artifact attribution and names an executable remedy | implementation | ☐ |

*(remaining rows skipped: no pricing, marketing, legal, localization, or enablement surface — repo-local hermetic tool)*

**Decision:** Launch gate remains pending until every row above is checked and the operator accepts the implemented result.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | pending |

### 6.7 Post-launch review

- **Review trigger** — 10 ledger-entry PRs through the hard congruence gate · **Owner** — @mdarabi
- **Evidence** — M1/M2 history, M3 required-check history, the independently reconstructed corpus inventory against the activation golden, false-red/false-green corrections, and every use of the F2/F3 repair paths.
- **Verdict recorded** — adopt / iterate / kill; any accepted after-only mismatch or non-executable remedy is a guardrail breach, not a thresholded metric.

## 7. Alignment

### 7.1 FAQ

**Customer (authoring agents):** **Q:** The projection looks wrong — can I fix `effective.md` directly? — **A:** Never. Fix the amendment op, or file a fold bug; `verify-generated` rejects hand-edits by construction, and the defect message will say exactly this.

**Internal:** **Q:** Why not ship the design-doc verbs (v2) now — the conventions are merged? — **A:** Scope discipline: v1 is the approved amendment-model scope; v2's checks depend on a proven parser and are enumerated in tech-design-conventions §3.4 (WR-1). One proven layer before the next.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | `verify-rebase` semantic equality: byte-equality of the regenerated projection, or normalized-AST equality? | tech design | Fold phase | open |
| Q2 | Pre-push hook installation mechanics — pyxis has no workbench installer yet (BENCH-2 is baseline inventory, not built) | tech design | Wire phase | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-06 | fixtures-first phasing | code-first | the contract is the product; goldens are the executable spec | @mdarabi |
| 2026-07-06 | machine-readable defects from birth (FR-2) | human text only, JSON later via amendment | CI annotation is a J3 need, known now; no reason to relearn it | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-06 | 000-base drafted for review | @mdarabi |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 gate signed (T2) | pass | existing problem gate plus build-result:PR-1090 and review-thread:PR-1091 evidence |
| Zero solution-blocking TBDs or open questions | pass | parser and checkpoint mechanisms are design decisions bounded by FR-32..FR-43/UB-4..UB-6; value equality, repair selection, unchanged-row behavior, addressability, Q state, and its checkpoint carrier are fixed by FR-38..FR-43/UB-6 |
| Every Must FR has a fit criterion and verification method | pass | FR-1..FR-9, live amended rows, FR-32..FR-43, and UB-4..UB-6 each carry executable Test criteria |
| Every high-risk row has a declared safety plan | pass | exact baseline subtraction, hard launch after complete golden, reversible wiring only on guardrail breach, no advisory or exemption path |
| Goals and metrics are closed | pass | G3→M3; M3 has required-check plus independent-audit feeds and a 10-entry checkpoint |
| Non-goals drawn; non-regression named | pass | §4.2 refuses new verbs/exemptions/corpus rewrite; NR-4 protects the human-only boundary |
| Every primary job is covered | pass | J1→FR-33..FR-43/UB-6/F1/F2/F3; J2→FR-32/DOD-7; J3→FR-32..FR-35/FR-38/UB-4..UB-5 |
| Every key flow has functional, failure, and non-regression coverage | pass | F1→FR-32..FR-34/FR-38/UB-4..UB-5/NR-4; F2→FR-35..FR-39/FR-41..FR-43/UB-4/UB-6/NR-4; F3→FR-32/FR-34/FR-37/FR-39..FR-41/UB-6/NR-4 |
| Every primary metric has instrumentation or an explicit plan | pass | M1/M2 unchanged; M3→required-check history + committed golden corpus audit |
| Every blocking DoD row is sourced and evidenced | pass | DOD-7 binds every new row, full matrix, golden, race suite, repair, and living rollup |
| Risks and assumptions have owners | pass | implementation owns parser/checkpoint risks; @mdarabi owns launch and first-10 review |
| Human-judgment criteria name an owner and rubric | pass | operator launch decision uses §6.6's closed checklist and §6.7's evidence set |
| Approved technical design precedes implementation | pending | the next governed result must supersede the affected amendment-model semantics and serve FR-32..FR-43/UB-4..UB-6/NR-4 before code resumes |
| DoD complete: every blocking row sourced and evidenced | pass | no n/a rows; DOD-7 is the projection-congruence completion envelope |

## Superseded & retired

- **FR-11** *(superseded by FR-16 (004))* — `FR-11 *(003)* | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only — range shorthand is a defect), reference resolution, mandatory-by-name Observability and Security & privacy subsections with the header pair, and slug rules, grandfathering designs merged before the v2 gate. | the class's conventions become checks | Must | med | one red fixture per defect class; the real corpus green with pre-v2 docs flagged grandfathered; a `FR-1..FR-8` ref → named defect | Test | G1`
- **FR-14** *(superseded by FR-18 (006))* — `FR-14 *(003)* | When invoked as `check-governing` with a merge base, docgate shall fail any diff touching the vision or principles unless the commits carry the `Governing-Change: operator` trailer, and shall fail principles line modifications or deletions even with the trailer (append-only). | the trust root gains machine backing; attestation, not authorization | Must | high | untrailed diff red; trailed append green; trailed line-edit of principles red | Test | G1, J3`
- **NG-1** *(rescoped (006))* — `NG-1 | Gating the placeholder corpus (flat `docs/prds/*.md`) | operator ruling: placeholders are exempt, never retrofitted`
- **NR-1** *(superseded by NR-3 (006))* — `NR-1 | The placeholder corpus and every non-ledger doc are never gated, touched, or reformatted | NG-1; operator ruling | low | a gate run over the current repo yields zero findings outside `docs/prds/<feature>/` directories | Test | NG-1`
- **FR-16** *(superseded by FR-20 (007))* — `FR-16 *(004 ← FR-11)* | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only), reference resolution, mandatory-by-name Observability and Security & privacy subsections with the header pair, and slug rules — and shall fail any design whose `Serves` cites a `prd:` ledger without at least one requirement-row anchor (FR-n, UB-n, or NFR-n): waiting-room, non-goal, and question anchors alone do not satisfy PRD-precedes-design. Designs merged before the v2 gate stay grandfathered; designs in the config's precedence grandfather list (I1) are exempt from the precedence clause only. | a resolvable stub must not read as a satisfied ordering rule (I1) | Must | med | WR-only `Serves` fixture red naming I1; FR-anchored fixture green; `docgate-v2` exempt via the precedence list; the real corpus green | Test | G1`
- **FR-15** *(superseded by FR-25 (010))* — `FR-15 *(003)* | When invoked as `lint-incidents`, docgate shall verify the registry is append-only vs the merge base and each entry carries a dense `I<n>` ID with the four named fields. | the append-only registry becomes checkable | Must | low | tampered-entry and gap fixtures red; real registry green | Test | J3`
- **FR-26** *(superseded by FR-28 (013))* — `FR-26 *(011)* | When invoked as `gen-index`, docgate shall additionally emit a requirements-coverage view in both index faces: for every requirement ID present in the flat placeholder corpus (the SCAF/PLAN/IMPL/GATE/FLEET/IDEN/GOV/KNOW/OBS/BENCH/VNEXT families) and every live unit of every ledger projection, the live designs serving it, plus the explicit list of IDs served by no live design; `verify-index` shall byte-verify the coverage content with the rest of the index. | the serves[] edge exists per design; planning needs its precomputed inverse and the unserved remainder, not archaeology (#54 C4) | Should | med | a fixture corpus + ledger + design set yields the expected served/unserved partition; the unserved list names IDs verbatim; a hand-edited coverage section red under `verify-index`; the real corpus green | Test | G1, J2`
- **FR-10** *(retired (017))* — `FR-10 *(002)* | When invoked with a merge base, docgate shall fail a diff that touches capability paths (cmd/, internal/, .githooks/, .github/workflows/ — excluding *_test.go and testdata/) without touching docs/prd.md or docs/techspec.md. | rule 3 becomes code, not reviewer memory; a Last-verified bump is the auditable no-capability-change attestation | Must | low | temp-repo fixtures: capability diff without rollup diff → red naming the remedy; with a rollup touch → green; test-only diff → green | Test | J3, G1`
