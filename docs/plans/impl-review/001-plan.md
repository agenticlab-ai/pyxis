# impl-review — the machine-payload proposal reviewer — plan 001

| Field | Value |
|---|---|
| **Feature** | impl-review |
| **PRD** | docs/prds/impl-review |
| **Design** | docs/tech-design/impl-review-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build impl-review — the machine-payload proposal reviewer: the impl-review-report/v1 schema, the GitHub persistence adapter, the four-lens sibling-session skill over a shared context pack, calibration + evals, depth bands, the attribution-bound harvest and adoption checkpoint, and the implement-now invocation seam — dogfooded advisory on codegate's own proposals |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The impl-review-report/v1 schema, its validation, and codegate grammar-compatibility

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 160–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/report |

**Goal:** One closed, versioned payload type carries a whole review — findings, dispositions, verdict, bands, and per-round revisions — validates its own integrity rules, and stays wire-compatible with codegate's finding grammar without importing it.

**Current behavior:** No review payload type exists; nothing defines a finding kin to codegate's, a per-round reviewed-revision, an attribution-bound disposition, or the validation that rejects a malformed one, and nothing pins the shared grammar against codegate.

**Desired behavior:** A dedicated payload type declares the finding — whose overlapping keys are byte-compatible with codegate's finding grammar plus the review-only fields (a lens identity, a lens tag, a severity, a calibration-row citation, an acceptance, and a disposition) — the disposition object (actor, verdict, mandatory reason on a decline, and an attribution field), and the report envelope carrying band and the live downgrade-only verdict with an append-only per-round record (each round's reviewed revision, its fresh-instance context and provenance, its finding set, and an escalation brief present only on an escalate verdict); a validator enforces one payload per proposal-review, a rendered region that never diverges from the payload, a calibration citation on every finding whose severity must match its cited row, a reviewed revision on every round, and a reason on every decline; a compatibility test asserts the overlapping keys equal codegate's current grammar with zero edit to codegate.

**Scope:**
- the closed, versioned payload type: finding, disposition, envelope, and the append-only round record with its own schema version
- the validator enforcing the one-payload, render-fidelity, calibration-citation, reviewed-revision, and decline-reason rules
- the grammar-compatibility test pinning the overlapping keys against codegate's finding grammar, importing none of it and carrying none of its rung-only keys

**Out of scope:**
- persisting or rendering the payload on any medium (T2)
- producing findings, dispositions, or verdicts at runtime (T4, T7, T8)

**Acceptance criteria:**
- [ ] a golden payload validates, and each seeded defect — a missing calibration citation, a severity contradicting its cited row, a second payload for one proposal-review, a rendered region diverging from the payload, a round missing its reviewed revision, a reason-less decline — fails validation naming its rule
- [ ] the compatibility test passes with the overlapping finding keys byte-identical to codegate's current grammar, and fails if a key drifts
- [ ] codegate's finding grammar and ledger are byte-unchanged by this task, and the payload imports no codegate struct and carries no rung-only key
- [ ] the payload round-record is append-only: a write that mutates a prior round's recorded revision is rejected

**Key interfaces:**
- the impl-review-report payload type and its schema-version field
- the payload validator verb every producer and reader calls
- the codegate grammar-compatibility assertion over the overlapping finding keys

**Traces:** prd:impl-review/FR-5 · prd:impl-review/NR-1 · design:impl-review-v1#DINV-6

### T2 — The persistence port and the GitHub adapter — one self-contained artifact

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 160–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/persist · internal/implreview/persist/github |

**Goal:** A review persists as exactly one self-contained artifact on the proposal's own medium through a two-method port whose GitHub adapter round-trips the payload byte-identically and renders the human face from it, never beside it.

**Current behavior:** No persistence port exists; nothing writes or reads a review on a proposal, nothing renders a human face from the payload, and nothing handles a write failure without losing the review.

**Desired behavior:** A port exposes persist-payload-to-reference and read-reference-to-payload; a GitHub adapter writes one bot-owned comment carrying the human rendering plus a fenced machine block, reads it back to a byte-identical payload including an oversize payload, and the human rendering is a pure function of the payload so a hand-authored region that diverges from it is invalid; the review is one artifact — never split across comments, never mirrored to another medium; a write failure retries to a bounded cap and then records a review-absence through an alternate signal rather than losing the review or hanging.

**Scope:**
- the persist/read port and its reference type
- the GitHub adapter: one comment carrying rendering plus fenced machine block, byte-identical round-trip including an oversize leg
- the render-from-payload function (the human face is derived, never separately authored)
- the bounded-retry-then-recorded-absence path on a write failure, with no split or mirror

