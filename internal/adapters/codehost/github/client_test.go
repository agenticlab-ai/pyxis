package ghcodehost

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// execRecorder is a fake gh runner: it records the argv of each call and
// returns a canned response, so the real client's argv construction and JSON
// decoding are tested without a network or a real `gh`.
type execRecorder struct {
	calls [][]string
	out   []byte
	err   error
}

func (r *execRecorder) run(_ context.Context, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return r.out, r.err
}

func (r *execRecorder) last() []string {
	if len(r.calls) == 0 {
		return nil
	}
	return r.calls[len(r.calls)-1]
}

func argvString(args []string) string { return strings.Join(args, " ") }

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// assertFreshRead is the shared exact-fresh contract for a read: an exact REST
// call (path under repos/…), never the search index, carrying the no-cache
// directive with no conditional (If-None-Match) header (AP-15).
func assertFreshRead(t *testing.T, args []string) {
	t.Helper()
	if !hasArg(args, "api") {
		t.Errorf("read is not a gh api call: %s", argvString(args))
	}
	if !hasArg(args, noCacheHeader) {
		t.Errorf("read omits the %q freshness directive: %s", noCacheHeader, argvString(args))
	}
	joined := argvString(args)
	if strings.Contains(joined, "search/") || strings.Contains(joined, "search?") {
		t.Errorf("read uses the search index (stale), not an exact REST endpoint: %s", joined)
	}
	if strings.Contains(strings.ToLower(joined), "if-none-match") || strings.Contains(strings.ToLower(joined), "etag") {
		t.Errorf("read uses a cacheable conditional GET: %s", joined)
	}
	if path := args[len(args)-1]; !strings.HasPrefix(path, "repos/") {
		t.Errorf("read path %q is not an exact repos/… REST endpoint", path)
	}
}

func TestClient_FindOpenPRByHead_ArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`[{"number":55}]`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	num, found, err := c.findOpenPRByHead(context.Background(), "feat/x")
	if err != nil {
		t.Fatalf("findOpenPRByHead: %v", err)
	}
	assertFreshRead(t, rec.last())
	path := rec.last()[len(rec.last())-1]
	if !strings.Contains(path, "repos/o/r/pulls") || !strings.Contains(path, "head=o:feat/x") || !strings.Contains(path, "state=open") {
		t.Errorf("pulls-by-head path %q lacks repos/o/r/pulls?head=o:feat/x&state=open", path)
	}
	if !found || num != 55 {
		t.Errorf("findOpenPRByHead = (%d, %v), want (55, true)", num, found)
	}

	// No open PR for the head.
	recNone := &execRecorder{out: []byte(`[]`)}
	cNone := &ghClient{repo: "o/r", run: recNone.run}
	if num, found, _ := cNone.findOpenPRByHead(context.Background(), "feat/x"); found || num != 0 {
		t.Errorf("empty result = (%d, %v), want (0, false)", num, found)
	}
}

func TestClient_CompareAhead_ArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"ahead_by":3,"behind_by":0,"total_commits":3}`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	ahead, err := c.compareAhead(context.Background(), "main", "feat/x")
	if err != nil {
		t.Fatalf("compareAhead: %v", err)
	}
	assertFreshRead(t, rec.last())
	if path := rec.last()[len(rec.last())-1]; path != "repos/o/r/compare/main...feat/x" {
		t.Errorf("compare path = %q, want repos/o/r/compare/main...feat/x", path)
	}
	if ahead != 3 {
		t.Errorf("aheadBy = %d, want 3", ahead)
	}
}

// The client-level proof of "opens from the exact head branch": createPR sends
// the head verbatim, targets the base, and opens a draft (never-merge).
func TestClient_CreatePR_ExactHeadDraftArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"number":77}`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	num, err := c.createPR(context.Background(), "feat/x", "main", "[T5] title", "body")
	if err != nil {
		t.Fatalf("createPR: %v", err)
	}
	args := rec.last()
	if !hasArg(args, "POST") {
		t.Errorf("createPR is not a POST: %s", argvString(args))
	}
	if !hasArg(args, "head=feat/x") {
		t.Errorf("createPR did not send the exact head=feat/x: %s", argvString(args))
	}
	if !hasArg(args, "base=main") {
		t.Errorf("createPR did not target base=main: %s", argvString(args))
	}
	if !hasArg(args, "draft=true") {
		t.Errorf("createPR did not open a draft (never-merge): %s", argvString(args))
	}
	if path := args[len(args)-1]; path != "repos/o/r/pulls" {
		t.Errorf("createPR path = %q, want repos/o/r/pulls", path)
	}
	if num != 77 {
		t.Errorf("createPR number = %d, want 77", num)
	}
}

