package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
)

func runReady(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("ready", env)
	baseFlag := fs.String("base", "", "trusted mainline ref")
	headRef := "HEAD"
	if env.Getenv != nil && env.Getenv("PYXIS_CI_EVENT") == "pull_request" {
		headRef = "HEAD^2" // checkout's synthetic merge commit has the PR head as parent two
	}
	headFlag := fs.String("head", headRef, "draft head ref")
	allFlag := fs.Bool("all", false, "judge draft features changed by this pull request")
	positionals, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if *allFlag && len(positionals) != 0 || !*allFlag && (len(positionals) != 1 || positionals[0] == "" || strings.Contains(positionals[0], "/")) {
		return exitUsage, usage("ready requires one feature name")
	}
	m, err := open(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	draftRoot := strings.TrimSuffix(m.cfg.Drafts, "/")
	draftPrefix := draftRoot + "/"
	base := *baseFlag
	if base == "" {
		base = m.repo.DefaultBase(ctx, m.remote, m.branch)
	}
	if base == "" {
		return exitRefused, fmt.Errorf("no configured mainline ref is available")
	}
	baseCommit, err := m.repo.ResolveCommit(ctx, base)
	if err != nil {
		return exitRefused, fmt.Errorf("resolve mainline: %w", err)
	}
	headCommit, err := m.repo.ResolveCommit(ctx, *headFlag)
	if err != nil {
		return exitRefused, fmt.Errorf("resolve draft head: %w", err)
	}
	features := positionals
	if *allFlag {
		changes, err := m.repo.ChangedPaths(ctx, baseCommit, headCommit)
		if err != nil {
			return exitRefused, fmt.Errorf("discover changed drafts: %w", err)
		}
		entries, err := m.repo.ListTree(ctx, headCommit, draftPrefix)
		if err != nil {
			return exitRefused, fmt.Errorf("list draft head: %w", err)
		}
		present := map[string]bool{}
		for _, entry := range entries {
			if feature, _, ok := strings.Cut(strings.TrimPrefix(entry.Path, draftPrefix), "/"); ok && strings.HasPrefix(entry.Path, draftPrefix) {
				present[feature] = true
			}
		}
		found := map[string]bool{}
		for _, name := range changes {
			if feature, _, ok := strings.Cut(strings.TrimPrefix(name, draftPrefix), "/"); ok && strings.HasPrefix(name, draftPrefix) && feature != "" && present[feature] {
				found[feature] = true
			}
		}
		features = make([]string, 0, len(found))
		for feature := range found {
			features = append(features, feature)
		}
		sort.Strings(features)
		if len(features) == 0 {
			if _, err := fmt.Fprintln(env.Stdout, `{"schema_version":"draft-ready-pr/v1","message":"no draft"}`); err != nil {
				return exitRefused, fmt.Errorf("write readiness verdict: %w", err)
			}
			return exitOK, nil
		}
	}
	sources, err := readiness.LoadSources(ctx, m.repo, baseCommit, m.cfg)
	if err != nil {
		return exitRefused, err
	}
	status := exitOK
	for _, feature := range features {
		result, err := compile.CompileBranch(ctx, m.repo, baseCommit, headCommit, draftRoot, feature)
		var verdict readiness.Verdict
		if err != nil {
			verdict = readiness.New(feature, "", baseCommit)
			verdict.Unmet = append(verdict.Unmet, readiness.Unmet{
				Condition: "UB-7", Item: compilePart(draftRoot, feature, err),
				Remedy: "repair the unreadable or invalid draft part: " + err.Error(),
			})
			verdict.Sort()
		} else {
			verdict = readiness.Judge(result, sources)
		}
		if err := json.NewEncoder(env.Stdout).Encode(verdict); err != nil {
			return exitRefused, fmt.Errorf("write readiness verdict: %w", err)
		}
		if !verdict.Ready {
			status = exitRefused
		}
	}
	return status, nil
}

func compilePart(root, feature string, err error) string {
	s := err.Error()
	prefix := root + "/" + feature + "/"
	if i := strings.Index(s, prefix); i >= 0 {
		rest := s[i:]
		if j := strings.IndexAny(rest, " :\"\n"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	switch {
	case strings.Contains(s, "prd.md"), strings.Contains(s, "PRD"):
		return prefix + "prd.md"
	case strings.Contains(s, "tech-spec.md"), strings.Contains(s, "tech spec"):
		return prefix + "tech-spec.md"
	case strings.Contains(s, "reviews/"):
		i := strings.Index(s, "reviews/")
		rest := s[i:]
		if j := strings.IndexAny(rest, " :\"\n"); j >= 0 {
			rest = rest[:j]
		}
		return prefix + rest
	case strings.Contains(s, "problem"):
		return prefix + "problems.md"
	}
	return root + "/" + feature
}
