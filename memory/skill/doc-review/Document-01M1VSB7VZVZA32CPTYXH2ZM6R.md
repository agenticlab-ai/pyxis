---
name: doc-review
description: Open a doc or artifact in a rich, browser-based, inline-annotatable review surface — the vendored lavish editor — and drive the live annotate→apply→reply loop with the reviewer. Use whenever a human should REVIEW something substantial you or they wrote — a design doc, a plan, a spec, a comparison, a report — especially any `docs/**/*.md` under discussion ("review this doc", "doc review", "design review", "open this in lavish", "let me annotate"). Renders markdown via the codified render-doc.mjs transform (never hand-authored HTML for docs), serves it through the VENDORED lavish-axi (`node .pyxis/vendor/lavish-axi/dist/cli.mjs` — never npx), polls the reviewer's inline annotations, applies each to the source doc by propose-and-confirm, and replies on the page. Makes ZERO GitHub writes and ZERO git mutations of its own. Degrades gracefully (static-open or render-only, reason logged) when the live layer is unavailable — never a silent no-op.
---

The **doc-review loop** for pyxis. A markdown design doc is a slog as raw
terminal text; doc-review renders it to a rich, *annotatable* HTML surface (the
vendored **lavish** editor), lets the reviewer drop inline comments in the
browser, and hands each comment back to you to apply to the source — turning a
skim into a real review. Everything here was distilled from harbormaster's
design-review/self-host integration and its incident history; the load-bearing
lessons are marked inline.

DO NOT MISS A STEP, and respect the two load-bearing invariants:

1. **ZERO GitHub writes and ZERO git mutations of your own.** This skill never
   runs `gh` (no PR/issue/label/comment), never `git commit`, never
   `git branch`/`checkout`/`switch`. It edits only the one target doc the
   reviewer's confirmed annotations demand, and writes scratch HTML to the
   gitignored `.lavish/`. Landing the edits is a separate, later step on the
   current branch.
2. **Fail soft, never a silent no-op.** When the live layer is unavailable, the
   driver probes the sandbox and degrades to static-open (auto-open the render,
   view-only) or render-only (print the path), logging the reason. The human
   never has to run a command.

## 1. Resolve the input

Two input shapes:

- **A markdown doc** (the common case — a design doc, plan, spec, README):
  resolve it from the repo root, confirm it exists, and **read it** — you hold
  its content for the comment→edit judgment later.
- **An ad-hoc artifact** you are presenting (a comparison, a diagram, a report
  that exists only in this conversation): author it as a standalone HTML file
  directly into `.lavish/<slug>.html` and skip §2. Authoring HTML is allowed
  here because the HTML *is* the artifact — for a markdown doc it is forbidden
  (§2).

```bash
ROOT="$(git rev-parse --show-toplevel)"
DOC="$ROOT/<path-to>.md"
[ -f "$DOC" ] || { echo "no such doc"; exit 0; }
SLUG="$(basename "$DOC" .md)"
```

Do **not** create or switch branches; whatever branch the clone is on is where
any resulting edits will later be committed. Note it for the handoff but never
change it.

## 2. Render — codified transform, never LLM-authored HTML for docs

The render is the committed Node script — do **not** hand-write HTML for a
markdown doc; that would be non-deterministic and spend tokens on a mechanical
transform. Write the scratch render into the gitignored `.lavish/`:

```bash
MJS="$ROOT/.pyxis/scripts/render-doc.mjs"
OUT="$ROOT/.lavish/$SLUG.html"
mkdir -p "$ROOT/.lavish"
node "$MJS" "$DOC" "$OUT"      # exit 2 = usage/unreadable; 0 = ok
```

