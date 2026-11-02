package readiness

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// LoadSources reads every readiness operand from one pinned mainline commit.
// The same loader serves local checks, hosted checks, and lock preparation.
func LoadSources(ctx context.Context, repo *gitsrc.Repo, base string, cfg config.Config) (Sources, error) {
	if repo == nil || base == "" {
		return Sources{}, fmt.Errorf("load pinned readiness sources: repository and mainline commit are required")
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: base, Prefix: store.Dir})
	if err != nil {
		return Sources{}, fmt.Errorf("load mainline memory: %w", err)
	}
	if len(set.Problems) > 0 || len(set.DocumentIssues) > 0 || len(set.Dangling) > 0 {
		return Sources{}, fmt.Errorf("mainline memory has unresolved store defects")
	}
	var paths []string
	seen := map[string]bool{}
	for _, root := range []string{cfg.PRDs, cfg.TechDesigns, cfg.Brainstorms} {
		entries, err := repo.ListTree(ctx, base, root)
		if err != nil {
			return Sources{}, fmt.Errorf("list mainline documents under %s: %w", root, err)
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Path, root+"/") || !strings.HasSuffix(entry.Path, ".md") && !strings.HasSuffix(entry.Path, "/effective.source.json") || seen[entry.Path] {
				continue
			}
			if !entry.Regular() {
				return Sources{}, fmt.Errorf("mainline document %s is not a regular file", entry.Path)
			}
			seen[entry.Path] = true
			paths = append(paths, entry.Path)
		}
	}
	paths = append(paths, schema.AuthorityContractPath)
	documents, err := repo.Files(ctx, base, paths)
	if err != nil {
		return Sources{}, fmt.Errorf("read mainline documents: %w", err)
	}
	return Sources{
		Mainline: resolve.New(set), Documents: documents,
		PRDsRoot: cfg.PRDs, DesignsRoot: cfg.TechDesigns, BrainstormsRoot: cfg.Brainstorms,
		OperatorLogin: cfg.Approval.OperatorLogin,
	}, nil
}
