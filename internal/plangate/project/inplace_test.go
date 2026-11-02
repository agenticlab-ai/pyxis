package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

// changePlanInPlace rewrites the plan source the way a merged contract
// change does: same plan version, the first task's goal edited.
func changePlanInPlace(t *testing.T, root, to string) {
	const from = "**Goal:** "
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(planRel))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(body), from, to, 1)
	if changed == string(body) {
		t.Fatalf("fixture lacks %q", from)
	}
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
}

func specOf(t *testing.T, comments []string) taskspec.Spec {
	t.Helper()
	spec, err := taskspec.Parse(comments)
	if err != nil {
		t.Fatalf("spec unreadable: %v", err)
	}
	return spec
}

func TestPlanChangedInPlaceUpdatesItsTasksWithoutNewIssues(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("first projection: %v", defects)
	}
	issues := len(f.issues)
	t1 := first.Items[0].Issue
	before := specOf(t, f.comments[t1])
	changePlanInPlace(t, root, "**Goal:** Revised by a contract change. ")

	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("in-place projection: %v", defects)
	}
	if len(f.issues) != issues || rep.Epic != first.Epic {
		t.Fatalf("in-place change created issues: %d → %d, epic %d → %d", issues, len(f.issues), first.Epic, rep.Epic)
	}
	if rep.Items[0].Issue != t1 || rep.Items[0].Status != "updated" {
		t.Fatalf("T1 item = %+v", rep.Items[0])
	}
	if !strings.Contains(f.issues[t1].Body, "Revised by a contract change.") {
		t.Fatalf("T1 body kept the old charter:\n%s", f.issues[t1].Body)
	}
	capsules := 0
	for _, c := range f.comments[t1] {
		if strings.Contains(c, "<!-- fs-spec:") {
			capsules++
		}
	}
	after := specOf(t, f.comments[t1])
	if capsules != 1 || !strings.HasPrefix(after.Goal, "Revised by a contract change.") || after.PlanFingerprint == before.PlanFingerprint {
		t.Fatalf("T1 spec not replaced in place: capsules=%d goal=%q", capsules, after.Goal)
	}
	for _, it := range rep.Items {
		if !f.issues[it.Issue].HasLabel("local-agent") {
			t.Fatalf("task %s lost its routing", it.TaskID)
		}
	}
	again, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatalf("re-run: %v", defects)
	}
	for _, it := range again.Items {
		if it.Status != "existing" {
			t.Fatalf("re-run after the update changed %s again: %s", it.TaskID, it.Status)
		}
	}
}

func TestPlanChangedInPlaceDryRunWritesNothing(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
		t.Fatal(defects)
	}
	changePlanInPlace(t, root, "**Goal:** Revised. ")
	before := f.muts
	o := opts()
	o.DryRun = true
	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) != 0 || f.muts != before || rep.Items[0].Notes != "charter would update" {
		t.Fatalf("dry run: defects=%v mutations=%d item=%+v", defects, f.muts-before, rep.Items[0])
	}
}

type immutableComments struct{ *fake }

func (immutableComments) ReplaceComment(int, string, string) error {
	return tracker.ErrInPlaceUnsupported
}

func TestPlanChangedInPlaceOnAnImmutableTrackerNamesTheRemedy(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
		t.Fatal(defects)
	}
	changePlanInPlace(t, root, "**Goal:** Revised. ")
	_, defects := Project(root, config.Defaults(), planRel, immutableComments{f}, dialect.Harbormaster(), opts())
	if len(defects) != 1 || !strings.Contains(defects[0].Remedy, "new plan version") {
		t.Fatalf("defects = %v", defects)
	}
}

func TestPlanChangedInPlaceRefusesAnAmbiguousSpec(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	t1 := first.Items[0].Issue
	f.comments[t1] = append(f.comments[t1], strings.Replace(f.comments[t1][0], "complex", "simple", 1))
	changePlanInPlace(t, root, "**Goal:** Revised. ")
	before := len(f.comments[t1])
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) == 0 {
		t.Fatal("two spec comments were resolved by guessing")
	}
	if len(f.comments[t1]) != before {
		t.Fatal("ambiguous spec comments were changed")
	}
}

func TestPlanChangedInPlaceUpdatesTheEpicBody(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	path := filepath.Join(root, filepath.FromSlash(planRel))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(string(body), "### T1 — Define the widget model", "### T1 — Define the durable widget model", 1)
	if renamed == string(body) {
		t.Fatal("fixture lacks T1's heading")
	}
	if err := os.WriteFile(path, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}
	o := opts()
	o.DryRun = true
	before := f.muts
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o); len(defects) != 0 || f.muts != before {
		t.Fatalf("dry run wrote: defects=%v mutations=%d", defects, f.muts-before)
	}
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
		t.Fatal(defects)
	}
	if !strings.Contains(f.issues[first.Epic].Body, "T1 — Define the durable widget model") {
		t.Fatalf("epic body kept the old task list:\n%s", f.issues[first.Epic].Body)
	}
	if got := f.issues[first.Items[0].Issue].Title; got != "[T1] Define the durable widget model" {
		t.Fatalf("T1 kept its old title %q", got)
	}
}

