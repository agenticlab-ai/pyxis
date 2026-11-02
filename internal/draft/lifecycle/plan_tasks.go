package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
)

var planFile = regexp.MustCompile(`^[0-9]{3}-plan\.md$`)
var githubRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// UnfinishedTasks inspects the newest merged plan source and exact tracker
// issue list. A task absent from the tracker is unfinished even when the plan
// merge has not yet been projected.
func UnfinishedTasks(ctx context.Context, root string, cfg config.Config, feature string, run RunCommand) ([]string, error) {
	if !validFeature(feature) || run == nil {
		return nil, fmt.Errorf("inspect plan tasks: feature and process adapter are required")
	}
	planDir := path.Join(cfg.Plans, feature)
	out, err := run(ctx, root, nil, "git", "ls-tree", "-r", "--name-only", cfg.MainlineRef(), "--", planDir)
	if err != nil {
		return nil, fmt.Errorf("list merged plans for %s: %w", feature, err)
	}
	var files []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if path.Dir(line) == planDir && planFile.MatchString(path.Base(line)) {
			files = append(files, line)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	selected := files[len(files)-1]
	body, err := run(ctx, root, nil, "git", "show", cfg.MainlineRef()+":"+selected)
	if err != nil {
		return nil, fmt.Errorf("read current plan %s: %w", selected, err)
	}
	doc, defects := plan.Parse(selected, body)
	if doc == nil || len(defects) != 0 || doc.Feature != feature {
		return nil, fmt.Errorf("current plan %s is invalid or names another feature: %v", selected, defects)
	}
	repo, err := run(ctx, root, nil, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return nil, fmt.Errorf("resolve tracker repository: %w", err)
	}
	slug := strings.TrimSpace(string(repo))
	if !githubRepo.MatchString(slug) {
		return nil, fmt.Errorf("tracker repository %q is invalid", slug)
	}
	type taskIssue struct {
		Number      int       `json:"number"`
		Body        string    `json:"body"`
		State       string    `json:"state"`
		PullRequest *struct{} `json:"pull_request"`
	}
	issues := map[string][]taskIssue{}
	for page := 1; page <= 100; page++ {
		endpoint := fmt.Sprintf("repos/%s/issues?state=all&per_page=100&page=%d", slug, page)
		data, err := run(ctx, root, nil, "gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", endpoint)
		if err != nil {
			return nil, fmt.Errorf("list tracker issues page %d: %w", page, err)
		}
		var batch []taskIssue
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("decode tracker issues page %d: %w", page, err)
		}
		for _, issue := range batch {
			if issue.PullRequest != nil {
				continue
			}
			f, version, taskID, ok := planref.ParseMarker(issue.Body)
			if ok && f == doc.Feature && version == doc.Version {
				issues[taskID] = append(issues[taskID], issue)
			}
		}
		if len(batch) < 100 {
			break
		}
		if page == 100 {
			return nil, fmt.Errorf("tracker history exceeds 10000 issues")
		}
	}
	var unfinished []string
	for _, task := range doc.Tasks {
		matched := issues[task.ID]
		if len(matched) > 1 {
			return nil, fmt.Errorf("plan task %s has duplicate tracker issues", task.PlanTaskID)
		}
		if len(matched) == 0 {
			unfinished = append(unfinished, task.ID+" (not projected)")
			continue
		}
		if !strings.EqualFold(matched[0].State, "closed") {
			unfinished = append(unfinished, fmt.Sprintf("%s (issue #%d %s)", task.ID, matched[0].Number, strings.ToLower(matched[0].State)))
		}
	}
	return unfinished, nil
}
