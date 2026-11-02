package main

import (
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

var draftVersionSuffix = regexp.MustCompile(`^-v(?:[2-9]|[1-9][0-9]+)$`)

type digestRepository interface {
	CurrentBranch(context.Context) (string, error)
	DirtyPaths(context.Context, []string) (bool, error)
	Head(context.Context) (string, error)
	ListTree(context.Context, string, string) ([]gitsrc.TreeEntry, error)
	Files(context.Context, string, []string) (map[string][]byte, error)
}

var openDigestRepository = func(ctx context.Context, root string) (digestRepository, error) {
	return gitsrc.Open(ctx, root)
}

func draftDigestVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	head, digest, err := draftDigest(context.Background(), root, cfg, feature)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft digest: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s %s\n", head, digest)
	return 0
}

// draftDigest binds the exact two contract blobs at one committed draft head.
// A reviewer reads that head, writes the digest into its working-file report,
// and can detect any later PRD or tech-spec edit without interpreting prose.
func draftDigest(ctx context.Context, root string, cfg config.Config, feature string) (string, string, error) {
	if feature == "" || strings.Contains(feature, "/") || !record.ValidSubject("prd/"+feature) {
		return "", "", fmt.Errorf("feature %q must be a lowercase feature slug", feature)
	}
	repo, err := openDigestRepository(ctx, root)
	if err != nil {
		return "", "", fmt.Errorf("open draft repository: %w", err)
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return "", "", fmt.Errorf("read draft branch: %w", err)
	}
	suffix := strings.TrimPrefix(branch, cfg.Draft.BranchPrefix+feature)
	if suffix == branch || suffix != "" && !draftVersionSuffix.MatchString(suffix) {
		return "", "", fmt.Errorf("current branch %q is not the draft for %s", branch, feature)
	}
	paths := []string{path.Join(cfg.Drafts, feature, "prd.md"), path.Join(cfg.Drafts, feature, "tech-spec.md")}
	dirty, err := repo.DirtyPaths(ctx, paths)
	if err != nil {
		return "", "", fmt.Errorf("inspect draft files: %w", err)
	}
	if dirty {
		return "", "", fmt.Errorf("draft PRD or tech spec has uncommitted edits; commit them before review")
	}
	head, err := repo.Head(ctx)
	if err != nil {
		return "", "", fmt.Errorf("resolve draft head: %w", err)
	}
	entries, err := repo.ListTree(ctx, head, path.Join(cfg.Drafts, feature))
	if err != nil {
		return "", "", fmt.Errorf("inspect committed draft: %w", err)
	}
	regular := map[string]bool{}
	for _, entry := range entries {
		if entry.Regular() {
			regular[entry.Path] = true
		}
	}
	for _, name := range paths {
		if !regular[name] {
			return "", "", fmt.Errorf("committed draft contract %s is missing or not a regular file", name)
		}
	}
	files, err := repo.Files(ctx, head, paths)
	if err != nil {
		return "", "", fmt.Errorf("read committed draft contract: %w", err)
	}
	return head, compile.ContractDigest(files[paths[0]], files[paths[1]]), nil
}
