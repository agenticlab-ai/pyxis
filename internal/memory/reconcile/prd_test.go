package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestProjectUnit_MappedKinds(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		cells      map[string]string
		wantKind   string
		wantStmt   string
		wantIntent string
		wantReason string
		wantFit    string
		wantPrio   string
		wantConseq string
	}{
		{name: "functional requirement", prefix: "FR", cells: map[string]string{"Requirement (EARS)": "  The system shall work. ", "Rationale": " needed ", "Fit criterion": " passes ", "Priority": " Must ", "Risk": "HIGH"}, wantKind: record.Promise, wantStmt: "The system shall work.", wantReason: "needed", wantFit: "passes", wantPrio: "Must", wantConseq: record.High},
		{name: "non regression", prefix: "NR", cells: map[string]string{"Existing behavior protected": " old behavior ", "Why protected": " compatibility ", "Fit criterion": " unchanged ", "Risk": "low"}, wantKind: record.Promise, wantStmt: "old behavior", wantReason: "compatibility", wantFit: "unchanged", wantConseq: record.Low},
		{name: "non functional", prefix: "NFR", cells: map[string]string{"Requirement": " respond quickly ", "Area": " latency ", "Fit criterion": " under 1 ms ", "Severity": " med "}, wantKind: record.Promise, wantStmt: "respond quickly", wantIntent: "latency", wantFit: "under 1 ms", wantConseq: record.Medium},
		{name: "user behavior", prefix: "UB", cells: map[string]string{"If… (trigger)": " signed in ", "Then the system shall…": " show the widget ", "User sees": " widget ", "Fit criterion": " visible "}, wantKind: record.Promise, wantStmt: "If signed in, then the system shall show the widget", wantIntent: "widget", wantFit: "visible"},
		{name: "abuse with vector", prefix: "ABUSE", cells: map[string]string{"Actor and intent": " attacker steals data ", "Vector": " crafted input ", "Impact": " disclosure ", "Mitigated by": " rejection ", "Severity": "critical"}, wantKind: record.Promise, wantStmt: "attacker steals data — vector: crafted input", wantIntent: "disclosure", wantFit: "rejection", wantConseq: record.High},
		{name: "abuse without vector", prefix: "ABUSE", cells: map[string]string{"Actor and intent": " attacker probes ", "Impact": " leak ", "Mitigated by": " denial "}, wantKind: record.Promise, wantStmt: "attacker probes", wantIntent: "leak", wantFit: "denial"},
		{name: "event", prefix: "EVT", cells: map[string]string{"Event name": " widget.created ", "Fires when": " saved ", "Properties (typed)": " id:string ", "Feeds": " audit "}, wantKind: record.Promise, wantStmt: "widget.created fires when saved", wantIntent: "id:string", wantFit: "audit"},
		{name: "done criterion", prefix: "DOD", cells: map[string]string{"Done criterion": " shipped ", "Source": " CI ", "Required evidence": " green run ", "Risk": "medium"}, wantKind: record.Expectation, wantStmt: "shipped", wantIntent: "CI", wantFit: "green run", wantConseq: record.Medium},
		{name: "evaluation", prefix: "EVAL", cells: map[string]string{"Behavior": " answers ", "Eval set / harness": " fixture ", "Threshold": " 99% "}, wantKind: record.Expectation, wantStmt: "answers", wantIntent: "fixture", wantFit: "99%"},
		{name: "goal", prefix: "G", cells: map[string]string{"We believe that…": " widgets help ", "Will result in…": " retention ", "Measured by…": " weekly use "}, wantKind: record.Need, wantStmt: "We believe widgets help; expected result: retention", wantFit: "weekly use"},
		{name: "job", prefix: "J", cells: map[string]string{"When… (situation)": " configuring ", "I want to… (motivation)": " preview ", "So I can… (outcome)": " avoid errors ", "Evidence": " interviews "}, wantKind: record.Need, wantStmt: "When configuring, I want preview, so I can avoid errors", wantReason: "interviews"},
		{name: "metric", prefix: "M", cells: map[string]string{"Metric": " success rate ", "Goal": " improve ", "Signal": " completed runs ", "Target (at checkpoint)": " 95% "}, wantKind: record.Need, wantStmt: "success rate: improve", wantIntent: "completed runs", wantFit: "95%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			columns, cells := sortedCells(tt.cells)
			u := fold.ResolvedUnit{ID: tt.prefix + "-1", Prefix: tt.prefix, Status: "live", IntroducedBy: "000", Columns: columns, Cells: cells}
			got := projectUnit(u)
			if got.Kind != tt.wantKind || got.Statement != tt.wantStmt || got.Intent != tt.wantIntent || got.Rationale != tt.wantReason || got.FitCriterion != tt.wantFit || got.Priority != tt.wantPrio || got.Consequence != tt.wantConseq || !got.Mapped {
				t.Fatalf("projectUnit() = %#v", got)
			}
			if len(got.SourceSHA256) != 64 || got.ID != u.ID || got.Status != u.Status || got.IntroducedBy != u.IntroducedBy {
				t.Fatalf("source identity = %#v", got)
			}
			columns[0] = "mutated"
			cells[0] = "mutated"
			if reflect.DeepEqual(got.Columns, columns) || reflect.DeepEqual(got.Cells, cells) {
				t.Fatal("projected row aliases parser slices")
			}
		})
	}
}

