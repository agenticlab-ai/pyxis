package lock

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// LoadContract reads the two literal Documents at a verified lock tag. It
// checks their recomputed digest against the approval record before returning
// bytes that may seed a new draft.
func LoadContract(ctx context.Context, root, remote, feature string, version int, run lifecycle.RunCommand) (lifecycle.LockedContract, error) {
	if remote == "" || run == nil {
		return lifecycle.LockedContract{}, fmt.Errorf("load locked contract: remote and process adapter are required")
	}
	ref := fmt.Sprintf("refs/tags/lock/%s/v%d", feature, version)
	if _, err := run(ctx, root, nil, "git", "fetch", remote, ref+":"+ref); err != nil {
		return lifecycle.LockedContract{}, fmt.Errorf("fetch binding lock %s: %w", ref, err)
	}
	lock, err := ReadLock(ctx, root, feature, version, run)
	if err != nil {
		return lifecycle.LockedContract{}, err
	}
	return contractAt(ctx, root, feature, version, ref, lock.ReadinessDigest)
}

func contractAt(ctx context.Context, root, feature string, version int, ref, expectedDigest string) (lifecycle.LockedContract, error) {
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return lifecycle.LockedContract{}, fmt.Errorf("open locked repository: %w", err)
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: ref, Prefix: store.Dir})
	if err != nil {
		return lifecycle.LockedContract{}, fmt.Errorf("load locked memory: %w", err)
	}
	if len(set.Problems) != 0 {
		return lifecycle.LockedContract{}, fmt.Errorf("binding lock %s has invalid memory: %v", ref, set.Problems)
	}
	projection := resolve.New(set)
	prd := projection.Head("prd/"+feature, "prd")
	tech := projection.Head("design/"+feature, "design")
	if prd == nil || tech == nil {
		return lifecycle.LockedContract{}, fmt.Errorf("binding lock %s lacks PRD or tech-spec Document", ref)
	}
	files := map[string][]byte{"prd.md": set.Bodies[prd.ID], "tech-spec.md": set.Bodies[tech.ID]}
	if len(files["prd.md"]) == 0 || len(files["tech-spec.md"]) == 0 {
		return lifecycle.LockedContract{}, fmt.Errorf("binding lock %s has empty stored contract text", ref)
	}
	digest := compile.ContractDigest(files["prd.md"], files["tech-spec.md"])
	if digest != expectedDigest {
		return lifecycle.LockedContract{}, fmt.Errorf("binding lock %s text digest differs from its approval record", ref)
	}
	return lifecycle.LockedContract{Version: version, PRD: append([]byte(nil), files["prd.md"]...), TechSpec: append([]byte(nil), files["tech-spec.md"]...), Digest: digest, SourceRef: ref}, nil
}
