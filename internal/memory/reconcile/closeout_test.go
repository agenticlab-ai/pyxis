package reconcile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestBuildCloseout_CanonicalCompleteAndIdempotent(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)

	draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	if draft.Kind != record.Document || draft.Subject != "closeout/widget" || draft.Family != "seal" || !draft.LiteralBody {
		t.Fatalf("draft identity = %#v", draft)
	}
	if len(receipt.SourceDocuments) != 2 || len(receipt.NativeRecords) != 3 || len(receipt.Designs) != 1 || len(receipt.CodeUnits) != 1 {
		t.Fatalf("receipt bindings = sources %d, native %d, designs %d, code %d", len(receipt.SourceDocuments), len(receipt.NativeRecords), len(receipt.Designs), len(receipt.CodeUnits))
	}
	if got, want := receipt.CodeUnits[0].Path, "internal/widget"; got != want {
		t.Fatalf("code path = %q, want %q", got, want)
	}
	if got := receipt.CodeUnits[0].MemoryRecords; len(got) != 1 || got[0] != f.decision.ID {
		t.Fatalf("code semantic bindings = %#v, want [%s]", got, f.decision.ID)
	}
	parsed, err := ParseCloseout(draft.Body)
	if err != nil {
		t.Fatalf("ParseCloseout: %v", err)
	}
	if parsed.ClosureSHA256 != receipt.ClosureSHA256 {
		t.Fatalf("parsed closure = %s, want %s", parsed.ClosureSHA256, receipt.ClosureSHA256)
	}
	draft2, receipt2, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("second BuildCloseout: %v", err)
	}
	if !bytes.Equal(draft.Body, draft2.Body) || !reflect.DeepEqual(receipt, receipt2) {
		t.Fatalf("repeated build is not deterministic")
	}

	f.addDraft(t, draft, 90)
	sealedSet, sealedProj := f.load(t)
	draft3, receipt3, err := BuildCloseout("widget", f.snap, sealedSet, sealedProj)
	if err != nil {
		t.Fatalf("idempotent BuildCloseout: %v", err)
	}
	if !reflect.DeepEqual(draft3, Draft{}) || !reflect.DeepEqual(receipt3, receipt) {
		t.Fatalf("existing identical receipt returned draft %#v", draft3)
	}
	active, err := ActiveCloseouts(sealedSet, sealedProj)
	if err != nil {
		t.Fatalf("ActiveCloseouts: %v", err)
	}
	if len(active) != 1 || active[0].Receipt.Feature != "widget" {
		t.Fatalf("active = %#v", active)
	}
	sealed := sealedSourcePaths(sealedSet, sealedProj)
	for _, source := range []string{f.prdPath, f.designPath} {
		if !sealed[source] {
			t.Errorf("source %s was not sealed", source)
		}
	}
}

func TestBuildCloseout_FailsClosedOnIncompleteFeature(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *closeoutFixture)
	}{
		{
			name: "active unanswered question",
			mutate: func(t *testing.T, f *closeoutFixture) {
				question := fixtureRecord(record.Question, "prd/widget", "", 25)
				question.Statement = "Is the feature really complete?"
				f.replaceRecord(t, question, nil)
			},
		},
		{
			name: "missing compiled PRD",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				f.snap.PRDs = nil
			},
		},
		{
			name: "unmapped live question",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				f.snap.PRDs[0].Units[0].Mapped = false
				f.snap.PRDs[0].Units[0].Prefix = "Q"
			},
		},
		{
			name: "drifted PRD memory",
			mutate: func(t *testing.T, f *closeoutFixture) {
				f.promise.Statement = "different"
				f.replaceRecord(t, f.promise, nil)
			},
		},
		{
			name: "invented durable head for context row",
			mutate: func(t *testing.T, f *closeoutFixture) {
				f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"})
				context := fixtureRecord(record.Promise, "prd/widget", "WR-1", 25)
				f.replaceRecord(t, context, nil)
			},
		},
		{
			name: "unserved PRD head",
			mutate: func(t *testing.T, f *closeoutFixture) {
				f.decision.Relations = nil
				f.replaceRecord(t, f.decision, nil)
			},
		},
		{
			name: "serving design has no Decision",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				delete(f.files, f.decision.Path())
			},
		},
		{
			name: "missing exact source Decision",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				delete(f.files, f.sourceDecision.Path())
			},
		},
		{
			name: "drifted exact source Decision",
			mutate: func(t *testing.T, f *closeoutFixture) {
				f.sourceDecision.Statement = "A different decision."
				f.replaceRecord(t, f.sourceDecision, nil)
			},
		},
		{
			name: "obsolete source Decision",
			mutate: func(t *testing.T, f *closeoutFixture) {
				obsolete := fixtureRecord(record.Decision, "design/widget-v1", "decision.removed.1", 25)
				obsolete.Tags = []string{designDecisionMemoryTag}
				f.replaceRecord(t, obsolete, nil)
			},
		},
		{
			name: "stale source Document",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				changed := []byte("changed PRD\n")
				file := f.snap.Files[f.prdPath]
				file.Body, file.SHA256 = changed, sourceDigest(changed)
				f.snap.Files[f.prdPath] = file
			},
		},
		{
			name: "missing design source Document",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				delete(f.files, f.designDoc.Path())
				delete(f.files, f.designDoc.BodyPath())
			},
		},
		{
			name: "plan and anchor are not semantic code memory",
			mutate: func(t *testing.T, f *closeoutFixture) {
				f.decision.Tags = nil
				f.replaceRecord(t, f.decision, nil)
				anchor := fixtureRecord(record.Need, "code/internal.widget", "anchor", 30)
				anchor.Statement = "anchor"
				anchor.Tags = []string{"code:internal/widget", "reconcile:anchor"}
				f.replaceRecord(t, anchor, nil)
				plan := fixtureRecord(record.Plan, "design/widget-v1", "", 31)
				plan.Statement = "implementation plan"
				plan.Relations = []record.Relation{{Type: record.About, Target: anchor.ID}}
				f.replaceRecord(t, plan, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseoutFixture(t)
			tt.mutate(t, f)
			set, proj := f.load(t)
			draft, _, err := BuildCloseout("widget", f.snap, set, proj)
			if !errors.Is(err, ErrFeatureIncomplete) {
				t.Fatalf("error = %v, want ErrFeatureIncomplete", err)
			}
			if !reflect.DeepEqual(draft, Draft{}) {
				t.Fatalf("incomplete feature returned a draft: %#v", draft)
			}
		})
	}
}

