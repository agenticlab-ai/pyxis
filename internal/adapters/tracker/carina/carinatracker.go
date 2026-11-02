// Package carinatracker is the Carina WorkTracker adapter. It reads the ready
// frontier and posts progress by shelling the pyxis-native `carina` CLI against
// a local work-item store — the medium-independent alternative to the GitHub
// issue tracker (ghtracker), selected when a plan is projected to Carina rather
// than to a code host (AP-8: the tracker is a swappable port binding). Only the
// work-item medium changes here; the claim (Lease), the workspace, and the code
// host stay on git + the code host, so a Carina-tracked run still opens ordinary
// proposals and never merges.
package carinatracker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
)

const (
	defaultBinary = "carina"
	defaultAuthor = "implement-now"
)

// Tracker implements port.WorkTracker over a Carina work-item store. The run
// seam is injectable so argv construction and JSON decoding are unit-tested
// without a real `carina` binary (docs/tech-design/implement-now.md §3.10).
type Tracker struct {
	root       string // carina --root: the operand root holding the work-item store
	author     string // author stamped on progress comments
	repository string
	run        func(ctx context.Context, args ...string) ([]byte, error)

	// planFeature/planVersion, when set, limit the frontier to one frozen
	// plan's tasks — the projection's own plan-task marker is the scope
	// authority, exactly as for ghtracker. Empty ⇒ the whole store's ready set
	// (the repository-wide frontier).
	planFeature string
	planVersion string
}

var _ port.WorkTracker = (*Tracker)(nil)

// Option customizes a Tracker.
type Option func(*Tracker)

// WithBinary overrides the `carina` executable. Callers that do not already
// hold a resolved path should use toolbin.Ensure so source checkouts do not
// depend on a globally installed binary.
func WithBinary(bin string) Option {
	return func(t *Tracker) {
		if strings.TrimSpace(bin) != "" {
			t.run = shellCarina(bin)
		}
	}
}

// WithAuthor sets the author stamped on Comment (default "implement-now").
func WithAuthor(a string) Option {
	return func(t *Tracker) {
		if strings.TrimSpace(a) != "" {
			t.author = a
		}
	}
}

// WithRepository binds semantic demand to the code repository whose plan the
// Carina store projects. The tracker root is a storage location, not repository
// identity, and must never participate in a portable DemandID.
func WithRepository(repository string) Option {
	return func(t *Tracker) {
		if strings.TrimSpace(repository) != "" {
			t.repository = strings.TrimSpace(repository)
		}
	}
}

// withRunner injects a fake run seam (tests only).
func withRunner(r func(ctx context.Context, args ...string) ([]byte, error)) Option {
	return func(t *Tracker) { t.run = r }
}

// New binds a Tracker to a work-item store root, shelling the real `carina`.
func New(root string, opts ...Option) *Tracker {
	t := &Tracker{root: root, author: defaultAuthor, repository: "local/carina", run: shellCarina(defaultBinary)}
	for _, o := range opts {
		o(t)
	}
	return t
}

// NewForPlan binds a Tracker whose frontier is limited to one frozen plan,
// exactly as ghtracker.NewGHForPlan does for the GitHub medium — the same
// admission grammar (planref.ParsePlanRef) and the same marker filter, so a
// plan is scoped IDENTICALLY on either tracker (AP-11). An invalid plan
// reference refuses construction rather than falling back to the store-wide
// ready set.
func NewForPlan(root, planRef string, opts ...Option) (*Tracker, error) {
	feature, version, err := planref.ParsePlanRef(planRef)
	if err != nil {
		return nil, fmt.Errorf("carinatracker: %w", err)
	}
	t := New(root, opts...)
	t.planFeature = feature
	t.planVersion = version
	return t, nil
}

// shellCarina returns a run seam that execs `<bin> <args…>` and returns stdout;
// a non-zero exit becomes an error carrying carina's stderr.
func shellCarina(bin string) func(ctx context.Context, args ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, bin, args...)
		out, err := cmd.Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) && len(ee.Stderr) > 0 {
				return nil, fmt.Errorf("carina %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
			}
			return nil, fmt.Errorf("carina %s: %w", strings.Join(args, " "), err)
		}
		return out, nil
	}
}

// predicateSet is carina's `worktracker.predicate-set/v1` envelope — the shape
// of `carina ready --json`.
type predicateSet struct {
	Predicate string     `json:"predicate"`
	Items     []workItem `json:"items"`
}

