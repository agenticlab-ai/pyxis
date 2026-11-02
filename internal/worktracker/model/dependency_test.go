package model

import "testing"

func TestClassifyRef(t *testing.T) {
	cases := []struct {
		ref      string
		wantKind RefKind
		wantID   int
	}{
		{"1", RefLocal, 1},
		{"42", RefLocal, 42},
		{"  7  ", RefLocal, 7},  // trimmed
		{"042", RefLocal, 42},   // leading zero is still an id
		{"0", RefMalformed, 0},  // ids are positive
		{"-3", RefMalformed, 0}, // negatives are not ids
		{"", RefMalformed, 0},   // empty
		{"owner/repo#1", RefCrossRepo, 0},
		{"agenticlab-ai/pyxis#286", RefCrossRepo, 0},
		{"owner/repo#0", RefMalformed, 0},       // non-positive tail
		{"owner/repo#x", RefMalformed, 0},       // non-numeric tail
		{"owner#1", RefMalformed, 0},            // no repo segment
		{"owner/repo/extra#1", RefMalformed, 0}, // too many path segments
		{"/repo#1", RefMalformed, 0},            // empty owner
		{"owner/#1", RefMalformed, 0},           // empty repo
		{"owner/repo", RefMalformed, 0},         // no id tail
		{"not a ref", RefMalformed, 0},          // whitespace
		{"just-text", RefMalformed, 0},
	}
	for _, c := range cases {
		gotKind, gotID := ClassifyRef(c.ref)
		if gotKind != c.wantKind || gotID != c.wantID {
			t.Errorf("ClassifyRef(%q) = (%v, %d), want (%v, %d)", c.ref, gotKind, gotID, c.wantKind, c.wantID)
		}
	}
}

func TestCanonicalRef(t *testing.T) {
	cases := []struct {
		ref       string
		wantCanon string
		wantKind  RefKind
	}{
		{"042", "42", RefLocal},                          // normalized to decimal
		{"  7 ", "7", RefLocal},                          // trimmed
		{" owner/repo#3 ", "owner/repo#3", RefCrossRepo}, // trimmed, otherwise verbatim
		{"garbage", "", RefMalformed},
	}
	for _, c := range cases {
		gotCanon, gotKind, _ := CanonicalRef(c.ref)
		if gotCanon != c.wantCanon || gotKind != c.wantKind {
			t.Errorf("CanonicalRef(%q) = (%q, %v), want (%q, %v)", c.ref, gotCanon, gotKind, c.wantCanon, c.wantKind)
		}
	}
}
