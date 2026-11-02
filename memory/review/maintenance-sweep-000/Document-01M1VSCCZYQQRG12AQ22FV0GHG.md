# Review: prd:maintenance-sweep/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories at the round-2 cap — round 1's four blockers (the FR-6/EVT-1 determinism contradiction, the over-claimed "3/3 historical" replay bar, out-of-contract DOD-6, incident I3's uncarried misparse lesson) and five advisories all verified applied on the revised base; two new advisories ship on stale cross-references the revision left behind (§6.5's struck-DOD-6 range; UB-5 absent from §6.4's and §8's enumerations). |
| **Relevant-when** | auditing or consuming the review of the maintenance-sweep PRD; asking why its 000-base was bounced at round 1 and approved with advisories at round 2; tuning prd-review calibration against a T2 read-only-tooling finding set |
| **Subject** | prd:maintenance-sweep/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide as of 2026-07-19 (`.pyxis/authoring/prd-review.md`) |
| **Date(s)** | 2026-07-19 (round 1) · 2026-07-19 (round 2) |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Floor excluded per protocol step 2: the docgate gate suite is green on this tree both base-free (`docgate gate`, exit 0) and `--merge-base origin/main` (exit 0) — nothing it enforces (ledger structure, projection integrity, typed-citation resolution, links, governing purity) is raised below. The subject's §0 was not routed by, and no finding below cites into or judges it (FR-17). Context assembled via `docgate context-pack --stage prd-review --subject maintenance-sweep`; the report template, artifact contract, basis record, problem ledgers, and PRD template were read directly. **Basis fidelity holds**: the record's live decisions D-1..D-7 are all carried (D-1→§4.1 A / NG-2 / decision-log r1; D-2→FR-2..FR-4 / FR-5 / decision-log r2; D-3→FR-1 / FR-6 / FR-8 / NG-1 / NR-1; D-4→FR-7 / F2; D-5→§2.2 incl. anti-segment; D-6→M1+M2 / DOD-2..DOD-3 — but see FND-2 on the PRD's extension of its replay bar; D-7→header Appetite / NG-4), fog is kept typed (G-1..G-3→WR-1..WR-3 / Q1..Q3), O-1..O-6 are honored (O-4 via WR-1), T-1..T-3→DOD-6, and P-1..P-5 are routed unresolved (§6.2, Q4, F1/F2 design pointers). The authority anchors resolve and are not re-judged: `problem:maintenance-sweep#A-1` (operator-authored intake pre-ruling, verbatim delegation), `delivery:maintenance-sweep#E-1..E-3` (E-3 records the fresh `sign-gate-verdict` grant check), and the tier-2 bundle at the authority contract (`.pyxis/skills/argus/authority-contract.md`, 2026-07-11 promotion) does contain `sign-gate-verdict`. Sibling/upstream spot-checks verify: docgate FR-10 retired by 017 exactly as §2.4 claims; `wake-channel-v1` §3.2 is the bounded-pass/nothing-resident pattern F-6 cites; the plan template's Touches line reads verbatim "plan-internal surface list — used for overlap sequencing, never rendered to workers" (FR-3's advisory phrasing is well-grounded); frozen plans really live at `docs/plans/<slug>/NNN-plan.json`; `internal/verify/checks/` and `internal/docgate/gate/` exist as §2.4's disjointness evidence says. Noted strengths, not findings: NG-2 honors amendment 017's ruling rather than re-fighting it; checkpoints throughout are observable exposure conditions, never dates; NFR-2 states the steady-state cost (AP-19); the anti-segment and the 017 moment-disjointness are held consistently across §2.2/§3.4/§4.1/§7.1. Four lenses ran in parallel; each survivor below cleared independent scrutiny; discarded candidates left no trace.*

### FND-1 — The determinism contract is unsatisfiable as written: FR-6 demands a byte-identical machine face that EVT-1 requires to carry wall-clock duration

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-6 (§5.1, Must) — "The sweep shall produce byte-identical output and identical exit codes for identical repository state and configuration", fit criterion "two consecutive runs on a pinned fixture produce byte-identical machine-face output" — vs — EVT-1 (§5.7), whose typed properties include "duration: duration" and whose Source is "the sweep's machine face (the report is the event record; no second telemetry plane)". Wall-clock duration varies across two consecutive runs regardless of repository state and configuration, so the one artifact cannot simultaneously satisfy FR-6's byte-identity fit and carry EVT-1's duration property; a builder must violate one row to satisfy the other, and M1..M3 all feed from EVT-1. This is the "contradictory" class the contract-readiness posture (§8, mirroring pipeline stage 6) exists to bounce.
- **Acceptance condition**: Reconcile the two rows explicitly — e.g. scope FR-6's byte-identity to the findings content of the machine face with volatile timing fields named and excluded from the compared bytes, or drop `duration` from EVT-1's machine-face properties (stating where duration lives instead), or pin duration deterministic under the fixture harness as part of the fit criterion — so that one stated contract satisfies both rows.
- **Disposition**: applied — the machine face is now fully deterministic: FR-6 restated (byte-identical machine face; volatile wall-clock fields carry on the human face only) with the fit criterion extended ("the machine face contains no wall-clock field"), and EVT-1's properties drop `duration` from the machine face, naming the human face as its only home — one stated contract satisfies both rows. (delivery:maintenance-sweep#E-5)

### FND-2 — The replay bar claims three historical rot instances; the PRD's own evidence supplies only two for the shipped detectors — the third attested instance belongs to the parked map-drift class

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M1 (primary): "historical rot instances detected on pinned replay fixtures — Baseline 0/3 — Target 3/3"; §1 Success: "M1: 3/3 historical rot instances detected on replay fixtures"; DOD-2 (blocking): "all three historical rot instances detected on their pinned fixtures (M1 = 3/3) — Source §3.2 M1; FR-2..FR-4" — vs — the PRD's own §2.4 row 1, whose three attested instances are "a feature's full implementation landed unwired; plan Touches named surfaces that never shipped; **map rows lagged for days**" — and the third of these is pipeline-map row drift, the class the PRD itself parks (WR-1, from record G-1/O-4) with no v1 detector. No historical living-rollup staleness instance appears anywhere in §2.4, and FR-4's own fit criterion describes a constructed fixture ("on a fixture where structure shipped after the rollup's last touch beyond the bound"), not a historical replay. Upstream agrees: the record's D-6(a) pins exactly two historical replay families (lane/wake unwired; the T4/T8 Touches gap) — this finding disputes no recorded decision; it targets the PRD's extension of that bar to "3/3 historical", which is unsatisfiable (if instance 3 is the map instance, no shipped detector can detect it) or mislabeled (if instance 3 is FR-4's constructed fixture, it is not historical). Either way the primary metric's denominator does not decompose against the PRD's own evidence and detector set.
- **Acceptance condition**: Make the replay bar exact and satisfiable: either restate M1/§1/DOD-2 as the two attested historical replay families (FR-2, FR-3) plus a named constructed staleness fixture for FR-4 (adjusting the 0/3→3/3 arithmetic and the word "historical" accordingly), or add a genuine historical rollup-staleness instance to §2.4 with evidence and pin its replay fixture — leaving D-6's recorded content intact under either close.
- **Disposition**: applied — the replay bar restated exactly and satisfiably as two attested historical replay families plus one NAMED constructed staleness fixture: §3.2 M1's signal now enumerates FR-2 (the #514/#515 unwired trees), FR-3 (the T4/T8 Touches gap), and FR-4's constructed rollup-staleness fixture; §1 Success, §0's sanity numbers, and DOD-2 reworded to match (3/3 arithmetic kept, "historical" no longer claimed of the third); §2.4 untouched (it evidences the rot class incl. the parked map instance, which M1 no longer counts); D-6's recorded observable intact. (delivery:maintenance-sweep#E-5)

### FND-3 — A blocking DoD row is sourced outside the contract: DOD-6 traces only to the brainstorm record, and §8's traceability row is checked ✅ over it

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: DOD-6 (§5.8, Blocking: yes): "The candidate glossary terms land in the domain glossary — Source: brainstorm T-1..T-3 — Verify: Inspection" — vs — §8's row "Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric — ✅ — DOD-1..DOD-6 sourced by reference", and the template's §5.8 rule (prd-template/v2): "a DoD row that invents a criterion found nowhere else is a requirement hiding from review — move it up". `brainstorm T-1..T-3` is none of the four admissible source classes — no §5 requirement, §3.3 guardrail, §6.4 criterion, or rubric anywhere in the document obliges the glossary landing (§6.6's Docs item is the runbook, not the glossary) — so the §8 self-attestation is false for DOD-6, and the coherence rule "§8 verified, not trusted" fails on exactly this row.
- **Acceptance condition**: Give the criterion an in-document home DOD-6 can cite (a §5 row or a §6.4/§6.6 item covering the glossary landing), or demote DOD-6 to non-blocking with its reason stated in the row, or strike it (the landing is already journaled at delivery:maintenance-sweep#E-2) — and re-walk §8's traceability row so its ✅ is true of the resulting DoD set.
- **Disposition**: applied — DOD-6 struck per the acceptance's third route: the glossary landing already happened in this very write set and is journaled at delivery:maintenance-sweep#E-2, so no done-criterion remains to gate; §8's traceability row re-walked to "DOD-1..DOD-5 sourced by reference" with the strike's reason stated in the row. No other DoD row renumbered. (delivery:maintenance-sweep#E-5)

### FND-4 — Incident I3's fail-closed-on-misparse lesson is uncarried: the new consequential CLI's failure space omits invocation misparse entirely

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §5.2's failure space covers unreadable repo state (UB-1), malformed waiver/config (UB-2), a faulted detector (UB-3), and an unparsable plan (UB-4); FR-8 makes the exit codes a scripted contract for schedules and drives; §6.4's Reliability criterion is "faults never masquerade as clean" — vs — incident I3 (docs/observability/incidents.md, tags [cli, fail-open]): "consequential CLIs fail closed on misparse: any flag-like positional is a usage refusal before any action (guard + tests in both mains); and never truncate the output of a consequential run" — the registry lesson paid for in this exact repo when a misparsed flag silently executed the wrong behavior on first live use. No row in the subject states the sweep's behavior on a misparsed invocation (unknown flag, flag-like positional): a misparse that silently runs a different pass than asked — e.g. ignoring an intended waiver/config selection — and exits "clean" is precisely the false-negative class the PRD's own reliability bar excludes, and the P-1 home question (own binary vs a verb on an existing one) does not discharge it: the observable refusal behavior is home-independent and belongs in this contract like UB-1..UB-4 do.
- **Acceptance condition**: Carry the lesson: add an unwanted-behavior row of the shape "If the invocation carries an unknown flag or a flag-like positional, then the sweep shall refuse with a usage defect and the fault exit code before evaluating any detector" (fixture: misparse cases produce the fault code and zero finding lines), or state the misparse posture explicitly in §5.2 with its rationale citing I3 — including, if the design later inherits an existing main's guard, that the inherited guard satisfies this row.
- **Disposition**: applied — incident I3's lesson carried as new row UB-5: an unknown flag or flag-like positional refuses with a usage defect and the fault exit code before any detector evaluates, rationale citing I3, fit criterion "misparse fixtures produce the fault code and zero finding lines", traces F1/F2 — the observable refusal behavior bound home-independently, exactly as the acceptance condition frames it. (delivery:maintenance-sweep#E-5)

### FND-5 — The waiver-with-reason behavior the narrative leans on is bound by no requirement, and §5.3's skip narrows the template's trigger past the one new abuse-relevant surface

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §4.4 ("the operator adds it to the waiver surface **with a reason**; every later sweep reports the waiver as applied") and §7.1 ("waiver rot — waivers accumulating past their reasons (mitigated by applied-waiver visibility)") — vs — the requirement set, where FR-5's fit demands only suppress-plus-applied-line and UB-2 only a closed parse: an implementation with bare package-name waivers passes every fit criterion, leaving the named failure mode (waiver rot; silent suppression of the dead-feature signal) with no contractual mitigation beyond the fact of application being printed. Relatedly, §5.3 is skipped on the ground that no "credentialed" surface exists, but the template's T2 trigger is "any new surface or role" — and the waiver surface is the one new surface whose misuse (waiving away real rot) the PRD itself names. Not load-bearing for §8 (jobs/flows stay covered), hence advisory.
- **Acceptance condition**: Bind the waiver-entry contract — a waiver carries a reason and the applied-waiver report surfaces it (an FR row or an amendment to FR-5's fit criterion), optionally with an ABUSE-row framing of deliberate suppression — or record the refusal explicitly (a §7.3 row / NG note) so the narrative stops promising what no row requires.
- **Disposition**: applied — the waiver-entry contract bound in FR-5 itself: a waiver carries a recorded reason, the applied-waiver line surfaces that reason, and a reasonless entry refuses to load under UB-2's closed parse (fit criterion extended for all three arms); §5.3 rewritten to name the waiver file as the one new surface and state its contractual mitigation instead of skipping past the T2 trigger. (delivery:maintenance-sweep#E-5)

### FND-6 — Body content rests on the read-to-rule section: §7.1 answers a FAQ by citing "(§0 call 3)"

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §7.1 Internal FAQ: "Why only three detectors? — A: … the parked ones (WR-1..WR-2) currently cannot (§0 call 3)." — vs — the template's §0 contract (prd-template/v2): the section is "informative, never normative", "machines ignore it", and "nothing downstream may cite into it"; the body is the contract and must stand without it. A machine consumer of §7.1 (the compiler, a worker, this reviewer) hits a support anchor it is required to ignore. This finding judges the body's citation practice only — it neither reads nor evaluates §0's content.
- **Acceptance condition**: Replace "(§0 call 3)" with body/record anchors that carry the same support — e.g. §7.3 decision-log row 2, D-2, or the WR-1..WR-2 entries already cited in the same sentence.
- **Disposition**: applied — the §7.1 answer now cites body/record anchors only: "(§7.3 decision-log r2; brainstorm:maintenance-sweep#D-2)" replaces "(§0 call 3)"; no body content rests on the read-to-rule section. (delivery:maintenance-sweep#E-5)

### FND-7 — The header cites "Appendix A rule 15", but Appendix A contains no numbered rules — the sign-off-collapse rule is rule 15 of "How to use this template"

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: Header Approver(s) row: "the operator's review is the standing overturn channel (Appendix A rule 15, adapted to the delegated-drive case)" — vs — prd-template/v2, where rule 15 ("Sign-offs collapse honestly at small team size…") is the fifteenth of the numbered rules in "How to use this template", and Appendix A ("Writing rules for requirements") carries EARS patterns, the vagueness blacklist, and ID namespaces — no numbered rule list. A reader or tool following the citation to Appendix A cannot resolve it. (§6.6's bare "(rule 15)" is fine.)
- **Acceptance condition**: Correct the locus — cite "template rule 15" / the How-to-use rules — in the Approver(s) row.
- **Disposition**: applied — the Approver(s) row's citation locus corrected to "template rule 15 — the 'How to use this template' rules"; Appendix A is no longer cited for a numbered rule it does not carry. (delivery:maintenance-sweep#E-5)

### FND-8 — Flow F2 promises "output durable for later reading" that no requirement, home, or retention statement delivers — and §5.4 declares no state is created

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §4.3 Flow F2: "each tick = one F1 pass; **output durable for later reading**" — vs — §5.4: "Skipped: the sweep creates no state beyond its emitted report (stdout/exit codes)…", with no FR/NFR/DoD row naming where scheduled-sweep output lands, for how long, or who owns it (the shim is delegate-only per FR-7). The claim is unbound: reconcilable only by an unstated operator-side mechanism. The AP-28 posture is likewise unstated — whether scheduled findings are regenerable projections (defensible via FR-6's determinism, with judgment retained in the committed waiver surface) or improvement fuel owed a committed event record ("a drift snapshot" is in AP-28's named set) is a question the contract should answer rather than leave implied. Does not make a Must row unverifiable (F2 traces to FR-7/FR-8, which stand alone), hence advisory.
- **Acceptance condition**: Either bind durable scheduled output into a requirement (named home, retention, owner), or strike "durable for later reading" from F2 and route retention explicitly to the design/consumer — in either case stating the AP-28 posture for findings output in one sentence.
- **Disposition**: applied — "output durable for later reading" struck from F2 and retention routed explicitly to the design with P-5 (the sweep stays stateless per §5.4), and §5.4 now states the AP-28 posture in one sentence: findings output is a regenerable projection under FR-6's determinism, judgment lives in the committed waiver surface, a committed drift-snapshot record is engine-era ground. (delivery:maintenance-sweep#E-5)

### FND-9 — The evaluated repository state is named three ways and pinned nowhere: "committed state" (FR-1), "mainline state" (FR-3), "live mainline" (DOD-3/§1)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-1: "over the repository's **committed state**"; FR-3: "every Touches surface absent from the repository's **mainline state**"; §1/§3.2 M2/DOD-3: "the **live mainline** sweep" — vs — Appendix A's quality bar (unambiguous, consistent) and the banned-vagueness discipline: on a non-mainline checkout the three readings diverge (a surface present on the branch but absent from main flips FR-3's answer), and a wrong read here manufactures exactly the R1-class false positive the PRD treats as existential (M2 = 0 is the adoption bar). Fit criteria stay verifiable on pinned fixtures, so no Must row is unverifiable — advisory; but the live bar (DOD-3) consumes the ambiguity directly.
- **Acceptance condition**: Define the evaluated state once — one term, one definition (e.g. the committed state of the invoked repository's checkout, with any intended mainline semantics named explicitly) — and use it consistently across FR-1, FR-3, and DOD-3/§1/M2.
- **Disposition**: applied — the evaluated state defined once in §4.1 (the committed HEAD state of the invoked repository's checkout; mainline semantics = invoking on a current mainline checkout) and used consistently: FR-1 and FR-3 now cite "the evaluated state (§4.1)", M2's signal and DOD-3's live bar name it explicitly; the three divergent phrasings are gone. (delivery:maintenance-sweep#E-5)

## Round 2

*Scope-locked re-review per protocol step 8: the nine named findings' spans plus regressions within changed spans, nothing else. No round-1 snapshot was committed, so changed spans were located through each finding's cited anchor — the guide's stated fallback. Floor re-excluded: `docgate gate` is green on this tree both base-free and `--merge-base origin/main` (exit 0 · exit 0), so projection integrity (the refold under the nine edits, fingerprint per delivery:maintenance-sweep#E-5) and typed-citation resolution are gate ground, not findings. §0 remains unrouted, uncited, unjudged (FR-17). Governing slices re-read only where a changed span implicates one: AP-28's text against §5.4's new posture sentence; the record's D-6 (its two named replay families survive intact under the restated M1) and P-5 (the scheduling-binding pointer F2 now routes retention to); incident I3 against UB-5 (via its round-1 quotation); delivery:maintenance-sweep#E-2/E-5 resolve and record what the dispositions claim. The unchanged remainder was not re-read or re-judged.*

*Disposition verification — all nine held, each satisfying its acceptance condition on the revised subject:*

- *FND-1 held — FR-6 now demands a byte-identical machine face whose fit includes "the machine face contains no wall-clock field"; EVT-1's properties are "deterministic fields only" with duration named human-face-only. One stated contract satisfies both rows; regression-checked: M1..M3's feeds need no duration, and NFR-1's ≤ 60s bound verifies without a machine-face timestamp.*
- *FND-2 held — M1's signal enumerates the two attested historical families (FR-2: the #514/#515 unwired trees; FR-3: the T4/T8 Touches gap) plus FR-4's named constructed staleness fixture; §1 and DOD-2 match; "historical" is no longer claimed of the third; §2.4 untouched; the record D-6(a)'s two-family observable intact.*
- *FND-3 held — DOD-6 struck (the acceptance's third route; the landing is journaled at delivery:maintenance-sweep#E-2, verified), no surviving row renumbered, §8's traceability row re-walked with the reason in-row and now true of DOD-1..DOD-5 (each source an admissible class). One cross-reference survived the strike dangling → FND-10.*
- *FND-4 held — UB-5 carries I3 exactly per the acceptance shape: an unknown flag or flag-like positional refuses with a usage defect and the fault exit code before any detector evaluates; fit "misparse fixtures produce the fault code and zero finding lines"; traces F1/F2. The verification-bar enumerations predating it were not extended → FND-11.*
- *FND-5 held — FR-5 binds the waiver-entry contract (reason required; reason surfaced on the applied line; a reasonless entry refuses to load under UB-2's closed parse — three-arm fit); §5.3 now names the waiver file as the one new surface and states its abuse case's contractual mitigation instead of skipping the T2 trigger; §4.4's and §7.1's waiver narrative is requirement-backed.*
- *FND-6 held — the §7.1 answer cites §7.3 decision-log r2 (verified present and supporting) and brainstorm:maintenance-sweep#D-2; zero §0 citations remain anywhere in the body.*
- *FND-7 held — the Approver(s) locus reads "template rule 15 — the 'How to use this template' rules"; Appendix A is no longer cited for a numbered rule.*
- *FND-8 held — "durable for later reading" struck; F2 routes capture/retention of scheduled output to the design with P-5 (apt: the record's P-5 is the scheduling-binding pointer); §5.4 states the AP-28 posture in one sentence — the regenerable-projection position, which FR-6's tightened determinism now genuinely supports, with judgment retained in the committed waiver surface.*
- *FND-9 held — the evaluated state is defined once (§4.1: the committed HEAD state of the invoked repository's checkout; mainline semantics obtained by invoking on a current mainline checkout) and cited from FR-1, FR-3, M2's signal, and DOD-3; "live mainline" and "mainline state" no longer occur.*

*Two regressions within the revision's blast radius, shipped below. Each is a cross-reference the round-1 edits invalidated — neither defect existed in the round-1 tree, so neither is a finding against unchanged content: they are regressions of the FND-3 and FND-4 spans manifesting at their cross-reference sites. Both dispositions are left to the author (advisory at the terminal round).*

### FND-10 — The DOD-6 strike left a dangling range: §6.5's on-demand rollout stage still exits on "DOD-1..DOD-6 green"

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §6.5 rollout table, on-demand stage: "DOD-1..DOD-6 green" as the stage's exit condition — vs — the revised §5.8, whose table ends at DOD-5 after FND-3's applied strike (the range was valid in the round-1 tree; the strike created the dangle, a regression of the FND-3 span). A criterion citing a row that does not exist cannot be evaluated as written — the same Appendix A quality bar (unambiguous, consistent references) FND-9 was closed against. Advisory per CAL-3: no Must row becomes unverifiable — the §5.8 set itself is coherent and DOD-1..DOD-5 carry the bar.
- **Acceptance condition**: Point the exit condition at the live set — "DOD-1..DOD-5 green" (or equivalently "the §5.8 DoD set green") — in §6.5's on-demand row.
- **Disposition**: applied — §6.5's on-demand exit condition repointed at the live set: "DOD-1..DOD-5 green (the §5.8 DoD set)"; no reference to the struck DOD-6 remains. (delivery:maintenance-sweep#E-6)

### FND-11 — UB-5 appears in no verification bar: §6.4's Reliability drill list and §8's flow-coverage note still enumerate UB-1..UB-4

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §6.4 Release criteria, Reliability: "UB-1..UB-4 drills pass; faults never masquerade as clean"; §8 flow-coverage note: "F1→FR-1..FR-8/UB-1..UB-4/NR-1; F2→FR-7/FR-8" — vs — the new Must row UB-5 (FND-4's applied close), which traces to F1 and F2 and is now the one failure row both enumerations omit (both were complete in the round-1 tree; the UB-5 addition made them stale — a regression of the FND-4 span). The misparse drill thereby sits outside every release-criterion enumeration — the drill class I3 exists to keep un-skipped. Advisory per CAL-3: UB-5 stays verifiable through its own fit criterion, and §8's checkmarks remain true as scoped (F1/F2 coverage exists), so neither CAL-3's nor CAL-4's blocking condition triggers.
- **Acceptance condition**: Extend both enumerations to the live failure set — §6.4 Reliability to "UB-1..UB-5 drills pass", and §8's flow-coverage note to include UB-5 under F1 and F2 — so the bars enumerate the misparse drill they already imply.
- **Disposition**: applied — both enumerations extended: §6.4 Reliability now "UB-1..UB-5 drills pass" with the misparse fixtures named in its evidence cell, and §8's flow-coverage note reads "F1→FR-1..FR-8/UB-1..UB-5/NR-1; F2→FR-7/FR-8/UB-5". (delivery:maintenance-sweep#E-6)

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-19 |
| 2 | approve-with-advisories | 2026-07-19 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 7 advisory *(round 1: 4 blocking + 5 advisory; round 2: 2 advisory regressions, 0 blocking)*
- **Dispositions**: 9 applied · 0 declined · 0 withheld-as-violation *(all round-1 findings applied and verified at round 2; FND-10..FND-11 are advisory and undisposed at the terminal verdict — the author's to take or leave)*

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
