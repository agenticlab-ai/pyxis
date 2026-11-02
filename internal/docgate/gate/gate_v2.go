// docgate v2 verbs (docs/tech-design/docgate-v2.md; PRD FR-11..FR-15):
// the design-class lint + index, and the four classes issue #37 found
// unguarded. Same DINV-3 discipline: verbs never write; cmd writes the
// two index artifacts from GenIndex output only.
package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func vdef(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// LoadDesigns reads every design document (index files excluded).
func LoadDesigns(root string, cfg config.Config) ([]*techdesigns.Design, []schema.Defect) {
	dir := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns))
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, []schema.Defect{vdef("lint-tech-designs", cfg.TechDesigns, "read", err.Error(), "the design directory must be readable")}
	}
	var designs []*techdesigns.Design
	var defects []schema.Defect
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "index.") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			defects = append(defects, vdef("lint-tech-designs", name, "read", err.Error(), "the design must be readable"))
			continue
		}
		d, ddef := techdesigns.Parse(strings.TrimSuffix(name, ".md"), b)
		defects = append(defects, ddef...)
		if d != nil {
			designs = append(designs, d)
		}
	}
	return designs, defects
}

// designEnv assembles resolution inputs: grandfather set, ledger source
// maps (unit → status), the corpus-ID universe, the new-design set (when a
// merge base is given), and a repo-relative existence check.
func designEnv(root, mergeBase, head string, cfg config.Config) techdesigns.Env {
	env := techdesigns.Env{
		Grandfathered:           map[string]bool{},
		PrecedenceGrandfathered: map[string]bool{},
		PRDUnits:                map[string]map[string]string{},
		CorpusIDs:               CorpusIDs(root, cfg),
		NewDesigns:              map[string]bool{},
	}
	for _, s := range cfg.Grandfathered {
		env.Grandfathered[s] = true
	}
	for _, s := range cfg.PrecedenceGrandfathered {
		env.PrecedenceGrandfathered[s] = true
	}
	if dirs, err := LedgerDirs(root, cfg); err == nil {
		for _, dir := range dirs {
			b, err := os.ReadFile(filepath.Join(dir, "effective.source.json"))
			if err != nil {
				continue
			}
			var src struct {
				Units map[string]struct {
					Status string `json:"status"`
				} `json:"units"`
			}
			if json.Unmarshal(b, &src) != nil {
				continue
			}
			units := map[string]string{}
			for id, u := range src.Units {
				units[id] = u.Status
			}
			env.PRDUnits[filepath.Base(dir)] = units
		}
	}
	if mergeBase != "" {
		args := append([]string{"-C", root, "diff", "--name-status"}, diffSpan(mergeBase, head)...)
		args = append(args, "--", filepath.FromSlash(cfg.TechDesigns))
		out, err := exec.Command("git", args...).Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				parts := strings.Fields(line)
				if len(parts) < 2 || parts[0][0] != 'A' {
					continue
				}
				base := filepath.Base(parts[len(parts)-1])
				if strings.HasSuffix(base, ".md") && !strings.HasPrefix(base, "index") {
					env.NewDesigns[strings.TrimSuffix(base, ".md")] = true
				}
			}
		}
	}
	env.DocExists = func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil
	}
	return env
}

// CorpusIDs scans the flat placeholder corpus for baseline requirement IDs
// — the one extractor FR-20 resolution and FR-26 coverage share (AP-11).
// Read-only pattern extraction: no structure is imposed on the corpus.
func CorpusIDs(root string, cfg config.Config) map[string]bool {
	ids := map[string]bool{}
	dir := filepath.Join(root, filepath.FromSlash(cfg.PRDs))
	items, err := os.ReadDir(dir)
	if err != nil {
		return ids
	}
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(it.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, it.Name()))
		if err != nil {
			continue
		}
		for _, m := range schema.CorpusIDRe.FindAllString(string(b), -1) {
			ids[m] = true
		}
	}
	return ids
}

// LoadShapes reads every solution-shape document from the shapes corpus
// (index files excluded). An absent corpus is not a defect — there are no
// shapes until a stateful design pairs one (§3.1).
func LoadShapes(root string, cfg config.Config) ([]*shapes.Shape, []schema.Defect) {
	dir := filepath.Join(root, filepath.FromSlash(cfg.SolutionShapes))
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var out []*shapes.Shape
	var defects []schema.Defect
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "index.") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			defects = append(defects, vdef("solution-shape", name, "read", err.Error(), "the shape must be readable"))
			continue
		}
		s, sdef := shapes.Parse(strings.TrimSuffix(name, ".md"), b)
		defects = append(defects, sdef...)
		if s != nil {
			out = append(out, s)
		}
	}
	return out, defects
}

// GenIndexBytes computes the index artifacts (FR-12) — design-plane data
// only; the coverage join moved to the on-read `coverage` verb (FR-28).
func GenIndexBytes(root string, cfg config.Config) (md, js []byte, defects []schema.Defect) {
	designs, defects := LoadDesigns(root, cfg)
	env := designEnv(root, "", "", cfg)
	md, js, gdef := techdesigns.GenIndex(designs, env)
	return md, js, append(defects, gdef...)
}

