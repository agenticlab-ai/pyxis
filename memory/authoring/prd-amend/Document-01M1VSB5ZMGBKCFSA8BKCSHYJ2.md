# prd-amend — evolve an approved PRD ledger

Author the **next amendment** in an existing ledger `docs/prds/<feature>/`.
Merged entries are immutable; the ledger only grows. Input: the feature,
what changed, and the evidence; ask for whichever is missing.

## Steps

1. **Read the current contract**: `docs/prds/<feature>/effective.md` (the
   projection is what you amend *against*; the ledger files are history).
   Read `docs/templates/prd-amendment-template.md` fully (op grammar table
   + worked examples).
2. **Next dense number**: `NNN-<slug>.md` — `docgate lint-structure` names
   the next free number if unsure; racing PRs renumber, never gap.
3. **Envelope — all seven fields, exact names**: `Amends` (feature + chain
   position) · `Status` (`Draft` while authoring; `Approved` only when the
   operator approves — it cannot merge otherwise) · `Owner` · `Date` ·
   `Learned-from` (typed: one of the nine classes, `class:reference`,
   `·`-separated) · `Blast radius` (`tier: <t>; touches: <IDs>; reason:
   <why>` — docgate computes the floor; declared may exceed, never
   understate) · `Template version` = `amendment-template/v1`.
4. **Operations — closed vocabulary, complete payloads**: `### op: add
   <KIND>` · `supersede <ID>` · `supersede §<sec>` · `retire <ID>` ·
   `answer Q<n>` (typed payload: Answer / Evidence / Decided-by / Effects)
   · `rescope <NG-n|WR-n>` (+ absorbing rows). Every payload row is a full
   template-schema row with a **fresh ID** — column names and order match
   the PRD template's schema exactly (the gate rejects otherwise). One
   amendment = one coherent intent; each op carries a `Reason:` line.
   **§0 maintenance**: when the ops change content the document's §0
   routes to, include `### op: supersede §0` with the complete
   replacement section body in the same op set, declaring the computed
   `contract-changing` tier with the §0-only informative scope named in
   the reason (prd:read-to-rule/FR-6 · design read-to-rule-v1 §3.6).
5. **Validate + fold + diff locally**:
   `go run ./cmd/docgate validate-amendments docs/prds/<feature>` →
   `... fold ...` (regenerates both artifacts — commit them with the
   amendment) → `... diff ...` and **paste the diff output into the PR
   body** (the reviewer judges the folded effect, not the mechanics).
5b. **Run `/prd-review` on the amendment** — the unmanned reviewer over
   the folded effect (blast-radius honesty, Learned-from resolution, the
   resulting projection's chain integrity). Apply blocking findings or
   decline-with-reason; its report (`docs/reviews/<feature>-<NNN>.md`)
   rides the amendment PR. *Advisory until the reviewer's ship rule
   promotes adoption.*
6. **Review scales with blast radius**: contract-changing → full
   `/doc-review`; additive/corrective → PR review suffices unless the
   operator asks. On a §0-bearing document the approving review carries
   the capture line (the template's gate-table guidance). Merge freezes
   the file.

## Never

Edit a merged `NNN-*.md` (evolve with another amendment); hand-edit the
projection (fix the op or the fold, refold); reuse or renumber IDs; leave
a payload for the fold to "interpret" — payloads are resolved, final rows.
