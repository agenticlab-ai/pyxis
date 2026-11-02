# Amendment 018 — projection and source-map congruence

| Field | Value |
|---|---|
| **Amends** | docgate after 017 |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-20 |
| **Learned-from** | build-result:PR-1090 · review-thread:PR-1091 · operator-decision:projection-congruence-corpus-compatibility-2026-08-20 · incident:I6 |
| **Blast radius** | tier: contract-changing; touches: G3, M3, FR-32, FR-33, FR-34, FR-35, FR-36, FR-37, FR-38, FR-39, FR-40, FR-41, FR-42, FR-43, UB-4, UB-5, UB-6, NR-4, DOD-7, §1, §3.3, §4.1, §4.2, §4.3, §4.4, §5.5, §5.7, §6.1, §6.4, §6.5, §6.6, §6.7, §8; reason: adds a required merge-base gate over PRD projection/source-map agreement, preserves merged-base kind inference while excluding the complete §0 subtree, freezes exact baseline-delta, addressability, checkpoint-carried Q-state, and lineage-preserving repair semantics, and joins the gate into the complete author-to-release path |
| **Template version** | amendment-template/v1 |

## Operations

### op: add G

Reason: Byte determinism alone can reproduce two stale artifacts; projection trust needs a goal that measures semantic agreement rather than repeatability only.

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G3 | a merge-base congruence gate over every newly added amendment and rebase checkpoint | no new projection/source-map disagreement can merge while historical debt remains non-blocking and repairable | M3 |

### op: add M

Reason: The new goal needs an independent signal that can detect a false-green gate, not another measurement of deterministic output.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M3 | G3 | newly introduced projection/source-map mismatches accepted by the required gate | no gate; the complete independently reconstructed debt set at gate activation, pinned to the activation merge-base revision and ledger fingerprint | zero across the full defect matrix and the first 10 gated ledger entries; the independent corpus audit remains byte-equal to the reviewed activation golden after only exact-key deletions made by explicit debt-repair proposals | required-check history + independent corpus-audit test |

### op: add FR

Reason: Deterministic generation currently allows a whole-section replacement to change binding rows in `effective.md` while the fold's unit model and `effective.source.json` remain stale.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-32 | When invoked as `check-projection-congruence` with a merge base, docgate shall evaluate every newly added amendment at its own immediately-before and immediately-after replay boundary and fail each after-only disagreement between a binding projected unit and its source-map record: missing on either face, non-live-but-rendered, live-but-absent, current-location drift, contract-field drift, or disagreement between the ID cell's human-visible lineage and machine lineage. | deterministic generation can repeat two mutually inconsistent artifacts; the gate must prove agreement at the entry that changed them | Must | high | each named mismatch class has an isolated red fixture; a same-ID mutation hidden in a section supersede is red; two added entries attribute only their own delta and name the actual inconsistent artifact plus the causative entry | Test | G3, M3, J3, FR-1, FR-3 |

### op: add FR

Reason: A consequential pre-push verdict must inspect the exact bytes being pushed, while the merge-target path still needs an equivalent working-tree read.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-33 | When a checked revision is supplied, `check-projection-congruence` shall read the complete ledger prefix from that exact revision; when none is supplied, it shall read the working tree through the ordinary ledger reader and produce the same verdict for the same bytes. | another session's dirty shared worktree must not alter the verdict for the revision being pushed, and the merge-target path must not become a twin predicate | Must | high | head-set and head-empty conformance fixtures produce identical defects for identical trees; a dirty worktree mutation cannot alter the head-set verdict | Test | G3, M3, AP-11, AP-13, AP-15 |

### op: add FR

