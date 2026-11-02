package feedback_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

// fakeRepo is a hermetic Repo: the observation facts are fixture data, so the
// disposition decision is exercised with no ambient git, clock, or filesystem
// (the Go testing discipline — inject the port, never the real repo).
type fakeRepo struct {
	resolves  map[string]bool             // subject anchor -> still resolves
	landings  map[string]feedback.Landing // edit-commit -> observed landing
	finalized map[string]string           // subject stem -> finalizing ref ("" = not finalized)
	err       error                       // when set, every question errors (observation-failure path)
}

func (r fakeRepo) SubjectResolves(anchor string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	v, ok := r.resolves[anchor]
	return ok && v, nil
}

func (r fakeRepo) EditLanded(editCommit string) (feedback.Landing, error) {
	if r.err != nil {
		return feedback.Landing{}, r.err
	}
	return r.landings[editCommit], nil
}

func (r fakeRepo) SubjectFinalized(subject string) (bool, string, error) {
	if r.err != nil {
		return false, "", r.err
	}
	ref, ok := r.finalized[subject]
	return ok && ref != "", ref, nil
}

// captured builds an open (un-disposed) Captured with the given anchor and
// recorded edit-commit — the shape the fold hands the observer.
func captured(seq, anchor, editCommit string) *feedback.Captured {
	return &feedback.Captured{
		Entry:         seq,
		Source:        "conversation",
		SubjectAnchor: anchor,
		EditCommit:    editCommit,
		SourceFile:    seq + "-conversation.md",
	}
}

