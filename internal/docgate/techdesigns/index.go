package techdesigns

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// IndexEntry is one design's computed state (DINV-3: liveness is
// computed from Supersedes edges, never written by any document).
type IndexEntry struct {
	Status           string   `json:"status"` // live | superseded
	SupersededBy     *string  `json:"superseded_by"`
	CurrentSuccessor *string  `json:"current_successor"`
	BuildsOn         []string `json:"builds_on"`
	Serves           []string `json:"serves"`
	Flags            []string `json:"flags"`
}

// indexDoc is the committed index — design-plane data only. The
// requirements-coverage join left it for on-read computation (`docgate
// coverage`, FR-28 / coverage-on-read design): a cross-plane join committed
// under one plane coupled the PRD and design planes. schema_version 2 drops
// the former `coverage` key.
type indexDoc struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedBy   string                 `json:"generated_by"`
	ToolVersion   string                 `json:"tool_version"`
	Designs       map[string]*IndexEntry `json:"designs"`
	ServesInverse map[string][]string    `json:"serves_inverse"`
}

// coverageDoc is the requirements-coverage face (FR-28): the universe is
// corpus IDs + live ledger units; only live designs serve (DINV-3 — a
// supersession is never laundered). Computed on read, committed nowhere.
type coverageDoc struct {
	UniverseSize int                 `json:"universe_size"`
	Served       map[string][]string `json:"served"`   // anchor → live design slugs
	Unserved     map[string][]string `json:"unserved"` // family → anchors, sorted
}

func idef(file, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "gen-index", File: file, Op: "graph", Reason: reason, Remedy: remedy}
}

// computeGraph builds the design liveness graph and the live serves-inverse.
// Shared by gen-index (which renders it) and the coverage verb (which joins
// the inverse against the requirement universe) — one computation, one
// liveness truth (AP-11).
func computeGraph(designs []*Design, env Env) (entries map[string]*IndexEntry, inverse map[string][]string, defects []schema.Defect) {
	entries = map[string]*IndexEntry{}
	supersededBy := map[string]string{} // target slug → successor slug
	for _, d := range designs {
		e := &IndexEntry{Status: "live", BuildsOn: []string{}, Serves: []string{}, Flags: []string{}}
		if env.Grandfathered[d.Slug] {
			e.Flags = append(e.Flags, "grandfathered")
		}
		if v := strings.TrimSpace(d.Header["Serves"]); v != "" && v != "—" {
			e.Serves = splitRefs(v)
		}
		if v := strings.TrimSpace(d.Header["Builds-on"]); v != "" && v != "—" {
			for _, r := range splitRefs(v) {
				e.BuildsOn = append(e.BuildsOn, refSlug(r))
			}
		}
		entries[d.Slug] = e
	}
	// Supersedes edges → liveness.
	for _, d := range designs {
		v := strings.TrimSpace(d.Header["Supersedes"])
		if v == "" || v == "—" {
			continue
		}
		for _, r := range splitRefs(v) {
			target := refSlug(r)
			if _, ok := entries[target]; !ok {
				defects = append(defects, idef(d.Slug+".md", fmt.Sprintf("Supersedes target %q does not exist", target), "name an existing design"))
				continue
			}
			if prev, dup := supersededBy[target]; dup {
				defects = append(defects, idef(d.Slug+".md",
					fmt.Sprintf("%q is superseded by both %q and %q — one live successor only", target, prev, d.Slug),
					"supersede the successor, not the ancestor"))
				continue
			}
			supersededBy[target] = d.Slug
			s := d.Slug
			entries[target].Status = "superseded"
			entries[target].SupersededBy = &s
		}
	}
	// Cycles + transitive current successor.
	for slug, e := range entries {
		if e.Status != "superseded" {
			continue
		}
		cur, seen := slug, map[string]bool{slug: true}
		for {
			next, ok := supersededBy[cur]
			if !ok {
				break
			}
			if seen[next] {
				defects = append(defects, idef(slug+".md", "supersession cycle detected", "a successor chain must end at a live design"))
				cur = ""
				break
			}
			seen[next] = true
			cur = next
		}
		if cur != "" && cur != slug {
			c := cur
			e.CurrentSuccessor = &c
		}
	}
	// Staleness flags. The specific offending anchors come from the shared
	// StaleServeAnchors / StaleBuildsOnAnchors predicates so the contract gate's
	// currency check (contract-gate-v1 §3.9) reads the SAME computation this
	// index renders as a flag — one staleness truth, no re-derivation (AP-11,
	// NR-2). The flag is set iff any offending anchor exists, byte-identical to
	// the prior inline computation.
	for _, d := range designs {
		e := entries[d.Slug]
		if e.Status != "live" {
			continue
		}
		if len(StaleServeAnchors(e, entries, env)) > 0 {
			e.Flags = appendOnce(e.Flags, "stale-serves")
		}
		if len(StaleBuildsOnAnchors(e, entries)) > 0 {
			e.Flags = appendOnce(e.Flags, "stale-builds-on")
		}
	}
	// Inverse map (live designs only).
	inverse = map[string][]string{}
	for slug, e := range entries {
		if e.Status != "live" {
			continue
		}
		for _, ref := range e.Serves {
			inverse[ref] = append(inverse[ref], slug)
		}
	}
	for k := range inverse {
		sort.Strings(inverse[k])
	}
	return entries, inverse, defects
}

