// Hermetic projection suites (design A-2..A-5, A-7): an in-memory tracker
// honoring the seam's contract (AP-14 — the fake models GitHub's actual
// semantics: additive verbs, no CAS on create), with failure injection for
// the interruption ladder and a checker wrapper proving the route-last
// invariant at every mutation.
package project

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

// ---- fixture repo (mirrors the plan package's golden shape) ----

const srcJSON = `{"schema_version":"prd-effective-source/v1","units":{"FR-1":{"status":"live"},"FR-2":{"status":"live"},"UB-1":{"status":"live"}}}`

const effMD = `# Widget — PRD (projection)

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The widget shall exist. | why | Must | low | fit | Test | J1 |
| FR-2 | The widget shall serve lookups. | why | Must | med | fit | Test | J1 |

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If a lookup misses | say so | a named miss | Must | low | fit | Test | FR-2 |
`

const designMD = "# Widget design\n\n### 3.1 The model\n\nbody\n"

func fixturePlan(t3Touches string) string {
	return `# Widget capability — plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | docs/prds/widget |
| **Design** | docs/tech-design/widget-design.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Widget capability v1 |
| **Status** | Draft |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Define the widget model

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | ` + requirementstest.Input + ` |
| **Pattern** | — |
| **Touches** | internal/widget/model |

**Goal:** The widget model type exists with closed validation.
**Current behavior:** No widget model exists.
**Desired behavior:** A typed widget model validates closed and rejects an unknown field with a defect naming that field.

**Scope:**
- the widget model type and its closed validation

**Out of scope:**
- persistence of widgets

**Acceptance criteria:**
- [ ] validating a widget with an unknown field yields a defect naming the field

**Key interfaces:**
- ` + "`Widget`" + ` — the model type

**Traces:** prd:widget/FR-1

### T2 — Serve widget lookups

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | ` + requirementstest.Input + ` |
| **Pattern** | — |
| **Touches** | internal/widget/serve |

**Goal:** Lookups return the stored widget or a named miss.
**Current behavior:** No lookup surface exists.
**Desired behavior:** A lookup returns the widget when present and a named miss when absent.

**Scope:**
- the lookup behavior

**Out of scope:**
- caching

**Acceptance criteria:**
- [ ] a lookup miss names the widget it did not find

**Key interfaces:**
- ` + "`Lookup`" + ` — the read surface

**Traces:** prd:widget/FR-2 · prd:widget/UB-1

### T3 — Wire the release check

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 20–40 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | ` + requirementstest.Input + ` |
| **Pattern** | — |
| **Touches** | ` + t3Touches + ` |

**Goal:** The release check gates widget lookups.
**Current behavior:** No release check runs.
**Desired behavior:** The release check runs on every change and names its verdict.

**Scope:**
- the release check wiring

**Out of scope:**
- the check's internals

**Acceptance criteria:**
- [ ] the check reports one verdict per change

**Key interfaces:**
- ` + "`Check`" + ` — the verdict surface

**Traces:** prd:widget/FR-2

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-1 | T1 |
| prd:widget/FR-2 | T2, T3 |
| prd:widget/UB-1 | T2 |
`
}

const planRel = "docs/plans/widget/001-plan.md"