**Out of scope:**
- the Carina adapter — a later phase, gated on the local work tracker existing (the design's operator gate ordering builds GitHub first)
- computing the verdict, dispositions, or findings the payload carries (T4, T7, T8)
- the harvest that reads persisted payloads back across proposals (T11)

**Acceptance criteria:**
- [ ] a payload persisted through the GitHub adapter reads back byte-identical, including an oversize payload, as one comment carrying one fenced machine block
- [ ] a rendered region hand-edited to diverge from the payload is rejected as invalid; the rendering regenerates identically from the payload
- [ ] a persist call that never splits or mirrors the review is the only accepted shape; a split or second-medium write is refused
- [ ] an injected write failure retries to the cap and then records a review-absence via the alternate signal, losing no review and never hanging

**Key interfaces:**
- the persistence port's persist and read methods and its reference type
- the GitHub adapter's comment-plus-fenced-block serialization
- the payload-to-rendering pure function

**Traces:** prd:impl-review/FR-6 · prd:impl-review/UB-5 · design:impl-review-v1#DINV-9

### T3 — The review context pack over the existing pack machinery

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 90–190 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/pack |

**Goal:** One deterministic read-only pack assembles every review input — the proposal at its reviewed revision, the deterministic verdict, the charter and governing design bindings, and the calibration table — over existing admitted pack kinds, and admits no authoring-trail kind.

**Current behavior:** No review context pack exists; nothing assembles the proposal, verdict, charter, design bindings, and calibration into one shared read-only input, and nothing excludes prior findings from that input.

**Desired behavior:** A pack builder projects the proposal at its reviewed revision and the deterministic verdict as a frozen subject, the task charter and the governing design's binding elements as governing-peer inputs, and the calibration table as the stage-calibration input — each mapped onto an already-admitted pack kind with no new kind introduced; no authoring-trail input (no prior findings or dispositions) enters the pack, so the reviewer never reads an answer key; the same built pack is the single shared input every lens reads, built once per review.

**Scope:**
- the pack builder mapping each review input onto an existing admitted pack kind, no enum change
- the exclusion of every authoring-trail input (prior findings, dispositions) by construction
- the single-build, shared-read contract feeding all lenses from one pack

**Out of scope:**
- spawning the lens subagents that read the pack (T4)
- the deep-band extra context the band layer adds to a review (T6)

**Acceptance criteria:**
- [ ] the built pack contains the proposal at its reviewed revision, the verdict, the charter, the governing design bindings, and the calibration table, each on an existing admitted kind with no new kind added
- [ ] a build that would admit any authoring-trail input (a prior finding or disposition) is refused, and the assembled pack contains none
- [ ] one pack is built per review and handed to every lens as the identical read-only input

**Key interfaces:**
- the review context-pack builder over the existing pack machinery
- the admitted-kind mapping for the proposal, verdict, charter, design bindings, and calibration
- the authoring-trail exclusion predicate

**Traces:** prd:impl-review/FR-2 · prd:impl-review/FR-3 · design:impl-review-v1#DINV-5

### T4 — The impl-review skill: sibling-session spawn, four blind lenses, adversarial screen

| Field | Value |
|---|---|
| **Blocked by** | T1, T3 |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/runtime · implreview-skill |

**Goal:** The reviewer runs as a session sibling of the implementing worker, spawns four lens subagents over the one shared pack each blind to the others, screens their candidates adversarially, and records a fresh-instance provenance that fails if it inherited the implementer's context.

**Current behavior:** No impl-review skill exists; nothing spawns a reviewer independent of the implementing session, nothing runs the four lenses over a shared pack, and nothing screens candidates or records a fresh-instance identity.

**Desired behavior:** The host invokes the skill at the proposal boundary; the reviewer starts as a fresh top-level session that is a sibling of — never a child of — the implementing worker, and spawns four lens subagents as its own children over the one T3 pack; each subagent's input is the pack plus its own lens instruction only, blind to the other lenses' findings; surviving candidates pass an adversarial screen that kills the plausible-but-wrong before they ship, and survivors serialize into the T1 payload; every round records a fresh-instance context and provenance derived from the reviewer root, and a run whose reviewer context resolves to the implementing session's own context fails the provenance check.

**Scope:**
- the skill invocation at the proposal boundary and the sibling top-level reviewer spawn
- the four lens subagents as children of the reviewer root over the single shared pack
- the finding-blind isolation between lenses and the adversarial survival screen
- the fresh-instance provenance record and its would-fail-if-inherited check

**Out of scope:**
- the lenses' judgment content, calibration, and floor exclusion (T5)
- band selection, deep-band context, and the dormant second-instance verify (T6)
- the verdict, staleness, dispositions, and two-round protocol (T7, T8)

**Acceptance criteria:**
- [ ] a run spawns the reviewer as a sibling of the implementing session and four lens subagents as children of the reviewer root over one shared pack
- [ ] each lens subagent's input is the pack plus its own instruction only, and an independence test shows no lens reads another lens's findings
- [ ] the provenance records a fresh-instance context distinct from the implementing session's, and a fixture where the reviewer inherited the implementer's context fails the check
- [ ] a seeded plausible-but-wrong candidate is killed by the adversarial screen before it reaches the payload

**Key interfaces:**
- the impl-review skill and its proposal-boundary invocation contract
- the sibling-session reviewer root and its four lens-subagent children
- the fresh-instance provenance record and its inheritance check

**Traces:** prd:impl-review/FR-1 · prd:impl-review/FR-2 · prd:impl-review/FR-3 · design:impl-review-v1#DINV-5

### T5 — The four lenses, calibration as data, and the mechanical-floor exclusion

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/calibration · implreview-lenses · implreview-calibration |

**Goal:** Four code-shaped lenses each cite a versioned calibration row for every severity, restate nothing the deterministic ladder already enforces, and treat an instruction planted in the diff as a finding candidate rather than a command.

**Current behavior:** No lens instructions or calibration table exist; nothing binds a finding's severity to auditable data, nothing excludes the mechanical floor, and nothing defends against directives embedded in the reviewed content.

**Desired behavior:** The four lenses — spec fidelity, design and convention conformance, security and abuse, and correctness critic — each attribute their findings and assign severity by citing a row of a versioned calibration table maintained as reviewed data, so a data-only calibration change flips a finding's severity with no instruction edit and a severity contradicting its cited row is invalid; no lens ships a finding whose defect class the deterministic ladder already enforces; a directive addressed to the reviewer inside the diff or its comments is surfaced as a finding candidate and never alters the review.

**Scope:**
- the four lens instructions and their per-finding attribution
- the versioned calibration table as reviewed data and the cite-a-row severity binding
- the mechanical-floor exclusion barring any finding that restates a ladder-enforced class
- the planted-directive discipline treating an embedded instruction as a candidate, never a command

**Out of scope:**
- the sibling-session spawn and lens-subagent runtime the lenses run inside (T4)
- the eval fixtures that regression-pin each lens's catches and the injection invariance (T12)

**Acceptance criteria:**
- [ ] every lens finding cites a calibration row, a data-only row change flips a finding's severity with no instruction edit, and a severity contradicting its cited row is rejected
- [ ] on a fixture seeded with ladder-catchable defects, zero lens findings restate a class the deterministic ladder enforces
- [ ] a diff carrying a directive addressed at the reviewer yields a finding candidate and an unchanged review, never an obeyed instruction
- [ ] each of the four lenses attributes its findings to itself on a mixed fixture

**Key interfaces:**
- the four lens instructions and their finding attribution
- the versioned calibration table and the severity-cites-a-row rule
- the floor-exclusion predicate over ladder-enforced defect classes

**Traces:** prd:impl-review/FR-4 · prd:impl-review/FR-12 · prd:impl-review/UB-3 · design:impl-review-v1#DINV-3

### T6 — Depth bands, deep-band mandatory context, the dormant second-instance verify, and oversize escalation

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/band |

**Goal:** The review reads its depth from the merge lane's own risk class, adds the abuse rows and incident registry only in the deep band with an escalated critic, keeps a second-instance verify present but off by default, and refuses to approve over a diff it could not fully read.

**Current behavior:** No band layer exists; nothing reads the lane's risk class, nothing concentrates scrutiny on the deep band, no dormant second-instance seam exists, and nothing stops an approve over a truncated oversize diff.

**Desired behavior:** The band layer reads the proposal's risk class from the merge lane's classification and applies skip, standard, or deep — a skip records a skip outcome naming its class and derives no second risk model; in the deep band the lenses run with the served abuse rows and the incident registry as mandatory context and the critic's calibration escalates so a load-bearing critic finding ships blocking; a second-instance verify is present but off by default, and when its data flag is on it re-judges a deep-band blocking finding through a second independent subagent and downgrades one the second instance does not confirm; a diff exceeding the band's honest read budget records a size-escalation routing the proposal to a human and never emits an approve over a partially read diff.

**Scope:**
- band selection from the lane's risk class, the recorded skip, and no review-side risk model
- the deep band's mandatory abuse-rows-and-incident-registry context and escalated critic
- the dormant, data-gated second-instance verify (off by default; on re-judges and downgrades an unconfirmed deep blocking finding)
- the oversize size-escalation that routes to a human rather than approving a partial read

**Out of scope:**
- the lane's risk classifier itself, which this only reads (owned by the merge lane)
- the lens judgment content and calibration the bands parameterize (T5)

**Acceptance criteria:**
- [ ] band selection equals the lane's class on a fixture per class, a skip records its class, and no review-side risk derivation exists
- [ ] a deep-band run carries the served abuse rows and incident registry and ships a seeded load-bearing critic finding blocking; a standard-band run omits that extra context
- [ ] with the flag off the second-instance verify never runs; with it on, a seeded unconfirmed deep blocking finding is downgraded through a second independent subagent
- [ ] an oversize diff records a size-escalation to a human and yields no approve verdict over the un-read extent

**Key interfaces:**
- the band selector reading the lane's risk class
- the deep-band mandatory-context loader and escalated critic calibration
- the dormant second-instance verify flag and its re-judge-and-downgrade path

**Traces:** prd:impl-review/FR-9 · prd:impl-review/FR-10 · prd:impl-review/UB-7

### T7 — The downgrade-only verdict, the escalation brief, the staleness binding, and the emitted signal

| Field | Value |
|---|---|
| **Blocked by** | T1, T4 |
| **Size band** | 110–230 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/verdict |

**Goal:** The verdict is one of four downgrade-only values that can force a human or bounce to the worker but can never grant the auto lane, an escalate carries a decision brief, each round binds the revision it judged, and the persisted verdict is a well-formed signal a lane can read.

**Current behavior:** Nothing computes a verdict, nothing forbids an auto-granting side effect, no escalation brief exists, no round binds the head it evaluated, and no downgrade signal is emitted for a lane to consume.

**Desired behavior:** The verdict is exactly one of approve, approve-with-advisories, request-changes, or escalate, and no value or side effect widens lane eligibility or writes a stage-nine sign-off — forcing a human or a return-to-worker is representable and granting auto is not; an escalate verdict carries a decision brief naming the single conflict, the options with their risks, and the decision needed, addressed to the human; every round records the head revision it evaluated and the live verdict is stale whenever the proposal head differs from the latest round's evaluated revision, in which case it is treated as absent; the persisted verdict is a well-formed downgrade signal a lane can read, while consuming it is out of scope here.

**Scope:**
- the four-value downgrade-only verdict with auto-granting unrepresentable by construction
- the escalation brief emitted only on an escalate verdict
- the per-round reviewed-revision binding and the treat-stale-as-absent rule
- the well-formed emitted downgrade signal (emit only; lane consumption is deferred)

**Out of scope:**
- the merge lane's consumption of the emitted signal — an owed separate merge-lane amendment (the design's Q4), gated before adoption relies on lane routing
- the two-round dispute protocol and disposition recording (T8)
- resume, the in-flight predicate, and review-absence on failure (T9)

