# Amendment 001 — the permission hand-off exception

| Field | Value |
|---|---|
| **Amends** | init-hardening *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-29 |
| **Learned-from** | review-thread:`agenticlab-ai/pyxis#812 impl-review FND-1 (comment 5125178984)` · operator-decision:`session init-pyxis 2026-07-29 — keep the hand-off` |
| **Blast radius** | tier: contract-changing; touches: §0, UB-1 (superseded by UB-4); reason: a Must UB row's recovery semantics reverse for the permission class — the composed acceptance matrix (T11) surfaced a genuine contradiction between this row and the installer's designed non-admin hand-off, and the operator ruled the hand-off stands |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede UB-1

Reason: T11's composed matrix proved UB-1 contradicts the installer's designed non-admin hand-off (its own UB-4 heritage: a permission-classified wiring failure completes what the credential allows and hands off the rest — deliberately green, never an error); the operator ruled the hand-off stands, so the rollback demand reverses for exactly this class while every other post-mutation failure keeps FR-10's restore.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-4 | a wiring operation fails for permission after the checks passed (scope gaps are not fully pre-probeable) | complete every remaining operation the credential allows, finish green, and record the handed-off operations with their exact remedial commands — the installer's non-admin hand-off; no rollback occurs for a permission-classified wiring failure | a green install + the hand-off block naming each remaining operation and its command | Must | high | injected permission failure at wiring → the run ends green with the hand-off recorded (operations + runnable commands) and the target NOT rolled back; every non-permission post-mutation failure still restores as-found (FR-10) | Test | F3, FR-10 |

### op: supersede §0

Reason: the op set changes content §0 routes — judge row 3's "as-found or fully-installed, nothing between" gains the ruled permission-hand-off exception and its anchor set shifts from UB-1..3 to UB-2..4; §0-only informative scope, replaced wholesale at the computed contract-changing tier.


*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these** *(ranked by irreversibility × blast radius)*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Checks-then-go: no plan artifact, no confirmation ceremony, no readiness verb — checks front-loaded, refusals fail-fast (`#D-7`) | plan→confirm→apply init, or a standalone `preflight` verb | veto reshapes FR-9's ordering invariant and re-opens NG-2 → §4.1, FR-9 |
| 2 | Empty-only admission; every other target routed to `adopt`/`apply` by name (`#D-7`) | a collision-diff init that negotiates with content | veto collides with adopt's charter and re-opens NG-1 → FR-1, FR-2 |
| 3 | Full rollback: as-found or fully-installed — with one ruled exception: a permission-classified wiring failure completes green and hands off instead of rolling back (`#D-5`; UB-4, the amendment-001 ruling) | debris-aware refusal, or a resumable pipeline; rollback-on-permission-too | veto rewrites FR-10 and UB-2..4's recovery shape → §5.2 |
| 4 | `ci_mode` wired real under its existing key name (`#D-3`) | rename (`docs_gate_mode`) or delete the knob | veto = FR-11 + the Q3 migration question → §7.2 |
| 5 | GitHub coordinate asked interactively when no origin exists; init sets origin itself (`#D-8`) | require the remote pre-set; or drop `repo=create` | veto = FR-6..FR-8 and the F4 flow → §4.3 |
| 6 | The primary metric is the unaided-install trial, human-judged; fixtures ride opportunistically (`#D-11`) | a contracted fixture-matrix gate | veto flips M1's verify method and DOD-6's blocking bit → §3.2, §5.8 |
| 7 | Code gate seeded at first use; init ships only the truth surface (`#D-4`) | a sixth init-time stack knob | veto adds a knob against `brainstorm:installer#D-6` → FR-14, Q1 |

**Sanity-check the numbers** — appetite ~8–12 tasks (header, `#D-12`); interactive re-ask bound: 3 attempts per question then a reported abort (FR-13); §0 itself and every fit criterion carry no other numeric targets — the trial is pass/fail (M1).

**The author's open questions** — Q1 first-use seeding owner · Q2 non-git remedy shape · Q3 `ci_mode` converge migration · Q4 gh scope-probe depth — all design-blocking, none solution-blocking → §7.2.

**Skip the rest** — structure/immutability → docgate; substance, coherence, upstream alignment → `/prd-review` (report rides this PR); prose/gloss quality → the conversation review; implementation defects → the design and its gates.

