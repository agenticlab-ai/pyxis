# Argus boundary enforcement — technical design — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Choose the enforcement *mechanism* per layer for Argus's three-layer boundary enforcement — the write-hook surface, the authority-contract structure-arm parse/composition, and the Argus-authorship derivation — and decide whether the feature is one technical design or a program of three (RW-5). |
| **Relevant-when** | designing or changing the mechanism behind any Argus boundary-enforcement layer (the per-CLI write hook, the authority-contract structure arm, the Argus-attribution CI gate); checking prior design rulings or refused mechanism shapes about protected-path / declared-scope / authorship-derivation enforcement; deciding the design-document decomposition of the argus-boundary-enforcement feature. |
| **Basis** | prd:argus-boundary-enforcement (merged — `effective.md`, folded as-of 000) |
| **Authority** | Argus (AI Engineer, decision principal at tier-2 — this record was driven by recorded, attributed, legality-tested assumptions, not a live human interview; each `D-n` cites its `assumption-recorded` journal entry in `delivery:argus-boundary-enforcement` and stands until a human overturns it through the normal channels) |
| **Date(s)** | 2026-07-12 |
| **Status** | gated: ready-for-design |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | `docs/tech-design/argus-authority-contract-structure-arm.md` · `docs/tech-design/argus-attribution-gate.md` · `docs/tech-design/argus-write-hook.md` (a program of three designs — D-1; the record rides the first-authored design's introducing PR) |
| **Prior records** | brainstorm:argus-boundary-enforcement (the stage-1 record — Framings, D-1..D-6, P-1..P-3) · design-brainstorm:argus-ledger-lint-tech-design (the sibling ledger-lint design session — the self-registered-check panel calls this session inherits) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/argus-boundary-enforcement/effective.md | the basis — the frozen T3 contract: FR-1..3 (write hook), FR-4..8 (structure arm), FR-9..11 (attribution gate), NR-1..4, UB-1..6, ABUSE-1..4, the P-1..P-3 → Q1..Q4 mechanism deferrals, §6.1 phasing, R4 |
| brainstorm:argus-boundary-enforcement (stage-1 record) | the parked technical constraints P-1 (write-hook mechanism), P-2 (attribution derivation), P-3 (structure-arm parse/composition) this session resolves; D-1's "each part likely its own design under RW-5"; T-1..T-4 glossary terms |
| docs/tech-design/argus-ledger-lint.md | the sibling design: §3.5 + Q2 scope the structure arm OUT and name it "the follow-on"; §3.1 excludes the authority contract from the history arm; §3.3/§3.4 the self-registered-check composition + shared-parse pattern this feature reuses |
| docs/tech-design/argus-artifacts.md | §3.3 the authority-contract data (ladder, action-class token vocabulary, change-history `move` enum, "Argus never writes this file"); §3.5 DINV-3/4/5 (the invariants the structure arm mechanizes); the citation grammars |
| docs/tech-design/distributed-check-registration.md | the descriptor/registry contract (`{Verb, Help, NeedsMergeBase, Run}`, file-not-line composition) the structure arm — and the attribution gate if a docgate check — self-register through |
| internal/docgate/gate/registry.go · reg_check_ledger_history.go · reg_check_content_paths.go | the live descriptor shape (`Register(Check{…})` at `init()`), the shared `Ctx{Root, MergeBase, Cfg}`, and the `check-content-paths` purity gate this record's prose must satisfy |
| internal/docgate/argusledger/ · internal/docgate/schema/schema.go | the shipped shared parse + the closed-parse `LedgerClass` registry (`problem-ledger`, `delivery-ledger` only — the authority contract has NO parse today; `Defect{Verb,File,Op,Reason,Remedy}` is the one failure type) |
| docs/architecture-principles.md | AP-10 (codify over prompt), AP-13/AP-14/AP-26 (verified artifact; execute in deployment context, no ambient credentials; a status certifies only its object), AP-17 (fail closed on policy), AP-23/AP-27 (file-not-line extension), AP-25 (vendor-neutral surface) |
| docs/observability/incidents.md | I1 (a class's first gate cannot gate its own introducing PR — operator review is the backstop), I3 (consequential CLIs fail closed on misparse) |
| docs/tech-design/index.md | the live decision graph — no design serves any `prd:argus-boundary-enforcement/` anchor yet (this session opens that coverage); `argus-ledger-lint` is live and is the `Builds-on` for the structure arm |
| docs/wiki/glossary.md | the design-class vocabulary (Serves, Supersedes, DINV-n, Binding elements, **Seam**, Deletion test) and the T-1..T-4 terms this feature introduces |

**Overlaps surfaced:**
- **The structure arm is reserved-not-covered ground.** `argus-ledger-lint` §3.5/Q2 explicitly deferred "the third *structure* arm (DINV-3/4/5 + the templates' checklist appendices)" to "a follow-on brainstorm/design." This session is that follow-on; it does not re-litigate the ledger-lint arms, it *reuses their composition substrate* and adds the deferred arm. Not a covering design — proceed.
- **No live design serves any FR of this basis.** The index shows zero designs with a `prd:argus-boundary-enforcement/` Serves anchor; the mechanism decisions are genuinely open. No `covered` verdict is available — proceed to the questions.
- **The self-registered-check seam is already decided** (distributed-check-registration, live): the structure arm — and the attribution gate *if* realized as a docgate check — *extends* that seam rather than opening a new one, so the panel there is judged accordingly (see the panel log).

## Design questions

- **DQ-1**: Is the argus-boundary-enforcement feature realized as **one** technical design or a **program of designs** (one per enforcement layer)? (Decides the document boundary = the supersession boundary — RW-5.)
- **DQ-2** (P-3): How does the **structure arm** parse the authority-contract file and compose into the gate — does it reuse the `argusledger` per-class package + the `schema` `LedgerClass` registry, or land its authority-contract parse in a sibling package with its own closed-parse registry?
- **DQ-3** (P-2): How is **"Argus-authored"** derived for the attribution gate — forgery-resistant within the trust model, available in CI without ambient credentials (AP-14), and not a mutable badge (AP-26)? *(The R4-critical question.)*
- **DQ-4** (P-1): What **write-hook mechanism** realizes the write-time protected-path deny on the agent surface, per supported CLI, fail-closed (AP-25 vendor-neutral) — and where does an unresolvable declared scope fail closed? *(R4-checked.)*

## Panel log

- **DQ-1**: opens/replaces (the document/supersession boundary — a module-boundary decision) — **fired** — two structurally distinct decompositions scored (one cohesive design vs a program of three), each judged against RW-5, AP-27, and the PRD's own §6.1/WR-2.
- **DQ-2**: opens/replaces (a new parse package / closed-parse registry vs extending `argusledger`'s — a module boundary) — **fired** — reuse-`argusledger` vs sibling-`authoritycontract` package scored; the *registration* seam (self-registered `reg_*.go`) is the already-decided one both share, so the panel diverges only on the parse-package boundary.
- **DQ-3**: opens/replaces (the attribution-derivation interface — the attribution gate's load-bearing seam; also the R4 gate) — **fired** — three derivation sources scored (git commit-trailer convention · delivery-journal cross-reference · code-host author identity/badge), each R4-judged for a new grant/outward surface.
- **DQ-4**: opens/replaces (the agent-surface write-hook seam — no such surface exists in the repo today, F-1) — **fired** — per-CLI native hook config on the vendor-neutral surface vs an OS/container sandbox scored, each R4-judged.

## Decisions

### D-1 — A program of three technical designs, one per enforcement layer

- **Question**: One technical design for the whole feature, or a program of designs (one per layer)?
- **Answer**: A **program of three** designs — `argus-authority-contract-structure-arm` (part 2, FR-4..8), `argus-attribution-gate` (part 3, FR-9..11), `argus-write-hook` (part 1, FR-1..3) — this record feeding all three. The shared substrate (the protected-path / declared-scope predicate) is acknowledged but is **not** promoted to its own design now: the basis parks a unified declared-scope manifest as a waiting-room item (WR-2), deliberately declining to couple the two scope-deriving layers at first.
- **Reasoning**: RW-5 binds one **independently-revisable** decision per document, and the three layers are exactly that — the structure-arm parse can be revised without touching the write-hook wiring or the attribution derivation; each is its own supersession boundary. The PRD's §4.1 and §6.1 already state "each part is a separate technical design — RW-5," and the stage-1 D-1 anticipated it. AP-27 favors file-disjoint parallel authorship — three designs authored/reviewed independently never contend on one document. A single cohesive design would bundle three independently-revisable §2 decisions (the conventions' explicit anti-pattern; the same one `argus-ledger-lint` invoked to split its own third arm out). WR-2 is the honest reason not to pre-build a shared-scope design: the shared predicate is real but the basis chose two independent derivations first, revisiting only if warranted.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-15)
- **Status**: live

### D-2 — Structure arm: a sibling `authoritycontract` parse package + its own closed-parse registry; self-registered base-free check

- **Question**: Does the structure arm reuse the `argusledger` package and the `schema` `LedgerClass` registry, or land in a sibling package with its own closed-parse registry?
- **Answer**: A **sibling parse package** (an authority-contract parse of record) with **its own closed-parse registry entries in `schema`** (the tier-name set, the action-class token vocabulary, the change-history `move` enum) — *not* an extension of the `argusledger` `LedgerClass`/`AnchorFamily` shape. It composes as a **single self-registered base-free `reg_*.go` descriptor** (`NeedsMergeBase: false`) — the already-settled registration seam — adding no hand-edited umbrella/dispatch/help line. DINV-5 is enforced as **token-membership** (every ladder bundle token is a defined action-class token), never a separate "no-never-grant" scan — matching the authority-contract template's Appendix checklist phrasing.
- **Reasoning**: The `argusledger` `LedgerClass` model is built around an `AnchorFamily` letter + an entry-bearing `## ` region + a derived-state region (`### A-n`/`### E-n` blocks). The authority contract has **none** of that shape — its structure is a *ladder table* + a *change-history table*, with no `A-n`/`E-n` anchors (this is precisely why `argus-ledger-lint` §3.1 excluded it from the history arm, and why `ClassForPath` doesn't match it). Forcing the authority contract into the `LedgerClass` shape would build an empty entry map and mis-model the file; a sibling parse keyed on the two tables is the honest fit. The closed-parse-registry-in-`schema` *pattern* (version-pinned, additive by version bump) is reused; only the specific `LedgerClass` struct is not. This is repo-internal, additive, report-only — **not breaking** (R4 evaluated: no new grant, no outward surface, no operator-owned boundary moved; see the R4 legality note in F-9). The panel (ALT-1/ALT-2) scored the two package boundaries.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-15)
- **Status**: live

### D-3 — Attribution: a repo-internal git commit-trailer convention as the derivation source; fail-closed on absence

- **Question**: How is "Argus-authored" derived for the attribution gate?
- **Answer**: Argus authorship is derived from a **repo-internal, agent-emitted commit trailer convention** (an explicit `Argus-Authored:`-style trailer the Argus drive already appends when it authors a commit, read from the PR's commit metadata that CI already has in-tree via `git`) — **not** from a code-host author-identity badge, and **not** requiring any new credential, identity, or external service. When the trailer is absent or unverifiable on a diff that touches a protected path, the gate **fails closed for that protected-path subset** (treats an underivable author touching a protected path as a refusal — FR-10, AP-17). The mechanism reads the commit graph with `git`, exactly as the shipped history/cross-document arms already do; it adds **no** ambient-credential dependency and **no** egress.
- **Reasoning**: This is the R4-decisive question. Evaluated honestly against R4's three breaking triggers: (a) **new grant** — none; reading commit trailers is within the drive's existing file/shell capability, no capability beyond tier-2. (b) **new outward surface** — none; the trailer travels *in the repo's own commit metadata*, which CI reads locally via `git` (the deployment-context, no-ambient-credential requirement of AP-14 is *satisfied*, not breached — a `git log`/`git show` is exactly what the ledger arms already run). (c) **operator-owned boundary moved** — none; no new identity/permission/egress is introduced. The rejected alternatives are what *would* have been breaking: a **code-host author-identity/badge** derivation (ALT-4) depends on a mutable platform badge (AP-26 forbids) and, to verify authorship server-side, on a code-host API credential in CI — a **new outward surface / ambient credential**, which is the R4 breaking condition; had that been the *only* viable mechanism, this session would have emitted a decision brief and parked. It is not the only viable mechanism — the repo-internal trailer needs no such surface — so the legal, additive, credential-free choice is assumed and recorded here, and the forgery-resistance *property* (FR-11, required at DoD) is delivered by making the trailer part of the signed/verifiable commit content the design pins, not by a badge. The forgery-resistance *strength* (bare trailer vs a cryptographically-bound one) is a non-blocking design refinement carried as G-1, not a blocking fog on the direction. The panel (ALT-3/ALT-4/ALT-5) scored the three sources.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-15)
- **Status**: live

### D-4 — Write hook: per-CLI native pre-write hook config on the vendor-neutral surface; deny fail-closed; scope-unresolvable ⇒ deny

- **Question**: What write-hook mechanism realizes the write-time deny, per supported CLI, fail-closed?
- **Answer**: A **per-CLI native pre-write hook** — each supported agent CLI's own pre-write hook surface, configured on the **vendor-neutral agent surface** with per-CLI adapters (a pointer/config per CLI, the canonical deny logic authored once — AP-25), each set to **fail closed** (a CLI whose hook defaults to fail-open has its fail-closed option set explicitly; a CLI exposing no honoring hook is documented with the merge-boundary layers as its stated guarantee — UB-2). The deny logic classifies the pending write's target against the protected-path set and the drive's declared scope; an **unresolvable declared scope ⇒ deny** (FR-2, AP-17); a hook process error/crash/timeout ⇒ deny (UB-6). An OS/container **sandbox** is the scored-and-rejected alternative (ALT-6).
- **Reasoning**: R4 evaluated honestly. The native-hook mechanism is **repo-internal and additive**: it wires the CLI's *own existing* pre-write hook surface via configuration on the agent surface — no new grant (the hook is a config the CLI already honors, within the existing agent-surface capability), no new outward surface (the decision is a local, synchronous allow/deny — no network, no external service, no new identity), no operator-owned boundary moved. Therefore **not breaking** — assumed and recorded with legality tests. The **sandbox** alternative (ALT-6) is where R4 would bite: an OS/container sandbox that intercepts writes generally needs **container/OS privileges beyond the repo** (a new privileged surface the operator owns) — that would be the R4 breaking condition, so it is *rejected on both design grounds and the R4 boundary*, not assumed through. Because a native per-CLI hook is a legal, additive, no-new-surface mechanism, no brief is owed. The honest-layer split (PRD D-4) already forbids over-claiming this layer as unbypassable — it is the fast layer; parts 2+3 carry the merge-boundary guarantee — so a CLI that exposes no honoring hook degrades to "gate/CI is your guarantee," never a silent fail-open. The panel (ALT-6) scored native-hook vs sandbox.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-15)
- **Status**: live

## Alternatives

### ALT-1 — One cohesive technical design (all three layers in one document)

- **Shape**: A single `docs/tech-design/argus-boundary-enforcement.md` with §3 subsections for each layer; one §2 Decision framing the three-layer defense-in-depth; one Serves set spanning FR-1..11.
- **Trade-offs**: Wins on a single reader-place for the defense-in-depth thesis and a single shared-substrate treatment. Costs: bundles three independently-revisable decisions (revising the write-hook mechanism later forces re-opening the whole document, or spawns a partial-override leaving two live docs in tension); one large document is a single contention point for parallel authorship and review.
- **Principles**: RW-5 — strained (three independently-revisable §2 decisions in one document is the anti-pattern); AP-27 — strained (one document is a shared surface three layers' authors/reviewers contend on); AP-11 — neutral.
- **Incidents**: none checked (incidents.md holds no document-decomposition entry; the paid-for lesson here is the conventions' own §2/R5, not an incident).
- **Verdict**: rejected — RW-5's one-independently-revisable-decision rule is decisive; the three layers are separately revisable, so they are three documents (D-1).
- **From**: DQ-1 · panel

### ALT-2 — A program of three designs (one per layer)

- **Shape**: Three designs — structure arm (part 2), attribution gate (part 3), write hook (part 1) — each its own §2 Decision, Serves subset, and supersession boundary; this record feeds all three; the shared predicate stays two independent derivations (WR-2), not a fourth "shared-scope" design now.
- **Trade-offs**: Wins on RW-5 conformance (each design independently revisable/supersedable), AP-27 file-disjoint parallel authorship and review, and honest scoping (WR-2 declines premature coupling). Costs: the defense-in-depth thesis and the shared-substrate note live in the record + each design's §1/§3.2 rather than one place — mitigated by the record being the shared provenance and each design citing it.
- **Principles**: RW-5 — honored (one independently-revisable decision per document); AP-27 — honored (three disjoint documents, no shared authoring surface); AP-23 — honored (parallel authors never contend); AP-11 — honored (the shared predicate is one place *within* whichever layer(s) own it, not duplicated — each design states its scope source).
- **Incidents**: none checked (incidents.md; no matching entry — the governing lesson is conventions §2, not an incident).
- **Verdict**: chosen — mirrors `argus-ledger-lint` (one PRD, arms split across designs) and satisfies RW-5/AP-27; the record carries the cross-layer thesis (D-1).
- **From**: DQ-1 · panel

### ALT-3 — Attribution via a repo-internal git commit-trailer convention

- **Shape**: The Argus drive appends an explicit authorship trailer to each commit it authors; the attribution gate reads the PR's commit metadata (in-tree, via `git`) to decide Argus authorship; absent/unverifiable ⇒ fail closed for the protected-path subset.
- **Trade-offs**: Wins on being credential-free and egress-free in CI (a `git log`/`git show` is exactly what the shipped arms run — AP-14 deployment-context satisfied), badge-independent (AP-26), and repo-internal (no new outward surface → R4-legal). Costs: a bare trailer is forgeable by anyone who can write a commit — but the structure arm (part 2) closes the *authority-contract* evade path regardless of author (ABUSE-3), and the forgery-resistance *strength* is a design refinement (G-1); the gate fails closed when the trailer is absent, so a stripped trailer never flips Argus→human silently on a protected path.
- **Principles**: AP-14 — honored (runs in CI with no ambient credentials — reads local commit metadata); AP-26 — honored (reads commit content, not a mutable merged/author badge); AP-17 — honored (absent ⇒ fail closed); AP-13 — honored (authorship is a verifiable artifact — the trailer in the commit — not a claimed status).
- **Incidents**: none checked (incidents.md; no attribution-derivation entry — I3's fail-closed-on-misparse lesson is honored by the absent⇒deny rule but is not a match on the derivation source itself).
- **Verdict**: chosen — the only scored source that needs **no** new grant/credential/outward surface, so it is R4-legal to assume; forgery-resistance property delivered by pinning the trailer to verifiable commit content (D-3).
- **From**: DQ-3 · panel

### ALT-4 — Attribution via the code-host author identity / a platform badge

- **Shape**: Derive Argus authorship from the code host's commit-author identity or a platform "authored-by-bot" badge, verified via the code-host API.
- **Trade-offs**: Wins on a platform-native signal that is hard for a casual actor to set. Costs: a platform badge/identity is **mutable** (AP-26 forbids certifying goal-state on it) and server-side verification needs a **code-host API credential in CI** — a **new ambient credential / outward surface**, which is exactly R4's breaking trigger; it also couples the gate to one code host (breaks the medium-independence posture, AP-1/AP-25).
- **Principles**: AP-26 — strained (a badge is a mutable status of the medium's object, not a verified authorship artifact); AP-14 — strained (needs an ambient credential at evaluation — the deployment-context corollary forbids); AP-1/AP-25 — strained (host-coupled).
- **Incidents**: none checked (incidents.md; no matching entry — I2/AP-26's "badge certifies the medium's object" is the governing *principle*, cited above, not a registry entry).
- **Verdict**: rejected — depends on a mutable badge (AP-26) and a new outward credential surface (the R4 breaking condition); **had this been the only viable mechanism, the session would have emitted a decision brief and parked** rather than assume it. It is not — ALT-3 needs no such surface — so it is rejected in favor of the repo-internal source.
- **From**: DQ-3 · panel

### ALT-5 — Attribution via a delivery-journal cross-reference

- **Shape**: Decide Argus authorship by cross-referencing the diff against the delivery ledger's attributed `E-n` journal entries (the drive that authored the change is the one whose journal records the write).
- **Trade-offs**: Wins on reusing the existing attribution surface (the journal already carries principal identity per DINV-3) with no new signal at all. Costs: the journal records a drive's *acts* at gist granularity, not a per-commit/per-hunk authorship map — resolving "did *this diff* come from an Argus drive" back to a journal entry is a fragile many-to-one join (a human PR that *touches* a problem's tree has no journal entry, but neither does every Argus micro-edit), so it is imprecise exactly where FR-9 needs per-diff precision; and it couples the CI gate to parsing ledger prose.
- **Principles**: AP-14 — honored (journal is in-tree, no credential); AP-26 — honored (reads authored content, not a badge); AP-6 — strained (the join is prose-granular, not a contractual per-diff signal); AP-11 — neutral.
- **Incidents**: none checked (incidents.md; no matching entry).
- **Verdict**: rejected — imprecise at the per-diff granularity FR-9 requires; **hybridized into ALT-3** as a *corroborating* signal the design may use (a trailer that names the driving problem can be cross-checked against that problem's journal), but the primary derivation is the per-commit trailer.
- **From**: DQ-3 · panel

### ALT-6 — Write hook via an OS / container sandbox

- **Shape**: Intercept writes at the OS/filesystem or container layer (a sandbox that denies writes to protected paths regardless of the CLI), rather than the CLI's own pre-write hook.
- **Trade-offs**: Wins on CLI-independence (one sandbox covers every CLI, and even a non-honoring CLI) and on catching out-of-band writes the native hook misses. Costs: an OS/container write-interception sandbox generally needs **container/OS privileges beyond the repo** — a **new privileged surface the operator owns** (R4 breaking); it is heavyweight relative to the appetite (D-6: "a per-CLI write-hook config/shim," not a sandbox runtime); and the honest-layer split already assigns the *unbypassable* guarantee to the merge-boundary layers (parts 2+3), so the write hook does **not** need to catch out-of-band writes — ABUSE-4 is explicitly the gate/CI's job.
- **Principles**: AP-25 — neutral (a sandbox is not per-CLI content, but it is a new runtime dependency); AP-17 — honored in principle (a sandbox can fail closed); AP-19 — strained (a standing sandbox runtime is steady-state cost the native-hook config is not).
- **Incidents**: none checked (incidents.md; no sandbox/write-interception entry).
- **Verdict**: rejected — needs container/OS privilege beyond the repo (a new privileged surface — the R4 breaking condition), exceeds the appetite, and is unnecessary because the merge-boundary layers already own the out-of-band guarantee. The native per-CLI hook (D-4) is the R4-legal, additive, appetite-fitting choice.
- **From**: DQ-4 · panel

## Seam map

- **S-1**: the **agent-surface pre-write hook** — *opens* — behind it: the per-CLI native hook config + the canonical deny logic (classify target vs protected-path set + declared scope) on the vendor-neutral surface — feeds: the `argus-write-hook` design's Scope + `Serves: prd:argus-boundary-enforcement/FR-1..3` (D-4).
- **S-2**: the **authority-contract parse** (a sibling `authoritycontract` parse of record + its closed-parse `schema` registry: tier names, action-class tokens, `move` enum) — *opens* — behind it: the DINV-3/4/5 structural evaluation — feeds: the `argus-authority-contract-structure-arm` design's Scope + `Serves: prd:argus-boundary-enforcement/FR-4..8` (D-2).
- **S-3**: the **Argus-authorship derivation** (the commit-trailer read + the absent⇒fail-closed rule) — *opens* — behind it: the per-diff protected-path classification the attribution gate keys on — feeds: the `argus-attribution-gate` design's Scope + `Serves: prd:argus-boundary-enforcement/FR-9..11` (D-3).
- **S-4**: the **self-registered docgate check** descriptor (`{Verb, Help, NeedsMergeBase, Run}`, file-not-line) — *extends* (already decided by distributed-check-registration) — behind it: the structure arm's base-free registration (and the attribution gate's, if realized as a docgate check) — feeds: each such design's composition subsection + `Serves: design:distributed-check-registration` (D-2).
- **S-5**: the **protected-path / declared-scope predicate** — *opens, deliberately un-promoted* — behind it: the shared "is this target a protected path / outside declared scope" logic the write hook and attribution gate both need — feeds: WR-2's future shared-manifest design *only if* two independent derivations prove insufficient; for now each layer derives it (D-1, O-4).

## Facts

- **F-1**: No vendor-CLI pre-write hook exists in the repo today — the agent surface exposes only git hooks (`pre-commit`/`pre-push` → docgate) and the vendor-neutral script/skill symlinks; the vendor dirs symlink `scripts`+`skills` only (no `hooks`). — source: the repo's desk git-hook shims + the vendor-neutral surface symlinks (brainstorm:argus-boundary-enforcement#F-1)
- **F-2**: The authority contract has **no** docgate parse today — the shipped shared parse matches only the two `A-n`/`E-n` ledger classes (`problem-ledger`, `delivery-ledger`); the authority-contract class is deliberately absent from the closed-parse registry, because the authority contract carries no `A-n`/`E-n` anchors. — source: `internal/docgate/argusledger/argusledger.go` `ClassForPath` + `internal/docgate/schema/schema.go` `LedgerClasses` (brainstorm:argus-boundary-enforcement#F-5)
- **F-3**: A new docgate check self-registers with a single descriptor at package init (`{Verb, Help, NeedsMergeBase, Run}`), and the umbrella/dispatch/help are assembled from the registry sorted by verb — adding a check edits no shared line and needs no merge base when `NeedsMergeBase: false`. — source: `docs/tech-design/distributed-check-registration.md` §3.1 + `internal/docgate/gate/registry.go` (brainstorm:argus-boundary-enforcement#F-6)
- **F-4**: The DINV-3/4/5 invariants and their exact structural forms are fixed by the artifacts design + the authority-contract template's Appendix checklist: DINV-3 attribution present; DINV-4 current-tier-in-ladder + latest-history-tier == current-tier + every history row complete (date · `move` from the closed enum · resulting tier · evidence refs); DINV-5 token-membership (every ladder bundle token is a defined action-class token — "no separate no-never-grant scan"). The action-class vocabulary and the tier names are the closed sets in the artifacts design §3.3. — source: `docs/tech-design/argus-artifacts.md` §3.3, §3.5 + the `authority-contract/v1` template Appendix (brainstorm:argus-boundary-enforcement#F-3, #F-4)
- **F-5**: The `docs-gate` CI context runs the umbrella with `--merge-base origin/main`; the desk pre-push hook runs the identical umbrella verb; a base-requiring check is skipped on the base-free path exactly as the existing immutability check degrades. — source: `docs/tech-design/argus-ledger-lint.md` §3.3 + the CI workflow (brainstorm:argus-boundary-enforcement#F-7)
- **F-6**: The one failure type everywhere is `Defect{Verb, File, Op, Reason, Remedy}` — fail closed, name the fix; a malformed ledger names its verb as a parse defect, never a crash or a false pass. — source: `internal/docgate/schema/schema.go` `Defect` + `internal/docgate/argusledger/argusledger.go` (the UB-3 posture the structure arm inherits)
- **F-7**: Incident I1 — a class's own first gate cannot gate its introducing PR (the checker doesn't exist at that commit); operator review is the sole backstop for that one bootstrap PR. Incident I3 — consequential CLIs fail closed on misparse. — source: `docs/observability/incidents.md` I1, I3 (brainstorm:argus-boundary-enforcement#F-8)
- **F-8**: The design prose must satisfy the `check-content-paths` purity gate — content cites templates by version pin and skills/guides by role, never a system-footprint path (a `.pyxis/` reference in a wiki/PRD/newly-added-design markdown is a purity defect). — source: `internal/docgate/gate/reg_check_content_paths.go` (repo-layout A-2/DINV-4)
- **F-9**: R4 legality of the three chosen mechanisms — each was tested against R4's three breaking triggers (new grant · new outward surface · operator-owned boundary moved) and **passed** (none breaks): the structure-arm sibling parse (D-2) is a repo-internal Go parse + a report-only gate check; the attribution trailer (D-3) reads in-tree commit metadata via `git` with no credential/egress; the native write hook (D-4) is a per-CLI config wiring the CLI's own hook with a local allow/deny. The two mechanisms that *would* have broken R4 — a code-host-badge/credential attribution (ALT-4) and an OS/container sandbox (ALT-6) — were **rejected**, not assumed through. — source: the R4 pre-commitment in prd:argus-boundary-enforcement §6.3 R4 + this session's per-ALT R4 judgment

## Fog

- **G-1** (non-blocking — blocks: the `argus-attribution-gate` design's own `Q-n`): the exact **forgery-resistance strength** of the authorship trailer — a bare in-tree trailer vs a cryptographically-bound one (e.g. a signature the design pins to verifiable commit content) — is a design-level refinement; the *direction* (a repo-internal, credential-free, badge-independent, fail-closed-on-absence derivation) holds either way (D-3, FR-11's property vs mechanism split). Rides the authored attribution design.
- **G-2** (non-blocking — blocks: the `argus-write-hook` design's own `Q-n` + WR-2): whether the write hook and the attribution gate ultimately **share one declared-scope source** (WR-2) or keep two independent derivations — decidable only once both layers exist; the direction (each derives its own scope for now) holds (D-1, S-5). Rides the authored write-hook design.
- **G-3** (non-blocking — blocks: each merge-boundary design's rollout subsection): the precise **report-then-enforce bootstrap sequencing** per the I1 exception (which PR flips each merge-boundary layer from report-only to a required context) — a rollout detail the basis already contracts (§6.5); the direction holds. Rides each authored design's rollout/reversibility subsection.

## Out of scope

- **O-1**: Selecting or building the specific *sandbox* for part 1 — rejected on design grounds **and** as the R4 breaking condition (needs OS/container privilege beyond the repo); the write hook is a per-CLI native hook config on the vendor-neutral surface. (from D-4)
- **O-2**: A code-host author-identity / platform-badge attribution derivation — rejected (mutable badge, AP-26; new outward credential surface, the R4 breaking condition); attribution is a repo-internal commit-trailer read. (from D-3)
- **O-3**: Widening the existing immutability check's governed set onto the Argus classes / the authority contract — the classes stay disjoint; the structure arm is a *separate* self-registered check (the same disjointness the ledger-lint program preserved). (from D-2; basis NG-2)
- **O-4**: A shared declared-scope manifest design *now* — deliberately un-promoted (WR-2); each scope-deriving layer derives its own scope until two independent derivations prove insufficient. (from D-1, S-5)
- **O-5**: Any authority-contract mutation by this feature — every layer *reads* the authority contract or *reports/denies* on it; none writes it (the never-list; basis NG-3). (from D-1..D-4)

## Candidate glossary terms

*The four terms carry over from the stage-1 record (brainstorm:argus-boundary-enforcement T-1..T-4) unchanged — they land in the glossary at the introducing PR per the basis's DOD-8. This session sharpens no new term; it names mechanism-of-record choices (sibling parse, commit-trailer derivation, per-CLI native hook), which are design content, not glossary vocabulary.*

- **T-1**: **Structure arm** — carried from brainstorm:argus-boundary-enforcement#T-1 (the base-free docgate check parsing the authority contract for DINV-3/4/5). This session decides its parse-of-record is a *sibling* package, not an `argusledger` extension (D-2) — a design detail, not a new term.

## Parked product gaps

none — the basis (the merged PRD) is a complete T3 contract; this session discovered no missing requirement, contradiction, or unstated non-goal. Every mechanism question was a *design* gap the basis deliberately parked (P-1..P-3 → Q1..Q4), resolved here by legality-passing assumption, never a *requirement* hole. No bounce; no `/prd-amend` owed.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-12 |
| **Authority** | Argus (decision principal, tier-2) — `sign-gate-verdict` GRANTED at tier-2 (grant checked at act time, one uncached lookup: `sign-gate-verdict` ∈ tier-2 bundle); verdict signed by Argus per the authority contract, journaled at delivery:argus-boundary-enforcement#E-16. Not a human sign-off; the operator overturns through the normal channels. |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog. §2 Decision (per-layer direction): D-2/D-3/D-4, each panel-logged (DQ-2/DQ-3/DQ-4 fired). §4 Alternatives: ALT-1..6, ≥ 2 scored per fired panel, every entry verdicted. header Serves + Scope: the S-1..S-5 seam map + the Basis anchors (prd:argus-boundary-enforcement/FR-1..11), enumerated. §3.2 Boundaries: the O-1..O-5 set. Zero blocking fog (G-1..G-3 all non-blocking, riding the authored designs). Plus DQ-1's decomposition ruling (D-1). |