func writeRepo(t *testing.T, planSrc string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"docs/prds/widget/effective.source.json": srcJSON,
		"docs/prds/widget/effective.md":          effMD,
		"docs/tech-design/widget-design.md":      designMD,
		planRel:                                  planSrc,
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestProjectReportsAnUnreadablePlanWithDefaultLogging(t *testing.T) {
	report, defects := Project(t.TempDir(), config.Config{}, "docs/plans/missing/001-plan.md", nil, dialect.Manifest{}, Options{})
	if report != nil || len(defects) == 0 {
		t.Fatalf("Project = %+v, defects=%v", report, defects)
	}
}

// ---- the fake tracker (contract-faithful: additive verbs, no create-CAS) ----

type fake struct {
	next      int
	issues    map[int]*tracker.Issue
	repoLabel map[string]bool
	subs      map[int]map[int]bool
	blocked   map[int]map[int]bool
	comments  map[int][]string
	preseed   map[int]bool
	audit     []string // mutations: "verb #target"
	failAfter int      // fail when mutations reach this count; <0 = never
	muts      int
}

func newFake() *fake {
	return &fake{next: 100, issues: map[int]*tracker.Issue{}, repoLabel: map[string]bool{},
		subs: map[int]map[int]bool{}, blocked: map[int]map[int]bool{}, comments: map[int][]string{},
		preseed: map[int]bool{}, failAfter: -1}
}

func (f *fake) seed(num int, is tracker.Issue) {
	is.Number, is.State = num, "open"
	f.issues[num] = &is
	f.preseed[num] = true
	if num >= f.next {
		f.next = num + 1
	}
}

func (f *fake) mutate(verb string, target int) error {
	f.muts++
	if f.failAfter >= 0 && f.muts >= f.failAfter {
		return fmt.Errorf("injected failure at mutation %d (%s #%d)", f.muts, verb, target)
	}
	f.audit = append(f.audit, fmt.Sprintf("%s #%d", verb, target))
	return nil
}

func (f *fake) EnsureLabel(name, color, desc string) error {
	if f.repoLabel[name] {
		return nil
	}
	if err := f.mutate("label", 0); err != nil {
		return err
	}
	f.repoLabel[name] = true
	return nil
}

func (f *fake) CreateIssue(title, body string, labels []string, issueType string) (int, error) {
	if err := f.mutate("create", f.next); err != nil {
		return 0, err
	}
	n := f.next
	f.next++
	f.issues[n] = &tracker.Issue{Number: n, Title: title, Body: body, Labels: append([]string{}, labels...), State: "open", Type: issueType}
	return n, nil
}

func (f *fake) OpenIssues() ([]tracker.Issue, error) {
	var out []tracker.Issue
	for _, is := range f.issues {
		if is.State == "open" {
			out = append(out, *is)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Number < out[b].Number })
	return out, nil
}

func (f *fake) SubIssues(parent int) ([]tracker.Issue, error) {
	var out []tracker.Issue
	for child := range f.subs[parent] {
		out = append(out, *f.issues[child])
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Number < out[b].Number })
	return out, nil
}

func (f *fake) LinkSubIssue(parent, child int) error {
	if err := f.mutate("link", child); err != nil {
		return err
	}
	if f.subs[parent] == nil {
		f.subs[parent] = map[int]bool{}
	}
	f.subs[parent][child] = true
	return nil
}

func (f *fake) BlockedBy(issue int) ([]int, error) {
	var out []int
	for b := range f.blocked[issue] {
		out = append(out, b)
	}
	sort.Ints(out)
	return out, nil
}

func (f *fake) AddBlockedBy(issue, blocker int) error {
	if err := f.mutate("wire", issue); err != nil {
		return err
	}
	if f.blocked[issue] == nil {
		f.blocked[issue] = map[int]bool{}
	}
	f.blocked[issue][blocker] = true
	return nil
}

func (f *fake) RemoveBlockedBy(issue, blocker int) error {
	if err := f.mutate("unwire", issue); err != nil {
		return err
	}
	delete(f.blocked[issue], blocker)
	return nil
}

func (f *fake) AddLabels(issue int, labels []string) error {
	if err := f.mutate("label-add", issue); err != nil {
		return err
	}
	is := f.issues[issue]
	for _, l := range labels {
		if !is.HasLabel(l) {
			is.Labels = append(is.Labels, l)
		}
	}
	return nil
}

func (f *fake) RemoveLabel(issue int, label string) error {
	if err := f.mutate("label-rm", issue); err != nil {
		return err
	}
	is := f.issues[issue]
	is.Labels = slices.DeleteFunc(is.Labels, func(l string) bool { return strings.EqualFold(l, label) })
	return nil
}

func (f *fake) Comments(issue int) ([]string, error) {
	return append([]string{}, f.comments[issue]...), nil
}

func (f *fake) Comment(issue int, body string) error {
	if err := f.mutate("comment", issue); err != nil {
		return err
	}
	f.comments[issue] = append(f.comments[issue], body)
	return nil
}

func (f *fake) EditIssue(issue int, title, body string) error {
	if err := f.mutate("edit", issue); err != nil {
		return err
	}
	f.issues[issue].Title, f.issues[issue].Body = title, body
	return nil
}

