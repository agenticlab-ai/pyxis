package verdict

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

// autoVerdict and humanVerdict are the two classifier outcomes a record
// projects — an AUTO verdict (a matched class, no floor hits) and a HUMAN
// verdict (floor hits, no class).
func autoVerdict() classifier.Verdict {
	return classifier.Verdict{
		Lane:      classifier.Auto,
		Reason:    "the whole diff is contained by AUTO class drive-doc",
		AutoClass: "drive-doc",
		Pinned:    classifier.Pinned{Base: "base0", Head: "head9", PolicyVersion: "risk-policy/v1", CheckState: "checks0"},
	}
}

func humanVerdict() classifier.Verdict {
	return classifier.Verdict{
		Lane:      classifier.Human,
		Reason:    "floor hit",
		FloorHits: []string{"path:internal/lane/**", "class:governing-doc-edit"},
		Pinned:    classifier.Pinned{Base: "base0", Head: "headA", PolicyVersion: "risk-policy/v1", CheckState: "checks0"},
	}
}

// TestNewRecord_NamesTheFiveProperties pins acceptance (c): a record projects
// exactly lane, floor hits, class, policy version, and head revision off the
// verdict, versioned.
func TestNewRecord_NamesTheFiveProperties(t *testing.T) {
	r := NewRecord(humanVerdict())
	if r.RecordVersion != RecordVersion {
		t.Errorf("record version = %q, want %q", r.RecordVersion, RecordVersion)
	}
	if r.Lane != "HUMAN" {
		t.Errorf("lane = %q, want HUMAN", r.Lane)
	}
	wantHits := []string{"path:internal/lane/**", "class:governing-doc-edit"}
	if len(r.FloorHits) != len(wantHits) {
		t.Fatalf("floor hits = %v, want %v", r.FloorHits, wantHits)
	}
	for i, h := range wantHits {
		if r.FloorHits[i] != h {
			t.Errorf("floor hit %d = %q, want %q", i, r.FloorHits[i], h)
		}
	}
	if r.PolicyVersion != "risk-policy/v1" {
		t.Errorf("policy version = %q, want risk-policy/v1", r.PolicyVersion)
	}
	if r.HeadRevision != "headA" {
		t.Errorf("head revision = %q, want headA", r.HeadRevision)
	}

	// And that every named property is present in the marshaled record — a cold
	// reader must find all five (NFR-4).
	var m map[string]json.RawMessage
	if err := json.Unmarshal(r.Marshal(), &m); err != nil {
		t.Fatalf("marshal is not valid JSON: %v", err)
	}
	for _, key := range []string{"record_version", "lane", "floor_hits", "policy_version", "head_revision"} {
		if _, ok := m[key]; !ok {
			t.Errorf("marshaled record missing %q", key)
		}
	}

	// The AUTO verdict names its class (and carries no floor hits).
	auto := NewRecord(autoVerdict())
	if auto.Class != "drive-doc" {
		t.Errorf("auto class = %q, want drive-doc", auto.Class)
	}
	if len(auto.FloorHits) != 0 {
		t.Errorf("auto record should carry no floor hits, got %v", auto.FloorHits)
	}
}

// TestMarshal_Deterministic pins the record's byte-identical serialization —
// the same verdict yields the same bytes, so a replayed evaluation reproduces
// its record (DINV-4 carried into the record).
func TestMarshal_Deterministic(t *testing.T) {
	a := NewRecord(autoVerdict()).Marshal()
	b := NewRecord(autoVerdict()).Marshal()
	if !bytes.Equal(a, b) {
		t.Errorf("marshal not deterministic:\n a=%s\n b=%s", a, b)
	}
}

// TestAppendBlock_IsAppendOnly pins acceptance (c)'s "appended per evaluation":
// each evaluation appends a record; earlier records are followed, never
// rewritten; and the block parses back into the full evaluation history in order.
func TestAppendBlock_IsAppendOnly(t *testing.T) {
	var block []byte
	block = AppendBlock(block, NewRecord(humanVerdict()))
	first := append([]byte{}, block...)
	block = AppendBlock(block, NewRecord(autoVerdict()))

	// The first record's bytes are still a prefix of the block — it was followed,
	// not rewritten.
	if !bytes.HasPrefix(block, first) {
		t.Errorf("append rewrote earlier history:\n first=%s\n block=%s", first, block)
	}

	recs, err := ParseBlock(block)
	if err != nil {
		t.Fatalf("ParseBlock: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("parsed %d records, want 2", len(recs))
	}
	if recs[0].Lane != "HUMAN" || recs[1].Lane != "AUTO" {
		t.Errorf("evaluation order lost: got %q then %q", recs[0].Lane, recs[1].Lane)
	}
	if recs[0].HeadRevision != "headA" || recs[1].HeadRevision != "head9" {
		t.Errorf("head revisions wrong: %q, %q", recs[0].HeadRevision, recs[1].HeadRevision)
	}
}

// TestParseBlock_ClosedParse pins the block's closed parse: a malformed line or
// a version-drifted record is a named defect, never a silent skip.
func TestParseBlock_ClosedParse(t *testing.T) {
	good := AppendBlock(nil, NewRecord(autoVerdict()))

	if _, err := ParseBlock(append(good, []byte("{not json}\n")...)); err == nil {
		t.Errorf("malformed line accepted; want a defect")
	}

	drift := append([]byte{}, []byte(`{"record_version":"lane-verdict/v2","lane":"AUTO","policy_version":"risk-policy/v1","head_revision":"h"}`+"\n")...)
	if _, err := ParseBlock(drift); err == nil {
		t.Errorf("version-drifted record accepted; want a defect")
	}

	// An empty block is not an error — it is an empty history.
	recs, err := ParseBlock(nil)
	if err != nil || len(recs) != 0 {
		t.Errorf("empty block: got %v, %v; want no records, no error", recs, err)
	}
}
