// Package project executes a frozen plan onto the tracker — the stateless,
// convergent projection of plan-new-v1 §3.7. No repo-resident state:
// reality is probed exactly every run; the idempotency key is the
// plan-task-id marker stamped at creation; ordering is create → spec →
// link → wire → route-last (DINV-5); a duplicate marker halts loudly and
// is never auto-deleted (DINV-3, AP-17); writes stop at the first failure
// and the report names what exists (PRD UB-4).
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

// ReportSchemaVersion pins the projection report's contract (PRD FR-24).
const ReportSchemaVersion = "plan-projection-report/v1"

// Item is one task's projection outcome.
type Item struct {
	TaskID     string `json:"task_id"`
	PlanTaskID string `json:"plan_task_id"`
	Issue      int    `json:"issue,omitempty"`
	Status     string `json:"status"` // created | existing | updated | duplicate | failed | dry-run
	Notes      string `json:"notes,omitempty"`
}

// Report is the schema-versioned projection outcome (FR-24): the resume
// substrate, the retirement guide, and the evidence artifact.
type Report struct {
	Schema  string `json:"schema_version"`
	Tool    string `json:"generated_by"`
	Plan    string `json:"plan"`
	Feature string `json:"feature,omitempty"`
	Intake  string `json:"intake,omitempty"`
	Version string `json:"plan_version"`
	Dialect string `json:"dialect"`
	Epic    int    `json:"epic,omitempty"`
	Routing string `json:"routing"` // applied | withheld | dry-run
	Items   []Item `json:"items"`
}

// JSON renders the report deterministically.
func (r *Report) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	return append(b, '\n'), nil
}

// Options steer one projection run.
type Options struct {
	DryRun    bool
	NoRouting bool
	// MergedBytes returns the frozen (merged) bytes of a repo-relative
	// path — DINV-1's oracle. The CLI wires `git show origin/main:<rel>`;
	// nil skips the check (hermetic tests only — never the CLI).
	MergedBytes func(rel string) ([]byte, error)
	// Log receives the per-step observability lines (§3.4); nil = stdout.
	Log io.Writer
}

func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "plangate", File: file, Op: op, Reason: reason, Remedy: remedy}
}

// probe is the exact-read snapshot the run decides from (DINV-4).
type probe struct {
	epic       int
	epicTitle  string
	epicBody   string
	epicDupes  []int
	taskIssues map[string][]tracker.Issue // plan-task-id → carriers
	linked     map[int]bool               // issue → linked under epic
	open       []tracker.Issue
}

func runProbe(tr tracker.Tracker, d *plan.Doc) (probe, error) {
	p := probe{taskIssues: map[string][]tracker.Issue{}, linked: map[int]bool{}}
	open, err := tr.OpenIssues()
	if err != nil {
		return p, fmt.Errorf("probe open issues: %w", err)
	}
	p.open = open
	epicMarker := dialect.PlanEpicMarker(d.Name(), d.Version)
	for _, is := range open {
		if strings.Contains(is.Body, epicMarker) {
			if p.epic == 0 {
				p.epic, p.epicTitle, p.epicBody = is.Number, is.Title, is.Body
			} else {
				p.epicDupes = append(p.epicDupes, is.Number)
			}
		}
	}
	candidates := open
	if p.epic != 0 {
		subs, err := tr.SubIssues(p.epic)
		if err != nil {
			return p, fmt.Errorf("probe epic sub-issues: %w", err)
		}
		for _, s := range subs {
			p.linked[s.Number] = true
		}
		candidates = append(candidates, subs...)
	}
	seen := map[int]bool{}
	for _, is := range candidates {
		if seen[is.Number] {
			continue
		}
		seen[is.Number] = true
		for i := range d.Tasks {
			if strings.Contains(is.Body, dialect.PlanTaskMarker(d.Tasks[i].PlanTaskID)) {
				p.taskIssues[d.Tasks[i].PlanTaskID] = append(p.taskIssues[d.Tasks[i].PlanTaskID], is)
			}
		}
	}
	return p, nil
}

