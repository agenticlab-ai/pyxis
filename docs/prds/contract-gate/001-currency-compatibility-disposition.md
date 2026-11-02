# Amendment 001 — currency-compatibility-disposition

| Field | Value |
|---|---|
| **Amends** | contract-gate *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-29 |
| **Learned-from** | operator-decision:design-brainstorm:planner-driven-model-routing-tech-design-3#D-7 |
| **Blast radius** | tier: contract-changing; touches: FR-2, FR-9, FR-10, FR-14..FR-19, NR-4, UB-7..UB-10, ABUSE-4, NFR-7, EVT-5, DOD-2, DOD-4, DOD-8, DOD-9, DOD-11..DOD-15, Q5, §0, §5.4, §5.5, §6.2, §6.3, §8; reason: permit one exact independently reviewed compatibility disposition to clear one otherwise-blocking stale edge while binding approval to the exact edge and record, validating disposition PRs continuously, and preserving fail-closed behavior for every uncovered, ambiguous, incompatible, unauthorized, or expired edge |
| **Tracking** | agenticlab-ai/pyxis#802 — the design effort parked on this contract change |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-2

Reason: successor churn can leave a frozen dependent semantically compatible but mechanically stale, and forcing a successor for every unaffected dependent creates a large, low-value rewrite cone.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-14 | When the currency check encounters a live design that serves a superseded or retired requirement or builds on a superseded design, the gate shall fail with a **currency** defect naming that exact stale edge unless one current, valid `currency-disposition/v1` record covers that edge; a valid record may clear only its covered edge, while every uncovered edge remains blocking. | currency remains fail-closed by default, but an exact compatibility proof avoids successor churn where the consumed contract did not change | Must | critical | fixtures prove: an uncovered stale edge fails; one valid disposition clears exactly its named edge; a second uncovered edge still fails; a retired requirement with no live replacement cannot be cleared; the check consumes the index's stale flags and dispositions rather than changing the shared flags | Test | G2, J3, F1, NR-2 |

### op: add FR

Reason: the exception needs one portable, closed semantic contract for both stale design dependencies and stale requirement-service edges.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-15 | A `currency-disposition/v1` record shall be a closed, versioned, committed datum covering exactly one stale edge and declaring: unique disposition id; author principal; edge kind `design_dependency` or `requirement_service`; feature and subject design; superseded target; one current live replacement target; a non-empty exact old-to-new binding mapping; conformance-evidence references; subject, old target, replacement target, cited-binding, mapping-set, and policy fingerprints; and one typed reference to a separate closed approval attestation. The projection shall compute a canonical disposition-record fingerprint without self-reference. The attestation shall bind that disposition id and computed fingerprint, exact edge, mapping-set digest, `compatible` verdict, explicit complete-scope and no-known-contradiction assertions, authenticated human reviewer principal distinct from the author, approval-authority policy id and fingerprint, and evidence reference. Wildcard, range, whole-document, empty, and multi-edge claims are forbidden. | the independent human review is the authority for which predecessor bindings were actually consumed; the gate validates that exact judgment rather than pretending the whole-document `Builds-on` edge already exposes a machine completeness oracle | Must | critical | schema fixtures accept both complete variants and reject every unknown key, missing field, unknown enum, empty/multi-edge/wildcard/range/document-level mapping, absent live replacement, non-human or author-equal reviewer, non-compatible verdict, false scope assertions, or separate attestation not bound to the exact disposition/edge/mapping/policy fingerprints; fingerprint fixtures prove canonical non-self-referential reproduction | Test + Inspection | FR-14, G2, ABUSE-4 |
| FR-16 | When validating a disposition, the gate shall require every old binding to name an exact binding element on the superseded target, every replacement binding to name an exact binding element on the current live target, every mapping and conformance-evidence reference to resolve, every pinned fingerprint to match the evaluated revision, and the approval attestation to resolve under its exact current authority policy; the gate shall not infer consumed bindings, score compatibility, or itself decide completeness or contradiction. | a deterministic gate can prove structure, freshness, and exact approval binding, while the independently authenticated human reviewer owns the consequential scope-completeness and compatibility judgment | Must | critical | positive fixtures validate exact mappings and attestations for both edge kinds; missing anchors, unresolved evidence, changed subject/target/mapping/record bytes, stale policy, unrelated approval, author-equal reviewer, non-approved verdict, and unauthorized or revoked reviewer each fail before the edge is cleared; inspection finds no model call, prose inference, or semantic scoring path | Test + Inspection | FR-14, FR-15, NG-2, NFR-3 |
| FR-17 | When one or more valid dispositions clear stale edges, the gate shall preserve the underlying stale flags, include each applied disposition id and exact edge in the structured verdict and EVT-5 audit output, and still return `clean` only when every other currency and non-currency check passes. | a cleared edge must remain visible and auditable without falsifying the shared graph or over-claiming the whole contract | Must | high | a mixed fixture reports every applied disposition, preserves every source stale flag, and returns clean only with no remaining defect; repeated runs produce the same ordered applied-disposition set | Test + Inspection | FR-7, NR-2, EVT-5 |

