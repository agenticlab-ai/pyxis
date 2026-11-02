# Session log — e2e-design T3: pick the harness form by judging whether existing harnesses fit

Started: 2026-08-08 00:00 · CLI: Claude Code · Stage: /implement-now · Task: e2e-verification/002/T3

## Status

Nothing escalated this session.

## Work completed

Task T3 (e2e-verification/002#T3) — "e2e-design picks the harness form by judging whether a fitting harness exists" — is **complete**.

The implementation provides the **judgment** half of the harness-form decision by reading the repository's existing test trees and deciding whether any can drive the feature's primary external interface.

### Core implementation

**Files modified:**
- `internal/e2e/design/harness.go` — 302 lines
- `internal/e2e/design/harness_test.go` — 321 lines

**Key functions:**
- `FindHarnesses(repoRoot, readSet)` — scans the bounded read set for existing test harnesses, scoring by framework type and test structure
- `PickHarnessForm(harnesses, featureInterface)` — judges whether any discovered harness fits the feature interface, returning reuse form for high-confidence matches (≥0.6) that fit the interface, build form otherwise
- `HarnessForm` — struct representing either form with fields for `IsReuse`, `ExistingPath`, `Additions`, `SlotAnswers`

### Acceptance criteria — all met

- ✓ on a fixture whose harness already exists, `PickHarnessForm` returns reuse form with `IsReuse: true` and `ExistingPath` set, no slot rows
- ✓ on a fixture with no fitting harness, `PickHarnessForm` returns build form with `IsReuse: false`
- ✓ across fixtures of two distinct app shapes (HTTP API via httptest, subprocess invocation via exec), both hold with no per-shape code branches — all shape choices are authored content (the `featureInterface` parameter), never code literals

### Design property verified

**App-shape neutrality**: The code contains no app-shape literals or branches on shape types. Instead:
- Framework detection reads actual test file imports and patterns
- Interface matching reads authored keywords from the feature interface description
- The same `PickHarnessForm` logic handles all shapes by delegating to the authored interface string

Test `TestAppShapeNeutralRule` explicitly verifies this: same harness set, different feature interface strings, different correct harness selections — all from one code path.

### Test results

All 16 harness-related tests pass:
- Framework detection (httptest, exec)
- Harness form selection (reuse, build, confidence threshold)
- Interface matching (HTTP, subprocess, mismatch cases)
- Multi-shape discovery (both frameworks found, sorted by confidence)
- App-shape neutrality (same code, different authored interfaces, correct picks)

Full test suite: all 51 e2e packages pass without error.