func TestBuildCloseout_RequiresExactDesignDeclarationForEveryServesEdge(t *testing.T) {
	setup := func(t *testing.T) *closeoutFixture {
		t.Helper()
		f := newCloseoutFixture(t)
		unit := PRDUnit{
			ID: "FR-2", Prefix: "FR", Status: "live", Kind: record.Promise,
			Statement:   "The system shall provide advanced widgets.",
			Consequence: record.High, SourceSHA256: strings.Repeat("2", 64), Mapped: true,
		}
		f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, unit)
		promise := prdMemoryRecord(unit, 9)
		f.replaceRecord(t, promise, nil)
		f.decision.Relations = append(f.decision.Relations, record.Relation{Type: record.Serves, Target: promise.ID})
		f.replaceRecord(t, f.decision, nil)
		return f
	}

	t.Run("extra memory edge absent from source declaration", func(t *testing.T) {
		f := setup(t)
		set, proj := f.load(t)
		_, _, err := BuildCloseout("widget", f.snap, set, proj)
		if !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "does not declare exact reference prd:widget/FR-2") {
			t.Fatalf("error = %v, want exact-family declaration failure", err)
		}
	})

	t.Run("every edge declared by live indexed design", func(t *testing.T) {
		f := setup(t)
		f.snap.Designs[0].Serves = append(f.snap.Designs[0].Serves, "prd:widget/FR-2")
		set, proj := f.load(t)
		if _, _, err := BuildCloseout("widget", f.snap, set, proj); err != nil {
			t.Fatalf("BuildCloseout: %v", err)
		}
	})
}

func TestBuildCloseout_RejectsUndeclaredCrossFeaturePRDEdge(t *testing.T) {
	f := newCloseoutFixture(t)
	unit := PRDUnit{
		ID: "FR-1", Prefix: "FR", Status: "live", Kind: record.Promise,
		Statement: "The system shall provide other behavior.", SourceSHA256: strings.Repeat("4", 64), Mapped: true,
	}
	f.snap.PRDs = append(f.snap.PRDs, PRD{Feature: "other", Units: []PRDUnit{unit}})
	target := prdMemoryRecord(unit, 9)
	target.Subject = "prd/other"
	f.replaceRecord(t, target, nil)
	f.decision.Relations = append(f.decision.Relations, record.Relation{Type: record.Serves, Target: target.ID})
	f.replaceRecord(t, f.decision, nil)
	set, proj := f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "does not declare exact reference prd:other/FR-1") {
		t.Fatalf("cross-feature unauthorized edge error = %v", err)
	}
}

func TestBuildCloseout_RejectsCurrentFeatureEdgeFromUnselectedDesign(t *testing.T) {
	f := newCloseoutFixture(t)
	f.snap.Designs = append(f.snap.Designs, Design{Slug: "rogue", Status: "live"})
	rogue := fixtureRecord(record.Decision, "design/rogue", "architecture", 51)
	rogue.Relations = []record.Relation{{Type: record.Serves, Target: f.promise.ID}}
	f.replaceRecord(t, rogue, nil)
	set, proj := f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "does not declare that feature in Serves metadata") {
		t.Fatalf("unselected design edge error = %v", err)
	}
}