// A-3 (acceptance 1): a landed review edit closes its entry `applied` with a
// stable outcome-ref naming the observed commit, and freezes edit_distance.
func TestObserveApplied(t *testing.T) {
	repo := fakeRepo{
		resolves: map[string]bool{"some-doc#3.1": true},
		landings: map[string]feedback.Landing{
			"abc123": {Landed: true, Outcome: "def789", EditSize: 4},
		},
	}
	got, err := feedback.Observe([]*feedback.Captured{captured("001", "some-doc#3.1", "abc123")}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Resolved() {
		t.Fatalf("want one resolved transition, got %+v", got)
	}
	d := got[0].Disposition
	if d.Disposition != "applied" {
		t.Fatalf("disposition = %q, want applied", d.Disposition)
	}
	if d.Outcome != "def789" {
		t.Fatalf("outcome = %q, want the stable landed ref def789 (DINV-4)", d.Outcome)
	}
	if !strings.Contains(d.ResolvedBy, "def789") || d.ResolvedBy == "" {
		t.Fatalf("resolved-by %q must name the observed event (DINV-3)", d.ResolvedBy)
	}
	if d.EditDistance == nil || *d.EditDistance != 4 {
		t.Fatalf("edit_distance = %v, want 4 frozen at applied-write (§3.6)", d.EditDistance)
	}
}

// A-3 (acceptance 2, no-action + pending + stale).
func TestObserveNoActionPendingStale(t *testing.T) {
	entries := []*feedback.Captured{
		captured("001", "concluded-doc#1", ""),  // no edit, subject finalized -> no-action
		captured("002", "open-doc#2", "pend01"), // edit not landed, not finalized -> pending
		captured("003", "gone-doc#3", "x"),      // subject anchor no longer resolves -> stale
	}
	repo := fakeRepo{
		resolves: map[string]bool{"concluded-doc#1": true, "open-doc#2": true, "gone-doc#3": false},
		landings: map[string]feedback.Landing{"pend01": {Landed: false}},
		finalized: map[string]string{
			"concluded-doc": "sha-final-01",
			"open-doc":      "", // not finalized
		},
	}
	got, err := feedback.Observe(entries, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 transitions, got %d", len(got))
	}

	// 001 -> no-action, citing the finalizing event (UB-3).
	na := got[0]
	if !na.Resolved() || na.Disposition.Disposition != "no-action" {
		t.Fatalf("001 want no-action, got %+v", na)
	}
	if na.Disposition.Outcome != "sha-final-01" || na.Disposition.ResolvedBy == "" {
		t.Fatalf("no-action must name a stable finalizing event: %+v", na.Disposition)
	}

	// 002 -> pending, no disposition written (UB-5).
	if got[1].Resolved() || got[1].Flag != feedback.FlagPending {
		t.Fatalf("002 want pending flag, got %+v", got[1])
	}

	// 003 -> stale, retained + flagged, no disposition (UB-6).
	if got[2].Resolved() || got[2].Flag != feedback.FlagStale {
		t.Fatalf("003 want stale flag, got %+v", got[2])
	}
}

// An entry whose edit-commit is recorded but has not yet landed, under a
// not-yet-finalized subject, stays pending — never a premature applied/no-action.
func TestObservePendingUnlandedEdit(t *testing.T) {
	repo := fakeRepo{
		resolves:  map[string]bool{"doc#1": true},
		landings:  map[string]feedback.Landing{"c1": {Landed: false}},
		finalized: map[string]string{"doc": ""},
	}
	got, _ := feedback.Observe([]*feedback.Captured{captured("001", "doc#1", "c1")}, repo)
	if got[0].Resolved() || got[0].Flag != feedback.FlagPending {
		t.Fatalf("unlanded edit under open subject must stay pending, got %+v", got[0])
	}
}

// An observation-facts error never closes an entry — the disposition fails
// closed on the unknown (DINV-3): the pass errors rather than guess.
func TestObserveFailsClosedOnRepoError(t *testing.T) {
	repo := fakeRepo{err: os.ErrPermission}
	if _, err := feedback.Observe([]*feedback.Captured{captured("001", "doc#1", "c1")}, repo); err == nil {
		t.Fatal("a repo-observation error must surface, never silently close an entry")
	}
}

// An already-disposed entry is terminal — the observer never re-observes it.
func TestObserveSkipsResolved(t *testing.T) {
	e := captured("001", "doc#1", "c1")
	e.Disposition = &feedback.Disposition{Disposition: "applied", Outcome: "x", ResolvedBy: "y"}
	repo := fakeRepo{resolves: map[string]bool{"doc#1": true}}
	got, err := feedback.Observe([]*feedback.Captured{e}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a resolved entry must not be re-observed, got %+v", got)
	}
}

// A-3 (acceptance 3): RenderDisposition refuses to set a disposition without an
// observed resolving event — no code path sets a disposition without one.
func TestRenderDispositionRequiresResolvingEvent(t *testing.T) {
	if _, err := feedback.RenderDisposition(feedback.Disposition{Disposition: "applied", Outcome: "sha"}); err == nil {
		t.Fatal("a disposition with no Resolved-by must be rejected (DINV-3)")
	}
	if _, err := feedback.RenderDisposition(feedback.Disposition{Disposition: "applied", ResolvedBy: "e"}); err == nil {
		t.Fatal("a disposition with no outcome-ref must be rejected (DINV-4)")
	}
	if _, err := feedback.RenderDisposition(feedback.Disposition{Disposition: "maybe", Outcome: "s", ResolvedBy: "e"}); err == nil {
		t.Fatal("a disposition outside the closed set must be rejected")
	}
}

// The full path: observe -> apply -> reparse. The terminal transition is
// appended to the entry file, the capture block is left byte-for-byte intact
// (DINV-1), and the appended disposition round-trips through the fold.
func TestApplyTransitionsAppendsAndPreserves(t *testing.T) {
	dir := t.TempDir()
	entryPath := filepath.Join(dir, "001-conversation.md")
	capture := `# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @reviewer-one |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | some-doc#3.1 |
| **Edit-commit** | abc123 |
| **Template version** | review-feedback-entry/v1 |

## Comment

Fix the typo in the header.
`
	if err := os.WriteFile(entryPath, []byte(capture), 0o644); err != nil {
		t.Fatal(err)
	}

	before, defs := feedback.ParseEntry("001-conversation.md", []byte(capture))
	if len(defs) != 0 {
		t.Fatalf("capture has defects: %v", defs)
	}
	if before.EditCommit != "abc123" {
		t.Fatalf("edit-commit not recorded at capture: %q", before.EditCommit)
	}

	repo := fakeRepo{
		resolves: map[string]bool{"some-doc#3.1": true},
		landings: map[string]feedback.Landing{"abc123": {Landed: true, Outcome: "landed-sha", EditSize: 1}},
	}
	transitions, err := feedback.Observe([]*feedback.Captured{before}, repo)
	if err != nil {
		t.Fatal(err)
	}
	disposed, err := feedback.ApplyTransitions(dir, transitions)
	if err != nil {
		t.Fatal(err)
	}
	if len(disposed) != 1 {
		t.Fatalf("want one disposed entry, got %v", disposed)
	}

	raw, _ := os.ReadFile(entryPath)
	if !strings.Contains(string(raw), capture[:strings.Index(capture, "## Comment")]) {
		t.Fatal("the capture block was not preserved byte-for-byte (DINV-1)")
	}
	after, defs := feedback.ParseEntry("001-conversation.md", raw)
	if len(defs) != 0 {
		t.Fatalf("disposed entry has defects: %v", defs)
	}
	if after.Comment != before.Comment || after.Author != before.Author {
		t.Fatal("capture attribution/comment changed across disposition (DINV-1)")
	}
	if after.Disposition == nil || after.Disposition.Disposition != "applied" || after.Disposition.Outcome != "landed-sha" {
		t.Fatalf("applied disposition not appended: %+v", after.Disposition)
	}
	if after.Disposition.EditDistance == nil || *after.Disposition.EditDistance != 1 {
		t.Fatalf("edit_distance not frozen in the entry: %+v", after.Disposition)
	}
}

// A disposition is append-once: applying a second one to an already-disposed
// entry is refused, never a rewrite (DINV-1).
func TestAppendDispositionAppendOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "001-conversation.md")
	if err := os.WriteFile(path, []byte(`# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @a |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | d#1 |
| **Template version** | review-feedback-entry/v1 |

## Comment

hi
`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := feedback.Disposition{Disposition: "no-action", Outcome: "sha", ResolvedBy: "subject finalized"}
	if err := feedback.AppendDisposition(path, d); err != nil {
		t.Fatalf("first disposition: %v", err)
	}
	if err := feedback.AppendDisposition(path, d); err == nil {
		t.Fatal("a second disposition must be refused — append-once (DINV-1)")
	}
}
