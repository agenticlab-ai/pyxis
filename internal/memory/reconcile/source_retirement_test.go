package reconcile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

type retirementFixture struct {
	snapshot Snapshot
	file     File
	capture  *record.Record
	draft    Draft
	seal     *record.Record
	active   ActiveSourceRetirement
	evidence SourceRetirementEvidence
}

func newRetirementFixture(t *testing.T) retirementFixture {
	t.Helper()
	name := ".pyxis/skills/review/SKILL.md"
	body := []byte("# Historical skill\n")
	f := File{Path: name, Mode: "100644", SHA256: sourceDigest(body), Body: body}
	capture := fixtureRecord(record.Document, "skill/review", "skill", 1)
	capture.LiteralBody = true
	capture.BodySHA256 = f.SHA256
	capture.Provenance.Source = name
	s := Snapshot{Commit: strings.Repeat("a", 40), Files: map[string]File{name: f}, AuthoredSources: []string{name}}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: capture, body: body})
	draft, receipt, err := BuildSourceRetirement(name, "Retire the review surface", strings.Repeat("b", 40), s, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	seal := retirementDraftRecord(draft, 2)
	trustedSet, _ := loadReconcileRecords(t, reconcileRecord{record: capture, body: body}, reconcileRecord{record: seal, body: draft.Body})
	e := SourceRetirementEvidence{Issued: s, IssuedSet: set, Trusted: s, TrustedSet: trustedSet, BeforeDeletion: &f, Introduction: receipt.Introduction, TrustedIntroduction: receipt.Introduction, CurrentIntroduction: receipt.Introduction}
	return retirementFixture{s, f, capture, draft, seal, ActiveSourceRetirement{seal.ID, receipt}, e}
}

func retirementDraftRecord(d Draft, n byte) *record.Record {
	r := fixtureRecord(d.Kind, d.Subject, d.Family, n)
	r.Statement, r.Intent, r.Rationale, r.FitCriterion = d.Statement, d.Intent, d.Rationale, d.FitCriterion
	r.Tags, r.Relations, r.Provenance = d.Tags, d.Relations, d.Provenance
	r.LiteralBody, r.BodySHA256 = true, sourceDigest(d.Body)
	return r
}

func TestStandaloneRetirementClosedCanonicalReceipt(t *testing.T) {
	f := newRetirementFixture(t)
	for _, mode := range []string{"version", "path", "mode", "hash", "capture hash", "commit", "introduction", "capture id", "malformed capture id", "reason", "reason padding"} {
		t.Run(mode, func(t *testing.T) {
			r := f.active.Receipt
			switch mode {
			case "version":
				r.Schema = "memory-source-retirement/v2"
			case "path":
				r.Path = "docs/product-vision.md"
			case "mode":
				r.Mode = "120000"
			case "hash":
				r.SHA256 = strings.Repeat("g", 64)
			case "capture hash":
				r.DocumentSHA256 = ""
			case "commit":
				r.SourceCommit = "HEAD"
			case "introduction":
				r.Introduction = "main"
			case "capture id":
				r.DocumentID = "Need-invalid"
			case "malformed capture id":
				r.DocumentID = "Document-invalid"
			case "reason":
				r.Reason = " "
			case "reason padding":
				r.Reason = " padded "
			}
			if _, err := MarshalSourceRetirement(r); err == nil {
				t.Fatal("invalid receipt marshaled")
			}
		})
	}
	for _, body := range [][]byte{[]byte("{"), append(append([]byte(nil), f.draft.Body...), []byte("{}")...), bytes.Replace(f.draft.Body, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1), bytes.TrimSpace(f.draft.Body)} {
		if _, err := ParseSourceRetirement(body); err == nil {
			t.Fatal("noncanonical receipt accepted")
		}
	}
	for _, name := range []string{".pyxis/skills/review/../SKILL.md", ".pyxis/skills/Review/SKILL.md", ".agents/skills/review/SKILL.md", ".pyxis/vendor/x/other.md", "--source", "/absolute", ".pyxis/skills/review/SKILL.md/extra", ".pyxis/authoring/nested/guide.md", ".pyxis/templates/nested/template.md", "docs/wiki/nested/entry.md", ".pyxis/skills/review/lenses/coverage.txt"} {
		if StandaloneSourcePath(name) {
			t.Fatalf("unsupported source %s", name)
		}
	}
}

func TestStandaloneSourcePathAcceptsMutableAuthoredClasses(t *testing.T) {
	for _, name := range []string{
		".pyxis/skills/review/SKILL.md",
		".pyxis/skills/review/calibration.md",
		".pyxis/skills/review/lenses/coverage.md",
		".pyxis/vendor/review/README.md",
		".pyxis/authoring/review.md",
		".pyxis/templates/review-template.md",
		"docs/wiki/review-reference.md",
	} {
		if !StandaloneSourcePath(name) {
			t.Fatalf("supported mutable source rejected: %s", name)
		}
	}
}

