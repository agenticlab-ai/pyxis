package readiness

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func judgeWithSources(t *testing.T, files map[string][]byte, sources Sources) Verdict {
	t.Helper()
	refreshReviews(t, files)
	result, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	return Judge(result, sources)
}

func TestLinkAndStructureRefusals(t *testing.T) {
	cases := []struct {
		name      string
		edit      func(map[string][]byte)
		sources   Sources
		condition string
		item      string
	}{
		{"missing read to rule", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "## 0. Read to rule", "## Read to rule", 1))
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| FR-99 | This is a human-only pointer. | none | Must | low | none | Inspection | — |", "", 1))
		}, Sources{}, "FR-41", "prd.md#0"},
		{"missing definition of done", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "### 5.8 Definition of Done", "### Definition of Done", 1))
		}, Sources{}, "FR-41", "prd.md#5.8"},
		{"empty definition of done", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| DOD-1 | Open test passes. | FR-1 | Test | CI run | yes |", "", 1))
		}, Sources{}, "FR-41", "prd.md#5.8/DOD"},
		{"missing contract readiness", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "## 8. Contract readiness", "## Contract readiness", 1))
		}, Sources{}, "FR-41", "prd.md#8"},
		{"dangling citation", func(f map[string][]byte) {
			f["tech-spec.md"] = append(f["tech-spec.md"], []byte("\nThe mechanism follows design:missing#D-1.\n")...)
		}, Sources{}, "FR-39", "design:missing#D-1"},
		{"retired external requirement", func(f map[string][]byte) {
			f["tech-spec.md"] = append(f["tech-spec.md"], []byte("\nThe mechanism serves prd:other/FR-9.\n")...)
		}, Sources{Documents: map[string][]byte{"docs/prds/other/effective.source.json": []byte(`{"units":{"FR-9":{"status":"retired"}}}`)}}, "FR-38", "prd:other/FR-9"},
		{"nonexistent serves target", func(f map[string][]byte) {
			f["tech-spec.md"] = []byte(strings.Replace(string(f["tech-spec.md"]), "FR-1 · UB-1", "FR-1 · UB-1 · FR-99", 1))
		}, Sources{}, "FR-42", "prd/widget#FR-99"},
		{"broken supersession", func(f map[string][]byte) {
			f["tech-spec.md"] = append(f["tech-spec.md"], []byte("\n### D-2 — Replace decision\n\n**Decision:** Replace queue choice.\n\n**Supersedes:** D-99\n")...)
		}, Sources{}, "FR-42", "design/widget#D-99"},
		{"decision dependency cycle", func(f map[string][]byte) {
			f["tech-spec.md"] = []byte(strings.Replace(string(f["tech-spec.md"]), "**Serves:** FR-1 · UB-1", "**Serves:** FR-1 · UB-1\n**Builds-on:** D-2", 1))
			f["tech-spec.md"] = append(f["tech-spec.md"], []byte("\n### D-2 — Close the cycle\n\n**Decision:** Depend on the queue choice.\n\n**Builds-on:** D-1\n")...)
		}, Sources{}, "FR-42", "design/widget#D-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := readyFiles(t)
			tc.edit(files)
			v := judgeWithSources(t, files, tc.sources)
			if v.Ready || len(v.Unmet) != 1 || v.Unmet[0].Condition != tc.condition || !strings.Contains(v.Unmet[0].Item, tc.item) {
				t.Fatalf("want only %s naming %s, got %+v", tc.condition, tc.item, v.Unmet)
			}
		})
	}
}

func TestCitationsSkipHumanRoutingAndCode(t *testing.T) {
	files := readyFiles(t)
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "## 0. Read to rule", "## 0. Read to rule\n\ndesign:missing#D-1", 1))
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\n`design:missing#D-1`\n\n```text\ndesign:missing#D-1\n```\n")...)
	if v := judgeWithSources(t, files, Sources{}); !v.Ready {
		t.Fatalf("human routing and code citations should not bind: %+v", v.Unmet)
	}
}

