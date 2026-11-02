# e2e-design — author a feature's e2e-design/v2 doc

Author the **design** half of the e2e authoring line: read one
fully-implemented feature's **PRD requirement rows** (the coverage
backbone), its **live technical-design set**, and its **built code**, and
write `docs/e2e/<feature>/design.md` — an `e2e-design/v2` document whose
coverage map carries every required behavior, whose every scenario carries
exactly one route with cost class and cadence, is restated in the harness's
own terms with fixture, seams, and owned artifact, and whose harness is
chosen for that feature's own primary interface or cites an existing one
(`docs/tech-design/e2e-verification-skills-v2.md` §3.1;
`docs/tech-design/e2e-verification-artifacts-v2.md` §3.1). Input: the feature
slug; ask if missing.

The **mechanical half is already coded and tested** at
`internal/e2e/design`, driven by `cmd/e2e-design` — the input-gating
refusals, the live-design-set resolution, the requirement extraction, the
binding-element extraction, the named-module scan, the bounded read set,
and the no-automatable-surface predicate. Run it; do not re-derive its work
by reading the tree yourself. The
**judgment half is this guide's**: which scenarios prove which behavior,
what is mocked, which harness fits, and how each scenario is routed. That
split is the point — a new app shape must need **no code change**
(`#DINV-2`), so every app-shape choice is authored *content* in the doc you
write, never a branch in skill code.

## Steps

