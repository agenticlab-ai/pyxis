package readiness

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

// Sources is the pinned mainline resolution universe. Documents use repository
// paths and bytes from the same commit as Mainline; no readiness check reads
// the mutable working tree.
type Sources struct {
	Mainline        *resolve.Projection
	Documents       map[string][]byte
	PRDsRoot        string
	DesignsRoot     string
	BrainstormsRoot string
	// OperatorLogin is the configured approval.operator_login, which a ruling
	// may name as "@<login>" alongside the "operator" role.
	OperatorLogin string
	// LockedVersion is set only when rechecking a contract that has already
	// passed lock preparation. Its own history numbers now exist on mainline.
	LockedVersion bool
}

func (s Sources) roots() (string, string, string) {
	defaults := config.Defaults()
	prd, designs, brainstorms := s.PRDsRoot, s.DesignsRoot, s.BrainstormsRoot
	if prd == "" {
		prd = defaults.PRDs
	}
	if designs == "" {
		designs = defaults.TechDesigns
	}
	if brainstorms == "" {
		brainstorms = defaults.Brainstorms
	}
	return prd, designs, brainstorms
}

func checkLinks(result compile.Result, sources Sources) []Unmet {
	local := map[string]compile.Record{}
	for _, entry := range result.Records {
		if entry.Draft.Family != "" {
			local[entry.Draft.Subject+"#"+entry.Draft.Family] = entry
		}
	}
	var unmet []Unmet
	seen := map[string]bool{}
	add := func(condition, item, remedy string) {
		key := condition + "\x00" + item
		if !seen[key] {
			seen[key] = true
			unmet = append(unmet, Unmet{condition, item, remedy})
		}
	}
	graph := map[string][]string{}
	for _, entry := range result.Records {
		from := entry.Draft.Subject + "#" + entry.Draft.Family
		for _, link := range entry.Links {
			if link.Type != record.Serves && link.Type != record.DependsOn && link.Type != record.Supersedes {
				continue
			}
			target, ok := local[link.Target]
			var kind string
			if ok {
				kind = target.Draft.Kind
			} else if sources.Mainline != nil {
				subject, family, hasFamily := strings.Cut(link.Target, "#")
				if hasFamily {
					if head := sources.Mainline.Head(subject, family); head != nil {
						kind, ok = head.Kind, true
					} else {
						for _, historical := range sources.Mainline.Set.BySubject[subject] {
							if historical.Family == family {
								kind, ok = historical.Kind, true
								break
							}
						}
					}
				}
			}
			if !ok {
				add("FR-42", from+" -> "+link.Target, "point "+link.Type+" at an existing active record")
				continue
			}
			if link.Type == record.Serves && kind != record.Promise && kind != record.Expectation && kind != record.Need ||
				link.Type == record.Supersedes && (kind != entry.Draft.Kind || !strings.HasPrefix(link.Target, entry.Draft.Subject+"#")) {
				add("FR-42", from+" -> "+link.Target, "repair the "+link.Type+" relation's target kind and subject")
			}
			if (link.Type == record.DependsOn || link.Type == record.Supersedes) && strings.HasPrefix(link.Target, "design/"+result.Feature+"#") {
				graph[from] = append(graph[from], link.Target)
			}
		}
	}
	for _, member := range graphCycles(graph) {
		add("FR-42", member, "break the decision dependency or supersession cycle")
	}
	for _, entry := range result.Records {
		if entry.Draft.Kind != record.Document {
			continue
		}
		name := documentPath(entry, result.Feature)
		body := entry.Draft.Body
		if name == "tech-spec.md" {
			for _, dep := range bareBuildsOn(body) {
				citation := gate.Citation{Grammar: "design", Payload: dep.Slug}
				item := fmt.Sprintf("%s:%d design:%s", name, dep.Line, dep.Slug)
				if !citationResolved(result, sources, local, citation) {
					add("FR-39", item, "build on an existing design")
				} else if citationStale(result.Feature, sources, citation) {
					add("FR-38", item, "build on an active design")
				}
			}
		}
		if name == "prd.md" {
			body = withoutReadToRule(body)
		}
		for _, citation := range gate.ScanCitations(body) {
			item := fmt.Sprintf("%s:%d %s:%s", name, citation.Line, citation.Grammar, citation.Payload)
			if !citationResolved(result, sources, local, citation) {
				add("FR-39", item, "cite an existing memory record or document anchor")
				continue
			}
			if citationStale(result.Feature, sources, citation) {
				add("FR-38", item, "cite an active requirement or decision")
			}
		}
	}
	return unmet
}

type buildOnRef struct {
	Line int
	Slug string
}

