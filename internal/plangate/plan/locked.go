package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// lockedInputs is the plan's immutable governing view. The tag is resolved to
// one commit before any records are read, so later draft or tag changes cannot
// change the meaning of a plan during its fold.
type lockedInputs struct {
	feature      string
	basis        string
	gov          governing
	requirements map[string]bool
	designBody   []byte
	projection   *resolve.Projection
}

func loadLocked(root string, d *Doc) (*lockedInputs, []schema.Defect) {
	refuse := func(reason, remedy string) (*lockedInputs, []schema.Defect) {
		return nil, []schema.Defect{def(d.sourceFile, "governing/"+d.Basis, reason, remedy)}
	}
	ctx := context.Background()
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return refuse(fmt.Sprintf("open repository for Basis %s: %v", d.Basis, err), "run the plan fold in a Git worktree containing the binding lock")
	}
	var commit string
	if d.pending != nil && d.pending.Basis == d.Basis {
		// The lock is being prepared: its tag appears only at merge, and the
		// prepared commit holds exactly the records that merge will lock. A
		// local tag of that name is stale or hand-made, never the lock.
		commit, err = repo.ResolveCommit(ctx, d.pending.Commit)
	} else {
		commit, err = repo.ResolveCommit(ctx, "refs/tags/"+d.Basis)
	}
	if err != nil {
		return refuse(fmt.Sprintf("cannot resolve Basis %s: %v", d.Basis, err), "fetch the binding lock tag and retry")
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: commit, Prefix: store.Dir})
	if err != nil {
		return refuse(fmt.Sprintf("read memory at Basis %s: %v", d.Basis, err), "repair or fetch the binding lock and retry")
	}
	if len(set.Problems) != 0 || len(set.Dangling) != 0 {
		return refuse(fmt.Sprintf("Basis %s has invalid memory: %v; dangling relations: %v", d.Basis, set.Problems, set.Dangling), "repair the locked memory; a plan cannot project from invalid records")
	}
	projection := resolve.New(set)
	if len(projection.Violations) != 0 || len(projection.Conflicts()) != 0 {
		return refuse(fmt.Sprintf("Basis %s breaks memory projection rules: %v; conflicts: %v", d.Basis, projection.Violations, projection.Conflicts()), "repair the binding lock before planning")
	}
	prd := projection.Head(d.PRD, "prd")
	tech := projection.Head(d.Design, "design")
	if prd == nil || prd.Kind != record.Document || tech == nil || tech.Kind != record.Document {
		return refuse(fmt.Sprintf("Basis %s has no active PRD and tech-spec Documents for %s", d.Basis, d.Feature), "use the feature's binding lock with both contract documents")
	}
	prdBody, techBody := set.Bodies[prd.ID], set.Bodies[tech.ID]
	if len(prdBody) == 0 || len(techBody) == 0 {
		return refuse(fmt.Sprintf("Basis %s has an empty PRD or tech spec", d.Basis), "repair the locked contract documents")
	}
	doc, defects := ledger.ParseProjection("prd.md", schema.Normalize(prdBody))
	if len(defects) != 0 || doc == nil {
		return refuse(fmt.Sprintf("Basis %s PRD cannot be projected: %v", d.Basis, defects), "repair the locked PRD document")
	}
	out := &lockedInputs{feature: d.Feature, basis: d.Basis, designBody: techBody, projection: projection,
		gov: governing{live: map[string]bool{}, must: map[string]bool{}, optional: map[string]bool{}}, requirements: map[string]bool{}}
	for _, id := range doc.UnitOrder {
		unit := doc.Units[id]
		if unit.Prefix != "NG" && unit.Prefix != "WR" {
			claim := projection.Head(d.PRD, id)
			if claim == nil && unit.Prefix != "Q" {
				return refuse(fmt.Sprintf("Basis %s PRD row %s has no active memory record", d.Basis, id), "repair the locked requirement projection")
			}
		}
		out.gov.live[id] = true
		switch unit.Prefix {
		case "FR", "NR", "NFR", "UB":
			out.requirements[id] = true
		}
		if unit.Prefix != "FR" && unit.Prefix != "UB" {
			continue
		}
		table := doc.Tables[unit.Table]
		for i, column := range table.Cols {
			if column != "Priority" || i >= len(unit.Cells) {
				continue
			}
			priority := strings.TrimSpace(unit.Cells[i])
			switch {
			case strings.HasPrefix(priority, "Must"):
				out.gov.must[id] = true
			case strings.HasPrefix(priority, "Should"), strings.HasPrefix(priority, "Could"):
				out.gov.optional[id] = true
			}
		}
	}
	return out, nil
}

func (l *lockedInputs) designAnchor(anchor string) bool {
	if strings.HasPrefix(anchor, "D-") {
		claim := l.projection.Head("design/"+l.feature, anchor)
		return claim != nil && claim.Kind == record.Decision
	}
	return anchorIn(l.designBody, anchor)
}