0. **Gate the inputs — one command, before anything else is read.**
   e2e-design is invoked on a feature whose implementation tasks have **all
   landed** — an end-to-end test needs the whole integrated feature. Gather
   the feature's own changed files (the same list step 1 needs below — e.g.
   the diff of the feature's merged work against its base branch), then pipe
   a request to `go run ./cmd/e2e-design gate`:

   ```json
   {
     "feature": "<feature-slug>",
     "prd": "docs/prds/<feature>/effective.md",
     "changed": ["<the files the feature added or modified, still present>"],
     "deleted": ["<the files the feature removed or renamed away>"]
   }
   ```

   **Split the diff.** `changed` is the paths that must be present now;
   `deleted` is the paths the feature's own work removed. A plain
   `git diff --name-only` mixes the two, and a removal listed under `changed`
   would be read as work that never landed — refusing a feature that is in
   fact complete. Split them at the source, e.g.
   `git diff --name-only --diff-filter=d <base>...HEAD` for `changed` and
   `--diff-filter=D` for `deleted`.

   **Claim only what you actually gathered.** A blank entry, a directory, or
   an unrelated file is not evidence the feature was built, and `gate` reads
   it as none — an empty `git diff` line-splits to `[""]`, which claims
   nothing and is refused. The gate cannot verify the claim is honest (it has
   no independent landing signal), so the claim's accuracy is yours to get
   right; do not pad it to get past the check.

   The index is **not** a request field: `gate` always reads docgate's
   generated index under the configured technical-design corpus, because
   liveness is bound to the generated artifact CI keeps fresh (`#DINV-8`). A
   request naming `index` is rejected.

   It checks, **in order, refusing at the first that fails** — prints the
   reason to stderr, exits non-zero, writes nothing:
   1. the PRD is missing or unreadable (`prd:e2e-verification/UB-2`);
   2. the generated technical-design index is missing, unreadable, or of a
      generation this reader does not parse (its `schema_version` is pinned,
      so a reshaped index is named as such rather than surfacing later as
      false ambiguity), or **no live design serves the feature** at all
      (`UB-2`);
   3. the feature's **live design set is ambiguous** — a superseded design
      with no clear live successor, or a status the index could not resolve
      (`#DINV-8`). **Never** refused merely because the resolved set has
      **more than one** live design — that is read as a whole set, not
      ambiguity, and this very feature is the canonical example
      (`e2e-verification-skills` + `e2e-verification-artifacts`, both live);
   4. a resolved live design's own file is missing or unreadable (`UB-2`);
   5. the claimed built code does not evidence a landed feature — nothing
      named at all (including a list of only blanks), a `changed` path that
      is absent or is not a regular file, or only removals with nothing
      added or modified left to read (`prd:e2e-verification/UB-1`).

   **A `gate` refusal means stop, right there.** Report the exact reason it
   printed to the operator, naming the input or the unlanded work — write
   nothing, run no further step, produce zero e2e documents (`#DINV-1`).
   Never resolve an ambiguous live design set yourself, and never author the
   doc over an input you had to guess or invent. On success `gate` prints
   the feature's resolved **live design set** as JSON (`{"designs":
   [...]}`) — however many designs, in one call, with no separate lookup —
   and that list is step 1's `designs`.

1. **Assemble the authoring context — one command, deterministic.** Pipe a
   request to `go run ./cmd/e2e-design context`, using `gate`'s resolved
   `designs` list from step 0 and the same `changed` list:

   ```json
   {
     "schema": "e2e-design-context/v1",
     "prd": "docs/prds/<feature>/effective.md",
     "designs": ["<gate's resolved design paths>"],
     "changed": ["<the feature's own changed files — the same list gate saw>"]
   }
   ```

   It returns the PRD-anchored `requirements` (FR-n · UB-n · NFR-n rows,
   each already gisted to a behavior statement), the design set's
   `bindingElements` (every `**Decision:**`, `DINV-n` and `A-n`, tagged with
   the design it came from), and the bounded `readSet`. The request parses
   **closed** — an unknown key is an error, not a silently-empty design set.

   **The read set is the bound, and it is a stop, not a suggestion**
   (`prd:e2e-verification/NFR-3`). Read the files it lists and no others. If
   the response reports `truncated`, the run **reached the bound**: stop and
   tell the operator the coverage would rest on a partial read, rather than
   fanning out across the tree. The excluded paths are deliberately not
   returned — a truncated run is not continued by reading the leftovers.

2. **Derive the required-behavior list.** The PRD rows are the **backbone** —
   every returned requirement is a candidate required behavior. **Top it up**
   from the design set's binding elements and from what the built code
   actually does: a `DINV-n` the PRD states no row for is still a required
   behavior. Do not narrow the backbone by judgment; a row you believe is not
   e2e-provable still appears in the coverage map, with a routed disposition
   (step 5).

   If this list comes back **empty** — no PRD row, no topped-up binding
   element, and nothing in the built code implies an end-to-end behavior —
   there is nothing to write a scenario about. Skip steps 3–6 entirely and
   go straight to step 7 with an empty `routes` list.

3. **Write the Scenarios** — the end-to-end flows the tests will exercise,
   one row per scenario, each with a **stable ID** (`S1`, `S2`, …) the
   Coverage map, the Scenario routes table and the Scenario build facts
   table all reference. A scenario is a flow
   through the built feature, not a restatement of a requirement: several
   requirements may ride one scenario, and one requirement may need several.
   Every scenario row carries an ID — a blank ID cell leaves nothing for a
   coverage row to cite and nothing for the plan to name its task after.

4. **Reason about the feature's primary external interface, then judge for
   yourself whether a harness that can already drive it exists**
   (`prd:e2e-verification/FR-6`, `FR-19`, `FR-20`, `NFR-1`, `#DINV-2`,
   `#DINV-9`). Ask first what a caller actually drives this feature through —
   an HTTP endpoint, a command-line invocation with its argv and stdout, a
   library's public API, a file the feature emits.

   Then go and look. List the test trees inside your bounded read set:

   ```
   go run ./cmd/e2e-design harness-candidates < request.json
   ```

   That verb names what EXISTS and nothing more — it never opens a test file
   and holds no opinion about what any tree drives. **The judgment is
   yours**: read the candidates that look plausible and decide whether any
   of them can drive *this* feature's interface. There is no harness
   registry to consult and no path convention to test — a directory existing
   is not a harness that fits, and a harness that fits perfectly would be
   invisible to a rule that only looks where it expects (§3.1). If the scan
   reports that it stopped at its bound, narrow the read set and look again;
   never read a truncated list as "no harness exists".

   Your judgment picks the section's form, and only one of the two is
   written:

   - **A fitting harness found → the reuse form.** Cite it by path — a
     citation a reviewer can check — and list **only this feature's own**
     fixtures, fakes and helpers. Re-decide none of its mechanics: the reuse
     form carries no slot rows at all, because re-answering a slot the cited
     harness already answers is either not reuse or a silent fork of it.
   - **None found → the build form.** Answer **every** slot of the closed
     vocabulary yourself, with mechanics a worker could build from. A slot
     you genuinely cannot settle is a **blocking open item** naming what is
     unresolved and why (`UB-8`, `#DINV-8`) — never a library or tool name
     standing in for missing mechanics.

     **Write it exactly like this**, in the slot's Answer cell:

     ```
     open item — <what is unresolved, and why it cannot be settled>
     ```

     The literal `open item` must be the **first thing in the cell**, spelled
     exactly that way. That leading token is what the class recognises and
     what the approval hold keys on. Any other phrasing — `blocking open
     item — …`, `open-item — …`, `unresolved — …`, or the marker placed after
     a partial answer — reads to the checker as an ordinary slot answer, and
     the document then becomes **approvable with the element still open**,
     which is the one outcome this form exists to prevent. Nothing warns you:
     the checker calls every one of those near-misses well-formed.

     The **reason after the marker is yours to enforce**. The checker does not
     read it for a harness slot, so a bare `open item` — or `open item —
     testcontainers` — passes the checklist. A hold that names nothing tells
     the operator and the buildability lens nothing; it is not a hold, it is a
     gap wearing a label. State what is unresolved and why it cannot be
     settled.

     **One slot cannot take an open item.** `decision-authority-stand-in`
     carries its own pre-write check (the `stand-in` verb in step 4 below),
     which refuses an open item and admits only the five `FR-22` properties or
     a `not needed — <why>`. If the exercised flow genuinely does wait on a
     human decision and you cannot specify the stand-in, do **not** write
     `not needed` — that is a false claim which both that verb and the
     checker accept, and it is exactly the manufactured green `FR-22` and
     `ABUSE-5` exist to stop. Stop and put it to the operator, the same as a
     `gate` or `surface` refusal.

   Check the form before writing it into the document:

   ```
   go run ./cmd/e2e-design harness-form < form.json
   ```

   When answering the `cost-controls` slot in build form, state the per-run
   bound in **tokens and session count** — never money or wall-clock time.
   This is what lets a planner and a reviewer derive a scheduled run's cost
   from the frozen document alone (`prd:e2e-verification/NFR-5`,
   `artifacts-v2` §3.2).

   Write the **Mock strategy** the same way: name the seam and what stands
   in for it, never "mocks as needed" — an unnamed mock is exactly what the
   anti-gaming lens exists to catch. Write **Setup and teardown**: what each
   scenario creates, what it tears down, and what state (if any) crosses
   scenarios and why that is safe.

   All of this reasoning is **content, not code**. There is no shape
   enumeration to extend and no per-shape branch to add — a feature whose
   shape nobody anticipated is handled by writing about it here. The
   mechanical half deliberately knows nothing about app shapes, frameworks
   or confidence scores: a table of those is blind to the shape it has not
   heard of, and teaching it one would be a code change — exactly what
   `#DINV-9` forbids.

   **Answer the `decision-authority-stand-in` slot — the conditional one.**
   Ask one question of the flow your scenarios exercise: **does anything in
   it wait on a human decision?** An approval, a judgment call, a free-text
   answer, a sign-off — anything a person supplies that the run cannot
   compute. Ask it of the flow *under test*, not of the feature's wider
   story: a feature with a human somewhere in its life whose exercised path
   asks nobody anything needs no stand-in.

   - **Nothing waits on a human → state a not-needed reason.** Write
     `not needed — <why>`, and make the why checkable: what supplies each
     input, and why no branch turns on a judgment. `not needed` on its own
     is a claim a reviewer cannot check, and is raised as a defect.
   - **Something does → specify the stand-in**, with all five properties
     `prd:e2e-verification/FR-22` fixes, as `;`-separated clauses in the
     slot's own cell:

     ```
     scripted-answers: <the answer set consulted first, and where it lives>;
     persona-fallback: <the persona, and the written character sheet it is bound to>;
     improvisation-record: recorded — <where a fallback answer is written, and how its assertions are reported non-authoritative>;
     adversarial-persona: <one or more of silence, contradiction, refusal> — <what each does>;
     assertion-blindness: blind — <what the stand-in is handed, and what it is not>
     ```

     The last three carry a **token**, not a sentence, and the prose after
     the em-dash explains it. That is deliberate: those three are exactly
     where a stand-in stops being a test and starts manufacturing a green,
     and prose can deny a property using the same words that state it — "no
     adversarial persona is scripted" reads as adversarial to anything
     scanning for the word. A token has no polarity to invert.

   Three shapes are **defects, not styles**, and each is raised by name:

   - **cooperative-only** — no adversarial persona at all. A stand-in that
     agrees with everything proves only that the harness works when the
     authority agrees, which is the case that was never in doubt. At least
     one persona must stay silent, contradict, or refuse.
   - **assertion-sighted** — the stand-in can see what the run is about to
     assert. It will answer to match, and the green is one it manufactured.
   - **improvising unrecorded** — a fallback answer is used and left no
     trace. An assertion resting on an answer the stand-in invented then
     reads exactly like one resting on the feature's real behavior.

   Check the slot before writing it into the document:

   ```
   go run ./cmd/e2e-design stand-in < standin.json
   ```

   **Specifying** the stand-in is this stage's whole job. Building one or
   running one is the deferred follow-on (`NG-1`, `WR-1`) — do not write
   stand-in code here, and do not let the slot promise a component nobody
   has been asked to build.

