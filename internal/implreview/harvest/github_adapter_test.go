package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func TestGitHubAdapterEnumeratesFullUniverseAndExactReadsArtifact(t *testing.T) {
	review := &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: "github:o/r#1", ReviewID: "review-1",
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: "abc", ContextID: "context-1", Findings: []implreport.Finding{}}}}
	body, err := persist.ComposeArtifact(review)
	if err != nil {
		t.Fatal(err)
	}
	commentJSON, _ := json.Marshal([][]map[string]any{{{"id": float64(71), "body": body, "user": map[string]any{"login": "reviewer"}}}})
	readJSON, _ := json.Marshal(map[string]any{"id": float64(71), "body": body, "user": map[string]any{"login": "operator"}})
	run := func(_ context.Context, args ...string) ([]byte, error) {
		endpoint := args[len(args)-1]
		switch {
		case strings.Contains(endpoint, "/pulls?"):
			return []byte(`[[{"number":1},{"number":2}]]`), nil
		case strings.Contains(endpoint, "/issues/1/comments?"):
			return commentJSON, nil
		case strings.Contains(endpoint, "/issues/2/comments?"):
			return []byte(`[[]]`), nil
		case strings.Contains(strings.Join(args, " "), "/issues/comments/71"):
			return readJSON, nil
		default:
			return nil, fmt.Errorf("unexpected gh args %v", args)
		}
	}
	adapter, err := newGitHubAdapter("o/r", run)
	if err != nil {
		t.Fatal(err)
	}
	universe, err := adapter.Enumerate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universe) != 2 || universe[0].Review == nil || universe[0].Review.Locator != "71" || universe[1].Review != nil {
		t.Fatalf("universe = %+v", universe)
	}
	got, err := adapter.Read(context.Background(), *universe[0].Review)
	if err != nil || got.ReviewID != "review-1" {
		t.Fatalf("exact-read review = %+v, %v", got, err)
	}
	author, found, err := adapter.AuthorOf(context.Background(), "github-comment:R_repo:71")
	if err != nil || !found || author != "operator" {
		t.Fatalf("author = %q found=%v err=%v", author, found, err)
	}
}

func TestGitHubAdapterEnumerationFailureIsNotPartial(t *testing.T) {
	adapter, err := newGitHubAdapter("o/r", func(context.Context, ...string) ([]byte, error) {
		return nil, fmt.Errorf("github unavailable")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := adapter.Enumerate(context.Background()); err == nil || got != nil {
		t.Fatalf("partial universe = %+v, %v", got, err)
	}
}

func TestGitHubAdapterQuotedFenceCannotShadowArtifactAndAbsenceIsTyped(t *testing.T) {
	review := &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: "github:o/r#1", ReviewID: "review-1",
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: "abc", ContextID: "context-1", Findings: []implreport.Finding{}}}}
	body, err := persist.ComposeArtifact(review)
	if err != nil {
		t.Fatal(err)
	}
	comments1, _ := json.Marshal([][]map[string]any{{
		{"id": float64(71), "body": body, "user": map[string]any{"login": "reviewer"}},
		{"id": float64(72), "body": "quoted from above:\n> ```impl-review\n> {}\n> ```", "user": map[string]any{"login": "reader"}},
	}})
	absence := "impl-review could not persist its review for this proposal.\nproposal: github:o/r#2\ncause: reviewer backend timed out\n```impl-review-absent\n{\"absent\":true,\"proposal_ref\":\"github:o/r#2\"}\n```\n"
	comments2, _ := json.Marshal([][]map[string]any{{
		{"id": float64(81), "body": absence, "user": map[string]any{"login": "reviewer"}},
	}})
	adapter, err := newGitHubAdapter("o/r", func(_ context.Context, args ...string) ([]byte, error) {
		endpoint := args[len(args)-1]
		switch {
		case strings.Contains(endpoint, "/pulls?"):
			return []byte(`[[{"number":1},{"number":2}]]`), nil
		case strings.Contains(endpoint, "/issues/1/comments?"):
			return comments1, nil
		case strings.Contains(endpoint, "/issues/2/comments?"):
			return comments2, nil
		default:
			return nil, fmt.Errorf("unexpected gh args %v", args)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	universe, err := adapter.Enumerate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universe) != 2 || universe[0].Review == nil || universe[0].Review.Locator != "71" {
		t.Fatalf("quoted comment shadowed exact artifact: %+v", universe)
	}
	if universe[1].Review != nil || !universe[1].Absent || universe[1].AbsentReason != "reviewer backend timed out" {
		t.Fatalf("absence marker was not classified: %+v", universe[1])
	}
}

func TestGitHubAdapterRejectsBoundaryFailures(t *testing.T) {
	runner := func(context.Context, ...string) ([]byte, error) { return nil, nil }
	if _, err := newGitHubAdapter("", runner); err == nil {
		t.Fatal("newGitHubAdapter accepted an empty repository")
	}
	if _, err := newGitHubAdapter("o/r", nil); err == nil {
		t.Fatal("newGitHubAdapter accepted a nil runner")
	}

	for _, tc := range []struct {
		name string
		run  func(context.Context, ...string) ([]byte, error)
	}{
		{name: "malformed pull response", run: func(context.Context, ...string) ([]byte, error) { return []byte("{"), nil }},
		{name: "missing pull number", run: func(context.Context, ...string) ([]byte, error) { return []byte(`[[{"number":0}]]`), nil }},
		{name: "comment transport failure", run: func(_ context.Context, args ...string) ([]byte, error) {
			if strings.Contains(args[len(args)-1], "/pulls?") {
				return []byte(`[[{"number":1}]]`), nil
			}
			return nil, fmt.Errorf("comments unavailable")
		}},
		{name: "malformed comments", run: func(_ context.Context, args ...string) ([]byte, error) {
			if strings.Contains(args[len(args)-1], "/pulls?") {
				return []byte(`[[{"number":1}]]`), nil
			}
			return []byte("{"), nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := newGitHubAdapter("o/r", tc.run)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := adapter.Enumerate(t.Context()); err == nil || got != nil {
				t.Fatalf("Enumerate = %+v, %v; want all-or-nothing failure", got, err)
			}
		})
	}
}

func TestGitHubAdapterExactReadAndAuthorBoundaries(t *testing.T) {
	adapter, err := newGitHubAdapter("o/r", func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"id":0}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Read(t.Context(), persist.Ref{Medium: "carina", Locator: "1"}); err == nil {
		t.Fatal("Read accepted a reference from another medium")
	}
	if _, err := adapter.Read(t.Context(), persist.Ref{Medium: persist.MediumGitHub, Locator: "not-a-number"}); err == nil {
		t.Fatal("Read accepted a malformed comment locator")
	}
	if _, err := adapter.Read(t.Context(), persist.Ref{Medium: persist.MediumGitHub, Locator: "1"}); err == nil {
		t.Fatal("Read accepted a missing comment")
	}

	adapter, err = newGitHubAdapter("o/r", func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"id":1,"user":{"login":" "}}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if author, found, err := adapter.AuthorOf(t.Context(), "github-comment:1"); err != nil || found || author != "" {
		t.Fatalf("AuthorOf empty login = %q, %v, %v", author, found, err)
	}
}
