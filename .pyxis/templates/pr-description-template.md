# Pull request description — template

*Version: `pr-description-template/v1`. Every pull request carries this
description, whoever opens it: an agent session, a skill, or the executor,
which fills it from the implementing agent's report and computes the line
count and complexity itself. Where the hosted check runs (in Pyxis's own
repository, the `pr-description` check inside the `pull-request` job), it
fails a pull request whose description misses it. The pull requests
`pyxis draft` opens for a contract draft or lock keep the approval view the
feature-draft contract defines.*

Copy the skeleton, replace every placeholder, and keep the sections in this
order. Sections of your own may follow `## Testing`.

````markdown
## Human Reviewer data
- Lines of code: <total> (+<added>/-<removed>)
- Complexity: <low | medium | high>
- Blast radius: <low | medium | high>
- Decision type: <one-way | two-way>
- Review notes: <where you want feedback; known limitations and edge cases; what you are unsure about>

## What
- <the net change, naming the components, commands, and functions it touches>
- Closes #<issue>

## Why
<the problem, bug, or need in plain language, with a link to its tracking item>

## How
- <the approach at the architecture level: key trade-offs, refactors, and the alternatives you rejected>

## Testing
- [x] <a check you ran, and its result>
- [ ] <a check you did not run, and why>
````

## Human Reviewer data

The reviewer reads this block first, to decide how to spend the review.

- **Lines of code** — the lines the change adds and removes in production
  code and agent prompts (AGENTS.md and the authoring guides, skills, and
  templates under `.pyxis/`), against the merge base. Tests, fixtures,
  comments, blank lines, generated and vendored files, and documents (PRDs,
  technical designs, plans, reviews, memory records) do not count. Never
  estimate it: run `go run ./cmd/pyxis pr-description lines` and paste what
  it prints. A change that touches no code or prompt file says
  `n/a (no code or prompt changes)`. Recount after every push: the hosted
  check compares this figure with the pull request's diff.
- **Complexity** — how hard the change is to reason about. For a planned or
  issue task it comes from the task's routing assessment: an implementation
  capability of economy or standard is `low`, strong is `medium`, and
  frontier is `high`. Otherwise judge it on the same scale: `low` for
  routine work in a known pattern, `medium` for several interacting parts or
  an adapted pattern, `high` for difficult reasoning, a change across
  boundaries, or the first of its kind.
- **Blast radius** — what breaks, and how badly, if the change is wrong,
  given how many places it touches. `low`: one feature or command, and a
  failure is visible and local. `medium`: several surfaces or a shared
  component, and a failure degrades a workflow. `high`: the system core,
  persisted state, security, or every task, and a failure corrupts data,
  blocks the pipeline, or goes unnoticed.
- **Decision type** — `two-way` when the decision can be reversed later
  without rewriting a large part of the system; `one-way` when reversing it
  is costly: a persisted format or schema, an interface or contract other
  code depends on, a data migration, or a published or frozen record.
- **Review notes** — where you want feedback, the known limitations and edge
  cases (and why you accepted them), and anything you are unsure about.
  Continue on indented lines when one line is not enough.

A value may carry a short note after it: `medium (touches the shared publish
path)` or `two-way — the format is versioned`.

## What, Why, How, Testing

- **What** — the net change at a high level, so a reviewer knows the scope
  without reading every file. Name the components, commands, screens, or
  functions; never "fixed stuff". End with `Closes #<issue>` when the change
  completes a tracked issue.
- **Why** — the problem, bug, or business need. Link the tracking item, and
  always summarize it in plain language so nobody has to read an external
  spec to follow it.
- **How** — a brief architectural overview: the approach, the key
  trade-offs and refactors, and the alternatives you considered and
  rejected. Not a line-by-line walkthrough.
- **Testing** — the exact manual steps, the unit and integration tests added
  or updated, and the edge cases checked. Mark each check you ran `- [x]`
  with its result, and each one you did not run `- [ ]` with the reason.

## The rules the check enforces

`go run ./cmd/pyxis pr-description check --body <file>` runs the same checks
before you open or edit the pull request.

- The description starts with `## Human Reviewer data`, followed by
  `## What`, `## Why`, `## How`, and `## Testing`, each once and in that
  order.
- Human Reviewer data holds the five fields as `- <Field>: <value>` bullets,
  in the order above.
- Lines of code matches the pull request's diff, or is `n/a` when the change
  touches no code or prompt file.
- Complexity and Blast radius start with `low`, `medium`, or `high`; Decision
  type starts with `one-way` or `two-way`.
- No field or section is empty, and Testing lists at least one `- [x]` or
  `- [ ]` item.

## Worked example

````markdown
## Human Reviewer data
- Lines of code: 139 (+101/-38)
- Complexity: low
- Blast radius: medium
- Decision type: two-way
- Review notes: Concurrent requests now wait for one shared token refresh. Two refreshes that start in the same instant in separate tabs can still both run; we accepted that because it is rare and preventing it needs a lock across tabs. Please check how `refreshToken()` reports a failed refresh to the user.

## What
- Serialize the token refresh in `authService.ts`, so an expired token no longer starts a redirect loop.
- Closes #482

## Why
When an access token expired during a burst of requests, every failing request started its own refresh. The refreshes raced, a 401 redirect loop followed, and the user was logged out.

## How
- One shared refresh promise: concurrent requests wait for it instead of starting their own.
- Rejected: retrying each request with backoff, which hides the race instead of removing it.

## Testing
- [x] `npm test authService.test.ts`, including a new race test with expired tokens: passed.
- [x] Revoked a token in staging during a burst of requests: one refresh, no redirect loop.
- [ ] Mobile clients: not run; they use a separate client library.
````
