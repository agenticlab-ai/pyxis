package lock

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// Fixed future source IDs reproduce a clock behind a previously issued batch.
// The old writer fails these projection checks without sleeps or a clock seam.
func TestWriteRecordsFutureBindingChronology(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2999, 1, 2, 3, 4, 5, 123000000, time.UTC)
	seed := func(n byte, unit compile.Record, relations ...record.Relation) *record.Record {
		return chronologySeed(t, root, at.Add(time.Duration(n)*time.Millisecond), n, unit, relations...)
	}
	promise := seed(0, diffUnit(record.Promise, "prd/widget", "FR-1", "Queue a widget."))
	removed := seed(1, diffUnit(record.Promise, "prd/widget", "FR-2", "Open a widget."))
	question := seed(2, diffUnit(record.Question, "prd/widget", "Q-1", "Who approves?"))
	seed(3, diffUnit(record.Answer, "prd/widget", "", "The operator."), record.Relation{Type: record.Answers, Target: question.ID})
	seed(4, diffUnit(record.Decision, "design/widget", "D-1", "Use a queue."), record.Relation{Type: record.Serves, Target: promise.ID})
	seed(5, diffDocument("prd/widget", "prd", "# Original PRD\r\n"))
	specBody := "# Literal spec\r\n{{untouched-marker}}\r\n"
	spec := seed(6, diffDocument("design/widget", "design", specBody))
	events := seed(7, diffDocument("feature/widget", "events", "# Original events\r\n"))
	unchanged := seed(8, diffUnit(record.Promise, "prd/widget", "FR-3", "Close a widget."))
	binding := chronologyProjection(t, root)
	for version := 2; version <= 4; version++ {
		oldPromise := binding.Head("prd/widget", "FR-1")
		oldPRD := binding.Head("prd/widget", "prd")
		oldQuestion := activeAt(binding, "prd/widget", "Q-1")
		oldAnswer := binding.Answer(oldQuestion.ID)
		prdBody := fmt.Sprintf("# PRD version %d\r\n{{literal-marker}}\r\n", version)
		result := compile.Result{Feature: "widget", Records: []compile.Record{
			// Consumers deliberately precede their local targets in the input.
			diffUnit(record.Decision, "design/widget", "D-1", "Use a queue.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-1"}),
			diffUnit(record.Decision, "design/widget", "D-2", "Close directly.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-3"}),
			diffUnit(record.Answer, "prd/widget", "", "The operator.", compile.Link{Type: record.Answers, Target: "prd/widget#Q-1"}),
			diffUnit(record.Promise, "prd/widget", "FR-1", fmt.Sprintf("Queue widgets under contract %d.", version)),
			diffUnit(record.Promise, "prd/widget", "FR-3", "Close a widget."),
			diffUnit(record.Question, "prd/widget", "Q-1", fmt.Sprintf("Who approves version %d?", version)),
			diffDocument("prd/widget", "prd", prdBody),
			diffDocument("design/widget", "design", specBody),
			diffDocument("feature/widget", "events", fmt.Sprintf("# Events version %d\r\n", version)),
		}}
		docs, _, err := writeRecordsWithBinding(root, "agent:test", result, readiness.Sources{}, binding, version)
		if err != nil {
			t.Fatal(err)
		}
		current := chronologyProjection(t, root)
		for path, original := range binding.Set.RawBytes {
			if !bytes.Equal(current.Set.RawBytes[path], original) {
				t.Fatalf("version %d changed immutable source %s", version, path)
			}
		}
		if got := current.Head("prd/widget", "FR-1"); got == nil || !reflect.DeepEqual(got.Rel(record.Supersedes), []string{oldPromise.ID}) {
			t.Fatalf("version %d lost promise predecessor: %+v", version, got)
		}
		if got := current.Head("prd/widget", "prd"); got == nil || !reflect.DeepEqual(got.Rel(record.Supersedes), []string{oldPRD.ID}) {
			t.Fatalf("version %d lost Document predecessor: %+v", version, got)
		}
		decision := current.Head("design/widget", "D-1")
		if decision == nil || !reflect.DeepEqual(current.Targets(decision, record.Serves), []string{current.Head("prd/widget", "FR-1").ID}) {
			t.Fatalf("version %d did not follow the new local promise: %+v", version, decision)
		}
		newQuestion := activeAt(current, "prd/widget", "Q-1")
		if newQuestion == nil || current.Answer(newQuestion.ID) == nil || current.Status(oldQuestion.ID) != resolve.Superseded || current.Status(oldAnswer.ID) != resolve.Revoked {
			t.Fatalf("version %d lost question/answer history", version)
		}
		if current.Status(removed.ID) != resolve.Revoked || current.Status(events.ID) != resolve.Active || current.Head("feature/widget", fmt.Sprintf("events-v%d", version)) == nil {
			t.Fatalf("version %d lost withdrawal or historical events", version)
		}
		if current.Head("design/widget", "design").ID != spec.ID {
			t.Fatal("unchanged spec identity was reissued")
		}
		if d2 := current.Head("design/widget", "D-2"); d2 == nil || !reflect.DeepEqual(current.Targets(d2, record.Serves), []string{unchanged.ID}) || current.Head("prd/widget", "FR-3").ID != unchanged.ID {
			t.Fatalf("version %d lost the unchanged binding target: %+v", version, d2)
		}
		for key, want := range map[string]string{"prd": prdBody, "tech-spec": specBody} {
			got, err := os.ReadFile(filepath.Join(root, docs[key]))
			if err != nil || string(got) != want {
				t.Fatalf("version %d %s literal body = %q, %v", version, key, got, err)
			}
		}
		binding = current
	}
}

