# Review: argus-artifacts — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories at round 2 — all seven round-1 blockings closed in substance (writer discipline, entry-field contracts, never-list arms, mutability posture, principles re-attribution) and all five advisories taken; two narrow round-2 advisories remain on the changed spans (the change-history entry vocabulary + EVT-5's `tier`, and the delivery status enum's home), neither impairing a binding element. |
| **Relevant-when** | auditing or consuming the review of argus-artifacts; asking why it was bounced at round 1; tuning tech-design-review calibration |
| **Subject** | design:argus-artifacts · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide `.pyxis/authoring/tech-design-review.md` as of 2026-07-11 · **advisory period** (verdicts block nothing; findings accumulate as measurement) |
| **Date(s)** | 2026-07-11 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (unfiled at review time) |
| **Template version** | design-review-report/v1 |

## Round 1

*Review basis: post-lint draft, shape none (every evidence quote below is `design:`). Record-fed — basis record `design-brainstorm:argus-tech-design` (gated ready-for-design, draft riding this PR) read as context; its frozen predecessors respected as rulings. Upstream corpus: prd:argus effective (as of 000), architecture-principles, incidents I1–I3, the design index and the Builds-on neighbors (brainstorm-record-model · design-brainstorm-record-model · prd-review-artifacts · tech-design-review-artifacts · plan-new-v1 · distributed-check-registration). Non-findings, recorded once: the Scope's exclusion of drive behavior is the record's D-1 partition (ruled, honored); the `ALT-n` ledger is carried faithfully and every §4 row is verdicted with the record's reasons; the `A-n` anchor family is record D-4's ruling and structurally disambiguated by the mandatory `problem:` namespace; materialization ownership (plan-new-v1's ground) is honored by exclusion.*

### FND-1 — problem.md's mutable status cell contradicts append-only-forever

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "a closed header (slug · … · status: `open | parked | delivered`)" and "the file is append-only forever — answers are permanent" (§3.1); design: "A problem ledger is append-only forever" (#DINV-1) — vs — the status enum's own transitions (open → parked → delivered), which require in-place mutation of that file; §4's ALT-1 rejection ("in-place mutation erases history") names the exact hazard.
- **Acceptance condition**: reconcile the two — either drop the status cell from `problem.md` (park/delivery state already lives in `delivery.md`'s journal-derived block, DINV-6's one sanctioned regenerable surface), or explicitly scope the append-only rule (Idea + `A-n` immutable; the status cell named as a regenerable projection with the journal authoritative) — stated in §3.1 and mirrored in DINV-1.
- **Disposition**: applied — problem.md's header reduced to intake-time facts only, no mutable cell; status lives on the delivery side's derived block; DINV-1 stated absolute

### FND-2 — the `A-n` entry contract omits the attribution field DINV-3 mandates

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: `A-n` entries are "dated, attributed to its human author, and (for answers) citing the brief it answers" (§3.1) — vs — design: "Every `A-n` and `E-n` carries its principal identity and authority-contract version; an unattributed entry is invalid" (#DINV-3) and A-2's "every DINV-3 field present". §3.2 binds the pair explicitly for `E-n` ("principal identity + authority-contract version (FR-12)"); §3.1 never binds the contract version for `A-n` — and the header's version is "at intake", stale after any mid-problem tier change.
- **Acceptance condition**: §3.1's `A-n` entry contract enumerates the full DINV-3 field set (principal identity + the authority-contract version current at the ruling), matching §3.2's form.
- **Disposition**: applied — §3.1's A-n contract now carries the authority-contract version current at recording (with date, author attribution, and brief citation for answers)

### FND-3 — the checkpoint evidence block's writer is unreconciled with the file's write prohibition

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: the walk "appends its output as the checkpoint's evidence block in the authority contract's change history … In the bootstrap the walk is performed by the implementing session" (§3.4) — vs — design: the never-list's "amending this file or any grant" (§3.3) and prd:argus/UB-1 + §5.3 ("Amend the authority contract … Argus: never"). Ten-problem checkpoints occur in Phase-3 graduated operation, still bootstrap-form — the performing session is then an Argus session writing the one file it must never amend. Secondary gap: §3.3's history entries are change-scoped ("date, direction (promotion · demotion · revocation), and the evidence refs"), so a checkpoint with no tier move has no declared landing spot for its evidence block.
- **Acceptance condition**: §3.3/§3.4 name the evidence block's writer and write path reconciled with the prohibition (e.g. the walk computes; the block lands only inside a human-authored data PR, or "append checkpoint evidence" is an explicitly carved non-amending act with the tier fields human-only), and state where a no-move checkpoint's evidence lands given the direction-scoped entry contract.
- **Disposition**: applied — writer discipline stated in §3.3: Argus never writes the file; the walk prepares the evidence block, a human lands it by PR; the change history admits `checkpoint` entries with `move: promotion | demotion | none`, giving no-move checkpoints a landing spot

### FND-4 — the PRD's typed EVT properties have no bound home in the entry contracts

- **Severity**: blocking (CAL-4 — escalated: the gap is load-bearing for A-5 and the DOD-5/FR-13/FR-14 feeds it serves)
- **Lens**: feasibility-completeness
- **Evidence**: design: entries carry "type, date, principal identity + authority-contract version (FR-12), a gist body, and … the external-effect identity" (§3.2); history entries carry "date, direction …, and the evidence refs" (§3.3) — vs — prd:argus §5.7's typed properties: EVT-5's `tier` (absent from the history-entry fields), EVT-2's `class (override · deadlock)` (unbound on `brief-emitted`), EVT-3's `touches count, breaking count, spend` (only spend is implied, via Q-2), EVT-4's `channel` and `missed-breaking?` (unbound on the reverse-citation convention — and missed-breaking is the §3.3 zero-tolerance guardrail's datum). Also unbound: `delivery.md`'s header status vocabulary (§3.2 says only "slug · status"). A-5's walk must compute M1–M3 from exactly these fields.
- **Acceptance condition**: each EVT row's typed properties get a named home in the §3.1–§3.4 contracts — a bound entry field, or an explicitly stated derivation rule over the journal (legal for touches/breaking counts; `tier`, `class`, `channel`, `missed-breaking?` are not derivable) — and the delivery header's status enum is closed.
- **Disposition**: applied — typed event properties bound: brief-emitted carries `class: override | deadlock`; delivered carries touch count, breaking count, spend; the overturn citation carries `channel` and `missed-breaking`; EVT-5 maps to the checkpoint change-history entry with move + evidence refs

### FND-5 — the never-list enumeration and A-6's fixture families drop UB-1's un-park arm

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "the structural refusals — amending this file or any grant, authoring governance-tier content, overriding a human ruling, an incident lesson, or a breaking-call answer, exceeding the problem's effective envelope, crossing ungranted outward surfaces" (§3.3) and A-6's "(self-grant, governance authoring, human-ruling override, envelope breach, ungranted outward act)" — vs — prd:argus/UB-1: "amend Argus's own authority contract, promote a tier, **or un-park an escalation Argus raised**". The third arm appears in neither the frozen enumeration nor the verification families, so a data edit minting an `unpark-own-escalation` class is a never-class act A-6 never probes.
- **Acceptance condition**: §3.3's enumeration and A-6's fixture-family list carry the un-park-own-escalation refusal (or §3.3 states the enumeration is a projection with UB-1..UB-5 governing in full — and A-6 still gains the family).
- **Disposition**: applied — the never-list enumeration and A-6 now include un-parking an escalation Argus itself raised (and tier self-promotion, made explicit)

### FND-6 — the classes' mutability/governance posture against the house taxonomy is unstated (AP-7 silently omitted)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "append-only forever" (§3.1), "the journal closes … and the file freezes" (§3.2) — novel semantics: both instances mutate by append *across merges*, unlike every existing class's merge-is-freezing rule — vs — AP-7 (document mutability classes, uncited in Principles) and the sibling precedent (brainstorm-record-model: "a provenance artifact, not a governed document kind", with an explicit AP-7-honored-by-extension row and a boundaries block). Nothing states the classes' taxonomy position, that docgate's verbs and gate output stay unchanged on trees carrying `docs/problems/` instances, that the fold/amendment machinery must not be inferred to apply, or where append-only/freeze is enforced pre-Q-1 (prompt + review, per the record classes' staging).
- **Acceptance condition**: a boundaries statement (the template's four prompts are the natural home) binds the posture — provenance/data class outside the governed kinds, docgate-neutrality, no fold/amendment machinery, enforcement home until Q-1's lint — and the Principles section gains an AP-7 row reasoning the append-after-merge semantics.
- **Disposition**: applied — §3.8 states the novel mutability class (registry-style living documents; entries freeze, files grow; the whole journal freezes at close), cites AP-7 at entry grain, and names the docgate boundaries (no fold, not ledger-class, check-immutable must not flag appends; templates' checklists + review guard until Q-1's lint)

### FND-7 — Principles rows: the AP-5 row carries AP-6's substance; RW-5 silently omitted

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "**AP-5 — honored.** Closed type vocabularies, pinned template versions, additive evolution by version bump" (Principles) — vs — AP-5's actual content (monitoring/tracing, end-to-end traceability) and AP-6's ("everything passed around is contractual … additive evolution"): the justification is AP-6's, filed under AP-5, while the design's genuine AP-5 contribution (the `problem:`/`delivery:` grammars extending the traceability chain — the claim every sibling design makes) goes unstated. RW-5 (one decision per document) is silently omitted although the document binds three independently-bumpable contracts (§3.8: evolution "decided by a successor design" per class) and the reviewer-artifact precedent split per class — the defense exists (record D-1 assigns all three to this document) but must be cited, not implied; the conventions rule "silent omission is not" legitimate.
- **Acceptance condition**: re-attribute the row to AP-6; add an AP-5 row for the grammars' traceability edge (or drop the claim); add an RW-5 row citing D-1's data/behavior partition as the one-decision reasoning (honored-via-ruling or deviation-with-reason).
- **Disposition**: applied — Principles corrected: AP-6 carries the contractual-data clause; AP-5 restated as the journal-as-event-surface; AP-7 and RW-5 added, RW-5 citing the record's D-1 partition

### FND-8 — the action-class tokens are undefined and the grant check's domain is half-open

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "A grant check is one lookup: is the act's class within the current tier's bundle?" (§3.3) with the bundles as bare tokens — vs — prd:argus/FR-11 ("where an action class is not granted, Argus shall escalate instead of acting"), §5.3 ("Rule at stages (answers, dispositions, gate verdicts) … per graduated list"), FR-4's unconditional assumption-ruling, and record D-5 ("the full action-class vocabulary the drive exercises"). The contract never defines what each class covers, never states the default for an act in no bundle (FR-11's escalate), and never draws the line between grant-checked action classes and the drive's inherent acts (assumption-recording, brief emission, ledger appends) — leaving §5.3's "answers … per graduated list" unresolvable against a ladder with no such class.
- **Acceptance condition**: §3.3 defines each action class in one line, states the not-in-any-bundle default by citing FR-11, and states which drive acts are grant-checked classes versus inherent behavior (with the act→class judgment explicitly the agent design's).
- **Disposition**: applied — §3.3 defines every action-class token, states the fail-closed default (a class in no bundle is not grantable — escalates; new classes are successor-design decisions), and draws the grant-checked vs inherent-drive line

### FND-9 — trigger-materialization's tier-2 placement is unreasoned

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: tier-2 = "+ dispose-blocking-finding · sign-gate-verdict · trigger-materialization" (§3.3) — vs — prd:argus §6.5's ruled stage-2 row ("+ blocking dispositions, gate verdicts") and record D-5, which binds enumeration of the class but not its placement. A promotion to tier-2 now grants more than the §6.5 row the operator ruled; the widening is legitimate design ground but carries no stated reason (tier-4 gets its NG-1 caveat; this cell gets nothing).
- **Acceptance condition**: one stated reason for the tier-2 placement in §3.3 (e.g. materialization is consent-guarded by the planning stage's own contract, so it rides the first unmanned-drive tier) — or a re-placement with reason.
- **Disposition**: applied — the tier-2 placement of trigger-materialization now carries its reason (plan-new's consent/idempotency contract bounds the act; items closable — reversible at feature scope; §6.5 named decision-class headlines only)

### FND-10 — class-contract completeness short of the record-class precedent: naming, checklists, defect fixtures

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "the classes' structure checklists at review time" (§3.6) and A-4's "rejected at review" lean on checklists no contract binds; §3.1's `docs/problems/<slug>/` carries no slug rules (case, permanence, reopen/collision — contrast brainstorm-record-model §3.1); acceptance has no seeded-defect matrix (contrast every sibling's A-2: one red fixture per invalid class) — DINV-1's edited-`A-n`, DINV-2's second `delivered`, DINV-3's unattributed entry, DINV-6's cited block have no fixture rows.
- **Acceptance condition**: the three template contracts bind slug/naming rules and a structure-checklist appendix each (§3.8/A-1's shipping list names them), and acceptance gains a sibling-style seeded-defect row (one red per DINV family).
- **Disposition**: applied — slug rules stated in §3.1 (kebab, minted at intake, collision-suffixed); templates ship with structure-checklist appendices (§3.8); A-7 added: per-DINV defect fixtures red at exactly their rows

### FND-11 — the overturn reverse-citation duty has no named owner

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "the overturning artifact carries `overturns delivery:<slug>#E-n`, discoverable by reverse citation" (§3.2, §3.4's EVT-4 row) — stated declaratively, binding no author — vs — prd:argus/FR-13 ("Argus shall record the overturn as measurement data") and F3's overturn flow: overturning artifacts arrive through normal channels, often human-authored with no live session, and an omitted citation makes the overturn invisible to the walk (M2/M3 silently undercount).
- **Acceptance condition**: §3.4's binding rule gains the overturn direction as an explicit obligation on the overturning artifact's author (symmetric with the assumption-spot rule), and names the backstop owner (the agent design's audit behavior or the seam amendments) for citations missed at source.
- **Disposition**: applied — the overturn duty binds the overturning change's author (seam/audit conventions prompt for the citation), and the checkpoint walk's sweep is the named backstop — a missed citation delays measurement by at most one checkpoint

### FND-12 — the grammar-set disjointness enumeration is volatile and already incomplete

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "disjoint from `brainstorm:` · `design-brainstorm:` · `prd:` · `design:`; anchors resolve by grep" (§3.4) — vs — the live namespaces it omits (`doc:` · `corpus:` from the conventions' Serves grammar; `review:` from the reviewer-artifact designs) and the template's rule that frozen prose states no volatile facts — "name the concept and point at the live surface that owns the current value". R3's own verification already points at the right owner ("inspection against the glossary's grammar set").
- **Acceptance condition**: the §3.4 parenthetical claims disjointness against the house grammar set as owned by its live surface (the glossary's set) instead of a frozen partial enumeration.
- **Disposition**: applied — the volatile namespace enumeration replaced by pointing at the glossary's grammar set as the owning surface

## Round 2 *(scope-locked: the twelve named defects + regressions within changed spans; the cap round)*

*Re-review basis: the revised draft (gate suite re-run green, index regenerated; shape still none), each disposition verified against its finding's stated acceptance condition. The signed-off changed spans examined for regressions: §3.1–§3.4, §3.8, §6 (A-6/A-7), Principles. Unchanged content not re-examined (protocol). Below-threshold candidates killed in adversarial survival: FND-8's missing literal FR-11 citation (the stated rule reproduces FR-11's content verbatim — a cite-format nit) and its unassigned act→class judgment ownership (the grant-checked line plus the Scope's behavior partition already place it with the agent design).*

**Closure verification:**

- **FND-1 — closed.** The status cell is gone; §3.1's header holds intake-time facts only, delivery's derived block owns status, and DINV-1 is stated absolute — the acceptance condition's first branch, taken cleanly.
- **FND-2 — closed.** The `A-n` contract now enumerates the full DINV-3 set: dated, human-author attribution (the principal), and the authority-contract version current at recording — matching §3.2's form.
- **FND-3 — closed.** Writer discipline stated: Argus never writes the file; the walk prepares, a human lands by PR; `checkpoint` entries with `move: promotion | demotion | none` give no-move checkpoints their landing spot. (A residual vocabulary drift this introduced is FND-13.)
- **FND-4 — closed in substance; residue re-scoped.** `class` on `brief-emitted`, counts + spend on `delivered`, `channel`/`missed-breaking` on the overturn citation, and the `A-n` cite on `answer-received` are all bound — A-5's walk computes M1–M3 from bound fields, which was this finding's blocking rationale (CAL-4's load-bearing condition), now satisfied. Two narrow residues remain — EVT-5's `tier` (neither bound nor derivation-stated) and the delivery status enum (never closed, home now split between the §3.2 header and §3.1's derived-block claim). Neither is read by M1–M3 nor impairs any A-row's decidability, so per CAL-3/CAL-4's own severity logic the residue lands advisory, not blocking: FND-13 and FND-14.
- **FND-5 — closed.** The un-park arm (and tier self-promotion, made explicit) now appears in both the §3.3 enumeration and A-6's family list.
- **FND-6 — closed.** §3.8 states the registry-style living-document posture (entries freeze, files grow, the journal freezes whole at close), applies AP-7 at entry grain with a Principles row, and names the docgate boundaries (no fold, not ledger-class, `check-immutable` must not flag appends, checklists + review as the guard until Q-1's lint).
- **FND-7 — closed.** The AP-6 row carries the contractual-data clause; the AP-5 row now claims genuinely AP-5 substance (the journal as the drive's event/observability surface); RW-5 added, citing the record's D-1 partition as the one-decision reasoning.
- **FND-8 — closed.** Every action-class token defined in one line; the fail-closed default stated (a class in no bundle is not grantable — escalates; new classes are successor-design decisions); the grant-checked vs inherent-drive line drawn.
- **FND-9 — closed.** The tier-2 placement carries its reason (plan-new's consent/idempotency contract bounds the act; items closable; §6.5 named decision-class headlines only).
- **FND-10 — closed.** Slug rules in §3.1 (kebab, intake-minted, collision-suffixed); structure-checklist appendices bound to all three templates in §3.8; A-7 adds the per-DINV seeded-defect row with one red fixture per family.
- **FND-11 — closed.** The overturn citation duty binds the overturning change's author, and the checkpoint walk's sweep is the named backstop — a missed citation delays measurement by at most one checkpoint.
- **FND-12 — closed.** The disjointness claim now points at the glossary's grammar set as the owning surface; no frozen enumeration remains.

### FND-13 — the change-history entry contract speaks two vocabularies, and EVT-5's `tier` is still unbound

- **Severity**: advisory (CAL-3 — no binding element goes untestable: A-4/A-5 remain decidable, M1–M3 read none of these fields)
- **Lens**: contract-quality
- **Evidence**: design: "entries record date, direction (promotion · demotion · revocation), and the evidence refs" (§3.3, the Decision paragraph) — vs — design: "`checkpoint` entries with `move: promotion | demotion | none`" (§3.3, writer discipline) and the EVT-5 row (§3.4): the same section states two field names (`direction` / `move`) with two different closed sets (`revocation` in one, `none` in the other) for one entry contract — a closed-parse file (AP-6) whose parse vocabulary must be guessed. And prd:argus §5.7's EVT-5 property `tier` still has no bound field and no stated derivation (it is replay-derivable from the initial tier plus the move sequence, but the contract never says so) — the unmet half of FND-4's acceptance condition.
- **Acceptance condition**: one entry contract for the change history — a single field name with one closed set (revocation and `none` both placed), and `tier` either a bound field on the entry or an explicitly stated replay-derivation rule.
- **Disposition**: applied — one closed vocabulary: change-history entries are `checkpoint` entries with `move: promotion | demotion | revocation | none` and the resulting `tier` (EVT-5's binding); the two other mentions now reference §3.3's vocabulary instead of re-enumerating

### FND-14 — the delivery status field: enum never closed, home stated twice

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "a closed header (slug · status)" (§3.2) with no enumeration anywhere of status's values — vs — AP-6's closed-vocabulary rule and the acceptance condition FND-4 carried ("the delivery header's status enum is closed"); compounded by design: "status lives on the delivery side, where the derived block owns it" (§3.1) — the header cell and the derived block are both presented as status's home, and DINV-6 forbids the derived block being authoritative.
- **Acceptance condition**: §3.2 closes the status enum in place and names its single authoritative home (the header cell as the coarse enum with the derived block carrying detail is one legal shape; dropping the header cell for the block is another — DINV-6 requires the authoritative one be journal-derived either way).
- **Disposition**: applied — status removed from the delivery header (slug only); the derived state block owns it with the one closed enum `open | parked | delivered`, derived from the journal — consistent with DINV-6

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-11 |
| 2 | approve-with-advisories | 2026-07-11 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (the cap; terminal)
- **Findings**: round 1 — 7 blocking · 5 advisory; round 2 — 2 advisory (changed spans; one is FND-4's re-scoped residue)
- **Dispositions**: 12 applied · 0 declined · 0 withheld-as-violation; FND-13/FND-14 pending (advisories — the author's to take or leave)
- **Closure**: 12/12 round-1 findings closed against their stated acceptance conditions (FND-4 in substance, residue re-scoped at its own calibrated severity)

## Overlay corrections

none