func TestReviewTemplateDraftSubjectCitation(t *testing.T) {
	for _, tc := range []struct {
		name, citation string
		ready          bool
	}{
		{"current PRD draft", "prd:widget/draft", true},
		{"other feature draft", "prd:other/draft", false},
		{"missing draft anchor", "prd:widget/draft#FR-99", false},
		{"missing draft path", "prd:widget/draft/missing", false},
		{"missing requirement", "prd:widget/FR-99", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := readyFiles(t)
			files["reviews/001.md"] = append(files["reviews/001.md"], []byte("\n| **Subject** | "+tc.citation+" |\n")...)
			v := judgeWithSources(t, files, Sources{})
			if tc.ready {
				if !v.Ready {
					t.Fatalf("canonical review subject refused: %+v", v.Unmet)
				}
			} else if v.Ready || len(v.Unmet) != 1 || v.Unmet[0].Condition != "FR-39" || !strings.Contains(v.Unmet[0].Item, tc.citation) {
				t.Fatalf("invalid subject should be a citation refusal: %+v", v)
			}
		})
	}
}

func TestStructureFailsClosedOnMissingOrUnreadablePRD(t *testing.T) {
	result := compile.Result{Feature: "widget"}
	if got := checkStructure(result); len(got) != 1 || got[0].Condition != "FR-41" {
		t.Fatalf("missing PRD: %+v", got)
	}
	result.Records = []compile.Record{{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", Body: []byte("bad PRD")}}}
	if got := checkStructure(result); len(got) != 1 || got[0].Remedy != "repair the PRD section structure" {
		t.Fatalf("unreadable PRD: %+v", got)
	}
}

func sourceProjection(t *testing.T, revokeRequirement, revokeDecision bool) *resolve.Projection {
	t.Helper()
	when := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	files := store.MapSource{}
	n := byte(0)
	add := func(kind, subject, family, statement string, relations ...record.Relation) *record.Record {
		n++
		var entropy [10]byte
		entropy[9] = n
		r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, when.Add(time.Duration(n)*time.Second), entropy), Kind: kind,
			Subject: subject, Family: family, Statement: statement, Issuer: "test@example.com", CreatedAt: when.Format(time.RFC3339), Relations: relations}
		body, err := record.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		files[r.Path()] = body
		return r
	}
	req := add(record.Promise, "prd/other", "FR-1", "The other feature shall open.")
	decision := add(record.Decision, "design/other", "D-1", "Open safely.")
	if revokeRequirement {
		add(record.Revocation, "prd/other", "", "Retired.", record.Relation{Type: record.Revokes, Target: req.ID})
	}
	if revokeDecision {
		add(record.Revocation, "design/other", "", "Replaced.", record.Relation{Type: record.Revokes, Target: decision.ID})
	}
	set, err := store.Load(context.Background(), files)
	if err != nil || len(set.Problems) != 0 {
		t.Fatalf("load memory fixture: %v %+v", err, set.Problems)
	}
	return resolve.New(set)
}

func TestCitationsResolveAgainstPinnedRecordsAndDocuments(t *testing.T) {
	live := sourceProjection(t, false, false)
	revokedRequirement := sourceProjection(t, true, false)
	revokedDecision := sourceProjection(t, false, true)
	docs := map[string][]byte{
		"docs/prds/other/effective.md":          []byte("# Other PRD\n"),
		"docs/tech-design/other.md":             []byte("# Other design\n\n### D-1 — Open safely\n"),
		"docs/brainstorms/note.md":              []byte("# Note\n\n### D-1 — Choice\n"),
		"docs/brainstorms/note-tech-design.md":  []byte("# Design note\n\n### D-2 — Choice\n"),
		"docs/prds/vnext-requirements.md":       []byte("| VNEXT-1 | Example requirement |\n"),
		"docs/prds/other/effective.source.json": []byte(`{"units":{"FR-2":{"status":"live"}}}`),
	}
	cases := []struct {
		name, citation, condition string
		sources                   Sources
	}{
		{"local requirement", "prd:widget/FR-1", "", Sources{}},
		{"local whole PRD", "prd:widget", "", Sources{}},
		{"local decision", "design:widget#D-1", "", Sources{}},
		{"live memory requirement", "prd:other/FR-1", "", Sources{Mainline: live}},
		{"revoked memory requirement", "prd:other/FR-1", "FR-38", Sources{Mainline: revokedRequirement}},
		{"live source map requirement", "prd:other/FR-2", "", Sources{Documents: docs}},
		{"whole external PRD", "prd:other", "", Sources{Documents: docs}},
		{"external decision", "design:other#D-1", "", Sources{Mainline: live, Documents: docs}},
		{"revoked decision", "design:other#D-1", "FR-38", Sources{Mainline: revokedDecision, Documents: docs}},
		{"design anchor missing", "design:other#D-99", "FR-39", Sources{Documents: docs}},
		{"brainstorm anchor", "brainstorm:note#D-1", "", Sources{Documents: docs}},
		{"design brainstorm anchor", "design-brainstorm:note-tech-design#D-2", "", Sources{Documents: docs}},
		{"brainstorm missing", "brainstorm:missing#D-1", "FR-39", Sources{Documents: docs}},
		{"corpus anchor", "corpus:VNEXT-1", "", Sources{Documents: docs}},
		{"corpus missing", "corpus:VNEXT-99", "FR-39", Sources{Documents: docs}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := readyFiles(t)
			files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\nContext: "+tc.citation+".\n")...)
			v := judgeWithSources(t, files, tc.sources)
			if tc.condition == "" && !v.Ready || tc.condition != "" && (len(v.Unmet) != 1 || v.Unmet[0].Condition != tc.condition || !strings.Contains(v.Unmet[0].Item, tc.citation)) {
				t.Fatalf("%s: verdict %+v", tc.citation, v.Unmet)
			}
		})
	}
}