**Acceptance criteria:**
- [ ] the verdict admits only the four values and no value or side effect grants the auto lane or writes a stage-nine signature; a fixture shows human-forcing works and auto-granting is unrepresentable
- [ ] an escalate verdict carries a brief naming the single conflict, options with risks, and the decision needed, and a non-escalate verdict carries none
- [ ] a proposal whose head differs from the latest round's evaluated revision marks the live verdict stale and treats it as absent
- [ ] the persisted verdict validates as a well-formed downgrade signal a reader can consume

**Key interfaces:**
- the downgrade-only verdict enumeration and its no-auto-grant guarantee
- the escalation-brief structure on an escalate verdict
- the per-round reviewed-revision field and the staleness predicate

**Traces:** prd:impl-review/FR-7 · prd:impl-review/FR-17 · prd:impl-review/UB-6 · design:impl-review-v1#DINV-7

### T8 — The two-round protocol: dispositions, disputes, scope-lock, drift, and disposition-before-raise

| Field | Value |
|---|---|
| **Blocked by** | T1, T7 |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/protocol |

**Goal:** A blocking finding is disputed only with a reason, a second round is scope-locked to the named defects and changed spans with a third refused, every blocking finding is disposed before raise, an applied-but-unverified one holds the verdict, and a declined blocking finding escalates.