Reason: Existing debt is adoptable only if the baseline subtraction has a closed identity; “worse” by count is ambiguous and can exchange one defect for another without a red verdict.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-34 | Where congruence debt exists immediately before a checked entry, docgate shall grandfather it by multiset subtraction over the key `(unit ID, mismatch class, field, projected value under FR-38, source-map value under FR-38)` and fail every after-key whose identical before-key multiplicity is exhausted. | exact identity makes baseline adoption decidable: unchanged debt stays green, but changing a wrong value or moving the disagreement cannot masquerade as no worse | Must | high | unchanged mismatch and byte-identical section restatement green; value replacement at constant count, field/class movement, and count increase red; count decrease and full repair green; projected units with no baseline lineage participate in the same key rule | Test | G3, M3, J1, J3, FR-38 |

### op: add FR

Reason: A rebase is the existing legal repair boundary for lineage-less debt, but it must not introduce a new human/machine lineage mismatch while compacting history.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-35 | When a semantically identical rebase checkpoint is newly added, `check-projection-congruence` shall evaluate the checkpoint as its own boundary and permit lineage compaction only when the checkpoint re-anchors the human-visible ID annotation and the source-map origin to that checkpoint consistently, introducing no after-only mismatch under FR-34. | ignoring checkpoints lets the named repair create fresh hidden debt; comparing them without defining compaction false-rejects a valid rebase | Must | high | a checkpoint containing both a historically annotated row and a grandfathered lineage-less row passes `verify-rebase` and congruence only when both faces re-anchor consistently; retaining old visible lineage with new machine origin and the inverse are red | Test | G3, M3, FR-7, incident:I6 |

### op: add FR

Reason: The checkpoint is a valid lineage repair only if every binding row enters the fold's live model rather than merely appearing in rendered Markdown.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-36 | After a rebase checkpoint passes FR-35, docgate shall materialize every binding addressable row as one live checkpoint-origin fold-state and source-map record. | rendered rows without fold state caused the dangling-target remedies and stale source map that motivated this amendment | Must | high | a checkpoint containing at least one row from every FR-41 addressable table-schema prefix yields the same complete ID set in the projection, live fold state, and source map, with checkpoint origin on every record | Test | G3, M3, J1, FR-35, FR-41, incident:I6 |

### op: add FR

Reason: Congruence must exclude the complete human-only §0 subtree without reversing the merged-base rule that existing row content establishes its own kind.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-37 | When discovering projected units for fold state or congruence, docgate shall retain the existing merged-base row-kind inference except that every row whose numbered-section ancestry contains §0 shall be non-addressable. | the reproduced phantoms are human-only §0 rows, while exact-header enforcement would silently de-govern legitimate rows in frozen bases whose cosmetic labels predate the registered payload schemas; as-of-review evidence (2026-08-20): origin/main carried 95 drifted UB, 26 drifted J, and 5 drifted R rows outside §0 | Must | high | the real impl-review §0 and nested §0.1 routing tables plus a typed FR-24 supersede are green; an origin/main corpus audit proves zero non-§0 live source-map record disappears from fold state | Test | G3, M3, NR-4 |

### op: add FR

Reason: A hard baseline comparator needs one observable definition of value equality; leaving canonicalization to design changes red/green outcomes.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-38 | When constructing a mismatch key, docgate shall encode each compared scalar as canonical JSON: a projected cell is the exact UTF-8 string returned after the ledger's line-ending/trailing-line-space normalization plus registered table decoding (outer delimiters removed and leading/trailing cell whitespace trimmed), a source-map string is its exact decoded JSON string, a boolean is literal `true` or `false`, and absence is `null`; it shall apply no further case folding, Unicode normalization, internal-whitespace folding, Markdown-markup normalization, or empty/absent equivalence. | an undefined “canonical value” lets two implementations disagree on whether historical debt changed | Must | high | CRLF versus LF, trailing line whitespace, and delimiter-adjacent whitespace normalize green; case change, internal whitespace change, composed versus decomposed Unicode, Markdown markup change, and empty string versus absent are each red; the baseline golden encodes absence as JSON `null` | Test | G3, M3, FR-34 |

### op: add FR

