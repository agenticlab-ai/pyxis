package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func fixtureFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "approval.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReaderObservesExactHeadAndReview(t *testing.T) {
	head := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	path := fixtureFile(t, `{"schema":"draft-approval-fixture/v1","pull_requests":[{"number":17,"head":"`+head+`","reviews":[{"id":1,"author":"draft-agent","state":"APPROVED","commit_id":"`+head+`","body":"agent"},{"id":2,"author":"mdarabi","state":"APPROVED","commit_id":"`+head+`","body":"8 min; FND-1 outside §0"}]}]}`)
	reader := Reader{Path: path, Operator: "mdarabi"}
	got, err := reader.Approval(t.Context(), 17, head)
	if err != nil || got.CaptureLine != "8 min; FND-1 outside §0" || got.Operator != "mdarabi" {
		t.Fatalf("approval=%+v error=%v", got, err)
	}
	for _, tc := range []struct {
		pull int
		head string
	}{{17, other}, {18, head}} {
		if _, err := reader.Approval(t.Context(), tc.pull, tc.head); !errors.Is(err, approval.ErrNoApproval) {
			t.Fatalf("pull %d head %s: %v", tc.pull, tc.head, err)
		}
	}
	if err := os.WriteFile(path, []byte(`{"schema":"draft-approval-fixture/v1","pull_requests":[{"number":17,"head":"`+other+`","reviews":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Approval(t.Context(), 17, head); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("moved fixture head: %v", err)
	}
}

func TestReaderRejectsMalformedFixture(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{`, "decode approval fixture"},
		{`{"schema":"draft-approval-fixture/v2"}`, "unsupported schema"},
		{`{"schema":"draft-approval-fixture/v1","unknown":true}`, "decode approval fixture"},
		{`{"schema":"draft-approval-fixture/v1"} {}`, "trailing content"},
		{`{"schema":"draft-approval-fixture/v1","pull_requests":[{"number":17,"head":"a"},{"number":17,"head":"a"}]}`, "duplicate pull request"},
	} {
		reader := Reader{Path: fixtureFile(t, tc.body), Operator: "mdarabi"}
		if _, err := reader.Approval(t.Context(), 17, "a"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("fixture %s: %v", tc.body, err)
		}
	}
	if _, err := (Reader{}).Approval(t.Context(), 0, "a"); err == nil {
		t.Fatal("invalid pull accepted")
	}
	if _, err := (Reader{Path: "missing"}).Approval(t.Context(), 17, "a"); err == nil {
		t.Fatal("missing fixture accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Reader{Path: "missing"}).Approval(ctx, 17, "a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
}
