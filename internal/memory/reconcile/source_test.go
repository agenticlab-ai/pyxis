package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestAuthoredSource(t *testing.T) {
	tests := []struct {
		name string
		path string
		opts Options
		want bool
	}{
		{name: "root agents", path: "AGENTS.md", want: true},
		{name: "root readme", path: "./README.md", want: true},
		{name: "other root markdown", path: "CONTRIBUTING.md", want: false},
		{name: "prd markdown", path: "docs/prds/widget/000-base.md", want: true},
		{name: "prd yaml", path: "docs/prds/widget/context.yaml", want: true},
		{name: "generated effective markdown", path: "docs/prds/widget/effective.md", want: false},
		{name: "generated effective source", path: "docs/prds/widget/effective.source.json", want: false},
		{name: "generated feedback source", path: "docs/feedback/widget/effective.source.json", want: false},
		{name: "authored wiki named effective", path: "docs/wiki/effective.md", want: true},
		{name: "authored research named effective source", path: "docs/research/effective.source.json", want: true},
		{name: "session log named effective", path: "logs/effective.md", want: false},
		{name: "design markdown", path: "docs/tech-design/widget.md", want: true},
		{name: "generated design index markdown", path: "docs/tech-design/index.md", want: false},
		{name: "generated design index json", path: "docs/tech-design/index.json", want: false},
		{name: "plan markdown", path: "docs/plans/widget/delivery.md", want: true},
		{name: "generated plan json", path: "docs/plans/widget/delivery.json", want: false},
		{name: "ordinary docs json", path: "docs/contracts/schema.json", want: true},
		{name: "docs path with uppercase and spaces", path: "docs/API Guides/reference.md", want: true},
		{name: "docs executable", path: "docs/tool.go", want: false},
		{name: "session log", path: "logs/2026-01-01-work.md", want: false},
		{name: "legacy baseline session log", path: "logs/2026-01-01-work.md", opts: Options{LegacySessionLogs: true}, want: true},
		{name: "non markdown log", path: "logs/state.json", want: false},
		{name: "pyxis root policy", path: ".pyxis/model-routing.yaml", want: true},
		{name: "pyxis root path with uppercase and spaces", path: ".pyxis/Model Routing.yaml", want: true},
		{name: "nested unregistered pyxis file", path: ".pyxis/misc/note.md", want: false},
		{name: "authoring doc", path: ".pyxis/authoring/argus.md", want: true},
		{name: "skill config", path: ".pyxis/skills/argus/config.yml", want: true},
		{name: "template", path: ".pyxis/templates/prd-template.md", want: true},
		{name: "writehook config", path: ".pyxis/writehook/claude.settings.json", want: true},
		{name: "vendor readme", path: ".pyxis/vendor/mermaid/README.md", want: true},
		{name: "vendor nested readme", path: ".pyxis/vendor/a/nested/README.md", want: true},
		{name: "vendor license", path: ".pyxis/vendor/mermaid/LICENSE.md", want: false},
		{name: "versions env", path: ".pyxis/scripts/versions.env", want: true},
		{name: "other script data", path: ".pyxis/scripts/versions.json", want: false},
		{name: "command readme", path: "cmd/memory/README.md", want: true},
		{name: "command path with punctuation", path: "cmd/Memory++Tools/README.md", want: true},
		{name: "command source", path: "cmd/memory/main.go", want: false},
		{name: "fixture excluded", path: "docs/fixtures/example.md", want: false},
		{name: "testdata excluded", path: "docs/testdata/example.md", want: false},
		{name: "node modules excluded", path: "docs/node_modules/readme.md", want: false},
		{name: "parent traversal excluded", path: "../docs/escape.md", want: false},
		{name: "empty excluded", path: ".", want: false},
		{name: "custom prd root", path: "contracts/prds/widget/000-base.markdown", opts: Options{PRDs: "contracts/prds"}, want: true},
		{name: "custom design generated index", path: "contracts/designs/index.json", opts: Options{TechDesigns: "contracts/designs"}, want: false},
		{name: "custom plan generated json", path: "contracts/plans/widget/plan.json", opts: Options{Plans: "contracts/plans"}, want: false},
		{name: "custom brainstorm root", path: "factory/brainstorms/widget.md", opts: Options{Brainstorms: "factory/brainstorms"}, want: true},
		{name: "custom problem root", path: "factory/problems/widget/problem.md", opts: Options{Problems: "factory/problems"}, want: true},
		{name: "custom review root", path: "factory/reviews/widget-000.md", opts: Options{Reviews: "factory/reviews"}, want: true},
		{name: "custom feedback root", path: "factory/feedback/widget/001-response.yaml", opts: Options{Feedback: "factory/feedback"}, want: true},
		{name: "custom feedback projection", path: "factory/feedback/widget/effective.source.json", opts: Options{Feedback: "factory/feedback"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AuthoredSource(tt.path, tt.opts); got != tt.want {
				t.Fatalf("AuthoredSource(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSourceAddress(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		opts        Options
		wantSubject string
		wantFamily  string
	}{
		{name: "log index", path: "logs/README.md", wantSubject: "history/session-logs", wantFamily: "index"},
		{name: "log record", path: "logs/2026-01-01-Agent Work.md", wantSubject: "session-log/x-2026-01-01-agent-work", wantFamily: "record"},
		{name: "prd base", path: "docs/prds/widget/000-base.md", wantSubject: "prd/widget", wantFamily: "base"},
		{name: "prd amendment", path: "docs/prds/Widget Feature/017-change.md", wantSubject: "prd/widget-feature", wantFamily: "amendment-017"},
		{name: "non ledger prd document falls back", path: "docs/prds/README.md", wantSubject: "source/docs-prds-readme", wantFamily: "document"},
		{name: "design", path: "docs/tech-design/widget-v2.md", wantSubject: "design/widget-v2", wantFamily: "design"},
		{name: "nested design falls back", path: "docs/tech-design/sub/widget.md", wantSubject: "source/docs-tech-design-sub-widget", wantFamily: "document"},
		{name: "plan", path: "docs/plans/widget/delivery.md", wantSubject: "plan/widget", wantFamily: "delivery"},
		{name: "plan suffix", path: "docs/plans/widget/delivery-plan.md", wantSubject: "plan/widget", wantFamily: "plan-delivery"},
		{name: "review", path: "docs/reviews/widget-001.md", wantSubject: "review/widget-001", wantFamily: "report"},
		{name: "brainstorm", path: "docs/brainstorms/widget.md", wantSubject: "brainstorm/widget", wantFamily: "record"},
		{name: "wiki", path: "docs/wiki/Memory Model.markdown", wantSubject: "wiki/memory-model", wantFamily: "entry"},
		{name: "research", path: "docs/research/market.md", wantSubject: "research/market", wantFamily: "study"},
		{name: "nested review falls back", path: "docs/reviews/archive/widget.md", wantSubject: "source/docs-reviews-archive-widget", wantFamily: "document"},
		{name: "generic", path: ".pyxis/model-routing.md", wantSubject: "source/pyxis-model-routing", wantFamily: "document"},
		{name: "custom roots", path: "contracts/requirements/alpha/002-tune.md", opts: Options{PRDs: "contracts/requirements", TechDesigns: "contracts/designs", Plans: "contracts/plans"}, wantSubject: "prd/alpha", wantFamily: "amendment-002"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSubject, gotFamily := sourceAddress(tt.path, optionDefaults(tt.opts))
			if gotSubject != tt.wantSubject || gotFamily != tt.wantFamily {
				t.Fatalf("sourceAddress(%q) = %s#%s, want %s#%s", tt.path, gotSubject, gotFamily, tt.wantSubject, tt.wantFamily)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Widget Feature", "widget-feature"},
		{"a///B---c", "a-b-c"},
		{"already.valid_name", "already.valid_name"},
		{"123", "x-123"},
		{"---", "x-"},
		{"", "x-"},
		{"Éclair", "clair"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSourceDraft(t *testing.T) {
	body := []byte("literal {body}\r\n")
	f := File{Path: "docs/wiki/widget.md", SHA256: sourceDigest(body), Body: body}
	draft := sourceDraft(f, "wiki/widget", "entry", "Document-old", strings.Repeat("a", 40))
	if draft.Kind != record.Document || draft.Subject != "wiki/widget" || draft.Family != "entry" || !draft.LiteralBody {
		t.Fatalf("identity = %#v", draft)
	}
	if !reflect.DeepEqual(draft.Relations, []record.Relation{{Type: record.Supersedes, Target: "Document-old"}}) {
		t.Fatalf("relations = %#v", draft.Relations)
	}
	if draft.Provenance.Source != f.Path || draft.Provenance.Commit != strings.Repeat("a", 40) || draft.Provenance.Note != "offline reconciliation" {
		t.Fatalf("provenance = %#v", draft.Provenance)
	}
	if got := draft.Tags; !reflect.DeepEqual(got, []string{"class:docs/wiki", "reconcile:source", "sha256:" + f.SHA256}) {
		t.Fatalf("tags = %#v", got)
	}
	body[0] = 'X'
	if string(draft.Body) != "literal {body}\r\n" {
		t.Fatalf("draft body aliases source: %q", draft.Body)
	}
	if got := sourceDraft(f, "wiki/widget", "entry", "", "commit").Relations; got != nil {
		t.Fatalf("no-supersedes relations = %#v, want nil", got)
	}
	logDraft := sourceDraft(File{Path: "logs/2026-09-08-work.md", SHA256: f.SHA256}, "session-log/work", "record", "", "commit")
	if got, want := logDraft.Tags, []string{"class:logs", "historical", "non-governing", "reconcile:source", "sha256:" + f.SHA256}; !reflect.DeepEqual(got, want) {
		t.Fatalf("log tags = %#v, want %#v", got, want)
	}
}

func TestSourceClass(t *testing.T) {
	tests := []struct{ path, want string }{
		{"AGENTS.md", "repo-root"}, {"README.md", "repo-root"},
		{"logs/work.md", "logs"}, {"docs/product-vision.md", "docs/root"},
		{"docs/prds/widget/000-base.md", "docs/prds"}, {".pyxis/config.yaml", ".pyxis/config.yaml"},
		{".pyxis/skills/argus/SKILL.md", ".pyxis/skills"}, {"cmd/memory/README.md", "cmd/memory"},
		{"docs/API Guides/reference.md", "docs/api-guides"},
		{"docs/R&D (API)/reference.md", "docs/r-d-api"},
		{".pyxis/Model Routing.yaml", ".pyxis/model-routing.yaml"},
		{"cmd/Memory++Tools/README.md", "cmd/memory-tools"},
		{"docs/!!!/reference.md", "docs/unknown"},
		{"custom/source.md", ""},
	}
	for _, tt := range tests {
		if got := sourceClass(tt.path); got != tt.want {
			t.Errorf("sourceClass(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestSourceDraftClassTagsValidateForAcceptedPaths(t *testing.T) {
	paths := []string{
		"docs/API Guides/reference.md",
		"docs/R&D (API)/reference.md",
		".pyxis/Model Routing.yaml",
		"cmd/Memory++Tools/README.md",
		"docs/!!!/reference.md",
	}
	for i, sourcePath := range paths {
		t.Run(sourcePath, func(t *testing.T) {
			if !AuthoredSource(sourcePath, Options{}) {
				t.Fatalf("test path %q is not an accepted authored source", sourcePath)
			}
			body := []byte("authored source\n")
			file := File{Path: sourcePath, SHA256: sourceDigest(body), Body: body}
			subject, family := sourceAddress(sourcePath, optionDefaults(Options{}))
			draft := sourceDraft(file, subject, family, "", strings.Repeat("a", 40))

			r := fixtureRecord(draft.Kind, draft.Subject, draft.Family, byte(40+i))
			r.Statement = draft.Statement
			r.Intent = draft.Intent
			r.Rationale = draft.Rationale
			r.FitCriterion = draft.FitCriterion
			r.Tags = append([]string(nil), draft.Tags...)
			r.Consequence = draft.Consequence
			r.Relations = append([]record.Relation(nil), draft.Relations...)
			r.Provenance = draft.Provenance
			r.LiteralBody = draft.LiteralBody
			r.BodySHA256 = sourceDigest(draft.Body)
			if err := r.Validate(); err != nil {
				t.Fatalf("draft for %q produced an invalid record: %v", sourcePath, err)
			}
		})
	}
}

func TestSourceClosure(t *testing.T) {
	files := map[string]File{
		"a.md": {Path: "a.md", Mode: "100644", SHA256: "aaa"},
		"b.md": {Path: "b.md", Mode: "100755", SHA256: "bbb"},
	}
	want := sourceDigest([]byte("a.md\x00100644\x00aaa\nb.md\x00100755\x00bbb\n"))
	for _, paths := range [][]string{{"b.md", "missing.md", "a.md"}, {"a.md", "b.md"}} {
		if got := sourceClosure(files, paths); got != want {
			t.Fatalf("sourceClosure(%v) = %s, want %s", paths, got, want)
		}
	}
	changed := cloneFiles(files)
	f := changed["b.md"]
	f.Mode = "100644"
	changed["b.md"] = f
	if got := sourceClosure(changed, []string{"a.md", "b.md"}); got == want {
		t.Fatal("closure ignored file mode")
	}
}