func TestRequiresDesignServe(t *testing.T) {
	for _, tt := range []struct {
		name string
		unit PRDUnit
		want bool
	}{
		{name: "live Must", unit: PRDUnit{Status: "live", Priority: " must "}, want: true},
		{name: "live should", unit: PRDUnit{Status: "live", Priority: "Should"}},
		{name: "retired Must", unit: PRDUnit{Status: "retired", Priority: "Must"}},
		{name: "done criterion has no priority", unit: PRDUnit{Status: "live", Prefix: "DOD"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiresDesignServe(tt.unit); got != tt.want {
				t.Fatalf("requiresDesignServe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectUnit_UnmappedAndDigestPreservesRawRow(t *testing.T) {
	for _, prefix := range []string{"NG", "WR", "Q", "UNKNOWN"} {
		t.Run(prefix, func(t *testing.T) {
			u := fold.ResolvedUnit{ID: prefix + "-1", Prefix: prefix, Status: "live", IntroducedBy: "000", Columns: []string{"Text"}, Cells: []string{"first"}}
			got := projectUnit(u)
			if got.Mapped || got.Kind != "" || got.SourceSHA256 == "" {
				t.Fatalf("projectUnit() = %#v", got)
			}
			u.Cells[0] = "second"
			if next := projectUnit(u); next.SourceSHA256 == got.SourceSHA256 {
				t.Fatal("source digest ignored raw cell change")
			}
		})
	}

	missingStatement := projectUnit(fold.ResolvedUnit{ID: "FR-2", Prefix: "FR", Status: "live", Columns: []string{"Requirement (EARS)"}, Cells: []string{"  "}})
	if missingStatement.Kind != record.Promise || missingStatement.Mapped {
		t.Fatalf("blank statement mapping = %#v", missingStatement)
	}
}

func TestProjectUnit_QuestionPreservesExactSourceFieldsWithoutBecomingAClaim(t *testing.T) {
	columns, cells := sortedCells(map[string]string{
		"Question":  " Which mechanism owns retries? ",
		"Owner":     " design ",
		"Needed by": " implementation gate ",
		"Status":    " open — design-owned ",
	})
	got := projectUnit(fold.ResolvedUnit{ID: "Q-1", Prefix: "Q", Status: "live", Columns: columns, Cells: cells})
	if got.Mapped || got.Kind != "" {
		t.Fatalf("question became a product claim: %#v", got)
	}
	if got.Statement != "Which mechanism owns retries?" || got.Intent != "design" || got.FitCriterion != "implementation gate" || got.Rationale != "open — design-owned" {
		t.Fatalf("question fields = %#v", got)
	}
}

func TestProjectUnit_MissingRequiredSemanticCellsIsUnmapped(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		cells  map[string]string
	}{
		{name: "user behavior needs result", prefix: "UB", cells: map[string]string{"If… (trigger)": "signed in"}},
		{name: "event needs trigger", prefix: "EVT", cells: map[string]string{"Event name": "widget.created"}},
		{name: "goal needs belief", prefix: "G", cells: map[string]string{"Will result in…": "retention"}},
		{name: "job needs outcome", prefix: "J", cells: map[string]string{"When… (situation)": "configuring", "I want to… (motivation)": "preview"}},
		{name: "metric needs goal", prefix: "M", cells: map[string]string{"Metric": "success rate"}},
		{name: "abuse needs actor", prefix: "ABUSE", cells: map[string]string{"Vector": "crafted input"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			columns, cells := sortedCells(tt.cells)
			got := projectUnit(fold.ResolvedUnit{ID: tt.prefix + "-1", Prefix: tt.prefix, Status: "live", Columns: columns, Cells: cells})
			if got.Mapped {
				t.Fatalf("projectUnit() mapped incomplete semantic row: %#v", got)
			}
		})
	}
}

func TestProjectUnit_SourceDigestIncludesAuthoredColumns(t *testing.T) {
	base := fold.ResolvedUnit{
		ID: "UB-1", Prefix: "UB", Status: "live", IntroducedBy: "000",
		Columns:         []string{"ID", "If… (trigger)", "Then the system shall…", "User sees"},
		Cells:           []string{"UB-1", "the gate runs", "reject invalid input", "an error"},
		AuthoredColumns: []string{"ID", "If… (trigger)", "Then the gate shall…", "Operator sees"},
	}
	changed := base
	changed.AuthoredColumns = []string{"ID", "If… (trigger)", "Then the executor shall…", "Host sees"}

	first, second := projectUnit(base), projectUnit(changed)
	if !first.Mapped || !second.Mapped {
		t.Fatalf("valid UB rows were not mapped: %#v %#v", first, second)
	}
	if first.Statement != second.Statement || first.Intent != second.Intent || first.FitCriterion != second.FitCriterion {
		t.Fatalf("authored header changed semantic cell projection: %#v %#v", first, second)
	}
	if first.SourceSHA256 == second.SourceSHA256 {
		t.Fatalf("source digest ignored authored actor/audience headers: %s", first.SourceSHA256)
	}
}

func TestProjectUnit_SourceDigestIgnoresSemanticNoopLineageReanchor(t *testing.T) {
	base := fold.ResolvedUnit{
		ID: "FR-1", Prefix: "FR", Status: "live", IntroducedBy: "000-base.md",
		Columns: []string{"ID", "Requirement (EARS)", "Priority"},
		Cells:   []string{"FR-1", "The system shall preserve meaning.", "Must"},
	}
	rebased := base
	rebased.IntroducedBy = "019-rebase.md"
	rebased.Supersedes = ptr("FR-0")
	rebased.SupersededBy = ptr("FR-2")
	rebased.Status = "superseded"

	if got, want := projectUnit(rebased).SourceSHA256, projectUnit(base).SourceSHA256; got != want {
		t.Fatalf("semantic-no-op lineage changed source digest: got %s want %s", got, want)
	}
	if got, want := projectUnit(rebased).LegacySourceSHA256s[0], projectUnit(base).LegacySourceSHA256s[0]; got == want {
		t.Fatalf("legacy identity unexpectedly ignored the lineage re-anchor: %s", got)
	}
	changed := base
	changed.Cells = append([]string(nil), base.Cells...)
	changed.Cells[1] = "The system shall change meaning."
	if projectUnit(changed).SourceSHA256 == projectUnit(base).SourceSHA256 {
		t.Fatal("semantic row change did not change source digest")
	}
}

func TestSamePRDSemanticsAcceptsOneLegacySourceIdentity(t *testing.T) {
	u := PRDUnit{
		ID: "FR-1", Kind: record.Promise, Statement: "Preserve compatibility.",
		SourceSHA256: strings.Repeat("1", 64), LegacySourceSHA256s: []string{strings.Repeat("2", 64)},
	}
	r := prdMemoryRecord(u, 24)
	r.Tags = []string{"source-unit:" + u.LegacySourceSHA256s[0]}
	if !samePRDSemantics(r, u) {
		t.Fatal("exact v1 source identity was not accepted")
	}
	r.Tags = append(r.Tags, "source-unit:"+u.SourceSHA256)
	if samePRDSemantics(r, u) {
		t.Fatal("multiple source identities were accepted")
	}
	r.Tags = []string{"source-unit:" + strings.Repeat("3", 64)}
	if samePRDSemantics(r, u) {
		t.Fatal("unrecognized source identity was accepted")
	}
}

func TestSamePRDSemanticsAcceptsHistoricalV1IdentityAcrossNoopRebase(t *testing.T) {
	base := fold.ResolvedUnit{
		ID: "FR-1", Prefix: "FR", Status: "live", IntroducedBy: "000-base.md",
		Columns: []string{"ID", "Requirement (EARS)", "Rationale", "Fit criterion"},
		Cells:   []string{"FR-1", "The system shall preserve meaning.", "Stable behavior.", "Meaning is preserved."},
	}
	rebased := base
	rebased.IntroducedBy = "019-rebase.md"
	old, current := projectUnit(base), projectUnit(rebased)
	if old.SourceSHA256 != current.SourceSHA256 || old.LegacySourceSHA256s[0] == current.LegacySourceSHA256s[0] {
		t.Fatalf("fixture does not model a semantic-no-op v1 identity change: old %#v current %#v", old, current)
	}
	current.LegacySourceSHA256s = append(current.LegacySourceSHA256s, old.LegacySourceSHA256s[0])
	record := prdMemoryRecord(old, 25)
	record.Tags = []string{"source-unit:" + old.LegacySourceSHA256s[0]}
	if !samePRDSemantics(record, current) {
		t.Fatal("pre-rebase v1 record drifted from an identical current semantic row")
	}
}

func ptr(s string) *string { return &s }

func TestConsequence(t *testing.T) {
	tests := []struct{ in, want string }{
		{" critical ", record.High}, {"HIGH", record.High}, {"med", record.Medium}, {" Medium ", record.Medium}, {"low", record.Low}, {"unknown", ""}, {"", ""},
	}
	for _, tt := range tests {
		if got := consequence(tt.in); got != tt.want {
			t.Errorf("consequence(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPRDDraft(t *testing.T) {
	old := "FR-0"
	u := PRDUnit{
		ID: "FR-1", Prefix: "FR", Status: "live", Kind: record.Promise,
		Statement: "Provide widgets.", Intent: "use widgets", Rationale: "needed", FitCriterion: "works", Consequence: record.High,
		SourceSHA256: strings.Repeat("1", 64), Supersedes: &old, Mapped: true,
	}
	prd := PRD{Feature: "widget", Source: "docs/prds/widget/effective.md", Fingerprint: "sha256:abc"}
	draft := prdDraft(prd, strings.Repeat("a", 40), u)
	if draft.Kind != u.Kind || draft.Subject != "prd/widget" || draft.Family != "FR-1" || draft.Statement != u.Statement || draft.Intent != u.Intent || draft.Rationale != u.Rationale || draft.FitCriterion != u.FitCriterion || draft.Consequence != u.Consequence {
		t.Fatalf("draft = %#v", draft)
	}
	if !reflect.DeepEqual(draft.Tags, []string{"reconcile:prd", "source-unit:" + u.SourceSHA256, "source:prd-ledger"}) {
		t.Fatalf("tags = %#v", draft.Tags)
	}
	if draft.Provenance.Source != prd.Source || draft.Provenance.Commit != strings.Repeat("a", 40) || draft.Provenance.Note != "compiled from sha256:abc" {
		t.Fatalf("provenance = %#v", draft.Provenance)
	}
	if draft.Relations != nil || draft.LiteralBody || draft.Body != nil {
		t.Fatalf("unexpected draft fields = %#v", draft)
	}
}

func TestPRDQuestionDraft(t *testing.T) {
	u := PRDUnit{
		ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which mechanism owns retries?",
		Intent: "design", Rationale: "open — design-owned", FitCriterion: "implementation gate",
		SourceSHA256: strings.Repeat("2", 64),
	}
	prd := PRD{Feature: "widget", Source: "docs/prds/widget/effective.md", Fingerprint: "sha256:abc"}
	draft, ok := prdQuestionDraft(prd, strings.Repeat("a", 40), u)
	if !ok || draft.Kind != record.Question || draft.Subject != "prd/widget" || draft.Family != "Q-1" || draft.Statement != u.Statement || draft.Intent != u.Intent || draft.Rationale != u.Rationale || draft.FitCriterion != u.FitCriterion {
		t.Fatalf("prdQuestionDraft() = %#v, %v", draft, ok)
	}
	if !reflect.DeepEqual(draft.Tags, []string{prdDispositionTag, "source-unit:" + u.SourceSHA256, "source:prd-ledger"}) || draft.Provenance.Source != prd.Source || draft.Provenance.Commit != strings.Repeat("a", 40) || draft.Provenance.Note != "captured from sha256:abc" {
		t.Fatalf("question identity/provenance = %#v", draft)
	}
	for _, bad := range []PRDUnit{
		{ID: "R-1", Prefix: "R", Statement: "risk", SourceSHA256: strings.Repeat("2", 64)},
		{ID: "Q-1", Prefix: "Q", SourceSHA256: strings.Repeat("2", 64)},
		{ID: "Q-1", Prefix: "Q", Statement: "question", SourceSHA256: "bad"},
	} {
		if _, ok := prdQuestionDraft(prd, strings.Repeat("a", 40), bad); ok {
			t.Fatalf("invalid question produced draft: %#v", bad)
		}
	}
}

func TestExplicitPRDDisposition(t *testing.T) {
	prd := PRD{Feature: "widget", Source: "docs/prds/widget/effective.md"}
	question := PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which mechanism?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("1", 64)}
	risk := PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: strings.Repeat("2", 64)}

	tests := []struct {
		name       string
		unit       PRDUnit
		records    func() []*record.Record
		wantRecord bool
		wantAnswer bool
		wantErr    string
	}{
		{name: "no disposition", unit: question},
		{name: "current row has invalid digest", unit: func() PRDUnit {
			u := question
			u.SourceSHA256 = ""
			return u
		}(), wantErr: "has no exact source-unit digest"},
		{name: "disposition outside exact subject is ignored", unit: question, records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 39)
			r.Subject = "prd/other"
			return []*record.Record{r}
		}},
		{name: "answered question", unit: question, wantRecord: true, wantAnswer: true, records: func() []*record.Record {
			q := fixturePRDDisposition(record.Question, prd, question, 40)
			return []*record.Record{q, fixturePRDAnswer(q, "prd/widget", 41)}
		}},
		{name: "unanswered question", unit: question, wantRecord: true, records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Question, prd, question, 42)}
		}},
		{name: "duplicate active answers", unit: question, wantErr: "has 2 active Answers", records: func() []*record.Record {
			q := fixturePRDDisposition(record.Question, prd, question, 43)
			return []*record.Record{q, fixturePRDAnswer(q, "prd/widget", 44), fixturePRDAnswer(q, "prd/widget", 45)}
		}},
		{name: "answer outside exact subject", unit: question, wantErr: "answered outside prd/widget", records: func() []*record.Record {
			q := fixturePRDDisposition(record.Question, prd, question, 46)
			return []*record.Record{q, fixturePRDAnswer(q, "prd/other", 47)}
		}},
		{name: "question disposition has wrong kind", unit: question, wantErr: "not a Question", records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Need, prd, question, 48)}
		}},
		{name: "valid risk decision", unit: risk, wantRecord: true, records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Decision, prd, risk, 49)}
		}},
		{name: "risk decision needs rationale", unit: risk, wantErr: "with rationale and fit criterion", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Decision, prd, risk, 50)
			r.Rationale = " "
			return []*record.Record{r}
		}},
		{name: "risk decision needs fit criterion", unit: risk, wantErr: "with rationale and fit criterion", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Decision, prd, risk, 51)
			r.FitCriterion = " "
			return []*record.Record{r}
		}},
		{name: "stale row digest", unit: question, wantErr: "must carry exactly one", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 52)
			r.Tags[1] = "source-unit:" + strings.Repeat("9", 64)
			return []*record.Record{r}
		}},
		{name: "wrong row address", unit: question, wantErr: "does not use exact address", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 53)
			r.Family = "Q-2"
			return []*record.Record{r}
		}},
		{name: "duplicate disposition tag", unit: question, wantErr: "must carry exactly one", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 54)
			r.Tags = append(r.Tags, prdDispositionTag)
			return []*record.Record{r}
		}},
		{name: "duplicate source unit tag", unit: question, wantErr: "must carry exactly one", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 55)
			r.Tags = append(r.Tags, "source-unit:"+question.SourceSHA256)
			return []*record.Record{r}
		}},
		{name: "multiple disposition records", unit: question, wantErr: "has 2 active disposition records", records: func() []*record.Record {
			return []*record.Record{
				fixturePRDDisposition(record.Question, prd, question, 56),
				fixturePRDDisposition(record.Question, prd, question, 57),
			}
		}},
		{name: "wrong source provenance", unit: question, wantErr: "does not carry exact PRD source provenance", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 58)
			r.Provenance.Source = "docs/prds/other/effective.md"
			return []*record.Record{r}
		}},
		{name: "non-commit provenance", unit: question, wantErr: "does not carry exact PRD source provenance", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Question, prd, question, 59)
			r.Provenance.Commit = "main"
			return []*record.Record{r}
		}},
		{name: "unsupported row kind cannot use a disposition", unit: PRDUnit{ID: "X-1", Prefix: "X", Status: "live", SourceSHA256: strings.Repeat("3", 64)}, wantErr: "unsupported disposition prefix X", records: func() []*record.Record {
			u := PRDUnit{ID: "X-1", Prefix: "X", Status: "live", SourceSHA256: strings.Repeat("3", 64)}
			return []*record.Record{fixturePRDDisposition(record.Need, prd, u, 60)}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				for _, r := range tt.records() {
					entries = append(entries, reconcileRecord{record: r})
				}
			}
			_, proj := loadReconcileRecords(t, entries...)
			got, err := explicitPRDDisposition(prd, tt.unit, proj)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("explicitPRDDisposition() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("explicitPRDDisposition(): %v", err)
			}
			if (got.Record != nil) != tt.wantRecord || (got.Answer != nil) != tt.wantAnswer {
				t.Fatalf("explicitPRDDisposition() = %#v", got)
			}
		})
	}

	if _, err := explicitPRDDisposition(prd, question, nil); err == nil || !strings.Contains(err.Error(), "projection is unavailable") {
		t.Fatalf("nil projection error = %v", err)
	}
}

