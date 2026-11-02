# Amendment 006 — governing fence completion: the vision's append-only half and the corpus freeze

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 |
| **Blast radius** | tier: contract-changing; touches: FR-14, FR-18, NG-1, FR-19, NR-1, NR-3; reason: supersedes a Must row and a non-regression row, and flips a non-goal inward |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-14

Reason: the 2026-07-08 review's code audit (#54, A2/A5) found `check-governing`
line-checks only the principles — a trailered edit can rewrite or delete
vision content despite AP-7 declaring the vision append-only from the
beginning — and rule 1's link-direction purity (the abstract layer links to
no concrete document and cites no requirement IDs) had no mechanical half at
all. The governing contract gains both: append-only applies to **both**
governing documents even with the trailer, and abstract-layer purity becomes
a check instead of a prompt rule.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-18 | When invoked as `check-governing`, docgate shall verify the vision and principles documents contain no relative markdown link and no baseline requirement-ID citation (the SCAF/PLAN/IMPL/GATE/FLEET/IDEN/GOV/KNOW/OBS/BENCH/VNEXT families); when additionally given a merge base, it shall fail any diff touching either document whose commits lack the `Governing-Change: operator` trailer, and shall fail line modifications or deletions in either document even with the trailer (append-only). | the trust root's machine backing was half-built: only principles were line-checked, and link-direction purity was prompt-side only (#54 A2/A5) | Must | high | trailered line-edit of the vision red; trailered append to the vision green; a vision fixture containing a relative link or a `VNEXT-1` citation red naming the line; principles behavior unchanged (trailered line-edit still red); the real corpus green | Test | G1, J3 |

### op: rescope NG-1

Reason: the flat placeholder corpus is the requirement source that `corpus:`
anchors and the stable-ID rule (AGENTS.md rule 4) depend on, and one document
already calls it immutable — yet nothing enforced any of it (#54 C5/A5).
Freezing the files' bytes honors NG-1's spirit (no format, template, or
content requirement is imposed — placeholders are never retrofitted) while
ending the honor-system mutability of citable requirement text. **Operator
attention:** merging this amendment is the ruling that the corpus is frozen;
extension happens by adding new files (the practiced `vnext-self-improvement`
pattern), and `README.md` stays editable as the index.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-19 | When invoked as `check-immutable` with a merge base, docgate shall fail any modify, delete, or rename of a flat placeholder-corpus file (a `.md` file directly under the configured PRDs path, excluding `README.md` and ledger directories) present at the merge base, while imposing no format, template, or content requirement on corpus files. | the corpus is citable requirement text (`corpus:` anchors, rule-4 stable IDs) with no mechanical stability; byte-freeze without retrofit resolves its undeclared mutability class (#54 C5) | Must | med | fixture: modify and delete of a merged flat corpus file each red naming the freeze; `README.md` edit green; a new flat file green; ledger directories unaffected; no content lint runs on corpus files; the real corpus green | Test | G1, J3 |

### op: supersede NR-1

Reason: NR-1 protected "the placeholder corpus and every non-ledger doc are
never gated" — FR-19's byte-freeze makes that statement false as written. The
protection it actually carried (no content gating, no reformatting, no
retrofit) survives narrowed; leaving the old row live beside FR-19 would be
two contract rows in direct tension (the semantic-tier miss the amendment
model warns about).

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-3 | Corpus and non-ledger docs are never content-gated, linted, or reformatted — the corpus fence (FR-19) freezes bytes of merged flat corpus files and nothing more; wiki, maps, and rollups keep exactly their own named checks | placeholders are never retrofitted (the surviving spirit of NG-1); gates never widen silently | low | a gate run over the current repo yields zero content findings on flat corpus files; FR-19 fires only on modify/delete/rename of merged flat corpus files; wiki/map/rollup checks unchanged | Test | G1, J3 |
