package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func TestProvenanceCarriesInputAndRejectsEmptyRoot(t *testing.T) {
	want := report.Provenance{Model: "review-model"}
	id, got, err := Provenance("reviewer-root", want)
	if err != nil {
		t.Fatalf("Provenance(valid): %v", err)
	}
	if id == "" || got != want {
		t.Fatalf("Provenance(valid) = (%q, %+v), want a context id and %+v", id, got, want)
	}
	if _, got, err := Provenance("", want); err == nil || got != (report.Provenance{}) {
		t.Fatalf("Provenance(empty) = (%+v, %v), want zero provenance and error", got, err)
	}
}

// TestCheckFreshInstance covers the would-fail-if-inherited leg (A-6, DINV-5):
// the check FAILS when the reviewer context equals or derives from the
// implementing session's, and PASSES when the two are genuinely distinct
// sessions (a true sibling spawn).
func TestCheckFreshInstance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		reviewer     string
		implementer  string
		wantErr      error // nil = must pass
		wantContains string
	}{
		{
			name:        "distinct sibling sessions pass",
			reviewer:    "reviewer-session-root-abc",
			implementer: "implementer-session-root-xyz",
			wantErr:     nil,
		},
		{
			name:         "reviewer equals implementer fails (a child reuses the parent identity)",
			reviewer:     "shared-session-root",
			implementer:  "shared-session-root",
			wantErr:      ErrInheritedContext,
			wantContains: "equals",
		},
		{
			name:         "reviewer derives from implementer fails (seeded from the implementer, whitespace only)",
			reviewer:     "  implementer-root  ",
			implementer:  "implementer-root",
			wantErr:      ErrInheritedContext, // trims to the same root → derives-from
			wantContains: "",
		},
		{
			name:        "empty reviewer root fails closed",
			reviewer:    "",
			implementer: "implementer-root",
			wantErr:     ErrEmptyRoot,
		},
		{
			name:        "empty implementer root fails closed",
			reviewer:    "reviewer-root",
			implementer: "",
			wantErr:     ErrEmptyRoot,
		},
		{
			name:        "both empty fail closed",
			reviewer:    "",
			implementer: "",
			wantErr:     ErrEmptyRoot,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckFreshInstance(tc.reviewer, tc.implementer)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("want pass, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
			if tc.wantContains != "" && !strings.Contains(err.Error(), tc.wantContains) {
				t.Errorf("want error containing %q, got %q", tc.wantContains, err.Error())
			}
		})
	}
}

// TestContextIDDeterministicAndDistinct — the reviewer-root-derived context_id
// is deterministic (a round's provenance is reproducible) and distinct roots
// yield distinct ids (§3.4).
func TestContextIDDeterministicAndDistinct(t *testing.T) {
	t.Parallel()

	a1, err := ContextID("reviewer-root-1")
	if err != nil {
		t.Fatalf("context id: %v", err)
	}
	a2, err := ContextID("reviewer-root-1")
	if err != nil {
		t.Fatalf("context id (repeat): %v", err)
	}
	if a1 != a2 {
		t.Fatalf("ContextID is not deterministic: %q != %q", a1, a2)
	}
	if !strings.HasPrefix(a1, contextIDPrefix) {
		t.Errorf("context id %q missing prefix %q", a1, contextIDPrefix)
	}

	b, err := ContextID("reviewer-root-2")
	if err != nil {
		t.Fatalf("context id 2: %v", err)
	}
	if a1 == b {
		t.Errorf("distinct reviewer roots must yield distinct context ids")
	}
}

// TestContextIDEmptyRootFailsClosed — an empty reviewer root has no derivable
// fresh-instance identity, so ContextID fails closed.
func TestContextIDEmptyRootFailsClosed(t *testing.T) {
	t.Parallel()
	if _, err := ContextID("   "); !errors.Is(err, ErrEmptyRoot) {
		t.Fatalf("want ErrEmptyRoot on empty root, got %v", err)
	}
}
