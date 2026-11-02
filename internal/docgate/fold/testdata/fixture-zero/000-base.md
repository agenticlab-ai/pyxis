# docgate — the document gate and fold — PRD

| Field | Value |
|---|---|
| **In one line** | The deterministic tool that folds PRD ledgers into projections and gates document validity in CI and at pre-push. |
| **Relevant-when** | planning, building, or reviewing the fold, the gate verbs, or ledger validation; triaging defects in projection generation or gate verdicts; changing anything that parses PRDs, amendments, or templates |
| **Status** | Problem review ✓ · Solution review ✓ (2026-07-06) — launch review pending, post-implementation |
| **Tier** | T2 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | operator (product) · factory agents (implementation) |
| **Informed** | future adapter and skill authors |
| **Date** | 2026-07-06 created · 2026-07-06 last updated |
| **Appetite** | small: ≤2 human review rounds per document; modest worker spend; zero rollout/support burden (repo-internal tool) |
| **Target release** | precondition for the first real per-feature PRD beyond this one — nothing else lands first |
| **Tracking** | introducing PR (filed on approval) |
| **Artifacts** | docs/tech-design/prd-amendment-model.md · docs/tech-design/tech-design-conventions.md · golden fixture corpus (arrives with implementation) |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — Approved PRDs must evolve without trusting any editor's discipline: today nothing regenerates or verifies the projection, nothing rejects malformed amendments, and nothing stops history edits — the ledger/projection invariant exists only as prompt text (AGENTS.md rule 8).
- **For whom** — agents authoring amendments (routinely, once real PRDs exist), human reviewers of amendment PRs, and CI on every PR touching `docs/prds/`.
- **Why now** — the amendment model and all three templates are merged; this document is itself the first ledger base, so the guarantee must arrive before the ledger grows.
- **Direction** — one static Go binary: a deterministic fold plus six gate verbs, wired as a required CI context and the identical pre-push hook. The factory's first code, enforcing its own document contract.
- **Appetite** — as header; deliberately stdlib-only and small.
- **Success** — 100% of fixture-corpus folds byte-identical across runs and platforms at the checkpoint "golden suite green in CI".
- **Guardrail** — zero hand-edited projections merged once the gate is required.
- **Top risks** — markdown-table parsing fragility; template/schema evolution outrunning the goldens; gate friction driving authors away from ledger directories.

## 2. Problem space

### 2.1 Problem statement

An approved PRD is a contract consumed by humans, reviewers, and (soon) the Contract Compiler. It must change as building teaches — but every mechanism that lets LLMs edit documents in place decays the contract. The amendment model solves this structurally: an append-only ledger of typed amendments, folded deterministically into a generated projection. Without the tool, the model is prose: projections would be hand-maintained (the drift the model exists to kill), immutability is prompt-side only, and malformed amendments are discovered by human reviewers late instead of by machines instantly. The cost of doing nothing is that the document system we just built operates on the honor system.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | factory agents authoring amendments in this repo | journeyman | master | a valid amendment on the first try; instant local verdicts | no mechanical feedback until human review | amendment-model review threads |
| P2 | human reviewers of amendment PRs | master | master | review the *effect*, trust the projection unconditionally | must mentally fold ledgers | prd-amendment-model §5 (`diff` rationale) |
| P3 | CI / branch protection | — | — | one deterministic verdict per PR | nothing enforces INV-41's CI half | INV-41; AGENTS.md rule 8 ("prompt-side half") |
| **Anti-segment** | authors of the placeholder corpus (flat `docs/prds/*.md`) | — | — | — | — | operator ruling: exempt, never retrofitted |

