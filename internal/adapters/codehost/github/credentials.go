package ghcodehost

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/ghidentity"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
)

// NewGHWithTokenEnv binds all code-host reads and writes to the selected
// credential. A nil budget derives the shared ledger from that credential's
// actor; callers with an existing selected-identity budget may pass it.
func NewGHWithTokenEnv(repo, base, tokenEnv string, b *budget.Budget, explicitHost ...string) *Host {
	host := ""
	if len(explicitHost) > 0 {
		host = explicitHost[0]
	}
	if tokenEnv == "" && host == "" {
		if b == nil {
			return NewGH(repo, base)
		}
		return NewGHWithBudget(repo, base, b)
	}
	c := &ghClient{repo: repo, budget: b}
	c.run = func(ctx context.Context, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, host, nil, args...)
	}
	c.runInput = func(ctx context.Context, input []byte, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, host, input, args...)
	}
	c.runLog = func(ctx context.Context, limit int, args ...string) ([]byte, error) {
		env, err := githubauth.EnvironmentForHost(tokenEnv, host)
		if err != nil {
			return nil, err
		}
		return boundedLogCommandEnv(ctx, limit, env, "gh", args...)
	}
	if b == nil {
		resolve := ghidentity.ResolverForTokenEnv(tokenEnv, host)
		host, login := resolve(context.Background())
		c.budget, c.budgetErr = budget.NewForIdentityResolver(host, login, resolve, nil, nil)
	}
	return newHost(c, base)
}

// NewStateReaderWithTokenEnv binds the batched GraphQL reader to the same
// code-host connection used for proposal writes.
func NewStateReaderWithTokenEnv(repo, tokenEnv string, explicitHost ...string) *StateReader {
	host := ""
	if len(explicitHost) > 0 {
		host = explicitHost[0]
	}
	r := NewStateReader(repo)
	r.client = graphqlShell{tokenEnv: tokenEnv, host: host}
	return r
}