// StaleServeAnchors returns the Serves refs of a live design that are stale —
// a `design:` ref whose target is superseded, or a `prd:<feature>/<unit>` ref
// whose unit is not live (superseded/retired) — in the order they appear in the
// design's Serves list. It is the single source of the `stale-serves` staleness
// truth: computeGraph sets the index flag from it (whether any exist) and the
// contract gate's currency check reads it to name each offending anchor
// (contract-gate-v1 §3.9, prd:contract-gate/FR-2). Read-only; returns nil when
// nothing is stale. NR-2: the flag's set-membership behavior is unchanged — this
// only additionally names the specific refs a consumer that wants them can read.
func StaleServeAnchors(e *IndexEntry, entries map[string]*IndexEntry, env Env) []string {
	var out []string
	for _, ref := range e.Serves {
		if strings.HasPrefix(ref, "design:") {
			if t, ok := entries[refSlug(ref)]; ok && t.Status == "superseded" {
				out = append(out, ref)
			}
		}
		if strings.HasPrefix(ref, "prd:") {
			rest := strings.TrimPrefix(ref, "prd:")
			feature, unit, _ := strings.Cut(rest, "/")
			if units, ok := env.PRDUnits[feature]; ok {
				if status, ok := units[unit]; ok && status != "live" {
					out = append(out, ref)
				}
			}
		}
	}
	return out
}

// StaleBuildsOnAnchors returns the Builds-on slugs of a live design that point
// at a superseded design — the `stale-builds-on` staleness truth, shared by the
// index flag and the currency check the same way as StaleServeAnchors
// (prd:contract-gate/FR-2). Read-only; nil when nothing is stale.
func StaleBuildsOnAnchors(e *IndexEntry, entries map[string]*IndexEntry) []string {
	var out []string
	for _, bo := range e.BuildsOn {
		if t, ok := entries[bo]; ok && t.Status == "superseded" {
			out = append(out, bo)
		}
	}
	return out
}

// FeatureUnserved returns the feature's live requirement anchors that no live
// design serves — GenCoverage's requirement-universe↔live-serves join
// (computeCoverage) sliced to the `prd:<feature>` family (contract-gate-v1
// §3.9, prd:contract-gate/FR-3, UB-5). It runs the SAME join the tree-wide
// coverage view runs (never a fork, NR-2) and returns only the feature's
// unserved anchors, sorted; a feature with live units but no live serving design
// yields all of them (UB-5 — never vacuously clean). `inverse` is the live
// serves-inverse from LiveGraph/computeGraph; anchors returned are the full
// `prd:<feature>/<unit>` tokens. Read-only.
func FeatureUnserved(env Env, inverse map[string][]string, feature string) []string {
	cov := computeCoverage(env, inverse)
	return cov.Unserved["prd:"+feature]
}