type failingInPlace struct {
	*fake
	editBody, replace, readback bool
	replaced                    bool
}

func (f *failingInPlace) EditIssue(issue int, title, body string) error {
	if f.editBody {
		return errors.New("edit refused")
	}
	return f.fake.EditIssue(issue, title, body)
}

func (f *failingInPlace) ReplaceComment(issue int, old, body string) error {
	if f.replace {
		return errors.New("replace refused")
	}
	f.replaced = true
	return f.fake.ReplaceComment(issue, old, body)
}

func (f *failingInPlace) Comments(issue int) ([]string, error) {
	if f.readback && f.replaced {
		return nil, errors.New("readback unavailable")
	}
	return f.fake.Comments(issue)
}

func TestPlanChangedInPlaceHaltsWhenAnUpdateFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    func(*fake) *failingInPlace
		want string
	}{
		{"body", func(f *fake) *failingInPlace { return &failingInPlace{fake: f, editBody: true} }, "edit refused"},
		{"spec", func(f *fake) *failingInPlace { return &failingInPlace{fake: f, replace: true} }, "replace refused"},
		{"readback", func(f *fake) *failingInPlace { return &failingInPlace{fake: f, readback: true} }, "readback unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeRepo(t, nativeSource())
			f := newFake()
			if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
				t.Fatal(defects)
			}
			changePlanInPlace(t, root, "**Goal:** Revised. ")
			_, defects := Project(root, config.Defaults(), planRel, tc.f(f), dialect.Harbormaster(), opts())
			if len(defects) == 0 || !strings.Contains(defects[0].Reason, tc.want) {
				t.Fatalf("defects = %v, want %q", defects, tc.want)
			}
		})
	}
	if !sameBody("a\r\nb  \n\n", "a\nb") || sameBody("a", "b") {
		t.Fatal("body comparison does not ignore tracker line endings only")
	}
}

// rerouteT3 changes T3 in place the way a contract change may: it now
// touches a surface only the operator can push, and no longer waits on T1.
func rerouteT3(t *testing.T, root string) {
	t.Helper()
	source := fixturePlan(".github/workflows/release.yml")
	at := strings.Index(source, "### T3")
	changed := source[:at] + strings.Replace(source[at:], "| **Blocked by** | T1 |", "| **Blocked by** | — |", 1)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(planRel)), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanChangedInPlaceMovesRoutingAndDropsStaleEdges(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	t1, t3 := first.Items[0].Issue, first.Items[2].Issue
	if !f.blocked[t3][t1] || !f.issues[t3].HasLabel("local-agent") {
		t.Fatalf("fixture: T3 blocked-by=%v labels=%v", f.blocked[t3], f.issues[t3].Labels)
	}
	f.seed(7, tracker.Issue{Title: "outside work"})
	f.blocked[t3][7] = true // a human's edge to an issue outside the plan
	rerouteT3(t, root)

	o := opts()
	o.DryRun = true
	before := f.muts
	dry, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) != 0 || f.muts != before || dry.Items[2].Notes != "routing would change" {
		t.Fatalf("dry run: defects=%v mutations=%d item=%+v", defects, f.muts-before, dry.Items[2])
	}
	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	if rep.Items[2].Status != "updated" || f.issues[t3].HasLabel("local-agent") || !f.issues[t3].HasLabel("operator") {
		t.Fatalf("T3 routing not moved: item=%+v labels=%v", rep.Items[2], f.issues[t3].Labels)
	}
	if f.blocked[t3][t1] || !f.blocked[t3][7] {
		t.Fatalf("T3 edges = %v, want only the outside edge kept", f.blocked[t3])
	}
	if !f.blocked[first.Items[1].Issue][t1] {
		t.Fatal("T2's unchanged edge was removed")
	}
}

type failingWithdrawal struct {
	*fake
	label, edge bool
}

func (f *failingWithdrawal) RemoveLabel(issue int, label string) error {
	if f.label {
		return errors.New("label withdrawal refused")
	}
	return f.fake.RemoveLabel(issue, label)
}

func (f *failingWithdrawal) RemoveBlockedBy(issue, blocker int) error {
	if f.edge {
		return errors.New("edge removal refused")
	}
	return f.fake.RemoveBlockedBy(issue, blocker)
}

func TestPlanChangedInPlaceHaltsWhenAWithdrawalFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    func(*fake) *failingWithdrawal
		want string
	}{
		{"routing", func(f *fake) *failingWithdrawal { return &failingWithdrawal{fake: f, label: true} }, "label withdrawal refused"},
		{"edge", func(f *fake) *failingWithdrawal { return &failingWithdrawal{fake: f, edge: true} }, "edge removal refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeRepo(t, nativeSource())
			f := newFake()
			if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
				t.Fatal(defects)
			}
			rerouteT3(t, root)
			_, defects := Project(root, config.Defaults(), planRel, tc.f(f), dialect.Harbormaster(), opts())
			if len(defects) == 0 || !strings.Contains(defects[0].Reason, tc.want) {
				t.Fatalf("defects = %v, want %q", defects, tc.want)
			}
		})
	}
}

