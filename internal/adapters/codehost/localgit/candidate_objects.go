package localgit

import (
	"context"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func verifyCandidateObjects(ctx context.Context, root string, c codehost.FinalCandidate) error {
	if err := commitObject(ctx, root, c.Commit); err != nil {
		return err
	}
	parents, err := proposalGit(ctx, root, nil, nil, "rev-list", "--parents", "-n", "1", c.Commit)
	if err != nil || !slices.Equal(strings.Fields(string(parents)), []string{c.Commit, c.MainHead, c.FeatureHead}) {
		return invalidProposal("candidate must retain the exact main and feature parents")
	}
	tree, err := proposalGit(ctx, root, nil, nil, "rev-parse", c.Commit+"^{tree}")
	if err != nil || strings.TrimSpace(string(tree)) != c.Tree {
		return invalidProposal("candidate tree differs from its complete identity")
	}
	return nil
}
