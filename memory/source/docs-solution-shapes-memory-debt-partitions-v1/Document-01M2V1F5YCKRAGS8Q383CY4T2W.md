# Independent memory debt allowances — solution shape

| Field | Value |
|---|---|
| **Design** | memory-debt-partitions-v1 |
| **Status** | authoritative |

Implementation reference for the paired design, approved on 2026-09-17.
No implementation is asserted. This shape becomes
`superseded-by-code` when its implementing code lands.

## S-1. Closed policy layout

Witnesses **DINV-1**, **DINV-2**, **DINV-3**, **DINV-4**.

```text
.github/memory-reconciliation-baseline.json       # stable v2 format/evaluator marker
.github/memory-reconciliation-baseline/
  <address-sha256>.json                          # one BaselineAllowance per address
```

The marker is exactly (**DINV-1**, **DINV-3**):

```json
{
  "schema_version": "memory-reconciliation-baseline/v2",
  "generator": "memory-reconcile/v3"
}
```

Members retain the existing allowance wire fields; `feature` is omitted when
empty. The filename is lowercase SHA-256 of the UTF-8 JSON array
`["memory-debt-address/v1", code, scope, feature]`, with Go `encoding/json`
escaping, no indentation and no trailing newline. Count, severity, and
repairability do not affect the filename. Use the complete digest. The storage
format change preserves the current evaluator identifier because it does not
change source classification. The existing allowance value realizes **DINV-2**:

```go
// Existing reconcile.BaselineAllowance; no new allowance semantics.
type BaselineAllowance struct {
    Code     string `json:"code"`
    Scope    string `json:"scope"`
    Feature  string `json:"feature,omitempty"`
    Count    int    `json:"count"`
    Blocking bool   `json:"blocking"`
    Writable bool   `json:"writable"`
}
```

Decode marker and members closed, rejecting duplicates and trailing values.
Members must be canonical regular blobs directly under the fixed directory.
Reject subdirectories and nonmember files. Count is positive; decoded address
must match the filename; duplicate decoded addresses fail. The root directory
may be absent only when the v2 marker is valid and there are zero findings.
Do not accept symlinks or implicit directory redirects. v1 with any v2 members
present is a mixed-format error; v2 does not accept an embedded allowance list.

## S-2. Existing package seams and result

Witnesses **DINV-2**, **DINV-3**, **DINV-5**, **DINV-6**, **DINV-8**.

```go
// Proposed concrete values/functions inside internal/memory/reconcile.
// IO remains in internal/memory/cli and internal/memory/gitsrc.
type PolicySet struct {
    Schema     string
    Generator  string
    Allowances []BaselineAllowance // sorted by the existing address comparator
}

func AllowancePath(a BaselineAllowance) (string, error)
func ParsePolicySet(marker []byte, members map[string][]byte) (PolicySet, error)
func MarshalPolicySet(p PolicySet) (map[string][]byte, error)

// Adapt existing CompareBaseline/CompareBaselines to normalized PolicySet;
// retain their existing count, blocking, and writable comparisons.
// Legacy Baseline still owns its four input hashes during v1 reproduction.
```

Use `gitsrc.ListTree` and the existing batch blob read for each pinned policy
root. Both formats normalize to the same sorted allowance values; generation
is checked before selecting a compatibility path. Reports retain the four
computed hashes and add the actual policy file hashes, format, generator,
base/head SHAs, changed addresses, and any legacy replay commit. Report format
is versioned when fields change; stdout human and JSON faces share one value.
No new checked-in report is mandatory for a successful check.

Candidate tree checks remain in `cli.runReconcile`; `RunGate` still composes
the immutable-record check and reconciliation. Keep code transition and source
retirement checks even when the allowance delta is empty. Recognized v1
generators use the existing reproduction options and replay proof. v2 trusted
policy is reproduced by recomputing its report at its own commit/config and
checking exact allowance equality under its declared evaluator; never require
an obsolete global hash to match the new candidate.

