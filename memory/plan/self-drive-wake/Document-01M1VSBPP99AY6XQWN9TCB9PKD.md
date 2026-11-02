# Self-drive wake — proof run 001

*T11's committed evidence (plan self-drive-wake/001; prd:self-drive-wake DOD-4/DOD-5, NFR-1..3). A proof run reports the truth it finds — it does not rubber-stamp. This one PASSED the live capability on the GitHub-backend verb path and its measured guardrails, and FOUND two real gaps that unit-green had hidden. Both are filed; their disposition is §5.*

| Field | Value |
|---|---|
| **Run** | 001 · dogfood-on-self (the wake capability run against its own build) |
| **Date** | 2026-07-30 |
| **Subject** | the shipped `pyxis status` + `pyxis watch` verbs (main @ `acc6916`), against the frozen `self-drive-wake/001` plan |
| **Method** | live verb runs on the GitHub backend + the three metric families distilled from this build's four real `implement-now` sweep journals |
| **Verdict** | **PASS with two named findings** (§4) — the live path works; the findings are the run's product, not its failure |

## 1. What ran, live

### 1.1 `pyxis status` — the real board (GitHub backend)

```
$ pyxis status --repo . --repo-slug agenticlab-ai/pyxis --plan docs/plans/self-drive-wake
TASK  STATE            WAIT    AGE  FLAG
T1    landed ✓         -       -
T2    landed ✓         -       -
…
T10   landed ✓         -       -
T11   ready-unclaimed  worker  -

checker: ⚠ cannot-verify-liveness: no check events recorded
EXIT: 0
```

Renders every task's real state (ten landed, one ready), the wait kind, and — loudly — the liveness footer. The ⚠ is **DINV-8 proving itself on its first run**: no watch had produced check events yet, so liveness is *unverifiable*, and the verb says so rather than printing a healthy-looking footer. The single most-repeated predecessor failure (a silent dead checker) is, by construction, not silent here.

### 1.2 `pyxis watch` — the loop (GitHub backend)

```
$ pyxis watch --repo . --repo-slug agenticlab-ai/pyxis --plan docs/plans/self-drive-wake …
watch: self-drive-wake/001 — 11 tasks · backend github · cadence 10s
^C
watch: stopped — 1 sweep(s), 1 await(s) outstanding; state is durable, re-run to continue
```

The loop derived the awaited set from durable state, triggered its sweep, and on interrupt exited cleanly with the durable-state message (UB-3: nothing lost, re-run continues). It emitted a typed `wake_delivered` event to the local, gitignored sink. (The sweep was run as a no-op here to isolate **watch's** contract — derive · cadence · wake · clean-exit — from **implement-now's** sweep, which this build already proved four times live.)

## 2. The three numbers — measured from four real sweeps

This feature was built across four `implement-now` sweeps that this session's operator invoked by hand — which is exactly the manual loop the capability exists to end. Those journals are the honest baseline.

| Metric | Target | Measured | Verdict |
|---|---|---|---|
| **M1 — operator-initiated sweeps** (the toil `pyxis watch` removes) | 0 with watch driving | **4 manual sweeps** (+ 2 hand-built verb remainders) to build the feature | **baseline established**: the 4 sweeps are precisely what an armed watch drives to 0 (each merge → next sweep, unattended) |
| **M2 — merge→notice latency** | p95 ≤ 15 s GitHub · ≤ 1 s Carina | **not captured by this focused run** (the no-op-sweep loop had no open proposals to time against) | **cadence bound proven** (10 s / 1 s advice, live); end-to-end timing deferred to a live drive with open proposals (§5) |
| **M3 — rate-limit trips (429s)** | 0 | **0** genuine across all four sweeps (18 dispatches, incl. concurrent fleet races) | **guardrail met** |

Throughput across the four sweeps: 18 tasks dispatched, all 11 wake tasks landed on `main` — several via the external fleet racing the same `local-agent` frontier, with the claim layer arbitrating and zero duplicate merges (INV-1..6 / AP-22, live, two execution domains).

## 3. Quiet-state diagnosis (DOD-5)

Of the five seeded scenarios, the dogfood exercised three against the real board and the verb's own render:

| Scenario | `pyxis status` output | ✓ |
|---|---|---|
| work landed | `T1..T10  landed ✓` | ✓ |
| ready, waiting on a worker | `T11  ready-unclaimed  worker` | ✓ |
| checker unverifiable | `⚠ cannot-verify-liveness: no check events recorded` | ✓ |
| waiting-on-review / offline (needs an open proposal in flight) | see Finding A | deferred |

The three that a closed-board static run can show are correct and legible. The remaining two need an in-flight proposal (Finding A).

## 4. Findings — the run's product

### FINDING A — liveness verification requires an open proposal to *read*, not just a live loop

`wake_check_ran` (the event `pyxis status`'s liveness footer reads) fires only when the loop invokes the reader — and the loop reads only when a task carries an **open proposal**. A frontier whose sole outstanding await is a *ready-unclaimed* task (this build's exact tail state) produces **zero** check events, so liveness reads `cannot-verify-liveness` even though the loop is alive and ticking. Liveness currently conflates *"the loop is alive"* with *"a proposal was read recently."* They coincide in the common case (you are watching open PRs); they diverge in the closed-board tail. **Filed: #894** — a check-ran (or distinct heartbeat) event should fire once per derivation tick so liveness reflects loop-aliveness directly.

### FINDING B — the `watch`/`status` verbs wire only the GitHub reader; Carina is adapter-proven, not verb-proven

The state-reader has both adapters, each with a green conformance kit (T2 Carina, T3 GitHub). But the shipped verbs construct only `ghcodehost.NewStateReader` — there is no `--tracker carina` path. So *"both backends"* holds at the reader/adapter layer (unit-proven) but **not through the shipped verbs** in v1. **Filed: #895** — wire the Carina reader + cadence into both verbs behind a tracker selector.

Both findings are the kind unit-green hides and a proof run exists to catch — the I11 / hermetic-fake class, this time catching real things.

## 5. Disposition (operator's call)

- **PASS, live GitHub path**: `pyxis status` and `pyxis watch` work against the real plan; M1 baseline and the M3 guardrail are measured and met.
- **Two named follow-ups**: #894 (liveness-per-tick, finding A) and #895 (Carina-verb-wiring, finding B). Together they are the honest remainder of T11's *"both backends + full diagnosis"* acceptance.
- **Recommendation**: close T11's live-proof arm on this evidence; carry A and B as the follow-ups. If *"both backends through the verbs"* is required before T11 closes, B is a small wiring task (the adapter exists) — hold #844 until it lands.

*This report is the retained teaching artifact (AP-28); the raw wake-events and sweep journals are the routine ticks it distills, not committed.*
