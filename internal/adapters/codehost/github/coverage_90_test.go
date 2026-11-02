package ghcodehost

import (
	"context"
	"testing"
)

func TestNewGHClientBindsProductionDependencies(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	c := newGHClient("owner/repo")
	if c.repo != "owner/repo" || c.run == nil || c.runInput == nil || c.budget == nil || c.budgetErr != nil {
		t.Fatalf("newGHClient = %+v, want fully bound production client", c)
	}
}

func TestCreateCommentSendsBodyOnStdinAndReturnsID(t *testing.T) {
	var gotInput []byte
	var gotArgs []string
	c := &ghClient{
		repo: "owner/repo",
		runInput: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
			gotInput = append([]byte(nil), input...)
			gotArgs = append([]string(nil), args...)
			return []byte(`{"id":42}`), nil
		},
	}

	id, err := c.createComment(t.Context(), 7, "line one\nline two")
	if err != nil {
		t.Fatal(err)
	}
	if id != "42" {
		t.Fatalf("comment id = %q, want 42", id)
	}
	if string(gotInput) != `{"body":"line one\nline two"}` {
		t.Fatalf("stdin = %q", gotInput)
	}
	if got := argvString(gotArgs); got != "api --method POST repos/owner/repo/issues/7/comments --input -" {
		t.Fatalf("argv = %q", got)
	}
}

func TestCreateCommentRejectsMalformedOrIDLessResponses(t *testing.T) {
	for _, response := range []string{"not-json", `{}`} {
		t.Run(response, func(t *testing.T) {
			c := &ghClient{
				repo: "owner/repo",
				runInput: func(context.Context, []byte, ...string) ([]byte, error) {
					return []byte(response), nil
				},
			}
			if _, err := c.createComment(t.Context(), 7, "body"); err == nil {
				t.Fatalf("createComment accepted response %q", response)
			}
		})
	}
}