// unroutedWhileChanging fails the test when a task's charter, spec, or
// edges change while the task still carries a routing label.
type unroutedWhileChanging struct {
	*fake
	t *testing.T
}

func (u unroutedWhileChanging) routed(issue int, op string) {
	if is := u.issues[issue]; is.HasLabel("local-agent") || is.HasLabel("operator") {
		u.t.Errorf("%s on #%d while it is still routed (%v)", op, issue, is.Labels)
	}
}

func (u unroutedWhileChanging) EditIssue(issue int, title, body string) error {
	u.routed(issue, "charter edit")
	return u.fake.EditIssue(issue, title, body)
}

func (u unroutedWhileChanging) ReplaceComment(issue int, old, body string) error {
	u.routed(issue, "spec replacement")
	return u.fake.ReplaceComment(issue, old, body)
}

func (u unroutedWhileChanging) RemoveBlockedBy(issue, blocker int) error {
	u.routed(issue, "edge removal")
	return u.fake.RemoveBlockedBy(issue, blocker)
}

func TestPlanChangedInPlaceUnroutesEachTaskWhileItChanges(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	t1 := first.Items[0].Issue
	changePlanInPlace(t, root, "**Goal:** Revised by a contract change. ")
	rep, defects := Project(root, config.Defaults(), planRel, unroutedWhileChanging{f, t}, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	for _, it := range rep.Items {
		if !f.issues[it.Issue].HasLabel("local-agent") {
			t.Fatalf("task %s was not routed again: %v", it.TaskID, f.issues[it.Issue].Labels)
		}
	}
	// The plan's fingerprint is in every spec, so every task changes.
	for _, it := range rep.Items {
		if it.Status != "updated" {
			t.Fatalf("items = %+v", rep.Items)
		}
	}

	// A projection that stops after the withdrawal leaves the task unrouted.
	changePlanInPlace(t, root, "**Goal:** Revised again. ")
	broken := &failingInPlace{fake: f, editBody: true}
	if _, defects := Project(root, config.Defaults(), planRel, broken, dialect.Harbormaster(), opts()); len(defects) == 0 {
		t.Fatal("a failed charter edit passed")
	}
	if f.issues[t1].HasLabel("local-agent") {
		t.Fatal("a half-updated task stayed dispatchable")
	}
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 || !f.issues[t1].HasLabel("local-agent") {
		t.Fatalf("re-run did not converge: defects=%v labels=%v", defects, f.issues[t1].Labels)
	}
}

func TestPlanChangedInPlaceReclassifiesATask(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	t1 := first.Items[0].Issue
	f.issues[t1].Labels = append(f.issues[t1].Labels, "mechanical") // T1 was mechanical before the change

	o := opts()
	o.DryRun = true
	before := f.muts
	dry, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), o)
	if len(defects) != 0 || f.muts != before || dry.Items[0].Notes != "classification would change" {
		t.Fatalf("dry run: defects=%v mutations=%d item=%+v", defects, f.muts-before, dry.Items[0])
	}
	broken := &failingWithdrawal{fake: f, label: true}
	if _, defects := Project(root, config.Defaults(), planRel, broken, dialect.Harbormaster(), opts()); len(defects) == 0 || !strings.Contains(defects[0].Op, "classify T1") {
		t.Fatalf("a failed reclassification passed: %v", defects)
	}
	rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 || f.issues[t1].HasLabel("mechanical") || rep.Items[0].Status != "updated" || !f.issues[t1].HasLabel("local-agent") {
		t.Fatalf("T1 not reclassified: defects=%v item=%+v labels=%v", defects, rep.Items[0], f.issues[t1].Labels)
	}
}

func TestPlanChangedInPlaceMovesARouteWhateverItsLabelCase(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	t3 := first.Items[2].Issue
	for i, l := range f.issues[t3].Labels {
		if l == "local-agent" {
			f.issues[t3].Labels[i] = "Local-Agent" // a Forgejo repository's existing spelling
		}
	}
	rerouteT3(t, root)
	if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts()); len(defects) != 0 {
		t.Fatal(defects)
	}
	if f.issues[t3].HasLabel("local-agent") || !f.issues[t3].HasLabel("operator") {
		t.Fatalf("T3 kept its worker route: %v", f.issues[t3].Labels)
	}
}

func TestProjectionRefusesTasksWhoseEpicWasClosed(t *testing.T) {
	root := writeRepo(t, nativeSource())
	f := newFake()
	first, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	f.issues[first.Epic].State = "closed"
	changePlanInPlace(t, root, "**Goal:** Revised. ")
	issues, before := len(f.issues), f.muts
	_, defects = Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts())
	if len(defects) != 1 || !strings.Contains(defects[0].Reason, "epic is not open") || len(f.issues) != issues || f.muts != before {
		t.Fatalf("closed epic replaced: defects=%v issues %d → %d mutations=%d", defects, issues, len(f.issues), f.muts-before)
	}
}
