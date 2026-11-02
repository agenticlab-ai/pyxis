package record

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvidenceClosedSchema(t *testing.T) {
	valid := func() *Record {
		return &Record{ID: "Learning-01ARZ3NDEKTSV4RRFFQ69G5FAV", Subject: "learning/example", Provenance: Provenance{Evidence: &Evidence{Schema: EvidenceSchema, Locator: EvidenceIntroduction, Files: []EvidenceFile{{Path: "internal/example/a.go", Mode: "100644", SHA256: strings.Repeat("a", 64)}}, Units: []EvidenceUnit{{Inventory: EvidenceInventory, Domain: EvidenceUnitDomain, Kind: "go-package", Path: "internal/example", SHA256: strings.Repeat("b", 64), Files: []string{"internal/example/a.go"}}}}}}
	}
	if err := valid().ValidateEvidence(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Record)
	}{
		{"version", func(r *Record) { r.Provenance.Evidence.Schema = "next" }},
		{"locator", func(r *Record) { r.Provenance.Evidence.Locator = "branch" }},
		{"intro commit", func(r *Record) { r.Provenance.Commit = strings.Repeat("a", 40) }},
		{"empty intro", func(r *Record) { r.Provenance.Evidence.Files = nil }},
		{"short stable", func(r *Record) { r.Provenance.Evidence.Locator = EvidenceMainline; r.Provenance.Commit = "abc" }},
		{"mode", func(r *Record) { r.Provenance.Evidence.Files[0].Mode = "120000" }},
		{"hash", func(r *Record) { r.Provenance.Evidence.Files[0].SHA256 = strings.Repeat("A", 64) }},
		{"duplicate", func(r *Record) {
			r.Provenance.Evidence.Files = append(r.Provenance.Evidence.Files, r.Provenance.Evidence.Files[0])
		}},
		{"self", func(r *Record) { r.Provenance.Evidence.Files[0].Path = "memory/" + r.Path() }},
		{"unit order", func(r *Record) {
			r.Provenance.Evidence.Units = append(r.Provenance.Evidence.Units, r.Provenance.Evidence.Units[0])
		}},
		{"inventory", func(r *Record) { r.Provenance.Evidence.Units[0].Inventory = "unknown" }},
		{"domain", func(r *Record) { r.Provenance.Evidence.Units[0].Domain = "wrong" }},
		{"kind", func(r *Record) { r.Provenance.Evidence.Units[0].Kind = "model" }},
		{"unit hash", func(r *Record) { r.Provenance.Evidence.Units[0].SHA256 = "bad" }},
		{"empty members", func(r *Record) { r.Provenance.Evidence.Units[0].Files = nil }},
		{"missing member", func(r *Record) { r.Provenance.Evidence.Units[0].Files = []string{"internal/example/b.go"} }},
		{"duplicate member", func(r *Record) {
			r.Provenance.Evidence.Units[0].Files = append(r.Provenance.Evidence.Units[0].Files, r.Provenance.Evidence.Units[0].Files[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.change(r)
			if err := r.ValidateEvidence(); err == nil {
				t.Fatal("accepted malformed evidence")
			}
		})
	}
	r := valid()
	r.Provenance.Evidence = &Evidence{Schema: EvidenceSchema, Locator: EvidenceMainline}
	r.Provenance.Commit = strings.Repeat("b", 40)
	if err := r.ValidateEvidence(); err != nil {
		t.Fatal(err)
	}
	r.Provenance.Evidence = nil
	if err := r.ValidateEvidence(); err != nil {
		t.Fatal(err)
	}
	var p Provenance
	dec := json.NewDecoder(strings.NewReader(`{"evidence":{"schema_version":"memory-evidence/v1","surprise":true}}`))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err == nil {
		t.Fatal("unknown nested field accepted")
	}
}

func TestEvidencePathsAndCommitIDs(t *testing.T) {
	for _, p := range []string{"", ".", "..", "/etc/passwd", "../a", "a/../b", "a//b", "a\\b", "a:b", "-p", "a\nfile", "a\x00b", "a\x1fb", "a\x7fb"} {
		if ValidEvidencePath(p) {
			t.Errorf("accepted path %q", p)
		}
	}
	for _, p := range []string{"internal/a.go", ".pyxis/config.yaml", "docs/a file.md"} {
		if !ValidEvidencePath(p) {
			t.Errorf("rejected path %q", p)
		}
	}
	for _, id := range []string{"HEAD", strings.Repeat("0", 40), strings.Repeat("a", 39), strings.Repeat("A", 40)} {
		if ValidCommitID(id) {
			t.Errorf("accepted %q", id)
		}
	}
	for _, n := range []int{40, 64} {
		if !ValidCommitID(strings.Repeat("a", n)) {
			t.Fatal(n)
		}
	}
}
