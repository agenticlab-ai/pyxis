# Review: argus-attribution-gate — design-review report

| Field | Value |
|---|---|
| **In one line** | **Approve** (post-escalation re-review, 2026-07-12): the operator ruling (problem:argus-boundary-enforcement#A-1) replaced the escalated first-PR fallback with a **slug-derived fixed-path grammar ∪ a static named allowlist** that reads **no same-PR self-attribution and no author-controlled input**, so FND-3's "widen your own scope" vector is **genuinely closed** (a first-PR Argus can no longer self-widen to a code path — naming a broader slug buys only more of that one slug's own `docs/**` subtree, and a same-PR self-attributing ledger entry buys nothing); the sole remaining residual — the allowlist's own trust — is **honestly owned** (a static author-independent named set, its members reviewed when the list changes, never labeled fail-closed over an author-controllable widen); the committed-ledger branch is unchanged and still sound, no over-block of legitimate stage-1–2 authoring, and the allowlist members (glossary; the generated tech-design index) are **not** themselves governance/protected paths, so the allowlist re-opens no hole. No new blocking issue; both gate paths green. *(Escalate was the frozen round-2 verdict; this is a fresh moment on the materially-revised design under the new ruling.)* |
| **Relevant-when** | auditing or consuming the review of argus-attribution-gate; asking why it was bounced; tuning tech-design-review calibration |
| **Subject** | design:argus-attribution-gate · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-07-12 (round 1, round 2 = escalate); 2026-07-12 (post-escalation re-review) |
| **Verdict** | **approve** (post-escalation re-review — the history below is authoritative; the round-2 escalation was answered by the operator ruling problem:argus-boundary-enforcement#A-1, and this fresh review moment verifies the resolution closes FND-3) |
| **Tracking** | the subject's introducing PR, once filed |
| **Template version** | design-review-report/v1 |

## 0. Scope note

*Advisory period: this verdict blocks nothing; the findings accumulate as
measurement toward the ten-review ship checkpoint. Subject verified: a
post-lint draft technical design (`lint-tech-designs` exit 0 on the
subject), shape `none` (recorded in the header, matching the subject's own
declaration). RECORD-FED: basis
`docs/brainstorms/argus-boundary-enforcement-tech-design.md`
(gated ready-for-design, riding this PR); record-fidelity checks applied.
The floor is excluded — nothing `lint-tech-designs` enforces (header
schema, Serves resolution, mandatory sections) is a finding here. Four
lenses generated in parallel; findings survived an adversarial pass and
carry the five elements + a `CAL-n` citation. Not findings, by ruling: the
operator's accepted R4 judgment (delivery:argus-boundary-enforcement#E-15);
FR-11's property-vs-mechanism-*strength* split carried as §7 Q1 (intended,
the design is honest that the DoD asserts the *property*); the glossary
(DOD-8) and I1 bootstrap-note (DOD-7) PR-time handoffs.*

## Round 1

### FND-1 — the out-of-scope class derives its boundary from the author-writable trailer, an evade path with no independent backstop

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§3.2 — "**Out of declared scope** — a file outside the paths the driving problem's scope permits … The gate **derives the driving problem from the authorship trailer** and treats any Argus-authored change outside that problem's declared scope as a protected-path hit." The scope *boundary itself* (which problem's paths are permitted) is read from a field inside the trailer, and design:§3.1 states the trailer is Argus-appended and "names the driving problem." — vs — the design's own coherence claim that spoofing is bounded, design:§3.8 ABUSE-3: "Mitigated by DINV-3 … and by the structure arm (part 2), which fails a structurally-illegal authority contract regardless of author, **closing the *authority-contract* evade path even if the trailer is spoofed**." That backstop is scoped to the authority-contract class only; the structure arm (part 2, `argus-authority-contract-structure-arm`) does **not** cover the out-of-scope class, and design:§1/§3.8 name out-of-scope as one of the two classes for which this gate is the **sole** merge-boundary guarantee. A mis-behaving/injected Argus authoring the commit controls the trailer's problem field: it can name a **different, broader problem slug** whose declared scope *includes* the target path, so an out-of-scope write classifies as in-scope and **passes**. The fail-closed-on-absence rule (design:#DINV-2) does not catch this — the trailer is *present and derivable*, it merely names a self-serving problem — so unlike the authorship-*presence* residual (owned at §7 Q1), this **problem-field-trust** residual is owned nowhere: absent from §3.8's threat model, the §5 risk table, and §7. The evasion contradicts the design's boundaries-vs-Decision coherence exactly where FR-11's guarantee is load-bearing.
- **Acceptance condition**: Close the out-of-scope evade path or own it honestly, one of: (a) derive the out-of-scope boundary from a source not controlled by the same author-writable trailer — e.g. elevate the ALT-5 delivery-ledger cross-reference (design:§3.1 "Corroboration (optional)") from *optional corroboration* to **load-bearing for the scope-derivation**, cross-checking the trailer's problem field against that problem's attributed journal so a self-named slug cannot widen scope; or (b) add an explicit §7 Q-n (and a §3.8 threat-model line + a §5 risk row) stating that the out-of-scope class's boundary integrity depends on the trailer's problem field, that this class has no independent structure-arm backstop, and scoping the *strength* of the out-of-scope guarantee honestly — mirroring how §7 Q1 owns the authorship-presence residual. Either path also needs an abuse fixture (extending A-4/§3.10) proving a widened-problem-slug trailer does not flip an out-of-scope write to in-scope.
- **Disposition**: applied — path (a), with a stated residual *(Argus, tier-2 — dispose-blocking-finding)* — the finding is a real security hole: the trailer's problem field is author-writable, so a mis-behaving Argus could name a broader slug whose scope includes the target and pass an out-of-scope write, with no structure-arm backstop and DINV-2 blind to it (trailer present, self-serving). **Fixed** by elevating the ALT-5 delivery-ledger cross-reference from optional corroboration to **load-bearing for the scope-derivation** (new §3.2.1 + new invariant DINV-6 "scope integrity"): the out-of-scope boundary's *width* is derived from the named problem's **gate-protected committed (merged) delivery ledger** — whose `E-n` journal is append-only against the merge base (the shipped ledger-history arm, which I verified makes merged Argus `E-n`/`A-n` add-only). **Does the fix close the hole?** Yes for the merged-ledger case: a current, in-flight drive cannot retroactively widen *any* committed ledger (its own or another problem's), so naming a broader slug buys nothing — either that slug's *merged* ledger genuinely owns the target (the real owner authored it — not an evasion) or the write reds. The self-serving-slug reclassification is structurally unavailable. §3.1 now splits the two trusts (trailer = per-diff authorship signal; committed ledger = scope-width boundary); the mermaid, §3.8 (ABUSE-3′), §5 R6, and A-4 (widen-slug abuse fixture) all carry it. **Residual (stated, per the acceptance condition):** the **first PR of a brand-new problem** has no committed ledger to bound its out-of-scope width — so that case falls back to **minimal-by-construction scope + fail-closed** (admit only positively-placeable paths — the problem's own `docs/problems/<slug>/**` + what the current PR's own gate-checked ledger entries attribute — refuse the rest; §3.2.1(b)). This residual errs toward *refusal* (AP-17), so it can only force a legitimate first-PR write to be split/human-landed — it can **never** widen scope — and the authority-contract + governance classes are untouched (fixed path sets, DINV-1). It is owned as §7 Q4 + §5 R6 + A-7 (first-PR fallback fixture). **R4 check: repo-internal + additive** — the gate already reads in-tree data (diff, trailers); reading the committed delivery ledger (already in-tree, already gate-protected) adds no new grant, credential, or outward surface → **no R4 escalation owed**.

### FND-2 — the bare trailer is labeled "forgery-resistant" in a binding invariant's prose when its named mechanism carries no forgery-resistance

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:#DINV-3 (a binding invariant) + design:§3.8 ABUSE-3: "Mitigated by **DINV-3 (a forgery-resistant, badge-independent, in-tree derivation** — a mutable badge is never trusted)"; design:§5 R1 mitigation: "**DINV-3's badge-independent in-tree derivation**" as a mitigation for forgeability. The prose attributes forgery-resistance to the *derivation* (the trailer read) itself. — vs — the design's own mechanism-of-record and honest core: design:§3.1 picks the bare trailer as the primary+sufficient signal, and §7 Q1 concedes "a **bare in-tree trailer** vs one cryptographically bound to verifiable commit content" is an *open* refinement — i.e. the chosen mechanism is a plaintext, author-writable string with **no** intrinsic forgery-resistance. The property FR-11 names *for the evasion direction* is in fact delivered by design:#DINV-2 (fail-closed-on-absence: a stripped trailer on a protected-path diff refuses, never passes), **not** by DINV-3. Mislabeling the derivation as "forgery-resistant" risks a reader concluding the bare trailer resists forgery, which it does not. Note this is distinct from — and does not re-litigate — the property-vs-strength split the operator accepted: the imprecision is the *attribution* of the forgery-resistance property to DINV-3's mechanism rather than to DINV-2's rule.
- **Acceptance condition**: Re-attribute the forgery-resistance in the binding prose — DINV-3 should claim what its mechanism actually delivers (badge-independent + credential-free; both testable and true, A-3), and the forgery-resistance of the *evasion* direction should route to DINV-2's fail-closed-on-absence rule. Update the §3.8 ABUSE-3 and §5 R1 mitigation lines so no reader reads the bare trailer itself as forgery-resistant. (Advisory: DINV-3's own invariant statement — "reads no mutable badge, no ambient credential" — stays testable and correct as written, so the binding element does not go untestable; hence CAL-3 stays advisory, not escalated.)

