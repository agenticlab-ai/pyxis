// The fold-time gate suite (plan-new-v1 §3.6): every structural rule the
// PRD binds — input resolution (FR-1/FR-2/UB-2), sizing as data (FR-5/FR-6),
// mechanical quarantine (FR-7), expand–contract shape (FR-8), dependency
// graph + cycles (FR-10/UB-3), overlap sequencing (FR-11), charter
// completeness + durability lint (FR-12/FR-13), coverage and all-or-nothing
// fog (FR-15/FR-16 — an unmapped requirement IS the mechanical fog signal),
// and the DINV-8 bucket rule. Verdicts, never prompt discipline (AP-10).
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

var (
	prdRefRe    = regexp.MustCompile(`^prd:([a-z0-9-]+)/([A-Z]+-?\d+)$`)
	designRefRe = regexp.MustCompile(`^design:([a-z0-9-]+)(#[A-Za-z0-9.-]+)?$`)
	corpusRefRe = regexp.MustCompile(`^corpus:((?:SCAF|PLAN|IMPL|GATE|FLEET|IDEN|GOV|KNOW|OBS|BENCH|VNEXT)-\d+)$`)
	fileNameRe  = regexp.MustCompile(`^(\d{3})-plan\.md$`)
	planNameRe  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	lockBasisRe = regexp.MustCompile(`^lock/([a-z][a-z0-9-]*)/v[1-9][0-9]*$`)

	// Durability lint (FR-13): charters describe behavior and interfaces —
	// never file paths or line numbers. Conservative patterns: a
	// code-extension path, a two-or-more-slash path, or a line reference.
	pathExtRe  = regexp.MustCompile(`[\w.-]*/[\w.-]+\.(go|md|sh|ya?ml|json|mjs|js|ts|py)\b`)
	deepPathRe = regexp.MustCompile(`[\w.-]+/[\w.-]+/[\w.-]+`)
	lineRefRe  = regexp.MustCompile(`\bline \d+`)
	urlishRe   = regexp.MustCompile(`\w+://`)
	// Vagueness blacklist (prd-template Appendix A), acceptance rows only —
	// an undecidable criterion is not a criterion.
	vagueRe = regexp.MustCompile(`(?i)\b(fast|easy|user[- ]friendly|intuitive|seamless|robust|significant|minimal|better|quicker|best|as appropriate|if possible|etc)\b`)
)

// governing is the loaded requirement surface of the plan's PRD.
type governing struct {
	live     map[string]bool // live unit IDs
	must     map[string]bool // live FR/UB rows with priority Must
	optional map[string]bool // live FR/UB rows with priority Should/Could
}

