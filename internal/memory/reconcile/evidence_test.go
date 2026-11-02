package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func evidenceFixture(t *testing.T) (*record.Record, EvidenceSnapshot) {
	t.Helper()
	s := EvidenceSnapshot{Commit: strings.Repeat("a", 40), Entries: map[string]gitsrc.TreeEntry{}, Files: map[string][]byte{}}
	for _, p := range []string{"internal/widget/a.go", "internal/widget/b.go", "internal/widget/a_test.go", "docs/design.md"} {
		s.Entries[p] = gitsrc.TreeEntry{Path: p, Mode: "100644", Type: "blob"}
		s.Files[p] = []byte("exact bytes of " + p + "\r\n")
	}
	buildEvidenceUnits(t, &s)
	r := fixtureRecord(record.Learning, "learning/widget", "", 1)
	r.Provenance.Source = "docs/design.md#D-1"
	r.LearnedFrom = []string{"internal/widget/a_test.go", "incident:I2", "a useful narrative"}
	r.Tags = []string{"code:internal/widget"}
	return r, s
}

func buildEvidenceUnits(t *testing.T, s *EvidenceSnapshot) {
	t.Helper()
	var entries []codeinventory.Entry
	for _, e := range s.Entries {
		entries = append(entries, codeinventory.Entry{Path: e.Path, Mode: e.Mode, Type: e.Type})
	}
	inv, err := codeinventory.Build(entries, s.Files)
	if err != nil {
		t.Fatal(err)
	}
	s.Units = inv.Units
}

func TestEvidenceCapturesCompleteUnitAndAllDeclaredFiles(t *testing.T) {
	r, s := evidenceFixture(t)
	e, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Files) != 4 || len(e.Units) != 1 || len(e.Units[0].Files) != 2 {
		t.Fatalf("incomplete witness: %+v", e)
	}
	r.Provenance.Evidence = e
	if err := r.ValidateEvidence(); err != nil {
		t.Fatal(err)
	}
	again, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction)
	if err != nil || !reflect.DeepEqual(e, again) {
		t.Fatal("capture was not deterministic", err)
	}
	if diff := CompareEvidence(e, again); len(diff) != 0 {
		t.Fatal(diff)
	}
	anchor := fixtureRecord(record.Need, "code/widget", "anchor", 2)
	anchor.Tags = r.Tags
	r.Tags = nil
	r.Relations = []record.Relation{{Type: record.About, Target: anchor.ID}}
	viaAbout, err := CaptureEvidence(r, &store.Set{Records: map[string]*record.Record{anchor.ID: anchor}}, s, record.EvidenceIntroduction)
	if err != nil || !reflect.DeepEqual(e, viaAbout) {
		t.Fatalf("about association: %+v %v", viaAbout, err)
	}
}

func TestEvidenceExactMutations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*EvidenceSnapshot)
	}{
		{"byte", func(s *EvidenceSnapshot) { s.Files["internal/widget/a.go"] = []byte("different bytes") }},
		{"mode", func(s *EvidenceSnapshot) {
			e := s.Entries["internal/widget/a.go"]
			e.Mode = "100755"
			s.Entries[e.Path] = e
		}},
		{"new member", func(s *EvidenceSnapshot) {
			p := "internal/widget/c.go"
			s.Entries[p] = gitsrc.TreeEntry{Path: p, Mode: "100644", Type: "blob"}
			s.Files[p] = []byte("package widget")
		}},
		{"removed member", func(s *EvidenceSnapshot) {
			delete(s.Entries, "internal/widget/b.go")
			delete(s.Files, "internal/widget/b.go")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, s := evidenceFixture(t)
			expected, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&s)
			buildEvidenceUnits(t, &s)
			observed, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction)
			if err != nil {
				t.Fatal(err)
			}
			if len(CompareEvidence(expected, observed)) == 0 {
				t.Fatal("mutation passed")
			}
		})
	}
	r, s := evidenceFixture(t)
	e, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction)
	if err != nil {
		t.Fatal(err)
	}
	missing := *e
	missing.Files = append([]record.EvidenceFile(nil), e.Files[:3]...)
	if len(CompareEvidence(&missing, e)) == 0 {
		t.Fatal("omitted cited test passed")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*record.Record, *EvidenceSnapshot)
	}{
		{"missing test", func(_ *record.Record, s *EvidenceSnapshot) { delete(s.Entries, "internal/widget/a_test.go") }},
		{"symlink", func(_ *record.Record, s *EvidenceSnapshot) {
			e := s.Entries["docs/design.md"]
			e.Mode = "120000"
			s.Entries[e.Path] = e
		}},
		{"missing blob", func(_ *record.Record, s *EvidenceSnapshot) { delete(s.Files, "docs/design.md") }},
		{"scope", func(r *record.Record, _ *EvidenceSnapshot) { r.Tags = []string{"code:internal/absent"} }},
		{"empty scope", func(r *record.Record, _ *EvidenceSnapshot) { r.Tags = []string{"code:"} }},
		{"unsafe scope", func(r *record.Record, _ *EvidenceSnapshot) { r.Tags = []string{"code:../secret"} }},
		{"unsafe source", func(r *record.Record, _ *EvidenceSnapshot) { r.Provenance.Source = "../secret" }},
		{"self", func(r *record.Record, _ *EvidenceSnapshot) { r.Provenance.Source = "memory/" + r.Path() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, s := evidenceFixture(t)
			tc.mutate(r, &s)
			if _, err := CaptureEvidence(r, nil, s, record.EvidenceIntroduction); err == nil {
				t.Fatal("invalid selection passed")
			}
		})
	}
}
