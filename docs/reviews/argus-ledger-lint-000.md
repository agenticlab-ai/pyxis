# Review: prd:argus-ledger-lint/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approve (round 2): the contract is faithful to its frozen basis and the docgate composition it builds on; round 1's two blocking findings — a blocking DoD row depending on a `Should`-priority behavior (FND-1) and an undefined citation-recognition boundary that would false-red the arm's own citation-bearing corpus (FND-2) — were both applied by Argus at tier-2 and verified closed in round 2, with no regression in the changed spans. |
| **Relevant-when** | auditing or consuming the review of the argus-ledger-lint 000-base PRD; asking why it was bounced; tuning prd-review calibration against real coherence/completeness findings on a gate-tooling PRD |
| **Subject** | `prd:argus-ledger-lint/000` |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07 (`.pyxis/authoring/prd-review.md`) |
| **Date(s)** | 2026-07-11 |
| **Verdict** | approve |
| **Tracking** | introducing PR (filed on approval) |
| **Template version** | review-report/v1 |

## Round 1

Scope: the post-fold 000-base at `docs/prds/argus-ledger-lint/000-base.md` (projection `effective.md` + `effective.source.json` verified faithful — guidance italics stripped, generated header block added, all 43 units `live`, fingerprint `sha256:0688d3a3…`). The docgate gate suite is green; nothing it enforces is a finding. Basis: the frozen brainstorm record `docs/brainstorms/argus-ledger-lint.md` (gated `ready-for-prd`), Argus as decision principal — every `D-n` an attributed agent assumption, treated as the basis exactly as a human-authored record. Four lenses run in parallel, adversarial survival applied; discarded candidates leave no trace.

