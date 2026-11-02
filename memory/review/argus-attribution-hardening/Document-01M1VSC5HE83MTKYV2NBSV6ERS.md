# Review: argus-attribution-hardening — design-review report

| Field | Value |
|---|---|
| **In one line** | **Approve** (round 2, terminal): round 1 returned request-changes (FND-1 blocking — the both-signals `Governing-Change: operator` collision was unstated on the drive-plantable positive-human arm; FND-2/FND-3 advisory); the author disposed all three as applied, and the scope-locked round-2 review confirms them resolved on the merits — FND-1 is closed by a **stated positively-Argus-wins precedence** (drive-plantability made explicit, a planted attestation provably cannot launder a positively-Argus diff, a both-signals collision fixture added to A-3/§3.13, the vector owned in ABUSE-2/§3.12) with no new coherence defect in the changed span; the committer identity is re-scoped non-adversarial with no binding element resting on it; AP-11 is cited with its own single-sourced reader. Not escalated. |
| **Relevant-when** | auditing or consuming the review of argus-attribution-hardening; asking why it was bounced; tuning tech-design-review calibration |
| **Subject** | design:argus-attribution-hardening · shape: none · supersedes: argus-attribution-gate |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-07-13 (round 1) · 2026-07-13 (round 2) |
| **Verdict** | **approve** (round 2, terminal — the verdict history below is authoritative) |
| **Tracking** | the subject's introducing PR, once filed |
| **Template version** | design-review-report/v1 |

## 0. Scope note

*Advisory period: this verdict blocks nothing mechanically; the findings
accumulate as measurement toward the ten-review ship checkpoint. Subject
verified: a post-lint draft superseding technical design
(`argus-attribution-hardening.md`, `Supersedes: argus-attribution-gate`)
with its declared frozen target `docs/tech-design/argus-attribution-gate.md`;
shape `none` (recorded in the header, matching the subject's own
declaration). RECORD-FED: basis
`docs/brainstorms/argus-attribution-hardening-tech-design.md`
(gated ready-for-design, riding this PR); record-fidelity + the four
design organs (Basis, ALT-n ledger, panel log, seam map) checked. The floor
is excluded — nothing `lint-tech-designs` enforces (header schema, Serves
resolution/liveness, mandatory sections) is a finding here. Four lenses
generated in parallel; findings survived an adversarial pass and carry the
five elements + a `CAL-n` citation. The load-bearing shipped-code claims (the
`base..HEAD` range, the `Human = trailer-absent` inference, the trailer parse,
the `Argus > Underivable > Human` fold, the `hasOperatorTrailer` mechanic, and
the absence of any content-binding today) were verified directly against
`internal/argus/attribution/derive.go`, `internal/argus/boundary/boundary.go`,
and `internal/docgate/gate/gate_v2.go`.*