The transform never fails to render (fail-open): design-doc house conventions
(metadata table, Decision/Acceptance callouts, risks table, Principles grid)
render richly; fenced code blocks render as source blocks; ` ```mermaid `
fences draw as diagrams — the pinned `.pyxis/vendor/mermaid/mermaid.min.js` is
inlined into the page at render time, so the HTML stays standalone + offline;
anything else renders as generic markdown. If the vendored mermaid bundle is
missing, mermaid fences degrade to labelled source blocks and the remedy is
`bash .pyxis/vendor/mermaid/refresh.sh` — relay it and continue; the render itself
never fails for it.

When the doc is a **technical design** (`docs/tech-design/<slug>.md`) that has a
paired **solution-shape** (`docs/solution-shapes/<slug>.md`, same slug), the
transform composes both onto **one** review page — the design's decisions above,
the shape's concrete schema/interfaces below — so the reviewer sees both in a
single review (solution-shape-v1 §3.7). No extra flag: just render the design.
A design with no paired shape renders exactly as before.

**If `node` is missing, or the render fails**, the live layer is unavailable —
say so explicitly and go to §5's degradation path. Do not abort the whole
skill; render-or-degrade is the contract.

## 3. Open + annotate via the driver — the self-host loop

Serving, inline annotation, and the feedback channel are **the vendored
lavish-axi, reused verbatim** — author no viewer, no annotation UI, no server.
The driver (`.pyxis/scripts/lavish-driver.sh`) node-launches it headless and
drives the `/api` loop over curl — no `npx` (remote fetch+exec is what sandboxes
block; local node + localhost HTTP is allowed), zero user commands:

```bash
DRV="$ROOT/.pyxis/scripts/lavish-driver.sh"
"$DRV" open "$OUT"           # probe → launch headless + register + open browser (or degrade — §5)
```

First run on a fresh clone: the vendored runtime deps must be installed once via
`bash .pyxis/scripts/setup-lavish.sh` (npm ci --omit=dev, lockfile-pinned). The probe
detects a deps-less vendor tree and names exactly that remedy — relay it to the
user, then continue degraded rather than failing.

**The poll must stay continuously in flight until the reviewer ends — zero user
prompting.** lavish-axi shows the agent as "listening" only while a poll request
is actually in flight; any gap flickers the UI to "Your agent is not listening"
and queued annotations sit undelivered (harbormaster #889). The driver's
`listen` verb owns that liveness loop **in bash** — it re-arms bounded
long-polls internally (re-arm gap ~ms) and returns only on a real event, so you
are woken only when there is judgment work. **Do not** drive one-shot `poll`
or `watch` yourself — run `listen` as a **background task**:

```bash
"$DRV" listen "$OUT"         # the listening loop; run as a BACKGROUND task.
#   exit 0 → feedback: ONE payload on stdout — go to §4 (apply, propose-and-confirm),
#            then `reply`, then relaunch `listen`.
#   exit 3 → ended: the reviewer is done — go to the §6 handoff.
#   exit 2 → idle cap (~16 min of silence, LAVISH_LISTEN_IDLE_MAX): relaunch `listen`;
#            after ~2 consecutive caps, ask the reviewer (via `reply` and in chat)
#            whether to end, and `end` if so — an abandoned session must not poll forever.
#   exit 1 → transport error (server gone): stop / degrade.
```

The harness re-invokes you when the background task exits. On exit 0 the payload
(the reviewer's inline annotation) is on stdout — apply it (§4), reply, and
relaunch `listen`:

```bash
"$DRV" reply "$OUT" "<the proposed edit, or a clarifying question>"
"$DRV" listen "$OUT" &       # re-arm — bash keeps listening until `end`
```

(`watch` remains the single-arm primitive `listen` composes — one bounded poll
per call, exit 2 = waiting; the tests drive it directly.)

When `listen` returns exit 3 (the reviewer signalled done), stop relaunching and
`end` the session — `end` also stops the headless server cleanly:

```bash
"$DRV" end "$OUT"
```

## 4. Comments become doc edits — propose-and-confirm

Applying a review comment to the source is a judgment you make **once, here, at
the live session** — you hold the comment, the doc, and the rendered context.
For each annotation:

- **Clear, actionable comment** (e.g. "tighten §3 to name the artifact") →
  **propose** the exact edit back through `"$DRV" reply`, and apply it to
  `$DOC` **only after the reviewer confirms**. Preserve the doc's structure
  (headings, tables, callout conventions) so the re-render still parses it.
- **Ambiguous comment** → surface a **clarifying question** via `"$DRV" reply`;
  **never** guess a silent edit. Apply nothing until the reviewer disambiguates.

**On confirm, capture the annotation to the ledger.** After the reviewer
confirms an annotation's disposition (an applied edit *or* a decline), invoke
the confirm-time writer **once** so the annotation persists as a review-feedback
ledger entry — the paid-for review signal the skill today discards
(`docs/tech-design/review-feedback-capture.md` §3.3). The snippet only *locates
the front door and invokes* the coded writer; this skill gains no capture logic:

```bash
# Locate the front door — the pinned `pyxis` binary on PATH in an installed
# operand, else the source tree via `go run` (no per-tool resolver named, so the
# operand payload stays clean of the retired shims). Fails open either way.
if command -v pyxis >/dev/null 2>&1; then
  printf '%s' "$COMMENT" | pyxis exec feedback-capture -- --author="$AUTHOR" --subject-anchor="$SLUG#$LOC"
