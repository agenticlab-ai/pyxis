package ghtracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/ghidentity"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
)

func clientWithTokenEnv(repo, tokenEnv string, b *budget.Budget, explicitHost ...string) *ghClient {
	host := ""
	if len(explicitHost) > 0 {
		host = explicitHost[0]
	}
	if tokenEnv == "" && host == "" {
		if b == nil {
			return newGHClient(repo)
		}
		return newGHClientWithBudget(repo, b)
	}
	c := &ghClient{repo: repo, budget: b, sleep: ctxSleep}
	c.run = func(ctx context.Context, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, host, nil, args...)
	}
	if b == nil {
		resolve := ghidentity.ResolverForTokenEnv(tokenEnv, host)
		host, login := resolve(context.Background())
		c.budget, c.budgetErr = budget.NewForIdentityResolver(host, login, resolve, nil, nil)
	}
	return c
}

// NewGHWithTokenEnv binds tracker operations to the task connection's actor.
func NewGHWithTokenEnv(repo, label, tokenEnv string, b *budget.Budget, explicitHost ...string) *Tracker {
	return newTrackerWithRepository(clientWithTokenEnv(repo, tokenEnv, b, explicitHost...), label, repo)
}

func NewGHForPlanWithTokenEnv(repo, label, planRef, tokenEnv string, b *budget.Budget, explicitHost ...string) (*Tracker, error) {
	return newTrackerForPlanWithRepository(clientWithTokenEnv(repo, tokenEnv, b, explicitHost...), label, planRef, repo)
}