func TestWriteRecordsFutureMainlineAndLocalRelations(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2999, 1, 2, 3, 4, 5, 123000000, time.UTC)
	promise := chronologySeed(t, root, at, 0xff, diffUnit(record.Promise, "prd/other", "FR-2", "Queue safely."))
	policy := chronologySeed(t, root, at.Add(time.Millisecond), 0xff, diffUnit(record.Policy, "principle/core", "AP-1", "Keep immutable history."))
	mainline := chronologyProjection(t, root)
	decision := diffUnit(record.Decision, "design/widget", "D-1", "Use the other contract.", compile.Link{Type: record.Serves, Target: "prd/other#FR-2"})
	decision.Draft.Relations = []record.Relation{{Type: record.DependsOn, Target: policy.ID}}
	result := compile.Result{Feature: "widget", Records: []compile.Record{
		diffUnit(record.Decision, "design/widget", "D-2", "Build on both queues.", compile.Link{Type: record.DependsOn, Target: "design/widget#D-1"}, compile.Link{Type: record.Serves, Target: "prd/widget#FR-1"}),
		decision,
		diffUnit(record.Promise, "prd/widget", "FR-1", "Open a queue."),
		diffDocument("prd/widget", "prd", "# PRD\r\n"),
		diffDocument("design/widget", "design", "# Spec\r\n"),
	}}
	if _, _, err := writeRecords(root, "agent:test", result, readiness.Sources{Mainline: mainline}); err != nil {
		t.Fatal(err)
	}
	current := chronologyProjection(t, root)
	d1, d2 := current.Head("design/widget", "D-1"), current.Head("design/widget", "D-2")
	if d1 == nil || d2 == nil || !reflect.DeepEqual(current.Targets(d1, record.Serves), []string{promise.ID}) || !reflect.DeepEqual(current.Targets(d1, record.DependsOn), []string{policy.ID}) {
		t.Fatalf("external target IDs changed: D-1=%+v D-2=%+v", d1, d2)
	}
	if !reflect.DeepEqual(current.Targets(d2, record.DependsOn), []string{d1.ID}) || !reflect.DeepEqual(current.Targets(d2, record.Serves), []string{current.Head("prd/widget", "FR-1").ID}) {
		t.Fatalf("local targets were not issued before their consumer: %+v", d2)
	}
	for path, original := range mainline.Set.RawBytes {
		if !bytes.Equal(current.Set.RawBytes[path], original) {
			t.Fatalf("mainline source changed: %s", path)
		}
	}
}

