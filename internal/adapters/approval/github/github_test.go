package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func testReader(head string, pages ...[]map[string]any) (*Reader, *int) {
	reads := new(int)
	r := &Reader{Repo: "example/repo", Operator: "mdarabi"}
	r.run = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) != 6 || args[0] != "api" || args[1] != "--method" || args[2] != "GET" || args[3] != "-H" || args[4] != noCache {
			return nil, fmt.Errorf("unexpected gh args %v", args)
		}
		path := args[5]
		if strings.Contains(path, "/reviews?") {
			for i, page := range pages {
				if strings.HasSuffix(path, fmt.Sprintf("page=%d", i+1)) {
					return json.Marshal(page)
				}
			}
			return []byte("[]"), nil
		}
		if path != "repos/example/repo/pulls/17" {
			return nil, fmt.Errorf("unexpected path %s", path)
		}
		*reads++
		return json.Marshal(map[string]any{"head": map[string]string{"sha": head}})
	}
	return r, reads
}

func TestReaderFindsOperatorReviewOfCurrentHead(t *testing.T) {
	head := strings.Repeat("a", 40)
	r, reads := testReader(head, []map[string]any{
		{"id": 1, "user": map[string]string{"login": "draft-agent"}, "state": "APPROVED", "commit_id": head},
		{"id": 2, "user": map[string]string{"login": "mdarabi"}, "state": "APPROVED", "commit_id": head, "body": "11 min; one item inside §0"},
	})
	got, err := r.Approval(t.Context(), 17, head)
	if err != nil || got.Operator != "mdarabi" || got.Head != head || got.ReviewID != 2 || got.CaptureLine != "11 min; one item inside §0" || *reads != 2 {
		t.Fatalf("approval=%+v reads=%d error=%v", got, *reads, err)
	}
}

func TestReaderRefusesStaleOrMovingHead(t *testing.T) {
	head := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	r, _ := testReader(other)
	if _, err := r.Approval(t.Context(), 17, head); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("old head: %v", err)
	}
	r, _ = testReader(head, []map[string]any{{"id": 1, "user": map[string]string{"login": "mdarabi"}, "state": "APPROVED", "commit_id": head}})
	prior := r.run
	reads := 0
	r.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[len(args)-1] == "repos/example/repo/pulls/17" {
			reads++
			if reads == 2 {
				return json.Marshal(map[string]any{"head": map[string]string{"sha": other}})
			}
		}
		return prior(ctx, args...)
	}
	if _, err := r.Approval(t.Context(), 17, head); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("moving head: %v", err)
	}
}

func TestReaderPagesReviewsAndFailsClosedOnMalformedResponses(t *testing.T) {
	head := strings.Repeat("a", 40)
	first := make([]map[string]any, 100)
	for i := range first {
		first[i] = map[string]any{"id": i + 1, "user": map[string]string{"login": "draft-agent"}, "state": "APPROVED", "commit_id": head}
	}
	r, _ := testReader(head, first, []map[string]any{{"id": 101, "user": map[string]string{"login": "mdarabi"}, "state": "APPROVED", "commit_id": head}})
	if got, err := r.Approval(t.Context(), 17, head); err != nil || got.ReviewID != 101 {
		t.Fatalf("paged approval=%+v error=%v", got, err)
	}
	for _, tc := range []struct {
		name, path, body, want string
	}{
		{"head transport", "/pulls/17", "", "read pull"},
		{"head decode", "/pulls/17", "{", "decode pull"},
		{"head absent", "/pulls/17", `{}`, "no valid head"},
		{"reviews transport", "/reviews?", "", "read reviews"},
		{"reviews decode", "/reviews?", "{", "decode reviews"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := testReader(head, nil)
			prior := r.run
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				if strings.Contains(args[len(args)-1], tc.path) {
					if tc.body == "" {
						return nil, errors.New("simulated GitHub failure")
					}
					return []byte(tc.body), nil
				}
				return prior(ctx, args...)
			}
			if _, err := r.Approval(t.Context(), 17, head); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestReaderRejectsInvalidRequest(t *testing.T) {
	configured := New("example/repo", "mdarabi", "GH_TOKEN")
	if configured.Repo != "example/repo" || configured.Operator != "mdarabi" || configured.run == nil {
		t.Fatalf("production reader not configured: %+v", configured)
	}
	for _, r := range []*Reader{nil, {}, {Repo: "not-a-slug", Operator: "mdarabi", run: func(context.Context, ...string) ([]byte, error) { return nil, nil }}} {
		if _, err := r.Approval(t.Context(), 17, strings.Repeat("a", 40)); err == nil {
			t.Fatalf("invalid reader %+v accepted", r)
		}
	}
	r, _ := testReader(strings.Repeat("a", 40))
	if _, err := r.Approval(t.Context(), 17, "short"); err == nil {
		t.Fatal("short head accepted")
	}
}
