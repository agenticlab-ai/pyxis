# Argus write hook — the write-time protected-path deny (adapter surface)

The **fast layer** of the three-layer boundary enforcement
(`docs/tech-design/argus-write-hook.md`). It denies a drive's write to a
**protected path** — the authority contract, a frozen/merged governance
document, another problem's ledger, or a file outside the drive's declared
scope — through each supported agent CLI's **own** pre-write hook, before any
bytes are written, with a reason naming the **path, the boundary, and a
remedy**.

This is **not** the unbypassable guarantee: a desk-side deny cannot stop an
out-of-band write (a raw shell redirect, a non-honoring CLI). The
**merge-boundary layers** — the authority-contract structure arm (docgate
gate) and the attribution gate (required CI check) — carry that guarantee.
Over-claiming this layer as absolute is the R1 risk the honest split avoids.

## Authored once, wired per CLI (AP-25 · DINV-5)

The deny logic and the protected-path knowledge live **once**:

- the canonical deny decision — `internal/argus/writehook` (`Decide`), reusing
  the shared classifier + scope resolver `internal/argus/boundary` (T1);
- the shim each CLI's hook points at — `cmd/argus-writehook`.

Each supported CLI's wiring is a **pointer**, never a content copy — no
protected path is named in any adapter. Onboarding a CLI is a **new adapter
file** (`internal/argus/writehook/adapter_<cli>.go` + a pointer config here),
extending by addition — never editing the shared decision.

## Supported CLIs and stated guarantees

The authoritative, single-sourced list is the adapter registry — print it with:

```
argus-writehook guarantees
```

| CLI | Pre-write hook | Stated guarantee |
|---|---|---|
| **claude** (Claude Code) | `PreToolUse` (matcher `Write\|Edit\|MultiEdit`) | write-time protected-path deny, **fail-closed** (`claude.settings.json`) |
| **codex** (Codex CLI) | none fail-closable | the **merge-boundary layers** (structure arm + attribution gate) — never a silent fail-open (UB-2) |

A CLI whose hook cannot express a fail-closed deny is **listed here with the
merge-boundary backstop as its guarantee** (DINV-4), never silently allowed.

## Fail-closed rules (§3.3 · AP-17)

- an **unresolvable declared scope** denies (no drive / no resolvable scope);
- a **hook process error, crash, or timeout** denies — the shim maps every
  internal error, and a panic, to a deny; Claude's PreToolUse default is
  fail-open, so this adapter is the explicit fail-closed setting.

## Enabling the Claude Code adapter

Merge `claude.settings.json` into `.claude/settings.json` and put the
`argus-writehook` binary on `PATH`. The drive's declared scope is resolved
from author-independent sources — the committed (merge-base) delivery ledger,
or the slug-derived first-PR set — via `--slug` / `$ARGUS_DRIVE_SLUG`
(`--first-pr` / `$ARGUS_DRIVE_FIRST_PR` for a brand-new problem's first PR).