Reason: The repair grammar must preserve lineage and explicitly reconcile the narrow design text with the broader implementation and merged-corpus precedent.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-39 | Where an author replaces the canonical content of a binding row, docgate shall accept only the lineage-preserving repair authorized by its immediately-before state: a row with live fold state uses `supersede <ID>` with a complete fresh same-prefix row for every FR-41 addressable prefix except a Q classified unanswered by FR-42, which uses `answer`, and NG/WR, which use `rescope`; a lineage-less FR/NR/UB/NFR/DOD/EVAL/EVT/Q/NG/WR row first uses a semantic-no-op rebase checkpoint and then the same prefix-authorized operation; and a lineage-less G/J/M/ABUSE/R row may use a direct whole-section `supersede` that refreshes and annotates only the canonically changed row with no prior fold-state or source-map lineage. `retire` remains available only for actual removal of a live row for which no live DOD `Source` cell or live row `Traces to` cell contains a resolvable reference, and shall never be emitted as part of a replacement repair. | the current design text lists only FR/NR/NFR/DOD/EVAL/UB as `supersede` targets, while docgate and merged amendments already accept lineage-preserving ABUSE and EVT supersession; `retire` plus `add` discards both source-map lineage links and can be structurally invalid for a cited row | Must | high | a lineage-bearing ABUSE-1 changed inside a section is red under FR-32, then `supersede ABUSE-1` with fresh ABUSE-9 is green with ABUSE-1 `superseded_by=ABUSE-9` and ABUSE-9 `supersedes=ABUSE-1`; existing ABUSE and EVT supersede corpus fixtures stay green; lineage-less R, M, and ABUSE rows change through direct whole-section supersede with matching new visible/machine lineage; FR-42/FR-43 unanswered/answered Q, NG/WR, EVT, and FR each pass only their named branch; a row referenced by a live DOD `Source` or live `Traces to` cell cannot retire; no replacement remedy uses `retire` plus `add` or fails cited-target validation | Test | G3, M3, J1, FR-32, FR-35..FR-36, FR-41..FR-43, incident:I6 |

### op: add FR

Reason: Unchanged-row preservation is an independently testable fold invariant, not a repair-selection branch.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-40 | When a whole-section supersede restates a binding row whose canonical content under FR-38 is unchanged from immediately before the operation, docgate shall preserve that row's exact projected bytes, fold-state presence and value, and source-map-record presence and value, including lineage, and shall not refresh or annotate it. | a byte-identical restatement must not manufacture a content delta or baseline mismatch merely because a neighboring row required repair | Must | high | a mixed-section fixture with one changed lineage-less row, one unchanged lineage-less row, and one unchanged lineage-bearing row refreshes and annotates only the changed row; both unchanged rows remain byte/state/source-map identical, and a wholly byte-identical section restatement stays green under FR-34 | Test | G3, M3, FR-34, FR-38 |

### op: add FR

Reason: Repair coverage must use the addressable table-schema registry, not the broader lexical token list that also recognizes non-governed flow labels.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-41 | When fold state, source-map projection, checkpoint materialization, or repair routing tests whether an ID prefix is registered and addressable, docgate shall use membership in its version-pinned addressable table-schema registry—the registry represented by `schema.TableSchemas`—and shall not infer addressability from membership in the lexical ID-token registry represented by `schema.Prefixes`. | `F` is intentionally recognized as a citation token but has no addressable table schema; treating the two registries as equivalent invents governance and an impossible repair obligation for flow prose | Must | high | an exact registry fixture proves every table-schema key is prefix-eligible for addressability outside §0; `F1` still parses as a lexical token but the real workbench-ui and planner-driven-model-routing F1..F5 rows remain absent from fold state and source maps and require no DOD-7 repair proof; a non-§0 G/J/M/R/EVT/Q/ABUSE/EVAL/NG/WR row remains addressable | Test | G3, M3, FR-36, FR-37, NR-4 |

### op: add FR

