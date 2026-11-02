// Package ghidentity resolves the GitHub host and authenticated login that
// key the shared content-write budget (design:state-reader-v4 §3.8).
//
// It exists so the two GitHub adapters cannot disagree about which ledger
// they meter against: two independent resolutions would silently partition
// one identity's quota into two ledgers, which is the defect the keyed
// ledger removes (AP-11).
package ghidentity

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DefaultHost is GitHub's public host, used when no GH_HOST names another. A
// GitHub Enterprise Server deployment meters its own limits, so it keys its
// own ledgers and its own cutover marker.
const DefaultHost = "github.com"

// resolveTimeout bounds the identity lookup. Failing to resolve is not an
// error — it degrades to the umbrella-only key — so it must never delay a
// write for long.
const resolveTimeout = 5 * time.Second

// runGH and getenv are the test seams; production binds the real ones.
var (
	runGH = func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "gh", args...).Output()
	}
	getenv = os.Getenv
)

var (
	mu       sync.Mutex
	resolved bool
	host     string
	login    string
)

// Resolve returns the medium host and the authenticated login, memoized once
// a login is found so the lookup costs one subprocess per process.
//
// Login is "" when it cannot be determined — an unauthenticated path, a
// credential the medium will not resolve, or a transient failure. That is the
// documented fallback rather than an error: the budget then charges the host
// umbrella alone, which is the same window every resolved identity charges,
// so an unresolved writer can never hold a window of its own.
//
// A FAILED lookup is not memoized: a transient blip must not pin the process
// to the coarse key for its lifetime.
func Resolve(ctx context.Context) (string, string) {
	mu.Lock()
	defer mu.Unlock()
	if resolved {
		return host, login
	}
	h := hostFromEnv()
	l := lookupLogin(ctx, h)
	host = h
	if l == "" {
		return h, ""
	}
	login, resolved = l, true
	return host, login
}

func hostFromEnv() string {
	if h := strings.TrimSpace(getenv("GH_HOST")); h != "" {
		return h
	}
	return DefaultHost
}

func lookupLogin(ctx context.Context, h string) string {
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	out, err := runGH(ctx, "api", "--hostname", h, "user", "-q", ".login")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
