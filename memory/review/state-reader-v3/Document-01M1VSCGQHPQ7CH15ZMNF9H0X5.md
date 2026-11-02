# Review: state-reader-v3 — design-review report

| Field | Value |
|---|---|
| **In one line** | Supersession review of state-reader-v3 over state-reader-v2 — the amendment-001 contract (absent/unreadable recovery split, machine-level bucket key) lands faithfully and the new DINV-8/A-10 close the EVT-4 hole, but four blocking gaps surfaced (an unrecorded live-dependent edge, an unspecified identity fallback, AP-16 omitted, and a §3.1 error clause naming an unreachable state) plus three advisories; all seven applied, approve. |
| **Relevant-when** | auditing or consuming the review of design:state-reader-v3; asking why the v2→v3 supersession was approved; tracing how prd:self-drive-wake amendment 001 and incident I7 reached the design layer; tuning tech-design-review calibration |
| **Subject** | design:state-reader-v3 · shape: none · supersedes: state-reader-v2 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-08-03 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Scope: the successor plus its declared target (target content is evidence, never the finding's object), against prd:self-drive-wake as amended by 001, the principles registry, and the incident registry. The v2→v3 delta is substantive, not lineage-only: §1 gains the failure account, §2's Decision gains the bucket's home/key/recovery clause, §3.3 restates DINV-6 and DINV-7 and adds DINV-8, §3.4 gains the EVT-4 clause, §3.8 is rewritten, §3.9 gains four bound cases, §4 gains five alternatives, §5 gains R6/R7, §6 gains A-10/A-11, §7 gains Q2, and Principles gains AP-5 and a sharpened AP-17. The integrity walk found every other binding element carried forward byte-identical, and the contradiction genuine — v2's DINV-6 ("corrupt or missing budget state starts conservative") is falsified outright by the amended prd:self-drive-wake §5.4, so whole-document supersession is the only legal shape (design:tech-design-conventions §2; incident I6).*

### FND-1 — the supersession retires a design a live dependent builds on, and records nothing about it

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (supersession integrity)
- **Evidence**: design:state-reader-v3 §2's supersession note claims only "Every other binding statement carries forward verbatim", saying nothing about dependents — vs — target anchor design:state-reader-v2 §3.4 ("the reader and budget accept an injected event hook"), which live design:wake-watch-v2 §3.4 cites by name ("the reader/budget event hook (state-reader-v2 §3.4) wires here"), and whose `Builds-on: state-reader-v2` edge the regenerated index flips to `stale-builds-on`. The lineage's own precedent cuts the other way and is unaddressed: review:state-reader-v2#Round-1 records "no dependent stranded — the intra-pair edge (wake-watch-v2 → Builds-on → state-reader-v2) supersedes in the same change." incident:I6's lesson states a stale-builds-on flag "is a planning blocker in waiting, never decoration."
- **Acceptance condition**: §2's supersession note records the dependent edge and its verdict — that no `**Decision:**` callout, `DINV-n`, or `A-n` row of wake-watch-v2 is falsified by this document (its §3.4 mention is a descriptive citation, and the reader port it depends on carries forward verbatim), so the conventions' supersession trigger is not met and the index flag is the intended surfaced review prompt rather than an unnoticed break.
- **Disposition**: applied

### FND-2 — the bucket key names an "authenticated identity" with no rule for when there isn't one

- **Severity**: blocking (CAL-4 — load-bearing for A-6)
- **Lens**: feasibility-completeness
- **Evidence**: design:state-reader-v3 §3.8 fixes the key as "one bucket per (medium host, authenticated identity)" and never says what keys the bucket when the identity cannot be resolved (unauthenticated call path, a token the medium will not resolve to a login, a transient identity lookup failure) — vs — the same section's own governing rule, "never partitioned finer than the platform documents its limit, and an undocumented scope is assumed coarser rather than narrower", and design:state-reader-v3 A-6's cross-checkout case, which cannot be written unambiguously without a stable resolvable key. An implementer filling the gap with a per-process or per-checkout substitute reinstates precisely the partition defect this document exists to remove (incident:I7).
- **Acceptance condition**: §3.8 states the fallback and binds its direction — an unresolvable identity keys the bucket by medium host alone (the coarser direction the section already mandates), never by a process, checkout, or repository substitute.
- **Disposition**: applied

### FND-3 — AP-16, the principle the defect most directly violated, is silently omitted

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (principles honesty)
- **Evidence**: design:state-reader-v3's Principles section cites AP-5, AP-17, AP-22 and AP-24 against the budget change but never names AP-16 — vs — AP-16 ("Nothing loops or skips silently. A safety skip that can repeat indefinitely must count, alarm, or self-heal after N occurrences"), which the failure this document repairs violated literally: per incident:I7, "the hook fired once, then a single one-hour timer slept", and "the contracted EVT-4 throttle_deferred event is never emitted". DINV-8 *is* AP-16's alarm clause; leaving the principle uncited is the cited-vs-actually-honored gap, and the design corpus's own rule is deviation-with-reason, never silent omission. Sibling design:wake-watch-v2 cites AP-16 for the same class of surface.
- **Acceptance condition**: Principles cites AP-16 as honored, anchored to DINV-8 and A-10 (the deferral counts and announces itself at every decision rather than sleeping silently).
- **Disposition**: applied

### FND-4 — §3.1's transport-fatal clause names a state §3.8 makes unreachable

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:state-reader-v3 §3.1 declares the top-level error "transport-fatal only (context cancelled, budget bucket unreadable *and* unrepairable)" — vs — design:state-reader-v3 §3.8 Recovery and #DINV-6, under which a present-but-unreadable bucket is *always* repaired by the conservative start and its replacement, so "unreadable and unrepairable" names no reachable state and leaves the genuinely fatal cases — the ones that defeat the bucket's substrate rather than its contents — unnamed. The predecessor's clause (target anchor design:state-reader-v2 §3.1, "budget file unreadable") was coherent under v2's single recovery rule; splitting that rule without revisiting the clause is what opened the gap.
- **Acceptance condition**: §3.1 names the substrate failures that are genuinely fatal — the state directory uncreatable, the lock unopenable, the replacement unwritable — and stops classifying unreadable *contents* as fatal, since §3.8 now recovers them.
- **Disposition**: applied

### FND-5 — §3.5 declares `none` while moving the bucket to shared ground without stating who may write it

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality (§3.5 substance-vs-`none` honesty)
- **Evidence**: design:state-reader-v3 §3.5 settles the question with "It is machine-local, created under the state directory's own permissions, and never transmitted" — vs — §3.8's "shared by every pyxis process on the machine" combined with #DINV-6's "present but unreadable ⇒ start conservative". Under v2 the bucket inherited a repository-private location; v3 moves it to a per-user machine path and specifies no ownership requirement, so on a shared host any local principal able to write that path can park every pyxis content write for a full hour-window simply by corrupting the file — the recovery rule is what converts a permissions gap into a denial of writes. `none` remains the right header verdict (no new data class, no egress), but the subsection reaches it without addressing the one property that carries the risk.
- **Acceptance condition**: §3.8 states the bucket's ownership/permission requirement — owner-only on both the directory and the file, as the ledger it replaces already used — and §3.5 rests its `none` on that rather than on an unstated inheritance.
- **Disposition**: applied

### FND-6 — Q2 poses as open a question shipped practice has already answered

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:state-reader-v3 §7 Q2 asks whether "the platform-conventional per-user state dir suffice[s]" for the bucket's home — vs — the convention this repository already ships and has shipped since before this document: `cmd/pyxis/logging.go:21` homes machine-level state at `${XDG_STATE_HOME:-$HOME/.local/state}/pyxis/…`. Leaving the root undecided invites a second state-dir authority invented at implementation time, which is the AP-11 twin-predicate seam this corpus repeatedly pays for.
- **Acceptance condition**: §3.8 names the existing convention as the bucket's root, and Q2 either closes or narrows to the genuinely open part (whether a `paths` override is later wanted), so no implementer chooses a root.
- **Disposition**: applied

### FND-7 — DINV-7's closing sentence binds the document rather than the implementation

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:state-reader-v3 #DINV-7 ends "the two are one claim stated at two altitudes, and a change to either that breaks the other is a defect in this document" — vs — the invariant's own first half ("its absence costs no writes and no delay"), which is property-testable and tested by A-6. A `DINV-n` states a property the implementation must hold; a clause whose subject is this document's internal consistency cannot be property-tested and dilutes the binding element it rides on. The coupling it describes is real and worth stating — in §3.8's prose, where explanatory content belongs.
- **Acceptance condition**: DINV-7 keeps only implementation-testable content; the design/document coupling moves to §3.8 prose.
- **Disposition**: applied

**Verdict: approve** — all four blocking findings and all three advisories applied in the subject before the verdict; the applied revisions were re-checked against `lint-tech-designs`, the full `docgate gate --merge-base origin/main`, and the regenerated index.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-08-03 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 4 blocking · 3 advisory
- **Dispositions**: 7 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
