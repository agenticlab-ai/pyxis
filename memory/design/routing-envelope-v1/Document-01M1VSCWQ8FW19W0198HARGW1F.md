# Plan-time routing envelope — the `routing` plan check — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge is the freezing event; "superseded" is computed by the index)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — plan-gate surface |
| **Date** | 2026-07-14 |
| **Relevant-when** | designing or changing plan-time model-routing enforcement; implementing the `routing` plan check; triaging a rung-envelope defect; adding per-task risk to routing; superseding the check |
| **Scope** | The plan-fold check that validates a task's frozen routing rung against the model-routing policy **envelope**: the rung ladder, the complexity→floor derivation from the existing `Mechanical` signal, and the envelope bound. **Not**: the executor's downstream capability clamp at dispatch (FR-14's runtime envelope, AP-21 — orchestration); per-task **risk** adjustment and the low-risk drop / high-risk pin (a successor increment); the tier→concrete-model lineup map; routing for authoring/review skills (they read the same policy at spawn, no plan-time freeze). |
| **Tracking** | — |
| **Builds-on** | docs/tech-design/plan-new-v1.md · docs/tech-design/distributed-check-registration.md |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | none — a read-only plan-fold check over committed plan text; no identity, secret, network, or egress surface. |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The plan carries an optional per-task routing proposal — `Routing: rung=<n>`, frozen at plan time so the executor dispatches on the planner's judgment rather than re-judging (`prd:plan-new/FR-14`, "judge once, clamp downstream"). Today plangate validates only that the value *parses* as an integer (`rungRe`): `rung=99` and `rung=1` on a behavioral task both pass. FR-14's own fit criterion already asks for more — "the proposal parses **and never exceeds the envelope**" — but the envelope half is unimplemented, and the routing policy that would define an envelope (the **model-routing policy** — least-powerful-that-suffices: task *complexity* sets a baseline tier, subject *risk* adjusts) lives only as prose a planner applies by hand. So the rung is a free integer chosen by reading the policy — the exact prompting-not-mechanism gap `check-citations` closed for citations: nothing catches a behavioral task routed to the cheapest model (the miss the policy calls expensive — "a missed judgment call costs more than the model that would have caught it"), or a typo'd rung off the ladder. The plan-new architecture is "judgment in the session, determinism in tooling, one freeze point between them"; the routing policy splits the same way — a **mechanical floor** and a **judgment adjustment** — but only the judgment half runs today.

## 2. Decision

**Decision:** a plangate task check, **`routing`**, validates every task that declares a routing rung against the model-routing **envelope**. The rung must sit within `[floor(complexity), strongest]`, where the complexity floor is derived from the task's existing `Mechanical` signal — a mechanical task floors at the cheapest rung, a behavioral one at the mid rung — and `strongest` is the top of the rung ladder. A rung **below the floor** (an under-powered proposal — a behavioral task at the cheapest model) or **above the ladder** (an invalid rung) is a defect; **absence is valid** (FR-14 — the proposal is a Should, and absence is a legal plan). The rung ladder and the complexity→floor map are versioned policy the check reads, of which the **model-routing policy** is the human face.

The check enforces the policy's **mechanical part** — the floor and the ladder bound — and leaves its **judgment part** (raising within the envelope for a risky surface) to the planner: exactly the judgment-in-session / determinism-in-tooling split the plan is built on. It reuses plangate's per-task check registry (one self-registered file — distributed-check-registration), reading the already-parsed `Mechanical` and `Rung` fields; **no plan-grammar change**.

A reader who stops here should see the shape: the routing policy's floor becomes a deterministic plan-fold bound on the frozen rung — a proposal within the envelope passes, an under-powered or off-ladder one fails, and an absent one is untouched (never manufactured).

## 3. The design

### 3.1 The rung ladder and the complexity floor

The policy names tiers by capability, not product ID. As a ladder of rungs:

| Rung | Tier (capability) | Task complexity that floors here |
|---|---|---|
| 0 | no model — code, not inference | deterministic (gates, folds) — never a plan implementation task |
| 1 | cheapest | mechanical |
| 2 | mid | authoring / behavioral |
| 3 | strongest | judgment / adversarial |

A plan task is implementation work: mechanical (`Mechanical: yes`) or behavioral (`Mechanical: no`). Its complexity floor is therefore **rung 1** or **rung 2**. Rung 0 (no model) is not a plan task's rung — a task is executed *by* a model — and rung 3 (strongest) is reachable by the planner **raising** for a risky surface, never a plan task's complexity baseline. So the floor is a total function of the plan's own signal: `floor(Mechanical=yes)=1`, `floor(Mechanical=no)=2`.

### 3.2 The envelope bound

For a task declaring `rung=r`: **resolves iff `floor(Mechanical) ≤ r ≤ 3`.** Below the floor is under-powered (a behavioral task at rung 1); above 3 is off the ladder (a typo). Both fail with a named defect (task ID, the rung, the envelope, the complexity word). Absence — `Routing: —`, which the parser records as the `Rung == 0` "no proposal" sentinel — is valid and untouched: the check never *manufactures* a rung, it only *bounds a declared one* (FR-14: absence is valid). One consequence of that pre-existing sentinel: a literal `rung=0` is indistinguishable from absence and is treated as absent (§7 Q3) — harmless, since rung 0 (no model) is never a plan task's rung.

### 3.3 Policy encoding and reconciliation with `model-routing.md`

The ladder and the complexity→floor map are a small **versioned table encoded with the check**, closed like the citation-grammar set — the routing floor is universal pyxis policy, not per-repo tuning (unlike the sizing thresholds in the config surface). The **model-routing policy** document stays the human-readable policy — its rationale and tier table — and names this encoding authoritative, the plan-template / `parse.go` precedent ("the authoritative grammar is `parse.go`; this skeleton mirrors it"). One source of truth for the envelope; the doc mirrors it, and a test asserts the encoded ladder matches the documented tiers (R2). The genuinely per-repo-tunable piece — the tier→concrete-model lineup that moves with the model roster — is separate config, out of this check's scope (§3.4).

### 3.4 Boundaries and non-goals

- **Not solved here** — the executor's downstream capability **clamp** at dispatch (FR-14's runtime envelope / AP-21: a task proposed at rung 3 still clamps to what the executor can run — orchestration); the tier→concrete-model lineup map; routing for authoring/review skills (no plan-time freeze — they read the same policy at spawn).
- **Owned by a successor increment** — per-task **risk** as a signal, enabling the *exact* deterministic rung and the policy's low-risk **drop** (a low-risk behavioral task → cheapest) and high-risk **pin** (any surface → strongest). v1 is complexity-floor only.
- **Implementation must not infer** — a rung for a task with no proposal (absence is valid, never manufactured, DINV-1); a risk level (no risk signal exists yet — the ceiling stays the full `strongest`, so the planner may raise, DINV-6).
- **Must remain unchanged** — the plan grammar (`Mechanical`, `Routing` fields, `rungRe`); every existing plangate verdict on a plan whose declared rungs are within the envelope.