// Validate runs the full gate suite. Defects name document, rule, reason,
// and remedy (NFR-4's quadruple); an empty return is the fold-green verdict.
func Validate(root string, cfg config.Config, d *Doc) []schema.Defect {
	var defects []schema.Defect
	file := d.sourceFile
	bad := func(op, reason, remedy string) {
		defects = append(defects, def(file, op, reason, remedy))
	}

	// ---- header + environment (FR-1's mechanical half, DINV-8) ----
	if m := fileNameRe.FindStringSubmatch(filepath.Base(file)); m == nil {
		bad("header", fmt.Sprintf("filename %q is not NNN-plan.md", filepath.Base(file)), "plans are dense-numbered: 001-plan.md, 002-plan.md, …")
	} else if m[1] != d.Version {
		bad("header", fmt.Sprintf("Plan version %q does not match filename number %q", d.Version, m[1]), "the header version and the filename agree")
	}
	if !planNameRe.MatchString(d.Name()) || filepath.Base(filepath.Dir(file)) != d.Name() {
		bad("header", fmt.Sprintf("plan name %q does not match the plan directory", d.Name()), "Feature or Intake is a slug matching the plan directory")
	}
	if d.Intake == "" {
		if base := filepath.Base(d.PRD); base != d.Feature {
			bad("header", fmt.Sprintf("Feature %q does not match the PRD directory %q", d.Feature, d.PRD), "the feature slug is the PRD's name")
		}
		if strings.HasPrefix(d.PRD, "prd/") {
			if d.PRD != "prd/"+d.Feature || d.Design != "design/"+d.Feature || !lockBasisRe.MatchString(d.Basis) || !strings.HasPrefix(d.Basis, "lock/"+d.Feature+"/") {
				bad("header", "draft-born plan has invalid memory subjects or lock Basis", "use PRD prd/<feature>, Design design/<feature>, and Basis lock/<feature>/vN")
			}
		} else if d.Basis != "" {
			bad("header", "document-pipeline plan carries a lock Basis", "omit Basis for document-pipeline plans")
		}
	} else if d.Feature != "" || d.PRD != "" || d.Design != "" || d.Basis != "" {
		bad("header", "non-feature plan carries feature-only fields", "use Intake alone to name a non-feature plan")
	}
	if !Dialects[d.Dialect] {
		bad("header", fmt.Sprintf("unknown dialect %q", d.Dialect), "known dialects: "+DialectHarbormaster+", "+DialectNative)
	}
	if (d.Schema == NativeSchemaVersion) != (d.Dialect == DialectNative) {
		bad("header", "plan schema and task-spec dialect do not match", "use fs-spec:v2 with plan-template/v2; fs-spec:v1 is for legacy plans")
	}
	if d.TemplateVersion != PlanTemplateVersion && d.TemplateVersion != NativeTemplateVersion {
		bad("header", fmt.Sprintf("template version %q unsupported", d.TemplateVersion), "this build parses "+PlanTemplateVersion)
	}
	if d.EpicTitle == "" {
		bad("header", "empty Epic title", "the epic title is the feature's tracker face (design §3.8)")
	}
	if len(d.Tasks) == 0 {
		bad("tasks", "a plan with zero tasks", "a plan exists to make work dispatchable; zero tasks is a fog report, not a plan")
	}

	var gov governing
	var designBody []byte
	var locked *lockedInputs
	if d.Intake == "" {
		var gdef, ddef []schema.Defect
		if d.PRD == "prd/"+d.Feature && d.Design == "design/"+d.Feature && lockBasisRe.MatchString(d.Basis) && strings.HasPrefix(d.Basis, "lock/"+d.Feature+"/") {
			locked, gdef = loadLocked(root, d)
			if locked != nil {
				gov, designBody = locked.gov, locked.designBody
			}
			defects = append(defects, gdef...)
		} else {
			gov, gdef = loadGoverning(root, d.PRD, file)
			defects = append(defects, gdef...)
			designBody, ddef = loadDesign(root, d.Design, file)
			defects = append(defects, ddef...)
		}
	}
	corpus := loadCorpus(root, cfg.PRDs)

	// ---- the dependency graph (FR-10, UB-3) ----
	ids := map[string]bool{}
	for i := range d.Tasks {
		ids[d.Tasks[i].ID] = true
	}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		for _, dep := range t.BlockedBy {
			if !ids[dep] {
				bad(t.ID, fmt.Sprintf("blocked-by target %q does not exist", dep), "edges reference plan task IDs (T<n>)")
			}
			if dep == t.ID {
				bad(t.ID, "task blocked by itself", "remove the self-edge")
			}
		}
	}
	if cycle := findCycle(d.Tasks); cycle != "" {
		bad("graph", "dependency cycle: "+cycle, "break the cycle — readiness would deadlock (PRD UB-3)")
	} else {
		computeFoundation(d.Tasks)
	}
	reach := transitiveOrder(d.Tasks)

	// ---- the distributed rule suite: per-task rules, then whole-doc rules ----
	// Each rule self-registers from its own reg_*.go and runs in name-sorted
	// order (design DINV-1); this frame carries no rule enumeration to conflict
	// on (DINV-2). The graph/governing/corpus setup above is the shared context.
	env := &Env{Root: root, Cfg: cfg, Doc: d, Reach: reach, Gov: gov, DesignBody: designBody, Corpus: corpus, Locked: locked}
	rules := TaskChecks()
	for i := range d.Tasks {
		t := &d.Tasks[i]
		for _, tc := range rules {
			tc.Run(env, t, bad)
		}
	}
	for _, dc := range DocChecks() {
		dc.Run(env, bad)
	}

	// ---- DINV-8: bucket-1 purity ----
	for n, line := range strings.Split(mustRead(root, file), "\n") {
		if strings.Contains(line, ".pyxis/") {
			bad(fmt.Sprintf("bucket line %d", n+1), "plan content references a `.pyxis/` path", "plans are bucket-1 operand content; cite capability by version pin, never footprint path (DINV-8)")
		}
	}
	return defects
}