- **Disposition**: applied *(Argus, tier-2 — dispose-advisory-finding)* — re-attributed the forgery-resistance across the binding prose. §3.8 ABUSE-3 now routes the *evade* direction to **DINV-2's fail-closed-on-absence** (plus the DINV-6 committed-ledger corroboration for the widen direction), with DINV-3 credited only for badge-independence + credential-freeness — the "forgery-resistant" adjective is removed from the DINV-3 clause. §5 R1's mitigation is rewritten to state the bare trailer carries **no intrinsic forgery-resistance** and that the evade-direction resistance comes from DINV-2 (+ DINV-6), not DINV-3. §3.1's forgery-resistance bullet now states plainly that the bare trailer string is a plaintext author-writable line with no intrinsic resistance, and names where the property actually comes from (DINV-2 for evade, DINV-6 for widen; DINV-3 = badge-independent + credential-free). DINV-3's own invariant statement is left unchanged (already testable and correct, per the finding). No reader now reads the bare trailer itself as forgery-resistant.

## Round 2 — scope-locked

*Scope lock: round 2 examines only (a) whether each round-1 disposition
satisfies its finding's acceptance condition, and (b) regressions within the
changed spans (§3.1 two-trust split + forgery-resistance re-attribution, new
§3.2.1, new DINV-6, §3.8 ABUSE-3′, §5 R1/R6, A-4/A-7, §7 Q4). A finding on
unchanged content is violation-withheld. Gate re-run green both paths (see
Measurements). `lint-tech-designs` exit 0 on the revised draft — floor
excluded as always.*

