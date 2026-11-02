# Amendment 001 — section-zero-skip

| Field | Value |
|---|---|
| **Amends** | prd-review *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-14 |
| **Learned-from** | operator-decision:brainstorm:gate-attention#D-7 · policy-change:AGENTS.md rule 9 (read-to-rule §0 is human-only) |
| **Blast radius** | tier: contract-changing; touches: FR-17 (new Must row); reason: adds a binding scope exclusion to the reviewer's contract — the read-to-rule adoption event (prd:read-to-rule Q2), landed before the first §0-bearing draft reaches a review moment |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: prd-template/v2 documents open with a human-only read-to-rule section (§0); the operator ruled machines never read, cite, or obey it — the reviewer adopts the skip before the first §0-bearing subject arrives.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-17 | While reviewing a subject or consulting a corpus document, the reviewer shall treat any read-to-rule section (§0) as outside its scope: no lens shall route by its content, and no finding shall cite into it or judge it. | §0 is human-only by operator ruling — informative, never normative; machine attention stays exhaustive over the contract body it reviews (AGENTS.md rule 9; prd:read-to-rule NG-2) | Must | med | on a §0-bearing subject, zero findings anchor into §0 and zero lens output cites §0 content; a §0-stripped re-review of the same subject yields the same finding set (the fixture arrives with the first §0-bearing subject) | Eval | G1, J1 |