// Coverage computes the requirements-coverage view on read (FR-28) — the
// cross-plane join is never committed (coverage-on-read supersedes
// index-coverage). Mirrors GenIndexBytes but writes nothing.
func Coverage(root string, cfg config.Config) (md, js []byte, defects []schema.Defect) {
	designs, defects := LoadDesigns(root, cfg)
	env := designEnv(root, "", "", cfg)
	cmd, cjs, cdef := techdesigns.GenCoverage(designs, env)
	return cmd, cjs, append(defects, cdef...)
}

var headerRowRe = regexp.MustCompile(`^\|\s*\*\*(.+?)\*\*\s*\|`)

func headerRows(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rows := map[string]string{}
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		if m := headerRowRe.FindStringSubmatch(l); m != nil {
			cells := strings.Split(strings.Trim(l, "|"), "|")
			if len(cells) >= 2 {
				rows[m[1]] = strings.TrimSpace(cells[1])
			}
		}
	}
	return rows, nil
}

var dateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// governingPaths returns the trust root (AGENTS.md rule 2) — the abstract
// governing layer and the citable principle registry — resolved from the
// configured governing-doc paths, never a fixed literal list (config-driven-
// portability-v1 §3.2, FR-1/FR-3; DINV-1). On a default config the two members
// are byte-identical to the former literals (DINV-2).
func governingPaths(cfg config.Config) []string {
	return []string{cfg.ProductVision, cfg.ArchitecturePrinciples}
}

// hasOperatorTrailer reports whether any commit in mergeBase..HEAD carries
// the Governing-Change attestation (one helper, two callers — FR-18/FR-23).
func hasOperatorTrailer(root, mergeBase string) bool {
	logOut, err := exec.Command("git", "-C", root, "log", "--format=%B", mergeBase+"..HEAD").Output()
	return err == nil && strings.Contains(string(logOut), "Governing-Change: operator")
}

// LintIncidentBytes runs the FR-25 shape checks over registry bytes — the
// one shape predicate, shared by the gate and the writer's self-check
// (incident-machinery DINV-3: one schema definition, all consumers).
func LintIncidentBytes(rel string, b []byte) []schema.Defect {
	var defects []schema.Defect
	lines := strings.Split(string(schema.Normalize(b)), "\n")
	fenced := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(l, "**I") {
			continue
		}
		if schema.IncidentIDRe.FindStringSubmatch(l) == nil {
			defects = append(defects, vdef("lint-incidents", rel, "entry",
				fmt.Sprintf("line %d: malformed entry head (want **I<n> (YYYY-MM-DD) [tags]**)", i+1), "follow the registry format"))
		}
	}
	entries := incidentEntries(b)
	var ids []int
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, n := range ids {
		block := entries[n]
		head := schema.IncidentIDRe.FindStringSubmatch(block[0])
		if head != nil {
			// Tags: bracketed, non-empty, comma-separated kebab-case tokens
			// — the retrieval key, checked as content (FR-25).
			for _, tag := range strings.Split(head[3], ",") {
				tag = strings.TrimSpace(tag)
				if tag == "" || !schema.IncidentTagRe.MatchString(tag) {
					defects = append(defects, vdef("lint-incidents", rel, "entry",
						fmt.Sprintf("I%d tag %q is not a kebab-case token", n, tag),
						"tags are the grep key: lowercase kebab-case, comma-separated"))
				}
			}
		}
		// The four named fields, each as a `- **<Field>:**`-anchored line.
		for _, f := range schema.IncidentFields {
			anchored := false
			for _, bl := range block {
				if strings.HasPrefix(strings.TrimSpace(bl), "- **"+f+":**") {
					anchored = true
					break
				}
			}
			if !anchored {
				defects = append(defects, vdef("lint-incidents", rel, "entry",
					fmt.Sprintf("I%d missing field %s as a `- **%s:**` line", n, f, f),
					"entries carry the four named fields as anchored lines"))
			}
		}
	}
	for i, n := range ids {
		if n != i+1 {
			defects = append(defects, vdef("lint-incidents", rel, "entry",
				fmt.Sprintf("IDs not dense: found I%d at position %d", n, i+1), "IDs are I1…In, never renumbered, never gapped"))
			break
		}
	}
	return defects
}

// incidentEntries slices the registry into ID → entry block (the head
// line through the last body line before the next head/heading/EOF).
func incidentEntries(b []byte) map[int][]string {
	entries := map[int][]string{}
	lines := strings.Split(string(schema.Normalize(b)), "\n")
	fenced := false
	cur, start := -1, 0
	flush := func(end int) {
		if cur >= 0 {
			block := lines[start:end]
			for len(block) > 0 && strings.TrimSpace(block[len(block)-1]) == "" {
				block = block[:len(block)-1]
			}
			entries[cur] = block
		}
		cur = -1
	}
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := schema.IncidentIDRe.FindStringSubmatch(l); m != nil {
			flush(i)
			cur, _ = strconv.Atoi(m[1])
			start = i
			continue
		}
		if strings.HasPrefix(l, "#") {
			flush(i)
		}
	}
	flush(len(lines))
	return entries
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