### 2.3 Jobs and desired outcomes

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I have authored an amendment | regenerate and verify the projection locally | open a PR that passes on the first try | hand-run the documented fold discipline | prd-amendment-template §0 (transitional note) |
| J2 | I review an amendment PR | see before/after effect, affected IDs, and computed blast radius | judge the change without replaying the ledger | mental fold | prd-amendment-model §5 |
| J3 | a PR touches a ledger | have CI refuse stale projections, edited history, and malformed amendments | trust `effective.md` without reading `docs/prds/<feature>/` history | none | prd-amendment-model §2 |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| Hand-maintained current-state docs drift ("the README that lied for a week") | VNEXT-6; predecessor incident history | High |
| Byte-deterministic doc tooling is achievable in this repo | existing golden-tested generators are byte-deterministic across runs | High |
| Desk-time + CI running the identical predicate prevents divergence | AP-11 / INV-30; GATE-2 lineage (local CI mirror) | High |
| Agents will attempt to "helpfully" repair generated artifacts | prd-amendment-model §5 actor table; observed LLM behavior during this repo's own tooling work | High |

### 2.5 The journey today

Skipped: the pre-tool journey is fully described in §2.1 — one manual path (hand-fold, attach a diff, hope) with no variation worth mapping.

### 2.6 Opportunity and why now

