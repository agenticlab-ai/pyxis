package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestValidateCloseoutRejectsMalformedBindings(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	_, good, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		want   string
		mutate func(*CloseoutReceipt)
	}{
		{name: "schema", want: "schema_version", mutate: func(r *CloseoutReceipt) { r.Schema = "memory-closeout/v2" }},
		{name: "feature mismatch", want: "feature and PRD feature", mutate: func(r *CloseoutReceipt) { r.PRD.Feature = "other" }},
		{name: "source commit length", want: "source_commit", mutate: func(r *CloseoutReceipt) { r.SourceCommit = strings.Repeat("a", 39) }},
		{name: "source commit uppercase", want: "source_commit", mutate: func(r *CloseoutReceipt) { r.SourceCommit = strings.Repeat("A", 40) }},
		{name: "digest", want: "source_closure_sha256", mutate: func(r *CloseoutReceipt) { r.SourceClosureSHA256 = strings.Repeat("g", 64) }},
		{name: "PRD source", want: "PRD binding", mutate: func(r *CloseoutReceipt) { r.PRD.Source = "../escape.md" }},
		{name: "PRD as-of", want: "PRD binding", mutate: func(r *CloseoutReceipt) { r.PRD.AsOf = "" }},
		{name: "PRD fingerprint", want: "PRD binding", mutate: func(r *CloseoutReceipt) { r.PRD.Fingerprint = "not-a-fingerprint" }},
		{name: "empty collection", want: "must all be present and non-empty", mutate: func(r *CloseoutReceipt) { r.SourceDocuments = nil }},
		{name: "invalid source binding", want: "source_documents[0] is invalid", mutate: func(r *CloseoutReceipt) { r.SourceDocuments[0].Path = "../escape.md" }},
		{name: "duplicate source binding", want: "repeat a path or Document id", mutate: func(r *CloseoutReceipt) { r.SourceDocuments[1] = r.SourceDocuments[0] }},
		{name: "invalid native binding", want: "native_records[0] is invalid", mutate: func(r *CloseoutReceipt) { r.NativeRecords[0].Kind = record.Need }},
		{name: "duplicate native binding", want: "native_records repeat id", mutate: func(r *CloseoutReceipt) { r.NativeRecords[1] = r.NativeRecords[0] }},
		{name: "claim without family", want: "omits a claim family", mutate: func(r *CloseoutReceipt) { r.NativeRecords[0].Family = "" }},
		{name: "invalid native family", want: "invalid family", mutate: func(r *CloseoutReceipt) { r.NativeRecords[0].Family = "bad/family" }},
		{name: "unsorted native bindings", want: "native_records are not strictly sorted", mutate: func(r *CloseoutReceipt) {
			r.NativeRecords[0], r.NativeRecords[1] = r.NativeRecords[1], r.NativeRecords[0]
		}},
		{name: "invalid code binding", want: "code_units[0] is invalid", mutate: func(r *CloseoutReceipt) { r.CodeUnits[0].Kind = "binary" }},
		{name: "unsupported script extension", want: "do not identify the executable script", mutate: func(r *CloseoutReceipt) {
			r.CodeUnits[0].Kind = codeinventory.ExecutableScript
			r.CodeUnits[0].Path = ".pyxis/scripts/startup/codex-worktree.fish"
			r.CodeUnits[0].Files = []string{r.CodeUnits[0].Path}
		}},
		{name: "code without memory", want: "has no semantic memory_records", mutate: func(r *CloseoutReceipt) { r.CodeUnits[0].MemoryRecords = nil }},
		{name: "code with unbound memory", want: "is not a bound native record", mutate: func(r *CloseoutReceipt) {
			r.CodeUnits[0].MemoryRecords = []string{record.NewID(record.Decision, fixtureTime(99), [10]byte{9: 99})}
		}},
		{name: "unsorted code memory", want: "memory_records are not strictly sorted", mutate: func(r *CloseoutReceipt) {
			r.CodeUnits[0].MemoryRecords = []string{r.NativeRecords[1].ID, r.NativeRecords[0].ID}
		}},
		{name: "duplicate code path", want: "code_units repeat path", mutate: func(r *CloseoutReceipt) {
			r.CodeUnits = append(r.CodeUnits, r.CodeUnits[0])
		}},
		{name: "unsorted code bindings", want: "code_units are not strictly sorted", mutate: func(r *CloseoutReceipt) {
			second := r.CodeUnits[0]
			second.Path = "internal/alpha"
			second.Files = []string{"internal/alpha/alpha.go"}
			r.CodeUnits = append(r.CodeUnits, second)
		}},
		{name: "invalid design binding", want: "designs[0] is invalid", mutate: func(r *CloseoutReceipt) { r.Designs[0].Slug = "bad/slug" }},
		{name: "duplicate design source", want: "designs repeat source", mutate: func(r *CloseoutReceipt) {
			second := r.Designs[0]
			second.Slug = "widget-v2"
			r.Designs = append(r.Designs, second)
		}},
		{name: "unsorted designs", want: "designs are not strictly sorted", mutate: func(r *CloseoutReceipt) {
			second := r.Designs[0]
			second.Slug = "alpha"
			second.Source = "docs/tech-design/alpha.md"
			r.Designs = append(r.Designs, second)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad := cloneCloseoutForCoverage(t, good)
			tt.mutate(&bad)
			bad.ClosureSHA256 = closeoutDigest(bad)
			err := ValidateCloseout(bad)
			if !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateCloseout error = %v, want ErrInvalidCloseout containing %q", err, tt.want)
			}
		})
	}

	bad := cloneCloseoutForCoverage(t, good)
	bad.Schema = "wrong"
	bad.ClosureSHA256 = closeoutDigest(bad)
	if _, err := MarshalCloseout(bad); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("MarshalCloseout malformed receipt error = %v", err)
	}

	zsh := cloneCloseoutForCoverage(t, good)
	zsh.CodeUnits[0].Kind = codeinventory.ExecutableScript
	zsh.CodeUnits[0].Path = ".pyxis/scripts/startup/codex-worktree.zsh"
	zsh.CodeUnits[0].Files = []string{zsh.CodeUnits[0].Path}
	zsh.ClosureSHA256 = closeoutDigest(zsh)
	if err := ValidateCloseout(zsh); err != nil {
		t.Fatalf("ValidateCloseout sourced zsh binding: %v", err)
	}
}

