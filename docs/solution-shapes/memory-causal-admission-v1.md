# Rebase-stable memory admission — solution shape

| Field | Value |
|---|---|
| **Design** | memory-causal-admission-v1 |
| **Status** | authoritative |

Implementation reference for the paired design, approved on 2026-09-17.
No implementation is asserted; retire this shape when its code lands.

## S-1. Existing record representation

Witnesses **DINV-1**, **DINV-2**, **DINV-4**, **DINV-6**.

```text
memory/<subject>/<Kind>-<ULID>.json
memory/<subject>/Document-<ULID>.md      # paired body where applicable
```

Keep `record.Record`, `record.NewID`, `record.IDBefore`, relation schemas,
and resolver chronology. No new issuance field or global counter. Remove only
the latest-record comparison in `cli.(*memory).admit` and the latest-ancestor
comparison in `packet.Compute`. `IDBefore(target, carrier)` remains mandatory
for every explicit relation through existing specific resolver checks.

## S-2. Shared transition guard

Witnesses **DINV-2**, **DINV-3**, **DINV-4**, **DINV-5**, **DINV-7**.

```go
// Proposed addition to existing internal/memory/resolve; no new package/port.
// Projections include their full Set, invalid relations, and dangling edges.
func HistoricalTransitionViolations(before, after *Projection) []Violation
```

Extract the existing packet loops for `historical-act-reinterpreted` into this
pure function. It identifies records present before by ID and exact immutable
content, and compares stable violation/edge identities, not diagnostic prose.
For each prior invalid act whose previously refused relation is now honored
while its carrier remains active, report reinterpretation. Use the resolver's
accepted-edge state (`Projection.Targets` / its internal act maps), not merely
the absence of the former diagnostic. A replacement rejection reason must not
be mistaken for activation. For each prior dangling relation now
resolved through a newly added target, report reinterpretation unless its
carrier has been explicitly retired under the existing valid repair rules.
Do not short-circuit because other defects exist. Sort by carrier, target,
and code, and retain related record IDs for diagnosis.

The function does not decide Git immutability or base authority. The caller
still rejects overwritten/deleted records and selects the trusted before set.
Missing projections are a refusal, not an empty result. Use the existing
`Violation` representation, adding explicit diagnostic context only if needed
without changing record storage. This call graph witnesses **DINV-5** and **DINV-7**:

```text
CLI record write:
    parse/validate -> relation targets present -> existing kind/family checks
    -> trial projection -> reject new projection defects
    -> HistoricalTransitionViolations(current, trial) -> store new file(s)

packet/check:
    pin base/ancestor/head -> existing three-way union
    -> immutable/add-collision/moved-head/exclusive-family checks
    -> existing projection defects + HistoricalTransitionViolations(base, union)

shared memory gate:
    packet/check as above -> unchanged reconciliation/evidence predicates
```

Extend the existing private admission/trial helpers to accept the candidate
Document body as well as its envelope. `memory.store` already owns both;
non-Document writers pass no body. Add both files to the trial source before
`store.Load`, verify the body hash, and reject new loader problems before
using the resulting projection. Otherwise a missing body would make the
trial silently omit the Document whose effects it is supposed to validate.
Include missing-body and mismatched-body refusal fixtures with zero writes.

`firstNewProjectionDefect` remains an admission check for all record kinds,
not only Revocation. Do not import `packet` into `resolve`: move only the
pure shared comparisons down into the existing resolver seam and map their
typed values to packet findings in the packet layer.

## S-3. Branch-order fixture

Witnesses **DINV-1**, **DINV-5**, **DINV-6**, **DINV-7**.

```text
M: valid policy/history
|- A: add Learning A, issuance order 20
`- B: add Learning B, issuance order 30, disjoint subject

1. Validate A against M.
2. Integrate B into main (merge variant; squash variant).
3. Rebase A onto main, preserving Learning A bytes and ID.
4. Validate A against the new main and integrate it.
5. Clone main through file transport with no alternates/topic refs.
6. Assert both records, their bytes, and expected gate classification.
7. Repeat with issuance/integration order reversed.
```

Use the CLI's existing injectable `Env.Now` and entropy for controlled records
or canonical records produced by that writer. No sleeps and no system-clock
adjustment. Where v1 baseline is used, refresh it separately and exclude those
bytes from the chronology guarantee; combined acceptance with
`memory-debt-partitions-v1` requires zero baseline edits as well.

## S-4. Adversarial and entry-path matrix

Witnesses **DINV-2**, **DINV-3**, **DINV-4**, **DINV-5**, **DINV-7**.

```text
keep exact fixtures:
  TestCandidateCannotInsertBeforeKnownHistoryAndRetroactivelyActivateARevocation
  TestCandidateCannotSilentlyActivateAConcurrentBaseRevocation
  TestAddedTargetCannotSilentlyActivateHistoricalDanglingAct

expected change:
  harmless older relationless record: record-history-insertion -> accepted
  backdated record activating old rejected act:
      record-history-insertion -> historical-act-reinterpreted

still refused:
  forward relations, missing targets, cross-subject acts, moved heads,
  competing claims/answers/revocations, duplicate IDs, changed base records

writer audit:
  add, revise, ask, answer, revoke, reconcile --write, closeout --write
  -> existing store/admit seam -> shared trial/transition validation
```

Each single-record refused write asserts no JSON/body file was created. For
multi-record reconciliation, preserve its documented partial-progress behavior:
an earlier independently valid append is reported as written, but the refused
record and its body are absent. This proposal does not introduce a new batch
transaction promise.

Implementation touches the existing `cli`, `packet`, and `resolve` packages,
their tests, and `cmd/memory/README.md`. It requires no dependencies, new module,
global issuance file, or stored-data migration. These fixtures are proposed
acceptance work, not evidence that the new behavior has executed.
