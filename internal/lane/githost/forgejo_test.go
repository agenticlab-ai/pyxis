package githost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

func TestLocalGitLaneCannotFallThroughToHostedMerge(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	body := "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: /configured/origin.git\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if host, err := NewConfigured("ignored/hosted", root, "origin", "main"); host != nil || !errors.Is(err, codehost.ErrWorkflowUnavailable) {
		t.Fatalf("local landing composed a hosted merge adapter: %v, %v", host, err)
	}
}

type laneStatus struct {
	ID          int64  `json:"id"`
	Context     string `json:"context"`
	Status      string `json:"status"`
	Description string `json:"description"`
}
type laneComment struct {
	ID             int64        `json:"id"`
	Body           string       `json:"body"`
	User           forgejo.User `json:"user"`
	OriginalAuthor string       `json:"original_author"`
}
type laneFixture struct {
	mu                          sync.Mutex
	head, actor                 string
	merged                      bool
	statuses                    []laneStatus
	comments                    []laneComment
	fail                        map[string]int
	posts                       []string
	moveOnStatus, moveOnApprove bool
}

func newLaneForgejo(t *testing.T) (*Host, *laneFixture) {
	t.Helper()
	f := &laneFixture{head: "sha", actor: "machine", statuses: []laneStatus{{ID: 1, Context: "ci", Status: "success"}}, fail: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	client, err := forgejo.New(server.URL+"/base", "token")
	if err != nil {
		t.Fatal(err)
	}
	h := New("o/r", t.TempDir(), "origin", "main")
	h.Forgejo = forgejocodehost.New(client, server.URL+"/base", "o/r", "main")
	return h, f
}

func (f *laneFixture) change(fn func()) { f.mu.Lock(); defer f.mu.Unlock(); fn() }
func (f *laneFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "token token" {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/base/api/v1")
	key := r.Method + " " + p
	if r.Method != "GET" {
		f.posts = append(f.posts, key)
	}
	if status := f.fail[key]; status != 0 {
		http.Error(w, "injected", status)
		return
	}
	respond := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
	}
	if r.Method == "GET" && r.URL.Query().Get("page") != "" && r.URL.Query().Get("page") != "1" {
		respond([]struct{}{})
		return
	}
	switch p {
	case "/user":
		respond(forgejo.User{ID: 5, Login: f.actor})
	case "/repos/o/r/pulls/7":
		p := forgejocodehost.Pull{Number: 7, State: "open", Merged: f.merged}
		p.Head.SHA = f.head
		p.Head.Repo.FullName = "o/r"
		p.Base.Ref = "main"
		if f.merged {
			p.MergeCommitSHA = "merge-sha"
		}
		respond(p)
	case "/repos/o/r/pulls":
		respond([]any{struct {
			Number int `json:"number"`
			Head   struct {
				SHA string `json:"sha"`
			} `json:"head"`
		}{Number: 7, Head: struct {
			SHA string `json:"sha"`
		}{SHA: f.head}}})
	case "/repos/o/r/branches/main":
		respond(struct {
			Name      string `json:"name"`
			Protected bool   `json:"protected"`
			Effective string `json:"effective_branch_protection_name"`
		}{"main", true, "main"})
	case "/repos/o/r/branch_protections/main":
		respond(struct {
			Rule     string   `json:"rule_name"`
			Enabled  bool     `json:"enable_status_check"`
			Contexts []string `json:"status_check_contexts"`
		}{"main", true, []string{"ci"}})
	case "/repos/o/r/commits/sha/statuses":
		respond(f.statuses)
		if f.moveOnStatus {
			f.head = "moved"
		}
	case "/repos/o/r/issues/7/comments":
		if r.Method == "GET" {
			respond(f.comments)
			return
		}
		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		comment := laneComment{ID: int64(len(f.comments) + 1), Body: body.Body, User: forgejo.User{ID: 5, Login: f.actor}}
		f.comments = append(f.comments, comment)
		respond(comment)
	case "/repos/o/r/pulls/7/reviews":
		var body struct {
			Event  string `json:"event"`
			Commit string `json:"commit_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Event != "APPROVED" || body.Commit != "sha" {
			http.Error(w, "wrong approval binding", http.StatusBadRequest)
			return
		}
		respond(struct{}{})
		if f.moveOnApprove {
			f.head = "moved"
		}
	case "/repos/o/r/pulls/7/merge":
		var body struct {
			Do       string `json:"Do"`
			Head     string `json:"head_commit_id"`
			Force    bool   `json:"force_merge"`
			Deferred bool   `json:"merge_when_checks_succeed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Do != "squash" || body.Force || body.Deferred {
			http.Error(w, "unsafe merge request", http.StatusBadRequest)
			return
		}
		if body.Head != f.head {
			http.Error(w, "head changed", http.StatusConflict)
			return
		}
		f.merged = true
		respond(struct{}{})
	default:
		http.Error(w, "unexpected path "+p, http.StatusNotFound)
	}
}

func TestForgejoLaneNativeChecksAndMovedHead(t *testing.T) {
	h, f := newLaneForgejo(t)
	ctx := context.Background()
	if head, err := h.Head(ctx, "7"); err != nil || head != "sha" {
		t.Fatalf("head=%q %v", head, err)
	}
	if green, err := h.AllRequiredGreen(ctx, "7", "sha"); err != nil || !green {
		t.Fatalf("green=%v %v", green, err)
	}
	if green, err := h.AllRequiredGreen(ctx, "7", "stale"); err != nil || green {
		t.Fatal("stale head accepted")
	}
	f.change(func() { f.statuses = nil })
	if green, err := h.AllRequiredGreen(ctx, "7", "sha"); err != nil || green {
		t.Fatal("missing required status accepted")
	}
	f.change(func() { f.statuses = []laneStatus{{ID: 1, Context: "ci", Status: "failure"}} })
	if green, err := h.AllRequiredGreen(ctx, "7", "sha"); err != nil || green {
		t.Fatal("failed check accepted")
	}
	f.change(func() {
		f.statuses = []laneStatus{{ID: 1, Context: "ci", Status: "success"}, {ID: 5, Context: verdict.CheckContext, Status: "success", Description: "approved"}, {ID: 6, Context: verdict.CheckContext, Status: "failure", Description: "new failure"}}
	})
	state, err := h.ReviewVerdict(ctx, "7", "sha")
	if err != nil || state.Status != verdict.NonApproving || state.Reason != "new failure" {
		t.Fatalf("latest verdict=%+v %v", state, err)
	}
	f.change(func() { f.statuses = f.statuses[:2] })
	state, err = h.ReviewVerdict(ctx, "7", "sha")
	if err != nil || state.Status != verdict.Approving {
		t.Fatalf("review=%+v %v", state, err)
	}
	f.change(func() { f.moveOnStatus = true })
	state, err = h.ReviewVerdict(ctx, "7", "sha")
	if err != nil || state.Status == verdict.Approving {
		t.Fatal("head movement during status read accepted")
	}
	f.change(func() { f.head = "sha" })
	if green, _ := h.AllRequiredGreen(ctx, "7", "sha"); green {
		t.Fatal("head movement during required-check read accepted")
	}
	for _, number := range []string{"0", "-1", "bad"} {
		if _, err := h.Head(ctx, number); err == nil {
			t.Fatal("invalid proposal accepted")
		}
		if _, err := h.AllRequiredGreen(ctx, number, "sha"); err == nil {
			t.Fatal("invalid check proposal accepted")
		}
		if _, err := h.MergedIntoMainline(ctx, number, "sha"); err == nil {
			t.Fatal("invalid merge proposal accepted")
		}
		if err := h.RecordVerdict(ctx, number, verdict.Record{}); err == nil {
			t.Fatal("invalid record proposal accepted")
		}
		if _, _, err := h.findBlockComment(ctx, number); err == nil {
			t.Fatal("invalid history proposal accepted")
		}
		if err := (&Machine{Host: h, Name: "machine"}).Act(ctx, number, "sha"); err == nil {
			t.Fatal("invalid act proposal accepted")
		}
	}
}

func TestForgejoLaneUsesAuthenticatedHistoryAndHeadGuard(t *testing.T) {
	h, f := newLaneForgejo(t)
	ctx := context.Background()
	record := verdict.Record{RecordVersion: verdict.RecordVersion, Lane: "HUMAN", PolicyVersion: "policy", HeadRevision: "sha"}
	if err := h.RecordVerdict(ctx, "7", record); err != nil {
		t.Fatal(err)
	}
	f.change(func() {
		f.comments = append(f.comments, laneComment{ID: 50, Body: BlockMarker + "\n```json\n" + string(verdict.AppendBlock(nil, verdict.Record{RecordVersion: verdict.RecordVersion, HeadRevision: "forged"})) + "```", User: forgejo.User{ID: 99}}, laneComment{ID: 51, Body: f.comments[0].Body, User: forgejo.User{ID: 5}, OriginalAuthor: "imported"})
	})
	if head, err := h.LastRecordedHead(ctx, "7"); err != nil || head != "sha" {
		t.Fatalf("trusted history=%q %v", head, err)
	}
	if proposals, err := h.OpenProposals(ctx); err != nil || len(proposals) != 1 || proposals[0].Head != "sha" {
		t.Fatalf("proposals=%+v %v", proposals, err)
	}
	m := &Machine{Host: h, Name: "machine"}
	f.change(func() { f.actor = "other" })
	if err := m.Act(ctx, "7", "sha"); err == nil {
		t.Fatal("wrong actor granted machine authority")
	}
	f.change(func() { f.actor = "machine" })
	if err := m.Act(ctx, "7", "stale"); err == nil {
		t.Fatal("stale approval sent")
	}
	f.change(func() { f.moveOnApprove = true })
	if err := m.Act(ctx, "7", "sha"); err == nil {
		t.Fatal("server expected-head guard was not enforced")
	}
	f.change(func() { f.head = "sha"; f.moveOnApprove = false })
	if err := m.Act(ctx, "7", "sha"); err != nil {
		t.Fatal(err)
	}
	var gitCalls []string
	h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return nil, nil
	}
	if merged, err := h.MergedIntoMainline(ctx, "7", "sha"); err != nil || !merged || len(gitCalls) != 2 || !strings.Contains(gitCalls[1], "merge-sha origin/main") {
		t.Fatalf("merge proof=%v %v %v", merged, err, gitCalls)
	}
	if merged, err := h.MergedIntoMainline(ctx, "7", "stale"); err != nil || merged {
		t.Fatal("different merged head certified")
	}
	h.Git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "merge-base" {
			return nil, errors.New("not ancestor")
		}
		return nil, nil
	}
	if merged, err := h.MergedIntoMainline(ctx, "7", "sha"); err != nil || merged {
		t.Fatal("lying merged badge accepted")
	}
	h.Git = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("fetch failed") }
	if _, err := h.MergedIntoMainline(ctx, "7", "sha"); err == nil {
		t.Fatal("fetch failure hidden")
	}
}

