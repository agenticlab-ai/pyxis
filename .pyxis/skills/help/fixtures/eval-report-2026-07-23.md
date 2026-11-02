# help — routing eval report · 2026-07-23

**Run:** first full run of `routing-eval.md`, walking the guide's §3 (route) /
§4 (first steps) / §2 (catalog) procedure against every scenario, with the
expected column re-derived live from `.pyxis/pipeline.md` + every
`.pyxis/skills/*/SKILL.md` frontmatter (runner protocol step 1). Skill under
test: `.pyxis/authoring/help.md` as of T1 (#654).

**Ground-truth self-check:** all 15 canonical expected entries resolve to a
real skill directory or map stage on the live surface; stage statuses used by
Set C match the map (6 `◐`, 10 `⛔`, 11 `⛔`). No fixture row names a
non-existent command.

## Set A — canonical (routing correctness)

| # | Ask (gist) | Expected | Routed to | Result |
|---|---|---|---|---|
| A1 | raw idea, unsettled | `/feature-brainstorm` | `/feature-brainstorm` — map entry rule 1 | PASS |
| A2 | idea already shaped | `/prd-new` | `/prd-new` — map entry rule 2 | PASS |
| A3 | approved PRD, explore design | `/tech-design-brainstorm` | `/tech-design-brainstorm` — stage 3 | PASS |
| A4 | author the design | `/tech-design-new` | `/tech-design-new` — stage 4 | PASS |
| A5 | review a drafted PRD | `/prd-review` | `/prd-review` — stage 2, advisory noted | PASS |
| A6 | review a drafted design | `/tech-design-review` | `/tech-design-review` — stage 5, advisory noted | PASS |
| A7 | decompose the contract | `/plan-new` | `/plan-new` — stage 7 | PASS |
| A8 | build the frozen plan | `/implement-now` | `/implement-now` — stage 8 | PASS |
| A9 | review a ladder-green proposal | `/impl-review` | `/impl-review` — stage 9, advisory noted | PASS |
| A10 | change an approved PRD | `/prd-amend` | `/prd-amend` — cross-cutting | PASS |
| A11 | annotate a doc in a browser | `/doc-review` | `/doc-review` — cross-cutting | PASS |
| A12 | hand over a whole problem | `/argus` | `/argus` — tooling | PASS |
| A13 | capture a lesson | `/incident` | `/incident` — stage 12 | PASS |
| A14 | what's installed | `/help` catalog | the §2 catalog | PASS |
| A15 | brand new, where to start | `/help start` → `/feature-brainstorm` | first-steps → map's first entry | PASS |

**Canonical correct-entry-naming rate: 15/15 (100%).** Meets `A-2` / plan-T3 AC.

## Set B — negative (refusal honesty, `UB-2`)

| # | Ask | Expected | Answer | Result |
|---|---|---|---|---|
| B1 | fix my SQL query | refuse+map | "not a pyxis entry point today" + pointer to `.pyxis/pipeline.md`; no command invented | PASS |
| B2 | what's the weather | refuse+map | off-domain; honest non-entry + map pointer | PASS |
| B3 | deploy to production | refuse+map | ops is a vision road-ahead item, not an installed stage/skill; refused, not routed to an invented deploy command | PASS |

**Fabricated commands/stages in Set B: 0.**

## Set C — unbuilt / partial stage (`UB-3`)

| # | Ask | Expected | Answer | Result |
|---|---|---|---|---|
| C1 | verify end-to-end | status+bridge | "Stage 10 (E2E verification) is `⛔` not built. Per-requirement verification today lives inside stage 8 — `/implement-now` drives each task to green." No stage-10 command offered. | PASS |
| C2 | run maintenance / find drift | status+bridge | "Stage 11 (periodic maintenance) is `⛔` not built." No command offered; pointed at the map. | PASS |
| C3 | compile/validate my contract | status+bridge | "Stage 6 is `◐`: the mechanical gate (`docgate contract-gate`, run by `plangate precheck` as the planning-entry precondition) is built; the LLM-judgment layer is not. Use the mechanical gate; the semantic layer isn't available yet." Named the built part, flagged the unbuilt remainder, offered no command for the missing layer. | PASS |

**Commands recommended for an unbuilt stage: 0. Fabricated stages: 0.**

## Set D — degraded surface (`UB-1`) — sandbox transcripts

**D1 — `docs/wiki/` absent.** The guide's step-4 read (`docs/wiki/README.md`,
on demand) finds nothing; the read set narrows to the map + frontmatter.
Sandbox observation: the catalog still derives all 14 skills + all 12 stage
rows; any wiki-derived routing detail is noted unavailable. Zero entries were
invented for the missing wiki. → PASS

```
step-4 on-demand read target docs/wiki/README.md → absent → no wiki routing headers in the read set
→ catalog still lists all 14 skills + all 12 stage rows from map+frontmatter;
  wiki-derived detail noted unavailable; zero entries invented
```

**D2 — a skill directory with malformed/absent frontmatter.** Sandbox: a
`broken-probe/SKILL.md` with no YAML block. Harvesting its `name`/`description`
yields nothing:

```
harvested name: '<none>'  desc: '<none>'
→ guide's honest output: 'broken-probe (description unavailable)' — never a fabricated one-liner
```

The guide's rule (a skill whose frontmatter is unreadable is shown by its
directory name, description unavailable) holds; no description was fabricated.
→ PASS

## Summary

- **Canonical correct-entry-naming: 15/15 (100%)** — `A-2` / plan-T3 AC met.
- **Fabricated commands or stages across all sets: 0** — `#DINV-2` held; the
  hard-fail condition did not fire.
- **Unbuilt-stage rows offering a command: 0** — `UB-3` held on C1–C3.
- **Degraded scenarios: 2/2** name the absent surface and fabricate nothing —
  `UB-1` held.

Full run **green**. The newcomer-proxy dogfood (`M1`/`DOD-4`) is a separate,
operator-judged exposure checkpoint, not part of this scripted run.
