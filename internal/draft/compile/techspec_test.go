package compile

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestCompileRemainingDraftPartsAndChanges(t *testing.T) {
	prd := readFixture(t, "testdata/prd/working.md")
	tech := readFixture(t, "testdata/techspec/working.md")
	problems := readFixture(t, "testdata/techspec/problems.md")
	digest := "sha256:" + strings.Repeat("a", 64)
	files := map[string][]byte{
		"prd.md": prd, "tech-spec.md": tech, "problems.md": problems,
		"reviews/001.md": []byte("# PRD review\n\n**Draft digest:** " + digest + "\n"),
	}
	before := bytes.Replace(prd, []byte("The widget shall open."), []byte("The widget should open."), 1)
	before = bytes.Replace(before, []byte("queue the open action"), []byte("retry immediately"), 1)
	compiled, err := CompileWithHistory("widget", "main", files, []PRDRevision{{Commit: "first", Body: before}, {Commit: "second", Body: prd}})
	if err != nil {
		t.Fatal(err)
	}
	var decision, answer, review *Record
	for i := range compiled.Records {
		r := &compiled.Records[i]
		switch {
		case r.Draft.Kind == record.Decision && r.Draft.Family == "D-1":
			decision = r
		case r.Draft.Kind == record.Answer && r.Draft.Statement == "Queue the open action while offline.":
			answer = r
		case r.Draft.Kind == record.Document && r.Draft.Family == "report-001":
			review = r
		}
	}
	if decision == nil || !reflect.DeepEqual(decision.Links, []Link{{Type: record.Serves, Target: "prd/widget#FR-3"}, {Type: record.Serves, Target: "prd/widget#UB-2"}}) {
		t.Fatalf("tech-spec decision serves = %#v", decision)
	}
	if answer == nil || answer.Draft.Intent != "operator" || answer.Draft.Rationale != "The user needs a bounded wait." || !reflect.DeepEqual(answer.Links, []Link{{Type: record.Answers, Target: "prd/widget#P-1"}}) {
		t.Fatalf("ruling Answer = %#v", answer)
	}
	if len(compiled.Problems) != 1 || compiled.Problems[0].Ruling == nil || compiled.Problems[0].Ruling.Problem != "P-1" || compiled.Problems[0].Requirement != "UB-1" {
		t.Fatalf("problem metadata = %#v", compiled.Problems)
	}
	if review == nil || !reflect.DeepEqual(review.Draft.Tags, []string{"draft-digest:" + digest}) || len(compiled.Reviews) != 1 || compiled.Reviews[0].Digest != digest {
		t.Fatalf("review digest = record %#v, reviews %#v", review, compiled.Reviews)
	}
	if len(compiled.Changes) != 2 {
		t.Fatalf("changes = %#v", compiled.Changes)
	}
	byID := map[string]RequirementChange{}
	for _, change := range compiled.Changes {
		byID[change.ID] = change
	}
	if change := byID["FR-1"]; !change.Unruled || change.Ruling != nil || change.FromCommit != "first" || change.ToCommit != "second" {
		t.Fatalf("unruled FR-1 change = %#v", change)
	}
	if change := byID["UB-1"]; change.Unruled || change.Ruling == nil || change.Ruling.Problem != "P-1" {
		t.Fatalf("ruled UB-1 change = %#v", change)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCompileChangesRefusesUntrustworthyHistory(t *testing.T) {
	prd := readFixture(t, "testdata/prd/working.md")
	if changes, err := compileChanges(prd, nil, nil); err != nil || len(changes) != 0 {
		t.Fatalf("no history: %#v, %v", changes, err)
	}
	for _, tc := range []struct {
		name      string
		revisions []PRDRevision
		want      string
	}{
		{"last differs", []PRDRevision{{Commit: "a", Body: []byte("different")}}, "last PRD revision differs"},
		{"repeated commit", []PRDRevision{{Commit: "a", Body: prd}, {Commit: "a", Body: prd}}, "repeated commit"},
		{"malformed earlier revision", []PRDRevision{{Commit: "a", Body: []byte("not a PRD")}, {Commit: "b", Body: prd}}, "compile draft changes at a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileChanges(prd, tc.revisions, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compileChanges() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCompileRejectsMalformedTechnicalParts(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"missing decision", "### D-1 — Queue\n\n**Serves:** FR-1\n", "Decision line is missing"},
		{"invalid serves", "### D-1 — Queue\n\n**Decision:** Queue.\n**Serves:** not-an-ID\n", "invalid or repeated Serves ID"},
		{"repeated decision", "### D-1 — Queue\n**Decision:** Queue.\n### D-1 — Again\n**Decision:** Again.\n", "invalid or repeated decision heading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileTechSpec("widget", []byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("tech spec error = %v, want %q", err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, body, want string
	}{
		{"missing problem", "### P-1 — FR-1 — Missing owner\n", "Problem line is missing"},
		{"bad heading", "### P-0 — FR-1 — Invalid\n", "invalid or repeated problem heading"},
		{"bad requirement", "### P-1 — BAD-1 — Invalid\n", "invalid requirement ID"},
		{"bad ruling", "### P-1 — FR-1 — Missing owner\n**Problem:** Who owns it?\n**Ruling:** anonymous\n", "ruling needs author, reason, and change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := compileProblems("widget", []byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("problems error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := compileReview("reviews/001.md", []byte("# Review without a digest\n")); err == nil || !strings.Contains(err.Error(), "Draft digest") {
		t.Fatalf("missing review digest error = %v", err)
	}
	if _, err := Compile("bad/feature", "main", map[string][]byte{"prd.md": []byte("x"), "tech-spec.md": []byte("x")}); err == nil || !strings.Contains(err.Error(), "invalid feature") {
		t.Fatalf("invalid feature error = %v", err)
	}
}

func TestCompileDecisionCrossFeatureAndLineageLinks(t *testing.T) {
	body := []byte("### D-1 — Queue\n**Decision:** Queue work.\n**Serves:** FR-1 · prd:other/NR-2\n**Builds-on:** design:other#D-3 · D-2\n\n### D-2 — Earlier choice\n**Decision:** Keep a queue.\n**Supersedes:** D-1\n")
	entries, err := compileTechSpec("widget", body)
	if err != nil {
		t.Fatal(err)
	}
	want := []Link{
		{Type: record.Serves, Target: "prd/widget#FR-1"},
		{Type: record.Serves, Target: "prd/other#NR-2"},
		{Type: record.DependsOn, Target: "design/other#D-3"},
		{Type: record.DependsOn, Target: "design/widget#D-2"},
	}
	if len(entries) != 2 || !reflect.DeepEqual(entries[0].Links, want) || !reflect.DeepEqual(entries[1].Links, []Link{{Type: record.Supersedes, Target: "design/widget#D-1"}}) {
		t.Fatalf("decision links = %#v", entries)
	}
	for _, tc := range []struct{ field, want string }{
		{"**Serves:** prd:other/nope", "Serves"},
		{"**Serves:** FR-1 · prd:widget/FR-1", "repeated Serves"},
		{"**Builds-on:** design:other#bad", "depends_on"},
		{"**Builds-on:** D-2 · D-2", "repeated depends_on"},
		{"**Supersedes:** design:other#D-1", "supersedes"},
	} {
		if _, err := compileTechSpec("widget", []byte("### D-1 — Queue\n**Decision:** Queue.\n"+tc.field+"\n")); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", tc.field, err, tc.want)
		}
	}
}