func TestForgejoLaneFailuresAndConfiguredSelection(t *testing.T) {
	for _, tc := range []struct {
		key  string
		call func(*Host) error
	}{
		{"GET /user", func(h *Host) error { _, _, e := h.findBlockComment(t.Context(), "7"); return e }},
		{"GET /user", func(h *Host) error { return (&Machine{Host: h, Name: "machine"}).Act(t.Context(), "7", "sha") }},
		{"GET /repos/o/r/pulls/7", func(h *Host) error { _, e := h.Head(t.Context(), "7"); return e }},
		{"GET /repos/o/r/pulls/7", func(h *Host) error { _, e := h.MergedIntoMainline(t.Context(), "7", "sha"); return e }},
		{"GET /repos/o/r/pulls/7", func(h *Host) error { return (&Machine{Host: h, Name: "machine"}).Act(t.Context(), "7", "sha") }},
		{"GET /repos/o/r/pulls", func(h *Host) error { _, e := h.OpenProposals(t.Context()); return e }},
		{"GET /repos/o/r/issues/7/comments", func(h *Host) error { _, e := h.LastRecordedHead(t.Context(), "7"); return e }},
		{"GET /repos/o/r/branches/main", func(h *Host) error { _, e := h.AllRequiredGreen(t.Context(), "7", "sha"); return e }},
		{"GET /repos/o/r/commits/sha/statuses", func(h *Host) error { _, e := h.ReviewVerdict(t.Context(), "7", "sha"); return e }},
		{"POST /repos/o/r/pulls/7/reviews", func(h *Host) error { return (&Machine{Host: h, Name: "machine"}).Act(t.Context(), "7", "sha") }},
	} {
		t.Run(tc.key, func(t *testing.T) {
			h, f := newLaneForgejo(t)
			f.change(func() { f.fail[tc.key] = 403 })
			if err := tc.call(h); err == nil {
				t.Fatal("native failure was swallowed")
			}
		})
	}
	root := t.TempDir()
	if h, err := NewConfigured("o/r", root, "", "main"); err != nil || h.Forgejo != nil {
		t.Fatalf("legacy=%+v %v", h, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    code_host:
      connection: local
      repository: configured/repo
`
	path := filepath.Join(root, ".pyxis/config.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if h, err := NewConfigured("o/r", root, "", "main"); err != nil || h.Forgejo == nil || h.Repo != "configured/repo" {
		t.Fatalf("configured=%+v %v", h, err)
	}
	if err := os.WriteFile(path, []byte("integrations:\n  unexpected: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfigured("o/r", root, "", "main"); err == nil {
		t.Fatal("malformed config accepted")
	}
	if _, err := NewConfigured("invalid", t.TempDir(), "", "main"); err == nil {
		t.Fatal("invalid repo accepted")
	}
}
