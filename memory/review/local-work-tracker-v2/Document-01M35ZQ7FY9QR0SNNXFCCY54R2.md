# Review: local-work-tracker-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes — one blocking finding (the sole live store's cutover undecided; three lenses converged) + three advisories; 3 applied, 1 declined-with-reason; round 2: approve. Supersession integrity confirmed: carry-forward byte-faithful, right vehicle, no stranded dependents, no PRD amendment owed. |
| **Relevant-when** | auditing or consuming the review of design local-work-tracker-v2 (the `.workitems` default supersession); asking why it was approved; tuning tech-design-review calibration against real findings |
| **Subject** | design:local-work-tracker-v2 (post-lint draft; shape: none; supersedes local-work-tracker; declared machine-generated 35-line delta) |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` (2026-07-16) |
| **Date(s)** | 2026-07-16 (rounds 1 and 2) |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | design-review-report/v1 |

## Round 1

*Process note: four lenses ran in parallel over the declared delta (`diff` v1→v2, independently re-derived byte-identical by two lenses), the stage context pack, the served PRD projection, and the dependent live designs. Independent three-lens convergence on FND-1 (contract-quality from the prose, feasibility from the code paths, upstream from the deployment); coherence returned no candidates with a full verification log.*

### FND-1 — the sole live store's cutover is undecided; the delta paragraph's "existing stores are untouched" is false comfort

- **Severity**: blocking (CAL-2; feasibility independently escalated CAL-4 — the sole binding change had zero acceptance/deployment coverage)
- **Lens**: upstream-alignment *(independently: feasibility-completeness, contract-quality)*
- **Evidence**: "Existing stores are untouched: the `paths.work_items` config key … lets any operand pin `work-items/`" (§1 delta) — vs — the one store that exists (this repo's, seeded by PR #389, the delta's own cited ground) rides the code default with **no config pin**; after the flip, `list`/`ready`/`triage` read the new root as silently empty (the localfs absent-root read is `nil, nil` by carried design), and a marker-probe re-projection re-creates every item beside the orphaned store. The pin was named as capability, decided for no one — the RW-5 "silently in code" class.
- **Acceptance condition**: one owned ruling — decide the reference deployment's cutover and name the accepted residual for unpinned operands.
- **Disposition**: applied — §1 now rules the cutover (the flipping change renames this repo's store in the same landing sequence), names the silent-empty + duplicate-projection consequence explicitly, and assigns the pin-or-move as the upgrade act for any future default-riding operand.

### FND-2 — the dot-directory measurably weakens the carried "greppable with ordinary tools" property, unacknowledged

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: §3.11 carries "the files *are* the debugging surface… greppable" and the PRD's J4/G3 promise ordinary-tool access — vs — `rg`/`fd`/globs/Finder skip hidden directories by default; the delta's `.pyxis`/`.github` analogy elides that `.workitems` is the one dot-dir whose contents humans are promised to grep.
- **Acceptance condition**: one honest sentence naming the trade-off and its acceptance.
- **Disposition**: applied — §3.8 names the cost (hidden-skipping tools need `--hidden` or an explicit path; git tooling unaffected; the machine is the primary reader; A-8 read with the store path explicit).

### FND-3 — the flip's consumer set, sequencing, and induced sibling staleness are undocumented

- **Severity**: advisory (CAL-4 · CAL-3)
- **Lens**: feasibility-completeness *(the sibling-illustration half independently: contract-quality)*
- **Evidence**: the default resolves at one code point consumed by carina and, through it, plangate — stated nowhere; the frozen `plangate-carina-adapter` instructs `ls work-items/` (illustrative, non-binding — no supersession owed, but a live instruction now points at the v1 default).
- **Acceptance condition**: a short implementation note — single resolution point, what moves together, how frozen siblings' prose reads post-flip.
- **Disposition**: applied — §3.8's implementation note: one resolution point; the flip + this repo's store rename move together and nothing else; frozen siblings naming `work-items/` read as v1-era defaults.

### FND-4 — R6's loss-risk input shifts (dot-dirs read as disposable scratch) without a row update

- **Severity**: advisory (CAL-4, low; the lens itself marked it droppable)
- **Lens**: feasibility-completeness
- **Evidence**: R6 guards backlog loss; this repo has visible gitignored scratch such as `.pyxis/feedback-queue/`, so `.workitems` inherits a "safe to ignore/delete" prior — vs — no mitigation update or counter-signal decided.
- **Acceptance condition**: amend R6's mitigation or accept the residual explicitly.
- **Disposition**: declined — speculative human-behavior prediction rather than a code path; the store is self-describing at its root (`signals.md` names it operand content), git-versioning is R6's standing guard and remains the sole one, and keeping the carry-forward tight outweighs a taste-grade row edit. (Advisory decline; no escalation.)

## Round 2

*Scope-locked re-review of the revision: FND-1..3's acceptance conditions verified met at their anchors (the cutover ruling, the residual named with its duplicate-projection consequence, the dot-dir cost sentence, the implementation note); the changed spans introduce no new contradiction — the delta paragraph's "exactly one binding change" claim survives (the cutover ruling is delivery provenance, not a second binding design change; the §3.8 additions are cost/sequencing notes). Supersession-integrity verdicts from round 1 stand: carry-forward faithful, `worktracker-hierarchy-index`'s `stale-builds-on` flag adjudicated tolerable (no binding statement of it goes false — grep-verified root-agnostic), `plangate-carina-adapter` and `repo-layout-and-system-footprint` undisturbed, `prd:local-work-tracker` requires no amendment (no live row names the path). No new findings.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-16 |
| 2 | approve | 2026-07-16 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 3 advisory
- **Dispositions**: 3 applied · 1 declined (advisory, with reason) · 0 withheld-as-violation

## Overlay corrections

- **2026-07-16 · the implementing session (post-approve, found at the survival test):** the FND-1 residual's pin path interacts with A-9's survival property — a pinned non-default root loses addressability when `.pyxis/` is removed (the pin dies with the config; files survive, reads fall back to the default). All four lenses missed the interaction. Corrected in the still-open introducing PR: the residual now names it and ranks move-to-default as the durable upgrade form, pinning as transitional. The survival test itself now pins the v2 default explicitly, so its post-removal fallback exercises the same path it seeds.
- **2026-07-16 · the implementing session (post-approve, found at CI):** `check-content-paths` — born after v1 froze — rejects the carried literal footprint paths (eight sites incl. the Builds-on annotation, DINV-8, and A-9). The carry-forward was re-phrased to concept language ("the system footprint", "the footprint's closed config surface") with semantics identical; the delta paragraph now names this mechanical exception, so round 1's byte-faithful verification reads as true of the reviewed revision, superseded by the gate-forced rewording. Meta-lesson recorded: the desk verification that missed this had filtered the gate's output (`grep -v | head`) — gates are read raw from now on.