// checkRef resolves one trace reference (FR-2/UB-2).
func checkRef(ref, taskID string, designBody []byte, designPath string, corpus map[string]bool, root string, cfg config.Config, locked *lockedInputs, bad func(op, reason, remedy string)) {
	switch {
	case strings.HasPrefix(ref, "prd:"):
		m := prdRefRe.FindStringSubmatch(ref)
		if m == nil {
			bad(taskID, fmt.Sprintf("malformed prd reference %q", ref), "prd:<feature>/<ID>")
			return
		}
		if locked != nil && m[1] == locked.feature {
			if locked.gov.live[m[2]] {
				return
			}
			bad(taskID, fmt.Sprintf("trace %q does not resolve to a live unit at %s", ref, locked.basis), "cite a requirement active in the plan's Basis lock")
			return
		}
		// Slug-based resolution — every ledger (the governing one included)
		// resolves against its own projection, so IDs never collide across
		// ledgers.
		if units := loadUnits(filepath.Join(root, filepath.FromSlash(cfg.PRDs), m[1], "effective.source.json")); units[m[2]] {
			return
		}
		bad(taskID, fmt.Sprintf("trace %q does not resolve to a live unit", ref), "trace references resolve or the task does not validate (PRD UB-2)")
	case strings.HasPrefix(ref, "design:"):
		m := designRefRe.FindStringSubmatch(ref)
		if m == nil {
			bad(taskID, fmt.Sprintf("malformed design reference %q", ref), "design:<slug>[#anchor]")
			return
		}
		if locked != nil && m[1] == locked.feature {
			anchor := strings.TrimPrefix(m[2], "#")
			if anchor == "" || locked.designAnchor(anchor) {
				return
			}
			bad(taskID, fmt.Sprintf("design decision or anchor %q not found at %s", ref, locked.basis), "cite an active decision or anchor in the locked tech spec")
			return
		}
		body := designBody
		if m[1] != strings.TrimSuffix(filepath.Base(designPath), ".md") {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg.TechDesigns), m[1]+".md"))
			if err != nil {
				bad(taskID, fmt.Sprintf("design reference %q: no such design", ref), "trace references resolve (PRD UB-2)")
				return
			}
			body = b
		}
		if m[2] != "" && !anchorIn(body, strings.TrimPrefix(m[2], "#")) {
			bad(taskID, fmt.Sprintf("design anchor %q not found", ref), "cite an existing §-number or DINV/A/R/Q anchor")
		}
	case strings.HasPrefix(ref, "corpus:"):
		m := corpusRefRe.FindStringSubmatch(ref)
		if m == nil || !corpus[m[1]] {
			bad(taskID, fmt.Sprintf("corpus reference %q does not resolve", ref), "corpus IDs live in the flat placeholder PRDs")
		}
	default:
		bad(taskID, fmt.Sprintf("unknown trace reference form %q", ref), "prd: · design: · corpus:")
	}
}

// ---- governing-surface loading ----

func loadGoverning(root, prdDir, file string) (governing, []schema.Defect) {
	gov := governing{}
	srcPath := filepath.Join(root, filepath.FromSlash(prdDir), "effective.source.json")
	gov.live = loadUnits(srcPath)
	if gov.live == nil {
		return gov, []schema.Defect{def(file, "governing", fmt.Sprintf("cannot load the governing projection %s/effective.source.json", prdDir),
			"the plan's PRD must be a folded ledger on this tree (PRD FR-1)")}
	}
	md, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prdDir), "effective.md"))
	if err != nil {
		return gov, []schema.Defect{def(file, "governing", fmt.Sprintf("cannot read %s/effective.md: %v", prdDir, err), "fold the governing ledger")}
	}
	gov.must, gov.optional = map[string]bool{}, map[string]bool{}
	for line := range strings.SplitSeq(string(md), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(t, "|"), "|")
		if len(cells) < 5 {
			continue
		}
		id, prefix, ok := schema.ParseID(cells[0])
		if !ok || !gov.live[id] {
			continue
		}
		var prio string
		switch prefix {
		case "FR":
			prio = strings.TrimSpace(cells[3])
		case "UB":
			if len(cells) > 4 {
				prio = strings.TrimSpace(cells[4])
			}
		default:
			continue
		}
		switch {
		case strings.HasPrefix(prio, "Must"):
			gov.must[id] = true
		case strings.HasPrefix(prio, "Should"), strings.HasPrefix(prio, "Could"):
			gov.optional[id] = true
		}
	}
	return gov, nil
}

