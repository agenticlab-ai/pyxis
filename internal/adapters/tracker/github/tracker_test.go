package ghtracker

import (
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// fakeClient is a hermetic stand-in for the GitHub surface: it returns recorded
// responses and records calls, so the adapter's filter + mapping logic is
// tested with no network and no gh.
type fakeClient struct {
	issues    []item        // response for openIssuesWithLabel
	issuesErr error         // error for openIssuesWithLabel
	blockers  map[int][]int // blockedBy response, keyed by issue number
	blockErr  error         // error for blockedBy
	blocked   []int         // issue numbers whose blockers were read
	specs     map[int][]string
	specErr   error
	specReads []int

	labelQueried string       // the label the adapter filtered by
	openCalls    int          // openIssuesWithLabel invocation count
	posted       []commentArg // recorded Comment calls
	commentErr   error
}

type commentArg struct {
	number int
	body   string
}

func (f *fakeClient) openIssuesWithLabel(_ context.Context, label string) ([]item, error) {
	f.openCalls++
	f.labelQueried = label
	return f.issues, f.issuesErr
}

func (f *fakeClient) blockedBy(_ context.Context, number int) ([]int, error) {
	f.blocked = append(f.blocked, number)
	if f.blockErr != nil {
		return nil, f.blockErr
	}
	return f.blockers[number], nil
}

func (f *fakeClient) comments(_ context.Context, number int) ([]string, error) {
	f.specReads = append(f.specReads, number)
	if f.specErr != nil {
		return nil, f.specErr
	}
	if f.specs == nil {
		id := fmt.Sprintf("T%d", number)
		for _, issue := range f.issues {
			if issue.Number == number && strings.HasPrefix(issue.Title, "[T") {
				id, _, _ = strings.Cut(strings.TrimPrefix(issue.Title, "["), "]")
			}
		}
		spec, err := taskspectest.Spec(id)
		if err != nil {
			return nil, err
		}
		body, err := taskspec.RenderNative(spec)
		return []string{body}, err
	}
	return f.specs[number], nil
}

func TestFrontier_PlanScopeExcludesOtherPlansBeforeBlockerReads(t *testing.T) {
	marker := func(ref string) string { return "charter\n\n<!-- pyxis:plan-task " + ref + " -->\n" }
	fc := &fakeClient{
		issues: []item{
			{Number: 21, Title: "[T1] target", Body: marker("routing/002/T1"), State: "open", Labels: []string{"local-agent"}},
			{Number: 22, Title: "[T1] other plan", Body: marker("other/001/T1"), State: "open", Labels: []string{"local-agent"}},
			{Number: 23, Title: "[T2] unscoped", Body: "ordinary issue", State: "open", Labels: []string{"local-agent"}},
			{Number: 24, Title: "[T3] malformed", Body: marker("routing/002/not-a-task"), State: "open", Labels: []string{"local-agent"}},
		},
		blockers: map[int][]int{21: {20}},
	}
	tr, err := newTrackerForPlan(fc, "local-agent", "routing/002")
	if err != nil {
		t.Fatalf("new scoped tracker: %v", err)
	}

	got, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(got) != 1 || got[0].Number != 21 {
		t.Fatalf("scoped frontier = %+v, want only target issue #21", got)
	}
	if !reflect.DeepEqual(fc.blocked, []int{21}) {
		t.Fatalf("blocker reads = %v, want target issue only", fc.blocked)
	}
	if !reflect.DeepEqual(fc.specReads, []int{21}) {
		t.Fatalf("spec reads = %v, want target issue only", fc.specReads)
	}
}

func TestPlanScopeRefusesMalformedReference(t *testing.T) {
	for _, ref := range []string{"", "routing", "routing/02", "routing/latest", "routing/002/T1", "../routing/002", "routing/../002", "-routing/002"} {
		t.Run(ref, func(t *testing.T) {
			if _, err := newTrackerForPlan(&fakeClient{}, "local-agent", ref); err == nil {
				t.Fatalf("newTrackerForPlan(%q) succeeded, want refusal", ref)
			}
		})
	}
}

func (f *fakeClient) comment(_ context.Context, number int, body string) error {
	f.posted = append(f.posted, commentArg{number, body})
	return f.commentErr
}

// A-of-T5(1): Frontier returns exactly the open, routing-tagged, non-PR items
// — the adapter re-verifies the contract even when the client over-returns
// (a closed item, an untagged item, a PR), and maps each to a port.Task with
// its ID parsed from the "[Tn] …" title, body as the brief, and blocker edges.
func TestFrontier_ReturnsExactlyOpenTagged(t *testing.T) {
	fc := &fakeClient{
		issues: []item{
			{Number: 11, Title: "[T1] Ports and worker", Body: "charter one", State: "open", Labels: []string{"kind:feat", "local-agent"}},
			{Number: 12, Title: "[T2] Dispatcher loop", Body: "charter two", State: "open", Labels: []string{"local-agent"}},
			{Number: 13, Title: "[T3] Already done", Body: "closed", State: "closed", Labels: []string{"local-agent"}},     // closed → excluded
			{Number: 14, Title: "[T4] Operator work", Body: "human", State: "open", Labels: []string{"operator"}},          // untagged → excluded
			{Number: 15, Title: "[T5] A proposal", Body: "pr", State: "open", Labels: []string{"local-agent"}, IsPR: true}, // PR → excluded
			{Number: 16, Title: "No bracket title", Body: "charter six", State: "open", Labels: []string{"local-agent"}},   // tagged, no ID prefix
		},
		blockers: map[int][]int{12: {11}, 16: {11, 12}},
		specs: map[int][]string{
			11: {taskspectest.Marker(t, "T1")},
			12: {taskspectest.Marker(t, "T2")},
			16: {taskspectest.Marker(t, "T16")},
		},
	}
	tr := newTracker(fc, "local-agent")

	got, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}

	want := []port_Task{
		{ID: "T1", Number: 11, SourceRef: "#11", Title: "Ports and worker", Brief: "charter one", BlockedBy: nil},
		{ID: "T2", Number: 12, SourceRef: "#12", Title: "Dispatcher loop", Brief: "charter two", BlockedBy: []int{11}},
		{ID: "T16", Number: 16, SourceRef: "#16", Title: "No bracket title", Brief: "charter six", BlockedBy: []int{11, 12}},
	}
	if got := taskShapes(got); !reflect.DeepEqual(got, want) {
		t.Errorf("Frontier mismatch:\n got %+v\nwant %+v", got, want)
	}
	if fc.labelQueried != "local-agent" {
		t.Errorf("filtered by label %q, want local-agent", fc.labelQueried)
	}
}

