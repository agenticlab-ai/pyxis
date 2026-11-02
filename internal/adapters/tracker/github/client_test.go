package ghtracker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

func TestCtxSleepCompletesWithoutCancellation(t *testing.T) {
	if err := ctxSleep(context.Background(), 0); err != nil {
		t.Fatalf("ctxSleep: %v", err)
	}
}

// assertFreshRead is the shared exact-fresh contract for a read: it is an exact
// REST call (path under repos/…), never the search index, and carries the
// no-cache directive with no conditional (If-None-Match) header (AP-15).
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
	// The endpoint path (the last non-flag arg) is an exact repos/… REST path.
	path := args[len(args)-1]
	if !strings.HasPrefix(path, "repos/") {
		t.Errorf("read path %q is not an exact repos/… REST endpoint", path)
	}
}

func TestClient_OpenIssuesWithLabel_ExactFreshArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(`[
		{"number":11,"title":"[T1] One","body":"c1","state":"open","labels":[{"name":"local-agent"},{"name":"kind:feat"}]},
		{"number":12,"title":"[T2] Two","body":"c2","state":"OPEN","labels":[{"name":"local-agent"}],"pull_request":{}}
	]`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	items, err := c.openIssuesWithLabel(context.Background(), "local-agent")
	if err != nil {
		t.Fatalf("openIssuesWithLabel: %v", err)
	}

	args := rec.last()
	assertFreshRead(t, args)
	if !hasArg(args, "--paginate") {
		t.Errorf("listing omits --paginate (an exact read must be complete): %s", argvString(args))
	}
	if !hasArg(args, "GET") {
		t.Errorf("listing is not an explicit GET: %s", argvString(args))
	}
	path := args[len(args)-1]
	if !strings.Contains(path, "repos/o/r/issues") || !strings.Contains(path, "state=open") || !strings.Contains(path, "labels=local-agent") {
		t.Errorf("listing path %q lacks repos/o/r/issues?state=open&labels=local-agent", path)
	}

	if len(items) != 2 {
		t.Fatalf("parsed %d items, want 2", len(items))
	}
	if items[0].Number != 11 || items[0].Body != "c1" || items[0].State != "open" || items[0].IsPR {
		t.Errorf("item[0] = %+v, want #11 open non-PR", items[0])
	}
	if len(items[0].Labels) != 2 || items[0].Labels[0] != "local-agent" {
		t.Errorf("item[0] labels = %v, want [local-agent kind:feat]", items[0].Labels)
	}
	if items[1].State != "open" { // "OPEN" is lowercased
		t.Errorf("item[1] state = %q, want lowercased open", items[1].State)
	}
	if !items[1].IsPR {
		t.Errorf("item[1] should be flagged as a pull request")
	}
}

// gh --paginate concatenates JSON arrays across pages; the parser stitches them.
func TestClient_OpenIssues_ParsesConcatenatedPages(t *testing.T) {
	rec := &execRecorder{out: []byte(
		`[{"number":1,"title":"[T1] a","state":"open","labels":[{"name":"local-agent"}]}]` +
			`[{"number":2,"title":"[T2] b","state":"open","labels":[{"name":"local-agent"}]}]`)}
	c := &ghClient{repo: "o/r", run: rec.run}
	items, err := c.openIssuesWithLabel(context.Background(), "local-agent")
	if err != nil {
		t.Fatalf("openIssuesWithLabel: %v", err)
	}
	if len(items) != 2 || items[0].Number != 1 || items[1].Number != 2 {
		t.Fatalf("parsed %+v, want #1 and #2 across two pages", items)
	}
}