// loadUnits reads a projection's live unit IDs; nil when unreadable.
func loadUnits(path string) map[string]bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var src struct {
		Units map[string]struct {
			Status string `json:"status"`
		} `json:"units"`
	}
	if json.Unmarshal(b, &src) != nil || src.Units == nil {
		return nil
	}
	live := map[string]bool{}
	for id, u := range src.Units {
		if u.Status == "live" {
			live[id] = true
		}
	}
	return live
}

func loadDesign(root, designPath, file string) ([]byte, []schema.Defect) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(designPath)))
	if err != nil {
		return nil, []schema.Defect{def(file, "governing", fmt.Sprintf("cannot read the governing design %s: %v", designPath, err),
			"the plan's Design header names an existing technical design (PRD FR-1)")}
	}
	return b, nil
}

// loadCorpus scans the flat placeholder PRDs for baseline IDs.
func loadCorpus(root, prds string) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(prds)))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prds), e.Name()))
		if err != nil {
			continue
		}
		for _, id := range schema.CorpusIDRe.FindAllString(string(b), -1) {
			out[id] = true
		}
	}
	return out
}

// anchorIn resolves a design anchor: a §-number ("3.1") matches a heading;
// a token anchor ("DINV-1", "A-2") matches its literal presence.
func anchorIn(body []byte, anchor string) bool {
	s := string(body)
	if regexp.MustCompile(`^\d+(\.\d+)*$`).MatchString(anchor) {
		re := regexp.MustCompile(`(?m)^#{2,4} ` + regexp.QuoteMeta(anchor) + `[ .]`)
		return re.MatchString(s)
	}
	return strings.Contains(s, anchor)
}

// ---- graph helpers ----

// findCycle DFSes the blocked-by graph; returns "T1 → T2 → T1" or "".
func findCycle(tasks []Task) string {
	adj := map[string][]string{}
	for i := range tasks {
		adj[tasks[i].ID] = tasks[i].BlockedBy
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var found []string
	var dfs func(string) bool
	dfs = func(u string) bool {
		color[u] = gray
		stack = append(stack, u)
		for _, v := range adj[u] {
			if color[v] == gray {
				start := 0
				for i, s := range stack {
					if s == v {
						start = i
						break
					}
				}
				found = append(append([]string{}, stack[start:]...), v)
				return true
			}
			if color[v] == white && dfs(v) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
		return false
	}
	for i := range tasks {
		if color[tasks[i].ID] == white && dfs(tasks[i].ID) {
			return strings.Join(found, " → ")
		}
	}
	return ""
}

// transitiveOrder returns reach[a][b] = true when a transitively depends on b.
func transitiveOrder(tasks []Task) map[string]map[string]bool {
	reach := map[string]map[string]bool{}
	var walk func(id string) map[string]bool
	walk = func(id string) map[string]bool {
		if r, ok := reach[id]; ok {
			return r
		}
		r := map[string]bool{}
		reach[id] = r // cycle-safe: partial map during recursion
		var deps []string
		for i := range tasks {
			if tasks[i].ID == id {
				deps = tasks[i].BlockedBy
			}
		}
		for _, dep := range deps {
			r[dep] = true
			for k := range walk(dep) {
				r[k] = true
			}
		}
		return r
	}
	for i := range tasks {
		walk(tasks[i].ID)
	}
	return reach
}

// computeFoundation derives the plan-time risk stamp (FR-22): high for a
// common root every other task transitively depends on, medium for a task
// with dependents, low for a leaf.
func computeFoundation(tasks []Task) {
	if len(tasks) == 0 {
		return
	}
	reach := transitiveOrder(tasks)
	dependents := map[string]int{}
	for i := range tasks {
		for _, dep := range tasks[i].BlockedBy {
			dependents[dep]++
		}
	}
	for i := range tasks {
		t := &tasks[i]
		common := len(tasks) > 1
		for j := range tasks {
			if tasks[j].ID != t.ID && !reach[tasks[j].ID][t.ID] {
				common = false
				break
			}
		}
		switch {
		case common:
			t.Foundation = Foundation{Level: FoundationRoot, Reason: ReasonRoot}
		case dependents[t.ID] > 0:
			t.Foundation = Foundation{Level: FoundationInternal, Reason: ReasonInternal}
		default:
			t.Foundation = Foundation{Level: FoundationLeaf, Reason: ReasonLeaf}
		}
	}
}

func mustRead(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(b)
}
