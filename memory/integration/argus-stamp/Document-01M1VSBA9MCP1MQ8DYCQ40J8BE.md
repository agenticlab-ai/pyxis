# Argus authorship producer — the commit-time stamp (wiring surface)

The **producer** half of the attribution hardening
(`docs/tech-design/argus-attribution-hardening.md` §3.1, §3.2). It stamps the
contractual `Argus-Drive: <slug>` authorship trailer — and, alongside it, the
`Argus-Bind: <binding>` in-tree content binding (a credential-free digest over
the commit's own tree+parents) — onto **every commit a drive's work is landed
as**, keyed on the active drive's slug — **by construction**, at the
commit-authoring surface, out of the drive's reach. This is the produced,
content-bound signal the **attribution gate** (the required CI check) re-derives
at the merge boundary; without it the signal is latent and a real Argus commit
derives as human, and without the binding a trailer replayed onto a different
commit could not be detected.

The drive (the model) authors **files, not commits** — the supervisor/harness
runs `git commit`, so the producer sits on a surface the model never reaches: it
can neither add a trailer to frame, omit one to evade, nor alter one (DINV-3).

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
left untouched. Only a drive context (which the supervisor/harness sets when it
lands a drive's work) produces the trailer.