func TestSupersededDesignSlugIsStale(t *testing.T) {
	files := readyFiles(t)
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\nBuilds on design:old#D-1.\n")...)
	sources := Sources{Documents: map[string][]byte{
		"docs/tech-design/old.md": []byte("# Old design\n\n### D-1 — Initial choice\n"),
		"docs/tech-design/new.md": []byte("# New design\n\n| **Supersedes** | old |\n"),
	}}
	v := judgeWithSources(t, files, sources)
	if len(v.Unmet) != 1 || v.Unmet[0].Condition != "FR-38" || !strings.Contains(v.Unmet[0].Item, "design:old#D-1") {
		t.Fatalf("superseded design: %+v", v.Unmet)
	}
}

func TestRetiredExternalLinksNameCurrencyOnly(t *testing.T) {
	for _, tc := range []struct {
		field, citation string
		sources         Sources
	}{
		{"**Serves:** FR-1 · UB-1 · prd:other/FR-1", "prd:other/FR-1", Sources{Mainline: sourceProjection(t, true, false)}},
		{"**Builds-on:** design:other#D-1", "design:other#D-1", Sources{Mainline: sourceProjection(t, false, true), Documents: map[string][]byte{"docs/tech-design/other.md": []byte("# Other\n\n### D-1 — Choice\n")}}},
	} {
		files := readyFiles(t)
		files["tech-spec.md"] = []byte(strings.Replace(string(files["tech-spec.md"]), "**Serves:** FR-1 · UB-1", tc.field, 1))
		if strings.HasPrefix(tc.field, "**Builds-on:**") {
			files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\n**Serves:** FR-1 · UB-1\n")...)
		}
		v := judgeWithSources(t, files, tc.sources)
		if len(v.Unmet) != 1 || v.Unmet[0].Condition != "FR-38" || !strings.Contains(v.Unmet[0].Item, tc.citation) {
			t.Fatalf("retired %s: %+v", tc.citation, v.Unmet)
		}
	}
}

func TestBareBuildsOnHeaderUsesPinnedDesignStatus(t *testing.T) {
	sources := Sources{Documents: map[string][]byte{
		"docs/tech-design/old.md": []byte("# Old design\n"),
		"docs/tech-design/new.md": []byte("# New design\n\n| **Supersedes** | old |\n"),
	}}
	for _, tc := range []struct{ slug, condition string }{{"old", "FR-38"}, {"missing", "FR-39"}} {
		files := readyFiles(t)
		files["tech-spec.md"] = []byte(strings.Replace(string(files["tech-spec.md"]), "# Widget — tech spec", "# Widget — tech spec\n\n| **Builds-on** | `"+tc.slug+"` |", 1))
		v := judgeWithSources(t, files, sources)
		if len(v.Unmet) != 1 || v.Unmet[0].Condition != tc.condition || !strings.Contains(v.Unmet[0].Item, "design:"+tc.slug) {
			t.Fatalf("bare Builds-on %s: %+v", tc.slug, v.Unmet)
		}
	}
}