Preserve `baselinePhaseAdvance` exactly for the existing directional pairs:
`missing-prd-memory` to `unserved-prd-memory`,
`missing-design-target-memory` to `missing-serves-memory`, and
`unmapped-prd-unit` to `open-prd-question`. The old address must disappear,
scope/feature must match, and count cannot grow. Synchronization deletes the
predecessor member and creates its permitted successor; it does not treat
that narrowly recognized pair as arbitrary new debt (**DINV-2**, **DINV-5**).

## S-3. Explicit writer

Witnesses **DINV-4**, **DINV-6**, **DINV-7**.

```text
memory reconcile --baseline-generate
    Existing v1 stdout behavior retained for v1 repositories.
    In v2: refuse with the explicit sync command; never emit misleading v1 policy.

memory reconcile --baseline-sync --baseline-ref <trusted-commit> [--json]
    v2 only; preview/preflight then synchronize exact reductions at owned member paths.

memory reconcile --baseline-sync --baseline-ref <trusted-commit> --migrate-v2 [--json]
    Explicit v1-to-v2 data migration; exact allowance preservation required.
```

These are modes of the existing `reconcile` command, mutually exclusive with
`--check`, `--write`, and `--baseline-generate`. `--baseline-ref` is mandatory
for writes and may equal HEAD only for an explicit clean mainline migration;
normal validation still requires a strict ancestor. CI-supplied base authority
cannot be overridden. `--baseline-sync` must not create initial policy where
neither format has been trusted. Keep the existing first-adoption v1 path
outside this migration.

The writer uses the existing `openLocked` repository write lock. Pin HEAD,
trusted commit and working policy bytes; recompute the report and whole write
plan; validate source/code/record transition requirements; compare to trusted
allowances; refuse widening. Normal v2 sync may operate after nonpolicy changes
are committed; policy worktree must equal HEAD or an exact subset of this
deterministically planned output from an interrupted prior attempt. It never
accepts arbitrary dirty policy as its input authority. Migration requires a
clean worktree on its first attempt and unchanged allowance inventory. Only
an exact interrupted-plan subset qualifies for the retry exception below.

Write only changed members via safe temporary files and renames, delete only
owned members proven obsolete by the plan, and write the version marker last
during migration. Recheck HEAD before applying the plan. Failure is explicit
with completed/planned paths; a partial set is not gate-valid. A retry proves
each dirty member is either its pinned input or exact planned output before
continuing. The migration retry uses the pinned v1 HEAD as authority even if
the worktree marker already changed. Do not touch records, code, configuration,
or generated docs, and never invoke Git commit/stage.

## S-4. Proof matrix and cutover

Witnesses **DINV-1** through **DINV-8**.

```text
pure: canonical address -> path; malformed JSON/tree -> refusal;
      exact/decreased/increased allowance matrix -> existing semantics
Git: 10 sibling fixtures -> 45 pairs x both orders -> disjoint baseline paths
     separate debt reductions -> distinct member changes
     same debt address -> preserved policy conflict / exactness check
migration: v1 exact reproduction -> identical v2 normalized set
           unknown generator/missing replay/extra allowance -> refusal
writer: preflight failure -> zero writes; interrupted plan -> refusal;
        exact retry -> convergence; completed retry -> zero writes
gate: desk/PR/push/schedule; real squash -> fresh transport clone, offline
```

Implement code and tests in the existing `internal/memory/reconcile`, `cli`,
and `cli/*_test.go` seams, with documentation in `cmd/memory/README.md`.
No new module or dependency is proposed. Land the dual reader/writer while
policy remains v1, then make a separate data-only migration. Regenerate only
feature-owned records needed for actual implementation changes. Once approved,
the acceptance tests must execute the built CLI against actual Git fixtures;
this matrix is a specification, not a test result.
