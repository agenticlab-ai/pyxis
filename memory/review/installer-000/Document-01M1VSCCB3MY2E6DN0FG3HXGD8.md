# Review: prd:installer/000 — review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes on 14 findings (8 blocking — an uncarried I3 fail-closed-CLI lesson, four §3/§8 trace/denominator breaks, an open-ended offline scope, two host-state seeding gaps; 6 advisory); all 14 applied; round 2 verified closure → approve. |
| **Relevant-when** | auditing or consuming the review of the installer PRD; asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | prd:installer/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide as of 2026-07-19 |
| **Date(s)** | 2026-07-19 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the registry's fail-closed-CLI lesson (I3) is uncarried on the new `pyxis` CLI

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-19/FR-4 define preview and flag modes, but no row required fail-closed behavior when a flag-like token lands in a positional slot — vs — incident I3: "consequential CLIs fail closed on misparse: any flag-like positional is a usage refusal before any action." The installer is the most consequential CLI shipped (writes into foreign repos, mutates GitHub state); a misparsed `--preview` executing the full write set is exactly I3's class.
- **Acceptance condition**: a UB row requiring every `pyxis` verb to refuse flag-like positionals and unknown flags with a usage error before any write or external operation, exercised in the e2e matrix in both modes.
- **Disposition**: applied — UB-9 added (Must/high; zero-writes fit in interactive and flag-only modes); DOD-5's drill set extended.

### FND-2 — §8's F3 coverage row cited FR-6 falsely and omitted FR-19

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §8: "F3 → FR-5/FR-6/UB-5/UB-8" — vs — FR-6's own Traces column read "G2, J2" (no F3), FR-19 (which traces J2, F3) was absent, and UB-8 traced "FR-18, WR-10" (no F3). §8 is verified, not trusted.
- **Acceptance condition**: the §8 F3 row and the cited rows' Traces columns agree, with FR-19 included.
- **Disposition**: applied — F3 added to FR-6's and UB-8's Traces (both honestly apply to the repair flow); §8 row now reads "F3 → FR-5, FR-6, FR-19/UB-5, UB-8, UB-9".

### FND-3 — F2 (offline flow) had no failure-coverage traces

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §8: "F2 → FR-13" alone, while the check row claims functional + failure + non-regression coverage per flow — vs — no §5.2 row carried F2 in its Traces.
- **Acceptance condition**: the applicable UB rows trace F2 and the §8 note reflects them.
- **Disposition**: applied — UB-2 (fetch-phase network failure) and UB-7 (missing prerequisite) now trace F2; §8 note reads "F2 → FR-13, UB-2, UB-7, UB-10".

### FND-4 — M2's "defined drift classes" denominator was never defined

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M2 target "100% of the defined drift classes repaired on the same matrix", consumed by the §3.3 ship rule — vs — no enumeration anywhere; EVT-2's `drift_classes` captures the observed set, not the expected one, so the ship rule had no computable threshold.
- **Acceptance condition**: a closed drift-class enumeration in the M2 row.
- **Disposition**: applied — M2 now enumerates: engine-verbatim tampered/deleted · generated stale/deleted · repo-owned schema-invalid (named, never rewritten) · interrupted partial install (UB-5).

### FND-5 — M3's Feeds column falsely attributed EVT-1 as its instrument

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M3 "Feeds from: EVT-1 + the repo's own pipeline artifacts" — vs — EVT-1 fires at `install_completed` with no property capturing a feature passing the pipeline; the real measurement vehicle is the DOD-8 demo record.
- **Acceptance condition**: M3's measurement basis stated as the DOD-8 demo record, EVT-1 demoted to the install precondition; §8's note matching.
- **Disposition**: applied — M3 and the §8 metrics row now name the measurement plan explicitly.

### FND-6 — FR-13's "every installed behavior" was an open-ended scope its fit criterion never closed

