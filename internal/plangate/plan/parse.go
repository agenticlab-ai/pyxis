// The closed plan grammar (plan-new-v1 §3.1; exact bytes fixed here and in
// the golden fixtures, per its Q1). A plan source is, in order:
//
//	# <title>                              one H1
//	| Field | Value | header table         closed row set: Feature · PRD ·
//	                                       Design · Basis · Intake · Plan version · Dialect ·
//	                                       Epic title · Status · Template version
//	## Tasks
//	### T<n> — <title>                     dense, ascending from T1
//	  | Field | Value | task table         closed rows: Blocked by · Size band ·
//	                                       Mechanical · Size accepted ·
//	                                       Overlap accepted · Routing · Pattern ·
//	                                       Touches
//	  **<Label>:** regions                 Goal/Current behavior/Desired behavior
//	                                       (text) · Scope/Out of scope/
//	                                       Acceptance criteria/Key interfaces/
//	                                       Provenance (bulleted) · Traces
//	                                       (`·`-separated refs)
//	## Coverage                            | Requirement | Tasks |
//	## Deferred                            optional; | Requirement | Reason |
//
// Unknown sections, header rows, table fields, or region labels are
// defects — never a lenient parse.
package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

var (
	taskHeadRe = regexp.MustCompile(`^### (T\d+) — (.+)$`)
	tableRowRe = regexp.MustCompile(`^\|\s*\*\*(.+?)\*\*\s*\|\s*(.*?)\s*\|$`)
	labelRe    = regexp.MustCompile(`^\*\*([A-Z][A-Za-z ]*?):\*\*\s*(.*)$`)
	bandRe     = regexp.MustCompile(`^(\d+)\s*[–-]\s*(\d+)$`)
)

// Header rows are closed; feature and intake rows have different requirements.
var headerRows = []string{"Feature", "PRD", "Design", "Basis", "Intake", "Plan version", "Dialect", "Epic title", "Status", "Template version"}
var commonHeaderRows = []string{"Plan version", "Dialect", "Epic title", "Status", "Template version"}

// region labels: text regions hold prose; list regions hold bullets.
var (
	textRegions = map[string]bool{"Goal": true, "Current behavior": true, "Desired behavior": true, "Traces": true}
	listRegions = map[string]bool{"Scope": true, "Out of scope": true, "Acceptance criteria": true, "Key interfaces": true, "Provenance": true}
)

