package lock

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func latestLockedVersion(ctx context.Context, root, remote, feature string, run lifecycle.RunCommand) (int, error) {
	tags, err := command(ctx, run, root, nil, "git", "ls-remote", "--tags", remote, "refs/tags/lock/"+feature+"/v*")
	if err != nil {
		return 0, fmt.Errorf("inspect latest lock: %w", err)
	}
	latest := 0
	for line := range strings.SplitSeq(tags, "\n") {
		if line == "" {
			continue
		}
		_, ref, ok := strings.Cut(line, "\t")
		if !ok {
			return 0, fmt.Errorf("malformed lock tag listing %q", line)
		}
		ref = strings.TrimSuffix(ref, "^{}")
		raw, ok := strings.CutPrefix(ref, "refs/tags/lock/"+feature+"/v")
		if !ok {
			return 0, fmt.Errorf("unexpected lock tag %q", ref)
		}
		version, err := strconv.Atoi(raw)
		if err != nil || version < 1 || strconv.Itoa(version) != raw {
			return 0, fmt.Errorf("invalid lock tag version %q", ref)
		}
		latest = max(latest, version)
	}
	return latest, nil
}

// guardReopened observes the first branch commit's bytes, not its name or an
// author's claim about its basis. The binding text is read from the verified
// lock tag, so a stale or hand-built seed cannot silently undo a version.
func guardReopened(ctx context.Context, root, remote string, repo *gitsrc.Repo, run lifecycle.RunCommand, base, head, draftsRoot, feature string, binding lifecycle.LockedContract, result compile.Result) ([]readiness.Unmet, error) {
	return guardReopenedWithReader(ctx, root, remote, repo, run, base, head, draftsRoot, feature, binding, result, LoadContract)
}

func guardReopenedWithReader(ctx context.Context, root, remote string, repo *gitsrc.Repo, run lifecycle.RunCommand, base, head, draftsRoot, feature string, binding lifecycle.LockedContract, result compile.Result, reader lifecycle.LockedReader) ([]readiness.Unmet, error) {
	commits, err := command(ctx, run, root, nil, "git", "rev-list", "--reverse", "--first-parent", base+".."+head)
	if err != nil {
		return nil, fmt.Errorf("read reopened draft history: %w", err)
	}
	first, _, _ := strings.Cut(commits, "\n")
	// This is a local Git object id. Hosted review records retain their
	// separate SHA-1 validation, while local origins also support SHA-256.
	if (len(first) != 40 && len(first) != 64) || strings.Trim(first, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("reopened draft has no first commit after mainline")
	}
	prefix := path.Join(draftsRoot, feature)
	prdPath, techPath := path.Join(prefix, "prd.md"), path.Join(prefix, "tech-spec.md")
	files, err := repo.Files(ctx, first, []string{prdPath, techPath})
	if err != nil {
		return nil, fmt.Errorf("read reopened seed at %s: %w", first, err)
	}
	seedPRD, hasPRD := files[prdPath]
	seedTech, hasTech := files[techPath]
	var unmet []readiness.Unmet
	if !hasPRD || !hasTech || !bytes.Equal(seedPRD, binding.PRD) || !bytes.Equal(seedTech, binding.TechSpec) {
		match := "no earlier locked version"
		if hasPRD && hasTech {
			for version := binding.Version - 1; version >= 1; version-- {
				older, err := reader(ctx, root, remote, feature, version, run)
				if err != nil {
					return nil, fmt.Errorf("compare seed with lock v%d: %w", version, err)
				}
				if bytes.Equal(seedPRD, older.PRD) && bytes.Equal(seedTech, older.TechSpec) {
					match = fmt.Sprintf("lock v%d", version)
					break
				}
			}
		}
		unmet = append(unmet, readiness.Unmet{
			Condition: "FR-20", Item: prefix,
			Remedy: fmt.Sprintf("first commit's PRD and tech spec match %s; reopen from binding lock v%d and carry the edits over", match, binding.Version),
		})
	}
	if result.Digest == binding.Digest {
		unmet = append(unmet, readiness.Unmet{
			Condition: "FR-22", Item: prefix,
			Remedy: "PRD and tech spec are unchanged from the binding lock; abandon this draft instead of locking another version",
		})
	}
	return unmet, nil
}