### op: supersede FR-9

Reason: a compatibility disposition is a contract input and must receive the same continuous pre-merge validation as a PRD or design change.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-19 | When a PRD, design, or currency-disposition PR would merge, the gate shall run the five checks over the affected feature's candidate contract as a CI context, reporting currency, link-resolution, structural, and disposition-integrity defects as blocking and inverse-coverage defects as advisory. | the record that can clear a critical currency edge must be validated before activation, while a PRD legitimately merges before its serving designs exist | Must | critical | fixture PRs seeding a currency/link/structural defect or malformed/expired/unauthorized disposition fail the context; a valid exact disposition clears only its edge; a coverage-only gap remains advisory; a clean PR passes | Test | G3, J3, F3, ABUSE-4 |

### op: supersede FR-10

Reason: the gate must read the committed compatibility disposition machine face in addition to the existing contract projections.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-18 | The gate shall read the contract exclusively from committed machine faces — the PRD source map, the design index and its `serves_inverse`, the DoD schema, and the governed currency-disposition projection carrying exact approval-attestation and authority-policy state — and shall not parse document prose for substance, re-run a reviewer lens, infer consumed bindings, or discover compatibility from citations not declared by a disposition. | the exception remains deterministic only when the stale edge, exact reviewed judgment, and current approval authority have closed machine faces | Must | high | inspection confirms these are the only substantive inputs; prose-only compatibility, an undeclared mapping, and PR/commit status without an exact attestation do not clear an edge; no code path invokes a review lens or scores semantic quality | Test + Inspection | J4, NG-3, FR-15, FR-16 |

### op: add NR

Reason: compatibility evidence must not rewrite frozen designs or conceal the graph state other consumers rely on.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-4 | A compatibility disposition never edits, refolds, relabels, or changes liveness of its subject, superseded target, replacement target, or the design index's `stale-serves` and `stale-builds-on` flags; it only changes the contract gate's disposition of its exact edge. | the evidence is an auditable compatibility judgment, not a hidden successor or mutation of frozen contract artifacts | critical | byte-identity and index fixtures prove every referenced document and stale flag unchanged before and after disposition evaluation; other readers observe the original graph | Test + Inspection | FR-14, FR-17, NR-2 |

### op: add UB

Reason: malformed, expired, and ambiguous compatibility claims need distinct fail-closed outcomes.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-7 | a currency-disposition projection or record is missing when referenced, unreadable, malformed, carries an unknown schema version or key, or lacks a required field | emit a `configuration-error` verdict naming the projection, record, and defect, run no currency exception for that record, and refuse planning fail-closed | configuration error with the exact record and repair | Must | critical | one fixture per malformed class produces no cleared edge and no clean verdict; independent ordinary checks remain represented according to the verdict contract | Test | FR-15, FR-18 |
| UB-8 | a disposition's subject, target, binding, evidence, policy, or fingerprint no longer matches the evaluated revision | classify the disposition as expired, leave its stale edge blocking, and name every mismatched or unresolved component and the need for a new reviewed disposition or a successor design | stale verdict with expired-disposition detail | Must | critical | mutations to each pinned component independently expire the record; no expired record clears an edge; restoring exact inputs reproduces the original result | Test | FR-16, NFR-7 |
| UB-9 | multiple, duplicate, or conflicting dispositions claim the same stale edge, or one disposition claims more than one edge | leave the edge blocking and emit a currency/configuration defect naming all competing records; never choose by file order, recency, or apparent strength | deterministic ambiguity defect and explicit unblock | Must | critical | multiple/duplicate/conflict/multi-edge fixtures all fail identically under input reordering and perform no implicit selection; zero records follows FR-14's ordinary uncovered-currency path | Test | FR-14, FR-15, NFR-7 |