### FND-1 closure — the committed-ledger derivation: closes the named-broader-slug widen, but the first-PR fallback is not the "never-widen" the disposition claims

- **Disposition assessed**: applied — path (a) + stated first-PR residual.
- **Acceptance condition (round 1)**: *close the out-of-scope evade path or own it honestly*; path (a) = derive the out-of-scope boundary from a source **not controlled by the same author-writable trailer** — elevate the ALT-5 delivery-ledger cross-reference from optional corroboration to load-bearing, cross-checking so **a self-named slug cannot widen scope**; plus an abuse fixture (extending A-4/§3.10) proving a widened-problem-slug trailer does not flip an out-of-scope write to in-scope.
- **Satisfied (the committed-ledger half) — verified on the merits.** For a problem with a **prior merged (committed) delivery ledger**, the disposition is correct and well-built. §3.1 splits the two trusts (trailer = per-diff authorship signal; committed ledger = scope-width boundary); §3.2.1 makes the out-of-scope *width* load-bearing on the named problem's `docs/problems/<slug>/delivery.md` "**gate-protected append-only against the merge base**"; DINV-6 anchors it property-testably; ABUSE-3′, R6, and the A-4 widen-slug fixture carry it. **The security question — does deriving width from the *committed* ledger close "name a broader slug to widen your own scope"? — is answered YES for this branch.** The load-bearing property is real: `docgate`'s `check-ledger-history` arm (verified green on the `--merge-base origin/main` path this round) makes merged Argus `E-n`/`A-n` add-only vs the merge base, so a current, in-flight drive **cannot retroactively widen any committed ledger** — its own or another problem's. Naming a broader slug therefore buys nothing: either that slug's *merged* ledger genuinely owns the target (the real owner scoped it — not an evasion) or the write reds. Same-PR self-widening is excluded on this branch because the derivation reads the **committed** ledger (what is already on main), not this PR's additions. The self-serving-slug reclassification is structurally unavailable. **This half is closed.**
- **NOT fully satisfied (the first-PR fallback) — the "cannot widen scope" promise is broken on the first-PR branch.** The acceptance condition's core promise is that **a self-named source cannot widen scope**. The disposition asserts this *unconditionally*, including for the first PR: §3.2.1(c) "it errs toward refusal (AP-17) … **it can never widen scope**"; DINV-6 "the out-of-scope class falls to the minimal-by-construction scope + fail-closed (admit only positively-placeable paths; refuse the rest)"; §5 R6 "it errs toward refusal, so it can force a split/human-land but **never widen**"; §7 Q4 "the fallback is **safe** — it errs toward refusal, AP-17, **so it cannot widen**." **That claim is false as written**, because §3.2.1(b) *defines* the first-PR positive-scope source to include an **author-controlled, same-PR** input over **arbitrary paths** — see FND-3. The committed-ledger half satisfies the condition; the first-PR half does not, and the disposition's own binding elements over-state its safety. **Partial closure → the residual is not the refusal-only residual the disposition owns; it is also a widen vector (FND-3).**

### FND-2 closure — forgery-resistance re-attribution: satisfied

- **Disposition assessed**: applied *(advisory, CAL-3)*.
- **Acceptance condition (round 1)**: re-attribute the forgery-resistance in the binding prose — DINV-3 claims only what its mechanism delivers (badge-independent + credential-free), and the *evade*-direction forgery-resistance routes to DINV-2's fail-closed-on-absence rule; update §3.8 ABUSE-3 and §5 R1 so no reader reads the bare trailer itself as forgery-resistant.
- **Satisfied — verified.** §3.1's forgery-resistance bullet now states plainly the bare trailer string "**carries no intrinsic forgery-resistance** (a plaintext, author-writable line)" and routes the property: evade → DINV-2, widen → DINV-6, badge-independence + credential-freeness → DINV-3. §3.8 ABUSE-3 routes the evade direction to "**DINV-2's fail-closed-on-absence rule**" with DINV-3 credited only for the badge-independent, credential-free derivation — the "forgery-resistant" adjective is removed from the DINV-3 clause. §5 R1's mitigation now reads "the bare trailer carries **no intrinsic forgery-resistance** … DINV-3 contributes only badge-independence + credential-freeness (not forgery-resistance)." DINV-3's own invariant statement is unchanged and stays testable (badge-independent + credential-free; A-3), as the finding required — the binding element does not go untestable, so CAL-3 stays advisory. **No reader now reads the bare trailer itself as forgery-resistant. Closed.**