**Basis fidelity (upstream-alignment) — clean.** All nine live `D-1..D-9` carried; all seven `F-n`; all four `O-n → NG-1..NG-4`; both fog `G-1/G-2` kept as §7.2 open questions (Q2/Q1, non-blocking, correctly not resolved); all three parked `P-1..P-3` passed to the technical design, not resolved here; both candidate terms `T-1/T-2` routed as PR-time handoffs in §6.6. Every citation resolves (`check-immutable`, the `docgate gate` umbrella, distributed-check-registration's descriptor model, the `lint-*`/`internal/docgate/<class>` shape in docgate-v2, argus-artifacts DINV-1/2/6 + §3.8). No silent conflict with merged siblings: the `docgate` PRD (NG-1..4, FR-27) governs the fold of PRD ledgers — a different concern the ledger classes take no part in (NG-1 here: these classes take no fold); `incident-machinery` shares the docgate binary and the self-registration pattern but governs the incident registry, a disjoint class — it corroborates rather than collides. The D-9 Q-1-timing ruling is faithfully quoted and attributed (frozen basis, not re-litigated). No upstream-alignment (CAL-2) finding.

### FND-1 — a blocking DoD row hard-requires a `Should`-priority behavior (DoD/priority incoherence)

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §5.8 DOD-2 (Blocking = **yes**) requires "every per-DINV defect fixture (… **dangling `delivery:` anchor**) is red at exactly its arm" and sources itself to "FR-1..FR-4, **UB-2**" — while §5.2 classifies **UB-2 (the dangling/unresolvable-anchor detection) as priority `Should`**. §5.1's priority bar reasons explicitly about what "is not severable" (the Must set); a `Should` is by the doc's own grammar severable/preference. — vs — prd-template §5.8 ("Blocking is yes/no/n/a"; DoD rows aggregate by reference to the IDs they roll up) and Appendix A keyword discipline (`should` = preference, `shall` = requirement): a completion gate that blocks on a preference-tier row contradicts the row's own declared priority. A blocking DoD must roll up behaviors the doc treats as non-severable, or the priority must rise to match the gate. (The "as applicable" hedge in DOD-2 qualifies the unattributed-entry fixture, not the dangling-anchor clause, which is stated unconditionally.)
- **Acceptance condition**: reconcile the priority and the completion status — either raise UB-2 to `Must` (if dangling-anchor detection is genuinely release-blocking, which its inclusion in a blocking DoD asserts), or drop the dangling-anchor fixture from DOD-2's blocking clause (carrying it as a non-blocking DoD row or a `Should`-scoped acceptance) so no `Blocking = yes` row depends on a `Should`-priority requirement. Either resolution makes §5.8 and §5.1/§5.2 agree.
- **Disposition**: **applied** (Argus, authority-contract/v1 tier-2, `dispose-blocking-finding` exercised — 2026-07-11). Resolved by **raising UB-2 to `Must`**, not by dropping the dangling-anchor fixture, because dangling-anchor detection *is* genuinely release-blocking: once the citation-recognition boundary is defined (FND-2 / new FR-9), a `delivery:` reference the arm recognizes as an in-scope citation but that resolves to neither a journal `E-n` nor a derived-state field is an *unhandled classification branch* — a silent false-green on the cross-document arm's own predicate, exactly the false-green-misses-a-real-corruption class the §3.3 guardrail and R2 already treat as blocking. Dropping it would leave that hole uncovered; raising it makes the completeness of the arm's classification a Must, which its inclusion in a blocking DoD asserts. Changes: §5.2 UB-2 Priority `Should → Must`, its trigger and fit criterion re-scoped to an in-scope citation (per FR-9), Traces extended to `FR-3, FR-4, FR-9`; §5.1 priority-bar ranking prose updated so the non-severable set reads coherently; §5.8 DOD-2 source now `FR-1..FR-4, FR-9, UB-2` (all Must); §8's "every blocking DoD row traces" row now records that every DOD-2 source is Must-priority — no `Blocking = yes` row depends on a `Should`. §5.8 and §5.1/§5.2 now agree.

### FND-2 — the cross-document arm's citation-recognition boundary is undefined, so its own repo-wide corpus (this PRD, the record, the design, the glossary) is a false-red surface

- **Severity**: blocking (CAL-4 — load-bearing for §8)
- **Lens**: completeness-critic
- **Evidence**: FR-3/J2/§4.1/§5.5 scope the cross-document arm as a **repo-wide scan of "any document in the repo"** for `delivery:<slug>#` citations, classified as journal-`E-n` (legal, FR-4), derived-region (DINV-6 defect, FR-3), or unresolvable (UB-2 defect). But the arm's own governing corpus is saturated with `delivery:<slug>#`-shaped strings used as **prose and grammar illustration with a literal `<slug>` placeholder**, not real citations into any existing `docs/problems/<slug>/` ledger: this 000-base carries 21 such literals, and the frozen record and `argus-artifacts` §3.2/§3.4 use `delivery:<slug>#E-n` / `delivery:<slug>#E-k` illustratively (argus-artifacts §3.4: "anchors resolve by grep"). A naive repo-wide scan resolves every one to no ledger → UB-2's "report a defect," so the arm — run on the very repo that introduces it — would red-flag its own PRD, its own basis record, and the argus-artifacts design. — vs — §3.3 guardrail ("no legal ledger append and **no non-ledger tree is ever false-redded**"), NR-1 ("`docgate gate` output byte-identical … on all non-ledger trees"), and the §4.3 **Empty** state ("no `delivery:` citation present → both arms no-op") — none of which can be verified while "a `delivery:` citation" is undefined against citation-bearing non-ledger prose. §8's "every key flow has non-regression coverage" and "every primary metric has a measurement plan" rest on this unstated boundary. This is not docgate-enforced — citation-target legality for the ledger classes is precisely the new behavior this PRD introduces (confirmed absent from docgate-v1/v2; `doc-link-integrity` owns relative-link resolution only, per T-2's avoid-note).
- **Acceptance condition**: state, as observable behavior in §5, the boundary between a real `problem:`/`delivery:` grammar citation into an existing ledger (classifiable, in scope) and a literal-`<slug>` prose/grammar illustration or fixture mention (not a citation, out of scope) — and which documents the repo-wide scan includes; then add a non-regression fixture asserting the arm stays green on the repo's own citation-bearing prose (this PRD, the record, the design, the glossary). The guardrail and NR-1 then have a testable referent.
- **Disposition**: **applied** (Argus, authority-contract/v1 tier-2, `dispose-blocking-finding` exercised — 2026-07-11). Added **FR-9** (Must, high) stating the recognition boundary as observable behavior: a `delivery:` reference is an in-scope citation — a candidate for FR-3/FR-4/UB-2 classification — **only** when it is a `delivery:<concrete-slug>#<anchor>` reference whose `<concrete-slug>` names an existing `docs/problems/<concrete-slug>/delivery.md` ledger; a `delivery:` string naming no existing ledger directory (in particular a literal-`<slug>` angle-bracketed metavariable illustration, or a mention of an absent ledger) is **not** a citation and is left untouched. Grounded in argus-artifacts §3.4 ("anchors resolve by grep"; "a ruling's load-bearing spot cites its journal entry") — the boundary is target-resolvability into a real ledger, never string shape alone. The scan's corpus is **every tracked document in the repo**. FR-9's fit criterion is a fixture pairing a real citation into a seeded ledger with a literal-`<slug>` prose quote and a `delivery:no-such-slug#…` mention, plus the arm green on the repo's own citation-bearing prose (this PRD, the record, the design). Threaded through: §5.1 FR-3/FR-4 triggers now read "an in-scope `delivery:` citation (per FR-9)"; §5.2 UB-2 re-scoped to an in-scope citation; §5.5 repo-corpus interface row; §4.1 approach prose; §4.3 Flow F2 (new step 3 — non-resolving mentions no-op) and the **Empty** state (now "no *in-scope* citation present"); §4.4 scenarios (new scenario — the arm green on its own `delivery:<slug>#` illustrations); §1 Direction + Guardrail; §3.3 guardrails (new "citation-mention false-reds" row, threshold `> 0`); §6.1 Fixtures phase + §6.3 R7 + §6.4 Reliability row + §7.1 FAQ. The load-bearing non-regression the finding asked for lands in **DOD-2** ("the cross-document arm is green on the repo's own citation-bearing prose … whose literal-`<slug>` illustrations are not in-scope citations (FR-9)"). §3.3 guardrail, NR-1, and the §4.3 Empty state now have a testable referent; §8's key-flow and DoD-trace rows updated to cite FR-9.

