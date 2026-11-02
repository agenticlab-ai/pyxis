# Amendment 001 — attribution hardening: the derivation property that makes the gate bite

| Field | Value |
|---|---|
| **Amends** | argus-boundary-enforcement *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-13 |
| **Learned-from** | review-thread:docs/reviews/argus-attribution-gate.md#FND-2 · operator-decision:problem:argus-attribution-hardening#A-1 |
| **Blast radius** | tier: contract-changing; touches: FR-11, FR-12, FR-13, DOD-1, DOD-3, DOD-10, DOD-11, DOD-12, DOD-13, Q2; reason: supersedes a Must FR (FR-11→FR-12) and adds a Must FR (FR-13) + two blocking DoD rows (DOD-10, DOD-11), and supersedes the two blanket DoD rows the FR-11 supersession made stale (DOD-1→DOD-12, DOD-3→DOD-13) to re-source them onto the live Must set — the attribution-gate derivation property is tightened so the merge-boundary guarantee actually bites |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-11

Reason: boundary-enforcement verification found the attribution gate does not
bite in practice — the derivation it contracted is *latent* (nothing produces
the authorship signal, so real Argus commits carry none and are served) and
*forgery-resistant only in direction, not in strength* (the frozen design's own
§7 Q1). FR-11 required the signal to be badge-independent and credential-free at
CI evaluation time, but not to be *produced by construction* on every
drive-authored commit, and did not bound its *forgery-strength*. This row keeps
the existing property verbatim and adds the two that close the latent gap and
the evade direction — stated as observable behavior; the *how* (where the signal
is produced, the forgery-resistance construction) is design-owned
(design:docs/tech-design/argus-attribution-gate.md supersession;
problem:argus-attribution-hardening#Q2).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-12 | The attribution gate shall determine Argus authorship from a signal that is **produced by construction** on every commit a drive's work is landed as — never chosen by the model, never hand-added — such that no drive path can land a commit whose Argus authorship is underivable; the derivation shall be **forgery-resistant in strength**, so a misbehaving or injected drive cannot forge, replay, or omit the signal to change the derived class; and it shall continue to require **no mutable platform badge and no ambient credentials at CI evaluation time**. | closes the latent-signal gap (a real produced signal, not a hand-seeded fixture) and §7 Q1's evade direction, while preserving FR-11's badge-independent/credential-free-at-CI property; the *property* is required at DoD, the *mechanism* (producer site, forgery-resistance construction) is design-owned | Must | critical | on a corpus that includes a **live drive-produced** commit (not only hand-seeded fixtures), the gate derives that commit as Argus by its produced signal; a **forged / replayed / stripped** signal on a protected-path diff does not flip the derived class (never Argus→human to evade, never human→Argus to frame); the derivation runs in the CI deployment context with no ambient credential and reads no mutable badge | Test | G1, J3, P-2, AP-14, AP-26, problem:argus-attribution-hardening#Q2 |

### op: add FR

Reason: the absence-masquerade gap — an absent signal on a protected-path diff
is served as human (DINV-2 as built), so stripping the signal looks like a
legitimate human edit, while the legitimate human-author path (NR-4) must
genuinely stay open. "Absence ⇒ human" is exactly how the masquerade works; the
fix at PRD altitude is that human authorship, like Argus authorship, must be
**positively established** on the protected subset, so the three states are
distinguishable. This sharpens what FR-9 (the refusal) and FR-10 (fail-closed on
underivable) leave under-specified: FR-10 already fails closed on *underivable*,
but neither row required *human* to be positively recognized rather than
inferred from absence (problem:argus-attribution-hardening#D-9). How human
authorship is positively established is design-owned.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-13 | On a diff touching a protected path, the attribution gate shall resolve authorship into exactly three distinguishable states and act on each: **positively-Argus** → refuse (FR-9); **positively-human** → pass, never refused by this gate (NR-4); **underivable** (neither positively established) → fail closed (FR-10). Human authorship shall be **positively established**, not inferred from the mere absence of an Argus signal, and its establishment shall not require the human author to perform any extra step to be recognized as human. | makes the frozen design's DINV-2 (underivable ∧ protected ⇒ fail closed) and DINV-4 (human path open) co-satisfiable — they are consistent only if human is positively derivable on the protected subset, else a human PR is "underivable" and DINV-2 wrongly refuses it, breaking NR-4; strengthens NR-4 (positively served, not inferred) | Must | critical | fixtures on a protected-path diff: a positively-Argus diff is refused; a positively-human diff passes; a stripped/absent-signal diff (neither state) fails closed and is **never** served as human; 0 legitimate human-authored protected-path PRs are refused | Test | G2, J3, F3, NR-4, problem:argus-attribution-hardening#D-9 |

### op: add DOD

Reason: the frozen DoD's fixture rows (DOD-1/DOD-3) verify hand-seeded fixtures;
the latent-signal gap means those fixtures passed while the live signal was
absent — so the completion bar must include a signal actually produced by a real
drive, the gap-1 closer (problem:argus-attribution-hardening#D-4).

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 | A genuine end-to-end drive of a protected-path write is refused **by the attribution gate** on its produced signal — proving the signal is actually produced on a real drive-authored commit, not assumed from a hand-seeded fixture. | FR-12; §3.2 M1 | Test | the live-drive artifact (the drive-authored commit) + the gate's refusal on it in CI | yes |

### op: add DOD

Reason: forgery-strength (§7 Q1's evade direction) must be proven, not assumed —
a forged, replayed, or stripped signal must be shown unable to flip the derived
class in either direction (problem:argus-attribution-hardening#D-4, #D-9).

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | A forged / replayed / stripped signal cannot flip the derived class: no forged signal makes a human diff derive as Argus (framing → false refusal), and no stripped signal makes an Argus protected-path diff derive as human (masquerade → served) — the diff instead fails closed. | FR-12; FR-13; ABUSE-3 | Test | the forgery/replay/strip fixture corpus red at the gate (class does not flip in either direction) | yes |

### op: supersede DOD-1

Reason: the FR-11 supersession (FR-11→FR-12) made this blanket completion row's
Source stale — it enumerated `FR-1..FR-11`, which now cites the superseded
FR-11 and excludes the two new live Must FRs (FR-12, FR-13), regressing the base
review's settled reconciliation that every blocking DoD proof rest only on Must
rows (review:argus-boundary-enforcement-000#FND-1). This replacement re-sources
the row to the concept — the live Must set — rather than a fresh range that a
future supersession could falsify again (the prd-template rule 14 fix: name the
concept and point at the live surface, never an enumeration). The completion
criterion itself is unchanged; only the volatile Source enumeration is repaired.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | All Must FRs pass their fit criteria | all live Must FRs (§5.1) | Test | CI run link + fixture-corpus report | yes |

### op: supersede DOD-3

Reason: same FR-11-supersession staleness as DOD-1 — this blocking row's Source
carried `§3.2 M1, M2; FR-1..FR-11`, whose `FR-1..FR-11` now cites the superseded
FR-11 and omits FR-12/FR-13. This replacement keeps the metric anchors (§3.2 M1,
M2) verbatim and re-sources the FR enumeration to the live Must set by concept
(prd-template rule 14), so no blocking DoD row cites a superseded ID and the
blanket bar covers FR-12 and FR-13. The completion criterion is unchanged.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | Every violation class in the seeded corpus is refused by the layer that owns it, red at exactly that layer; every legitimate case passes (0 false refusals) | §3.2 M1, M2; all live Must FRs (§5.1) | Test | the golden violation + legitimate corpus + first-run refusal report | yes |

### op: answer Q2

| Field | Value |
|---|---|
| **Answer** | The attribution-gate derivation property is now contracted in full: the signal is produced by construction on every drive-authored commit, forgery-resistant in strength, badge-independent and credential-free at CI evaluation time (FR-12), with authorship resolved into three positively-established states on the protected subset (FR-13). Q2 remains **open at the mechanism altitude** and is handed to the stage-3 technical design (a supersession of `docs/tech-design/argus-attribution-gate.md`): *how* the signal is produced (producer site — the commit-authoring actor, not the drive prose), *how* forgery-resistance is constructed, *how* human authorship is positively established, and the pre-mechanism/post-mechanism transition rule. |
| **Evidence** | problem:argus-attribution-hardening#D-4 · #D-5 · #D-9 · fog G-1..G-4 · docs/reviews/argus-attribution-gate.md#FND-2 |
| **Decided-by** | Argus (AI Engineer, decision principal at tier-2; authority-contract/v1) |
| **Effects** | FR-12, FR-13, DOD-10, DOD-11 |
