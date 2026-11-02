package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

// fakeContinuity scripts Session/Record: the first call for a task starts a
// session, later calls resume it, and every launch is recorded.
type fakeContinuity struct {
	sessions map[string]string // task → id
	specs    []port.SessionSpec
	records  []port.RunResult
	harness  []string
	dirs     []string
	err      error
}

func (f *fakeContinuity) Session(_ context.Context, task port.Task, harness entity.Harness, worktree string) (port.SessionSpec, error) {
	f.harness = append(f.harness, harness.String())
	f.dirs = append(f.dirs, worktree)
	if f.err != nil {
		return port.SessionSpec{}, f.err
	}
	if f.sessions == nil {
		f.sessions = map[string]string{}
	}
	if id, ok := f.sessions[task.ID]; ok {
		spec := port.SessionSpec{ID: id, Resume: true}
		f.specs = append(f.specs, spec)
		return spec, nil
	}
	f.sessions[task.ID] = "sess-" + task.ID
	spec := port.SessionSpec{ID: "sess-" + task.ID}
	f.specs = append(f.specs, spec)
	return spec, nil
}

func (f *fakeContinuity) Record(_ context.Context, _ port.Task, _ entity.Harness, _ string, _ port.SessionSpec, res port.RunResult) error {
	f.records = append(f.records, res)
	return nil
}

type sessionEvents struct{ events []SessionEvent }

func (s *sessionEvents) observe(_ port.Task, e SessionEvent) { s.events = append(s.events, e) }

// The implementation launch starts the task's session; a remediation sublaunch
// resumes it; each finished launch is recorded; the reviewer never sees it.
func TestRun_SessionContinuityAcrossImplementationAndRemediation(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	runtime.result = port.RunResult{Tokens: 5, SessionID: "sess-T-42"}
	cont := &fakeContinuity{}
	obs := &sessionEvents{}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	task := newTestTask()
	task.Harness = "claude"
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(&fakeHeadReader{head: "sha-2"}), WithSessionContinuity(cont), WithSessionObserver(obs.observe))
	if outcome, err := w.Run(context.Background(), task); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	if len(runtime.launches) != 2 {
		t.Fatalf("launches = %d", len(runtime.launches))
	}
	first, second := runtime.launches[0], runtime.launches[1]
	if first.Session == nil || first.Session.Resume || first.Session.ID != "sess-T-42" {
		t.Fatalf("implementation launch session = %+v, want a fresh pre-assigned session", first.Session)
	}
	if second.Session == nil || !second.Session.Resume || second.Session.ID != "sess-T-42" {
		t.Fatalf("remediation launch session = %+v, want a resume of the same session", second.Session)
	}
	if first.RuntimeStateDir == "" || first.RuntimeStateDir == space.dir || second.RuntimeStateDir != first.RuntimeStateDir {
		t.Fatalf("implementation/remediation must share isolated native state: %q, %q", first.RuntimeStateDir, second.RuntimeStateDir)
	}
	if len(cont.records) != 2 || cont.records[0].SessionID != "sess-T-42" {
		t.Fatalf("records = %+v", cont.records)
	}
	if cont.harness[0] != "claude" || cont.dirs[0] != space.dir {
		t.Fatalf("continuity asked for harness %q in %q", cont.harness[0], cont.dirs[0])
	}
	if len(obs.events) != 2 || obs.events[0].Action != "started" || obs.events[0].Purpose != "implementation" || obs.events[1].Action != "resumed" || obs.events[1].Purpose != "remediation" {
		t.Fatalf("session events = %+v", obs.events)
	}
	for _, l := range runtime.launches {
		if l.Purpose == port.LaunchPurposeReview && l.Session != nil {
			t.Fatal("a review launch must never carry a session")
		}
	}
}