Reason: Q repair routing needs replay-derived state rather than interpretation of an unconstrained Markdown status cell.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-42 | At each replay boundary, docgate shall classify a Q as answered if and only if either its validated replay-base checkpoint carries `question_answered: true` under FR-43 or a successful `answer Q<n>` operation targeting that ID occurs after that replay base and at or before the boundary; otherwise it shall classify the Q as unanswered. Repair routing shall not read or interpret the Q row's free-text `Status` cell, and missing or invalid required checkpoint state shall fail under FR-43 rather than default to unanswered. | merged Q status cells contain multiple open-like forms and answer-rendered prose, while operations before the latest checkpoint are outside the replay window; both text classification and replay-window-only classification lose a decidable answer state | Must | high | Q rows whose Status cells contain `open`, qualified open prose, `resolved (001) — …`, or arbitrary text but have neither a true replay-base value nor a post-base successful answer op classify unanswered and route to `answer`; after a successful `answer Q1`, another `answer Q1` and a section mutation are red with the `supersede Q1` fresh-Q remedy; after a semantic-no-op checkpoint, the same Q remains answered solely through `question_answered: true`, and a fresh Q introduced after the checkpoint begins false | Test | G3, M3, FR-35..FR-36, FR-39, FR-43, UB-6 |

### op: add FR

Reason: A replay base must carry Q answer state explicitly because the operation that established it is no longer inside the replay window.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-43 | When folding a Q, docgate shall maintain a required boolean `question_answered` in its fold-state and `effective.source.json` unit record: a Q introduced without checkpoint-carried state starts false and a successful `answer Q<n>` sets it true; non-Q records omit the field. When docgate creates, validates, or consumes a rebase checkpoint, it shall copy each live Q's immediately-before value into typed replay-base metadata, restore that value before applying later entries, and have `verify-rebase` compare it with the pre-checkpoint fold, failing any missing, invalid, or unequal value. | the checkpoint Markdown re-anchors units but neither its free-text Status cell nor the current source-map lineage fields can preserve an answer operation that the replay window no longer contains | Must | high | a base Q with arbitrary Status and a fresh post-checkpoint Q each emit explicit false; a ledger with `answer Q1` followed by `002-rebase.md` emits and restores Q1 true while unanswered Q2 remains false; folding the full pre-checkpoint history and folding from the checkpoint produce identical Q classifications and source-map booleans; deleting or flipping either checkpoint value reds `verify-rebase`, deleting or flipping the generated source-map value reds `verify-generated`, and a second faithful checkpoint preserves both values | Test | G3, M3, FR-1, FR-3, FR-7, FR-35..FR-36, FR-42 |

### op: add UB

Reason: A merge gate that silently passes when its named entry is outside the replay window, or that discards all but the first ledger-read defect, turns an attribution problem into an invisible false green.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-4 | If a named added amendment or rebase checkpoint is not replayed at the checked revision | fail closed without a congruence verdict | a defect naming the missing entry, replay range, and remedy; never a silent pass | Must | high | absent entry and entry outside the replay window each red; no case returns green | Test | FR-2, FR-32, FR-35 |

### op: add UB

Reason: Collapsing ledger-read defects to the first reason destroys the artifact attribution the defect schema exists to preserve.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-5 | If the ledger prefix cannot be read without one or more defects | fail closed before congruence evaluation and preserve every underlying defect unchanged | one defect per cause with its original artifact path, operation, reason, and remedy | Must | high | a fixture with multiple read defects returns all defects in stable order; none is rewritten against the added amendment or reduced to reason-only text | Test | FR-2, FR-33 |

### op: add UB

Reason: A repair diagnostic must not direct an author to an operation the closed grammar rejects.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-6 | If an author applies a unit or section operation whose prefix, canonical-content delta, or immediately-before lineage or Q-answer state requires another repair under FR-39 | fail before mutation and select the authorized branch | a remedy naming rebase→unit operation, changed-lineage-less whole-section repair, or lineage-bearing `supersede <ID>` with a fresh same-prefix row; never a refresh that hides a changed lineage-bearing row or a replacement expressed as `retire` plus `add` | Must | high | `supersede R6`, `supersede M4`, and `supersede ABUSE-5` while lineage-less each red with their owning-section branch; a lineage-bearing ABUSE-1 changed inside section supersede reds with `supersede ABUSE-1` and a fresh ABUSE ID; `answer Q1` after a successful prior answer, including after checkpoint restoration, and direct `rescope NG-5` each red with the correct branch; every emitted remedy passes the next validation layer and a row referenced by a live DOD `Source` or live `Traces to` cell is never sent to `retire` | Test | FR-2, FR-32, FR-39, FR-41..FR-43, incident:I6 |

