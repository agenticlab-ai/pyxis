package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
)

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// RecordRange processes every commit introduced by one mainline push in order.
// A push may contain more than one merge, so inspecting only its final SHA can
// silently miss a lock.
func RecordRange(ctx context.Context, root string, cfg config.Config, repoSlug, before, after string, reader approval.Port, run lifecycle.RunCommand) ([]LockRecord, error) {
	if run == nil || !fullSHA.MatchString(before) || !fullSHA.MatchString(after) {
		return nil, fmt.Errorf("record pushed locks: full before and after commits and process adapter are required")
	}
	if before == strings.Repeat("0", 40) {
		return nil, nil // first mainline push has no prior merge to inspect
	}
	commits, err := command(ctx, run, root, nil, "git", "rev-list", "--first-parent", "--reverse", before+".."+after)
	if err != nil {
		return nil, fmt.Errorf("enumerate mainline push: %w", err)
	}
	var records []LockRecord
	for commit := range strings.SplitSeq(commits, "\n") {
		if commit == "" {
			continue
		}
		out, err := RecordPush(ctx, root, cfg, repoSlug, commit, reader, run)
		if err != nil {
			return nil, fmt.Errorf("record commit %s: %w", commit, err)
		}
		if out.Lock != nil {
			records = append(records, *out.Lock)
		}
	}
	return records, nil
}

