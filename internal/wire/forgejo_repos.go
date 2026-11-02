package wire

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
)

type forgejoRepository struct {
	ID          int64  `json:"id"`
	FullName    string `json:"full_name"`
	Empty       bool   `json:"empty"`
	Archived    bool   `json:"archived"`
	Permissions struct {
		Admin bool `json:"admin"`
		Push  bool `json:"push"`
	} `json:"permissions"`
}

// ListEmptyForgejoRepositories is the native repository-discovery counterpart
// to the GitHub onboarding list. All pages are read before returning a list.
func ListEmptyForgejoRepositories(ctx context.Context, client *forgejo.Client) (RepoListing, error) {
	if client == nil {
		return RepoListing{}, fmt.Errorf("wire: Forgejo repository listing requires a client")
	}
	var repos []forgejoRepository
	if err := client.List(ctx, "user/repos", &repos); err != nil {
		return RepoListing{}, fmt.Errorf("list Forgejo repositories: %w", err)
	}
	var out RepoListing
	for _, repo := range repos {
		if !repo.Empty || repo.Archived || (!repo.Permissions.Admin && !repo.Permissions.Push) || !ValidCoordinate(repo.FullName) {
			continue
		}
		permission := "WRITE"
		if repo.Permissions.Admin {
			permission = "ADMIN"
		}
		out.Candidates = append(out.Candidates, RepoCandidate{Slug: repo.FullName, Permission: permission})
	}
	return out, nil
}

func ProbeForgejoRepository(ctx context.Context, client *forgejo.Client, repo string) (RepoState, error) {
	if client == nil || !ValidCoordinate(repo) {
		return RepoUnknown, fmt.Errorf("wire: Forgejo repository probe needs a client and owner/name")
	}
	var record forgejoRepository
	if err := client.Request(ctx, "GET", forgejo.RepoPath(repo, ""), nil, &record); err != nil {
		return RepoUnknown, err
	}
	if record.ID <= 0 {
		return RepoUnknown, fmt.Errorf("wire: Forgejo repository response has no identity")
	}
	if record.Empty {
		return RepoEmpty, nil
	}
	return RepoPopulated, nil
}