**Current behavior:** No disposition protocol exists; nothing records a dispute with a reason, nothing scope-locks re-review, nothing verifies an applied finding closed, and nothing bars a raise with an undisposed blocking finding.

**Desired behavior:** A worker records a disposition of applied or disputed-with-reason and a reason-less dispute voids its round; a second round re-reviews only the named defects plus the changed spans, a round-two finding on unchanged content is withheld as a violation, and a third round is refused; before raise every blocking finding carries a recorded disposition, an applied blocking finding renders verified only after a second-round re-review confirms the named defect closed and otherwise renders an unverified claim that holds the verdict at request-changes; a revision that changes spans outside the named defects flags scope drift rather than silently re-reviewing them; a declined blocking finding escalates; a schema-invalid round is voided, retried once, and then recorded absent.

**Scope:**
- the disposition recording with a mandatory reason on a decline and the reason-less-dispute void
- the scope-locked second round, the withheld-on-unchanged-content rule, and the refused third round
- the disposition-before-raise gate and the applied-but-unverified hold at request-changes
- scope-drift flagging and the void-retry-once-then-absent path on a schema-invalid round

**Out of scope:**
- the verdict values and staleness the protocol operates over (T7)
- the operator-side attribution and the corpus harvest (T11)

**Acceptance criteria:**
- [ ] a reason-less dispute voids its round, a declined blocking finding escalates, and a third round is refused
- [ ] a second round reviews only named defects plus changed spans, and a round-two finding on unchanged content is withheld as a violation
- [ ] a raise with an undisposed blocking finding is a protocol defect, and an applied-but-unverified blocking finding renders unverified and holds the verdict at request-changes
- [ ] a revision changing spans outside the named defects flags scope drift, and a schema-invalid round voids, retries once, then records absent