func TestStandaloneRetirementLegacyReplayKeepsSourceObligations(t *testing.T) {
	f := newRetirementFixture(t)
	s := f.snapshot
	s.Files, s.AuthoredSources = map[string]File{}, nil
	s.SourceRetirements = map[string]SourceRetirementProof{f.seal.ID: {Evidence: f.evidence}}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: f.capture, body: f.file.Body}, reconcileRecord{record: f.seal, body: f.draft.Body})
	current := Analyze(s, set, proj, Options{})
	legacy := Analyze(s, set, proj, Options{LegacySourceRetirement: true})
	if current.Blocking() || current.Generator != Generator {
		t.Fatalf("current receipt did not authorize the proven absence: %+v", current)
	}
	if !legacy.Blocking() || legacy.Generator != "memory-reconcile/v4" {
		t.Fatalf("historical replay silently adopted source retirement: %+v", legacy)
	}
}

func TestStandaloneRetirementRejectsConfiguredGovernedSources(t *testing.T) {
	const source = ".pyxis/skills/review/SKILL.md"
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"vision", Options{ProductVision: source}},
		{"principles", Options{ArchitecturePrinciples: source}},
		{"incidents", Options{Incidents: source}},
		{"normalized governing path", Options{ProductVision: "./.pyxis/skills/review/../review/SKILL.md"}},
		{"requirements", Options{PRDs: ".pyxis/skills"}},
		{"designs", Options{TechDesigns: ".pyxis/skills/review"}},
		{"plans", Options{Plans: ".pyxis/skills"}},
		{"brainstorms", Options{Brainstorms: ".pyxis/skills/review"}},
		{"problems", Options{Problems: ".pyxis/skills"}},
		{"reviews", Options{Reviews: ".pyxis/skills/review"}},
		{"feedback", Options{Feedback: ".pyxis/skills"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRetirementFixture(t)
			protected := f.snapshot
			protected.SourceOptions = tc.opts
			if _, _, err := BuildSourceRetirement(source, "Retire the source", f.active.Receipt.Introduction, protected, f.evidence.IssuedSet, resolve.New(f.evidence.IssuedSet)); err == nil || !strings.Contains(err.Error(), "governed source") {
				t.Fatalf("writer admitted a governed source: %v", err)
			}
			for _, stage := range []string{"issuance", "trusted base", "candidate"} {
				t.Run(stage, func(t *testing.T) {
					e, current := f.evidence, f.snapshot
					switch stage {
					case "issuance":
						e.Issued = protected
					case "trusted base":
						e.Trusted = protected
					case "candidate":
						current = protected
					}
					set := f.evidence.TrustedSet
					if err := ValidateSourceRetirement(f.active, e, current, set, resolve.New(set), true); err == nil || !strings.Contains(err.Error(), "governed source") {
						t.Fatalf("reader admitted a governed source: %v", err)
					}
				})
			}
		})
	}
}

func TestStandaloneRetirementSharedValidatorRejectsTampering(t *testing.T) {
	for _, mode := range []string{"valid", "absent", "missing trusted seal", "changed current source", "changed trusted source", "changed issuance", "changed introduction", "changed trusted introduction", "changed historical introduction", "changed capture identity", "missing issuance memory", "missing capture", "conflicting capture", "revoked capture", "missing deletion history", "changed before deletion", "feature owned"} {
		t.Run(mode, func(t *testing.T) {
			f := newRetirementFixture(t)
			e := f.evidence
			s := f.snapshot
			s.Files = map[string]File{f.file.Path: f.file}
			r := f.active
			records := []reconcileRecord{{record: f.capture, body: f.file.Body}, {record: f.seal, body: f.draft.Body}}
			switch mode {
			case "absent":
				s.Files = map[string]File{}
				s.AuthoredSources = nil
			case "missing trusted seal":
				e.TrustedSet = e.IssuedSet
			case "changed current source":
				bad := f.file
				bad.Mode = "100755"
				s.Files[f.file.Path] = bad
			case "changed trusted source":
				e.Trusted.Files = map[string]File{f.file.Path: {Path: f.file.Path, Mode: "100644", SHA256: strings.Repeat("c", 64)}}
			case "changed issuance":
				e.Issued.Commit = strings.Repeat("c", 40)
			case "changed introduction":
				e.CurrentIntroduction = strings.Repeat("c", 40)
			case "changed trusted introduction":
				e.TrustedIntroduction = strings.Repeat("c", 40)
			case "changed historical introduction":
				e.Introduction = strings.Repeat("c", 40)
			case "changed capture identity":
				r.Receipt.DocumentSHA256 = strings.Repeat("c", 64)
			case "missing issuance memory":
				e.IssuedSet = nil
			case "missing capture":
				records = records[1:]
			case "conflicting capture":
				other := *f.capture
				other.ID = fixtureRecord(record.Document, "skill/another", "skill", 3).ID
				other.Subject = "skill/another"
				records = append(records, reconcileRecord{record: &other, body: f.file.Body})
			case "revoked capture":
				rev := fixtureRecord(record.Revocation, f.capture.Subject, "", 3)
				rev.Relations = []record.Relation{{Type: record.Revokes, Target: f.capture.ID}}
				records = append(records, reconcileRecord{record: rev})
			case "missing deletion history":
				s.Files = map[string]File{}
				e.BeforeDeletion = nil
			case "changed before deletion":
				s.Files = map[string]File{}
				bad := f.file
				bad.Mode = "100755"
				e.BeforeDeletion = &bad
			case "feature owned":
				owned := newCloseoutFixture(t)
				e.Issued.PRDs = owned.snap.PRDs
				e.Issued.Designs = []Design{{Slug: "widget", Source: f.file.Path, Status: "live", Serves: []string{"prd:widget/FR-1"}}}
				e.Issued.Files = map[string]File{f.file.Path: f.file, owned.prdPath: owned.snap.Files[owned.prdPath]}
				e.Issued.AuthoredSources = []string{f.file.Path, owned.prdPath}
			}
			set, proj := loadReconcileRecords(t, records...)
			err := ValidateSourceRetirement(r, e, s, set, proj, true)
			valid := mode == "valid" || mode == "absent"
			if (err == nil) != valid {
				t.Fatalf("validator = %v", err)
			}
			if valid {
				s.SourceRetirements = map[string]SourceRetirementProof{r.RecordID: {Evidence: e}}
				report := VerifyOffline(s, set, proj, Options{})
				if report.Blocking() {
					t.Fatalf("reader disagrees: %+v", report.Findings)
				}
				if mode == "absent" && !TrustedStandaloneRetirements(s, set, proj)[f.file.Path] {
					t.Fatal("trusted absence not authorized")
				}
			}
		})
	}
}