elif command -v go >/dev/null 2>&1; then
  printf '%s' "$COMMENT" | ( cd "$ROOT" && go run ./cmd/pyxis exec feedback-capture -- --author="$AUTHOR" --subject-anchor="$SLUG#$LOC" )
fi
```

Capture is **off the review critical path and fails open** — the shim never
blocks the loop and never loses the comment (a failed write is queued for
retry), so run it and continue regardless of its result. Do this per confirmed
annotation, independent of whether it produced an edit.

After applying confirmed edits, **re-render** (re-run §2) and `"$DRV" reply` so
the reviewer sees the updated surface. A broken edit shows up immediately
because the re-render re-parses the doc — that is the in-loop safety net.

Edits are confined to the one target doc and the gitignored `.lavish/` scratch —
the clone is never mutated otherwise.

## 5. Runtime degradation — the driver probes + degrades

When the live layer is unavailable — node missing, vendored lavish-axi absent,
its deps not installed, no background server, no localhost — `"$DRV" open` does
**not** fail or no-op. It probes and selects the richest available mode, logging
the chosen mode and the reason:

```
full        → launch headless + register + open browser + drive the /api loop (zero user commands)
static-open → auto-open the rendered file in a browser, view-only (zero user commands)
render-only → print the artifact path to open by hand
```

The rendered HTML opens standalone in any browser with no server — portability
is preserved. Relay the logged reason and its remedy (usually
`bash .pyxis/scripts/setup-lavish.sh`) to the user, exit cleanly, and never invent a
degraded "guess at edits" mode — without the annotate loop there are no
annotations to apply.

## 6. Wrap up

When the session ends, summarize plainly:

```
Reviewed <doc> — <n> annotation(s) applied as confirmed edit(s), <m> raised as clarifying question(s).
Render: .lavish/<slug>.html (gitignored scratch). No GitHub writes, no commit, no branch change.
The edits sit uncommitted on the current branch — commit/PR them as a separate step when ready.
```

If you ran the degraded path, say so; if there are no edits, stop after printing
the render path and the enablement remedy — there is nothing to land.

`git status` after a session shows only the intended edits to the target doc
(the `.lavish/` render is gitignored); no branch was created or switched; no
PR/issue/label exists from this skill.

## Appendix — the machinery (for maintenance, not per-run reading)

Naming: **doc-review** is the capability (this skill, the `/doc-review`
invocation); **lavish** is the tool layer it drives — the vendored editor, its
driver, setup script, scratch dir, and `LAVISH_*` env knobs keep the tool's
real name.

- **Vendored runtime**: `.pyxis/vendor/lavish-axi/` — un-bundled upstream `dist/` +
  `package.json` + committed `package-lock.json`; deps install once via
  `.pyxis/scripts/setup-lavish.sh`. NEVER esbuild-bundle it and NEVER `npx` it — see
  `.pyxis/vendor/lavish-axi/README.md` for the three-breakage history behind this.
- **Version pin**: `LAVISH_AXI_VERSION` in `.pyxis/scripts/versions.env`; bump + re-vendor
  via `.pyxis/vendor/lavish-axi/refresh.sh`, review the diff content (path leaks,
  non-determinism), then re-run the contract test.
- **Vendored diagram bundle**: `.pyxis/vendor/mermaid/` — the pinned upstream-built
  `mermaid.min.js` that `render-doc.mjs` inlines when a doc contains
  ` ```mermaid ` fences (pin: `MERMAID_VERSION` in `.pyxis/scripts/versions.env`;
  bump + re-vendor via `.pyxis/vendor/mermaid/refresh.sh`).
- **Tests**: `.pyxis/scripts/tests/test-lavish-driver.sh` (driver loop, bash-3.2 floor,
  no-npx guarantee), `.pyxis/scripts/tests/test-lavish-api-contract.sh` (pins the
  /api wire shapes against the vendored cli — fails on contract drift at a
  version bump), and `.pyxis/scripts/tests/test-render-doc.mjs` (the render contract:
  fences, mermaid inlining + degradation, house conventions, vendor pin). Run
  them after touching the driver, the renderer, or a vendor tree.
- **Bash floor**: the driver and tests must stay bash-3.2-clean (macOS stock
  bash); Linux `bash -n` will not catch bash-4-only syntax.