**Key interfaces:**
- the disposition record and its mandatory-reason-on-decline rule
- the scope-locked two-round re-review with the withheld-on-unchanged rule
- the disposition-before-raise gate and the applied-verified transition

**Traces:** prd:impl-review/FR-8 · prd:impl-review/FR-15 · prd:impl-review/UB-2 · prd:impl-review/UB-4

### T9 — Lifecycle: idempotent resume, the in-flight predicate, and recorded review-absence

| Field | Value |
|---|---|
| **Blocked by** | T1, T7 |
| **Size band** | 90–190 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/lifecycle |

**Goal:** A re-invocation resumes from the persisted round state as a no-op rather than a fresh round, a bounded predicate tells a slow deep review from an orphan, and a reviewer that fails past its bound records a visible absence rather than a silent pass or an endless stall.

**Current behavior:** Nothing resumes a review from its persisted state, no predicate identifies an orphaned in-flight review, and a failed reviewer neither records an absence nor bounds its wait.

**Desired behavior:** A re-invocation reads the persisted payload's round state as authoritative and resumes there, so a double invocation is a no-op that starts no fresh round-one; an in-flight predicate — a started round with no terminal verdict past a configured absent-timeout — distinguishes a slow deep-band review from an orphan, with the timeout carried as configuration data; a reviewer that errors or exceeds its bound after bounded retry records a review-absence on the proposal that is never a silent pass and never an indefinite stall, so the host proceeds flagged in advisory and the absence routes a proposal to a human once adoption lands.

**Scope:**
- the idempotent resume from the persisted round state and the double-invocation no-op
- the in-flight predicate and the configured absent-timeout defining the reap contract
- the recorded review-absence on a bounded failure, never a silent pass or indefinite stall

**Out of scope:**
- the host janitor that acts on the predicate to reap and re-invoke (T14)
- the verdict and round record the lifecycle operates over (T1, T7)

**Acceptance criteria:**
- [ ] a re-invocation resumes at the persisted round and a double invocation is a no-op that starts no fresh round-one
- [ ] the in-flight predicate flags a started-round-no-verdict review past the configured absent-timeout and does not flag one within it
- [ ] a reviewer failing past its bounded retry records a review-absence on the proposal, and no fixture yields a silent pass or a non-terminating wait

**Key interfaces:**
- the resume-from-persisted-round-state entry and its no-op guarantee
- the in-flight predicate and the configured absent-timeout
- the recorded review-absence outcome

**Traces:** prd:impl-review/UB-1 · prd:impl-review/UB-8 · design:impl-review-v1#DINV-1

### T10 — Invocation validation: charter-fed refusal and defective-charter escalation

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/runtime · implreview-invocation |

**Goal:** A review with no charter is refused outright, a review whose charter is materially defective escalates upstream and withholds findings against the broken span, and no invocation degrades to a silent pass.

**Current behavior:** Nothing validates the invocation's charter; a charter-less or self-contradictory charter would be reviewed against silently.

**Desired behavior:** An invocation carrying no task charter is refused with a named defect and produces no review; a charter present but materially defective for review — ambiguous, self-contradictory, or contradicting the governing design — escalates an upstream-defect class and withholds findings against the defective span rather than reviewing against a broken spec; no invocation path degrades to a silent pass.