// workItem is the subset of `worktracker.work-item/v1` the frontier consumes.
type workItem struct {
	ID        int      `json:"id"`
	Type      string   `json:"type"`
	State     string   `json:"state"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	BlockedBy []string `json:"blocked_by"`
	Log       []struct {
		Kind    string `json:"kind"`
		Payload string `json:"payload"`
	} `json:"log"`
}

// Frontier returns the ready tasks. Carina's `ready` predicate already yields
// the open, authorized (agent-signalled), unblocked, non-rollup leaves — the
// frontier the design defines (prd:implement-now/FR-1) — so this maps each item
// to a Task; the Dispatcher's own open∧unblocked gate is a no-op superset. Each
// read is exact and fresh: `ready` recomputes from the item files, never a
// stale index (AP-15). Frozen requirements are read from the projected
// fs-spec:v2 carrier before return; Harness and HeadBranch are resolved
// downstream (config routing and the workspace), exactly as for the gh adapter.
func (t *Tracker) Frontier(ctx context.Context) ([]port.Task, error) {
	out, err := t.run(ctx, "ready", "--root", t.root, "--json")
	if err != nil {
		return nil, fmt.Errorf("carinatracker: read frontier: %w", err)
	}
	var ps predicateSet
	if err := json.Unmarshal(out, &ps); err != nil {
		return nil, fmt.Errorf("carinatracker: decode ready set: %w", err)
	}
	tasks := make([]port.Task, 0, len(ps.Items))
	for _, it := range ps.Items {
		if !strings.EqualFold(it.State, "open") || !isLeafTask(it.Type) {
			continue // ready already excludes these; defensive, mirrors the gh adapter's filter
		}
		if t.planFeature != "" && !planref.InPlan(it.Body, t.planFeature, t.planVersion) {
			continue // scoped: only this frozen plan's tasks (the same filter ghtracker applies)
		}
		spec, err := taskspec.ParseTask(it.Body, commentPayloads(it))
		if err != nil {
			return nil, fmt.Errorf("carinatracker: validate task capsule for item %d: %w", it.ID, err)
		}
		id, title := splitTaskTitle(it.Title)
		id = spec.TaskIdentity(id)
		demandTaskID := id
		if demandTaskID == "" {
			demandTaskID = fmt.Sprintf("work-item-%d", it.ID)
		}
		routing, err := spec.RoutingData(taskspec.DemandIdentity{Repository: t.repository, Task: demandTaskID})
		if err != nil {
			return nil, fmt.Errorf("tracker: task %s routing: %w", demandTaskID, err)
		}
		tasks = append(tasks, port.Task{
			ID:        id,
			Number:    it.ID,
			SourceRef: fmt.Sprintf("Carina work-item %d", it.ID), // local store: no bare #N, so GitHub never mis-links it
			Title:     title,
			Brief:     it.Body,
			BlockedBy: parseRefs(it.BlockedBy),

			DemandID:     routing.ID,
			Requirements: routing.Requirements,
		})
	}
	return tasks, nil
}

func commentPayloads(it workItem) []string {
	var comments []string
	for _, entry := range it.Log {
		if entry.Kind == "comment" {
			comments = append(comments, entry.Payload)
		}
	}
	return comments
}

// Comment appends a progress note to a work item's log. Per the port contract it
// is a notification, never the success artifact — a failure here is non-fatal to
// the caller; the proposal is what counts (DINV-3).
func (t *Tracker) Comment(ctx context.Context, number int, body string) error {
	if _, err := t.run(ctx, "comment", strconv.Itoa(number), "--root", t.root, "--body", body, "--author", t.author); err != nil {
		return fmt.Errorf("carinatracker: comment on #%d: %w", number, err)
	}
	return nil
}

// isLeafTask reports whether a work-item type is an implementable leaf, not a
// rollup container (epic/feature/milestone) the frontier never hands to a Worker.
func isLeafTask(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "epic", "feature", "milestone":
		return false
	default:
		return true
	}
}

// splitTaskTitle peels a leading "[T1] " tag off a projected title, returning
// the plan-task id and the remaining human title. A title without the tag keeps
// an empty id and its whole text.
func splitTaskTitle(s string) (id, title string) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		if end := strings.IndexByte(s, ']'); end > 1 {
			return strings.TrimSpace(s[1:end]), strings.TrimSpace(s[end+1:])
		}
	}
	return "", s
}

// parseRefs keeps the numeric local ids from carina's opaque dep refs; a ref may
// also be a cross-repo "owner/repo#id" form, which carries no local work-item
// number and is dropped (the frontier only walks local blockers).
func parseRefs(refs []string) []int {
	if len(refs) == 0 {
		return nil
	}
	out := make([]int, 0, len(refs))
	for _, r := range refs {
		if n, err := strconv.Atoi(strings.TrimSpace(r)); err == nil {
			out = append(out, n)
		}
	}
	return out
}