func bareBuildsOn(body []byte) []buildOnRef {
	var out []buildOnRef
	lineNo := 0
	for line := range strings.SplitSeq(string(body), "\n") {
		lineNo++
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "| **Builds-on** |")
		if !ok {
			continue
		}
		value, _, _ = strings.Cut(value, "|")
		for raw := range strings.SplitSeq(value, " · ") {
			raw = strings.Trim(strings.TrimSpace(raw), "`")
			if raw == "" || raw == "—" || strings.HasPrefix(raw, "design:") {
				continue
			}
			out = append(out, buildOnRef{Line: lineNo, Slug: raw})
		}
	}
	return out
}

func documentPath(entry compile.Record, feature string) string {
	switch entry.Draft.Subject {
	case "prd/" + feature:
		if entry.Draft.Family == "prd" {
			return "prd.md"
		}
		return "problems.md"
	case "design/" + feature:
		return "tech-spec.md"
	case "feature/" + feature:
		return "events.md"
	case "brainstorm/" + feature:
		return "brainstorm/" + strings.TrimPrefix(entry.Draft.Family, "record-") + ".md"
	case "review/" + feature:
		return "reviews/" + strings.TrimPrefix(entry.Draft.Family, "report-") + ".md"
	}
	return entry.Draft.Subject + "#" + entry.Draft.Family
}

func withoutReadToRule(body []byte) []byte {
	lines := strings.Split(string(schema.Normalize(body)), "\n")
	inside := false
	for i, line := range lines {
		if strings.HasPrefix(line, "## 0.") || line == "## 0" {
			inside = true
		}
		if strings.HasPrefix(line, "## ") && len(line) > 3 && line[3] >= '1' && line[3] <= '9' {
			inside = false
		}
		if inside {
			lines[i] = ""
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func citationResolved(result compile.Result, sources Sources, local map[string]compile.Record, citation gate.Citation) bool {
	payload := citation.Payload
	prdRoot, designRoot, brainstormRoot := sources.roots()
	switch citation.Grammar {
	case "prd":
		feature, unit, hasUnit := strings.Cut(payload, "/")
		if !hasUnit || unit == "" {
			if feature == result.Feature {
				return true
			}
			return hasSubject(sources, "prd/"+feature) || sources.Documents[path.Join(prdRoot, feature, "effective.md")] != nil
		}
		if feature == result.Feature && unit == "draft" {
			// The review template names the current PRD file, not a row.
			document, ok := local["prd/"+feature+"#prd"]
			return ok && document.Draft.Kind == record.Document
		}
		unit = firstToken(unit)
		if feature == result.Feature {
			_, ok := local["prd/"+feature+"#"+unit]
			return ok
		}
		if hasFamily(sources, "prd/"+feature, unit) {
			return true
		}
		_, ok := sourceStatus(sources.Documents[path.Join(prdRoot, feature, "effective.source.json")], unit)
		return ok
	case "design":
		slug, anchor, _ := strings.Cut(payload, "#")
		var body []byte
		if slug == result.Feature {
			for _, entry := range result.Records {
				if entry.Draft.Kind == record.Document && entry.Draft.Subject == "design/"+slug {
					body = entry.Draft.Body
				}
			}
		} else {
			body = sources.Documents[path.Join(designRoot, slug+".md")]
		}
		if body == nil {
			if memoryDocumentAnchor(sources, "design/"+slug, anchor) {
				return true
			}
			return hasSubject(sources, "design/"+slug) && (anchor == "" || hasFamily(sources, "design/"+slug, firstToken(anchor)))
		}
		return anchor == "" || hasAnchor(body, firstToken(anchor))
	case "brainstorm", "design-brainstorm":
		slug, anchor, _ := strings.Cut(payload, "#")
		if slug == result.Feature || slug == result.Feature+"-tech-design" {
			for _, entry := range result.Records {
				if entry.Draft.Kind == record.Document && entry.Draft.Subject == "brainstorm/"+result.Feature && (anchor == "" || hasAnchor(entry.Draft.Body, firstToken(anchor))) {
					return true
				}
			}
		}
		body := sources.Documents[path.Join(brainstormRoot, slug+".md")]
		return body != nil && (anchor == "" || hasAnchor(body, firstToken(anchor))) || memoryDocumentAnchor(sources, "brainstorm/"+slug, anchor)
	case "corpus":
		unit := firstToken(payload)
		for name, body := range sources.Documents {
			if !strings.HasPrefix(name, prdRoot+"/") || strings.Count(strings.TrimPrefix(name, prdRoot+"/"), "/") != 0 || !strings.HasSuffix(name, ".md") {
				continue
			}
			if slices.Contains(schema.CorpusIDRe.FindAllString(string(body), -1), unit) {
				return true
			}
		}
	}
	return false
}

func memoryDocumentAnchor(sources Sources, subject, anchor string) bool {
	if sources.Mainline == nil {
		return false
	}
	for _, entry := range sources.Mainline.Set.BySubject[subject] {
		body := sources.Mainline.Set.Bodies[entry.ID]
		if entry.Kind == record.Document && body != nil && (anchor == "" || hasAnchor(body, firstToken(anchor))) {
			return true
		}
	}
	return false
}

func firstToken(value string) string {
	value = strings.SplitN(value, "..", 2)[0]
	for _, token := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '#' }) {
		if token != "" {
			return token
		}
	}
	return value
}

func hasAnchor(body []byte, anchor string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(anchor) + `\b`).Match(body)
}

