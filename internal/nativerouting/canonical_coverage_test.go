package nativerouting

import (
	"testing"
	"time"
)

func TestCanonicalQualificationOrdersTiedClaimsAndInvalidations(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	q := Qualification{
		Claims: []QualificationClaim{
			{Component: ComponentAssurance, AssertedLevel: "z"},
			{Component: ComponentAssurance, AssertedLevel: "a"},
		},
		Validity: Validity{Invalidations: []Invalidation{
			{At: now, EventID: "z"},
			{At: now.Add(-time.Hour), EventID: "middle"},
			{At: now, EventID: "a"},
		}},
	}
	got := canonicalQualification(q)
	if got.Claims[0].AssertedLevel != "a" {
		t.Fatalf("claims not canonical: %+v", got.Claims)
	}
	if got.Validity.Invalidations[0].EventID != "middle" || got.Validity.Invalidations[1].EventID != "a" || got.Validity.Invalidations[2].EventID != "z" {
		t.Fatalf("invalidations not canonical: %+v", got.Validity.Invalidations)
	}
}