- **Size** — every future PRD and amendment flows through this gate; it is the trust anchor for stage-6 compilation and the first executable piece of the document system.
- **Alternatives users have** — keep prose discipline (drift, proven by the predecessor); LLM-verified projections (unverifiable — rejected in the amendment model's alternatives).
- **Why us / strategic fit** — RW-4's spirit: substrate first; the factory's first Go code enforces its own document contract.
- **Why now** — the model and templates are merged and exactly one ledger exists (this document): the guarantee can be born-in rather than migrated-to.
- **Cost of doing nothing** — the amendment model remains ceremony; drift returns wearing a process costume.

**Decision:** Problem gate — signed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | 2026-07-06 | approved |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a deterministic fold + byte-verifying gate | projections trusted without reading ledgers | M1 |
| G2 | instant local verdicts (pre-push, identical binary) | amendments passing CI on the first try | M2 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | fixture folds byte-identical across runs/platforms | n/a (no tool) | 100% at "golden suite green in CI" | measurement plan: golden-suite results in CI |
| M2 | G2 | amendment PRs green on first docgate run | n/a | ≥80% at "10 amendments through the required gate" | measurement plan: PR check history via `gh` |

*(§5.7 tracking plan skipped with reason — no product analytics in a hermetic tool; both metrics are measured from CI and PR records, plans stated above.)*

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| hand-edited projection merged | any single occurrence | incident entry + gate-wiring fix before the next amendment merges |
| false-red rate on valid fixtures | >0 in the golden suite | fold bug — fix before enforcement widens |

**Ship rule (pre-registered):** golden suite 100% green AND both templates' example ops parse as fixtures → wire as a required context; any guardrail breach → unwire, fix, re-wire.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Gating the placeholder corpus (flat `docs/prds/*.md`) | operator ruling: placeholders are exempt, never retrofitted |
| NG-2 | Semantic conflict detection (an `answer` contradicting a live row, duplicate-in-different-words) | the model's tier boundary: semantic conflicts belong to review and the Contract Compiler (VNEXT-1) |
| NG-3 | Any LLM invocation inside the tool | AP-10; the gate job is hermetic — no network, no model |
| NG-4 | Design-doc verbs and the generated design index | scoped to v2 by the tech-design conventions; v1 ships the approved PRD scope only |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | docgate v2: design-header lint, mandatory-subsection + security-pair checks, `Supersedes`/`Serves` resolution, index generation | after v1 is proven; scope enumerated in tech-design-conventions §3.4 |
| WR-2 | `docgate diff` output posted as a PR comment | when GitHub-adapter work begins |
| WR-3 | absorption into the engine as the Contract Compiler's front door | at that component's design (prd-amendment-model §7 Q4) |

## 4. Solution space

### 4.1 Approach and alternatives considered

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | one Go binary: `fold` + six gate verbs, stdlib-only, golden-fixture-driven | — |
| B | a new Node script | violates Go-by-default and introduces an unnecessary runtime |
| C | CI-only checks (no local binary) | violates AP-11 — desk and CI must run one predicate; agents would discover failures minutes late |
| Do nothing | prose discipline | drift wins; §2.6 |

### 4.2 Scope perimeter

- **In (plan of record)** — `fold`; `lint-structure`; `check-immutable`; `validate-amendments`; `verify-generated`; `diff`; `verify-rebase`; required-context + pre-push wiring; the golden fixture corpus.
- **Future (believed next, not committed)** — WR-1 (v2 design scope), WR-2, WR-3.
- **Out** — NG-1..4.

### 4.3 Experience: flows and states

**Flow F1 — the author loop (serves J1)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | writes `NNN-<slug>.md` from the amendment template | — | — | prd-amendment-template |
| 2 | runs `docgate fold docs/prds/<feature>` | regenerates both projection artifacts | ok · named defect | prd-amendment-model §4 |
| 3 | runs `docgate diff docs/prds/<feature>` | before/after, affected IDs, computed blast radius, invalidation levels | ok | prd-amendment-model §5, §12 |
| 4 | pushes | pre-push runs the gate verbs; CI repeats them identically | green · red with named defects | prd-amendment-model §5 |

*(five-UI-states bullets skipped with reason — a CLI's states are exit codes and defect messages, specified as FR-2 and the UB rows; accessibility/localization skipped — terminal tool, English-only repo convention.)*

### 4.4 Scenarios

A worker supersedes a Must row: `validate-amendments` computes contract-changing, the declared tier matches, `diff` attaches the folded effect, review runs at the full bar, merge freezes the file. — A rushed agent hand-patches `effective.md` to "fix" a typo: `verify-generated` recomputes, bytes differ, CI goes red naming the file; the fix is an amendment op, and the defect message says so. — Two agents race for `003`: the loser gets the dense-numbering defect naming the next free number, renumbers, and passes.

**Decision:** Solution gate — signed; zero solution-blocking TBDs open.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | 2026-07-06 | approved |

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked as `fold`, docgate shall emit `effective.md` and `effective.source.json` from the ledger deterministically, with the content fingerprint over normalized replayed-ledger bytes and the tool version recorded separately. | the projection is derived, never authored; identity tracks contract, not tooling | Must | med | byte-identical output across 100 runs on 2 platforms over the fixture corpus; fingerprint stable across a tool-version bump with unchanged ledger | Test | G1, J1 |
| FR-2 | If an operation is invalid (dangling target, double-supersede, ID reuse, schema-violating payload), then docgate shall abort with a defect naming file, op, reason, and remedy — as human text on stderr and as one machine-readable JSON line per defect on a `--defects-json` stream. | fail closed; authors fix fast; CI annotates precisely | Must | med | 100% of fixture defect cases produce the named quadruple in both forms; JSON parses against the defect-schema golden; zero best-effort output | Test | J1, J3, G2 |
| FR-3 | When invoked as `verify-generated`, docgate shall recompute the fold hermetically and byte-compare every generated artifact against the committed set, failing on any difference or fingerprint mismatch. | unforgeable projection | Must | high | hand-edited fixture projection → red naming the artifact; regenerated → green | Test | G1, J3 |
| FR-4 | When invoked as `check-immutable` with a merge base, docgate shall fail on any modify, delete, or rename of an existing ledger file. | INV-41's CI half; merge is the freezing event | Must | high | fixture M/D/R each red; add-only green; rename detected as delete+add | Test | J3 |
| FR-5 | When invoked as `validate-amendments`, docgate shall enforce the closed op vocabulary, `Status: Approved` at merge-target, typed `Learned-from` evidence classes, and a computed blast radius that the declared one may exceed but never understate. | the envelope contract; declarations are checked, not trusted | Must | high | one fixture per violation class, each red with its named defect | Test | J1, J3 |
| FR-6 | When invoked as `diff`, docgate shall render the projection before/after, affected IDs, computed blast radius, and downstream invalidation levels. | reviewers judge the effect, not the mechanics | Must | low | fixture amendment yields all four outputs with expected IDs and levels | Test | J2 |
| FR-7 | When invoked as `verify-rebase`, docgate shall fail any rebase checkpoint that is not semantically identical to the projection immediately preceding it. | rebases launder nothing | Must | high | tampered fixture rebase red; faithful one green | Test | G1 |
| FR-8 | When invoked as `lint-structure`, docgate shall verify every ledger directory has exactly one `000-base.md`, dense monotonic numbering, and headers parsing against their declared template versions. | malformed layout never lands | Must | med | gap, duplicate, missing-base, and unknown-header fixtures each red with named files | Test | J3 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If ledger numbering has a gap or duplicate | fail lint naming the files and the next free number | named defect + remedy | Must | low | both fixture cases name files and the number | Test | F1, FR-8 |
| UB-2 | If a payload table's columns mismatch the pinned template schema | abort the fold naming the column diff | named defect | Must | med | reordered and missing-column fixtures both named | Test | FR-2 |
| UB-3 | If the working tree contains uncommitted ledger changes during `verify-generated` | verify against the working tree exactly and say so in output | verdict + dirty-tree note | Should | low | dirty-tree fixture produces the note and correct verdict | Test | J1 |

### 5.3 Permissions and abuse cases

Skipped: repo-local CLI with no roles and no new surface — the gate itself is the abuse control (FR-3/FR-4), and its own change path is governed by this repo's review rules.

### 5.4 Data

Skipped: no state created or owned — inputs are tracked files; outputs are the two generated artifacts whose contract is FR-1/FR-3. No retention, no privacy surface, no egress.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| git (merge-base name-status) | in | immutability check | M/D/R detection vs merge base | rename appears as delete+add → red | FR-4 |
| CI required context `docs-gate` | out | the enforcement point | exit code + human defects + `--defects-json` | red blocks merge (contexts calibrated per GATE-7 lineage) | FR-2..8 |
| pre-push hook | out | desk-time verdicts | the identical binary and verbs | AP-11: never a twin predicate | G2 |
| templates (`docs/templates/*`) | in | anchor + payload schema registries, version-pinned | schemas derive from templates; template PRs ship updated goldens | schema drift → goldens fail in that PR | FR-1, FR-8 |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Performance | docgate shall verify a 50-entry ledger fast enough for pre-push. | p95 < 1s over the fixture corpus on developer hardware | Test |
| NFR-2 | Operability | docgate shall run hermetically as a single static binary. | zero network syscalls in the gate path; no runtime deps beyond the binary | Analysis |
| NFR-3 | Maintainability | the repo's Go rules apply in full. | gofmt/go vet/golangci-lint clean; `go test -race` green; stdlib-only | Inspection |

### 5.7 Instrumentation — the tracking plan

Skipped with reason: see §3.2 — metrics are measured from CI and PR records; a hermetic tool carries no runtime telemetry.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..8 | Test | CI golden-suite run | yes |
| DOD-2 | No unwanted-behavior or non-regression row broken | UB-1..3, NR-1 | Test | suite result | yes |
| DOD-3 | Gate wired: required CI context + pre-push hook, same binary, same verbs | §5.5 | Inspection | branch-protection config + hook file | yes |
| DOD-4 | Both templates' example ops parse as golden fixtures | §5.5 templates row | Test | fixture manifest | yes |
| DOD-5 | This ledger's own projection is generated and verified by the shipped tool | FR-1, FR-3 | Test | committed `effective.*` for `docs/prds/docgate/` passing `verify-generated` | yes |
| DOD-6 | Guardrails intact through the first 10 gated amendments | §3.3 | Analysis | PR history at the M2 checkpoint | yes |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The placeholder corpus and every non-ledger doc are never gated, touched, or reformatted | NG-1; operator ruling | low | a gate run over the current repo yields zero findings outside `docs/prds/<feature>/` directories | Test | NG-1 |
| NR-2 | Existing document tooling and its tests keep passing | docgate is additive tooling | low | document-tooling suites green in the introducing PR | Test | §5.5 |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Fixtures-first | pin the contract before code | golden corpus derived from the templates' examples + the model's defect classes | goldens reviewed and committed |
| Fold | the hard core | FR-1, FR-2, UB-2 | fold goldens green; determinism fit criterion met |
| Verbs | the gate | FR-3..8, UB-1, UB-3 | one fixture per defect class red/green as specified |
| Wire | enforcement | §5.5 CI + pre-push; DOD-5 self-projection | DOD-3, DOD-5 |

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| amendment model + all three templates merged | docs | Fixtures-first | confirmed (#13–#15, #18–#19) | — |
| approved technical design (RW-5) | docs | Fold | next step after this PRD | @mdarabi |
| branch-protection edit rights | approval | Wire | confirmed | @mdarabi |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | markdown-table parsing fragility — the fold's rabbit hole | M | H | closed parser, hard errors, one fixture per op × table kind; no lenient mode exists to accrete around | implementation |
| R2 | template evolution desyncs the goldens | M | M | template PRs ship updated goldens in the same change (prd-amendment-model R3); the goldens fail otherwise | implementation |
| R3 | gate friction drives authors away from ledger directories | L | H | pre-push verdicts in seconds; every defect names file, reason, and remedy | @mdarabi |
| R4 | the self-projection bootstrap (DOD-5) surfaces fold bugs against this very document | M | L | that is the point — this PRD is fixture zero; bugs found here are bugs not found in the field | implementation |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-1, DOD-2, DOD-4 | golden suite in CI |
| Reliability | FR-1's determinism criterion on both platforms | CI matrix |
| Performance | NFR-1 | timed suite |
| Supportability | every defect names file, op, reason, remedy; the defect catalog doubles as the runbook | fixture review |

### 6.5 Rollout and reversibility

Safety plan (staged product rollout n/a — repo-internal tool): the required-context wiring is the switch — unwiring disables the gate instantly with nothing else depending on it; the pre-push hook is advisory until CI is wired; the first 10 gated amendments run with reviewer double-check per the §3.3 guardrail window; `verify-generated` failures never block by deletion — the remedy is always regenerate-and-recommit.

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | AGENTS.md rule 8's transitional clause replaced by the gate reference; amendment template's provisional note updated | @mdarabi | ☐ |
| Support | defect-message catalog committed beside the fixtures (doubles as the runbook) | implementation | ☐ |

*(remaining rows skipped: no pricing, marketing, legal, localization, or enablement surface — repo-internal tool)*

**Decision:** Launch gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | pending |

### 6.7 Post-launch review

- **Review trigger** — 10 amendment PRs through the required gate · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with M1/M2 and guardrail evidence

## 7. Alignment

### 7.1 FAQ

**Customer (authoring agents):** **Q:** The projection looks wrong — can I fix `effective.md` directly? — **A:** Never. Fix the amendment op, or file a fold bug; `verify-generated` rejects hand-edits by construction, and the defect message will say exactly this.

**Internal:** **Q:** Why not ship the design-doc verbs (v2) now — the conventions are merged? — **A:** Scope discipline: v1 is the approved amendment-model scope; v2's checks depend on a proven parser and are enumerated in tech-design-conventions §3.4 (WR-1). One proven layer before the next.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | `verify-rebase` semantic equality: byte-equality of the regenerated projection, or normalized-AST equality? | tech design | Fold phase | open |
| Q2 | Pre-push hook installation mechanics — pyxis has no workbench installer yet (BENCH-2 is baseline inventory, not built) | tech design | Wire phase | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-06 | fixtures-first phasing | code-first | the contract is the product; goldens are the executable spec | @mdarabi |
| 2026-07-06 | machine-readable defects from birth (FR-2) | human text only, JSON later via amendment | CI annotation is a J3 need, known now; no reason to relearn it | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-06 | 000-base drafted for review | @mdarabi |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 gate signed (T2) | pass | gates signed 2026-07-06 (operator review session) |
| Zero solution-blocking TBDs or open questions (§7.2) | pass | Q1/Q2 block design/wire phases, not the solution gate |
| Every Must FR has a fit criterion and a verification method (§5.1) | pass | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | pass | non-applicability rationale + unwire switch |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | pass | |
| Every primary job (§2.3) is covered by a requirement, non-goal, or waiting-room entry | pass | J1→FR-1/2/5/8; J2→FR-6; J3→FR-3/4/5/8 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | pass | F1→FR-1..8 / UB-1..3 / NR-1..2 |
| Every primary metric (§3.2) has instrumentation or an explicit measurement plan | pass | measurement plans stated in §3.2 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | pass | |
| Risks and assumptions have owners (§6.3) | pass | |
| Human-judgment criteria name an owner and a rubric (§5.8) | pass | none present |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | pass | no n/a rows |