### op: add NR

Reason: The new checker is an automated consumer and must preserve the merged read-to-rule boundary even when routing tables deliberately contain requirement-looking identifiers.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-4 | §0 and all of its subsections remain human-only: no docgate contract-unit parser, fold comparison, source-map projection, or gate verdict treats an ID-looking row anywhere in that subtree as an addressable unit | read-to-rule routing prose binds nothing and automated consumers are contractually forbidden to read, cite, or obey it | high | the complete prd-template/v2 §0 fixture, including exact and slash-joined live IDs under §0 and §0.1, contributes zero units and cannot shadow a binding row with the same ID | Test | FR-32, FR-37, prd:read-to-rule/FR-5 |

### op: add DOD

Reason: The new hard gate is complete only when both its defect matrix and its adoption behavior are proven against the real corpus that motivated it.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-7 | Projection congruence ships with the full FR-32..FR-43/UB-4..UB-6/NR-4 matrix, exact-revision and working-tree conformance, every FR-41-addressable-prefix repair proof, checkpoint Q-state preservation, and the live Wiring rollup naming the verb | FR-32..FR-43, UB-4..UB-6, NR-4 | Test + Inspection | focused red/green tests; full race suite; `docs/techspec.md` Wiring diff; a committed sorted golden at `internal/docgate/gate/testdata/projection-congruence-baseline.json`, generated once by the implementation owner from the gate-activation merge base, carrying that immutable revision and ledger fingerprint plus every `(ledger, unit ID, mismatch class, field, projected value, source-map value)` record, independently reconstructed and byte-compared by a corpus audit whose omission and hidden-mismatch fixtures red; ordinary proposals never regenerate it, an explicit debt-repair proposal may delete only the exact repaired multiplicity, and a proven golden defect requires a dedicated reviewed gate-fix proposal while the verb is unwired | yes |

### op: supersede §1 Summary

Reason: The effective summary must name the new trust boundary and avoid the stale fixed verb count.

- **Problem** — Approved PRDs must evolve without trusting any editor's discipline: deterministic generation can still repeat a stale markdown projection and stale source map after an untyped whole-section row change, so artifact agreement needs its own merge-base guard.
- **For whom** — agents authoring amendments and rebase checkpoints, human reviewers of ledger PRs, and CI on every PR touching `docs/prds/`.
- **Why now** — PR #1090 reproduced a same-ID row mutation hidden inside a section supersede; the corpus audit also found historical projection/source-map debt that makes naïve hard adoption false-red.
- **Direction** — one static Go binary: a deterministic fold plus self-registering gate verbs, with projection congruence checking each new ledger boundary against an exact grandfathered baseline; the identical predicate runs in required CI and pre-push.
- **Appetite** — as header; stdlib-only, fixture-first, and no corpus rewrite as the price of adoption.
- **Success** — M1/M2 remain green and M3 records zero newly introduced mismatch accepted by the gate.
- **Guardrail** — no §0 routing row becomes a contract unit; no baseline debt blocks an unchanged amendment; no named repair dead-ends under the next gate.
- **Top risks** — parser inference reading non-binding tables; an ambiguous baseline comparator exchanging defects; a rebase repairing machine lineage while leaving human lineage stale; a partial corpus inventory presenting itself as complete.

### op: supersede §3.3 Guardrails and decision rule