// Project runs one convergent projection pass. The returned report always
// reflects what exists, defects or not.
func Project(root string, cfg config.Config, rel string, tr tracker.Tracker, m dialect.Manifest, opts Options) (*Report, []schema.Defect) {
	logw := opts.Log
	if logw == nil {
		logw = os.Stdout
	}
	logf := func(format string, args ...any) {
		// Diagnostics are best effort; tracker errors and the report determine
		// the projection result even when its optional log sink has closed.
		_, _ = fmt.Fprintf(logw, "plangate: "+format+"\n", args...)
	}

	res := plan.FoldFile(root, cfg, rel)
	if len(res.Defects) > 0 {
		return nil, res.Defects
	}
	d := res.Doc
	rep := &Report{Schema: ReportSchemaVersion, Tool: plan.ToolVersion, Plan: rel,
		Feature: d.Feature, Intake: d.Intake, Version: d.Version, Dialect: d.Dialect, Routing: "withheld"}
	if d.Schema != plan.NativeSchemaVersion {
		return rep, []schema.Defect{def(rel, "requirements", "historical plans cannot be projected for execution", "create a current plan with complete task requirements")}
	}

	// DINV-1 — projection reads only merged plan content.
	if opts.MergedBytes != nil {
		merged, err := opts.MergedBytes(rel)
		local, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || rerr != nil || string(schema.Normalize(merged)) != string(schema.Normalize(local)) {
			return rep, []schema.Defect{def(rel, "freeze",
				"the working plan does not match its merged form on the default branch",
				"merge the plan PR first — projection reads only frozen plans (DINV-1)")}
		}
	}

	pr, err := runProbe(tr, d)
	if err != nil {
		return rep, []schema.Defect{def(rel, "probe", err.Error(), "the probe is read-only — fix tracker access and re-run")}
	}
	var defects []schema.Defect
	if len(pr.epicDupes) > 0 {
		defects = append(defects, def(rel, "duplicate",
			fmt.Sprintf("multiple epics carry this plan's marker: #%d and %v", pr.epic, pr.epicDupes),
			"close the surplus epic(s) by hand and re-run — never auto-deleted (DINV-3)"))
		return rep, defects
	}

	// Task issues found with no open epic mean the epic was closed while
	// its tasks were not: a new epic would duplicate the tasks it cannot
	// see, so the projection stops before writing anything.
	if pr.epic == 0 && len(pr.taskIssues) > 0 {
		defects = append(defects, def(rel, "probe",
			"this plan's tasks have open issues, but its epic is not open",
			"reopen the plan's epic, or close its tasks, and re-run — a closed epic is never replaced (DINV-3)"))
		return rep, defects
	}

	halt := func(op string, err error) []schema.Defect {
		defects = append(defects, def(rel, op, err.Error(), "re-run after fixing — projection converges from any cut point (DINV-3)"))
		return defects
	}

	// Labels — ensure missing only (FR-23).
	for _, l := range m.Labels() {
		if opts.DryRun {
			logf("would ensure label %s", l.Name)
			continue
		}
		if err := tr.EnsureLabel(l.Name, l.Color, l.Description); err != nil {
			return rep, halt("labels", err)
		}
	}

	// Epic.
	if pr.epic == 0 {
		if opts.DryRun {
			logf("would create epic %q", d.EpicTitle)
		} else {
			n, err := tr.CreateIssue(m.EpicTitle(d), m.EpicBody(d), m.EpicLabels, m.EpicType)
			if err != nil {
				return rep, halt("epic", err)
			}
			pr.epic = n
			logf("created epic → #%d", n)
		}
	} else {
		logf("epic exists → #%d", pr.epic)
		// A plan changed in place keeps its epic; its title and body follow
		// the plan.
		if title, want := m.EpicTitle(d), m.EpicBody(d); strings.TrimSpace(pr.epicTitle) != title || !sameBody(pr.epicBody, want) {
			if opts.DryRun {
				logf("would update epic #%d", pr.epic)
			} else {
				if err := tr.EditIssue(pr.epic, title, want); err != nil {
					return rep, halt("epic update", err)
				}
				logf("updated epic #%d", pr.epic)
			}
		}
	}
	rep.Epic = pr.epic

	// Tasks — create missing; duplicates recorded, never touched. A changed
	// charter is only recorded here; it is written after its task stops
	// being dispatchable.
	issueOf := map[string]int{}
	duped := map[string]bool{}
	type charter struct{ title, body string }
	recharter := map[string]charter{}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		carriers := pr.taskIssues[t.PlanTaskID]
		switch {
		case len(carriers) > 1:
			nums := make([]string, len(carriers))
			for j, c := range carriers {
				nums[j] = fmt.Sprintf("#%d", c.Number)
			}
			duped[t.PlanTaskID] = true
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID,
				Status: "duplicate", Notes: strings.Join(nums, ", ")})
			defects = append(defects, def(rel, "duplicate",
				fmt.Sprintf("%s exists as %s — concurrent projections raced", t.PlanTaskID, strings.Join(nums, ", ")),
				"close one by hand and re-run; routing stays withheld for the pair (DINV-3, AP-17)"))
		case len(carriers) == 1:
			issueOf[t.PlanTaskID] = carriers[0].Number
			item := Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Issue: carriers[0].Number, Status: "existing"}
			logf("task %s exists → #%d", t.ID, carriers[0].Number)
			if opts.DryRun && len(m.StaleRouting(*t, carriers[0].Labels)) > 0 {
				item.Notes = "routing would change"
				logf("would move the routing of #%d (%s)", carriers[0].Number, t.ID)
			}
			if add, remove := m.Classification(*t, carriers[0].Labels); opts.DryRun && len(add)+len(remove) > 0 {
				item.Notes = strings.TrimPrefix(item.Notes+"; classification would change", "; ")
				logf("would reclassify #%d (%s)", carriers[0].Number, t.ID)
			}
			// A plan changed in place keeps its tasks' issues; each title and
			// body follows the plan, so the next dispatch reads the new charter.
			if title, want := m.TaskTitle(*t), m.TaskBody(d, *t, nil); strings.TrimSpace(carriers[0].Title) != title || !sameBody(carriers[0].Body, want) {
				recharter[t.PlanTaskID] = charter{title, want}
				if opts.DryRun {
					item.Notes = strings.TrimPrefix(item.Notes+"; charter would update", "; ")
					logf("would update task %s charter on #%d", t.ID, carriers[0].Number)
				}
			}
			rep.Items = append(rep.Items, item)
		case opts.DryRun:
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Status: "dry-run"})
			logf("would create task %s — %s", t.ID, t.Title)
		default:
			n, err := tr.CreateIssue(m.TaskTitle(*t), m.TaskBody(d, *t, nil), m.TaskCreateLabels(*t), m.TaskType)
			if err != nil {
				rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Status: "failed", Notes: err.Error()})
				return rep, halt("create "+t.ID, err)
			}
			issueOf[t.PlanTaskID] = n
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Issue: n, Status: "created"})
			logf("created %s → #%d", t.ID, n)
		}
	}
	if opts.DryRun {
		rep.Routing = "dry-run"
		logf("dry-run: no spec/link/wire/route writes")
		return rep, defects
	}

	blockersOf := func(t *plan.Task) []int {
		var nums []int
		for _, dep := range t.BlockedBy {
			if n, ok := issueOf[d.Name()+"/"+d.Version+"/"+dep]; ok {
				nums = append(nums, n)
			}
		}
		return nums
	}

	// A task that changes in place stops being dispatchable before its
	// charter, spec, or edges change: every routing label it carries is
	// withdrawn, and the route step gives back the one the plan names, last
	// (DINV-5). A projection that stops part way leaves it unrouted.
	withdrawn := map[string]bool{}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		carriers := pr.taskIssues[t.PlanTaskID]
		if len(carriers) != 1 {
			continue
		}
		n := carriers[0].Number
		labels := m.StaleRouting(*t, carriers[0].Labels)
		_, changing := recharter[t.PlanTaskID]
		if !changing {
			comments, err := tr.Comments(n)
			if err != nil {
				return rep, halt("route probe "+t.ID, err)
			}
			spec, err := m.SpecComment(d, *t, blockersOf(t))
			if err != nil {
				return rep, halt("spec render "+t.ID, err)
			}
			present, err := taskspec.MatchNative(comments, spec)
			changing = err != nil || !present
		}
		if changing && carriers[0].HasLabel(m.Routing(*t)) {
			labels = append(labels, m.Routing(*t))
		}
		for _, label := range labels {
			if err := tr.RemoveLabel(n, label); err != nil {
				return rep, halt("withdraw route "+t.ID, err)
			}
			withdrawn[t.PlanTaskID] = true
			markUpdated(rep, t.PlanTaskID)
			logf("withdrew %s from #%d (%s)", label, n, t.ID)
		}
	}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		c, ok := recharter[t.PlanTaskID]
		if !ok {
			continue
		}
		if err := tr.EditIssue(issueOf[t.PlanTaskID], c.title, c.body); err != nil {
			return rep, halt("update "+t.ID, err)
		}
		markUpdated(rep, t.PlanTaskID)
		logf("updated task %s charter on #%d", t.ID, issueOf[t.PlanTaskID])
	}
	// The mechanical-quarantine signal is classification the projection
	// owns; it follows a plan changed in place as the charter does.
	for i := range d.Tasks {
		t := &d.Tasks[i]
		carriers := pr.taskIssues[t.PlanTaskID]
		if len(carriers) != 1 {
			continue
		}
		n := carriers[0].Number
		add, remove := m.Classification(*t, carriers[0].Labels)
		if len(add) > 0 {
			if err := tr.AddLabels(n, add); err != nil {
				return rep, halt("classify "+t.ID, err)
			}
		}
		for _, label := range remove {
			if err := tr.RemoveLabel(n, label); err != nil {
				return rep, halt("classify "+t.ID, err)
			}
		}
		if len(add)+len(remove) > 0 {
			markUpdated(rep, t.PlanTaskID)
			logf("reclassified #%d (%s)", n, t.ID)
		}
	}

	// Spec comments — the frozen charter in the executor's grammar; a
	// crash between create and spec heals here on the next run.
	for i := range d.Tasks {
		t := &d.Tasks[i]
		n, ok := issueOf[t.PlanTaskID]
		if !ok {
			continue
		}
		comments, err := tr.Comments(n)
		if err != nil {
			return rep, halt("spec probe "+t.ID, err)
		}
		if d.Schema != plan.NativeSchemaVersion && m.HasSpec(comments) {
			continue
		}
		body, err := m.SpecComment(d, *t, blockersOf(t))
		if err != nil {
			return rep, halt("spec render "+t.ID, err)
		}
		if d.Schema == plan.NativeSchemaVersion {
			present, err := taskspec.MatchNative(comments, body)
			if err != nil {
				// The projected spec differs from the plan: the plan changed
				// in place. Rewrite the projection's own single spec comment;
				// anything ambiguous fails closed.
				stale, ok := staleSpec(comments, d.SourceFile(), t.PlanTaskID)
				if !ok {
					return rep, halt("spec verify "+t.ID, err)
				}
				if err := tr.ReplaceComment(n, stale, body); err != nil {
					if errors.Is(err, tracker.ErrInPlaceUnsupported) {
						defects = append(defects, def(rel, "spec update "+t.ID, err.Error(),
							"write the change as a new plan version; this tracker's comments cannot change in place"))
						return rep, defects
					}
					return rep, halt("spec update "+t.ID, err)
				}
				actual, err := tr.Comments(n)
				if err != nil {
					return rep, halt("spec readback "+t.ID, err)
				}
				if present, err := taskspec.MatchNative(actual, body); err != nil || !present {
					return rep, halt("spec readback "+t.ID, errors.Join(err, fmt.Errorf("updated native spec is missing")))
				}
				markUpdated(rep, t.PlanTaskID)
				logf("spec updated on #%d (%s)", n, t.ID)
				continue
			}
			if present {
				continue
			}
		}
		if err := tr.Comment(n, body); err != nil {
			return rep, halt("spec "+t.ID, err)
		}
		if d.Schema == plan.NativeSchemaVersion {
			actual, err := tr.Comments(n)
			if err != nil {
				return rep, halt("spec readback "+t.ID, err)
			}
			present, err := taskspec.MatchNative(actual, body)
			if err != nil {
				return rep, halt("spec readback "+t.ID, err)
			}
			if !present {
				return rep, halt("spec readback "+t.ID, fmt.Errorf("posted native spec is missing"))
			}
		}
		logf("spec posted on #%d (%s)", n, t.ID)
	}

	// Link every task under the epic.
	for i := range d.Tasks {
		t := &d.Tasks[i]
		n, ok := issueOf[t.PlanTaskID]
		if !ok || pr.linked[n] {
			continue
		}
		if err := tr.LinkSubIssue(pr.epic, n); err != nil {
			return rep, halt("link "+t.ID, err)
		}
		logf("linked #%d under epic #%d", n, pr.epic)
	}

	// Wire every dependency edge (before any routing — DINV-5). An edge
	// between two of this plan's tasks that the plan no longer names was wired
	// by an earlier projection of a plan changed in place, so it goes; edges
	// to other issues are not the projection's.
	ownTask := map[int]bool{}
	for _, n := range issueOf {
		ownTask[n] = true
	}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		n, ok := issueOf[t.PlanTaskID]
		if !ok {
			continue
		}
		existing, err := tr.BlockedBy(n)
		if err != nil {
			return rep, halt("wire probe "+t.ID, err)
		}
		planned := map[int]bool{}
		for _, b := range blockersOf(t) {
			planned[b] = true
		}
		have := map[int]bool{}
		for _, e := range existing {
			have[e] = true
			if planned[e] || !ownTask[e] {
				continue
			}
			if err := tr.RemoveBlockedBy(n, e); err != nil {
				return rep, halt("unwire "+t.ID, err)
			}
			logf("unwired #%d blocked-by #%d", n, e)
		}
		for _, b := range blockersOf(t) {
			if have[b] {
				continue
			}
			if err := tr.AddBlockedBy(n, b); err != nil {
				return rep, halt("wire "+t.ID, err)
			}
			logf("wired #%d blocked-by #%d", n, b)
		}
	}

	// Route last — the task becomes dispatchable only now (DINV-5); a
	// duplicated plan-task-id never routes (DINV-3).
	if opts.NoRouting {
		logf("routing withheld (--no-routing)")
	} else {
		for i := range d.Tasks {
			t := &d.Tasks[i]
			n, ok := issueOf[t.PlanTaskID]
			if !ok || duped[t.PlanTaskID] {
				continue
			}
			label := m.Routing(*t)
			if d.Schema == plan.NativeSchemaVersion {
				expected, err := m.SpecComment(d, *t, blockersOf(t))
				if err != nil {
					return rep, halt("route spec "+t.ID, err)
				}
				comments, err := tr.Comments(n)
				if err != nil {
					return rep, halt("route readback "+t.ID, err)
				}
				present, err := taskspec.MatchNative(comments, expected)
				if err != nil {
					return rep, halt("route verify "+t.ID, err)
				}
				if !present {
					return rep, halt("route verify "+t.ID, fmt.Errorf("native spec disappeared before routing"))
				}
			}
			carrier := pr.taskIssues[t.PlanTaskID]
			if len(carrier) == 1 && carrier[0].HasLabel(label) && !withdrawn[t.PlanTaskID] {
				continue
			}
			if err := tr.AddLabels(n, []string{label}); err != nil {
				return rep, halt("route "+t.ID, err)
			}
			logf("routed #%d (%s → %s)", n, t.ID, label)
		}
		rep.Routing = "applied"
	}

	// The report rides the epic — evidence where execution lives (AP-13).
	js, err := rep.JSON()
	if err != nil {
		return rep, halt("report", err)
	}
	body := fmt.Sprintf("Projection report (`%s`):\n\n```json\n%s```\n", ReportSchemaVersion, js)
	if err := tr.Comment(pr.epic, m.Neutralize(body)); err != nil {
		return rep, halt("report", err)
	}
	logf("report posted on epic #%d", pr.epic)
	return rep, defects
}

