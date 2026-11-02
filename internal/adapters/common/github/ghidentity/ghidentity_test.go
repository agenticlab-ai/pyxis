package ghidentity

import (
	"context"
	"errors"
	"testing"
)

func reset(t *testing.T) {
	t.Helper()
	set := func() {
		mu.Lock()
		resolved, host, login = false, "", ""
		mu.Unlock()
	}
	set()
	t.Cleanup(set)
}

func stubEnv(t *testing.T, vals map[string]string) {
	t.Helper()
	prev := getenv
	getenv = func(k string) string { return vals[k] }
	t.Cleanup(func() { getenv = prev })
}

func TestResolveReturnsHostAndLogin(t *testing.T) {
	reset(t)
	stubEnv(t, nil)
	prev := runGH
	runGH = func(context.Context, ...string) ([]byte, error) { return []byte("octocat\n"), nil }
	t.Cleanup(func() { runGH = prev })

	h, l := Resolve(context.Background())
	if h != DefaultHost || l != "octocat" {
		t.Errorf("Resolve = (%q, %q), want (%q, octocat)", h, l, DefaultHost)
	}
}

// A GitHub Enterprise Server deployment meters its own limits, so GH_HOST
// keys its own ledgers and its own cutover marker.
func TestResolveHonorsGHHost(t *testing.T) {
	reset(t)
	stubEnv(t, map[string]string{"GH_HOST": "ghe.example.com"})
	prev := runGH
	runGH = func(context.Context, ...string) ([]byte, error) { return []byte("octocat"), nil }
	t.Cleanup(func() { runGH = prev })

	if h, _ := Resolve(context.Background()); h != "ghe.example.com" {
		t.Errorf("host = %q, want ghe.example.com", h)
	}
}

// An unresolvable identity is the documented fallback, never an error: the
// caller then charges the host umbrella alone — the same window every
// resolved identity charges, so an unresolved writer holds none of its own.
func TestResolveDegradesToHostOnly(t *testing.T) {
	reset(t)
	stubEnv(t, nil)
	prev := runGH
	runGH = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("gh: not authenticated") }
	t.Cleanup(func() { runGH = prev })

	h, l := Resolve(context.Background())
	if h != DefaultHost {
		t.Errorf("host = %q, want %q even when the login is unavailable", h, DefaultHost)
	}
	if l != "" {
		t.Errorf("login = %q, want empty on a failed lookup", l)
	}
}

// A transient failure must not pin the process to the coarse key for its
// lifetime — the lookup retries until it succeeds, then memoizes.
func TestResolveRetriesAfterFailureThenMemoizes(t *testing.T) {
	reset(t)
	stubEnv(t, nil)
	calls := 0
	prev := runGH
	runGH = func(context.Context, ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return []byte("octocat"), nil
	}
	t.Cleanup(func() { runGH = prev })

	if _, l := Resolve(context.Background()); l != "" {
		t.Fatalf("first resolve returned %q, want empty", l)
	}
	if _, l := Resolve(context.Background()); l != "octocat" {
		t.Fatalf("retry returned %q, want octocat — a transient failure was memoized", l)
	}
	if _, l := Resolve(context.Background()); l != "octocat" {
		t.Fatalf("third resolve returned %q", l)
	}
	if calls != 2 {
		t.Errorf("gh was invoked %d times, want 2 — the success is not memoized", calls)
	}
}