Reason: The ship rule must measure congruence and baseline completeness, not deterministic bytes alone.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| hand-edited projection merged | any single occurrence | incident entry + gate-wiring fix before the next amendment merges |
| false-red rate on valid fixtures | >0 in the golden suite | fold or gate bug — fix before enforcement widens |
| §0 row classified as a contract unit | any | block release; repair parser scope and add the real document as a fixture |
| incomplete baseline-debt golden | any independently discovered omitted record | block release or unwire the gate; the implementation owner regenerates it only in a dedicated reviewed gate-fix proposal against the immutable activation fingerprint |
| named repair fails under the next enforcement layer | any | block release; make the repair executable before emitting it |

**Ship rule (amended):** the existing fold golden suite is 100% green, the complete FR-32..FR-43/UB-4..UB-6/NR-4 matrix is green, the independently reconstructed corpus inventory byte-equals its reviewed activation golden, and every FR-41-addressable-prefix repair plus checkpoint Q-state transition runs end to end → wire `check-projection-congruence` as a hard required check from first release. Any guardrail breach → do not wire, or unwire and repair before the next ledger entry merges. Historical keys in the reviewed golden are non-blocking only while identical; there is no advisory bypass for a new key.

### op: supersede §4.1 Approach and alternatives considered

Reason: The plan of record is a self-registering gate suite, not the base document's stale fixed count, and this amendment chooses exact baseline subtraction over corpus rewrite or advisory rollout.

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

### op: supersede §4.2 Scope perimeter

Reason: The effective scope must include the new verb, its adoption boundary, and its deliberately separate design step.

- **In (plan of record)** — every live docgate requirement; specifically for this amendment: `check-projection-congruence`, complete §0-subtree exclusion with existing merged-base row-kind inference and exact payload schemas, per-entry attribution, exact-revision reads, exact-value baseline multiset subtraction, checkpoint congruence, addressable-table-registry and checkpoint-carried Q-state boundaries, lineage-preserving repairs under the closed operation grammar, unchanged-row preservation, complete diagnostics, the reviewed activation-debt golden, required-context + pre-push wiring, and the live Wiring rollup.
- **Next governed result** — a superseding technical design for the fold/parser/checkpoint and section-supersede semantics serving FR-32..FR-43, UB-4..UB-6, and NR-4; it must explicitly supersede the amendment model's narrow `supersede` target list so every live FR-41-addressable ID may be superseded with a fresh same-prefix row except an FR-42-unanswered Q (use `answer`) and NG/WR (use `rescope`), preserve `retire` as removal rather than replacement with the FR-39 citation domain, and define the typed replay-base encoding, restoration, source-map projection, and verification of FR-43 `question_answered`. Implementation remains blocked until it is approved.
- **Future (believed next, not committed)** — WR-1 (additional design-class scope), WR-2, WR-3.
- **Out** — scanning pre-checkpoint history to reconstruct current Q state; registration or source-map governance of `F` flow rows; semantic duplicate detection; automatic mutation of frozen historical entries; an advisory bypass; a new operation verb; hand-authored exemptions or a corpus-wide mandatory repair campaign.

### op: supersede §4.3 Experience: flows and states

Reason: Authors need both the ordinary hard-gated path and an executable path for changing a grandfathered lineage-less unit.

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

### op: supersede §4.4 Scenarios

Reason: The motivating regression and the historical-debt repair need end-to-end product scenarios, not only isolated fixtures.

An agent applies review feedback by replacing a whole section and silently changes lineage-bearing ABUSE-1 under the same ID: the existing live state prevents refresh, the exact pushed-revision gate reds the actual inconsistent artifact and names the boundary, and `supersede ABUSE-1` with fresh ABUSE-9 is green while recording both ABUSE-1 `superseded_by=ABUSE-9` and ABUSE-9 `supersedes=ABUSE-1`. — An ordinary amendment restates a section containing a historical lineage-less R6 byte-for-byte: the fold preserves every projected byte and absent lineage record under FR-40, its exact baseline key subtracts away, and the amendment stays green. — The author must change lineage-less R6: `supersede R6` is rejected with the prefix-correct whole-section remedy, and only R6 receives new matching visible/machine lineage. — A Q row whose Status says `resolved (001)` but has neither a true replay-base value nor a successful post-base answer op is unanswered under FR-42 and routes to `answer`; after that answer, `002-rebase.md` materializes `question_answered: true`, a checkpoint-only replay restores answered state, and another answer reds with the fresh-Q supersede remedy. — The author must change lineage-less NG-5: the diagnostic names a semantic-no-op checkpoint followed by `rescope NG-5`; both proposals pass their own gates. — Workbench-ui and planner-driven-model-routing F1..F5 rows remain non-addressable under FR-41 and create neither source-map records nor repair obligations. — A prd-template/v2 §0 routing table names FR-24 and FR-19/FR-20/FR-26: it contributes zero units and cannot shadow the binding FR-24 row.

