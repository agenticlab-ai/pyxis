# /summarize — report this session's own state

This is a **self-report from your own loaded context**, never a parse of any harness's session record.
It is also why the report is better than a transcript could be — you know
your plan, your pending decisions and what you were about to do next, and
a file on disk does not.

## Read-only

`/summarize` observes and reports. It makes **no** git mutation, no code-host
write, and no change to the work in progress — not a commit, not a push, not
a file edit anywhere. If the session is
mid-task it stays exactly where it is: summarizing is not a reason to tidy
up, finish a step, or fix something you noticed while looking.

## Steps

1. **Recover the operator's first message** — the request that started this
   session, in their words. Normally it is in context. If this conversation
   has been compacted and the original wording is gone, say so in the report
   (see §Initial prompt) rather than reconstructing one. You may recover it
   from your harness's own session record if it is available without
   guesswork; if you do, label it as recovered.

2. **Write the five sections** exactly as specified below — same order, same
   headings, no extras. Obey the word limits; count the words.

## The five sections

### Initial prompt

The operator's first message, **verbatim and complete** — no summarizing, no
trimming, no eliding with `[...]`, no fixing their typos, however long it
runs. If it opened with a slash command, include it as typed. If it carried
a pasted handoff prompt or document, reproduce that too. Put it in a fenced
block so its own formatting cannot be read as the report's.

This is the one section that must never be lossy: it is the operator's own
record of what they asked for, and everything else in the report is your
interpretation of it.

If the wording is genuinely unrecoverable, write exactly
`unavailable — this conversation was compacted and the original wording is
no longer in context`, then give the earliest operator wording you still
hold, labelled `paraphrase, not verbatim`.

### Goal

Two sentences: what this session is trying to achieve or solve for — the
objective behind the request, not a restatement of it. If the operator
redirected mid-session, describe where the goal stands **now**, not where it
started.

### What is done

Fully resolved parts, one per line, **25 words maximum each**.

Done means it needs no further action from anyone: landed, verified, closed.
Code you wrote but never reviewed is not done. A pull request that is open —
however green its gates — is not done; it belongs under remaining until it
merges. If you cannot confirm something landed, it is not done.

`nothing yet` is a legitimate and useful answer. Never promote
work-in-progress to fill the section.

### What is remaining

Items already planned for, one per line, in the order they must happen,
**25 words maximum each**.

Only work that is genuinely part of reaching the goal and already decided —
not ideas you had, not improvements you noticed, not scope you would add if
asked. If an item is waiting on the operator, say so inside the item and
name the decision they owe you.

### Next step

One sentence: the single next action, concrete enough to act on without
rereading the rest.

## Report shape

```markdown
# <slug> — session report

Written: <YYYY-MM-DD HH:MM> · CLI: <harness> · Repo: <repo> · Branch: <branch>

## Initial prompt

> <the operator's first message, verbatim and complete>

## Goal

<two sentences>

## What is done

- <=25 words>
- <=25 words>

## What is remaining

- <=25 words>
- <=25 words>

## Next step

<one sentence>
```

## Never

- Never paraphrase, trim or tidy the initial prompt, and never present a
  reconstruction as verbatim.
- Never list as done anything you have not confirmed actually landed.
- Never invent remaining work that was never planned for.
- Never exceed 25 words in a done or remaining item, or two sentences in the
  goal.
- Never mutate the repo, git, or the code host — including the work in
  progress you are reporting on.
