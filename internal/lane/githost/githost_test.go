package githost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

func TestProductionRunnerAndDefaultHostBoundaries(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	h := New("o/r", repo, "", "")
	if h.remote() == "" || h.mainline() == "" {
		t.Fatal("default remote and mainline were not resolved")
	}
	if _, err := h.git(t.Context(), "rev-parse", "--git-dir"); err != nil {
		t.Fatalf("default git runner: %v", err)
	}
	if _, err := gitRun(t.Context(), repo, "definitely-not-a-git-command"); err == nil {
		t.Fatal("gitRun hid a git command failure")
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := shellGH(t.Context(), "api", "user"); err == nil || !strings.Contains(err.Error(), "gh api user") {
		t.Fatalf("shellGH missing executable error = %v", err)
	}
	if _, err := h.run(t.Context(), "api", "user"); err == nil {
		t.Fatal("Host.run default hid a missing gh executable")
	}
}

// call records one runner invocation.
type call struct{ args []string }

// script serves canned responses keyed by a substring of the joined argv, and
// records every call — the hermetic stand-in for `gh`.
type script struct {
	calls     []call
	responses map[string]string // substring → stdout
	errs      map[string]error  // substring → error
}

func (s *script) run(_ context.Context, args ...string) ([]byte, error) {
	s.calls = append(s.calls, call{args})
	joined := strings.Join(args, " ")
	// Longest key first, so "pulls/7/reviews" wins over "pulls/7" — map order
	// is random and a shorter key is a substring of a longer endpoint.
	for _, k := range keysByLength(s.errs) {
		if strings.Contains(joined, k) {
			return nil, s.errs[k]
		}
	}
	for _, k := range keysByLength(s.responses) {
		if strings.Contains(joined, k) {
			return []byte(s.responses[k]), nil
		}
	}
	return nil, fmt.Errorf("unscripted gh call: %s", joined)
}

func keysByLength[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return len(keys[a]) > len(keys[b]) })
	return keys
}

func host(s *script) *Host { return &Host{Repo: "o/r", Root: ".", Run: s.run} }

func TestHead(t *testing.T) {
	s := &script{responses: map[string]string{"pulls/7": `{"head":{"sha":"abc123"}}`}}
	got, err := host(s).Head(context.Background(), "7")
	if err != nil || got != "abc123" {
		t.Fatalf("Head = %q, %v", got, err)
	}
	s = &script{responses: map[string]string{"pulls/7": `{"head":{}}`}}
	if _, err := host(s).Head(context.Background(), "7"); err == nil {
		t.Fatal("want error for missing head sha")
	}
	s = &script{errs: map[string]error{"pulls/7": errors.New("transport failed")}}
	if _, err := host(s).Head(context.Background(), "7"); err == nil || !strings.Contains(err.Error(), "reading head") {
		t.Fatalf("want contextual transport error, got %v", err)
	}
	s = &script{responses: map[string]string{"pulls/7": `{`}}
	if _, err := host(s).Head(context.Background(), "7"); err == nil || !strings.Contains(err.Error(), "decoding PR") {
		t.Fatalf("want contextual decode error, got %v", err)
	}
}

