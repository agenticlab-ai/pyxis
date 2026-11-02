package forgejocodehost

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestNativeInlineCommentsUseOneUnpaginatedRead(t *testing.T) {
	at := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	actor := forgejo.User{ID: 1, Login: "owner"}
	route := snapshotRoute(pullFixture(), protection{}, []nativeReview{{ID: 2, State: "COMMENT", User: actor, SubmittedAt: at, Body: "review summary"}}, nil)
	reads := 0
	client, err := forgejo.NewWithHTTP("https://forgejo.test", "token", &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		var body any
		code := 200
		if strings.HasSuffix(req.URL.Path, "/reviews/2/comments") {
			reads++
			if req.URL.RawQuery != "" {
				t.Errorf("unpaginated endpoint received %s", req.URL.RawQuery)
				code = 400
			}
			body = []nativeComment{{ID: 3, User: actor, CreatedAt: at, Body: "inline request"}}
		} else if page := req.URL.Query().Get("page"); page != "" && page != "1" {
			body = []any{}
		} else {
			body, code = route(req)
		}
		wire, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		// Deliberately no Link or X-Total-Count: Forgejo's inline-comment
		// endpoint emits neither, unlike the paginated review list.
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(wire))), Request: req}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	h := New(client, "https://forgejo.test", "owner/repo", "main")
	state, err := h.state(t.Context(), port.ProposalRef{Number: 7}, port.Cursor{})
	if err != nil || reads != 1 || len(state.Comments) != 2 {
		t.Fatalf("native inline read count=%d state=%+v err=%v", reads, state, err)
	}
	if state.Comments[1].Surface != port.SurfaceInlineReview || state.Comments[1].Body != "inline request" {
		t.Fatalf("inline review was lost: %+v", state.Comments)
	}
}
