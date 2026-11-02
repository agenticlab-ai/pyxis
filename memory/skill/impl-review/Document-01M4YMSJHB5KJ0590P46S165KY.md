# impl-review-calibration/v1 — the reviewed severity dial

The closed calibration table every impl-review finding cites (`design:impl-review-v1#3.5`,
FR-12, DINV-3). It is **reviewed data**: a finding's severity is not a lens's free
choice — the lens picks the *row* whose failure class its finding matches, and the
severity is whatever that row carries here. Changing a row's `Default severity`
column re-tunes the reviewer with **no lens-instruction edit and no Go edit** — that
is the whole point of a data dial (design A-7).

`internal/implreview/calibration` parses this table **closed** — an unknown column
header or a malformed row is an error, never a lenient read (AP-6) — and exposes a
`report.CalLookup` (`cal_row → severity`) so `report.Validate` rejects any finding
whose `severity` contradicts the row it cites (DINV-3). Severity is exactly one of
`blocking` or `advisory`.

Rows are grouped by lens for the reader; the `Row` id is the citation a finding
carries in its `cal_row` field (e.g. `impl-review:<proposal-ref>#FND-3` cites
`CAL-3`). Row ids are stable — a row is superseded or a new row appended, never
renumbered.

## The table

| Row | Lens | Default severity | Escalation condition |
|---|---|---|---|
| CAL-1 | spec-fidelity | blocking | A charter Must is unmet or contradicted by the implementation — a stated required behavior is absent, inverted, or only partially built. |
| CAL-2 | spec-fidelity | advisory | The implementation satisfies the charter but a stated Should/intent is unmet, or an acceptance is met by a brittle path a later change would silently break. |
| CAL-3 | spec-fidelity | blocking | The implementation diverges from the charter's stated behavior on a load-bearing path the charter names — the reviewed code does something the charter forbids or omits something it requires. Escalate to `escalate` (not merely `request-changes`) when the charter itself is ambiguous or self-contradictory about the divergent behavior, so the fix is a spec decision, not a code fix. |
| CAL-4 | design-conformance | blocking | A binding element of the governing design (a DINV, a named contract, a partition line) is violated — the implementation breaks an invariant the design declares load-bearing. |
| CAL-5 | design-conformance | blocking | A repo convention the design or AGENTS.md makes binding is broken in a way the deterministic ladder does not enforce — e.g. an adapter package imported by the engine core (a boundary lint the ladder does not carry), a `map[string]any` cross-boundary contract, or a port defined on the wrong side. |
| CAL-6 | design-conformance | advisory | A non-binding design preference or a stylistic convention is departed from — legible-but-suboptimal structure, a naming or packaging choice weaker than the design's stated shape, with no invariant at risk. |
| CAL-7 | design-conformance | advisory | A documentation or clarity gap against the design — a doc comment that omits a consumer the design names, or a rendered-face wording weaker than the payload it derives from, with no behavior at risk. |
| CAL-8 | security-abuse | blocking | An abuse or security failure the served abuse rows or the security posture forbids — an injection sink, a secret path the ladder's scan cannot see (a computed or externally-sourced secret), a trust-boundary crossing that lets a lower-trust input steer a higher-trust act, or an identity-confusion (a bot act attributable to a human). |
| CAL-9 | security-abuse | blocking | A fail-open on a destructive or authority-granting path — a check that widens eligibility on error, an unbounded resource an attacker controls, or a disposition/attribution a lower-trust actor can forge. |
| CAL-10 | security-abuse | advisory | A defense-in-depth gap that is not itself exploitable — a missing secondary bound, an over-broad scope with no demonstrated reachable abuse, or a hardening the posture recommends but does not require. |
| CAL-11 | correctness-critic | blocking | A plausible-but-wrong defect the ladder's determinism cannot catch — a logic error, an off-by-one on a load-bearing bound, a concurrency race between steps the change itself runs or with a concurrent actor the charter or governing design says the code must withstand, an error swallowed on a path that must fail closed, or an edge case (empty, nil, boundary, unicode) that produces a wrong result rather than a caught one. |
| CAL-12 | correctness-critic | advisory | A latent correctness risk that is not a present defect — a fragile assumption that holds today but a named future change would break, a missing test over a real edge, or a non-hermetic dependency (clock/randomness/network) in a path that should be deterministic. |
| CAL-13 | correctness-critic | advisory | A race that needs an unrelated outside process to act on the same resource at the same moment, when neither the charter nor the governing design promises protection against it. No contract rests on it, so it is never a load-bearing critic finding: the deep band's critic escalation (which promotes CAL-12) leaves this row advisory. |

## How a lens uses a row

1. The lens judges a genuine defect the **deterministic ladder cannot see** (the
   floor exclusion — a finding that merely restates a ladder-enforced class is
   dropped, never emitted; `internal/implreview/floor`).
2. It picks the **one row** whose failure class the defect matches and cites that
   row's id in the finding's `cal_row`.
3. The finding's `severity` **is** that row's `Default severity` — the lens does
   not override it. A finding whose `severity` contradicts its cited row is invalid
   (`report.Validate`, DINV-3).
4. When a row's Escalation condition is met, the *round's* verdict is escalated per
   that condition (the verdict layer, T7) — but the finding's own severity is still
   the row's default; escalation is a verdict act, not a per-finding severity
   rewrite. Every blocking row also carries the shared condition below.

Changing severities or wording here is a **reviewed data PR** — never a code change,
never a lens-instruction change (design §3.5). The eval suite (T12) pins that a
data-only change to this table flips the affected findings' validity with no other
edit.

## Contract conflicts — escalate, never redesign through a finding

A blocking finding is a code fix only when the implementation can be corrected
while still doing everything the charter and the governing design state. When it
cannot — the contract cannot be met as written for the case the finding names, two
of its statements contradict each other, or every fix in view would replace or drop
something it states (a named command, step, path, or behavior) — the defect is a
contract decision, not a code fix. The lens reports it as a candidate that cites its
row as usual, with a message naming the clause and the case. When the candidate
survives the screen, the round's verdict is escalated with the conflict carried in
the escalation brief, and the finding against the conflicting span is withheld from
the round's findings (FR-14, FR-31) — this extends CAL-3's escalation condition to
every blocking row. A finding never sends the worker to change the governing design
in code.

A finding states the defect, the case that shows it, and the clause or behavior it
breaks. It may state what a correct result must satisfy; it never prescribes a
mechanism, command, data structure, or file layout that the charter or the governing
design does not already name.
