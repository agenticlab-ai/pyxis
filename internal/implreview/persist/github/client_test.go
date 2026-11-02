package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestGHClientPostComment — the production client builds the right POST argv,
// passes the body on stdin (never argv), and decodes the created comment id.
// The subprocess is faked via the run seam; no real `gh`, no network.
func TestGHClientPostComment(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	var gotStdin string
	c := &ghClient{
		repo: "agenticlab-ai/pyxis",
		run: func(_ context.Context, stdin []byte, args ...string) ([]byte, error) {
			gotArgs = args
			gotStdin = string(stdin)
			return []byte(`{"id": 987654321}`), nil
		},
	}
	id, err := c.postComment(context.Background(), 412, "the artifact body\nwith ```impl-review fences\n")
	if err != nil {
		t.Fatalf("postComment: %v", err)
	}
	if id != "987654321" {
		t.Fatalf("id=%q, want 987654321", id)
	}
	// argv: api --method POST repos/agenticlab-ai/pyxis/issues/412/comments --input -
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"api", "--method POST", "repos/agenticlab-ai/pyxis/issues/412/comments", "--input -"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q missing %q", joined, want)
		}
	}
	// The fence-carrying body went on stdin, not argv (arbitrary content never
	// touches the command line).
	if !strings.Contains(gotStdin, "impl-review fences") {
		t.Fatalf("body not passed on stdin: %q", gotStdin)
	}
	if strings.Contains(joined, "impl-review fences") {
		t.Fatalf("body leaked into argv: %q", joined)
	}
}

// TestGHClientReadComment — the read builds the right GET argv with the no-cache
// header and decodes the body.
func TestGHClientReadComment(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	c := &ghClient{
		repo: "agenticlab-ai/pyxis",
		run: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte(`{"body": "the recovered comment body"}`), nil
		},
	}
	body, err := c.readComment(context.Background(), "987654321")
	if err != nil {
		t.Fatalf("readComment: %v", err)
	}
	if body != "the recovered comment body" {
		t.Fatalf("body=%q", body)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"api", "repos/agenticlab-ai/pyxis/issues/comments/987654321", noCacheHeader} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q missing %q", joined, want)
		}
	}
}

func TestGHClientCurrentLoginUsesAuthenticatedNativeIdentity(t *testing.T) {
	var gotArgs []string
	c := &ghClient{
		repo: "agenticlab-ai/pyxis",
		run: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte("review-bot\n"), nil
		},
	}
	login, err := c.currentLogin(context.Background())
	if err != nil {
		t.Fatalf("currentLogin: %v", err)
	}
	if login != "review-bot" {
		t.Fatalf("login = %q", login)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"api user", "--jq .login", "--header " + noCacheHeader} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args = %q, missing %q", joined, want)
		}
	}
}

// TestGHClientPostCommentPropagatesError — a `gh` failure surfaces as an error,
// never a silent empty id (the adapter's retry then depends on this).
func TestGHClientPostCommentPropagatesError(t *testing.T) {
	t.Parallel()
	c := &ghClient{
		repo: "agenticlab-ai/pyxis",
		run: func(_ context.Context, _ []byte, _ ...string) ([]byte, error) {
			return nil, errors.New("gh: 502 bad gateway")
		},
	}
	if _, err := c.postComment(context.Background(), 412, "x"); err == nil {
		t.Fatal("expected the gh failure to propagate")
	}
}

// TestGHClientPostCommentNoID — a success response carrying no id is an error
// (a created comment must have an id to address later).
func TestGHClientPostCommentNoID(t *testing.T) {
	t.Parallel()
	c := &ghClient{
		repo: "agenticlab-ai/pyxis",
		run: func(_ context.Context, _ []byte, _ ...string) ([]byte, error) {
			return []byte(`{}`), nil
		},
	}
	if _, err := c.postComment(context.Background(), 412, "x"); err == nil {
		t.Fatal("expected an error when the created comment carried no id")
	}
}

func TestGHClientDecodeAndReadFailures(t *testing.T) {
	c := &ghClient{repo: "agenticlab-ai/pyxis", run: func(context.Context, []byte, ...string) ([]byte, error) {
		return []byte("{"), nil
	}}
	if _, err := c.postComment(t.Context(), 1, "body"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("post decode error = %v", err)
	}
	transport := errors.New("transport")
	c.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, transport }
	if _, err := c.readComment(t.Context(), "1"); !errors.Is(err, transport) {
		t.Fatalf("read transport error = %v", err)
	}
	c.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte("{"), nil }
	if _, err := c.readComment(t.Context(), "1"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("read decode error = %v", err)
	}
}
