package ghtracker

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
)

// DefaultRoutingLabel is the automated-implementation tag a task must carry to
// be eligible for autonomous agent pickup — the harbormaster dialect's
// RoutingLabel ("Factory: eligible for autonomous agent pickup"). It is the
// default frontier filter; a repo may override it through the config surface
// (AP-8). Operator-routed work carries a different label and is never returned.
const DefaultRoutingLabel = "local-agent"

// client is the narrow, consumer-side GitHub surface the tracker adapter needs
// — one method per exact `gh api` call. The production implementation shells
// `gh` (client.go); tests inject a fake with recorded responses so the adapter
// logic is hermetic (AP-1, the hexagon; §3.10).
type client interface {
	// openIssuesWithLabel lists the OPEN issues carrying label by an exact,
	// fresh REST read — never the search index, never a cacheable conditional
	// GET (AP-15).
	openIssuesWithLabel(ctx context.Context, label string) ([]item, error)
	// blockedBy lists the issue numbers that block the given issue, freshly. The
	// frontier walk that consumes these edges lives in the Dispatcher, never in
	// the Worker (port.Task.BlockedBy).
	blockedBy(ctx context.Context, number int) ([]int, error)
	// comments lists issue-comment bodies in order, freshly. Projection writes
	// the frozen fs-spec:v1 capsule there; the adapter must hydrate it before a
	// task can reach the claim boundary.
	comments(ctx context.Context, number int) ([]string, error)
	// comment posts a progress note onto an issue by number.
	comment(ctx context.Context, number int, body string) error
}

// item is the adapter's view of one tracker issue — exactly the fields the
// frontier read consumes, decoded from GitHub by the client.
type item struct {
	Number int
	Title  string
	Body   string
	State  string // lowercased: "open" | "closed"
	Labels []string
	IsPR   bool // GitHub models pull requests as issues; the frontier excludes them
}

// Tracker is the GitHub-backed WorkTracker for one repository. It holds no task
// list and no cache: every Frontier call re-reads the live tracker (AP-15).
type Tracker struct {
	gh          client
	label       string
	repository  string
	planFeature string
	planVersion string
}

// compile-time proof the adapter satisfies the port.
var _ port.WorkTracker = (*Tracker)(nil)

// NewGH builds a production Tracker over a real gh-shelling client, filtering
// the frontier by routingLabel (empty ⇒ DefaultRoutingLabel). repo is the
// "owner/name" slug.
func NewGH(repo, routingLabel string) *Tracker {
	return newTrackerWithRepository(newGHClient(repo), routingLabel, repo)
}

func NewGHWithBudget(repo, routingLabel string, b *budget.Budget) *Tracker {
	return newTrackerWithRepository(newGHClientWithBudget(repo, b), routingLabel, repo)
}

// NewGHForPlanWithBudget builds a production Tracker whose frontier is limited
// to one frozen plan. The projection's own plan-task marker is the scope
// authority; the shared routing label remains only the coarse server-side
// eligibility filter. Invalid plan references refuse construction rather than
// falling back to the repository-wide label frontier.
func NewGHForPlanWithBudget(repo, routingLabel, planRef string, b *budget.Budget) (*Tracker, error) {
	return newTrackerForPlanWithRepository(newGHClientWithBudget(repo, b), routingLabel, planRef, repo)
}

// newTracker is the injection seam shared by NewGH and the hermetic tests.
func newTracker(gh client, routingLabel string) *Tracker {
	return newTrackerWithRepository(gh, routingLabel, "test/repository")
}

func newTrackerWithRepository(gh client, routingLabel, repository string) *Tracker {
	if routingLabel == "" {
		routingLabel = DefaultRoutingLabel
	}
	return &Tracker{gh: gh, label: routingLabel, repository: strings.TrimSpace(repository)}
}

func newTrackerForPlan(gh client, routingLabel, planRef string) (*Tracker, error) {
	return newTrackerForPlanWithRepository(gh, routingLabel, planRef, "test/repository")
}