func hasSubject(sources Sources, subject string) bool {
	return sources.Mainline != nil && len(sources.Mainline.Set.BySubject[subject]) > 0
}

func hasFamily(sources Sources, subject, family string) bool {
	if sources.Mainline == nil {
		return false
	}
	for _, entry := range sources.Mainline.Set.BySubject[subject] {
		if entry.Family == family {
			return true
		}
	}
	return false
}

func sourceStatus(body []byte, unit string) (string, bool) {
	var source struct {
		Units map[string]struct {
			Status string `json:"status"`
		} `json:"units"`
	}
	if json.Unmarshal(body, &source) != nil {
		return "", false
	}
	state, ok := source.Units[unit]
	return state.Status, ok
}

func citationStale(feature string, sources Sources, citation gate.Citation) bool {
	prdRoot, designRoot, _ := sources.roots()
	switch citation.Grammar {
	case "prd":
		other, unit, ok := strings.Cut(citation.Payload, "/")
		if !ok || other == feature || unit == "" {
			return false
		}
		unit = firstToken(unit)
		if status, found := sourceStatus(sources.Documents[path.Join(prdRoot, other, "effective.source.json")], unit); found {
			return status != "live"
		}
		if hasFamily(sources, "prd/"+other, unit) {
			return sources.Mainline.Head("prd/"+other, unit) == nil
		}
	case "design":
		slug, anchor, _ := strings.Cut(citation.Payload, "#")
		if slug == feature {
			return false
		}
		if designSuperseded(sources, designRoot, slug) {
			return true
		}
		if anchor != "" && hasFamily(sources, "design/"+slug, firstToken(anchor)) && sources.Mainline.Head("design/"+slug, firstToken(anchor)) == nil {
			return true
		}
		return memoryDocumentStale(sources, "design/"+slug)
	case "brainstorm", "design-brainstorm":
		slug, _, _ := strings.Cut(citation.Payload, "#")
		if slug != feature && slug != feature+"-tech-design" {
			return memoryDocumentStale(sources, "brainstorm/"+slug)
		}
	}
	return false
}

func designSuperseded(sources Sources, root, slug string) bool {
	declares := func(body []byte) bool {
		for line := range strings.SplitSeq(string(body), "\n") {
			if strings.HasPrefix(line, "| **Supersedes** |") && strings.Contains(" "+line+" ", " "+slug+" ") {
				return true
			}
		}
		return false
	}
	for name, body := range sources.Documents {
		if strings.HasPrefix(name, root+"/") && strings.HasSuffix(name, ".md") && declares(body) {
			return true
		}
	}
	if sources.Mainline != nil {
		for _, entry := range sources.Mainline.Set.Records {
			if entry.Kind == record.Document && strings.HasPrefix(entry.Subject, "design/") && entry.Subject != "design/"+slug && declares(sources.Mainline.Set.Bodies[entry.ID]) {
				return true
			}
		}
	}
	return false
}

func memoryDocumentStale(sources Sources, subject string) bool {
	if sources.Mainline == nil {
		return false
	}
	seen := false
	for _, entry := range sources.Mainline.Set.BySubject[subject] {
		if entry.Kind == record.Document {
			seen = true
			if sources.Mainline.Status(entry.ID) == resolve.Active {
				return false
			}
		}
	}
	return seen
}

func graphCycles(graph map[string][]string) []string {
	var nodes []string
	for node := range graph {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	color := map[string]uint8{}
	cycles := map[string]bool{}
	var visit func(string, []string)
	visit = func(node string, trail []string) {
		color[node] = 1
		trail = append(trail, node)
		deps := append([]string(nil), graph[node]...)
		sort.Strings(deps)
		for _, dep := range deps {
			if color[dep] == 0 {
				visit(dep, trail)
			} else if color[dep] == 1 {
				for i, member := range trail {
					if member == dep {
						cycle := append([]string(nil), trail[i:]...)
						sort.Strings(cycle)
						cycles[cycle[0]] = true
						break
					}
				}
			}
		}
		color[node] = 2
	}
	for _, node := range nodes {
		if color[node] == 0 {
			visit(node, nil)
		}
	}
	var out []string
	for node := range cycles {
		out = append(out, node)
	}
	sort.Strings(out)
	return out
}
