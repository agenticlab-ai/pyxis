# Argus boundary enforcement — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Make Argus's behavioral boundaries — the never-list, grant checks, in-scope-only writing, never authoring its own authority contract — mechanically enforced (fail-closed) rather than prompt-deep, at three complementary layers. |
| **Relevant-when** | brainstorming or contracting mechanical enforcement of the Argus agent's boundaries (write-time deny hooks, the authority-contract structure lint, the Argus-attribution CI gate); checking prior rulings about codifying Argus's never-list / grant / scope boundaries; triaging why an Argus boundary is prompt-only. |
| **Idea** | Harden Argus's boundaries mechanically. Right now Argus's behavioral boundaries — the never-list, the grant checks, staying in scope, never writing its own authority contract — are prompt-deep: enforced only by the drive guide's instructions, backstopped downstream by the human merge and the docgate document gates. That violates AP-10 (codify over prompt). Build the mechanical enforcement so those boundaries become impossible to violate, not merely prompt-reliable, in three parts: (1) fail-closed write hooks on the agent surface that DENY a write to a protected path — the authority contract, any frozen/merged document, another problem's ledger, or a path outside the drive's declared scope — at the moment of the write; (2) WR-1, the deferred authority-contract structure arm: a docgate check enforcing the authority-contract invariants (DINV-3 attribution present, DINV-4 tier-in-ladder + no bare pointer flip, DINV-5 no grant of a never-class act) that the shipped argus-ledger-lint explicitly deferred; (3) an attribution gate: a CI check that fails a PR whose Argus-authored diff touches the authority contract, a governance doc, or out-of-declared-scope files. |
| **Operator** | @mdarabi (operator) — the decision authority of record |
| **Date(s)** | 2026-07-12 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | docs/prds/argus-boundary-enforcement/ |
| **Prior records** | brainstorm:argus-ledger-lint, brainstorm:argus-tech-design, brainstorm:autonomous-developer |
| **Template version** | brainstorm-record/v1 |

*Authoring note — agent principal (feature-brainstorm FR-16/FR-17). This record was driven by Argus as decision principal at tier-2, not by a live human interview. Per `.pyxis/authoring/argus.md`, each decision below was resolved by a **recorded, attributed assumption** that passed the legality tests (tier · channel · regression) and is cited to its `assumption-recorded` journal entry in `delivery:argus-boundary-enforcement`, OR would have been escalated as breaking had no legal assumption remained. Every `D-n` therefore carries an Argus assumption anchor with its `(delivery:argus-boundary-enforcement#E-n)` citation, standing until a human overturns it through the normal channels. No governance-tier content was authored or modified.*

## Orientation