5. **Map coverage — every required behavior, no silent drops.** Each behavior
   from step 2 gets a Coverage map row citing the scenario(s) that prove it,
   **or** an explicit `open item — <reason>` disposition. An individual
   behavior you cannot route — not CI-runnable, not sensibly mockable, not
   cleanly manual — is surfaced as an open item and **never dropped**, never
   silently folded into a neighbouring row (`prd:e2e-verification/UB-3`,
   `#DINV-7`). A behavior no scenario proves and no disposition explains is
   the one defect this document exists to make visible.

6. **Route every scenario and declare its cost.** Each scenario named in step 3
   gets exactly one row in Scenario routes carrying exactly one of
   `run-for-real` · `mocked` · `manual-human` (`prd:e2e-verification/FR-4`).
   A `manual-human` scenario **names its human-judgment Owner**; every other
   route's Owner column reads `—`. Every scenario also carries exactly one
   cost class — `deterministic` · `bounded-model` · `full-session` — and
   exactly one cadence — `per-change` · `scheduled` · `checkpoint` ·
   `on-demand` — so a planner can read one scheduled run's cost directly from
   the document (`FR-23`, `NFR-5`). A behavior that cannot run in CI appears
   routed `mocked` or `manual-human` — never absent.

   **Restate each scenario in the harness's own terms:** in the Scenario build
   facts table, name the fixture each scenario requires, the seam(s) it
   exercises, and the **one artifact this scenario exclusively owns** under the
   harness's scenario contract (`prd:e2e-verification/FR-21`). Every seam a
   scenario names must be one the **harness section** carries — in the build
   form, a seam named in its `seams` slot; in the reuse form, a seam of the
   harness it cites, which carries no slot rows of its own (step 4). A seam the
   harness section does not carry, in either form, is a defect. No two
   scenarios name the same artifact — the checker detects it — and the planner
   reads these owned artifacts to assign non-colliding parallel work.

