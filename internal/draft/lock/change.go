package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

// OtherChanges is what a lock proposal carries beside the compiled contract:
// the feature's plan and its problem ledgers ride the same approval, so one
// pull request settles a contract change and everything it touches.
type OtherChanges struct {
	Paths []string // "<status> <path>", sorted
	Plan  string   // the live plan's source path when the proposal changes it
	Tasks []string // plan tasks whose charter changed, was added, or was removed
	// Version names a plan version the proposal adds beside the live plan, or
	// a plan it removes: a plan changed with its contract keeps its version.
	Version string
}

// planFace matches either face of a numbered plan: its source or its fold.
var planFace = regexp.MustCompile(`^([0-9]{3})-plan\.(md|json)$`)

// AppendOnlyTasks is a task tracker whose task comments cannot be rewritten,
// such as Carina's append-only item log. A plan changed in place cannot reach
// its tasks there: every move onto a new contract changes their specs.
type AppendOnlyTasks interface {
	worktracker.Lister
	AppendOnly() bool
}

// errAppendOnlyTasks refuses moving a live plan whose tasks cannot take an
// in-place change.
var errAppendOnlyTasks = errors.New("the live plan is in use and its tasks are on a tracker whose task comments cannot change, so the plan cannot move onto the new contract in place; finish or close its tasks first, then plan the new version from the new lock")

// errFinishedPlan refuses editing a plan whose every task is closed: its
// charters are history, and its closed epic would not be found again.
var errFinishedPlan = errors.New("the draft edits the live plan, but every one of its tasks is closed; a finished plan stays history — plan the new version from the new lock in its own pull request")

// planUnfitError is a live plan that no longer fits the prepared contract.
// Preparation refuses it; the author changes the plan in the same draft.
type planUnfitError struct {
	defects []schema.Defect
}

func (e *planUnfitError) Error() string {
	reasons := make([]string, 0, len(e.defects))
	for _, d := range e.defects {
		reasons = append(reasons, d.File+": "+d.Reason)
	}
	return "the feature's live plan does not fit the prepared contract; change the plan on the draft branch: " + strings.Join(reasons, "; ")
}

// rebasePreparedPlan moves the live plan onto the lock being prepared and
// folds it against the prepared records, when the plan is still in use: the
// draft edits it, or it has unfinished tasks. A finished plan stays history:
// left alone it keeps its Basis, and a draft that edits it is refused, since
// its tasks and epic are closed; the new version is planned afresh. With no
// task reader (unfinished is nil) the plan's task state is unknown, so it
// counts as in use: a plan whose tasks may still run never keeps an old
// Basis. A changed plan joins the preparation commit, so the approved head
// holds contract and plan together. A plan in use whose tasks are
// append-only is refused.
func rebasePreparedPlan(ctx context.Context, dir string, cfg config.Config, feature string, version int, base string, run lifecycle.RunCommand, env []string, unfinished func(context.Context) ([]string, error), appendOnly bool) error {
	diffed, err := command(ctx, run, dir, nil, "git", "diff", "--name-only", base, "HEAD", "--", path.Join(filepath.ToSlash(cfg.Plans), feature))
	if err != nil {
		return fmt.Errorf("inspect the draft's plan changes: %w", err)
	}
	// Only the plan's own faces count as an edit: a plan directory may hold
	// other evidence, such as a proof run's notes.
	edited := ""
	for file := range strings.SplitSeq(diffed, "\n") {
		if planFace.MatchString(path.Base(file)) {
			edited = file
		}
	}
	if unfinished != nil {
		open, err := unfinished(ctx)
		if err != nil {
			return fmt.Errorf("inspect the live plan's unfinished tasks: %w", err)
		}
		if len(open) == 0 && edited == "" {
			return nil
		}
		if len(open) == 0 {
			return errFinishedPlan
		}
	}
	if appendOnly {
		return errAppendOnlyTasks
	}
	head, err := command(ctx, run, dir, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve preparation commit: %w", err)
	}
	pending := plan.PendingBasis{Basis: fmt.Sprintf("lock/%s/v%d", feature, version), Commit: head}
	changed, defects, err := rebaseLivePlan(dir, cfg, feature, pending)
	if err != nil {
		return fmt.Errorf("rebase live plan onto %s: %w", pending.Basis, err)
	}
	if len(defects) > 0 {
		return &planUnfitError{defects: defects}
	}
	if len(changed) == 0 {
		return nil
	}
	if _, err := run(ctx, dir, nil, "git", append([]string{"add", "--"}, changed...)...); err != nil {
		return fmt.Errorf("stage rebased plan: %w", err)
	}
	if _, err := run(ctx, dir, env, "git", "commit", "--amend", "--no-edit"); err != nil {
		return fmt.Errorf("add rebased plan to the preparation commit: %w", err)
	}
	return nil
}

