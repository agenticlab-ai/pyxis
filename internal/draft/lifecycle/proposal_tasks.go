package lifecycle

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// UnfinishedTasksWithTracker reads the exact merged plan and the selected
// tracker. A missing projection is unfinished, and duplicate markers refuse.
func UnfinishedTasksWithTracker(ctx context.Context, root string, cfg config.Config, feature, mainline string, tracker worktracker.Lister, run RunCommand) ([]string, error) {
	if !validFeature(feature) || mainline == "" || run == nil {
		return nil, fmt.Errorf("inspect plan tasks requires feature, mainline, and process adapter")
	}
	dir := path.Join(cfg.Plans, feature)
	listed, err := run(ctx, root, nil, "git", "ls-tree", "-r", "--name-only", mainline, "--", dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for file := range strings.SplitSeq(strings.TrimSpace(string(listed)), "\n") {
		if path.Dir(file) == dir && planFile.MatchString(path.Base(file)) {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	file := files[len(files)-1]
	body, err := run(ctx, root, nil, "git", "show", mainline+":"+file)
	if err != nil {
		return nil, err
	}
	doc, defects := plan.Parse(file, body)
	if doc == nil || len(defects) != 0 || doc.Feature != feature {
		return nil, fmt.Errorf("current plan is invalid or names another feature: %v", defects)
	}
	if tracker == nil {
		return nil, fmt.Errorf("reopen requires the configured tracker reader for the merged plan")
	}
	items, defects := tracker.List(ctx, model.Query{})
	if len(defects) != 0 {
		return nil, fmt.Errorf("read tracker: %v", defects)
	}
	matched := map[string][]model.WorkItem{}
	for _, item := range items {
		f, v, id, ok := planref.ParseMarker(item.Body)
		if ok && f == feature && v == doc.Version {
			matched[id] = append(matched[id], item)
		}
	}
	var unfinished []string
	for _, task := range doc.Tasks {
		items := matched[task.ID]
		if len(items) > 1 {
			return nil, fmt.Errorf("plan task %s has duplicate tracker items", task.ID)
		}
		if len(items) == 0 {
			unfinished = append(unfinished, task.ID+" (not projected)")
			continue
		}
		if items[0].State != model.StateClosed {
			unfinished = append(unfinished, fmt.Sprintf("%s (item #%d %s)", task.ID, items[0].ID, items[0].State))
		}
	}
	return unfinished, nil
}