| Source | Why loaded |
|---|---|
| docs/tech-design/argus-agent-v1.md | the drive protocol, the never-list (§ Never), DINV-3/6, the escalation spine — the boundaries this feature enforces |
| docs/tech-design/argus-artifacts.md | the authority-contract schema and its DINV-3/4/5, §3.3 writer discipline ("Argus never writes this file"), the never-list-is-frozen-design claim (§3.3), the deferred lint (§3.6/§7 Q-1) |
| docs/tech-design/argus-ledger-lint.md | WR-1's explicit home: §3.5 + Q2 name the deferred "third *structure* arm (DINV-3/4/5 + the templates' checklist appendices)"; its Scope/non-goals scope it OUT and hand it to a follow-on design |
| docs/architecture-principles.md | AP-10 (codify-over-prompt — the governing principle), AP-17 (fail-closed on destructive/policy), AP-23/AP-27 (check composition), AP-25 (vendor-neutral surface), AP-13/AP-26 (verified artifact) |
| docs/tech-design/distributed-check-registration.md | how a new docgate check self-registers (a `reg_*.go` descriptor, file-not-line extension) — the composition substrate parts 2 and 3 reuse |
| .pyxis/hooks/ · .githooks · .claude/ · .cursor/ · .codex/ | the agent surface: `.pyxis/hooks/` are **git** hooks (pre-commit/pre-push → docgate); `.claude/` symlinks only `scripts`+`skills`; there is **no** vendor-CLI write-hook precedent in the repo today |
| .pyxis/templates/authority-contract-template.md | the file part 2 parses; its Appendix checklist already states DINV-3/4/5 as human-walked review items — WR-1 mechanizes exactly this checklist |
| internal/docgate/argusledger/ · internal/docgate/schema/ · internal/docgate/gate/reg_check_ledger_*.go | the shipped ledger-lint arms + shared parse; ClassForPath matches only problem.md/delivery.md, so the authority contract has NO parse yet — part 2's gap |
| docs/observability/incidents.md | I1 (bootstrap gate can't gate its own introducing class), I3 (consequential CLIs fail closed on misparse) — paid-for lessons this feature must honor |
| docs/wiki/glossary.md | the citation-namespace grammar set; existing Argus terms (History arm, Cross-document arm, Authority contract, Decision principal) — the naming baseline |

**Overlaps surfaced:**
- **WR-1 (part 2)** — argus-ledger-lint §3.5 and its Q2 name this exact deferred arm ("the third *structure* arm — DINV-3/4/5 + the templates' checklist appendices") and scope it OUT of that design "by RW-5," pointing at "a follow-on brainstorm/design." This problem **is** that follow-on. *Reserved ground, not covered capability* — proceed.
- **The two shipped ledger-lint arms** (history + cross-document) do **not** cover the authority contract: `argusledger.ClassForPath` matches only `problem.md`/`delivery.md`, and argus-ledger-lint §3.1 explicitly excludes the authority contract from the history arm (its change history carries no `A-n`/`E-n` anchors). No coverage of part 2.
- **`check-immutable` / `check-governing` (docgate)** enforce frozen/governing-doc immutability at push/CI, but (a) not fail-closed *at write time* and (b) with no notion of *Argus attribution* or *drive scope*. They are the downstream doc-gate backstop the idea names as insufficient. Parts 1 and 3 are genuinely new even where they touch the same paths.
- No existing capability performs write-time deny (part 1) or Argus-authorship-scoped CI refusal (part 3). No coverage.

*Conclusion: no covering source; proceed to framings.*

## Framings

*Three genuinely distinct theses of what this feature is — different answers to "what is the unit being contracted?"*

- **Framing A — Three separate features (a program).** Treat each part as its own PRD/feature, because the three mechanisms (a write-time desk hook, a push/CI structure lint, a CI attribution gate) are disjoint at three layers with three precedent-statuses. *Implications:* maximal decomposition, three contracts, three review cycles; but fragments one coherent security goal into three, loses the defense-in-depth thesis at the contract layer, and contradicts the operator's explicit instruction to drive **one** brainstorm → **one** PRD 000-base. Opens: independent shipping of each part.
- **Framing B — One feature: three-layer defense-in-depth enforcement of one boundary set (chosen).** The unit is *Argus's boundary set*, enforced at three complementary layers so a boundary is caught (i) fast, at write time on the agent surface (the write hooks), (ii) unbypassably, at the gate/CI layer for the two invariants a diff can express (the structure arm + the attribution gate). The parts are **one contract, three requirement-clusters**; downstream design decomposes them (each part its own technical design, per RW-5). *Implications:* one PRD contract, the defense-in-depth thesis is first-class, honest about what each layer can and cannot guarantee; matches the operator's one-PRD instruction. Opens: a coherent boundary-enforcement contract whose parts share fit criteria and non-goals.
- **Framing C — Extend the shipped argus-ledger-lint feature (amend its PRD to add the third arm + the write/attribution layers).** Treat this as evolving the existing ledger-lint contract. *Implications:* reuses that PRD's substrate framing; but ledger-lint's own §3.5/RW-5 deliberately scoped the structure arm OUT as a *separate* effort, and parts 1 and 3 are not ledger-lint at all (they are agent-surface and CI-attribution, not ledger parsing) — folding them into ledger-lint would violate the one-decision-per-document discipline ledger-lint itself invoked. Rejected as a category error.

**Destination:** **Framing B** — one PRD contract specifying three-layer, fail-closed, defense-in-depth mechanical enforcement of Argus's boundary set; the three parts are three requirement-clusters within it, decomposed to separate technical designs downstream. *Rationale (Argus, tier-2 assumption D-1):* the operator's idea frames the three parts as one hardening goal and directs one brainstorm → one PRD; the parts share one subject (the boundary set) and one governing principle (AP-10 + AP-17), which is the definition of one feature; RW-5's one-decision-per-*document* is honored at the design stage (each part its own design), not violated by one PRD spanning three clusters — exactly as argus-ledger-lint was one PRD whose two arms plus a deferred third were then split across designs.

## Decisions

### D-1 — Destination: one feature (three-layer), not a program of three

- **Question**: Is this ONE feature (one PRD contract with three enforcement clusters) or a program of three separate features?
- **Answer**: ONE feature — Framing B. Three-layer defense-in-depth enforcement of one boundary set; the three parts are three FR-clusters in one 000-base; downstream design decomposes them (each part likely its own technical design under RW-5).
- **Reasoning**: The parts share one subject (Argus's boundary set) and one governing principle (AP-10/AP-17) — the definition of one feature. The operator directed one brainstorm → one PRD. RW-5's one-decision-per-document binds the *design* stage, not the PRD; a single PRD spanning three clusters mirrors argus-ledger-lint (one PRD, arms split across designs). Framing A fragments the security goal at the contract layer; Framing C mis-nests into a feature that deliberately scoped this out.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-2)
- **Status**: live

### D-2 — Enforcement layers are operator-given; mechanism detail defers to technical design

- **Question**: What is the enforcement model — hooks vs sandbox vs CI, and which layers? Is the security-model choice a decision I (Argus) may assume through, or a breaking decision the operator owns?
- **Answer**: The *layer selection* is **given by the idea** (not assumed): (1) a write-time deny on the agent surface, (2) a docgate structure check at push/CI, (3) an Argus-attribution CI gate. The PRD contracts the **observable behavior** of each layer. The **mechanism detail** — which specific vendor-CLI hook API (Claude PreToolUse / Cursor `.cursor/hooks.json` / Codex), sandbox-vs-hook for part 1, and exactly how "Argus-authored" is derived (git trailer, journal cross-reference, author identity) — is a **technical-design decision (stage 4)**, parked here as a technical constraint (P-1..P-3), not resolved in the PRD.
- **Reasoning**: The prd-template ownership boundary (§0) puts observable behavior in the PRD and architecture/mechanism in the design. The security *posture* (fail-closed, mechanically enforced, "impossible to violate not merely prompt-reliable") is the operator's own ruling — it **is** the idea — so recording it is recording an operator boundary, not moving one; **not breaking**. The cost/risk tradeoff the operator owns (is fail-closed desk-enforcement worth its cross-CLI portability cost?) is a *design-stage* mechanism tradeoff the operator still reviews at stage 5; deferring it to design with the appetite as the envelope is the correct altitude, not an assume-through. See the escalation-honesty note below D-4.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-3)
- **Status**: live

### D-3 — Scope perimeter and non-goals

- **Question**: What is in scope, and what are the deliberate refusals/deferrals?
- **Answer**: **In:** the three enforcement layers as one behavioral contract over Argus's boundary set (protected-path write-deny; authority-contract DINV-3/4/5 structure lint; Argus-attributed protected-path CI refusal). **Non-goals (NG):** (a) selecting or building a *specific* vendor-CLI hook mechanism — that is design (D-2); (b) widening `check-immutable`'s governed set (argus-artifacts §3.8 disjointness — the same refusal ledger-lint took); (c) the engine-era runtime/registry-verb graduation of these checks; (d) any new grant, tier change, or edit to the authority contract by this feature — it *enforces* the contract, never mutates it (the never-list). **Waiting room (WR):** engine-era graduation of these mechanical checks to registry-born verbs (AP-10's arc).
- **Reasoning**: The In set is the idea's three parts. NG-(a) follows D-2's altitude split. NG-(b) preserves argus-artifacts §3.8's deliberate disjointness (a divergent widening would false-red legal ledger appends and re-open a closed decision). NG-(c) mirrors ledger-lint's engine-era deferral. NG-(d) is the never-list made explicit: this feature must not itself become a path that writes the authority contract.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-4)
- **Status**: live

### D-4 — Honest enforceability: the write hook is the fast layer, the gate/CI is the unbypassable layer

- **Question**: Can part 1 (write-time deny hooks) actually make a boundary "impossible to violate," and how do the three parts divide that guarantee?
- **Answer**: No single layer makes it impossible. A desk-side write hook denies writes made **through a hook-honoring agent surface** — it cannot stop an out-of-band bypass (a raw shell redirect, a CLI that does not honor the hook, a direct git write). Therefore the guarantee is **divided by design**: part 1 is the **fast, at-write-time** deny on the supported agent surfaces; parts 2 and 3 are the **fail-closed, unbypassable backstop at the gate/CI layer** that no desk bypass escapes — the structure arm catches an authority-contract DINV-3/4/5 violation on push/CI regardless of how the bytes got there, and the attribution gate refuses a PR whose Argus-authored diff touches a protected path. Fit criteria are set to this honest split: part 1's fit is "denies a protected-path write at write time through each supported agent CLI surface," never "OS-level unbypassable"; the unbypassable claim is carried by parts 2+3 at CI (the required context that must be green to merge).
- **Reasoning**: AP-13/AP-26 (a claim must be a verified artifact; a status certifies only its own object) forbid over-claiming part 1. AP-17 (fail-closed on the policy surface) is satisfied at the merge boundary by parts 2+3, which is where "impossible to violate, not merely prompt-reliable" is actually delivered — the human merge already gates, and these make the gate mechanically refuse the violation rather than relying on review to notice. This is the load-bearing reason the three layers are one feature (D-1).
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-5)
- **Status**: live

