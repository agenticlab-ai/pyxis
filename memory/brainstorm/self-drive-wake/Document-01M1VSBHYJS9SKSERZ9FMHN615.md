# Self-drive wake — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The self-drive clock — the factory notices a task PR's merge/comment/CI transition and keeps the feature moving by itself instead of the operator re-invoking implement-now; settles whether that is new contract ground or the already-contracted wake (prd:argus-unattended-drive FR-14/FR-17) plus design-owned mechanism, and carries the session's measured transport/quota facts as design-stage provenance. |
| **Relevant-when** | brainstorming or contracting wake/trigger/self-drive mechanisms, push-vs-poll transports, proposal-state reads (merged/commented/checks), GitHub quota budgeting, or adapter cadence/throttle; checking prior rulings on where the self-drive clock lives; entering the wake mechanism's tech-design (the park at brainstorm:argus-unattended-drive#P-2) |
| **Idea** | "the main problem i am trying to solve is to enable self-drive feature in which rather than i run implement-now after merging each task, the system keep going by itself (it checks the status of a pr for a feature and if it approved moves on to the next thing); there are two approaches to this problem: Pull (scheduler + api calls) vs Push (webhook). … are there any other solution? has anyone already solved this problem so we can just reuse ?" *(operator, 2026-07-29, chat, verbatim; preceded by a webhook/Cloudflare-tunnel exploration the session walked back)* |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-29 |
| **Status** | gated: ready-for-prd *(after gating, the record changes only through the introducing PR's review loop — decisions added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — |
| **Feeds** | `docs/prds/self-drive-wake/` |
| **Prior records** | `brainstorm:argus-unattended-drive` |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` | governing scope — §11 road-ahead names "Push triggers — webhook invocation alongside the polling backstop"; §6 trust ladder bounds unattended authority |
| `docs/domain-model.md` | the Trigger port rows (§3.2 + ports table): pull/push/manual indistinguishable to the engine (AP-2); poll remains the backstop; "webhook was backlog #921" |
| `docs/prds/README.md` | index — parity-gap row: "Webhook push trigger — designed-first backlog (harbormaster #921); poll-only today"; no covering ledger |
| `docs/prds/argus-unattended-drive/effective.md` | the wake contract: FR-13 (await as ledger state), FR-14 (wake + resume), FR-15 (instruction-free signal), FR-16 (parked = zero cost), FR-17 (bounded backstop), UB-7/UB-9; WR-2 parks beyond-this-repo |
| `docs/brainstorms/argus-unattended-drive.md` | prior rulings that must not be re-asked: D-5 (contract the behavior, not the mechanism; mechanism → stage-3/4), P-2 (the mechanism park), O-5/O-6 |
| `docs/prds/feature-implementation-drive/effective.md` *(draft, working tree)* | the consuming lifecycle: verified landing advances the DAG; "review waits checkpoint and exit rather than keeping a process alive"; P2 segment = "autonomous driver operating the same feature lifecycle unattended"; NG-1/NG-7 keep merges human |
| `docs/prds/implement-now/effective.md` | the executor stays one sweep: FR-12 (idle is free), FR-14 (idempotent re-invocation — the wake's substrate), FR-18 (single predicate); "driver owns landing and re-invocation" |
| `docs/prds/installer/effective.md` | the offline floor (carina + local hooks, zero network after fetch) — the medium-independence bar any wake must clear |
| `docs/wiki/glossary.md` | standing terms: **Wake signal** (slug + reason, never logic), **Feature integration drive**, **Maintenance sweep** ("the wake-tick pattern"), AUTO/HUMAN lane |
| `docs/research/harbormaster-self-drive-failure-taxonomy.md` | the predecessor evidence base (written 2026-07-29, this session): failure classes + the prior webhook-wake build |
| `internal/executor/port/{codehost,worktracker}.go` | code read — the port-shape gap (F-3) |

**Overlaps surfaced:**

- **The wake behavior is already contracted** — prd:argus-unattended-drive FR-13..FR-17 + UB-7/UB-9 bind exactly "an awaited completion re-enters the drive within a latency bound, poll backstop armed, silence is a defect." The mechanism was **deliberately not contracted**: brainstorm:argus-unattended-drive#D-5 ruled "mechanism selection (webhook/CI-completion trigger vs bounded poll vs harness scheduler, per CLI) is a stage-3/4 design decision," parked as brainstorm:argus-unattended-drive#P-2.
- **The consuming lifecycle is already contracted (draft)** — prd:feature-implementation-drive owns "verified landing unlocks dependents" and names the unattended driver as its P2 segment, but checkpoints-and-exits at review waits: it deliberately does not own the clock.
- **The every-user / engine-era scope is parked ground** — prd:argus-unattended-drive WR-2 ("deliberately repo-scoped"), brainstorm:argus-unattended-drive#O-5 (native stage-8 engine stays engine-era), prd:argus WR-1. A "self-drive for all pyxis users" reading reopens those parks.
- **The requirements index already names the gap** — docs/prds/README.md: "Webhook push trigger — designed-first backlog (harbormaster #921); poll-only today."

## Framings

- **A — The Trigger port: wake-as-capability** — the feature is the *engine capability*: a medium-independent trigger surface (Trigger port + adapters: Carina fs-watch · GitHub poll/runner/webhook-URL · manual) plus the state reads that make a wake actionable (the plural proposal-state read, P-1/P-2) and the budget guards (P-3/P-4). Infrastructure only — the user-facing loops (FID's driver, argus's wake, maintenance sweep's tick) consume it. Implications: smallest park friction; the domain model's Trigger "Generalize" row gets its contract; brainstorm:argus-unattended-drive#P-2's design lands on it instead of inventing transport per-CLI.
- **B — Self-drive: the standing driver** — the feature is the *user-facing resident loop*: a per-machine supervisor that owns registered awaits, polls/wakes, re-invokes sweeps, and alarms on staleness (the UB-9 shape) — for any pyxis user, argus or not. Implications: delivers the literal "I never re-invoke implement-now" promise; but carries process-lifecycle scope (launchd/systemd, supervision, multi-machine coexistence on the lease layer) and reopens the engine-era parks hardest (vision §4's supervisor loop starts existing).
- **C — The awaitable factory: state-read parity** — the feature is the *observation substrate* only: the factory can write (PRs, comments) but cannot *see* (merged? commented? checks?) through its ports; contract the read side + await registry semantics + staleness surfacing, consumed by any driver including a human running a status verb. Implications: smallest and unblocks FID/argus designs immediately; by itself does not deliver "keeps going".

**Destination:** Hybrid **A+C — "eyes + alarm"**: one feature contracting the status reads (which PRs merged / got comments / failed CI — batched, budget-aware, per medium) **and** the wake machinery (a normalized wake channel with a timer fallback). The always-on background driver (framing B) is deferred to its own later feature that plugs into this one. Operator adopted the recommendation (D-2): both already-planned consumers — the feature drive and argus's parked wake design — need exactly these two pieces; the riskiest build (a resident background program, the predecessor's worst failure surface) waits until the foundations are proven; and the standing parks stay respected — WR-2/O-5 remain parked because the loop they cover is exactly what's deferred.

## Decisions

### D-1 — Coverage: new stage-1 ground, not covered

- **Question**: Is the idea covered by prd:argus-unattended-drive (wake behavior contracted; mechanism parked at brainstorm:argus-unattended-drive#P-2) + prd:feature-implementation-drive (the consuming lifecycle), leaving only design work — or is there new stage-1 ground?
- **Answer**: New ground — continue the grill toward a feature of its own; `covered`-and-stop rejected.
- **Reasoning**: Operator's selection against the `covered` recommendation, accepting the option's stated condition: the standing parks (WR-2 "deliberately repo-scoped"; #O-5 engine-era) lift early. The *why* is carried to the Destination rationale — the parks' own revisit triggers are arguably firing (installer shipped; the native executor exists; FID is drafted).
- **Anchor**: 2026-07-29 · coverage question (AskUserQuestion #1)
- **Status**: live

### D-2 — Destination: "eyes + alarm" as one capability feature; the background driver deferred

- **Question**: How much is this feature — the status reads only ("eyes"), the wake machinery only ("alarm"), both as one feature, or the whole thing including the always-on background driver?
- **Answer**: Eyes + alarm as one feature (framing hybrid A+C). The always-on background driver is a separate, later feature that plugs into this one.
- **Reasoning**: Adopted the recommendation — the feature drive (prd:feature-implementation-drive P2 segment) and argus's parked wake design (brainstorm:argus-unattended-drive#P-2) both need exactly these two pieces; the resident background program is the predecessor's most failure-prone surface (F-6) and waits until foundations are proven. This also resolves D-1's carried condition: the parks (WR-2/#O-5) stay parked — the deferred driver is what they cover; this feature is the un-owned brick beneath them.
- **Anchor**: 2026-07-29 · scope question (AskUserQuestion #2, re-asked in plain language)
- **Status**: live

### D-3 — Reach: any pyxis-installed repo, both backends in v1

- **Question**: Who can use "eyes + alarm" in v1 — any pyxis repo on both backends (GitHub + Carina), GitHub-only, or this repo only?
- **Answer**: Any pyxis repo, both backends from day one: GitHub (batched API reads, ~15 s notice) and Carina (local file reads, ~1 s notice, zero network).
- **Reasoning**: Adopted the recommendation — the installer's offline floor already promises "works with zero network" (prd:installer), so a GitHub-only wake would break that promise for this feature; and building against the free local backend first keeps the port shape honest (F-3's gap came from designing around one medium). Also consistent with D-1's "every pyxis user" ground.
- **Anchor**: 2026-07-29 · reach question (AskUserQuestion #3)
- **Status**: live

### D-4 — The human window: a status verb with wait ages in v1

- **Question**: Does v1 include a human-facing status command, and does it show how long each wait has been sitting (with a "too long" flag)?
- **Answer**: Yes — one command shows what the system sees per task/proposal (merged / waiting on review / CI state / blocked) **plus** each wait's age and a too-long flag, and the checker's own liveness ("last check: 6 s ago").
- **Reasoning**: Adopted the recommendation — nearly free once the reads exist; the operator's escape hatch when automation misbehaves; and it makes silent-stuck visible at a glance ("quiet" vs "dead" being indistinguishable was the predecessor's most repeated failure, F-6). Waiting-on-you vs stuck is the trust-building distinction.
- **Anchor**: 2026-07-29 · status-view question (AskUserQuestion #4)
- **Status**: live

### D-5 — The alarm: timer-only in v1; push channels are reserved plug-in slots

- **Question**: Is v1's alarm timer-only (GitHub ~15 s while awaits exist, Carina file-watch/instant, zero when idle), does a push channel (GitHub Actions listener) ship in v1 too, or is v1 push-first with a slow backup timer?
- **Answer**: Timer-only in v1. The contract reserves an explicit plug-in slot so push channels (e.g. the Actions-runner listener) are added later as adapters, with no contract change. When nothing is awaited, no checking runs — idle is zero.
- **Reasoning**: Adopted the recommendation — measured cost is ~5% of the free GitHub budget at 15 s (F-4) and ~10 s of extra latency buys nothing before a ~30-minute agent step; every analyzed push channel adds a silently-failable moving part (the predecessor's #1 failure class, F-6/F-7) plus per-machine install/registration. Push's real value arrives with the deferred background driver (D-2).
- **Anchor**: 2026-07-29 · alarm-speed question (AskUserQuestion #5)
- **Status**: live

### D-6 — Success bar: zero re-runs + notice-speed bound + zero rate-limit trips, proven on a real run

- **Question**: What must be measurably true, on a real 6-task feature driven end-to-end on both backends, to call this shipped and proven?
- **Answer**: Three numbers from the run's own logs: (1) **zero** manual re-invocations while the drive process is up — the operator's only acts are reviews and merges; (2) merge→notice within the promised bound (~15 s GitHub p95, ~1 s Carina p95); (3) **zero** GitHub rate-limit trips (429 count = 0) across the run, bursts included.
- **Reasoning**: Adopted the recommendation — mirrors prd:argus-unattended-drive M1's "non-owned touches = 0" shape; every number reads off logs cheaply; wall-clock alternatives measure the operator's review calendar, not the feature (rejected); demo-only sign-off is how the predecessor shipped a never-validated notification path (F-6, rejected).
- **Anchor**: 2026-07-29 · success-bar question (AskUserQuestion #6)
- **Status**: live

### D-7 — The refusal set: six recorded no's

- **Question**: Adopt the six-refusal list — (1) no background program, (2) no push in v1, (3) never a hosted central relay, (4) never self-merge, (5) no multi-machine coordination, (6) no cadence knob in v1 — or variants keeping the relay door open / adding a speed knob?
- **Answer**: All six adopted as put. Anti-segment named by (3)+(5): fleets/orgs wanting a centrally-hosted watcher or multi-machine drives are not this feature's users.
- **Reasoning**: (1)(2) formalize D-2/D-5; (3) closes the door this session opened and walked back — a hosted relay is a required middleman seeing every user's repo activity (F-8, "no privileged substrate"); (4) keeps merge authority where other contracts put it; (5) keeps v1 one-machine-per-feature; (6) follows the installer's fewest-knobs instinct — knobs are easy to add, near-impossible to remove.
- **Anchor**: 2026-07-29 · refusals question (AskUserQuestion #7)
- **Status**: live

### D-8 — Appetite: everything or nothing — no pre-agreed cut-line

- **Question**: Which budget does the contract carry — bounded with a pre-agreed cut-line (comment reads drop first), everything-or-nothing (the finish line moves, nothing drops), or a GitHub-first sequenced landing?
- **Answer**: Everything or nothing: v1 ships when the whole list works — both backends' reads (merged/CI **and** comments), the timer wake with reserved push slots, the status verb with wait ages, the rate-cap safety, and the 6-task proof run with all three D-6 numbers. No piece drops; if it takes longer, it takes longer.
- **Reasoning**: Operator's selection against the cut-line recommendation — a complete, verified v1 over a partial fast one; consistent with the operator's standing direction that authoring effort is not the scarce resource. The sequenced-GitHub-first variant was rejected with D-3's own logic (the port shape must be settled with both backends in view).
- **Anchor**: 2026-07-29 · appetite question (AskUserQuestion #8)
- **Status**: live

## Facts

- **F-1**: The wake's *behavior* is contracted and its *mechanism* is explicitly design-owned — FR-13..FR-17/UB-7/UB-9 bind outcome + cost envelope; D-5/P-2 park transport selection to stages 3–4. — source: `docs/prds/argus-unattended-drive/effective.md`; `docs/brainstorms/argus-unattended-drive.md#D-5/#P-2`
- **F-2**: The feature lifecycle checkpoints-and-exits at review waits and its "driver owns landing and re-invocation" — the drive, not the executor, owns the clock; implement-now FR-14 makes re-invocation idempotent (no duplicate proposals), which is the property a wake-driven driver rides. — source: `docs/prds/feature-implementation-drive/effective.md` (§4.3 Loading; §2.4 evidence); `docs/prds/implement-now/effective.md` FR-14
- **F-3**: The executor ports cannot yet express the needed reads: `CodeHost` has only `OpenProposal` + `Checks(one proposal)` (an N+1 on a metered medium), `WorkTracker.Comment` is write-only — no merged?/commented? read exists on any port. — source: `internal/executor/port/codehost.go`, `worktracker.go` (read 2026-07-29)
- **F-4**: Measured live (2026-07-29, personal `gh` auth, this repo): one GraphQL query covering 50 PRs × (merged state + last-5 comments + check rollup) costs **1 point** of the separate 5,000/hr GraphQL budget (`nodeCount` 350) — cost is set by the query's `first:`/`last:` shape, not by how many PRs exist. REST equivalent: 3+N requests per PR per sweep. — source: live `gh api graphql` run; docs.github.com GraphQL rate-limits page
- **F-5**: The binding write limits are secondary: **80 content-generating requests/min and 500/hr** (plus 900 REST points/min; mutations = 5 points). A review posted as `comments[]` + `event` in one `POST /pulls/{n}/reviews` call collapses ~7 writes/task to ~3, moving the burst ceiling from ~13 to ~40 concurrent workers. — source: docs.github.com REST rate-limits + pulls/reviews pages (fetched 2026-07-29)
- **F-6**: Predecessor evidence: **0 of 18** harbormaster incidents were transport-caused; the dominant classes are the silently-dead clock and swallowed evidence; the claim/lease layer was the most defect-dense surface (6 failures). Harbormaster already designed and merged webhook-wake over a Cloudflare Tunnel — its e2e validation issue (hm#1176) never closed. — source: `docs/research/harbormaster-self-drive-failure-taxonomy.md`
- **F-7**: GitHub Actions `on: schedule:` is throttled (observed 4.5 h gaps) and unusable as a load-bearing cadence; *event* triggers dispatch immediately, and a self-hosted runner is outbound-only with ~24 h server-side job queuing — push latency with queued delivery, no tunnel. — source: research record §3.8/§5
- **F-8**: A GitHub webhook relay cannot be centrally hosted for third-party users without becoming a privileged substrate (vision: "no privileged substrate") and a plaintext-custody point; Cloudflare tunnel DNS routing is account-scoped (`cfargotunnel.com` proxies only same-account records). — source: session verification against developers.cloudflare.com (2026-07-29); `docs/product-vision.md` §2

## Fog

- **G-1** (non-blocking — blocks: stage-3 design detail): what "too long" means for the status verb's ⚠ flag — a fixed default per wait kind (review / CI / claim), a config value, or learned from the repo's own history. Lands as a PRD open question.
- **G-2** (non-blocking — blocks: the argus wake design, brainstorm:argus-unattended-drive#P-2): how argus's contracted wake (prd:argus-unattended-drive FR-14/FR-17) formally binds to this capability — its design should consume this feature's contract rather than invent transport; the sequencing between the two designs. Lands as a PRD open question.

## Out of scope

- **O-1**: No always-on background program in this feature — it is the deferred later feature that plugs into this one. (from D-2, D-7)
- **O-2**: No push channels in v1 — the contract reserves explicit plug-in slots; revisit when the background driver ships or measured latency hurts. WR candidate. (from D-5, D-7)
- **O-3**: Never host a central notify/webhook relay for pyxis users — a required middleman with custody of every user's repo activity ("no privileged substrate", F-8); users wanting push later bring their own channel. (from D-7)
- **O-4**: Never merge or approve anything — merges stay human; the AUTO lane belongs to prd:argus-unattended-drive, not here. (from D-7)
- **O-5**: No multi-machine coordination — one drive process per feature; cross-machine sharing is the deferred driver's ground. (from D-7)
- **O-6**: No check-speed knob in v1 — adapter-advised defaults (≈15 s GitHub, instant/1 s Carina); a knob only on demonstrated need. (from D-7)

## Candidate glossary terms

- **T-1**: **"self-drive"** — session shorthand for the composed outcome (a drive + this feature's wake and reads), not a contract term; contracts use the existing wake/drive vocabulary (glossary: *Wake signal*, *Feature integration drive*) · avoid: coining "self-drive" in requirement text

## Parked technical constraints

- **P-1**: **Plural state reads at the port** — the proposal-state read must be batch-shaped (`StatesOf([]Proposal)` ≈ one round trip per batch, asserted in the port's conformance kit): per-item reads are free on Carina and an unfixable N+1 on GitHub; port granularity must be set by the metered medium. (stages 3–4; F-3/F-4)
- **P-2**: **Per-context check reads, never an aggregate rollup** — `codehost.go` already binds "an exact, fresh read, never a status rollup" (AP-13/AP-15); the batched GraphQL read must decompose `statusCheckRollup` to `contexts` and compare against the required set client-side. (stages 3–4)
- **P-3**: **Adapter-owned write throttle** — a ≤80/min · ≤500/hr token bucket inside the GitHub adapter (state-dir backed so N processes share one budget), surfaced to the engine only as backpressure; Carina unthrottled. Batch the review write (F-5). (stages 3–4)
- **P-4**: **Adapter-advised cadence** — poll interval is an adapter hint (GitHub ~15 s quota-bounded, generalizing `X-Poll-Interval`; Carina ~1 s or fsnotify), never an engine constant. (stages 3–4)
- **P-5**: **A 403/429 on a verification read resolves to *unknown — retry*, never "no"** — the hm#1192 class: a rate-limit response misread as absent state drives destructive recovery; fail-closed is for destructive acts, not read probes. (stages 3–4; F-6)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-29 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 problem: D-1/D-2 · §2.2 segments: D-3/D-7 · §2.3 jobs: D-2/D-4/D-6 · §3.1 goals: D-6 · §3.4 non-goals: O-1..O-6 · appetite: D-8), zero blocking fog (G-1/G-2 both non-blocking) |
