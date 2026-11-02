# Argus attribution hardening — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Make the Argus attribution gate bite in practice: mechanically produce Argus authorship on every drive commit, make it forgery-resistant in strength (not only in direction), and keep absence fail-closed on protected paths without ever refusing a legitimate human author. |
| **Relevant-when** | brainstorming or contracting how the attribution gate identifies an Argus-authored diff, how that authorship signal is produced/authenticated, or how absence is handled on protected paths; checking prior rulings about attribution-signal production, forgery-resistance strength, or the human-author path (NR-4) at the merge boundary |
| **Idea** | The boundary-enforcement verification found the attribution gate — the layer meant to refuse, at the merge boundary, an Argus-authored diff that touches a protected path (the authority contract, a governance document, or an out-of-declared-scope file) — does not actually bite in practice. Three gaps: (1) LATENT — the gate keys on an `Argus-Drive` commit trailer nothing produces (not the drive, skill, git hook, or worker), so real Argus commits carry none, classify as Human, and are served; (2) UNAUTHENTICATED — the trailer is a plaintext, author-writable line with no forgery resistance (the design's own §7 Q1); (3) ABSENT-MASQUERADES — an absent trailer on a protected-path diff is served as Human (DINV-2 as built), so stripping it looks like a legitimate human edit, while the legitimate human-author path (NR-4) must genuinely stay open. Make the gate a real, unbypassable merge-boundary guarantee: authorship mechanically produced (unbypassable), forgery-resistant, fail-closed on absence for the protected subset — without closing the human-author path. Evolve the frozen attribution-gate design and the boundary-enforcement PRD by supersession, never edit them. |
| **Operator** | Argus (AI Engineer, decision principal at tier-2 — authoring this record as the stage-1 decision authority; @mdarabi is the problem provider and the human backstop who overturns through the normal channels) |
| **Date(s)** | 2026-07-13 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | brainstorm:argus-boundary-enforcement, brainstorm:argus-boundary-enforcement-tech-design |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/tech-design/argus-attribution-gate.md | the frozen subject design being evolved — its trailer derivation (§3.1), fail-closed rule (§3.3), DINV set, and §7 Q1 (forgery-strength) are the exact matter |
| docs/prds/argus-boundary-enforcement/effective.md | the governing PRD contract — FR-9/FR-10/FR-11 (the attribution-gate cluster), NR-4 (human-author path), ABUSE-3, DOD; the behavior this feature must preserve and make real |
| docs/reviews/argus-attribution-gate.md | the prior design review — its FND-2 (advisory) already flagged that the bare trailer carries no intrinsic forgery-resistance and routed the property to DINV-2, not the mechanism |
| docs/tech-design/argus-agent-v1.md | the Argus behavior contract — checked for any commit-authoring / trailer-emission step; there is none (the latent-signal root) |
| docs/tech-design/argus-artifacts.md | the Argus data contract — confirms Argus authors ledgers/documents (files), and the drive's own DINV set; no trailer producer |
| the Argus drive guide (`argus` authoring guide) | confirms the drive authors files and prepares proposals; git/PR mechanics are the supervisor's, so no drive step stamps a commit |
| docs/product-vision.md | governance-tier scope check — §1 (the engineer's two touches), §6 (trust model / earned autonomy), §7 (document mutability) |
| docs/architecture-principles.md | the principle registry the destination must honor — AP-10 (codify over prompt), AP-13/AP-14 (verified artifact / verify by execution), AP-17 (fail closed), AP-25/AP-26 (vendor-neutral / badge), AP-27 (extension by addition) |
| docs/observability/incidents.md | paid-for lessons — I1 (bootstrap: a class's first gate cannot gate its own introducing PR) governs this feature's own rollout |
| docs/wiki/glossary.md | prior term landings — attribution gate, protected path, declared scope, structure arm (checked for term conflicts) |

**Overlaps surfaced:**
- brainstorm:argus-boundary-enforcement (the parent feature) · ruled the attribution gate is one of three enforcement layers keying on *Argus-authored* diffs (its D-5), with the mechanism deferred to design (its D-2, O-1). This feature does **not** re-open that framing — it repairs the *derivation* the frozen design chose. No conflict: the parent contracted the *behavior* (FR-9/10/11); this feature makes the derivation actually deliver it.
- docs/reviews/argus-attribution-gate.md#FND-2 · the review already recorded (advisory) that the bare trailer carries no intrinsic forgery-resistance and that the evade-direction resistance lives in the fail-closed-on-absence rule, not the trailer. This feature elevates that owned residual + §7 Q1 from "refinement carried forward" to "the defect to close" — consistent with, not contradicting, the frozen design's own honest ownership.
- docs/tech-design/argus-attribution-gate.md §7 Q1 (forgery-strength) and the *unstated* trailer-producer assumption · the frozen design assumed a producer exists ("the Argus drive, when it authors a commit, appends an authorship trailer") but no such step exists in the behavior contract; and it carried the strength as an open question. Both are this feature's subject. No prior ruling *closed* either — so this is new ground, not a duplicate.
- No merged capability or NG/WR row refuses this work. NG-3 of the parent PRD (no authority-contract mutation by the enforcement feature) is a constraint this feature inherits and honors, not an overlap that blocks it.

## Framings

- **A — Repair the derivation of the existing gate (a hardening supersession).** The subject is unchanged (the attribution gate; FR-9/10/11's behavior); only *how Argus authorship is derived and how absence is handled* is wrong. Thesis: this is one feature that evolves the frozen part-3 design by a superseding design, closing the three named gaps against the same contracted behavior. Implications: no new PRD requirement *area*; the boundary-enforcement PRD is evolved by a typed amendment where its FR/DOD text must sharpen (the derivation property was under-specified); the frozen design is superseded, never edited. Opens: what "mechanically produced" and "forgery-resistant strength" mean as *product* requirements (stage 3 picks mechanisms).
- **B — A new cross-cutting "agent authorship provenance" primitive.** Thesis: Argus authorship is one instance of a general need — every agent's commits should carry a produced, authenticated provenance signal — so build a repo-wide provenance primitive, of which the attribution gate is a consumer. Implications: much larger scope; pulls in other agents (the fleet workers), a general trailer/signing standard, and vendor-CLI commit interception broadly. Opens: a program, not a feature.
- **C — Drop the trailer; derive Argus authorship from something already authenticated.** Thesis: instead of producing and authenticating a new signal, key authorship off an existing authenticated artifact (e.g. the drive's own gate-protected delivery-ledger journal, or the identity under which the supervisor commits). Implications: leans on the already-rejected ALT-5 (ledger cross-reference is prose-granular, not per-diff) or on a code-host identity (the already-rejected ALT-4 badge, credential-bearing). Opens: re-litigating settled alternatives.

**Destination:** **Framing A — repair the derivation of the existing gate, as a hardening supersession.** Rationale: the idea explicitly directs "evolve by supersession, never edit," and the subject (the attribution gate and its contracted behavior) is stable and correct — the defect is confined to the *derivation and absence-handling*, exactly the span a supersession replaces. B is a genuinely distinct, larger bet (a provenance primitive) that this problem does not ask for and whose scope would swallow the fix; it is recorded as a non-goal/waiting-room seed, not chosen. C re-opens ALT-4/ALT-5, both rejected in the frozen design on standing grounds (credential surface; prose-granularity) — but its *insight* (prefer an already-authenticated source over a hand-produced one) is a live stage-3 input and is carried as fog, not resolved. Framing A keeps the blast radius at one feature and honors the frozen-doc channel.

## Decisions

### D-1 — one feature or a split?

- **Question**: Are the three gaps (latent signal, unauthenticated trailer, absence-masquerade) one feature, or do they split into separate features/PRDs?
- **Answer**: One feature — a hardening of the attribution gate's derivation, delivered as a superseding design (plus the minimal PRD amendment its under-specified derivation property needs). The three gaps share one subject and one failure (the gate does not bite) and are only separable as sub-questions the *design* answers.
- **Reasoning**: The subject is a single security mechanism (the attribution derivation); the three gaps are three faces of "the produced/authenticated/absence-handled authorship signal is unsound." Splitting fragments one coherent security goal at the contract layer — the same argument the parent PRD used to keep its three layers in one PRD (Framing A there). RW-5's "one decision per document" applies at the *design* layer (the superseding design makes one decision: how Argus authorship is produced, authenticated, and absence-handled), not by fracturing the feature.
- **Anchor**: 2026-07-13 · destination + opening grill (this session)
- **Status**: live

### D-2 — what is in scope; what is the subject?

- **Question**: What exactly does this feature change, and what does it leave untouched?
- **Answer**: In scope: (a) a **mechanically-produced** Argus-authorship signal on every commit a drive's work lands as — produced by construction, not by the model choosing to add a line; (b) **forgery-resistance of the signal in strength**, closing §7 Q1's evade direction so a mis-behaving/injected drive cannot forge or omit it to pass; (c) an **absence rule that stays fail-closed on the protected subset** (an un-attributed diff touching a protected path refuses) *while the legitimate human-author path stays fully open*. Out of scope: the protected-path classification itself (unchanged — authority contract / governance doc / out-of-scope), the committed-ledger and first-PR slug-grammar scope-width derivation (already sound per the frozen design + operator ruling A-1 of the parent), the write hook and structure arm (their own designs), and any authority-contract mutation.
- **Reasoning**: The idea names exactly these three gaps and names the classification/scope-width machinery as sound prior art to preserve. Keeping classification and scope-width out of scope prevents re-litigating closed decisions (the parent's FND-1/FND-3 resolutions) and keeps the blast radius at the derivation.
- **Anchor**: 2026-07-13 · scope grill (this session)
- **Status**: live

### D-3 — primary segment and anti-segment

- **Question**: Who is this for, and who must it explicitly not impede?
- **Answer**: Primary: **the operator** (P2 of the parent) — who must be able to trust that a forbidden Argus write is *mechanically* refused at the merge boundary, which today it is not (the gate is latent). Secondary: **Argus** (P1) — whose boundaries should hold by construction. Anti-segment (unchanged, and the hard guardrail): **a human author editing a protected path by an ordinary human-authored PR** — this path must never be refused by the gate, and the produced-authorship mechanism must not require a human to do anything to be recognized as human.
- **Reasoning**: The parent PRD already fixed these segments; this feature inherits them verbatim. The novel emphasis is that the anti-segment now bounds *two* things: the absence rule (absence must fail closed on protected paths yet a human PR, which legitimately carries no Argus signal, must pass) and the production mechanism (it must not stamp or burden human commits).
- **Anchor**: 2026-07-13 · segment grill (this session)
- **Status**: live

### D-4 — the goals direction (the bet) and its observable success

- **Question**: What is the falsifiable bet, and how is success observed?
- **Answer**: The bet: **if Argus authorship is produced by construction and authenticated in strength, and absence fails closed on protected paths, then the attribution gate becomes a real unbypassable merge-boundary guarantee — a forbidden Argus protected-path change cannot reach the mainline, and no legitimate human PR is ever refused.** Observable success (the definition of done at product altitude): on a corpus that includes *real drive-produced commits* (not hand-stamped fixtures), the gate refuses 100% of Argus-authored protected-path diffs; a stripped/forged-signal fixture on a protected-path diff refuses (never passes as human); 0 human-authored PRs to protected paths are refused; and — the gap-1 closer — a genuine end-to-end drive of a protected-path write is refused *by the gate*, proving the signal is actually produced, not assumed.
- **Reasoning**: This restates the parent's G1/G2 but adds the load-bearing new evidence the parent lacked: a *produced-signal* proof (an Argus drive actually lands the signal) and a *forgery-strength* proof (a forged signal cannot flip the class). The parent's M1/M2 measured hand-seeded fixtures; the latent-signal gap means those fixtures passed while the live signal was absent — so the success bar must include a live-produced signal.
- **Anchor**: 2026-07-13 · goals grill (this session)
- **Status**: live

### D-5 — the "produced-signal" boundary: PRD requires the property, design owns the mechanism

- **Question**: Does the PRD/brainstorm settle *how* the signal is produced (git hook vs harness stamping vs identity-derived) and *how* it is authenticated (cryptographic binding vs signed commits vs an existing authenticated artifact)?
- **Answer**: No — the PRD-altitude requirement is the *property* (produced-by-construction, forgery-resistant-in-strength, fail-closed-on-absence-without-closing-the-human-path); the *mechanism* is a stage-3 (technical-design) decision. This mirrors the parent's D-2 exactly: the security posture is the operator's ruling (the idea); the mechanism is design-owned and operator-reviewed at stage 5.
- **Reasoning**: prd-template §0's ownership boundary: every requirement is observable behavior; the mechanism is the design's. Settling the mechanism here would (i) pre-empt the design's own alternatives analysis and (ii) risk assuming a mechanism that needs a new grant/outward surface — which would be a *breaking* decision at *that* stage, not this one. So the mechanism questions are surfaced as non-blocking fog (G-1..G-4), not resolved.
- **Anchor**: 2026-07-13 · mechanism-ownership grill (this session)
- **Status**: live

### D-6 — the appetite / spend bound

- **Question**: How large is this feature; what bounds the spend?
- **Answer**: Bounded and small. This is a *repair* of an existing frozen mechanism, not a new subsystem: one superseding technical design (replacing the derivation span of the frozen design), the minimal boundary-enforcement PRD amendment its under-specified derivation property needs (FR-11 and the DoD sharpen; the classification/scope-width FRs are untouched), and the implementation the design specifies (a produced-signal mechanism + its golden corpus, including a live-drive proof). Human attention: operator review of the PRD amendment, the superseding design, and its plan. Excludes: any new egress/identity/outward surface beyond what the chosen mechanism's design justifies-and-the-operator-approves; a general cross-agent provenance primitive (that is Framing B, waiting-room).
- **Reasoning**: The subject is confined to the derivation; the classification, scope-width, and the sibling layers are untouched. This is the same "additive over a settled substrate" appetite the parent took, scaled down to one arm's derivation.
- **Anchor**: 2026-07-13 · appetite grill (this session)
- **Status**: live

### D-7 — the frozen prior art: amend the PRD, supersede the design

- **Question**: How does this feature relate mechanically to the two frozen governing documents (the attribution-gate design and the boundary-enforcement PRD)?
- **Answer**: The frozen **design** (`argus-attribution-gate`) is evolved by a **superseding design** — a new document that `Supersedes` it and re-decides the derivation (production + authentication + absence handling); the frozen design is never edited. The frozen **PRD** (`argus-boundary-enforcement`) is evolved by a **typed amendment** to the extent its contracted derivation property was under-specified (FR-11 states "without relying on a mutable badge and without requiring ambient credentials" but does not require the signal to be *produced by construction* nor bound its *forgery-strength*; the DoD's fixture rows did not require a live-produced-signal proof — these are the amendment's targets). Its classification/scope-width FRs (FR-9's protected-path set, DINV-6) are untouched.
- **Reasoning**: The document-system rules are absolute: frozen designs evolve by supersession (a new doc citing the old), frozen PRDs by typed amendments folded into the projection (AP-7; the repo's mutability gate). The idea directs exactly this. Whether the PRD amendment is *needed* (vs the property already being adequately implied) is a stage-2/stage-3 judgment; this record flags that the derivation-property text is the likely amendment surface and hands the precise amendment scope to those stages.
- **Anchor**: 2026-07-13 · prior-art channel grill (this session)
- **Status**: live

### D-8 — the feature's own bootstrap: the produced-signal has no history before it ships

- **Question**: How does this feature's own introducing change handle the fact that the produced-authorship signal does not exist on commits authored before the mechanism ships?
- **Answer**: This is a real product concern the design must answer, surfaced here as a decision the feature must carry (not resolved mechanistically): the gate must behave correctly across the transition — pre-existing history carries no produced signal, and the rollout must not (a) suddenly fail-closed-refuse the mainline's whole history nor (b) leave a window where the gate is still latent. The likely shape (design-owned) is the parent's own incident-I1 posture — the introducing PR is operator-reviewed as its own backstop, and the gate binds every subsequent PR — plus a clear rule for how absence is interpreted on pre-mechanism commits vs post-mechanism ones. Recorded as a decision-shaped concern; the mechanism is G-4.
- **Reasoning**: Incident I1 is a paid-for lesson: a class's first gate cannot gate its own introducing PR. The latent-signal repair has an analogous transition (the signal begins existing at a point in history). Naming it now prevents the design from shipping a rule that either false-refuses history or re-introduces a latent window.
- **Anchor**: 2026-07-13 · rollout grill (this session)
- **Status**: live

### D-9 — does resolving gap 1 mean the human-author path can no longer be distinguished from a stripped Argus signal?

- **Question**: The idea's tension: absence must fail closed on protected paths (so a stripped Argus signal cannot masquerade as human), yet a legitimate human PR to a protected path also carries no Argus signal and must pass. Is this a genuine contradiction that must be escalated, or a resolvable product requirement?
- **Answer**: Resolvable, not contradictory — and it is a *product requirement*, not a mechanism, so it does not escalate. The requirement is: **the gate must be able to positively recognize a legitimate human-authored diff as human** (not merely infer "human" from the *absence* of an Argus signal). The moment "absence ⇒ human" is how the masquerade works; the fix is that human authorship, like Argus authorship, must be *positively established* on the protected subset — so that on a protected path the three states are distinguishable: positively-Argus (refuse), positively-human (pass, NR-4), and neither/underivable (fail closed). How human authorship is positively established (a produced human signal, an authenticated committer identity, a verified human-review attestation, …) is the design's decision (G-3).
- **Reasoning**: This is the crux the idea surfaces, and it is decidable at the product altitude *as a property* without picking the mechanism. The frozen design's DINV-2 ("underivable ∧ protected-path ⇒ fail closed") plus DINV-4 ("human-author path open") are consistent *only if* human is positively derivable on the protected subset — otherwise a human PR is "underivable" and DINV-2 refuses it, breaking NR-4. So the product requirement "human authorship is positively established on the protected subset" is what makes the two frozen invariants co-satisfiable. No governance boundary moves; no human ruling is overridden; NR-4 is *strengthened* (positively served, not inferred). Legal by all three tests → assumption, not escalation.
- **Anchor**: 2026-07-13 · absence-vs-human grill (this session)
- **Status**: live

## Facts

- **F-1**: The Argus behavior contract (argus-agent-v1) defines the drive as authoring *files* and preparing proposals; it contains no commit-authoring or authorship-trailer-emission step. Source: docs/tech-design/argus-agent-v1.md (grepped for commit/trailer — the only "commit" hits are resume-verification against a commit's goal-object, never authoring one).
- **F-2**: Git/PR mechanics (commit, push, branch, PR, merge) are the supervisor's, not the drive's. Source: the Argus drive guide ("Drive proposals home … landed means mainline-content-verified") + the drive's hard constraint that the supervisor does all git/PR mechanics. Consequence: the trailer's assumed producer ("the Argus drive, when it authors a commit, appends …", attribution-gate design §3.1) has no counterpart in the actual commit-authoring actor.
- **F-3**: The frozen attribution-gate design already states, in its own binding prose, that the bare trailer "carries no intrinsic forgery-resistance (a plaintext, author-writable line)" and routes the evade-direction resistance to DINV-2 (fail-closed-on-absence), not the trailer. Source: docs/tech-design/argus-attribution-gate.md §3.1 forgery-resistance bullet, §3.8 ABUSE-3, §5 R1.
- **F-4**: The prior design review recorded this as advisory FND-2 (re-attribute the forgery-resistance; the bare trailer is not forgery-resistant) and the design carried the *strength* as §7 Q1 (open, "decided at implementation/review against the trust model; the FR-11 property is required at DoD, the strength is this refinement"). Source: docs/reviews/argus-attribution-gate.md#FND-2; docs/tech-design/argus-attribution-gate.md §7 Q1.
- **F-5**: The frozen design's DINV-2 refuses an underivable author on a protected-path diff, and DINV-4 keeps the human-author path open by asserting a determinable human change "never triggers" the fail-closed rule. Source: docs/tech-design/argus-attribution-gate.md §3.6 DINV-2, DINV-4. (This is only self-consistent if human authorship is positively derivable on the protected subset — the crux D-9 names.)
- **F-6**: The boundary-enforcement PRD contracts FR-11 as "determine Argus authorship without relying on a mutable platform badge and without requiring ambient credentials at evaluation time," with the *mechanism* design-owned (its R4). It does **not** contract "produced by construction" nor bound forgery-*strength*, and its DoD fixture rows (DOD-1/3) verify hand-seeded fixtures, not a live-produced signal. Source: docs/prds/argus-boundary-enforcement/effective.md FR-11, DOD-1/DOD-3, §6.3 R2.
- **F-7**: The protected-path classification and the scope-width derivation (committed-ledger for later PRs; slug-derived fixed set ∪ static allowlist for the first PR) are already reviewed sound (parent review approve, post-escalation, under operator ruling problem:argus-boundary-enforcement#A-1). Source: docs/reviews/argus-attribution-gate.md post-escalation re-review. Consequence: this feature leaves them untouched.
- **F-8**: Incident I1 is the paid-for lesson that a class's first gate cannot gate its own introducing PR; operator review is the sole backstop for that one bootstrap PR. Source: docs/observability/incidents.md I1.

## Fog

- **G-1** (non-blocking — blocks: stage-3 technical design): *How* the Argus-authorship signal is mechanically produced so no drive path can land an un-attributed commit — candidates include a supervisor/harness commit-time stamp (since the supervisor owns commit authoring, F-2), a vendor-CLI pre-commit/commit hook, or deriving authorship from the identity the supervisor commits under. The producer must sit where commits are actually authored (not inside the drive prose, which authors files). This is the direct gap-1 closer and is design-owned (D-5).
- **G-2** (non-blocking — blocks: stage-3 technical design): The forgery-resistance *construction* that gives the signal real strength (§7 Q1's evade direction) — e.g. a cryptographic binding to verifiable commit content, signed commits under a controlled key, or an attestation the CI verifies — such that a mis-behaving/injected drive cannot forge or replay it. Must remain credential-free/badge-independent *at CI evaluation time* per FR-11/AP-14/AP-26 (the verification reads in-tree, even if signing happens at authoring time under a controlled key). Design-owned (D-5).
- **G-3** (non-blocking — blocks: stage-3 technical design): How **human** authorship is *positively established* on the protected subset (the D-9 crux) so the three states (positively-Argus / positively-human / underivable) are distinguishable — candidates: a positively-derivable committer identity, a human-review attestation, or an authenticated non-Argus signal — without burdening the human author (the anti-segment must not have to do extra work to be recognized). Design-owned; it is the mechanism half of NR-4's positive service.
- **G-4** (non-blocking — blocks: stage-3 technical design + rollout): The transition/bootstrap rule (D-8) — how the gate interprets absence on pre-mechanism history vs post-mechanism commits so the rollout neither false-refuses the mainline's history nor leaves a latent window; likely the incident-I1 posture plus a from-here-forward rule. Design/rollout-owned.
- **G-5** (non-blocking — blocks: a future program, not this feature): Whether a general cross-agent authorship-provenance primitive (Framing B) is warranted once other agents (the fleet workers) also need produced/authenticated provenance — a WR seed, explicitly deferred.

## Out of scope

- **O-1**: The protected-path *classification* (which paths are protected: authority contract / governance doc / out-of-scope) is not changed — it is sound prior art (from D-2). Reason: the defect is in the derivation, not the classification; changing it would re-open closed decisions and widen blast radius.
- **O-2**: The scope-*width* derivation (committed-ledger for later PRs; slug-derived fixed set ∪ static named allowlist for the first PR) is not changed (from D-2). Reason: already reviewed sound under operator ruling problem:argus-boundary-enforcement#A-1 (F-7); re-opening it is out of this problem's ask.
- **O-3**: The write hook (part 1) and the authority-contract structure arm (part 2) are not changed (from D-2). Reason: they are independently-revisable decisions with their own frozen designs; this feature is confined to part 3's derivation.
- **O-4**: Any authority-contract mutation, tier change, or self-grant by this feature (from D-2, inheriting parent NG-3 and the never-list). Reason: this feature *enforces* the boundary; it must never itself become a path that writes the authority contract or grants authority.
- **O-5**: A general cross-agent authorship-provenance primitive (Framing B) (from D-1 destination). Reason: a larger, distinct bet not asked for here; deferred to a waiting-room entry (G-5) so the fix stays one feature.
- **O-6**: Picking the specific production/forgery-resistance/human-recognition *mechanism* here (from D-5). Reason: mechanism is a stage-3 technical-design decision, operator-reviewed at stage 5; settling it at the PRD altitude breaches the ownership boundary and risks assuming a new-surface mechanism.

## Candidate glossary terms

- **T-1**: Produced authorship signal — an Argus-authorship marker generated *by construction* at the point commits are authored (not chosen by the model, not hand-added), so every commit a drive's work lands as carries it. · avoid: "trailer" (names one candidate mechanism — a commit-message line — not the concept; the produced signal may be a signature, an attestation, or a trailer), "Argus-Drive trailer" (the specific latent mechanism this feature repairs).
- **T-2**: Forgery-resistant in strength — the property that a mis-behaving or injected drive *cannot* forge, replay, or omit-to-masquerade the authorship signal (§7 Q1's evade direction closed), as distinct from forgery-resistant in *direction* (repo-internal, credential-free, badge-independent — already held by the frozen derivation). · avoid: "forgery-resistant" unqualified (the frozen design uses it for the direction; this feature adds the strength).
- **T-3**: Positively-established authorship — deriving a diff's author as *positively Argus* or *positively human* on the protected subset, rather than inferring "human" from the *absence* of an Argus signal; the three-state distinction (positively-Argus / positively-human / underivable) that lets absence fail closed without refusing a legitimate human PR. · avoid: "attribution" alone (too broad — this is the positive, three-state form the human-author guardrail needs).

## Parked technical constraints

- **P-1**: The verification must stay **credential-free and badge-independent at CI evaluation time** (FR-11; AP-14 deployment-context; AP-26). A signing/attestation mechanism may use a controlled key at *authoring* time, but the *gate's read* must remain an in-tree verification with no ambient credential — the same constraint that rejected the code-host-badge alternative (ALT-4). Conveyed to stage 3.
- **P-2**: The production mechanism must be **vendor-neutral** (AP-25) — authored once on the vendor-neutral surface, wired per commit-authoring path — since commits may be authored via different agent CLIs / the supervisor; no CLI-specific content fork. Conveyed to stage 3 (and it interacts with the write hook's own per-CLI surface, WR-2 of the parent).
- **P-3**: Where the gate composes as a check, it must remain **file-not-line self-registered** (AP-27) — no hand-edited shared enumeration/dispatch line — matching the frozen design's §3.4 composition seam. Conveyed to stage 3.
- **P-4**: The absence rule change must preserve **byte-identical verdicts on every non-protected, non-Argus tree** (the parent's NR-1 / M2 guardrail) — a produced-signal or human-recognition mechanism must not alter any unrelated gate verdict. Conveyed to stage 3.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-13 |
| **Operator** | Argus (decision principal at tier-2; authority-contract/v1) |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |

---