func TestBuildCloseoutRejectsUntrustworthyInputs(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	question := fixtureRecord(record.Question, "prd/widget", "", 70)
	answerA := fixturePRDAnswer(question, question.Subject, 71)
	answerB := fixturePRDAnswer(question, question.Subject, 72)
	answerConflictSet, answerConflictProjection := loadReconcileRecords(t,
		reconcileRecord{record: question}, reconcileRecord{record: answerA}, reconcileRecord{record: answerB})
	if _, err := ActiveCloseouts(answerConflictSet, answerConflictProjection); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "1 answer conflicts") {
		t.Fatalf("ActiveCloseouts answer-conflict error = %v", err)
	}
	target := fixtureRecord(record.Need, "prd/widget", "G-2", 73)
	revokeA := fixtureRecord(record.Revocation, target.Subject, "", 74)
	revokeA.Relations = []record.Relation{{Type: record.Revokes, Target: target.ID}}
	revokeB := fixtureRecord(record.Revocation, target.Subject, "", 75)
	revokeB.Relations = []record.Relation{{Type: record.Revokes, Target: target.ID}}
	revocationConflictSet, revocationConflictProjection := loadReconcileRecords(t,
		reconcileRecord{record: target}, reconcileRecord{record: revokeA}, reconcileRecord{record: revokeB})
	if _, err := ActiveCloseouts(revocationConflictSet, revocationConflictProjection); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "1 revocation conflicts") {
		t.Fatalf("ActiveCloseouts revocation-conflict error = %v", err)
	}

	for _, tc := range []struct {
		name    string
		feature string
		mutate  func(*Snapshot)
		set     *store.Set
		proj    *resolve.Projection
		want    string
	}{
		{name: "invalid feature", feature: "Bad/Feature", set: set, proj: proj, want: "canonical feature slug"},
		{name: "nil memory", feature: "widget", set: nil, proj: proj, want: "unavailable or unrelated"},
		{name: "unrelated projection", feature: "widget", set: set, proj: resolve.New(&store.Set{Records: map[string]*record.Record{}}), want: "unavailable or unrelated"},
		{name: "answer conflict", feature: "widget", set: answerConflictSet, proj: answerConflictProjection, want: "1 answer conflicts"},
		{name: "revocation conflict", feature: "widget", set: revocationConflictSet, proj: revocationConflictProjection, want: "1 revocation conflicts"},
		{name: "invalid snapshot", feature: "widget", set: set, proj: proj, mutate: func(s *Snapshot) { s.Commit = "short" }, want: "snapshot identity is incomplete"},
		{name: "compiler blocker", feature: "widget", set: set, proj: proj, mutate: func(s *Snapshot) {
			s.CompilerFindings = []Finding{{Feature: "widget", Scope: "prd/widget#FR-1", Detail: "diverged", Blocking: true}}
		}, want: "compiled feature contract is inconsistent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := f.snap
			if tc.mutate != nil {
				tc.mutate(&snapshot)
			}
			_, _, err := BuildCloseout(tc.feature, snapshot, tc.set, tc.proj)
			if !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("BuildCloseout error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAlignedPRDHeadsRejectsMalformedOrMissingHeads(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	base := f.snap.PRDs[0]

	duplicate := base
	duplicate.Units = append(append([]PRDUnit(nil), base.Units...), base.Units[0])
	if _, _, err := alignedPRDHeads(duplicate, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "duplicate or empty unit") {
		t.Fatalf("duplicate unit error = %v", err)
	}

	nonLive := base
	nonLive.Units = append([]PRDUnit(nil), base.Units...)
	nonLive.Units[0].Status = "superseded"
	if _, _, err := alignedPRDHeads(nonLive, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "still has active memory head") {
		t.Fatalf("non-live active head error = %v", err)
	}

	emptySet, err := store.Load(context.Background(), store.MapSource{})
	if err != nil {
		t.Fatal(err)
	}
	emptyProj := resolve.New(emptySet)
	if _, _, err := alignedPRDHeads(nonLive, emptyProj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no live mapped PRD units") {
		t.Fatalf("only non-live unit error = %v", err)
	}

	allowedContext := base
	allowedContext.Units = append(append([]PRDUnit(nil), base.Units...), PRDUnit{ID: "NG-1", Prefix: "NG", Status: "live"})
	if heads, _, err := alignedPRDHeads(allowedContext, proj); err != nil || len(heads) != 1 {
		t.Fatalf("allowed contextual unit = heads %v, error %v", heads, err)
	}
	contextHead := fixtureRecord(record.Promise, "prd/widget", "NG-1", 61)
	contextSet, contextProj := loadReconcileRecords(t, reconcileRecord{record: f.promise}, reconcileRecord{record: contextHead})
	if _, _, err := alignedPRDHeads(allowedContext, contextProj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "context unit NG-1 unexpectedly has active memory head") {
		t.Fatalf("context-head error = %v", err)
	}
	_ = contextSet

	missingDigest := base
	missingDigest.Units = append([]PRDUnit(nil), base.Units...)
	missingDigest.Units[0].SourceSHA256 = ""
	if _, _, err := alignedPRDHeads(missingDigest, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no exact source-unit digest") {
		t.Fatalf("source digest error = %v", err)
	}

	if _, _, err := alignedPRDHeads(base, emptyProj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no active memory head") {
		t.Fatalf("missing active head error = %v", err)
	}

	_ = set
}

func TestCloseoutBindingFailures(t *testing.T) {
	t.Run("design fingerprints", func(t *testing.T) {
		f := newCloseoutFixture(t)
		bad := f.snap.Designs[0]
		bad.SHA256 = ""
		if err := validateDesignSource(f.snap, bad); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no exact source fingerprint") {
			t.Fatalf("missing design fingerprint error = %v", err)
		}
		bad = f.snap.Designs[0]
		bad.Source = "docs/tech-design/missing.md"
		if err := validateDesignSource(f.snap, bad); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "does not match its pinned source") {
			t.Fatalf("missing design source error = %v", err)
		}
		bad = f.snap.Designs[0]
		snapshot := f.snap
		snapshot.DesignIndexSHA256 = ""
		if err := validateDesignSource(snapshot, bad); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "design index identity is unavailable") {
			t.Fatalf("missing design index error = %v", err)
		}
	})

	t.Run("source corpus", func(t *testing.T) {
		f := newCloseoutFixture(t)
		set, proj := f.load(t)
		prd := f.snap.PRDs[0]
		subjects := map[string]bool{"prd/widget": true, "design/widget-v1": true}

		snapshot := f.snap
		snapshot.AuthoredSources = []string{f.designPath}
		if _, err := bindCurrentSources(snapshot, prd, subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "no pinned authored PRD ledger sources") {
			t.Fatalf("missing PRD ledger source error = %v", err)
		}

		snapshot = f.snap
		snapshot.Files = cloneFiles(f.snap.Files)
		file := snapshot.Files[f.prdPath]
		file.Path = "wrong/path.md"
		snapshot.Files[f.prdPath] = file
		if _, err := bindCurrentSources(snapshot, prd, subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "absent or internally inconsistent") {
			t.Fatalf("inconsistent source error = %v", err)
		}

		extraBody := []byte("extra\n")
		extra := fixtureDocument("prd/widget", "extra", "notes/extra.md", extraBody, 70)
		f.replaceRecord(t, extra, extraBody)
		set, proj = f.load(t)
		if _, err := bindCurrentSources(f.snap, prd, subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "is not selected feature context") {
			t.Fatalf("untracked selected source error = %v", err)
		}

		snapshot = f.snap
		snapshot.Files = cloneFiles(f.snap.Files)
		snapshot.Files[extra.Provenance.Source] = File{Path: extra.Provenance.Source, Mode: "100644", SHA256: sourceDigest(extraBody), Body: extraBody}
		snapshot.AuthoredSources = append(append([]string(nil), f.snap.AuthoredSources...), extra.Provenance.Source)
		if _, err := bindCurrentSources(snapshot, prd, subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "is not selected feature context") {
			t.Fatalf("extra selected-subject source was accepted: %v", err)
		}
	})

	t.Run("missing envelopes", func(t *testing.T) {
		f := newCloseoutFixture(t)
		set, proj := f.load(t)
		delete(set.Bytes, set.Paths[f.prdDoc.ID])
		subjects := map[string]bool{"prd/widget": true, "design/widget-v1": true}
		if _, err := bindCurrentSources(f.snap, f.snap.PRDs[0], subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "bind source Document") {
			t.Fatalf("missing source envelope error = %v", err)
		}

		f = newCloseoutFixture(t)
		set, proj = f.load(t)
		delete(set.Bytes, set.Paths[f.promise.ID])
		if _, _, err := bindNativeRecords(subjects, set, proj); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "bind native record") {
			t.Fatalf("missing native envelope error = %v", err)
		}
	})

	t.Run("invalid code inventory", func(t *testing.T) {
		f := newCloseoutFixture(t)
		set, proj := f.load(t)
		native, records, err := bindNativeRecords(map[string]bool{"prd/widget": true, "design/widget-v1": true}, set, proj)
		if err != nil || len(native) == 0 {
			t.Fatalf("bind fixture native records: %v", err)
		}
		bad := append([]codeinventory.Unit(nil), f.snap.Code...)
		bad = append(bad, bad[0])
		if _, err := bindCodeUnits(bad, map[string]bool{"prd/widget": true, "design/widget-v1": true}, set, proj, records); !errors.Is(err, ErrFeatureIncomplete) || !strings.Contains(err.Error(), "invalid or duplicate unit") {
			t.Fatalf("duplicate code unit error = %v", err)
		}
	})
}