func TestClient_CreatePR_FailsClosedWithoutBudget(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"number":77}`)}
	c := newGHClientWithBudget("o/r", nil)
	c.run = rec.run

	if _, err := c.createPR(context.Background(), "feat/x", "main", "title", "body"); err == nil {
		t.Fatal("createPR succeeded without a content-write budget")
	}
	if len(rec.calls) != 0 {
		t.Fatalf("createPR made %d medium call(s) after budget construction failed", len(rec.calls))
	}
}

func TestClient_CreateCommentFailsClosedWhenBudgetConstructionFailed(t *testing.T) {
	wantErr := errors.New("budget unavailable")
	c := &ghClient{repo: "o/r", budgetErr: wantErr}
	if _, err := c.createComment(t.Context(), 7, "body"); !errors.Is(err, wantErr) {
		t.Fatalf("createComment error = %v, want wrapped budget error", err)
	}
}

func TestClient_PullHeadSHA_ArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"number":9,"head":{"sha":"abc123","ref":"feat/x"}}`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	sha, err := c.pullHeadSHA(context.Background(), 9)
	if err != nil {
		t.Fatalf("pullHeadSHA: %v", err)
	}
	assertFreshRead(t, rec.last())
	if path := rec.last()[len(rec.last())-1]; path != "repos/o/r/pulls/9" {
		t.Errorf("pull path = %q, want repos/o/r/pulls/9", path)
	}
	if sha != "abc123" {
		t.Errorf("head sha = %q, want abc123", sha)
	}
}