func newTrackerForPlanWithRepository(gh client, routingLabel, planRef, repository string) (*Tracker, error) {
	feature, version, err := planref.ParsePlanRef(planRef)
	if err != nil {
		return nil, fmt.Errorf("ghtracker: %w", err)
	}
	t := newTrackerWithRepository(gh, routingLabel, repository)
	t.planFeature = feature
	t.planVersion = version
	return t, nil
}

// ValidatePlanRef checks the closed <feature>/<version> scope accepted by the
// plan-scoped tracker. It is exported so the command surface can refuse bad
// input before preparing workspaces or performing any external read. The
// scope grammar is single-sourced in planref so GitHub and Carina admit a
// plan identically (AP-11).
func ValidatePlanRef(planRef string) error {
	_, _, err := planref.ParsePlanRef(planRef)
	return err
}

// Frontier returns exactly the OPEN, routing-tagged tasks over the projected
// plan (prd:implement-now/FR-1). The unblocked ∧ unclaimed narrowing of the DAG
// is the Dispatcher's frontier walk (port.Task.BlockedBy); this adapter
// supplies the tagged open set and each item's blocker edges.
//
// The client's server-side state+label filter is an efficiency (fetch only the
// tagged set); the re-verification below is the contract (AP-15 — Frontier
// returns exactly open ∧ tagged ∧ not-a-PR regardless of how the client
// fetched, "verify an item's tag").
func (t *Tracker) Frontier(ctx context.Context) ([]port.Task, error) {
	items, err := t.gh.openIssuesWithLabel(ctx, t.label)
	if err != nil {
		return nil, fmt.Errorf("ghtracker: read frontier: %w", err)
	}
	tasks := make([]port.Task, 0, len(items))
	for _, it := range items {
		if it.IsPR || !strings.EqualFold(it.State, "open") || !hasLabel(it.Labels, t.label) {
			continue
		}
		if t.planFeature != "" && !planref.InPlan(it.Body, t.planFeature, t.planVersion) {
			continue
		}
		comments, err := t.gh.comments(ctx, it.Number)
		if err != nil {
			return nil, fmt.Errorf("ghtracker: read task capsule for #%d: %w", it.Number, err)
		}
		spec, err := taskspec.ParseTask(it.Body, comments)
		if err != nil {
			return nil, fmt.Errorf("ghtracker: validate task capsule for #%d: %w", it.Number, err)
		}
		id, title := splitTaskTitle(it.Title)
		id = spec.TaskIdentity(id)
		demandTaskID := id
		if demandTaskID == "" {
			demandTaskID = fmt.Sprintf("issue-%d", it.Number)
		}
		routing, err := spec.RoutingData(taskspec.DemandIdentity{Repository: t.repository, Task: demandTaskID})
		if err != nil {
			return nil, fmt.Errorf("tracker: task %s routing: %w", demandTaskID, err)
		}
		blocked, err := t.gh.blockedBy(ctx, it.Number)
		if err != nil {
			return nil, fmt.Errorf("ghtracker: read blockers for #%d: %w", it.Number, err)
		}
		tasks = append(tasks, port.Task{
			ID:        id,
			Number:    it.Number,
			SourceRef: fmt.Sprintf("#%d", it.Number), // same-repo GitHub issue: links correctly
			Title:     title,
			Brief:     it.Body,
			BlockedBy: blocked,

			DemandID:     routing.ID,
			Requirements: routing.Requirements,
		})
	}
	return tasks, nil
}

// Comment posts a progress note onto a tracker item. It is a notification, not
// the success artifact: the caller treats its failure as non-fatal (DINV-3).
func (t *Tracker) Comment(ctx context.Context, issue int, body string) error {
	if err := t.gh.comment(ctx, issue, body); err != nil {
		return fmt.Errorf("ghtracker: comment on #%d: %w", issue, err)
	}
	return nil
}

// hasLabel reports whether labels contains want (case-insensitively).
func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, want) {
			return true
		}
	}
	return false
}

// splitTaskTitle extracts the plan-task ID from a projected issue title of the
// form "[T3] The task title" (the harbormaster dialect's TaskTitle form),
// returning the ID ("T3") and the remaining human title. A title without the
// bracketed prefix yields an empty ID and the title unchanged.
func splitTaskTitle(raw string) (id, title string) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i > 1 {
			return strings.TrimSpace(s[1:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return "", s
}
