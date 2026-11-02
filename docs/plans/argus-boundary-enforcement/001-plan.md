# Argus boundary enforcement — the three-layer mechanical boundary guard — plan 001

| Field | Value |
|---|---|
| **Feature** | argus-boundary-enforcement |
| **PRD** | docs/prds/argus-boundary-enforcement |
| **Design** | docs/tech-design/argus-write-hook.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Enforce Argus's boundaries mechanically — the protected-path/declared-scope predicate, the authority-contract structure arm, the attribution gate, the write hook, and their acceptance |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The protected-path classifier and the declared-scope resolver

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/argus/boundary |

**Goal:** One reusable predicate answers "is this target a protected path or outside the drive's declared scope?" — classifying a target into the protected-path classes and resolving a drive's declared scope from the named problem's committed delivery ledger, or, on a problem's first documentation PR, from a slug-derived fixed set plus a fixed named allowlist — so the write hook and the attribution gate share one honest scope-boundary source and neither derives that boundary from any author-controlled same-PR input.

**Current behavior:** No shared classifier or scope resolver exists. Nothing in the repo can decide whether a target is the authority contract, a frozen/merged document, another problem's ledger, or outside a drive's declared scope, and nothing derives a drive's permitted scope from an author-independent source; each consuming layer would otherwise invent its own, and a scope boundary read from an author-writable signal would let a mis-behaving drive widen its own scope.