func TestStandaloneRetirementSealEnvelopeAndReplacement(t *testing.T) {
	for _, mode := range []string{"kind", "family", "literal", "tag", "subject", "commit", "provenance", "schema note", "body", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			f := newRetirementFixture(t)
			r := *f.seal
			body := f.draft.Body
			switch mode {
			case "kind":
				r.Kind = record.Learning
				r.ID = fixtureRecord(record.Learning, r.Subject, r.Family, 2).ID
				r.LiteralBody = false
				r.BodySHA256 = ""
				body = nil
			case "family":
				r.Family = "wrong"
			case "literal":
				r.LiteralBody = false
				r.BodySHA256 = record.HashBody(body)
			case "tag":
				r.Tags = nil
			case "subject":
				r.Subject = SourceRetirementSubject(".pyxis/vendor/other/README.md")
			case "commit":
				r.Provenance.Commit = strings.Repeat("c", 40)
			case "provenance":
				r.Provenance.Source = "source-retirement:other"
			case "schema note":
				r.Provenance.Note = "other"
			case "body":
				body = append(append([]byte(nil), body...), byte('\n'))
				r.BodySHA256 = sourceDigest(body)
			}
			records := []reconcileRecord{{record: f.capture, body: f.file.Body}, {record: &r, body: body}}
			if mode == "revoked" {
				rev := fixtureRecord(record.Revocation, r.Subject, "", 3)
				rev.Relations = []record.Relation{{Type: record.Revokes, Target: r.ID}}
				records = append(records, reconcileRecord{record: rev})
			}
			set, proj := loadReconcileRecords(t, records...)
			active, err := ActiveSourceRetirements(set, proj)
			if mode == "revoked" {
				if err != nil || len(active) != 0 {
					t.Fatalf("revoked seal: %v %v", active, err)
				}
			} else if err == nil {
				t.Fatal("bad envelope admitted")
			}
		})
	}
	f := newRetirementFixture(t)
	set := f.evidence.TrustedSet
	proj := resolve.New(set)
	draft, r, err := BuildSourceRetirement(f.file.Path, f.active.Receipt.Reason, f.active.Receipt.Introduction, f.snapshot, set, proj)
	if err != nil || draft.Kind != "" || r != f.active.Receipt {
		t.Fatalf("already sealed: %v %+v", err, draft)
	}
	draft, _, err = BuildSourceRetirement(f.file.Path, "A revised retirement reason", f.active.Receipt.Introduction, f.snapshot, set, proj)
	if err != nil || len(draft.Relations) != 1 || draft.Relations[0].Target != f.seal.ID {
		t.Fatalf("replacement: %v %+v", err, draft)
	}
	missing := f.snapshot
	missing.Files = map[string]File{}
	missing.AuthoredSources = nil
	report := Analyze(missing, set, proj, Options{})
	if !hasFinding(report, "source-retirement-history-unavailable", f.file.Path) || !hasFinding(report, "orphan-document-source", f.file.Path) {
		t.Fatalf("missing history hidden: %+v", report.Findings)
	}
}