// otherChanges lists the proposal's paths outside the feature's compiled
// contract records, and names the plan tasks whose charter it changes.
func otherChanges(ctx context.Context, dir, base, head, feature, plans string, run lifecycle.RunCommand) (OtherChanges, error) {
	planDir := path.Join(filepath.ToSlash(plans), feature)
	var out OtherChanges
	names, err := command(ctx, run, dir, nil, "git", "diff", "--no-renames", "--name-status", base, head)
	if err != nil {
		return out, fmt.Errorf("list proposal changes: %w", err)
	}
	var live []string // plan version numbers on the base, oldest first; read once
	contract := []string{"memory/prd/" + feature + "/", "memory/design/" + feature + "/", "memory/review/" + feature + "/", "memory/feature/" + feature + "/"}
	for line := range strings.SplitSeq(names, "\n") {
		status, file, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		owned := false
		for _, prefix := range contract {
			owned = owned || strings.HasPrefix(file, prefix)
		}
		if owned {
			continue
		}
		out.Paths = append(out.Paths, status+" "+file)
		face := planFace.FindStringSubmatch(path.Base(file))
		if face == nil || path.Dir(file) != planDir {
			continue
		}
		// Either face of the live plan pair counts: removing the source alone
		// would drop every task as surely as removing the folded face. Only
		// the live plan, the newest version on the base, changes in place; a
		// first plan is planned from the merged lock in its own pull request.
		source := path.Join(planDir, face[1]+"-plan.md")
		if live == nil {
			if live, err = basePlans(ctx, dir, base, planDir, run); err != nil {
				return out, err
			}
		}
		newest := ""
		if len(live) > 0 {
			newest = live[len(live)-1]
		}
		switch {
		case status == "D":
			out.Version = "removes " + source
			continue
		case status == "T":
			// A face turned into a symlink or a gitlink has no plan in it.
			out.Version = "changes the file type of " + file
			continue
		case status == "A" && newest == "":
			out.Version = "adds the first plan " + source
		case status == "A" && newest != face[1]:
			out.Version = fmt.Sprintf("adds %s beside the live plan %s", source, path.Join(planDir, newest+"-plan.md"))
		case status != "A" && newest != face[1]:
			out.Version = fmt.Sprintf("changes the earlier plan %s; only the live plan %s changes in place", source, path.Join(planDir, newest+"-plan.md"))
		}
		if face[2] == "json" {
			out.Plan = source
			// Only an added face may be absent at the base, where every task
			// is new; a modified face that cannot be read is a failure.
			before, err := run(ctx, dir, nil, "git", "show", base+":"+file)
			if err != nil && status != "A" {
				return out, fmt.Errorf("read the base plan %s: %w", file, err)
			}
			if err != nil {
				before = nil
			}
			after, err := run(ctx, dir, nil, "git", "show", head+":"+file)
			if err != nil {
				return out, fmt.Errorf("read proposed plan %s: %w", file, err)
			}
			if out.Tasks, err = changedTasks(before, after); err != nil {
				return out, fmt.Errorf("compare plan %s: %w", file, err)
			}
		}
	}
	sort.Strings(out.Paths)
	return out, nil
}

// basePlans lists the plan version numbers in a plan directory on the base,
// oldest first, from either face.
func basePlans(ctx context.Context, dir, base, planDir string, run lifecycle.RunCommand) ([]string, error) {
	listed, err := command(ctx, run, dir, nil, "git", "ls-tree", "--name-only", base, "--", planDir+"/")
	if err != nil {
		return nil, fmt.Errorf("list the live plans: %w", err)
	}
	numbers := []string{}
	for file := range strings.SplitSeq(listed, "\n") {
		if face := planFace.FindStringSubmatch(path.Base(file)); face != nil && !slices.Contains(numbers, face[1]) {
			numbers = append(numbers, face[1])
		}
	}
	sort.Strings(numbers)
	return numbers, nil
}

// changedTasks names each task whose folded charter differs between two plan
// faces, including tasks only one face holds.
func changedTasks(before, after []byte) ([]string, error) {
	tasks := func(face []byte) (map[string]plan.Task, error) {
		out := map[string]plan.Task{}
		if len(face) == 0 {
			return out, nil
		}
		var doc plan.Doc
		if err := json.Unmarshal(face, &doc); err != nil {
			return nil, err
		}
		for _, t := range doc.Tasks {
			out[t.ID] = t
		}
		return out, nil
	}
	old, err := tasks(before)
	if err != nil {
		return nil, err
	}
	current, err := tasks(after)
	if err != nil {
		return nil, err
	}
	var changed []string
	for id, t := range current {
		if previous, ok := old[id]; !ok || !reflect.DeepEqual(previous, t) {
			changed = append(changed, id)
		}
	}
	for id := range old {
		if _, ok := current[id]; !ok {
			changed = append(changed, id+" (removed)")
		}
	}
	sort.Slice(changed, func(i, j int) bool { return taskOrder(changed[i]) < taskOrder(changed[j]) })
	return changed, nil
}

func taskOrder(id string) int {
	n := 0
	for _, r := range strings.TrimPrefix(id, "T") {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// keepsLivePlan refuses a plan change that drops a task or changes the plan
// version. Task IDs and their issues stay stable: a task is cancelled by
// closing its issue as not planned, and the plan keeps the task as written.
// A new split of the work is a new plan version, planned from the new lock in
// its own pull request.
func keepsLivePlan(other OtherChanges) error {
	if other.Version != "" {
		return fmt.Errorf("the proposal %s; a plan changed with its contract keeps its version and task IDs — change the live plan in place, or plan a new version from the new lock in its own pull request", other.Version)
	}
	var removed []string
	for _, id := range other.Tasks {
		if task, ok := strings.CutSuffix(id, " (removed)"); ok {
			removed = append(removed, task)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	return fmt.Errorf("the plan change removes %s; a plan changed in place keeps every task — close a task's issue as not planned to cancel it", strings.Join(removed, ", "))
}
