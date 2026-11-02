package precheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// judgeVersion rechecks the literal contract and review Documents at a binding
// lock against current mainline sources. It never reads a reopened draft.
func judgeVersion(ctx context.Context, root string, cfg config.Config, feature string, version int, mainline string, run lifecycle.RunCommand) (readiness.Verdict, error) {
	seed, err := lock.LoadContract(ctx, root, cfg.Remote, feature, version, run)
	if err != nil {
		return readiness.Verdict{}, err
	}
	lockRecord, err := lock.ReadLock(ctx, root, feature, version, run)
	if err != nil {
		return readiness.Verdict{}, err
	}
	return judgeContract(ctx, root, cfg, feature, version, mainline, run, seed, lockRecord.MergeCommit)
}

func judgeContract(ctx context.Context, root string, cfg config.Config, feature string, version int, mainline string, run lifecycle.RunCommand, seed lifecycle.LockedContract, mergeCommit string) (readiness.Verdict, error) {
	return judgeContractAt(ctx, root, cfg, feature, version, mainline, run, seed, mergeCommit, fmt.Sprintf("refs/tags/lock/%s/v%d", feature, version))
}

func judgeContractAt(ctx context.Context, root string, cfg config.Config, feature string, version int, mainline string, run lifecycle.RunCommand, seed lifecycle.LockedContract, mergeCommit, ref string) (readiness.Verdict, error) {
	if _, err := run(ctx, root, nil, "git", "merge-base", "--is-ancestor", mergeCommit, mainline); err != nil {
		return readiness.Verdict{}, fmt.Errorf("binding lock/%s/v%d is not merged into current mainline: %w", feature, version, err)
	}
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return readiness.Verdict{}, fmt.Errorf("open locked repository: %w", err)
	}
	commit, err := repo.ResolveCommit(ctx, ref)
	if err != nil {
		return readiness.Verdict{}, fmt.Errorf("load locked memory: resolve %s: %w", ref, err)
	}
	if commit != mergeCommit {
		return readiness.Verdict{}, fmt.Errorf("binding lock %s moved during verification", ref)
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: mergeCommit, Prefix: store.Dir})
	if err != nil {
		return readiness.Verdict{}, fmt.Errorf("load locked memory: %w", err)
	}
	if len(set.Problems) != 0 || len(set.DocumentIssues) != 0 || len(set.Dangling) != 0 {
		return readiness.Verdict{}, fmt.Errorf("binding lock %s has invalid memory", ref)
	}
	files := map[string][]byte{"prd.md": seed.PRD, "tech-spec.md": seed.TechSpec}
	projection := resolve.New(set)
	for _, subject := range []string{"prd/" + feature, "feature/" + feature, "brainstorm/" + feature, "review/" + feature} {
		for _, entry := range set.BySubject[subject] {
			if entry.Kind != record.Document || projection.Head(subject, entry.Family) != entry {
				continue
			}
			name := lockedPart(subject, feature, entry.Family)
			if name == "" || name == "prd.md" {
				continue
			}
			body := set.Bodies[entry.ID]
			if len(body) == 0 {
				return readiness.Verdict{}, fmt.Errorf("binding lock %s has empty %s Document", ref, name)
			}
			files[name] = body
		}
	}
	result, err := compile.Compile(feature, mainline, files)
	if err != nil {
		return readiness.Verdict{}, fmt.Errorf("compile locked contract %s: %w", ref, err)
	}
	if result.Digest != seed.Digest {
		return readiness.Verdict{}, fmt.Errorf("binding lock %s recompiles to a different digest", ref)
	}
	sources, err := readiness.LoadSources(ctx, repo, mainline, cfg)
	if err != nil {
		return readiness.Verdict{}, fmt.Errorf("load current mainline readiness sources: %w", err)
	}
	sources.LockedVersion = true
	return readiness.Judge(result, sources), nil
}

func lockedPart(subject, feature, family string) string {
	switch subject {
	case "prd/" + feature:
		switch family {
		case "prd":
			return "prd.md"
		case "problems":
			return "problems.md"
		}
	case "feature/" + feature:
		if family == "events" {
			return "events.md"
		}
	case "brainstorm/" + feature:
		if number, ok := strings.CutPrefix(family, "record-"); ok {
			return "brainstorm/" + number + ".md"
		}
	case "review/" + feature:
		if number, ok := strings.CutPrefix(family, "report-"); ok {
			return "reviews/" + number + ".md"
		}
	}
	return ""
}