func TestBuildCloseout_IgnoresExactLiveWorkflowContextTarget(t *testing.T) {
	f := newCloseoutFixture(t)
	f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"})
	f.snap.Designs[0].Serves = append(f.snap.Designs[0].Serves, "prd:widget/WR-1")
	set, proj := f.load(t)
	if _, receipt, err := BuildCloseout("widget", f.snap, set, proj); err != nil {
		t.Fatalf("live WR context required a memory head or serves edge: %v", err)
	} else if len(receipt.Designs) != 1 || receipt.Designs[0].Slug != "widget-v1" || len(receipt.NativeRecords) != 3 || len(receipt.CodeUnits) != 1 || len(receipt.CodeUnits[0].MemoryRecords) != 1 || receipt.CodeUnits[0].MemoryRecords[0] != f.decision.ID {
		t.Fatalf("WR-bearing design was not fully sealed: %#v", receipt)
	}

	f.snap.PRDs[0].Units[len(f.snap.PRDs[0].Units)-1].Status = "retired"
	set, proj = f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "exact live mapped PRD head is absent") {
		t.Fatalf("retired WR reference error = %v, want fail-closed stale header", err)
	}
}

func TestBuildCloseout_SealsContextOnlyDesign(t *testing.T) {
	f := newCloseoutFixture(t)
	f.snap.PRDs[0].Units[0].Priority = "Should"
	f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"})
	f.snap.Designs[0].Serves = []string{"prd:widget/WR-1"}
	f.decision.Relations = nil
	f.replaceRecord(t, f.decision, nil)
	set, proj := f.load(t)
	_, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("context-only design could not be sealed: %v", err)
	}
	if len(receipt.Designs) != 1 || receipt.Designs[0].Slug != "widget-v1" {
		t.Fatalf("context-only design missing from receipt: %#v", receipt.Designs)
	}
	if len(receipt.NativeRecords) != 3 || !recordInCloseoutBindings(f.decision.ID, receipt.NativeRecords) || !recordInCloseoutBindings(f.sourceDecision.ID, receipt.NativeRecords) {
		t.Fatalf("context-only design Decision missing from native bindings: %#v", receipt.NativeRecords)
	}
	if len(receipt.CodeUnits) != 1 || len(receipt.CodeUnits[0].MemoryRecords) != 1 || receipt.CodeUnits[0].MemoryRecords[0] != f.decision.ID {
		t.Fatalf("context-only design code binding missing: %#v", receipt.CodeUnits)
	}
}

func recordInCloseoutBindings(id string, bindings []CloseoutNativeRecord) bool {
	for _, binding := range bindings {
		if binding.ID == id {
			return true
		}
	}
	return false
}

