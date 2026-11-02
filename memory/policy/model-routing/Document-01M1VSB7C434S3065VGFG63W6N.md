# Model routing — least-powerful-that-suffices

The routing policy for every model-backed task in the pipeline: **run each
task on the least-powerful model that reliably does it, and reserve the
strongest models for judgment whose miss is expensive.** The task's
*complexity* sets a baseline tier; the subject's *risk* adjusts it.

This is data an orchestrator (a human driver, Argus, or a future engine)
reads to pick a model — not a per-task guess. It is the bootstrap form of
routing-as-data: a table today, a mechanical picker when the engine consumes
it. Tiers are named by capability, not by product ID, so the concrete model
per tier is a configuration choice that moves with the model lineup.

## Baseline tier by task complexity

| Complexity | Task classes | Tier |
|---|---|---|
| **Deterministic** | docgate / plangate gates, folds, projections, index generation | **no model** — code, not inference |
| **Mechanical** | applying a specified disposition (the finding's acceptance condition dictates the change); confirming materialization; index / glossary handoffs; verifying an artifact against a gate or checklist; mechanical amendments (pure re-anchoring / renumbering); git and PR mechanics | **cheapest** |
| **Authoring** | prd-new, plan-new, tech-design-new — bounded synthesis from a gated record or an approved contract; triggering materialization | **mid** |
| **Judgment / adversarial** | prd-review, tech-design-review; brainstorm grilling (feature- and tech-design-); breaking-decision judgment and escalation classification; security- or governance-surface analysis | **strongest** |

The dividing line is whether a miss is cheap to catch downstream. Mechanical
work is checked by a gate or a human glance; a wrong judgment call on a
contract or a security surface is not, so it earns the strongest model.

## Risk adjusts the baseline

- A **low-risk** subject (a T1 PRD; a design with no security or
  factory-control surface) may drop the baseline **one tier** — a low-risk
  review runs at the authoring tier rather than the strongest — but never
  below the cheapest.
- A **high-risk** surface (identity, permissions, secrets, governance, the
  merge gate, or any breaking decision) **pins the strongest tier**,
  whatever the task's baseline. Risk never lowers a judgment task on a
  high-risk surface.

## The rule, in one line

Complexity picks the floor, risk can only *raise* toward the strongest (or
lower a low-risk task by one), and deterministic work spends no inference at
all. When unsure between two tiers, pick the lower one for mechanical and
authoring work and the higher one for judgment work — a cheap mechanical
retry costs less than an over-powered fleet, and a missed judgment call costs
more than the model that would have caught it.

## Enforcement — the plan-time routing envelope

The policy's **mechanical half** is enforced at plan time by the `routing` plan
check (`internal/plangate/plan/reg_routing.go`, per
`docs/tech-design/routing-envelope-v1.md`): a task's frozen `Routing: rung=<n>`
must sit within `[floor(complexity), strongest]`, where the complexity floor is
derived from the task's `Mechanical` signal — a mechanical task floors at the
cheapest rung, a behavioral one at the mid. A rung under the floor
(under-powered) or off the ladder fails the fold; an absent proposal is valid.
The encoded rung ladder there is **authoritative**; this document is its human
face and stays in parity with it (a test pins the two). The policy's **judgment
half** — raising within the envelope for a risky surface, and the low-risk
drop — remains the planner's call (a per-task risk signal is a later increment).
