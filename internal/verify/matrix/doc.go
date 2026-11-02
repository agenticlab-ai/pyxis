// Package matrix is the codegate multi-stack fixture matrix and permanent
// catch suite (design codegate-v1 §3.11, A-1/A-2/A-3; plan
// implementation-verification/001 T10). It has no non-test code of its own —
// this file exists only so `go build ./...` and `go vet ./...` see a package
// here; all behavior lives in the _test.go files, which drive the real ladder
// (internal/verify/ladder) against fixture repos of three distinct stacks
// (go, python, node) to prove two properties as tests, not claims:
//
//   - technology-independence (FR-16): the identical, stack-blind harness
//     binds and adjudicates all three stacks through ecosystem-table data
//     alone — asserted structurally by the inspection test (A-1).
//   - the anti-gaming bar (M2): every adversarial seed family — a tautological
//     test, a silenced test (deletion, skip-marker, assertion-removal), and a
//     machinery edit including a config-pin downgrade — stays blocked on every
//     stack it applies to, while the paired legitimate diffs pass untouched
//     with a byte-identical workspace tree after every rung-3 run (A-2/A-3).
package matrix