**Scope:**
- the charter-presence check refusing a charter-less invocation with a named defect
- the material-defect check escalating an upstream class and withholding findings against the defective span
- the guarantee that neither path degrades to a silent pass

**Out of scope:**
- the escalation brief structure the escalation reuses (T7)
- the pack assembly that carries the charter (T3)

**Acceptance criteria:**
- [ ] a charter-less invocation is refused with a named defect and produces no review
- [ ] a materially defective charter escalates an upstream-defect class and withholds findings against the affected span
- [ ] no invocation path yields a degraded silent pass

**Key interfaces:**
- the charter-presence validation at invocation
- the material-defect escalation and the finding-withholding over the defective span

**Traces:** prd:impl-review/FR-14 · prd:impl-review/FR-17

### T11 — Attribution binding and the measurement harvest

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 150–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/harvest |

**Goal:** An operator disposition counts only when its own medium author is the operator, and a harvest enumerating the full proposal universe reconstructs coverage, precision, the nit rate, and the miss corpus from payload fields alone.

**Current behavior:** Nothing binds an operator disposition to the operator's medium identity, nothing excludes a forged one, and no harvest reconstructs the corpus from proposals — including never-reviewed ones — without reading prose.

**Desired behavior:** An operator-class disposition is valid only when its attribution resolves to the operator's own authored artifact on the medium — a separate operator-authored review or comment whose author is the operator — never a payload-writable field, so a worker-authored "operator accepted" is excluded from the measured corpus; a harvest enumerates a repo's full proposal universe, parses each persisted payload, and computes coverage over the full universe with a never-reviewed proposal counted against it, precision over the disposition floor, the nit rate, and the miss corpus — every measurement from payload fields alone and none from rendered prose.

**Scope:**
- the operator-disposition attribution rule binding validity to the operator's own medium-authored artifact
- the exclusion of an unattributable or forged operator disposition from the corpus
- the full-universe harvest computing coverage, precision, nit rate, and miss corpus from payload fields only

**Out of scope:**
- the ten-review checkpoint and the ship rule that consume the harvest (T13)
- the GitHub squash-merge survival of the operator-disposition reference, verified at integration against the real code host

**Acceptance criteria:**
- [ ] a forged operator disposition whose disposition-artifact author is not the operator is excluded from the precision metric on the GitHub path
- [ ] an operator disposition derives from the operator's own medium-authored artifact and never from a payload-writable field
- [ ] the harvest reconstructs coverage, precision over the floor, nit rate, and the miss corpus from payload fields alone over a mixed proposal set
- [ ] a never-reviewed proposal in the set is enumerated and counted against coverage

**Key interfaces:**
- the operator-attribution resolver over the disposition-artifact's medium author
- the full-universe harvest and its payload-only measurement computation
- the unattributable-disposition exclusion

**Traces:** prd:impl-review/FR-13 · prd:impl-review/FR-16 · design:impl-review-v1#DINV-4 · design:impl-review-v1#DINV-8

### T12 — The eval suite and its execution protocol

| Field | Value |
|---|---|
| **Blocked by** | T2, T5, T6 |
| **Size band** | 180–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/eval · implreview-fixtures |

**Goal:** A permanent eval suite runs every seeded-defect family a fixed number of times and passes only on a catch every run, scores floor-exclusion and injection-invariance by a deterministic comparator, bounds payload size, and carries the band budgets as data.

**Current behavior:** No eval suite exists; nothing pins the lenses' catches, the floor exclusion, injection invariance, the nit rate, truncation resistance, or payload compactness, and no run protocol governs them.

**Desired behavior:** The eval families — the per-lens seeded catches including the deep-band legs, the floor-exclusion, the injection-invariance, the nit-discipline goldens, the truncation resistance, and a compactness bound on payload size — ship as fixtures and run a fixed minimum number of times per release on a decode-pinned configuration, passing only when caught on every run so an intermittent miss is a defect rather than a silent rerun; the floor-exclusion and injection-invariance legs are scored by a deterministic comparator rather than a judge model; families grow only by reviewed data change and a removal or threshold relaxation is a reviewed decision; the suite is permanent in continuous integration and the band token and latency budgets live as versioned data beside the calibration table.

**Scope:**
- the eval families as fixtures: per-lens catches, deep-band legs, floor exclusion, injection invariance, nit goldens, truncation, compactness
- the run-a-fixed-minimum, caught-every-run protocol on a decode-pinned configuration in permanent continuous integration
- the deterministic comparators for the floor-exclusion and injection-invariance legs
- the band token and latency budgets as versioned data, changed only by reviewed data change

