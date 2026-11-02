package ghidentity

import (
	"context"
	"crypto/sha256"
	"os"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
)

// scopedLookup is the test seam for the per-connection actor lookup.
var scopedLookup = githubauth.RunForHost

// ResolverForTokenEnv resolves the actor behind one connection. Each resolver
// owns its memoization; changing a token causes a fresh actor lookup. A failed
// lookup keeps the existing host-wide budget fallback and never becomes a
// separate per-connection quota.
func ResolverForTokenEnv(tokenEnv string, explicitHost ...string) func(context.Context) (string, string) {
	selectedHost := ""
	if len(explicitHost) > 0 {
		selectedHost = explicitHost[0]
	}
	if tokenEnv == "" && selectedHost == "" {
		return Resolve
	}
	var mu sync.Mutex
	var fingerprint [32]byte
	var cached string
	return func(ctx context.Context) (string, string) {
		mu.Lock()
		defer mu.Unlock()
		token := os.Getenv(tokenEnv)
		current := sha256.Sum256([]byte(token))
		if cached != "" && fingerprint == current {
			return DefaultHost, cached
		}
		cached = ""
		if tokenEnv != "" && token == "" {
			return DefaultHost, ""
		}
		ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
		defer cancel()
		out, err := scopedLookup(ctx, tokenEnv, selectedHost, nil, "api", "--hostname", DefaultHost, "user", "-q", ".login")
		if err != nil {
			return DefaultHost, ""
		}
		cached, fingerprint = strings.TrimSpace(string(out)), current
		return DefaultHost, cached
	}
}
