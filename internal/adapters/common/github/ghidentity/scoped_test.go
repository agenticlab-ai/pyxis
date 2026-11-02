package ghidentity

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestScopedIdentitySeparatesAccountsAndRefreshesChangedToken(t *testing.T) {
	prev := scopedLookup
	scopedLookup = func(ctx context.Context, tokenEnv, _ string, _ []byte, _ ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		token := os.Getenv(tokenEnv)
		if token == "" {
			return nil, errors.New("missing token")
		}
		return []byte(token), nil
	}
	t.Cleanup(func() { scopedLookup = prev })

	t.Setenv("TEST_TASK_ACTOR", "task-actor")
	t.Setenv("TEST_CODE_ACTOR", "code-actor")
	t.Setenv("GH_HOST", "ambient.example")
	tasks, code := ResolverForTokenEnv("TEST_TASK_ACTOR"), ResolverForTokenEnv("TEST_CODE_ACTOR")
	for _, tc := range []struct {
		resolver func(context.Context) (string, string)
		want     string
	}{{tasks, "task-actor"}, {code, "code-actor"}, {tasks, "task-actor"}} {
		host, actor := tc.resolver(context.Background())
		if host != DefaultHost || actor != tc.want {
			t.Fatalf("identity %s/%s, want %s", host, actor, tc.want)
		}
	}
	t.Setenv("TEST_CODE_ACTOR", "rotated-actor")
	if _, actor := code(context.Background()); actor != "rotated-actor" {
		t.Fatalf("stale actor after rotation: %s", actor)
	}
	t.Setenv("TEST_TASK_ACTOR", "")
	if host, actor := tasks(context.Background()); host != DefaultHost || actor != "" {
		t.Fatalf("absent credential reused actor: %s/%s", host, actor)
	}
	t.Setenv("TEST_TASK_ACTOR", "task-actor")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, actor := tasks(ctx); actor != "" {
		t.Fatalf("failed lookup memoized: %s", actor)
	}
	if _, actor := tasks(context.Background()); actor != "task-actor" {
		t.Fatalf("failed lookup was not retried: %s", actor)
	}
}
