package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/plangate/render"
)

// This is authored input to the real planner, not a precomputed task capsule.
// The PRD/design are minimal fixture contracts resolved by the normal gates.
func seedNativeE2EPlan(t *testing.T, f *implementNowE2EFixture, source string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module example.invalid/native-routing-fixture\n\ngo 1.26.0\n",
		"docs/prds/native-routing/effective.source.json": `{"schema_version":"prd-effective-source/v1","units":{"FR-1":{"status":"live"}}}`,
		"docs/prds/native-routing/effective.md": `# Native routing fixture

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The fixture shall emit the requested output. | Exercise native delivery. | Must | low | The committed output matches the request. | Test | J1 |
`,
		"docs/tech-design/native-routing.md": "# Native routing fixture\n\n### 3.1 Output\n\nCommit the requested output.\n",
		nativeE2EPlanDir + "/001-plan.md": fmt.Sprintf(`# Native routing — plan 001

| Field | Value |
|---|---|
| **Feature** | native-routing |
| **PRD** | docs/prds/native-routing |
| **Design** | docs/tech-design/native-routing.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Native routing fixture |
| **Status** | Draft |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Add e2e fixture

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | %s |
| **Pattern** | — |
| **Touches** | internal/fixture |

**Goal:** %s
**Current behavior:** No fixture output exists.
**Desired behavior:** Reading the committed output returns the requested implementation text.

**Scope:**
- output behavior

**Out of scope:**
- unrelated output formats

**Acceptance criteria:**
- [ ] %s

**Key interfaces:**
- Output — the fixture result

**Traces:** prd:native-routing/FR-1

## Coverage

| Requirement | Tasks |
|---|---|
| prd:native-routing/FR-1 | T1 |
`, nativeE2ERoutes, nativeE2EGoal, nativeE2EAcceptance),
	}
	for path, data := range files {
		writeNativeE2EFile(t, filepath.Join(f.repo, path), []byte(data))
	}
	writeNativeE2EFile(t, filepath.Join(f.repo, render.VendorMermaid), readNativeE2EFile(t, filepath.Join(source, render.VendorMermaid)))
}