func TestBuildCloseout_CodeScopeIsDirectionalAndAnchorMustBeActive(t *testing.T) {
	t.Run("exact tracked file selects its package", func(t *testing.T) {
		f := newCloseoutFixture(t)
		f.decision.Tags = []string{"code:internal/widget/widget.go"}
		f.replaceRecord(t, f.decision, nil)
		set, proj := f.load(t)
		_, receipt, err := BuildCloseout("widget", f.snap, set, proj)
		if err != nil {
			t.Fatalf("BuildCloseout: %v", err)
		}
		if len(receipt.CodeUnits) != 1 || receipt.CodeUnits[0].Path != "internal/widget" {
			t.Fatalf("code units = %#v", receipt.CodeUnits)
		}
	})

	t.Run("nonexistent child does not select package", func(t *testing.T) {
		f := newCloseoutFixture(t)
		f.decision.Tags = []string{"code:internal/widget/missing.go"}
		f.replaceRecord(t, f.decision, nil)
		set, proj := f.load(t)
		_, _, err := BuildCloseout("widget", f.snap, set, proj)
		if !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "matches no production code unit") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("inactive about anchor lends no scope", func(t *testing.T) {
		f := newCloseoutFixture(t)
		f.decision.Tags = nil
		anchor := fixtureRecord(record.Need, "code/internal.widget", "anchor", 9)
		anchor.Statement = "old anchor"
		anchor.Tags = []string{"code:internal/widget", "reconcile:anchor"}
		f.replaceRecord(t, anchor, nil)
		f.decision.Relations = append(f.decision.Relations, record.Relation{Type: record.About, Target: anchor.ID})
		f.replaceRecord(t, f.decision, nil)
		successor := fixtureRecord(record.Need, anchor.Subject, anchor.Family, 31)
		successor.Statement = "replacement anchor without a code tag"
		successor.Relations = []record.Relation{{Type: record.Supersedes, Target: anchor.ID}}
		f.replaceRecord(t, successor, nil)
		set, proj := f.load(t)
		_, _, err := BuildCloseout("widget", f.snap, set, proj)
		if !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no production code unit") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestBuildCloseout_AllowsAnsweredQuestion(t *testing.T) {
	f := newCloseoutFixture(t)
	question := fixtureRecord(record.Question, "prd/widget", "", 25)
	question.Statement = "Which widget?"
	f.replaceRecord(t, question, nil)
	answer := fixtureRecord(record.Answer, "prd/widget", "", 26)
	answer.Statement = "The documented widget."
	answer.Relations = []record.Relation{{Type: record.Answers, Target: question.ID}}
	f.replaceRecord(t, answer, nil)
	set, proj := f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); err != nil {
		t.Fatalf("answered question blocked closeout: %v", err)
	}
}

func TestBuildCloseout_BindsExplicitPRDDispositions(t *testing.T) {
	f := newCloseoutFixture(t)
	prd := f.snap.PRDs[0]
	questionUnit := PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which widget?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("2", 64)}
	riskUnit := PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: strings.Repeat("3", 64)}
	f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, questionUnit, riskUnit)

	question := fixturePRDDisposition(record.Question, prd, questionUnit, 80)
	answer := fixturePRDAnswer(question, "prd/widget", 81)
	risk := fixturePRDDisposition(record.Decision, prd, riskUnit, 82)
	for _, r := range []*record.Record{question, answer, risk} {
		f.replaceRecord(t, r, nil)
	}

	set, proj := f.load(t)
	draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	if len(receipt.NativeRecords) != 6 {
		t.Fatalf("native bindings = %#v", receipt.NativeRecords)
	}
	want := map[string]struct {
		kind   string
		family string
	}{
		question.ID: {kind: record.Question, family: questionUnit.ID},
		answer.ID:   {kind: record.Answer},
		risk.ID:     {kind: record.Decision, family: riskUnit.ID},
	}
	for _, binding := range receipt.NativeRecords {
		expect, ok := want[binding.ID]
		if !ok {
			continue
		}
		if binding.Kind != expect.kind || binding.Subject != "prd/widget" || binding.Family != expect.family || !validDigest(binding.SHA256) {
			t.Errorf("disposition binding = %#v, want kind=%s family=%s", binding, expect.kind, expect.family)
		}
		delete(want, binding.ID)
	}
	if len(want) != 0 {
		t.Fatalf("receipt omitted disposition records: %#v", want)
	}

	parsed, err := ParseCloseout(draft.Body)
	if err != nil {
		t.Fatalf("ParseCloseout: %v", err)
	}
	if !reflect.DeepEqual(parsed.NativeRecords, receipt.NativeRecords) {
		t.Fatalf("serialized receipt lost disposition bindings: got %#v want %#v", parsed.NativeRecords, receipt.NativeRecords)
	}
}

func TestBuildCloseout_RejectsIncompletePRDDispositions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		answers int
		want    string
	}{
		{name: "unanswered question", want: "has no active Answer"},
		{name: "duplicate active answers", answers: 2, want: "1 answer conflicts"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseoutFixture(t)
			prd := f.snap.PRDs[0]
			unit := PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which widget?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("2", 64)}
			f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, unit)
			question := fixturePRDDisposition(record.Question, prd, unit, 83)
			f.replaceRecord(t, question, nil)
			for i := 0; i < tt.answers; i++ {
				f.replaceRecord(t, fixturePRDAnswer(question, "prd/widget", byte(84+i)), nil)
			}

			set, proj := f.load(t)
			draft, _, err := BuildCloseout("widget", f.snap, set, proj)
			if !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildCloseout error = %v, want %q", err, tt.want)
			}
			if !reflect.DeepEqual(draft, Draft{}) {
				t.Fatalf("incomplete disposition returned a closeout draft: %#v", draft)
			}
		})
	}

	t.Run("risk without rationale", func(t *testing.T) {
		f := newCloseoutFixture(t)
		prd := f.snap.PRDs[0]
		unit := PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: strings.Repeat("3", 64)}
		f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, unit)
		risk := fixturePRDDisposition(record.Decision, prd, unit, 86)
		risk.Rationale = ""
		f.replaceRecord(t, risk, nil)
		set, proj := f.load(t)
		if draft, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "with rationale and fit criterion") || !reflect.DeepEqual(draft, Draft{}) {
			t.Fatalf("BuildCloseout = draft %#v, error %v", draft, err)
		}
	})

	t.Run("retired question keeps active disposition", func(t *testing.T) {
		f := newCloseoutFixture(t)
		prd := f.snap.PRDs[0]
		unit := PRDUnit{ID: "Q-1", Prefix: "Q", Status: "superseded", Statement: "Which widget?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("2", 64)}
		f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, unit)
		f.replaceRecord(t, fixturePRDDisposition(record.Question, prd, unit, 87), nil)
		set, proj := f.load(t)
		if draft, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "still has active native disposition") || !reflect.DeepEqual(draft, Draft{}) {
			t.Fatalf("BuildCloseout = draft %#v, error %v", draft, err)
		}
	})

	for _, tt := range []struct {
		name string
		unit PRDUnit
	}{
		{name: "live question missing source digest", unit: PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live"}},
		{name: "live risk with invalid source digest", unit: PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: "not-a-digest"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseoutFixture(t)
			f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, tt.unit)
			set, proj := f.load(t)
			if draft, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "invalid native disposition") || !strings.Contains(err.Error(), "no exact source-unit digest") || !reflect.DeepEqual(draft, Draft{}) {
				t.Fatalf("BuildCloseout = draft %#v, error %v", draft, err)
			}
		})
	}
}