func TestIssueRecordsFutureLiteralCaptureChronology(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2999, 1, 2, 3, 4, 5, 123000000, time.UTC)
	oldBody := "# Captured delivery\r\n{{literal-evidence}}\r\n"
	old := chronologySeed(t, root, at, 0xff, diffDocument("problem/widget", "delivery", oldBody))
	policy := chronologySeed(t, root, at.Add(3*time.Millisecond), 0xff, diffUnit(record.Policy, "principle/core", "AP-1", "Keep immutable evidence."))
	before := chronologyProjection(t, root)
	newBody := oldBody + "\r\n## E-2\r\nNew evidence.\r\n"
	next := diffDocument("problem/widget", "delivery", newBody)
	next.Draft.Relations = []record.Relation{{Type: record.Supersedes, Target: old.ID}, {Type: record.DependsOn, Target: policy.ID}}
	// Native source captures supply literal targets without binding/mainline.
	if _, _, err := issueRecords(root, "agent:test", compile.Result{Records: []compile.Record{next}}, readiness.Sources{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	current := chronologyProjection(t, root)
	head := current.Head("problem/widget", "delivery")
	if head == nil || !reflect.DeepEqual(head.Rel(record.Supersedes), []string{old.ID}) || !reflect.DeepEqual(current.Targets(head, record.DependsOn), []string{policy.ID}) || current.Status(old.ID) != resolve.Superseded {
		t.Fatalf("capture lost its immutable predecessor: %+v", head)
	}
	got, err := os.ReadFile(filepath.Join(root, store.Dir, head.BodyPath()))
	if err != nil || string(got) != newBody {
		t.Fatalf("capture literal body = %q, %v", got, err)
	}
	for path, original := range before.Set.RawBytes {
		if !bytes.Equal(current.Set.RawBytes[path], original) {
			t.Fatalf("original capture changed: %s", path)
		}
	}
}

func TestIssueRecordsChronologyRefusalLeavesStoreUnchanged(t *testing.T) {
	last := time.Date(9999, 12, 31, 23, 59, 59, 999000000, time.UTC)
	for _, tc := range []struct {
		name      string
		at        time.Time
		invalidID bool
	}{
		{"no later timestamp", last, false},
		{"whole batch exceeds timestamp range", last.Add(-2 * time.Millisecond), false},
		{"noncanonical source timestamp", last, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			r := &record.Record{Schema: record.Schema, Kind: record.Policy, Subject: "principle/core", Family: "AP-1", Statement: "Keep history.", Issuer: "operator:test", CreatedAt: tc.at.Format(time.RFC3339), ID: record.NewID(record.Policy, tc.at, [10]byte{})}
			if tc.invalidID {
				r.ID = "Policy-8" + strings.Repeat("0", 25)
			}
			if _, err := store.Write(filepath.Join(root, store.Dir), r); err != nil {
				t.Fatal(err)
			}
			basis := chronologyProjection(t, root)
			result := compile.Result{Feature: "widget", Records: []compile.Record{
				diffUnit(record.Decision, "design/widget", "D-1", "Choose a queue."),
				diffDocument("prd/widget", "prd", "# PRD\n"),
				diffDocument("design/widget", "design", "# Spec\n"),
			}}
			chronologyRefusal(t, root, result, readiness.Sources{Mainline: basis}, nil)
		})
	}
	t.Run("reserve revocations before issuing", func(t *testing.T) {
		root := t.TempDir()
		chronologySeed(t, root, last.Add(-2*time.Millisecond), 0, diffUnit(record.Promise, "prd/widget", "FR-1", "Open a widget."))
		chronologySeed(t, root, last.Add(-time.Millisecond), 0, diffDocument("prd/widget", "prd", "# Old PRD\n"))
		chronologySeed(t, root, last.Add(-time.Millisecond), 1, diffDocument("design/widget", "design", "# Spec\n"))
		binding := chronologyProjection(t, root)
		result := compile.Result{Feature: "widget", Records: []compile.Record{
			diffDocument("prd/widget", "prd", "# New PRD\n"),
			diffDocument("design/widget", "design", "# Spec\n"),
		}}
		chronologyRefusal(t, root, result, readiness.Sources{Mainline: binding}, binding)
	})
}