### D-5 — Segments and the human-author boundary

- **Question**: Who is this for, and who must it deliberately NOT impede?
- **Answer**: **Primary segment:** Argus (the agent whose writes are gated) and the operator (who relies on the enforcement holding at the merge boundary). **Anti-segment:** a **human author acting outside a drive** — editing the authority contract, a governance doc, or any file by an ordinary human-authored PR. The enforcement must not block legitimate human authorship: the attribution gate keys on *Argus-authored* diffs (part 3 explicitly says "Argus-authored diff"); the write hooks are scoped to the agent-drive surface; the structure arm (part 2) is content-legality only (it fails a malformed/never-granting authority contract regardless of author, which is correct — a human PR that breaks DINV-3/4/5 *should* fail too, exactly as the template checklist already demands of a human reviewer).
- **Reasoning**: The idea's part 3 names "Argus-authored diff" — attribution scope is operator-given. argus-artifacts §3.3 writer discipline ("Argus never writes this file"; "a human lands [changes] by PR") means the human-author path must stay open; only the *Argus* path is closed. Part 2's author-independence is a feature, not a bug: the authority contract's DINV-3/4/5 are structural legality, which any author must satisfy (NR-3-style author-independence, matching the gate-suite's author-independent-verdict property).
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-6)
- **Status**: live

