# Harbormaster self-drive — failure taxonomy

| Field | Value |
|---|---|
| **In one line** | Post-hoc study of every recorded way the predecessor factory's automated self-drive loop (poll → gate → dispatch → merge → advance) failed, harvested from harbormaster's incident registry and issue tracker — plus the finding that harbormaster already designed, built, and merged webhook-wake over a Cloudflare Tunnel without ever validating it end-to-end. |
| **Relevant-when** | designing or reviewing pyxis's wake mechanism, Trigger port, reconcile loop, staleness detection, or claim/lease layer; choosing a push transport (webhook / tunnel / Actions runner / poll); writing the `argus-unattended-drive` technical designs (FR-13..FR-17, UB-7, UB-9); triaging a stalled or silent drive; deciding what to alarm on |
| **Status** | study complete 2026-07-29; findings not yet folded into any design |
| **Class** | descriptive research record — **never authoritative**: on any conflict the cited sources govern (harbormaster's own incident entries and issue bodies for internal claims, GitHub's docs for platform claims); on product intent, pyxis's vision and PRDs govern |
| **Date** | 2026-07-29. Internal sources read from `~/agenticlab-ai/harbormaster` at that date and from the live `agenticlab-ai/harbormaster` issue tracker via `gh`. Platform claims re-verified against GitHub docs the same day. |
| **Method** | Four passes over the predecessor repo: (1) full read of the incident registry `docs/observability/incidents.md` — all 18 entries (hm-I2..hm-I19; `I1` appears in that file only as the format example inside a code fence, and its content was reworked into hm-I2/hm-I3, so it is not counted as a distinct entry); (2) `gh issue list --state all` over ~400 issues, then a keyword sweep (`scheduler OR watchdog OR poll OR quota OR heartbeat OR cron OR stale OR silent OR wake`) yielding ~80 titles in scope; (3) full issue bodies for the five most load-bearing (hm#251, hm#921, hm#1017, hm#1176, hm#359 context); (4) full read of `docs/design/webhook-wake.md` (440 lines) and the relevant sections of `docs/design/script-gated-supervisor.md`, plus `git log` to establish what shipped. |
| **Verification honesty** | Internal claims are **primary-source reads** — quoted from harbormaster's own incident entries, issue bodies, and design docs. They are the operator's contemporaneous account of each failure, not an independent reconstruction: where an entry's root-cause analysis is wrong, this record inherits that error. **Not adversarially voted.** Coverage is partial by construction: all 18 incidents were read in full, but only ~80 of ~400 issues were examined and only 5 read in full — the taxonomy is therefore a lower bound on failure classes, not an exhaustive census. One claim is explicitly **unverified**: whether webhook-wake ever processed a live delivery (§4.3). |

**ID convention in this record.** `hm#NNN` = a harbormaster *issue*; `hm-I<n>` = a harbormaster *incident* entry. Neither shares a namespace with pyxis's own `I1..I5` incident registry or pyxis issue numbers — the two systems' IDs collide numerically and must never be cited without the `hm` prefix.

---

## 1. What this studies, and why

Harbormaster is the predecessor factory (bash-cored, ~tens of thousands of lines) that pyxis is the rewrite of. It ran a real automated self-drive loop in production for roughly two months: a poll cycle discovered ready work, a gate decided whether to spend inference, workers implemented, a merge sweep landed green PRs, and the closing of one leaf made the next one ready. That is precisely the loop pyxis's `argus-unattended-drive` PRD contracts (FR-11..FR-17).

So harbormaster's failure record is the closest thing to an empirical prior available for that contract. The question this record answers is narrow and specific:

> **When an automated self-drive loop breaks in practice, what actually breaks?**

The answer turns out to bear directly on a choice pyxis was in the middle of making — which push transport to build — and to substantially reframe it.

---

## 2. The headline: the transport was never the problem

Across 18 incident entries and ~80 in-scope issues, **the number of failures attributable to webhook or event delivery is zero.**

Not "few." Zero. Harbormaster ran its entire production self-drive on a 5-minute poll and never had a delivery-loss incident, because it never depended on delivery.

What did fail, in descending frequency:

| Class | Failures found | Shape |
|---|---|---|
| §3.1 The clock dies silently | 6 | nothing drives the loop; idle is indistinguishable from healthy |
| §3.2 Swallowed evidence | ≥6 (cross-cutting) | the system reports success while doing nothing |
| §3.3 Stale/cached reads | 3 | a probe returns a pre-change body; the loop decides on it |
| §3.4 Duplicated predicates | 4 | two implementations of "is this ready?" diverge → phantom loop |
| §3.5 Claim/lease leaks | 6 | the mutex leaks, deadlocks, or is itself gated |
| §3.6 Non-converging recovery | 3 | self-heal loops without a round bound or escalation |
| §3.7 Green tests over dead features | 5 | the double satisfies the code's assumption, not reality |
| §3.8 Platform scheduler unreliability | 1 (large) | GitHub's own cron showed 4.5 h gaps |

The transport question — Cloudflare Tunnel vs. self-hosted Actions runner vs. faster polling — is a rounding error against this list. Every item above is a property of the *loop*, not of the *wire*.

---

## 3. The taxonomy

### 3.1 The clock dies, silently — the dominant class

**hm#251** is the canonical entry. Harbormaster moved from a self-firing in-session `/loop` (which re-fired itself every 5 minutes from inside the session) to a "gated" mode driven by an external watchdog. The watchdog was a launchd agent in the `gui/<uid>` domain — which **only fires while that user holds the active console session.** On a headless always-on agent account that never owns `/dev/console`, it never ticked at all.

Observed: the supervisor sat idle 20+ minutes, the heartbeat went stale, and a freshly-created `local-agent` issue was never picked up. From the issue:

> *"A single missing/stalled watchdog now silences the entire node (no firing, no maintenance, no heartbeat) indefinitely. With one node, there's no peer to notice. The failure is invisible (idle looks identical to "no work") until someone runs a smoke test."*

And the diagnosis was that the *logic was fine* — only the caller was missing:

> *"So **detection + firing work**; the sole defect is that nothing was calling `ensure`."*

The chosen remedy is worth recording verbatim, because it is the architectural conclusion:

> *"Don't make the console-session-dependent launchd agent 'more reliable' — **remove the dependency.** Co-locate the poll driver with the tmux server it drives: `pool start` launches a lightweight self-sustaining bash driver (`while true; do <script> pool ensure; sleep <interval>; done`) as its own detached tmux session… Keep launchd/cron only as the boot-time resurrect backstop, not as the steady-state cadence."*

Same class, five more times:

- **hm#1017 / hm#923** — "Fleet silent: all heartbeats stale," filed **twice**; hm#1017 is still open. The alarm text: *"No node has force-pushed `refs/heartbeats/*` within the threshold, so no machine is draining `local-agent` work."*
- **hm#419** — they had to build a **watcher for the watcher**: a scheduled Actions workflow that files one deduped operator issue when the fleet goes quiet. The detection had to live *off* the machine that fails.
- **hm#1153** — "supervisor tick discovers 0 repos when the driver cwd isn't the rig clone (**silent dispatch outage**)."
- **hm#257** — "`run_maintenance` runs cleanup / prune-worktrees / auto-close-epics from a non-git cwd — three janitors **silently no-op every tick**."
- **hm-I5** — a worker mutated a clone's main checkout, stranding it on a feature branch; `sync-rig` then *"skipped silently every cycle and the node ran stale supervisor code for hours."* Its lesson: *"make a repeatable safety skip count/alarm/self-heal after N rather than recur silently."*

**Distilled lesson.** An external scheduler is a dependency with its own failure modes, and on a developer machine those modes are environmental and invisible (session ownership, cwd, sleep/wake, a user not logged in). A long-running self-driving process that owns its own cadence removes the class. Whatever remains as a backstop must be *observed*, not trusted.

### 3.2 Swallowed evidence — the most repeated shape in the registry

This is cross-cutting rather than a discrete incident class, and it is the single most common phrase pattern across the corpus.

The clearest instance, from `script-gated-supervisor.md` §2a: the merge-ready sweep listed PRs under the App token with `2>/dev/null || true`. The App installation lacks checks-read for the `statusCheckRollup` GraphQL field, so the listing failed — and the error suppression converted that failure into an **empty list**, which printed the benign `"no open PRs — nothing to do"`. The design's own words:

> *"a permanently broken sweep masked as a no-op (the third **swallow-the-evidence** instance after #350/#354; the stubbed test could not catch it because stubs fake `gh`)."*

The same shape elsewhere:

- **hm#1193** — "scheduler under-dispatch: ready children of a READY epic never claim … **reason swallowed**"
- **hm#1200** — "sync-rig silently leaves a diverged managed clone frozen (**reports rig-clone sha as success**) — no self-heal, no alarm"
- **hm-I5** — *"skipped silently every cycle"*
- **hm#257** — *"silently no-op every tick"*
- **hm#1153** — *"silent dispatch outage"*

**Distilled lesson.** The characteristic failure of an autonomous loop is not a crash. It is **a success report over an action that did not happen.** `|| true`, `2>/dev/null`, a default-empty result, and a skip-without-counting are all the same bug. Any step whose no-op and whose success are textually indistinguishable is an outage waiting to be invisible.

### 3.3 Stale and cached reads — and a fix that didn't sweep its class

**hm-I4 (2026-06-10).** The gate probed for a closing PR's existence via an ETag conditional GET on a *list* endpoint. Weak list-endpoint ETags served a **pre-creation 304 body**, so a just-created PR was invisible for ~13 minutes. The gate concluded the issue was unshipped, re-claimed it, and a redundant worker re-implemented already-shipped work.

> **Lesson (hm-I4):** *"Decision-critical PR-existence probes use fresh exact REST listings keyed by the deterministic head branch, never a search index or cacheable conditional GET."*

**hm-I12 (2026-06-29).** The identical bug, three weeks later, on a different read — because I4's fix had covered only the PR-existence probes. The spec-marker read (backing the spec-ready dispatch gate) still used a default conditional GET on the comments list endpoint. Right after the plan phase posted the marker, spec-ready got a 304 serving the pre-marker body → `no-spec` → a `no-spec-after-plan` loop where *"claims pile up, zero PRs"*, flapping run-to-run.

> **Lesson (hm-I12):** *"When fixing a list-ETag 304 stale-read (I4), sweep EVERY decision-critical list read of that class, not just the one that bit… Tests must not set the freshness bypass globally or they hide the very cache path prod uses."*

**hm#1192.** The inverse failure: `verify-shipped` **fail-closed on transient GitHub API errors**, producing a false gate-fail and an Escape-kill of an already-shipped leaf. Fail-closed is right for destructive paths and wrong for read probes whose failure means "unknown," not "no."

**Distilled lesson.** Conditional requests are a *cost* optimization with a *correctness* hazard. Their free-304 property (confirmed in GitHub's REST best-practices: a 304 on a correctly-authorized conditional request does not count against the primary rate limit) makes them attractive for cheap change-detection, and dangerous the moment a decision turns on the body. The two uses must be separated explicitly: ETag for "did anything change," fresh exact reads keyed on a deterministic identifier for "may I act."

### 3.4 Duplicated predicates → phantom loops → token bonfires

**hm-I2 / hm-I3 (2026-06-08/10).** The bash watchdog gate and the LLM skill each evaluated issue candidacy with their **own** predicate implementation. Whenever the two disagreed, a non-candidate issue re-qualified every poll cycle — a perpetual candidate loop that fired LLM passes forever, measured at **~2M tokens/hour**.

> **Lesson:** *"One candidacy predicate, one implementation; a non-candidate gets an explicit stop-marker so the steady state is a cheap no-op."* Graduated to their principle P1.

**hm-I15 (2026-06-30).** Cross-epic readiness (hold a downstream epic's leaves until its foundational epic closes) was enforced in the skill's pick-loop and a conditional re-check, but **not** in the bash readiness gate that is the source of truth. Multi-epic plan hm#1031 spawned downstream leaves before their foundational epics closed — including a VERIFY leaf that **auto-closed against an unbuilt feature**, voiding the plan umbrella's verified-done.

**hm#1186 — still open — is an explicit hm-I15 recurrence:** *"Verify leaf of a blocked epic was dispatched — epic-ready hold not honored on the scheduler path (I15 recurrence)."* This class came back after being fixed.

**hm#1165.** "ready feed re-lists a done leaf forever when its merged PR lacks a closes-link — **2.5h phantom dispatch loop**." A readiness predicate whose done-ness test depended on a convention (a `Closes #N` link) that the artifact could legitimately lack.

**Distilled lesson.** Every self-drive loop has a readiness predicate. If it exists twice — once in the cheap gate and once in the expensive actor — it will diverge, and divergence in this position does not fail closed. It fails *expensively and forever*.

### 3.5 The claim/lease layer — where the bugs actually lived

Six distinct failures against the cross-node mutex, more than any other single component:

- **hm#1164** — terminal dispatch outcomes (exhaustion, hard-fail) **leak the claim**; the reaper's 24h open-issue protection then turns the leaked claim into a **day-long dead leaf**.
- **hm#1158** — K-way parallel worktree prep races on a shared clone: prep fails *and* the claim leaks, stalling the leaf for the full TTL.
- **hm#1190** — worker self-deadlock: a stale `.LEASE_LOST` plus its own dispatcher's claim **misread as foreign** → completed work was yielded instead of PR'd.
- **hm#1019** — non-numeric `claim/triage-*` refs were never matched by the reaper → a **stale claim held for 2 weeks**.
- **hm#1198** — `claim/*` ref pushes had to be exempted from the docgate pre-push hook: **a lease marker gated by a code toolchain stalled all pyxis dispatch for ~15 hours.**
- **hm#1193** — ready children of a READY epic never claim (claim lost/failed) → the wave stalls with idle slots, reason swallowed.

**Distilled lesson.** The claim mutex is architecturally the thing that makes concurrent and duplicate wakes safe — and empirically the most defect-dense surface in the system. Its failure mode is characteristically a *stall with idle capacity*, which reads as "no work" rather than as an error. Note that hm#1198 is a governance-layer own-goal: the repo's own quality gate blocked the coordination primitive.

### 3.6 Recovery loops that don't converge

**hm-I13 (2026-06-30).** The self-heal for a green-but-conflicting PR re-spawned a worker to merge origin/main and re-finalize. On a high-churn shared single line (a ~5KB usage string in one file), main advanced faster than the recovery could land: merge → re-finalize → main moves → re-conflict → re-spawn, **with no round bound and no escalation.** Two issues each needed manual resolution.

> **Lesson:** *"A re-spawning self-heal must bound its reconcile rounds and escalate… when it cannot converge — never loop silently; and remove the common case structurally."*

**hm-I16 (2026-06-30).** The Actions-side merge-ready reaper surfaced a conflicting PR by labelling it — but the labelling lived **only inside the merge-failure branch**, behind a guard that skipped PRs GitHub already reported as `DIRTY`. So an already-dirty PR hit the skip *before any merge attempt*, was never labelled, still counted as blocking, and its issue never re-spawned: a silent dead-end. The aggravating detail:

> *"the whole fleet was down at the time, so the reaper was the only live recovery surface and it dropped it."*

Its corollary is a testability finding: *"YAML-embedded workflow bash is neither `bash -n`-gated by CI nor unit-tested; extract-and-drive the `run:` block in a test."*

**hm#706.** "Stuck-PR recovery: a red-CI PR whose claim was released is never flagged or re-engaged."

**Distilled lesson.** A recovery surface must handle the **already-computed** failure state directly, not as a side effect of attempting and failing an action. And every self-heal needs a round bound plus an escalation path, or it converts a bounded problem into an unbounded one.

### 3.7 Green tests over features that never worked

**hm-I11 (2026-06-26)** is the one to internalize. The design-review driver readiness-probed `/` and `/api/health`; the real server's route is `/health`. Both probes 404'd, so **every review silently degraded to view-only from the commit that introduced it.** And:

> *"The full-loop test's stub was written to serve the SAME wrong paths the driver assumed, so it went green. The real-cli contract test never pinned the readiness path the stub faked — both tests missed a never-working feature."*
>
> **Lesson:** *"A test double must satisfy the real dependency's contract, not the code-under-test's assumption about it."*

Four more in the same family:

- **hm-I12** — tests set the freshness bypass globally, hiding the exact cache path production used.
- **hm-I18** — *"reality-pin fixtures MUST include the echoed prompt (suites were green precisely because fixtures omitted it)."*
- **hm-I17** — the deterministic safety-classifier suite ran only in the local gate, never in required CI, so classifier PRs **merged red** and a dead control-plane floor shipped undetected.
- **hm#1195** — "`test-scheduler.sh` 15-red on main … AND not in the merge gate — **scheduler-logic regressions merge undetected**."

**Distilled lesson.** This is pyxis's `hermetic-fakes-mask-integration-gaps` lesson in its native habitat, plus hm-I9/hm-I17's local-gate-vs-CI drift (which pyxis has already paid for independently as its own I5). A mock authored from the caller's assumption cannot detect that the assumption is wrong.

### 3.8 GitHub's own scheduler is not a scheduler

From **hm#359** and quoted in `webhook-wake.md` §1:

> *"The Actions-side backstop is worse: GitHub throttles scheduled workflows — `merge-ready-reaper.yml` has shown **4.5 h gaps**."*

Harbormaster built a watchdog-side merge sweep specifically to stop depending on Actions cron for the final merge.

**Distilled lesson.** `on: schedule:` in GitHub Actions is best-effort and observably delayed by hours under load. It is unusable as a cadence for anything load-bearing. Note the important asymmetry: this constrains **`schedule:`** only. Event-driven triggers (`push`, `pull_request_review`, `check_suite`, `workflow_run`) are dispatched on the event and are not subject to the same throttling — so an Actions-based *wake* is sound where an Actions-based *clock* is not.

### 3.9 Two adjacent classes, recorded for completeness

Not self-drive-specific, but they shaped the same loop and are cheap to note:

- **Settle heuristics raced live work (hm-I18, hm-I19).** Turn-end was detected by scraping a tmux pane for an idle streak. A mid-turn lull settled early; the scheduler acted on it immediately — releasing a claim while the turn was still live (hm-I18), and injecting the next brief into a still-live session (hm-I19), producing duplicate charters and one PR that contained another issue's entire deliverable set. Lesson: *"A settle heuristic asserts 'probably ended', never 'ended'"* — and every gate reading the same signal must use the same settle window, or the weakest call site is the bug.
- **Quota as a shared resource (hm#162).** Idle LLM passes and aggressive polling competed with real implementation work for the same GraphQL bucket until it was exhausted. `script-gated-supervisor.md` frames the whole gate design around this: *"an idle iteration burns tens of thousands of tokens to discover 'nothing to do'"* — ~1M tokens/hour across a pool of five.

---

## 4. Prior art: harbormaster already built webhook-wake over a Cloudflare Tunnel

This is the most directly reusable finding, and it was not previously surfaced in pyxis's own documents.

### 4.1 The lineage

| Artifact | What it is |
|---|---|
| **hm#921** (open) | The captured backlog item: *"[Paid feature] Event-driven (webhook) trigger for the supervisor — cut the 5-min poll latency."* Problem statement + a front-end trade study. |
| **`docs/design/webhook-wake.md`** (2026-07-13, 440 lines) | The reviewed design: signed receiver, wake verb, driver slice-sleep, Cloudflare Tunnel ingress, provisioning, failure table, security model, acceptance list. |
| **hm#1166** (closed) | The plan that decomposed it. |
| Seven merged commits | `c2bb068` design · `5c5bc3b` wizard secret provisioning · `1f45042` per-repo webhook upsert · `3990240` acceptance-criteria tests + doc-drift audit · `0ad3468` event kinds · `23dbb50` listener supervision + **pinned `cloudflared`** + doctor check · `d576e18` stdlib signed receiver + driver reactive wake verb |

`scripts/webhook-wake-receiver.py` exists on disk. The capability was designed, planned, implemented, tested, supervised, and merged.

### 4.2 Its central decision is the level-triggered one

The design's §2 is titled **"Decision — wake, never decide"**:

> *"**A webhook delivery carries zero decision authority. Its only effect is to make the existing cycle run sooner.** The woken cycle is the same code the timer beat runs — the same merge-ready qualification, the same read-only recon, the same claim-then-fire mutex — evaluating the same predicates from the same fresh probes. The payload is never an input to any predicate; at most it enriches a log line."*

And it derives that constraint *from the incidents in §3 above*, by name:

> *"**No predicate forks (P1, I2/I3).** The moment the receiver 'knows' what a delivery means… it is a second implementation of candidacy/merge qualification that will drift from the bash one and loop. The receiver stays a doorbell."*
>
> *"**Payload state is a cache, not reality (P4, I4/I12).** …The woken cycle re-probes through the existing exact fresh REST reads, so a wake that arrives 'too early' simply finds not-green and does nothing; the *last* relevant event re-wakes and finds green. Convergence needs no event bookkeeping."*
>
> *"**Delivery semantics need no bookkeeping (P7).** Duplicate deliveries coalesce into one wake. A dropped delivery means the timer catches the work ≤1 interval later… So there is no delivery ledger, no replay defense beyond the rate floor, no missed-event recovery machinery."*

And the posture statement:

> *"**Additional capability, not a replacement.** A node with no tunnel/secret configured behaves byte-for-byte as today. Every failure mode of the new path degrades to the poll cadence. **The capability's correctness weight is zero; it is purely latency.**"*

§7 is a failure table in which every row's floor behavior is *"exactly today's poll cadence — the capability can fail off, never fail wrong."* It also records an honest limit worth carrying forward:

> *"**Honest limit:** we cannot alarm on 'no deliveries for N hours' — a quiet org is legitimately silent, so there is no expected-rate floor to test against."*

§8's security model: outbound-only ingress, HMAC over the raw body before anything else, constant-time compare, a validly-signed request can only append to two files and trigger the same idempotent rate-floored cycle, receiver bound to `127.0.0.1` so the tunnel is the only route in, and tunnel-token compromise buys event metadata plus a latency degrade — *"never control."*

### 4.3 It was never validated end-to-end — and that is the lesson

**hm#1176 — "Validate `docs/design/webhook-wake.md` acceptance criteria end-to-end" — is still OPEN.** Its acceptance criteria are unchecked. So are hm#1169 (receiver), hm#1170 (wake verb + slice-sleep consumption), and hm#1171 (supervise the listener pair in-container).

The scope of hm#1176 contains the tell:

> *"Run the design's §9 acceptance list end-to-end **where a live node + tunnel is available**; where not, run the bash test suites… and verify the event-kind docs."*

That escape hatch is hm-I11's exact shape (§3.7): a feature whose suites are green because they exercise the code's own assumptions, with the live-path proof deferred to an issue that never closed.

**Explicitly unverified in this record:** whether the receiver ever processed a real GitHub delivery. The check that would settle it — grepping the observability event stream for the four `webhook_*` / `wake_*` event kinds hm#1180 added — was not completed. Absence of that evidence is not evidence of absence; it is simply an open question, and the fastest way to close it.

### 4.4 The cost note harbormaster wrote to itself

From hm#921, in its own trade study:

> *"Lowering `SUPERVISOR_DRIVER_INTERVAL_SECS` 300 → 60 (or 30) captures **~80% of the latency win for ~zero cost/risk and no new moving parts.** Webhooks then take you from ~60 s → ~2 s. The design should justify the ingress against this baseline."*

hm#921 also scored `gh webhook forward` against a tunnel and found the tunnel addressed 3 of 4 caveats — while *"it reintroduces a public ingress (the very thing `gh webhook forward` avoids)."* The paid-feature framing was deliberate: *"the production-grade delivery path carries ongoing/external cost."*

---

## 5. Platform constraints re-verified against GitHub's docs (2026-07-29)

Recorded because pyxis's wake design will need to cite them, and because one of them (§5.1) directly corroborates hm#359.

| Claim | Source |
|---|---|
| `on: schedule:` workflows are throttled; harbormaster observed 4.5 h gaps | hm#359 · `script-gated-supervisor.md` §2a |
| A self-hosted-runner job **queues up to 24 h** waiting for a runner, then is automatically cancelled | GitHub Actions limits reference |
| A job may execute for up to **5 days**; a workflow run may span **35 days** including wait time | GitHub Actions limits reference · GitHub changelog 2024-04-04 |
| The runner connects **outbound-only over HTTPS/443** — no inbound port, no tunnel, no public hostname | GitHub self-hosted runners reference |
| The runner uses an HTTPS **long poll held open ~50 s**, reopened on timeout — so dispatch is near-immediate, not interval-bound | actions/runner issue discussion (community-sourced, not official docs) |
| **Documented stuck-queue failures**: jobs remaining `Queued` with an online idle runner, reportedly resolving *only* on manual intervention (re-run, `workflow_dispatch`, or a new commit) | actions/runner#3609 · community #186811 · #120813 — **community-sourced, not an official acknowledgement** |
| `concurrency` **deduplicates pending runs by default**: a new entrant to a group cancels and replaces the existing pending run (`queue: single`); `queue: max` holds up to 100 FIFO instead | GitHub workflow-syntax reference |
| Self-hosted runner minutes are currently **free**; a $0.002/min "Actions cloud platform" charge for **private** repos was announced for 2026-03-01 and then **postponed** ("GitHub has postponed the self-hosted runner billing change to re-evaluate its approach"). Public repos remain free. Minutes round up per job. | GitHub changelog 2025-12-16 (incl. its postponement update) · Actions runner pricing reference |
| Self-hosted runners on **public** repos are strongly warned against; on private repos, anyone able to fork and open a PR can compromise the runner | GitHub secure-use reference |
| For `pull_request`, the workflow runs from the PR's **merge ref** (fork-influenceable); `workflow_run` requires the workflow file on the **default branch** | GitHub events/workflow-syntax references |
| A 304 on a correctly-authorized conditional REST request **does not** count against the primary rate limit | GitHub REST best-practices |

**The §5 asymmetry that matters.** hm#359 condemns Actions `schedule:` as a clock. It does **not** condemn Actions as a wake: event triggers dispatch on the event. A self-hosted runner woken by `pull_request_review` / `push` / `workflow_run` is therefore compatible with hm#359's lesson, provided nothing load-bearing is hung on `schedule:`.

---

## 6. Derived implications for pyxis *(interpretation, not source)*

Everything in this section is this record's inference. It binds nothing; the PRDs and designs govern. Cited pyxis requirements are pointers, not claims of coverage.

1. **UB-9 is the highest-leverage requirement in `argus-unattended-drive`, and it should be built before any transport.** Its text — *"make the silence loud… an unresumed await past its bound is a defect, never a resting state"* — is the direct antidote to §3.1 and §3.2, which together account for roughly half the corpus. Harbormaster needed an **off-machine** observer (hm#419) to catch what on-machine checks structurally could not; pyxis should decide deliberately whether UB-9's detection can live on the same machine as the loop it watches.

2. **FR-17's poll backstop is load-bearing under every transport, including a self-hosted runner.** The runner's 24 h queue is a much longer safety margin than a webhook's fire-and-forget, but §5's stuck-queue reports are hm#251's silent stall with a GitHub-shaped cause. The backstop's *interval* can lengthen; the backstop cannot go away.

3. **Never hang the backstop on `on: schedule:`.** hm#359. The backstop must be local to the machine that owns the loop.

4. **Separate ETag reads from decision reads explicitly in the design.** hm-I4 and hm-I12 are the same bug twice because the first fix didn't sweep the class. Any pyxis reconcile loop that polls cheaply *and* decides on what it read needs both mechanisms named, with the boundary tested.

5. **Budget design attention toward the claim/lease layer, not the wake.** §3.5 is six failures; transport is zero. Pyxis's `refs/claim/<issue>` inherits this surface, and the operator's own `implement-now` notes already record stale-claim races, orphaned holders, and a two-constants-for-one-concept label drift. hm#1198 is a specific warning: do not let pyxis's own gates block the coordination primitive.

6. **`concurrency`-based pending dedup (§5) is a better fit for a level-triggered loop than any relay's at-least-once delivery.** A burst of wakes collapsing to one is exactly right when the woken cycle re-derives the frontier from state. This is a genuine advantage of the Actions-runner transport over a webhook relay, not a tie.

7. **Reuse `webhook-wake.md`, don't re-derive it.** Its §2 decision, §7 failure table, and §8 security model are a reviewed design for this exact problem in the same operator's voice, and its §7 "honest limit" (no expected-delivery-rate floor to alarm on) is a constraint pyxis's UB-9 must work around rather than rediscover.

8. **Close hm#1176's question first, cheaply.** Whether harbormaster's receiver ever handled a live delivery is a single grep against its event stream, and the answer changes how much of that implementation is reusable evidence versus untested code.

## 7. Graduation candidates for pyxis's own registries

Offered for the operator's judgment; graduation is a human act and this record performs none of it.

- **Swallowed evidence as a named principle.** §3.2 is a distinct, highly repeated failure mode that pyxis's existing principles do not name directly: *a step whose no-op is textually indistinguishable from its success is an outage waiting to be invisible.* Pyxis has paid adjacent costs already (its own I3 — a consequential CLI silently reinterpreting a flag, with the halt tail hidden by piping through `head`), which suggests the class is live here too, not merely inherited.
- **Platform schedulers are not clocks.** §3.8 plus §5. Narrow, mechanical, and citable by any future design tempted to put a cadence in `on: schedule:`.
- Both are candidates for `docs/observability/incidents.md` via the coded writer (`docgate add-incident`) if the operator judges them paid-for lessons rather than external research — noting that they were paid for by the predecessor, not by pyxis, which may argue for leaving them here as research and citing them from designs instead.

## 8. What this record does not establish

- **It is not a census.** ~80 of ~400 harbormaster issues were examined; five were read in full. Additional failure classes almost certainly exist in the unexamined remainder.
- **Root causes are inherited, not independently reproduced.** Each incident's diagnosis is the operator's contemporaneous account. None was re-derived from logs for this record.
- **No claim is adversarially verified.** Unlike `ai-software-engineer-research-study.md` §2, nothing here survived a refutation vote.
- **Frequency counts are not rates.** "Six failures in this class" reflects what was recorded and found, not incidence per unit time or per opportunity. Classes that were fixed early may be under-represented; classes in actively-developed code over-represented.
- **It does not establish that harbormaster's webhook-wake works or does not work** (§4.3).
- **It makes no product decision.** The transport choice, the build order, and the graduation calls are the operator's.

---

*Superseded rather than rewritten, per the research-record class. A newer study of the same corpus should be a new dated record citing this one.*