// The routing label is configurable (AP-8); an empty label defaults to the
// harbormaster dialect's tag.
func TestFrontier_DefaultAndCustomLabel(t *testing.T) {
	fc := &fakeClient{}
	if _, err := newTracker(fc, "").Frontier(context.Background()); err != nil {
		t.Fatalf("default label Frontier: %v", err)
	}
	if fc.labelQueried != DefaultRoutingLabel {
		t.Errorf("empty label queried %q, want default %q", fc.labelQueried, DefaultRoutingLabel)
	}
	fc2 := &fakeClient{}
	if _, err := newTracker(fc2, "agent-eligible").Frontier(context.Background()); err != nil {
		t.Fatalf("custom label Frontier: %v", err)
	}
	if fc2.labelQueried != "agent-eligible" {
		t.Errorf("custom label queried %q, want agent-eligible", fc2.labelQueried)
	}
}

// A-of-T5(3): the stale-read fixture. The adapter holds no cache — a second
// Frontier reflects tracker state that changed between calls, and each call
// re-reads through the client (AP-15, the fresh path, not a memoized result).
func TestFrontier_FreshReadNeverCached(t *testing.T) {
	fc := &fakeClient{
		issues:   []item{{Number: 11, Title: "[T1] one", Body: "b1", State: "open", Labels: []string{"local-agent"}}},
		blockers: map[int][]int{},
	}
	tr := newTracker(fc, "local-agent")

	first, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("first Frontier: %v", err)
	}
	if len(first) != 1 || first[0].Number != 11 {
		t.Fatalf("first frontier = %+v, want single #11", first)
	}

	// The tracker advances: #11 is claimed away and #12 opens. A cached adapter
	// would still report the stale #11-only set.
	fc.issues = []item{{Number: 12, Title: "[T2] two", Body: "b2", State: "open", Labels: []string{"local-agent"}}}

	second, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("second Frontier: %v", err)
	}
	if len(second) != 1 || second[0].Number != 12 {
		t.Fatalf("second frontier = %+v, want fresh single #12 (stale read would keep #11)", second)
	}
	if fc.openCalls != 2 {
		t.Errorf("openIssuesWithLabel called %d times, want 2 (one exact read per Frontier)", fc.openCalls)
	}
}