### op: add UB

Reason: an approval reference that does not prove independent authorization over the exact disposition must fail distinctly from malformed or expired content.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-10 | a disposition's approval attestation is absent, unresolved, non-compatible, bound to different disposition/edge/mapping/policy bytes, names a non-human or author-equal reviewer, or the reviewer is unauthorized or revoked under the pinned authority policy | leave the edge blocking, emit a configuration/currency defect naming the exact approval failure and required independent review, and perform no implicit fallback to commit or PR status | unauthorized-disposition defect with the failed binding and unblock | Must | critical | each trigger independently blocks; a valid but unrelated approval reference, a green PR status without the attestation, and a changed disposition after approval all fail; only an exact current independent attestation permits FR-14 | Test + Inspection | FR-15, FR-16, ABUSE-4 |

### op: add ABUSE

Reason: a compatibility disposition weakens an otherwise-blocking governance edge and is therefore a privileged policy surface.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-4 | a contributor, worker, or mistaken operator trying to bypass design currency | authors a broad, forged, stale, self-approved, or selectively mapped disposition that hides an incompatible dependency | a contract enters planning while a frozen dependent contradicts its live requirement or predecessor | critical | FR-15's one-edge exact schema and disposition-bound independent attestation; FR-16's anchor/evidence/fingerprint/authority validation; UB-7..UB-10 fail closed; protected-review policy authenticates the human lane, while DOD-14 makes scope completeness and compatibility an explicit reviewer judgment | contributor-only, worker-authored, self-approved, broad/empty mapping, forged/unrelated approval, changed-target, duplicate, and conflict fixtures all leave the edge blocking. Residual: an authorized independent human can make a wrong semantic judgment; the exact attestation makes that decision attributable, M1 detects a downstream escape, and the incident path owns recurrence |

### op: add NFR

Reason: exception evaluation must remain as deterministic and inference-free as the currency check it qualifies.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-7 | Determinism / auditability | Given identical machine faces, disposition records, policy version, and gate version, compatibility evaluation shall produce one byte-identical ordered applied/expired/rejected disposition result using zero model tokens and no network access; record order shall not affect the verdict. | repeated and permuted fixture runs are byte-identical modulo declared timing fields; dependency inspection proves zero model or network client; each outcome names the exact disposition and edge | Test + Inspection |

### op: add EVT

Reason: a mechanically clean verdict that relied on human-reviewed compatibility must expose that reliance to downstream audit.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-5 | `currency_disposition_applied` | one valid compatibility disposition clears one stale edge during a gate run | `run_id:string`, `feature:string`, `disposition_id:string`, `disposition_fingerprint:string`, `edge_kind:enum{design_dependency,requirement_service}`, `subject_design:string`, `superseded_target:string`, `replacement_target:string`, `subject_fingerprint:string`, `old_target_fingerprint:string`, `replacement_target_fingerprint:string`, `mapping_set_digest:string`, `author_principal:string`, `reviewer_principal:string`, `approval_attestation_id:string`, `approval_authority_policy_id:string`, `approval_authority_policy_fingerprint:string` | gate structured log + verdict | M1, M2, M3, audit |

### op: supersede DOD-2

Reason: the currency-check proof must cover exact compatibility success and every fail-closed exception class.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | Each of the five checks catches its seeded defect class and passes a clean fixture; the currency check additionally clears only exact, current `design_dependency` and `requirement_service` dispositions carrying a compatible independently authenticated approval over their exact edge, mapping set, policy, and record, and blocks uncovered, retired-without-replacement, malformed, expired, ambiguous, broad, empty, and unauthorized cases. | FR-3..FR-6, FR-14..FR-19, UB-7..UB-10 | Test + Inspection | per-check fixtures plus complete positive/negative currency-disposition and approval-attestation matrix | yes |

### op: supersede DOD-8