func TestBuildCloseout_DoesNotInferFeatureFromNamesOrProse(t *testing.T) {
	f := newCloseoutFixture(t)
	extraBody := []byte("# Widget Extra\n\nMentions internal/widget and prd/widget.\n")
	extraPath := "docs/tech-design/widget-extra.md"
	f.snap.Files[extraPath] = File{Path: extraPath, Mode: "100644", SHA256: sourceDigest(extraBody), Body: extraBody}
	f.snap.AuthoredSources = append(f.snap.AuthoredSources, extraPath)
	f.snap.Designs = append(f.snap.Designs, Design{Slug: "widget-extra", Source: extraPath, SHA256: sourceDigest(extraBody), Status: "live"})
	extraDoc := fixtureDocument("design/widget-extra", "design", extraPath, extraBody, 40)
	f.replaceRecord(t, extraDoc, extraBody)
	extraDecision := fixtureRecord(record.Decision, "design/widget-extra", "choice", 41)
	extraDecision.Statement = "Use internal/widget while discussing prd/widget."
	extraDecision.Provenance.Source = "internal/widget/widget.go"
	f.replaceRecord(t, extraDecision, nil)

	set, proj := f.load(t)
	_, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	if len(receipt.Designs) != 1 || receipt.Designs[0].Slug != "widget-v1" {
		t.Fatalf("similar-name design was selected: %#v", receipt.Designs)
	}
	for _, binding := range receipt.NativeRecords {
		if binding.Subject == "design/widget-extra" {
			t.Fatalf("prose/provenance inferred code or feature association: %#v", binding)
		}
	}
}

func TestBuildCloseout_IgnoresHistoricalSupersededServingDesign(t *testing.T) {
	f := newCloseoutFixture(t)
	oldBody := []byte("# Superseded widget design\n")
	oldPath := "docs/tech-design/widget-v0.md"
	f.snap.Files[oldPath] = File{Path: oldPath, Mode: "100644", SHA256: sourceDigest(oldBody), Body: oldBody}
	f.snap.AuthoredSources = append(f.snap.AuthoredSources, oldPath)
	f.snap.Designs = append(f.snap.Designs, Design{
		Slug: "widget-v0", Source: oldPath, SHA256: sourceDigest(oldBody), Status: "superseded",
		CurrentSuccessor: "widget-v1", Serves: []string{"prd:widget/FR-1"},
	})
	oldDoc := fixtureDocument("design/widget-v0", "design", oldPath, oldBody, 42)
	f.replaceRecord(t, oldDoc, oldBody)

	set, proj := f.load(t)
	_, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	if len(receipt.Designs) != 1 || receipt.Designs[0].Slug != "widget-v1" {
		t.Fatalf("superseded design was selected: %#v", receipt.Designs)
	}
}

func TestParseCloseout_StrictClosedCanonicalReceipt(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		body []byte
	}{
		{name: "unknown field", body: bytes.Replace(draft.Body, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)},
		{name: "trailing value", body: append(append([]byte(nil), draft.Body...), []byte("{}\n")...)},
		{name: "noncanonical compact JSON", body: []byte(strings.ReplaceAll(string(draft.Body), "  ", ""))},
		{name: "noncanonical CRLF", body: bytes.ReplaceAll(draft.Body, []byte("\n"), []byte("\r\n"))},
		{name: "wrong closure", body: func() []byte {
			bad := receipt
			bad.ClosureSHA256 = strings.Repeat("0", 64)
			body, marshalErr := jsonMarshalIndentForTest(bad)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			return body
		}()},
		{name: "unsorted sources", body: func() []byte {
			bad := receipt
			bad.SourceDocuments[0], bad.SourceDocuments[1] = bad.SourceDocuments[1], bad.SourceDocuments[0]
			bad.ClosureSHA256 = closeoutDigest(bad)
			body, marshalErr := jsonMarshalIndentForTest(bad)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			return body
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseCloseout(tt.body); !errors.Is(err, ErrInvalidCloseout) {
				t.Fatalf("error = %v, want ErrInvalidCloseout", err)
			}
		})
	}
}

func TestActiveCloseouts_InvalidReceiptFailsSealedPathsClosed(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)

	badBody := []byte("{\"not\":\"a receipt\"}\n")
	bad := fixtureDocument("closeout/broken", "seal", "memory-closeout:broken", badBody, 91)
	bad.Tags = []string{"reconcile:closeout", "sha256:" + strings.Repeat("0", 64)}
	bad.Provenance.Commit = f.snap.Commit
	f.replaceRecord(t, bad, badBody)
	badSet, badProj := f.load(t)
	if _, err := ActiveCloseouts(badSet, badProj); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("ActiveCloseouts error = %v", err)
	}
	if got := sealedSourcePaths(badSet, badProj); len(got) != 0 {
		t.Fatalf("malformed active receipt granted partial seals: %#v", got)
	}
}