func fixturePRDDisposition(kind string, prd PRD, unit PRDUnit, seq byte) *record.Record {
	r := fixtureRecord(kind, "prd/"+prd.Feature, unit.ID, seq)
	if kind == record.Question {
		r.Statement = unit.Statement
		r.Intent = unit.Intent
		r.Rationale = unit.Rationale
		r.FitCriterion = unit.FitCriterion
	} else {
		r.Statement = "Disposition for " + unit.ID
		r.Rationale = "The risk is accepted with an explicit owner."
		r.FitCriterion = "The recorded mitigation remains effective."
	}
	r.Tags = []string{prdDispositionTag, "source-unit:" + unit.SourceSHA256}
	r.Provenance = record.Provenance{Source: prd.Source, Commit: strings.Repeat("a", 40)}
	return r
}

func fixturePRDAnswer(question *record.Record, subject string, seq byte) *record.Record {
	r := fixtureRecord(record.Answer, subject, "", seq)
	r.Statement = "The question is resolved explicitly."
	r.Relations = []record.Relation{{Type: record.Answers, Target: question.ID}}
	return r
}

func sortedCells(values map[string]string) ([]string, []string) {
	order := []string{
		"Requirement (EARS)", "Rationale", "Fit criterion", "Priority", "Risk", "Existing behavior protected", "Why protected", "Requirement", "Area", "Severity",
		"If… (trigger)", "Then the system shall…", "User sees", "Actor and intent", "Vector", "Impact", "Mitigated by", "Event name", "Fires when", "Properties (typed)", "Feeds",
		"Done criterion", "Source", "Required evidence", "Behavior", "Eval set / harness", "Threshold", "We believe that…", "Will result in…", "Measured by…",
		"When… (situation)", "I want to… (motivation)", "So I can… (outcome)", "Evidence", "Metric", "Goal", "Signal", "Target (at checkpoint)",
		"Question", "Owner", "Needed by", "Status",
	}
	var columns, cells []string
	for _, column := range order {
		if value, ok := values[column]; ok {
			columns = append(columns, column)
			cells = append(cells, value)
		}
	}
	return columns, cells
}
