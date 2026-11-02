# Argus authorship producer — the commit-time stamp (wiring surface)

The **producer** half of attribution
(`docs/tech-design/argus-draft-attribution.md` §3.2). It stamps the
contractual `Argus-Drive: <slug>` authorship trailer — and, alongside it, the
`Argus-Bind: <binding>` in-tree content binding (a credential-free digest over
the commit's own tree+parents) — onto commits made with an active drive slug
at the Git hook surface. This is the produced,
content-bound signal the **attribution gate** (the required CI check) re-derives
at the merge boundary; without it the signal is latent and a real Argus commit
derives as human, and without the binding a trailer replayed onto a different
commit could not be detected.

In the current shared-account workflow the drive can invoke Git and set its
own environment. The hook supplies audit evidence for honestly labeled
commits, not an inaccessible supervisor boundary. The operator reviews the
exact prepared head before merge.

## Authored once, wired per CLI (AP-25)

The stamp decision and the trailer contract live **once**:

- the canonical stamp — `internal/argus/stamp` (`Stamp`), single-sourcing the
  trailer key from the gate's reader (`internal/argus/attribution`);
- the hook entry each commit-authoring path points at — `cmd/argus-stamp`,
  invoked by the repo's `core.hooksPath` **prepare-commit-msg** shim
  (`.githooks/prepare-commit-msg`, delegate-only).

Each supported CLI's wiring is a **pointer**, never a content copy — no trailer
logic and no protected path is named in any adapter or config here. Onboarding a
CLI whose own commit path needs distinct wiring is a **new adapter file**
(`internal/argus/stamp/adapter_<cli>.go` + a pointer config here), extending by
addition — never editing the shared stamp.

The authoritative, single-sourced list is the adapter registry — print it with:

```
argus-stamp paths
```

## Supported CLIs and commit-path wiring

| CLI | Commit-authoring path | Wiring (a pointer) |
|---|---|---|
| **claude** (Claude Code) | runs `git commit` via its Bash tool — standard git | the repo's `core.hooksPath` prepare-commit-msg hook fires on every commit; `claude.md` |

A commit authored **outside any drive** (`$ARGUS_DRIVE_SLUG` unset) carries **no
trailer** and derives as human — correct, it is not a drive's landed work.

## A commit outside a drive carries no trailer

The producer keys strictly on the active drive's slug (`--slug` /
`$ARGUS_DRIVE_SLUG`). With no slug the stamper is a no-op: the commit message is
left untouched. The drive guide requires the slug on each drive-authored
commit; `draft prepare-lock` verifies source bindings before refreshing valid
stamped commits during rebase.