Reason: the non-regression gate must prove that compatibility evidence never mutates or conceals shared graph state.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | No live non-regression row is broken, including byte-identity of referenced documents and preservation of advisory stale flags for every non-gate reader. | NR-1..NR-4 | Test + Inspection | one result per live NR row plus referenced-document manifest and before/after design-index comparison | yes |

### op: supersede DOD-9

Reason: unwanted-behavior coverage must include malformed, expired, and ambiguous disposition inputs.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | Every live unwanted-behavior row is demonstrated fail-closed, including every currency-disposition configuration, expiry, ambiguity, and approval-authorization path. | UB-1..UB-10 | Test | failure-injection matrix with one result per live UB row and zero omitted critical path | yes |

### op: add DOD

Reason: a privileged compatibility exception needs an explicit blocking abuse and audit proof.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | The complete currency-disposition abuse matrix leaves every unauthorized, broad, stale, forged, self-approved, or conflicting edge blocking, while each accepted disposition is independently human-reviewed, exact, governed, and visible with its record and approval-policy fingerprints in the verdict plus EVT-5. | ABUSE-4, FR-15..FR-17, NFR-7 | Test + Inspection + Human judgment — @mdarabi; rubric: for every accepted real-corpus edge, the reviewer is authorized and distinct from the author, the mapped old anchors are the complete set actually consumed by the dependent, every replacement preserves the consumed contract, the conformance evidence is current, and no known contradiction remains | abuse-drill report, accepted-disposition corpus, exact approval attestations, reviewer sign-off, and joined verdict/EVT-5 audit | yes |

### op: supersede DOD-4

Reason: continuous validation must prove the new disposition PR class and its exact approval boundary, not only PRD/design drift.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | The continuous CI context reports currency/link/structural/disposition-integrity defects blocking and inverse coverage advisory across PRD, design, and currency-disposition PRs; exact valid dispositions clear only their named edges before merge. | FR-13, FR-19, UB-7..UB-10 | Test | per-PR-class CI fixtures including exact approval, unrelated approval, expiry, ambiguity, and coverage-only cases | yes |

### op: supersede §5.4 Data

Reason: the contract now consumes a governed compatibility record and its generated machine projection.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| verdict | one run's structured result: the per-check outcomes, the overall class, the identity of what was evaluated, and every compatibility disposition applied | the gate (returned per run) | rides the host's invocation; the plan's own fingerprint pin is the durable audit trace on pass | closed schema (FR-7, FR-17); records each input's content fingerprint + gate version; a pass is never labeled executable; a pass-class never coexists with a failed/errored check | internal |
| defect report | one bounce's bundled named defects, two faces | the gate (emitted only on failure) | retained/parseable so downstream consumers can read it; the storage mechanism is design-owned (P-3) | template-governed closed schema (FR-8); one report per bounce; every defect names its check id, offender, and unblock; emitted on failure only, never on pass | internal |
| severity-map policy | which checks are blocking vs advisory per trigger (planning entry / contract-doc PR) | the repo (`pyxis-config/`), human-authored | repo lifetime | closed schema, versioned; unknown keys are errors (FR-13); the default encodes `#D-4`'s split; demotion is a human-lane edit (§3.3) | internal |
| currency disposition | one independently human-reviewed compatibility judgment covering exactly one stale edge under `currency-disposition/v1` | committed governed record; scope completeness and semantic truth owned by the exact approving human attestation | repo lifetime; changes, replacement, removal, or revocation require the same protected independent-review lane and remain in git history | closed schema; one edge; non-empty exact binding mappings; no wildcard/range/document claim; current fingerprints; author and distinct reviewer; attestation binds exact record/edge/mapping/policy and compatible verdict (FR-15, FR-16) | internal |
| approval attestation + authority state | proof that an authorized human distinct from the author approved this exact disposition fingerprint, edge, and mapping set as complete and compatible under one pinned policy | governed human-review adapter's committed machine face, carried in the disposition projection | same lifecycle as the disposition; policy/reviewer changes invalidate applicability without rewriting prior evidence | closed and fingerprinted; exact subject binding; current authorized principal; no status/badge inference; unavailable, unrelated, revoked, or mismatched proof fails closed (FR-16, UB-10) | internal |
| currency-disposition projection | the closed machine face through which the gate reads live dispositions and exact approval/authority state | docgate-generated and byte-verified from committed records and governed approval evidence | regenerated with its source records and current authority state; history retained in git | deterministic projection; unknown or duplicate identities fail; source and approval-policy fingerprints preserved; hand edits fail `verify-generated` | internal |
| the other machine faces (read) | the PRD source map, the design index + `serves_inverse`, the DoD schema | docgate-generated and byte-verified — **read-only to this gate** | per the doc system | the gate never writes them; it trusts docgate's `verify-generated` guarantee (ABUSE-3) | internal |

