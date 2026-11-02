package ghcodehost

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCommitStatusRequiresOptionalCapabilityAndExactRevision(t *testing.T) {
	// A legacy client may still provide proposals and check runs. It must not
	// be mistaken for proof that a local verification status can be published.
	h := newHost(&fakeClient{}, "main")
	for _, tc := range []struct {
		sha  string
		want string
	}{
		{sha: "main", want: "full commit SHA"},
		{sha: statusTestSHA, want: "unavailable"},
	} {
		t.Run(tc.sha, func(t *testing.T) {
			if err := h.SetStatus(context.Background(), tc.sha, "pyxis/local-gate", "success", "", ""); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("publication error=%v, want %q", err, tc.want)
			}
			state, err := h.ReadStatus(context.Background(), tc.sha, "pyxis/local-gate")
			if state != port.CheckPending || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("read state=%v error=%v, want pending and %q", state, err, tc.want)
			}
		})
	}
}