func TestValidateReplacementEvidenceRequiresRetiredCodeAssociation(t *testing.T) {
	id := record.NewID(record.Decision, fixtureTime(80), [10]byte{9: 80})
	unit := CloseoutCodeUnit{Kind: codeinventory.GoPackage, Path: "internal/widget", SHA256: strings.Repeat("a", 64), MemoryRecords: []string{id}}
	before := CloseoutReceipt{
		NativeRecords: []CloseoutNativeRecord{{ID: id}},
		CodeUnits:     []CloseoutCodeUnit{unit},
	}
	after := cloneCloseoutForCoverage(t, before)
	after.CodeUnits = nil
	if err := validateReplacementEvidence(before, after); err == nil || !strings.Contains(err.Error(), "removed code unit internal/widget retains every prior semantic association") {
		t.Fatalf("retained removed-code association error = %v", err)
	}
	after.NativeRecords = nil
	if err := validateReplacementEvidence(before, after); err != nil {
		t.Fatalf("retired association should authorize removed code unit: %v", err)
	}
}

func TestValidateReceiptRecordsDetectsBindingDrift(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	_, good, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}

	bad := cloneCloseoutForCoverage(t, good)
	bad.SourceDocuments[0].RecordSHA256 = strings.Repeat("0", 64)
	if err := validateReceiptRecords(bad, "receipt", set); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "envelope hash differs") {
		t.Fatalf("source envelope drift error = %v", err)
	}

	bad = cloneCloseoutForCoverage(t, good)
	bad.Designs = append(bad.Designs, CloseoutDesign{Slug: "other", Source: "docs/tech-design/other.md", SHA256: strings.Repeat("1", 64)})
	if err := validateReceiptRecords(bad, "receipt", set); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "selected subject design/other has no bound source Document") {
		t.Fatalf("missing selected-subject source error = %v", err)
	}

	bad = cloneCloseoutForCoverage(t, good)
	bad.NativeRecords[0].Subject = "design/other"
	if err := validateReceiptRecords(bad, "receipt", set); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "native binding") {
		t.Fatalf("native identity drift error = %v", err)
	}

	bad = cloneCloseoutForCoverage(t, good)
	bad.NativeRecords[0].SHA256 = strings.Repeat("0", 64)
	if err := validateReceiptRecords(bad, "receipt", set); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "envelope hash differs") {
		t.Fatalf("native envelope drift error = %v", err)
	}

	if _, err := storedRecordDigest(&store.Set{Paths: map[string]string{}, Bytes: map[string][]byte{}}, "missing"); err == nil || !strings.Contains(err.Error(), "has no immutable envelope bytes") {
		t.Fatalf("missing stored record error = %v", err)
	}
}