## Round 2 (scope-locked)

Examined only the two round-1 dispositions against their acceptance conditions and regressions within the changed spans (UB-2 and its trace propagation; the new FR-9 and everywhere it was threaded — FR-3/FR-4, §5.1 priority bar, §5.2, §5.5, §4.1/§4.3/§4.4, §1, §3.3, §6.1/§6.3/§6.4, §7.1, §8, DOD-2). `docgate gate` green.

- **FND-1 — closed.** UB-2 raised `Should → Must` (§5.2), its trigger and fit re-scoped to an in-scope citation (per FR-9), Traces extended to `FR-3, FR-4, FR-9`; DOD-2's source is now `FR-1..FR-4, FR-9, UB-2` — every one Must — so no `Blocking = yes` row depends on a `Should`, and §5.1's priority bar now reads "every row is Must" with FR-9 ranked explicitly. §5.8 and §5.1/§5.2 agree. The acceptance condition's raise-to-Must path is satisfied.
- **FND-2 — closed.** FR-9 (Must) states the recognition boundary as observable behavior — an in-scope citation is a `delivery:<concrete-slug>#<anchor>` resolving into an existing `docs/problems/<concrete-slug>/delivery.md` ledger; a literal-`<slug>` illustration or an absent-ledger mention is not a citation — names the scan corpus (every tracked document), and its fit criterion plus DOD-2's clause carry the own-corpus non-regression fixture (the arm green on this PRD, the record, and the design). §3.3 gained a "citation-mention false-reds" guardrail; §1, NR-1, and the §4.3 Empty state now have a testable referent. All three parts of the acceptance condition are satisfied.

No regression in the changed spans: the FR-9 threading is internally consistent (every "in-scope citation (per FR-9)" reference resolves to the definition), the priority-bar ranking is coherent with the Must reclassification, and the gate is green. No new finding on the changed spans; a finding on unchanged content would be a scope violation and none was raised.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-11 |
| 2 | approve | 2026-07-11 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (terminal — request-changes → approve)
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation *(FND-1 + FND-2 both applied by Argus at authority-contract/v1 tier-2, exercising `dispose-blocking-finding` — 2026-07-11; verified closed at the scope-locked round 2, no regression)*

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
