// Package contextpack compiles a pipeline stage's stable context into one
// deterministic, manifest-headed artifact — the context pack — built fresh
// on demand and never stored (docs/tech-design/context-pack-v1.md; PRD
// docs/prds/context-pack). The pack lets every authoring and review session
// read a stage's governing context once instead of re-crawling the corpus,
// while a closed admissible-kind set keeps the authoring trail structurally
// out of reach so a cold reviewer's pack is answer-key-free by construction
// (DINV-3). Stdlib only, per the repo Go rules.
package contextpack

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// DeclarationFile is the per-stage stable-context declaration, closed-parse
// data at a fixed repo-relative path (design §3.2; FR-3 — the class set is
// data, not code). Absent or unreadable is a named defect, never a lenient
// empty declaration.
const DeclarationFile = ".pyxis/context-pack.yaml"

// SubjectToken is substituted with the subject slug during selector
// resolution; a selector with no token is a fixed source (e.g. governing).
const SubjectToken = "{subject}"

// Kind is an admissible stable-context class kind. The set is CLOSED and
// contains NO authoring-trail kind — the delivery journal, review
// dispositions, and orchestration have no kind a declaration can name, so
// the compiler has no path that reads them into a pack (DINV-3, the
// independence boundary; FR-4). Adding a kind is a reviewed code change,
// never a data edit — that asymmetry is what makes the boundary structural.
type Kind string

const (
	KindGoverning   Kind = "governing"   // the governing documents (vision, principles)
	KindSubject     Kind = "subject"     // the frozen artifact under work, resolved per subject
	KindCalibration Kind = "calibration" // the stage's calibration inputs
)

// AdmissibleKinds is the closed set of legal kinds. Its members are the
// stable-context kinds only; membership is the sole gate on what a
// declaration may reference (DINV-3).
var AdmissibleKinds = map[Kind]bool{
	KindGoverning:   true,
	KindSubject:     true,
	KindCalibration: true,
}

// Entry is one declared class: a kind, a selector (a repo-relative path that
// may contain SubjectToken), and whether the source is optional — an
// optional source that resolves to nothing is recorded absent (FR-7), a
// required one that does not resolve is a fail-closed defect (UB-1, DINV-6).
type Entry struct {
	Kind     Kind
	Selector string
	Optional bool
}

// Declaration maps a stage name to its ordered class entries. Order is the
// declared order and is preserved verbatim into the pack (DINV-1, DINV-4).
type Declaration map[string][]Entry

// ParseDeclaration validates the closed declaration grammar over normalized
// bytes: indent-0 `stage:` headers, indent-2 `- <kind>[?] <selector>` items.
// An unknown kind, a malformed line, or unrecognized structure is a named
// defect (DINV-3 — a non-admitted kind can never enter a declaration),
// never a lenient parse. Stage order is irrelevant; entry order within a
// stage is preserved.
func ParseDeclaration(b []byte) (Declaration, []schema.Defect) {
	d := Declaration{}
	var defects []schema.Defect
	bad := func(n int, reason, remedy string) {
		defects = append(defects, def("parse", fmt.Sprintf("line %d: %s", n, reason), remedy))
	}
	stage := ""
	for i, raw := range strings.Split(string(schema.Normalize(b)), "\n") {
		n := i + 1
		line := strings.TrimRight(raw, " ")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimSpace(line)
		switch {
		case indent == 0 && strings.HasSuffix(t, ":"):
			stage = strings.TrimSuffix(t, ":")
			if stage == "" {
				bad(n, "empty stage name", "a stage header is `<stage-name>:`")
				continue
			}
			if _, seen := d[stage]; seen {
				bad(n, fmt.Sprintf("duplicate stage %q", stage), "declare each stage once; entries are an ordered list under it")
				continue
			}
			d[stage] = nil
		case indent == 2 && strings.HasPrefix(t, "- "):
			if stage == "" {
				bad(n, "entry before any stage header", "list a stage header before its class entries")
				continue
			}
			kindTok, selector, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(t, "- ")), " ")
			if !ok || strings.TrimSpace(selector) == "" {
				bad(n, "entry is not `<kind>[?] <selector>`", "each entry names a kind and a selector path")
				continue
			}
			optional := strings.HasSuffix(kindTok, "?")
			kind := Kind(strings.TrimSuffix(kindTok, "?"))
			if !AdmissibleKinds[kind] {
				bad(n, fmt.Sprintf("unknown kind %q", kind),
					"kinds are the closed stable-context set (governing · subject · calibration); no authoring-trail kind is admissible (DINV-3)")
				continue
			}
			d[stage] = append(d[stage], Entry{Kind: kind, Selector: strings.TrimSpace(selector), Optional: optional})
		default:
			bad(n, "unrecognized structure", "stage headers at column 0, `- <kind> <selector>` items indented two spaces")
		}
	}
	return d, defects
}

// Source is one resolved declaration entry: its kind, the repo-relative path
// after subject substitution, and whether it is optional.
type Source struct {
	Kind     Kind
	Path     string
	Optional bool
}

// Resolve turns a stage's declared entries into an ordered source list for a
// subject, substituting SubjectToken. An undeclared stage is a named defect
// (fail closed). Paths are cleaned but not read here — existence is the
// assembler's concern (Build), so resolution stays a pure path computation.
func (d Declaration) Resolve(stage, subject string) ([]Source, []schema.Defect) {
	entries, ok := d[stage]
	if !ok {
		return nil, []schema.Defect{def("resolve",
			fmt.Sprintf("no declaration for stage %q", stage),
			"declare the stage's stable-context classes in "+DeclarationFile)}
	}
	sources := make([]Source, 0, len(entries))
	for _, e := range entries {
		p := strings.ReplaceAll(e.Selector, SubjectToken, subject)
		sources = append(sources, Source{Kind: e.Kind, Path: filepath.ToSlash(filepath.Clean(p)), Optional: e.Optional})
	}
	return sources, nil
}

func def(op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "context-pack", File: DeclarationFile, Op: op, Reason: reason, Remedy: remedy}
}
