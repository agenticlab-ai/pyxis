package preflight

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// IntegrationChecks probes only the repositories used by declared operations.
// Native Forgejo calls never invoke gh. Mixed task/code hosts are separate rows.
func IntegrationChecks(ctx context.Context, in Input) (Report, error) {
	ops, err := wire.TableForIntegrations(wire.Config{Repo: config.Config{Knobs: in.Knobs}, Slug: in.Coordinate}, in.Integrations)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var report Report
	for _, op := range ops {
		identity := op.Connection.Key() + "\n" + op.Connection.TokenEnv
		if op.Transport == wire.TransportConverge || seen[identity] {
			continue
		}
		seen[identity] = true
		out := Outcome{Check: "integration:" + op.Connection.Provider + ":" + op.Slug, Status: StatusPass}
		if op.Connection.Provider == integrations.GitHub {
			runner := runnerFor(in)
			if _, ok := runner.(wire.GHRunner); ok {
				runner = wire.GHRunner{TokenEnv: op.Connection.TokenEnv, Host: "github.com"}
			}
			if result, ok := probeGHPresence(ctx, runner); !ok {
				out = result
			} else if result, ok := probeGHAuth(ctx, runner); !ok {
				out = result
			} else if op.Name != wire.OpCreateRepo {
				if result, ok := probeRepoExists(ctx, runner, op.Slug); !ok {
					out = result
				}
			}
		} else {
			var client *forgejo.Client
			if in.ForgejoClientFactory != nil {
				client, err = in.ForgejoClientFactory(op.Connection)
			} else if op.Connection.Token() == "" {
				err = fmt.Errorf("set %s for %s", op.Connection.TokenEnv, op.Connection.BaseURL)
			} else {
				client, err = forgejo.New(op.Connection.BaseURL, op.Connection.Token())
			}
			if err == nil {
				_, err = client.CurrentUser(ctx)
			}
			if err == nil && op.Name != wire.OpCreateRepo {
				var repo struct {
					ID int64 `json:"id"`
				}
				err = client.Request(ctx, "GET", forgejo.RepoPath(op.Slug, ""), nil, &repo)
				if err == nil && repo.ID <= 0 {
					err = fmt.Errorf("repository response is incomplete")
				}
			}
			if err != nil {
				out.Status = StatusRefuse
				out.Reason = fmt.Sprintf("Forgejo connection %s repository %s: %v", op.Connection.BaseURL, op.Slug, err)
				out.Remedy = "check the configured base_url, repository, and token_env access before retrying"
			}
		}
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		out.Check = "integration:" + op.Connection.Provider + ":" + op.Slug
		report = append(report, out)
	}
	return report, nil
}
