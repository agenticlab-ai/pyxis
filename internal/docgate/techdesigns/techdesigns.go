// Package techdesigns implements docgate v2's design-class checks
// (docs/tech-design/docgate-v2.md; PRD FR-11/FR-12): the closed header
// lint, typed Serves grammar with enumerated refs only (the operator's
// Q1 ruling — range shorthand is a defect), mandatory-by-name
// subsections, and the generated tech-design index whose liveness is
// computed from Supersedes edges, never written (DINV-3). Pure like the
// fold: all inputs are passed in; the gate package does the IO.
package techdesigns

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Design is one parsed design document.
type Design struct {
	Slug   string
	Doc    *ledger.Doc
	Header map[string]string
}

// Env is everything resolution needs, supplied by the caller.
type Env struct {
	Grandfathered map[string]bool
	// PrecedenceGrandfathered: exempt from FR-16's precedence clause
	// ONLY (I1); every other check still applies.
	PrecedenceGrandfathered map[string]bool
	// PRDUnits: feature → unit ID → status (from each ledger's source map).
	PRDUnits map[string]map[string]string
	// CorpusIDs: the requirement IDs actually present in the flat
	// placeholder corpus — the corpus: resolution universe (FR-20).
	CorpusIDs map[string]bool
	// NewDesigns: slugs added since the merge base; empty without one.
	// Only these are held to liveness-at-introduction (FR-20; DINV-2 of
	// the serves-hardening design: frozen documents are never retro-red).
	NewDesigns map[string]bool
	// DocExists reports whether a repo-relative path exists.
	DocExists func(string) bool
}

// HeaderFields is the closed design header schema (conventions §3.1).
var HeaderFields = []string{
	"Status", "Owner", "Approver(s)", "Date", "Relevant-when", "Scope",
	"Tracking", "Builds-on", "Supersedes", "Serves",
	"Security/privacy impact", "Template version",
}

const TemplateVersion = "tech-design-template/v1"

var (
	slugRe   = regexp.MustCompile(`^[a-z0-9-]+$`)
	servesRe = regexp.MustCompile(`^(prd:[a-z0-9-]+/[A-Z]+-?[0-9]+|corpus:[A-Z]+-[0-9]+|design:[a-z0-9-]+(#\S+)?|doc:\S+)$`)
	// reqAnchorRe matches a requirement-row anchor — FR/UB/NFR — the only
	// prd: refs that satisfy PRD-precedes-design (FR-16; incident I1).
	reqAnchorRe = regexp.MustCompile(`^prd:[a-z0-9-]+/(FR|UB|NFR)-?[0-9]+$`)
)

func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "lint-tech-designs", File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Parse reads one design document (normalizing internally).
func Parse(slug string, b []byte) (*Design, []schema.Defect) {
	doc, defects := parseDoc(slug+".md", schema.Normalize(b))
	if doc == nil {
		return nil, defects
	}
	return &Design{Slug: slug, Doc: doc, Header: doc.Header}, nil
}

// parseDoc reuses the ledger's base parser but drops its PRD-specific
// template-version defect — designs pin their own template.
func parseDoc(name string, normalized []byte) (*ledger.Doc, []schema.Defect) {
	doc, defects := ledger.ParseBase(name, normalized)
	kept := defects[:0]
	for _, d := range defects {
		if !strings.Contains(d.Reason, "Template version") {
			kept = append(kept, d)
		}
	}
	return doc, kept
}

