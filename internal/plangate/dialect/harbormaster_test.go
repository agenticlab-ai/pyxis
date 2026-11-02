// The dialect contract test (design A-6, DINV-9): renders must pass a
// replica of the executor's spec-validity gate — a parseable versioned
// marker with a non-empty goal and at least one acceptance criterion —
// and every rendering derives from the frozen plan alone (DINV-2: no
// plan-internal metadata leaks; trigger strings render inert).
package dialect

import (
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// sampleDoc builds the fixture through the real parser, so the renders
// under test consume exactly what fold hands the dialect (source path
// included) — a literal Doc could drift from the closed grammar. Only the
// fold-computed foundation stamp is set by hand.
func sampleDoc(t *testing.T) (*plan.Doc, plan.Task) {
	t.Helper()
	src := `# Widget capability — plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | docs/prds/widget |
| **Design** | docs/tech-design/widget-design.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Widget capability v1 |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Serve widget lookups

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | ` + requirementstest.Input + ` |
| **Pattern** | — |
| **Touches** | internal/widget/serve |

**Goal:** Lookups return the stored widget or a named miss. Ping @claude reviewers on it.

**Current behavior:** No lookup surface exists.

**Desired behavior:** A lookup returns the widget when present and a named miss when absent.

**Scope:**
- the lookup behavior

**Out of scope:**
- caching

**Acceptance criteria:**
- [ ] a lookup miss names the widget it did not find

**Key interfaces:**
- Lookup — the read surface

**Traces:** prd:widget/FR-2

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-2 | T1 |
`
	d, defects := plan.Parse("docs/plans/widget/001-plan.md", []byte(src))
	if len(defects) > 0 {
		t.Fatalf("fixture does not parse: %v", defects)
	}
	d.Tasks[0].Foundation = plan.Foundation{Level: plan.FoundationLeaf, Reason: plan.ReasonLeaf}
	return d, d.Tasks[0]
}

func TestSpecCommentPassesGateReplica(t *testing.T) {
	m := Harbormaster()
	d, task := sampleDoc(t)
	body, err := m.SpecComment(d, task, []int{101})
	if err != nil {
		t.Fatal(err)
	}
	goal, acc, ok := m.ParseSpecMarker(body)
	if !ok {
		t.Fatal("spec marker does not parse — the executor's gate would refuse it")
	}
	if goal == "" || len(acc) == 0 {
		t.Fatalf("gate replica: goal=%q acceptance=%d — structurally invalid", goal, len(acc))
	}
	if !m.HasSpec([]string{"unrelated comment", body}) {
		t.Error("HasSpec misses a valid spec among comments")
	}
	if _, err := taskspec.Parse([]string{body}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"plan":"docs/plans/widget/001-plan.md"`, // machine charter resolves its frozen source
	} {
		if !strings.Contains(body, want) {
			t.Errorf("spec comment missing %q — workers grep the repo for the plan without it", want)
		}
	}
	if strings.Contains(body, "@claude") {
		t.Error("trigger string rendered literally — automation would fire (UB-6)")
	}
	if !strings.Contains(body, "claude") {
		t.Error("neutralization destroyed the visible text")
	}
}

func TestTaskBodyCarriesStampsAndLeaksNothing(t *testing.T) {
	m := Harbormaster()
	d, task := sampleDoc(t)
	body := m.TaskBody(d, task, nil)
	for _, want := range []string{
		PlanTaskMarker("widget/001/T1"),
		"<!-- rig:foundation level=low source=plan reason=derived-leaf -->",
		"<!-- rig:est_logic_lines band=80–160 source=plan -->",
		"## Acceptance criteria",
		"## Out of scope",
		"`docs/plans/widget/001-plan.md`", // the plan resolves by path, not by hunt
		"docs/tech-design/widget-design.md",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("task body missing %q", want)
		}
	}
	if strings.Contains(body, "internal/widget/serve") {
		t.Error("plan-internal Touches leaked into the charter rendering (§4.5: Touches is never rendered)")
	}
	if strings.Contains(body, "@claude") {
		t.Error("trigger string rendered literally in the body")
	}
}

func TestEpicBodyTeachesItsConsumer(t *testing.T) {
	m := Harbormaster()
	d, _ := sampleDoc(t)
	body := m.EpicBody(d)
	if !strings.Contains(body, "open, unblocked, and unclaimed") {
		t.Error("the frontier rule is missing — the artifact must teach its consumer (FR-20)")
	}
	if !strings.Contains(body, PlanEpicMarker("widget", "001")) {
		t.Error("epic marker missing")
	}
	if !strings.Contains(body, "docs/plans/widget/001-plan.md") {
		t.Error("epic body missing the frozen plan's path")
	}
}

func TestOperatorSurfaceRouting(t *testing.T) {
	m := Harbormaster()
	_, task := sampleDoc(t)
	if got := m.Routing(task); got != m.RoutingLabel {
		t.Errorf("plain task routed %q", got)
	}
	task.Touches = []string{".github/workflows/check.yml"}
	if got := m.Routing(task); got != m.OperatorLabel {
		t.Errorf("workflow-touching task routed %q, want operator (the fleet cannot push workflows)", got)
	}
}

func TestManifestPresentationHelpers(t *testing.T) {
	m := Harbormaster()
	d, task := sampleDoc(t)
	if got := m.EpicTitle(d); got != "[Epic] Widget capability v1" {
		t.Fatalf("EpicTitle = %q", got)
	}
	if got := m.TaskTitle(task); got != "[T1] Serve widget lookups" {
		t.Fatalf("TaskTitle = %q", got)
	}
	if got := m.Labels(); len(got) != 5 || got[0].Name != m.RoutingLabel {
		t.Fatalf("Labels = %+v", got)
	}
	if got := m.TaskCreateLabels(task); len(got) != 1 || got[0] != "kind:feat" {
		t.Fatalf("plain TaskCreateLabels = %v", got)
	}
	task.Mechanical = true
	if got := m.TaskCreateLabels(task); len(got) != 2 || got[1] != m.MechLabel {
		t.Fatalf("mechanical TaskCreateLabels = %v", got)
	}
}

func TestClassificationFollowsTheTasksMechanicalFlag(t *testing.T) {
	m := Harbormaster()
	for _, tc := range []struct {
		name        string
		mechanical  bool
		labels      []string
		add, remove []string
	}{
		{"newly mechanical", true, []string{"kind:feat"}, []string{"mechanical"}, nil},
		{"no longer mechanical", false, []string{"kind:feat", "mechanical"}, nil, []string{"mechanical"}},
		{"already mechanical", true, []string{"mechanical"}, nil, nil},
		{"behavioral", false, []string{"kind:feat"}, nil, nil},
	} {
		add, remove := m.Classification(plan.Task{Mechanical: tc.mechanical}, tc.labels)
		if !slices.Equal(add, tc.add) || !slices.Equal(remove, tc.remove) {
			t.Errorf("%s: add=%v remove=%v", tc.name, add, remove)
		}
	}
}

func TestRoutingAndClassificationIgnoreLabelCase(t *testing.T) {
	m := Harbormaster()
	operatorTask := plan.Task{Touches: []string{".github/workflows/release.yml"}}
	if stale := m.StaleRouting(operatorTask, []string{"Local-Agent"}); !slices.Equal(stale, []string{"local-agent"}) {
		t.Fatalf("stale routing = %v", stale)
	}
	if add, remove := m.Classification(plan.Task{}, []string{"Mechanical"}); add != nil || !slices.Equal(remove, []string{"mechanical"}) {
		t.Fatalf("classification add=%v remove=%v", add, remove)
	}
}