func (f *fake) ReplaceComment(issue int, old, body string) error {
	matches := 0
	for _, c := range f.comments[issue] {
		if c == old {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("issue #%d has %d matching comments", issue, matches)
	}
	if err := f.mutate("replace", issue); err != nil {
		return err
	}
	for i, c := range f.comments[issue] {
		if c == old {
			f.comments[issue][i] = body
		}
	}
	return nil
}

// routeChecker proves DINV-5 at every mutation: an issue receiving the
// routing signal must already be linked and fully wired.
type routeChecker struct {
	*fake
	t        *testing.T
	m        dialect.Manifest
	expected map[string][]string // plan-task-id → blocker plan-task-ids
}

func (rc *routeChecker) AddLabels(issue int, labels []string) error {
	for _, l := range labels {
		if l != rc.m.RoutingLabel && l != rc.m.OperatorLabel {
			continue
		}
		linked := false
		for _, children := range rc.subs {
			if children[issue] {
				linked = true
			}
		}
		if !linked {
			rc.t.Errorf("issue #%d routed before its parent link exists (DINV-5)", issue)
		}
		body := rc.issues[issue].Body
		for id, deps := range rc.expected {
			if !strings.Contains(body, dialect.PlanTaskMarker(id)) {
				continue
			}
			for _, depID := range deps {
				found := false
				for b := range rc.blocked[issue] {
					if strings.Contains(rc.fake.issues[b].Body, dialect.PlanTaskMarker(depID)) {
						found = true
					}
				}
				if !found {
					rc.t.Errorf("issue #%d routed with edge to %s missing (DINV-5)", issue, depID)
				}
			}
		}
	}
	return rc.fake.AddLabels(issue, labels)
}

// ---- suites ----

func opts() Options {
	return Options{Log: io.Discard}
}

func expectedEdges() map[string][]string {
	return map[string][]string{
		"widget/001/T1": {},
		"widget/001/T2": {"widget/001/T1"},
		"widget/001/T3": {"widget/001/T1"},
	}
}

func TestProjectHappyAndConvergent(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	f := newFake()
	f.seed(1, tracker.Issue{Title: "unrelated", Body: "pre-existing foreign issue"})
	rc := &routeChecker{fake: f, t: t, m: dialect.Harbormaster(), expected: expectedEdges()}

	rep, defects := Project(root, config.Defaults(), planRel, rc, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if rep.Epic == 0 || len(rep.Items) != 3 {
		t.Fatalf("report wrong: %+v", rep)
	}
	// Legibility: hierarchy reads from the list — prefixes + native types.
	if e := f.issues[rep.Epic]; !strings.HasPrefix(e.Title, "[Epic] ") || e.Type != "Feature" {
		t.Errorf("epic title/type illegible: %q / %q", e.Title, e.Type)
	}
	if t1 := f.issues[rep.Items[0].Issue]; !strings.HasPrefix(t1.Title, "[T1] ") || t1.Type != "Task" {
		t.Errorf("task title/type illegible: %q / %q", t1.Title, t1.Type)
	}
	for _, it := range rep.Items {
		if it.Status != "created" {
			t.Errorf("item %s status %q, want created", it.TaskID, it.Status)
		}
		if !dialect.Harbormaster().HasSpec(f.comments[it.Issue]) {
			t.Errorf("task %s (#%d) has no valid spec comment", it.TaskID, it.Issue)
		}
		if f.issues[it.Issue].HasLabel("local-agent") == false {
			t.Errorf("task %s not routed", it.TaskID)
		}
	}
	if len(f.comments[rep.Epic]) == 0 || !strings.Contains(f.comments[rep.Epic][len(f.comments[rep.Epic])-1], ReportSchemaVersion) {
		t.Error("projection report not posted on the epic")
	}
	// A-7's audit half: the pre-existing foreign issue is byte-untouched.
	for _, a := range f.audit {
		if strings.HasSuffix(a, "#1") {
			t.Errorf("foreign issue mutated: %s (DINV-7)", a)
		}
	}

	// Convergence: a second run adds nothing but the refreshed report.
	before := f.muts
	rep2, defects := Project(root, config.Defaults(), planRel, rc, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("re-run defects: %v", defects)
	}
	for _, it := range rep2.Items {
		if it.Status != "existing" {
			t.Errorf("re-run item %s status %q, want existing", it.TaskID, it.Status)
		}
	}
	if got := f.muts - before; got != 1 {
		t.Errorf("re-run performed %d mutations, want exactly 1 (the report refresh)", got)
	}
}

func TestNonFeatureIntakeProjectsThroughTheSameTracker(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	body, err := os.ReadFile("../plan/testdata/header/intake-001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(body), "plan-template/v1", "plan-template/v2")
	source = strings.ReplaceAll(source, "harbormaster/fs-spec:v1", "harbormaster/fs-spec:v2")
	source = strings.Replace(source, "| **Routing** | rung=2 |", "| **Requirements** | "+requirementstest.Input+" |", 1)
	rel := "docs/plans/maintenance/001-plan.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	intake := newFake()
	rep, defects := Project(root, config.Defaults(), rel, intake, dialect.Harbormaster(), opts())
	if len(defects) != 0 || rep == nil || len(rep.Items) != 1 || rep.Feature != "" || rep.Intake != "maintenance" {
		t.Fatalf("intake projection report=%+v defects=%v", rep, defects)
	}
	issue := intake.issues[rep.Items[0].Issue]
	if issue == nil || issue.Type != "Task" || len(issue.Labels) == 0 || !strings.Contains(issue.Body, "<!-- pyxis:plan-task maintenance/001/T1 -->") || strings.Contains(issue.Body, "**Design:**") {
		t.Fatalf("intake task issue=%+v", issue)
	}
	if epic := intake.issues[rep.Epic]; epic == nil || !strings.Contains(epic.Body, "**Intake:** maintenance") || strings.Contains(epic.Body, "**PRD:**") || strings.Contains(epic.Body, "**Design:**") {
		t.Fatalf("intake epic=%+v", epic)
	}
	comments := strings.Join(intake.comments[rep.Items[0].Issue], "\n")
	if !dialect.Harbormaster().HasSpec(intake.comments[rep.Items[0].Issue]) || strings.Contains(comments, `"design":""`) || strings.Contains(comments, "- design:") {
		t.Fatalf("intake spec=%s", comments)
	}
	face, err := rep.JSON()
	if err != nil || strings.Contains(string(face), `"feature"`) || !strings.Contains(string(face), `"intake": "maintenance"`) {
		t.Fatalf("intake report face=%s error=%v", face, err)
	}
	feature := newFake()
	featureRep, defects := Project(root, config.Defaults(), planRel, feature, dialect.Harbormaster(), opts())
	if len(defects) != 0 || featureRep == nil {
		t.Fatalf("feature comparison defects=%v", defects)
	}
	featureIssue := feature.issues[featureRep.Items[0].Issue]
	if issue.Type != featureIssue.Type || strings.Join(issue.Labels, ",") != strings.Join(featureIssue.Labels, ",") {
		t.Fatalf("issue carrier fields changed: intake=%+v feature=%+v", issue, featureIssue)
	}
}

func TestInterruptionConverges(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	clean := newFake()
	if _, d := Project(root, config.Defaults(), planRel, clean, dialect.Harbormaster(), opts()); len(d) != 0 {
		t.Fatalf("clean run defects: %v", d)
	}
	total := clean.muts

	for k := 1; k < total; k++ {
		f := newFake()
		f.failAfter = k
		if _, d := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(d) == 0 {
			t.Fatalf("cut %d: expected a halt defect", k)
		}
		f.failAfter = -1
		rep, d := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
		if len(d) != 0 {
			t.Fatalf("cut %d resume defects: %v", k, d)
		}
		// Exactly one issue per task, spec present, edges complete, routed.
		m := dialect.Harbormaster()
		for id, deps := range expectedEdges() {
			var carriers []int
			for n, is := range f.issues {
				if strings.Contains(is.Body, dialect.PlanTaskMarker(id)) {
					carriers = append(carriers, n)
				}
			}
			if len(carriers) != 1 {
				t.Fatalf("cut %d: %s has %d carriers %v (want exactly one — DINV-3)", k, id, len(carriers), carriers)
			}
			n := carriers[0]
			if !m.HasSpec(f.comments[n]) {
				t.Errorf("cut %d: %s missing spec after resume", k, id)
			}
			if got, want := len(f.blocked[n]), len(deps); got != want {
				t.Errorf("cut %d: %s has %d edges, want %d", k, id, got, want)
			}
			if !f.issues[n].HasLabel(m.RoutingLabel) {
				t.Errorf("cut %d: %s not routed after resume", k, id)
			}
		}
		if rep.Epic == 0 {
			t.Errorf("cut %d: no epic in resume report", k)
		}
	}
}

func TestDuplicateHaltsLoudly(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	f := newFake()
	marker := dialect.PlanTaskMarker("widget/001/T1")
	// The racing projections share the epic they found or created first.
	f.seed(29, tracker.Issue{Title: "[Epic] widget", Body: "body\n" + dialect.PlanEpicMarker("widget", "001")})
	f.seed(30, tracker.Issue{Title: "racer A", Body: "body\n" + marker})
	f.seed(31, tracker.Issue{Title: "racer B", Body: "body\n" + marker})

	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	found := false
	for _, d := range defects {
		found = found || strings.Contains(d.Reason, "raced")
	}
	if !found {
		t.Fatalf("expected a duplicate-race defect, got %v", defects)
	}
	if f.issues[30].HasLabel("local-agent") || f.issues[31].HasLabel("local-agent") {
		t.Error("a duplicated task was routed (DINV-3)")
	}
	if f.issues[30].State != "open" || f.issues[31].State != "open" {
		t.Error("a duplicate was closed — never auto-deleted (AP-17)")
	}
	for _, it := range rep.Items {
		if it.TaskID == "T1" && it.Status != "duplicate" {
			t.Errorf("T1 status %q, want duplicate", it.Status)
		}
		if it.TaskID == "T2" && it.Status != "created" {
			t.Errorf("T2 status %q, want created (non-duplicates proceed)", it.Status)
		}
	}
}

func TestFreezeRefusalAndDryRun(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	f := newFake()
	o := opts()
	o.MergedBytes = func(rel string) ([]byte, error) { return []byte("different frozen bytes"), nil }
	_, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) == 0 || !strings.Contains(defects[0].Reason, "merged form") {
		t.Fatalf("expected the freeze refusal, got %v", defects)
	}
	if f.muts != 0 {
		t.Errorf("freeze refusal still performed %d mutations (DINV-1)", f.muts)
	}

	o = opts()
	o.DryRun = true
	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) != 0 {
		t.Fatalf("dry-run defects: %v", defects)
	}
	if f.muts != 0 {
		t.Errorf("dry-run performed %d mutations", f.muts)
	}
	for _, it := range rep.Items {
		if it.Status != "dry-run" {
			t.Errorf("dry-run item status %q", it.Status)
		}
	}
}

func TestNoRoutingAndOperatorSurface(t *testing.T) {
	root := writeRepo(t, fixturePlan(".github/workflows/widget"))
	f := newFake()
	o := opts()
	o.NoRouting = true
	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) != 0 {
		t.Fatalf("defects: %v", defects)
	}
	if rep.Routing != "withheld" {
		t.Errorf("routing %q, want withheld", rep.Routing)
	}
	for _, is := range f.issues {
		if is.HasLabel("local-agent") || is.HasLabel("operator") {
			t.Errorf("issue #%d routed under --no-routing", is.Number)
		}
	}

	rep, defects = Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("defects: %v", defects)
	}
	if rep.Routing != "applied" {
		t.Errorf("routing %q, want applied", rep.Routing)
	}
	var t3 *tracker.Issue
	for _, is := range f.issues {
		if strings.Contains(is.Body, dialect.PlanTaskMarker("widget/001/T3")) {
			t3 = is
		}
	}
	if t3 == nil || !t3.HasLabel("operator") || t3.HasLabel("local-agent") {
		t.Errorf("workflow-touching task not operator-routed: %+v", t3)
	}
}

func TestProbeReport(t *testing.T) {
	root := writeRepo(t, fixturePlan("internal/widget/release"))
	f := newFake()
	if _, d := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(d) != 0 {
		t.Fatalf("defects: %v", d)
	}
	before := f.muts
	rep, defects := Probe(root, config.Defaults(), planRel, f, dialect.Harbormaster())
	if len(defects) != 0 {
		t.Fatalf("probe defects: %v", defects)
	}
	for _, it := range rep.Items {
		if it.Status != "existing" {
			t.Errorf("probe item %s status %q", it.TaskID, it.Status)
		}
	}
	if f.muts != before {
		t.Error("probe mutated the tracker")
	}
}