### FND-3 — regression (changed span): the first-PR fallback's positive-scope source is author-controlled over arbitrary paths, a live widen vector the disposition claims is impossible

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Scope**: on a changed span — the newly-added §3.2.1(b) first-PR fallback and the newly-added DINV-6 / §3.8 ABUSE-3′ / §5 R6 / §7 Q4 that assert it "can never widen." A regression finding within the disposition's own new content; permitted at round 2.
- **Evidence**: design:§3.2.1(b) (new) defines the first-PR positive-scope source: "the gate admits only the minimal by-construction scope (the problem's own `docs/problems/<slug>/**` ledgers **and the paths the *current* PR's own delivery ledger entries — added in the same PR, themselves subject to the structure/attribution checks — attribute to the drive**), and treats any target it cannot positively place in scope as out-of-scope (refuse)." — vs — design:§3.2.1(c) (new): "it errs toward refusal (AP-17), so it can only force a legitimate first-PR write to be split/human-landed — **it can never widen scope**"; design:#DINV-6 (new): "On a first PR … the out-of-scope class falls to the minimal-by-construction scope + fail-closed (admit only positively-placeable paths; refuse the rest)"; design:§7 Q4 (new): "the fallback is **safe** … **so it cannot widen**." **The contradiction**: on a first PR there is no committed ledger (the honestly-owned empty backstop), so the *only* non-fixed positive-scope source is item 2 — "the paths the current PR's own delivery ledger entries attribute to the drive." Those `E-n` entries are authored **by the same drive, in the same PR**, and the gate's own threat actor for this class is exactly "a mis-behaving or injected Argus [that] controls the trailer's problem field" (design:§3.2.1) — the same actor also authors these ledger entries. The two named checks on those entries do **not** constrain the *target path* an `E-n` may name: the attribution check verifies the entry is Argus-authored (it is — the attacker is Argus), and the ledger structure/history check verifies shape + add-only-vs-base (dense numbering, envelope) — neither validates that an `E-n`'s recorded external-effect path is one the drive legitimately owns (nor could it: on a first PR there is no independent scope anchor to validate against — that circularity is the residual). Item 2 is therefore an **author-controlled, arbitrary-path** positive-scope source. A mis-behaving first-PR Argus authors, in the same PR, an out-of-scope write to e.g. `internal/engine/core.go` **and** a same-PR ledger entry `E-1` attributing `internal/engine/core.go` to the drive → the gate **positively places** `internal/engine/core.go` in scope and the write **passes** as in-scope. This is precisely the "widen your own scope" outcome FND-1 named, relocated from the committed-ledger branch (now closed) to the first-PR branch (open) — the *same threat actor, same sole-backstop class, same pass outcome*. Item 2 is not doc-restricted (§3.2.1(b) says "the **paths**," and the class must cover code paths for the gate to admit legitimate first-PR code writes at all, since Argus at tier-2 materializes code, not only docs — design:§1/why-now), so the vector is general, not a doc-only edge. The design's own §3.2.1(c)/DINV-6/R6/Q4 "never widen / fail-closed / admit only positively-placeable paths" claims are contradicted by §3.2.1(b)'s definition of "positively-placeable" as this author-controlled same-PR input.
- **Acceptance condition**: On the first PR / no-prior-committed-ledger branch, make the out-of-scope fallback genuinely refusal-only (never a widen), OR own the widen honestly (stop asserting "cannot widen"), one of: (a) **narrow item 2 so it is not attacker-controllable** — e.g. drop the same-PR ledger-self-attribution from the first-PR positive-scope source entirely, admitting only the fixed, self-referential `docs/problems/<slug>/**` subtree (and any other author-independent fixed set) as in-scope on a first PR, and **refusing every other target** (a true minimal-by-construction floor) so a first-PR out-of-scope write can only be split/human-landed — never self-widened; or restrict item 2 to a fixed, non-self-attributable path grammar (e.g. only `docs/**` documents the drive authors under the problem's own subtree, never engine/code paths), and state why that grammar is not attacker-controllable; or (b) **own the residual honestly** — amend §3.2.1(c), DINV-6, §5 R6, and §7 Q4 to state that the first-PR fallback's positive-scope source *includes an author-controlled same-PR input and therefore CAN widen the out-of-scope boundary on a first PR to any path that PR's own ledger self-attributes*, scope the strength of the first-PR out-of-scope guarantee honestly (as §7 Q1 owns the authorship-presence residual and as the committed-ledger branch owns its), and drop every "can never widen / fail-closed / refuse the rest" claim that the mechanism does not deliver on this branch. Either path also needs the A-7 first-PR fixture **extended** to include the malicious case — a first-PR commit whose *same-PR* ledger self-attributes an out-of-scope target (e.g. a code path) — asserting the chosen posture (refused under path (a); or, under path (b), documented as an admitted-and-owned widen, never labeled fail-closed). *(Note for the author: the committed-ledger branch — the bulk of FND-1's fix — is correct and closed; this finding is confined to the first-PR branch's over-claimed safety.)*
- **CAL citation**: CAL-1 (coherence, default blocking) — the design's own binding elements (DINV-6, §3.2.1(c), R6, Q4) contradict §3.2.1(b), exactly where the gate is the *sole* merge-boundary backstop for the out-of-scope class (design:§1/§3.8), so the incoherence is load-bearing on the feature's DoD (FR-9/FR-11 for the out-of-scope class). CAL-5 also applies: this blocking finding is raised at the two-round cap without a further disposition round available → escalate (see the Escalation brief).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-12 |
| 2 | escalate | 2026-07-12 |
| post-escalation re-review | **approve** | 2026-07-12 |

## Escalation brief

**Why escalated (not approved).** Round 2 is the disposition cap
(two rounds; no third). FND-2 is fully closed. FND-1's committed-ledger
derivation is correct and closes the named-broader-slug widen for any problem
with a prior merged ledger — that half is verified sound. But the disposition
introduced, in its own changed spans (§3.2.1(b), DINV-6, §3.8 ABUSE-3′, §5 R6,
§7 Q4), a **blocking coherence regression** (FND-3): the first-PR fallback's
positive-scope source is an **author-controlled, same-PR, arbitrary-path**
input (item 2 — "the paths the current PR's own delivery ledger entries
attribute to the drive"), which is a live "widen your own scope" vector — the
same outcome FND-1 named — while the same disposition asserts four times that
this fallback "can never widen / is fail-closed / admits only positively-placeable
paths; refuse the rest." A blocking finding surviving to the round cap without
convergence is `escalate` by protocol (CAL-1 / CAL-5), embedded-brief +
fail-closed park.

**Fail-closed park (advisory-period effect).** During the advisory period this
verdict blocks nothing mechanically; the park is the *review-state* posture —
the design is **not** review-clean and must not be treated as converged. The
out-of-scope class's first-PR guarantee is the open item: as written, a
mis-behaving/injected first-PR Argus can self-widen its out-of-scope boundary
to any path (including code paths) by attributing that path in a same-PR
delivery-ledger entry, and the design mislabels this branch as refusal-only.

**Embedded brief — the decision the operator/author must make.** Pick one and
land it as a superseding design or (advisory-period) a pre-merge revision of
the still-open draft:
1. **Refusal-only floor (close it).** Drop same-PR ledger self-attribution from
   the first-PR positive-scope source; on a first PR admit only the fixed,
   author-independent `docs/problems/<slug>/**` subtree and refuse every other
   target, so a first-PR out-of-scope write can only be split or human-landed.
   This makes the "never widen" claim true. Cost: more legitimate first-PR
   friction (the §7 Q4 concern — now the *real* trade, not a mislabeled one).
2. **Own the widen honestly.** Keep the same-PR positive-scope source but
   amend §3.2.1(c) / DINV-6 / R6 / Q4 to state plainly that the first-PR
   out-of-scope boundary CAN be self-widened via same-PR ledger
   self-attribution, scope that guarantee's strength honestly, and stop
   labeling the branch fail-closed / refusal-only.
3. **A tighter positive-scope declaration** (the §7 Q4 refinement) — e.g. a
   first-PR scope manifest whose entries are constrained by an
   author-independent grammar — *if* one can be specified that is not itself
   attacker-controllable; today Q4 defers this, so it is not yet a closure.

Either way, extend the A-7 first-PR fixture to include the **malicious**
same-PR-ledger-self-attribution case (an out-of-scope code target
self-attributed in the same PR), asserting the chosen posture.

**Not owed (verified this round).** No R4 escalation is owed by the committed-ledger
fix: reading the in-tree, gate-protected committed delivery ledger adds no new
grant, credential, or outward surface (repo-internal, additive — confirmed
against §3.8/§5 R4 and the `check-ledger-history` arm). FND-3's remedies are
likewise repo-internal (they *narrow* or *re-document* an existing in-tree
read), so no new-surface concern arises from the fix direction either.

## Post-escalation re-review — 2026-07-12

*A fresh review moment on the materially-revised design under a new operator
ruling — **not** a scope-locked round of the escalated cycle (the two-round
cap governs dispositions within one cycle; an operator ruling that changes the
Decision opens a new moment). The frozen round-1 / round-2 bodies and the
Escalation brief above are untouched. FND-3 escalated at the cap; the operator
answered it (recorded as `problem:argus-boundary-enforcement#A-1`) directing a
**slug-derived fixed-path grammar** for first-PR scope; Argus applied it
(§3.2.1(b), DINV-6, §5 R6, §7 Q4, the A-7 fixture — now three cases — §3.1 /
§3.2 / §3.8 ABUSE-3′ / §3.9 / §3.10, and the A-1 citation). This section
verifies the resolution on the merits. Both gate paths re-run green; `lint-tech-designs`
floor excluded as always. This moment may reach any verdict; it is not capped
by the prior cycle.*

**Verdict: approve.** FND-3 is genuinely closed on the merits; the allowlist
residual is honestly owned (not mislabeled fail-closed); no over-block of
legitimate first-PR authoring; the committed-ledger branch is unchanged and
sound; the allowlist re-opens no hole; and the derivation needs no new
grant/outward surface. No new blocking finding; no advisory rises to a finding
(one strengthening observation is noted, non-blocking, below Q4).

### Q1 — does the slug grammar CLOSE FND-3? **Yes — verified on the merits.**

The escalated hole (FND-3) was that §3.2.1(b)'s first-PR positive-scope source
included **item 2** — "the paths the current PR's own delivery-ledger entries
attribute to the drive" — an **author-controlled, same-PR, arbitrary-path**
input: a first-PR Argus could write `internal/engine/core.go` and, in the same
PR, an `E-1` self-attributing that path, and the gate would positively place it
in-scope and pass.

The revised §3.2.1(b) (operator ruling problem:argus-boundary-enforcement#A-1)
**removes item 2 entirely** and defines the first-PR positive scope as a **fixed
function of the slug** — `docs/problems/<slug>/**`, `docs/brainstorms/<slug>*.md`,
`docs/prds/<slug>/**`, `docs/reviews/<slug>-*.md` — **∪ a fixed named allowlist**
(`docs/wiki/glossary.md`, the tech-design index). I checked the design for any
stale remnant of the old vector: the phrase "the paths the current PR's own
delivery-ledger entries attribute" appears **nowhere** in the revised design;
every remaining mention of same-PR self-attribution is in the **negative**
(design:§3.2.1(b) "the grammar never consults that entry"; design:#DINV-6 "nothing
derived from same-PR self-attribution or any other author-controlled input";
design:§3.9 "which never consults same-PR self-attribution and contains no code
path"). **The three attack inputs the question names are all closed:**

- *Self-attributing same-PR ledger entry* → the grammar **never reads** the
  same-PR ledger (design:§3.2.1(b)); a same-PR `E-1` self-attributing
  `internal/engine/core.go` "buys **nothing**" — the path is not a slug member
  and not an allowlist member → out-of-scope → refuse (design:§3.9 row, A-7(i)).
- *A broader slug* → the slug is still read from the trailer, but every
  slug-derived member is a `docs/**` document **keyed to that one slug**; naming
  a broader slug "buys only *more of that slug's own doc subtree*, never
  `internal/engine/core.go` or any code path" (design:§3.2.1(b), §3.1) — **no
  code path is a grammar member**, so a broader slug cannot reach one (A-7(ii)).
- *Any other author-controllable input* → the grammar takes **none** other than
  the slug string, which only *names the subtree the drive owns by construction*
  (design:§3.2.1(b)); the allowlist is static and author-independent.

The first-PR branch is now **genuinely refusal-only** (design:§3.2.1(b) "the
branch is now genuinely refusal-only, not merely labeled so"): it can at most
force a legitimate first-PR out-of-scope-looking write to be split or
human-landed. The escalation's `problem:argus-boundary-enforcement#A-1` ruling —
"never the paths a same-PR, author-controlled delivery-ledger entry
self-attributes … Naming a broader slug may yield only more doc-subtree, never
an arbitrary code path" — is faithfully realized. **FND-3 is closed.**

### Q2 — is the allowlist residual honestly owned (not mislabeled fail-closed)? **Yes.**

The design correctly narrows the residual to exactly one thing — **the
allowlist's own trust** — and owns it without any surviving "never widen /
fail-closed" claim over an author-controllable widen:

- design:§3.2.1(b): "The one residual that remains — owned honestly (§7 Q4) — is
  the allowlist's own trust: the allowlist is a small fixed *named* set of shared
  targets whose members are author-independent (a static list, not derived from
  any PR input), so a broader-slug or self-attributing drive cannot add to it;
  its only trust is that the enumerated shared targets are ones a first-PR drive
  *may* legitimately touch … **not** an author-controllable widen."
- design:§7 Q4 status is honestly "**resolved (direction) — allowlist-content
  refinement open**," and its Resolution states plainly that "the
  **author-controllable** widen the escalation named
  (review:argus-attribution-gate#FND-3) is **closed**," with the remaining trust
  being the static list's *membership* (a content-level review discipline), which
  "does not reopen the widen path."

Crucially, the residual is **author-independent** — the allowlist is a static
named set, not derived from any PR input, so no author (broader-slug or
self-attributing) can *add* to it. The only trust is that the enumerated members
are legitimately first-PR-touchable, a property of the **static list** reviewed
when the list itself changes. That is a genuine, correctly-scoped residual — not
a relabeled author-controllable widen (the class of error the round-2 escalation
caught). The four binding elements that round 2 flagged as over-claiming
(§3.2.1(c) "can never widen," DINV-6, §5 R6, §7 Q4) now assert "never widen"
**only over the slug-grammar-∪-static-allowlist set**, which genuinely never
widens to an author-chosen path — the claim and the mechanism now agree. **No
binding element still labels an author-controllable widen fail-closed.**

### Q3 — does the grammar OVER-block legitimate first-PR authoring? And is the committed-ledger branch still sound/unchanged? **No over-block; committed branch unchanged and sound.**

*Over-block check.* A real stage-1–2 documentation PR writes exactly: the
problem's own ledgers (`docs/problems/<slug>/**`), the stage-1 brainstorm record
(`docs/brainstorms/<slug>*.md` — both `<slug>.md` and the `-tech-design.md`
sibling share the slug prefix), the PRD ledger (`docs/prds/<slug>/**` incl. the
generated projection), the review report(s) (`docs/reviews/<slug>-*.md`), and —
when regenerated — the glossary and the tech-design index. The grammar admits
**all** of these (design:§3.2.1(b) list + allowlist; A-7(iii) legitimate-case
passes). The design's scoping of "first PR = stage-1–2 doc PR" is accurate: the
pipeline (`.pyxis/pipeline.md` stages 1–2) has `/prd-new` produce the PRD and
stage 2 **merge** it, so by the design/plan PRs a committed delivery ledger
context already exists and the (unchanged, sound) committed-ledger branch governs
every later PR — only the very first PR lacks a committed ledger, exactly as
design:§3.2.1(b) claims. No legitimate first-PR path is refused.

*Committed-ledger branch.* Unchanged from the round-1 disposition this report
already verified sound (round-2 FND-1 closure): §3.2.1's committed-ledger
mechanism, DINV-6's first clause (width from the gate-protected append-only
merged ledger), ABUSE-3′'s committed-ledger arm, R6's committed-ledger arm, and
the A-4 widen-slug fixture are all intact. I re-ran `docgate gate --merge-base
origin/main` (the path exercising the load-bearing `check-ledger-history` arm) —
exit 0. The branch remains sound: a current drive cannot retroactively widen any
committed ledger, so a self-named broader slug against a committed ledger buys
nothing. **No regression on this branch.**

### Q4 — coherence of the rewrite, and does the allowlist quietly re-open a hole? **Coherent; no hole.**

*Cross-element coherence.* DINV-6 ↔ §3.2.1 ↔ A-7 ↔ §5 R6 ↔ §7 Q4 are mutually
consistent: all five describe the **same** first-PR scope (the four slug-derived
subtrees ∪ the static named allowlist of glossary + design-index), all five cite
`problem:argus-boundary-enforcement#A-1`, all five describe the residual
identically (the allowlist's own trust, author-independent), and all five state
the same closure (no code path is a member; a broader slug or same-PR
self-attribution can never widen to an author-chosen path). §3.1 (two-trusts
bullet + forgery-resistance bullet), §3.2 (out-of-scope bullet), the mermaid
node, §3.9 (the new first-PR failure-mode row), and §3.10 (the classification and
abuse-fixture bins) all carry the same grammar. The A-7 acceptance row is now the
required **three cases** — (i) malicious same-PR self-attribution reds, (ii)
broader-slug first-PR code write reds, (iii) legitimate slug-subtree passes —
matching §3.10's fixture description and DINV-6. **No coherence regression from
the rewrite.**

*The allowlist-re-opens-a-hole check (the question's sharpest sub-point — is a
member ALSO a protected/governance path this allowlist would wrongly permit an
Argus write to?).* I checked both members against the protected-path definitions:

- **`docs/wiki/glossary.md`** — **not** a governance/protected path. The
  governance-tier content set is precisely **vision, principles, incidents**
  (`.pyxis/authoring/argus.md`:124 "governance-tier content (vision, principles;
  incident entries are appendable observations)"; `argus-artifacts.md`:54). The
  wiki is explicitly the opposite: `docs/wiki/README.md` — "distilled knowledge,
  **never authority** … the one deliberately mutable documentation class …
  Entries are freely edited via normal PRs." So the glossary is a legitimately
  first-PR-touchable mutable doc; admitting an Argus write to it does **not**
  permit a write to any governance/frozen/authority path.
- **The tech-design index (`docs/tech-design/index.md` · `index.json`)** — a
  **generated** artifact ("**GENERATED — DO NOT EDIT.** Sole writer: `docgate
  gen-index`"), and — importantly — **independently byte-verified by a separate
  gate arm**, `verify-index` (`internal/docgate/gate/reg_verify_index.go`:
  "never hand-edit the index: run `docgate gen-index` and recommit"). So even
  though this allowlist admits an Argus write to the index on a first PR, any
  content that is **not** the deterministic regenerated fold is refused by
  `verify-index` on a **different** arm — the allowlist permits *regeneration*,
  not arbitrary bytes. It is also a fixed **doc** pair, never a code path.

Neither allowlist member is a governance/authority/frozen path, and neither can
be leveraged to reach a code path (both are fixed named docs). The allowlist
therefore does **not** quietly re-open a hole: it does not permit an Argus write
to any shared file that should be gated. **No hole.**

*Strengthening observation (non-blocking, not a finding).* The index-allowlist
entry is safe partly **because** `verify-index` independently byte-checks it
against the deterministic fold — a first-PR Argus admitted by this allowlist to
`index.md` still cannot land non-regenerated bytes there. The design owns the
index entry only under §7 Q4's "legitimately first-PR-touchable" trust and does
not *cite* the `verify-index` cross-check as part of why the entry is safe.
Noting the cross-check would strengthen the residual's ownership, but its absence
is **not** a finding: the allowlist creates no widen (it is a fixed named doc
pair, no code path), the residual is already honestly scoped, and CAL-4 does not
bite (the gap is not load-bearing for acceptance — A-7(iii) already asserts the
index is in-scope on a first PR, and `verify-index` is an existing shipped arm).
A content-level refinement for §7 Q4 at implementation, at most.

### Q5 — does the slug+allowlist derivation need a new grant/outward surface? **No — verified.**

The first-PR derivation is a **pure function of the slug** (the slug string is
already read from the trailer the gate already parses) **∪ a static in-repo named
list** — it reads no new input, invokes no service, and needs no credential. This
is strictly *less* surface than the committed-ledger branch (which reads an
in-tree ledger), and the R4 posture is unchanged: DINV-5 ("adds no new egress,
identity, secret, credential, or network surface"), §3.8 ("introduces no new
identity, permission, secret, credential, network, or egress surface"), and R4
(delivery:argus-boundary-enforcement#E-15, operator-accepted) all still hold. The
A-1 ruling itself records the direction as "repo-internal and additive (a pure
function of the slug + a static allowlist — no new grant or outward surface)."
Both gate paths green (no new check dependency introduced). **No new
grant/outward surface; no R4 escalation owed.**

### FND-3 disposition — closed by operator ruling + faithful application

- **Finding**: FND-3 (blocking, coherence, CAL-1/CAL-5) — the first-PR
  fallback's positive-scope source was author-controlled over arbitrary paths, a
  live widen vector the disposition claimed impossible.
- **Resolution path**: the escalation's embedded-brief option 1 ("Refusal-only
  floor — drop same-PR ledger self-attribution … admit only the fixed,
  author-independent subtree") + option 3's spirit (a tighter author-independent
  positive-scope declaration), as directed by the operator
  (problem:argus-boundary-enforcement#A-1).
- **Status**: **closed** — verified on the merits above (Q1). The disposition's
  own binding elements no longer over-state safety: "never widen" is now asserted
  only over a set that genuinely never widens to an author-chosen path. The
  honestly-owned residual (the allowlist's static membership) is a content-level
  trust, not an author-controllable widen (Q2).

## Measurements

- **Rounds**: 2 (round 2 was the disposition cap) + 1 post-escalation re-review (a fresh moment under the operator ruling, not a capped round)
- **Findings**: 2 blocking (FND-1 round 1, FND-3 round 2) · 1 advisory (FND-2)
- **Round-1 dispositions**: 2 applied · 0 declined · 0 withheld-as-violation *(both applied by Argus, tier-2, during the advisory period — dispose-blocking-finding exercised on FND-1, the widen-your-own-scope security hole; the fix elevates the committed-ledger cross-reference to load-bearing and carries a stated first-PR residual)*
- **Round-2 closure**: FND-1 **partially satisfied** (committed-ledger half closed + verified sound on the merits; first-PR half over-claims safety → FND-3) · FND-2 **fully satisfied** (advisory, closed) · **FND-3** raised — blocking coherence regression on the changed spans (§3.2.1(b) vs DINV-6/§3.2.1(c)/R6/Q4)
- **Post-escalation resolution**: FND-3 **closed** by operator ruling (problem:argus-boundary-enforcement#A-1: first-PR scope = a **slug-derived fixed-path grammar ∪ a static named allowlist**, never same-PR self-attribution) + Argus's faithful application (§3.2.1(b), DINV-6, §5 R6, §7 Q4, A-7 three-case fixture, §3.1/§3.2/§3.8 ABUSE-3′/§3.9/§3.10). Verified on the merits: the FND-3 vector is gone (item 2 removed; no code path is a grammar member; a broader slug buys only that slug's own doc-subtree; a same-PR self-attributing ledger entry buys nothing — the grammar never consults it).
- **Post-escalation findings**: **0 new blocking** · **0 new advisory** (one non-blocking strengthening observation noted at Q4 — the index-allowlist entry is additionally protected by the independent `verify-index` byte-check, which the design could cite but need not, as the entry creates no widen and CAL-4 does not bite).
- **Security-merits judgment (post-escalation, live)**: FND-3 is **genuinely closed** — the first-PR out-of-scope branch is now a **fixed function of the slug ∪ a static author-independent named allowlist**, reading **no** author-controlled same-PR input, so a mis-behaving/injected first-PR Argus **cannot** self-widen to a code path (naming a broader slug buys only more of that slug's own `docs/**` subtree; a same-PR self-attributing ledger entry buys nothing). The residual is **now the allowlist's own trust** — a static named set, author-independent, hence not attacker-widenable; its members (glossary; the generated tech-design index) are verified **not** governance/protected paths (the wiki is "never authority, freely edited"; the index is generated + independently byte-verified by `verify-index`), so the allowlist re-opens **no** hole. The committed-ledger branch is unchanged and still sound (re-verified via the `check-ledger-history` arm). The slug+allowlist derivation adds **no** new grant/outward surface (a pure function of the already-read slug + a static in-repo list — repo-internal, additive). *(Round-2's judgment — that the first-PR residual was mislabeled fail-closed — was correct and is now remediated by the ruling.)*
- **Gate (post-escalation re-run)**: `go run ./cmd/docgate gate` exit 0 (tree-local) · `go run ./cmd/docgate gate --merge-base origin/main` exit 0 (history path, incl. the load-bearing `check-ledger-history` arm) · `lint-tech-designs` floor excluded as always
- **Verdict (live)**: **approve** (post-escalation re-review — FND-3 closed on the merits by problem:argus-boundary-enforcement#A-1; the allowlist residual is honestly owned; no over-block, no new-surface, no re-opened hole, no new blocking finding). *Frozen prior verdicts: round 1 request-changes, round 2 escalate (the Escalation brief above stays authoritative for its moment).*

## Overlay corrections

none