### op: supersede §5.5 Interfaces and integrations

Reason: The gate's checked-revision read and its ownership in both enforcement surfaces are contract interfaces.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| git (merge base + exact checked revision) | in | immutability and congruence boundaries | merge-base name-status identifies added ledger entries; exact tree reads supply the complete prefix; no shell interpolation | unresolved revision or missing entry → fail closed with original attribution | FR-4, FR-33, UB-4..UB-5 |
| PRD projection pair | in/out | the two faces whose agreement is gated | merged-base inferred rows outside the §0 subtree ↔ versioned source-map unit records, including required per-Q `question_answered`; FR-38 defines exact value equality; deterministic fold remains the sole writer | after-only mismatch → red; missing/altered Q state → red; historical identical key → grandfathered | FR-1, FR-3, FR-32, FR-34, FR-37, FR-38, FR-43 |
| rebase checkpoint machine state | in/out | complete replay-base semantics | each live Q carries required typed `question_answered`; `verify-rebase` compares it with the pre-checkpoint fold and replay restores it before later ops | missing, invalid, or unequal value → red; no fallback to Status or pre-checkpoint history | FR-7, FR-35..FR-36, FR-42..FR-43 |
| CI required context `docs-gate` | out | merge enforcement | the umbrella includes every registered applicable verb, including `check-projection-congruence`; exit code + human defects + `--defects-json` | red blocks merge; unwiring is rollback only after a guardrail breach | FR-2, FR-32..FR-43, UB-4..UB-6 |
| pre-push hook | out | desk-time verdict | the same umbrella binary and registered verb set over the exact pushed revision | AP-11: never a twin predicate; deletion-only behavior remains FR-29 | G2, G3, FR-33 |
| templates + addressable table-schema registry | in | addressable row kinds and op payloads | FR-41 registry membership defines addressability; payload headers remain exact-schema; merged base rows retain their existing inferred kind; §0 ancestry is excluded before row inference | malformed payload schema → red; lexical-only `F` and every §0-subtree row are non-addressable; cosmetic base-header drift remains governed | FR-5, FR-37, FR-41, NR-4 |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: M3 adds an independent audit feed whose retention and checkpoint must be explicit.

Runtime telemetry remains skipped: docgate is hermetic. M1/M2 continue to read CI and PR history. M3 reads two repository-verifiable feeds: required-check history for every ledger-entry PR after activation, and the committed activation-debt golden reconstructed independently by the corpus-audit test. The implementation owner generates that golden once from the exact merge-base revision where the verb is first wired; its revision and ledger fingerprint never move with later merges. Ordinary proposals never rewrite it. A proposal whose explicit purpose is to repair grandfathered debt deletes only the exact repaired multiplicity in the same change; no new or altered key may enter. A proven baseline defect requires the verb to be unwired and a dedicated reviewed gate-fix proposal to regenerate the record set from the unchanged activation revision. The first checkpoint is the first 10 gated ledger entries; any accepted new canonical mismatch is both an M3 failure and a gate incident, never an aggregate to discuss away.

### op: supersede §6.1 Phasing and milestones

