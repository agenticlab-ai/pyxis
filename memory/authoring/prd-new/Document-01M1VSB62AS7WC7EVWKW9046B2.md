# prd-new — start a new PRD ledger

Author the **base document of a new PRD ledger**. The result is a contract
the factory can execute, not prose. Input: a feature slug (kebab-case,
permanent) and the problem/idea; ask for whichever is missing.

## Input: a gated brainstorm record (optional)

When the operator names a brainstorm record (`docs/brainstorms/<slug>.md` —
produced by `/feature-brainstorm`; an idea whose decisions are not yet
resolved should start there, not here):

- **Refuse** a record that is ungated or whose verdict is not
  `ready-for-prd` — name the record, its state, and the remedy (finish the
  session; or gate it), and write nothing.
- **Synthesize, don't re-interview**: read the record whole; live `D-n`
  entries are resolved decisions — never re-ask one. Ask a new
  decision-question only for a genuine gap the record neither decides nor
  parks (each such question is a defect signal against the session,
  measured by the PRD's M1).
- **Cite by the record's grammar** (defined in
  `.pyxis/templates/brainstorm-record-template.md` Appendix C, never
  restated here): every §7.3 decision-log row carries its
  `(brainstorm:<slug>#D-n)`; non-blocking `G-n` fog lands in §7.2 as open
  questions with `(brainstorm:<slug>#G-n)`; `F-n` facts may back §2.4
  evidence rows. `O-n` rulings seed §3.4 NG/WR rows; `P-n` constraints pass
  to the technical design via §6.2/§7.2, never resolved in the PRD.
- **Route `T-n` candidate glossary terms** into `docs/wiki/glossary.md`
  (canonical term + avoid list, in the wiki's format), in the same
  introducing PR — the session coined them; synthesis is where they land.
- **Set the record's `Feeds` row** to the new ledger path, and land the
  record in the same introducing PR as the 000-base.

A recordless invocation follows the steps below unchanged.

## Synthesis discipline (both paths)

- **Synthesis never invents a product decision.** A genuine gap — something
  the record or conversation neither decides nor parks — is named
  precisely, then either asked **once** (recommendation attached) or parked
  in §7.2 as an owned open question. Plausible filler in a decision's place
  is the defect this whole stage exists to prevent.
- **The review adjusts; the merge freezes.** The `/doc-review` session on
  the draft (step 7) is the *sanctioned* surface for adding and modifying
  decisions — that is its point. Every adjustment is propose-and-confirm,
  and on a record-fed PRD it lands twice in the same change: the record
  gains the new or superseding `D-n` (anchored `YYYY-MM-DD · review`), and
  the affected PRD rows update their `(brainstorm:<slug>#D-n)` citations —
  provenance stays complete until merge freezes both documents.
- *(Waiting room: a mechanical avoided-term lint over PRD prose against the
  glossary — a future docgate verb; until then glossary compliance is
  review discipline.)*

## Steps

0. **Check the ground** (recordless path — a gated record already did
   this): scan merged PRD scopes + NG/WR rows and `docs/brainstorms/`
   records for coverage or prior refusal. Covered → point at the source and
   stop; no duplicate contract. An idea whose decisions are *unresolved*
   belongs in `/feature-brainstorm` first, not here.

1. **Read the template fully**: `docs/templates/prd-template.md` (the
   how-to section leads it; §0 is the read-to-rule section your synthesis
   writes last; Appendix A holds the writing rules — EARS patterns, the
   vagueness blacklist, ID namespaces, verification methods).
2. **Create the ledger**: `docs/prds/<slug>/000-base.md`. Instantiate — do
   not copy skeleton + notes: delete §0 and every guidance italic; keep
   every numbered section either filled or as heading + `Skipped: <reason>`
   (§5.8 and §8 are never skipped; a skip note is one honest line).
3. **Pick the tier honestly** (T1/T2/T3 per §0's table — rigor scales with
   risk and blast radius, not size of prose).
4. **Header is the routing surface**: `In one line` + `Relevant-when` decide
   whether future agents load the doc — write them like a skill
   description. `Template version` = `prd-template/v1`. Appetite is
   machine-native: human attention + machine work + rollout burden — never
   human time. Status starts `Draft — problem review pending`.
5. **The load-bearing rules while writing**:
   - Requirements are EARS-shaped rows with a **fit criterion** and a
     **Verify** method (Test · Demo · Inspection · Analysis · Eval · Human
     judgment — the last only with named owner + rubric).
   - Metric targets bind to **checkpoints**, never calendar dates.
   - Boundaries are anchored: refusals as `NG-n` rows with reasons,
     deferrals as `WR-n` rows with revisit triggers.
   - IDs are permanent; never renumber; supersession is the only change.
   - Cite upward (vision/principles); the vision cites nothing concrete.
   - **§0 is written last** (like §1): synthesize the read-to-rule section
     from the document's own settled content per the template's §0
     contract — ranked calls with veto semantics, the numbers, the
     author's open questions, the informed-skip map; every routed item
     anchored and resolving in this document; honest zeros asserted,
     never padded.
6. **Fold the projection** (never hand-author it):
   `go run ./cmd/docgate fold docs/prds/<slug>` — commit `000-base.md` +
   both generated artifacts together. `docgate gate` must be green.
6b. **Run `/prd-review` on the folded draft** — the unmanned reviewer
   (four lenses, closable findings, a verdict). Apply blocking findings or
   decline-with-reason (a declined blocking finding escalates); its report
   (`docs/reviews/<slug>-000.md`) rides this introducing PR as the
   document's evidence package. *Advisory period: until the reviewer's
   ship rule promotes adoption, its verdict blocks nothing — findings
   accumulate as measurement.*
7. **Review = the gates**: run `/doc-review` on the base; the session is the
   problem review and solution review — record sign-offs in the §2.6/§4.4
   tables and advance the Status ladder when the operator approves. Walk §8
   (contract readiness) by hand and mark the rows. Sign-offs on a
   §0-bearing document carry the capture line (the template's gate-table
   guidance beside §2.6).
8. **PR**: feature branch `docs/<slug>-prd`, one concern, conventional
   title. State in the body: merge is the freezing event; all future change
   arrives as typed amendments (`/prd-amend`).

## Never

Hand-edit `effective.md`/`effective.source.json`; renumber anything; leave
a Must row without a fit criterion; write dates where checkpoints belong;
touch the flat placeholder corpus conventions (exempt, never retrofitted);
invent a product decision the input didn't resolve (ask once, or park it in
§7.2); apply a review-born decision to a record-fed PRD without landing its
`D-n` in the record.