- **Migration** — existing contracts need no record while current; every stale edge remains blocking until its dependent is superseded/retired or an exact disposition is added. The planner-driven-routing design set is the first intended corpus and must pass DOD-14 before planning resumes.
- **Parallel-run / cutover** — none: disposition consumption is additive to currency evaluation and fail-closed when malformed; no shadow or dual truth.
- **Rollback** — remove or revoke a disposition from the active projection through the same protected review lane and its edge immediately returns to ordinary blocking currency semantics; stopping gate invocation remains the whole-feature rollback.
- **Decommission** — a disposition leaves the active projection when revoked or when its subject or edge is no longer live; repository history and prior EVT-5 fingerprints preserve the audit trail.
- **Privacy check** — records contain repository identities and approval references only; no personal data beyond the existing contributor identity surface, no DPIA trigger.
- **Egress** — none: the gate makes no network calls and reads only committed repo files (FR-12, NFR-2, NFR-7).

### op: supersede §5.5 Interfaces and integrations

Reason: the gate has one new governed input boundary and exposes applied dispositions in its verdict.

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| plan-new ⇄ gate (planning-entry precondition) | in/out | the hard gate (F1) | in: a feature id / contract handle; out: the verdict (+ defect report on failure); plan-new refuses to draft on any non-clean verdict; a clean verdict names every applied compatibility disposition | UB-1..UB-10; a plan-new amendment that mis-wires the precondition | this feature (gate); plan-new (the precondition row) |
| contract-doc CI context | out | the continuous advisory surface (F3) | the same checks as a CI context on PRD/design/disposition PRs; currency/link/structural/disposition-integrity blocking, coverage advisory (FR-19) | a false block on a mid-authoring contract; malformed, expired, unauthorized, unrelated, or ambiguous disposition evidence remains blocking | this feature |
| contract machine faces | in | contract resolution (FR-18) | reads the PRD source map, design index + `serves_inverse`, DoD schema, and governed currency-disposition projection; all closed and docgate-generated | UB-2, UB-7..UB-10; drift if a machine-face schema changes without updating the reader | docgate (faces); this feature (reader) |
| currency-disposition authoring/review | in | provide the independent human semantic judgment the mechanical gate may consume | one governed `currency-disposition/v1` record per stale edge; exact non-empty mappings/fingerprints; exact compatible attestation by an authorized human distinct from author; activated only through the protected lane | forged/broad/self approval, unrelated evidence, changed bindings or policy, unauthorized/revoked reviewer, duplicates, conflicts | contract owner (author); independent human (approval); review adapter + docgate (projection) |
| the defect report | out | the bounce consumers | one dual-face structured report; consumed by plan-new's session, the CI context, and future readers | consumer drift if the report schema changes | this feature (schema) |
| the LLM follow-up | out (future) | the semantic + analytical verdicts land additively | the verdict schema reserves the semantic-verdict fields (FR-7); the follow-up fills them; it does not replace disposition approval | deferred (WR-1) | future feature |

### op: supersede §6.2 Dependencies