***Settled points — not re-judged (the never-re-judge boundary).*** *The
following are frozen by the prior review (`review:argus-attribution-gate`), the
operator, or the basis, and are treated as evidence, never as this review's
object:* **(a)** *the operator's accepted R4 judgment for the in-tree
commit-trailer derivation (delivery:argus-boundary-enforcement#E-15);* **(b)**
*the DINV-6 scope-width derivation — the committed-ledger branch and the
operator-ruled first-PR slug-grammar ∪ static allowlist
(problem:argus-boundary-enforcement#A-1) — carried forward verbatim here (§3.5),
already approved;* **(c)** *the property-vs-mechanism-strength split (the DoD
asserts the property; the exact strength is a refinement), honestly carried;*
**(d)** *the honest-layer split and the program decomposition (RW-5). The frozen
target's content is untouched and is cited only where a supersession-integrity
question requires it.*

## Round 1

### FND-1 — the positive-human attestation is a drive-influenced commit-body signal, and the three-state resolution's behavior on a *both-signals* collision is unstated — a load-bearing coherence gap on the sole-backstop class

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§3.1 states the load-bearing property precisely — the producer "appends the `Argus-Drive: <slug>` trailer to the commit message's **final trailer block**," and only *that stamp* is out of reach: "the drive … authors files, not commits, so **it never reaches the surface that emits the trailer**." The out-of-reach guarantee is scoped to the trailer the *harness* writes, not to the commit **message body**, which is drive-prepared content (design:§3.1 "the drive authors files and prepares proposals; git/PR mechanics … are the supervisor's" — the *proposal*, including its message, is the drive's). — vs — design:§3.3 (+ #DINV-5, the binding invariant) makes human establishment rest on "the shipped, credential-free `Governing-Change: operator` pattern (`hasOperatorTrailer`)." The shipped `hasOperatorTrailer` (`internal/docgate/gate/gate_v2.go:217-220`) is `strings.Contains(<git log --format=%B base..HEAD>, "Governing-Change: operator")` — a raw substring match over the **whole concatenated commit-body range**, not a final-trailer-block parse, not per-commit, not anchored to the touched file, and explicitly "attestation, not authorization … review verifies the claim" (`reg_check_governing.go:67,89-91`). So `Governing-Change: operator` is a **plaintext line the drive-influenced commit body can carry**. This opens a collision the design never resolves: a drive authors a protected-path file, the supervisor authors the commit → the harness stamps `Argus-Drive` (**positively-Argus**, out of reach), **and** the drive plants `Governing-Change: operator` in the same commit's body (**positively-human**). §3.3's state table and #DINV-5 enumerate the three states as if mutually exclusive and give **no precedence** for a diff that is positively-Argus *and* positively-human. If positively-human is evaluated first (the table/DINV enumeration ordering suggests it), the drive's protected-path write **passes as human** — the exact masquerade FR-13 exists to close, re-opened on the arm meant to close it. If positively-Argus wins (as the shipped `Argus > … > Human` fold would imply), the design's soundness rests entirely on that tie-break — which is **unstated**, so an implementer is free to build the evade. Note the *frame* direction is correctly closed (a human commit carries no harness-stamped `Argus-Drive`, §3.1), so this is an **evade**, not a frame, gap. Because this gate is the *sole* merge-boundary backstop for the governance-doc and out-of-scope classes (design:§3.11), the incoherence is load-bearing on FR-13/NR-4/DOD-11 for those classes. (design:§3.8 ABUSE-2 addresses only a planted directive *steering the gate's structural verdict* — "trailer presence + content binding + protected-path classification" — it does **not** address a planted *attestation string* being consumed as a positive-human signal by the new §3.3 mechanism; the vector is unowned.)
- **Acceptance condition**: Close the evade or own it, one of — **(a)** state the precedence explicitly in a binding element: on a protected-path diff that is **both** positively-Argus and positively-human, **positively-Argus wins → refuse** (mirroring the shipped `Argus > Underivable > Human` fold), and add a #DINV / §3.3 sentence + a DOD-11-family fixture (extending A-3) proving a commit carrying **both** an `Argus-Drive` stamp and a planted `Governing-Change: operator` on a protected-path diff is **refused**, not passed; **or (b)** make the positive-human signal itself out of the drive's reach (e.g. bind the attestation to a producer/identity the drive cannot influence, or restrict the positive-human read to a source the drive-prepared commit body cannot supply) and state why; **or (c)** own the residual honestly — amend §3.3/#DINV-5/§3.11 to state that the positive-human attestation is a drive-writable body signal, that the three-state resolution therefore depends on a stated positively-Argus-precedence tie-break for its evade-soundness, and stop implying the two positive signals are cleanly disjoint. Either path also needs the A-3/A-4 fixtures extended with the both-signals collision case.
- **Disposition**: **applied** — round 1 (principal Argus, authority-contract/v1 · tier-2; `dispose-blocking-finding` granted, checked at act time; delivery:argus-attribution-hardening#E-17). Judged correct on the merits and load-bearing on the sole-backstop class — no governing-doc basis to decline. Resolved via the reviewer's **option (a): state the precedence — on a protected-path diff that is *both* positively-Argus and positively-human, positively-Argus wins → refuse**, mirroring the shipped `Argus > Underivable > Human` fold (`fileAuthorship`, derive.go) and AP-17 (on any ambiguity the gate refuses, never passes). This is the conservative fail-closed default and needs **no new grant, machinery, or outward surface** — hence disposed, not escalated (not breaking; no operator-owned boundary moved). The design now (i) makes the positive-human attestation's **drive-plantability explicit** (it is *not* out of the drive's reach, unlike the harness-stamped `Argus-Drive` trailer — §2, §3.3, #DINV-5, §3.11), (ii) states that **a planted human attestation can never launder a positively-Argus protected-path diff** (the produced out-of-reach stamp is present → positively-Argus → refuse; the planted attestation is inert), (iii) binds the precedence in **#DINV-5** and the §3.3 state table (positively-Argus evaluated first), (iv) adds the **both-signals collision fixture** to §3.13 testing strategy and to acceptance **A-3** (a protected-path commit carrying both an `Argus-Drive` stamp and a planted `Governing-Change: operator` is *refused*), and (v) owns the vector in **ABUSE-2** (a planted attestation string cannot launder the write) and a §3.12 failure-mode row.

### FND-2 — the "corroborating in-tree committer identity" is over-claimed against the shipped code and as an adversarial signal

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§2 Decision, §3.3, and #DINV-5 (a binding invariant) each lean on human establishment being "**corroborated by a determinable in-tree committer identity**," with §3.3 asserting "the corroborating in-tree committer identity is derived from the commit with **zero human action**" and the identity corroboration is "**free**." — vs — the shipped derivation reads **no** committer or author identity anywhere: every git invocation in `internal/argus/attribution/derive.go` formats `%H%x1f%B%x1e` / `%H` (SHA + body only, `derive.go:84,121`) and `hasOperatorTrailer` formats `%B` only — no `%an/%ae/%cn/%ce` in any production path (the only `GIT_AUTHOR_*/GIT_COMMITTER_*` uses are fixture setup in `*_test.go`). So the corroboration is **net-new**, not a reuse of a shipped read, and "free/determinable" is true only in the trivial sense that `git log --format=%ae` is cheap to add. More materially: an in-tree committer identity (`%ae`/`%an`) is itself **committer-settable** (`user.email`/`GIT_AUTHOR_EMAIL` — exactly as the fixtures set it), so against the adversary FR-12 names it corroborates little without an external trust anchor the design (correctly) does not introduce. As written, #DINV-5 attributes weight to a corroborating signal that is (i) not shipped and (ii) not adversarially load-bearing, which risks a reader treating identity as a real second factor.
- **Acceptance condition**: Re-scope the committer-identity clause so the binding prose claims only what the mechanism delivers — either **(a)** demote the identity corroboration to an explicitly non-adversarial, best-effort disambiguation (state plainly it is committer-settable and carries no forgery-resistance, so the *primary* positive-human signal is the attestation trailer and identity adds no security weight), keeping #DINV-5 testable on the attestation alone; **or (b)** drop the "corroborating in-tree committer identity" from the Decision/#DINV-5 and defer it wholly to Q4 as an implementation option, so no binding element rests on it. (CAL-3 stays advisory: #DINV-5's *core* — positive attestation over absence-inference, three states, no extra human step — remains testable on the attestation trailer alone, so the binding element does not fully go untestable; but see FND-1, on which the attestation arm's *soundness* turns.)
- **Disposition**: **applied — option (a)** (principal Argus, tier-2; delivery:argus-attribution-hardening#E-17). Correct: verified no shipped code reads `%ae/%an` (derive.go formats only `%H%x1f%B%x1e`/`%H`; `hasOperatorTrailer` only `%B`), so the corroboration is net-new and committer-settable → no adversarial weight. Demoted the committer identity to **explicitly non-adversarial best-effort disambiguation** across §2, §3.3, #DINV-5, acceptance A-4, §7 Q4, and AP-26 — stated plainly it is committer-settable with **no** forgery-resistance, so the **primary and sole security-bearing** positive-human signal is the attestation trailer and **no binding element rests on identity** (#DINV-5 and A-4 are testable on the attestation trailer alone). Chose (a) over (b) so the disambiguation remains an honest, non-load-bearing implementation option (fully deferred to Q4 for whether to add it at all); either way nothing binding rests on it.

### FND-3 — reusing `hasOperatorTrailer` for a second, distinct predicate (positive-human establishment) is a twin-predicate risk (AP-11) not addressed at design altitude

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§2/§3.3 adopt "the shipped … `Governing-Change: operator` pattern (`hasOperatorTrailer`)" as the positive-human source. In the shipped code that one helper serves a *single* predicate — "a governing-doc / gate-policy change is operator-attested" (`reg_check_governing.go`, `reg_check_config_policy.go`, both callers). This design makes it serve a *second, semantically different* predicate — "this protected-path diff is positively human-authored for attribution." — vs — AP-11 ("one predicate, one place"; "twin predicates always diverge"): the two uses answer different questions (governing-change attestation vs authorship establishment) and will want to evolve independently (e.g. the attribution use needs the both-signals precedence of FND-1 and the per-file scoping of the attribution gate; the governing use is a whole-range substring by design). The design's Principles section cites AP-6/AP-24 but is **silent on AP-11**, and §7 Q4 frames "does `Governing-Change: operator` serve directly, or a sibling human-authorship key" as a pure implementation refinement — under-weighting that the *predicate is different*, which is exactly AP-11's concern rather than a key-naming detail. This is not a claim the reuse is wrong; it is that the AP-11 tension is a design-altitude decision the document leaves implicit.
- **Acceptance condition**: Address AP-11 at design altitude, one of — **(a)** add an `## Principles` AP-11 line stating whether the positive-human predicate *shares* `hasOperatorTrailer`'s implementation (and why divergence is safe) or gets its **own** single-sourced predicate (a sibling key/reader), and reflect that in §7 Q4's framing (predicate identity, not just key naming); **or (b)** if the reuse is deliberate, state the shared-predicate invariant (one implementation, both consumers) the way the shipped incident-machinery does ("one schema definition, all consumers"), so the two uses cannot silently diverge.
- **Disposition**: **applied — option (a)** (principal Argus, tier-2; delivery:argus-attribution-hardening#E-17). Correct: the positive-human establishment answers a **distinct predicate** from `hasOperatorTrailer`'s governing-change attestation, and the two will diverge (the attribution use needs FND-1's positively-Argus precedence + the attribution gate's per-file scoping; the governing use is a whole-range substring by design). Added an **`## Principles` AP-11 line** stating the positive-human read gets its **own single-sourced reader** (not a silent second consumer of `hasOperatorTrailer`), and **reframed §7 Q4** as a *predicate-identity* question — own key vs the shared `Governing-Change: operator` key — rather than mere key naming; §2/§3.3 now describe the reader as "its own single-sourced positive-human reader, distinct from `hasOperatorTrailer`." `## Principles` deviation remains **none** (AP-11 is now honored).

## Round 2 — scope-locked

*Terminal round (the two-round cap). Scope: **only** whether the author's E-17
dispositions resolved the round-1 findings, with no new defect introduced
**inside the changed span** — no re-review of settled substance, no new lines
of critique (the never-re-judge boundary holds: the R4 judgment and everything
cleared in round 1 stay settled). Re-validated green: `go run ./cmd/docgate
gate --merge-base origin/main` exit 0; frozen `argus-attribution-gate.md`
byte-untouched vs `origin/main` (empty diff, no status change), so supersession
integrity is undisturbed. Reviewed the current working tree of
`docs/tech-design/argus-attribution-hardening.md`, the disposition channels +
E-17, and the design-brainstorm D-3 refinement note + S-3.*

### FND-1 — resolved (blocking, coherence · CAL-1)

The author took the reviewer's **option (a): state the precedence — positively-Argus wins → refuse**, grounded in the shipped `Argus > Underivable > Human` authorship fold and AP-17. Verified against the changed span, all four sub-conditions met:

- **(a) precedence stated, positively-Argus-first, collision row explicit + bound.** §3.3's state table now labels the positively-Argus row "incl. the *both-signals collision*" and states it is "**evaluated first**, so a diff that is positively-Argus refuses **even when a `Governing-Change: operator` attestation is also present**"; the §3.3 precedence bullet states the states are "evaluated positively-Argus-first, not as disjoint alternatives" and that "building the positively-human-first order … is **not** permitted"; **#DINV-5** binds it ("evaluated **positively-Argus-first** … **including a diff that is *both* positively-Argus and positively-human, which refuses**"). The unstated tie-break that was the blocking gap is now a binding element, not an implementer's inference.
- **(b) drive-plantability explicit; planted attestation can never launder a positively-Argus diff.** §3.3 states the `Governing-Change: operator` attestation is "a **plaintext line the drive-influenced commit body can carry**," that it is "**not out of the drive's reach**" (unlike the harness-stamped `Argus-Drive`), and that "a planted human attestation can therefore **never launder a positively-Argus protected-path diff** … the planted attestation is inert." §2 Decision and the §2 shape paragraph carry the same. The out-of-reach `Argus-Drive` stamp wins; the plantable attestation is inert — exactly the acceptance condition.
- **(c) collision fixture testable + planted-attestation vector owned.** §3.13 adds "the collision fixture (FND-1) — a protected-path commit that carries *both* a produced `Argus-Drive` stamp *and* a planted `Governing-Change: operator` attestation in its body is *refused*, not passed"; acceptance **A-3** extends the forge/replay/strip corpus with the same both-signals case (traced to FR-13 · DOD-11 · #DINV-5). **ABUSE-2** now owns the planted-attestation string ("**Nor can a planted *attestation string* launder the write** … the vector the design-review's FND-1 named is owned by the precedence"), and a §3.12 failure-mode row states the both-signals collision refuses. The vector round-1 flagged unowned is owned.
- **(d) correct, complete, no new coherence defect in the changed span.** The precedence bites **only** on the both-signals collision, which only a drive can produce (the `Argus-Drive` stamp is out of the drive's reach). The positively-human row is now scoped "**and not positively-Argus**," so a genuine human PR — which carries no harness-stamped `Argus-Drive` — still resolves positively-human and passes: NR-4 is preserved and no false-refusal is introduced. The *frame* direction stays closed independently (§3.1/§3.3), so the fix closes the *evade* the plantability opened without collateral. DINV-1 ("positively-Argus ∧ protected ⇒ refuse") and DINV-5 remain consistent (DINV-1's positively-Argus subsumes the collision per DINV-5's elaboration; the state table cites both). No contradiction introduced.

The blocking coherence gap is closed on the merits.

### FND-2 — resolved (advisory, contract-quality · CAL-3)

The committer identity is re-scoped to **explicitly non-adversarial best-effort disambiguation** across §2, §3.3, #DINV-5, A-4, §7 Q4, and AP-26 — stated committer-settable with **no** forgery-resistance, so the attestation trailer is the "primary and sole security-bearing" positive-human signal. Every binding element is now testable on the attestation trailer alone: #DINV-5 states "no part of this invariant rests" on identity and it is "testable on the attestation trailer and the precedence alone"; A-4's pass "rests on the **attestation trailer alone**." **No binding element rests on the committer identity.** Option (a) applied as disposed.

### FND-3 — resolved (advisory, contract-quality · CAL-3)

`## Principles` now carries an **AP-11 line** ("*One predicate, one place.*") stating with reason that positive-human establishment answers a **distinct predicate** from `hasOperatorTrailer` (attribution-authorship vs governing-change attestation) and that the two evolve independently, so the positive-human read gets its **own single-sourced reader**, "not a silent second consumer of `hasOperatorTrailer`." §3.3 carries a matching AP-11 bullet; §7 Q4 is reframed as a *predicate-identity* question (own key vs the shared `Governing-Change: operator` key), not key naming. The `## Principles` "Deviation: none" line is now honest (AP-11 was the sole round-1 omission). Option (a) applied as disposed.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-13 |
| 2 | **approve** | 2026-07-13 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (terminal — the two-round cap; round 2 scope-locked to the round-1 dispositions)
- **Findings**: 1 blocking · 2 advisory (all round 1; **0 new** in round 2 — no defect found inside the changed span)
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation *(round 1 — all three applied by principal Argus, tier-2; delivery:argus-attribution-hardening#E-17: FND-1 via the positively-Argus-wins precedence, FND-2 identity-demote option (a), FND-3 AP-11 own-reader option (a). No declines → no auto-escalation.)*
- **Round-2 outcome**: all three dispositions **verified resolved** on the merits — FND-1 (blocking) closed via the stated positively-Argus-wins precedence (the four sub-conditions met, no new coherence defect in the changed span), FND-2 + FND-3 (advisory) closed. Terminal verdict **approve**; **not escalated** (the blocking finding was resolved). Green re-validated (`docgate gate --merge-base origin/main` exit 0); frozen predecessor byte-untouched.

## Overlay corrections

none

---

## Appendix — reviewer's judgment ledger *(not part of the schema; retained for the caller's audit — delete if the report is normalized to the template)*

- **R4 (the star question) — the call was CORRECT.** The design declines a controlled signing key on the R4 boundary and does **not** escalate. This is sound: FR-12's fit criterion names the adversary as "a **misbehaving or injected drive**" (basis effective.md FR-12; brainstorm F-6), not an arbitrary external actor with raw commit access. The producer-out-of-reach property is real against *that* adversary — the domain model makes the supervisor the commit author (`Change` = "commit on the current branch"; INV-21) and the drive authors files/prepares proposals (drive guide `.pyxis/authoring/argus.md` "prepare evidence; a human lands changes"), so the drive cannot add/omit/alter the harness-stamped trailer, and the in-tree binding closes replay — both credential-free. The residual (a raw-commit-access forger) was **already** the frozen design's owned limit and the operator **already accepted** the R4 judgment for the trailer-read derivation (delivery:...#E-15). The signing key genuinely is a new operator-owned surface (identity-and-security §28), so assuming it would be the R4 breach; escalating a mechanism the design *can* build credential-free would be over-escalation. Parking it as §7 Q3 / the escalation-candidate is the right posture. **No R4 finding in either direction.** (The R4 *soundness for the Argus signal* does not transfer to the positive-human arm — that gap is FND-1, a coherence finding, not an R4 one.)
- **G-1 (producer out of the drive's reach) — holds, with the residual honestly owned.** No current drive path authors a commit directly (supervisor/harness authors; drive authors files). The git-hook producer is un-omittable by the drive because the drive never reaches the emitting surface. Verified against the domain model and drive guide. The frozen DINV-6 first-PR grammar admits **no code path**, and later PRs bound scope on the committed ledger, so even a tier-2 code-materializing drive cannot self-attribute an out-of-scope code write — carried forward correctly.
- **G-2 (forgery-resistance closes forge + replay) — holds for the named adversary.** Producer-out-of-reach closes forge/omit by the drive; the in-tree binding closes replay; the raw-commit-access residual is backstopped by DINV-2 (evade) and the structure arm (authority-contract evade). Confirmed: today the trailer carries **no** content-binding (`derive.go` parseTrailer is bare plaintext), so the binding is genuinely new strength, and the structure arm is genuinely author-independent (reads the file as it stands, `argus-authority-contract-structure-arm.md`). One honesty check passes: the design does **not** repeat the frozen design's mislabel — it routes evade to DINV-2 and credits the binding only for replay, credential-free.
- **G-3 (three-state / NR-4) — the gap.** The positive-attestation-trailer mechanism does **not**, as specified, distinguish positively-human from underivable without a seam: the attestation is a drive-writable body signal and the committer-identity corroboration is illusory against the adversary (FND-1 + FND-2). The reused `Governing-Change`/`hasOperatorTrailer` pattern is credential-free (true) but is "attestation, review verifies" — it does not *establish* human authorship on its own. This is why FND-1 is blocking.
- **G-4 (rollout) — holds, verified in code.** `Evaluate` reads exactly `base..HEAD` (`derive.go:84`), base is the fork-point merge-base (`mergebase.go:22`), so pre-mechanism history is never in a later PR's range (no false-refuse) and the producer stamps from-here-forward (no latent window). AP-7 honored (no history rewrite). The I1 bootstrap for the one introducing PR matches the incident precedent.
- **Completeness / altitude / supersession hygiene — clean.** Serves FR-12/FR-13 (live) + carries FR-9/FR-10/NR-4/UB-4/UB-5/ABUSE-2..4/NFR-1..3; header `Supersedes: argus-attribution-gate` correct; frozen target untouched; the index (`docs/tech-design/index.md`) already shows `argus-attribution-gate` → superseded and the hardening `live` with no `stale-serves` and no stranded dependent; altitude is right (commit-hook class Q1, binding form Q2, attestation key Q4 all correctly deferred as implementation refinements). `## Principles` cites the relevant AP/RW with deviation: none — the one omission is **AP-11** (FND-3).