func TestActiveCloseouts_SourceBindingMustRemainExact(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)
	// Removing a bound immutable envelope makes both receipt enumeration and
	// source-retirement authorization fail closed.
	delete(f.files, f.prdDoc.Path())
	delete(f.files, f.prdDoc.BodyPath())
	brokenSet, brokenProj := f.load(t)
	if _, err := ActiveCloseouts(brokenSet, brokenProj); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("ActiveCloseouts error = %v", err)
	}
	if got := sealedSourcePaths(brokenSet, brokenProj); len(got) != 0 {
		t.Fatalf("broken binding granted seals: %#v", got)
	}
}

func TestActiveCloseouts_NonActiveSourceFailsClosedButCanBeResealed(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	seal := f.addDraft(t, draft, 90)

	replacement := fixtureDocument("prd/widget", "base", f.prdPath, f.snap.Files[f.prdPath].Body, 91)
	replacement.Relations = []record.Relation{{Type: record.Supersedes, Target: f.prdDoc.ID}}
	f.replaceRecord(t, replacement, f.snap.Files[f.prdPath].Body)
	set, proj = f.load(t)
	if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("ActiveCloseouts error = %v", err)
	}
	if got := sealedSourcePaths(set, proj); len(got) != 0 {
		t.Fatalf("inactive source binding authorized absence: %#v", got)
	}

	reseal, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout replacement: %v", err)
	}
	if len(reseal.Relations) != 1 || reseal.Relations[0] != (record.Relation{Type: record.Supersedes, Target: seal.ID}) {
		t.Fatalf("replacement seal relation = %#v", reseal.Relations)
	}
	if receipt.SourceDocuments[0].DocumentID != replacement.ID {
		t.Fatalf("replacement receipt did not bind current source: %#v", receipt.SourceDocuments)
	}
}

func TestBuildCloseout_CodeDriftCanReuseApplicableSemanticMemory(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	seal := f.addDraft(t, draft, 90)
	set, proj = f.load(t)

	f.snap.Code[0].SHA256 = strings.Repeat("9", 64)
	reseal, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout with existing semantics: %v", err)
	}
	if len(reseal.Relations) != 1 || reseal.Relations[0] != (record.Relation{Type: record.Supersedes, Target: seal.ID}) {
		t.Fatalf("replacement seal relation = %#v", reseal.Relations)
	}
	if got := receipt.CodeUnits[0].MemoryRecords; len(got) != 1 || got[0] != f.decision.ID {
		t.Fatalf("replacement code semantic bindings = %#v", got)
	}
	if receipt.CodeUnits[0].SHA256 != f.snap.Code[0].SHA256 {
		t.Fatal("replacement receipt did not bind the changed code")
	}
}

func TestBuildCloseout_DesignDriftRequiresNewDecision(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)

	changed := []byte("# Updated widget design\n\n## 3. Design\n\n**Decision:** Preserve updated exact widget behavior.\n")
	f.snap.Files[f.designPath] = File{Path: f.designPath, Mode: "100644", SHA256: sourceDigest(changed), Body: changed}
	f.snap.Designs[0].SHA256 = sourceDigest(changed)
	replacementDoc := fixtureDocument("design/widget-v1", "design", f.designPath, changed, 91)
	replacementDoc.Relations = []record.Relation{{Type: record.Supersedes, Target: f.designDoc.ID}}
	f.replaceRecord(t, replacementDoc, changed)
	set, proj = f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "drifted source Decision") {
		t.Fatalf("same-Decision reseal error = %v", err)
	}

	successor := fixtureRecord(record.Decision, f.decision.Subject, f.decision.Family, 92)
	successor.Statement = "The updated design still implements FR-1."
	successor.Intent = f.decision.Intent
	successor.Tags = append([]string(nil), f.decision.Tags...)
	successor.Relations = []record.Relation{
		{Type: record.Supersedes, Target: f.decision.ID},
		{Type: record.Serves, Target: f.promise.ID},
	}
	f.replaceRecord(t, successor, nil)
	blocks, err := ExtractLiveDesignDecisions(f.snap.Designs[0], f.snap.Files[f.designPath])
	if err != nil || len(blocks) != 1 {
		t.Fatalf("extract updated Decision: %#v, %v", blocks, err)
	}
	exactDraft, ok := designDecisionDraft(f.snap, blocks[0], replacementDoc)
	if !ok {
		t.Fatal("updated exact Decision produced no draft")
	}
	exactSuccessor := fixtureRecord(exactDraft.Kind, exactDraft.Subject, exactDraft.Family, 93)
	exactSuccessor.Statement = exactDraft.Statement
	exactSuccessor.Tags = append([]string(nil), exactDraft.Tags...)
	exactSuccessor.Relations = append(append([]record.Relation(nil), exactDraft.Relations...), record.Relation{Type: record.Supersedes, Target: f.sourceDecision.ID})
	exactSuccessor.Provenance = exactDraft.Provenance
	f.replaceRecord(t, exactSuccessor, nil)
	set, proj = f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); err != nil {
		t.Fatalf("BuildCloseout with current Decision: %v", err)
	}
}