**Out of scope:**
- the lenses and calibration the evals exercise (T5) and the band layer they run against (T6)
- the live adoption checkpoint reading the harvested corpus (T13)

**Acceptance criteria:**
- [ ] every family runs the fixed minimum and passes only on a catch every run, and an intermittent miss is reported as a defect
- [ ] the floor-exclusion and injection-invariance legs are scored by a deterministic comparator, not a judge model
- [ ] a compactness leg bounds payload size on the golden fixtures and a family removal or threshold relaxation requires a reviewed data change
- [ ] the band token and latency budgets are versioned data the suite reads, and the suite runs in continuous integration permanently

**Key interfaces:**
- the eval family fixtures and the caught-every-run protocol harness
- the deterministic comparators for floor exclusion and injection invariance
- the band-budget versioned data files

**Traces:** prd:impl-review/FR-4 · prd:impl-review/FR-10 · prd:impl-review/UB-3

### T13 — The advisory ramp and the adoption checkpoint

| Field | Value |
|---|---|
| **Blocked by** | T11 |
| **Size band** | 110–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implreview/checkpoint |

**Goal:** Until an operator promotes it the verdict blocks nothing, and a ten-review checkpoint computes coverage, precision over the disposition floor, and the nit rate from the harvested corpus, holding the ramp at advisory on any guardrail breach.

**Current behavior:** No advisory ramp exists; a verdict would gate from day one, and no checkpoint computes the promotion metrics or surfaces disposition engagement.

**Desired behavior:** While the checkpoint has not promoted the reviewer the verdict gates nothing — findings deliver and persist but raise and lane routing proceed regardless; a ten-review checkpoint computes coverage, precision evaluated only over at least the disposition-floor count of operator-disposed blocking findings, and the nit rate from the harvested corpus, and surfaces a disposition-engagement signal as an advisory flag rather than an auto-fail; promotion is an explicit operator act recorded per the ship rule, default off and per repo, and any guardrail breach holds the ramp at advisory while calibration iterates only by reviewed data change.

**Scope:**
- the advisory gate: verdicts block nothing until an operator promotion
- the ten-review checkpoint computing coverage, floor-gated precision, and the nit rate from the harvest
- the recorded, per-repo, default-off operator promotion act and the hold-at-advisory-on-breach rule
- the disposition-engagement signal as an advisory checkpoint flag

