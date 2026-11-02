package compile

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestCompilePRDAndDocuments(t *testing.T) {
	prd, err := os.ReadFile("testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"prd.md":            prd,
		"tech-spec.md":      []byte("# Widget — tech spec\n"),
		"brainstorm/001.md": []byte("# Brainstorm\n"),
		"reviews/002.md":    []byte("# Review\n\n**Draft digest:** sha256:" + strings.Repeat("a", 64) + "\n"),
		"problems.md":       []byte("# Problems\n"),
		"events.md":         []byte("# Draft events\n"),
	}
	first, err := Compile("widget", "mainline-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile("widget", "mainline-commit", files)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("compile is not deterministic: error %v", err)
	}
	want := map[string]string{
		"FR-1": record.Promise, "NR-1": record.Promise, "NFR-1": record.Promise,
		"UB-1": record.Promise, "ABUSE-1": record.Promise, "EVT-1": record.Promise,
		"DOD-1": record.Expectation, "EVAL-1": record.Expectation,
		"G-1": record.Need, "J-1": record.Need, "M-1": record.Need,
		"Q-1": record.Question, "Q-2": record.Question, "R-1": record.Decision,
	}
	seen := map[string]bool{}
	var answers, documents int
	for _, compiled := range first.Records {
		draft := compiled.Draft
		if draft.Provenance.Source != "" || draft.Provenance.Commit != "" {
			t.Fatalf("compiled record carries document-capture provenance: %#v", draft)
		}
		for _, tag := range draft.Tags {
			if strings.HasPrefix(tag, "reconcile:") || strings.HasPrefix(tag, "source:") || strings.HasPrefix(tag, "source-unit:") {
				t.Fatalf("compiled record carries a source-capture tag: %#v", draft)
			}
		}
		if kind, ok := want[draft.Family]; ok {
			if draft.Kind != kind || seen[draft.Family] {
				t.Fatalf("row %s maps to %s, want %s (duplicate %t)", draft.Family, draft.Kind, kind, seen[draft.Family])
			}
			seen[draft.Family] = true
		}
		if draft.Family == "FR-99" {
			t.Fatal("human-only section 0 compiled as a requirement")
		}
		if draft.Kind == record.Answer {
			answers++
			if draft.Statement != "Memory stores it." || !reflect.DeepEqual(compiled.Links, []Link{{Type: record.Answers, Target: "prd/widget#Q-2"}}) {
				t.Fatalf("answer mapping = %#v", compiled)
			}
		}
		if draft.Kind == record.Document {
			documents++
			if !draft.LiteralBody || len(draft.Body) == 0 {
				t.Fatalf("document lost exact text: %#v", draft)
			}
			if draft.Family == "prd" && string(draft.Body) != string(prd) {
				t.Fatal("PRD Document differs from the working file's exact bytes")
			}
		}
	}
	if len(seen) != len(want) || answers != 1 || documents != len(files) {
		t.Fatalf("compiled rows=%d/%d answers=%d documents=%d/%d", len(seen), len(want), answers, documents, len(files))
	}
	if first.Digest == "" || !strings.HasPrefix(first.Digest, "sha256:") {
		t.Fatalf("digest = %q", first.Digest)
	}
	if got := ContractDigest(files["prd.md"], files["tech-spec.md"]); got != first.Digest {
		t.Fatalf("locked-text digest = %q, compiled digest = %q", got, first.Digest)
	}
	foreign, err := events.New(events.DraftOpened, "other", first.Digest, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	foreign.Origin = "new"
	if _, err := WithEvents(first, foreign); err == nil || !strings.Contains(err.Error(), "differs from widget") {
		t.Fatalf("foreign appended event = %v", err)
	}
	if _, err := WithEvents(first, events.Event{Feature: "widget"}); err == nil || !strings.Contains(err.Error(), "append compiled draft events") {
		t.Fatalf("invalid appended event = %v", err)
	}
	foreignLog, _, err := events.Append(nil, foreign)
	if err != nil {
		t.Fatal(err)
	}
	files["events.md"] = foreignLog
	if _, err := Compile("widget", "mainline-commit", files); err == nil || !strings.Contains(err.Error(), "belongs to other") {
		t.Fatalf("foreign working event = %v", err)
	}
	files["events.md"] = []byte("# Draft events\n")
	files["reviews/002.md"] = []byte("# Changed review\n\n**Draft digest:** sha256:" + strings.Repeat("b", 64) + "\n")
	changedReview, err := Compile("widget", "mainline-commit", files)
	if err != nil || changedReview.Digest != first.Digest {
		t.Fatalf("review edit changed contract digest: %q, %v", changedReview.Digest, err)
	}
	files["tech-spec.md"] = []byte("# Changed tech spec\n")
	changedContract, err := Compile("widget", "mainline-commit", files)
	if err != nil || changedContract.Digest == first.Digest {
		t.Fatalf("contract edit kept digest: %q, %v", changedContract.Digest, err)
	}
}

func TestCompileRejectsInvalidParts(t *testing.T) {
	base := map[string][]byte{"prd.md": []byte("# Widget — PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v2 |\n"), "tech-spec.md": []byte("# Widget\n")}
	for _, tt := range []struct {
		name  string
		files map[string][]byte
		want  string
	}{
		{name: "missing PRD", files: map[string][]byte{"tech-spec.md": base["tech-spec.md"]}, want: "prd.md is missing"},
		{name: "missing tech spec", files: map[string][]byte{"prd.md": base["prd.md"]}, want: "tech-spec.md is missing"},
		{name: "path escape", files: map[string][]byte{"prd.md": base["prd.md"], "tech-spec.md": base["tech-spec.md"], "../secret.md": []byte("secret")}, want: "invalid working-file path"},
		{name: "malformed event log", files: map[string][]byte{"prd.md": base["prd.md"], "tech-spec.md": base["tech-spec.md"], "events.md": []byte("bad log\n")}, want: "events.md"},
		{name: "malformed PRD", files: map[string][]byte{"prd.md": []byte("no H1"), "tech-spec.md": base["tech-spec.md"]}, want: "does not start with an H1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile("widget", "main", tt.files)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Compile() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCompilePRDRejectsRowsWithoutMeaning(t *testing.T) {
	prd, err := os.ReadFile("testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, before, after, want string
	}{
		{"empty question", "Who owns retries?", " ", "question is empty"},
		{"empty risk", "Retries may duplicate work.", " ", "risk or assumption is empty"},
		{"unmapped requirement", "The widget shall open.", " ", "no complete claim mapping"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(strings.Replace(string(prd), tc.before, tc.after, 1))
			_, _, err := compilePRD("widget", body)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compilePRD() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCompilePRDOpenQuestionInTemplateTableHasNoAnswer(t *testing.T) {
	body := []byte("# Widget — PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v2 |\n\n## 7. Open questions\n\n| # | Question | Owner | Needed by | Status |\n|---|---|---|---|---|\n| Q-1 | Who owns retries? | PM | lock | open |\n")
	records, _, err := compilePRD("widget", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Draft.Kind != record.Question || records[0].Draft.Family != "Q-1" {
		t.Fatalf("open question produced records %#v", records)
	}
}