### D-6 — Appetite

- **Question**: What total spend is this problem worth (machine-native: human attention + machine work + rollout burden)?
- **Answer**: Bounded. **Machine work:** three self-contained additive artifacts — a per-CLI write-hook config/shim on the vendor-neutral surface (new substrate, part 1), one self-registered docgate structure check reusing the settled `distributed-check-registration` + `argusledger`/`schema` substrate (part 2), and one CI attribution check (part 3, composed under the same self-registration where it is a docgate check, or a discrete CI step where it needs git-authorship context). **Human attention:** operator review of one PRD, then one review per downstream design (three parts → up to three designs) and their plans. **Rollout burden:** the checks ship report-then-enforce where the bootstrap rule (I1) requires it (a class's own first gate can't gate its introducing PR); the write hooks ship fail-closed per the operator's posture. No new egress, identity, or outward surface — repo-internal enforcement only.
- **Reasoning**: Part 2's substrate is fully settled and dogfooded (sixteen-plus checks self-register today), so its cost is one check body + fixtures. Part 1 is the genuine new-substrate cost (vendor-CLI hooks, cross-CLI). Part 3 is a CI check with authorship-scoping logic. The appetite is a budget shaping the solution, not an estimate; the mechanism choices that could eat it (part 1's cross-CLI hook portability) are named as rabbit holes for the design stage.
- **Anchor**: 2026-07-12 · Argus assumption (delivery:argus-boundary-enforcement#E-7)
- **Status**: live

## Facts

- **F-1**: The `.pyxis/hooks/` shims are **git** hooks (`pre-commit`/`pre-push`) delegating to `docgate-resolve.sh`; there is no vendor-CLI write-hook (PreToolUse-style) in the repo, and `.claude/` symlinks only `scripts`+`skills` (no `hooks`). — source: .pyxis/hooks/pre-commit, .pyxis/hooks/pre-push, `ls .claude/`
- **F-2**: WR-1 is named and scoped-out in argus-ledger-lint §3.5 and Q2 as "the third *structure* arm (DINV-3/4/5 + the templates' checklist appendices)," "a single-file, base-free, different-DINV check," home = "a follow-on brainstorm/design." — source: docs/tech-design/argus-ledger-lint.md §3.5, §7 Q2
- **F-3**: The authority-contract invariants are DINV-3 (every `A-n`/`E-n` carries principal identity + authority-contract version — attribution present), DINV-4 (`current-tier` always names a declared tier + every change appends a history entry with evidence refs; a bare pointer flip with no history entry is invalid), DINV-5 (no content of the authority-contract file can grant a never-class act; the never-list binds from frozen design/contract text). — source: docs/tech-design/argus-artifacts.md §3.5
- **F-4**: The authority-contract **template's Appendix checklist** already states exactly these as human-walked review items: "Current tier names a tier declared in the ladder"; "every bundle token a defined action-class token — never-class acts are absent from the vocabulary by construction (DINV-5 is structural)"; "every history row carries date, move, resulting tier, evidence refs"; "the latest history row's tier equals Current tier." WR-1 mechanizes this checklist. — source: .pyxis/templates/authority-contract-template.md Appendix
- **F-5**: `argusledger.ClassForPath` matches only `docs/problems/*/problem.md` and `docs/problems/*/delivery.md`; the authority contract (`.pyxis/skills/argus/authority-contract.md`) has no parse in the package — part 2 must add authority-contract parsing (reusing the region/anchor registry pattern in `internal/docgate/schema`). — source: internal/docgate/argusledger/argusledger.go
- **F-6**: A new docgate check self-registers via a single `reg_*.go` descriptor `{Verb, Help, NeedsMergeBase, Run}` at package init (`internal/docgate/gate/`); the umbrella/dispatch/help are derived from the registry by verb — adding a check edits no shared line (AP-23/AP-27, distributed-check-registration DINV-2). — source: docs/tech-design/distributed-check-registration.md §3.1, internal/docgate/gate/reg_check_ledger_citations.go
- **F-7**: The `docs-gate` CI context runs `docgate gate --merge-base origin/main`; the pre-push hook runs the identical umbrella verb (desk/CI parity, docgate-v1 DINV-4). A base-requiring check is skipped on the base-free path exactly as `check-immutable` degrades. — source: .github/workflows/docs-gate.yml, docs/tech-design/argus-ledger-lint.md §3.3
- **F-8**: Incident I1: a class's own first gate cannot gate its introducing PR (the checker doesn't exist yet at that commit); operator review is the sole backstop for that one bootstrap PR. Incident I3: consequential CLIs must fail closed on misparse. — source: docs/observability/incidents.md I1, I3