func TestActiveCloseoutsRejectsInvalidMemoryAndEnvelopes(t *testing.T) {
	if _, err := ActiveCloseouts(nil, nil); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("nil memory error = %v", err)
	}

	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	set.Problems = append(set.Problems, store.Problem{Path: "bad.json", Err: errors.New("bad")})
	if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "memory integrity is unresolved") {
		t.Fatalf("invalid memory error = %v", err)
	}

	f = newCloseoutFixture(t)
	badSeal := fixtureRecord(record.Need, "closeout/widget", "seal", 80)
	badSeal.Statement = "not a receipt"
	badSeal.Tags = []string{"reconcile:closeout"}
	f.replaceRecord(t, badSeal, nil)
	set, proj = f.load(t)
	if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "not a literal reconciliation Document") {
		t.Fatalf("bad seal kind error = %v", err)
	}

	f = newCloseoutFixture(t)
	set, proj = f.load(t)
	draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	draft.Tags = []string{"reconcile:closeout", "sha256:" + strings.Repeat("0", 64)}
	f.addDraft(t, draft, 81)
	set, proj = f.load(t)
	if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) || !strings.Contains(err.Error(), "envelope does not bind its receipt") {
		t.Fatalf("unbound receipt envelope error = %v (receipt %s)", err, receipt.ClosureSHA256)
	}
}