func TestClient_BlockedBy_ArgvParseAnd404Tolerance(t *testing.T) {
	rec := &execRecorder{out: []byte(`[{"number":11},{"number":12}]`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	nums, err := c.blockedBy(context.Background(), 20)
	if err != nil {
		t.Fatalf("blockedBy: %v", err)
	}
	assertFreshRead(t, rec.last())
	path := rec.last()[len(rec.last())-1]
	if !strings.Contains(path, "repos/o/r/issues/20/dependencies/blocked_by") {
		t.Errorf("blocked_by path %q wrong", path)
	}
	if len(nums) != 2 || nums[0] != 11 || nums[1] != 12 {
		t.Errorf("blockedBy = %v, want [11 12]", nums)
	}

	// A repo without the dependency surface (HTTP 404) reads as "no blockers".
	rec404 := &execRecorder{err: errors.New("gh api ...: HTTP 404: Not Found")}
	c404 := &ghClient{repo: "o/r", run: rec404.run}
	nums, err = c404.blockedBy(context.Background(), 20)
	if err != nil || nums != nil {
		t.Errorf("404 blockedBy = (%v, %v), want (nil, nil)", nums, err)
	}
}

func TestClient_Comments_ExactFreshArgvAndParse(t *testing.T) {
	rec := &execRecorder{out: []byte(
		`[{"body":"ordinary"}]` +
			`[{"body":"<!-- fs-spec:v1 {\"goal\":\"x\"} -->"}]`)}
	c := &ghClient{repo: "o/r", run: rec.run}

	bodies, err := c.comments(context.Background(), 20)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	args := rec.last()
	assertFreshRead(t, args)
	if !hasArg(args, "--paginate") || !hasArg(args, "GET") {
		t.Errorf("comment listing is not a complete explicit GET: %s", argvString(args))
	}
	if path := args[len(args)-1]; path != "repos/o/r/issues/20/comments?per_page=100" {
		t.Errorf("comments path = %q, want exact issue-comments endpoint", path)
	}
	if len(bodies) != 2 || bodies[0] != "ordinary" || !strings.Contains(bodies[1], "fs-spec:v1") {
		t.Fatalf("comments = %q, want two bodies across concatenated pages", bodies)
	}
}

func TestClient_Comment_ArgvIsPost(t *testing.T) {
	rec := &execRecorder{out: []byte(`{}`)}
	c := &ghClient{repo: "o/r", run: rec.run}
	if err := c.comment(context.Background(), 7, "hello body"); err != nil {
		t.Fatalf("comment: %v", err)
	}
	args := rec.last()
	if !hasArg(args, "POST") || !hasArg(args, "body=hello body") {
		t.Errorf("comment argv = %s, want POST with -f body=…", argvString(args))
	}
	if args[len(args)-1] != "repos/o/r/issues/7/comments" {
		t.Errorf("comment path = %q, want repos/o/r/issues/7/comments", args[len(args)-1])
	}
}

func TestClient_Comment_FailsClosedWithoutBudget(t *testing.T) {
	rec := &execRecorder{out: []byte(`{}`)}
	c := newGHClientWithBudget("o/r", nil)
	c.run = rec.run

	if err := c.comment(context.Background(), 7, "hello"); err == nil {
		t.Fatal("comment succeeded without a content-write budget")
	}
	if len(rec.calls) != 0 {
		t.Fatalf("comment made %d medium call(s) after budget construction failed", len(rec.calls))
	}
}

// --- #388: bounded transient-retry on idempotent reads ------------------------

// seqRunner returns a scripted sequence of results, one per call (repeating the
// last), so the retry loop is driven deterministically.
type seqRunner struct {
	results []runResult
	calls   int
}

type runResult struct {
	out []byte
	err error
}

func (s *seqRunner) run(_ context.Context, _ ...string) ([]byte, error) {
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	return s.results[i].out, s.results[i].err
}

// noSleep is an injected backoff that never actually waits, so retry tests are
// instant; it records how many times the loop backed off.
type noSleep struct{ n int }

func (s *noSleep) sleep(context.Context, time.Duration) error { s.n++; return nil }

func transientErr() error { return errors.New("gh api …: gh: HTTP 503 Service Unavailable") }

// A transient 5xx on an idempotent read is retried with backoff, then succeeds —
// a momentary GitHub hiccup no longer takes a whole frontier sweep to zero
// (AP-16). This is the A-12 dogfood symptom.
func TestClient_Read_RetriesTransientThenSucceeds(t *testing.T) {
	ok := []byte(`[{"number":1,"title":"[T1] a","state":"open","labels":[{"name":"local-agent"}]}]`)
	seq := &seqRunner{results: []runResult{{err: transientErr()}, {err: transientErr()}, {out: ok}}}
	sl := &noSleep{}
	c := &ghClient{repo: "o/r", run: seq.run, sleep: sl.sleep}

	items, err := c.openIssuesWithLabel(context.Background(), "local-agent")
	if err != nil {
		t.Fatalf("openIssuesWithLabel after transient retries: %v", err)
	}
	if len(items) != 1 || items[0].Number != 1 {
		t.Fatalf("parsed %+v, want the #1 item from the succeeding attempt", items)
	}
	if seq.calls != 3 {
		t.Errorf("run called %d times, want 3 (two transient failures then success)", seq.calls)
	}
	if sl.n != 2 {
		t.Errorf("backed off %d times, want 2 (one per retry)", sl.n)
	}
}

// A persistently transient read fails closed after the bound — not an unbounded
// loop (AP-16) — and the final error still names the endpoint.
func TestClient_Read_FailsClosedAfterMaxAttempts(t *testing.T) {
	seq := &seqRunner{results: []runResult{{err: transientErr()}}}
	sl := &noSleep{}
	c := &ghClient{repo: "o/r", run: seq.run, sleep: sl.sleep}

	_, err := c.openIssuesWithLabel(context.Background(), "local-agent")
	if err == nil {
		t.Fatal("want an error once the retry bound is exhausted")
	}
	if !strings.Contains(err.Error(), "list open issues labeled") {
		t.Errorf("error lost the endpoint context: %v", err)
	}
	if seq.calls != maxReadAttempts {
		t.Errorf("run called %d times, want maxReadAttempts=%d", seq.calls, maxReadAttempts)
	}
	if sl.n != maxReadAttempts-1 {
		t.Errorf("backed off %d times, want maxReadAttempts-1=%d", sl.n, maxReadAttempts-1)
	}
}

// A non-transient error (here a 404) is never retried — retrying a definitive
// answer is futile — and blockedBy still tolerates it as "no blockers".
func TestClient_Read_DoesNotRetryNonTransient(t *testing.T) {
	seq := &seqRunner{results: []runResult{{err: errors.New("gh api …: HTTP 404: Not Found")}}}
	sl := &noSleep{}
	c := &ghClient{repo: "o/r", run: seq.run, sleep: sl.sleep}

	nums, err := c.blockedBy(context.Background(), 20)
	if err != nil || nums != nil {
		t.Fatalf("blockedBy on 404 = (%v, %v), want (nil, nil)", nums, err)
	}
	if seq.calls != 1 {
		t.Errorf("run called %d times, want 1 (a 404 is not retried)", seq.calls)
	}
	if sl.n != 0 {
		t.Errorf("backed off %d times, want 0", sl.n)
	}
}

// Cancellation during backoff stops the retry promptly with the context error —
// a backoff never outlives a cancelled run. Uses the real ctx-aware sleep with an
// already-cancelled context, so it is still instant.
func TestClient_Read_HonorsCancellationDuringBackoff(t *testing.T) {
	seq := &seqRunner{results: []runResult{{err: transientErr()}}}
	c := &ghClient{repo: "o/r", run: seq.run} // sleep nil -> real ctxSleep
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.openIssuesWithLabel(ctx, "local-agent")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled from a cancelled backoff", err)
	}
	if seq.calls != 1 {
		t.Errorf("run called %d times, want 1 (cancelled before a second attempt)", seq.calls)
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"http 503", errors.New("gh: HTTP 503 Service Unavailable"), true},
		{"http 500", errors.New("gh api x: HTTP 500"), true},
		{"http 502", errors.New("HTTP 502 Bad Gateway"), true},
		{"http 504", errors.New("HTTP 504 Gateway Timeout"), true},
		{"connection reset", errors.New("dial tcp: connection reset by peer"), true},
		{"i/o timeout", errors.New("read tcp: i/o timeout"), true},
		{"404 is not transient", errors.New("HTTP 404: Not Found"), false},
		{"422 is not transient", errors.New("HTTP 422: Unprocessable"), false},
		{"issue #500 in a 404 path is not transient", errors.New("gh api repos/o/r/issues/500/dependencies/blocked_by: HTTP 404"), false},
		{"nil", nil, false},
		{"parse error", errors.New("decode issues: unexpected end of JSON"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