func TestWriteRecordsUnchangedAtChronologyLimit(t *testing.T) {
	root := t.TempDir()
	at := time.Date(9999, 12, 31, 23, 59, 59, 999000000, time.UTC)
	prd := chronologySeed(t, root, at, 0, diffDocument("prd/widget", "prd", "# PRD\r\n"))
	spec := chronologySeed(t, root, at, 0xff, diffDocument("design/widget", "design", "# Spec\r\n"))
	binding := chronologyProjection(t, root)
	result := compile.Result{Feature: "widget", Records: []compile.Record{
		diffDocument("prd/widget", "prd", "# PRD\r\n"),
		diffDocument("design/widget", "design", "# Spec\r\n"),
	}}
	docs, paths, err := writeRecordsWithBinding(root, "agent:test", result, readiness.Sources{Mainline: binding}, binding, 2)
	if err != nil || len(paths) != 0 || docs["prd"] != filepath.ToSlash(filepath.Join(store.Dir, prd.BodyPath())) || docs["tech-spec"] != filepath.ToSlash(filepath.Join(store.Dir, spec.BodyPath())) {
		t.Fatalf("unchanged documents need no new timestamp: docs=%v paths=%v err=%v", docs, paths, err)
	}
	if current := chronologyProjection(t, root); !reflect.DeepEqual(binding.Set.RawBytes, current.Set.RawBytes) {
		t.Fatal("unchanged batch wrote records")
	}
}

func chronologySeed(t *testing.T, root string, at time.Time, entropy byte, unit compile.Record, relations ...record.Relation) *record.Record {
	t.Helper()
	var e [10]byte
	for i := range e {
		e[i] = entropy
	}
	d := unit.Draft
	r := &record.Record{Schema: record.Schema, ID: record.NewID(d.Kind, at, e), Kind: d.Kind, Subject: d.Subject, Family: d.Family, Statement: d.Statement, LiteralBody: d.LiteralBody, Relations: relations, Issuer: "agent:test", CreatedAt: at.Format(time.RFC3339)}
	var err error
	if r.Kind == record.Document {
		_, err = store.WriteDocument(filepath.Join(root, store.Dir), r, d.Body)
	} else {
		_, err = store.Write(filepath.Join(root, store.Dir), r)
	}
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func chronologyProjection(t *testing.T, root string) *resolve.Projection {
	t.Helper()
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Problems) != 0 || len(set.DocumentIssues) != 0 || len(set.Dangling) != 0 {
		t.Fatalf("invalid memory: problems=%v documents=%v dangling=%v", set.Problems, set.DocumentIssues, set.Dangling)
	}
	p := resolve.New(set)
	if len(p.Violations) != 0 || len(p.Conflicts()) != 0 || len(p.AnswerConflicts()) != 0 || len(p.RevocationConflicts()) != 0 {
		t.Fatalf("invalid projection: violations=%v conflicts=%v answers=%v revocations=%v", p.Violations, p.Conflicts(), p.AnswerConflicts(), p.RevocationConflicts())
	}
	return p
}

func chronologyRefusal(t *testing.T, root string, result compile.Result, sources readiness.Sources, binding *resolve.Projection) {
	t.Helper()
	source := store.DirSource{Root: filepath.Join(root, store.Dir)}
	before, _, err := source.Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	docs, paths, err := issueRecords(root, "agent:test", result, sources, binding, 2)
	if !errors.Is(err, record.ErrInvalid) || !strings.Contains(err.Error(), "chronology") || docs != nil || paths != nil {
		t.Fatalf("unrepresentable chronology accepted: docs=%v paths=%v err=%v", docs, paths, err)
	}
	after, _, err := source.Files(t.Context())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("chronology refusal changed the memory store: %v", err)
	}
}