**Out of scope:**
- the harvest that produces the corpus the checkpoint reads (T11)
- the merge-lane routing a promoted verdict would drive — an owed separate merge-lane amendment (the design's Q4)

**Acceptance criteria:**
- [ ] before promotion a request-changes verdict blocks neither raise nor lane routing, and findings still deliver and persist
- [ ] the checkpoint computes coverage, precision over at least the disposition-floor count, and the nit rate from the harvested corpus, and extends the window when the floor is unmet rather than passing vacuously
- [ ] promotion is a recorded operator act, default off and per repo, and a guardrail breach holds the ramp at advisory
- [ ] the checkpoint surfaces a disposition-engagement signal as an advisory flag, not an auto-fail

**Key interfaces:**
- the advisory gate over the verdict's effect
- the ten-review checkpoint computation over the harvested corpus
- the recorded operator-promotion act and the guardrail-hold rule

**Traces:** prd:impl-review/FR-11

### T14 — The implement-now invocation seam and the host-side reap

| Field | Value |
|---|---|
| **Blocked by** | T4, T9 |
| **Size band** | 90–190 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/implnow/verificationgate |

**Goal:** One additive amendment of the verification-gate class adds the reviewer invocation at the worker's ladder-green exit with the drive-side contract byte-untouched, and the host janitor reaps an orphaned review through the lifecycle predicate.

**Current behavior:** The worker exit has no reviewer invocation seam, and nothing host-side reaps an orphaned in-flight review.

**Desired behavior:** The worker-exit boundary gains the reviewer invocation as one additive amendment of the verification-gate class — the invocation slot only, with the drive-side rows byte-untouched — and the invocation is host configuration whose absence is the prior behavior; the amendment joins the shared verification-gate integration wave when phase alignment holds and otherwise lands as a separate additive amendment of the same class; the host janitor uses the lifecycle in-flight predicate to reap an orphaned review and re-invoke it, so the reap executes host-side while the predicate and timeout remain defined by the reviewer.

**Scope:**
- the additive verification-gate-class amendment adding only the reviewer invocation slot at ladder-green
- the drive-side contract left byte-untouched and the host-config on/off with absence as prior behavior
- the host janitor's use of the lifecycle predicate to reap and re-invoke an orphaned review

**Out of scope:**
- the reviewer runtime the seam invokes (T4) and the predicate it reuses (T9)
- the worker fix loop, caps, and other janitors owned by implement-now

**Acceptance criteria:**
- [ ] the seam lands as one additive verification-gate-class amendment adding only the invocation slot, with the drive-side rows byte-unchanged
- [ ] the invocation is host configuration whose absence reproduces the prior behavior exactly
- [ ] the host janitor reaps a seeded orphaned in-flight review through the lifecycle predicate and re-invokes it

**Key interfaces:**
- the verification-gate-class invocation slot at the worker's ladder-green exit
- the host-config on/off toggle for the invocation
- the host janitor's reap over the lifecycle in-flight predicate

**Traces:** prd:impl-review/FR-1 · prd:impl-review/UB-8 · prd:impl-review/NR-2

### T15 — The living rollup, glossary terms, and the review reference

| Field | Value |
|---|---|
| **Blocked by** | T1, T7 |
| **Size band** | 40–100 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | docs/techspec.md · docs/wiki/glossary.md · docs/wiki |

**Goal:** The shipped reviewer is legible without tooling — the living techspec rollup carries the capability, the glossary defines its two terms, and one reference documents the report, verdict, and finding schema with the triage states.

**Current behavior:** The living rollup does not yet carry the reviewer, the glossary lacks its terms, and no reference documents the report schema, verdict set, or the review-absent, review-stale, dispute, and escalation states.

**Desired behavior:** The living techspec rollup gains the impl-review capability once its code ships; the glossary defines the two terms this feature introduces; and one reference documents the report, verdict, and finding schema together with the review-absent, review-stale, dispute, and escalation triage notes, so a human reads a persisted review's meaning without external tooling.

**Scope:**
- the living techspec rollup entry for the shipped reviewer capability
- the two glossary term definitions this feature introduces
- the single reference over the report, verdict, and finding schema and the triage-state notes

**Out of scope:**
- the schema, verdict, and persistence surfaces the reference documents (T1, T2, T7)
- any normative rule — the reference is descriptive and the payload governs on conflict

**Acceptance criteria:**
- [ ] the living techspec rollup carries the impl-review capability entry once the code ships
- [ ] the glossary defines the two terms this feature introduces
- [ ] one reference documents the report, verdict, and finding schema and the review-absent, review-stale, dispute, and escalation states

**Key interfaces:**
- the living techspec rollup capability entry
- the two glossary term definitions
- the descriptive review reference covering the report, verdict, and finding schema and the triage-state notes

**Traces:** prd:impl-review/FR-6 · prd:impl-review/FR-13

## Coverage

| Requirement | Tasks |
|---|---|
| prd:impl-review/FR-1 | T4, T14 |
| prd:impl-review/FR-2 | T3, T4 |
| prd:impl-review/FR-3 | T3, T4, T5 |
| prd:impl-review/FR-4 | T5, T12 |
| prd:impl-review/FR-5 | T1 |
| prd:impl-review/FR-6 | T2, T15 |
| prd:impl-review/FR-7 | T7 |
| prd:impl-review/FR-8 | T8 |
| prd:impl-review/FR-9 | T6 |
| prd:impl-review/FR-10 | T6, T12 |
| prd:impl-review/FR-11 | T13 |
| prd:impl-review/FR-12 | T5 |
| prd:impl-review/FR-13 | T11, T15 |
| prd:impl-review/FR-14 | T10 |
| prd:impl-review/FR-15 | T8 |
| prd:impl-review/FR-16 | T11 |
| prd:impl-review/FR-17 | T7, T8, T10 |
| prd:impl-review/UB-1 | T9 |
| prd:impl-review/UB-2 | T8 |
| prd:impl-review/UB-3 | T5, T12 |
| prd:impl-review/UB-4 | T8 |
| prd:impl-review/UB-5 | T2 |
| prd:impl-review/UB-6 | T7 |
| prd:impl-review/UB-7 | T6 |
| prd:impl-review/UB-8 | T9, T14 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:impl-review/WR-1 | The charter-free remote twin for human and bypass PRs — a waiting-room item revisited at the remote phase, aligned with codegate's remote surface; out of this contract's scope. |
| prd:impl-review/WR-2 | The stage-nine presentation surface (read-to-rule for proposals) — its own feature in the gate-attention lineage, once the findings corpus this plan produces exists. |
| prd:impl-review/WR-3 | Mirroring dispositions into the review-feedback-capture ledger — a future derived mirror off the harvest (T11), never the source; the harvest stays authoritative. |