// GenIndex computes the tech-design index (FR-12) — design-plane data only.
// The requirements-coverage join is computed on read (`docgate coverage`,
// FR-28), never committed here.
func GenIndex(designs []*Design, env Env) (indexMD, indexJSON []byte, defects []schema.Defect) {
	entries, inverse, defects := computeGraph(designs, env)
	out, err := json.MarshalIndent(indexDoc{
		SchemaVersion: "2", GeneratedBy: "docgate", ToolVersion: schema.ToolVersion,
		Designs: entries, ServesInverse: inverse,
	}, "", "  ")
	if err != nil {
		return nil, nil, append(defects, idef("index.json", "marshal failed: "+err.Error(), "report a docgate bug"))
	}
	indexJSON = append(out, '\n')

	var b strings.Builder
	b.WriteString("# Tech-design index *(generated)*\n\n---\n\n")
	b.WriteString("**GENERATED — DO NOT EDIT.** Sole writer: `docgate gen-index`. Liveness is computed from `Supersedes` edges (DINV-3); byte-verified by `verify-index`. Tool version (recorded): `" + schema.ToolVersion + "`.\n\n---\n\n")
	b.WriteString("| Design | Status | Serves | Builds-on | Flags |\n|---|---|---|---|---|\n")
	var slugs []string
	for s := range entries {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	for _, s := range slugs {
		e := entries[s]
		status := e.Status
		if e.SupersededBy != nil {
			status += " → " + *e.SupersededBy
		}
		b.WriteString("| `" + s + "` | " + status + " | " + cellList(e.Serves) + " | " + cellList(e.BuildsOn) + " | " + cellList(e.Flags) + " |\n")
	}
	indexMD = schema.Normalize([]byte(b.String()))
	return indexMD, indexJSON, defects
}

// LiveGraph exposes the computed design liveness graph and the live
// serves-inverse for a read-only consumer (the contract gate's contract
// resolution, design contract-gate-v1 §3.1/§3.9) — the same single liveness
// truth gen-index and coverage share (AP-11), so the gate never re-derives
// liveness or re-parses a design body. entries maps slug → its computed state
// (Status, Serves, Flags, …); inverse maps a Serves ref → the sorted live
// design slugs that serve it. Committed nowhere; computes, writes nothing.
func LiveGraph(designs []*Design, env Env) (entries map[string]*IndexEntry, inverse map[string][]string, defects []schema.Defect) {
	return computeGraph(designs, env)
}

// GenCoverage computes the requirements-coverage view on read (FR-28): the
// universe (corpus IDs + live ledger units) joined against live designs'
// serves. Committed nowhere — the join is cross-plane, so gen-index stopped
// embodying it (coverage-on-read supersedes index-coverage).
func GenCoverage(designs []*Design, env Env) (coverageMD, coverageJSON []byte, defects []schema.Defect) {
	_, inverse, defects := computeGraph(designs, env)
	cov := computeCoverage(env, inverse)
	out, err := json.MarshalIndent(cov, "", "  ")
	if err != nil {
		return nil, nil, append(defects, idef("coverage", "marshal failed: "+err.Error(), "report a docgate bug"))
	}
	coverageJSON = append(out, '\n')
	var b strings.Builder
	writeCoverageMD(&b, cov)
	coverageMD = schema.Normalize([]byte(b.String()))
	return coverageMD, coverageJSON, defects
}

// computeCoverage joins the requirement universe against the live inverse
// map (FR-28). Families: the corpus prefix, or "prd:<feature>".
func computeCoverage(env Env, inverse map[string][]string) *coverageDoc {
	c := &coverageDoc{Served: map[string][]string{}, Unserved: map[string][]string{}}
	add := func(anchor, family string) {
		c.UniverseSize++
		if slugs := inverse[anchor]; len(slugs) > 0 {
			c.Served[anchor] = slugs
		} else {
			c.Unserved[family] = append(c.Unserved[family], anchor)
		}
	}
	for id := range env.CorpusIDs {
		add("corpus:"+id, strings.SplitN(id, "-", 2)[0])
	}
	for feature, units := range env.PRDUnits {
		for unit, status := range units {
			if status == "live" {
				add("prd:"+feature+"/"+unit, "prd:"+feature)
			}
		}
	}
	for f := range c.Unserved {
		sortAnchors(c.Unserved[f])
	}
	return c
}

// sortAnchors orders anchors family-then-numeric for deterministic output.
func sortAnchors(xs []string) {
	num := func(s string) int {
		i := len(s)
		for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
			i--
		}
		var n int
		fmt.Sscanf(s[i:], "%d", &n)
		return n
	}
	stem := func(s string) string {
		i := len(s)
		for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
			i--
		}
		return s[:i]
	}
	sort.Slice(xs, func(a, b int) bool {
		if stem(xs[a]) != stem(xs[b]) {
			return stem(xs[a]) < stem(xs[b])
		}
		return num(xs[a]) < num(xs[b])
	})
}

func writeCoverageMD(b *strings.Builder, c *coverageDoc) {
	b.WriteString("# Requirements coverage *(computed on read — `docgate coverage`, FR-28)*\n\n")
	fmt.Fprintf(b, "Universe: **%d** anchors (corpus IDs + live ledger units) · served: **%d** · unserved: **%d**.\n\n",
		c.UniverseSize, len(c.Served), c.UniverseSize-len(c.Served))
	b.WriteString("| Family | Total | Served | Unserved |\n|---|---|---|---|\n")
	totals := map[string]int{}
	served := map[string]int{}
	family := func(anchor string) string {
		if strings.HasPrefix(anchor, "prd:") {
			return anchor[:strings.LastIndex(anchor, "/")]
		}
		return strings.SplitN(strings.TrimPrefix(anchor, "corpus:"), "-", 2)[0]
	}
	for a := range c.Served {
		totals[family(a)]++
		served[family(a)]++
	}
	for f, xs := range c.Unserved {
		totals[f] += len(xs)
	}
	var fams []string
	for f := range totals {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	for _, f := range fams {
		fmt.Fprintf(b, "| %s | %d | %d | %d |\n", f, totals[f], served[f], totals[f]-served[f])
	}
	b.WriteString("\nUnserved anchors (unserved ≠ work-now — some areas are engine-era by intent):\n\n")
	var ufams []string
	for f := range c.Unserved {
		ufams = append(ufams, f)
	}
	sort.Strings(ufams)
	for _, f := range ufams {
		b.WriteString("- **" + f + "**: " + strings.Join(c.Unserved[f], ", ") + "\n")
	}
	if len(ufams) == 0 {
		b.WriteString("- none — every anchor is served by a live design\n")
	}
}

func cellList(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	return strings.Join(xs, " · ")
}

func appendOnce(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