// Parse parses one plan source. file is the repo-relative path (defects +
// version cross-check); structural defects abort with a nil Doc, field
// defects accumulate.
func Parse(file string, b []byte) (*Doc, []schema.Defect) {
	d := &Doc{Schema: SchemaVersion, Tool: ToolVersion, sourceFile: file}
	var defects []schema.Defect
	bad := func(n int, reason, remedy string) {
		defects = append(defects, def(file, fmt.Sprintf("parse line %d", n), reason, remedy))
	}
	norm := schema.Normalize(b)
	d.Fingerprint = schema.Fingerprint([][]byte{norm})
	lines := strings.Split(strings.TrimRight(string(norm), "\n"), "\n")

	const (
		sHead = iota
		sHeader
		sTasks
		sCoverage
		sDeferred
	)
	state := sHead
	header := map[string]string{}
	headerAt := map[string]int{}
	var cur *Task
	rows := map[string]bool{}
	var region string // active **Label:** region within the current task
	flushRegionText := func() {
		if cur == nil || region == "" {
			return
		}
		set := func(dst *string) { *dst = strings.TrimSpace(*dst) }
		switch region {
		case "Goal":
			set(&cur.Goal)
		case "Current behavior":
			set(&cur.Current)
		case "Desired behavior":
			set(&cur.Desired)
		}
	}
	endTask := func() { flushRegionText(); region, cur = "", nil }

	for i, raw := range lines {
		n := i + 1
		line := strings.TrimRight(raw, " ")
		t := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(t, "# ") && !strings.HasPrefix(t, "## "):
			if state != sHead {
				bad(n, "second H1", "one plan per file")
				return nil, defects
			}
			d.Title = strings.TrimPrefix(t, "# ")
			state = sHeader
			continue
		case t == "## Tasks":
			endTask()
			state = sTasks
			continue
		case t == "## Coverage":
			endTask()
			state = sCoverage
			continue
		case t == "## Deferred":
			endTask()
			state = sDeferred
			continue
		case strings.HasPrefix(t, "## "):
			bad(n, fmt.Sprintf("unknown section %q", t), "sections are closed: Tasks · Coverage · Deferred")
			return nil, defects
		}

		switch state {
		case sHead:
			if t != "" {
				bad(n, "content before the H1 title", "the plan opens with `# <title>`")
				return nil, defects
			}
		case sHeader:
			if t == "" || t == "|---|---|" || t == "| Field | Value |" {
				continue
			}
			m := tableRowRe.FindStringSubmatch(t)
			if m == nil {
				bad(n, "non-table content in the header", "the header is one `| **Field** | value |` table")
				continue
			}
			if _, dup := header[m[1]]; dup {
				bad(n, fmt.Sprintf("duplicate header row %q", m[1]), "each header row appears exactly once")
				continue
			}
			header[m[1]], headerAt[m[1]] = m[2], n
		case sTasks:
			if m := taskHeadRe.FindStringSubmatch(t); m != nil {
				endTask()
				d.Tasks = append(d.Tasks, Task{ID: m[1], Title: strings.TrimSpace(m[2])})
				cur = &d.Tasks[len(d.Tasks)-1]
				rows = map[string]bool{}
				continue
			}
			if t == "" {
				continue
			}
			if cur == nil {
				bad(n, "content before the first task heading", "tasks open with `### T<n> — <title>`")
				continue
			}
			if t == "|---|---|" || t == "| Field | Value |" {
				continue
			}
			if m := tableRowRe.FindStringSubmatch(t); m != nil && region == "" {
				if rows[m[1]] {
					bad(n, fmt.Sprintf("duplicate task row %q", m[1]), "each task row appears once")
					continue
				}
				rows[m[1]] = true
				if header["Template version"] == NativeTemplateVersion && m[1] == "Routing" {
					bad(n, "legacy Routing row in a dimensioned plan", "provide Requirements assessments for both roles")
					continue
				}
				parseTaskRow(cur, m[1], m[2], n, bad)
				continue
			}
			if m := labelRe.FindStringSubmatch(t); m != nil {
				flushRegionText()
				region = m[1]
				rest := strings.TrimSpace(m[2])
				switch {
				case textRegions[region]:
					if region == "Traces" {
						cur.Traces = splitRefs(rest)
						region = ""
					} else {
						appendText(cur, region, rest)
					}
				case listRegions[region]:
					if rest != "" {
						bad(n, fmt.Sprintf("content on the %q label line", region), "list regions start their items on the following lines")
					}
				default:
					bad(n, fmt.Sprintf("unknown region label %q", region), "region labels are closed (grammar header of parse.go)")
					region = ""
				}
				continue
			}
			switch {
			case region == "":
				bad(n, "content outside any region", "task prose lives under a `**Label:**` region")
			case listRegions[region] && strings.HasPrefix(t, "- "):
				item := strings.TrimSpace(strings.TrimPrefix(t, "- "))
				if region == "Acceptance criteria" {
					if !strings.HasPrefix(item, "[ ]") {
						bad(n, "acceptance item without a checkbox", "acceptance criteria are `- [ ] <criterion>` items")
						continue
					}
					item = strings.TrimSpace(strings.TrimPrefix(item, "[ ]"))
				}
				appendList(cur, region, item)
			case listRegions[region]:
				bad(n, fmt.Sprintf("non-bullet content in the %q region", region), "list regions hold `- ` items only")
			default:
				appendText(cur, region, t)
			}
		case sCoverage, sDeferred:
			if t == "" || strings.HasPrefix(t, "|---") || t == "| Requirement | Tasks |" || t == "| Requirement | Reason |" {
				continue
			}
			cells := splitRow(t)
			if cells == nil {
				bad(n, "non-table content", "coverage and deferred are two-column tables")
				continue
			}
			if state == sCoverage {
				d.Coverage = append(d.Coverage, CoverageRow{Requirement: cells[0], Tasks: splitList(cells[1])})
			} else {
				d.Deferred = append(d.Deferred, DeferredRow{Requirement: cells[0], Reason: cells[1]})
			}
		}
	}
	endTask()

	// Header completeness + field validation.
	for _, k := range commonHeaderRows {
		if _, ok := header[k]; !ok {
			defects = append(defects, def(file, "parse header", fmt.Sprintf("missing header row %q", k), "the header row set is closed and complete"))
		}
	}
	if _, intake := header["Intake"]; intake {
		for _, k := range []string{"Feature", "PRD", "Design", "Basis"} {
			if _, present := header[k]; present {
				defects = append(defects, def(file, "parse header", fmt.Sprintf("Intake plan carries feature-only row %q", k), "omit Feature, PRD, Design, and Basis for a non-feature intake"))
			}
		}
	} else {
		for _, k := range []string{"Feature", "PRD", "Design"} {
			if _, present := header[k]; !present {
				defects = append(defects, def(file, "parse header", fmt.Sprintf("missing header row %q", k), "feature plans require Feature, PRD, and Design"))
			}
		}
	}
	for k := range header {
		known := false
		for _, want := range headerRows {
			known = known || k == want
		}
		if !known {
			bad(headerAt[k], fmt.Sprintf("unknown header row %q", k), "the header row set is closed")
		}
	}
	d.Feature, d.PRD, d.Design = header["Feature"], header["PRD"], header["Design"]
	d.Basis, d.Intake = header["Basis"], header["Intake"]
	d.Version, d.Dialect = header["Plan version"], header["Dialect"]
	d.EpicTitle, d.Status, d.TemplateVersion = header["Epic title"], header["Status"], header["Template version"]
	if d.TemplateVersion == NativeTemplateVersion {
		d.Schema = NativeSchemaVersion
	}
	for i := range d.Tasks {
		task := &d.Tasks[i]
		if task.Requirements != nil {
			compiled, err := task.Requirements.Compile(taskrequirements.Sources(task.Traces))
			if err != nil {
				bad(0, task.ID+": "+err.Error(), "supply every assessment dimension and valid charter/trace citations")
			} else {
				task.Requirements = &compiled
			}
		}
		if d.Schema == NativeSchemaVersion && task.Requirements == nil {
			bad(0, task.ID+" missing Requirements", "every task requires complete implementation and review assessments")
		}
		if d.Schema != NativeSchemaVersion && task.Requirements != nil {
			bad(0, task.ID+" requirements in a legacy plan", "use plan-template/v2 for dimensioned plans")
		}
	}

	// Structural task checks the parser owns: dense ascending IDs.
	for i := range d.Tasks {
		if want := fmt.Sprintf("T%d", i+1); d.Tasks[i].ID != want {
			defects = append(defects, def(file, "parse tasks",
				fmt.Sprintf("task IDs must be dense and ascending: found %s where %s was expected", d.Tasks[i].ID, want),
				"number tasks T1..Tn in order; gaps and reordering break plan-task-id stability"))
			break
		}
	}
	for i := range d.Tasks {
		d.Tasks[i].PlanTaskID = fmt.Sprintf("%s/%s/%s", d.Name(), d.Version, d.Tasks[i].ID)
	}
	if len(defects) > 0 {
		return nil, defects
	}
	return d, nil
}