func TestClient_PullSnapshot_ParsesLifecycleAndHead(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"number":9,"state":"closed","merged":true,"head":{"sha":"abc123","ref":"feat/x"}}`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	got, err := c.pullSnapshot(context.Background(), 9)
	if err != nil {
		t.Fatalf("pullSnapshot: %v", err)
	}
	assertFreshRead(t, rec.last())
	if got.State != "closed" || !got.Merged || got.HeadSHA != "abc123" || got.HeadRef != "feat/x" {
		t.Fatalf("snapshot = %+v, want merged closed feat/x@abc123", got)
	}
}

func TestClient_CheckRuns_ArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`{"total_count":2,"check_runs":[
		{"name":"build","status":"completed","conclusion":"success"},
		{"name":"test","status":"in_progress","conclusion":null}
	]}`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	runs, err := c.checkRuns(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("checkRuns: %v", err)
	}
	args := rec.last()
	assertFreshRead(t, args)
	if !hasArg(args, "--paginate") {
		t.Errorf("check-runs read omits --paginate: %s", argvString(args))
	}
	// The Checks API (check-runs), latest-run-per-check — never a status rollup.
	path := args[len(args)-1]
	if !strings.Contains(path, "repos/o/r/commits/abc123/check-runs") || !strings.Contains(path, "filter=latest") {
		t.Errorf("check-runs path %q lacks commits/abc123/check-runs?filter=latest", path)
	}
	if strings.Contains(path, "/status") {
		t.Errorf("check read uses the status rollup, not the Checks API: %q", path)
	}
	if len(runs) != 2 || runs[0].Name != "build" || runs[0].Conclusion != "success" || runs[1].Status != "in_progress" {
		t.Fatalf("parsed runs = %+v, want build/success + test/in_progress", runs)
	}
}

// gh --paginate concatenates the check-runs object across pages; the parser
// stitches each page's check_runs array.
func TestClient_CheckRuns_ParsesConcatenatedPages(t *testing.T) {
	rec := &execRecorder{out: []byte(
		`{"check_runs":[{"name":"a","status":"completed","conclusion":"success"}]}` +
			`{"check_runs":[{"name":"b","status":"completed","conclusion":"failure"}]}`)}
	c := &ghClient{repo: "o/r", run: rec.run}
	runs, err := c.checkRuns(context.Background(), "sha")
	if err != nil {
		t.Fatalf("checkRuns: %v", err)
	}
	if len(runs) != 2 || runs[0].Name != "a" || runs[1].Name != "b" {
		t.Fatalf("parsed %+v, want a and b across two pages", runs)
	}
}

func TestClientReadAndCreateFailuresRemainTyped(t *testing.T) {
	wantErr := errors.New("gh unavailable")

	for _, tc := range []struct {
		name string
		call func(*ghClient) error
	}{
		{"find transport", func(c *ghClient) error { _, _, err := c.findOpenPRByHead(t.Context(), "head"); return err }},
		{"compare transport", func(c *ghClient) error { _, err := c.compareAhead(t.Context(), "main", "head"); return err }},
		{"create transport", func(c *ghClient) error {
			_, err := c.createPR(t.Context(), "head", "main", "title", "body")
			return err
		}},
		{"snapshot transport", func(c *ghClient) error { _, err := c.pullSnapshot(t.Context(), 7); return err }},
		{"checks transport", func(c *ghClient) error { _, err := c.checkRuns(t.Context(), "sha"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &execRecorder{err: wantErr}
			client := &ghClient{repo: "o/r", run: recorder.run}
			if err := tc.call(client); err == nil {
				t.Fatal("transport failure was accepted")
			}
		})
	}

	for _, tc := range []struct {
		name string
		call func(*ghClient) error
	}{
		{"find decode", func(c *ghClient) error { _, _, err := c.findOpenPRByHead(t.Context(), "head"); return err }},
		{"compare decode", func(c *ghClient) error { _, err := c.compareAhead(t.Context(), "main", "head"); return err }},
		{"create decode", func(c *ghClient) error {
			_, err := c.createPR(t.Context(), "head", "main", "title", "body")
			return err
		}},
		{"snapshot decode", func(c *ghClient) error { _, err := c.pullSnapshot(t.Context(), 7); return err }},
		{"checks decode", func(c *ghClient) error { _, err := c.checkRuns(t.Context(), "sha"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &execRecorder{out: []byte("not json")}
			client := &ghClient{repo: "o/r", run: recorder.run}
			if err := tc.call(client); err == nil {
				t.Fatal("malformed response was accepted")
			}
		})
	}

	emptyHead := &execRecorder{out: []byte(`{"state":"open","head":{"sha":""}}`)}
	if _, err := (&ghClient{repo: "o/r", run: emptyHead.run}).pullSnapshot(t.Context(), 7); err == nil {
		t.Fatal("empty pull head was accepted")
	}
	if got := (&ghClient{repo: "repository", run: emptyHead.run}).owner(); got != "repository" {
		t.Fatalf("owner without slash = %q", got)
	}
}

func TestClientParsersStitchPullPagesAndRejectMalformedPages(t *testing.T) {
	pulls, err := parsePulls([]byte(`[{"number":1}][{"number":2}]`))
	if err != nil || len(pulls) != 2 {
		t.Fatalf("pull pages = %+v, %v", pulls, err)
	}
	if _, err := parsePulls([]byte(`[{"number":1}]bad`)); err == nil {
		t.Fatal("malformed pull page was accepted")
	}
	if _, err := parseCheckRuns([]byte(`{"check_runs":[]}bad`)); err == nil {
		t.Fatal("malformed check-run page was accepted")
	}
}

func TestShellGHNamesExecutableFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := shellGH(t.Context(), "api", "user"); err == nil || !strings.Contains(err.Error(), "gh api user") {
		t.Fatalf("shellGH executable failure = %v", err)
	}
}
