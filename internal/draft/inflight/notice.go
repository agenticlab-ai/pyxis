// Package inflight reports advisory overlap among open feature drafts.
package inflight

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// OpenPull is an open, same-repository feature-draft pull request observed by
// the code host. Head must be fetched and verified before Build reads it.
type OpenPull struct {
	Number  int
	Branch  string
	Head    string
	Feature string
}

type Match struct {
	Pull   OpenPull
	Shared []string // typed requirement or decision addresses
}

type Report struct {
	Feature  string
	Head     string
	Matches  []Match
	Warnings []string
}

// Build compiles only the open pull requests supplied by the code-host
// observer. Each draft is read at one pinned Git head. A malformed or
// unavailable draft yields a warning; this report never feeds readiness.
func Build(ctx context.Context, repo *gitsrc.Repo, mainline, draftsRoot string, current OpenPull, open []OpenPull) Report {
	out := Report{Feature: current.Feature, Head: current.Head}
	if repo == nil || current.Number < 1 || current.Feature == "" || current.Head == "" {
		out.Warnings = append(out.Warnings, "current open draft is unavailable")
		return out
	}
	found := false
	for _, pull := range open {
		if pull.Number == current.Number && pull.Branch == current.Branch {
			found = true
			break
		}
	}
	if !found {
		out.Warnings = append(out.Warnings, "current branch has no open feature-draft pull request")
		return out
	}
	ours, err := compile.CompileBranch(ctx, repo, mainline, current.Head, draftsRoot, current.Feature)
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("current draft: %v", err))
		return out
	}
	oursTouched := touched(ours)
	seen := map[int]bool{current.Number: true}
	for _, pull := range open {
		if pull.Number < 1 || pull.Feature == "" || pull.Head == "" || seen[pull.Number] {
			continue
		}
		seen[pull.Number] = true
		other, err := compile.CompileBranch(ctx, repo, mainline, pull.Head, draftsRoot, pull.Feature)
		if err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("PR #%d (%s): %v", pull.Number, pull.Branch, err))
			continue
		}
		shared := intersection(oursTouched, touched(other))
		if len(shared) > 0 {
			out.Matches = append(out.Matches, Match{Pull: pull, Shared: shared})
		}
	}
	sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].Pull.Number < out.Matches[j].Pull.Number })
	sort.Strings(out.Warnings)
	return out
}

func touched(result compile.Result) map[string]bool {
	out := map[string]bool{}
	for _, entry := range result.Records {
		draft := entry.Draft
		if (record.IsClaim(draft.Kind) || draft.Kind == record.Decision) && draft.Family != "" {
			out[draft.Subject+"#"+draft.Family] = true
		}
		for _, link := range entry.Links {
			if (link.Type == record.Serves || link.Type == record.DependsOn || link.Type == record.Supersedes) &&
				(strings.HasPrefix(link.Target, "prd/") || strings.HasPrefix(link.Target, "design/")) {
				out[link.Target] = true
			}
		}
	}
	return out
}

func intersection(a, b map[string]bool) []string {
	shared := make([]string, 0)
	for address := range a {
		if b[address] {
			shared = append(shared, address)
		}
	}
	sort.Strings(shared)
	return shared
}
