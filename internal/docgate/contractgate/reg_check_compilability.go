package contractgate

import (
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// compilability — the joint well-formedness half of stage-6 readiness
// (prd:contract-gate/FR-6; design contract-gate-v1 §3.9, class `malformed`). It
// validates that the source map + the Serves joins assemble consistently, read
// ONLY from the resolved machine-face data (DINV-3 — never re-parsing a body):
//
//   - no Serves target naming a requirement ID that does not exist (an orphan
//     Serves — a live serving design pointing at a prd:<feature>/<ID> unit
//     absent from the source map);
//   - no requirement in a broken supersession state — a supersedes/superseded_by
//     pointing at a unit that does not exist, or forming a cycle — over the
//     source map's own units;
//   - no cyclic design build-on chain — over the resolved build-on graph
//     (c.BuildsOn, from techdesigns.LiveGraph; reused, never re-derived).
//
// The corpus-wide DESIGN supersession joins are already validated at resolution
// (resolveServingDesigns fails closed as `error` on any techdesigns graph
// defect before a check runs — a broken/cyclic Supersedes among designs never
// reaches here); this check adds the two joins that resolution does not cover:
// the REQUIREMENT supersession in the source map, and the design BUILD-ON cycle
// (computeGraph flags stale-builds-on but does not detect a build-on cycle).
func init() {
	Register(Check{
		Name:          "compilability",
		Class:         ClassMalformed,
		SeverityClass: SeverityClassCompilability,
		Run:           checkCompilability,
	})
}

// checkCompilability reports every malformed join in the assembled contract.
// Output is fully sorted so the verdict is deterministic (DINV-5). Read-only
// over the resolved contract (DINV-1).
func checkCompilability(c *contract) Defects {
	var defects []schema.Defect

	// (1) Orphan Serves — a live serving design's prd:<feature>/<ID> target must
	// resolve to a unit in the source map. The serving designs are already
	// feature-scoped and their Serves refs sorted; iterate them in slug order.
	prefix := "prd:" + c.Feature + "/"
	for _, d := range c.Serving {
		for _, ref := range d.Serves {
			unit := strings.TrimPrefix(ref, prefix)
			// A range target (FR-1..3) resolves on its first endpoint, matching
			// the citation resolver's existence posture.
			head := firstUnitOfRange(unit)
			if _, ok := c.Source.Units[head]; !ok {
				defects = append(defects, cdef(c, d.Slug+".md",
					"design "+d.Slug+" Serves "+ref+" but requirement "+head+" does not exist in the source map",
					"point Serves at an existing requirement, or add the requirement to the PRD (FR-6)"))
			}
		}
	}

	// (2) Requirement supersession integrity — over the source map's own units.
	// Every supersedes/superseded_by must name an existing unit; the successor
	// chain must not cycle. supersededBy: unit → its declared successor.
	supersededBy := map[string]string{}
	var ids []string
	for id := range c.Source.Units {
		ids = append(ids, id)
	}
	schema.SortIDs(ids)
	for _, id := range ids {
		u := c.Source.Units[id]
		if u.Supersedes != nil {
			if _, ok := c.Source.Units[*u.Supersedes]; !ok {
				defects = append(defects, cdef(c, c.SourcePath,
					"requirement "+id+" supersedes "+*u.Supersedes+" which does not exist in the source map",
					"the superseded unit must exist; fix the amendment lineage then re-fold (FR-6)"))
			}
		}
		if u.SupersededBy != nil {
			if _, ok := c.Source.Units[*u.SupersededBy]; !ok {
				defects = append(defects, cdef(c, c.SourcePath,
					"requirement "+id+" is superseded by "+*u.SupersededBy+" which does not exist in the source map",
					"the successor unit must exist; fix the amendment lineage then re-fold (FR-6)"))
				continue // a dangling successor cannot enter the cycle walk
			}
			supersededBy[id] = *u.SupersededBy
		}
	}
	// Cycle detection over the successor chains, deterministic in id order.
	for _, id := range ids {
		if _, has := supersededBy[id]; !has {
			continue
		}
		cur, seen := id, map[string]bool{id: true}
		for {
			next, ok := supersededBy[cur]
			if !ok {
				break
			}
			if seen[next] {
				defects = append(defects, cdef(c, c.SourcePath,
					"requirement "+id+" is in a supersession cycle (a successor chain must end at a live unit)",
					"a superseded_by chain must terminate; fix the amendment lineage then re-fold (FR-6)"))
				break
			}
			seen[next] = true
			cur = next
		}
	}

	// (3) Design build-on cycle — over the resolved live build-on graph. A cycle
	// means the design plane is not a well-formed DAG before anything consumes
	// it as one (FR-6). Deterministic: walk slugs in sorted order.
	defects = append(defects, buildOnCycles(c)...)

	return defects
}

// buildOnCycles reports every design that participates in a build-on cycle over
// c.BuildsOn (slug → the slugs it builds on). Each cycle is reported once,
// naming a stable representative (the sorted-smallest slug in the cycle), so the
// same corpus yields the same defect set regardless of map order (DINV-5).
func buildOnCycles(c *contract) []schema.Defect {
	if len(c.BuildsOn) == 0 {
		return nil
	}
	var slugs []string
	for s := range c.BuildsOn {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)

	// Iterative colored DFS: 0=unvisited, 1=on-stack, 2=done. A back-edge to an
	// on-stack node is a cycle; report the cycle's members once by their
	// sorted-smallest slug so it is named deterministically and de-duplicated.
	const white, gray, black = 0, 1, 2
	color := map[string]int{}
	reported := map[string]bool{}
	var defects []schema.Defect

	var walk func(node string, stack []string)
	walk = func(node string, stack []string) {
		color[node] = gray
		stack = append(stack, node)
		deps := append([]string(nil), c.BuildsOn[node]...)
		sort.Strings(deps)
		for _, dep := range deps {
			switch color[dep] {
			case white:
				walk(dep, stack)
			case gray:
				// Back-edge: the cycle is the stack suffix from dep to node.
				cycle := cycleSuffix(stack, dep)
				rep := smallest(cycle)
				if !reported[rep] {
					reported[rep] = true
					defects = append(defects, cdef(c, rep+".md",
						"design build-on cycle: "+strings.Join(cycle, " → ")+" → "+dep,
						"a build-on chain must be acyclic; break the cycle in the designs' Builds-on headers (FR-6)"))
				}
			}
		}
		color[node] = black
	}
	for _, s := range slugs {
		if color[s] == white {
			walk(s, nil)
		}
	}
	sort.Slice(defects, func(i, j int) bool { return defects[i].Reason < defects[j].Reason })
	return defects
}

// cycleSuffix returns the stack slice from the first occurrence of `back` to the
// top — the nodes forming the detected cycle.
func cycleSuffix(stack []string, back string) []string {
	for i, s := range stack {
		if s == back {
			return append([]string(nil), stack[i:]...)
		}
	}
	return append([]string(nil), stack...)
}

// smallest returns the lexicographically smallest slug — a cycle's stable
// representative, so a cycle is reported once regardless of entry node.
func smallest(xs []string) string {
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

// firstUnitOfRange returns the first endpoint of a range unit token
// (FR-1..3 → FR-1); a non-range returns unchanged — a Serves target to a range
// resolves on its start (the citation resolver's existence posture).
func firstUnitOfRange(s string) string { return strings.SplitN(s, "..", 2)[0] }

// cdef builds one compilability defect naming the offending face (a design file
// for a Serves/build-on join, the source map for a requirement supersession).
func cdef(c *contract, file, reason, remedy string) schema.Defect {
	return schema.Defect{
		Verb: "contract-gate", File: file, Op: "compilability",
		Reason: reason, Remedy: remedy,
	}
}