### 3.5 Observability

- **Events/logs** — one named `plangate` defect per off-envelope rung: the citing task ID, the declared rung, the envelope `[floor, strongest]`, and the complexity word.
- **Metrics / Alerts / Trace** — none: a plan-fold check with no standing process (AP-19); its result is the fold's exit code.
- **Debugging surface** — the defect names the rung and the envelope; the fix is to route within it (or drop the proposal).

### 3.6 Security & privacy

none — matches the header row: a read-only pass over committed plan markdown; no identity, secret, network, or egress surface.

### 3.7 Invariants

- **DINV-1** — a task with no routing proposal (the `Rung == 0` sentinel) is never a defect and never gains a manufactured rung.
- **DINV-2** — a declared rung within `[floor(Mechanical), strongest]` never fails.
- **DINV-3** — a declared rung below the complexity floor (under-powered) or above the ladder (off-ladder) fails with a named defect identifying the task, the rung, and the envelope.
- **DINV-4** — the check reads only the parsed plan (`Mechanical`, `Rung`) and the encoded policy; it writes no file and requires no merge base.
- **DINV-5** — the complexity floor is a total function of the existing `Mechanical` signal; the check reads no new plan field.
- **DINV-6** — the planner's judgment to *raise* a rung within the envelope (for a risky surface) is never overridden; the check **bounds**, it does not **determine**.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Derive the exact rung deterministically (`rung = f(complexity, risk)`); the planner declares nothing | needs a per-task **risk** signal the plan does not carry, and over-removes the routing judgment FR-14 deliberately preserves ("routing judgment available at plan time"). The envelope (mechanical floor + planner-raises) is the judgment-in-session / determinism-in-tooling fit; a successor with a risk signal can tighten toward exact. |
| Enforce only the ceiling (`rung ≤ strongest`), not the floor | catches a typo but not the *expensive* miss — a behavioral task routed to the cheapest model — which is the policy's whole point. The floor is the load-bearing half. |
| Add a per-task `Risk` field now | a plan-grammar change (parse, template, golden fixtures) for the adjustment half; v1's complexity floor delivers the mechanical guarantee with **zero** grammar change — risk is a clean successor increment. |
| Put the ladder in the config surface as tunable data | the ladder and floor are universal pyxis policy, not per-repo tuning; encoding them as closed code with the model-routing policy as the face avoids a config surface and its drift. The tunable lineup map is separate and out of scope. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the floor forbids the policy's legitimate low-risk **drop** (a low-risk behavioral task → cheapest) | that task can't be routed to the cheapest model; slight over-spend | the drop is an optimization, not a floor; v1 is conservative (never *under*-powers); the drop arrives with the risk signal (§7 Q1) | inspection: the envelope floors at complexity, ceilings at `strongest` |
| R2 | the encoded ladder drifts from `model-routing.md` | the enforced envelope and the documented policy disagree | one encoded source of truth; `model-routing.md` names the code authoritative (plan-template precedent) | a test asserts the encoded tiers match the documented ladder |
| R3 | the `Rung == 0` sentinel collides with rung-0 (no-model) | a literal `rung=0` reads as "no proposal" | rung 0 is never a plan task's rung (a task runs *on* a model); v1 treats `0 = absent`, documented; a distinct no-model proposal is a sentinel change out of scope (§7 Q3) | §3.2 |
| R4 | own-corpus false positive on a real frozen rung | the plan fold reds on merge day | A-6 own-corpus green gate; red-first fixtures precede it | A-6: `routing` green on the live plans |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | a behavioral task (`Mechanical: no`) declaring `rung=1` reds — below the complexity floor | Test | fixture red at exactly that task | §3.2 · #DINV-3 |
| A-2 | a rung within `[floor, strongest]` passes (mechanical `rung=1`; behavioral `rung=2` or `rung=3`) | Test | envelope fixtures green | §3.2 · #DINV-2 |
| A-3 | a task with `Routing: —` (no proposal) is untouched — no defect, no manufactured rung | Test | absent-proposal fixture green | §3.2 · #DINV-1 |
| A-4 | a rung above the ladder (`rung=9`) reds — off-ladder | Test | off-ladder fixture red | §3.2 · #DINV-3 |
| A-5 | the check reads only `Mechanical` + `Rung` + the encoded policy, mutates no file, needs no merge base | Test | fold run + working-tree diff | #DINV-4 · #DINV-5 |
| A-6 | the current repository's plans pass `routing` — no false positive on a real frozen rung | Test / Inspection | green fold on the live plans | §3.3 · R4 |
| A-7 | the encoded rung ladder matches the tiers documented in `model-routing.md` | Test | ladder-vs-doc parity | §3.3 · R2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | per-task **risk** signal — add a `Risk:` field (or derive it from `Touches`) to enable the exact rung + the low-risk drop + the high-risk pin? | successor | nothing | open | a successor increment; v1's complexity floor stands alone |
| Q2 | the tier→concrete-model lineup map — per-repo config (moves with the model roster) vs a release constant? | operator | nothing | open | out of this check's scope; the envelope is a capability rung, not a product ID |
| Q3 | should a literal `rung=0` (no model) be a distinct valid proposal, vs the current `0 = absent` sentinel? | implementation | nothing | open | v1: `0 = absent`; a distinct no-model proposal is a sentinel change, out of scope |

## Principles

- **AP-10 — honored.** The planner's "pick the right rung per the policy" becomes a deterministic plan-fold bound, not prose discipline — the second review/authoring instruction (after `check-citations`) turned from prompt to mechanism.
- **AP-11 / INV-30 — honored.** One more self-registered check under the one fold, sorted by name — the distributed-check-registration pattern, plangate's half.
- **AP-26 — honored.** The envelope is enforced against the task's own declared complexity signal, never a trusted external badge or the model's say-so.
- **AP-6 — honored.** The rung ladder is a closed set; a rung off it is a defect, never a leniently-accepted integer.
- **AP-19 — honored.** Zero steady-state cost: a plan-fold check, no standing process.
- **AP-13 — honored.** "The rung is apt for the task" becomes a verifiable fold artifact, not a planning claim taken on trust.
- **RW-5 — honored.** One decision — the plan-time routing envelope — in one document; the executor's runtime clamp and the risk adjustment are boundaries, not second decisions.
- **Deviation:** none.