func TestBuildCloseout_PRDIdentityDriftRequiresNewNativeMemory(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)

	f.snap.PRDs[0].Fingerprint = "sha256:" + strings.Repeat("8", 64)
	set, proj = f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "has no new native memory") {
		t.Fatalf("same-memory PRD reseal error = %v", err)
	}

	disposition := fixtureRecord(record.Need, "prd/widget", "context-refresh", 91)
	disposition.Statement = "The changed PRD context adds no uncaptured implementation obligation."
	f.replaceRecord(t, disposition, nil)
	set, proj = f.load(t)
	if _, _, err := BuildCloseout("widget", f.snap, set, proj); err != nil {
		t.Fatalf("BuildCloseout with current PRD memory: %v", err)
	}
}

func TestSealedSourcePaths_AuthorizeAbsenceButNeverStaleBytes(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)
	set, proj = f.load(t)

	absent := f.snap
	absent.Files = cloneFiles(f.snap.Files)
	delete(absent.Files, f.prdPath)
	absent.AuthoredSources = []string{f.designPath}
	report := Analyze(absent, set, proj, Options{})
	if hasFinding(report, "orphan-document-source", f.prdPath) {
		t.Fatalf("valid seal did not authorize absent source: %#v", report.Findings)
	}

	stale := f.snap
	stale.Files = cloneFiles(f.snap.Files)
	changed := []byte("changed after closeout\n")
	stale.Files[f.prdPath] = File{Path: f.prdPath, Mode: "100644", SHA256: sourceDigest(changed), Body: changed}
	report = Analyze(stale, set, proj, Options{})
	if !hasFinding(report, "stale-document", f.prdPath) {
		t.Fatalf("seal hid stale source bytes: %#v", report.Findings)
	}
}

type closeoutFixture struct {
	snap           Snapshot
	files          store.MapSource
	prdPath        string
	designPath     string
	promise        *record.Record
	decision       *record.Record
	sourceDecision *record.Record
	prdDoc         *record.Record
	designDoc      *record.Record
}

func newCloseoutFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	prdPath := "docs/prds/widget/000-base.md"
	designPath := "docs/tech-design/widget-v1.md"
	prdBody := []byte("# Widget PRD\n")
	designBody := []byte("# Widget design\n\n## 3. Design\n\n**Decision:** Preserve exact widget behavior.\n")
	goBody := []byte("package widget\n")

	promise := fixtureRecord(record.Promise, "prd/widget", "FR-1", 10)
	promise.Statement = "The system shall provide widgets."
	promise.Rationale = "Users need widgets."
	promise.FitCriterion = "A widget is produced."
	promise.Consequence = record.High
	sourceUnitSHA := strings.Repeat("1", 64)
	promise.Tags = []string{"reconcile:prd", "source-unit:" + sourceUnitSHA, "source:prd-ledger"}
	prdDoc := fixtureDocument("prd/widget", "base", prdPath, prdBody, 11)
	designDoc := fixtureDocument("design/widget-v1", "design", designPath, designBody, 12)
	decision := fixtureRecord(record.Decision, "design/widget-v1", "architecture", 13)
	decision.Statement = "Build the widget package."
	decision.Tags = []string{"code:internal/widget"}
	decision.Relations = []record.Relation{{Type: record.Serves, Target: promise.ID}}
	commit := strings.Repeat("a", 40)
	prdSHA, designSHA := sourceDigest(prdBody), sourceDigest(designBody)
	unit := codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "internal/widget", Files: []string{"internal/widget/widget.go"}, SHA256: sourceDigest(goBody)}
	design := Design{Slug: "widget-v1", Source: designPath, SHA256: designSHA, Status: "live", Serves: []string{"prd:widget/FR-1"}}
	designFile := File{Path: designPath, Mode: "100644", SHA256: designSHA, Body: designBody}
	blocks, err := ExtractLiveDesignDecisions(design, designFile)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("extract fixture Decision: %#v, %v", blocks, err)
	}
	sourceDraft, ok := designDecisionDraft(Snapshot{
		Commit: commit, Files: map[string]File{designPath: designFile}, Code: []codeinventory.Unit{unit},
	}, blocks[0], designDoc)
	if !ok {
		t.Fatal("build exact source Decision fixture")
	}
	sourceDecision := fixtureRecord(sourceDraft.Kind, sourceDraft.Subject, sourceDraft.Family, 14)
	sourceDecision.Statement = sourceDraft.Statement
	sourceDecision.Tags = append([]string(nil), sourceDraft.Tags...)
	sourceDecision.Relations = append([]record.Relation(nil), sourceDraft.Relations...)
	sourceDecision.Provenance = sourceDraft.Provenance

	f := &closeoutFixture{
		files:          store.MapSource{},
		prdPath:        prdPath,
		designPath:     designPath,
		promise:        promise,
		decision:       decision,
		sourceDecision: sourceDecision,
		prdDoc:         prdDoc,
		designDoc:      designDoc,
	}
	for _, item := range []struct {
		r    *record.Record
		body []byte
	}{{promise, nil}, {prdDoc, prdBody}, {designDoc, designBody}, {decision, nil}, {sourceDecision, nil}} {
		f.replaceRecord(t, item.r, item.body)
	}
	f.snap = Snapshot{
		Commit: commit, SourceClosureSHA256: strings.Repeat("b", 64),
		DesignIndexSHA256: strings.Repeat("c", 64), CodeInventorySHA256: strings.Repeat("d", 64),
		Files: map[string]File{
			prdPath:    {Path: prdPath, Mode: "100644", SHA256: prdSHA, Body: prdBody},
			designPath: {Path: designPath, Mode: "100644", SHA256: designSHA, Body: designBody},
		},
		AuthoredSources: []string{prdPath, designPath},
		Code:            []codeinventory.Unit{unit},
		PRDs: []PRD{{
			Feature: "widget", Source: "docs/prds/widget/effective.md", AsOf: "000",
			Fingerprint: "sha256:" + strings.Repeat("e", 64), SourceMapSHA256: strings.Repeat("f", 64),
			Units: []PRDUnit{{ID: "FR-1", Prefix: "FR", Status: "live", Kind: record.Promise, Statement: promise.Statement, Rationale: promise.Rationale, FitCriterion: promise.FitCriterion, Priority: "Must", Consequence: promise.Consequence, SourceSHA256: sourceUnitSHA, Mapped: true}},
		}},
		Designs: []Design{design},
	}
	return f
}