## Fog

none — every decision-shaped input (destination, enforcement layers, scope/non-goals, appetite, segments) resolved by a legality-passing recorded assumption; no blocking fog remains. Mechanism-detail unknowns are carried forward as **parked technical constraints (P-1..P-3)** for stages 3–4, and as the draft PRD's non-blocking open questions (§7.2), not as fog on this record's decision-shaped inputs.

## Out of scope

- **O-1**: Selecting or building a *specific* vendor-CLI hook mechanism (Claude/Cursor/Codex hook API, sandbox-vs-hook for part 1) — a technical-design decision, not a PRD one. (from D-2, D-3)
- **O-2**: Widening `check-immutable`'s governed set onto the Argus classes — the classes stay disjoint from it; part 2 is a *separate* self-registered check (argus-artifacts §3.8; the refusal ledger-lint took as its NR-2). (from D-3)
- **O-3**: The engine-era runtime/closed-parse reader and the graduation of these mechanical checks to registry-born verbs — the engine-era other half, deferred (WR). (from D-3)
- **O-4**: Any new grant, tier promotion, or edit to the authority contract by this feature — it enforces the contract, never mutates it (the never-list is out of reach of this feature's writes). (from D-3)

## Candidate glossary terms

*Terms born or sharpened this session, for the domain glossary — routed into `docs/wiki/glossary.md` at synthesis (`/prd-new`), in the same introducing PR.*

- **T-1**: **Structure arm** — the third Argus ledger-lint arm: a base-free docgate check parsing the authority-contract file for its structural invariants (DINV-3 attribution present · DINV-4 tier-in-ladder + no bare pointer flip · DINV-5 no grant of a never-class act) — the mechanical form of the authority-contract template's Appendix checklist. Complements the shipped *history arm* and *cross-document arm*. Avoid: "immutability check" (`check-immutable`'s disjoint set), "ledger arm" (it targets the authority contract, not the `A-n`/`E-n` ledgers).
- **T-2**: **Attribution gate** — a CI check that fails a PR whose **Argus-authored** diff touches a protected path (the authority contract, a governance doc, or a file outside the drive's declared scope); keys on authorship, so it never impedes a human-authored PR to the same paths. Avoid: "author check" (too broad — it is scoped to Argus and to protected paths), "governance gate" (`check-governing` owns governing-doc immutability regardless of author).
- **T-3**: **Protected path** — a repo path a drive may not write: the authority contract, any frozen/merged document, another problem's ledger, or a path outside the current drive's declared scope. The write-hook layer denies a write to one at write time; the attribution gate refuses an Argus-authored diff touching one at CI. Avoid: "governed path" (overlaps the narrower governing-doc set).
- **T-4**: **Declared scope** — the set of paths a given drive is permitted to write (its own problem's ledgers, the documents it is authoring for that problem); a write outside it is a protected-path violation. The unit the write hooks and the attribution gate both test "in-scope-ness" against. Avoid: "workspace" (implies a filesystem sandbox, which is one candidate mechanism, not the concept).

## Parked technical constraints

*Surfaced technical matter conveyed to stages 3–4 (design brainstorm / technical design), never resolved here.*

- **P-1**: **Per-CLI write-hook mechanism (part 1).** Each vendor CLI exposes a different pre-write hook surface with different fail-open/fail-closed defaults (Claude PreToolUse; Cursor `.cursor/hooks.json`, which fails **open** by default and needs `failClosed`; Codex hooks; OpenCode's hook gap). The design must choose per-CLI adapters on the vendor-neutral surface (AP-25) with a stated fail-closed guarantee per CLI, and a stated sequential/degradation fallback where a CLI exposes no honoring hook — at which point the gate/CI backstop (parts 2+3) is the guarantee. Sandbox-vs-hook is the design's alternative to weigh.
- **P-2**: **How "Argus-authored" is derived (part 3).** Candidates: an operator/agent-trailer convention on the commit, a cross-reference to the delivery journal's attributed entries, or a code-host author identity. The design must pick a derivation that is forgery-resistant within the trust model, works in CI without ambient credentials (AP-14 deployment-context), and does not depend on a badge (AP-26). This is the attribution gate's load-bearing seam.
- **P-3**: **Authority-contract parse + composition (part 2).** The structure arm must parse the authority-contract file (not currently in `argusledger.ClassForPath`), reusing the version-pinned region/anchor registry pattern in `internal/docgate/schema` and composing as a single `reg_*.go` self-registered descriptor (base-free, `NeedsMergeBase: false`), adding no shared umbrella line (AP-27). DINV-5's structural nature (never-class acts are absent from the token vocabulary by construction) means the check verifies "every bundle token is a defined action-class token," not a separate "no-never-grant" scan — exactly the template checklist's phrasing. Whether it reuses ledger-lint's per-class package or lands in a sibling package is a design call.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-12 |
| **Operator** | Argus (decision principal, tier-2) — `sign-gate-verdict` GRANTED at tier-2; verdict signed by Argus per the authority contract, journaled at delivery:argus-boundary-enforcement#E-8. Not a human sign-off; the operator overturns through the normal channels. |
| **Covering source** | — |
| **Coverage check** | pass — all six decision-shaped inputs covered (§2.1 problem via Destination/D-1; §2.2 segments via D-5; §2.3 jobs via D-1/D-4 enforcement jobs; §3.1 goals direction via D-1/D-4; §3.4 non-goals via the O-1..O-4 set; header appetite via D-6), zero blocking fog. |
