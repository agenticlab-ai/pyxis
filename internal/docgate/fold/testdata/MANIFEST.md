# docgate fixture manifest — PRD fit criteria → tests (design A-2)

| PRD anchor | Fit criterion (short) | Test / fixture |
|---|---|---|
| prd:docgate/FR-1 | byte-identical folds; fingerprint stable | `TestFixtureZero` (determinism + the DOD-5 fingerprint constant), `TestOpsSuiteGolden` (byte goldens) |
| prd:docgate/FR-2 | named defect quadruple, both faces | every red in `TestRedCases` asserts reason + remedy; `--defects-json` wired in `cmd` and exercised in CI |
| prd:docgate/FR-3 | hand-edited projection red; regenerated green | `TestVerifyGeneratedRejectsHandEdits` |
| prd:docgate/FR-4 | M/D/R red vs merge base; add-only green | `CheckImmutable` — exercised end-to-end by the CI `docgate gate --merge-base` step (subprocess; unit-hermetic tests deliberately skip git) |
| prd:docgate/FR-5 | one red per envelope violation class | `TestRedCases`: status-draft, bad-evidence, understated-blast, unknown-envelope-field, unknown-verb |
| prd:docgate/FR-6 | diff yields effect + affected + tier + levels | `TestOpsSuiteGolden` (AmendMeta assertions); `gate.Diff` composes the same meta |
| prd:docgate/FR-7 | tampered rebase red; faithful green | `TestVerifyRebase` |
| prd:docgate/FR-8 | layout defects named | `TestLintStructure` (gap, missing base); duplicate-seq covered by `LintStructure` logic on same path |
| prd:docgate/UB-1 | gap/duplicate name files + next free number | `TestLintStructure` |
| prd:docgate/UB-2 | payload column mismatch named | `TestRedCases/payload-schema` |
| prd:docgate/NR-1 | zero findings outside ledger dirs | `TestGateUmbrellaOnRealRepo` (green over the real repo) |
| prd:docgate/NFR-1 | fast enough for pre-push | suite runtime; fold of the real ledger ≪ 1s (see CI timings) |
| prd:docgate/NFR-2 | hermetic | `TestFoldPurity` (import allowlist over schema/ledger/fold) |
| prd:docgate/NFR-3 | repo Go rules | CI: gofmt, go vet, `go test -race` |

Ops coverage (DOD-4 — the templates' example op forms): `ops-suite` exercises
supersede-row, add FR, add DOD, retire, answer (typed payload), rescope with
absorbing row, and supersede-§section, against goldens.