// A continuity failure degrades to a fresh launch, observably, and never fails
// the task; without continuity installed no session is ever set.
func TestRun_SessionContinuityDegradesAndIsOptional(t *testing.T) {
	deps, _, _, _, runtime, _ := greenDeps(t)
	cont := &fakeContinuity{err: errors.New("ledger locked")}
	obs := &sessionEvents{}
	task := newTestTask()
	task.Harness = "claude"
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithSessionContinuity(cont), WithSessionObserver(obs.observe))
	if outcome, err := w.Run(context.Background(), task); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	if runtime.launches[0].Session != nil {
		t.Fatal("a failed continuity must launch fresh")
	}
	if len(obs.events) != 1 || obs.events[0].Action != "degraded" || !strings.Contains(obs.events[0].Err.Error(), "ledger locked") {
		t.Fatalf("events = %+v", obs.events)
	}

	deps, _, _, _, runtime, _ = greenDeps(t)
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}))
	if _, err := w.Run(context.Background(), newTestTask()); err != nil {
		t.Fatal(err)
	}
	if runtime.launches[0].Session != nil {
		t.Fatal("no continuity installed ⇒ no session")
	}
}

// A feedback-round task carries its comments into the implementation launch
// spec (the brief assembler renders them) and resumes the session.
func TestRun_FeedbackRoundCarriesCommentsAndResumes(t *testing.T) {
	deps, _, _, _, runtime, _ := greenDeps(t)
	cont := &fakeContinuity{sessions: map[string]string{"T-42": "sess-old"}}
	task := newTestTask()
	task.Harness = "claude"
	task.Feedback = &port.ReviewFeedback{Proposal: 7, Round: 1, Comments: []port.Comment{{Author: "mdarabi", Body: "use errors.Is", CreatedAt: time.Now()}}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithSessionContinuity(cont))
	if _, _, err := executorruntime.PrepareTaskState(t.Context(), deps.Space.(*fakeWorkspace).dir, task.Harness); err != nil {
		t.Fatal(err)
	}
	if outcome, err := w.Run(context.Background(), task); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	l := runtime.launches[0]
	if l.Feedback == nil || l.Feedback.Proposal != 7 || len(l.Feedback.Comments) != 1 {
		t.Fatalf("launch feedback = %+v", l.Feedback)
	}
	if l.Session == nil || !l.Session.Resume || l.Session.ID != "sess-old" {
		t.Fatalf("feedback launch must resume the task's session: %+v", l.Session)
	}
}

func TestRun_OlderSessionStartsFreshInIsolatedStorage(t *testing.T) {
	deps, _, _, _, runtime, _ := greenDeps(t)
	cont := &fakeContinuity{sessions: map[string]string{"T-42": "sess-old"}}
	obs := &sessionEvents{}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithSessionContinuity(cont), WithSessionObserver(obs.observe))
	if outcome, err := w.Run(t.Context(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	launch := runtime.launches[0]
	if launch.Session == nil || launch.Session.Resume || launch.RuntimeStateDir == "" {
		t.Fatalf("older conversation must start fresh in isolated storage: %+v", launch)
	}
	found := false
	for _, event := range obs.events {
		if event.Action == "degraded" && event.Err != nil && strings.Contains(event.Err.Error(), "predates isolated runtime state") {
			found = true
		}
	}
	if !found || len(cont.records) != 1 {
		t.Fatalf("migration must be observable and record the new conversation: events=%+v records=%+v", obs.events, cont.records)
	}
}

// The fixer's driver receives the same continuity once the harness is known.
type sessionAwareDriver struct {
	fakeGreenDriver
	got     SessionContinuity
	harness string
}

func (d *sessionAwareDriver) UseSessionContinuity(c SessionContinuity, harness entity.Harness, _ SessionObserver) {
	d.got, d.harness = c, harness.String()
}

func TestRun_HandsContinuityToTheGreenDriver(t *testing.T) {
	deps, _, _, _, _, _ := greenDeps(t)
	cont := &fakeContinuity{}
	driver := &sessionAwareDriver{}
	task := newTestTask()
	task.Harness = "codex"
	w := mustWorker(t, deps, WithGreenDriver(driver), WithSessionContinuity(cont))
	if _, err := w.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if driver.got == nil || driver.harness != "codex" {
		t.Fatalf("driver got continuity=%v harness=%q", driver.got != nil, driver.harness)
	}
}