// Lint runs FR-11 over a set of designs.
func Lint(designs []*Design, env Env) []schema.Defect {
	var defects []schema.Defect
	slugs := map[string]bool{}
	for _, d := range designs {
		slugs[d.Slug] = true
	}
	for _, d := range designs {
		file := d.Slug + ".md"
		if !slugRe.MatchString(d.Slug) {
			defects = append(defects, def(file, "slug", "slug is not [a-z0-9-]+", "rename the file; slugs are permanent"))
		}
		if env.Grandfathered[d.Slug] {
			continue // indexed, never linted (design R1)
		}
		// Closed header.
		for _, f := range HeaderFields {
			if _, ok := d.Header[f]; !ok {
				defects = append(defects, def(file, "header",
					fmt.Sprintf("missing header field %q", f), "the design header schema is closed: all twelve fields"))
			}
		}
		for f := range d.Header {
			if !contains(HeaderFields, f) {
				defects = append(defects, def(file, "header",
					fmt.Sprintf("unknown header field %q", f), "the design header schema is closed: remove the field"))
			}
		}
		if v := d.Header["Template version"]; v != "" && !strings.Contains(v, TemplateVersion) {
			defects = append(defects, def(file, "header",
				fmt.Sprintf("Template version %q is not %s", v, TemplateVersion), "pin the template the document instantiates"))
		}
		// FR-20: a design's Status records Approved — the amendment model's
		// merge-target rule applied to the design class (conventions §3.1).
		if s := d.Header["Status"]; s != "" && !strings.Contains(s, "Approved") {
			defects = append(defects, def(file, "header",
				fmt.Sprintf("Status %q does not record Approved", s),
				"flip Status to Approved when review passes — a merged design never carries Draft"))
		}
		// Serves grammar + resolution (enumerated refs only).
		serves := strings.TrimSpace(d.Header["Serves"])
		if serves != "" && serves != "—" {
			prdRefs, reqRefs := 0, 0
			for _, ref := range splitRefs(serves) {
				if strings.HasPrefix(ref, "prd:") {
					prdRefs++
					if reqAnchorRe.MatchString(ref) {
						reqRefs++
					}
				}
				if strings.Contains(ref, "..") {
					defects = append(defects, def(file, "serves",
						fmt.Sprintf("range shorthand %q — refs are enumerated, one per anchor", ref),
						"list each anchor explicitly (operator ruling, amendment 003)"))
					continue
				}
				if !servesRe.MatchString(ref) {
					defects = append(defects, def(file, "serves",
						fmt.Sprintf("ref %q does not match the typed grammar", ref),
						"prd:<feature>/<ID> · corpus:<ID> · design:<slug>[#anchor] · doc:<path> · —"))
					continue
				}
				defects = append(defects, resolveRef(file, "serves", ref, slugs, env)...)
				// FR-20: liveness at introduction — a design added since the
				// merge base must not anchor superseded or retired units.
				if env.NewDesigns[d.Slug] && strings.HasPrefix(ref, "prd:") {
					feature, unit, _ := strings.Cut(strings.TrimPrefix(ref, "prd:"), "/")
					if status, ok := env.PRDUnits[feature][unit]; ok && status != "live" {
						defects = append(defects, def(file, "serves",
							fmt.Sprintf("ref %q anchors a unit with status %q — new designs serve live requirements only", ref, status),
							"anchor the successor unit; dead intent never anchors new work (FR-20)"))
					}
				}
			}
			// FR-16: requirement precedence — a stub anchor is not a requirement.
			if prdRefs > 0 && reqRefs == 0 && !env.PrecedenceGrandfathered[d.Slug] {
				defects = append(defects, def(file, "serves",
					"Serves cites a prd: ledger with no requirement-row anchor (FR/UB/NFR) — a waiting-room, non-goal, or question anchor does not satisfy PRD-precedes-design (incident I1)",
					"anchor at least one committed FR/UB/NFR row; requirements precede designs, never back-filled"))
			}
		}
		// Builds-on / Supersedes resolution.
		for _, field := range []string{"Builds-on", "Supersedes"} {
			v := strings.TrimSpace(d.Header[field])
			if v == "" || v == "—" {
				continue
			}
			for _, ref := range splitRefs(v) {
				target := refSlug(ref)
				if !slugs[target] {
					defects = append(defects, def(file, strings.ToLower(field),
						fmt.Sprintf("target %q does not resolve to a design", ref), "name an existing docs/tech-design/<slug>.md"))
				}
			}
		}
		// Mandatory-by-name subsections + the security pair.
		obs, sec, secBody := findMandatory(d.Doc)
		if !obs {
			defects = append(defects, def(file, "sections", "no Observability subsection", "§3 must contain it by name, with substance or `none — <reason>`"))
		}
		if !sec {
			defects = append(defects, def(file, "sections", "no Security & privacy subsection", "§3 must contain it by name, paired with the header row"))
		} else {
			headerNone := strings.HasPrefix(strings.ToLower(strings.TrimSpace(d.Header["Security/privacy impact"])), "none")
			bodyNone := strings.HasPrefix(strings.ToLower(strings.TrimSpace(secBody)), "none")
			if headerNone != bodyNone {
				defects = append(defects, def(file, "sections",
					"Security/privacy header row and §Security & privacy subsection disagree on `none`", "the pair states one truth"))
			}
		}
	}
	return defects
}