func TestActiveCloseoutsRejectsForgedCodeMemoryBindings(t *testing.T) {
	t.Run("bound semantic record without covering scope", func(t *testing.T) {
		f := newCloseoutFixture(t)
		set, proj := f.load(t)
		draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
		if err != nil {
			t.Fatal(err)
		}
		receipt.CodeUnits[0].MemoryRecords = []string{f.promise.ID}
		installForgedCoverageReceipt(t, f, draft, receipt, 90)
		set, proj = f.load(t)
		if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) {
			t.Fatalf("ActiveCloseouts accepted a semantic record whose explicit scope does not cover its code unit: %v", err)
		}
	})

	t.Run("bound nonsemantic Plan", func(t *testing.T) {
		f := newCloseoutFixture(t)
		plan := fixtureRecord(record.Plan, "design/widget-v1", "", 70)
		plan.Statement = "implementation scaffold"
		f.replaceRecord(t, plan, nil)
		set, proj := f.load(t)
		draft, receipt, err := BuildCloseout("widget", f.snap, set, proj)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := storedRecordDigest(set, plan.ID)
		if err != nil {
			t.Fatal(err)
		}
		receipt.NativeRecords = append(receipt.NativeRecords, CloseoutNativeRecord{
			ID: plan.ID, Kind: plan.Kind, Subject: plan.Subject, SHA256: hash,
		})
		sort.Slice(receipt.NativeRecords, func(i, j int) bool { return lessNative(receipt.NativeRecords[i], receipt.NativeRecords[j]) })
		receipt.CodeUnits[0].MemoryRecords = []string{plan.ID}
		installForgedCoverageReceipt(t, f, draft, receipt, 90)
		set, proj = f.load(t)
		if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) {
			t.Fatalf("ActiveCloseouts accepted a bound nonsemantic Plan as code memory: %v", err)
		}
	})
}