// sameBody compares an issue body with the projection's rendering, ignoring
// the line-ending and trailing-space changes a tracker may apply.
func sameBody(got, want string) bool {
	clean := func(s string) string {
		return strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), " \n")
	}
	return clean(got) == clean(want)
}

// staleSpec returns the one comment that carries this task's projected spec.
// Several spec comments, or one for another task or plan, is no in-place
// change the projection made, so it reports none.
func staleSpec(comments []string, planFile, planTaskID string) (string, bool) {
	var carrier string
	found := 0
	for _, c := range comments {
		if strings.Contains(c, "<!-- fs-spec:") {
			carrier = c
			found++
		}
	}
	if found != 1 {
		return "", false
	}
	spec, err := taskspec.Parse([]string{carrier})
	if err != nil || spec.PlanTaskID != planTaskID || spec.Plan != planFile {
		return "", false
	}
	return carrier, true
}

func markUpdated(rep *Report, planTaskID string) {
	for i := range rep.Items {
		if rep.Items[i].PlanTaskID == planTaskID && rep.Items[i].Status == "existing" {
			rep.Items[i].Status = "updated"
		}
	}
}

// Probe regenerates the report from tracker reality without writing — the
// `report` verb (statelessness makes the tracker the debugger, §3.4).
func Probe(root string, cfg config.Config, rel string, tr tracker.Tracker, m dialect.Manifest) (*Report, []schema.Defect) {
	res := plan.FoldFile(root, cfg, rel)
	if len(res.Defects) > 0 {
		return nil, res.Defects
	}
	d := res.Doc
	rep := &Report{Schema: ReportSchemaVersion, Tool: plan.ToolVersion, Plan: rel,
		Feature: d.Feature, Intake: d.Intake, Version: d.Version, Dialect: d.Dialect, Routing: "probe"}
	pr, err := runProbe(tr, d)
	if err != nil {
		return rep, []schema.Defect{def(rel, "probe", err.Error(), "the probe is read-only — fix tracker access and re-run")}
	}
	rep.Epic = pr.epic
	for i := range d.Tasks {
		t := &d.Tasks[i]
		carriers := pr.taskIssues[t.PlanTaskID]
		switch len(carriers) {
		case 0:
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Status: "absent"})
		case 1:
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Issue: carriers[0].Number, Status: "existing"})
		default:
			nums := make([]string, len(carriers))
			for j, c := range carriers {
				nums[j] = fmt.Sprintf("#%d", c.Number)
			}
			rep.Items = append(rep.Items, Item{TaskID: t.ID, PlanTaskID: t.PlanTaskID, Status: "duplicate", Notes: strings.Join(nums, ", ")})
		}
	}
	return rep, nil
}