Reason: Contract, design, fixtures, implementation, and hard wiring are distinct exit-gated phases for this follow-up.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Existing docgate | preserve the shipped fold and registered gate suite | all live requirements through amendment 017 | current corpus and gates remain green |
| Congruence contract | freeze the observable and adoption boundary | G3, M3, FR-32..FR-43, UB-4..UB-6, NR-4, DOD-7 | this amendment reviewed and operator-approved |
| Congruence design | decide parser refresh, §0-ancestry tracking, addressable-registry scope, section-repair annotation, unchanged-row preservation, checkpoint Q-state encoding/restoration/projection, citation-domain validation, artifact comparison, checkpoint compaction, defect attribution, and the lineage-preserving `supersede` target-domain expansion while preserving merged-base inference and implementing FR-38's fixed equality contract | the next governed design result | technical design review approved; no open DINV or migration question |
| Fixtures first | pin every mismatch transition, FR-41-addressable-prefix repair, unchanged-row invariant, pre-answer→answer→checkpoint→post-checkpoint Q-state transitions, and the complete corpus baseline before production code | FR-32..FR-43, UB-4..UB-6, NR-4 | reviewed golden + all red/green fixtures fail against the pre-change implementation for the intended reason |
| Implement and wire | add the self-registering verb, refresh model semantics as designed, update the living rollup | code + tests + `docs/techspec.md` | DOD-7 complete; full code gate and docs gate green; hard check active with no advisory mode |

### op: supersede §6.4 Release criteria

Reason: Release now needs an adoption oracle and repair proof in addition to the existing fold suite.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all live Must requirements, including the complete FR-32..FR-43/UB-4..UB-6/NR-4 matrix | hermetic fixture suite |
| Reliability | deterministic fold remains byte-identical; exact revision and working-tree paths conform | full race suite + conformance fixtures |
| Adoption safety | independently reconstructed debt set byte-equals the reviewed activation golden after permitted repair-only deletions; every after-only key reds from first release | corpus audit + required gate |
| Repairability | checkpoint compaction passes both gates; every FR-41-addressable formerly lineage-less row kind follows its authorized repair without a dangling target; Q routing is invariant to Status prose and checkpoint compaction; every live replacement preserves both source-map lineage links | end-to-end FR-39/FR-41/FR-42/FR-43 fixtures |
| Supportability | every defect preserves actual artifact attribution and names an executable remedy | defect catalog + UB-4..UB-6 fixtures |

### op: supersede §6.5 Rollout and reversibility

Reason: Existing debt permits hard launch only because exact baseline keys are grandfathered, and rollback must not become a bypass.

The gate launches hard once DOD-7 is complete: the reviewed activation golden and its immutable activation fingerprint are the adoption boundary, identical historical keys subtract away, and any after-only key blocks locally and in CI. Later merge bases do not regenerate the golden; only explicit repairs shrink it, and only a dedicated reviewed gate-fix while unwired corrects a proven oracle defect. There is no advisory phase and no corpus-wide repair prerequisite. A lineage-less row changes only through the prefix branch in F2 or F3. Reversibility is operational, not semantic: a proven false-red guardrail breach may temporarily unwire the registered verb from the umbrella while the code is repaired, but no exemption list or best-effort mode is added and every change merged during the gap is independently audited before rewiring.

### op: supersede §6.6 Launch readiness

Reason: The hard gate cannot launch until its upstream design, complete baseline, repair proof, and living rollup are present.

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

### op: supersede §6.7 Post-launch review

Reason: The existing checkpoint omits the congruence goal and independent false-green signal.

- **Review trigger** — 10 ledger-entry PRs through the hard congruence gate · **Owner** — @mdarabi
- **Evidence** — M1/M2 history, M3 required-check history, the independently reconstructed corpus inventory against the activation golden, false-red/false-green corrections, and every use of the F2/F3 repair paths.
- **Verdict recorded** — adopt / iterate / kill; any accepted after-only mismatch or non-executable remedy is a guardrail breach, not a thresholded metric.

### op: supersede §8 Contract readiness — the executability gate

Reason: The readiness proof must include the new goal, flows, Must rows, repair boundary, corpus oracle, and next-design dependency.

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