// splitRefs splits a header value on the interpunct and strips any
// trailing parenthetical gloss — the gloss is human margin; the ref is
// the machine part (the same identity-modulo-gloss rule v1 applies to
// template column names). Backticks and post-ref prose (grandfathered
// free-text Builds-on rows) are stripped the same way: the ref is the
// first token, cleanly (#54 E3 — no more garbled index edges).
func splitRefs(v string) []string {
	var out []string
	for _, r := range strings.Split(v, "·") {
		r = strings.Trim(strings.TrimSpace(r), "`")
		if i := strings.Index(r, " ("); i > 0 {
			r = strings.TrimSpace(r[:i])
		}
		if f := strings.Fields(r); len(f) > 0 {
			r = strings.Trim(f[0], "`")
		} else {
			r = ""
		}
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

// refSlug extracts a design slug from a Builds-on/Supersedes entry
// (accepts a bare slug, docs/tech-design/<slug>.md, or design:<slug>).
func refSlug(ref string) string {
	ref = strings.TrimPrefix(ref, "design:")
	ref = strings.TrimSuffix(ref, ".md")
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.Index(ref, "#"); i >= 0 {
		ref = ref[:i]
	}
	return ref
}

func resolveRef(file, op, ref string, slugs map[string]bool, env Env) []schema.Defect {
	switch {
	case strings.HasPrefix(ref, "prd:"):
		rest := strings.TrimPrefix(ref, "prd:")
		feature, unit, _ := strings.Cut(rest, "/")
		units, ok := env.PRDUnits[feature]
		if !ok {
			return []schema.Defect{def(file, op, fmt.Sprintf("ref %q: no ledger for feature %q", ref, feature), "reference an existing docs/prds/<feature>/ ledger")}
		}
		if _, ok := units[unit]; !ok {
			return []schema.Defect{def(file, op, fmt.Sprintf("ref %q: unit %s not in the ledger's source map", ref, unit), "reference an existing anchored unit")}
		}
	case strings.HasPrefix(ref, "design:"):
		if !slugs[refSlug(ref)] {
			return []schema.Defect{def(file, op, fmt.Sprintf("ref %q does not resolve", ref), "name an existing design slug")}
		}
	case strings.HasPrefix(ref, "doc:"):
		path := strings.TrimPrefix(ref, "doc:")
		if i := strings.Index(path, "#"); i >= 0 {
			path = path[:i]
		}
		if env.DocExists != nil && !env.DocExists(path) {
			return []schema.Defect{def(file, op, fmt.Sprintf("ref %q: path does not exist", ref), "point at a real repo path")}
		}
	case strings.HasPrefix(ref, "corpus:"):
		// FR-20: corpus refs resolve against the IDs actually present in
		// the flat placeholder corpus — "by review" ended with #54 A4.
		if env.CorpusIDs != nil && !env.CorpusIDs[strings.TrimPrefix(ref, "corpus:")] {
			return []schema.Defect{def(file, op,
				fmt.Sprintf("ref %q: ID not present in the placeholder corpus", ref),
				"cite an ID that exists in a flat docs/prds/*.md corpus file")}
		}
	}
	return nil
}

func findMandatory(doc *ledger.Doc) (obs, sec bool, secBody string) {
	for i, s := range doc.Sections {
		title := headingTitle(doc.Lines[s.Start])
		if strings.Contains(title, "Observability") {
			obs = true
		}
		if strings.Contains(title, "Security & privacy") {
			sec = true
			for _, l := range doc.Lines[s.Start+1 : s.End] {
				if strings.TrimSpace(l) != "" {
					secBody = l
					break
				}
			}
		}
		_ = i
	}
	return
}

func headingTitle(line string) string {
	return strings.TrimLeft(line, "# ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// sortedSlugs returns design slugs in stable order.
func sortedSlugs(designs []*Design) []string {
	out := make([]string, len(designs))
	for i, d := range designs {
		out[i] = d.Slug
	}
	sort.Strings(out)
	return out
}
