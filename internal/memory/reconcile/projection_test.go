package reconcile

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// A caller may supply a projection it already computed, to save the work. The
// verifier believes it only for the exact set it was computed from.
func TestSuppliedProjectionIsBelievedOnlyForItsOwnSet(t *testing.T) {
	f := newRetirementFixture(t)
	issued, trusted := resolve.New(f.evidence.IssuedSet), resolve.New(f.evidence.TrustedSet)
	if projectionOf(f.evidence.IssuedSet, issued) != issued {
		t.Fatal("the projection of this very set was computed again")
	}
	for name, supplied := range map[string]*resolve.Projection{"no": nil, "another set's": trusted} {
		if got := projectionOf(f.evidence.IssuedSet, supplied); got == supplied || got.Set != f.evidence.IssuedSet {
			t.Fatalf("%s projection was believed for this set", name)
		}
	}

	set, proj := loadReconcileRecords(t, reconcileRecord{record: f.capture, body: f.file.Body}, reconcileRecord{record: f.seal, body: f.draft.Body})
	deleted := f.snapshot
	deleted.Files, deleted.AuthoredSources = map[string]File{}, nil
	for name, supply := range map[string]func(*SourceRetirementEvidence){
		"computed":         func(*SourceRetirementEvidence) {},
		"supplied":         func(e *SourceRetirementEvidence) { e.IssuedProjection, e.TrustedProjection = issued, trusted },
		"wrongly supplied": func(e *SourceRetirementEvidence) { e.IssuedProjection, e.TrustedProjection = trusted, issued },
	} {
		e := f.evidence
		supply(&e)
		if err := ValidateSourceRetirement(f.active, e, deleted, set, proj, true); err != nil {
			t.Fatalf("%s projections changed a valid retirement's verdict: %v", name, err)
		}
		// A trusted base that never held the seal stays refused, even next to
		// the projection of a tree that did hold it.
		e.TrustedSet = e.IssuedSet
		if err := ValidateSourceRetirement(f.active, e, deleted, set, proj, true); err == nil {
			t.Fatalf("%s projections let a missing trusted seal pass", name)
		}
	}
}

func TestContractHistoryProjectionIsBelievedOnlyForItsOwnSet(t *testing.T) {
	f := retiredCompletionFixture(t)
	set, proj := f.load(t)
	history := *f.snap.ContractHistory
	want := VerifyOffline(f.snap, set, proj, Options{})
	// An empty tree's projection knows no predecessor record at all.
	empty, err := store.Load(t.Context(), store.MapSource{})
	if err != nil {
		t.Fatal(err)
	}
	for name, supplied := range map[string]*resolve.Projection{"its own": resolve.New(history.Set), "another set's": resolve.New(empty)} {
		supplying := history
		supplying.Projection = supplied
		f.snap.ContractHistory = &supplying
		if got := VerifyOffline(f.snap, set, proj, Options{}); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s projection changed the report:\n%+v\n%+v", name, got, want)
		}
	}
}