func TestVerifyOfflineReportsPRDAndDesignDrift(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		scope  string
		mutate func(*Snapshot)
	}{
		{name: "PRD identity", code: "closeout-prd-drift", scope: "prd/widget", mutate: func(s *Snapshot) {
			s.PRDs[0].Fingerprint = "sha256:" + strings.Repeat("0", 64)
		}},
		{name: "design identity", code: "closeout-design-drift", scope: "design/widget-v1", mutate: func(s *Snapshot) {
			s.Designs[0].Status = "superseded"
		}},
		{name: "new serving design", code: "closeout-design-unsealed", scope: "design/widget-v2", mutate: func(s *Snapshot) {
			s.Designs = append(s.Designs, Design{Slug: "widget-v2", Status: "live", Serves: []string{"prd:widget/FR-1"}})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseoutFixture(t)
			set, proj := f.load(t)
			draft, _, err := BuildCloseout("widget", f.snap, set, proj)
			if err != nil {
				t.Fatal(err)
			}
			f.addDraft(t, draft, 90)
			set, proj = f.load(t)
			snapshot := f.snap
			snapshot.PRDs = append([]PRD(nil), f.snap.PRDs...)
			snapshot.Designs = append([]Design(nil), f.snap.Designs...)
			tt.mutate(&snapshot)
			report := VerifyOffline(snapshot, set, proj, Options{})
			if !hasFinding(report, tt.code, tt.scope) {
				t.Fatalf("missing %s on %s: %#v", tt.code, tt.scope, report.Findings)
			}
		})
	}

	f := newCloseoutFixture(t)
	if _, ok := currentPRD("widget", append(f.snap.PRDs, f.snap.PRDs[0])); ok {
		t.Fatal("duplicate compiled PRDs yielded a current identity")
	}
	if containsFinding([]Finding{{Code: "other", Scope: "scope"}}, "missing", "scope") {
		t.Fatal("containsFinding matched a different code")
	}
}