func TestFrontier_PropagatesReadError(t *testing.T) {
	sentinel := errors.New("gh boom")
	tr := newTracker(&fakeClient{issuesErr: sentinel}, "local-agent")
	if _, err := tr.Frontier(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("Frontier error = %v, want wrapped sentinel", err)
	}
}

func TestFrontier_PropagatesBlockerError(t *testing.T) {
	sentinel := errors.New("deps down")
	fc := &fakeClient{
		issues:   []item{{Number: 11, Title: "[T1] one", State: "open", Labels: []string{"local-agent"}}},
		blockErr: sentinel,
	}
	if _, err := newTracker(fc, "local-agent").Frontier(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("Frontier error = %v, want wrapped blocker sentinel", err)
	}
}

func TestFrontier_PropagatesSpecReadError(t *testing.T) {
	sentinel := errors.New("comments down")
	fc := &fakeClient{
		issues:  []item{{Number: 11, Title: "[T1] one", State: "open", Labels: []string{"local-agent"}}},
		specErr: sentinel,
	}
	if _, err := newTracker(fc, "local-agent").Frontier(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("Frontier error = %v, want wrapped spec-read sentinel", err)
	}
	if len(fc.blocked) != 0 {
		t.Fatalf("blockers read after spec-read failure: %v", fc.blocked)
	}
}

func TestFrontier_MissingSpecFailsBeforeBlockerRead(t *testing.T) {
	fc := &fakeClient{
		issues: []item{{Number: 11, Title: "[T1] one", State: "open", Labels: []string{"local-agent"}}},
		specs:  map[int][]string{11: nil},
	}
	if _, err := newTracker(fc, "local-agent").Frontier(context.Background()); err == nil || !strings.Contains(err.Error(), "no fs-spec:v2 capsule") {
		t.Fatalf("Frontier error = %v, want missing-capsule refusal", err)
	}
	if len(fc.blocked) != 0 {
		t.Fatalf("blockers read after capsule refusal: %v", fc.blocked)
	}
}

func TestComment_Delegates(t *testing.T) {
	fc := &fakeClient{}
	if err := newTracker(fc, "local-agent").Comment(context.Background(), 42, "opened proposal #7"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
	if len(fc.posted) != 1 || fc.posted[0] != (commentArg{42, "opened proposal #7"}) {
		t.Errorf("recorded comments = %+v, want single {42, ...}", fc.posted)
	}
}

func TestComment_PropagatesError(t *testing.T) {
	sentinel := errors.New("no such issue")
	err := newTracker(&fakeClient{commentErr: sentinel}, "local-agent").Comment(context.Background(), 1, "x")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Comment error = %v, want wrapped sentinel", err)
	}
}

// port_Task and taskShapes reduce a []port.Task to the fields this adapter
// populates, keeping the golden comparison focused (Backend/HeadBranch are the
// Dispatcher's to fill at dispatch; Rung comes from the frozen task capsule).
type port_Task struct {
	ID        string
	Number    int
	SourceRef string
	Title     string
	Brief     string
	BlockedBy []int
}

func taskShapes(tasks []port.Task) []port_Task {
	out := make([]port_Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, port_Task{ID: t.ID, Number: t.Number, SourceRef: t.SourceRef, Title: t.Title, Brief: t.Brief, BlockedBy: t.BlockedBy})
	}
	return out
}