Reason: exact compatibility now depends on a portable approval-attestation machine face and protected independent-review policy.

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| the machine faces — the PRD source map, the design index + `serves_inverse`, the citation resolver, and the §12 invalidation computation | system (shipped) | Phase A | confirmed (in docgate/plangate today) | @mdarabi |
| the check-input data path — per-requirement fit/verification fields the source map does not expose today (P-2) | system (may need a fold enrichment) | Phase A | at-risk — a flag-and-surface budget risk (§6.3 R1) | @mdarabi |
| currency-disposition + approval-attestation projection — exact disposition, record/edge/mapping fingerprints, independent-review evidence, and current authority-policy state | system + approval (new; adapter-neutral committed machine face) | compatibility increment in Phase A | at-risk — Q5 technical design must close home, codec, authority adapter, lifecycle, and fingerprint semantics before implementation | @mdarabi |
| protected independent-review policy — authenticates a human reviewer distinct from the author and exports an exact current attestation without making a code host privileged | approval / adapter | every disposition activation | required; conformance fixture must cover each supported review medium and offline gate input | @mdarabi |
| plan-new — the host: the planning-entry precondition | system (merged; integration is a typed amendment to its ledger) | Phase C | confirmed; amendment lands the precondition (NR-1) | @mdarabi |
| docgate `verify-generated` — the machine-face integrity guarantee the gate relies on (ABUSE-3) | system (shipped) | Phase A | confirmed | @mdarabi |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: the compatibility exception adds false-clear judgment and portable approval-authority risks that the base did not carry.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | The check-input data path (P-2) balloons — exposing per-requirement fit/verification fields to the checks turns into a large machine-face change | M | H | flag-and-surface per `#D-8`: if the fold enrichment is large, raise it before absorbing; Phase A front-loads it so the cost is known before integration compounds | @mdarabi |
| R2 | The gate false-bounces legitimately-mid-authoring contracts and trains bypass | M | H | the advisory-vs-hard severity split (FR-13, FR-19); advisory-first rollout at planning entry (§6.5); the §3.3 wrong-bounce guardrail with a human-lane demotion path | @mdarabi |
| R3 | Reviewers over-trust a mechanically clean pass as the full stage-6 guarantee | M | M | FR-7 forbids labeling a pass executable; the honest-floor framing remains in §1, §4.3 F4, and NG-2; ABUSE-1 names the residual | @mdarabi |
| R4 | The plan-new precondition amendment perturbs its frozen contract beyond the local-gate row | L | M | NR-1 confines the amendment to the precondition FR; an inspection diff proves its other rows untouched | @mdarabi |
| R5 | An authorized independent reviewer incorrectly attests that the mapping is complete or compatible | L | H | DOD-14's exact per-edge rubric and conformance evidence; record/approval fingerprints make the judgment attributable; M1 detects an escape; a paid-for miss enters the incident path rather than being hidden | @mdarabi |
| R6 | Approval verification accidentally becomes GitHub-specific, online-only, or trusts a badge/status instead of the exact attestation | M | H | Q5 must design an adapter-neutral committed machine face; FR-16/FR-18 and UB-10 reject unavailable/unrelated proof and forbid status inference; conformance runs across every supported review adapter | @mdarabi |
| A1 | *Assumption (from §2.4, Med):* assembly drift occurs at meaningful rates in real contracts | M | M | the dogfood sweep (DOD-7) measures it over the repo's own live contracts; if near-zero, the currency check stays justified as cheap insurance | @mdarabi |

### op: add Q

Reason: the product semantics are closed, but the adapter-neutral approval and disposition projection need one named technical-design decision.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q5 | What concrete home, codec, active/revoked lifecycle, fingerprint canonicalization, approval-authority adapter contract, and committed projection represent `currency-disposition/v1` and its exact independent approval without privileging one code host? | design | technical design before compatibility implementation | open — must preserve FR-14..FR-19 and UB-7..UB-10 exactly |

### op: supersede §8 Contract readiness — the executability gate *(required at every tier)*