type mergedPull struct {
	Number         int    `json:"number"`
	MergedAt       string `json:"merged_at"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	Head           struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// RecordPush is the mainline push entry point. It identifies the one merged
// draft PR that added contract records, reads the preparation digest from its
// approved head, counts the draft's proposals, and delegates to RecordMerge.
func RecordPush(ctx context.Context, root string, cfg config.Config, repoSlug, merge string, reader approval.Port, run lifecycle.RunCommand) (RecordOutcome, error) {
	if run == nil || !repoName.MatchString(repoSlug) || !fullSHA.MatchString(merge) {
		return RecordOutcome{}, fmt.Errorf("record pushed lock: repository, full merge commit, and process adapter are required")
	}
	changed, err := command(ctx, run, root, nil, "git", "diff-tree", "--no-commit-id", "--name-status", "-r", merge+"^", merge)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("inspect pushed merge: %w", err)
	}
	contractAdded := false
	for line := range strings.SplitSeq(changed, "\n") {
		path, ok := strings.CutPrefix(line, "A\t")
		if ok && (strings.HasPrefix(path, "memory/prd/") || strings.HasPrefix(path, "memory/design/")) && strings.Contains(path, "/Document-") && strings.HasSuffix(path, ".json") {
			contractAdded = true
			break
		}
	}
	if !contractAdded {
		return RecordOutcome{}, nil
	}
	pullsPath := "repos/" + repoSlug + "/commits/" + merge + "/pulls?per_page=100"
	data, err := command(ctx, run, root, nil, "gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", pullsPath)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("find pull request for lock merge: %w", err)
	}
	var pulls []mergedPull
	if err := json.Unmarshal([]byte(data), &pulls); err != nil {
		return RecordOutcome{}, fmt.Errorf("decode lock merge pull requests: %w", err)
	}
	var selected *mergedPull
	for i := range pulls {
		p := &pulls[i]
		if p.MergeCommitSHA != merge || p.MergedAt == "" || p.Base.Ref != cfg.Branch || p.Head.Repo.FullName != repoSlug || !strings.HasPrefix(p.Head.Ref, cfg.Draft.BranchPrefix) {
			continue
		}
		if selected != nil {
			return RecordOutcome{}, fmt.Errorf("lock merge %s has multiple draft pull requests", merge)
		}
		selected = p
	}
	if selected == nil {
		return RecordOutcome{}, nil // a native-record merge outside the draft branch is not a lock
	}
	if selected.Number < 1 || !fullSHA.MatchString(selected.Head.SHA) {
		return RecordOutcome{}, fmt.Errorf("merged draft pull request has no number or full head")
	}
	feature, version, err := featureVersion(ctx, root, cfg, selected.Head.Ref, run)
	if err != nil {
		return RecordOutcome{}, err
	}
	commitPath := "repos/" + repoSlug + "/commits/" + selected.Head.SHA
	data, err = command(ctx, run, root, nil, "gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", commitPath)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("read prepared draft commit: %w", err)
	}
	var commit struct {
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal([]byte(data), &commit); err != nil {
		return RecordOutcome{}, fmt.Errorf("decode prepared draft commit: %w", err)
	}
	digest, err := preparationDigest(feature, commit.Commit.Message)
	if err != nil {
		return RecordOutcome{}, err
	}
	owner, _, _ := strings.Cut(repoSlug, "/")
	count, err := proposalCount(ctx, root, repoSlug, owner, selected.Head.Ref, selected.Number, run)
	if err != nil {
		return RecordOutcome{}, err
	}
	request := RecordRequest{Feature: feature, Remote: cfg.Remote, ExpectedVersion: version, MergeCommit: merge, PullRequest: selected.Number, ApprovedHead: selected.Head.SHA, ReadinessDigest: digest, ProposalsSinceOpened: count}
	return RecordMerge(ctx, root, request, reader, run)
}

func featureVersion(ctx context.Context, root string, cfg config.Config, branch string, run lifecycle.RunCommand) (string, int, error) {
	feature, ok := strings.CutPrefix(branch, cfg.Draft.BranchPrefix)
	if !ok || !featureName.MatchString(feature) {
		return "", 0, fmt.Errorf("merged branch %q is not a draft", branch)
	}
	if i := strings.LastIndex(feature, "-v"); i > 0 && featureName.MatchString(feature[:i]) {
		before, raw := feature[:i], feature[i+2:]
		version, err := strconv.Atoi(raw)
		if err == nil && version > 1 {
			prior := fmt.Sprintf("refs/tags/lock/%s/v%d", before, version-1)
			found, err := command(ctx, run, root, nil, "git", "ls-remote", "--tags", cfg.Remote, prior)
			if err != nil {
				return "", 0, fmt.Errorf("inspect prior draft lock: %w", err)
			}
			if found != "" {
				return before, version, nil
			}
		}
	}
	return feature, 1, nil
}

func preparationDigest(feature, message string) (string, error) {
	lines := strings.Split(message, "\n")
	if len(lines) == 0 || lines[0] != "docs(draft): prepare lock "+feature {
		return "", fmt.Errorf("merged draft head is not a preparation commit for %s", feature)
	}
	var digest string
	for _, line := range lines[1:] {
		if value, ok := strings.CutPrefix(line, "Draft-Digest: "); ok {
			if digest != "" || !draftDigest.MatchString(value) {
				return "", fmt.Errorf("prepared draft has an invalid or duplicate digest")
			}
			digest = value
		}
	}
	if digest == "" {
		return "", fmt.Errorf("prepared draft has no digest")
	}
	return digest, nil
}

func proposalCount(ctx context.Context, root, repoSlug, owner, branch string, selected int, run lifecycle.RunCommand) (int, error) {
	count, found := 0, false
	for page := 1; page <= 100; page++ {
		endpoint := fmt.Sprintf("repos/%s/pulls?head=%s:%s&state=all&per_page=100&page=%d", repoSlug, owner, branch, page)
		data, err := command(ctx, run, root, nil, "gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", endpoint)
		if err != nil {
			return 0, fmt.Errorf("count draft proposals: %w", err)
		}
		var pulls []mergedPull
		if err := json.Unmarshal([]byte(data), &pulls); err != nil {
			return 0, fmt.Errorf("decode draft proposals: %w", err)
		}
		for _, p := range pulls {
			if p.Head.Ref == branch && p.Head.Repo.FullName == repoSlug {
				count++
				found = found || p.Number == selected
			}
		}
		if len(pulls) < 100 {
			if !found {
				return 0, fmt.Errorf("merged draft PR #%d is absent from its proposal history", selected)
			}
			return count, nil
		}
	}
	return 0, fmt.Errorf("draft proposal history exceeds 10000 entries")
}
