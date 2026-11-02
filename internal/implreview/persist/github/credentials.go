package github

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

// NewHostAdapterWithTokenEnv binds identity, permissions, publication and
// exact-comment recovery to the code-host connection's selected credential.
func NewHostAdapterWithTokenEnv(repository, tokenEnv string, state publication.Store, explicitHost ...string) (*HostAdapter, error) {
	host := ""
	if len(explicitHost) > 0 {
		host = explicitHost[0]
	}
	if tokenEnv == "" && host == "" {
		return NewHostAdapter(repository, state)
	}
	c := &ghClient{repo: repository, run: func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, host, stdin, args...)
	}}
	return newHostAdapter(repository, c, state)
}