7. **Check for an automatable surface — before writing anything**
   (`prd:e2e-verification/UB-7`, `#DINV-7`). Collect the route token you
   assigned each scenario in step 6 — an **empty** list if step 2's
   required-behavior derivation found nothing to test at all, so no
   scenario was ever written — and pipe it to
   `go run ./cmd/e2e-design surface`:

   ```json
   {"feature": "<feature-slug>", "routes": ["<one route token per scenario from step 6>"]}
   ```

   It refuses (exit 1, a reason on stderr) when `routes` is **empty** (the
   feature is pure-doc/refactor, or every required behavior turned out an
   individually unroutable open item — nothing at all to build or mock) or
   **every entry is `manual-human`** (every required behavior routes to a
   human) — the **no-automatable-surface stop**. A single `run-for-real` or
   `mocked` route anywhere in the list is enough to pass: this check never
   inspects an individual behavior's own `open item` disposition from
   step 5, which stays exactly that — a routed disposition inside an
   otherwise-automatable feature (`prd:e2e-verification/UB-3`), never this
   stop.

   **A `surface` refusal means stop, right there — the same as a `gate`
   refusal in step 0.** Discard whatever Scenarios / Coverage map /
   Scenario routes draft you were assembling: write **no e2e document**,
   skip step 8 entirely, produce zero e2e documents. The halt is over
   **authoring and everything downstream** — it is not a halt over your own
   bookkeeping: **step 9 still runs**, and records this stop as
   `refused` (a stop nobody records is invisible to the very measurement it
   should appear in).

   Report the reason to the operator, and **name which of the two causes
   applied** — the predicate sees only the shape of the route list, so this
   half is yours:

   - **pure-doc / refactor** — step 2 derived no required behavior at all,
     so no scenario was ever written. Nothing here is testable end-to-end.
   - **every required behavior turned out individually unroutable** — real
     behaviors exist and each one ended up an open item. Say so explicitly
     and **list those behaviors in your report to the operator**: zero
     documents are produced, so this stop is the only place they are ever
     recorded (`prd:e2e-verification/UB-3`, `#DINV-7`'s no-silent-drop half).
   - **all-manual** — the third reason (`every scenario is routed
     manual-human`) names its own cause already; just relay it.

   Then state that the feature has no automatable e2e surface, and that
   **e2e-review and e2e-plan must not run for it**. Never respond to a
   `surface` refusal by writing a manual-only design doc or inventing a
   scenario to get past it — that is exactly the fabrication this stop
   exists to prevent (`#DINV-7`).

   Exit 1 is the stop and only the stop. **Exit 2 is a request fault** —
   malformed JSON, a foreign schema, a blank feature, an omitted `routes`
   key, an unrecognized route token — and is never a verdict about the
   feature: fix the request and re-run. Send `"routes": []` to report that
   no scenario was derived; omitting the key is a fault, not the pure-doc
   case.

8. **Write the doc and check it — check first, emit second.** Instantiate
   `.pyxis/templates/e2e-design-template.md` **at a scratch path** — say
   `.lavish/e2e-design-<feature>.md`, anywhere outside `docs/e2e/` — with all
   seven sections, each present (an empty one reading a stated
   `none — <reason>`), the header's routing pair, its `Basis` join
   (`prd:<feature>`), the governing design revision(s) the coverage was
   derived from, `Status: draft`, and the `Template version` pin to
   `e2e-design/v2`. Delete the template's §0 and Appendix A. Include the
   Scenario routes and Scenario build facts tables with every scenario
   routed, costed, and restated.

   **The scratch path is the point.** `check` reads a file, so the document
   has to exist somewhere before it can be checked; writing it straight to
   `docs/e2e/<feature>/design.md` would mean a malformed draft lands at the
   exact path `/e2e-review` and the operator read, and stays there if the run
   stops before it is clean. `#DINV-12` says e2e-design checks its output
   **before emitting** and never emits a malformed doc — so the emission is
   the move into `docs/e2e/`, and it happens only after a clean check.

   ```
   go run ./cmd/e2e-design check .lavish/e2e-design-<feature>.md
   ```

   It exits 0 on a well-formed doc, or lists each defect at its own rule.

   - **Clean (exit 0)** → move the file into place at
     `docs/e2e/<feature>/design.md`. That move is the emission.
   - **Defects** → fix them at the scratch path and check again.
   - **Defects you cannot settle** → **stop, right there**, the same as a
     `gate` refusal in step 0. Delete the scratch file, write **no** document
     under `docs/e2e/`, and **name the failing rule identities to the
     operator** — the rule name and detail the checker printed, verbatim, so
     the defect is legible to someone who did not watch the run. Produce zero
     e2e documents. **Step 9 still runs** and records the stop as `refused`.

   The checklist is well-formedness only (presence and shape): every section
   present, every slot answered or a blocking open item, every scenario
   routed/costed/restated, no duplicate artifacts — structural properties the
   checker can verify deterministically.

   **Blocking open items — the escape hatch for unsettleable mechanics.** The
   form, the exact marker, the reason you owe after it, and the one slot that
   cannot take an open item are all in **step 4**, where the harness section
   is authored — not here. What matters at this step is the consequence: a
   blocking open item is well-formed and passes the checker **as a draft**, so
   the document reaches the review gate, but a document whose `Status` reads
   `approved` while any slot carries one is malformed and rejected.

   **How the hold lifts.** Two ways, and only two:

   - **The element is settled** — the slot's open item is replaced by real
     mechanics. Re-run `check`.
   - **The dependent scenarios are re-routed so the element is no longer
     owed** — and this is more than a sentence. Identify the scenarios whose
     `Seams exercised` cell names the unsettled element; change their route in
     **Scenario routes**, update their **Scenario build facts** row so it no
     longer names that seam, then re-run `go run ./cmd/e2e-design surface`
     over the revised route list (step 7's stop applies again — re-routing
     every dependent scenario to `manual-human` can leave the feature with no
     automatable surface at all) and re-run `check`. Note that in **build
     form the slot itself is still required**: every slot of the closed
     vocabulary must be present and non-empty whatever the routing, so a
     re-route lifts the hold only when the element is genuinely no longer
     exercised **and** the slot can then carry a real answer. If it cannot,
     the element is not re-routable — settle it, or stop and put it to the
     operator.

   **What the checker does and does not catch.** Its bare-token-run rule is a
   deliberately narrow structural floor — a short run of words with no
   sentence structure — so it catches `testcontainers` but passes
   `testcontainers-go with docker` and `a k6 harness`, which name a tool and
   no mechanics at all (`artifacts-v2` §3.5: a structural proxy, never a
   judgment). Do not read a clean `check` as clearance that a slot specifies a
   machine. Whether coverage is *complete* and whether a slot's answer is
   *sufficient* to build from are the e2e review's coverage and buildability
   lenses, not this checker's. `Status` stays `draft` until the operator
   approves.

9. **Record the line-step event** (`prd:e2e-verification/§5.7` `EVT-1`,
   `e2e_line_step`): `go run ./cmd/e2e-design log-step <feature> design
   <produced|refused> <tokens>` — on every invocation, including a `gate`
   refusal, a `surface` refusal, or any other stop.

## Never

- Carry an app shape in **code** — no shape enumeration, no per-shape branch,
  no harness lookup table, and no framework-detection or confidence score
  standing in for one (naming the tools each shape is tested with is the same
  branch in different words). Every app-shape choice is authored content in
  the doc (`#DINV-2`, `#DINV-9`, `prd:e2e-verification/NFR-1`).
- Write a **reuse form with slot rows**, or a **build form with a slot
  unanswered** — the first re-decides mechanics the citation claims are
  already settled, the second ships a form that specifies no machine
  (`FR-19`, `FR-20`; `harness-form` refuses both).
- Read a **stopped** candidate scan as an exhaustive one — a truncated list
  says where the run stopped looking, never that nothing is there.
- Continue past a `gate` or `surface` refusal, or resolve an ambiguous live
  design set yourself — a missing/unreadable input, an unlanded feature, a
  superseded design with no clear live successor, or a no-automatable-surface
  feature is a stop, never something to guess or write past (`#DINV-1`,
  `#DINV-7`, `#DINV-8`).
- Refuse merely because a feature's live design set has more than one
  design — plurality is read as a whole set, never ambiguity (`#DINV-8`).
- Read past the bounded read set, or continue a run that reported
  `truncated` — the bound is a stop (`prd:e2e-verification/NFR-3`).
- Drop a required behavior. Every one is mapped to a scenario or carries an
  explicit `open item — <reason>` (`prd:e2e-verification/UB-3`, `#DINV-7`).
- Write a manual-only design doc, or fabricate a scenario, to route around a
  `surface` refusal — a no-automatable-surface feature gets zero e2e
  documents and a stop naming the reason, never a degenerate artifact
  (`#DINV-7`, `prd:e2e-verification/UB-7`).
- Give a scenario two routes, no route, or a `manual-human` route with no
  named Owner (`prd:e2e-verification/FR-4`, `FR-5`).
- Invent a requirement, a behavior, or a passing scenario the built feature
  does not have — this doc is derived from the committed PRD, the live design
  set, and the code as built, never from what the feature ought to do.
- Author the doc over an input you had to guess, or mark it `approved`
  yourself — approval is the operator's act, after `/e2e-review`.
- Hand-edit the well-formedness result: fix the document, never the checker.
- Emit a document the checker rejects. Author at a scratch path, check there,
  and move into `docs/e2e/<feature>/design.md` only once `check` is clean; a
  defect you cannot settle deletes the scratch file, names the failing rules
  to the operator, and produces zero e2e documents. Nothing malformed ever
  reaches the class's home, so nothing malformed reaches review.
- Let a near-miss of the open-item marker stand. The approval hold keys on the
  literal `open item` leading the cell; `blocking open item — …` and
  `open-item — …` are silently ordinary answers, and the document becomes
  approvable with the element still open.
