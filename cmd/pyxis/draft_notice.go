package main

import (
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/inflight"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

var draftNotice = noticeForDraft
var draftHeadSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var draftVersion = regexp.MustCompile(`-v(?:[2-9]|[1-9][0-9]+)$`)

func draftNoticeVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	report := draftNotice(context.Background(), root, cfg, feature, draftCommand)
	for _, match := range report.Matches {
		terminalPrintf(stdout, "PR #%d (%s) also touches %s\n", match.Pull.Number, match.Pull.Branch, strings.Join(match.Shared, ", "))
	}
	if len(report.Matches) == 0 {
		terminalPrintln(stdout, "no other open drafts touch the same requirements or decisions")
	}
	for _, warning := range report.Warnings {
		terminalPrintf(stdout, "notice unavailable for %s\n", warning)
	}
	return 0 // advisory: never a readiness or lock gate
}

// noticeForDraft observes the open PR set, then fetches only those draft
// heads. It verifies each fetched SHA against the code-host observation
// before compiling; a race or unavailable branch becomes a warning.
func noticeForDraft(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand) inflight.Report {
	report := inflight.Report{Feature: feature}
	if run == nil {
		report.Warnings = append(report.Warnings, "draft process adapter is unavailable")
		return report
	}
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("repository: %v", err))
		return report
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("current branch: %v", err))
		return report
	}
	if !strings.HasPrefix(branch, cfg.Draft.BranchPrefix) {
		report.Warnings = append(report.Warnings, "current branch is not a feature draft")
		return report
	}
	head, err := repo.Head(ctx)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("current head: %v", err))
		return report
	}
	report.Head = head
	if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, cfg.Branch); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("current mainline: %v", err))
		return report
	}
	mainline, err := repo.ResolveCommit(ctx, cfg.MainlineRef())
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("current mainline: %v", err))
		return report
	}
	const query = `.[] | select(.head.repo.full_name == .base.repo.full_name) | [.number, .head.ref, .head.sha] | @tsv`
	output, err := run(ctx, root, nil, "gh", "api", "--paginate", "repos/{owner}/{repo}/pulls?state=open&per_page=100", "--jq", query)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("open pull requests: %v", err))
		return report
	}
	var open []inflight.OpenPull
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			report.Warnings = append(report.Warnings, "malformed open pull request observation")
			continue
		}
		number, numberErr := strconv.Atoi(fields[0])
		if numberErr != nil || number < 1 || !draftHeadSHA.MatchString(fields[2]) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("invalid open pull request %q", fields[0]))
			continue
		}
		name := fields[1]
		if !strings.HasPrefix(name, cfg.Draft.BranchPrefix) {
			continue
		}
		observed := inflight.OpenPull{Number: number, Branch: name, Head: fields[2]}
		if name == branch {
			observed.Head = head // local unpushed round is the reader's exact subject
			observed.Feature = feature
			open = append(open, observed)
			continue
		}
		if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, "refs/heads/"+name); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("PR #%d fetch: %v", number, err))
			continue
		}
		fetched, err := repo.ResolveCommit(ctx, "FETCH_HEAD")
		if err != nil || fetched != fields[2] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("PR #%d moved since the open-PR observation", number))
			continue
		}
		observed.Feature, err = draftFeatureAtHead(ctx, repo, cfg.Drafts, cfg.Draft.BranchPrefix, observed)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("PR #%d: %v", number, err))
			continue
		}
		open = append(open, observed)
	}
	computed := inflight.Build(ctx, repo, mainline, cfg.Drafts, inflight.OpenPull{Number: currentNumber(open, branch), Branch: branch, Head: head, Feature: feature}, open)
	report.Matches = computed.Matches
	report.Warnings = append(report.Warnings, computed.Warnings...)
	sort.Strings(report.Warnings)
	return report
}

func currentNumber(open []inflight.OpenPull, branch string) int {
	for _, pull := range open {
		if pull.Branch == branch {
			return pull.Number
		}
	}
	return 0
}

func draftFeatureAtHead(ctx context.Context, repo *gitsrc.Repo, root, prefix string, pull inflight.OpenPull) (string, error) {
	branchFeature := strings.TrimPrefix(pull.Branch, prefix)
	candidates := []string{branchFeature}
	if loc := draftVersion.FindStringIndex(branchFeature); loc != nil {
		candidates = append(candidates, branchFeature[:loc[0]])
	}
	var found []string
	for _, candidate := range candidates {
		if candidate == "" || strings.Contains(candidate, "/") {
			continue
		}
		entries, err := repo.ListTree(ctx, pull.Head, path.Join(root, candidate))
		if err != nil {
			return "", err
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if entry.Regular() {
				seen[entry.Path] = true
			}
		}
		if seen[path.Join(root, candidate, "prd.md")] && seen[path.Join(root, candidate, "tech-spec.md")] {
			found = append(found, candidate)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("expected one draft contract at the pinned head; found %d", len(found))
	}
	return found[0], nil
}
