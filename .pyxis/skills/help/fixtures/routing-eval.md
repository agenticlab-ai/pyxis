# help — routing eval scenarios

The scenario checklist for `EVAL-1` / `A-2` (`ecosystem-help-v1`). Each row is
one ask, the entry the surface says it should route to (the **expected**
column — asserted against a live read per the runner protocol), and a
**result** column a run fills. Grounding notes cite the map row or skill that
owns the answer, so a surface change that breaks a row is visible here.

Legend for expected outcomes: a `/command` = route to that entry;
`refuse+map` = state it is not a pyxis entry point and point at
`.pyxis/pipeline.md`; `status+bridge` = state the stage's built-status and the
nearest built entry, never a command for the stage itself.

## Set A — canonical (one ask per entry class → `FR-2`, `A-2`)

| # | Ask | Expected | Grounding |
|---|---|---|---|
| A1 | "I have a rough idea for a feature; the decisions aren't settled yet." | `/feature-brainstorm` | map "Where a problem enters" rule 1 |
| A2 | "My idea is already shaped and the decisions are made — I want the contract." | `/prd-new` | map "Where a problem enters" rule 2 |
| A3 | "My PRD is approved. What explores the technical direction?" | `/tech-design-brainstorm` | stage 3 |
| A4 | "I've settled the design direction and want to author the design doc." | `/tech-design-new` | stage 4 |
| A5 | "I have a drafted PRD and want it reviewed to a verdict." | `/prd-review` | stage 2 (advisory) |
| A6 | "I have a drafted technical design and want it reviewed." | `/tech-design-review` | stage 5 (advisory) |
| A7 | "The contract is approved — decompose it into implementable work." | `/plan-new` | stage 7 |
| A8 | "The plan is frozen and projected; build its ready tasks here, now." | `/implement-now` | stage 8 |
| A9 | "A proposal is ladder-green; review the implementation before merge." | `/impl-review` | stage 9 (advisory) |
| A10 | "I need to change a PRD that's already approved and frozen." | `/prd-amend` | cross-cutting |
| A11 | "Let me annotate this doc in a browser before its PR." | no browser-annotation skill; offer operator review of the source document | current review workflow in AGENTS.md |
| A12 | "Here's a whole problem — drive it through the pipeline for me." | `/argus` | tooling |
| A13 | "Something we built broke; capture the lesson." | `/incident` | stage 12 |
| A14 | "What's installed here / what can this repo do?" | `/help` (catalog) | self (§2) |
| A15 | "I'm brand new — where do I start?" | `/help start` → the map's first entry (`/feature-brainstorm`) | first-steps (§4) |

## Set B — negative (off-domain → honest refusal, `UB-2`)

| # | Ask | Expected | Grounding |
|---|---|---|---|
| B1 | "Fix my SQL query." | refuse+map | no pyxis entry owns ad-hoc coding |
| B2 | "What's the weather today?" | refuse+map | off-domain entirely |
| B3 | "Deploy my app to production." | refuse+map | ops is a vision road-ahead item, not an installed stage/skill |

## Set C — unbuilt / partial stage (`UB-3` — status stated, never a command)

| # | Ask | Expected | Grounding |
|---|---|---|---|
| C1 | "Verify my whole feature end-to-end against every requirement." | status+bridge: stage 10 E2E verification is `⛔` not built; nearest built = per-task verification in stage 8 (`/implement-now`'s drive-to-green) | stage 10 `⛔` |
| C2 | "Run periodic maintenance — find drift and dead code." | status+bridge: stage 11 is `⛔` not built; no command offered | stage 11 `⛔` |
| C3 | "Compile and validate my contract before planning." | status+bridge: stage 6 is `◐` — the mechanical gate (`docgate contract-gate`, run by `plangate precheck`) is built; the LLM-judgment layer is not; name the built mechanical part, flag the unbuilt remainder | stage 6 `◐` |

## Set D — degraded surface (`UB-1` — absence named, nothing fabricated)

| # | Scenario | Expected |
|---|---|---|
| D1 | Bare `/help` on a repo whose `docs/wiki/` is absent | the catalog still lists every skill + stage from the map and frontmatter; any wiki-derived routing detail is noted unavailable; zero entries invented for the missing wiki |
| D2 | Bare `/help` where a skill directory carries a malformed/absent frontmatter | that skill appears by its directory name with its description noted unavailable; no description is fabricated for it |