func (f *closeoutFixture) replaceRecord(t *testing.T, r *record.Record, body []byte) {
	t.Helper()
	if r.Kind == record.Document {
		r.BodySHA256 = record.HashBody(body)
		r.LiteralBody = true
	}
	encoded, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	f.files[r.Path()] = encoded
	if r.Kind == record.Document {
		f.files[r.BodyPath()] = append([]byte(nil), body...)
	}
}

func (f *closeoutFixture) addDraft(t *testing.T, draft Draft, seq byte) *record.Record {
	t.Helper()
	r := fixtureRecord(draft.Kind, draft.Subject, draft.Family, seq)
	r.Statement = draft.Statement
	r.Intent = draft.Intent
	r.Rationale = draft.Rationale
	r.FitCriterion = draft.FitCriterion
	r.Tags = append([]string(nil), draft.Tags...)
	r.Consequence = draft.Consequence
	r.Relations = append([]record.Relation(nil), draft.Relations...)
	r.Provenance = draft.Provenance
	r.LiteralBody = draft.LiteralBody
	f.replaceRecord(t, r, draft.Body)
	return r
}

func (f *closeoutFixture) load(t *testing.T) (*store.Set, *resolve.Projection) {
	t.Helper()
	set, err := store.Load(context.Background(), f.files)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Problems) > 0 {
		t.Fatalf("fixture memory problems: %#v", set.Problems)
	}
	return set, resolve.New(set)
}

func fixtureRecord(kind, subject, family string, seq byte) *record.Record {
	var entropy [10]byte
	entropy[9] = seq
	created := time.Date(2026, time.January, 1, 0, 0, int(seq), 0, time.UTC)
	return &record.Record{
		Schema: record.Schema, ID: record.NewID(kind, created, entropy), Kind: kind,
		Subject: subject, Family: family, Statement: "fixture", Issuer: "test", CreatedAt: created.Format(time.RFC3339),
	}
}

func fixtureDocument(subject, family, source string, body []byte, seq byte) *record.Record {
	r := fixtureRecord(record.Document, subject, family, seq)
	r.Statement = "Captured " + source
	r.LiteralBody = true
	r.BodySHA256 = record.HashBody(body)
	r.Tags = []string{"reconcile:source", "sha256:" + sourceDigest(body)}
	r.Provenance = record.Provenance{Source: source, Commit: strings.Repeat("a", 40)}
	return r
}

func jsonMarshalIndentForTest(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func cloneFiles(in map[string]File) map[string]File {
	out := make(map[string]File, len(in))
	for name, file := range in {
		file.Body = append([]byte(nil), file.Body...)
		out[name] = file
	}
	return out
}

func hasFinding(report Report, code, scope string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code && finding.Scope == scope {
			return true
		}
	}
	return false
}