**Desired behavior:** The classifier places a target into exactly one of the protected-path classes (the authority contract as the fixed policy file, a frozen/merged document, another problem's ledger, or out-of-declared-scope) or reports it in-scope. The scope resolver derives a drive's permitted width from an author-independent source: for any drive whose problem already has a committed delivery ledger, the width comes from that gate-protected committed ledger's own recorded reality (which a current drive cannot retroactively widen), so naming a broader problem buys only what that problem's merged ledger genuinely owns; for the single first documentation PR of a brand-new problem, whose ledger is not yet committed, the width is a fixed function of the drive's slug — the slug's own problem, brainstorm, PRD, and review document subtrees — united with a fixed named allowlist of shared targets, and it consults no same-PR self-attribution and admits no code path. When scope cannot be resolved the predicate reports unresolvable so a consumer fails closed. The classifier and resolver are pure and hermetic — a planted directive inside any read input is data, never an instruction that steers the verdict.

**Scope:**
- the protected-path classifier over the protected-path classes plus the in-scope case
- the committed-ledger scope-width derivation for a drive whose problem has a merged ledger
- the first-PR slug-derived fixed-set-plus-named-allowlist scope, taking no author-controlled same-PR input
- the unresolvable-scope signal a consumer turns into a fail-closed deny/refusal

**Out of scope:**
- wiring into any agent-CLI hook or CI context — this task is the predicate only (T3, T4)
- the authority-contract structural parse and its three-invariant evaluation — a separate layer that needs none of this predicate (T2)
- the abuse-fixture corpus and the cross-layer honest-split proofs (T5)

**Acceptance criteria:**
- [ ] a target in each protected-path class is classified as protected and an in-scope target is classified in-scope, each naming the class it matched
- [ ] against a committed delivery ledger, a target the merged ledger does not own is out-of-scope even when a broader problem slug is named, and a target it owns is in-scope
- [ ] on a first-PR resolution the slug's own document subtrees and the named allowlist resolve in-scope while a code target and any same-PR self-attributed target resolve out-of-scope
- [ ] an unresolvable declared scope yields the unresolvable signal, never an in-scope result

**Key interfaces:**
- a classifier that maps a target to a protected-path class or the in-scope result
- a declared-scope resolver with a committed-ledger branch and a first-PR slug-derived-set-plus-named-allowlist branch, both author-independent
- the unresolvable-scope result a consumer maps to fail-closed
- the named-allowlist of shared first-PR targets as version-pinned data

**Traces:** prd:argus-boundary-enforcement/FR-9 · prd:argus-boundary-enforcement/FR-10 · prd:argus-boundary-enforcement/NR-4 · design:argus-attribution-gate#3.2 · design:argus-attribution-gate#3.2.1 · design:argus-attribution-gate#DINV-6 · design:argus-write-hook#3.1

### T2 — The authority-contract structure arm

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/authoritycontract, internal/docgate/schema |

**Goal:** A base-free self-registered gate check parses the authority contract against version-pinned closed vocabularies and reports a named defect on any structural violation of attribution-present, tier-in-ladder, no-bare-pointer-flip, or action-class token-membership — while a legal contract and the born contract pass, a malformed contract is a named parse defect not a crash, and every non-authority-contract tree's gate verdict stays byte-identical.

**Current behavior:** The authority contract's structural invariants live only in the template's human-walked checklist; no parse or gate check reads the file, so a contract that names a current tier absent from the ladder, flips the pointer with no history row (including a wiped or empty change history), drops a history row's attribution, or grants a token that is not a defined action class reads as valid prose and can merge unnoticed.

**Desired behavior:** A sibling parse of record — not an extension of the entry-anchored ledger parse, which the authority contract's shape does not fit — reads the file's two delimited tables and its current-tier pointer against closed, version-pinned vocabularies of tier names, action-class tokens, and the change-history move enum. The evaluation reports one named defect per violation: a current tier not present in the ladder; a bare pointer flip, defined so that an empty or heading-only change history under a non-initial current tier reds as a missing-history flip and only the born contract's single seed move at the initial tier is exempt; any change-history row missing its date, move-from-the-closed-enum, resulting tier, or evidence refs; and any ladder bundle token that is not a defined action-class token — the structural form of "no never-class act can be granted", never a separate blocklist scan. A wrong or absent template version and an undelimitable region are named parse defects, never a crash or a false pass. The arm registers from its own descriptor file with no merge base required, adding no hand-edited line to the umbrella enumeration, dispatch, or help; it mutates nothing, leaves the existing immutability check's governed set unchanged, and leaves every non-authority-contract tree's verdict byte-identical.

**Scope:**
- the sibling authority-contract parse of the two tables and the pointer, against version-pinned closed vocabularies
- the three-invariant evaluation of attribution-present, tier-in-ladder with no bare pointer flip, and token-membership, including the empty-and-non-initial-history flip red and the born-contract exemption
- the named parse defect for a wrong/absent version or an undelimitable region
- the base-free single-descriptor self-registration that adds no shared hand-edited line

**Out of scope:**
- the shared protected-path/declared-scope predicate — this layer governs one fixed file and needs none of it (T1)
- the write hook and the attribution gate (T3, T4)
- the cross-layer honest-split proofs and the full abuse corpus (T5)

**Acceptance criteria:**
- [ ] on a current-tier-not-in-ladder, a latest-history-tier mismatch, and an empty-history-under-a-non-initial-tier fixture the arm reports a defect naming the file and the invariant, and a legal contract and the born contract pass
- [ ] on a history row missing each required field and on an out-of-enum move the arm reports a defect naming the row and the field, and a complete contract passes
- [ ] on a ladder bundle token that is not a defined action-class token the arm reports a defect naming the token, and the pinned legal vocabulary passes
- [ ] a malformed contract and a wrong/absent-version contract each yield a named parse defect and a non-zero verdict, never a crash and never a pass

**Key interfaces:**
- a sibling authority-contract parse producing the current-tier pointer, the ladder rows, and the change-history rows
- the version-pinned closed vocabularies of tier names, action-class tokens, and the move enum
- the three-invariant structural evaluation emitting one named defect per violation
- a base-free self-registered gate descriptor and the named parse-defect record

**Traces:** prd:argus-boundary-enforcement/FR-4 · prd:argus-boundary-enforcement/FR-5 · prd:argus-boundary-enforcement/FR-6 · prd:argus-boundary-enforcement/FR-7 · prd:argus-boundary-enforcement/FR-8 · prd:argus-boundary-enforcement/UB-3 · prd:argus-boundary-enforcement/NR-1 · prd:argus-boundary-enforcement/NR-2 · design:argus-authority-contract-structure-arm#3.1 · design:argus-authority-contract-structure-arm#3.2 · design:argus-authority-contract-structure-arm#3.3 · design:argus-authority-contract-structure-arm#DINV-2 · design:argus-artifacts#DINV-4

### T3 — The attribution gate: authorship derivation and protected-path refusal

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/argus/attribution, .github/workflows |

**Goal:** A required CI check derives whether each of a PR's diffs is Argus-authored from an in-tree commit trailer read with no ambient credential, refuses the PR when an Argus-authored diff touches a protected path using the shared classifier and scope resolver for the boundary, fails closed when authorship is underivable on a protected-path diff, and never refuses a human-authored diff.

**Current behavior:** No authorship-scoped merge-boundary refusal exists. A drifted or injected Argus whose protected-path write bypassed the write-time layer reaches the mainline unless a human reviewer notices, and there is no check that fails a PR because an Argus-authored diff touched the authority contract, a governance document, or an out-of-scope file — the sole merge-boundary backstop for the governance-document and out-of-scope classes is a person's attention.

**Desired behavior:** The gate reads each PR commit's authorship trailer from in-tree commit metadata — never a mutable platform badge, never an ambient credential — and classifies each diff as Argus-authored or not by the trailer's presence. For an Argus-authored diff it applies the shared classifier and scope resolver to the target: a hit on the authority contract, a governance document, or an out-of-declared-scope file refuses the PR with a reason naming the path and the boundary; the out-of-scope boundary's width comes from the resolver's author-independent source, so a self-named broader problem slug cannot reclassify an out-of-scope write as in-scope. When authorship cannot be determined for a diff touching a protected path, that protected-path subset is refused — fail closed — while a non-protected-path diff with an underivable author and any human-authored diff, including a human PR to a protected path, are never refused by this layer. The verdict is structural — trailer presence plus classification — so a planted directive in a commit body or a diff cannot steer it. It composes without editing any shared enumeration line.

**Scope:**
- the in-tree commit-trailer authorship derivation, credential-free and badge-independent, at commit granularity
- the Argus-authored-and-protected-path refusal using the shared classifier and the committed-ledger/first-PR scope width
- the fail-closed rule scoped to the protected-path subset when authorship is underivable
- the human-author-path-open behavior and the no-shared-line CI composition

**Out of scope:**
- the classifier and the scope resolver themselves — reused from T1, not re-derived here
- the trailer's cryptographic forgery-resistance strength, an open design refinement, not this task's behavior
- the write hook and the structure arm (T2, T4) and the cross-layer proofs (T5)

**Acceptance criteria:**
- [ ] an Argus-authored diff touching each of the authority contract, a governance document, and an out-of-scope file is refused with a reason naming the path and the boundary, and a human-authored diff to the same paths is not refused
- [ ] an authorship-underivable diff touching a protected path is refused, while a determinable human-authored one and a non-protected-path underivable diff are not
- [ ] the derivation runs with no ambient credential and reads no mutable badge, reading only in-tree commit trailers
- [ ] a widen-your-own-scope diff whose trailer names a broader problem slug does not reclassify an out-of-scope write as in-scope

**Key interfaces:**
- the commit-trailer authorship derivation reading in-tree commit metadata at commit granularity
- the refusal that composes the shared classifier and scope resolver with the trailer signal
- the protected-path-subset fail-closed rule for an underivable author
- the required CI context that carries no ambient credential and adds no shared enumeration line

**Traces:** prd:argus-boundary-enforcement/FR-9 · prd:argus-boundary-enforcement/FR-10 · prd:argus-boundary-enforcement/FR-11 · prd:argus-boundary-enforcement/UB-4 · prd:argus-boundary-enforcement/NR-4 · prd:argus-boundary-enforcement/NFR-1 · design:argus-attribution-gate#3.1 · design:argus-attribution-gate#3.3 · design:argus-attribution-gate#3.4 · design:argus-attribution-gate#DINV-1 · design:argus-attribution-gate#DINV-2 · design:argus-attribution-gate#DINV-3

### T4 — The write hook: canonical deny decision and per-CLI fail-closed adapters

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/argus/writehook |

**Goal:** A write-time layer denies a protected-path or out-of-scope write before any bytes through each supported agent CLI's own pre-write hook — one canonical deny decision authored once and reused by per-CLI adapters, each fail-closed — returning a reason naming the path, the boundary, and a remedy, and denying too when scope is unresolvable or the hook process errors.

**Current behavior:** The agent surface exposes no write-time deny mechanism; the filesystem accepts any write a drive attempts, and the only barrier to a protected-path write is the drive guide's prose plus downstream attention. No pre-write hook is wired to any CLI, and there is no canonical deny decision for a CLI to call.

**Desired behavior:** One canonical deny decision takes the pending write's target and the drive's declared scope, applies the shared classifier and scope resolver, and returns allow for an in-scope target or deny for a protected-path or out-of-scope target — before any bytes are written — with a reason naming the target path, the boundary it hit, and a remedy. Each supported CLI is wired to that decision through its own pre-write hook as a per-CLI adapter on the vendor-neutral surface, set fail-closed; the deny logic and the protected-path knowledge live once and each CLI's wiring is a pointer, not a content copy, so a portability check finds no divergence and onboarding a CLI is a new adapter file. An unresolvable declared scope denies, and a hook process error, crash, or timeout denies, each with the same three-part reason. A CLI whose hook cannot express a fail-closed deny is listed with the merge-boundary layers as its stated guarantee, never a silent fail-open. The layer is scoped as the write-time layer and makes no claim to stop an out-of-band write.

**Scope:**
- the canonical deny decision returning allow or a three-part-reason deny, authored once, reusing the shared predicate
- the per-CLI adapters on the vendor-neutral surface, each fail-closed, wiring each CLI's own pre-write hook by pointer
- the unresolvable-scope-denies and hook-process-error-denies fail-closed rules
- the fail-open-default CLI listed with the merge-boundary backstop as its stated guarantee

**Out of scope:**
- the classifier and scope resolver themselves — reused from T1 (T1)
- the unbypassable out-of-band guarantee, which the merge-boundary layers own (T2, T3)
- the structure arm and the attribution gate (T2, T3) and the cross-layer proofs (T5)

**Acceptance criteria:**
- [ ] a protected-path write in each class is denied before any bytes with a reason naming path, boundary, and remedy, and an in-scope write is allowed
- [ ] for each supported CLI a protected-path write is denied fail-closed against the CLI's real hook contract, and a portability check finds the deny logic single-sourced with each CLI's wiring a pointer
- [ ] an unresolvable-scope write and a forced-hook-error write are each denied with the three-part reason, never allowed
- [ ] a CLI whose hook cannot fail closed is listed with the merge-boundary backstop as its stated guarantee, never silently allowed

**Key interfaces:**
- the canonical deny decision returning allow or a deny carrying path, boundary, and remedy
- the per-CLI adapter shape wiring a CLI's own pre-write hook to the canonical decision, fail-closed
- the unresolvable-scope and hook-runtime-error deny rules
- the stated-guarantee list for a CLI with no fail-closable hook

**Traces:** prd:argus-boundary-enforcement/FR-1 · prd:argus-boundary-enforcement/FR-2 · prd:argus-boundary-enforcement/FR-3 · prd:argus-boundary-enforcement/UB-1 · prd:argus-boundary-enforcement/UB-2 · prd:argus-boundary-enforcement/UB-6 · prd:argus-boundary-enforcement/NR-3 · prd:argus-boundary-enforcement/NFR-4 · design:argus-write-hook#3.1 · design:argus-write-hook#3.2 · design:argus-write-hook#3.3 · design:argus-write-hook#DINV-1 · design:argus-write-hook#DINV-4

### T5 — The abuse corpus, the honest-layer-split proof, and the bootstrap exception

| Field | Value |
|---|---|
| **Blocked by** | T2, T3, T4 |
| **Size band** | 110–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/argus, internal/docgate/authoritycontract, docs/wiki/glossary.md |

**Goal:** The seeded abuse corpus proves each boundary violation is refused by the layer that owns it — the wiped-history self-elevated pointer at the structure arm, the malicious same-PR self-attribution and the widen-slug at the attribution gate, and the out-of-band bypass caught at the merge boundary — while every legitimate case passes with zero false refusals and the non-authority verdict stays byte-identical; and the bootstrap exception and the glossary landings are in place.

**Current behavior:** The three layers exist and pass their own per-layer fixtures, but nothing proves the cross-layer properties: that each abuse class is red at exactly its owning layer, that the layers compose as honest defense-in-depth, that the legitimate corpus is never falsely refused, and that the merge-boundary layers back up an out-of-band write. No glossary terms are landed and no PR states the introducing-PR bootstrap exception.

**Desired behavior:** The corpus assembles the abuse fixtures and asserts each is refused at exactly the layer that owns it: the wiped- or empty-history self-elevated authority-contract pointer reds at the structure arm; the first-PR malicious same-PR self-attribution of an out-of-scope code target and the widen-your-own-scope broader-slug case each red at the attribution gate, and the legitimate first-PR case confined to the slug's own document subtrees and the named allowlist passes; the injection-induced protected-path write is denied at the write-time layer and unsteered; and the out-of-band write that bypassed the write-time layer is still refused at the merge boundary — the honest split made concrete. A whole-corpus check shows every legitimate case passes with zero false refusals and the gate's verdict on every non-authority-contract tree is byte-identical with and without the new checks. The candidate glossary terms are landed, and the introducing PR states the incident-bootstrap exception — the merge-boundary check cannot gate the very PR that introduces it, so operator review is that one PR's backstop, and each check gates every subsequent PR.

**Scope:**
- the abuse corpus asserting each violation red at exactly its owning layer, spanning the wiped-history, malicious-self-attribution, widen-slug, injection, and out-of-band cases
- the honest-layer-split demonstration that the merge-boundary layers back up an out-of-band write the write-time layer cannot stop
- the whole-corpus zero-false-refusal and byte-identical-non-authority-verdict proofs
- the glossary term landings and the introducing-PR bootstrap-exception statement

**Out of scope:**
- the per-layer deny, refusal, and parse logic itself (T1, T2, T3, T4)
- the engine-era graduation of these checks, deferred to a future design
- any authority-contract mutation — the corpus reads and asserts, it never writes the contract

**Acceptance criteria:**
- [ ] each abuse fixture is refused at exactly its owning layer — the wiped-history pointer at the structure arm, the malicious-self-attribution and widen-slug at the attribution gate, the injection at the write-time layer, and the out-of-band write at the merge boundary
- [ ] every legitimate fixture passes with zero false refusals, including the legitimate first-PR case and a human-authored protected-path PR
- [ ] the gate verdict on every non-authority-contract tree is byte-identical with and without the new checks
- [ ] the glossary terms are landed and the introducing PR states the bootstrap exception

**Key interfaces:**
- the abuse-and-legitimate corpus with each fixture's expected owning layer and verdict declared
- the honest-split demonstration tying the out-of-band write to the merge-boundary refusal
- the byte-identical-non-authority-verdict and zero-false-refusal whole-corpus checks
- the glossary entries and the introducing-PR bootstrap-exception note

**Traces:** prd:argus-boundary-enforcement/ABUSE-1 · prd:argus-boundary-enforcement/ABUSE-2 · prd:argus-boundary-enforcement/ABUSE-3 · prd:argus-boundary-enforcement/ABUSE-4 · prd:argus-boundary-enforcement/UB-5 · prd:argus-boundary-enforcement/EVAL-1 · prd:argus-boundary-enforcement/EVAL-2 · design:argus-attribution-gate#A-7 · design:argus-attribution-gate#3.8 · design:argus-authority-contract-structure-arm#3.7 · design:argus-write-hook#3.7

## Coverage

| Requirement | Tasks |
|---|---|
| prd:argus-boundary-enforcement/FR-1 | T4 |
| prd:argus-boundary-enforcement/FR-2 | T4 |
| prd:argus-boundary-enforcement/FR-3 | T4 |
| prd:argus-boundary-enforcement/FR-4 | T2 |
| prd:argus-boundary-enforcement/FR-5 | T2 |
| prd:argus-boundary-enforcement/FR-6 | T2 |
| prd:argus-boundary-enforcement/FR-7 | T2 |
| prd:argus-boundary-enforcement/FR-8 | T2 |
| prd:argus-boundary-enforcement/FR-9 | T1, T3 |
| prd:argus-boundary-enforcement/FR-10 | T1, T3 |
| prd:argus-boundary-enforcement/FR-11 | T3 |
| prd:argus-boundary-enforcement/UB-1 | T4 |
| prd:argus-boundary-enforcement/UB-2 | T4 |
| prd:argus-boundary-enforcement/UB-3 | T2 |
| prd:argus-boundary-enforcement/UB-4 | T3 |
| prd:argus-boundary-enforcement/UB-5 | T5 |
| prd:argus-boundary-enforcement/UB-6 | T4 |
