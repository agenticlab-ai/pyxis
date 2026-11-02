# Review: acquisition-trust-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — a faithful, invariant-anchored reconciliation of shipped-and-proven behavior; three blocking gaps (AP-17 omitted, the fetch-credential unreconciled with the PRD's declared-operation scope, the cross-host token-drop unexercised) applied in round 1, with operational-completeness advisories recorded and two carried to a code follow-up. |
| **Relevant-when** | auditing or consuming the review of `acquisition-trust-v1`; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:acquisition-trust-v1 · shape: none · supersedes: — |
| **Moment** | new-design (a scoped `Builds-on` revision of installer-v1#DINV-1; the supersession scope is operator-ruled, subject §7 Q-1) |
| **Reviewer** | tech-design-review · 2026-07-21 |
| **Date(s)** | 2026-07-21 |
| **Verdict** | approve-with-advisories |
| **Tracking** | PR #585 |
| **Template version** | design-review-report/v1 |

## 0. How to use this template

*Round-1 findings and their dispositions below; the verdict history is authoritative. Cite findings as `review:acquisition-trust-v1#FND-2`.*

## Round 1

### FND-1 — AP-17 (fail-closed) silently omitted from Principles, on the surface it most governs

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design Principles § cited only AP-14/19/13/6/9 and installer-v1#DINV-1/5/8/2 — vs — architecture-principles.md AP-17 (*"Fail closed on destructive … a failed probe reads as 'alive': destructive actions require positive evidence"*), which installer-v1 cited for exactly this acquisition floor. The registry mandates citing every principle honored or deviated, by ID; the design's whole value (checksum-mismatch / no-pin / private-host-404 / missing-token refusals) is textbook AP-17.
- **Acceptance condition**: cite AP-17 in Principles, reasoned as honored on the refusal surface.
- **Disposition**: applied — AP-17 added to Principles (honored, central), reasoned against §3.3/§3.6/§3.7/DINV-3.

### FND-2 — the fetch credential is a new use unreconciled with the PRD's declared-operation scope (ABUSE-3)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design §3.2 (*"token source … `gh auth token` … suffices"*) — vs — prd:installer §5.3 (*"GitHub operations use their ambient credential for the declared operation set only"*), ABUSE-3 (credential over-reach *"beyond the declared wiring set"*), and §5.4's egress enumeration (which lists wiring writes, not an authenticated fetch). The design attached the ambient credential to a new *fetch* use without reconciling it with the PRD's credential-scope stance; §3.7 covered mirror-exfil but not this.
- **Acceptance condition**: reconcile the new credential use in §3.7 — read-only, least-privilege (the user's own read access), host-scoped, adding exactly one entry to the declared egress set.
- **Disposition**: applied — §3.7 gains a "credential over-reach (ABUSE-3 / §5.4)" bullet reasoning the fetch is read-only, host-scoped (DINV-2), never touching the wiring write scope; the egress set gains one read-only asset GET.

### FND-3 — the cross-host `Authorization`-drop mitigation is claimed but unexercised, with no acceptance row

- **Severity**: blocking (CAL-4 — load-bearing for the DINV-2 containment claim)
- **Lens**: feasibility-completeness
- **Evidence**: design §3.7 (*"Go additionally drops the `Authorization` header across the cross-host redirect"*) and the §3.2 diagram (*"Authorization dropped cross-host"*) — vs — the code's `TestGitHubAuthFetcher` (`fetch_internal_test.go`), whose httptest returns the asset bytes at 200 directly and never issues a distinct-host 302; A-2 proves only the *attach* side (the mirror gets no auth leg), not the *detach* on the authenticated redirect the token-exfil mitigation relies on.
- **Acceptance condition**: name the two-host redirect-drop seam in §3.8 and add an acceptance row; the code test is a follow-up.
- **Disposition**: applied (design) — §3.8 names the two-host redirect seam; A-6 added (the second-hop request carries no `Authorization`). Code test tracked as a follow-up.

### FND-4 — DINV-1's "not re-checked per execution" over-states what §3.1 actually keeps

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality (coherence-surfaced)
- **Evidence**: design DINV-1 (*"it is not re-checked per execution"*) — vs — §3.1 (*"The manifest anchor-check stays on every resolve"*) and the code (`manifest()` re-hashes the cached manifest against the pin every call). The crisp-invariant layer read as "nothing is re-hashed per invocation," which is true only for the tool binary.
- **Acceptance condition**: scope DINV-1's clause to the tool-binary cache; name the retained manifest anchor-check.
- **Disposition**: applied — DINV-1 now scopes the "not re-checked" clause to the tool-binary cache and names the retained 27 KB manifest anchor-check.

### FND-5 — the 100-asset tag-list ceiling is a bounded assumption left unrecorded

- **Severity**: advisory (CAL-4 — breaks no current acceptance; 56 assets today)
- **Lens**: feasibility-completeness
- **Evidence**: design absent — vs — `assetID` reads `releases/tags/<tag>` once and scans the inlined `assets[]` with no pagination; GitHub caps that inline set at 100. Past the cap an asset surfaces as a `no asset "<name>"` error that reads like a bad pin.
- **Acceptance condition**: record the ceiling as a bounded assumption / future-owned item.
- **Disposition**: applied — §5 and §3.4 record the ceiling, the misleading error surface, and paginated lookup as future-owned.

### FND-6 — the new per-init API call cost (redundant re-listing, rate-limit exposure) is unstated

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design §1.1 trades away the per-exec re-hash on cost grounds — vs — the auth path re-lists the whole release per cache-miss binary (no memoization), so a cold init is O(binaries) list+asset pairs, with rate-limit (403/429) an unnamed failure mode.
- **Acceptance condition**: record the per-init call cost + rate-limit failure mode; note map-reuse as a deferred optimization.
- **Disposition**: applied — §5 records the call volume, the redundant re-listing (map-reuse deferred), and the fail-closed rate-limit surface.

### FND-7 — the expired/under-scoped-token failure mode is distinct from no-token and unstated

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design §5 covered *absent/failed* `gh`; §3.7 called the token "short-lived" as a privacy virtue — vs — a stale or under-scoped token authenticates yet 401/403s, a case the remedy narrative (§3.6, framed for no-token) did not name.
- **Acceptance condition**: name the 401/403 failure mode with the `gh auth refresh` remedy.
- **Disposition**: applied — §3.7 adds the expired/under-scoped-token bullet (fail-closed, named 401/403, refresh remedy).

### FND-8 — the reconciliation relaxes installer-v1's "on-any-path" guardrail without naming the `#ALT-8` window it bounds

- **Severity**: advisory (CAL-3)
- **Lens**: upstream-alignment (reconciliation honesty)
- **Evidence**: design §3.1 argued the residual threat is bounded — vs — installer-v1 rejected `#ALT-8` precisely to avoid "a tamper window … until doctor" on the "on any path" guardrail; the design relaxed that guardrail without naming that it now accepts the bounded form of that exact window.
- **Acceptance condition**: name the `#ALT-8` window and how this bounds it.
- **Disposition**: applied — §3.1 names the `#ALT-8` window and states the bounding (same-version, attacker-owned path) as a deliberate, non-silent reduction.

### FND-9 — installer-v1 A-9 left describing a retired mechanism; the AP-9 claim over-reaches

- **Severity**: advisory (CAL-3)
- **Lens**: upstream-alignment
- **Evidence**: design Principles claimed plain "AP-9 … honored" — vs — installer-v1 hedged AP-9 ("honored (not precluded)"; channels/cohorts deferred), and its frozen A-9 cost model ("config parse + one sha256 + execve") now names a per-exec sha256 this design retires (the ≤100 ms bound still holds).
- **Acceptance condition**: hedge AP-9 to the observability leg; note A-9's bound still holds without the per-exec sha256.
- **Disposition**: applied — Principles now scope AP-9 to the observability leg (channels/cohorts = installer-v1's O-2) and note A-9's bound holds.

### FND-10 — acceptance-row evidence precision (A-1 label, A-2 construction-vs-request, A-5 review-vs-test)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: A-1 Method read "Test (e2e/live)" for a one-time manual run (no automated artifact — the installer e2e suite has no live-private-release+gh leg); A-2's wording ("an override fetch carries no Authorization header") is proven by *construction* (nil auth leg), not an observed request; A-5's "Review + test" names a test that does not exist (token non-logging is review-backed).
- **Acceptance condition**: relabel A-1 (manual/AP-14), rephrase A-2 to the construction property, drop A-5's "+ test".
- **Disposition**: applied — A-1 → "Live install (manual, AP-14)"; A-2 → "served by an unauthenticated fetcher (no auth leg constructed)"; A-5 → "Review …".

### FND-11 — §4's `gh`-shell-out rejection leans on an uncited "prefer self-contained Go" premise

- **Severity**: advisory (CAL-1)
- **Lens**: coherence
- **Evidence**: design §4 rejected the shell-out because "the API endpoint is self-contained Go" — vs — that premise is the repo's Go-by-default / bash-only-on-proven-necessity policy, uncited, so the verdict read asserted.
- **Acceptance condition**: cite the language policy so the rejection is grounded.
- **Disposition**: applied — §4 now cites the Go-by-default policy and the bootstrap-exception carve-out.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-21 |
| 2 | approve-with-advisories | 2026-07-21 |

*Round 2 is a scope-locked re-review of the eleven applied spans: all three blocking findings are closed at the design layer (FND-1/2 fully; FND-3 via the named seam + A-6, its code test a tracked follow-up), and the eight advisories are applied. No new content was introduced beyond the dispositions. The residual advisories are the two code follow-ups the design now honestly records (the cross-host redirect test, the asset-map memoization/pagination) — none blocks the design as a durable record.*

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 8 advisory
- **Dispositions**: 11 applied · 0 declined · 0 withheld-as-violation
- **Lens yield**: upstream-alignment 2 blocking + 2 advisory · feasibility-completeness 1 blocking + 3 advisory · contract-quality 3 advisory · coherence 1 advisory (the design was substantively clean; findings were principle-honesty, credential-scope, and operational-completeness gaps, not correctness defects — consistent with a reconciliation of shipped-and-proven behavior)
- **Adversarial survival**: all four lenses independently confirmed the load-bearing supersession quote is verbatim-accurate, the four DINVs property-testable, and the three cited unit tests exist and pass; no fabricated-evidence candidate survived.

## Overlay corrections

none