func TestSnapshotCompilerHelpersRejectMalformedInputs(t *testing.T) {
	if _, ok := prdFeature("docs/prds/widget", "docs/prds"); ok {
		t.Fatal("feature directory without a child file was accepted")
	}
	if isPRDProjection("docs/prds/widget/nested/effective.md", "docs/prds") {
		t.Fatal("nested generated projection was accepted")
	}
	if _, err := sourceMapStatuses([]byte("{not json")); err == nil {
		t.Fatal("malformed source map was accepted")
	}
	if _, err := sourceMapStatuses([]byte(`{"schema":"prd-effective-source/v1"}`)); err == nil || !strings.Contains(err.Error(), "units is absent") {
		t.Fatalf("source map without units error = %v", err)
	}
	findings := prdProjectionFindings(
		PRD{Feature: "widget", Units: []PRDUnit{{ID: "FR-1", Status: "live"}}},
		map[string]string{"FR-2": "live"},
	)
	if len(findings) != 2 || findings[0].Code != "prd-projection-divergence" || !findings[0].Blocking {
		t.Fatalf("projection mismatch findings = %#v", findings)
	}

	snapshot := Snapshot{
		Commit: strings.Repeat("a", 40),
		Files: map[string]File{
			"docs/tech-design/bad.md":     {Path: "docs/tech-design/bad.md", Body: []byte("not a design\n")},
			"docs/tech-design/index.json": {Path: "docs/tech-design/index.json", Body: []byte("{}\n")},
		},
		ExistingPaths: map[string]bool{},
	}
	if err := compileDesignIndex(&snapshot, optionDefaults(Options{})); err == nil || !strings.Contains(err.Error(), "parse technical design") {
		t.Fatalf("malformed design compiler input error = %v", err)
	}

	badPRD := Snapshot{
		Commit:      strings.Repeat("a", 40),
		PRDFeatures: []string{"widget"},
		Files: map[string]File{
			"docs/prds/widget/000-base.md": {Path: "docs/prds/widget/000-base.md", Body: []byte("not a PRD\n")},
		},
		PRDSourceMapUnits: map[string]map[string]string{},
	}
	if err := compilePRDs(&badPRD, optionDefaults(Options{})); err == nil || !strings.Contains(err.Error(), "fold PRD") {
		t.Fatalf("malformed PRD compiler input error = %v", err)
	}
}

func TestCloseoutOrderingHelpers(t *testing.T) {
	if recordInList("missing", []*record.Record{{ID: "present"}}) {
		t.Fatal("recordInList matched an absent id")
	}
	if validDigest(strings.Repeat("a", 63)) || validDigest(strings.Repeat("g", 64)) {
		t.Fatal("invalid digest was accepted")
	}
	if validGitObjectID(strings.Repeat("a", 39)) || validGitObjectID(strings.Repeat("G", 40)) || !validGitObjectID(strings.Repeat("b", 64)) {
		t.Fatal("Git object id validation is not closed")
	}
	if !lessSource(CloseoutSourceDocument{Path: "same", DocumentID: "a"}, CloseoutSourceDocument{Path: "same", DocumentID: "b"}) {
		t.Fatal("source tie-break did not use Document id")
	}
	if !lessCode(CloseoutCodeUnit{Path: "a"}, CloseoutCodeUnit{Path: "b"}) || !lessCode(CloseoutCodeUnit{Path: "same", Kind: codeinventory.ExecutableScript}, CloseoutCodeUnit{Path: "same", Kind: codeinventory.GoPackage}) {
		t.Fatal("code binding order is not path then kind")
	}

	report := Report{
		Findings: []Finding{
			{Code: "same", Scope: "same", Detail: "z"},
			{Code: "same", Scope: "same", Detail: "a"},
			{Code: "same", Scope: "z"},
		},
		Actions: []Action{{Code: "same", Scope: "z"}, {Code: "same", Scope: "a"}},
	}
	sortReport(&report)
	if report.Findings[0].Detail != "a" || report.Actions[0].Scope != "a" {
		t.Fatalf("canonical report order = findings %#v, actions %#v", report.Findings, report.Actions)
	}
}

func cloneCloseoutForCoverage(t *testing.T, receipt CloseoutReceipt) CloseoutReceipt {
	t.Helper()
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var cloned CloseoutReceipt
	if err := json.Unmarshal(body, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func installForgedCoverageReceipt(t *testing.T, f *closeoutFixture, draft Draft, receipt CloseoutReceipt, seq byte) {
	t.Helper()
	receipt.ClosureSHA256 = closeoutDigest(receipt)
	body, err := MarshalCloseout(receipt)
	if err != nil {
		t.Fatalf("forged receipt must remain canonical and structurally valid: %v", err)
	}
	draft.Body = body
	draft.Tags = []string{"reconcile:closeout", "sha256:" + receipt.ClosureSHA256}
	f.addDraft(t, draft, seq)
}

func fixtureTime(seq byte) time.Time {
	return time.Date(2026, time.January, 1, 0, 1, int(seq), 0, time.UTC)
}