- **Severity**: blocking (CAL-3 — the defect made a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-13: "init and every installed behavior shall complete with zero network operations…" with fit "the full offline combo passes" — vs — Appendix A's ban on open-ended clauses: a test exercising only init + gate would pass the criterion while uncovered behaviors violate the requirement.
- **Acceptance condition**: an explicit behavior enumeration in the requirement and a per-behavior fit criterion.
- **Disposition**: applied — FR-13 enumerates init, converge/repair, doctor, pinned-binary resolution, the hook-run gate, and skill invocation from a wired CLI surface; the fit criterion exercises each and fails on any observed network operation.

### FND-7 — unset git committer identity would fail the seeding path on fresh machines

- **Severity**: blocking (CAL-4 — load-bearing for §8: FR-8's green-on-install fit is unverifiable if seeding cannot commit)
- **Lens**: completeness-critic
- **Evidence**: UB-7 covered a missing git *binary* only; §4.3's "first commit-able tree" implies a commit — vs — the common fresh-machine state (git present, user.name/user.email unset), squarely on the design audience's path.
- **Acceptance condition**: a UB row handling unset identity (establish repo-local identity or refuse by name; never a failing commit attempt) plus a prerequisites-note update.
- **Disposition**: applied — UB-10 added; §6.6's platform prerequisites note now includes committer identity.

### FND-8 — "empty repository" was ambiguous: pre-existing git repo or bare directory?

- **Severity**: blocking (CAL-4 — a stage-6 ambiguity: FR-7/FR-8 fits unverifiable without the entry-state definition)
- **Lens**: completeness-critic
- **Evidence**: FR-7 "When init runs in an empty repository…" and §4.3 "init on a repo with no commits…" — vs — the no-`.git` case was unstated: neither a named prerequisite nor an installer obligation.
- **Acceptance condition**: define the precondition and state whether init runs `git init`.
- **Disposition**: applied — FR-7 now defines an empty target (no tracked content) and obliges init to initialize git where absent (consistent with D-4's "from nothing"); the fit criterion covers both entry states; §4.3 updated.

### FND-9 — FR-8 packed two requirements with two distinct triggers into one row

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-8 bound "when init completes" (local hook gate) and "on first push" (CI required check) in one sentence — vs — EARS one-requirement-per-sentence; the two obligations were not independently trackable.
- **Acceptance condition**: split into two rows with separate fits.
- **Disposition**: applied — FR-8 is the local hook-gate half; FR-20 carries the first-push CI check.

### FND-10 — FR-17 carried three independently falsifiable clauses in one sentence

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-17: pinned+checksum resolution, never-build-from-source, and override honoring, three shalls in one row — vs — EARS one-requirement-per-sentence; failures could not be reported against separate anchors.
- **Acceptance condition**: three rows with sliced fit criteria.
- **Disposition**: applied — FR-17 (pinned + checksum), FR-21 (no source builds, named failure instead), FR-22 (override with the same verification, surfaced in preview and doctor).

### FND-11 — NFR-4's "every known failure state" was a moving-window loophole

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: requirement said "every known failure state"; fit said "all seeded states" — vs — the 29148 open-ended-clause ban: a known-but-unseeded state satisfied the fit while violating the requirement.
- **Acceptance condition**: bind the scope to the §5.2 enumeration in both requirement and fit.
- **Disposition**: applied — NFR-4 now reads "every failure state named in §5.2", fit enumerates UB-1..UB-10.

### FND-12 — §4.2's waiting-room parenthetical was stale (WR-1..WR-8 vs the table's WR-1..WR-10)

- **Severity**: advisory (CAL-1, downgraded: the rows exist in §3.4; the defect is a stale cross-reference in prose, not a missing contract row)
- **Lens**: coherence
- **Evidence**: §4.2 "…the platform loop (WR-1..WR-8)" — vs — §3.4's WR-9 (non-main branch) and WR-10 (path honoring).
- **Acceptance condition**: the perimeter cites the full range.
- **Disposition**: applied — §4.2 lists all ten with the two added items named.

### FND-13 — the bootstrap script itself is an attack surface ABUSE-1 did not name

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: ABUSE-1 mitigates artifact tampering via "checksum before any execution" — vs — the curl-pipe executes fetched shell bytes before any binary verification can run; the script, not the binary, is the first executed artifact.
- **Acceptance condition**: the vector named in §5.3 with its mitigation and a DOD-5 drill fixture.
- **Disposition**: applied — ABUSE-4 added (critical; pinned HTTPS bootstrap URL, checksum set fetched before executing any fetched byte beyond the shim, docs state the floor; drill added).

### FND-14 — transient GitHub API failures (rate limit) during wiring had no specified behavior

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.5's GitHub API failure-mode column covered permission and credential absence only — vs — a 429 mid-wiring leaves a partial state distinct from UB-4's documented scenario.
- **Acceptance condition**: extend UB-4 (or §5.5) to the transient class with the same partial-report + hand-off semantics.
- **Disposition**: applied — UB-4 now covers transient API failures with bounded retry then hand-off; a 429 fixture added to its fit.

## Round 2

Scope-locked re-review over the named defects and the changed spans only. All 14 acceptance conditions verified met in the revised draft: UB-9/UB-10/FR-20/FR-21/FR-22/ABUSE-4 present with fits and risk tiers; the §8 trace rows now agree with the cited rows' own Traces columns; M2's denominator is closed; M3 names its measurement plan; FR-7/FR-13 are precondition-complete and enumerated; DOD-1/DOD-3/DOD-5 ranges and the §6.6 prerequisites note follow. Regression sweep of the changed spans: the §5.1 priority-bar line ("every row above is Must") still holds over FR-20..22; every new row carries a risk tier; fold and gate green on the revised tree. No new findings; no `violation-withheld` entries.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-19 |
| 2 | approve | 2026-07-19 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 8 blocking · 6 advisory
- **Dispositions**: 14 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
