# Amendment 001 — Parent→children query surface, and children lookup that scales

| Field | Value |
|---|---|
| **Amends** | local-work-tracker *(chain: 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-15 |
| **Learned-from** | operator-decision:carina-dogfood-review-2026-07-15 |
| **Blast radius** | tier: additive; touches: FR-19, NFR-8; reason: adds one Should query capability and its scale NFR — no Must row, DoD blocking row, non-goal, or §8 content changes |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: FR-7 exposes only the *open*-children rollup, so a completed epic's full task set and any parent-scoped listing are unreachable except by scanning the store files by hand — the parent→children direction has no first-class query, surfaced while dogfooding Carina (the epic file lists no children; only a scan or `show <epic>`'s open-only rollup resolves them).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-19 | When asked to list a work item's direct children — optionally filtered by state — the tracker shall return every item whose parent is the given item, across all states by default, through one implementation shared by the engine (port) and the human (CLI). | the parent→children direction must be directly queryable, not only reachable as the open-children rollup (FR-7) or by a hand scan; a completed epic must still surface its full task set, and the port and CLI must not diverge (FR-16) | Should | low | over a fixture corpus, a parent-scoped query returns exactly the given item's direct children in every state, byte-identical from the CLI and the port; a state filter narrows the set to the expected members; a parent with no children returns empty, not an error | Test | J2, J3, FR-7, FR-16 |

### op: add NFR

Reason: children enumeration and the epic rollup are today a full-store scan — O(N) in the total item count to find one parent's k children — which will not hold the interactive/scheduling latency bound as a backlog grows. The intended mechanism is the design's already-deferred index (Q3/R1) applied to hierarchy: a rebuildable derived index off the authoritative `parent` fields, bypassed on decision-path reads (FR-14/NFR-3, DINV-4) and never a second source of truth (DINV-1/AP-27) — the requirement here states the outcome; the design chooses the mechanism.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-8 | Performance / Scale | The tracker shall resolve a work item's direct children (FR-19) and its open-children rollup (FR-7) in time bounded by the number of children, not by the size of the whole store — with no full-corpus scan on the children or rollup query path — while the child's `parent` field remains the sole authoritative record of the edge. | children/rollup latency for a parent with a fixed k children stays within NFR-1 and does not grow with total item count N, measured at N = 10³, 10⁴, and 10⁵; any index that backs the path is rebuildable from the item files and is bypassed by decision-path reads (NFR-3) | Test |