func TestAllRequiredGreen(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"zero checks is never green (fail-closed)", `{"total_count":0,"check_runs":[]}`, false},
		{"in-progress check is not green", `{"check_runs":[{"status":"in_progress","conclusion":""}]}`, false},
		{"failed conclusion is not green", `{"check_runs":[{"status":"completed","conclusion":"failure"}]}`, false},
		{"all success is green", `{"check_runs":[{"status":"completed","conclusion":"success"},{"status":"completed","conclusion":"skipped"}]}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &script{responses: map[string]string{"check-runs": tc.body}}
			got, err := host(s).AllRequiredGreen(context.Background(), "7", "abc")
			if err != nil || got != tc.want {
				t.Fatalf("AllRequiredGreen = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestReviewVerdict(t *testing.T) {
	t.Run("absent arm state is non-approving", func(t *testing.T) {
		s := &script{responses: map[string]string{"check-runs": `{"check_runs":[]}`}}
		st, err := host(s).ReviewVerdict(context.Background(), "7", "abc")
		if err != nil {
			t.Fatal(err)
		}
		if st.IsApproving() || st.Context != verdict.CheckContext {
			t.Fatalf("want non-approving under the arm context, got %+v", st)
		}
	})
	t.Run("published approving state round-trips", func(t *testing.T) {
		pub := verdict.CheckState{Context: verdict.CheckContext, Status: verdict.Approving, Reason: "report approves; fingerprint matches"}
		b, err := json.Marshal(pub)
		if err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf(`{"check_runs":[{"name":%q,"output":{"summary":%q}}]}`, verdict.CheckContext, string(b))
		s := &script{responses: map[string]string{"check-runs": body}}
		st, err := host(s).ReviewVerdict(context.Background(), "7", "abc")
		if err != nil || !st.IsApproving() {
			t.Fatalf("want approving, got %+v, %v", st, err)
		}
	})
	t.Run("unreadable published state fails closed", func(t *testing.T) {
		body := fmt.Sprintf(`{"check_runs":[{"name":%q,"output":{"summary":"not json"}}]}`, verdict.CheckContext)
		s := &script{responses: map[string]string{"check-runs": body}}
		st, err := host(s).ReviewVerdict(context.Background(), "7", "abc")
		if err != nil || st.IsApproving() {
			t.Fatalf("want non-approving on unreadable state, got %+v, %v", st, err)
		}
	})
}

func TestMergedIntoMainline(t *testing.T) {
	t.Run("unmerged reads false", func(t *testing.T) {
		s := &script{responses: map[string]string{"pulls/7": `{"merged":false}`}}
		got, err := host(s).MergedIntoMainline(context.Background(), "7", "abc")
		if err != nil || got {
			t.Fatalf("want false for unmerged, got %v, %v", got, err)
		}
	})
	t.Run("merged with ancestor content reads true", func(t *testing.T) {
		s := &script{responses: map[string]string{"pulls/7": `{"merged":true,"merge_commit_sha":"mc1"}`}}
		h := host(s)
		h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) { return nil, nil }
		got, err := h.MergedIntoMainline(context.Background(), "7", "abc")
		if err != nil || !got {
			t.Fatalf("want true, got %v, %v", got, err)
		}
	})
	t.Run("merged badge with non-ancestor content is a lying badge", func(t *testing.T) {
		s := &script{responses: map[string]string{"pulls/7": `{"merged":true,"merge_commit_sha":"mc1"}`}}
		h := host(s)
		h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "merge-base" {
				return nil, errors.New("exit 1: not an ancestor")
			}
			return nil, nil
		}
		got, err := h.MergedIntoMainline(context.Background(), "7", "abc")
		if err != nil || got {
			t.Fatalf("want false for lying badge, got %v, %v", got, err)
		}
	})
	t.Run("mainline fetch failure is surfaced", func(t *testing.T) {
		s := &script{responses: map[string]string{"pulls/7": `{"merged":true,"merge_commit_sha":"mc1"}`}}
		h := host(s)
		h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "fetch" {
				return nil, errors.New("fetch failed")
			}
			return nil, nil
		}
		got, err := h.MergedIntoMainline(context.Background(), "7", "abc")
		if err == nil || got || !strings.Contains(err.Error(), "fetching mainline") {
			t.Fatalf("want contextual fetch failure, got %v, %v", got, err)
		}
	})
	// The ancestry read resolves the CONFIGURED remote/branch, not a literal
	// (config-driven-portability-v1 §3.2, FR-2, DINV-1): a non-default Host
	// fetches "<remote> <branch>" and checks ancestry against "<remote>/<branch>".
	t.Run("resolves a non-default remote and branch", func(t *testing.T) {
		s := &script{responses: map[string]string{"pulls/7": `{"merged":true,"merge_commit_sha":"mc1"}`}}
		h := &Host{Repo: "o/r", Root: ".", Remote: "upstream", Mainline: "trunk", Run: s.run}
		var gitCalls [][]string
		h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gitCalls = append(gitCalls, args)
			return nil, nil
		}
		if _, err := h.MergedIntoMainline(context.Background(), "7", "abc"); err != nil {
			t.Fatalf("MergedIntoMainline: %v", err)
		}
		var sawFetch, sawAncestry bool
		for _, args := range gitCalls {
			j := strings.Join(args, " ")
			if j == "fetch upstream trunk --quiet" {
				sawFetch = true
			}
			if j == "merge-base --is-ancestor mc1 upstream/trunk" {
				sawAncestry = true
			}
		}
		if !sawFetch {
			t.Errorf("want a `fetch upstream trunk --quiet` (configured remote/branch); calls: %v", gitCalls)
		}
		if !sawAncestry {
			t.Errorf("want ancestry against `upstream/trunk` (configured); calls: %v", gitCalls)
		}
	})
}

func TestRecordVerdictCreateThenAppend(t *testing.T) {
	rec := verdict.Record{RecordVersion: verdict.RecordVersion, Lane: "HUMAN", PolicyVersion: "risk-policy/v1", HeadRevision: "abc"}

	t.Run("first evaluation creates the marker comment", func(t *testing.T) {
		s := &script{responses: map[string]string{
			"issues/7/comments?per_page=100": `[]`,
			"issues/7/comments":              `{}`, // the POST (the longer GET key wins for the list read)
		}}
		if err := host(s).RecordVerdict(context.Background(), "7", rec); err != nil {
			t.Fatal(err)
		}
		last := s.calls[len(s.calls)-1].args
		joined := strings.Join(last, " ")
		if !strings.Contains(joined, "POST") || !strings.Contains(joined, BlockMarker) {
			t.Fatalf("want POST with marker body, got %v", joined)
		}
	})

	t.Run("later evaluations append, never rewrite", func(t *testing.T) {
		prior := verdict.AppendBlock(nil, rec)
		body := BlockMarker + "\n```json\n" + strings.TrimRight(string(prior), "\n") + "\n```"
		list, err := json.Marshal([]map[string]any{{"id": 42, "body": body}})
		if err != nil {
			t.Fatal(err)
		}
		s := &script{responses: map[string]string{
			"issues/7/comments?per_page=100": string(list),
			"issues/comments/42":             `{}`, // PATCH
		}}
		rec2 := rec
		rec2.HeadRevision = "def"
		if err := host(s).RecordVerdict(context.Background(), "7", rec2); err != nil {
			t.Fatal(err)
		}
		last := strings.Join(s.calls[len(s.calls)-1].args, " ")
		if !strings.Contains(last, "PATCH") {
			t.Fatalf("want PATCH of the existing comment, got %v", last)
		}
		// The patched body must carry BOTH records in order — append-only.
		if !strings.Contains(last, `"head_revision":"abc"`) || !strings.Contains(last, `"head_revision":"def"`) {
			t.Fatalf("want both records in the appended block, got %v", last)
		}
		if strings.Index(last, `"head_revision":"abc"`) > strings.Index(last, `"head_revision":"def"`) {
			t.Fatal("append order violated: the earlier record must precede the later one")
		}
	})
}

func TestExtractBlockRoundTrip(t *testing.T) {
	rec := verdict.Record{RecordVersion: verdict.RecordVersion, Lane: "HUMAN", PolicyVersion: "p", HeadRevision: "h"}
	block := verdict.AppendBlock(nil, rec)
	body := BlockMarker + "\n```json\n" + strings.TrimRight(string(block), "\n") + "\n```"
	got := extractBlock(body)
	recs, err := verdict.ParseBlock(got)
	if err != nil || len(recs) != 1 || recs[0].HeadRevision != "h" {
		t.Fatalf("round-trip failed: %v, %v", recs, err)
	}
	if extractBlock("no fences here") != nil {
		t.Fatal("want nil block for an unfenced body")
	}
}

func TestMachineActVoidsOnMovedHead(t *testing.T) {
	s := &script{responses: map[string]string{"pulls/7": `{"head":{"sha":"moved"}}`}}
	m := &Machine{Host: host(s), Name: "bot"}
	err := m.Act(context.Background(), "7", "evaluated")
	if err == nil || !strings.Contains(err.Error(), "DINV-3") {
		t.Fatalf("want DINV-3 void on moved head, got %v", err)
	}
	if !m.Identity().Machine || m.Identity().IsHuman() {
		t.Fatal("machine identity must be structurally machine")
	}
}

func TestMachineActApprovesThenMergesBoundToHead(t *testing.T) {
	s := &script{responses: map[string]string{
		"pulls/7":         `{"head":{"sha":"abc"}}`, // the head GET (shortest key — matched last)
		"pulls/7/reviews": `{}`,
		"pulls/7/merge":   `{}`,
	}}
	m := &Machine{Host: host(s), Name: "bot"}
	if err := m.Act(context.Background(), "7", "abc"); err != nil {
		t.Fatal(err)
	}
	var sawApprove, sawMerge bool
	for _, c := range s.calls {
		j := strings.Join(c.args, " ")
		if strings.Contains(j, "reviews") && strings.Contains(j, "event=APPROVE") && strings.Contains(j, "commit_id=abc") {
			sawApprove = true
		}
		if strings.Contains(j, "merge") && strings.Contains(j, "sha=abc") {
			if !sawApprove {
				t.Fatal("merge before approval")
			}
			sawMerge = true
		}
	}
	if !sawApprove || !sawMerge {
		t.Fatalf("want approve then merge bound to head; calls: %+v", s.calls)
	}
}

func TestMachineActSurfacesPlatformFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		errKey  string
		wantErr string
	}{
		{name: "approval", errKey: "pulls/7/reviews", wantErr: "machine approval"},
		{name: "merge", errKey: "pulls/7/merge", wantErr: "machine merge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &script{
				responses: map[string]string{
					"pulls/7":         `{"head":{"sha":"abc"}}`,
					"pulls/7/reviews": `{}`,
				},
				errs: map[string]error{tc.errKey: errors.New("platform failed")},
			}
			m := &Machine{Host: host(s), Name: "bot"}
			if err := m.Act(context.Background(), "7", "abc"); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Act error = %v, want %q context", err, tc.wantErr)
			}
		})
	}
}