func TestConfigurableCitationCorpusRoots(t *testing.T) {
	files := readyFiles(t)
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\nSee prd:other/FR-2, design:other#D-1, brainstorm:note#D-1, and corpus:VNEXT-1.\n")...)
	sources := Sources{
		PRDsRoot: "specs/prds", DesignsRoot: "specs/designs", BrainstormsRoot: "specs/brainstorms",
		Documents: map[string][]byte{
			"specs/prds/other/effective.source.json": []byte(`{"units":{"FR-2":{"status":"live"}}}`),
			"specs/prds/vnext.md":                    []byte("| VNEXT-1 | Example |\n"),
			"specs/designs/other.md":                 []byte("### D-1 — Choice\n"),
			"specs/brainstorms/note.md":              []byte("### D-1 — Choice\n"),
		},
	}
	if v := judgeWithSources(t, files, sources); !v.Ready {
		t.Fatalf("configured corpus references: %+v", v.Unmet)
	}
}

func TestDraftAndLockedBrainstormCitations(t *testing.T) {
	files := readyFiles(t)
	files["brainstorm/001.md"] = []byte("# Widget brainstorm\n\n### D-1 — Choice\n")
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\nSee brainstorm:widget#D-1 and design-brainstorm:widget-tech-design#D-1.\n")...)
	if v := judgeWithSources(t, files, Sources{}); !v.Ready {
		t.Fatalf("draft brainstorm citations: %+v", v.Unmet)
	}
	memoryDoc := &record.Record{ID: "Document-locked", Kind: record.Document, Subject: "brainstorm/locked", Family: "record-001"}
	set := &store.Set{BySubject: map[string][]*record.Record{"brainstorm/locked": {memoryDoc}}, Bodies: map[string][]byte{memoryDoc.ID: []byte("### D-2 — Locked choice\n")}}
	sources := Sources{Mainline: &resolve.Projection{Set: set}}
	if !citationResolved(compile.Result{Feature: "widget"}, sources, nil, gate.Citation{Grammar: "brainstorm", Payload: "locked#D-2"}) {
		t.Fatal("locked brainstorm body did not resolve")
	}
	if citationResolved(compile.Result{Feature: "widget"}, sources, nil, gate.Citation{Grammar: "brainstorm", Payload: "locked#D-99"}) {
		t.Fatal("missing locked brainstorm anchor resolved")
	}
}

func TestSupersededNativeDesignBodyIsStale(t *testing.T) {
	old := &record.Record{ID: "Document-old", Kind: record.Document, Subject: "design/old", Family: "design"}
	newer := &record.Record{ID: "Document-new", Kind: record.Document, Subject: "design/new", Family: "design"}
	set := &store.Set{
		Records:   map[string]*record.Record{old.ID: old, newer.ID: newer},
		BySubject: map[string][]*record.Record{"design/old": {old}, "design/new": {newer}},
		Bodies: map[string][]byte{
			old.ID:   []byte("# Old design\n\n### D-1 — Original choice\n"),
			newer.ID: []byte("# New design\n\n| **Supersedes** | old |\n"),
		},
	}
	sources := Sources{Mainline: &resolve.Projection{Set: set}}
	citation := gate.Citation{Grammar: "design", Payload: "old#D-1"}
	if !citationResolved(compile.Result{Feature: "widget"}, sources, nil, citation) || !citationStale("widget", sources, citation) {
		t.Fatal("native design body supersession did not resolve as stale")
	}
}

func TestMissingSummaryStillChecksLaterCitations(t *testing.T) {
	files := readyFiles(t)
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "## 1. Summary", "## Summary", 1))
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "## 2. Problem space", "## 2. Problem space\n\nThe problem cites design:missing#D-1.", 1))
	v := judgeWithSources(t, files, Sources{})
	if len(v.Unmet) != 2 || v.Unmet[0].Condition != "FR-39" || v.Unmet[1].Condition != "FR-41" {
		t.Fatalf("missing summary must not hide later citation: %+v", v.Unmet)
	}
}