Reason: the readiness matrix must evaluate the live amended contract, including its human-judgment criterion, rather than certify the superseded base counts.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2/T3) | ✅ | §2.6 problem gate signed 2026-07-17; amendment basis approved at design-brainstorm:planner-driven-model-routing-tech-design-3#D-7 on 2026-07-29 |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1–Q5 are technical-design-owned; Q5 closes representation only, while FR-14..FR-19 and UB-7..UB-10 close product semantics |
| Every live Must FR has a fit criterion and a verification method (§5.1) | ✅ | FR-1, FR-3..FR-8, FR-11..FR-19 each carry both; FR-2, FR-9, and FR-10 are superseded |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ✅ | critical gate/disposition rows are covered by fail-closed UB-1..UB-10, exact independent approval, DOD-14, the severity-demotion kill switch, and §6.5 rollout |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✅ | NG-1..NG-5; NR-1..NR-4 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ✅ | J1→FR-8; J2→FR-1, FR-3..FR-6, FR-14..FR-18; J3→FR-17, FR-19; J4→FR-7, FR-17 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ✅ | F1→FR-1, FR-14..FR-18, UB-1..UB-10; F2→FR-8; F3→FR-17, FR-19; F4→FR-7, FR-17; NR-1..NR-4 protect integration/shared state |
| Every primary metric (§3.2) has instrumentation (§5.7) or a measurement plan | ✅ | M1→EVT-1/EVT-3/EVT-5; M2→EVT-2/EVT-5; M3→EVT-1/EVT-2/EVT-5; M4→EVT-2 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | ✅ | live DOD-1, DOD-3, DOD-5..DOD-7, DOD-10..DOD-15 are sourced; DOD-2, DOD-4, DOD-8, and DOD-9 are superseded |
| Risks and assumptions have owners (§6.3) | ✅ | R1..R6 and A1 name @mdarabi; R5/R6 cover the new judgment and authority surfaces |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✅ | DOD-14 names @mdarabi and the exact independence, completeness, preservation, evidence-currency, and contradiction rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ✅ | every live blocking row above names required evidence; §5.9's no-model skip remains valid because the gate consumes but never generates human judgment |

### op: supersede §0 Read to rule — the human reviewer's section (~7 min)

Reason: the human routing section must expose the narrow compatibility exception because it changes a critical blocking requirement.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Gate, not translator** — a passing contract materializes nothing; only a failure emits a defect report | Materialize the ExecutableContract on every pass | The artifact economy flips: O-1's revisit trigger becomes a Must, §4.1's chosen shape changes → §4.1, NG-1 |
| 2 | **Mechanical-only; the LLM layer is a separate follow-up** | Build semantic + analytical checks into this gate | Pass semantics change from mechanically clean to executable and cost stops being zero → WR-1, NFR-3 |
| 3 | **Pass is a floor, not the full stage-6 guarantee** | Treat a green result as proof of semantic quality | Reviewers over-trust a mechanical verdict → FR-7, NG-2 |
| 4 | **Stale stays blocking by default; one exact reviewed disposition may clear one exact edge** | Require a successor for every stale dependent; or accept a prose waiver | The first creates a large unaffected rewrite cone; the second destroys fail-closed currency → FR-14..FR-18, ABUSE-4 |
| 5 | **The human owns compatibility judgment; the gate validates shape, evidence, approval, and freshness only** | Let the gate infer compatibility; or trust an unreviewed author claim | Determinism and the protected human lane both disappear → FR-16, NFR-7, DOD-14 |
| 6 | **No wildcard, document-level, multi-edge, expired, duplicate, or contradictory disposition** | Let one broad approval clear a dependency cone | One stale review could silently mask later semantic drift → FR-15, UB-7..UB-9 |
| 7 | **The shared stale flags and frozen artifacts remain unchanged** | Rewrite the graph to make compatibility look current | Other readers lose the actual lineage state and compatibility becomes invisible mutation → FR-17, NR-4 |
| 8 | **Inverse coverage remains hard at planning entry, advisory on contract-doc PRs** | Hard everywhere | Mid-authoring contracts false-bounce continuously → FR-3, FR-9 |
| 9 | **Defect report retained + parseable** | Ephemeral console/session output | Future consumers lose the named bounce input → FR-8, §7.2 Q3 |

**Sanity-check the numbers** — one disposition covers exactly **one stale edge**; every accepted record maps **all consumed old bindings** to live replacements; the false-bounce guardrail remains **zero unresolved wrong-bounces**; the gate remains **p95 ≤ 5 s** and **zero model tokens**.

**The author's open questions** — binary home, check-input data path, defect-report storage, and reserved semantic-verdict fields remain design-owned in §7.2. The compatibility artifact's concrete path, codec, projection command, fingerprint granularity, lifecycle encoding, and protected-review integration are also design-owned, but the product semantics above are closed: one edge, two variants, exact mappings, governed reviewed evidence, fail-closed freshness.

**Skip the rest** — structure/ID/fold correctness → docgate; semantic compatibility → the named approving human using DOD-14's rubric; disposition storage and projection internals → the technical design; plan-new decomposition/sizing/projection → plan-new; the LLM judgment follow-up → WR-1.
