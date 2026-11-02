package harvest

import (
	"context"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// Adapter is the harvest's combined medium edge: enumerate, exact-read, and
// author-resolve against one code host.
type Adapter interface {
	Enumerator
	persist.Reader
	AuthorResolver
}

// NewConfiguredAdapter constructs the GitHub harvest adapter from integration
// bindings. Forgejo construction belongs to adapters/persist/forgejo, wired by
// the composition root.
func NewConfiguredAdapter(root, repository string) (Adapter, error) {
	cfg, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	binding, err := cfg.CodeHost(repository)
	if err != nil {
		return nil, err
	}
	if binding.Provider == integrations.Forgejo {
		return nil, errors.New("harvest: forgejo adapter is constructed by adapters/persist/forgejo")
	}
	if binding.Provider == integrations.LocalGit {
		return nil, fmt.Errorf("%w: local Git review harvest is not available in this build", codehost.ErrWorkflowUnavailable)
	}
	if binding.TokenEnv != "" || cfg.Bindings.CodeHost != (integrations.Binding{}) {
		return newGitHubAdapter(binding.Repository, func(ctx context.Context, args ...string) ([]byte, error) {
			return githubauth.RunForHost(ctx, binding.TokenEnv, "github.com", nil, args...)
		})
	}
	return NewGitHubAdapter(binding.Repository)
}
