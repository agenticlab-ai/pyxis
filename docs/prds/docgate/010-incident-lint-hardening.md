# Amendment 010 — incident lint hardening: tags and fields checked as content, not silhouettes

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005 + 006 + 007 + 008 + 009) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 |
| **Blast radius** | tier: contract-changing; touches: FR-15, FR-25; reason: supersedes the Must-row registry lint contract |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-15

Reason: the shape half of `lint-incidents` validated silhouettes — a
non-empty tag *bracket* whose content could be anything, and the four fields
by substring presence anywhere in the entry body (#54 E7). Tags are the
registry's retrieval key ("consumers grep their tags"), so malformed tags
silently break retrieval; and a field matched as a substring can pass while
carrying no anchored line. The lint contract now names the content rules the
registry header has always documented, and reads the registry from its
configured path.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | When invoked as `lint-incidents`, docgate shall verify the registry at its configured path is append-only vs the merge base — committed `I<n>` blocks immutable except the sanctioned `Links:`-line growth — and that each entry carries a dense `I<n>` ID, a `YYYY-MM-DD` date, a bracketed tag list of one or more comma-separated kebab-case tokens, and the four named fields (Decision, Broke, Lesson, Links) each present as a `- **<Field>:**`-anchored line within the entry. | tags are the retrieval key and fields are the contract — silhouette checks let malformed entries read as conformant (#54 E7) | Must | med | empty-tag and non-kebab-tag fixtures red naming the entry; a field present only as an unanchored substring red; `I1` and the real registry green; graduation `Links:` growth still green; density and append-only fixtures keep their verdicts | Test | J3 |