func parseTaskRow(cur *Task, key, val string, n int, bad func(int, string, string)) {
	dash := val == "—" || val == "-" || val == ""
	switch key {
	case "Requirements":
		s, err := taskrequirements.Parse([]byte(val))
		if err != nil {
			bad(n, err.Error(), "provide a complete task-requirements/v1 JSON object")
			return
		}
		cur.Requirements = &s
	case "Blocked by":
		if !dash {
			cur.BlockedBy = splitList(val)
		}
	case "Size band":
		if val == "0" {
			return // 0–0: the mechanical zero band
		}
		m := bandRe.FindStringSubmatch(val)
		if m == nil {
			bad(n, fmt.Sprintf("size band %q is not `<lo>–<hi>` or `0`", val), "estimate an added-logic-lines band, e.g. 80–160")
			return
		}
		cur.Band.Lo, _ = strconv.Atoi(m[1])
		cur.Band.Hi, _ = strconv.Atoi(m[2])
	case "Mechanical":
		switch val {
		case "yes":
			cur.Mechanical = true
		case "no":
		default:
			bad(n, fmt.Sprintf("mechanical is %q", val), "yes or no")
		}
	case "Size accepted":
		if !dash {
			cur.SizeAccepted = val
		}
	case "Overlap accepted":
		if !dash {
			cur.OverlapAccepted = val
		}
	case "Routing":
		// Historical plan text is readable but supplies no execution route.
	case "Pattern":
		if dash {
			return
		}
		switch val {
		case PatternExpand, PatternMigrate, PatternContract:
			cur.Pattern = val
		default:
			bad(n, fmt.Sprintf("pattern %q unknown", val), "expand · migrate-batch · contract (PRD FR-8)")
		}
	case "Touches":
		if !dash {
			cur.Touches = splitList(val)
		}
	default:
		bad(n, fmt.Sprintf("unknown task field %q", key), "the task table row set is closed")
	}
}

func appendText(cur *Task, region, text string) {
	if text == "" {
		return
	}
	var dst *string
	switch region {
	case "Goal":
		dst = &cur.Goal
	case "Current behavior":
		dst = &cur.Current
	case "Desired behavior":
		dst = &cur.Desired
	default:
		return
	}
	if *dst != "" {
		*dst += " "
	}
	*dst += text
}

func appendList(cur *Task, region, item string) {
	switch region {
	case "Scope":
		cur.Scope = append(cur.Scope, item)
	case "Out of scope":
		cur.OutOfScope = append(cur.OutOfScope, item)
	case "Acceptance criteria":
		cur.Acceptance = append(cur.Acceptance, item)
	case "Key interfaces":
		cur.KeyInterfaces = append(cur.KeyInterfaces, item)
	case "Provenance":
		cur.Provenance = append(cur.Provenance, item)
	}
}

// splitRow splits a two-column table row into its trimmed cells.
func splitRow(t string) []string {
	if !strings.HasPrefix(t, "|") || !strings.HasSuffix(t, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(t, "|"), "|")
	if len(parts) != 2 {
		return nil
	}
	return []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
}

// splitList splits a `, `-separated cell; splitRefs a ` · `-separated one.
func splitList(v string) []string {
	var out []string
	for p := range strings.SplitSeq(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitRefs(v string) []string {
	var out []string
	for p := range strings.SplitSeq(v, "·") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
